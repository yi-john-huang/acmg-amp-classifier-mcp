"""Strict, provenance-preserving domain models for ACMG evidence."""

from __future__ import annotations

from collections.abc import Iterable
from datetime import UTC, datetime
from enum import StrEnum
from typing import Annotated, Literal, Self

from pydantic import (
    BaseModel,
    ConfigDict,
    Field,
    StringConstraints,
    field_validator,
    model_validator,
)

from acmg_classifier.domain.canonical import canonical_hash
from acmg_classifier.domain.enums import GenomeBuild, InheritanceMode

NonEmptyText = Annotated[str, StringConstraints(strip_whitespace=True, min_length=1)]
VariantKey = Annotated[
    str, StringConstraints(strip_whitespace=True, min_length=1, max_length=500)
]
OntologyId = Annotated[str, StringConstraints(pattern=r"^[A-Z][A-Z0-9_]*:\d+$")]
TranscriptAccession = Annotated[
    str, StringConstraints(pattern=r"^(?:N[MR]_\d+\.\d+|ENST\d+\.\d+)$")
]
SourceId = Annotated[str, StringConstraints(pattern=r"^[a-z][a-z0-9_:-]{1,63}$")]
CitationId = Annotated[
    str, StringConstraints(pattern=r"^(?:PMID:\d+|DOI:.+|https://.+)$")
]
EvidenceId = Annotated[str, StringConstraints(pattern=r"^ev_[0-9a-f]{64}$")]
EvidenceSnapshotId = Annotated[str, StringConstraints(pattern=r"^es_[0-9a-f]{64}$")]
RawSnapshotRef = Annotated[str, StringConstraints(pattern=r"^raw_[0-9a-f]{64}$")]
UserId = Annotated[str, StringConstraints(pattern=r"^usr_[A-Za-z0-9_.:-]{3,128}$")]
ReviewerId = Annotated[
    str, StringConstraints(pattern=r"^(?:agent|usr)_[A-Za-z0-9_.:-]{3,128}$")
]
ReviewId = Annotated[str, StringConstraints(pattern=r"^review_[0-9a-f]{32,64}$")]


class EvidenceModel(BaseModel):
    """Immutable domain model that rejects data outside its declared contract."""

    model_config = ConfigDict(extra="forbid", frozen=True)


class ObservationKind(StrEnum):
    """Supported scientific observation families."""

    POPULATION = "population"
    CLINICAL_ASSERTION = "clinical_assertion"
    COMPUTATIONAL = "computational"
    FUNCTIONAL = "functional"
    SEGREGATION = "segregation"
    DE_NOVO = "de_novo"
    ALLELIC = "allelic"
    PHENOTYPE = "phenotype"
    GENE_MECHANISM = "gene_mechanism"
    CONSEQUENCE = "consequence"
    VARIANT_LOCATION = "variant_location"
    CASE_CONTROL = "case_control"


class EvidenceDerivation(StrEnum):
    """Origin classes that may produce a typed evidence item."""

    SOURCE = "source"
    USER = "user"
    DERIVED = "derived"
    REVIEW = "review"


class QualityFlag(StrEnum):
    """Controlled quality annotations, deliberately not a score."""

    VERIFIED = "verified"
    STALE = "stale"
    MALFORMED = "malformed"
    INCOMPLETE = "incomplete"
    CONFLICTING = "conflicting"
    FILTERED = "filtered"


class SourceStatusValue(StrEnum):
    """Availability/freshness state for an evidence source."""

    FRESH = "fresh"
    CACHED = "cached"
    STALE = "stale"
    UNAVAILABLE = "unavailable"
    RATE_LIMITED = "rate_limited"
    TIMEOUT = "timeout"
    SCHEMA_CHANGED = "schema_changed"
    INELIGIBLE_CACHE = "ineligible_cache"


