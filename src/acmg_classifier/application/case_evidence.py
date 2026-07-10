"""Conversion of validated case assertions into provenance-bearing evidence."""

from __future__ import annotations

from datetime import datetime

from pydantic import field_validator

from acmg_classifier.domain.evidence import (
    AllelicObservation,
    CaseControlObservation,
    DeNovoObservation,
    EvidenceContextScope,
    EvidenceDerivation,
    EvidenceItem,
    EvidenceModel,
    FunctionalObservation,
    NonEmptyText,
    ObservationKind,
    PhenotypeObservation,
    SegregationObservation,
    UserId,
    UserProvenance,
    VariantKey,
    _normalize_utc,
)
from acmg_classifier.presentation.schemas.case_evidence import (
    AllelicEvidenceInput,
    CaseControlEvidenceInput,
    CaseEvidenceInput,
    DeNovoEvidenceInput,
    FunctionalAssayEvidenceInput,
    PhenotypeEvidenceInput,
    SegregationEvidenceInput,
    validate_case_evidence_conflicts,
)


class CaseEvidenceSubmissionContext(EvidenceModel):
    """Application-owned provenance and applicability for one submission.

    This model is intentionally not populated from the public request. The caller
    authenticates the actor and obtains ``submitted_at`` from its clock before
    invoking the converter.
    """

    actor_id: UserId
    submitted_at: datetime
    confirmation_method: NonEmptyText
    variant_key: VariantKey
    context_scope: EvidenceContextScope

    @field_validator("submitted_at")
    @classmethod
    def normalize_submitted_at(cls, value: datetime) -> datetime:
        return _normalize_utc(value)


class CaseEvidenceConverter:
    """Create immutable user-derived facts without any source-evidence fields."""

    def convert(
        self,
        evidence: tuple[CaseEvidenceInput, ...],
        submission: CaseEvidenceSubmissionContext,
    ) -> tuple[EvidenceItem, ...]:
        """Convert and content-address all validated assertions deterministically."""

        validate_case_evidence_conflicts(evidence)
        items_by_id: dict[str, EvidenceItem] = {}
        for item in evidence:
            evidence_item = self._convert_one(item, submission)
            assert evidence_item.evidence_id is not None
            items_by_id[evidence_item.evidence_id] = evidence_item
        return tuple(items_by_id[item_id] for item_id in sorted(items_by_id))

    @staticmethod
    def _provenance(
        item: CaseEvidenceInput,
        submission: CaseEvidenceSubmissionContext,
    ) -> UserProvenance:
        return UserProvenance(
            kind=EvidenceDerivation.USER,
            actor_id=submission.actor_id,
            submitted_at=submission.submitted_at,
            confirmation_method=submission.confirmation_method,
            citation_ids=item.citation_ids,
        )

    def _convert_one(
        self,
        item: CaseEvidenceInput,
        submission: CaseEvidenceSubmissionContext,
    ) -> EvidenceItem:
        observation, kind = self._observation(item)
        return EvidenceItem.model_validate(
            {
                "variant_key": submission.variant_key,
                "kind": kind,
                "observation": observation,
                "context_scope": submission.context_scope,
                "provenance": self._provenance(item, submission),
                "derivation": EvidenceDerivation.USER,
            }
        )

    @staticmethod
    def _observation(
        item: CaseEvidenceInput,
    ) -> tuple[
        PhenotypeObservation
        | SegregationObservation
        | DeNovoObservation
        | AllelicObservation
        | FunctionalObservation
        | CaseControlObservation,
        ObservationKind,
    ]:
        if isinstance(item, PhenotypeEvidenceInput):
            return (
                PhenotypeObservation(
                    kind=ObservationKind.PHENOTYPE,
                    term_id=item.term_id,
                    state=item.state,
                    specificity=item.specificity,
                ),
                ObservationKind.PHENOTYPE,
            )
        if isinstance(item, SegregationEvidenceInput):
            return (
                SegregationObservation(
                    kind=ObservationKind.SEGREGATION,
                    family_count=item.family_count,
                    informative_meioses=item.informative_meioses,
                    lod_score=item.lod_score,
                    co_segregations=item.co_segregations,
                    non_segregations=item.non_segregations,
                    phenotype_defined=item.phenotype_defined,
                ),
                ObservationKind.SEGREGATION,
            )
        if isinstance(item, DeNovoEvidenceInput):
            return (
                DeNovoObservation(
                    kind=ObservationKind.DE_NOVO,
                    confirmation=item.confirmation,
                    occurrence_count=item.occurrence_count,
                    paternity_confirmed=item.paternity_confirmed,
                    maternity_confirmed=item.maternity_confirmed,
                    phenotype_consistent=item.phenotype_consistent,
                ),
                ObservationKind.DE_NOVO,
            )
        if isinstance(item, AllelicEvidenceInput):
            return (
                AllelicObservation(
                    kind=ObservationKind.ALLELIC,
                    other_variant_key=item.other_variant_key,
                    phase=item.phase,
                    occurrence_count=item.occurrence_count,
                    observed_in_affected=item.observed_in_affected,
                ),
                ObservationKind.ALLELIC,
            )
        if isinstance(item, FunctionalAssayEvidenceInput):
            return (
                FunctionalObservation(
                    kind=ObservationKind.FUNCTIONAL,
                    assay_id=item.assay_id,
                    assay_type=item.assay_type,
                    result=item.result,
                    validation_status=item.validation_status,
                    effect_size=item.effect_size,
                    confidence_interval_lower=item.confidence_interval_lower,
                    confidence_interval_upper=item.confidence_interval_upper,
                    calibration_reference=item.calibration_reference,
                ),
                ObservationKind.FUNCTIONAL,
            )
        if isinstance(item, CaseControlEvidenceInput):
            return (
                CaseControlObservation(
                    kind=ObservationKind.CASE_CONTROL,
                    study_id=item.study_id,
                    case_count=item.case_count,
                    control_count=item.control_count,
                    case_allele_count=item.case_allele_count,
                    control_allele_count=item.control_allele_count,
                    odds_ratio=item.odds_ratio,
                    p_value=item.p_value,
                ),
                ObservationKind.CASE_CONTROL,
            )
        raise TypeError(f"unsupported case evidence input: {type(item).__name__}")
