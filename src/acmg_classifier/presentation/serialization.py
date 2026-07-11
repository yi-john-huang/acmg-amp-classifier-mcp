"""Shared deterministic CLI/MCP serialization for application workflow values."""

from __future__ import annotations

from collections.abc import Mapping
from dataclasses import asdict
from typing import cast

from acmg_classifier.application.bootstrap import BootstrapReport
from acmg_classifier.application.classification import (
    ClassificationWorkflowResponse,
    CompletedClassificationResponse,
    ConflictClassificationResponse,
    DegradedClassificationResponse,
    FailedClassificationResponse,
    NeedsContextClassificationResponse,
    UnsupportedClassificationResponse,
)
from acmg_classifier.application.explanation import (
    ExplanationDetail,
    ExplanationService,
)
from acmg_classifier.domain.combination import ClassificationDecision
from acmg_classifier.domain.errors import JsonValue


def workflow_content(response: ClassificationWorkflowResponse) -> dict[str, JsonValue]:
    """Serialize every workflow state with the same schema used by both adapters."""
    common: dict[str, JsonValue] = {
        "schema_version": "1.0",
        "status": response.status.value,
        "limitations": list(response.limitations),
    }
    if isinstance(response, CompletedClassificationResponse):
        classification = response.decision.classification
        if classification is None:
            raise ValueError(
                "completed classification response requires a classification"
            )
        return {
            **common,
            "classification_id": response.classification_id,
            "classification": classification.value,
            "normalized_variant": response.normalized_variant.to_canonical_content(),
            "context": _context_content(response.context),
            "decision": response.decision.to_canonical_content(),
            "explanation": response.explanation.to_canonical_content(),
            "snapshot_id": response.snapshot_id,
            "ruleset": {
                "id": response.ruleset.ruleset_id,
                "version": response.ruleset.version,
            },
        }
    if isinstance(response, NeedsContextClassificationResponse):
        return {
            **common,
            "classification": None,
            "normalized_variant": response.normalized_variant.to_canonical_content(),
            "draft_id": response.draft_id,
            "resume_token": response.resume_token,
            "questions": [
                question.to_canonical_content() for question in response.questions
            ],
        }
    if isinstance(response, DegradedClassificationResponse):
        return {
            **common,
            "classification": (
                response.decision.classification.value
                if response.decision.classification is not None
                else None
            ),
            "normalized_variant": response.normalized_variant.to_canonical_content(),
            "context": _context_content(response.context),
            "decision": response.decision.to_canonical_content(),
            "explanation": response.explanation.to_canonical_content(),
            "snapshot_id": response.snapshot_id,
            "unavailable_sources": list(response.unavailable_sources),
        }
    if isinstance(response, ConflictClassificationResponse):
        conflict = response.decision.conflict
        return {
            **common,
            "classification": None,
            "normalized_variant": response.normalized_variant.to_canonical_content(),
            "context": _context_content(response.context),
            "decision": response.decision.to_canonical_content(),
            "explanation": response.explanation.to_canonical_content(),
            "snapshot_id": response.snapshot_id,
            "conflict": (
                conflict.to_canonical_content() if conflict is not None else None
            ),
        }
    if isinstance(response, UnsupportedClassificationResponse):
        return {**common, "classification": None, "reason": response.reason}
    if isinstance(response, FailedClassificationResponse):
        return {**common, "classification": None, "error_code": response.error_code}
    raise TypeError("unknown classification workflow response")


def stored_explanation_content(
    content: Mapping[str, JsonValue],
    *,
    detail: ExplanationDetail,
) -> dict[str, JsonValue]:
    """Render a selected detail level from a stored immutable decision only."""
    decision_content = content.get("decision")
    if not isinstance(decision_content, Mapping):
        raise ValueError("stored classification lacks a decision")
    try:
        decision = ClassificationDecision.model_validate(decision_content)
    except ValueError as error:
        raise ValueError("stored classification decision is invalid") from error
    return (
        ExplanationService()
        .render_decision(
            decision,
            detail=detail,
        )
        .to_canonical_content()
    )


def bootstrap_content(report: BootstrapReport) -> dict[str, JsonValue]:
    """Serialize diagnostics without leaking internal exceptions or credentials."""
    state_database = (
        asdict(report.state_database) if report.state_database is not None else None
    )
    issue: dict[str, JsonValue] | None = None
    if report.issue is not None:
        issue = {
            "code": report.issue.code.value,
            "message": report.issue.message,
            "repair_action": report.issue.repair_action,
            "retryable": report.issue.retryable,
            "component": report.issue.component,
            "details": list(report.issue.details),
        }
    return {
        "schema_version": "1.0",
        "ready": report.ready,
        "bundle_version": report.bundle_version,
        "state_database": cast(JsonValue, state_database),
        "issue": issue,
    }


def _context_content(context: object) -> dict[str, JsonValue]:
    genome_build = getattr(context, "genome_build", None)
    inheritance = getattr(context, "inheritance", None)
    return {
        "genome_build": genome_build.value if genome_build is not None else None,
        "transcript": getattr(context, "transcript", None),
        "disease_id": getattr(context, "disease_id", None),
        "disease_label": getattr(context, "disease_label", None),
        "inheritance": inheritance.value if inheritance is not None else None,
    }