class EvidencePolicyMode(StrEnum):
    """Acquisition policy represented by an evidence snapshot."""

    LIVE = "live"
    CACHE = "cache"
    OFFLINE = "offline"


def _normalize_utc(value: datetime) -> datetime:
    """Reject ambiguous instants and store accepted instants in UTC."""
    if value.tzinfo is None or value.utcoffset() is None:
        raise ValueError("timestamp must be timezone-aware")
    return value.astimezone(UTC)


# Source AF values are commonly rendered to six decimal places. Accept only
# that published rounding error when AC and AN are also authoritative.
POPULATION_FREQUENCY_ROUNDING_TOLERANCE = 1e-6


class PopulationObservation(EvidenceModel):
    kind: Literal[ObservationKind.POPULATION]
    source_release: NonEmptyText
    ancestry: NonEmptyText | None = None
    allele_count: Annotated[int, Field(ge=0, strict=True)] | None = None
    allele_number: Annotated[int, Field(ge=0, strict=True)] | None = None
    allele_frequency: (
        Annotated[float, Field(ge=0, le=1, strict=True, allow_inf_nan=False)] | None
    ) = None
    homozygote_count: Annotated[int, Field(ge=0, strict=True)] | None = None
    hemizygote_count: Annotated[int, Field(ge=0, strict=True)] | None = None
    coverage: Annotated[float, Field(ge=0, strict=True, allow_inf_nan=False)] | None = (
        None
    )
    filter_status: NonEmptyText

    @model_validator(mode="after")
    def validate_counts(self) -> Self:
        if (
            self.allele_count is not None
            and self.allele_number is not None
            and self.allele_count > self.allele_number
        ):
            raise ValueError("allele_count must not exceed allele_number")
        if (
            self.allele_count is not None
            and self.allele_number is not None
            and self.allele_frequency is not None
        ):
            if self.allele_number == 0:
                if self.allele_frequency != 0:
                    raise ValueError(
                        "allele_frequency must be zero when allele_number is zero"
                    )
            elif (
                abs(self.allele_frequency - (self.allele_count / self.allele_number))
                > POPULATION_FREQUENCY_ROUNDING_TOLERANCE
            ):
                raise ValueError(
                    "allele_frequency must match allele_count / allele_number "
                    "within the documented 1e-6 rounding tolerance"
                )
        return self


class ClinicalAssertionObservation(EvidenceModel):
    kind: Literal[ObservationKind.CLINICAL_ASSERTION]
    accession: Annotated[
        str, StringConstraints(pattern=r"^(?:SCV|RCV|VCV)\d+(?:\.\d+)?$")
    ]
    clinical_significance: NonEmptyText
    review_status: NonEmptyText
    condition_id: OntologyId | None = None
    condition_label: NonEmptyText | None = None
    submitter: NonEmptyText | None = None
    last_evaluated: datetime | None = None
    citation_ids: tuple[CitationId, ...] = ()

    @field_validator("last_evaluated")
    @classmethod
    def normalize_last_evaluated(cls, value: datetime | None) -> datetime | None:
        return None if value is None else _normalize_utc(value)


class ComputationalObservation(EvidenceModel):
    kind: Literal[ObservationKind.COMPUTATIONAL]
    predictor: NonEmptyText
    prediction: NonEmptyText
    score: Annotated[float, Field(strict=True, allow_inf_nan=False)] | None = None
    threshold: Annotated[float, Field(strict=True, allow_inf_nan=False)] | None = None
    dataset_version: NonEmptyText
    transcript: TranscriptAccession | None = None


