"""Privacy-safe, structured boundary inputs for case-specific evidence."""

from __future__ import annotations

from typing import Annotated, Literal, Self

from pydantic import Field, StringConstraints, field_validator, model_validator

from acmg_classifier.presentation.schemas.base import StrictModel


type CaseCitationId = Annotated[
    str,
    StringConstraints(
        pattern=(
            r"^(?:PMID:\d+|DOI:10\.\d{4,9}/\S+|https://[A-Za-z0-9][A-Za-z0-9.-]*"
            r"(?::\d{1,5})?(?:/[^\s]*)?)$"
        )
    ),
]
type StructuredText = Annotated[
    str, StringConstraints(strip_whitespace=True, min_length=1, max_length=200)
]
type VariantKey = Annotated[
    str, StringConstraints(strip_whitespace=True, min_length=1, max_length=500)
]

def _reject_path_like_value(value: str) -> str:
    if value.startswith(("/", "~", "./", "../", "file:")) or (
        len(value) >= 3 and value[0].isalpha() and value[1:3] in (":/", ":\\")
    ):
        raise ValueError("local paths are not permitted in case evidence")
    return value


class CaseEvidenceInputBase(StrictModel):
    """Fields shared by privacy-safe user assertions.

    Submission actor, timestamp, and confirmation method are application-owned so
    clients cannot forge provenance. A citation is a stable reference, never an
    attachment path or raw source payload.
    """

    citation_ids: tuple[CaseCitationId, ...] = ()

    @field_validator("citation_ids")
    @classmethod
    def reject_duplicate_citations(cls, value: tuple[str, ...]) -> tuple[str, ...]:
        if len(value) != len(set(value)):
            raise ValueError("citation_ids must not contain duplicates")
        return value


class PhenotypeEvidenceInput(CaseEvidenceInputBase):
    """One HPO-coded phenotype state without patient identifiers or notes."""

    kind: Literal["phenotype"]
    term_id: Annotated[str, StringConstraints(pattern=r"^HP:\d+$")]
    state: Literal["present", "absent", "unknown", "not_applicable"]
    specificity: Literal[
        "highly_specific", "consistent", "nonspecific", "unknown"
    ] | None = None


class SegregationEvidenceInput(CaseEvidenceInputBase):
    """Aggregated, non-identifying segregation evidence."""

    kind: Literal["segregation"]
    family_count: Annotated[int, Field(ge=0, strict=True)] | None = None
    informative_meioses: Annotated[int, Field(ge=0, strict=True)] | None = None
    lod_score: Annotated[float, Field(strict=True, allow_inf_nan=False)] | None = None
    co_segregations: Annotated[int, Field(ge=0, strict=True)] | None = None
    non_segregations: Annotated[int, Field(ge=0, strict=True)] | None = None
    phenotype_defined: bool | None = None

    @model_validator(mode="after")
    def validate_segregation_counts(self) -> Self:
        if self.co_segregations is None and self.non_segregations is None:
            return self
        if self.informative_meioses is None or self.informative_meioses == 0:
            raise ValueError(
                "segregation counts require positive informative_meioses"
            )
        if (
            (self.co_segregations or 0) + (self.non_segregations or 0)
            > self.informative_meioses
        ):
            raise ValueError(
                "co_segregations plus non_segregations must not exceed "
                "informative_meioses"
            )
        return self


class DeNovoEvidenceInput(CaseEvidenceInputBase):
    """Aggregated de novo assertion with explicit confirmation state."""

    kind: Literal["de_novo"]
    confirmation: Literal["confirmed", "assumed", "unknown", "not_applicable"]
    occurrence_count: Annotated[int, Field(ge=0, strict=True)] | None = None
    paternity_confirmed: bool | None = None
    maternity_confirmed: bool | None = None
    phenotype_consistent: bool | None = None


class AllelicEvidenceInput(CaseEvidenceInputBase):
    """Observed other allele and phase, scoped to a canonical variant key."""

    kind: Literal["allelic"]
    other_variant_key: VariantKey
    phase: Literal["cis", "trans", "unknown"]
    occurrence_count: Annotated[int, Field(ge=0, strict=True)] | None = None
    observed_in_affected: bool | None = None

    @field_validator("other_variant_key")
    @classmethod
    def reject_path_like_variant_key(cls, value: str) -> str:
        return _reject_path_like_value(value)


