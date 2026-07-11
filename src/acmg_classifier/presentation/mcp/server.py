"""Official MCP stdio adapter for the shared classification workflow."""

from __future__ import annotations

import base64
import json
from typing import Any

from mcp.server.fastmcp import Context, FastMCP
from pydantic import BaseModel, ConfigDict, model_validator

from acmg_classifier.application.classification import (
    ClassificationRequest,
    CompletedClassificationResponse,
    ConflictClassificationResponse,
)
from acmg_classifier.application.drafts import DraftAnswer, DraftAnswerState
from acmg_classifier.application.explanation import ExplanationDetail
from acmg_classifier.application.feedback import FeedbackSubmission
from acmg_classifier.domain.context import ContextField
from acmg_classifier.domain.enums import AnalysisIntent, GenomeBuild, InheritanceMode
from acmg_classifier.domain.evidence import EvidencePolicy, EvidencePolicyMode
from acmg_classifier.domain.feedback import FeedbackType
from acmg_classifier.domain.models import InterpretationContext
from acmg_classifier.presentation.mcp.review import McpSamplingHostAgent
from acmg_classifier.presentation.serialization import (
    stored_explanation_content,
    workflow_content,
)
from acmg_classifier.presentation.services import PresentationServices, default_services


class ResumeAnswerInput(BaseModel):
    """Validated one-question continuation input for the MCP boundary."""

    model_config = ConfigDict(extra="forbid", strict=True)

    field: ContextField
    state: DraftAnswerState
    value: str | int | float | bool | dict[str, Any] | list[Any] | None = None

    @model_validator(mode="after")
    def validate_state_value(self) -> ResumeAnswerInput:
        DraftAnswer(field=self.field, state=self.state, value=self.value)
        return self

    def to_domain(self) -> DraftAnswer:
        """Return the validated application continuation answer."""
        return DraftAnswer(field=self.field, state=self.state, value=self.value)


