"""Official gnomAD GraphQL population-evidence adapter.

The adapter deliberately keeps gnomAD's exome, genome, and joint frequency
strata separate. It makes no ACMG criterion decision.
"""

from __future__ import annotations

import hashlib
import json
import re
from collections.abc import Callable, Mapping
from dataclasses import dataclass
from datetime import UTC, datetime, timedelta

from acmg_classifier.domain.enums import GenomeBuild
from acmg_classifier.domain.evidence import (
    EvidenceContextScope,
    EvidenceDerivation,
    EvidenceItem,
    EvidencePolicy,
    ObservationKind,
    PopulationObservation,
    QualityFlag,
    SourceProvenance,
    SourceStatus,
    SourceStatusValue,
)
from acmg_classifier.infrastructure.http.policy import (
    HttpRequest,
    HttpStatusKind,
    SourceHttpClient,
    SourceRequestExecutor,
    source_status_from_outcome,
)
from acmg_classifier.infrastructure.storage.evidence import (
    EvidenceNotFoundError,
    SQLiteEvidenceStore,
)
from acmg_classifier.infrastructure.storage.source_cache import (
    CacheLookup,
    CacheLookupState,
    SourceCacheStatus,
    SQLiteSourceCache,
)
from acmg_classifier.ports.evidence import (
    CacheState,
    EvidenceSourceResult,
    NormalizedVariantQuery,
    SourceQuery,
)

GNOMAD_GRAPHQL_ENDPOINT = "https://gnomad.broadinstitute.org/api"
_ADAPTER_VERSION = "1"
_QUERY_VERSION = "population-evidence-v1"
_VARIANT_KEY = re.compile(
    r"^(?P<build>GRCh(?:37|38)):(?P<chrom>(?:[1-9]|1[0-9]|2[0-2]|X|Y|MT)):"
    r"(?P<position>[1-9][0-9]*):(?P<ref>[ACGT]+):(?P<alt>[ACGT]+)$"
)

# Source-maintained DatasetId enum and reference-genome mapping, recorded from
# broadinstitute/gnomad-browser graphql-api/src/datasets.ts on 2026-07-11.
_DATASETS: Mapping[str, tuple[str, str]] = {
    "gnomad_r4": ("GRCh38", "gnomAD v4.1.1"),
    "gnomad_r4_non_ukb": ("GRCh38", "gnomAD v4.1.1 (non-UKB)"),
    "gnomad_r3": ("GRCh38", "gnomAD v3"),
    "gnomad_r3_controls_and_biobanks": ("GRCh38", "gnomAD v3 (controls and biobanks)"),
    "gnomad_r3_non_cancer": ("GRCh38", "gnomAD v3 (non-cancer)"),
    "gnomad_r3_non_neuro": ("GRCh38", "gnomAD v3 (non-neuro)"),
    "gnomad_r3_non_topmed": ("GRCh38", "gnomAD v3 (non-TOPMed)"),
    "gnomad_r3_non_v2": ("GRCh38", "gnomAD v3 (non-v2)"),
    "gnomad_r2_1": ("GRCh37", "gnomAD v2"),
    "gnomad_r2_1_controls": ("GRCh37", "gnomAD v2 (controls)"),
    "gnomad_r2_1_non_neuro": ("GRCh37", "gnomAD v2 (non-neuro)"),
    "gnomad_r2_1_non_cancer": ("GRCh37", "gnomAD v2 (non-cancer)"),
    "gnomad_r2_1_non_topmed": ("GRCh37", "gnomAD v2 (non-TOPMed)"),
    "exac": ("GRCh37", "ExAC"),
}

# ``af`` is selected only to detect source-schema changes; the source marks it
# deprecated. AF evidence is calculated from AC/AN below.
POPULATION_EVIDENCE_QUERY = (
    "query PopulationEvidenceVariant("
    "$variantId: String!, $dataset: DatasetId!) {\n"
    """  variant(variantId: $variantId, dataset: $dataset) {
    variant_id
    reference_genome
    chrom
    pos
    ref
    alt
    coverage {
      exome { mean }
      genome { mean }
    }
    exome {
      ac
      an
      af
      homozygote_count
      hemizygote_count
      filters
      populations { id ac an homozygote_count hemizygote_count }
    }
    genome {
      ac
      an
      af
      homozygote_count
      hemizygote_count
      filters
      populations { id ac an homozygote_count hemizygote_count }
    }
    joint {
      ac
      an
      homozygote_count
      hemizygote_count
      filters
      populations { id ac an homozygote_count hemizygote_count }
    }
  }
}"""
)


