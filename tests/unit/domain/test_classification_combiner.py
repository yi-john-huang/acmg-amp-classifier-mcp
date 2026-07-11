from __future__ import annotations

import pytest
from hypothesis import given, settings
from hypothesis import strategies as st

from acmg_classifier.domain.combination import (
    ClassificationCombiner,
    ConflictKind,
)
from acmg_classifier.domain.criteria import CriterionAssessment
from acmg_classifier.domain.enums import ClassificationTier, CriterionStatus
from acmg_classifier.domain.rules import (
    CriterionCode,
    CriterionSpecification,
    CriterionStrength,
    RulesetScope,
    RulesetSpecification,
    RulesetState,
)


def _default_strength(code: CriterionCode) -> CriterionStrength:
    if code is CriterionCode.PVS1:
        return CriterionStrength.VERY_STRONG
    if code.value.startswith("PS") or code.value.startswith("BS"):
        return CriterionStrength.STRONG
    if code.value.startswith("PM"):
        return CriterionStrength.MODERATE
    if code is CriterionCode.BA1:
        return CriterionStrength.STANDALONE
    return CriterionStrength.SUPPORTING


def _ruleset(
    *,
    algorithm_id: str = "acmg_2015",
) -> RulesetSpecification:
    return RulesetSpecification(
        ruleset_id="combination-test",
        version="1.0.0",
        state=RulesetState.APPROVED,
        publication_reference="PMID:25741868",
        scope=RulesetScope(),
        criteria=tuple(
            CriterionSpecification(
                code=code,
                enabled=True,
                base_strength=_default_strength(code),
                allowed_strengths=tuple(CriterionStrength),
                evaluator_id=code.value,
                evaluator_version_constraint=">=1.0.0,<2.0.0",
                rationale_template=f"{code.value} rationale",
            )
            for code in CriterionCode
        ),
        combination_algorithm_id=algorithm_id,
        combination_algorithm_version="1.0.0",
    )


def _assessment(
    code: CriterionCode,
    strength: CriterionStrength,
    *,
    status: CriterionStatus = CriterionStatus.APPLIED,
) -> CriterionAssessment:
    evidence_id = f"ev_{list(CriterionCode).index(code) + 1:064x}"
    return CriterionAssessment(
        code=code,
        status=status,
        original_strength=_default_strength(code),
        applied_strength=strength if status is CriterionStatus.APPLIED else None,
        evidence_ids=(evidence_id,) if status is CriterionStatus.APPLIED else (),
        comparisons=(),
        rationale_template=f"{code.value} rationale",
        rationale_values={},
        limitations=(),
        ruleset_id="combination-test",
        ruleset_version="1.0.0",
        evaluator_id=code.value,
        evaluator_version="1.0.0",
    )


CombinationInputs = tuple[tuple[CriterionCode, CriterionStrength], ...]