def create_server(
    services: PresentationServices | None = None,
    *,
    advanced: bool = False,
) -> FastMCP:
    """Build the stdio server without leaking SDK types into application services."""
    server = FastMCP(
        name="acmg-amp-classifier",
        instructions=(
            "Research-use ACMG/AMP classification. Use classify_variant for one "
            "workflow, explain_classification for immutable stored content, and "
            "submit_feedback for non-authoritative feedback."
        ),
    )

    @server.tool(
        name="classify_variant",
        description="Classify one variant or resume a typed context continuation.",
    )
    async def classify_variant(
        ctx: Context,  # type: ignore[type-arg]
        variant: str | None = None,
        genome_build: GenomeBuild | None = None,
        transcript: str | None = None,
        disease_id: str | None = None,
        disease_label: str | None = None,
        inheritance: InheritanceMode | None = None,
        analysis_intent: AnalysisIntent = AnalysisIntent.GERMLINE_MENDELIAN,
        offline: bool = False,
        interactive: bool = True,
        agent_review: bool = False,
        resume_token: str | None = None,
        answers: list[ResumeAnswerInput] | None = None,
    ) -> dict[str, Any]:
        """Execute one application workflow without reimplementing its decisions."""
        await _report_progress(ctx, 0, 100, "starting classification")
        if services is None:
            return _runtime_unavailable()
        try:
            if resume_token is not None:
                if variant is not None:
                    raise ValueError("variant must not accompany a resume token")
                if not answers:
                    raise ValueError("resume requires at least one answer")
                response = await services.classifier.resume(
                    resume_token,
                    answers=tuple(answer.to_domain() for answer in answers),
                )
            else:
                if variant is None or not variant.strip():
                    raise ValueError("variant is required")
                if (disease_id is None) != (disease_label is None):
                    raise ValueError(
                        "disease_id and disease_label must be supplied together"
                    )
                response = await services.classifier.classify(
                    ClassificationRequest(
                        variant=variant,
                        context=InterpretationContext(
                            genome_build=genome_build,
                            transcript=transcript,
                            disease_id=disease_id,
                            disease_label=disease_label,
                            inheritance=inheritance,
                        ),
                        evidence_policy=EvidencePolicy(
                            mode=(
                                EvidencePolicyMode.OFFLINE
                                if offline
                                else EvidencePolicyMode.LIVE
                            )
                        ),
                        analysis_intent=analysis_intent,
                        interactive=interactive,
                    )
                )
        except ValueError as error:
            return _invalid_input(str(error))
        await _report_progress(ctx, 100, 100, "classification complete")
        payload = workflow_content(response)
        if not agent_review:
            return payload
        packet = (
            response.review_packet
            if isinstance(
                response,
                (CompletedClassificationResponse, ConflictClassificationResponse),
            )
            else None
        )
        if packet is None:
            payload["optional_review"] = {"status": "not_recommended"}
            return payload
        if services.review is None:
            payload["optional_review"] = {"status": "disabled"}
            return payload
        try:
            attempt = await services.review.orchestrate_with_host(
                packet,
                host=McpSamplingHostAgent(ctx),
            )
        except Exception:
            payload["optional_review"] = {"status": "host_failure"}
            return payload
        payload["optional_review"] = {
            "status": attempt.status.value,
            "review_id": attempt.review_id,
        }
        return payload

    @server.tool(
        name="explain_classification",
        description="Read the stored explanation for one immutable classification.",
    )
    async def explain_classification(
        ctx: Context,  # type: ignore[type-arg]
        classification_id: str,
        detail: ExplanationDetail = ExplanationDetail.STANDARD,
    ) -> dict[str, Any]:
        """Return stored display content without source queries or rule evaluation."""
        await _report_progress(ctx, 0, 100, "loading stored classification")
        if services is None:
            return _runtime_unavailable()
        try:
            replay = services.replay.replay(classification_id)
            content = replay.to_canonical_content()
        except Exception:
            return _failed("CLASSIFICATION_NOT_FOUND")
        try:
            explanation = stored_explanation_content(content, detail=detail)
        except ValueError:
            return _failed("EXPLANATION_UNAVAILABLE")
        await _report_progress(ctx, 100, 100, "explanation loaded")
        return {
            "schema_version": "1.0",
            "status": "completed",
            "classification_id": classification_id,
            "explanation": explanation,
        }

    @server.tool(
        name="submit_feedback",
        description="Append non-authoritative feedback to one stored classification.",
    )
    async def submit_feedback(
        ctx: Context,  # type: ignore[type-arg]
        classification_id: str,
        feedback_type: FeedbackType,
        rationale: str,
        actor_id: str,
        proposed_correction: str | None = None,
        evidence_references: list[str] | None = None,
    ) -> dict[str, Any]:
        """Append feedback through the non-authoritative application service."""
        await _report_progress(ctx, 0, 100, "saving feedback")
        if services is None:
            return _runtime_unavailable()
        if services.feedback is None:
            return _failed("FEEDBACK_UNAVAILABLE")
        try:
            record = services.feedback.submit(
                FeedbackSubmission(
                    classification_id=classification_id,
                    feedback_type=feedback_type,
                    proposed_correction=proposed_correction,
                    rationale=rationale,
                    evidence_references=tuple(evidence_references or ()),
                    actor_id=actor_id,
                )
            )
        except Exception:
            return _failed("INVALID_FEEDBACK")
        await _report_progress(ctx, 100, 100, "feedback saved")
        return {
            "schema_version": "1.0",
            "status": "completed",
            "feedback_id": record.feedback_id,
            "feedback": record.to_canonical_content(),
        }

    @server.resource(
        "acmg://classifications/{classification_id}",
        name="classification",
        description="Immutable stored classification content by ID.",
        mime_type="application/json",
    )
    def classification_resource(classification_id: str) -> str:
        """Expose one stored record only when a caller names its identifier."""
        if services is None:
            return _resource_json(_runtime_unavailable())
        try:
            content = services.replay.replay(classification_id).to_canonical_content()
        except Exception:
            return _resource_json(_failed("CLASSIFICATION_NOT_FOUND"))
        return _resource_json(content)

    @server.resource(
        "acmg://evidence/{snapshot_id}",
        name="evidence_snapshot",
        description="Immutable evidence snapshot metadata by ID.",
        mime_type="application/json",
    )
    def evidence_resource(snapshot_id: str) -> str:
        """Expose snapshot metadata without embedding it in routine tool responses."""
        if services is None or services.resources is None:
            return _resource_json(_failed("RESOURCE_UNAVAILABLE"))
        try:
            content = services.resources.get_evidence_snapshot(snapshot_id)
        except Exception:
            return _resource_json(_failed("EVIDENCE_SNAPSHOT_NOT_FOUND"))
        return _resource_json(dict(content))

    @server.resource(
        "acmg://rulesets/{ruleset_id}/{version}",
        name="ruleset",
        description="Versioned ruleset content by explicit identity.",
        mime_type="application/json",
    )
    def ruleset_resource(ruleset_id: str, version: str) -> str:
        """Expose a ruleset only by stable ID and version."""
        if services is None or services.resources is None:
            return _resource_json(_failed("RESOURCE_UNAVAILABLE"))
        try:
            content = services.resources.get_ruleset(ruleset_id, version)
        except Exception:
            return _resource_json(_failed("RULESET_NOT_FOUND"))
        return _resource_json(dict(content))

    if advanced:
        @server.resource(
            "acmg://raw/{raw_snapshot_ref}",
            name="raw_snapshot",
            description="Raw source payload by explicit content-addressed reference.",
            mime_type="application/json",
        )
        def raw_resource(raw_snapshot_ref: str) -> str:
            """Expose raw content only under an explicit identifier-scoped URI."""
            return _resource_json(_raw_snapshot_content(services, raw_snapshot_ref))

        _register_advanced_tools(server, services)
    return server