@dataclass(frozen=True, slots=True)
class GnomADSettings:
    """Fixed, source-recognized release configuration for this adapter."""

    dataset: str = "gnomad_r4"
    cache_ttl_seconds: int = 24 * 60 * 60

    def __post_init__(self) -> None:
        if self.dataset not in _DATASETS:
            raise ValueError(
                "dataset must be an official short-variant gnomAD DatasetId"
            )
        if self.cache_ttl_seconds < 1:
            raise ValueError("cache_ttl_seconds must be positive")

    @property
    def reference_genome(self) -> str:
        return _DATASETS[self.dataset][0]

    @property
    def release_label(self) -> str:
        return _DATASETS[self.dataset][1]


class _SchemaError(ValueError):
    """A response no longer satisfies the narrow, versioned GraphQL contract."""


class _PreflightError(ValueError):
    """A normalized query is incompatible with this configured source release."""


@dataclass(frozen=True, slots=True)
class _GnomADQuery:
    source_query: SourceQuery
    variant_id: str
    expected_build: str


class GnomADAdapter:
    """Fetch one normalized small variant from the official gnomAD GraphQL API."""

    source_id = "gnomad"

    def __init__(
        self,
        *,
        client: SourceHttpClient,
        cache: SQLiteSourceCache,
        evidence_store: SQLiteEvidenceStore,
        clock: Callable[[], datetime],
        settings: GnomADSettings | None = None,
    ) -> None:
        self._client = client
        self._cache = cache
        self._evidence_store = evidence_store
        self._clock = clock
        self._settings = settings or GnomADSettings()

    async def query(
        self,
        variant: NormalizedVariantQuery,
        *,
        context_scope: EvidenceContextScope,
        policy: EvidencePolicy,
    ) -> EvidenceSourceResult:
        now = _utc(self._clock())
        try:
            built = self._build_query(variant)
        except _PreflightError as error:
            return self._result(
                status=SourceStatus(
                    source_id=self.source_id,
                    status=SourceStatusValue.UNAVAILABLE,
                    checked_at=now,
                    source_version=self._settings.release_label,
                    normalized_query_key=variant.variant_key,
                    detail=str(error),
                ),
                cache_state=CacheState.INELIGIBLE,
            )

        request = HttpRequest(
            method="POST",
            url=GNOMAD_GRAPHQL_ENDPOINT,
            headers={"Content-Type": "application/json", "Accept": "application/json"},
            body=json.dumps(
                {
                    "query": POPULATION_EVIDENCE_QUERY,
                    "variables": {
                        "variantId": built.variant_id,
                        "dataset": self._settings.dataset,
                    },
                },
                separators=(",", ":"),
            ).encode(),
            read_only=True,
        )
        execution = await SourceRequestExecutor(
            cache=self._cache, client=self._client, clock=self._clock
        ).request(built.source_query, request, policy)

        if execution.cache_lookup.state is CacheLookupState.FRESH:
            return self._from_cache(built.source_query, execution.cache_lookup, now)
        if execution.outcome is None:
            return self._result(
                status=SourceStatus(
                    source_id=self.source_id,
                    status=(
                        SourceStatusValue.INELIGIBLE_CACHE
                        if execution.cache_lookup.state is CacheLookupState.INELIGIBLE
                        else SourceStatusValue.UNAVAILABLE
                    ),
                    checked_at=now,
                    source_version=self._settings.release_label,
                    normalized_query_key=variant.variant_key,
                    detail=(
                        "offline_no_eligible_cache"
                        if execution.cache_lookup.state is CacheLookupState.OFFLINE_MISS
                        else "cache_ineligible"
                    ),
                ),
                cache_state=(
                    CacheState.OFFLINE_MISS
                    if execution.cache_lookup.state is CacheLookupState.OFFLINE_MISS
                    else CacheState.INELIGIBLE
                ),
            )

        outcome = execution.outcome
        if outcome.kind is HttpStatusKind.NO_RECORD:
            self._cache.put_negative(
                built.source_query,
                retrieved_at=now,
                expires_at=now + timedelta(seconds=self._settings.cache_ttl_seconds),
                raw_snapshot_ref=None,
                response_hash=None,
                media_type=None,
                byte_size=0,
            )
            return self._result(
                status=self._fresh_status(variant.variant_key, now, "no_record"),
                cache_state=CacheState.LIVE,
            )
        if outcome.kind is not HttpStatusKind.SUCCESS:
            self._cache.put_failure_metadata(
                built.source_query,
                retrieved_at=now,
                error_code=f"HTTP_{outcome.kind.value.upper()}",
            )
            return self._result(
                status=source_status_from_outcome(
                    source_id=self.source_id,
                    outcome=outcome,
                    checked_at=now,
                    normalized_query_key=variant.variant_key,
                    source_version=self._settings.release_label,
                ),
                cache_state=CacheState.LIVE,
            )

        raw_ref = self._evidence_store.put_raw_snapshot(
            outcome.body, "application/json"
        )
        raw_refs = (raw_ref.snapshot_hash,)
        try:
            parsed = _json_object(outcome.body)
            graphql_error = _graphql_error(parsed)
            if graphql_error is not None:
                return self._graphql_error_result(
                    built.source_query,
                    variant.variant_key,
                    graphql_error,
                    now,
                    raw_refs,
                    outcome.body,
                )
            variant_payload = _variant_payload(parsed)
            if variant_payload is None:
                self._put_negative(
                    built.source_query, now, raw_ref.snapshot_hash, outcome.body
                )
                return self._result(
                    status=self._fresh_status(variant.variant_key, now, "no_record"),
                    cache_state=CacheState.LIVE,
                    raw_snapshot_refs=raw_refs,
                )
            items = self._map_items(
                variant, built, variant_payload, now, raw_ref.snapshot_hash
            )
        except (TypeError, ValueError, _SchemaError):
            self._cache.put_failure_metadata(
                built.source_query, retrieved_at=now, error_code="SOURCE_SCHEMA_CHANGED"
            )
            return self._result(
                status=SourceStatus(
                    source_id=self.source_id,
                    status=SourceStatusValue.SCHEMA_CHANGED,
                    checked_at=now,
                    source_version=self._settings.release_label,
                    normalized_query_key=variant.variant_key,
                    detail="graphql_schema_changed",
                ),
                cache_state=CacheState.LIVE,
                raw_snapshot_refs=raw_refs,
            )

        if not items:
            self._put_negative(
                built.source_query, now, raw_ref.snapshot_hash, outcome.body
            )
            return self._result(
                status=self._fresh_status(
                    variant.variant_key, now, "no_usable_denominator"
                ),
                cache_state=CacheState.LIVE,
                raw_snapshot_refs=raw_refs,
            )

        stored_item_pairs = tuple(
            (
                item.model_copy(update={"evidence_id": evidence_id}),
                evidence_id,
            )
            for item in items
            for evidence_id in (
                self._evidence_store.put_evidence(
                    item.model_dump(mode="json", exclude={"evidence_id"})
                ),
            )
        )
        stored_items = tuple(item for item, _ in stored_item_pairs)
        self._cache.put_success(
            built.source_query,
            retrieved_at=now,
            expires_at=now + timedelta(seconds=self._settings.cache_ttl_seconds),
            evidence_ids=tuple(evidence_id for _, evidence_id in stored_item_pairs),
            raw_snapshot_ref=raw_ref.snapshot_hash,
            response_hash=hashlib.sha256(outcome.body).hexdigest(),
            media_type="application/json",
            byte_size=len(outcome.body),
        )
        return self._result(
            status=self._fresh_status(variant.variant_key, now, "record_found"),
            cache_state=CacheState.LIVE,
            items=stored_items,
            raw_snapshot_refs=raw_refs,
        )

    def _build_query(self, variant: NormalizedVariantQuery) -> _GnomADQuery:
        if variant.genome_build != self._settings.reference_genome:
            raise _PreflightError("build_release_mismatch")
        match = _VARIANT_KEY.fullmatch(variant.variant_key)
        if match is None:
            raise _PreflightError("invalid_normalized_variant_key")
        if match["build"] != self._settings.reference_genome:
            raise _PreflightError("build_release_mismatch")
        variant_id = "-".join(
            (match["chrom"], match["position"], match["ref"], match["alt"])
        )
        return _GnomADQuery(
            source_query=SourceQuery(
                source_id=self.source_id,
                normalized_query_key=variant.variant_key,
                request_fingerprint=f"{_QUERY_VERSION}:{self._settings.dataset}",
                source_version=self._settings.release_label,
                adapter_version=_ADAPTER_VERSION,
                media_type="application/json",
            ),
            variant_id=variant_id,
            expected_build=self._settings.reference_genome,
        )

    def _from_cache(
        self, query: SourceQuery, lookup: CacheLookup, now: datetime
    ) -> EvidenceSourceResult:
        entry = lookup.entry
        if entry is None:
            return self._result(
                status=SourceStatus(
                    source_id=self.source_id,
                    status=SourceStatusValue.INELIGIBLE_CACHE,
                    checked_at=now,
                    source_version=self._settings.release_label,
                    normalized_query_key=query.normalized_query_key,
                    detail="cache_entry_missing",
                ),
                cache_state=CacheState.INELIGIBLE,
            )
        if entry.status is SourceCacheStatus.NO_RECORD:
            return self._result(
                status=SourceStatus(
                    source_id=self.source_id,
                    status=SourceStatusValue.CACHED,
                    checked_at=now,
                    source_version=entry.source_version,
                    normalized_query_key=query.normalized_query_key,
                    detail="no_record",
                ),
                cache_state=CacheState.HIT_FRESH,
                raw_snapshot_refs=(
                    () if entry.raw_snapshot_ref is None else (entry.raw_snapshot_ref,)
                ),
            )
        try:
            items = tuple(
                EvidenceItem.model_validate_json(
                    self._evidence_store.get_evidence(evidence_id)
                ).model_copy(update={"evidence_id": evidence_id})
                for evidence_id in entry.evidence_ids
            )
        except (EvidenceNotFoundError, ValueError):
            return self._result(
                status=SourceStatus(
                    source_id=self.source_id,
                    status=SourceStatusValue.INELIGIBLE_CACHE,
                    checked_at=now,
                    source_version=entry.source_version,
                    normalized_query_key=query.normalized_query_key,
                    detail="cached_evidence_unavailable",
                ),
                cache_state=CacheState.INELIGIBLE,
            )
        raw_refs = tuple(
            sorted({item.raw_snapshot_ref for item in items if item.raw_snapshot_ref})
        )
        if (
            entry.raw_snapshot_ref is not None
            and entry.raw_snapshot_ref not in raw_refs
        ):
            raw_refs = tuple(sorted((*raw_refs, entry.raw_snapshot_ref)))
        return self._result(
            status=SourceStatus(
                source_id=self.source_id,
                status=SourceStatusValue.CACHED,
                checked_at=now,
                source_version=entry.source_version,
                normalized_query_key=query.normalized_query_key,
                detail="record_found",
            ),
            cache_state=CacheState.HIT_FRESH,
            items=items,
            raw_snapshot_refs=raw_refs,
        )

    def _graphql_error_result(
        self,
        query: SourceQuery,
        normalized_query_key: str,
        message: str,
        now: datetime,
        raw_refs: tuple[str, ...],
        raw_body: bytes,
    ) -> EvidenceSourceResult:
        lowered = message.lower()
        if "variant not found" in lowered:
            self._put_negative(query, now, raw_refs[0], raw_body)
            return self._result(
                status=self._fresh_status(normalized_query_key, now, "no_record"),
                cache_state=CacheState.LIVE,
                raw_snapshot_refs=raw_refs,
            )
        if "rate limit" in lowered:
            status = SourceStatusValue.RATE_LIMITED
            detail = "graphql_rate_limited"
        elif _is_schema_message(lowered):
            status = SourceStatusValue.SCHEMA_CHANGED
            detail = "graphql_schema_changed"
        else:
            status = SourceStatusValue.UNAVAILABLE
            detail = "graphql_service_error"
        self._cache.put_failure_metadata(
            query,
            retrieved_at=now,
            error_code=(
                "SOURCE_SCHEMA_CHANGED"
                if status is SourceStatusValue.SCHEMA_CHANGED
                else "SOURCE_UNAVAILABLE"
            ),
        )
        return self._result(
            status=SourceStatus(
                source_id=self.source_id,
                status=status,
                checked_at=now,
                source_version=self._settings.release_label,
                normalized_query_key=normalized_query_key,
                detail=detail,
            ),
            cache_state=CacheState.LIVE,
            raw_snapshot_refs=raw_refs,
        )

    def _map_items(
        self,
        variant: NormalizedVariantQuery,
        query: _GnomADQuery,
        payload: Mapping[str, object],
        retrieved_at: datetime,
        raw_snapshot_ref: str,
    ) -> tuple[EvidenceItem, ...]:
        if payload.get("reference_genome") != query.expected_build:
            raise _SchemaError(
                "response reference genome differs from configured dataset"
            )
        if payload.get("variant_id") != query.variant_id:
            raise _SchemaError("response variant identifier differs from request")
        if "coverage" not in payload:
            raise _SchemaError("response is missing coverage")
        coverage = _mapping_or_none(payload["coverage"], "coverage")
        if coverage is None:
            raise _SchemaError("coverage must be an object")
        items: list[EvidenceItem] = []
        for stratum in ("genome", "exome", "joint"):
            if stratum not in payload:
                raise _SchemaError(f"response is missing {stratum}")
            frequency = _mapping_or_none(payload[stratum], stratum)
            if frequency is None:
                continue
            stratum_coverage = None
            if stratum != "joint" and coverage is not None:
                coverage_value = _mapping_or_none(
                    coverage.get(stratum), f"coverage.{stratum}"
                )
                if coverage_value is not None:
                    stratum_coverage = _number_or_none(
                        coverage_value.get("mean"), f"coverage.{stratum}.mean"
                    )
            observations = self._map_stratum(frequency, stratum, stratum_coverage)
            for observation in observations:
                flags = (
                    (QualityFlag.FILTERED,)
                    if observation.filter_status != "PASS"
                    else ()
                )
                item = EvidenceItem.model_validate(
                    {
                        "variant_key": variant.variant_key,
                        "kind": ObservationKind.POPULATION,
                        "observation": observation,
                        "context_scope": EvidenceContextScope(
                            genome_build=GenomeBuild(query.expected_build)
                        ),
                        "provenance": SourceProvenance(
                            kind=EvidenceDerivation.SOURCE,
                            source_id=self.source_id,
                            source_record_id=(
                                f"{self._settings.dataset}:{query.variant_id}:{stratum}:"
                                f"{observation.ancestry or 'overall'}"
                            ),
                            retrieved_at=retrieved_at,
                            normalized_query_key=variant.variant_key,
                        ),
                        "raw_snapshot_ref": raw_snapshot_ref,
                        "quality_flags": flags,
                        "derivation": EvidenceDerivation.SOURCE,
                    }
                )
                items.append(item)
        return tuple(items)

    def _map_stratum(
        self,
        frequency: Mapping[str, object],
        stratum: str,
        coverage: float | None,
    ) -> tuple[PopulationObservation, ...]:
        ac = _required_nonnegative_int(frequency.get("ac"), f"{stratum}.ac")
        an = _required_nonnegative_int(frequency.get("an"), f"{stratum}.an")
        if an == 0:
            return ()
        filters = _filters(frequency.get("filters"), f"{stratum}.filters")
        release = (
            f"{self._settings.release_label} ({self._settings.dataset}; {stratum})"
        )
        overall = PopulationObservation(
            kind=ObservationKind.POPULATION,
            source_release=release,
            allele_count=ac,
            allele_number=an,
            allele_frequency=ac / an,
            homozygote_count=_optional_nonnegative_int(
                frequency.get("homozygote_count"), f"{stratum}.homozygote_count"
            ),
            hemizygote_count=_optional_nonnegative_int(
                frequency.get("hemizygote_count"), f"{stratum}.hemizygote_count"
            ),
            coverage=coverage,
            filter_status=filters,
        )
        populations = frequency.get("populations")
        if populations is None:
            populations = ()
        if not isinstance(populations, list):
            raise _SchemaError(f"{stratum}.populations must be an array or null")
        mapped = [overall]
        for index, population in enumerate(populations):
            if not isinstance(population, dict):
                raise _SchemaError(f"{stratum}.populations[{index}] must be an object")
            ancestry = population.get("id")
            if not isinstance(ancestry, str) or not ancestry.strip():
                raise _SchemaError(
                    f"{stratum}.populations[{index}].id must be a string"
                )
            population_ac = _required_nonnegative_int(
                population.get("ac"), f"{stratum}.populations[{index}].ac"
            )
            population_an = _required_nonnegative_int(
                population.get("an"), f"{stratum}.populations[{index}].an"
            )
            if population_an == 0:
                continue
            mapped.append(
                PopulationObservation(
                    kind=ObservationKind.POPULATION,
                    source_release=release,
                    ancestry=ancestry,
                    allele_count=population_ac,
                    allele_number=population_an,
                    allele_frequency=population_ac / population_an,
                    homozygote_count=_optional_nonnegative_int(
                        population.get("homozygote_count"),
                        f"{stratum}.populations[{index}].homozygote_count",
                    ),
                    hemizygote_count=_optional_nonnegative_int(
                        population.get("hemizygote_count"),
                        f"{stratum}.populations[{index}].hemizygote_count",
                    ),
                    coverage=coverage,
                    filter_status=filters,
                )
            )
        return tuple(mapped)

    def _put_negative(
        self,
        query: SourceQuery,
        now: datetime,
        raw_snapshot_ref: str,
        raw_body: bytes,
    ) -> None:
        self._cache.put_negative(
            query,
            retrieved_at=now,
            expires_at=now + timedelta(seconds=self._settings.cache_ttl_seconds),
            raw_snapshot_ref=raw_snapshot_ref,
            response_hash=hashlib.sha256(raw_body).hexdigest(),
            media_type="application/json",
            byte_size=len(raw_body),
        )

    def _fresh_status(self, query_key: str, now: datetime, detail: str) -> SourceStatus:
        return SourceStatus(
            source_id=self.source_id,
            status=SourceStatusValue.FRESH,
            checked_at=now,
            source_version=self._settings.release_label,
            normalized_query_key=query_key,
            detail=detail,
        )

    def _result(
        self,
        *,
        status: SourceStatus,
        cache_state: CacheState,
        items: tuple[EvidenceItem, ...] = (),
        raw_snapshot_refs: tuple[str, ...] = (),
    ) -> EvidenceSourceResult:
        return EvidenceSourceResult(
            source_id=self.source_id,
            evidence_items=items,
            source_status=status,
            cache_state=cache_state,
            raw_snapshot_refs=raw_snapshot_refs,
        )


