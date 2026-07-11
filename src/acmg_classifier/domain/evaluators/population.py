"""Pure, ruleset-configured population criterion evaluators."""

from __future__ import annotations

import math
from collections import defaultdict
from collections.abc import Iterable, Mapping
from dataclasses import dataclass
from typing import Literal

from acmg_classifier.domain.criteria import CriterionAssessment, CriterionComparison
from acmg_classifier.domain.enums import CriterionStatus
from acmg_classifier.domain.errors import JsonValue
from acmg_classifier.domain.evidence import FactSet, PopulationObservation
from acmg_classifier.domain.models import InterpretationContext
from acmg_classifier.domain.rules import (
    CriterionCode,
    CriterionSpecification,
    CriterionStrength,
    RulesetSpecification,
)

_POPULATION_CODES = frozenset(
    (CriterionCode.BA1, CriterionCode.BS1, CriterionCode.BS2, CriterionCode.PM2)
)


@dataclass(frozen=True, slots=True)
class _PopulationConfiguration:
    minimum_allele_number: int
    minimum_coverage: float
    required_filter_status: str
    frequency_threshold: float | None
    minimum_homozygote_count: int | None
    minimum_hemizygote_count: int | None
    maximum_frequency_spread: float | None
    applied_strength: CriterionStrength


@dataclass(frozen=True, slots=True)
class _EligiblePopulation:
    observation: PopulationObservation
    allele_frequency: float
    allele_number: int
    coverage: float


