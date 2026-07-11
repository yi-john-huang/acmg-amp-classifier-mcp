from __future__ import annotations

from datetime import UTC, datetime

from acmg_classifier.domain.criteria import CriterionAssessment
from acmg_classifier.domain.enums import CriterionStatus, GenomeBuild
from acmg_classifier.domain.errors import JsonValue
from acmg_classifier.domain.evaluators.functional_computational import (
    FunctionalComputationalCriterionEvaluator,
)
from acmg_classifier.domain.evidence import (
    ComputationalObservation,
    EvidenceContextScope,
    EvidenceDerivation,
    EvidenceItem,
    FactSet,
    FunctionalObservation,
    Observation,
    ObservationKind,
    SourceProvenance,
)
from acmg_classifier.domain.models import InterpretationContext
from acmg_classifier.domain.rules import (
    CriterionCode,
    CriterionSpecification,
    CriterionStrength,
    RulesetScope,
    RulesetSpecification,
    RulesetState,
)


def _item(
    observation: Observation,
    kind: ObservationKind,
    identifier: str,
) -> EvidenceItem:
    return EvidenceItem(
        variant_key="ga4gh:VA.functional-example",
        kind=kind,
        observation=observation,
        context_scope=EvidenceContextScope(genome_build=GenomeBuild.GRCH38),
        provenance=SourceProvenance(
            kind=EvidenceDerivation.SOURCE,
            source_id="functional_source",
            source_record_id=identifier,
            source_version="1.0",
            retrieved_at=datetime(2026, 7, 11, tzinfo=UTC),
            normalized_query_key="ga4gh:VA.functional-example",
        ),
        raw_snapshot_ref="raw_" + "d" * 64,
        derivation=EvidenceDerivation.SOURCE,
    )


def _functional(
    assay_id: str,
    *,
    result: str,
    validation_status: str = "validated",
    calibration_reference: str | None = "ClinGen-SVI-v1",
    record_id: str | None = None,
) -> EvidenceItem:
    return _item(
        FunctionalObservation(
            kind=ObservationKind.FUNCTIONAL,
            assay_id=assay_id,
            assay_type="protein function",
            result=result,
            validation_status=validation_status,
            calibration_reference=calibration_reference,
        ),
        ObservationKind.FUNCTIONAL,
        record_id or f"functional-{assay_id}-{result}-{validation_status}",
    )


def _computational(
    predictor: str,
    prediction: str,
    score: float | None,
) -> EvidenceItem:
    return _item(
        ComputationalObservation(
            kind=ObservationKind.COMPUTATIONAL,
            predictor=predictor,
            prediction=prediction,
            score=score,
            threshold=0.5,
            dataset_version="1.0",
            transcript="NM_000492.4",
        ),
        ObservationKind.COMPUTATIONAL,
        f"computational-{predictor}-{prediction}-{score}",
    )


def _ruleset(
    code: CriterionCode,
    parameters: dict[str, JsonValue],
) -> tuple[RulesetSpecification, CriterionSpecification]:
    criteria: list[CriterionSpecification] = []
    selected: CriterionSpecification | None = None
    for criterion_code in CriterionCode:
        enabled = criterion_code is code
        criterion = CriterionSpecification(
            code=criterion_code,
            enabled=enabled,
            base_strength=CriterionStrength.STRONG if enabled else None,
            allowed_strengths=(CriterionStrength.STRONG,) if enabled else (),
            evaluator_id="functional_computational"
            if enabled
            else criterion_code.value,
            evaluator_version_constraint=">=1.0.0,<2.0.0",
            rationale_template=f"{criterion_code.value} rationale",
            parameters=parameters if enabled else {},
        )
        criteria.append(criterion)
        if enabled:
            selected = criterion
    assert selected is not None
    ruleset = RulesetSpecification(
        ruleset_id="functional-test",
        version="1.0.0",
        state=RulesetState.APPROVED,
        publication_reference="PMID:25741868",
        scope=RulesetScope(),
        criteria=tuple(criteria),
        combination_algorithm_id="acmg_2015",
        combination_algorithm_version="1.0.0",
    )
    return ruleset, selected


