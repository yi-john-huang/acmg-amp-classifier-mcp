"""Thin MCP sampling adapter for optional, non-authoritative review."""

from __future__ import annotations

import json
from collections.abc import Mapping
from enum import Enum
from typing import Any

from mcp import types
from mcp.server.fastmcp import Context

from acmg_classifier.application.review import (
    MAX_REVIEW_OUTPUT_BYTES,
    HostAgentCapabilities,
    HostAgentResponse,
    ReviewPacket,
)

MAX_REVIEW_INPUT_BYTES = 32 * 1024


class McpSamplingHostAgent:
    """Adapt one MCP request context to the SDK-free host-agent application port."""

    def __init__(self, context: Context[Any, Any, Any]) -> None:
        self._context = context

    async def get_capabilities(self) -> HostAgentCapabilities:
        """Query sampling support without validating or requesting review content."""
        supported = self._context.session.check_client_capability(
            types.ClientCapabilities(sampling=types.SamplingCapability())
        )
        return HostAgentCapabilities(sampling=supported)

    async def request_review(self, packet: ReviewPacket) -> HostAgentResponse:
        """Request a single text-only response constrained by the packet's token cap."""
        prompt = _sampling_prompt(packet)
        if len(prompt.encode("utf-8")) > MAX_REVIEW_INPUT_BYTES:
            raise ValueError("review sampling input exceeds maximum byte size")
        result = await self._context.session.create_message(
            [
                types.SamplingMessage(
                    role="user",
                    content=types.TextContent(type="text", text=prompt),
                )
            ],
            max_tokens=packet.token_cap,
            system_prompt=(
                "You are a non-authoritative ACMG/AMP specialist reviewer. "
                "Return only the requested JSON review; do not change evidence, "
                "criteria, decisions, or classifications."
            ),
            temperature=0.0,
            related_request_id=self._context.request_id,
        )
        if not isinstance(result.content, types.TextContent):
            raise ValueError("review sampling response must be plain text")
        content = result.content.text.encode("utf-8")
        if len(content) > MAX_REVIEW_OUTPUT_BYTES:
            raise ValueError("review sampling response exceeds maximum byte size")
        return HostAgentResponse(content=content)


def _sampling_prompt(packet: ReviewPacket) -> str:
    """Serialize only the packet's selected, already-sanitized review context."""
    request = {
        "task_type": packet.task_type.value,
        "reason": packet.reason.value,
        "input_hash": packet.input_hash,
        "snapshot_id": packet.snapshot_id,
        "ruleset": {"id": packet.ruleset_id, "version": packet.ruleset_version},
        "evidence_ids": list(packet.selected_evidence_ids),
        "request": _json_value(packet.request),
        "observations": [
            {
                "evidence_id": observation.evidence_id,
                "kind": _json_value(observation.kind),
                "observation": _json_value(observation.observation),
                "derivation": _json_value(observation.derivation),
                "source_id": observation.source_id,
                "quality_flags": _json_value(observation.quality_flags),
            }
            for observation in packet.observations
        ],
        "required_response": {
            "schema_version": "1.0",
            "task_type": packet.task_type.value,
            "input_hash": packet.input_hash,
            "evidence_ids": list(packet.selected_evidence_ids),
            "requested_output_schema": packet.requested_output_schema.value,
            "status": "completed",
            "reviewer": "non-empty identifier",
            "model": "non-empty identifier",
            "timestamp": "ISO-8601 timestamp with offset",
            "output": {
                "conclusion": "non-empty synthesis text",
                "uncertainty": "non-empty uncertainty text",
                "questions": ["optional non-empty question"],
                "limitations": ["optional non-empty limitation"],
            },
        },
    }
    return json.dumps(request, sort_keys=True, separators=(",", ":"), ensure_ascii=True)


def _json_value(value: object) -> object:
    if isinstance(value, Mapping):
        return {str(key): _json_value(item) for key, item in value.items()}
    if isinstance(value, (tuple, list)):
        return [_json_value(item) for item in value]
    if isinstance(value, Enum):
        return value.value
    model_dump = getattr(value, "model_dump", None)
    if callable(model_dump):
        return _json_value(model_dump(mode="json"))
    return value
