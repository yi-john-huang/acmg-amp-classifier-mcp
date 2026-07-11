"""Pure, deterministic ACMG/AMP assessment combination and conflict detection."""

from __future__ import annotations

from collections import Counter
from collections.abc import Mapping
from dataclasses import dataclass
from enum import StrEnum
from typing import Self

from pydantic import model_validator

from acmg_classifier.domain.canonical import canonical_hash
from acmg_classifier.domain.criteria import CriterionAssessment
from acmg_classifier.domain.enums import ClassificationTier, CriterionStatus
from acmg_classifier.domain.errors import JsonValue
from acmg_classifier.domain.evidence import EvidenceModel, NonEmptyText
from acmg_classifier.domain.rules import (
    CriterionCode,
    CriterionStrength,
    RulesetSpecification,
)


class ConflictKind(StrEnum):
    """Unresolved sources of a non-classifying combination result."""

    SOURCE = "source"
    CRITERION = "criterion"
    DIRECTIONAL = "directional"
    INVALID_STRENGTH = "invalid_strength"
    ALGORITHM = "algorithm"


class ClassificationConflict(EvidenceModel):
    """An explicit conflict that prevents a five-tier classification."""

    kind: ConflictKind
    criterion_codes: tuple[CriterionCode, ...] = ()
    source_ids: tuple[NonEmptyText, ...] = ()
    limitations: tuple[NonEmptyText, ...] = ()

    @model_validator(mode="after")
    def normalize_conflict(self) -> Self:
        criterion_codes = tuple(
            sorted(set(self.criterion_codes), key=lambda code: code.value)
        )
        source_ids = tuple(sorted(set(self.source_ids)))
        if self.kind is ConflictKind.SOURCE and not source_ids:
            raise ValueError("source conflict requires source_ids")
        if self.kind is not ConflictKind.SOURCE and not criterion_codes:
            raise ValueError("criterion conflict requires criterion_codes")
        object.__setattr__(self, "criterion_codes", criterion_codes)
        object.__setattr__(self, "source_ids", source_ids)
        return self

    def to_canonical_content(self) -> dict[str, JsonValue]:
        """Return the deterministic conflict payload used by decision hashing."""
        return {
            "kind": self.kind.value,
            "criterion_codes": [code.value for code in self.criterion_codes],
            "source_ids": list(self.source_ids),
            "limitations": list(self.limitations),
        }


class ClassificationDecision(EvidenceModel):
    """A named-algorithm decision or unresolved conflict over typed assessments."""

    algorithm_id: NonEmptyText
    algorithm_version: NonEmptyText
    classification: ClassificationTier | None
    conflict: ClassificationConflict | None = None
    matched_rule_id: NonEmptyText | None = None
    assessments: tuple[CriterionAssessment, ...] = ()
    criteria_hash: str | None = None
    limitations: tuple[NonEmptyText, ...] = ()

    @model_validator(mode="after")
    def validate_and_assign_hash(self) -> Self:
        assessments = tuple(
            sorted(self.assessments, key=lambda value: value.code.value)
        )
        codes = tuple(assessment.code for assessment in assessments)
        if len(codes) != len(set(codes)):
            raise ValueError(
                "classification decision cannot contain duplicate criterion codes"
            )
        if self.conflict is not None:
            if self.classification is not None or self.matched_rule_id is not None:
                raise ValueError(
                    "conflicted decision must not include a classification rule"
                )
        elif self.classification is None or self.matched_rule_id is None:
            raise ValueError(
                "non-conflicted decision requires classification and matched rule"
            )
        criteria_content: dict[str, JsonValue] = {}
        for assessment in assessments:
            criteria_content[assessment.code.value] = assessment.to_canonical_content()
        computed_hash = canonical_hash(criteria_content)
        if self.criteria_hash is not None and self.criteria_hash != computed_hash:
            raise ValueError("criteria_hash does not match canonical assessments")
        object.__setattr__(self, "assessments", assessments)
        object.__setattr__(self, "criteria_hash", computed_hash)
        return self

    def to_canonical_content(self) -> dict[str, JsonValue]:
        """Return deterministic decision content independent of input map order."""
        assessments: list[JsonValue] = []
        for assessment in self.assessments:
            assessments.append(assessment.to_canonical_content())
        return {
            "algorithm_id": self.algorithm_id,
            "algorithm_version": self.algorithm_version,
            "classification": (
                self.classification.value if self.classification is not None else None
            ),
            "conflict": (
                self.conflict.to_canonical_content()
                if self.conflict is not None
                else None
            ),
            "matched_rule_id": self.matched_rule_id,
            "assessments": assessments,
            "criteria_hash": self.criteria_hash,
            "limitations": list(self.limitations),
        }


