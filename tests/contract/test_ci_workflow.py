from __future__ import annotations

from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
WORKFLOW = ROOT / ".github" / "workflows" / "ci.yml"


def test_ci_workflow_exposes_the_locked_python_quality_gate() -> None:
    workflow = WORKFLOW.read_text(encoding="utf-8")

    assert "pull_request:" in workflow
    assert "      - develop" in workflow
    assert "      - master" in workflow
    assert 'python-version: ["3.12", "3.13"]' in workflow
    assert "permissions:\n  contents: read" in workflow

    for command in (
        "uv sync --frozen --all-groups",
        "uv run --frozen --no-sync pytest",
        "uv run --frozen --no-sync ruff check src tests",
        "uv run --frozen --no-sync mypy",
        "uv lock --check",
        "uv build",
    ):
        assert command in workflow