class FunctionalObservation(EvidenceModel):
    kind: Literal[ObservationKind.FUNCTIONAL]
    assay_id: NonEmptyText
    assay_type: NonEmptyText
    result: NonEmptyText
    validation_status: NonEmptyText
    effect_size: Annotated[float, Field(strict=True, allow_inf_nan=False)] | None = None
    confidence_interval_lower: (
        Annotated[float, Field(strict=True, allow_inf_nan=False)] | None
    ) = None
    confidence_interval_upper: (
        Annotated[float, Field(strict=True, allow_inf_nan=False)] | None
    ) = None
    calibration_reference: NonEmptyText | None = None

    @model_validator(mode="after")
    def validate_interval(self) -> Self:
        if (
            self.confidence_interval_lower is not None
            and self.confidence_interval_upper is not None
            and self.confidence_interval_lower > self.confidence_interval_upper
        ):
            raise ValueError(
                "confidence interval lower bound must not exceed upper bound"
            )
        return self


class SegregationObservation(EvidenceModel):
    kind: Literal[ObservationKind.SEGREGATION]
    family_count: Annotated[int, Field(ge=0, strict=True)] | None = None
    informative_meioses: Annotated[int, Field(ge=0, strict=True)] | None = None
    lod_score: Annotated[float, Field(strict=True, allow_inf_nan=False)] | None = None
    co_segregations: Annotated[int, Field(ge=0, strict=True)] | None = None
    non_segregations: Annotated[int, Field(ge=0, strict=True)] | None = None
    phenotype_defined: bool | None = None

    @model_validator(mode="after")
    def validate_segregation_counts(self) -> Self:
        if (
            self.informative_meioses is not None
            and self.co_segregations is not None
            and self.non_segregations is not None
            and self.co_segregations + self.non_segregations
            > self.informative_meioses
        ):
            raise ValueError(
                "co_segregations plus non_segregations must not exceed "
                "informative_meioses"
            )
        return self


class DeNovoObservation(EvidenceModel):
    kind: Literal[ObservationKind.DE_NOVO]
    confirmation: Literal["confirmed", "assumed", "unknown", "not_applicable"]
    occurrence_count: Annotated[int, Field(ge=0, strict=True)] | None = None
    paternity_confirmed: bool | None = None
    maternity_confirmed: bool | None = None
    phenotype_consistent: bool | None = None


class AllelicObservation(EvidenceModel):
    kind: Literal[ObservationKind.ALLELIC]
    other_variant_key: VariantKey
    phase: Literal["cis", "trans", "unknown"]
    occurrence_count: Annotated[int, Field(ge=0, strict=True)] | None = None
    observed_in_affected: bool | None = None


class PhenotypeObservation(EvidenceModel):
    kind: Literal[ObservationKind.PHENOTYPE]
    term_id: Annotated[str, StringConstraints(pattern=r"^HP:\d+$")]
    state: Literal["present", "absent", "unknown", "not_applicable"]
    specificity: (
        Literal["highly_specific", "consistent", "nonspecific", "unknown"] | None
    ) = None


class GeneMechanismObservation(EvidenceModel):
    kind: Literal[ObservationKind.GENE_MECHANISM]
    gene_id: Annotated[str, StringConstraints(pattern=r"^HGNC:\d+$")]
    disease_id: OntologyId
    mechanism: Literal[
        "loss_of_function",
        "gain_of_function",
        "dominant_negative",
        "haploinsufficiency",
        "unknown",
    ]
    validity: Literal[
        "definitive", "strong", "moderate", "limited", "disputed", "refuted", "unknown"
    ]
    inheritance: InheritanceMode | None = None
    missense_mechanism: Literal["established", "not_established", "unknown"] = "unknown"
    benign_missense_rate: Literal["low", "high", "unknown"] = "unknown"


