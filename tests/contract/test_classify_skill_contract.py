from __future__ import annotations

import json
import re
from pathlib import Path

import pytest

from acmg_classifier.presentation.mcp.server import create_server


@pytest.mark.asyncio
async def test_classify_skill_uses_only_the_primary_mcp_workflow_contract() -> None:
    skill_path = Path(".claude/skills/classify/SKILL.md")
    text = skill_path.read_text(encoding="utf-8")
    server = create_server()
    tools = await server.list_tools()
    classify_schema = next(
        tool.inputSchema for tool in tools if tool.name == "classify_variant"
    )

    assert len(text.split()) <= 500
    assert "classify_variant" in text
    assert '"variant"' in text
    assert '"resume_token"' in text
    assert '"answers"' in text
    assert {"variant", "resume_token", "answers"} <= set(classify_schema["properties"])
    request_block = re.search(r"```json\n(\{.*?\})\n```", text, flags=re.DOTALL)
    assert request_block is not None
    assert json.loads(request_block.group(1)) == {
        "variant": "<HGVS or supported notation>",
        "interactive": True,
    }
    for legacy_or_low_level_tool in (
        "validate_hgvs",
        "query_evidence",
        "apply_rule",
        "combine_evidence",
        "generate_report",
    ):
        assert legacy_or_low_level_tool not in text
