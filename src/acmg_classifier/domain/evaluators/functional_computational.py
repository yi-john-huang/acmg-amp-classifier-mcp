"""Pure, ruleset-configured functional and computational criterion evaluators."""

from __future__ import annotations

import math
from collections.abc import Iterable, Mapping
from dataclasses import dataclass

from acmg_classifier.domain.criteria import CriterionAssessment, CriterionComparison
from acmg_classifier.domain.enums import CriterionStatus
from acmg_classifier.domain.errors import JsonValue
from acmg_classifier.domain.evidence import (
    ComputationalObservation,
    FactSet,
    FunctionalObservation,
    Observation,
)
from acmg_classifier.domain.models import InterpretationContext
from acmg_classifier.domain.rules import (
    CriterionCode,
    CriterionSpecification,
    CriterionStrength,
    RulesetSpecification,
)

_SUPPORTED_CODES = frozenset(
    (CriterionCode.PS3, CriterionCode.BS3, CriterionCode.PP3, CriterionCode.BP4)
)


@dataclass(frozen=True, slots=True)
class _FunctionalConfiguration:
    required_result: str
    required_validation_status: str
    required_calibration_reference: str
    minimum_assay_count: int
    applied_strength: CriterionStrength


@dataclass(frozen=True, slots=True)
class _ComputationalConfiguration:
    required_prediction: str
    opposing_prediction: str
    minimum_concordant_predictors: int
    minimum_score: float
    applied_strength: CriterionStrength


