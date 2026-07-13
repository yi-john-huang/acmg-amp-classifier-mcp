#!/usr/bin/env python3
"""Evaluate the source-controlled release readiness contract.

This maintainer-facing tool reads release metadata only. It never treats local
engineering tests as scientific, security, usability, or performance evidence.
"""

from __future__ import annotations

import argparse
import json
import sys
from collections.abc import Mapping, Sequence
from dataclasses import dataclass
from datetime import UTC, datetime
from enum import StrEnum
from pathlib import Path
from typing import Any

ROOT = Path(__file__).resolve().parents[1]
DEFAULT_MATRIX = ROOT / "docs" / "release" / "capabilities.json"
REQUIRED_GATE_IDS = ("10.1", "10.2", "10.3", "10.4", "10.5", "10.6")
_ALLOWED_GATE_STATES = frozenset({"blocked", "complete"})
_ALLOWED_REVIEW_STATES = frozenset({"pending", "approved"})
_EVIDENCE_MANIFEST_STATUSES = frozenset({"missing", "accepted"})
_EVIDENCE_REVIEW_STATUSES = frozenset({"pending", "approved", "rejected"})
_REQUIRED_EVIDENCE_KINDS = (
    "scientific_validation",
    "controlled_catalog",
    "bundle_installation",
    "platform_usability",
    "security_assessment",
    "live_performance",
    "release_owner_approval",
)


class MatrixValidationError(ValueError):
    """The readiness matrix is missing required or contradictory metadata."""


class ReadinessStatus(StrEnum):
    """Stable evaluator result states."""

    READY = "ready"
    BLOCKED = "blocked"


@dataclass(frozen=True, slots=True)
class Blocker:
    """One actionable reason a release cannot proceed."""

    gate_id: str
    reason: str
    required_evidence: str
    owner_role: str

    def as_dict(self) -> dict[str, str]:
        return {
            "gate_id": self.gate_id,
            "reason": self.reason,
            "required_evidence": self.required_evidence,
            "owner_role": self.owner_role,
        }


@dataclass(frozen=True, slots=True)
class GateResult:
    """Validated state for one release gate."""

    task_id: str
    state: str
    status: str
    blockers: tuple[str, ...]
    evidence: str
    owner_role: str
    required_evidence: str
    review_status: str

    def as_dict(self) -> dict[str, Any]:
        return {
            "task_id": self.task_id,
            "state": self.state,
            "status": self.status,
            "blockers": list(self.blockers),
            "evidence": self.evidence,
            "owner_role": self.owner_role,
            "required_evidence": self.required_evidence,
            "review_status": self.review_status,
        }


@dataclass(frozen=True, slots=True)
class ReadinessResult:
    """Complete, serializable release readiness decision."""

    status: ReadinessStatus
    release_status: str
    blockers: tuple[Blocker, ...]
    gates: tuple[GateResult, ...]
    checked_at: str

    def as_dict(self) -> dict[str, Any]:
        return {
            "status": self.status.value,
            "release_status": self.release_status,
            "blockers": [blocker.as_dict() for blocker in self.blockers],
            "gates": [gate.as_dict() for gate in self.gates],
            "checked_at": self.checked_at,
        }


def _require_mapping(value: Any, label: str) -> Mapping[str, Any]:
    if not isinstance(value, Mapping):
        raise MatrixValidationError(f"{label} must be an object")
    return value




def _require_sha256(value: Any, label: str) -> str:
    result = _require_nonempty_string(value, label)
    if len(result) != 64 or any(
        character not in "0123456789abcdef" for character in result
    ):
        raise MatrixValidationError(f"{label} must be a lowercase SHA-256 digest")
    if result == "0" * 64:
        raise MatrixValidationError(f"{label} must not be a placeholder digest")
    return result