@pytest.mark.parametrize(
    ("inputs", "classification", "rule_id"),
    (
        pytest.param(
            (
                (CriterionCode.PVS1, CriterionStrength.VERY_STRONG),
                (CriterionCode.PS1, CriterionStrength.STRONG),
            ),
            ClassificationTier.PATHOGENIC,
            "pathogenic_pvs1_plus_strong",
            id="pathogenic-pvs1-strong",
        ),
        pytest.param(
            (
                (CriterionCode.PVS1, CriterionStrength.VERY_STRONG),
                (CriterionCode.PM1, CriterionStrength.MODERATE),
                (CriterionCode.PM2, CriterionStrength.MODERATE),
            ),
            ClassificationTier.PATHOGENIC,
            "pathogenic_pvs1_plus_two_moderate",
            id="pathogenic-pvs1-two-moderate",
        ),
        pytest.param(
            (
                (CriterionCode.PVS1, CriterionStrength.VERY_STRONG),
                (CriterionCode.PM1, CriterionStrength.MODERATE),
                (CriterionCode.PP1, CriterionStrength.SUPPORTING),
            ),
            ClassificationTier.PATHOGENIC,
            "pathogenic_pvs1_moderate_supporting",
            id="pathogenic-pvs1-moderate-supporting",
        ),
        pytest.param(
            (
                (CriterionCode.PVS1, CriterionStrength.VERY_STRONG),
                (CriterionCode.PP1, CriterionStrength.SUPPORTING),
                (CriterionCode.PP2, CriterionStrength.SUPPORTING),
            ),
            ClassificationTier.PATHOGENIC,
            "pathogenic_pvs1_plus_two_supporting",
            id="pathogenic-pvs1-two-supporting",
        ),
        pytest.param(
            (
                (CriterionCode.PS1, CriterionStrength.STRONG),
                (CriterionCode.PS2, CriterionStrength.STRONG),
            ),
            ClassificationTier.PATHOGENIC,
            "pathogenic_two_strong",
            id="pathogenic-two-strong",
        ),
        pytest.param(
            (
                (CriterionCode.PS1, CriterionStrength.STRONG),
                (CriterionCode.PM1, CriterionStrength.MODERATE),
                (CriterionCode.PM2, CriterionStrength.MODERATE),
                (CriterionCode.PM3, CriterionStrength.MODERATE),
            ),
            ClassificationTier.PATHOGENIC,
            "pathogenic_strong_plus_three_moderate",
            id="pathogenic-strong-three-moderate",
        ),
        pytest.param(
            (
                (CriterionCode.PS1, CriterionStrength.STRONG),
                (CriterionCode.PM1, CriterionStrength.MODERATE),
                (CriterionCode.PM2, CriterionStrength.MODERATE),
                (CriterionCode.PP1, CriterionStrength.SUPPORTING),
                (CriterionCode.PP2, CriterionStrength.SUPPORTING),
            ),
            ClassificationTier.PATHOGENIC,
            "pathogenic_strong_two_moderate_two_supporting",
            id="pathogenic-strong-two-moderate-two-supporting",
        ),
        pytest.param(
            (
                (CriterionCode.PS1, CriterionStrength.STRONG),
                (CriterionCode.PM1, CriterionStrength.MODERATE),
                (CriterionCode.PP1, CriterionStrength.SUPPORTING),
                (CriterionCode.PP2, CriterionStrength.SUPPORTING),
                (CriterionCode.PP3, CriterionStrength.SUPPORTING),
                (CriterionCode.PP4, CriterionStrength.SUPPORTING),
            ),
            ClassificationTier.PATHOGENIC,
            "pathogenic_strong_moderate_four_supporting",
            id="pathogenic-strong-moderate-four-supporting",
        ),
        pytest.param(
            (
                (CriterionCode.PVS1, CriterionStrength.VERY_STRONG),
                (CriterionCode.PM1, CriterionStrength.MODERATE),
            ),
            ClassificationTier.LIKELY_PATHOGENIC,
            "likely_pathogenic_pvs1_plus_moderate",
            id="likely-pathogenic-pvs1-moderate",
        ),
        pytest.param(
            (
                (CriterionCode.PS1, CriterionStrength.STRONG),
                (CriterionCode.PM1, CriterionStrength.MODERATE),
                (CriterionCode.PM2, CriterionStrength.MODERATE),
            ),
            ClassificationTier.LIKELY_PATHOGENIC,
            "likely_pathogenic_strong_plus_moderate",
            id="likely-pathogenic-strong-one-or-two-moderate",
        ),
        pytest.param(
            (
                (CriterionCode.PS1, CriterionStrength.STRONG),
                (CriterionCode.PP1, CriterionStrength.SUPPORTING),
                (CriterionCode.PP2, CriterionStrength.SUPPORTING),
            ),
            ClassificationTier.LIKELY_PATHOGENIC,
            "likely_pathogenic_strong_two_supporting",
            id="likely-pathogenic-strong-two-supporting",
        ),
        pytest.param(
            (
                (CriterionCode.PM1, CriterionStrength.MODERATE),
                (CriterionCode.PM2, CriterionStrength.MODERATE),
                (CriterionCode.PM3, CriterionStrength.MODERATE),
            ),
            ClassificationTier.LIKELY_PATHOGENIC,
            "likely_pathogenic_three_moderate",
            id="likely-pathogenic-three-moderate",
        ),
        pytest.param(
            (
                (CriterionCode.PM1, CriterionStrength.MODERATE),
                (CriterionCode.PM2, CriterionStrength.MODERATE),
                (CriterionCode.PP1, CriterionStrength.SUPPORTING),
                (CriterionCode.PP2, CriterionStrength.SUPPORTING),
            ),
            ClassificationTier.LIKELY_PATHOGENIC,
            "likely_pathogenic_two_moderate_two_supporting",
            id="likely-pathogenic-two-moderate-two-supporting",
        ),
        pytest.param(
            (
                (CriterionCode.PM1, CriterionStrength.MODERATE),
                (CriterionCode.PP1, CriterionStrength.SUPPORTING),
                (CriterionCode.PP2, CriterionStrength.SUPPORTING),
                (CriterionCode.PP3, CriterionStrength.SUPPORTING),
                (CriterionCode.PP4, CriterionStrength.SUPPORTING),
            ),
            ClassificationTier.LIKELY_PATHOGENIC,
            "likely_pathogenic_moderate_four_supporting",
            id="likely-pathogenic-moderate-four-supporting",
        ),
        pytest.param(
            ((CriterionCode.BA1, CriterionStrength.STANDALONE),),
            ClassificationTier.BENIGN,
            "benign_ba1",
            id="benign-ba1",
        ),
        pytest.param(
            (
                (CriterionCode.BS1, CriterionStrength.STRONG),
                (CriterionCode.BS2, CriterionStrength.STRONG),
            ),
            ClassificationTier.BENIGN,
            "benign_two_strong",
            id="benign-two-strong",
        ),
        pytest.param(
            (
                (CriterionCode.BS1, CriterionStrength.STRONG),
                (CriterionCode.BP1, CriterionStrength.SUPPORTING),
            ),
            ClassificationTier.LIKELY_BENIGN,
            "likely_benign_strong_supporting",
            id="likely-benign-strong-supporting",
        ),
        pytest.param(
            (
                (CriterionCode.BP1, CriterionStrength.SUPPORTING),
                (CriterionCode.BP2, CriterionStrength.SUPPORTING),
            ),
            ClassificationTier.LIKELY_BENIGN,
            "likely_benign_two_supporting",
            id="likely-benign-two-supporting",
        ),
    ),
)
def test_acmg_2015_combination_table(
    inputs: CombinationInputs,
    classification: ClassificationTier,
    rule_id: str,
) -> None:
    decision = ClassificationCombiner().combine(
        {code: _assessment(code, strength) for code, strength in inputs},
        _ruleset(),
    )

    assert decision.classification is classification
    assert decision.matched_rule_id == rule_id


