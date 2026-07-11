"""Official NCBI Variation Services adapter for canonical allele normalization.

The adapter uses only the documented HTTPS Variation Services HGVS and SPDI
endpoints.  It intentionally receives a Task 5.2 :class:`SourceHttpClient`
rather than constructing an HTTP client itself, so TLS, host policy, response
bounds, retries, and circuit handling remain at the shared boundary.
"""

from __future__ import annotations

import asyncio
import hashlib
import json
import time
from collections.abc import Callable, Mapping
from datetime import UTC, datetime
from typing import Literal, TypedDict, TypeGuard
from urllib.parse import quote

from acmg_classifier.domain.enums import GenomeBuild
from acmg_classifier.domain.errors import JsonValue
from acmg_classifier.domain.models import InterpretationContext
from acmg_classifier.domain.normalization import (
    CanonicalAlleleKey,
    HgvsAlias,
    NormalizationFailureCode,
    NormalizedVariant,
    ParsedVariant,
    ProviderProvenance,
    VariantNotation,
)
from acmg_classifier.infrastructure.http.policy import (
    HttpOutcome,
    HttpPolicy,
    HttpRequest,
    HttpStatusKind,
    SourceHttpClient,
)
from acmg_classifier.ports.normalization import (
    NormalizationFailure,
    NormalizationPolicy,
    NormalizationProviderResult,
    NormalizationSuccess,
)

_NCBI_VARIATION_HOST = "api.ncbi.nlm.nih.gov"
_NCBI_VARIATION_BASE_URL = f"https://{_NCBI_VARIATION_HOST}/variation/v0"
_NCBI_VARIATION_VERSION = "0.1.10"
_NCBI_MIN_REQUEST_INTERVAL_SECONDS = 1.0
_NCBI_ASSEMBLY_ACCESSIONS = {
    GenomeBuild.GRCH38: "GCF_000001405.40",
    GenomeBuild.GRCH37: "GCF_000001405.25",
}

# These versioned primary-reference accessions identify the public human
# assemblies without guessing from an unversioned chromosome label.  Alternate
# loci are deliberately not inferred: callers must provide an assembly for
# them, which prevents false build provenance.
_GRCH38_ACCESSIONS = {
    "NC_000001.11",
    "NC_000002.12",
    "NC_000003.12",
    "NC_000004.12",
    "NC_000005.10",
    "NC_000006.12",
    "NC_000007.14",
    "NC_000008.11",
    "NC_000009.12",
    "NC_000010.11",
    "NC_000011.10",
    "NC_000012.12",
    "NC_000013.11",
    "NC_000014.9",
    "NC_000015.10",
    "NC_000016.10",
    "NC_000017.11",
    "NC_000018.10",
    "NC_000019.10",
    "NC_000020.11",
    "NC_000021.9",
    "NC_000022.11",
    "NC_000023.11",
    "NC_000024.10",
}
_GRCH37_ACCESSIONS = {
    "NC_000001.10",
    "NC_000002.11",
    "NC_000003.11",
    "NC_000004.11",
    "NC_000005.9",
    "NC_000006.11",
    "NC_000007.13",
    "NC_000008.10",
    "NC_000009.11",
    "NC_000010.10",
    "NC_000011.9",
    "NC_000012.11",
    "NC_000013.10",
    "NC_000014.8",
    "NC_000015.9",
    "NC_000016.9",
    "NC_000017.10",
    "NC_000018.9",
    "NC_000019.9",
    "NC_000020.10",
    "NC_000021.8",
    "NC_000022.10",
    "NC_000023.10",
    "NC_000024.9",
}
_ASSEMBLY_BY_ACCESSION = {
    **{accession: GenomeBuild.GRCH38 for accession in _GRCH38_ACCESSIONS},
    **{accession: GenomeBuild.GRCH37 for accession in _GRCH37_ACCESSIONS},
}


def _is_json_object(value: object) -> TypeGuard[Mapping[str, object]]:
    """Narrow an untrusted decoded JSON value to a string-keyed object."""
    return isinstance(value, Mapping) and all(isinstance(key, str) for key in value)


class _SpdiAllele(TypedDict):
    """Validated SPDI fields returned by the NCBI endpoints."""

    seq_id: str
    position: int
    deleted_sequence: str
    inserted_sequence: str


