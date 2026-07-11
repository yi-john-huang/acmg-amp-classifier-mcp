from __future__ import annotations

import unittest

from acmg_classifier.application.explanation import (
    ExplanationDetail,
    ExplanationService,
    ExplanationTemplateError,
)
from acmg_classifier.domain.combination import (
    ClassificationConflict,
    ClassificationDecision,
    ConflictKind,
)
from acmg_classifier.domain.criteria import CriterionAssessment, CriterionComparison
from acmg_classifier.domain.enums import ClassificationTier, CriterionStatus
from acmg_classifier.domain.rules import CriterionCode, CriterionStrength

_EVIDENCE_ID = "ev_" + "a" * 64


class ExplanationServiceTests(unittest.TestCase):
    def setUp(self) -> None:
        self.service = ExplanationService()

    def test_standard_rendering_is_deterministic_and_references_only_assessment_facts(
        self,
    ) -> None:
        decision = _decision((_assessment(CriterionCode.PM2), _not_evaluable()))

        first = self.service.render_decision(
            decision, detail=ExplanationDetail.STANDARD
        )
        second = self.service.render_decision(
            _decision((_not_evaluable(), _assessment(CriterionCode.PM2))),
            detail=ExplanationDetail.STANDARD,
        )

        self.assertEqual(first, second)
        self.assertEqual(first.classification, "Uncertain Significance")
        self.assertEqual(first.blocks[0].kind, "summary")
        pm2_block = next(
            block for block in first.blocks if block.criterion_code == "PM2"
        )
        self.assertEqual(pm2_block.evidence_ids, (_EVIDENCE_ID,))
        self.assertIn("frequency=0.0001", pm2_block.text)
        self.assertNotIn("unverified", first.to_canonical_content().__repr__())
        self.assertIn("Research use only", first.disclaimer)

    def test_compact_omits_criterion_prose_and_full_includes_comparisons(self) -> None:
        decision = _decision((_assessment(CriterionCode.PM2),))

        compact = self.service.render_decision(
            decision, detail=ExplanationDetail.COMPACT
        )
        full = self.service.render_decision(decision, detail=ExplanationDetail.FULL)

        self.assertEqual(tuple(block.kind for block in compact.blocks), ("summary",))
        self.assertEqual(
            tuple(block.kind for block in full.blocks),
            ("summary", "criterion", "comparison"),
        )
        self.assertEqual(full.blocks[-1].evidence_ids, (_EVIDENCE_ID,))

    def test_conflict_never_claims_a_five_tier_classification(self) -> None:
        decision = ClassificationDecision(
            algorithm_id="acmg-2015",
            algorithm_version="1.0.0",
            classification=None,
            conflict=ClassificationConflict(
                kind=ConflictKind.DIRECTIONAL,
                criterion_codes=(CriterionCode.PS1, CriterionCode.BS1),
                limitations=("pathogenic and benign evidence conflict",),
            ),
            assessments=(_assessment(CriterionCode.PS1),),
            limitations=("manual review required",),
        )

        explanation = self.service.render_decision(decision)

        self.assertIsNone(explanation.classification)
        self.assertIn("Conflict", explanation.blocks[0].text)
        self.assertEqual(explanation.blocks[0].criterion_codes, ("BS1", "PS1"))
        self.assertEqual(
            explanation.limitations,
            (
                "frequency is from the available population dataset",
                "manual review required",
                "pathogenic and benign evidence conflict",
            ),
        )
    def test_conflicting_assessment_renders_in_standard_and_full_conflicts(self) -> None:
        decision = ClassificationDecision(
            algorithm_id="acmg-2015",
            algorithm_version="1.0.0",
            classification=None,
            conflict=ClassificationConflict(
                kind=ConflictKind.CRITERION,
                criterion_codes=(CriterionCode.PS1,),
                limitations=("criterion-level evidence conflict requires review",),
            ),
            assessments=(
                _assessment(CriterionCode.PS1, status=CriterionStatus.CONFLICTING),
            ),
        )

        for detail in (ExplanationDetail.STANDARD, ExplanationDetail.FULL):
            with self.subTest(detail=detail):
                explanation = self.service.render_decision(decision, detail=detail)

                criterion_block = next(
                    block
                    for block in explanation.blocks
                    if block.criterion_code == CriterionCode.PS1.value
                )
                self.assertIn("conflicting", criterion_block.text)
                self.assertIsNone(explanation.classification)


    def test_templates_fail_closed_for_missing_or_unsafe_fact_references(self) -> None:
        missing_value = _assessment(
            CriterionCode.PM2,
            template="frequency={missing}",
            values={"frequency": 0.0001},
        )
        unsafe_field = _assessment(
            CriterionCode.PM2,
            template="frequency={frequency.__class__}",
            values={"frequency": 0.0001},
        )

        with self.assertRaises(ExplanationTemplateError):
            self.service.render_decision(_decision((missing_value,)))
        with self.assertRaises(ExplanationTemplateError):
            self.service.render_decision(_decision((unsafe_field,)))


def _decision(
    assessments: tuple[CriterionAssessment, ...],
) -> ClassificationDecision:
    return ClassificationDecision(
        algorithm_id="acmg-2015",
        algorithm_version="1.0.0",
        classification=ClassificationTier.UNCERTAIN_SIGNIFICANCE,
        matched_rule_id="vus_default",
        assessments=assessments,
        limitations=("limited evidence",),
    )


def _assessment(
    code: CriterionCode,
    *,
    status: CriterionStatus = CriterionStatus.APPLIED,
    template: str = "frequency={frequency}",
    values: dict[str, object] | None = None,
) -> CriterionAssessment:
    return CriterionAssessment(
        code=code,
        status=status,
        original_strength=CriterionStrength.MODERATE,
        applied_strength=(
            CriterionStrength.MODERATE
            if status is CriterionStatus.APPLIED
            else None
        ),
        evidence_ids=(_EVIDENCE_ID,),
        comparisons=(
            CriterionComparison(
                input_name="frequency",
                operator="<=",
                observed=0.0001,
                expected=0.001,
                matched=True,
            ),
        ),
        rationale_template=template,
        rationale_values={"frequency": 0.0001} if values is None else values,
        limitations=("frequency is from the available population dataset",),
        ruleset_id="test",
        ruleset_version="1.0.0",
        evaluator_id="population",
        evaluator_version="1.0.0",
    )


def _not_evaluable() -> CriterionAssessment:
    return CriterionAssessment(
        code=CriterionCode.BA1,
        status=CriterionStatus.NOT_EVALUABLE,
        original_strength=CriterionStrength.STANDALONE,
        evidence_ids=(),
        comparisons=(),
        rationale_template="population data unavailable",
        rationale_values={},
        limitations=("source unavailable",),
        ruleset_id="test",
        ruleset_version="1.0.0",
        evaluator_id="population",
        evaluator_version="1.0.0",
    )


if __name__ == "__main__":
    unittest.main()
