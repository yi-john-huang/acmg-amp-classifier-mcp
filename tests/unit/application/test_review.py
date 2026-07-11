from __future__ import annotations

from dataclasses import FrozenInstanceError
from datetime import UTC, datetime

import pytest

from acmg_classifier.application.explanation import ExplanationDetail
from acmg_classifier.application.review import (
    DEFAULT_REVIEW_TOKEN_CAP,
    ReviewPacketBuilder,
    ReviewReason,
    ReviewTaskType,
)
from acmg_classifier.domain.canonical import canonical_hash
from acmg_classifier.domain.combination import (
    ClassificationConflict,
    ClassificationDecision,
    ConflictKind,
)
from acmg_classifier.domain.criteria import CriterionAssessment
from acmg_classifier.domain.enums import ClassificationTier, CriterionStatus
from acmg_classifier.domain.evidence import EvidenceItem
from acmg_classifier.domain.rules import CriterionCode, CriterionStrength


def _evidence(kind: str, *, cited: bool = True) -> EvidenceItem:
    observation: dict[str, object]
    if kind == "functional":
        observation = {
            "kind": "functional",
            "assay_id": "PMID:12345678:assay" if cited else "assay-1",
            "assay_type": "DNA repair",
            "result": "abnormal",
            "validation_status": "validated",
        }
    elif kind == "case_control":
        observation = {
            "kind": "case_control",
            "study_id": "PMID:23456789" if cited else "study-1",
            "case_count": 100,
            "control_count": 1000,
            "case_allele_count": 6,
            "control_allele_count": 1,
            "odds_ratio": 4.2,
            "p_value": 0.01,
        }
    else:
        observation = {
            "kind": "population",
            "source_release": "gnomad-r4.1",
            "allele_count": 1,
            "allele_number": 1000,
            "allele_frequency": 0.001,
            "filter_status": "pass",
        }
    return EvidenceItem.model_validate(
        {
            "variant_key": "ga4gh:VA.example",
            "kind": kind,
            "observation": observation,
            "context_scope": {"genome_build": "GRCh38"},
            "provenance": {
                "kind": "source",
                "source_id": "literature",
                "source_record_id": "record-1",
                "retrieved_at": datetime(2026, 7, 11, tzinfo=UTC),
                "normalized_query_key": "ga4gh:VA.example",
            },
            "raw_snapshot_ref": "raw_" + "a" * 64,
            "derivation": "source",
        }
    )


def _assessment(evidence_id: str) -> CriterionAssessment:
    return CriterionAssessment(
        code=CriterionCode.PS3,
        status=CriterionStatus.APPLIED,
        original_strength=CriterionStrength.STRONG,
        applied_strength=CriterionStrength.STRONG,
        evidence_ids=(evidence_id,),
        rationale_template="PS3 rationale",
        rationale_values={},
        ruleset_id="default-rules",
        ruleset_version="1.0.0",
        evaluator_id="PS3",
        evaluator_version="1.0.0",
    )


def _completed_decision(evidence_id: str) -> ClassificationDecision:
    return ClassificationDecision(
        algorithm_id="acmg_2015",
        algorithm_version="1.0.0",
        classification=ClassificationTier.PATHOGENIC,
        matched_rule_id="test-rule",
        assessments=(_assessment(evidence_id),),
    )


def _conflict_decision(evidence_id: str) -> ClassificationDecision:
    return ClassificationDecision(
        algorithm_id="acmg_2015",
        algorithm_version="1.0.0",
        classification=None,
        conflict=ClassificationConflict(
            kind=ConflictKind.CRITERION,
            criterion_codes=(CriterionCode.PS3,),
        ),
        assessments=(_assessment(evidence_id),),
    )


def _build(
    decision: ClassificationDecision,
    evidence_items: tuple[EvidenceItem, ...],
    *,
    detail: ExplanationDetail = ExplanationDetail.STANDARD,
    request: dict[str, object] | None = None,
):
    return ReviewPacketBuilder().build(
        classification_id="cls_123",
        decision=decision,
        evidence_items=evidence_items,
        request=request or {"variant": "NM_007294.4:c.1A>G", "intent": "germline"},
        snapshot_id="es_" + "b" * 64,
        ruleset_id="default-rules",
        ruleset_version="1.0.0",
        detail=detail,
    )


def test_routine_completed_decision_has_no_review_recommendation() -> None:
    evidence = _evidence("population")

    packet = _build(_completed_decision(evidence.evidence_id), (evidence,))

    assert packet is None