class ConsequenceObservation(EvidenceModel):
    """Structured transcript consequence facts required by location criteria."""

    kind: Literal[ObservationKind.CONSEQUENCE]
    transcript: TranscriptAccession
    consequence: Literal[
        "nonsense",
        "frameshift",
        "canonical_splice",
        "missense",
        "inframe_indel",
        "synonymous",
        "splice_region",
        "start_lost",
        "stop_lost",
        "unknown",
    ]
    protein_change: NonEmptyText | None = None
    nmd_predicted: bool | None = None
    same_amino_acid_change: bool | None = None
    same_amino_acid_splice_difference: bool | None = None
    same_residue_different_amino_acid: bool | None = None
    inframe_length: Annotated[int, Field(ge=0, strict=True)] | None = None
    splice_impact: Literal["none", "predicted", "confirmed", "unknown"] = "unknown"

    @model_validator(mode="after")
    def validate_consequence_details(self) -> Self:
        if self.inframe_length is not None and self.consequence != "inframe_indel":
            raise ValueError("inframe_length requires an inframe_indel consequence")
        return self


class VariantLocationObservation(EvidenceModel):
    kind: Literal[ObservationKind.VARIANT_LOCATION]
    region_type: NonEmptyText
    region_id: NonEmptyText | None = None
    is_critical: bool | None = None
    distance_to_splice_site: Annotated[int, Field(strict=True)] | None = None


class CaseControlObservation(EvidenceModel):
    kind: Literal[ObservationKind.CASE_CONTROL]
    study_id: NonEmptyText
    case_count: Annotated[int, Field(ge=0, strict=True)] | None = None
    control_count: Annotated[int, Field(ge=0, strict=True)] | None = None
    case_allele_count: Annotated[int, Field(ge=0, strict=True)] | None = None
    control_allele_count: Annotated[int, Field(ge=0, strict=True)] | None = None
    odds_ratio: (
        Annotated[float, Field(ge=0, strict=True, allow_inf_nan=False)] | None
    ) = None
    p_value: (
        Annotated[float, Field(ge=0, le=1, strict=True, allow_inf_nan=False)] | None
    ) = None

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
            raise ValueError(
                "control_allele_count exceeds diploid control allele capacity"
            )
        return self


Observation = Annotated[
    PopulationObservation
    | ClinicalAssertionObservation
    | ComputationalObservation
    | FunctionalObservation
    | SegregationObservation
    | DeNovoObservation
    | AllelicObservation
    | PhenotypeObservation
    | GeneMechanismObservation
    | ConsequenceObservation
    | VariantLocationObservation
    | CaseControlObservation,
    Field(discriminator="kind"),
]


class SourceProvenance(EvidenceModel):
    kind: Literal[EvidenceDerivation.SOURCE]
    source_id: SourceId
    source_record_id: NonEmptyText
    source_version: NonEmptyText | None = None
    retrieved_at: datetime
    normalized_query_key: VariantKey

    @field_validator("retrieved_at")
    @classmethod
    def normalize_retrieved_at(cls, value: datetime) -> datetime:
        return _normalize_utc(value)


class UserProvenance(EvidenceModel):
    kind: Literal[EvidenceDerivation.USER]
    actor_id: UserId
    submitted_at: datetime
    confirmation_method: NonEmptyText
    citation_ids: tuple[CitationId, ...] = ()

    @field_validator("submitted_at")
    @classmethod
    def normalize_submitted_at(cls, value: datetime) -> datetime:
        return _normalize_utc(value)


class DerivedProvenance(EvidenceModel):
    kind: Literal[EvidenceDerivation.DERIVED]
    derivation_name: NonEmptyText
    component_version: NonEmptyText
    input_evidence_ids: Annotated[tuple[EvidenceId, ...], Field(min_length=1)]
    generated_at: datetime

    @field_validator("generated_at")
    @classmethod
    def normalize_generated_at(cls, value: datetime) -> datetime:
        return _normalize_utc(value)


class ReviewProvenance(EvidenceModel):
    kind: Literal[EvidenceDerivation.REVIEW]
    review_id: ReviewId
    reviewer_id: ReviewerId
    reviewed_at: datetime
    input_evidence_ids: Annotated[tuple[EvidenceId, ...], Field(min_length=1)]

    @field_validator("reviewed_at")
    @classmethod
    def normalize_reviewed_at(cls, value: datetime) -> datetime:
        return _normalize_utc(value)


