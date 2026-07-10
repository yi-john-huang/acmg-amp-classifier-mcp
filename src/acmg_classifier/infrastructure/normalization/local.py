"""Offline normalization provider backed only by immutable local bundle data."""

from __future__ import annotations

from collections.abc import Mapping
from dataclasses import replace
from types import MappingProxyType
from typing import Literal

from acmg_classifier.domain.models import InterpretationContext
from acmg_classifier.domain.normalization import (
    NormalizationFailureCode,
    NormalizedVariant,
    ParsedVariant,
    ProviderProvenance,
)
from acmg_classifier.infrastructure.normalization.knowledge import (
    KnowledgeBundleRepository,
)
from acmg_classifier.ports.normalization import (
    NormalizationFailure,
    NormalizationPolicy,
    NormalizationProviderResult,
    NormalizationSuccess,
)


class LocalBundleNormalizationProvider:
    """Replay cached alleles without treating metadata as sequence data.

    The Task 3.4 core knowledge artifact contains transcript and gene metadata, not
    reference bases. Consequently this provider only returns a normalized allele
    when an injected immutable runtime cache holds a prior reference-validated
    result for the exact parsed query. It never attempts a network fallback.
    """

    provider_id = "local_bundle"
    capabilities: frozenset[Literal["live", "offline"]] = frozenset({"offline"})

    def __init__(
        self,
        knowledge: KnowledgeBundleRepository,
        *,
        cached_results: Mapping[str, NormalizedVariant] | None = None,
    ) -> None:
        cache = dict(cached_results or {})
        if any(not isinstance(query, str) or not query for query in cache):
            raise ValueError("normalization cache keys must be non-empty query strings")
        if any(not isinstance(result, NormalizedVariant) for result in cache.values()):
            raise ValueError(
                "normalization cache values must be NormalizedVariant instances"
            )
        self._knowledge = knowledge
        self._cached_results: Mapping[str, NormalizedVariant] = MappingProxyType(cache)

    def normalize(
        self,
        parsed: ParsedVariant,
        context: InterpretationContext,
        policy: NormalizationPolicy,
    ) -> NormalizationProviderResult:
        """Return an exact cached replay or an honest unavailable result.

        ``context`` is deliberately not used to manufacture sequence validation
        from a transcript range. Build constraints only make an otherwise cached
        result ineligible.
        """
        cached = self._cached_results.get(parsed.normalized_input)
        if cached is None:
            return self._unavailable(parsed, "no_reference_data")
        if (
            policy.genome_build is not None
            and cached.genome_build != policy.genome_build
        ):
            return self._unavailable(parsed, "requested_build_not_cached")
        if cached.genome_build not in self._knowledge.bundle.manifest.genome_builds:
            return self._unavailable(parsed, "cached_build_not_in_bundle")

        provenance = ProviderProvenance(
            provider_id=self.provider_id,
            provider_version="1.0",
            source_record_id=None,
            retrieved_at=None,
            bundle_version=self._knowledge.bundle.manifest.bundle_version,
            raw_snapshot_ref=self._knowledge.bundle.artifact_path,
            query_key=parsed.normalized_input,
        )
        normalized = replace(
            cached,
            original_input=parsed.original_input,
            parsed_input=parsed.normalized_input,
            provider_provenance=(*cached.provider_provenance, provenance),
        )
        return NormalizationSuccess(normalized=normalized)

    def _unavailable(self, parsed: ParsedVariant, reason: str) -> NormalizationFailure:
        bundle = self._knowledge.bundle
        return NormalizationFailure(
            code=NormalizationFailureCode.NORMALIZATION_UNAVAILABLE,
            message=(
                "active knowledge bundle has no reference sequence or cached "
                "normalization for this allele"
            ),
            retryable=False,
            provider_id=self.provider_id,
            details={
                "reason": reason,
                "status": "unavailable",
                "bundle_version": bundle.manifest.bundle_version,
                "resource": bundle.artifact_path,
                "query_key": parsed.normalized_input,
            },
        )
