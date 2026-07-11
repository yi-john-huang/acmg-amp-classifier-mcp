"""Presentation-facing application service contracts and explicit composition seam."""

from __future__ import annotations

from collections.abc import Mapping
from dataclasses import dataclass
from typing import Protocol

from acmg_classifier.application.bootstrap import BootstrapReport
from acmg_classifier.application.classification import (
    ClassificationRequest,
    ClassificationWorkflowResponse,
)
from acmg_classifier.application.drafts import DraftAnswer
from acmg_classifier.application.feedback import FeedbackRecord, FeedbackSubmission
from acmg_classifier.application.reinterpretation import ReplayedClassification
from acmg_classifier.application.review import (
    HostAgentPort,
    ReviewAttempt,
    ReviewPacket,
)
from acmg_classifier.domain.errors import JsonValue


class ClassificationWorkflow(Protocol):
    """The narrow workflow surface shared by CLI and MCP adapters."""

    async def classify(
        self,
        request: ClassificationRequest,
    ) -> ClassificationWorkflowResponse: ...

    async def resume(
        self,
        resume_token: str,
        *,
        answers: tuple[DraftAnswer, ...],
    ) -> ClassificationWorkflowResponse: ...


class BootstrapWorkflow(Protocol):
    """The user-facing first-use and diagnostics surface."""

    def ensure_ready(self) -> BootstrapReport: ...

    def doctor(self, *, repair: bool = False) -> BootstrapReport: ...


class ReplayWorkflow(Protocol):
    """The read-only explanation/replay boundary."""

    def replay(self, classification_id: str) -> ReplayedClassification: ...


class FeedbackWorkflow(Protocol):
    """The append-only feedback surface exposed by CLI and MCP adapters."""

    def submit(self, submission: FeedbackSubmission) -> FeedbackRecord: ...

    def export(
        self,
        classification_id: str | None = None,
    ) -> tuple[FeedbackRecord, ...]: ...

    def import_records(
        self, records: tuple[FeedbackRecord, ...]
    ) -> tuple[str, ...]: ...


class ResourceWorkflow(Protocol):
    """Explicit identifier-based retrieval of immutable non-routine content."""

    def get_evidence_snapshot(self, snapshot_id: str) -> Mapping[str, JsonValue]: ...

    def get_ruleset(
        self,
        ruleset_id: str,
        version: str,
    ) -> Mapping[str, JsonValue]: ...

    def get_raw_snapshot(self, raw_snapshot_ref: str) -> bytes: ...


class OptionalReviewWorkflow(Protocol):
    """Request-scoped, non-authoritative specialist review orchestration."""

    async def orchestrate_with_host(
        self,
        packet: ReviewPacket | None,
        *,
        host: HostAgentPort | None,
    ) -> ReviewAttempt: ...


@dataclass(frozen=True, slots=True)
class PresentationServices:
    """Explicitly composed application services for one presentation process."""

    classifier: ClassificationWorkflow
    bootstrap: BootstrapWorkflow
    replay: ReplayWorkflow
    feedback: FeedbackWorkflow | None = None
    resources: ResourceWorkflow | None = None
    review: OptionalReviewWorkflow | None = None


class RuntimeConfigurationError(RuntimeError):
    """No real application composition has been configured for this process."""


def default_services() -> PresentationServices:
    """Reject startup honestly until a real runtime composition is installed."""
    raise RuntimeConfigurationError(
        "No application runtime is configured; install a signed compatible data bundle "
        "and configure the application composition."
    )
