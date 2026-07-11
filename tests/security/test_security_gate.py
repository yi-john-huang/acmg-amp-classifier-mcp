from __future__ import annotations

import json
from pathlib import Path

import pytest

from acmg_classifier.application.explanation import (
    ExplanationDetail,
    ExplanationService,
)
from acmg_classifier.application.redaction import (
    REDACTED,
    TRUNCATED,
    ResourceTooLargeError,
    enforce_resource_size,
    redact_for_report,
)
from acmg_classifier.domain.combination import ClassificationDecision
from acmg_classifier.domain.criteria import CriterionAssessment
from acmg_classifier.domain.enums import (
    ClassificationTier,
    CriterionStatus,
)
from acmg_classifier.domain.rules import CriterionCode, CriterionStrength
from acmg_classifier.presentation.mcp.server import _raw_snapshot_content
from acmg_classifier.presentation.serialization import stored_explanation_content
from acmg_classifier.presentation.services import PresentationServices


class _OversizedResources:
    def get_raw_snapshot(self, raw_snapshot_ref: str) -> bytes:
        return b"x" * (4 * 1024 * 1024 + 1)


def test_recursive_report_redaction_removes_credentials_and_phi_without_mutation() -> (
    None
):
    opaque_value = "api" + "-key-" + "demo" + "-token"
    auth_header = "Bearer " + opaque_value
    patient_name = "Test" + " Person"
    patient_email = "person" + "@example.test"
    patient_identifier = "MRN" + "-000123"
    credential_url = (
        "https://"
        + "researcher"
        + ":"
        + "demo-password"
        + "@api.example.test/v1?access_token="
        + opaque_value
    )
    original = {
        "headers": {"Authorization": auth_header, "X-Api-Key": opaque_value},
        "endpoint": credential_url,
        "case": {
            "patient_name": patient_name,
            "email": patient_email,
            "mrn": patient_identifier,
        },
    }

    redacted = redact_for_report(original)
    rendered = json.dumps(redacted, sort_keys=True)

    for sensitive_value in (
        opaque_value,
        auth_header,
        patient_name,
        patient_email,
        patient_identifier,
        credential_url,
    ):
        assert sensitive_value not in rendered
    assert redacted["headers"]["Authorization"] == REDACTED
    assert redacted["headers"]["X-Api-Key"] == REDACTED
    assert redacted["endpoint"] == REDACTED
    assert redacted["case"]["patient_name"] == REDACTED
    assert original["headers"]["Authorization"] == auth_header


def test_redaction_canonicalizes_camel_case_credential_keys() -> None:
    opaque_value = "camel" + "-case-" + "opaque-value"
    credential_url = "https://api.example.test/v1?apiKey=" + opaque_value

    report = redact_for_report(
        {
            "apiKey": opaque_value,
            "endpoint": credential_url,
        }
    )

    assert report == {"apiKey": REDACTED, "endpoint": REDACTED}
    assert opaque_value not in json.dumps(report, sort_keys=True)


def test_redaction_preserves_non_credential_workflow_identifiers() -> None:
    report = redact_for_report({"resume_token": "resume-identifier", "token_cap": 256})

    assert report == {"resume_token": "resume-identifier", "token_cap": 256}


def test_unparseable_url_is_safely_redacted_instead_of_raising() -> None:
    report = redact_for_report({"endpoint": "https://[not-a-valid-host"})

    assert report == {"endpoint": REDACTED}


def test_report_redaction_is_recursively_bounded_and_safe_for_untrusted_objects() -> (
    None
):
    list_report = redact_for_report(
        {"events": ["abcdefghij", "ignored"]},
        max_depth=2,
        max_items=1,
        max_string_chars=8,
    )
    nested_report = redact_for_report(
        {"nested": {"level": {"deeper": "hidden"}}, "opaque": object()},
        max_depth=2,
        max_items=2,
        max_string_chars=8,
    )

    assert list_report["events"] == ["abcdefgh" + TRUNCATED, TRUNCATED]
    assert nested_report["nested"]["level"] == TRUNCATED
    assert nested_report["opaque"] == TRUNCATED
    assert json.dumps(list_report)
    assert json.dumps(nested_report)