def _utc(value: datetime) -> datetime:
    if value.tzinfo is None or value.utcoffset() is None:
        raise ValueError("clock must return a timezone-aware datetime")
    return value.astimezone(UTC)


def _json_object(raw: bytes) -> Mapping[str, object]:
    value = json.loads(raw)
    if not isinstance(value, dict):
        raise _SchemaError("GraphQL response must be a JSON object")
    return value


def _graphql_error(payload: Mapping[str, object]) -> str | None:
    errors = payload.get("errors")
    if errors is None:
        return None
    if not isinstance(errors, list) or not errors:
        raise _SchemaError("GraphQL errors must be a non-empty array")
    first = errors[0]
    if not isinstance(first, Mapping):
        raise _SchemaError("GraphQL error must contain a message")
    message = first.get("message")
    if not isinstance(message, str):
        raise _SchemaError("GraphQL error must contain a message")
    return message


def _variant_payload(payload: Mapping[str, object]) -> Mapping[str, object] | None:
    data = payload.get("data")
    if not isinstance(data, dict):
        raise _SchemaError("GraphQL response has no data object")
    variant = data.get("variant")
    if variant is None:
        return None
    if not isinstance(variant, dict):
        raise _SchemaError("GraphQL variant must be an object or null")
    return variant


def _mapping_or_none(value: object, field: str) -> Mapping[str, object] | None:
    if value is None:
        return None
    if not isinstance(value, dict):
        raise _SchemaError(f"{field} must be an object or null")
    return value


