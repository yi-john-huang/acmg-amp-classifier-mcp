"""Task 10.2 strict golden-fixture and pure-metric contracts."""

from __future__ import annotations

import json
from copy import deepcopy
from pathlib import Path

import pytest

from acmg_classifier.domain.enums import ClassificationTier, CriterionStatus
from acmg_classifier.validation.fixtures import (
    GoldenFixtureError,
    GoldenValidationManifest,
    load_golden_manifest,
)
from acmg_classifier.validation.metrics import (
    CaseValidationOutcome,
    calculate_validation_metrics,
)
from acmg_classifier.validation.report import build_validation_report, render_report


@pytest.fixture
def approved_manifest_data() -> dict[str, object]:
    return {
        "schema_version": 1,
        "dataset_id": "independent-reference-set",
        "dataset_version": "2026.1",
        "dataset_kind": "independently_curated_clinical",
        "provenance": {
            "approval_status": "approved",
            "approval_id": "IRB-VALIDATION-42",
            "protocol_id": "independent-golden-protocol",
            "protocol_version": "2.0",
            "approved_by": "Independent curation board",
            "approved_at": "2026-01-01T00:00:00Z",
            "license": "CC-BY-4.0",
            "independent_label_basis": "Blinded expert panel consensus",
            "source_artifacts": [
                {
                    "artifact_id": "reference-set-export",
                    "release": "2026.1",
                    "url": "https://example.org/reference-set-export.json",
                    "retrieved_at": "2026-01-02T00:00:00Z",
                    "sha256": "a" * 64,
                    "transformation_version": "1.0.0",
                }
            ],
            "missing_prerequisite": None,
        },
        "exclusions": [
            {
                "case_id": "excluded-embargoed-case",
                "reason": "Redistribution permission is pending.",
            }
        ],
        "cases": [
            {
                "case_id": "case-pathogenic",
                "request": {
                    "variant": "NM_000059.4:c.7008-1G>A",
                    "context": {"disease_id": "MONDO:0007254"},
                    "source_observations": [
                        {
                            "source_id": "clinvar",
                            "fields": {
                                "classification": "Pathogenic",
                                "record_id": "VCV000000001",
                            },
                        }
                    ],
                },
                "masked_source_fields": [
                    {"source_id": "clinvar", "field": "classification"}
                ],
                "expected": {
                    "classification": "pathogenic",
                    "criteria": {"PS3": "applied", "BP4": "not_evaluable"},
                    "normalization_failure": False,
                },
            },
            {
                "case_id": "case-vus",
                "request": {
                    "variant": "NM_000059.4:c.1A>G",
                    "context": {},
                    "source_observations": [],
                },
                "masked_source_fields": [],
                "expected": {
                    "classification": "uncertain_significance",
                    "criteria": {"PS3": "not_applied", "BP4": "not_applied"},
                    "normalization_failure": False,
                },
            },
            {
                "case_id": "case-benign",
                "request": {
                    "variant": "NM_000059.4:c.2A>G",
                    "context": {},
                    "source_observations": [],
                },
                "masked_source_fields": [],
                "expected": {
                    "classification": "benign",
                    "criteria": {"PS3": "applied", "BP4": "not_applied"},
                    "normalization_failure": False,
                },
            },
            {
                "case_id": "case-normalization-failure",
                "request": {
                    "variant": "not-a-supported-variant",
                    "context": {},
                    "source_observations": [],
                },
                "masked_source_fields": [],
                "expected": {
                    "classification": None,
                    "criteria": {},
                    "normalization_failure": True,
                },
            },
        ],
    }


def test_manifest_requires_complete_approved_provenance(
    approved_manifest_data: dict[str, object],
) -> None:
    manifest = GoldenValidationManifest.from_mapping(approved_manifest_data)

    assert manifest.provenance.is_approved is True
    assert manifest.provenance.source_artifacts[0].sha256 == "a" * 64

    invalid = deepcopy(approved_manifest_data)
    provenance = invalid["provenance"]
    assert isinstance(provenance, dict)
    provenance["approval_id"] = None

    with pytest.raises(GoldenFixtureError, match="approval_id"):
        GoldenValidationManifest.from_mapping(invalid)


def test_manifest_fails_closed_when_source_classification_is_not_masked(
    approved_manifest_data: dict[str, object],
) -> None:
    invalid = deepcopy(approved_manifest_data)
    cases = invalid["cases"]
    assert isinstance(cases, list)
    first_case = cases[0]
    assert isinstance(first_case, dict)
    first_case["masked_source_fields"] = []

    with pytest.raises(GoldenFixtureError, match="must be masked"):
        GoldenValidationManifest.from_mapping(invalid)


def test_manifest_rejects_unmasked_camel_case_source_classification(
    approved_manifest_data: dict[str, object],
) -> None:
    invalid = deepcopy(approved_manifest_data)
    cases = invalid["cases"]
    assert isinstance(cases, list)
    first_case = cases[0]
    assert isinstance(first_case, dict)
    request = first_case["request"]
    assert isinstance(request, dict)
    observations = request["source_observations"]
    assert isinstance(observations, list)
    first_observation = observations[0]
    assert isinstance(first_observation, dict)
    fields = first_observation["fields"]
    assert isinstance(fields, dict)
    fields["clinicalSignificance"] = fields.pop("classification")
    first_case["masked_source_fields"] = []

    with pytest.raises(GoldenFixtureError, match="must be masked"):
        GoldenValidationManifest.from_mapping(invalid)


