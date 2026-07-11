"""Application-client port for executing golden validation cases.

The runner has no MCP dependency and deliberately does not know about legacy mock
clients. Runtime composition supplies an application client that evaluates the
label-masked request through the real workflow.
"""

from __future__ import annotations

from collections.abc import Mapping, Sequence
from dataclasses import dataclass
from typing import Protocol

from acmg_classifier.domain.enums import ClassificationTier, CriterionStatus
from acmg_classifier.validation.fixtures import (
    GoldenCaseRequest,
    GoldenValidationManifest,
)
from acmg_classifier.validation.metrics import CaseValidationOutcome


@dataclass(frozen=True, slots=True)
class ApplicationCaseResult:
    """Normalized real-application result required for validation comparison."""

    classification: ClassificationTier | None
    criterion_statuses: Mapping[str, CriterionStatus]
    normalization_failed: bool
    case_id: str | None = None


class GoldenValidationApplicationClient(Protocol):
    """Port implemented by an adapter over the real application workflow."""

    async def classify_case(self, request: GoldenCaseRequest) -> ApplicationCaseResult:
        """Classify the masked fixture request without consulting source labels."""


class ApplicationClientRunner:
    """Execute every non-excluded case through an injected real application client."""

    def __init__(self, client: GoldenValidationApplicationClient) -> None:
        self._client = client

    async def run(
        self, manifest: GoldenValidationManifest
    ) -> Sequence[CaseValidationOutcome]:
        """Run a manifest in deterministic case-ID order and preserve all outcomes."""
        outcomes: list[CaseValidationOutcome] = []
        for case in manifest.cases:
            result = await self._client.classify_case(case.masked_request())
            if result.case_id is not None and result.case_id != case.case_id:
                raise ValueError(
                    "application client returned a result for a different case ID"
                )
            outcomes.append(
                CaseValidationOutcome(
                    case_id=case.case_id,
                    classification=result.classification,
                    criterion_statuses=result.criterion_statuses,
                    normalization_failed=result.normalization_failed,
                )
            )
        return tuple(outcomes)