def main() -> None:
    """Run the official MCP stdio server with the default local composition."""
    create_server(default_services()).run(transport="stdio")


def _runtime_unavailable() -> dict[str, Any]:
    return _failed("RUNTIME_UNAVAILABLE")


def _invalid_input(message: str) -> dict[str, Any]:
    return {
        "schema_version": "1.0",
        "status": "failed",
        "classification": None,
        "error_code": "INVALID_REQUEST",
        "limitations": [message],
    }


async def _report_progress(
    ctx: Context,  # type: ignore[type-arg]
    progress: float,
    total: float,
    message: str,
) -> None:
    """Emit progress only when the SDK provides an active request context."""
    try:
        await ctx.report_progress(progress, total, message)
    except ValueError:
        # Direct in-process contract calls intentionally have no request context.
        return


def _resource_json(content: object) -> str:
    """Encode resource content deterministically without emitting server logs."""
    return json.dumps(content, sort_keys=True, separators=(",", ":"))


def _raw_snapshot_content(
    services: PresentationServices | None,
    raw_snapshot_ref: str,
) -> dict[str, Any]:
    """Load raw source content only by explicit identifier and within a hard bound."""
    if services is None or services.resources is None:
        return _failed("RESOURCE_UNAVAILABLE")
    try:
        content = services.resources.get_raw_snapshot(raw_snapshot_ref)
    except Exception:
        return _failed("RAW_SNAPSHOT_NOT_FOUND")
    if len(content) > 4 * 1024 * 1024:
        return _failed("RAW_SNAPSHOT_TOO_LARGE")
    return {
        "schema_version": "1.0",
        "status": "completed",
        "raw_snapshot_ref": raw_snapshot_ref,
        "encoding": "base64",
        "content": base64.b64encode(content).decode("ascii"),
    }


def _failed(error_code: str) -> dict[str, Any]:
    return {
        "schema_version": "1.0",
        "status": "failed",
        "classification": None,
        "error_code": error_code,
        "limitations": [],
    }


def _register_advanced_tools(
    server: FastMCP,
    services: PresentationServices | None,
) -> None:
    """Register raw-content access only when an explicit advanced mode is enabled."""

    @server.tool(
        name="get_raw_snapshot",
        description="Load one raw source payload by its explicit snapshot reference.",
    )
    async def get_raw_snapshot(
        ctx: Context,  # type: ignore[type-arg]
        raw_snapshot_ref: str,
    ) -> dict[str, Any]:
        await _report_progress(ctx, 0, 100, "loading raw snapshot")
        content = _raw_snapshot_content(services, raw_snapshot_ref)
        if content["status"] == "completed":
            await _report_progress(ctx, 100, 100, "raw snapshot loaded")
        return content