def test_manifest_removes_masked_camel_case_source_classification(
    approved_manifest_data: dict[str, object],
) -> None:
    data = deepcopy(approved_manifest_data)
    cases = data["cases"]
    assert isinstance(cases, list)
    first_case = cases[0]
    assert isinstance(first_case, dict)
    request = first_case["request"]
    assert isinstance(request, dict)
    observations = request["source_observations"]
    assert isinstance(observations, list)
    first_observation = observations[0]
    assert isinstance(first_observation, dict)
    fields = first_observation["fields"]
    assert isinstance(fields, dict)
    fields["clinicalSignificance"] = fields.pop("classification")
    first_case["masked_source_fields"] = [
        {"source_id": "clinvar", "field": "clinicalSignificance"}
    ]

    manifest = GoldenValidationManifest.from_mapping(data)
    case = next(case for case in manifest.cases if case.case_id == "case-pathogenic")
    assert case.masked_request().source_observations[0].fields == {
        "record_id": "VCV000000001"
    }


def test_manifest_masks_source_classification_before_an_application_client_receives_it(
    approved_manifest_data: dict[str, object],
) -> None:
    manifest = GoldenValidationManifest.from_mapping(approved_manifest_data)

    request = next(
        case.masked_request()
        for case in manifest.cases
        if case.case_id == "case-pathogenic"
    )
    observation = request.source_observations[0]

    assert observation.fields == {"record_id": "VCV000000001"}
    assert request.variant == "NM_000059.4:c.7008-1G>A"


def test_manifest_rejects_unknown_fields(
    approved_manifest_data: dict[str, object],
) -> None:
    invalid = deepcopy(approved_manifest_data)
    invalid["unreviewed_shortcut"] = True

    with pytest.raises(GoldenFixtureError, match="unknown field"):
        GoldenValidationManifest.from_mapping(invalid)


def test_pure_metrics_report_exact_adjacent_opposite_and_criterion_rates(
    approved_manifest_data: dict[str, object],
) -> None:
    manifest = GoldenValidationManifest.from_mapping(approved_manifest_data)
    outcomes = (
        CaseValidationOutcome(
            case_id="case-pathogenic",
            classification=ClassificationTier.LIKELY_PATHOGENIC,
            criterion_statuses={
                "PS3": CriterionStatus.APPLIED,
                "BP4": CriterionStatus.NOT_EVALUABLE,
            },
            normalization_failed=False,
        ),
        CaseValidationOutcome(
            case_id="case-vus",
            classification=ClassificationTier.BENIGN,
            criterion_statuses={
                "PS3": CriterionStatus.APPLIED,
                "BP4": CriterionStatus.NOT_APPLIED,
            },
            normalization_failed=False,
        ),
        CaseValidationOutcome(
            case_id="case-benign",
            classification=ClassificationTier.BENIGN,
            criterion_statuses={
                "PS3": CriterionStatus.NOT_APPLIED,
                "BP4": CriterionStatus.NOT_APPLIED,
            },
            normalization_failed=False,
        ),
        CaseValidationOutcome(
            case_id="case-normalization-failure",
            classification=None,
            criterion_statuses={},
            normalization_failed=True,
        ),
    )

    metrics = calculate_validation_metrics(manifest, outcomes)

    assert metrics.exact_five_tier_concordance.numerator == 1
    assert metrics.exact_five_tier_concordance.denominator == 3
    assert metrics.adjacent_disagreement.numerator == 1
    assert metrics.opposite_direction_disagreement.numerator == 1
    assert metrics.normalization_failure_rate.numerator == 1
    assert metrics.normalization_failure_rate.denominator == 4
    assert metrics.exclusion_count == 1
    assert metrics.criterion_metrics["PS3"].true_positive == 1
    assert metrics.criterion_metrics["PS3"].false_positive == 1
    assert metrics.criterion_metrics["PS3"].false_negative == 1
    assert metrics.criterion_metrics["PS3"].precision.rate == pytest.approx(0.5)
    assert metrics.criterion_metrics["PS3"].recall.rate == pytest.approx(0.5)
    assert metrics.criterion_metrics["PS3"].f1.rate == pytest.approx(0.5)
    assert metrics.criterion_metrics["BP4"].not_evaluable_rate.numerator == 1
    assert metrics.criterion_metrics["BP4"].not_evaluable_rate.denominator == 3


def test_report_is_deterministic_and_synthetic_fixture_is_release_blocked() -> None:
    manifest_path = (
        Path(__file__).parents[2]
        / "fixtures"
        / "golden"
        / "synthetic_smoke_manifest.json"
    )
    manifest = load_golden_manifest(manifest_path)
    metrics = calculate_validation_metrics(
        manifest,
        (
            CaseValidationOutcome(
                case_id="synthetic-smoke-001",
                classification=ClassificationTier.PATHOGENIC,
                criterion_statuses={"PVS1": CriterionStatus.APPLIED},
                normalization_failed=False,
            ),
        ),
    )

    report = build_validation_report(manifest, metrics)
    rendered = render_report(report)

    assert report.release_eligible is False
    assert (
        "provenance-approved independently curated scientific case set"
        in report.release_blockers[0]
    )
    rendered_report = json.loads(rendered)
    assert rendered_report["provenance"]["approval_status"] == "missing"
    assert rendered_report["exclusions"] == [
        {
            "case_id": "synthetic-excluded-001",
            "reason": (
                "Synthetic smoke fixtures are not independently curated clinical cases."
            ),
        }
    ]
    assert rendered_report["source_classification_masking"] == [
        {"field": "classification", "source_id": "clinvar"}
    ]
    assert {
        "exact_five_tier_concordance",
        "adjacent_disagreement",
        "opposite_direction_disagreement",
        "criterion_metrics",
        "normalization_failure_rate",
        "not_evaluable_rate",
    } <= set(rendered_report["metrics"])
    assert rendered == render_report(report)
