from __future__ import annotations

from acmg_classifier.application.explanation import ExplanationDetail
from acmg_classifier.domain.combination import ClassificationDecision
from acmg_classifier.domain.enums import ClassificationTier
from acmg_classifier.presentation.serialization import stored_explanation_content


def test_stored_explanation_renders_requested_detail_from_immutable_decision() -> None:
    decision = ClassificationDecision(
        algorithm_id="acmg-amp",
        algorithm_version="2015.1",
        classification=ClassificationTier.LIKELY_PATHOGENIC,
        matched_rule_id="pathogenic_ps1_moderate_supporting",
    )

    explanation = stored_explanation_content(
        {"decision": decision.to_canonical_content()},
        detail=ExplanationDetail.COMPACT,
    )

    assert explanation["detail"] == "compact"
    assert explanation["classification"] == "Likely Pathogenic"
    blocks = explanation["blocks"]
    assert isinstance(blocks, list)
    assert isinstance(blocks[0], dict)
    assert blocks[0]["kind"] == "summary"
