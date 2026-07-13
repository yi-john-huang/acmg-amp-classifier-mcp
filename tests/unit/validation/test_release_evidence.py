from __future__ import annotations

import json
import subprocess
import sys
from copy import deepcopy
from datetime import UTC, datetime
from pathlib import Path

from acmg_classifier.validation.release_evidence import (
    EvidenceKind,
    EvidenceValidationStatus,
    validate_evidence_manifest,
)

_SHA = "a" * 64


def _review(*, independent: bool = True) -> dict[str, object]:
    return {
        "status": "approved",
        "reviewed_by": "reviewer@example.org",
        "reviewed_at": "2026-07-13T12:00:00Z",
        "independent": independent,
    }


def _record(kind: str, details: dict[str, object]) -> dict[str, object]:
    return {
        "kind": kind,
        "status": "accepted",
        "artifact_uri": f"https://evidence.example/{kind}.json",
        "retention_uri": f"https://retention.example/{kind}/{_SHA}",
        "artifact_sha256": _SHA,
        "owner": f"{kind}-owner@example.org",
        "submitted_at": "2026-07-13T10:00:00Z",
        "review": _review(),
        "details": details,
    }


def _manifest() -> dict[str, object]:
    return {
        "schema_version": "1.0",
        "release_id": "core-2026.7.10",
        "candidate_bundle_version": "2026.7.10",
        "matrix_sha256": _SHA,
        "records": [
            _record(
                EvidenceKind.SCIENTIFIC_VALIDATION.value,
                {
                    "dataset_id": "independent-acmg-validation-1",
                    "provenance_uris": ["https://data.example/dataset"],
                    "redistribution_permission": "CC-BY-4.0",
                    "evaluation_protocol": "blinded_holdout_v1",
                    "result_summary": "pre-registered results retained",
                    "independent_review": True,
                    "synthetic_only": False,
                },
            ),
            _record(
                EvidenceKind.CONTROLLED_CATALOG.value,
                {
                    "key_id": "release-2026",
                    "algorithm": "Ed25519",
                    "public_key_sha256": _SHA,
                    "catalog_sha256": _SHA,
                    "archive_sha256": _SHA,
                    "manifest_sha256": _SHA,
                    "manifest_verified": True,
                    "signature_verified": True,
                    "published_uri": "https://release.example/catalog.json",
                    "revocation_uri": "https://release.example/revocations.json",
                    "retention_uri": "https://retention.example/catalog",
                },
            ),
            _record(
                EvidenceKind.BUNDLE_INSTALLATION.value,
                {
                    "bundle_version": "2026.7.10",
                    "platform": "ubuntu-24.04",
                    "python_version": "3.12",
                    "clean_install": True,
                    "first_use_seconds": 42.0,
                    "installed_manifest_sha256": _SHA,
                    "cache_sha256": _SHA,
                    "runtime_smoke_status": "passed",
                },
            ),
            _record(
                EvidenceKind.PLATFORM_USABILITY.value,
                {
                    "platforms": ["ubuntu-24.04", "windows-2025"],
                    "python_versions": ["3.12"],
                    "clean_install_report_uri": "https://evidence.example/install.json",
                    "participant_record_uri": "https://evidence.example/participants.json",
                    "participant_count": 3,
                    "report_sha256": _SHA,
                    "first_use_max_seconds": 300.0,
                    "automation_only": False,
                },
            ),
            _record(
                EvidenceKind.SECURITY_ASSESSMENT.value,
                {
                    "assessor": "Independent Security Lab",
                    "assessor_independent": True,
                    "standard": "OWASP ASVS 4.0.3",
                    "issued_at": "2026-07-12T00:00:00Z",
                    "valid_until": "2027-07-12T00:00:00Z",
                    "report_sha256": _SHA,
                    "findings_status": "resolved",
                    "scope": "source, package, CI",
                },
            ),
            _record(
                EvidenceKind.LIVE_PERFORMANCE.value,
                {
                    "workload_id": "live-research-workload-1",
                    "synthetic": False,
                    "platform": "ubuntu-24.04",
                    "python_version": "3.12",
                    "sample_count": 100,
                    "p95_ms": 120.0,
                    "p99_ms": 180.0,
                    "collection_period": "2026-07-10/2026-07-12",
                    "report_uri": "https://evidence.example/performance.json",
                },
            ),
            _record(
                EvidenceKind.RELEASE_OWNER_APPROVAL.value,
                {
                    "release_version": "2026.7.10",
                    "compatibility_window_end": "2027-07-13",
                    "decision_id": "release-decision-2026.7.10",
                    "bound_candidate_bundle_version": "2026.7.10",
                    "candidate_bundle_sha256": _SHA,
                    "evidence_manifest_sha256": _SHA,
                    "approved_at": "2026-07-13T00:00:00Z",
                    "approved": True,
                },
            ),
        ],
    }


def test_complete_manifest_is_accepted() -> None:
    result = validate_evidence_manifest(_manifest())

    assert result.status is EvidenceValidationStatus.COMPLETE
    assert result.blockers == ()
    assert result.manifest is not None


