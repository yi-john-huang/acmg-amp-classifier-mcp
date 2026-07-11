"""Pure, deterministic golden-validation metrics.

No application or transport code is imported here: a report is a comparison of a
strict fixture manifest and explicit execution outcomes.
"""

from __future__ import annotations

from collections.abc import Mapping, Sequence
from dataclasses import dataclass
from types import MappingProxyType

from acmg_classifier.domain.enums import ClassificationTier, CriterionStatus
from acmg_classifier.validation.fixtures import GoldenValidationManifest


class ValidationMetricError(ValueError):
    """Raised when outcomes cannot be compared safely to a fixture manifest."""


_TIER_ORDER = {
    ClassificationTier.PATHOGENIC: 0,
    ClassificationTier.LIKELY_PATHOGENIC: 1,
    ClassificationTier.UNCERTAIN_SIGNIFICANCE: 2,
    ClassificationTier.LIKELY_BENIGN: 3,
    ClassificationTier.BENIGN: 4,
}


@dataclass(frozen=True, slots=True)
class Rate:
    """An auditable count and denominator with an explicit not-applicable state."""

    numerator: int
    denominator: int

    def __post_init__(self) -> None:
        if self.numerator < 0 or self.denominator < 0:
            raise ValueError("rate values must not be negative")
        if self.numerator > self.denominator:
            raise ValueError("rate numerator must not exceed denominator")

    @property
    def rate(self) -> float | None:
        if self.denominator == 0:
            return None
        return self.numerator / self.denominator

    def to_dict(self) -> dict[str, int | float | None]:
        return {
            "numerator": self.numerator,
            "denominator": self.denominator,
            "rate": self.rate,
        }


@dataclass(frozen=True, slots=True)
class CaseValidationOutcome:
    """Structured application result for one golden case."""

    case_id: str
    classification: ClassificationTier | None
    criterion_statuses: Mapping[str, CriterionStatus]
    normalization_failed: bool

    def __post_init__(self) -> None:
        if not self.case_id.strip():
            raise ValueError("validation outcome case_id must not be empty")
        invalid_statuses = [
            code
            for code, status in self.criterion_statuses.items()
            if not isinstance(code, str) or not isinstance(status, CriterionStatus)
        ]
        if invalid_statuses:
            raise ValueError("validation outcome criterion statuses must be typed")
        object.__setattr__(
            self,
            "criterion_statuses",
            MappingProxyType(dict(sorted(self.criterion_statuses.items()))),
        )


@dataclass(frozen=True, slots=True)
class CriterionMetrics:
    """Positive-class criterion agreement and not-evaluable execution rate."""

    true_positive: int
    false_positive: int
    false_negative: int
    precision: Rate
    recall: Rate
    f1: Rate
    not_evaluable_rate: Rate

    def to_dict(self) -> dict[str, object]:
        return {
            "true_positive": self.true_positive,
            "false_positive": self.false_positive,
            "false_negative": self.false_negative,
            "precision": self.precision.to_dict(),
            "recall": self.recall.to_dict(),
            "f1": self.f1.to_dict(),
            "not_evaluable_rate": self.not_evaluable_rate.to_dict(),
        }


@dataclass(frozen=True, slots=True)
class ValidationMetrics:
    """All Task 10.2 metrics, including denominators and disclosed exclusions."""

    executed_case_count: int
    exclusion_count: int
    exact_five_tier_concordance: Rate
    adjacent_disagreement: Rate
    opposite_direction_disagreement: Rate
    unclassified_disagreement: Rate
    normalization_failure_rate: Rate
    not_evaluable_rate: Rate
    criterion_metrics: Mapping[str, CriterionMetrics]

    def __post_init__(self) -> None:
        object.__setattr__(
            self,
            "criterion_metrics",
            MappingProxyType(dict(sorted(self.criterion_metrics.items()))),
        )

    def to_dict(self) -> dict[str, object]:
        return {
            "executed_case_count": self.executed_case_count,
            "exclusion_count": self.exclusion_count,
            "exact_five_tier_concordance": self.exact_five_tier_concordance.to_dict(),
            "adjacent_disagreement": self.adjacent_disagreement.to_dict(),
            "opposite_direction_disagreement": (
                self.opposite_direction_disagreement.to_dict()
            ),
            "unclassified_disagreement": self.unclassified_disagreement.to_dict(),
            "normalization_failure_rate": self.normalization_failure_rate.to_dict(),
            "not_evaluable_rate": self.not_evaluable_rate.to_dict(),
            "criterion_metrics": {
                code: metric.to_dict()
                for code, metric in self.criterion_metrics.items()
            },
        }