_ORDERING_INPUTS: CombinationInputs = (
    (CriterionCode.PS1, CriterionStrength.STRONG),
    (CriterionCode.PM1, CriterionStrength.MODERATE),
    (CriterionCode.PM2, CriterionStrength.MODERATE),
    (CriterionCode.PP1, CriterionStrength.SUPPORTING),
    (CriterionCode.PP2, CriterionStrength.SUPPORTING),
)


@settings(max_examples=30)
@given(st.permutations(_ORDERING_INPUTS))
def test_combination_is_invariant_under_all_generated_input_orders(
    inputs: list[tuple[CriterionCode, CriterionStrength]],
) -> None:
    decision = ClassificationCombiner().combine(
        {code: _assessment(code, strength) for code, strength in inputs},
        _ruleset(),
    )

    assert decision.classification is ClassificationTier.PATHOGENIC
    assert decision.matched_rule_id == "pathogenic_strong_two_moderate_two_supporting"


def test_modified_strength_and_no_matching_rule_use_applied_strengths() -> None:
    ruleset = _ruleset()
    modified = ClassificationCombiner().combine(
        {
            CriterionCode.PVS1: _assessment(
                CriterionCode.PVS1, CriterionStrength.MODERATE
            ),
            CriterionCode.PS1: _assessment(CriterionCode.PS1, CriterionStrength.STRONG),
        },
        ruleset,
    )
    insufficient = ClassificationCombiner().combine(
        {CriterionCode.PM1: _assessment(CriterionCode.PM1, CriterionStrength.MODERATE)},
        ruleset,
    )

    assert modified.classification is ClassificationTier.LIKELY_PATHOGENIC
    assert modified.matched_rule_id == "likely_pathogenic_strong_plus_moderate"
    assert insufficient.classification is ClassificationTier.UNCERTAIN_SIGNIFICANCE
    assert insufficient.matched_rule_id == "uncertain_significance_no_matching_rule"