def test_platform_and_approval_records_require_digest_bound_details() -> None:
    payload = deepcopy(_manifest())
    platform = next(
        record
        for record in payload["records"]
        if record["kind"] == EvidenceKind.PLATFORM_USABILITY.value
    )
    approval = next(
        record
        for record in payload["records"]
        if record["kind"] == EvidenceKind.RELEASE_OWNER_APPROVAL.value
    )
    platform["details"].pop("report_sha256")
    approval["details"].pop("evidence_manifest_sha256")

    result = validate_evidence_manifest(payload)

    assert result.status is EvidenceValidationStatus.BLOCKED
    assert any(
        "platform_usability detail is missing: report_sha256" in blocker
        for blocker in result.blockers
    )
    assert any(
        "release_owner_approval detail is missing: evidence_manifest_sha256"
        in blocker
        for blocker in result.blockers
    )


def test_missing_record_is_blocked_with_canonical_kind() -> None:
    payload = _manifest()
    payload["records"] = [
        record
        for record in payload["records"]
        if record["kind"] != EvidenceKind.SECURITY_ASSESSMENT.value
    ]

    result = validate_evidence_manifest(payload)

    assert result.status is EvidenceValidationStatus.BLOCKED
    assert any("security_assessment" in blocker for blocker in result.blockers)


def test_synthetic_performance_cannot_satisfy_live_gate() -> None:
    payload = deepcopy(_manifest())
    performance = next(
        record
        for record in payload["records"]
        if record["kind"] == EvidenceKind.LIVE_PERFORMANCE.value
    )
    performance["details"]["synthetic"] = True

    result = validate_evidence_manifest(payload)

    assert result.status is EvidenceValidationStatus.BLOCKED
    assert any("synthetic" in blocker.lower() for blocker in result.blockers)


def test_non_https_reference_is_invalid() -> None:
    payload = deepcopy(_manifest())
    payload["records"][0]["artifact_uri"] = "file:///tmp/evidence.json"

    result = validate_evidence_manifest(payload)

    assert result.status is EvidenceValidationStatus.INVALID
    assert any("https" in blocker.lower() for blocker in result.blockers)


def test_secret_bearing_detail_is_invalid() -> None:
    payload = deepcopy(_manifest())
    secret_field = "".join(("priv", "ate", "_", "key"))
    payload["records"][1]["details"][secret_field] = "x"

    result = validate_evidence_manifest(payload)

    assert result.status is EvidenceValidationStatus.INVALID
    assert any("secret" in blocker.lower() for blocker in result.blockers)


def test_unknown_detail_and_inline_signature_are_invalid() -> None:
    payload = deepcopy(_manifest())
    payload["records"][1]["details"]["signature"] = "inline-signature"

    result = validate_evidence_manifest(payload)

    assert result.status is EvidenceValidationStatus.INVALID
    assert any("detail is unknown: signature" in blocker for blocker in result.blockers)


def test_placeholder_digest_cannot_complete_an_accepted_record() -> None:
    payload = deepcopy(_manifest())
    payload["records"][0]["artifact_sha256"] = "0" * 64

    result = validate_evidence_manifest(payload)

    assert result.status is EvidenceValidationStatus.BLOCKED
    assert any("placeholder" in blocker.lower() for blocker in result.blockers)


def test_non_independent_security_review_is_blocked() -> None:
    payload = deepcopy(_manifest())
    security = next(
        record
        for record in payload["records"]
        if record["kind"] == EvidenceKind.SECURITY_ASSESSMENT.value
    )
    security["details"]["assessor_independent"] = False

    result = validate_evidence_manifest(payload)

    assert result.status is EvidenceValidationStatus.BLOCKED
    assert any("independent" in blocker.lower() for blocker in result.blockers)


def test_unapproved_record_is_blocked() -> None:
    payload = deepcopy(_manifest())
    approval = next(
        record
        for record in payload["records"]
        if record["kind"] == EvidenceKind.RELEASE_OWNER_APPROVAL.value
    )
    approval["status"] = "pending"

    result = validate_evidence_manifest(payload)

    assert result.status is EvidenceValidationStatus.BLOCKED
    assert any("accepted" in blocker.lower() for blocker in result.blockers)


def test_result_serialization_has_no_raw_details() -> None:
    result = validate_evidence_manifest(_manifest())

    payload = result.as_dict()

    assert payload["status"] == "complete"
    assert "details" not in payload
    assert payload["checked_at"].endswith("Z")
    datetime.fromisoformat(payload["checked_at"].replace("Z", "+00:00")).astimezone(UTC)


def test_evidence_cli_reports_complete_without_raw_details(tmp_path: Path) -> None:
    manifest_path = tmp_path / "evidence.json"
    manifest_path.write_text(json.dumps(_manifest()), encoding="utf-8")

    result = subprocess.run(
        [
            sys.executable,
            "scripts/validate_release_evidence.py",
            "--manifest",
            str(manifest_path),
            "--format",
            "json",
        ],
        check=False,
        capture_output=True,
        text=True,
    )

    assert result.returncode == 0, result.stderr
    payload = json.loads(result.stdout)
    assert payload["status"] == "complete"
    assert "details" not in payload


def test_evidence_cli_returns_one_for_incomplete_manifest(tmp_path: Path) -> None:
    payload = _manifest()
    payload["records"] = payload["records"][:-1]
    manifest_path = tmp_path / "evidence.json"
    manifest_path.write_text(json.dumps(payload), encoding="utf-8")

    result = subprocess.run(
        [
            sys.executable,
            "scripts/validate_release_evidence.py",
            "--manifest",
            str(manifest_path),
        ],
        check=False,
        capture_output=True,
        text=True,
    )

    assert result.returncode == 1
    assert "release_owner_approval" in result.stdout