@dataclass(frozen=True, slots=True)
class CombinationRule:
    """One explicit, minimum-strength evidence pattern in a named algorithm."""

    rule_id: str
    classification: ClassificationTier
    required_strengths: tuple[tuple[CriterionStrength, int], ...]

    def matches(self, counts: Counter[CriterionStrength]) -> bool:
        """Return whether every named strength meets its table minimum."""
        return all(
            counts[strength] >= minimum for strength, minimum in self.required_strengths
        )


_PATHOGENIC_RULES = (
    CombinationRule(
        "pathogenic_pvs1_plus_strong",
        ClassificationTier.PATHOGENIC,
        ((CriterionStrength.VERY_STRONG, 1), (CriterionStrength.STRONG, 1)),
    ),
    CombinationRule(
        "pathogenic_pvs1_plus_two_moderate",
        ClassificationTier.PATHOGENIC,
        ((CriterionStrength.VERY_STRONG, 1), (CriterionStrength.MODERATE, 2)),
    ),
    CombinationRule(
        "pathogenic_pvs1_moderate_supporting",
        ClassificationTier.PATHOGENIC,
        (
            (CriterionStrength.VERY_STRONG, 1),
            (CriterionStrength.MODERATE, 1),
            (CriterionStrength.SUPPORTING, 1),
        ),
    ),
    CombinationRule(
        "pathogenic_pvs1_plus_two_supporting",
        ClassificationTier.PATHOGENIC,
        ((CriterionStrength.VERY_STRONG, 1), (CriterionStrength.SUPPORTING, 2)),
    ),
    CombinationRule(
        "pathogenic_two_strong",
        ClassificationTier.PATHOGENIC,
        ((CriterionStrength.STRONG, 2),),
    ),
    CombinationRule(
        "pathogenic_strong_plus_three_moderate",
        ClassificationTier.PATHOGENIC,
        ((CriterionStrength.STRONG, 1), (CriterionStrength.MODERATE, 3)),
    ),
    CombinationRule(
        "pathogenic_strong_two_moderate_two_supporting",
        ClassificationTier.PATHOGENIC,
        (
            (CriterionStrength.STRONG, 1),
            (CriterionStrength.MODERATE, 2),
            (CriterionStrength.SUPPORTING, 2),
        ),
    ),
    CombinationRule(
        "pathogenic_strong_moderate_four_supporting",
        ClassificationTier.PATHOGENIC,
        (
            (CriterionStrength.STRONG, 1),
            (CriterionStrength.MODERATE, 1),
            (CriterionStrength.SUPPORTING, 4),
        ),
    ),
)

_LIKELY_PATHOGENIC_RULES = (
    CombinationRule(
        "likely_pathogenic_pvs1_plus_moderate",
        ClassificationTier.LIKELY_PATHOGENIC,
        ((CriterionStrength.VERY_STRONG, 1), (CriterionStrength.MODERATE, 1)),
    ),
    CombinationRule(
        "likely_pathogenic_strong_plus_moderate",
        ClassificationTier.LIKELY_PATHOGENIC,
        ((CriterionStrength.STRONG, 1), (CriterionStrength.MODERATE, 1)),
    ),
    CombinationRule(
        "likely_pathogenic_strong_two_supporting",
        ClassificationTier.LIKELY_PATHOGENIC,
        ((CriterionStrength.STRONG, 1), (CriterionStrength.SUPPORTING, 2)),
    ),
    CombinationRule(
        "likely_pathogenic_three_moderate",
        ClassificationTier.LIKELY_PATHOGENIC,
        ((CriterionStrength.MODERATE, 3),),
    ),
    CombinationRule(
        "likely_pathogenic_two_moderate_two_supporting",
        ClassificationTier.LIKELY_PATHOGENIC,
        ((CriterionStrength.MODERATE, 2), (CriterionStrength.SUPPORTING, 2)),
    ),
    CombinationRule(
        "likely_pathogenic_moderate_four_supporting",
        ClassificationTier.LIKELY_PATHOGENIC,
        ((CriterionStrength.MODERATE, 1), (CriterionStrength.SUPPORTING, 4)),
    ),
)

