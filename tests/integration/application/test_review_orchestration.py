from __future__ import annotations

import asyncio
import json
from datetime import UTC, datetime
from typing import Any

import pytest

from acmg_classifier.application.explanation import ExplanationDetail
from acmg_classifier.application.review import (
    HostAgentCapabilities,
    HostAgentResponse,
    ReviewAttemptStatus,
    ReviewOrchestrator,
    ReviewPacketBuilder,
)
from acmg_classifier.domain.combination import ClassificationDecision
from acmg_classifier.domain.criteria import CriterionAssessment
from acmg_classifier.domain.enums import ClassificationTier, CriterionStatus
from acmg_classifier.domain.evidence import EvidenceItem
from acmg_classifier.domain.rules import CriterionCode, CriterionStrength


class FakeReviewStore:
    def __init__(self) -> None:
        self.appended: list[tuple[str, dict[str, object]]] = []

    def append_review(self, classification_id: str, review: dict[str, object]) -> str:
        self.appended.append((classification_id, review))
        return "review_123"


class FakeHostAgent:
    def __init__(
        self,
        response: HostAgentResponse | Exception | None = None,
        *,
        sampling: bool = True,
    ) -> None:
        self.response = response
        self.capabilities = HostAgentCapabilities(sampling=sampling)
        self.capability_calls = 0
        self.review_calls = 0

    async def get_capabilities(self) -> HostAgentCapabilities:
        self.capability_calls += 1
        return self.capabilities

    async def request_review(self, packet: object) -> HostAgentResponse:
        self.review_calls += 1
        if isinstance(self.response, Exception):
            raise self.response
        assert self.response is not None
        return self.response