def ncbi_variation_http_policy() -> HttpPolicy:
    """Return the restrictive default policy for NCBI Variation Services.

    Variation Services asks clients to limit requests to one per second.  The
    provider enforces that spacing between its own requests; the one-second
    retry backoff also avoids turning a transient source error into a burst.
    """
    return HttpPolicy(
        source_id="ncbi_variation",
        allowed_hosts=frozenset({_NCBI_VARIATION_HOST}),
        timeout_seconds=10.0,
        deadline_seconds=30.0,
        max_response_bytes=1_000_000,
        max_retries=1,
        base_backoff_seconds=1.0,
        max_backoff_seconds=5.0,
        user_agent="acmg-classifier/0.1 ncbi-variation",
    )


class NCBIVariationNormalizationProvider:
    """Normalize a parsed variant through official NCBI Variation Services."""

    provider_id = "ncbi_variation"
    capabilities: frozenset[Literal["live", "offline"]] = frozenset({"live"})

    def __init__(
        self,
        *,
        client: SourceHttpClient,
        clock: Callable[[], datetime] = lambda: datetime.now(UTC),
        monotonic: Callable[[], float] = time.monotonic,
        sleep: Callable[[float], None] = time.sleep,
    ) -> None:
        if client.policy.allowed_hosts != frozenset({_NCBI_VARIATION_HOST}):
            raise ValueError(
                "NCBI Variation client must allow only api.ncbi.nlm.nih.gov"
            )
        self._client = client
        self._clock = clock
        self._monotonic = monotonic
        self._sleep = sleep
        self._last_request_at: float | None = None

    def normalize(
        self,
        parsed: ParsedVariant,
        context: InterpretationContext | None,
        policy: NormalizationPolicy,
    ) -> NormalizationProviderResult:
        """Return a canonical NCBI allele or a typed, non-fabricated failure."""
        if policy.mode == "offline":
            return self._unavailable(parsed, "offline_policy")
        if not policy.allow_remote:
            return self._unavailable(parsed, "remote_disabled")

        requested_build = policy.genome_build or (
            context.genome_build if context else None
        )
        if (
            parsed.accession is not None
            and parsed.accession.lower().startswith("chr")
            and requested_build is None
        ):
            return NormalizationFailure(
                code=NormalizationFailureCode.GENOME_BUILD_REQUIRED,
                message=(
                    "a genome build is required for chromosome-based NCBI HGVS input"
                ),
                retryable=False,
                provider_id=self.provider_id,
                field="genome_build",
                details={"query_key": parsed.normalized_input},
            )

        contextual = self._request_json(self._contextual_url(parsed, requested_build))
        if isinstance(contextual, NormalizationFailure):
            return contextual
        contextual_payload, contextual_raw = contextual

        contextual_allele = self._contextual_allele(parsed, contextual_payload)
        if isinstance(contextual_allele, NormalizationFailure):
            return contextual_allele

        spdi = self._spdi_string(contextual_allele)
        encoded_spdi = quote(spdi, safe="")
        canonical_url = (
            f"{_NCBI_VARIATION_BASE_URL}/spdi/{encoded_spdi}/canonical_representative"
        )
        canonical = self._request_json(canonical_url)
        if isinstance(canonical, NormalizationFailure):
            return canonical
        canonical_payload, canonical_raw = canonical
        canonical_allele = self._canonical_allele(parsed, canonical_payload)
        if isinstance(canonical_allele, NormalizationFailure):
            return canonical_allele

        assembly = self._assembly_for(canonical_allele["seq_id"], requested_build)
        if isinstance(assembly, NormalizationFailure):
            return assembly
        try:
            canonical_key = CanonicalAlleleKey(
                assembly=assembly,
                sequence_accession=canonical_allele["seq_id"],
                start=canonical_allele["position"],
                end=canonical_allele["position"]
                + len(canonical_allele["deleted_sequence"]),
                deleted_sequence=canonical_allele["deleted_sequence"],
                inserted_sequence=canonical_allele["inserted_sequence"],
            )
        except ValueError as error:
            return self._schema_drift(
                parsed, f"NCBI canonical SPDI is invalid: {error}"
            )

        raw_reference = hashlib.sha256(
            contextual_raw + b"\n" + canonical_raw
        ).hexdigest()
        provenance = ProviderProvenance(
            provider_id=self.provider_id,
            provider_version=_NCBI_VARIATION_VERSION,
            source_record_id=None,
            retrieved_at=self._clock(),
            bundle_version=None,
            raw_snapshot_ref=f"sha256:{raw_reference}",
            query_key=parsed.normalized_input,
        )
        alias = HgvsAlias(
            expression=parsed.normalized_input,
            notation=self._alias_notation(parsed.notation),
            accession=parsed.accession,
            source="input",
        )
        normalized = NormalizedVariant(
            original_input=parsed.original_input,
            parsed_input=parsed.normalized_input,
            canonical_key=canonical_key,
            genome_build=assembly,
            genomic_accession=canonical_key.sequence_accession,
            genomic_start=canonical_key.start,
            genomic_end=canonical_key.end,
            reference_allele=canonical_key.deleted_sequence,
            alternate_allele=canonical_key.inserted_sequence,
            normalized_hgvs=alias.expression,
            hgvs_aliases=(alias,),
            gene_symbol=parsed.gene_symbol,
            transcript=parsed.accession
            if parsed.notation is VariantNotation.TRANSCRIPT_HGVS
            else None,
            transcript_hgvs=(
                parsed.normalized_input
                if parsed.notation is VariantNotation.TRANSCRIPT_HGVS
                else None
            ),
            provider_provenance=(provenance,),
        )
        # The canonical representative endpoint verifies the SPDI returned by
        # contextualization.  Returning it allows application orchestration to
        # enforce the provider round-trip invariant.
        return NormalizationSuccess(normalized=normalized, round_trip_key=canonical_key)

    def _request_json(
        self, url: str
    ) -> tuple[Mapping[str, object], bytes] | NormalizationFailure:
        try:
            asyncio.get_running_loop()
        except RuntimeError:
            pass
        else:
            # The public provider port is synchronous.  Do not create a nested
            # event loop and risk bypassing the injected client policy.
            return self._active_event_loop_failure()
        self._wait_for_ncbi_slot()
        try:
            outcome = asyncio.run(
                self._client.request(HttpRequest("GET", url, read_only=True))
            )
        except RuntimeError:
            return self._active_event_loop_failure()
        failure = self._outcome_failure(outcome)
        if failure is not None:
            return failure
        try:
            decoded = json.loads(outcome.body)
        except (UnicodeDecodeError, json.JSONDecodeError):
            return self._schema_drift_for_response("NCBI returned non-JSON data")
        if not _is_json_object(decoded):
            return self._schema_drift_for_response(
                "NCBI JSON response must be an object"
            )
        return decoded, outcome.body

    def _active_event_loop_failure(self) -> NormalizationFailure:
        return NormalizationFailure(
            code=NormalizationFailureCode.PROVIDER_OUTAGE,
            message="NCBI normalization cannot run from an active event loop",
            retryable=True,
            provider_id=self.provider_id,
            details={"reason": "active_event_loop"},
        )

    def _wait_for_ncbi_slot(self) -> None:
        now = self._monotonic()
        if self._last_request_at is not None:
            delay = _NCBI_MIN_REQUEST_INTERVAL_SECONDS - (now - self._last_request_at)
            if delay > 0:
                self._sleep(delay)
                now += delay
        self._last_request_at = now

    def _contextual_url(self, parsed: ParsedVariant, build: GenomeBuild | None) -> str:
        encoded_hgvs = quote(parsed.normalized_input, safe="")
        url = "/".join((_NCBI_VARIATION_BASE_URL, "hgvs", encoded_hgvs, "contextuals"))
        if parsed.accession is not None and parsed.accession.lower().startswith("chr"):
            assert build is not None
            return f"{url}?assembly={_NCBI_ASSEMBLY_ACCESSIONS[build]}"
        return url

    def _contextual_allele(
        self, parsed: ParsedVariant, payload: Mapping[str, object]
    ) -> _SpdiAllele | NormalizationFailure:
        if self._is_reference_mismatch(payload):
            return NormalizationFailure(
                code=NormalizationFailureCode.REFERENCE_MISMATCH,
                message=(
                    "NCBI reports that the supplied reference allele does not match"
                ),
                retryable=False,
                provider_id=self.provider_id,
                field="variant",
                details={
                    "reason": "reference_mismatch",
                    "query_key": parsed.normalized_input,
                },
            )
        if isinstance(payload.get("code"), str):
            return NormalizationFailure(
                code=NormalizationFailureCode.ALLELE_NORMALIZATION_FAILED,
                message="NCBI rejected the HGVS query",
                retryable=False,
                provider_id=self.provider_id,
                field="variant",
                details={
                    "reason": "invalid_query",
                    "query_key": parsed.normalized_input,
                },
            )
        data = payload.get("data")
        if not isinstance(data, Mapping):
            return self._schema_drift(
                parsed, "NCBI contextual response is missing data"
            )
        validity = data.get("input_hgvs_validity")
        if validity == "invalid":
            return NormalizationFailure(
                code=NormalizationFailureCode.ALLELE_NORMALIZATION_FAILED,
                message="NCBI rejected the HGVS query as invalid",
                retryable=False,
                provider_id=self.provider_id,
                field="variant",
                details={
                    "reason": "invalid_query",
                    "query_key": parsed.normalized_input,
                },
            )
        if validity != "valid":
            return self._schema_drift(
                parsed, "NCBI contextual response has no valid HGVS status"
            )
        errors = data.get("errors")
        if self._is_reference_mismatch(errors):
            return NormalizationFailure(
                code=NormalizationFailureCode.REFERENCE_MISMATCH,
                message=(
                    "NCBI reports that the supplied reference allele does not match"
                ),
                retryable=False,
                provider_id=self.provider_id,
                field="variant",
                details={
                    "reason": "reference_mismatch",
                    "query_key": parsed.normalized_input,
                },
            )
        spdis = data.get("spdis")
        if not isinstance(spdis, list):
            return self._schema_drift(
                parsed, "NCBI contextual response is missing data.spdis"
            )
        if not spdis:
            return self._unavailable(parsed, "no_record")
        if len(spdis) != 1:
            return NormalizationFailure(
                code=NormalizationFailureCode.ALLELE_NORMALIZATION_FAILED,
                message="NCBI returned multiple contextual alleles for one query",
                retryable=False,
                provider_id=self.provider_id,
                details={
                    "reason": "ambiguous_contextual_alleles",
                    "query_key": parsed.normalized_input,
                },
            )
        return self._validated_spdi(parsed, spdis[0], "contextual")

    def _canonical_allele(
        self, parsed: ParsedVariant, payload: Mapping[str, object]
    ) -> _SpdiAllele | NormalizationFailure:
        return self._validated_spdi(
            parsed, payload.get("data"), "canonical representative"
        )

    def _validated_spdi(
        self, parsed: ParsedVariant, candidate: object, response_name: str
    ) -> _SpdiAllele | NormalizationFailure:
        if not isinstance(candidate, Mapping):
            return self._schema_drift(
                parsed, f"NCBI {response_name} response is missing an SPDI object"
            )
        required = ("seq_id", "position", "deleted_sequence", "inserted_sequence")
        if any(name not in candidate for name in required):
            return self._schema_drift(
                parsed, f"NCBI {response_name} SPDI is missing required fields"
            )
        seq_id = candidate["seq_id"]
        position = candidate["position"]
        deleted = candidate["deleted_sequence"]
        inserted = candidate["inserted_sequence"]
        if (
            not isinstance(seq_id, str)
            or not isinstance(position, int)
            or isinstance(position, bool)
            or not isinstance(deleted, str)
            or not isinstance(inserted, str)
        ):
            return self._schema_drift(
                parsed, f"NCBI {response_name} SPDI fields have invalid types"
            )
        return {
            "seq_id": seq_id.upper(),
            "position": position,
            "deleted_sequence": deleted.upper(),
            "inserted_sequence": inserted.upper(),
        }

    def _assembly_for(
        self, accession: str, requested_build: GenomeBuild | None
    ) -> GenomeBuild | NormalizationFailure:
        resolved = _ASSEMBLY_BY_ACCESSION.get(accession)
        if requested_build is not None:
            if resolved is not None and resolved != requested_build:
                return NormalizationFailure(
                    code=NormalizationFailureCode.ALLELE_NORMALIZATION_FAILED,
                    message=(
                        "NCBI response accession does not match the requested "
                        "genome build"
                    ),
                    retryable=False,
                    provider_id=self.provider_id,
                    field="genome_build",
                    details={"reason": "genome_build_mismatch", "accession": accession},
                )
            return requested_build
        if resolved is None:
            return NormalizationFailure(
                code=NormalizationFailureCode.GENOME_BUILD_REQUIRED,
                message=(
                    "NCBI response accession cannot be assigned a supported "
                    "genome build"
                ),
                retryable=False,
                provider_id=self.provider_id,
                field="genome_build",
                details={"reason": "unmapped_accession", "accession": accession},
            )
        return resolved

    def _outcome_failure(self, outcome: HttpOutcome) -> NormalizationFailure | None:
        mapping = {
            HttpStatusKind.NO_RECORD: (
                NormalizationFailureCode.NORMALIZATION_UNAVAILABLE,
                "NCBI has no record for this normalized allele",
                False,
                "no_record",
            ),
            HttpStatusKind.RATE_LIMITED: (
                NormalizationFailureCode.PROVIDER_RATE_LIMITED,
                "NCBI Variation Services rate limited this request",
                True,
                "rate_limited",
            ),
            HttpStatusKind.TIMEOUT: (
                NormalizationFailureCode.PROVIDER_TIMEOUT,
                "NCBI Variation Services request timed out",
                True,
                "timeout",
            ),
            HttpStatusKind.UNAVAILABLE: (
                NormalizationFailureCode.PROVIDER_OUTAGE,
                "NCBI Variation Services is unavailable",
                True,
                "outage",
            ),
            HttpStatusKind.INVALID_QUERY: (
                NormalizationFailureCode.ALLELE_NORMALIZATION_FAILED,
                "NCBI Variation Services rejected the request",
                False,
                "invalid_query",
            ),
            HttpStatusKind.RESPONSE_TOO_LARGE: (
                NormalizationFailureCode.PROVIDER_OUTAGE,
                "NCBI Variation Services response exceeded the configured limit",
                True,
                "response_too_large",
            ),
        }
        if outcome.kind is HttpStatusKind.SUCCESS:
            return None
        code, message, retryable, reason = mapping[outcome.kind]
        details: dict[str, JsonValue] = {
            "reason": reason,
            "attempts": outcome.attempts,
        }
        if outcome.retry_after_seconds is not None:
            details["retry_after_seconds"] = outcome.retry_after_seconds
        return NormalizationFailure(
            code=code,
            message=message,
            retryable=retryable,
            provider_id=self.provider_id,
            details=details,
        )

    def _unavailable(self, parsed: ParsedVariant, reason: str) -> NormalizationFailure:
        return NormalizationFailure(
            code=NormalizationFailureCode.NORMALIZATION_UNAVAILABLE,
            message=(
                "NCBI Variation Services was not permitted or had no matching allele"
            ),
            retryable=False,
            provider_id=self.provider_id,
            details={
                "reason": reason,
                "query_key": parsed.normalized_input,
                "status": "unavailable",
            },
        )

    def _schema_drift(
        self, parsed: ParsedVariant, message: str
    ) -> NormalizationFailure:
        return NormalizationFailure(
            code=NormalizationFailureCode.PROVIDER_SCHEMA_DRIFT,
            message=message,
            retryable=False,
            provider_id=self.provider_id,
            details={"reason": "schema_drift", "query_key": parsed.normalized_input},
        )

    def _schema_drift_for_response(self, message: str) -> NormalizationFailure:
        return NormalizationFailure(
            code=NormalizationFailureCode.PROVIDER_SCHEMA_DRIFT,
            message=message,
            retryable=False,
            provider_id=self.provider_id,
            details={"reason": "schema_drift"},
        )

    @staticmethod
    def _spdi_string(allele: _SpdiAllele) -> str:
        return ":".join(
            (
                allele["seq_id"],
                str(allele["position"]),
                allele["deleted_sequence"],
                allele["inserted_sequence"],
            )
        )

    @staticmethod
    def _alias_notation(
        notation: VariantNotation,
    ) -> Literal["genomic", "transcript", "protein"]:
        if notation is VariantNotation.GENOMIC_HGVS:
            return "genomic"
        return "transcript"

    @staticmethod
    def _is_reference_mismatch(errors: object) -> bool:
        if not isinstance(errors, Mapping):
            return False
        flattened = " ".join(str(value).lower() for value in errors.values())
        return "reference" in flattened and (
            "mismatch" in flattened or "match" in flattened
        )
