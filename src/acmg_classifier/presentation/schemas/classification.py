"""Strict classification request and workflow response schemas."""

from typing import Annotated, Literal

from pydantic import Field, StringConstraints

from acmg_classifier.domain.enums import (
    AnalysisIntent,
    ClassificationTier,
    DetailLevel,
    GenomeBuild,
    InheritanceMode,
    WorkflowStatus,
)
from acmg_classifier.domain.identifiers import (
    ClassificationId,
    DraftId,
    ResumeToken,
)
from acmg_classifier.domain.models import InterpretationContext, VariantInput
from acmg_classifier.presentation.schemas.base import StrictModel

type TranscriptAccession = Annotated[
    str,
    StringConstraints(pattern=r"^(?:N[MR]_\d+\.\d+|ENST\d+\.\d+)$"),
]
type OntologyIdentifier = Annotated[
    str,
    StringConstraints(pattern=r"^[A-Z][A-Z0-9_]*:\d+$"),
]


class OntologyReference(StrictModel):
    """Stable ontology identifier with a scientist-readable label."""

    identifier: OntologyIdentifier
    label: Annotated[str, StringConstraints(min_length=1, max_length=500)]


class ClassificationRequest(StrictModel):
    """Boundary request whose only required field is a variant."""

    schema_version: Literal["1.0"] = "1.0"
    variant: Annotated[str, StringConstraints(min_length=1, max_length=2_000)]
    genome_build: GenomeBuild | None = None
    transcript: TranscriptAccession | None = None
    disease: OntologyReference | None = None
    inheritance: InheritanceMode | None = None
    analysis_intent: AnalysisIntent = AnalysisIntent.GERMLINE_MENDELIAN
    detail_level: DetailLevel = DetailLevel.STANDARD

    def to_domain(self) -> tuple[VariantInput, InterpretationContext]:
        """Convert validated boundary values into immutable domain values."""
        return (
            VariantInput(self.variant),
            InterpretationContext(
                genome_build=self.genome_build,
                transcript=self.transcript,
                disease_id=self.disease.identifier if self.disease else None,
                disease_label=self.disease.label if self.disease else None,
                inheritance=self.inheritance,
            ),
        )


class ContextQuestion(StrictModel):
    """One focused request for scientifically necessary context."""

    field: Annotated[str, StringConstraints(min_length=1, max_length=100)]
    prompt: Annotated[str, StringConstraints(min_length=1, max_length=1_000)]
    reason: Annotated[str, StringConstraints(min_length=1, max_length=1_000)]


class ResponseBase(StrictModel):
    """Fields shared by every classification workflow response."""

    schema_version: Literal["1.0"] = "1.0"
    limitations: tuple[str, ...] = ()


class CompletedResponse(ResponseBase):
    """A completed five-tier classification."""

    status: Literal[WorkflowStatus.COMPLETED]
    classification_id: ClassificationId
    classification: ClassificationTier


class NeedsContextResponse(ResponseBase):
    """A resumable workflow awaiting required interpretation context."""

    status: Literal[WorkflowStatus.NEEDS_CONTEXT]
    draft_id: DraftId
    resume_token: ResumeToken
    questions: Annotated[list[ContextQuestion], Field(min_length=1)]


class DegradedResponse(ResponseBase):
    """An honest partial result produced with unavailable sources."""

    status: Literal[WorkflowStatus.DEGRADED]
    classification: ClassificationTier | None
    unavailable_sources: Annotated[list[str], Field(min_length=1)]


class ConflictResponse(ResponseBase):
    """An unresolved criterion conflict with no final classification."""

    status: Literal[WorkflowStatus.CONFLICT]
    conflicting_criteria: Annotated[list[str], Field(min_length=1)]


class UnsupportedResponse(ResponseBase):
    """A request outside the supported release scope."""

    status: Literal[WorkflowStatus.UNSUPPORTED]
    reason: Annotated[str, StringConstraints(min_length=1, max_length=2_000)]


class FailedResponse(ResponseBase):
    """A workflow failure represented without internal exception details."""

    status: Literal[WorkflowStatus.FAILED]
    error_code: Annotated[str, StringConstraints(pattern=r"^[A-Z][A-Z0-9_]+$")]


type ClassificationResponse = Annotated[
    CompletedResponse
    | NeedsContextResponse
    | DegradedResponse
    | ConflictResponse
    | UnsupportedResponse
    | FailedResponse,
    Field(discriminator="status"),
]