def _validate_evidence_manifest_metadata(
    value: Any, candidate_bundle_version: str
) -> tuple[Blocker, ...]:
    evidence = _require_mapping(value, "evidence_manifest")
    status = _require_nonempty_string(
        evidence.get("status"), "evidence_manifest status"
    )
    if status not in _EVIDENCE_MANIFEST_STATUSES:
        raise MatrixValidationError(
            f"evidence_manifest has unknown status {status!r}"
        )
    release_id = _require_nonempty_string(
        evidence.get("release_id"), "evidence_manifest release_id"
    )
    path = _require_nonempty_string(
        evidence.get("path"), "evidence_manifest path"
    )
    if (
        path.startswith(("/", "\\"))
        or "\\" in path
        or ".." in path.split("/")
    ):
        raise MatrixValidationError("evidence_manifest path must be safe")
    bound_version = _require_nonempty_string(
        evidence.get("candidate_bundle_version"),
        "evidence_manifest candidate_bundle_version",
    )
    if bound_version != candidate_bundle_version:
        raise MatrixValidationError(
            "evidence_manifest candidate_bundle_version does not match candidate"
        )
    required_kinds = _require_string_list(
        evidence.get("required_kinds"), "evidence_manifest required_kinds"
    )
    if required_kinds != _REQUIRED_EVIDENCE_KINDS:
        raise MatrixValidationError(
            "evidence_manifest required_kinds must match the canonical evidence kinds"
        )
    review_status = _require_nonempty_string(
        evidence.get("review_status"), "evidence_manifest review_status"
    )
    if review_status not in _EVIDENCE_REVIEW_STATUSES:
        raise MatrixValidationError(
            f"evidence_manifest has unknown review_status {review_status!r}"
        )
    if status == "accepted":
        _require_sha256(evidence.get("sha256"), "evidence_manifest sha256")
        _require_nonempty_string(
            evidence.get("verified_at"), "evidence_manifest verified_at"
        )
        _require_nonempty_string(
            evidence.get("verified_by"), "evidence_manifest verified_by"
        )
        if review_status != "approved":
            raise MatrixValidationError(
                "accepted evidence_manifest requires an approved review"
            )
        return ()
    return (
        Blocker(
            gate_id="evidence-manifest",
            reason=(
                f"Evidence manifest {release_id!r} at {path!r} "
                f"has status {status!r}; all external evidence must be accepted."
            ),
            required_evidence=(
                "accepted digest-bound evidence manifest covering scientific, "
                "catalog, bundle, platform, security, performance, and approval records"
            ),
            owner_role="controlled-release-owner",
        ),
    )
def _require_nonempty_string(value: Any, label: str) -> str:
    if not isinstance(value, str) or not value.strip():
        raise MatrixValidationError(f"{label} must be a non-empty string")
    return value


def _require_string_list(value: Any, label: str) -> tuple[str, ...]:
    if not isinstance(value, list) or not all(isinstance(item, str) for item in value):
        raise MatrixValidationError(f"{label} must be a list of strings")
    return tuple(value)


