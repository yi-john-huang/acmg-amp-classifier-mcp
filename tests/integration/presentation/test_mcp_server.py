from __future__ import annotations

import json
from collections.abc import Mapping
from datetime import UTC, datetime
from typing import Never

import pytest

from acmg_classifier.application.bootstrap import BootstrapReport
from acmg_classifier.application.classification import (
    ClassificationRequest,
    FailedClassificationResponse,
)
from acmg_classifier.application.drafts import DraftAnswer
from acmg_classifier.application.feedback import FeedbackSubmission
from acmg_classifier.domain.enums import GenomeBuild, WorkflowStatus
from acmg_classifier.domain.errors import JsonValue
from acmg_classifier.domain.feedback import (
    FeedbackContext,
    FeedbackRecord,
    FeedbackType,
)
from acmg_classifier.presentation.mcp.server import create_server
from acmg_classifier.presentation.services import PresentationServices


class _Classifier:
    def __init__(self) -> None:
        self.requests: list[ClassificationRequest] = []

    async def classify(
        self, request: ClassificationRequest
    ) -> FailedClassificationResponse:
        self.requests.append(request)
        return FailedClassificationResponse(
            status=WorkflowStatus.FAILED,
            error_code="EVIDENCE_ACQUISITION_FAILED",
        )

    async def resume(
        self,
        resume_token: str,
        *,
        answers: tuple[DraftAnswer, ...],
    ) -> FailedClassificationResponse:
        raise AssertionError("resume is not expected")


class _Bootstrap:
    def ensure_ready(self) -> BootstrapReport:
        raise AssertionError("bootstrap is not expected")

    def doctor(self, *, repair: bool = False) -> BootstrapReport:
        raise AssertionError("doctor is not expected")


class _Replay:
    def replay(self, classification_id: str) -> Never:
        raise AssertionError("replay is not expected")


class _Feedback:
    def __init__(self) -> None:
        self.submissions: list[FeedbackSubmission] = []

    def submit(self, submission: FeedbackSubmission) -> FeedbackRecord:
        self.submissions.append(submission)
        return FeedbackRecord(
            feedback_id="fb_" + "a" * 32,
            submitted_at=datetime(2026, 7, 11, 12, 0, tzinfo=UTC),
            variant_key="cak1:GRCh38:NC_000001.11:100:A>G",
            context=FeedbackContext(genome_build=GenomeBuild.GRCH38),
            **submission.model_dump(),
        )

    def export(
        self, classification_id: str | None = None
    ) -> tuple[FeedbackRecord, ...]:
        raise AssertionError("feedback export is not expected")

    def import_records(self, records: tuple[FeedbackRecord, ...]) -> tuple[str, ...]:
        raise AssertionError("feedback import is not expected")


class _Resources:
    def get_evidence_snapshot(self, snapshot_id: str) -> Mapping[str, JsonValue]:
        return {"snapshot_id": snapshot_id, "evidence_ids": ["ev_test"]}

    def get_ruleset(
        self,
        ruleset_id: str,
        version: str,
    ) -> Mapping[str, JsonValue]:
        return {"id": ruleset_id, "version": version}

    def get_raw_snapshot(self, raw_snapshot_ref: str) -> bytes:
        return raw_snapshot_ref.encode()


class _Review:
    def __init__(self) -> None:
        self.calls = 0

    async def orchestrate_with_host(self, *_: object, **__: object) -> Never:
        self.calls += 1
        raise AssertionError("routine classification must not invoke review")