Provenance = Annotated[
    SourceProvenance | UserProvenance | DerivedProvenance | ReviewProvenance,
    Field(discriminator="kind"),
]


class EvidenceContextScope(EvidenceModel):
    """The bounded biological context in which an item can be applied."""

    genome_build: GenomeBuild | None = None
    transcript: TranscriptAccession | None = None
    disease_id: OntologyId | None = None
    inheritance: InheritanceMode | None = None

    @model_validator(mode="after")
    def require_scope(self) -> Self:
        if not any(
            (
                self.genome_build,
                self.transcript,
                self.disease_id,
                self.inheritance,
            )
        ):
            raise ValueError("context_scope must contain at least one scoped field")
        return self


class EvidenceItem(EvidenceModel):
    """One content-addressed, scoped observation with typed provenance."""

    evidence_id: EvidenceId | None = None
    variant_key: VariantKey
    kind: ObservationKind
    observation: Observation
    context_scope: EvidenceContextScope
    provenance: Provenance
    raw_snapshot_ref: RawSnapshotRef | None = None
    quality_flags: tuple[QualityFlag, ...] = ()
    derivation: EvidenceDerivation

    @model_validator(mode="after")
    def validate_and_assign_evidence_id(self) -> Self:
        if self.kind is not self.observation.kind:
            raise ValueError("kind must match observation.kind")
        if self.derivation is not self.provenance.kind:
            raise ValueError("derivation must match provenance.kind")
        if self.derivation is EvidenceDerivation.SOURCE:
            if self.raw_snapshot_ref is None:
                raise ValueError("source evidence requires raw_snapshot_ref")
        elif self.raw_snapshot_ref is not None:
            raise ValueError("raw_snapshot_ref is only valid for source evidence")

        computed = self.compute_evidence_id()
        if self.evidence_id is not None and self.evidence_id != computed:
            raise ValueError("evidence_id does not match canonical content")
        object.__setattr__(self, "evidence_id", computed)
        return self

    def canonical_content(self) -> dict[str, object]:
        """Return the complete identity-bearing payload, excluding its derived ID."""
        return self.model_dump(mode="json", exclude={"evidence_id"})

    def compute_evidence_id(self) -> str:
        """Compute the deterministic content ID from validated domain data."""
        return f"ev_{canonical_hash(self.canonical_content())}"

    def assert_applies_to(
        self,
        *,
        variant_key: str,
        context_scope: EvidenceContextScope,
    ) -> None:
        """Reject evidence whose derivation-specific scope cannot serve the request."""
        if self.variant_key != variant_key:
            raise ValueError("evidence variant_key does not match requested variant")
        if self.derivation is not EvidenceDerivation.SOURCE:
            if self.context_scope != context_scope:
                raise ValueError("evidence context_scope must exactly match request")
            return
        for field_name in (
            "genome_build",
            "transcript",
            "disease_id",
            "inheritance",
        ):
            source_value = getattr(self.context_scope, field_name)
            if source_value is not None and source_value != getattr(
                context_scope, field_name
            ):
                raise ValueError("evidence context_scope is incompatible with request")


class SourceStatus(EvidenceModel):
    """Auditable acquisition result for one source in a snapshot."""

    source_id: SourceId
    status: SourceStatusValue
    checked_at: datetime
    source_version: NonEmptyText | None = None
    normalized_query_key: VariantKey | None = None
    detail: NonEmptyText | None = None

    @field_validator("checked_at")
    @classmethod
    def normalize_checked_at(cls, value: datetime) -> datetime:
        return _normalize_utc(value)


class EvidencePolicy(EvidenceModel):
    """The source-acquisition policy in force when a snapshot was made."""

    mode: EvidencePolicyMode
    max_age_seconds: Annotated[int, Field(ge=0, strict=True)] | None = None