def _validate_gate(gate: Any, allowed_statuses: frozenset[str]) -> GateResult:
    value = _require_mapping(gate, "release gate")
    task_id = _require_nonempty_string(value.get("task_id"), "gate task_id")
    state = _require_nonempty_string(value.get("state"), f"gate {task_id} state")
    if state not in _ALLOWED_GATE_STATES:
        raise MatrixValidationError(f"gate {task_id} has unknown state {state!r}")
    status = _require_nonempty_string(value.get("status"), f"gate {task_id} status")
    if status not in allowed_statuses:
        raise MatrixValidationError(f"gate {task_id} has unknown status {status!r}")
    blockers = _require_string_list(value.get("blockers"), f"gate {task_id} blockers")
    evidence = _require_nonempty_string(
        value.get("evidence"), f"gate {task_id} evidence"
    )
    owner_role = _require_nonempty_string(
        value.get("owner_role"), f"gate {task_id} owner_role"
    )
    required_evidence = _require_nonempty_string(
        value.get("required_evidence"), f"gate {task_id} required_evidence"
    )
    review = _require_mapping(value.get("review"), f"gate {task_id} review")
    review_status = _require_nonempty_string(
        review.get("status"), f"gate {task_id} review status"
    )
    if review_status not in _ALLOWED_REVIEW_STATES:
        raise MatrixValidationError(
            f"gate {task_id} has unknown review status {review_status!r}"
        )
    if state == "complete":
        _require_nonempty_string(
            review.get("reviewed_at"), f"gate {task_id} review reviewed_at"
        )
        _require_nonempty_string(
            review.get("reviewed_by"), f"gate {task_id} review reviewed_by"
        )
    if state == "complete" and (blockers or review_status != "approved"):
        raise MatrixValidationError(
            f"complete gate {task_id} must have no blockers and an approved review"
        )
    if state == "blocked" and not blockers:
        raise MatrixValidationError(f"blocked gate {task_id} must list a blocker")
    return GateResult(
        task_id=task_id,
        state=state,
        status=status,
        blockers=blockers,
        evidence=evidence,
        owner_role=owner_role,
        required_evidence=required_evidence,
        review_status=review_status,
    )


def _blocker_for_gate(gate: GateResult) -> tuple[Blocker, ...]:
    if gate.state == "complete":
        return ()
    return tuple(
        Blocker(
            gate_id=gate.task_id,
            reason=reason,
            required_evidence=gate.required_evidence,
            owner_role=gate.owner_role,
        )
        for reason in gate.blockers
    )


def _append_global_blocker(
    blockers: list[Blocker], reason: str, required_evidence: str
) -> None:
    blockers.append(
        Blocker(
            gate_id="candidate-bundle",
            reason=reason,
            required_evidence=required_evidence,
            owner_role="controlled-release-owner",
        )
    )


