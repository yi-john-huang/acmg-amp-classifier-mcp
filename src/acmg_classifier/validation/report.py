"""Deterministic reporting for provenance-aware golden validation."""

from __future__ import annotations

import json
from dataclasses import dataclass

from acmg_classifier.validation.fixtures import GoldenValidationManifest
from acmg_classifier.validation.metrics import ValidationMetrics


@dataclass(frozen=True, slots=True)
class ValidationReport:
    """Release-gated report that never turns fixture execution into a release claim."""

    dataset_id: str
    dataset_version: str
    dataset_kind: str
    provenance: dict[str, object]
    exclusions: tuple[dict[str, str], ...]
    source_classification_masking: tuple[dict[str, str], ...]
    metrics: ValidationMetrics
    release_eligible: bool
    release_blockers: tuple[str, ...]
    research_use_only: bool = True

    def to_dict(self) -> dict[str, object]:
        return {
            "dataset_id": self.dataset_id,
            "dataset_kind": self.dataset_kind,
            "dataset_version": self.dataset_version,
            "exclusions": list(self.exclusions),
            "metrics": self.metrics.to_dict(),
            "provenance": self.provenance,
            "release_blockers": list(self.release_blockers),
            "release_eligible": self.release_eligible,
            "research_use_only": self.research_use_only,
            "scientific_release_assertion": "not made by this validation framework",
            "source_classification_masking": list(self.source_classification_masking),
        }


def build_validation_report(
    manifest: GoldenValidationManifest, metrics: ValidationMetrics
) -> ValidationReport:
    """Build a complete report; missing provenance remains an explicit blocker."""
    if manifest.provenance.is_approved:
        release_eligible = True
        release_blockers: tuple[str, ...] = ()
    else:
        release_eligible = False
        missing_prerequisite = manifest.provenance.missing_prerequisite
        if missing_prerequisite is None:
            raise ValueError("missing provenance must name a release blocker")
        release_blockers = (missing_prerequisite,)
    masks = tuple(
        sorted(
            (
                mask.to_dict()
                for case in manifest.cases
                for mask in case.masked_source_fields
            ),
            key=lambda item: (item["source_id"], item["field"]),
        )
    )
    return ValidationReport(
        dataset_id=manifest.dataset_id,
        dataset_version=manifest.dataset_version,
        dataset_kind=manifest.dataset_kind,
        provenance=manifest.provenance.to_dict(),
        exclusions=tuple(exclusion.to_dict() for exclusion in manifest.exclusions),
        source_classification_masking=masks,
        metrics=metrics,
        release_eligible=release_eligible,
        release_blockers=release_blockers,
    )


def render_report(report: ValidationReport) -> str:
    """Render canonical JSON without timestamps, host data, or incidental ordering."""
    return json.dumps(
        report.to_dict(), sort_keys=True, separators=(",", ":"), allow_nan=False
    )
