"""Immutable, non-authoritative scientist feedback records."""

from __future__ import annotations

from collections.abc import Mapping
from datetime import UTC, datetime
from enum import StrEnum
from typing import Annotated, Self, cast

from pydantic import StringConstraints, field_validator, model_validator

from acmg_classifier.domain.enums import GenomeBuild, InheritanceMode
from acmg_classifier.domain.errors import JsonValue
from acmg_classifier.domain.evidence import EvidenceModel, NonEmptyText, UserId
from acmg_classifier.domain.normalization import CanonicalAlleleKey

FeedbackId = Annotated[
    str,
    StringConstraints(pattern=r"^fb_[0-9a-f]{32,64}$"),
]
ClassificationReference = Annotated[
    str,
    StringConstraints(pattern=r"^cls_[0-9a-f]{32,64}$"),
]
FeedbackReference = Annotated[
    str,
    StringConstraints(strip_whitespace=True, min_length=1, max_length=500),
]


class FeedbackType(StrEnum):
    """The only feedback actions; neither action changes a stored decision."""

    AGREEMENT = "agreement"
    CORRECTION = "correction"


class FeedbackContext(EvidenceModel):
    """The immutable interpretation context copied from a classified record."""

    genome_build: GenomeBuild | None = None
    transcript: str | None = None
    disease_id: str | None = None
    disease_label: str | None = None
    inheritance: InheritanceMode | None = None


class FeedbackSubmission(EvidenceModel):
    """Scientist-provided content before application-owned audit fields are added."""

    classification_id: ClassificationReference
    feedback_type: FeedbackType
    proposed_correction: NonEmptyText | None = None
    rationale: NonEmptyText
    evidence_references: tuple[FeedbackReference, ...] = ()
    actor_id: UserId

    @model_validator(mode="after")
    def validate_feedback_semantics(self) -> Self:
        if (
            self.feedback_type is FeedbackType.CORRECTION
            and self.proposed_correction is None
        ):
            raise ValueError("correction feedback requires a proposed correction")
        if (
            self.feedback_type is FeedbackType.AGREEMENT
            and self.proposed_correction is not None
        ):
            raise ValueError(
                "agreement feedback must not include a proposed correction"
            )
        references = tuple(sorted(set(self.evidence_references)))
        object.__setattr__(self, "evidence_references", references)
        return self


class FeedbackRecord(FeedbackSubmission):
    """A complete feedback artifact with application-derived audit context."""

    feedback_id: FeedbackId
    submitted_at: datetime
    variant_key: NonEmptyText
    context: FeedbackContext

    @field_validator("submitted_at")
    @classmethod
    def normalize_submitted_at(cls, value: datetime) -> datetime:
        if value.tzinfo is None or value.utcoffset() is None:
            raise ValueError("feedback timestamp must be timezone-aware")
        return value.astimezone(UTC)

    def to_canonical_content(self) -> dict[str, JsonValue]:
        """Return only immutable feedback and audit content for storage/export."""
        return cast(dict[str, JsonValue], self.model_dump(mode="json"))


def feedback_context_from_classification(
    content: Mapping[str, object],
) -> tuple[str, FeedbackContext]:
    """Extract canonical variant identity and context from persisted classification."""
    try:
        normalized = _mapping(content["normalized_variant"])
        allele = _mapping(normalized["canonical_key"])
        schema_version = _text(allele["schema_version"])
        if schema_version != "1.0":
            raise ValueError("unsupported canonical allele schema version")
        key = CanonicalAlleleKey(
            assembly=GenomeBuild(_text(allele["assembly"])),
            sequence_accession=_text(allele["sequence_accession"]),
            start=_integer(allele["start"]),
            end=_integer(allele["end"]),
            deleted_sequence=_text(allele["deleted_sequence"]),
            inserted_sequence=_text(allele["inserted_sequence"]),
        )
        context = FeedbackContext.model_validate(_mapping(content["context"]))
    except (KeyError, TypeError, ValueError) as error:
        raise ValueError(
            "classification record lacks valid feedback context"
        ) from error
    return key.value, context


def _mapping(value: object) -> Mapping[str, object]:
    if not isinstance(value, Mapping):
        raise TypeError("expected object")
    return value


def _text(value: object) -> str:
    if not isinstance(value, str):
        raise TypeError("expected text")
    return value


def _integer(value: object) -> int:
    if not isinstance(value, int) or isinstance(value, bool):
        raise TypeError("expected integer")
    return value