def evaluate_matrix(matrix: Mapping[str, Any]) -> ReadinessResult:
    """Validate and evaluate a release matrix without performing I/O."""
    root = _require_mapping(matrix, "matrix")
    if root.get("readiness_schema_version") != "1.0":
        raise MatrixValidationError("readiness_schema_version must be '1.0'")
    required_ids = _require_string_list(
        root.get("required_release_gate_ids"), "required_release_gate_ids"
    )
    if required_ids != REQUIRED_GATE_IDS:
        raise MatrixValidationError(
            "required_release_gate_ids must contain the canonical release gates"
        )
    vocabulary = _require_string_list(
        root.get("status_vocabulary"), "status_vocabulary"
    )
    allowed_statuses = frozenset(vocabulary)
    raw_gates = root.get("release_gates")
    if not isinstance(raw_gates, list):
        raise MatrixValidationError("release_gates must be a list")
    gates_by_id: dict[str, GateResult] = {}
    for raw_gate in raw_gates:
        gate = _validate_gate(raw_gate, allowed_statuses)
        if gate.task_id in gates_by_id:
            raise MatrixValidationError(f"duplicate release gate {gate.task_id}")
        gates_by_id[gate.task_id] = gate
    if set(gates_by_id) != set(REQUIRED_GATE_IDS):
        raise MatrixValidationError("release_gates do not match required gate IDs")
    gates = tuple(gates_by_id[task_id] for task_id in REQUIRED_GATE_IDS)
    blockers = [blocker for gate in gates for blocker in _blocker_for_gate(gate)]

    release_status = _require_nonempty_string(
        root.get("release_status"), "release_status"
    )
    if release_status != "ready":
        blockers.append(
            Blocker(
                gate_id="release-status",
                reason=(
                    f"Matrix release_status is {release_status!r}; "
                    "controlled release approval is not recorded."
                ),
                required_evidence="approved controlled release decision",
                owner_role="controlled-release-owner",
            )
        )
    candidate = _require_mapping(root.get("candidate_bundle"), "candidate_bundle")
    signature_status = _require_nonempty_string(
        candidate.get("release_signature_status"),
        "candidate_bundle.release_signature_status",
    )
    scientific_gate = _require_nonempty_string(
        candidate.get("scientific_release_gate"),
        "candidate_bundle.scientific_release_gate",
    )
    candidate_bundle_version = _require_nonempty_string(
        candidate.get("bundle_version"), "candidate_bundle.bundle_version"
    )
    blockers.extend(
        _validate_evidence_manifest_metadata(
            root.get("evidence_manifest"), candidate_bundle_version
        )
    )
    if signature_status != "verified":
        _append_global_blocker(
            blockers,
            "Candidate bundle does not have a verified controlled release signature.",
            "controlled Ed25519 release signature and verification record",
        )
    if scientific_gate != "complete":
        _append_global_blocker(
            blockers,
            "Scientific release validation gate is not complete.",
            "independently curated, provenance-approved validation evidence",
        )

    capabilities = root.get("capabilities")
    if not isinstance(capabilities, list):
        raise MatrixValidationError("capabilities must be a list")
    default_runtime = next(
        (
            _require_mapping(capability, "capability")
            for capability in capabilities
            if isinstance(capability, Mapping)
            and capability.get("id") == "default-classification-runtime"
        ),
        None,
    )
    if default_runtime is None:
        raise MatrixValidationError(
            "default-classification-runtime capability is missing"
        )
    if default_runtime.get("status") == "unavailable":
        _append_global_blocker(
            blockers,
            "The default scientific classification runtime is unavailable.",
            "controlled signed catalog and compatible scientific runtime bundle",
        )

    status = (
        ReadinessStatus.READY
        if release_status == "ready" and not blockers
        else ReadinessStatus.BLOCKED
    )
    return ReadinessResult(
        status=status,
        release_status=release_status,
        blockers=tuple(blockers),
        gates=gates,
        checked_at=datetime.now(UTC).isoformat().replace("+00:00", "Z"),
    )


def _load_matrix(path: Path) -> Mapping[str, Any]:
    try:
        value = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as error:
        raise MatrixValidationError(f"unable to read matrix: {error}") from error
    return _require_mapping(value, "matrix")


def _invalid_payload(error: Exception) -> dict[str, Any]:
    return {
        "status": "invalid",
        "release_status": None,
        "blockers": [
            {
                "gate_id": "matrix",
                "reason": str(error),
                "required_evidence": "valid canonical release matrix",
                "owner_role": "release-maintainer",
            }
        ],
        "gates": [],
        "checked_at": datetime.now(UTC).isoformat().replace("+00:00", "Z"),
    }


def _render_text(payload: Mapping[str, Any]) -> str:
    lines = [
        f"Release status: {payload['status']}",
        f"Matrix release_status: {payload.get('release_status') or 'invalid'}",
    ]
    blockers = payload.get("blockers", [])
    if blockers:
        lines.append("Blockers:")
        lines.extend(
            f"- {item['gate_id']}: {item['reason']}"
            for item in blockers
            if isinstance(item, Mapping)
        )
    else:
        lines.append("Blockers: none")
    return "\n".join(lines)


def main(argv: Sequence[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--matrix", type=Path, default=DEFAULT_MATRIX)
    parser.add_argument("--format", choices=("text", "json"), default="text")
    args = parser.parse_args(argv)
    try:
        result = evaluate_matrix(_load_matrix(args.matrix))
        payload = result.as_dict()
    except MatrixValidationError as error:
        payload = _invalid_payload(error)
    if args.format == "json":
        print(json.dumps(payload, indent=2, sort_keys=True))
    else:
        print(_render_text(payload))
    if payload["status"] == ReadinessStatus.READY.value:
        return 0
    return 2 if payload["status"] == "invalid" else 1


if __name__ == "__main__":
    sys.exit(main())