def test_untrusted_resource_size_failure_does_not_echo_content() -> None:
    marker = b"raw-resource-marker"

    with pytest.raises(ResourceTooLargeError) as error:
        enforce_resource_size(marker, max_bytes=len(marker) - 1)

    assert "raw-resource-marker" not in str(error.value)


def test_mcp_raw_snapshot_size_failure_omits_raw_content() -> None:
    services = PresentationServices(
        classifier=object(),  # type: ignore[arg-type]
        bootstrap=object(),  # type: ignore[arg-type]
        replay=object(),  # type: ignore[arg-type]
        resources=_OversizedResources(),
    )

    response = _raw_snapshot_content(services, "sha256:" + "a" * 64)

    assert response["schema_version"] == "1.0"
    assert response["status"] == "failed"
    assert response["error_code"] == "RAW_SNAPSHOT_TOO_LARGE"
    assert "content" not in response


def test_explanation_is_research_only_and_does_not_offer_treatment_advice() -> None:
    decision = ClassificationDecision(
        algorithm_id="acmg-2015",
        algorithm_version="1.0.0",
        classification=ClassificationTier.UNCERTAIN_SIGNIFICANCE,
        matched_rule_id="vus_default",
    )

    explanation = ExplanationService().render_decision(decision)
    content = json.dumps(explanation.to_canonical_content()).lower()

    assert "research use only" in content
    assert "not for clinical diagnosis" in content
    assert "treatment" not in content


def test_stored_explanation_redacts_credential_text_at_shared_report_boundary() -> None:
    opaque_value = "api" + "-key-" + "demo" + "-token"
    decision = ClassificationDecision(
        algorithm_id="acmg-2015",
        algorithm_version="1.0.0",
        classification=ClassificationTier.UNCERTAIN_SIGNIFICANCE,
        matched_rule_id="vus_default",
        assessments=(
            CriterionAssessment(
                code=CriterionCode.PM2,
                status=CriterionStatus.APPLIED,
                original_strength=CriterionStrength.MODERATE,
                applied_strength=CriterionStrength.MODERATE,
                evidence_ids=("ev_" + "a" * 64,),
                rationale_template="source detail={detail}",
                rationale_values={"detail": "Bearer " + opaque_value},
                ruleset_id="test",
                ruleset_version="1.0.0",
                evaluator_id="security-test",
                evaluator_version="1.0.0",
            ),
        ),
    )

    explanation = stored_explanation_content(
        {"decision": decision.to_canonical_content()},
        detail=ExplanationDetail.STANDARD,
    )
    rendered = json.dumps(explanation, sort_keys=True)

    assert opaque_value not in rendered
    assert explanation["classification"] == "Uncertain Significance"
    blocks = explanation["blocks"]
    assert isinstance(blocks, list)
    assert blocks[1]["text"] == REDACTED


def test_security_workflow_uses_locked_audit_and_fail_closed_secret_scan() -> None:
    root = Path(__file__).resolve().parents[2]
    workflow = (root / ".github" / "workflows" / "security.yml").read_text(
        encoding="utf-8"
    )
    baseline = json.loads((root / ".secrets.baseline").read_text(encoding="utf-8"))

    assert "actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683" in workflow
    assert "actions/setup-python@a26af69be951a213d495a4c3e4e4022e16d87065" in workflow
    assert "uv export --locked" in workflow
    assert "pip-audit==2.9.0" in workflow
    assert "detect-secrets==1.5.0" in workflow
    assert "detect-secrets-hook" in workflow
    assert "--baseline .secrets.baseline" in workflow
    assert "git ls-files -z" in workflow
    assert "branches:" in workflow
    assert "      - develop" in workflow
    assert "gitleaks/gitleaks-action@ff98106e4c7b2bc287b24eaf42907196329070c7" in workflow
    assert "contents: read" in workflow
    assert baseline["version"] == "1.5.0"
    assert baseline["results"]["data_builder/recipes/core-2026.7.10.json"]
    findings = [
        finding for entries in baseline["results"].values() for finding in entries
    ]
    assert findings
    assert all((root / filename).is_file() for filename in baseline["results"])
    assert all(
        {"filename", "hashed_secret", "is_verified"} <= finding.keys()
        and finding["is_verified"] is False
        for finding in findings
    )
