from __future__ import annotations

import json
from types import SimpleNamespace

import pytest
from mcp import types

from acmg_classifier.application.review import (
    MAX_REVIEW_OUTPUT_BYTES,
    ReviewAttemptStatus,
    ReviewOrchestrator,
    ReviewOutputSchema,
    ReviewReason,
    ReviewTaskType,
)
from acmg_classifier.domain.evidence import EvidenceDerivation, ObservationKind
from acmg_classifier.presentation.mcp.review import McpSamplingHostAgent


def _packet(*, request: object = None) -> SimpleNamespace:
    return SimpleNamespace(
        task_type=ReviewTaskType.EXPLANATION_REVIEW,
        reason=ReviewReason.FULL_EXPLANATION_REQUESTED,
        input_hash="hash_123",
        snapshot_id="snapshot_123",
        ruleset_id="rules",
        ruleset_version="1.0",
        classification_id="cls_123",
        selected_evidence_ids=("ev_123",),
        observations=(
            SimpleNamespace(
                evidence_id="ev_123",
                kind=ObservationKind.FUNCTIONAL,
                observation={"kind": "functional", "result": "abnormal"},
                derivation=EvidenceDerivation.SOURCE,
                source_id="literature",
                quality_flags=(),
            ),
        ),
        request=request if request is not None else {"variant": "NM_007294.4:c.1A>G"},
        token_cap=42,
        requested_output_schema=ReviewOutputSchema.EXPLANATION_SUGGESTIONS_V1,
    )


class FakeSession:
    def __init__(
        self, *, supported: bool = True, content: object | None = None
    ) -> None:
        self.supported = supported
        self.content = content or types.TextContent(type="text", text="{}")
        self.capability_requests: list[types.ClientCapabilities] = []
        self.requests: list[dict[str, object]] = []

    def check_client_capability(self, capability: types.ClientCapabilities) -> bool:
        self.capability_requests.append(capability)
        return self.supported

    async def create_message(
        self, messages: list[types.SamplingMessage], **kwargs: object
    ):
        self.requests.append({"messages": messages, **kwargs})
        return SimpleNamespace(content=self.content, model="fake-model")


class FakeReviewStore:
    def __init__(self) -> None:
        self.appended: list[tuple[str, dict[str, object]]] = []

    def append_review(self, classification_id: str, review: dict[str, object]) -> str:
        self.appended.append((classification_id, review))
        return "review_123"


@pytest.mark.asyncio
async def test_adapter_detects_sampling_and_sends_minimal_text_request() -> None:
    session = FakeSession()
    context = SimpleNamespace(session=session, request_id="request_123")
    adapter = McpSamplingHostAgent(context)

    capabilities = await adapter.get_capabilities()
    response = await adapter.request_review(_packet())

    assert capabilities.sampling is True
    assert response.content == b"{}"
    assert len(session.capability_requests) == 1
    assert session.capability_requests[0].sampling is not None
    request = session.requests[0]
    assert request["max_tokens"] == 42
    assert request["related_request_id"] == "request_123"
    assert "include_context" not in request
    message = request["messages"][0]
    assert isinstance(message.content, types.TextContent)
    assert '"evidence_ids":["ev_123"]' in message.content.text


@pytest.mark.asyncio
async def test_adapter_drives_application_orchestrator_without_server_wiring() -> None:
    packet = _packet()
    session = FakeSession(
        content=types.TextContent(
            type="text",
            text=json.dumps(
                {
                    "schema_version": "1.0",
                    "task_type": packet.task_type.value,
                    "input_hash": packet.input_hash,
                    "evidence_ids": list(packet.selected_evidence_ids),
                    "requested_output_schema": "review.explanation-suggestions.v1",
                    "status": "completed",
                    "reviewer": "specialist-1",
                    "model": "fake-model",
                    "timestamp": "2026-07-11T12:00:00Z",
                    "output": {
                        "conclusion": "No deterministic change proposed.",
                        "uncertainty": "Only selected evidence was reviewed.",
                        "questions": [],
                        "limitations": [],
                    },
                }
            ),
        )
    )
    store = FakeReviewStore()
    adapter = McpSamplingHostAgent(
        SimpleNamespace(session=session, request_id="request_123")
    )

    attempt = await ReviewOrchestrator(host=adapter, store=store).orchestrate(packet)

    assert attempt.status is ReviewAttemptStatus.STORED
    assert len(store.appended) == 1
    assert store.appended[0][1]["model"] == "fake-model"


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "content",
    [
        types.ImageContent(type="image", data="aGVsbG8=", mimeType="image/png"),
        [
            types.TextContent(type="text", text="{}"),
            types.ImageContent(type="image", data="aGVsbG8=", mimeType="image/png"),
        ],
    ],
)
async def test_adapter_rejects_non_text_or_multimodal_sampling_response(
    content: object,
) -> None:
    session = FakeSession(content=content)
    adapter = McpSamplingHostAgent(SimpleNamespace(session=session, request_id=None))

    with pytest.raises(ValueError, match="text"):
        await adapter.request_review(_packet())


@pytest.mark.asyncio
async def test_adapter_rejects_oversized_sampling_response_and_input() -> None:
    session = FakeSession(
        content=types.TextContent(type="text", text="x" * (MAX_REVIEW_OUTPUT_BYTES + 1))
    )
    adapter = McpSamplingHostAgent(SimpleNamespace(session=session, request_id=None))

    with pytest.raises(ValueError, match="maximum byte size"):
        await adapter.request_review(_packet())

    with pytest.raises(ValueError, match="input"):
        await adapter.request_review(_packet(request={"large": "x" * (32 * 1024)}))
