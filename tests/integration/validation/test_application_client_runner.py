"""Task 10.2 application-client validation runner contracts."""

from __future__ import annotations

from dataclasses import dataclass, field
from pathlib import Path

import pytest

from acmg_classifier.domain.enums import ClassificationTier, CriterionStatus
from acmg_classifier.validation.fixtures import load_golden_manifest
from acmg_classifier.validation.runner import (
    ApplicationCaseResult,
    ApplicationClientRunner,
    GoldenValidationApplicationClient,
)


@dataclass
class RecordingApplicationClient:
    """A focused client double for the application-client port, never an MCP mock."""

    received_requests: list[object] = field(default_factory=list)

    async def classify_case(self, request: object) -> ApplicationCaseResult:
        self.received_requests.append(request)
        return ApplicationCaseResult(
            classification=ClassificationTier.PATHOGENIC,
            criterion_statuses={"PVS1": CriterionStatus.APPLIED},
            normalization_failed=False,
        )


@pytest.mark.asyncio
async def test_runner_masks_source_labels_before_execution() -> None:
    manifest = load_golden_manifest(
        Path(__file__).parents[2]
        / "fixtures"
        / "golden"
        / "synthetic_smoke_manifest.json"
    )
    client: GoldenValidationApplicationClient = RecordingApplicationClient()

    outcomes = await ApplicationClientRunner(client).run(manifest)

    assert [outcome.case_id for outcome in outcomes] == ["synthetic-smoke-001"]
    assert client.received_requests[0].source_observations[0].fields == {
        "record_id": "synthetic-clinvar-record"
    }


@pytest.mark.asyncio
async def test_runner_rejects_non_manifest_case() -> None:
    manifest = load_golden_manifest(
        Path(__file__).parents[2]
        / "fixtures"
        / "golden"
        / "synthetic_smoke_manifest.json"
    )

    class InvalidResultClient:
        async def classify_case(self, request: object) -> ApplicationCaseResult:
            return ApplicationCaseResult(
                classification=ClassificationTier.PATHOGENIC,
                criterion_statuses={},
                normalization_failed=False,
                case_id="a-different-case",
            )

    with pytest.raises(ValueError, match="case ID"):
        await ApplicationClientRunner(InvalidResultClient()).run(manifest)
