"""Contract tests for the source-controlled release support matrix."""

from __future__ import annotations

import asyncio
import json
import re
import tomllib
from pathlib import Path
from typing import Any

from acmg_classifier.presentation.cli.app import app as cli_app
from acmg_classifier.presentation.cli.app import data_app as cli_data_app
from acmg_classifier.presentation.mcp.server import create_server
from acmg_classifier.presentation.services import default_services

ROOT = Path(__file__).resolve().parents[2]
DOCS = ROOT / "docs"
MATRIX_PATH = DOCS / "release" / "capabilities.json"
README_PATH = ROOT / "README.md"
ONBOARDING_PATH = DOCS / "onboarding.md"
LIMITS_PATH = DOCS / "known-limits.md"
MIGRATION_PATH = DOCS / "migration.md"
SAFETY_PATH = DOCS / "safety-and-privacy.md"
SUPPORT_POLICY_PATH = DOCS / "support-policy.md"

_ALLOWED_STATUSES = {
    "experimental",
    "validated_for_research",
    "unavailable",
    "deprecated",
    "external_prerequisite",
}
_REQUIRED_GATE_BLOCKERS = {
    "10.2": "provenance-approved independently curated scientific case set",
    "10.4": "linux and windows",
    "10.5": "external security",
    "10.6": "live benchmark",
}


def _matrix() -> dict[str, Any]:
    return json.loads(MATRIX_PATH.read_text(encoding="utf-8"))


def _project() -> dict[str, Any]:
    project_path = ROOT / "pyproject.toml"
    return tomllib.loads(project_path.read_text(encoding="utf-8"))["project"]


def _markdown_links(path: Path) -> tuple[str, ...]:
    text = path.read_text(encoding="utf-8")
    return tuple(
        target
        for target in re.findall(r"(?<!!)]\(([^)#]+)(?:#[^)]+)?\)", text)
        if "://" not in target and not target.startswith("mailto:")
    )


def test_capability_matrix_matches_installed_package_and_mcp_surface() -> None:
    matrix = _matrix()
    project = _project()

    assert matrix["schema_version"] == "1.0"
    assert matrix["package"] == {
        "name": project["name"],
        "version": project["version"],
        "python_requires": project["requires-python"],
        "console_scripts": project["scripts"],
    }
    assert matrix["default_runtime"] == {
        "language": "python",
        "storage": "sqlite",
        "transport": "stdio",
        "requires_docker": False,
        "requires_postgresql": False,
        "requires_redis": False,
        "requires_api_keys_for_default_workflow": False,
    }

    server = create_server(default_services())
    actual_tools = {tool.name for tool in asyncio.run(server.list_tools())}
    documented_tools = set(matrix["interfaces"]["mcp"]["primary_tools"])
    assert documented_tools == {
        "classify_variant",
        "explain_classification",
        "submit_feedback",
    }
    assert documented_tools <= actual_tools
    assert matrix["interfaces"]["mcp"]["advanced_tools"] == ["get_raw_snapshot"]
    routine_resources = {
        resource.uriTemplate
        for resource in asyncio.run(server.list_resource_templates())
    }
    advanced_server = create_server(default_services(), advanced=True)
    advanced_resources = {
        resource.uriTemplate
        for resource in asyncio.run(advanced_server.list_resource_templates())
    }
    assert set(matrix["interfaces"]["mcp"]["routine_resources"]) == routine_resources
    assert set(matrix["interfaces"]["mcp"]["advanced_resources"]) == (
        advanced_resources - routine_resources
    )


def test_capability_matrix_matches_cli_command_metadata() -> None:
    matrix = _matrix()
    root_commands = {
        command.name or command.callback.__name__
        for command in cli_app.registered_commands
    }
    data_commands = {
        f"data {command.name or command.callback.__name__.removeprefix('data_')}"
        for command in cli_data_app.registered_commands
    }
    documented = set(matrix["interfaces"]["cli"]["commands"])
    assert documented == root_commands | data_commands


def test_capability_matrix_labels_every_claim_and_preserves_release_blockers() -> None:
    matrix = _matrix()
    assert set(matrix["status_vocabulary"]) == _ALLOWED_STATUSES

    for section in ("capabilities", "evidence_sources", "criteria"):
        for claim in matrix[section]:
            assert claim["status"] in _ALLOWED_STATUSES
            assert claim["validation_level"]
            assert claim["evidence"]
            assert claim["source_path"]
            if claim["status"] == "validated_for_research":
                assert claim["validation_level"] == "independent"

    gates = {gate["task_id"]: gate for gate in matrix["release_gates"]}
    assert set(gates) == {
        "10.1",
        "10.2",
        "10.3",
        "10.4",
        "10.5",
        "10.6",
    }
    for task_id, required_fragment in _REQUIRED_GATE_BLOCKERS.items():
        gate = gates[task_id]
        assert gate["status"] == "external_prerequisite"
        assert required_fragment in " ".join(gate["blockers"]).lower()

    golden = next(
        claim for claim in matrix["capabilities"] if claim["id"] == "golden-validation"
    )
    assert golden["status"] == "external_prerequisite"
    assert golden["dataset_kind"] == "synthetic_smoke"
    assert golden["release_eligible"] is False


