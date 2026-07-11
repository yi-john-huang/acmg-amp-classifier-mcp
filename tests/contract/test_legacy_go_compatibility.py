"""Contract fixtures for the controlled legacy Go migration boundary."""

from __future__ import annotations

import json
from pathlib import Path
from typing import Any

import pytest

from acmg_classifier.presentation.compat import LegacyGoCompatibilityAdapter

_FIXTURE_PATH = Path(__file__).parents[1] / "fixtures" / "legacy_go" / "contracts.json"
_REQUIRED_ENVELOPE_FIELDS = {
    "schema_version",
    "status",
    "error_code",
    "legacy_tool",
    "compatibility_decision",
    "replacement",
    "limitations",
}


def _contracts() -> dict[str, Any]:
    return json.loads(_FIXTURE_PATH.read_text(encoding="utf-8"))


@pytest.mark.parametrize("case", _contracts()["cases"], ids=lambda case: case["name"])
def test_legacy_go_contract_decisions_are_stable(case: dict[str, Any]) -> None:
    """Every captured legacy request gets a truthful migration envelope."""
    envelope = LegacyGoCompatibilityAdapter().adapt(
        case["legacy_tool"], case["request"]
    )
    expected = case["expected"]

    assert envelope.keys() >= _REQUIRED_ENVELOPE_FIELDS
    assert envelope["schema_version"] == "1.0"
    assert envelope["legacy_tool"] == case["legacy_tool"]
    assert envelope["status"] == expected["status"]
    assert envelope["error_code"] == expected["error_code"]
    assert envelope["compatibility_decision"] == expected["compatibility_decision"]
    assert envelope["replacement"]["tool"] == expected["replacement_tool"]
    if "mapped_request" in expected:
        assert envelope["mapped_request"] == expected["mapped_request"]
    else:
        assert "mapped_request" not in envelope


@pytest.mark.parametrize(
    "legacy_tool",
    _contracts()["legacy_public_tools"],
)
def test_manifest_captures_every_legacy_public_tool(legacy_tool: str) -> None:
    """The manifest decides every known public Go tool without MCP registration."""
    adapter = LegacyGoCompatibilityAdapter()

    assert legacy_tool in adapter.public_legacy_tools
    assert legacy_tool in adapter.manifest["tools"]
    assert adapter.manifest["tools"][legacy_tool]["replacement"]["tool"] in {
        "classify_variant",
        "explain_classification",
        "submit_feedback",
    }


def test_compatibility_never_preserves_fabricated_legacy_content() -> None:
    """Migration errors cannot carry fixed VUS, mock evidence, or recommendations."""
    envelope = LegacyGoCompatibilityAdapter().adapt(
        "classify_variant",
        {
            "hgvs_notation": "NM_000001.1:c.100A>G",
            "mock_evidence": {"clinvar": "Pathogenic"},
            "classification": "VUS",
            "recommendations": ["Refer patient for clinical management"],
        },
    )
    serialized = json.dumps(envelope, sort_keys=True).lower()

    assert envelope["status"] == "rejected"
    assert "classification" not in envelope
    assert "evidence" not in envelope
    assert "recommendations" not in envelope
    assert "vus" not in serialized
    assert "pathogenic" not in serialized
    assert "clinical management" not in serialized


def test_phi_like_report_fields_are_rejected_without_echoing_nested_content() -> None:
    """Nested and camel-case report identifiers cannot leak through an envelope."""
    envelope = LegacyGoCompatibilityAdapter().adapt(
        "generate_report",
        {"clinical_context": {"medicalRecordNumber": "MRN-12345"}},
    )

    assert envelope["error_code"] == "PRIVACY_REJECTED"
    assert "MRN-12345" not in json.dumps(envelope)


def test_adapter_does_not_register_or_change_the_primary_tool_surface() -> None:
    """Compatibility is opt-in data translation, not an MCP tool registration path."""
    adapter = LegacyGoCompatibilityAdapter()

    assert adapter.primary_public_tools == (
        "classify_variant",
        "explain_classification",
        "submit_feedback",
    )
    assert not hasattr(adapter, "register")
    assert not hasattr(adapter, "register_tool")
    assert not hasattr(adapter, "server")