def _required_nonnegative_int(value: object, field: str) -> int:
    if not isinstance(value, int) or isinstance(value, bool) or value < 0:
        raise _SchemaError(f"{field} must be a non-negative integer")
    return value


def _optional_nonnegative_int(value: object, field: str) -> int | None:
    if value is None:
        return None
    return _required_nonnegative_int(value, field)


def _number_or_none(value: object, field: str) -> float | None:
    if value is None:
        return None
    if isinstance(value, bool) or not isinstance(value, (int, float)):
        raise _SchemaError(f"{field} must be a number or null")
    converted = float(value)
    if converted < 0 or converted == float("inf") or converted != converted:
        raise _SchemaError(f"{field} must be a finite non-negative number")
    return converted


def _filters(value: object, field: str) -> str:
    if value is None:
        raise _SchemaError(f"{field} is required")
    if not isinstance(value, list) or any(
        not isinstance(item, str) or not item for item in value
    ):
        raise _SchemaError(f"{field} must be an array of non-empty strings")
    return "PASS" if not value else ";".join(sorted(set(value)))


def _is_schema_message(message: str) -> bool:
    return any(
        marker in message
        for marker in (
            "cannot query field",
            "unknown argument",
            "unknown type",
            "validation",
            "variable",
            "syntax error",
        )
    )