def test_combination_is_order_independent_and_records_pathogenic_rule() -> None:
    ruleset = _ruleset()
    assessments = {
        CriterionCode.PVS1: _assessment(
            CriterionCode.PVS1, CriterionStrength.VERY_STRONG
        ),
        CriterionCode.PS1: _assessment(CriterionCode.PS1, CriterionStrength.STRONG),
    }

    first = ClassificationCombiner().combine(assessments, ruleset)
    second = ClassificationCombiner().combine(
        dict(reversed(tuple(assessments.items()))), ruleset
    )

    assert first.classification is ClassificationTier.PATHOGENIC
    assert first.matched_rule_id == "pathogenic_pvs1_plus_strong"
    assert first.to_canonical_content() == second.to_canonical_content()


def test_benign_and_likely_pathogenic_rules_are_named() -> None:
    ruleset = _ruleset()

    benign = ClassificationCombiner().combine(
        {
            CriterionCode.BA1: _assessment(
                CriterionCode.BA1, CriterionStrength.STANDALONE
            )
        },
        ruleset,
    )
    likely_pathogenic = ClassificationCombiner().combine(
        {
            CriterionCode.PS1: _assessment(CriterionCode.PS1, CriterionStrength.STRONG),
            CriterionCode.PM1: _assessment(
                CriterionCode.PM1, CriterionStrength.MODERATE
            ),
        },
        ruleset,
    )

    assert benign.classification is ClassificationTier.BENIGN
    assert benign.matched_rule_id == "benign_ba1"
    assert likely_pathogenic.classification is ClassificationTier.LIKELY_PATHOGENIC
    assert likely_pathogenic.matched_rule_id == "likely_pathogenic_strong_plus_moderate"


def test_pathogenic_benign_and_criterion_conflicts_have_no_classification() -> None:
    ruleset = _ruleset()

    directional = ClassificationCombiner().combine(
        {
            CriterionCode.PS1: _assessment(CriterionCode.PS1, CriterionStrength.STRONG),
            CriterionCode.BS1: _assessment(CriterionCode.BS1, CriterionStrength.STRONG),
        },
        ruleset,
    )
    criterion = ClassificationCombiner().combine(
        {
            CriterionCode.PS1: _assessment(
                CriterionCode.PS1,
                CriterionStrength.STRONG,
                status=CriterionStatus.CONFLICTING,
            )
        },
        ruleset,
    )

    assert directional.classification is None
    assert directional.conflict is not None
    assert directional.conflict.kind is ConflictKind.DIRECTIONAL
    assert criterion.classification is None
    assert criterion.conflict is not None
    assert criterion.conflict.kind is ConflictKind.CRITERION


def test_source_conflicts_and_invalid_strengths_fail_closed_separately() -> None:
    ruleset = _ruleset()

    source = ClassificationCombiner().combine(
        {CriterionCode.PS1: _assessment(CriterionCode.PS1, CriterionStrength.STRONG)},
        ruleset,
        source_conflicts={"clinvar": ("ev_" + "f" * 64,)},
    )
    invalid_strength = ClassificationCombiner().combine(
        {
            CriterionCode.PVS1: _assessment(
                CriterionCode.PVS1, CriterionStrength.STANDALONE
            )
        },
        ruleset,
    )

    assert source.conflict is not None
    assert source.conflict.kind is ConflictKind.SOURCE
    assert invalid_strength.conflict is not None
    assert invalid_strength.conflict.kind is ConflictKind.INVALID_STRENGTH


def test_unknown_algorithm_is_an_unresolved_algorithm_conflict() -> None:
    decision = ClassificationCombiner().combine(
        {}, _ruleset(algorithm_id="unapproved_algorithm")
    )

    assert decision.classification is None
    assert decision.conflict is not None
    assert decision.conflict.kind is ConflictKind.ALGORITHM
