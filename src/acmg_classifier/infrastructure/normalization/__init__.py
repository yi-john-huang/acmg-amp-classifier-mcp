"""Infrastructure adapters for provider-backed normalization."""

from acmg_classifier.infrastructure.normalization.knowledge import (
    ActiveKnowledgeBundleLocator,
    GeneDiseaseRow,
    KnowledgeBundle,
    KnowledgeBundleError,
    KnowledgeBundleRepository,
    TranscriptMappingRow,
)
from acmg_classifier.infrastructure.normalization.local import (
    LocalBundleNormalizationProvider,
)

__all__ = [
    "ActiveKnowledgeBundleLocator",
    "GeneDiseaseRow",
    "KnowledgeBundle",
    "KnowledgeBundleError",
    "KnowledgeBundleRepository",
    "LocalBundleNormalizationProvider",
    "TranscriptMappingRow",
]