_BENIGN_RULES = (
    CombinationRule(
        "benign_ba1",
        ClassificationTier.BENIGN,
        ((CriterionStrength.STANDALONE, 1),),
    ),
    CombinationRule(
        "benign_two_strong",
        ClassificationTier.BENIGN,
        ((CriterionStrength.STRONG, 2),),
    ),
)

_LIKELY_BENIGN_RULES = (
    CombinationRule(
        "likely_benign_strong_supporting",
        ClassificationTier.LIKELY_BENIGN,
        ((CriterionStrength.STRONG, 1), (CriterionStrength.SUPPORTING, 1)),
    ),
    CombinationRule(
        "likely_benign_two_supporting",
        ClassificationTier.LIKELY_BENIGN,
        ((CriterionStrength.SUPPORTING, 2),),
    ),
)


class ClassificationCombiner:
    """Apply the named ACMG 2015 table without evaluation-order side effects."""

    _SUPPORTED_ALGORITHM = ("acmg_2015", "1.0.0")

    def combine(
        self,
        assessments: Mapping[CriterionCode, CriterionAssessment],
        ruleset: RulesetSpecification,
        *,
        source_conflicts: Mapping[str, tuple[str, ...]] | None = None,
    ) -> ClassificationDecision:
        """Combine a code-keyed assessment map into one decision or conflict."""
        ordered = tuple(
            sorted(assessments.values(), key=lambda assessment: assessment.code.value)
        )
        if (
            ruleset.combination_algorithm_id,
            ruleset.combination_algorithm_version,
        ) != self._SUPPORTED_ALGORITHM:
            return self._conflict_decision(
                ordered,
                ruleset,
                ClassificationConflict(
                    kind=ConflictKind.ALGORITHM,
                    criterion_codes=tuple(assessment.code for assessment in ordered)
                    or (CriterionCode.PVS1,),
                    limitations=("selected combination algorithm is not supported",),
                ),
            )
        invalid_codes = _invalid_assessment_codes(assessments, ruleset)
        if invalid_codes:
            return self._conflict_decision(
                ordered,
                ruleset,
                ClassificationConflict(
                    kind=ConflictKind.INVALID_STRENGTH,
                    criterion_codes=invalid_codes,
                    limitations=(
                        "assessment strength is not permitted by selected ruleset",
                    ),
                ),
            )
        if source_conflicts:
            source_ids = tuple(
                sorted(source for source, values in source_conflicts.items() if values)
            )
            if source_ids:
                return self._conflict_decision(
                    ordered,
                    ruleset,
                    ClassificationConflict(
                        kind=ConflictKind.SOURCE,
                        source_ids=source_ids,
                        limitations=("source-level evidence conflict requires review",),
                    ),
                )
        conflicting_codes = tuple(
            assessment.code
            for assessment in ordered
            if assessment.status is CriterionStatus.CONFLICTING
        )
        if conflicting_codes:
            return self._conflict_decision(
                ordered,
                ruleset,
                ClassificationConflict(
                    kind=ConflictKind.CRITERION,
                    criterion_codes=conflicting_codes,
                    limitations=("criterion-level evidence conflict requires review",),
                ),
            )

        applied = tuple(
            assessment
            for assessment in ordered
            if assessment.status is CriterionStatus.APPLIED
        )
        pathogenic = tuple(
            assessment for assessment in applied if not assessment.code.is_benign
        )
        benign = tuple(
            assessment for assessment in applied if assessment.code.is_benign
        )
        if pathogenic and benign:
            return self._conflict_decision(
                ordered,
                ruleset,
                ClassificationConflict(
                    kind=ConflictKind.DIRECTIONAL,
                    criterion_codes=tuple(assessment.code for assessment in applied),
                    limitations=("pathogenic and benign criteria are both applied",),
                ),
            )
        if pathogenic:
            tier, rule = _pathogenic_combination(pathogenic)
        elif benign:
            tier, rule = _benign_combination(benign)
        else:
            tier, rule = (
                ClassificationTier.UNCERTAIN_SIGNIFICANCE,
                "uncertain_significance_no_applied_criteria",
            )
        return ClassificationDecision(
            algorithm_id=ruleset.combination_algorithm_id,
            algorithm_version=ruleset.combination_algorithm_version,
            classification=tier,
            conflict=None,
            matched_rule_id=rule,
            assessments=ordered,
            limitations=_limitations(ordered),
        )

    @staticmethod
    def _conflict_decision(
        assessments: tuple[CriterionAssessment, ...],
        ruleset: RulesetSpecification,
        conflict: ClassificationConflict,
    ) -> ClassificationDecision:
        return ClassificationDecision(
            algorithm_id=ruleset.combination_algorithm_id,
            algorithm_version=ruleset.combination_algorithm_version,
            classification=None,
            conflict=conflict,
            matched_rule_id=None,
            assessments=assessments,
            limitations=_limitations(assessments),
        )