def test_conflict_outranks_literature_and_full_detail_with_minimal_subset() -> None:
    conflict_evidence = _evidence("population")
    literature_evidence = _evidence("functional")

    packet = _build(
        _conflict_decision(conflict_evidence.evidence_id),
        (literature_evidence, conflict_evidence),
        detail=ExplanationDetail.FULL,
    )

    assert packet is not None
    assert packet.task_type is ReviewTaskType.EVIDENCE_CONFLICT
    assert packet.reason is ReviewReason.UNRESOLVED_CONFLICT
    assert packet.selected_evidence_ids == (conflict_evidence.evidence_id,)
    assert tuple(item.evidence_id for item in packet.observations) == (
        conflict_evidence.evidence_id,
    )
    assert packet.token_cap == DEFAULT_REVIEW_TOKEN_CAP
    assert packet.token_cap <= 1200


@pytest.mark.parametrize("kind", ("functional", "case_control"))
def test_cited_study_evidence_triggers_literature_synthesis(kind: str) -> None:
    evidence = _evidence(kind)

    packet = _build(_completed_decision(evidence.evidence_id), (evidence,))

    assert packet is not None
    assert packet.task_type is ReviewTaskType.LITERATURE_SYNTHESIS
    assert packet.reason is ReviewReason.CITED_STUDY_SYNTHESIS
    assert packet.selected_evidence_ids == (evidence.evidence_id,)


def test_unreferenced_cited_evidence_does_not_trigger_literature_synthesis() -> None:
    referenced_evidence = _evidence("population")
    unreferenced_literature = _evidence("functional")

    packet = _build(
        _completed_decision(referenced_evidence.evidence_id),
        (referenced_evidence, unreferenced_literature),
    )

    assert packet is None


def test_explicit_full_detail_triggers_explanation_review() -> None:
    evidence = _evidence("population")

    packet = _build(
        _completed_decision(evidence.evidence_id),
        (evidence,),
        detail=ExplanationDetail.FULL,
    )

    assert packet is not None
    assert packet.task_type is ReviewTaskType.EXPLANATION_REVIEW
    assert packet.reason is ReviewReason.FULL_EXPLANATION_REQUESTED
    assert packet.classification_override_prohibited is True


def test_packet_input_hash_is_canonical_and_request_is_deeply_immutable() -> None:
    evidence = _evidence("population")
    request = {"nested": {"b": 2, "a": ["x"]}, "variant": "NM_007294.4:c.1A>G"}

    packet = _build(
        _completed_decision(evidence.evidence_id), (evidence,), request=request
    )

    assert packet is None
    recommendation = ReviewPacketBuilder().recommend(
        decision=_completed_decision(evidence.evidence_id),
        evidence_items=(evidence,),
        detail=ExplanationDetail.FULL,
    )
    assert recommendation is not None
    packet = ReviewPacketBuilder().build(
        classification_id="cls_123",
        decision=_completed_decision(evidence.evidence_id),
        evidence_items=(evidence,),
        request=request,
        snapshot_id="es_" + "b" * 64,
        ruleset_id="default-rules",
        ruleset_version="1.0.0",
        detail=ExplanationDetail.FULL,
    )
    assert packet is not None
    expected_hash = canonical_hash(
        {
            "request": {
                "variant": "NM_007294.4:c.1A>G",
                "nested": {"a": ["x"], "b": 2},
            },
            "snapshot_id": "es_" + "b" * 64,
            "ruleset": {"id": "default-rules", "version": "1.0.0"},
        }
    )
    assert packet.input_hash == expected_hash
    request["nested"]["a"].append("mutated")
    assert packet.request["nested"]["a"] == ("x",)
    with pytest.raises(TypeError):
        packet.request["other"] = "value"  # type: ignore[index]
    with pytest.raises(FrozenInstanceError):
        packet.token_cap = 1  # type: ignore[misc]


def test_packet_rejects_referenced_evidence_that_is_not_available() -> None:
    unknown_evidence_id = "ev_" + "f" * 64

    with pytest.raises(ValueError, match="unavailable evidence"):
        _build(_conflict_decision(unknown_evidence_id), ())


def test_compact_observation_excludes_raw_snapshot_reference_and_raw_content() -> None:
    evidence = _evidence("functional")

    packet = _build(_completed_decision(evidence.evidence_id), (evidence,))

    assert packet is not None
    observation = packet.observations[0]
    assert not hasattr(observation, "raw_snapshot_ref")
    assert not hasattr(observation, "raw_content")
    assert observation.evidence_id == evidence.evidence_id
    assert observation.observation == evidence.observation


def test_conflict_packet_accepts_missing_persisted_classification_id() -> None:
    evidence = _evidence("population")

    packet = ReviewPacketBuilder().build(
        classification_id=None,
        decision=_conflict_decision(evidence.evidence_id),
        evidence_items=(evidence,),
        request={"variant": "NM_007294.4:c.1A>G"},
        snapshot_id="es_" + "b" * 64,
        ruleset_id="default-rules",
        ruleset_version="1.0.0",
    )

    assert packet is not None
    assert packet.classification_id is None