@pytest.mark.asyncio
async def test_primary_mcp_tools_expose_compact_schema_and_share_workflow_content() -> (
    None
):
    classifier = _Classifier()
    server = create_server(
        PresentationServices(
            classifier=classifier,
            bootstrap=_Bootstrap(),
            replay=_Replay(),
        )
    )

    tools = await server.list_tools()
    tool_names = {tool.name for tool in tools}

    assert {
        "classify_variant",
        "explain_classification",
        "submit_feedback",
    } <= tool_names
    classify = next(tool for tool in tools if tool.name == "classify_variant")
    assert "variant" in classify.inputSchema["properties"]
    assert "resume_token" in classify.inputSchema["properties"]
    assert "agent_review" in classify.inputSchema["properties"]

    result = await server.call_tool(
        "classify_variant",
        {
            "variant": "NC_000001.11:g.101A>G",
            "genome_build": "GRCh38",
            "interactive": False,
        },
    )

    assert isinstance(result, tuple)
    _, structured = result
    assert structured == {
        "schema_version": "1.0",
        "status": "failed",
        "classification": None,
        "error_code": "EVIDENCE_ACQUISITION_FAILED",
        "limitations": [],
    }
    assert classifier.requests[0].context.genome_build is GenomeBuild.GRCH38


@pytest.mark.asyncio
async def test_agent_review_opt_in_skips_routine_workflow_without_host_call() -> None:
    review = _Review()
    server = create_server(
        PresentationServices(
            classifier=_Classifier(),
            bootstrap=_Bootstrap(),
            replay=_Replay(),
            review=review,
        )
    )

    result = await server.call_tool(
        "classify_variant",
        {
            "variant": "NC_000001.11:g.101A>G",
            "genome_build": "GRCh38",
            "interactive": False,
            "agent_review": True,
        },
    )

    assert isinstance(result, tuple)
    _, structured = result
    assert structured["optional_review"] == {"status": "not_recommended"}
    assert review.calls == 0


@pytest.mark.asyncio
async def test_submit_feedback_uses_the_shared_append_only_service() -> None:
    feedback = _Feedback()
    server = create_server(
        PresentationServices(
            classifier=_Classifier(),
            bootstrap=_Bootstrap(),
            replay=_Replay(),
            feedback=feedback,
        )
    )

    result = await server.call_tool(
        "submit_feedback",
        {
            "classification_id": "cls_" + "b" * 32,
            "feedback_type": "correction",
            "proposed_correction": "Likely Pathogenic",
            "rationale": "Expert-reviewed segregation supports a correction.",
            "evidence_references": ["PMID:12345"],
            "actor_id": "usr_scientist-1",
        },
    )

    assert isinstance(result, tuple)
    _, structured = result
    assert structured["status"] == "completed"
    assert structured["feedback_id"] == "fb_" + "a" * 32
    assert feedback.submissions[0].feedback_type is FeedbackType.CORRECTION
    assert feedback.submissions[0].classification_id == "cls_" + "b" * 32


@pytest.mark.asyncio
async def test_resources_are_identifier_scoped_and_advanced_tools_are_opt_in() -> None:
    services = PresentationServices(
        classifier=_Classifier(),
        bootstrap=_Bootstrap(),
        replay=_Replay(),
        resources=_Resources(),
    )
    routine = create_server(services)
    advanced = create_server(services, advanced=True)

    routine_templates = await routine.list_resource_templates()
    advanced_templates = await advanced.list_resource_templates()
    routine_template_uris = {template.uriTemplate for template in routine_templates}
    advanced_template_uris = {template.uriTemplate for template in advanced_templates}
    routine_tools = {tool.name for tool in await routine.list_tools()}
    advanced_tools = {tool.name for tool in await advanced.list_tools()}
    evidence_content = tuple(await routine.read_resource("acmg://evidence/es_test"))
    raw_content = tuple(await advanced.read_resource("acmg://raw/raw_test"))

    assert "acmg://classifications/{classification_id}" in routine_template_uris
    assert "acmg://evidence/{snapshot_id}" in routine_template_uris
    assert "acmg://rulesets/{ruleset_id}/{version}" in routine_template_uris
    assert "acmg://raw/{raw_snapshot_ref}" not in routine_template_uris
    assert "acmg://raw/{raw_snapshot_ref}" in advanced_template_uris
    assert "get_raw_snapshot" not in routine_tools
    assert "get_raw_snapshot" in advanced_tools
    assert json.loads(evidence_content[0].content) == {
        "evidence_ids": ["ev_test"],
        "snapshot_id": "es_test",
    }
    assert json.loads(raw_content[0].content)["content"] == "cmF3X3Rlc3Q="