@dataclass(frozen=True, slots=True)
class FunctionalComputationalCriterionEvaluator:
    """Evaluate PS3, BS3, PP3, or BP4 only from configured structured evidence."""

    code: CriterionCode
    evaluator_id: str = "functional_computational"
    evaluator_version: str = "1.0.0"

    def __post_init__(self) -> None:
        if self.code not in _SUPPORTED_CODES:
            raise ValueError("unsupported functional or computational criterion code")

    def evaluate(
        self,
        facts: FactSet,
        context: InterpretationContext,
        specification: CriterionSpecification,
        ruleset: RulesetSpecification,
    ) -> CriterionAssessment:
        """Evaluate one criteria definition without treating raw output as evidence."""
        del context
        if specification.code is not self.code:
            raise ValueError("functional evaluator specification code does not match")
        if self.code in (CriterionCode.PS3, CriterionCode.BS3):
            return self._evaluate_functional(facts, specification, ruleset)
        return self._evaluate_computational(facts, specification, ruleset)

    def _evaluate_functional(
        self,
        facts: FactSet,
        specification: CriterionSpecification,
        ruleset: RulesetSpecification,
    ) -> CriterionAssessment:
        configuration, error = _functional_configuration(specification)
        if error is not None:
            return self._not_evaluable(specification, ruleset, error)
        assert configuration is not None
        if not facts.functional:
            return self._not_evaluable(
                specification, ruleset, "missing structured functional assay evidence"
            )
        validated = tuple(
            observation
            for observation in facts.functional
            if observation.validation_status == configuration.required_validation_status
        )
        if not validated:
            return self._not_evaluable(
                specification,
                ruleset,
                "functional assay validation does not meet the ruleset requirement",
            )
        calibrated = tuple(
            observation
            for observation in validated
            if observation.calibration_reference
            == configuration.required_calibration_reference
        )
        if not calibrated:
            return self._not_evaluable(
                specification,
                ruleset,
                "functional assay calibration does not meet the ruleset requirement",
            )
        matching = _one_per_assay(
            observation
            for observation in calibrated
            if observation.result == configuration.required_result
        )
        if len(matching) < configuration.minimum_assay_count:
            return self._not_applied(
                specification,
                ruleset,
                "independent calibrated assays do not meet the ruleset count",
            )
        return self._applied(
            facts,
            specification,
            ruleset,
            matching,
            configuration.applied_strength,
        )

    def _evaluate_computational(
        self,
        facts: FactSet,
        specification: CriterionSpecification,
        ruleset: RulesetSpecification,
    ) -> CriterionAssessment:
        configuration, error = _computational_configuration(specification)
        if error is not None:
            return self._not_evaluable(specification, ruleset, error)
        assert configuration is not None
        if not facts.computational:
            return self._not_evaluable(
                specification, ruleset, "missing structured computational evidence"
            )

        required: dict[str, ComputationalObservation] = {}
        opposing: dict[str, ComputationalObservation] = {}
        incomplete = False
        for observation in facts.computational:
            if observation.prediction not in {
                configuration.required_prediction,
                configuration.opposing_prediction,
            }:
                continue
            if observation.score is None:
                incomplete = True
                continue
            if observation.score < configuration.minimum_score:
                continue
            target = (
                required
                if observation.prediction == configuration.required_prediction
                else opposing
            )
            previous = target.get(observation.predictor)
            if previous is None or _computational_key(observation) > _computational_key(
                previous
            ):
                target[observation.predictor] = observation
        if incomplete:
            return self._not_evaluable(
                specification,
                ruleset,
                "configured predictor evidence is missing a calibrated score",
            )
        if required and opposing:
            return self._assessment(
                status=CriterionStatus.CONFLICTING,
                specification=specification,
                ruleset=ruleset,
                evidence_ids=_evidence_ids(
                    facts, (*required.values(), *opposing.values())
                ),
                limitations=("high-quality predictors support opposing directions",),
            )
        if len(required) < configuration.minimum_concordant_predictors:
            return self._not_applied(
                specification,
                ruleset,
                "concordant predictor count does not meet the ruleset minimum",
            )
        comparison = CriterionComparison(
            input_name="concordant_predictor_count",
            operator=">=",
            observed=len(required),
            expected=configuration.minimum_concordant_predictors,
            matched=True,
        )
        return self._applied(
            facts,
            specification,
            ruleset,
            tuple(required.values()),
            configuration.applied_strength,
            comparisons=(comparison,),
        )

    def _applied(
        self,
        facts: FactSet,
        specification: CriterionSpecification,
        ruleset: RulesetSpecification,
        observations: Iterable[Observation],
        strength: CriterionStrength,
        *,
        comparisons: tuple[CriterionComparison, ...] = (),
    ) -> CriterionAssessment:
        evidence_ids = _evidence_ids(facts, observations)
        if not evidence_ids:
            return self._not_evaluable(
                specification,
                ruleset,
                "decisive observations lack evidence identifiers",
            )
        return self._assessment(
            status=CriterionStatus.APPLIED,
            specification=specification,
            ruleset=ruleset,
            evidence_ids=evidence_ids,
            comparisons=comparisons,
            applied_strength=strength,
        )

    def _not_applied(
        self,
        specification: CriterionSpecification,
        ruleset: RulesetSpecification,
        limitation: str,
    ) -> CriterionAssessment:
        return self._assessment(
            status=CriterionStatus.NOT_APPLIED,
            specification=specification,
            ruleset=ruleset,
            limitations=(limitation,),
        )

    def _not_evaluable(
        self,
        specification: CriterionSpecification,
        ruleset: RulesetSpecification,
        limitation: str,
    ) -> CriterionAssessment:
        return self._assessment(
            status=CriterionStatus.NOT_EVALUABLE,
            specification=specification,
            ruleset=ruleset,
            limitations=(limitation,),
        )

    def _assessment(
        self,
        *,
        status: CriterionStatus,
        specification: CriterionSpecification,
        ruleset: RulesetSpecification,
        evidence_ids: tuple[str, ...] = (),
        comparisons: tuple[CriterionComparison, ...] = (),
        rationale_values: Mapping[str, JsonValue] | None = None,
        limitations: tuple[str, ...] = (),
        applied_strength: CriterionStrength | None = None,
    ) -> CriterionAssessment:
        return CriterionAssessment(
            code=specification.code,
            status=status,
            original_strength=specification.base_strength,
            applied_strength=applied_strength,
            evidence_ids=evidence_ids,
            comparisons=comparisons,
            rationale_template=specification.rationale_template,
            rationale_values={} if rationale_values is None else rationale_values,
            limitations=limitations,
            ruleset_id=ruleset.ruleset_id,
            ruleset_version=ruleset.version,
            evaluator_id=self.evaluator_id,
            evaluator_version=self.evaluator_version,
        )