def _invalid_assessment_codes(
    assessments: Mapping[CriterionCode, CriterionAssessment],
    ruleset: RulesetSpecification,
) -> tuple[CriterionCode, ...]:
    specifications = {
        specification.code: specification for specification in ruleset.criteria
    }
    invalid: set[CriterionCode] = set()
    for key, assessment in assessments.items():
        specification = specifications.get(key)
        if key is not assessment.code or specification is None:
            invalid.add(assessment.code)
            continue
        if (
            assessment.ruleset_id != ruleset.ruleset_id
            or assessment.ruleset_version != ruleset.version
        ):
            invalid.add(assessment.code)
            continue
        if assessment.status is not CriterionStatus.APPLIED:
            continue
        strength = assessment.applied_strength
        if strength is None or strength not in specification.allowed_strengths:
            invalid.add(assessment.code)
            continue
        if strength is CriterionStrength.STANDALONE and not assessment.code.is_benign:
            invalid.add(assessment.code)
        if strength is CriterionStrength.VERY_STRONG and assessment.code.is_benign:
            invalid.add(assessment.code)
    return tuple(sorted(invalid, key=lambda code: code.value))


def _pathogenic_combination(
    assessments: tuple[CriterionAssessment, ...],
) -> tuple[ClassificationTier, str]:
    return _match_combination(
        _strength_counts(assessments),
        _PATHOGENIC_RULES + _LIKELY_PATHOGENIC_RULES,
    )


def _benign_combination(
    assessments: tuple[CriterionAssessment, ...],
) -> tuple[ClassificationTier, str]:
    return _match_combination(
        _strength_counts(assessments),
        _BENIGN_RULES + _LIKELY_BENIGN_RULES,
    )


def _match_combination(
    counts: Counter[CriterionStrength],
    rules: tuple[CombinationRule, ...],
) -> tuple[ClassificationTier, str]:
    for rule in rules:
        if rule.matches(counts):
            return rule.classification, rule.rule_id
    return (
        ClassificationTier.UNCERTAIN_SIGNIFICANCE,
        "uncertain_significance_no_matching_rule",
    )


def _strength_counts(
    assessments: tuple[CriterionAssessment, ...],
) -> Counter[CriterionStrength]:
    return Counter(
        assessment.applied_strength
        for assessment in assessments
        if assessment.applied_strength is not None
    )


def _limitations(
    assessments: tuple[CriterionAssessment, ...],
) -> tuple[str, ...]:
    return tuple(
        sorted(
            {
                limitation
                for assessment in assessments
                for limitation in assessment.limitations
            }
        )
    )
