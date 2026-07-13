from __future__ import annotations

from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]


def _workflow(name: str) -> str:
    return (ROOT / ".github" / "workflows" / name).read_text(encoding="utf-8")


def test_release_workflow_guards_before_any_publication() -> None:
    workflow = _workflow("release.yml")
    guard = "python scripts/check_release_readiness.py --format json"

    assert "workflow_dispatch:" in workflow
    assert "tags:" in workflow
    assert "v*" in workflow
    assert "permissions:" in workflow
    assert "contents: read" in workflow
    assert guard in workflow
    assert workflow.index(guard) < workflow.index("No publication")
    assert "actions/checkout@" in workflow
    assert "actions/setup-python@" in workflow
    assert "--force" not in workflow
    assert "twine upload" not in workflow
    assert "gh release create" not in workflow


def test_platform_smoke_workflow_covers_supported_platforms_and_runtimes() -> None:
    workflow = _workflow("platform-smoke.yml")

    assert "pull_request:" in workflow
    assert "workflow_dispatch:" in workflow
    for operating_system in ("ubuntu-24.04", "windows-2025", "macos-14"):
        assert operating_system in workflow
    for python_version in ('"3.12"', '"3.13"'):
        assert python_version in workflow
    assert "uv sync --frozen --all-groups" in workflow
    assert "tests/packaging/test_package_smoke.py" in workflow
    assert "contents: read" in workflow
    assert "actions/checkout@" in workflow
    assert "astral-sh/setup-uv@" in workflow


def test_benchmark_workflow_retains_explicitly_synthetic_report() -> None:
    workflow = _workflow("benchmark-evidence.yml")

    assert "workflow_dispatch:" in workflow
    assert "scripts/run_benchmark_evidence.py" in workflow
    assert "--output" in workflow
    assert (
        "actions/upload-artifact@ea165f8d65b6e75b540449e92b4886f43607fa02"
        in workflow
    )
    assert "retention-days:" in workflow
    assert "contents: read" in workflow
    assert "live performance" in workflow.lower()