def _evaluate(
    code: CriterionCode,
    parameters: dict[str, JsonValue],
    *items: EvidenceItem,
) -> CriterionAssessment:
    ruleset, specification = _ruleset(code, parameters)
    return FunctionalComputationalCriterionEvaluator(code).evaluate(
        FactSet.from_evidence(items), InterpretationContext(), specification, ruleset
    )


def test_ps3_requires_validated_calibrated_abnormal_assays() -> None:
    parameters: dict[str, JsonValue] = {
        "required_result": "abnormal",
        "required_validation_status": "validated",
        "required_calibration_reference": "ClinGen-SVI-v1",
        "minimum_assay_count": 1,
    }
    validated = _functional("assay-1", result="abnormal")

    applied = _evaluate(CriterionCode.PS3, parameters, validated)
    unvalidated = _evaluate(
        CriterionCode.PS3,
        parameters,
        _functional("assay-1", result="abnormal", validation_status="unvalidated"),
    )

    assert applied.status is CriterionStatus.APPLIED
    assert applied.evidence_ids == (validated.evidence_id,)
    assert unvalidated.status is CriterionStatus.NOT_EVALUABLE


def test_bs3_uses_ruleset_direction_and_does_not_count_duplicate_assay_ids() -> None:
    parameters: dict[str, JsonValue] = {
        "required_result": "normal",
        "required_validation_status": "validated",
        "required_calibration_reference": "ClinGen-SVI-v1",
        "minimum_assay_count": 2,
    }
    first = _functional("assay-1", result="normal")
    duplicate_assay = _functional(
        "assay-1",
        result="normal",
        record_id="functional-assay-1-second-source",
    )

    assessment = _evaluate(CriterionCode.BS3, parameters, first, duplicate_assay)

    assert assessment.status is CriterionStatus.NOT_APPLIED


def test_pp3_requires_concordant_configured_predictors_and_scores() -> None:
    parameters: dict[str, JsonValue] = {
        "required_prediction": "pathogenic_supporting",
        "opposing_prediction": "benign_supporting",
        "minimum_concordant_predictors": 2,
        "minimum_score": 0.7,
    }
    revel = _computational("REVEL", "pathogenic_supporting", 0.8)
    cadd = _computational("CADD", "pathogenic_supporting", 0.9)

    applied = _evaluate(CriterionCode.PP3, parameters, revel, cadd)
    missing_score = _evaluate(
        CriterionCode.PP3,
        parameters,
        _computational("REVEL", "pathogenic_supporting", None),
        cadd,
    )

    assert applied.status is CriterionStatus.APPLIED
    assert set(applied.evidence_ids) == {revel.evidence_id, cadd.evidence_id}
    assert missing_score.status is CriterionStatus.NOT_EVALUABLE


def test_bp4_marks_opposing_high_quality_predictors_as_conflicting() -> None:
    parameters: dict[str, JsonValue] = {
        "required_prediction": "benign_supporting",
        "opposing_prediction": "pathogenic_supporting",
        "minimum_concordant_predictors": 1,
        "minimum_score": 0.7,
    }

    assessment = _evaluate(
        CriterionCode.BP4,
        parameters,
        _computational("REVEL", "benign_supporting", 0.8),
        _computational("CADD", "pathogenic_supporting", 0.9),
    )

    assert assessment.status is CriterionStatus.CONFLICTING


def test_missing_calibration_or_predictor_configuration_is_not_evaluable() -> None:
    functional = _evaluate(
        CriterionCode.PS3,
        {
            "required_result": "abnormal",
            "required_validation_status": "validated",
            "minimum_assay_count": 1,
        },
        _functional("assay-1", result="abnormal"),
    )
    computational = _evaluate(
        CriterionCode.PP3,
        {
            "required_prediction": "pathogenic_supporting",
            "minimum_concordant_predictors": 1,
        },
        _computational("REVEL", "pathogenic_supporting", 0.8),
    )

    assert functional.status is CriterionStatus.NOT_EVALUABLE
    assert computational.status is CriterionStatus.NOT_EVALUABLE