class FunctionalAssayEvidenceInput(CaseEvidenceInputBase):
    """Structured functional assay result without a free-text clinical note."""

    kind: Literal["functional"]
    assay_id: StructuredText
    assay_type: StructuredText
    result: StructuredText
    validation_status: StructuredText
    effect_size: Annotated[float, Field(strict=True, allow_inf_nan=False)] | None = None
    confidence_interval_lower: Annotated[
        float, Field(strict=True, allow_inf_nan=False)
    ] | None = None
    confidence_interval_upper: Annotated[
        float, Field(strict=True, allow_inf_nan=False)
    ] | None = None
    calibration_reference: StructuredText | None = None

    @field_validator(
        "assay_id",
        "assay_type",
        "result",
        "validation_status",
        "calibration_reference",
    )
    @classmethod
    def reject_path_like_assay_fields(cls, value: str | None) -> str | None:
        return None if value is None else _reject_path_like_value(value)

    @model_validator(mode="after")
    def validate_interval(self) -> Self:
        if (
            self.confidence_interval_lower is not None
            and self.confidence_interval_upper is not None
            and self.confidence_interval_lower > self.confidence_interval_upper
        ):
            raise ValueError("confidence interval lower bound must not exceed upper bound")
        return self


class CaseControlEvidenceInput(CaseEvidenceInputBase):
    """Aggregate case-control counts; no participant-level inputs are accepted."""

    kind: Literal["case_control"]
    study_id: StructuredText

    @field_validator("study_id")
    @classmethod
    def reject_path_like_study_id(cls, value: str) -> str:
        return _reject_path_like_value(value)
    case_count: Annotated[int, Field(ge=0, strict=True)] | None = None
    control_count: Annotated[int, Field(ge=0, strict=True)] | None = None
    case_allele_count: Annotated[int, Field(ge=0, strict=True)] | None = None
    control_allele_count: Annotated[int, Field(ge=0, strict=True)] | None = None
    odds_ratio: Annotated[
        float, Field(ge=0, strict=True, allow_inf_nan=False)
    ] | None = None
    p_value: Annotated[
        float, Field(ge=0, le=1, strict=True, allow_inf_nan=False)
    ] | None = None

    @model_validator(mode="after")
    def validate_counts(self) -> Self:
        if (
            self.case_allele_count is not None
            and self.case_count is not None
            and self.case_allele_count > 2 * self.case_count
        ):
            raise ValueError("case_allele_count exceeds diploid case allele capacity")
        if (
            self.control_allele_count is not None
            and self.control_count is not None
            and self.control_allele_count > 2 * self.control_count
        ):
            raise ValueError("control_allele_count exceeds diploid control allele capacity")
        return self


type CaseEvidenceInput = Annotated[
    PhenotypeEvidenceInput
    | SegregationEvidenceInput
    | DeNovoEvidenceInput
    | AllelicEvidenceInput
    | FunctionalAssayEvidenceInput
    | CaseControlEvidenceInput,
    Field(discriminator="kind"),
]


def validate_case_evidence_conflicts(
    evidence: tuple[CaseEvidenceInput, ...],
) -> None:
    """Fail closed on incompatible assertions lacking a safe subject identifier."""

    phenotype_states: dict[str, set[str]] = {}
    de_novo_confirmations: set[str] = set()
    allelic_phases: dict[str, set[str]] = {}

    for item in evidence:
        if isinstance(item, PhenotypeEvidenceInput):
            phenotype_states.setdefault(item.term_id, set()).add(item.state)
        elif isinstance(item, DeNovoEvidenceInput):
            de_novo_confirmations.add(item.confirmation)
        elif isinstance(item, AllelicEvidenceInput) and item.phase != "unknown":
            allelic_phases.setdefault(item.other_variant_key, set()).add(item.phase)

    if any(len(states) > 1 for states in phenotype_states.values()):
        raise ValueError("contradictory phenotype states in case_evidence")
    if "not_applicable" in de_novo_confirmations and len(de_novo_confirmations) > 1:
        raise ValueError("contradictory de novo confirmations in case_evidence")
    if any(len(phases) > 1 for phases in allelic_phases.values()):
        raise ValueError("contradictory allelic phases in case_evidence")