class EvidenceSnapshot(EvidenceModel):
    """Order-independent content address for a complete evidence acquisition state."""

    snapshot_id: EvidenceSnapshotId | None = None
    evidence_ids: tuple[EvidenceId, ...]
    source_statuses: tuple[SourceStatus, ...]
    policy: EvidencePolicy
    created_at: datetime

    @field_validator("created_at")
    @classmethod
    def normalize_created_at(cls, value: datetime) -> datetime:
        return _normalize_utc(value)

    @model_validator(mode="after")
    def validate_and_assign_snapshot_id(self) -> Self:
        evidence_ids = tuple(sorted(set(self.evidence_ids)))
        source_statuses = tuple(
            sorted(
                self.source_statuses,
                key=lambda status: (
                    status.source_id,
                    status.status.value,
                    status.checked_at.isoformat(),
                    status.source_version or "",
                    status.normalized_query_key or "",
                    status.detail or "",
                ),
            )
        )
        source_ids = tuple(status.source_id for status in source_statuses)
        if len(source_ids) != len(set(source_ids)):
            raise ValueError(
                "source_statuses must contain at most one status per source"
            )
        if not evidence_ids and (
            not source_statuses
            or all(
                status.status in (SourceStatusValue.FRESH, SourceStatusValue.CACHED)
                for status in source_statuses
            )
        ):
            raise ValueError("empty evidence_ids require a non-success source status")

        computed = self.compute_snapshot_id(
            evidence_ids=evidence_ids,
            source_statuses=source_statuses,
        )
        if self.snapshot_id is not None and self.snapshot_id != computed:
            raise ValueError("snapshot_id does not match canonical content")
        object.__setattr__(self, "evidence_ids", evidence_ids)
        object.__setattr__(self, "source_statuses", source_statuses)
        object.__setattr__(self, "snapshot_id", computed)
        return self

    def canonical_content(
        self,
        *,
        evidence_ids: tuple[str, ...] | None = None,
        source_statuses: tuple[SourceStatus, ...] | None = None,
    ) -> dict[str, object]:
        """Return snapshot identity content; creation time is metadata, not identity."""
        return {
            "evidence_ids": list(evidence_ids or self.evidence_ids),
            "source_statuses": [
                status.model_dump(mode="json")
                for status in (source_statuses or self.source_statuses)
            ],
            "policy": self.policy.model_dump(mode="json"),
        }

    def compute_snapshot_id(
        self,
        *,
        evidence_ids: tuple[str, ...] | None = None,
        source_statuses: tuple[SourceStatus, ...] | None = None,
    ) -> str:
        """Compute the deterministic snapshot ID from its canonical content."""
        content = self.canonical_content(
            evidence_ids=evidence_ids,
            source_statuses=source_statuses,
        )
        return f"es_{canonical_hash(content)}"


