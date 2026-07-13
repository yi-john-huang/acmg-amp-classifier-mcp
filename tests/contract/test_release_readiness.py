from __future__ import annotations

import copy
import json
import subprocess
import sys
from pathlib import Path

import pytest
from scripts.check_release_readiness import (
    MatrixValidationError,
    ReadinessStatus,
    evaluate_matrix,
)

ROOT = Path(__file__).resolve().parents[2]
MATRIX_PATH = ROOT / "docs" / "release" / "capabilities.json"
EVALUATOR = ROOT / "scripts" / "check_release_readiness.py"
BENCHMARK_SCRIPT = ROOT / "scripts" / "run_benchmark_evidence.py"

REQUIRED_GATE_IDS = {"10.1", "10.2", "10.3", "10.4", "10.5", "10.6"}


def _matrix() -> dict[str, object]:
    return json.loads(MATRIX_PATH.read_text(encoding="utf-8"))

def _mark_evidence_manifest_accepted(matrix: dict[str, object]) -> None:
    evidence = matrix["evidence_manifest"]
    assert isinstance(evidence, dict)
    evidence.update(
        {
            "status": "accepted",
            "candidate_bundle_version": "2026.7.10",
            "sha256": "c" * 64,
            "verified_at": "2026-07-13T12:00:00Z",
            "verified_by": "controlled-release-owner",
            "review_status": "approved",
        }
    )

def test_current_matrix_has_a_complete_fail_closed_ledger() -> None:
    matrix = _matrix()

    assert matrix["readiness_schema_version"] == "1.0"
    assert set(matrix["required_release_gate_ids"]) == REQUIRED_GATE_IDS
    gates = {gate["task_id"]: gate for gate in matrix["release_gates"]}
    assert set(gates) == REQUIRED_GATE_IDS
    for gate in gates.values():
        assert gate["state"] == "blocked"
        assert gate["owner_role"]
        assert gate["required_evidence"]
        assert gate["review"]["status"] == "pending"


def test_current_matrix_is_blocked_by_every_unresolved_gate() -> None:
    result = evaluate_matrix(_matrix())

    assert result.status is ReadinessStatus.BLOCKED
    assert result.release_status == "experimental"
    assert {gate.task_id for gate in result.gates} == REQUIRED_GATE_IDS
    assert {blocker.gate_id for blocker in result.blockers} >= REQUIRED_GATE_IDS
    assert any("controlled" in blocker.reason.lower() for blocker in result.blockers)
    assert any("scientific" in blocker.reason.lower() for blocker in result.blockers)


def test_future_explicitly_complete_matrix_is_ready() -> None:
    matrix = copy.deepcopy(_matrix())
    matrix["release_status"] = "ready"
    matrix["candidate_bundle"]["release_signature_status"] = "verified"
    matrix["candidate_bundle"]["scientific_release_gate"] = "complete"
    _mark_evidence_manifest_accepted(matrix)
    for capability in matrix["capabilities"]:
        if capability["id"] == "default-classification-runtime":
            capability["status"] = "experimental"
    for gate in matrix["release_gates"]:
        gate["state"] = "complete"
        gate["blockers"] = []
        gate["review"] = {
            "status": "approved",
            "reviewed_at": "2026-07-13T00:00:00Z",
            "reviewed_by": "release-owner",
        }

    result = evaluate_matrix(matrix)

    assert result.status is ReadinessStatus.READY
    assert result.blockers == ()


def test_ready_matrix_requires_accepted_evidence_manifest() -> None:
    matrix = copy.deepcopy(_matrix())
    matrix["release_status"] = "ready"
    matrix["candidate_bundle"]["release_signature_status"] = "verified"
    matrix["candidate_bundle"]["scientific_release_gate"] = "complete"
    _mark_evidence_manifest_accepted(matrix)
    matrix["evidence_manifest"]["status"] = "missing"
    for capability in matrix["capabilities"]:
        if capability["id"] == "default-classification-runtime":
            capability["status"] = "experimental"
    for gate in matrix["release_gates"]:
        gate["state"] = "complete"
        gate["blockers"] = []
        gate["review"] = {
            "status": "approved",
            "reviewed_at": "2026-07-13T00:00:00Z",
            "reviewed_by": "release-owner",
        }

    result = evaluate_matrix(matrix)

    assert result.status is ReadinessStatus.BLOCKED
    assert any(blocker.gate_id == "evidence-manifest" for blocker in result.blockers)