@dataclass(frozen=True, slots=True)
class PopulationCriterionEvaluator:
    """Evaluate BA1, BS1, BS2, or PM2 from quality-eligible population facts."""

    code: CriterionCode
    evaluator_id: str = "population"
    evaluator_version: str = "1.0.0"

    def __post_init__(self) -> None:
        if self.code not in _POPULATION_CODES:
            raise ValueError("PopulationCriterionEvaluator supports BA1, BS1, BS2, PM2")

    def evaluate(
        self,
        facts: FactSet,
        context: InterpretationContext,
        specification: CriterionSpecification,
        ruleset: RulesetSpecification,
    ) -> CriterionAssessment:
        """Evaluate one configured population criterion without universal thresholds."""
        del context
        if specification.code is not self.code:
            raise ValueError("population evaluator specification code does not match")
        configuration, configuration_error = _configuration(specification)
        if configuration_error is not None:
            return self._assessment(
                status=CriterionStatus.NOT_EVALUABLE,
                specification=specification,
                ruleset=ruleset,
                limitations=(configuration_error,),
            )
        assert configuration is not None

        eligible = tuple(_eligible_populations(facts.population, configuration))
        if not eligible:
            return self._assessment(
                status=CriterionStatus.NOT_EVALUABLE,
                specification=specification,
                ruleset=ruleset,
                limitations=("no quality-eligible population observations",),
            )

        conflict = _frequency_spread_conflict(
            eligible, configuration.maximum_frequency_spread
        )
        if conflict:
            conflict_ids = _evidence_ids(facts, conflict)
            return self._assessment(
                status=CriterionStatus.CONFLICTING,
                specification=specification,
                ruleset=ruleset,
                evidence_ids=conflict_ids,
                limitations=("population frequency spread exceeds the ruleset limit",),
            )

        if self.code is CriterionCode.BS2:
            return self._evaluate_bs2(
                facts, eligible, configuration, specification, ruleset
            )
        return self._evaluate_frequency(
            facts, eligible, configuration, specification, ruleset
        )

    def _evaluate_frequency(
        self,
        facts: FactSet,
        eligible: tuple[_EligiblePopulation, ...],
        configuration: _PopulationConfiguration,
        specification: CriterionSpecification,
        ruleset: RulesetSpecification,
    ) -> CriterionAssessment:
        threshold = configuration.frequency_threshold
        if threshold is None:
            return self._assessment(
                status=CriterionStatus.NOT_EVALUABLE,
                specification=specification,
                ruleset=ruleset,
                limitations=("ruleset population frequency threshold is missing",),
            )
        decisive = max(
            eligible,
            key=lambda value: (
                value.allele_frequency,
                value.allele_number,
                value.observation.ancestry or "",
            ),
        )
        operator: Literal["<=", ">="]
        if self.code is CriterionCode.PM2:
            operator = "<="
            matched = decisive.allele_frequency <= threshold
        else:
            operator = ">="
            matched = decisive.allele_frequency >= threshold
        input_name = "maximum_allele_frequency"
        comparison = CriterionComparison(
            input_name=input_name,
            operator=operator,
            observed=decisive.allele_frequency,
            expected=threshold,
            matched=matched,
        )
        if not matched:
            return self._assessment(
                status=CriterionStatus.NOT_APPLIED,
                specification=specification,
                ruleset=ruleset,
                comparisons=(comparison,),
                limitations=("population threshold was not met",),
            )
        evidence_ids = _evidence_ids(facts, (decisive,))
        if not evidence_ids:
            return self._assessment(
                status=CriterionStatus.NOT_EVALUABLE,
                specification=specification,
                ruleset=ruleset,
                comparisons=(comparison,),
                limitations=(
                    "eligible population observations lack evidence identifiers",
                ),
            )
        return self._assessment(
            status=CriterionStatus.APPLIED,
            specification=specification,
            ruleset=ruleset,
            evidence_ids=evidence_ids,
            comparisons=(comparison,),
            rationale_values={
                "ancestry": decisive.observation.ancestry or "unspecified",
                "allele_frequency": decisive.allele_frequency,
                "threshold": threshold,
            },
            applied_strength=configuration.applied_strength,
        )

    def _evaluate_bs2(
        self,
        facts: FactSet,
        eligible: tuple[_EligiblePopulation, ...],
        configuration: _PopulationConfiguration,
        specification: CriterionSpecification,
        ruleset: RulesetSpecification,
    ) -> CriterionAssessment:
        homozygote_threshold = configuration.minimum_homozygote_count
        hemizygote_threshold = configuration.minimum_hemizygote_count
        if homozygote_threshold is None and hemizygote_threshold is None:
            return self._assessment(
                status=CriterionStatus.NOT_EVALUABLE,
                specification=specification,
                ruleset=ruleset,
                limitations=("ruleset BS2 count threshold is missing",),
            )

        matched_candidates: list[tuple[_EligiblePopulation, CriterionComparison]] = []
        for candidate in eligible:
            homozygote_count = candidate.observation.homozygote_count
            hemizygote_count = candidate.observation.hemizygote_count
            homozygote_match = (
                homozygote_threshold is not None
                and homozygote_count is not None
                and homozygote_count >= homozygote_threshold
            )
            hemizygote_match = (
                hemizygote_threshold is not None
                and hemizygote_count is not None
                and hemizygote_count >= hemizygote_threshold
            )
            if not homozygote_match and not hemizygote_match:
                continue
            if homozygote_match:
                assert homozygote_threshold is not None
                comparison = CriterionComparison(
                    input_name="homozygote_count",
                    operator=">=",
                    observed=homozygote_count,
                    expected=homozygote_threshold,
                    matched=True,
                )
            else:
                assert hemizygote_threshold is not None
                comparison = CriterionComparison(
                    input_name="hemizygote_count",
                    operator=">=",
                    observed=hemizygote_count,
                    expected=hemizygote_threshold,
                    matched=True,
                )
            matched_candidates.append((candidate, comparison))
        if not matched_candidates:
            return self._assessment(
                status=CriterionStatus.NOT_APPLIED,
                specification=specification,
                ruleset=ruleset,
                limitations=(
                    "population homozygote and hemizygote thresholds were not met",
                ),
            )
        decisive, comparison = max(
            matched_candidates,
            key=lambda value: (
                value[0].observation.homozygote_count or 0,
                value[0].observation.hemizygote_count or 0,
                value[0].observation.ancestry or "",
            ),
        )
        evidence_ids = _evidence_ids(facts, (decisive,))
        if not evidence_ids:
            return self._assessment(
                status=CriterionStatus.NOT_EVALUABLE,
                specification=specification,
                ruleset=ruleset,
                comparisons=(comparison,),
                limitations=(
                    "eligible population observations lack evidence identifiers",
                ),
            )
        return self._assessment(
            status=CriterionStatus.APPLIED,
            specification=specification,
            ruleset=ruleset,
            evidence_ids=evidence_ids,
            comparisons=(comparison,),
            applied_strength=configuration.applied_strength,
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


def _configuration(
    specification: CriterionSpecification,
) -> tuple[_PopulationConfiguration | None, str | None]:
    parameters = specification.parameters
    try:
        minimum_allele_number = _integer(parameters, "minimum_allele_number", minimum=1)
        minimum_coverage = _number(parameters, "minimum_coverage", minimum=0.0)
        required_filter_status = _text(parameters, "required_filter_status")
        applied_strength = _strength(parameters, specification)
        frequency_threshold = None
        homozygote_threshold = None
        hemizygote_threshold = None
        if specification.code in (CriterionCode.BA1, CriterionCode.BS1):
            frequency_threshold = _number(
                parameters, "minimum_allele_frequency", minimum=0.0, maximum=1.0
            )
        elif specification.code is CriterionCode.PM2:
            frequency_threshold = _number(
                parameters, "maximum_allele_frequency", minimum=0.0, maximum=1.0
            )
        elif specification.code is CriterionCode.BS2:
            if (
                "minimum_homozygote_count" not in parameters
                and "minimum_hemizygote_count" not in parameters
            ):
                raise ValueError("one BS2 count threshold is required")
            if "minimum_homozygote_count" in parameters:
                homozygote_threshold = _integer(
                    parameters, "minimum_homozygote_count", minimum=1
                )
            if "minimum_hemizygote_count" in parameters:
                hemizygote_threshold = _integer(
                    parameters, "minimum_hemizygote_count", minimum=1
                )
        maximum_frequency_spread = None
        if "maximum_frequency_spread" in parameters:
            maximum_frequency_spread = _number(
                parameters, "maximum_frequency_spread", minimum=0.0, maximum=1.0
            )
    except ValueError as error:
        return None, f"ruleset population configuration is invalid: {error}"
    return (
        _PopulationConfiguration(
            minimum_allele_number=minimum_allele_number,
            minimum_coverage=minimum_coverage,
            required_filter_status=required_filter_status,
            frequency_threshold=frequency_threshold,
            minimum_homozygote_count=homozygote_threshold,
            minimum_hemizygote_count=hemizygote_threshold,
            maximum_frequency_spread=maximum_frequency_spread,
            applied_strength=applied_strength,
        ),
        None,
    )


def _integer(parameters: Mapping[str, object], key: str, *, minimum: int) -> int:
    value = parameters.get(key)
    if not isinstance(value, int) or isinstance(value, bool) or value < minimum:
        raise ValueError(f"{key} must be an integer >= {minimum}")
    return value


def _number(
    parameters: Mapping[str, object],
    key: str,
    *,
    minimum: float,
    maximum: float | None = None,
) -> float:
    value = parameters.get(key)
    if (
        not isinstance(value, (int, float))
        or isinstance(value, bool)
        or not math.isfinite(value)
        or value < minimum
        or (maximum is not None and value > maximum)
    ):
        raise ValueError(f"{key} must be a finite number in the allowed range")
    return float(value)


def _text(parameters: Mapping[str, object], key: str) -> str:
    value = parameters.get(key)
    if not isinstance(value, str) or not value.strip():
        raise ValueError(f"{key} must be non-empty text")
    return value.strip()


def _strength(
    parameters: Mapping[str, object],
    specification: CriterionSpecification,
) -> CriterionStrength:
    configured = parameters.get("applied_strength")
    if configured is None:
        assert specification.base_strength is not None
        return specification.base_strength
    if not isinstance(configured, str):
        raise ValueError("applied_strength must be a controlled strength")
    try:
        value = CriterionStrength(configured)
    except ValueError as error:
        raise ValueError("applied_strength must be a controlled strength") from error
    if value not in specification.allowed_strengths:
        raise ValueError("applied_strength is not allowed by this criterion")
    return value


def _eligible_populations(
    observations: Iterable[PopulationObservation],
    configuration: _PopulationConfiguration,
) -> Iterable[_EligiblePopulation]:
    for observation in observations:
        allele_frequency = observation.allele_frequency
        allele_number = observation.allele_number
        coverage = observation.coverage
        if (
            allele_frequency is None
            or allele_number is None
            or coverage is None
            or allele_number < configuration.minimum_allele_number
            or coverage < configuration.minimum_coverage
            or observation.filter_status != configuration.required_filter_status
        ):
            continue
        yield _EligiblePopulation(
            observation=observation,
            allele_frequency=allele_frequency,
            allele_number=allele_number,
            coverage=coverage,
        )


def _frequency_spread_conflict(
    eligible: tuple[_EligiblePopulation, ...],
    maximum_spread: float | None,
) -> tuple[_EligiblePopulation, ...]:
    if maximum_spread is None:
        return ()
    by_ancestry: dict[str, list[_EligiblePopulation]] = defaultdict(list)
    for candidate in eligible:
        by_ancestry[candidate.observation.ancestry or "unspecified"].append(candidate)
    for candidates in by_ancestry.values():
        frequencies = [candidate.allele_frequency for candidate in candidates]
        if (
            len(frequencies) > 1
            and max(frequencies) - min(frequencies) > maximum_spread
        ):
            return tuple(candidates)
    return ()


def _evidence_ids(
    facts: FactSet,
    candidates: Iterable[_EligiblePopulation],
) -> tuple[str, ...]:
    evidence_ids = {
        evidence_id
        for candidate in candidates
        for evidence_id in facts.evidence_ids_for(candidate.observation)
    }
    return tuple(sorted(evidence_ids))