class FactSet(EvidenceModel):
    """Immutable typed indexes consumed by criteria evaluators."""

    evidence_items: tuple[EvidenceItem, ...] = ()
    population: tuple[PopulationObservation, ...] = ()
    clinical_assertions: tuple[ClinicalAssertionObservation, ...] = ()
    computational: tuple[ComputationalObservation, ...] = ()
    functional: tuple[FunctionalObservation, ...] = ()
    segregation: tuple[SegregationObservation, ...] = ()
    de_novo: tuple[DeNovoObservation, ...] = ()
    allelic: tuple[AllelicObservation, ...] = ()
    phenotype: tuple[PhenotypeObservation, ...] = ()
    gene_mechanism: tuple[GeneMechanismObservation, ...] = ()
    consequence: tuple[ConsequenceObservation, ...] = ()
    variant_location: tuple[VariantLocationObservation, ...] = ()
    case_control: tuple[CaseControlObservation, ...] = ()

    @model_validator(mode="after")
    def validate_evidence_items(self) -> Self:
        """Reject duplicate IDs and retain a deterministic item order."""
        if not self.evidence_items:
            return self
        ordered = tuple(
            sorted(
                self.evidence_items,
                key=lambda item: item.evidence_id or "",
            )
        )
        evidence_ids = tuple(item.evidence_id for item in ordered)
        if any(evidence_id is None for evidence_id in evidence_ids):
            raise ValueError("FactSet evidence_items must have evidence_id values")
        if len(evidence_ids) != len(set(evidence_ids)):
            raise ValueError("FactSet contains duplicate evidence_id values")
        object.__setattr__(self, "evidence_items", ordered)
        return self

    def evidence_ids_for(self, observation: Observation) -> tuple[str, ...]:
        """Return sorted source item IDs for exactly matching typed observations."""
        evidence_ids = tuple(
            item.evidence_id
            for item in self.evidence_items
            if item.observation == observation
        )
        if any(evidence_id is None for evidence_id in evidence_ids):
            raise RuntimeError("FactSet evidence_items must have evidence_id values")
        return tuple(
            evidence_id for evidence_id in evidence_ids if evidence_id is not None
        )

    @classmethod
    def from_evidence(cls, items: Iterable[EvidenceItem]) -> Self:
        """Index evidence deterministically without criterion conclusions."""
        population: list[PopulationObservation] = []
        clinical_assertions: list[ClinicalAssertionObservation] = []
        computational: list[ComputationalObservation] = []
        functional: list[FunctionalObservation] = []
        segregation: list[SegregationObservation] = []
        de_novo: list[DeNovoObservation] = []
        allelic: list[AllelicObservation] = []
        phenotype: list[PhenotypeObservation] = []
        gene_mechanism: list[GeneMechanismObservation] = []
        consequence: list[ConsequenceObservation] = []
        variant_location: list[VariantLocationObservation] = []
        case_control: list[CaseControlObservation] = []
        ordered_items = tuple(
            sorted(items, key=lambda evidence: evidence.evidence_id or "")
        )
        evidence_ids = tuple(item.evidence_id for item in ordered_items)
        if any(evidence_id is None for evidence_id in evidence_ids):
            raise ValueError("FactSet evidence_items must have evidence_id values")
        if len(evidence_ids) != len(set(evidence_ids)):
            raise ValueError("FactSet contains duplicate evidence_id values")
        for item in ordered_items:
            observation = item.observation
            if isinstance(observation, PopulationObservation):
                population.append(observation)
            elif isinstance(observation, ClinicalAssertionObservation):
                clinical_assertions.append(observation)
            elif isinstance(observation, ComputationalObservation):
                computational.append(observation)
            elif isinstance(observation, FunctionalObservation):
                functional.append(observation)
            elif isinstance(observation, SegregationObservation):
                segregation.append(observation)
            elif isinstance(observation, DeNovoObservation):
                de_novo.append(observation)
            elif isinstance(observation, AllelicObservation):
                allelic.append(observation)
            elif isinstance(observation, PhenotypeObservation):
                phenotype.append(observation)
            elif isinstance(observation, GeneMechanismObservation):
                gene_mechanism.append(observation)
            elif isinstance(observation, ConsequenceObservation):
                consequence.append(observation)
            elif isinstance(observation, VariantLocationObservation):
                variant_location.append(observation)
            elif isinstance(observation, CaseControlObservation):
                case_control.append(observation)
        return cls(
            evidence_items=ordered_items,
            population=tuple(population),
            clinical_assertions=tuple(clinical_assertions),
            computational=tuple(computational),
            functional=tuple(functional),
            segregation=tuple(segregation),
            de_novo=tuple(de_novo),
            allelic=tuple(allelic),
            phenotype=tuple(phenotype),
            gene_mechanism=tuple(gene_mechanism),
            consequence=tuple(consequence),
            variant_location=tuple(variant_location),
            case_control=tuple(case_control),
        )
