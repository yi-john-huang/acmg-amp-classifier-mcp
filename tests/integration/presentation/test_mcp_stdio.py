from __future__ import annotations

from pathlib import Path

import pytest
from mcp.client.session import ClientSession
from mcp.client.stdio import StdioServerParameters, stdio_client


@pytest.mark.asyncio
async def test_stdio_server_composes_actionable_bundle_failure() -> None:
    project_root = Path(__file__).parents[3]
    parameters = StdioServerParameters(
        command="uv",
        args=["run", "acmg-mcp"],
        cwd=project_root,
    )

    async with (
        stdio_client(parameters) as (read_stream, write_stream),
        ClientSession(read_stream, write_stream) as session,
    ):
        await session.initialize()
        tools = await session.list_tools()
        assert {tool.name for tool in tools.tools} >= {
            "classify_variant",
            "explain_classification",
            "submit_feedback",
        }

        result = await session.call_tool(
            "classify_variant",
            {"variant": "NC_000001.11:g.101A>G", "interactive": False},
        )

    assert result.isError is False
    assert result.structuredContent == {
        "schema_version": "1.0",
        "status": "failed",
        "classification": None,
        "error_code": "BUNDLE_UNAVAILABLE",
        "limitations": [
            "No compatible data bundle is available",
            "no bundle candidates are available",
            "acmg doctor --repair",
        ],
    }
