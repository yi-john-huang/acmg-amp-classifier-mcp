"""Deterministic, provenance-bearing interpretation-context values."""

from __future__ import annotations

from collections.abc import Mapping
from dataclasses import dataclass
from enum import StrEnum
from types import MappingProxyType

from acmg_classifier.domain.enums import GenomeBuild
from acmg_classifier.domain.errors import JsonValue
from acmg_classifier.domain.models import InterpretationContext


class ContextField(StrEnum):
    """Interpretation context fields resolved before criteria evaluation."""

    GENOME_BUILD = "genome_build"
    TRANSCRIPT = "transcript"
    DISEASE = "disease"
    INHERITANCE = "inheritance"


class ContextValueSource(StrEnum):
    """Explicit source labels, ordered by authority in the application service."""

    USER_CONFIRMED = "user_confirmed"
    RULESET_SPECIFICATION = "ruleset_specification"
    BUNDLE_METADATA = "bundle_metadata"
    NORMALIZATION_PROVIDER = "normalization_provider"
    UNRESOLVED = "unresolved"


class ContextIssueCode(StrEnum):
    """Stable context conflicts and missing-context reasons."""

    TRANSCRIPT_NOT_FOUND = "TRANSCRIPT_NOT_FOUND"
    TRANSCRIPT_AMBIGUOUS = "TRANSCRIPT_AMBIGUOUS"
    TRANSCRIPT_CONFLICT = "TRANSCRIPT_CONFLICT"
    GENE_NOT_FOUND = "GENE_NOT_FOUND"
    DISEASE_REQUIRED = "DISEASE_REQUIRED"
    DISEASE_AMBIGUOUS = "DISEASE_AMBIGUOUS"
    INHERITANCE_REQUIRED = "INHERITANCE_REQUIRED"
    CONTEXT_CONFLICT = "CONTEXT_CONFLICT"


@dataclass(frozen=True, slots=True)
class ResolutionProvenance:
    """Why a context value is present, without exposing infrastructure internals."""

    source: ContextValueSource
    source_id: str | None
    record_id: str | None
    confidence: str
    rationale: str

    def to_canonical_content(self) -> dict[str, JsonValue]:
        return {
            "source": self.source.value,
            "source_id": self.source_id,
            "record_id": self.record_id,
            "confidence": self.confidence,
            "rationale": self.rationale,
        }


@dataclass(frozen=True, slots=True)
class ResolvedValue[ValueT]:
    """A context field coupled to its source and any confirmation requirement."""

    field: ContextField
    value: ValueT | None
    provenance: ResolutionProvenance
    requires_confirmation: bool = False

    def to_canonical_content(self) -> dict[str, JsonValue]:
        value = self.value.value if isinstance(self.value, StrEnum) else self.value
        if value is not None and not isinstance(value, (str, int, bool)):
            raise ValueError("resolved context values must be JSON scalar values")
        return {
            "field": self.field.value,
            "value": value,
            "provenance": self.provenance.to_canonical_content(),
            "requires_confirmation": self.requires_confirmation,
        }


@dataclass(frozen=True, slots=True)
class TranscriptCandidate:
    """A compatible transcript retained for transparent ranking or a question."""

    refseq_transcript: str
    ensembl_transcript: str
    gene_symbol: str
    hgnc_id: str
    mane_status: str
    genome_build: GenomeBuild
    genomic_accession: str
    rank: int
    rank_reason: str
    provenance: ResolutionProvenance

    def to_canonical_content(self) -> dict[str, JsonValue]:
        return {
            "refseq_transcript": self.refseq_transcript,
            "ensembl_transcript": self.ensembl_transcript,
            "gene_symbol": self.gene_symbol,
            "hgnc_id": self.hgnc_id,
            "mane_status": self.mane_status,
            "genome_build": self.genome_build.value,
            "genomic_accession": self.genomic_accession,
            "rank": self.rank,
            "rank_reason": self.rank_reason,
            "provenance": self.provenance.to_canonical_content(),
        }


@dataclass(frozen=True, slots=True)
class ContextIssue:
    """A non-silent conflict that prevents a context from being trusted."""

    code: ContextIssueCode
    message: str

    def to_canonical_content(self) -> dict[str, JsonValue]:
        return {"code": self.code.value, "message": self.message}