def _validate_outcomes(
    manifest: GoldenValidationManifest, outcomes: Sequence[CaseValidationOutcome]
) -> Mapping[str, CaseValidationOutcome]:
    expected_ids = {case.case_id for case in manifest.cases}
    outcomes_by_id = {outcome.case_id: outcome for outcome in outcomes}
    if len(outcomes_by_id) != len(outcomes):
        raise ValidationMetricError("validation outcomes have duplicate case IDs")
    if set(outcomes_by_id) != expected_ids:
        missing = sorted(expected_ids - set(outcomes_by_id))
        unexpected = sorted(set(outcomes_by_id) - expected_ids)
        details: list[str] = []
        if missing:
            details.append("missing outcomes: " + ", ".join(missing))
        if unexpected:
            details.append("unexpected outcomes: " + ", ".join(unexpected))
        raise ValidationMetricError("; ".join(details))
    return MappingProxyType(outcomes_by_id)


def _criterion_metrics(
    manifest: GoldenValidationManifest,
    outcomes_by_id: Mapping[str, CaseValidationOutcome],
) -> tuple[Mapping[str, CriterionMetrics], Rate]:
    expected_codes = {
        code for case in manifest.cases for code in case.expected.criterion_statuses
    }
    calculated: dict[str, CriterionMetrics] = {}
    all_not_evaluable = 0
    all_criterion_results = 0
    for code in sorted(expected_codes):
        true_positive = false_positive = false_negative = not_evaluable = (
            denominator
        ) = 0
        for case in manifest.cases:
            expected_status = case.expected.criterion_statuses.get(code)
            outcome_status = outcomes_by_id[case.case_id].criterion_statuses.get(code)
            if expected_status is None:
                if outcome_status is not None:
                    raise ValidationMetricError(
                        f"outcome {case.case_id} returned undeclared criterion {code}"
                    )
                continue
            if outcome_status is None:
                raise ValidationMetricError(
                    f"outcome {case.case_id} omitted expected criterion {code}"
                )
            denominator += 1
            all_criterion_results += 1
            if outcome_status is CriterionStatus.NOT_EVALUABLE:
                not_evaluable += 1
                all_not_evaluable += 1
            expected_applied = expected_status is CriterionStatus.APPLIED
            observed_applied = outcome_status is CriterionStatus.APPLIED
            if expected_applied and observed_applied:
                true_positive += 1
            elif observed_applied:
                false_positive += 1
            elif expected_applied:
                false_negative += 1
        precision_denominator = true_positive + false_positive
        recall_denominator = true_positive + false_negative
        precision = Rate(true_positive, precision_denominator)
        recall = Rate(true_positive, recall_denominator)
        f1_numerator = 2 * true_positive
        f1_denominator = 2 * true_positive + false_positive + false_negative
        calculated[code] = CriterionMetrics(
            true_positive=true_positive,
            false_positive=false_positive,
            false_negative=false_negative,
            precision=precision,
            recall=recall,
            f1=Rate(f1_numerator, f1_denominator),
            not_evaluable_rate=Rate(not_evaluable, denominator),
        )
    return MappingProxyType(calculated), Rate(all_not_evaluable, all_criterion_results)


def calculate_validation_metrics(
    manifest: GoldenValidationManifest, outcomes: Sequence[CaseValidationOutcome]
) -> ValidationMetrics:
    """Calculate deterministic, denominator-preserving validation metrics.

    Expected classifications are ordered P, LP, VUS, LB, B.  An adjacent
    disagreement differs by one tier; an opposite-direction disagreement differs
    by two or more tiers.  A missing observed tier is reported separately rather
    than silently classified as an adjacent or opposite disagreement.
    """
    outcomes_by_id = _validate_outcomes(manifest, outcomes)
    classification_denominator = 0
    exact = adjacent = opposite = unclassified = normalization_failures = 0
    for case in manifest.cases:
        outcome = outcomes_by_id[case.case_id]
        if outcome.normalization_failed:
            normalization_failures += 1
        expected_tier = case.expected.classification
        if expected_tier is None:
            continue
        classification_denominator += 1
        observed_tier = outcome.classification
        if observed_tier is None:
            unclassified += 1
            continue
        distance = abs(_TIER_ORDER[expected_tier] - _TIER_ORDER[observed_tier])
        if distance == 0:
            exact += 1
        elif distance == 1:
            adjacent += 1
        else:
            opposite += 1
    criterion_metrics, not_evaluable_rate = _criterion_metrics(manifest, outcomes_by_id)
    return ValidationMetrics(
        executed_case_count=len(manifest.cases),
        exclusion_count=len(manifest.exclusions),
        exact_five_tier_concordance=Rate(exact, classification_denominator),
        adjacent_disagreement=Rate(adjacent, classification_denominator),
        opposite_direction_disagreement=Rate(opposite, classification_denominator),
        unclassified_disagreement=Rate(unclassified, classification_denominator),
        normalization_failure_rate=Rate(normalization_failures, len(manifest.cases)),
        not_evaluable_rate=not_evaluable_rate,
        criterion_metrics=criterion_metrics,
    )