def _functional_configuration(
    specification: CriterionSpecification,
) -> tuple[_FunctionalConfiguration | None, str | None]:
    result, result_error = _text(specification.parameters, "required_result")
    validation, validation_error = _text(
        specification.parameters, "required_validation_status"
    )
    calibration, calibration_error = _text(
        specification.parameters, "required_calibration_reference"
    )
    assay_count, count_error = _integer(
        specification.parameters, "minimum_assay_count", minimum=1
    )
    strength, strength_error = _applied_strength(specification)
    if any(
        error is not None
        for error in (
            result_error,
            validation_error,
            calibration_error,
            count_error,
            strength_error,
        )
    ):
        return None, "ruleset functional configuration is invalid"
    assert result is not None
    assert validation is not None
    assert calibration is not None
    assert assay_count is not None
    assert strength is not None
    return (
        _FunctionalConfiguration(
            required_result=result,
            required_validation_status=validation,
            required_calibration_reference=calibration,
            minimum_assay_count=assay_count,
            applied_strength=strength,
        ),
        None,
    )


def _computational_configuration(
    specification: CriterionSpecification,
) -> tuple[_ComputationalConfiguration | None, str | None]:
    required, required_error = _text(specification.parameters, "required_prediction")
    opposing, opposing_error = _text(specification.parameters, "opposing_prediction")
    predictor_count, count_error = _integer(
        specification.parameters, "minimum_concordant_predictors", minimum=1
    )
    minimum_score, score_error = _number(
        specification.parameters, "minimum_score", minimum=0.0
    )
    strength, strength_error = _applied_strength(specification)
    if any(
        error is not None
        for error in (
            required_error,
            opposing_error,
            count_error,
            score_error,
            strength_error,
        )
    ):
        return None, "ruleset computational configuration is invalid"
    assert required is not None
    assert opposing is not None
    assert predictor_count is not None
    assert minimum_score is not None
    assert strength is not None
    if required == opposing:
        return None, "ruleset computational directions must differ"
    return (
        _ComputationalConfiguration(
            required_prediction=required,
            opposing_prediction=opposing,
            minimum_concordant_predictors=predictor_count,
            minimum_score=minimum_score,
            applied_strength=strength,
        ),
        None,
    )


def _one_per_assay(
    observations: Iterable[FunctionalObservation],
) -> tuple[FunctionalObservation, ...]:
    by_assay: dict[str, FunctionalObservation] = {}
    for observation in observations:
        previous = by_assay.get(observation.assay_id)
        if previous is None or _functional_key(observation) > _functional_key(previous):
            by_assay[observation.assay_id] = observation
    return tuple(by_assay[assay_id] for assay_id in sorted(by_assay))


def _functional_key(observation: FunctionalObservation) -> tuple[str, str, str]:
    return (
        observation.calibration_reference or "",
        observation.validation_status,
        observation.result,
    )


def _computational_key(observation: ComputationalObservation) -> tuple[float, str]:
    return (observation.score or 0.0, observation.dataset_version)


def _text(
    parameters: Mapping[str, JsonValue], key: str
) -> tuple[str | None, str | None]:
    value = parameters.get(key)
    if not isinstance(value, str) or not value.strip():
        return None, f"{key} must be non-empty text"
    return value.strip(), None


def _integer(
    parameters: Mapping[str, JsonValue], key: str, *, minimum: int
) -> tuple[int | None, str | None]:
    value = parameters.get(key)
    if not isinstance(value, int) or isinstance(value, bool) or value < minimum:
        return None, f"{key} must be an integer >= {minimum}"
    return value, None


def _number(
    parameters: Mapping[str, JsonValue], key: str, *, minimum: float
) -> tuple[float | None, str | None]:
    value = parameters.get(key)
    if (
        not isinstance(value, (int, float))
        or isinstance(value, bool)
        or not math.isfinite(value)
        or value < minimum
    ):
        return None, f"{key} must be a finite number >= {minimum}"
    return float(value), None


def _applied_strength(
    specification: CriterionSpecification,
) -> tuple[CriterionStrength | None, str | None]:
    configured = specification.parameters.get("applied_strength")
    if configured is None:
        return specification.base_strength, None
    if not isinstance(configured, str):
        return None, "applied_strength must be a controlled strength"
    try:
        strength = CriterionStrength(configured)
    except ValueError:
        return None, "applied_strength must be a controlled strength"
    if strength not in specification.allowed_strengths:
        return None, "applied_strength is not allowed by this criterion"
    return strength, None


def _evidence_ids(
    facts: FactSet, observations: Iterable[Observation]
) -> tuple[str, ...]:
    return tuple(
        sorted(
            {
                evidence_id
                for observation in observations
                for evidence_id in facts.evidence_ids_for(observation)
            }
        )
    )