@dataclass(frozen=True, slots=True)
class ContextQuestion:
    """A minimal, machine-readable request for one unresolved context value."""

    field: ContextField
    code: ContextIssueCode
    prompt: str
    reason: str
    answer_schema: Mapping[str, JsonValue]
    choices: tuple[Mapping[str, JsonValue], ...] = ()
    provenance: tuple[ResolutionProvenance, ...] = ()

    def __post_init__(self) -> None:
        object.__setattr__(
            self, "answer_schema", MappingProxyType(dict(self.answer_schema))
        )
        object.__setattr__(
            self,
            "choices",
            tuple(MappingProxyType(dict(choice)) for choice in self.choices),
        )
        object.__setattr__(
            self,
            "provenance",
            tuple(
                sorted(
                    self.provenance,
                    key=lambda item: (
                        item.source.value,
                        item.source_id or "",
                        item.record_id or "",
                        item.rationale,
                    ),
                )
            ),
        )

    def to_canonical_content(self) -> dict[str, JsonValue]:
        return {
            "field": self.field.value,
            "code": self.code.value,
            "prompt": self.prompt,
            "reason": self.reason,
            "answer_schema": dict(self.answer_schema),
            "choices": [dict(choice) for choice in self.choices],
            "provenance": [item.to_canonical_content() for item in self.provenance],
        }


@dataclass(frozen=True, slots=True)
class ContextSpecification:
    """The context selectors of one already-selected approved ruleset."""

    ruleset_id: str
    version: str
    transcript: str | None = None
    disease_id: str | None = None
    disease_label: str | None = None
    inheritance: str | None = None

    @property
    def source_id(self) -> str:
        return f"{self.ruleset_id}@{self.version}"


@dataclass(frozen=True, slots=True)
class ContextResolution:
    """The ready context or the smallest ordered set of questions needed next."""

    resolved: InterpretationContext
    values: tuple[ResolvedValue[object], ...]
    transcript_candidates: tuple[TranscriptCandidate, ...]
    questions: tuple[ContextQuestion, ...]
    issues: tuple[ContextIssue, ...] = ()
    status: str = "complete"

    def __post_init__(self) -> None:
        ordered_values = tuple(sorted(self.values, key=lambda value: value.field.value))
        if len({value.field for value in ordered_values}) != len(ContextField):
            raise ValueError("context resolution must include every context field once")
        object.__setattr__(self, "values", ordered_values)
        object.__setattr__(
            self,
            "transcript_candidates",
            tuple(
                sorted(
                    self.transcript_candidates,
                    key=lambda item: (
                        item.rank,
                        item.refseq_transcript,
                        item.ensembl_transcript,
                    ),
                )
            ),
        )
        object.__setattr__(
            self,
            "issues",
            tuple(
                sorted(self.issues, key=lambda issue: (issue.code.value, issue.message))
            ),
        )
        if self.status not in {"complete", "needs_context", "conflict"}:
            raise ValueError("context resolution status is unsupported")
        if self.status == "complete" and (self.questions or self.issues):
            raise ValueError(
                "complete context resolution cannot have questions or conflicts"
            )
        if self.status == "conflict" and not self.issues:
            raise ValueError("conflict context resolution requires an issue")

    def value_for(self, field: ContextField) -> ResolvedValue[object]:
        """Return exactly one resolved value for the requested field."""
        return next(value for value in self.values if value.field is field)

    def to_canonical_content(self) -> dict[str, JsonValue]:
        return {
            "resolved": {
                "genome_build": self.resolved.genome_build.value
                if self.resolved.genome_build is not None
                else None,
                "transcript": self.resolved.transcript,
                "disease_id": self.resolved.disease_id,
                "disease_label": self.resolved.disease_label,
                "inheritance": self.resolved.inheritance.value
                if self.resolved.inheritance is not None
                else None,
            },
            "values": [value.to_canonical_content() for value in self.values],
            "transcript_candidates": [
                candidate.to_canonical_content()
                for candidate in self.transcript_candidates
            ],
            "questions": [
                question.to_canonical_content() for question in self.questions
            ],
            "issues": [issue.to_canonical_content() for issue in self.issues],
            "status": self.status,
        }
