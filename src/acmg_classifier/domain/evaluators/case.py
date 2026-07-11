"""Pure, ruleset-configured evaluators for structured case evidence."""

from __future__ import annotations

import math
from collections.abc import Iterable, Mapping
from dataclasses import dataclass

from acmg_classifier.domain.criteria import CriterionAssessment, CriterionComparison
from acmg_classifier.domain.enums import CriterionStatus
from acmg_classifier.domain.errors import JsonValue
from acmg_classifier.domain.evidence import (
    FactSet,
    Observation,
    SegregationObservation,
)
from acmg_classifier.domain.models import InterpretationContext
from acmg_classifier.domain.rules import (
    CriterionCode,
    CriterionSpecification,
    CriterionStrength,
    RulesetSpecification,
)

_SUPPORTED_CODES = frozenset(
    (
        CriterionCode.PS2,
        CriterionCode.PS4,
        CriterionCode.PM3,
        CriterionCode.PM6,
        CriterionCode.PP1,
        CriterionCode.PP4,
        CriterionCode.BS4,
        CriterionCode.BP2,
        CriterionCode.BP5,
    )
)


@dataclass(frozen=True, slots=True)
class CaseCriterionEvaluator:
    """Evaluate case criteria only from structured provenance-bearing observations."""

    code: CriterionCode
    evaluator_id: str = "case"
    evaluator_version: str = "1.0.0"

    def __post_init__(self) -> None:
        if self.code not in _SUPPORTED_CODES:
            raise ValueError("unsupported case criterion code")

    def evaluate(
        self,
        facts: FactSet,
        context: InterpretationContext,
        specification: CriterionSpecification,
        ruleset: RulesetSpecification,
    ) -> CriterionAssessment:
        """Evaluate one configured case criterion without making clinical inferences."""
        del context
        if specification.code is not self.code:
            raise ValueError("case evaluator specification code does not match")
        if self.code in (CriterionCode.PS2, CriterionCode.PM6):
            return self._de_novo(facts, specification, ruleset)
        if self.code in (CriterionCode.PM3, CriterionCode.BP2):
            return self._allelic(facts, specification, ruleset)
        if self.code in (CriterionCode.PP1, CriterionCode.BS4):
            return self._segregation(facts, specification, ruleset)
        if self.code is CriterionCode.PP4:
            return self._phenotype(facts, specification, ruleset)
        if self.code is CriterionCode.PS4:
            return self._case_control(facts, specification, ruleset)
        return self._not_evaluable(
            specification,
            ruleset,
            "structured alternative molecular cause evidence is not available",
        )

    def _de_novo(
        self,
        facts: FactSet,
        specification: CriterionSpecification,
        ruleset: RulesetSpecification,
    ) -> CriterionAssessment:
        confirmation, confirmation_error = _text(
            specification.parameters, "required_confirmation"
        )
        minimum_count, count_error = _integer(
            specification.parameters, "minimum_occurrence_count", minimum=1
        )
        parentage, parentage_error = _bool(
            specification.parameters, "require_parentage_confirmation"
        )
        phenotype, phenotype_error = _bool(
            specification.parameters, "require_consistent_phenotype"
        )
        if any(
            error is not None
            for error in (
                confirmation_error,
                count_error,
                parentage_error,
                phenotype_error,
            )
        ) or confirmation not in {"confirmed", "assumed"}:
            return self._invalid_configuration(specification, ruleset)
        assert confirmation is not None
        assert minimum_count is not None
        assert parentage is not None
        assert phenotype is not None
        if not facts.de_novo:
            return self._not_evaluable(
                specification, ruleset, "missing structured de novo evidence"
            )
        candidates = tuple(
            observation
            for observation in facts.de_novo
            if observation.confirmation == confirmation
        )
        if not candidates:
            return self._not_applied(
                specification, ruleset, "required de novo confirmation is absent"
            )
        if parentage and any(
            observation.paternity_confirmed is None
            or observation.maternity_confirmed is None
            for observation in candidates
        ):
            return self._not_evaluable(
                specification, ruleset, "missing parentage confirmation fields"
            )
        if phenotype and any(
            observation.phenotype_consistent is None for observation in candidates
        ):
            return self._not_evaluable(
                specification, ruleset, "missing phenotype-consistency fields"
            )
        matching = tuple(
            observation
            for observation in candidates
            if (
                (not parentage)
                or (
                    observation.paternity_confirmed is True
                    and observation.maternity_confirmed is True
                )
            )
            and ((not phenotype) or observation.phenotype_consistent is True)
            and observation.occurrence_count is not None
            and observation.occurrence_count >= minimum_count
        )
        if not matching:
            return self._not_applied(
                specification,
                ruleset,
                "de novo evidence does not meet the ruleset confirmation requirements",
            )
        return self._applied(facts, specification, ruleset, matching)

    def _allelic(
        self,
        facts: FactSet,
        specification: CriterionSpecification,
        ruleset: RulesetSpecification,
    ) -> CriterionAssessment:
        required_phase, phase_error = _text(specification.parameters, "required_phase")
        minimum_count, count_error = _integer(
            specification.parameters, "minimum_occurrence_count", minimum=1
        )
        require_affected, affected_error = _optional_bool(
            specification.parameters, "require_affected"
        )
        if (
            phase_error
            or count_error
            or affected_error
            or required_phase not in {"cis", "trans"}
        ):
            return self._invalid_configuration(specification, ruleset)
        assert required_phase is not None
        assert minimum_count is not None
        if not facts.allelic:
            return self._not_evaluable(
                specification, ruleset, "missing structured allelic evidence"
            )
        candidates = tuple(
            observation
            for observation in facts.allelic
            if observation.phase == required_phase
        )
        if not candidates:
            return self._not_applied(
                specification, ruleset, "required allelic phase is absent"
            )
        if any(observation.occurrence_count is None for observation in candidates):
            return self._not_evaluable(
                specification, ruleset, "missing allelic occurrence count"
            )
        if require_affected and any(
            observation.observed_in_affected is None for observation in candidates
        ):
            return self._not_evaluable(
                specification, ruleset, "missing affected-status evidence"
            )
        matching = tuple(
            observation
            for observation in candidates
            if observation.occurrence_count is not None
            and observation.occurrence_count >= minimum_count
            and (not require_affected or observation.observed_in_affected is True)
        )
        if not matching:
            return self._not_applied(
                specification,
                ruleset,
                "allelic evidence does not meet the ruleset requirements",
            )
        return self._applied(facts, specification, ruleset, matching)

    def _segregation(
        self,
        facts: FactSet,
        specification: CriterionSpecification,
        ruleset: RulesetSpecification,
    ) -> CriterionAssessment:
        if self.code is CriterionCode.PP1:
            key = "minimum_lod_score"
            values = tuple(observation.lod_score for observation in facts.segregation)
        else:
            key = "minimum_non_segregations"
            values = tuple(
                observation.non_segregations for observation in facts.segregation
            )
        minimum, error = _number(specification.parameters, key, minimum=0.0)
        require_phenotype, phenotype_error = _bool(
            specification.parameters, "require_defined_phenotype"
        )
        if error or phenotype_error:
            return self._invalid_configuration(specification, ruleset)
        assert minimum is not None
        assert require_phenotype is not None
        if not facts.segregation:
            return self._not_evaluable(
                specification, ruleset, "missing structured segregation evidence"
            )
        if any(value is None for value in values):
            return self._not_evaluable(
                specification, ruleset, f"missing {key} evidence"
            )
        if require_phenotype and any(
            observation.phenotype_defined is None for observation in facts.segregation
        ):
            return self._not_evaluable(
                specification, ruleset, "missing phenotype-definition evidence"
            )
        matching: list[SegregationObservation] = []
        for observation, value in zip(facts.segregation, values, strict=True):
            assert value is not None
            if value >= minimum and (
                not require_phenotype or observation.phenotype_defined is True
            ):
                matching.append(observation)
        if not matching:
            return self._not_applied(
                specification,
                ruleset,
                "segregation evidence does not meet the ruleset threshold",
            )
        return self._applied(facts, specification, ruleset, tuple(matching))

    def _phenotype(
        self,
        facts: FactSet,
        specification: CriterionSpecification,
        ruleset: RulesetSpecification,
    ) -> CriterionAssessment:
        specificity, specificity_error = _text(
            specification.parameters, "required_specificity"
        )
        minimum_terms, count_error = _integer(
            specification.parameters, "minimum_present_terms", minimum=1
        )
        if specificity_error or count_error:
            return self._invalid_configuration(specification, ruleset)
        assert specificity is not None
        assert minimum_terms is not None
        if not facts.phenotype:
            return self._not_evaluable(
                specification, ruleset, "missing structured phenotype evidence"
            )
        if all(observation.specificity is None for observation in facts.phenotype):
            return self._not_evaluable(
                specification, ruleset, "missing phenotype specificity evidence"
            )
        matching = tuple(
            observation
            for observation in facts.phenotype
            if observation.state == "present" and observation.specificity == specificity
        )
        distinct_terms = {observation.term_id for observation in matching}
        if len(distinct_terms) < minimum_terms:
            return self._not_applied(
                specification,
                ruleset,
                "phenotype specificity count does not meet the ruleset minimum",
            )
        return self._applied(facts, specification, ruleset, matching)

    def _case_control(
        self,
        facts: FactSet,
        specification: CriterionSpecification,
        ruleset: RulesetSpecification,
    ) -> CriterionAssessment:
        minimum_odds_ratio, odds_error = _number(
            specification.parameters, "minimum_odds_ratio", minimum=0.0
        )
        maximum_p_value, p_value_error = _number(
            specification.parameters,
            "maximum_p_value",
            minimum=0.0,
            maximum=1.0,
        )
        minimum_cases, cases_error = _integer(
            specification.parameters, "minimum_case_count", minimum=1
        )
        minimum_controls, controls_error = _integer(
            specification.parameters, "minimum_control_count", minimum=1
        )
        if any(
            error is not None
            for error in (odds_error, p_value_error, cases_error, controls_error)
        ):
            return self._invalid_configuration(specification, ruleset)
        assert minimum_odds_ratio is not None
        assert maximum_p_value is not None
        assert minimum_cases is not None
        assert minimum_controls is not None
        if not facts.case_control:
            return self._not_evaluable(
                specification, ruleset, "missing structured case-control evidence"
            )
        required_values_missing = any(
            observation.odds_ratio is None
            or observation.p_value is None
            or observation.case_count is None
            or observation.control_count is None
            for observation in facts.case_control
        )
        if required_values_missing:
            return self._not_evaluable(
                specification,
                ruleset,
                "case-control effect or count fields are missing",
            )
        matching = tuple(
            observation
            for observation in facts.case_control
            if observation.odds_ratio is not None
            and observation.p_value is not None
            and observation.case_count is not None
            and observation.control_count is not None
            and observation.odds_ratio >= minimum_odds_ratio
            and observation.p_value <= maximum_p_value
            and observation.case_count >= minimum_cases
            and observation.control_count >= minimum_controls
        )
        if not matching:
            return self._not_applied(
                specification,
                ruleset,
                "case-control evidence does not meet the ruleset thresholds",
            )
        return self._applied(facts, specification, ruleset, matching)

    def _applied(
        self,
        facts: FactSet,
        specification: CriterionSpecification,
        ruleset: RulesetSpecification,
        observations: Iterable[Observation],
    ) -> CriterionAssessment:
        evidence_ids = _evidence_ids(facts, observations)
        if not evidence_ids:
            return self._not_evaluable(
                specification,
                ruleset,
                "decisive observations lack evidence identifiers",
            )
        strength, error = _applied_strength(specification)
        if error is not None or strength is None:
            return self._invalid_configuration(specification, ruleset)
        return self._assessment(
            status=CriterionStatus.APPLIED,
            specification=specification,
            ruleset=ruleset,
            evidence_ids=evidence_ids,
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

    def _invalid_configuration(
        self,
        specification: CriterionSpecification,
        ruleset: RulesetSpecification,
    ) -> CriterionAssessment:
        return self._not_evaluable(
            specification, ruleset, "ruleset case-evidence configuration is invalid"
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


def _text(
    parameters: Mapping[str, JsonValue], key: str
) -> tuple[str | None, str | None]:
    value = parameters.get(key)
    if not isinstance(value, str) or not value.strip():
        return None, f"{key} must be non-empty text"
    return value.strip(), None


def _bool(
    parameters: Mapping[str, JsonValue], key: str
) -> tuple[bool | None, str | None]:
    value = parameters.get(key)
    if not isinstance(value, bool):
        return None, f"{key} must be boolean"
    return value, None


def _optional_bool(
    parameters: Mapping[str, JsonValue], key: str
) -> tuple[bool, str | None]:
    if key not in parameters:
        return False, None
    value, error = _bool(parameters, key)
    return False if value is None else value, error


def _integer(
    parameters: Mapping[str, JsonValue], key: str, *, minimum: int
) -> tuple[int | None, str | None]:
    value = parameters.get(key)
    if not isinstance(value, int) or isinstance(value, bool) or value < minimum:
        return None, f"{key} must be an integer >= {minimum}"
    return value, None


def _number(
    parameters: Mapping[str, JsonValue],
    key: str,
    *,
    minimum: float,
    maximum: float | None = None,
) -> tuple[float | None, str | None]:
    value = parameters.get(key)
    if (
        not isinstance(value, (int, float))
        or isinstance(value, bool)
        or not math.isfinite(value)
        or value < minimum
        or (maximum is not None and value > maximum)
    ):
        range_description = (
            f"between {minimum} and {maximum}"
            if maximum is not None
            else f">= {minimum}"
        )
        return None, f"{key} must be a finite number {range_description}"
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
