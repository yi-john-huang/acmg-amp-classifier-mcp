"""Read-only ClinVar VCV XML evidence adapter over the shared source protocol.

The adapter uses NCBI ESearch only to discover a single candidate and EFetch
``rettype=vcv`` XML as the authoritative assertion payload.  It intentionally
returns each submitted SCV assertion independently; it does not classify,
merge conflicts, or emit ACMG criteria.
"""

from __future__ import annotations

import json
import re
from collections.abc import Callable
from dataclasses import dataclass
from datetime import UTC, datetime, timedelta
from hashlib import sha256
from urllib.parse import urlencode
from xml.etree import ElementTree

from acmg_classifier.domain.enums import GenomeBuild
from acmg_classifier.domain.evidence import (
    ClinicalAssertionObservation,
    EvidenceContextScope,
    EvidenceDerivation,
    EvidenceItem,
    EvidencePolicy,
    ObservationKind,
    SourceProvenance,
    SourceStatus,
    SourceStatusValue,
)
from acmg_classifier.infrastructure.http.policy import (
    HttpOutcome,
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

_EUTILS_BASE = "https://eutils.ncbi.nlm.nih.gov/entrez/eutils"
_VCV_ACCESSION = re.compile(r"^VCV\d{9}(?:\.\d+)?$")
_VARIATION_ID = re.compile(r"^[1-9]\d*$")
_ONTOLOGY_ID = re.compile(r"^[A-Z][A-Z0-9_]*:\d+$")


class ClinVarSchemaError(ValueError):
    """The source response lacks a required VCV XML identity or assertion field."""


@dataclass(frozen=True, slots=True)
class ClinVarSettings:
    """Bounded official E-utilities request settings without secret persistence."""

    tool: str = "acmg_amp_classifier"
    email: str | None = None
    api_key: str | None = None
    retmax: int = 20
    cache_ttl: timedelta = timedelta(days=7)
    source_schema_version: str = "ClinVar_VCV_2.6"

    def __post_init__(self) -> None:
        if not self.tool or any(character.isspace() for character in self.tool):
            raise ValueError("ClinVar tool must be a non-empty token")
        if self.email is not None and not self.email:
            raise ValueError("ClinVar email must be non-empty when configured")
        if self.retmax < 1 or self.retmax > 20:
            raise ValueError("ClinVar retmax must be between 1 and 20")
        if self.cache_ttl.total_seconds() < 0:
            raise ValueError("ClinVar cache_ttl must not be negative")


@dataclass(frozen=True, slots=True)
class ClinVarQuery:
    """The sanitized source-query identity and official final-payload request."""

    source_query: SourceQuery
    final_request: HttpRequest | None
    search_request: HttpRequest | None
    requested_condition_id: str | None
    requested_condition_label: str | None


@dataclass(frozen=True, slots=True)
class _Condition:
    condition_id: str | None
    label: str | None


@dataclass(frozen=True, slots=True)
class _AuthoritativeAllele:
    assembly: str
    accession: str
    start: int
    stop: int
    reference: str
    alternate: str


class ClinVarAdapter:
    """Retrieve and map official ClinVar VCV XML without clinical interpretation."""

    source_id = "clinvar"
    adapter_version = "1"

    def __init__(
        self,
        *,
        client: SourceHttpClient,
        cache: SQLiteSourceCache,
        evidence_store: SQLiteEvidenceStore,
        clock: Callable[[], datetime],
        settings: ClinVarSettings | None = None,
    ) -> None:
        self._client = client
        self._cache = cache
        self._evidence_store = evidence_store
        self._clock = clock
        self._settings = settings if settings is not None else ClinVarSettings()
        if client.policy.source_id != self.source_id:
            raise ValueError("ClinVar adapter requires a clinvar SourceHttpClient")

    async def query(
        self,
        variant: NormalizedVariantQuery,
        *,
        context_scope: EvidenceContextScope,
        policy: EvidencePolicy,
    ) -> EvidenceSourceResult:
        """Return VCV/SCV observations or one honest source status.

        A normalized optional ``clinvar_accession``/``clinvar_variation_id`` is
        used directly when supplied. Otherwise, genomic HGVS is only candidate
        discovery through ESearch and evidence is emitted only from EFetch VCV
        XML after exactly one candidate is found.
        """
        checked_at = _as_utc(self._clock())
        try:
            query = self._build_query(variant, context_scope)
            scope = EvidenceContextScope(
                genome_build=GenomeBuild(variant.genome_build),
                disease_id=context_scope.disease_id,
            )
        except (TypeError, ValueError):
            return self._unavailable_result(
                checked_at=checked_at,
                query_key=_variant_key_or_none(variant),
                detail="invalid_query",
            )

        execution = await SourceRequestExecutor(
            cache=self._cache, client=self._client, clock=self._clock
        ).request(
            query.source_query,
            query.search_request
            or query.final_request
            or HttpRequest(method="GET", url="https://eutils.ncbi.nlm.nih.gov/"),
            policy,
        )
        if execution.outcome is None:
            return self._from_cache(
                lookup=execution.cache_lookup,
                query=query,
                scope=scope,
                checked_at=checked_at,
            )

        if execution.outcome.kind is not HttpStatusKind.SUCCESS:
            return self._from_failed_outcome(
                outcome=execution.outcome,
                query=query,
                checked_at=checked_at,
            )

        raw_refs: list[str] = []
        final_outcome = execution.outcome
        if query.search_request is not None:
            search_raw_ref = self._persist_raw(final_outcome)
            if search_raw_ref is not None:
                raw_refs.append(search_raw_ref)
            try:
                candidate = _single_esearch_candidate(final_outcome.body)
            except ClinVarSchemaError:
                return self._schema_changed(
                    query=query,
                    checked_at=checked_at,
                    raw_snapshot_refs=tuple(raw_refs),
                )
            if candidate is None:
                self._put_negative(
                    query.source_query, checked_at, final_outcome, search_raw_ref
                )
                return self._result(
                    status=SourceStatus(
                        source_id=self.source_id,
                        status=SourceStatusValue.FRESH,
                        checked_at=checked_at,
                        source_version=self._settings.source_schema_version,
                        normalized_query_key=query.source_query.normalized_query_key,
                        detail="no_record",
                    ),
                    cache_state=CacheState.LIVE,
                    raw_snapshot_refs=tuple(raw_refs),
                )
            if candidate == "":
                return self._result(
                    status=SourceStatus(
                        source_id=self.source_id,
                        status=SourceStatusValue.FRESH,
                        checked_at=checked_at,
                        source_version=self._settings.source_schema_version,
                        normalized_query_key=query.source_query.normalized_query_key,
                        detail="multiple_candidates_needs_disambiguation",
                    ),
                    cache_state=CacheState.LIVE,
                    raw_snapshot_refs=tuple(raw_refs),
                )
            final_outcome = await self._client.request(
                self._efetch_by_variation_id_request(candidate)
            )
            if final_outcome.kind is not HttpStatusKind.SUCCESS:
                return self._from_failed_outcome(
                    outcome=final_outcome,
                    query=query,
                    checked_at=checked_at,
                    raw_snapshot_refs=tuple(raw_refs),
                )

        raw_ref = self._persist_raw(final_outcome)
        if raw_ref is not None:
            # The VCV response, not candidate-discovery ESearch, is the
            # authoritative evidence payload and replayable raw reference.
            raw_refs = [raw_ref]
        if raw_ref is None:
            return self._schema_changed(
                query=query, checked_at=checked_at, raw_snapshot_refs=tuple(raw_refs)
            )
        try:
            allele_matches = _vcv_allele_matches(final_outcome.body, variant)
        except ClinVarSchemaError:
            return self._schema_changed(
                query=query, checked_at=checked_at, raw_snapshot_refs=tuple(raw_refs)
            )
        if not allele_matches:
            self._put_negative(query.source_query, checked_at, final_outcome, raw_ref)
            return self._result(
                status=SourceStatus(
                    source_id=self.source_id,
                    status=SourceStatusValue.FRESH,
                    checked_at=checked_at,
                    source_version=self._settings.source_schema_version,
                    normalized_query_key=query.source_query.normalized_query_key,
                    detail="allele_mismatch_excluded",
                ),
                cache_state=CacheState.LIVE,
                raw_snapshot_refs=tuple(raw_refs),
            )
        try:
            source_version, items, condition_mismatch = self._map_vcv(
                raw_xml=final_outcome.body,
                raw_snapshot_ref=raw_ref,
                variant_key=query.source_query.normalized_query_key,
                scope=scope,
                retrieved_at=checked_at,
                requested_condition_id=query.requested_condition_id,
                requested_condition_label=query.requested_condition_label,
            )
        except ClinVarSchemaError:
            return self._schema_changed(
                query=query, checked_at=checked_at, raw_snapshot_refs=tuple(raw_refs)
            )

        if not items:
            if condition_mismatch:
                self._put_success(
                    query.source_query, checked_at, final_outcome, raw_ref, ()
                )
                detail = "condition_mismatch_excluded"
            else:
                self._put_negative(
                    query.source_query, checked_at, final_outcome, raw_ref
                )
                detail = "no_record"
            return self._result(
                status=SourceStatus(
                    source_id=self.source_id,
                    status=SourceStatusValue.FRESH,
                    checked_at=checked_at,
                    source_version=source_version,
                    normalized_query_key=query.source_query.normalized_query_key,
                    detail=detail,
                ),
                cache_state=CacheState.LIVE,
                raw_snapshot_refs=tuple(raw_refs),
            )

        evidence_ids = tuple(
            self._evidence_store.put_evidence(item.canonical_content())
            for item in items
        )
        items = tuple(
            item.model_copy(update={"evidence_id": evidence_id})
            for item, evidence_id in zip(items, evidence_ids, strict=True)
        )
        self._put_success(
            query.source_query, checked_at, final_outcome, raw_ref, evidence_ids
        )
        return self._result(
            status=SourceStatus(
                source_id=self.source_id,
                status=SourceStatusValue.FRESH,
                checked_at=checked_at,
                source_version=source_version,
                normalized_query_key=query.source_query.normalized_query_key,
            ),
            evidence_items=items,
            cache_state=CacheState.LIVE,
            raw_snapshot_refs=tuple(raw_refs),
        )

    def _build_query(
        self,
        variant: NormalizedVariantQuery,
        context_scope: EvidenceContextScope,
    ) -> ClinVarQuery:
        variant_key = variant.variant_key
        if not variant_key:
            raise ValueError("variant_key is required")
        condition_id = context_scope.disease_id
        condition_label = None
        condition_fingerprint = f"condition={condition_id or '-'}"
        accession = _optional_text(variant, "clinvar_accession")
        variation_id = _optional_text(
            variant, "clinvar_variation_id"
        ) or _optional_text(variant, "variation_id")
        if accession is not None:
            if _VCV_ACCESSION.fullmatch(accession) is None:
                raise ValueError("clinvar_accession must be a VCV accession")
            request_fingerprint = f"efetch-vcv-accession-v1;{condition_fingerprint}"
            return ClinVarQuery(
                source_query=self._source_query(variant_key, request_fingerprint),
                final_request=self._efetch_by_accession_request(accession),
                search_request=None,
                requested_condition_id=condition_id,
                requested_condition_label=condition_label,
            )
        if variation_id is not None:
            if _VARIATION_ID.fullmatch(variation_id) is None:
                raise ValueError("clinvar_variation_id must be positive integer text")
            request_fingerprint = f"efetch-vcv-variation-id-v1;{condition_fingerprint}"
            return ClinVarQuery(
                source_query=self._source_query(variant_key, request_fingerprint),
                final_request=self._efetch_by_variation_id_request(variation_id),
                search_request=None,
                requested_condition_id=condition_id,
                requested_condition_label=condition_label,
            )
        if variant.genomic_hgvs is None or not variant.genomic_hgvs.strip():
            raise ValueError(
                "ClinVar query requires genomic HGVS or a ClinVar identifier"
            )
        request_fingerprint = f"esearch-hgvs-then-efetch-vcv-v1;{condition_fingerprint}"
        return ClinVarQuery(
            source_query=self._source_query(variant_key, request_fingerprint),
            final_request=None,
            search_request=self._esearch_request(variant.genomic_hgvs),
            requested_condition_id=condition_id,
            requested_condition_label=condition_label,
        )

    def _source_query(self, variant_key: str, request_fingerprint: str) -> SourceQuery:
        return SourceQuery(
            source_id=self.source_id,
            normalized_query_key=variant_key,
            request_fingerprint=request_fingerprint,
            source_version=self._settings.source_schema_version,
            adapter_version=self.adapter_version,
            media_type="application/xml",
        )

    def _esearch_request(self, hgvs: str) -> HttpRequest:
        parameters = self._request_parameters(
            {
                "db": "clinvar",
                "term": hgvs,
                "retmax": str(self._settings.retmax),
                "retmode": "xml",
            }
        )
        return HttpRequest(
            method="GET",
            url=f"{_EUTILS_BASE}/esearch.fcgi?{urlencode(parameters)}",
        )

    def _efetch_by_variation_id_request(self, variation_id: str) -> HttpRequest:
        parameters = self._request_parameters(
            {
                "db": "clinvar",
                "rettype": "vcv",
                "is_variationid": "",
                "id": variation_id,
                "from_esearch": "true",
            }
        )
        return HttpRequest(
            method="GET",
            url=f"{_EUTILS_BASE}/efetch.fcgi?{urlencode(parameters)}",
        )

    def _efetch_by_accession_request(self, accession: str) -> HttpRequest:
        parameters = self._request_parameters(
            {"db": "clinvar", "rettype": "vcv", "id": accession}
        )
        return HttpRequest(
            method="GET",
            url=f"{_EUTILS_BASE}/efetch.fcgi?{urlencode(parameters)}",
        )

    def _request_parameters(self, parameters: dict[str, str]) -> dict[str, str]:
        result = {**parameters, "tool": self._settings.tool}
        if self._settings.email is not None:
            result["email"] = self._settings.email
        if self._settings.api_key is not None:
            result["api_key"] = self._settings.api_key
        return result

    def _from_cache(
        self,
        *,
        lookup: CacheLookup,
        query: ClinVarQuery,
        scope: EvidenceContextScope,
        checked_at: datetime,
    ) -> EvidenceSourceResult:
        if lookup.state is CacheLookupState.OFFLINE_MISS:
            return self._result(
                status=SourceStatus(
                    source_id=self.source_id,
                    status=SourceStatusValue.UNAVAILABLE,
                    checked_at=checked_at,
                    source_version=self._settings.source_schema_version,
                    normalized_query_key=query.source_query.normalized_query_key,
                    detail="offline_no_eligible_cache",
                ),
                cache_state=CacheState.OFFLINE_MISS,
            )
        if lookup.state is not CacheLookupState.FRESH or lookup.entry is None:
            return self._result(
                status=SourceStatus(
                    source_id=self.source_id,
                    status=SourceStatusValue.INELIGIBLE_CACHE,
                    checked_at=checked_at,
                    source_version=self._settings.source_schema_version,
                    normalized_query_key=query.source_query.normalized_query_key,
                    detail="cache_ineligible",
                ),
                cache_state=CacheState.INELIGIBLE,
            )
        entry = lookup.entry
        if entry.status is SourceCacheStatus.NO_RECORD:
            return self._result(
                status=SourceStatus(
                    source_id=self.source_id,
                    status=SourceStatusValue.CACHED,
                    checked_at=checked_at,
                    source_version=entry.source_version,
                    normalized_query_key=query.source_query.normalized_query_key,
                    detail="no_record",
                ),
                cache_state=CacheState.HIT_FRESH,
                raw_snapshot_refs=()
                if entry.raw_snapshot_ref is None
                else (entry.raw_snapshot_ref,),
            )
        try:
            items = tuple(
                EvidenceItem.model_validate(
                    json.loads(self._evidence_store.get_evidence(evidence_id))
                )
                for evidence_id in entry.evidence_ids
            )
        except (EvidenceNotFoundError, ValueError, TypeError, json.JSONDecodeError):
            return self._result(
                status=SourceStatus(
                    source_id=self.source_id,
                    status=SourceStatusValue.INELIGIBLE_CACHE,
                    checked_at=checked_at,
                    source_version=entry.source_version,
                    normalized_query_key=query.source_query.normalized_query_key,
                    detail="cache_evidence_missing_or_invalid",
                ),
                cache_state=CacheState.INELIGIBLE,
            )
        if any(
            not _cached_item_matches(
                item,
                variant_key=query.source_query.normalized_query_key,
                scope=scope,
                source_id=self.source_id,
            )
            for item in items
        ):
            return self._result(
                status=SourceStatus(
                    source_id=self.source_id,
                    status=SourceStatusValue.INELIGIBLE_CACHE,
                    checked_at=checked_at,
                    source_version=entry.source_version,
                    normalized_query_key=query.source_query.normalized_query_key,
                    detail="cache_evidence_scope_mismatch",
                ),
                cache_state=CacheState.INELIGIBLE,
            )
        raw_refs = tuple(
            sorted(
                {
                    item.raw_snapshot_ref
                    for item in items
                    if item.raw_snapshot_ref is not None
                }
            )
        )
        source_version = _source_version_from_items(items) or entry.source_version
        detail = None
        if not items:
            detail = self._empty_vcv_cache_detail(entry.raw_snapshot_ref, query, scope)
        return self._result(
            status=SourceStatus(
                source_id=self.source_id,
                status=SourceStatusValue.CACHED,
                checked_at=checked_at,
                source_version=source_version,
                normalized_query_key=query.source_query.normalized_query_key,
                detail=detail,
            ),
            evidence_items=items,
            cache_state=CacheState.HIT_FRESH,
            raw_snapshot_refs=raw_refs
            or (() if entry.raw_snapshot_ref is None else (entry.raw_snapshot_ref,)),
        )

    def _empty_vcv_cache_detail(
        self, raw_ref: str | None, query: ClinVarQuery, scope: EvidenceContextScope
    ) -> str:
        if raw_ref is None:
            return "no_record"
        try:
            source_version, items, condition_mismatch = self._map_vcv(
                raw_xml=self._evidence_store.get_raw_snapshot(raw_ref),
                raw_snapshot_ref=raw_ref,
                variant_key=query.source_query.normalized_query_key,
                scope=scope,
                retrieved_at=self._clock(),
                requested_condition_id=query.requested_condition_id,
                requested_condition_label=query.requested_condition_label,
            )
            del source_version, items
        except Exception:
            return "no_record"
        return "condition_mismatch_excluded" if condition_mismatch else "no_record"

    def _from_failed_outcome(
        self,
        *,
        outcome: HttpOutcome,
        query: ClinVarQuery,
        checked_at: datetime,
        raw_snapshot_refs: tuple[str, ...] = (),
    ) -> EvidenceSourceResult:
        status = source_status_from_outcome(
            source_id=self.source_id,
            outcome=outcome,
            checked_at=checked_at,
            normalized_query_key=query.source_query.normalized_query_key,
            source_version=self._settings.source_schema_version,
        )
        self._cache.put_failure_metadata(
            query.source_query,
            retrieved_at=checked_at,
            error_code={
                SourceStatusValue.RATE_LIMITED: "SOURCE_RATE_LIMITED",
                SourceStatusValue.TIMEOUT: "SOURCE_TIMEOUT",
            }.get(status.status, "SOURCE_UNAVAILABLE"),
        )
        return self._result(
            status=status,
            cache_state=CacheState.LIVE,
            raw_snapshot_refs=raw_snapshot_refs,
            diagnostics=(f"http_attempts={outcome.attempts}",),
        )

    def _schema_changed(
        self,
        *,
        query: ClinVarQuery,
        checked_at: datetime,
        raw_snapshot_refs: tuple[str, ...],
    ) -> EvidenceSourceResult:
        self._cache.put_failure_metadata(
            query.source_query,
            retrieved_at=checked_at,
            error_code="SOURCE_SCHEMA_CHANGED",
        )
        return self._result(
            status=SourceStatus(
                source_id=self.source_id,
                status=SourceStatusValue.SCHEMA_CHANGED,
                checked_at=checked_at,
                source_version=self._settings.source_schema_version,
                normalized_query_key=query.source_query.normalized_query_key,
                detail="required_vcv_xml_field_missing_or_malformed",
            ),
            cache_state=CacheState.LIVE,
            raw_snapshot_refs=raw_snapshot_refs,
        )

    def _unavailable_result(
        self, *, checked_at: datetime, query_key: str | None, detail: str
    ) -> EvidenceSourceResult:
        return self._result(
            status=SourceStatus(
                source_id=self.source_id,
                status=SourceStatusValue.UNAVAILABLE,
                checked_at=checked_at,
                normalized_query_key=query_key,
                detail=detail,
            ),
            cache_state=CacheState.INELIGIBLE,
        )

    def _persist_raw(self, outcome: HttpOutcome) -> str | None:
        if outcome.kind is not HttpStatusKind.SUCCESS:
            return None
        media_type = "application/xml"
        return self._evidence_store.put_raw_snapshot(
            outcome.body, media_type
        ).snapshot_hash

    def _put_success(
        self,
        query: SourceQuery,
        retrieved_at: datetime,
        outcome: HttpOutcome,
        raw_ref: str,
        evidence_ids: tuple[str, ...],
    ) -> None:
        self._cache.put_success(
            query,
            retrieved_at=retrieved_at,
            expires_at=retrieved_at + self._settings.cache_ttl,
            evidence_ids=evidence_ids,
            raw_snapshot_ref=raw_ref,
            response_hash=sha256(outcome.body).hexdigest(),
            media_type="application/xml",
            byte_size=len(outcome.body),
        )

    def _put_negative(
        self,
        query: SourceQuery,
        retrieved_at: datetime,
        outcome: HttpOutcome,
        raw_ref: str | None,
    ) -> None:
        self._cache.put_negative(
            query,
            retrieved_at=retrieved_at,
            expires_at=retrieved_at + self._settings.cache_ttl,
            raw_snapshot_ref=raw_ref,
            response_hash=sha256(outcome.body).hexdigest() if outcome.body else None,
            media_type="application/xml" if outcome.body else None,
            byte_size=len(outcome.body),
        )

    def _map_vcv(
        self,
        *,
        raw_xml: bytes,
        raw_snapshot_ref: str,
        variant_key: str,
        scope: EvidenceContextScope,
        retrieved_at: datetime,
        requested_condition_id: str | None,
        requested_condition_label: str | None,
    ) -> tuple[str, tuple[EvidenceItem, ...], bool]:
        try:
            root = ElementTree.fromstring(raw_xml)
        except ElementTree.ParseError as error:
            raise ClinVarSchemaError("invalid VCV XML") from error
        archives = [
            element
            for element in root.iter()
            if _local_name(element.tag) == "VariationArchive"
        ]
        if len(archives) != 1:
            raise ClinVarSchemaError("expected exactly one VariationArchive")
        archive = archives[0]
        variation_id = archive.get("VariationID")
        vcv_accession = archive.get("Accession")
        version = archive.get("Version")
        if not variation_id or not vcv_accession or not version:
            raise ClinVarSchemaError("VariationArchive identity is incomplete")
        vcv_version = f"{vcv_accession}.{version}"
        assertions = [
            element
            for element in archive.iter()
            if _local_name(element.tag) == "ClinicalAssertion"
        ]
        mapped: list[EvidenceItem] = []
        any_condition_mismatch = False
        for assertion in assertions:
            assertion_items, mismatch = self._map_assertion(
                assertion=assertion,
                raw_snapshot_ref=raw_snapshot_ref,
                variant_key=variant_key,
                scope=scope,
                retrieved_at=retrieved_at,
                vcv_version=vcv_version,
                requested_condition_id=requested_condition_id,
                requested_condition_label=requested_condition_label,
            )
            mapped.extend(assertion_items)
            any_condition_mismatch = any_condition_mismatch or mismatch
        mapped.sort(key=_clinical_assertion_sort_key)
        return vcv_version, tuple(mapped), any_condition_mismatch

    def _map_assertion(
        self,
        *,
        assertion: ElementTree.Element,
        raw_snapshot_ref: str,
        variant_key: str,
        scope: EvidenceContextScope,
        retrieved_at: datetime,
        vcv_version: str,
        requested_condition_id: str | None,
        requested_condition_label: str | None,
    ) -> tuple[tuple[EvidenceItem, ...], bool]:
        accession_element = _first_descendant(assertion, "ClinVarAccession")
        significance = _first_descendant(assertion, "ClinicalSignificance")
        if accession_element is None or significance is None:
            raise ClinVarSchemaError("SCV assertion identity or significance is absent")
        accession = accession_element.get("Accession")
        version = accession_element.get("Version")
        description = _descendant_text(significance, "Description")
        review_status = _descendant_text(significance, "ReviewStatus")
        if not accession or not version or not description or not review_status:
            raise ClinVarSchemaError("SCV assertion required fields are absent")
        scv_accession = f"{accession}.{version}"
        submitter = _submitter(assertion)
        last_evaluated = _parse_date(significance.get("DateLastEvaluated"))
        citations = _citation_ids(assertion)
        conditions = _conditions(assertion)
        selected = _select_conditions(
            conditions,
            requested_condition_id=requested_condition_id,
            requested_condition_label=requested_condition_label,
        )
        if selected is None:
            return (), True
        if not selected:
            selected = (_Condition(None, None),)
        items = tuple(
            EvidenceItem(
                variant_key=variant_key,
                kind=ObservationKind.CLINICAL_ASSERTION,
                observation=ClinicalAssertionObservation(
                    kind=ObservationKind.CLINICAL_ASSERTION,
                    accession=scv_accession,
                    clinical_significance=description,
                    review_status=review_status,
                    condition_id=condition.condition_id,
                    condition_label=condition.label,
                    submitter=submitter,
                    last_evaluated=last_evaluated,
                    citation_ids=citations,
                ),
                context_scope=scope,
                provenance=SourceProvenance(
                    kind=EvidenceDerivation.SOURCE,
                    source_id=self.source_id,
                    source_record_id=scv_accession,
                    source_version=vcv_version,
                    retrieved_at=retrieved_at,
                    normalized_query_key=variant_key,
                ),
                raw_snapshot_ref=raw_snapshot_ref,
                derivation=EvidenceDerivation.SOURCE,
            )
            for condition in selected
        )
        return items, False

    def _result(
        self,
        *,
        status: SourceStatus,
        evidence_items: tuple[EvidenceItem, ...] = (),
        cache_state: CacheState,
        raw_snapshot_refs: tuple[str, ...] = (),
        diagnostics: tuple[str, ...] = (),
    ) -> EvidenceSourceResult:
        return EvidenceSourceResult(
            source_id=self.source_id,
            evidence_items=evidence_items,
            source_status=status,
            cache_state=cache_state,
            raw_snapshot_refs=raw_snapshot_refs,
            diagnostics=diagnostics,
        )


def _vcv_allele_matches(raw_xml: bytes, variant: NormalizedVariantQuery) -> bool:
    """Require an authoritative VCV SequenceLocation to equal the request allele."""
    try:
        root = ElementTree.fromstring(raw_xml)
    except ElementTree.ParseError as error:
        raise ClinVarSchemaError("invalid VCV XML") from error
    authoritative: list[_AuthoritativeAllele] = []
    for element in root.iter():
        if _local_name(element.tag) != "SequenceLocation":
            continue
        assembly = element.get("Assembly")
        accession = element.get("Accession")
        start = element.get("start")
        stop = element.get("stop")
        reference = element.get("referenceAllele")
        alternate = element.get("alternateAllele")
        if None in (assembly, accession, start, stop, reference, alternate):
            continue
        try:
            authoritative.append(
                _AuthoritativeAllele(
                    assembly=assembly,
                    accession=accession,
                    start=int(start),
                    stop=int(stop),
                    reference=reference,
                    alternate=alternate,
                )
            )
        except ValueError as error:
            raise ClinVarSchemaError(
                "VCV SequenceLocation coordinates are invalid"
            ) from error
    if not authoritative:
        raise ClinVarSchemaError("VCV response lacks an authoritative SequenceLocation")
    return any(
        allele.assembly == variant.genome_build
        and allele.accession == variant.genomic_accession
        and allele.start == variant.genomic_start + 1
        and allele.stop == variant.genomic_end
        and allele.reference == variant.reference_allele
        and allele.alternate == variant.alternate_allele
        for allele in authoritative
    )


def _single_esearch_candidate(raw_xml: bytes) -> str | None:
    try:
        root = ElementTree.fromstring(raw_xml)
    except ElementTree.ParseError as error:
        raise ClinVarSchemaError("invalid ESearch XML") from error
    count_text = _descendant_text(root, "Count")
    if count_text is None or not count_text.isdigit():
        raise ClinVarSchemaError("ESearch Count is absent or invalid")
    count = int(count_text)
    identifiers = [
        (element.text or "").strip()
        for element in root.iter()
        if _local_name(element.tag) == "Id" and (element.text or "").strip()
    ]
    if count == 0:
        return None
    if (
        count != 1
        or len(identifiers) != 1
        or _VARIATION_ID.fullmatch(identifiers[0]) is None
    ):
        return ""
    return identifiers[0]


def _conditions(assertion: ElementTree.Element) -> tuple[_Condition, ...]:
    result: list[_Condition] = []
    for trait in assertion.iter():
        if _local_name(trait.tag) != "Trait":
            continue
        label = _descendant_text(trait, "ElementValue")
        identifier = None
        for xref in trait.iter():
            if _local_name(xref.tag) != "XRef":
                continue
            database = (xref.get("DB") or "").upper()
            value = xref.get("ID") or ""
            candidate = f"{database}:{value}"
            if _ONTOLOGY_ID.fullmatch(candidate):
                identifier = candidate
                break
        result.append(_Condition(identifier, label))
    return tuple(
        sorted(
            set(result),
            key=lambda condition: (condition.condition_id or "", condition.label or ""),
        )
    )


def _select_conditions(
    conditions: tuple[_Condition, ...],
    *,
    requested_condition_id: str | None,
    requested_condition_label: str | None,
) -> tuple[_Condition, ...] | None:
    if requested_condition_id is None and requested_condition_label is None:
        return conditions
    selected = tuple(
        condition
        for condition in conditions
        if (
            requested_condition_id is not None
            and condition.condition_id == requested_condition_id
        )
        or (
            requested_condition_label is not None
            and condition.label is not None
            and condition.label.casefold() == requested_condition_label.casefold()
        )
    )
    return selected or None


def _citation_ids(assertion: ElementTree.Element) -> tuple[str, ...]:
    citations: set[str] = set()
    for citation in assertion.iter():
        if _local_name(citation.tag) != "Citation":
            continue
        source = (citation.get("Source") or citation.get("Type") or "").casefold()
        identifier = (citation.get("ID") or "").strip()
        if source in {"pubmed", "pmid"} and identifier.isdigit():
            citations.add(f"PMID:{identifier}")
        elif source == "doi" and identifier:
            citations.add(f"DOI:{identifier}")
        elif source in {"url", "uri"} and identifier.startswith("https://"):
            citations.add(identifier)
    return tuple(sorted(citations))


def _submitter(assertion: ElementTree.Element) -> str | None:
    for name in ("ClinVarSubmissionID", "Submitter", "Organization"):
        element = _first_descendant(assertion, name)
        if element is None:
            continue
        for attribute in ("SubmitterName", "Name", "OrgAbbreviation"):
            value = element.get(attribute)
            if value and value.strip():
                return value.strip()
    return None


def _parse_date(value: str | None) -> datetime | None:
    if value is None or not value.strip():
        return None
    text = value.strip()
    try:
        if len(text) == 10:
            return datetime.fromisoformat(text).replace(tzinfo=UTC)
        parsed = datetime.fromisoformat(text.replace("Z", "+00:00"))
    except ValueError as error:
        raise ClinVarSchemaError("DateLastEvaluated is invalid") from error
    if parsed.tzinfo is None or parsed.utcoffset() is None:
        return parsed.replace(tzinfo=UTC)
    return parsed.astimezone(UTC)


def _first_descendant(
    element: ElementTree.Element, local_name: str
) -> ElementTree.Element | None:
    return next(
        (child for child in element.iter() if _local_name(child.tag) == local_name),
        None,
    )


def _descendant_text(element: ElementTree.Element, local_name: str) -> str | None:
    descendant = _first_descendant(element, local_name)
    if descendant is None or descendant.text is None:
        return None
    value = descendant.text.strip()
    return value or None


def _local_name(tag: str) -> str:
    return tag.rsplit("}", 1)[-1]


def _optional_text(value: object, attribute: str) -> str | None:
    candidate = getattr(value, attribute, None)
    if not isinstance(candidate, str):
        return None
    candidate = candidate.strip()
    return candidate or None


def _variant_key_or_none(variant: object) -> str | None:
    candidate = getattr(variant, "variant_key", None)
    return candidate if isinstance(candidate, str) and candidate.strip() else None


def _as_utc(value: datetime) -> datetime:
    if value.tzinfo is None or value.utcoffset() is None:
        raise ValueError("clock must return a timezone-aware datetime")
    return value.astimezone(UTC)


def _cached_item_matches(
    item: EvidenceItem,
    *,
    variant_key: str,
    scope: EvidenceContextScope,
    source_id: str,
) -> bool:
    provenance = item.provenance
    return (
        item.variant_key == variant_key
        and item.context_scope == scope
        and isinstance(provenance, SourceProvenance)
        and provenance.source_id == source_id
    )


def _clinical_assertion_sort_key(item: EvidenceItem) -> tuple[str, str, str, str]:
    observation = item.observation
    if not isinstance(observation, ClinicalAssertionObservation):
        raise ClinVarSchemaError("ClinVar mapping produced a non-clinical assertion")
    return (
        observation.accession,
        observation.submitter or "",
        observation.condition_id or "",
        observation.condition_label or "",
    )


def _source_version_from_items(items: tuple[EvidenceItem, ...]) -> str | None:
    versions: set[str | None] = set()
    for item in items:
        provenance = item.provenance
        if not isinstance(provenance, SourceProvenance):
            return None
        versions.add(provenance.source_version)
    return next(iter(versions)) if len(versions) == 1 else None
