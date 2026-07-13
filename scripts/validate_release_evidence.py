#!/usr/bin/env python3
"""Validate metadata for external release evidence without reading artifacts."""

from __future__ import annotations

import argparse
import json
import sys
from collections.abc import Mapping, Sequence
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

from acmg_classifier.validation.release_evidence import (
    EvidenceValidationResult,
    EvidenceValidationStatus,
    validate_evidence_manifest,
)


def _load_manifest(path: Path) -> Mapping[str, Any]:
    try:
        value = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as error:
        raise ValueError(f"unable to read evidence manifest: {error}") from error
    if not isinstance(value, Mapping):
        raise ValueError("evidence manifest must be a JSON object")
    return value


def _invalid_result(message: str) -> EvidenceValidationResult:
    return EvidenceValidationResult(
        status=EvidenceValidationStatus.INVALID,
        blockers=(message,),
        manifest=None,
        checked_at=datetime.now(UTC).isoformat().replace("+00:00", "Z"),
    )


def _render_text(result: EvidenceValidationResult) -> str:
    payload = result.as_dict()
    lines = [
        f"Evidence status: {payload['status']}",
        f"Release ID: {payload.get('release_id') or 'invalid'}",
        f"Candidate bundle: {payload.get('candidate_bundle_version') or 'invalid'}",
    ]
    if result.blockers:
        lines.append("Blockers:")
        lines.extend(f"- {blocker}" for blocker in result.blockers)
    else:
        lines.append("Blockers: none")
    return "\n".join(lines)


def main(argv: Sequence[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--manifest", type=Path, required=True)
    parser.add_argument("--format", choices=("text", "json"), default="text")
    args = parser.parse_args(argv)

    try:
        result = validate_evidence_manifest(_load_manifest(args.manifest))
    except ValueError as error:
        result = _invalid_result(str(error))

    payload = result.as_dict()
    if args.format == "json":
        print(json.dumps(payload, indent=2, sort_keys=True))
    else:
        print(_render_text(result))
    if result.status is EvidenceValidationStatus.COMPLETE:
        return 0
    if result.status is EvidenceValidationStatus.BLOCKED:
        return 1
    return 2


if __name__ == "__main__":
    sys.exit(main())
