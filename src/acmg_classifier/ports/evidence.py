"""Narrow ports shared by normalized-variant evidence adapters."""

from __future__ import annotations

from dataclasses import dataclass, field
from enum import StrEnum
from typing import Protocol

from acmg_classifier.domain.canonical import canonical_hash
from acmg_classifier.domain.evidence import (
    EvidenceContextScope,
    EvidenceItem,
    EvidencePolicy,
    SourceStatus,
)


class NormalizedVariantQuery(Protocol):
    """The stable, normalization-independent view needed by evidence sources."""

    @property
    def variant_key(self) -> str: ...

    @property
    def genome_build(self) -> str: ...

    @property
    def genomic_hgvs(self) -> str | None: ...


@dataclass(frozen=True, slots=True)
class SourceQuery:
    """Sanitized, versioned source-cache identity inputs.

    ``request_fingerprint`` describes query shape only; adapters must never put
    URLs, headers, credentials, or timestamps into it.
    """

    source_id: str
    normalized_query_key: str
    request_fingerprint: str
    source_version: str | None = None
    adapter_version: str = "1"
    media_type: str | None = None

    def __post_init__(self) -> None:
        if not all(
            (self.source_id, self.normalized_query_key, self.request_fingerprint)
        ):
            raise ValueError("source query fields must not be empty")

    @property
    def cache_key(self) -> str:
        return f"sc_{
            canonical_hash(
                {
                    'source_id': self.source_id,
                    'normalized_query_key': self.normalized_query_key,
                    'request_fingerprint': self.request_fingerprint,
                    'source_version': self.source_version,
                    'adapter_version': self.adapter_version,
                }
            )
        }"

    def with_fingerprint(self, request_fingerprint: str) -> SourceQuery:
        return SourceQuery(
            source_id=self.source_id,
            normalized_query_key=self.normalized_query_key,
            request_fingerprint=request_fingerprint,
            source_version=self.source_version,
            adapter_version=self.adapter_version,
            media_type=self.media_type,
        )


class CacheState(StrEnum):
    """Whether a source result came from an eligible cache entry or live request."""

    LIVE = "live"
    HIT_FRESH = "hit_fresh"
    STALE = "stale"
    INELIGIBLE = "ineligible"
    OFFLINE_MISS = "offline_miss"


@dataclass(frozen=True, slots=True)
class EvidenceSourceResult:
    """Adapter observations and an honest source availability state."""

    source_id: str
    evidence_items: tuple[EvidenceItem, ...]
    source_status: SourceStatus
    cache_state: CacheState
    raw_snapshot_refs: tuple[str, ...] = ()
    diagnostics: tuple[str, ...] = field(default_factory=tuple)


class EvidenceAdapter(Protocol):
    """Implemented by source adapters without importing normalization internals."""

    source_id: str

    async def query(
        self,
        variant: NormalizedVariantQuery,
        *,
        context_scope: EvidenceContextScope,
        policy: EvidencePolicy,
    ) -> EvidenceSourceResult: ...
