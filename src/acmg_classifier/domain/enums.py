"""Stable domain status values."""

from enum import StrEnum


class WorkflowStatus(StrEnum):
    """Outcome of an end-to-end classification request."""

    COMPLETED = "completed"
    NEEDS_CONTEXT = "needs_context"
    DEGRADED = "degraded"
    CONFLICT = "conflict"
    UNSUPPORTED = "unsupported"
    FAILED = "failed"


class CriterionStatus(StrEnum):
    """Outcome of evaluating one ACMG/AMP criterion."""

    APPLIED = "applied"
    NOT_APPLIED = "not_applied"
    NOT_EVALUABLE = "not_evaluable"
    DISABLED = "disabled"
    CONFLICTING = "conflicting"


class AnalysisIntent(StrEnum):
    """Supported scientific analysis intent."""

    GERMLINE_MENDELIAN = "germline_mendelian"


class DetailLevel(StrEnum):
    """Amount of presentation detail requested by a caller."""

    COMPACT = "compact"
    STANDARD = "standard"
    FULL = "full"


class GenomeBuild(StrEnum):
    """Supported human genome assemblies."""

    GRCH37 = "GRCh37"
    GRCH38 = "GRCh38"


class InheritanceMode(StrEnum):
    """Controlled inheritance modes for release-one context."""

    AUTOSOMAL_DOMINANT = "autosomal_dominant"
    AUTOSOMAL_RECESSIVE = "autosomal_recessive"
    X_LINKED = "x_linked"
    UNKNOWN = "unknown"


class ClassificationTier(StrEnum):
    """Five-tier ACMG/AMP classification vocabulary."""

    PATHOGENIC = "pathogenic"
    LIKELY_PATHOGENIC = "likely_pathogenic"
    UNCERTAIN_SIGNIFICANCE = "uncertain_significance"
    LIKELY_BENIGN = "likely_benign"
    BENIGN = "benign"