def test_candidate_bundle_metadata_is_sourced_from_release_artifacts() -> None:
    matrix = _matrix()
    recipe = json.loads(
        (ROOT / "data_builder" / "recipes" / "core-2026.7.10.json").read_text(
            encoding="utf-8"
        )
    )
    release = json.loads(
        (ROOT / "data_builder" / "releases" / "core-2026.7.10.json").read_text(
            encoding="utf-8"
        )
    )
    bundle = matrix["candidate_bundle"]
    assert bundle["bundle_version"] == release["bundle_version"]
    assert bundle["archive_sha256"] == release["archive_sha256"]
    assert bundle["manifest_sha256"] == release["manifest_sha256"]
    assert bundle["sources"] == recipe["sources"]
    assert bundle["release_signature_status"] == "requires_controlled_release_key"
    assert bundle["scientific_release_gate"] == "pending_task_10.2"


def test_matrix_documents_all_legacy_contract_decisions() -> None:
    matrix = _matrix()
    manifest = json.loads(
        (
            ROOT
            / "src"
            / "acmg_classifier"
            / "presentation"
            / "compat"
            / "legacy_go_manifest.json"
        ).read_text(encoding="utf-8")
    )
    matrix_tools = {
        entry["tool"]: entry for entry in matrix["migration"]["legacy_tools"]
    }
    assert set(matrix_tools) == set(manifest["tools"])
    for tool, decision in manifest["tools"].items():
        documented = matrix_tools[tool]
        assert documented["decision"] == decision["decision"]
        assert documented["replacement"] == decision["replacement"]
        assert documented["status"] in {"deprecated", "experimental", "unavailable"}


def test_entry_docs_are_current_and_research_use_only() -> None:
    entry_docs = (
        README_PATH,
        ONBOARDING_PATH,
        LIMITS_PATH,
        MIGRATION_PATH,
        SAFETY_PATH,
        SUPPORT_POLICY_PATH,
        DOCS / "release" / "capabilities.md",
    )
    for path in entry_docs:
        text = path.read_text(encoding="utf-8").lower()
        assert "research" in text
        assert "not for clinical" in text

    quick_start = (
        README_PATH.read_text(encoding="utf-8")
        + ONBOARDING_PATH.read_text(encoding="utf-8")
    ).lower()
    assert "acmg-mcp" in quick_start
    assert "acmg classify" in quick_start
    assert "bundle_unavailable" in quick_start
    for obsolete_pattern in (
        r"\bgo (run|build|install)\b",
        r"\bmcp-server-lite\b",
        r"\bdocker(?:[- ]compose)?\s+(up|run)\b",
        r"\bpostgresql\s+(setup|required)\b",
        r"\bredis\s+(setup|required)\b",
        r"\bproduction[ -]ready\b",
        r"\bclinical[- ]grade\b",
        r"\bhipaa[- ]compliant\b",
        r"\btreatment recommendation\b",
    ):
        assert re.search(obsolete_pattern, quick_start) is None


def test_legacy_documentation_and_examples_exclude_obsolete_claims() -> None:
    paths = (
        DOCS / "README.md",
        DOCS / "user-guide.md",
        DOCS / "api-documentation.md",
        DOCS / "architecture.md",
        DOCS / "database.md",
        DOCS / "maintenance-troubleshooting.md",
        DOCS / "security-compliance.md",
        *sorted((ROOT / "examples" / "ai-agents").rglob("*.md")),
    )
    prohibited = (
        r"\bproduction[ -]ready\b",
        r"\bclinical[- ]grade\b",
        r"\bhipaa[- ]compliant\b",
        r"\bcap/clia\b",
        r"\bsoc ?2\b",
        r"\bdocker(?:[- ]compose)?\s+(up|run)\b",
        r"\bmcp-server-lite\b",
        r"\bgo (run|build|install)\b",
        r"\bpostgresql\s+(setup|required)\b",
        r"\bredis\s+(setup|required)\b",
    )
    for path in paths:
        text = path.read_text(encoding="utf-8").lower()
        for pattern in prohibited:
            assert re.search(pattern, text) is None, (path, pattern)


def test_release_documentation_internal_links_resolve() -> None:
    paths = (
        README_PATH,
        ROOT / "data_builder" / "README.md",
        *sorted(DOCS.rglob("*.md")),
        *sorted((ROOT / "examples" / "ai-agents").rglob("*.md")),
    )
    for path in paths:
        for target in _markdown_links(path):
            assert (path.parent / target).resolve().is_file(), (path, target)