def test_ready_matrix_rejects_placeholder_evidence_digest() -> None:
    matrix = copy.deepcopy(_matrix())
    matrix["release_status"] = "ready"
    matrix["candidate_bundle"]["release_signature_status"] = "verified"
    matrix["candidate_bundle"]["scientific_release_gate"] = "complete"
    _mark_evidence_manifest_accepted(matrix)
    matrix["evidence_manifest"]["sha256"] = "0" * 64
    for capability in matrix["capabilities"]:
        if capability["id"] == "default-classification-runtime":
            capability["status"] = "experimental"
    for gate in matrix["release_gates"]:
        gate["state"] = "complete"
        gate["blockers"] = []
        gate["review"] = {
            "status": "approved",
            "reviewed_at": "2026-07-13T00:00:00Z",
            "reviewed_by": "release-owner",
        }

    with pytest.raises(MatrixValidationError, match="placeholder"):
        evaluate_matrix(matrix)

def test_complete_gate_requires_review_identity() -> None:
    matrix = copy.deepcopy(_matrix())
    matrix["release_status"] = "ready"
    matrix["candidate_bundle"]["release_signature_status"] = "verified"
    matrix["candidate_bundle"]["scientific_release_gate"] = "complete"
    for capability in matrix["capabilities"]:
        if capability["id"] == "default-classification-runtime":
            capability["status"] = "experimental"
    for gate in matrix["release_gates"]:
        gate["state"] = "complete"
        gate["blockers"] = []
        gate["review"] = {"status": "approved"}

    with pytest.raises(MatrixValidationError, match="reviewed_at"):
        evaluate_matrix(matrix)


def test_complete_evidence_stays_blocked_without_ready_release_status() -> None:
    matrix = copy.deepcopy(_matrix())
    matrix["release_status"] = "experimental"
    matrix["candidate_bundle"]["release_signature_status"] = "verified"
    matrix["candidate_bundle"]["scientific_release_gate"] = "complete"
    for capability in matrix["capabilities"]:
        if capability["id"] == "default-classification-runtime":
            capability["status"] = "experimental"
    for gate in matrix["release_gates"]:
        gate["state"] = "complete"
        gate["blockers"] = []
        gate["review"] = {
            "status": "approved",
            "reviewed_at": "2026-07-13T00:00:00Z",
            "reviewed_by": "release-owner",
        }

    result = evaluate_matrix(matrix)

    assert result.status is ReadinessStatus.BLOCKED
    assert any(blocker.gate_id == "release-status" for blocker in result.blockers)


def test_evaluator_cli_reports_blocked_json_without_traceback() -> None:
    result = subprocess.run(
        [sys.executable, str(EVALUATOR), "--format", "json"],
        cwd=ROOT,
        check=False,
        capture_output=True,
        text=True,
    )

    assert result.returncode == 1
    assert "Traceback" not in result.stderr
    payload = json.loads(result.stdout)
    assert payload["status"] == "blocked"
    assert payload["release_status"] == "experimental"
    assert {item["gate_id"] for item in payload["blockers"]} >= REQUIRED_GATE_IDS


def test_evaluator_cli_rejects_invalid_matrix(tmp_path: Path) -> None:
    invalid_matrix = tmp_path / "invalid.json"
    invalid_matrix.write_text("{\"release_status\": \"ready\"}", encoding="utf-8")

    result = subprocess.run(
        [
            sys.executable,
            str(EVALUATOR),
            "--matrix",
            str(invalid_matrix),
            "--format",
            "json",
        ],
        cwd=ROOT,
        check=False,
        capture_output=True,
        text=True,
    )

    assert result.returncode == 2
    payload = json.loads(result.stdout)
    assert payload["status"] == "invalid"
    assert payload["blockers"]


def test_synthetic_benchmark_report_has_an_explicit_external_exclusion(
    tmp_path: Path,
) -> None:
    output = tmp_path / "benchmark.json"

    result = subprocess.run(
        [sys.executable, str(BENCHMARK_SCRIPT), "--output", str(output)],
        cwd=ROOT,
        check=False,
        capture_output=True,
        text=True,
    )

    assert result.returncode == 0, result.stderr
    report = json.loads(output.read_text(encoding="utf-8"))
    assert report["fixture_type"] == "cached-classification"
    assert report["timing_kind"] == "synthetic"
    assert report["platform"]
    assert report["python_version"]
    assert any("synthetic" in item.lower() for item in report["exclusions"])
