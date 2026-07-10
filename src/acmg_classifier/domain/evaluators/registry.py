"""Complete built-in registry for release-one ACMG/AMP evaluators."""

from __future__ import annotations

from acmg_classifier.domain.criteria import CriterionEvaluator, EvaluatorRegistry
from acmg_classifier.domain.evaluators.case import CaseCriterionEvaluator
from acmg_classifier.domain.evaluators.consequence import ConsequenceCriterionEvaluator
from acmg_classifier.domain.evaluators.functional_computational import (
    FunctionalComputationalCriterionEvaluator,
)
from acmg_classifier.domain.evaluators.population import PopulationCriterionEvaluator
from acmg_classifier.domain.evaluators.remaining import RemainingCriterionEvaluator
from acmg_classifier.domain.rules import CriterionCode

_POPULATION_CODES = (
    CriterionCode.BA1,
    CriterionCode.BS1,
    CriterionCode.BS2,
    CriterionCode.PM2,
)
_CONSEQUENCE_CODES = (
    CriterionCode.PVS1,
    CriterionCode.PS1,
    CriterionCode.PM1,
    CriterionCode.PM4,
    CriterionCode.PM5,
    CriterionCode.BP1,
    CriterionCode.BP3,
    CriterionCode.BP7,
)
_FUNCTIONAL_COMPUTATIONAL_CODES = (
    CriterionCode.PS3,
    CriterionCode.BS3,
    CriterionCode.PP3,
    CriterionCode.BP4,
)
_CASE_CODES = (
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
_REMAINING_CODES = (CriterionCode.PP2, CriterionCode.PP5, CriterionCode.BP6)


def build_default_evaluator_registry() -> EvaluatorRegistry:
    """Return one explicit evaluator registration for every 2015 criterion code."""
    evaluators: list[CriterionEvaluator] = []
    evaluators.extend(PopulationCriterionEvaluator(code) for code in _POPULATION_CODES)
    evaluators.extend(
        ConsequenceCriterionEvaluator(code) for code in _CONSEQUENCE_CODES
    )
    evaluators.extend(
        FunctionalComputationalCriterionEvaluator(code)
        for code in _FUNCTIONAL_COMPUTATIONAL_CODES
    )
    evaluators.extend(CaseCriterionEvaluator(code) for code in _CASE_CODES)
    evaluators.extend(RemainingCriterionEvaluator(code) for code in _REMAINING_CODES)
    registry = EvaluatorRegistry(tuple(evaluators))
    registry.assert_complete()
    return registry