def _evidence() -> EvidenceItem:
    return EvidenceItem.model_validate(
        {
            "variant_key": "ga4gh:VA.example",
            "kind": "functional",
            "observation": {
                "kind": "functional",
                "assay_id": "PMID:12345678:assay",
                "assay_type": "DNA repair",
                "result": "abnormal",
                "validation_status": "validated",
            },
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


def _packet():
    evidence = _evidence()
    decision = ClassificationDecision(
        algorithm_id="acmg_2015",
        algorithm_version="1.0.0",
        classification=ClassificationTier.PATHOGENIC,
        matched_rule_id="test-rule",
        assessments=(
            CriterionAssessment(
                code=CriterionCode.PS3,
                status=CriterionStatus.APPLIED,
                original_strength=CriterionStrength.STRONG,
                applied_strength=CriterionStrength.STRONG,
                evidence_ids=(evidence.evidence_id,),
                rationale_template="PS3 rationale",
                rationale_values={},
                ruleset_id="default-rules",
                ruleset_version="1.0.0",
                evaluator_id="PS3",
                evaluator_version="1.0.0",
            ),
        ),
    )
    packet = ReviewPacketBuilder().build(
        classification_id="cls_123",
        decision=decision,
        evidence_items=(evidence,),
        request={"variant": "NM_007294.4:c.1A>G"},
        snapshot_id="es_" + "b" * 64,
        ruleset_id="default-rules",
        ruleset_version="1.0.0",
        detail=ExplanationDetail.FULL,
    )
    assert packet is not None
    return packet


def _response(
    packet: Any, payload: dict[str, object] | None = None
) -> HostAgentResponse:
    return HostAgentResponse(
        content=json.dumps(
            payload
            or {
                "schema_version": "1.0",
                "task_type": packet.task_type.value,
                "input_hash": packet.input_hash,
                "evidence_ids": list(packet.selected_evidence_ids),
                "requested_output_schema": packet.requested_output_schema.value,
                "status": "completed",
                "reviewer": "specialist-1",
                "model": "test-model",
                "timestamp": "2026-07-11T12:00:00Z",
                "output": {
                    "conclusion": "Evidence is internally consistent.",
                    "uncertainty": "Limited to the selected evidence subset.",
                    "questions": [],
                    "limitations": [],
                },
            }
        ).encode(),
    )


@pytest.mark.asyncio
async def test_review_output_requires_typed_conclusion_and_uncertainty() -> None:
    packet = _packet()
    response = _response(packet)
    payload = json.loads(response.content)
    payload["output"] = {"summary": "Untyped review output."}
    host = FakeHostAgent(_response(packet, payload))
    store = FakeReviewStore()

    attempt = await ReviewOrchestrator(host=host, store=store).orchestrate(packet)

    assert attempt.status is ReviewAttemptStatus.INVALID_OUTPUT
    assert store.appended == []


@pytest.mark.asyncio
async def test_absent_recommendation_does_not_call_host_or_store() -> None:
    host = FakeHostAgent()
    store = FakeReviewStore()

    attempt = await ReviewOrchestrator(host=host, store=store).orchestrate(None)

    assert attempt.status is ReviewAttemptStatus.NOT_RECOMMENDED
    assert host.capability_calls == 0
    assert host.review_calls == 0
    assert store.appended == []


@pytest.mark.asyncio
@pytest.mark.parametrize("host", [None, FakeHostAgent(sampling=False)])
async def test_disabled_or_unsupported_host_skips_without_validation_or_storage(
    host: Any,
) -> None:
    store = FakeReviewStore()

    attempt = await ReviewOrchestrator(host=host, store=store).orchestrate(_packet())

    expected = (
        ReviewAttemptStatus.DISABLED
        if host is None
        else ReviewAttemptStatus.UNSUPPORTED
    )
    assert attempt.status is expected
    assert host is None or host.review_calls == 0
    assert store.appended == []


@pytest.mark.asyncio
async def test_valid_review_is_bounded_typed_and_appended_once() -> None:
    packet = _packet()
    host = FakeHostAgent(_response(packet))
    store = FakeReviewStore()

    attempt = await ReviewOrchestrator(host=host, store=store).orchestrate(packet)

    assert attempt.status is ReviewAttemptStatus.STORED
    assert attempt.review is not None
    assert attempt.review.classification_id == "cls_123"
    assert attempt.review.input_hash == packet.input_hash
    assert attempt.review.evidence_ids == packet.selected_evidence_ids
    assert attempt.review.task_type is packet.task_type
    assert attempt.review.status == "completed"
    assert attempt.review.reviewer == "specialist-1"
    assert attempt.review.model == "test-model"
    assert attempt.review.timestamp.isoformat() == "2026-07-11T12:00:00+00:00"
    assert host.review_calls == 1
    assert attempt.review_id == "review_123"
    assert len(store.appended) == 1
    classification_id, artifact = store.appended[0]
    assert classification_id == "cls_123"
    assert artifact == {
        "schema_version": "1.0",
        "classification_id": "cls_123",
        "task_type": packet.task_type.value,
        "reason": packet.reason.value,
        "input_hash": packet.input_hash,
        "evidence_ids": list(packet.selected_evidence_ids),
        "status": "completed",
        "requested_output_schema": packet.requested_output_schema.value,
        "reviewer": "specialist-1",
        "model": "test-model",
        "timestamp": "2026-07-11T12:00:00+00:00",
        "output": {
            "conclusion": "Evidence is internally consistent.",
            "uncertainty": "Limited to the selected evidence subset.",
            "questions": [],
            "limitations": [],
        },
    }


@pytest.mark.asyncio
async def test_host_timeout_is_stable_and_not_appended() -> None:
    host = FakeHostAgent(TimeoutError("sampling timed out"))
    store = FakeReviewStore()

    attempt = await ReviewOrchestrator(host=host, store=store).orchestrate(_packet())

    assert attempt.status is ReviewAttemptStatus.TIMED_OUT
    assert attempt.review is None
    assert store.appended == []


@pytest.mark.asyncio
async def test_review_request_timeout_is_bounded_and_not_appended() -> None:
    class HangingHost(FakeHostAgent):
        async def request_review(self, packet: object) -> HostAgentResponse:
            self.review_calls += 1
            await asyncio.Event().wait()
            raise AssertionError("unreachable")

    host = HangingHost()
    store = FakeReviewStore()

    attempt = await ReviewOrchestrator(
        host=host,
        store=store,
        timeout_seconds=0.01,
    ).orchestrate(_packet())

    assert attempt.status is ReviewAttemptStatus.TIMED_OUT
    assert host.review_calls == 1
    assert store.appended == []


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "field,value",
    [
        ("task_type", "evidence_conflict"),
        ("input_hash", "different_input_hash"),
    ],
)
async def test_task_and_input_hash_mismatches_are_invalid_and_not_appended(
    field: str, value: str
) -> None:
    packet = _packet()
    response = _response(packet)
    payload = json.loads(response.content)
    payload[field] = value
    host = FakeHostAgent(_response(packet, payload))
    store = FakeReviewStore()

    attempt = await ReviewOrchestrator(host=host, store=store).orchestrate(packet)

    assert attempt.status is ReviewAttemptStatus.INVALID_OUTPUT
    assert attempt.review is None
    assert store.appended == []


@pytest.mark.asyncio
async def test_unknown_evidence_id_is_invalid_and_not_appended() -> None:
    packet = _packet()
    response = _response(packet)
    payload = json.loads(response.content)
    payload["evidence_ids"] = ["ev_unknown"]
    host = FakeHostAgent(_response(packet, payload))
    store = FakeReviewStore()

    attempt = await ReviewOrchestrator(host=host, store=store).orchestrate(packet)

    assert attempt.status is ReviewAttemptStatus.INVALID_OUTPUT
    assert attempt.review is None
    assert store.appended == []


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "override",
    [
        {"classification": "benign"},
        {"output": {"nested": {"criteria": [{"status": "applied"}]}}},
    ],
)
async def test_override_attempts_are_invalid_and_not_appended(
    override: dict[str, object],
) -> None:
    packet = _packet()
    response = _response(packet)
    payload = json.loads(response.content)
    payload.update(override)
    host = FakeHostAgent(_response(packet, payload))
    store = FakeReviewStore()

    attempt = await ReviewOrchestrator(host=host, store=store).orchestrate(packet)

    assert attempt.status is ReviewAttemptStatus.INVALID_OUTPUT
    assert attempt.review is None
    assert store.appended == []


@pytest.mark.asyncio
async def test_oversized_output_is_rejected_before_json_parsing_or_storage() -> None:
    packet = _packet()
    host = FakeHostAgent(HostAgentResponse(content=b"{" + b"x" * (32 * 1024)))
    store = FakeReviewStore()

    attempt = await ReviewOrchestrator(host=host, store=store).orchestrate(packet)

    assert attempt.status is ReviewAttemptStatus.INVALID_OUTPUT
    assert attempt.review is None
    assert store.appended == []
