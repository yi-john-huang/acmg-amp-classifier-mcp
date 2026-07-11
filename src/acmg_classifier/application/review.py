"""Typed, bounded review recommendations outside deterministic classification."""

from __future__ import annotations

import asyncio
import json
from collections.abc import Callable, Iterable, Mapping
from dataclasses import dataclass
from datetime import datetime
from enum import StrEnum
from types import MappingProxyType
from typing import Literal, Protocol, cast

from acmg_classifier.application.explanation import ExplanationDetail
from acmg_classifier.domain.canonical import canonical_hash
from acmg_classifier.domain.combination import ClassificationDecision
from acmg_classifier.domain.evidence import (
    CaseControlObservation,
    EvidenceDerivation,
    EvidenceItem,
    FunctionalObservation,
    Observation,
    ObservationKind,
    QualityFlag,
    SourceProvenance,
)

DEFAULT_REVIEW_TOKEN_CAP = 1200


class ReviewTaskType(StrEnum):
    """The only specialist tasks that can be recommended."""

    EVIDENCE_CONFLICT = "evidence_conflict"
    LITERATURE_SYNTHESIS = "literature_synthesis"
    EXPLANATION_REVIEW = "explanation_review"


class ReviewReason(StrEnum):
    """Stable, non-authoritative reasons for specialist review."""

    UNRESOLVED_CONFLICT = "unresolved_conflict"
    CITED_STUDY_SYNTHESIS = "cited_study_synthesis"
    FULL_EXPLANATION_REQUESTED = "full_explanation_requested"


class ReviewOutputSchema(StrEnum):
    """Expected specialist response shape for a recommendation."""

    CONFLICT_SUMMARY_V1 = "review.conflict-summary.v1"
    LITERATURE_SYNTHESIS_V1 = "review.literature-synthesis.v1"
    EXPLANATION_SUGGESTIONS_V1 = "review.explanation-suggestions.v1"


@dataclass(frozen=True, slots=True)
class ReviewRecommendation:
    """One optional task recommendation; it has no classification authority."""

    task_type: ReviewTaskType
    reason: ReviewReason
    evidence_ids: tuple[str, ...]
    requested_output_schema: ReviewOutputSchema
    token_cap: int = DEFAULT_REVIEW_TOKEN_CAP

    def __post_init__(self) -> None:
        _validate_recommendation_shape(
            task_type=self.task_type,
            reason=self.reason,
            requested_output_schema=self.requested_output_schema,
            token_cap=self.token_cap,
        )
        object.__setattr__(
            self, "evidence_ids", _normalized_evidence_ids(self.evidence_ids)
        )


@dataclass(frozen=True, slots=True)
class CompactEvidenceObservation:
    """A safe evidence projection that deliberately omits source payload handles."""

    evidence_id: str
    kind: ObservationKind
    observation: Observation
    derivation: EvidenceDerivation
    source_id: str | None
    quality_flags: tuple[QualityFlag, ...]

    def __post_init__(self) -> None:
        if not self.evidence_id:
            raise ValueError("compact observation requires an evidence ID")
        if self.kind is not self.observation.kind:
            raise ValueError("compact observation kind must match its observation")
        object.__setattr__(
            self, "quality_flags", tuple(sorted(set(self.quality_flags)))
        )


@dataclass(frozen=True, slots=True)
class ReviewPacket:
    """Immutable minimum context for a non-authoritative specialist task."""

    task_type: ReviewTaskType
    reason: ReviewReason
    classification_id: str | None
    input_hash: str
    request: Mapping[str, object]
    snapshot_id: str
    ruleset_id: str
    ruleset_version: str
    selected_evidence_ids: tuple[str, ...]
    observations: tuple[CompactEvidenceObservation, ...]
    requested_output_schema: ReviewOutputSchema
    token_cap: int = DEFAULT_REVIEW_TOKEN_CAP
    classification_override_prohibited: Literal[True] = True

    def __post_init__(self) -> None:
        _validate_recommendation_shape(
            task_type=self.task_type,
            reason=self.reason,
            requested_output_schema=self.requested_output_schema,
            token_cap=self.token_cap,
        )
        if self.classification_id is not None and not self.classification_id:
            raise ValueError("review packet classification ID must not be empty")
        if not self.snapshot_id:
            raise ValueError("review packet requires a snapshot ID")
        if not self.ruleset_id or not self.ruleset_version:
            raise ValueError("review packet requires a ruleset identity")
        if not self.input_hash:
            raise ValueError("review packet requires an input hash")
        if self.classification_override_prohibited is not True:
            raise ValueError("review packets must prohibit classification overrides")

        evidence_ids = _normalized_evidence_ids(self.selected_evidence_ids)
        observations = tuple(
            sorted(self.observations, key=lambda item: item.evidence_id)
        )
        observation_ids = tuple(item.evidence_id for item in observations)
        if observation_ids != evidence_ids:
            raise ValueError(
                "review packet observations must exactly match selected evidence IDs"
            )
        object.__setattr__(self, "selected_evidence_ids", evidence_ids)
        object.__setattr__(self, "observations", observations)
        object.__setattr__(self, "request", _freeze_mapping(self.request))


MAX_REVIEW_OUTPUT_BYTES = 32 * 1024


class ReviewAttemptStatus(StrEnum):
    """Stable outcomes for a non-authoritative review attempt."""

    NOT_RECOMMENDED = "not_recommended"
    DISABLED = "disabled"
    UNSUPPORTED = "unsupported"
    NOT_PERSISTABLE = "not_persistable"
    TIMED_OUT = "timed_out"
    HOST_FAILURE = "host_failure"
    INVALID_OUTPUT = "invalid_output"
    STORAGE_FAILURE = "storage_failure"
    STORED = "stored"


@dataclass(frozen=True, slots=True)
class HostAgentCapabilities:
    """Host capabilities relevant to optional specialist review."""

    sampling: bool


@dataclass(frozen=True, slots=True)
class HostAgentResponse:
    """SDK-free, byte-bounded response from a host sampling request."""

    content: bytes

    def __post_init__(self) -> None:
        if not isinstance(self.content, bytes):
            raise TypeError("host review content must be bytes")


class HostAgentPort(Protocol):
    """Optional host sampling boundary owned by the application layer."""

    async def get_capabilities(self) -> HostAgentCapabilities:
        """Report host support without initiating a review request."""

    async def request_review(self, packet: ReviewPacket) -> HostAgentResponse:
        """Request one bounded specialist review for an approved packet."""


class ReviewStoragePort(Protocol):
    """The minimal append-only persistence boundary for validated reviews."""

    def append_review(
        self, classification_id: str, review: Mapping[str, object]
    ) -> str:
        """Append an immutable review artifact linked to a classification."""


@dataclass(frozen=True, slots=True)
class SpecialistReviewOutput:
    """Schema-validated synthesis that cannot carry a deterministic override."""

    conclusion: str
    uncertainty: str
    questions: tuple[str, ...] = ()
    limitations: tuple[str, ...] = ()

    def __post_init__(self) -> None:
        if not self.conclusion or not self.uncertainty:
            raise ValueError("specialist review requires conclusion and uncertainty")
        if any(not value for value in self.questions + self.limitations):
            raise ValueError("specialist review text values must not be empty")

    def to_canonical_content(self) -> dict[str, object]:
        """Return the fixed, storage-safe schema for agent synthesis."""
        return {
            "conclusion": self.conclusion,
            "uncertainty": self.uncertainty,
            "questions": list(self.questions),
            "limitations": list(self.limitations),
        }


@dataclass(frozen=True, slots=True)
class ReviewArtifact:
    """Validated, non-authoritative review metadata safe for append-only storage."""

    classification_id: str
    task_type: ReviewTaskType
    reason: ReviewReason
    input_hash: str
    evidence_ids: tuple[str, ...]
    requested_output_schema: ReviewOutputSchema
    status: Literal["completed"]
    reviewer: str
    model: str
    timestamp: datetime
    output: SpecialistReviewOutput

    def __post_init__(self) -> None:
        if not self.classification_id:
            raise ValueError("review artifact requires a classification ID")
        if not self.reviewer or not self.model:
            raise ValueError("review artifact requires reviewer and model IDs")
        if self.timestamp.tzinfo is None or self.timestamp.utcoffset() is None:
            raise ValueError("review artifact timestamp must include an offset")
        object.__setattr__(
            self, "evidence_ids", _normalized_evidence_ids(self.evidence_ids)
        )

    def storage_record(self) -> dict[str, object]:
        """Return the complete JSON-safe artifact passed to append-only storage."""
        return {
            "schema_version": "1.0",
            "classification_id": self.classification_id,
            "task_type": self.task_type.value,
            "reason": self.reason.value,
            "input_hash": self.input_hash,
            "evidence_ids": list(self.evidence_ids),
            "status": self.status,
            "requested_output_schema": self.requested_output_schema.value,
            "reviewer": self.reviewer,
            "model": self.model,
            "timestamp": self.timestamp.isoformat(),
            "output": self.output.to_canonical_content(),
        }


@dataclass(frozen=True, slots=True)
class ReviewAttempt:
    """Outcome metadata which cannot influence deterministic classification."""

    status: ReviewAttemptStatus
    review: ReviewArtifact | None = None
    review_id: str | None = None


class ReviewOutputValidator:
    """Parse and validate a specialist response independently from host capability."""

    _REQUIRED_FIELDS = frozenset(
        {
            "schema_version",
            "task_type",
            "input_hash",
            "evidence_ids",
            "requested_output_schema",
            "status",
            "reviewer",
            "model",
            "timestamp",
            "output",
        }
    )
    _OVERRIDE_KEYS = frozenset(
        {
            "classification",
            "classificationid",
            "classificationoverride",
            "decision",
            "criteria",
            "criterion",
            "assessment",
            "assessments",
            "evidence",
            "evidenceid",
            "evidenceids",
        }
    )

    def validate(
        self, *, packet: ReviewPacket, response: HostAgentResponse
    ) -> ReviewArtifact:
        """Return an artifact only when strict JSON exactly matches the packet."""
        if len(response.content) > MAX_REVIEW_OUTPUT_BYTES:
            raise ValueError("review output exceeds maximum byte size")
        try:
            decoded = response.content.decode("utf-8")
            payload = json.loads(decoded, parse_constant=_reject_json_constant)
        except (UnicodeDecodeError, json.JSONDecodeError, ValueError) as error:
            raise ValueError("review output is not valid JSON") from error
        if not isinstance(payload, dict):
            raise ValueError("review output must be a JSON object")
        if set(payload) != self._REQUIRED_FIELDS:
            raise ValueError("review output has an invalid field set")
        self._reject_nested_overrides(payload["output"])

        if payload["schema_version"] != "1.0":
            raise ValueError("review output schema version is unsupported")
        task_type = _required_string(payload, "task_type")
        if task_type != packet.task_type.value:
            raise ValueError("review output task type does not match packet")
        requested_output_schema = _required_string(payload, "requested_output_schema")
        if requested_output_schema != packet.requested_output_schema.value:
            raise ValueError("review output schema does not match packet")
        input_hash = _required_string(payload, "input_hash")
        if input_hash != packet.input_hash:
            raise ValueError("review output input hash does not match packet")
        evidence_ids = _validated_evidence_ids(payload["evidence_ids"], packet)
        if payload["status"] != "completed":
            raise ValueError("review output status must be completed")
        output = _validated_specialist_output(payload["output"])
        timestamp = _parse_review_timestamp(_required_string(payload, "timestamp"))
        return ReviewArtifact(
            classification_id=_required_packet_classification_id(packet),
            task_type=packet.task_type,
            input_hash=input_hash,
            reason=packet.reason,
            evidence_ids=evidence_ids,
            status="completed",
            reviewer=_required_string(payload, "reviewer"),
            requested_output_schema=packet.requested_output_schema,
            model=_required_string(payload, "model"),
            timestamp=timestamp,
            output=output,
        )

    def _reject_nested_overrides(self, value: object) -> None:
        if isinstance(value, Mapping):
            for key, item in value.items():
                if not isinstance(key, str):
                    raise ValueError("review output keys must be strings")
                normalized_key = "".join(
                    character for character in key.lower() if character.isalnum()
                )
                if normalized_key in self._OVERRIDE_KEYS:
                    raise ValueError("review output attempts a prohibited override")
                self._reject_nested_overrides(item)
        elif isinstance(value, list):
            for item in value:
                self._reject_nested_overrides(item)


class ReviewOrchestrator:
    """Optionally request and persist a validated specialist review."""

    def __init__(
        self,
        *,
        host: HostAgentPort | None,
        store: ReviewStoragePort,
        validator: ReviewOutputValidator | None = None,
        timeout_seconds: float = 10.0,
    ) -> None:
        if timeout_seconds <= 0:
            raise ValueError("review timeout must be positive")
        self._host = host
        self._store = store
        self._validator = validator or ReviewOutputValidator()
        self._timeout_seconds = timeout_seconds

    async def orchestrate_with_host(
        self,
        packet: ReviewPacket | None,
        *,
        host: HostAgentPort | None,
    ) -> ReviewAttempt:
        """Run this configured policy against one request-scoped host adapter."""
        return await ReviewOrchestrator(
            host=host,
            store=self._store,
            validator=self._validator,
            timeout_seconds=self._timeout_seconds,
        ).orchestrate(packet)

    async def orchestrate(self, packet: ReviewPacket | None) -> ReviewAttempt:
        """Run one optional review without affecting classification semantics."""
        if packet is None:
            return ReviewAttempt(ReviewAttemptStatus.NOT_RECOMMENDED)
        if self._host is None:
            return ReviewAttempt(ReviewAttemptStatus.DISABLED)
        if packet.classification_id is None:
            return ReviewAttempt(ReviewAttemptStatus.NOT_PERSISTABLE)
        try:
            capabilities = await self._host.get_capabilities()
        except Exception:
            return ReviewAttempt(ReviewAttemptStatus.HOST_FAILURE)
        if not capabilities.sampling:
            return ReviewAttempt(ReviewAttemptStatus.UNSUPPORTED)
        try:
            response = await asyncio.wait_for(
                self._host.request_review(packet),
                timeout=self._timeout_seconds,
            )
        except TimeoutError:
            return ReviewAttempt(ReviewAttemptStatus.TIMED_OUT)
        except Exception:
            return ReviewAttempt(ReviewAttemptStatus.HOST_FAILURE)
        try:
            review = self._validator.validate(packet=packet, response=response)
        except (TypeError, ValueError):
            return ReviewAttempt(ReviewAttemptStatus.INVALID_OUTPUT)
        try:
            review_id = self._store.append_review(
                packet.classification_id,
                review.storage_record(),
            )
        except Exception:
            return ReviewAttempt(ReviewAttemptStatus.STORAGE_FAILURE)
        if not review_id:
            return ReviewAttempt(ReviewAttemptStatus.STORAGE_FAILURE)
        return ReviewAttempt(ReviewAttemptStatus.STORED, review, review_id)


@dataclass(frozen=True, slots=True)
class _ReviewContext:
    decision: ClassificationDecision
    evidence_items: tuple[EvidenceItem, ...]
    detail: ExplanationDetail


@dataclass(frozen=True, slots=True)
class _ReviewTriggerRule:
    """Declarative policy entry deliberately separate from criteria evaluation."""

    task_type: ReviewTaskType
    reason: ReviewReason
    requested_output_schema: ReviewOutputSchema
    applies: Callable[[_ReviewContext], bool]
    evidence_ids: Callable[[_ReviewContext], tuple[str, ...]]


class ReviewPacketBuilder:
    """Choose at most one review task and construct its immutable minimum context."""

    def __init__(self, *, token_cap: int = DEFAULT_REVIEW_TOKEN_CAP) -> None:
        if not 1 <= token_cap <= DEFAULT_REVIEW_TOKEN_CAP:
            raise ValueError(
                f"review token cap must be between 1 and {DEFAULT_REVIEW_TOKEN_CAP}"
            )
        self._token_cap = token_cap

    def recommend(
        self,
        *,
        decision: ClassificationDecision,
        evidence_items: Iterable[EvidenceItem],
        detail: ExplanationDetail = ExplanationDetail.STANDARD,
    ) -> ReviewRecommendation | None:
        """Return the one highest-priority applicable recommendation, if any."""
        context = _ReviewContext(
            decision=decision,
            evidence_items=_normalized_evidence_items(evidence_items),
            detail=detail,
        )
        for rule in _REVIEW_TRIGGER_RULES:
            if rule.applies(context):
                return ReviewRecommendation(
                    task_type=rule.task_type,
                    reason=rule.reason,
                    evidence_ids=rule.evidence_ids(context),
                    requested_output_schema=rule.requested_output_schema,
                    token_cap=self._token_cap,
                )
        return None

    def build(
        self,
        *,
        classification_id: str | None,
        decision: ClassificationDecision,
        evidence_items: Iterable[EvidenceItem],
        request: Mapping[str, object],
        snapshot_id: str,
        ruleset_id: str,
        ruleset_version: str,
        detail: ExplanationDetail = ExplanationDetail.STANDARD,
    ) -> ReviewPacket | None:
        """Build a packet only when declarative policy recommends specialist review."""
        normalized_items = _normalized_evidence_items(evidence_items)
        recommendation = self.recommend(
            decision=decision,
            evidence_items=normalized_items,
            detail=detail,
        )
        if recommendation is None:
            return None
        by_id = {item.evidence_id: item for item in normalized_items}
        missing = tuple(
            evidence_id
            for evidence_id in recommendation.evidence_ids
            if evidence_id not in by_id
        )
        if missing:
            raise ValueError(
                "review recommendation references unavailable evidence IDs: "
                + ", ".join(missing)
            )
        observations = tuple(
            _compact_observation(by_id[evidence_id])
            for evidence_id in recommendation.evidence_ids
        )
        frozen_request = _freeze_mapping(request)
        return ReviewPacket(
            task_type=recommendation.task_type,
            reason=recommendation.reason,
            classification_id=classification_id,
            input_hash=review_input_hash(
                request=frozen_request,
                snapshot_id=snapshot_id,
                ruleset_id=ruleset_id,
                ruleset_version=ruleset_version,
            ),
            request=frozen_request,
            snapshot_id=snapshot_id,
            ruleset_id=ruleset_id,
            ruleset_version=ruleset_version,
            selected_evidence_ids=recommendation.evidence_ids,
            observations=observations,
            requested_output_schema=recommendation.requested_output_schema,
            token_cap=recommendation.token_cap,
        )


def review_input_hash(
    *,
    request: Mapping[str, object],
    snapshot_id: str,
    ruleset_id: str,
    ruleset_version: str,
) -> str:
    """Hash the exact request, evidence snapshot, and ruleset identity canonically."""
    return canonical_hash(
        {
            "request": request,
            "snapshot_id": snapshot_id,
            "ruleset": {"id": ruleset_id, "version": ruleset_version},
        }
    )


def _has_conflict(context: _ReviewContext) -> bool:
    return context.decision.conflict is not None


def _conflict_evidence_ids(context: _ReviewContext) -> tuple[str, ...]:
    conflict = context.decision.conflict
    if conflict is None:
        return ()
    criterion_codes = set(conflict.criterion_codes)
    evidence_ids = {
        evidence_id
        for assessment in context.decision.assessments
        if assessment.code in criterion_codes
        for evidence_id in assessment.evidence_ids
    }
    source_ids = set(conflict.source_ids)
    evidence_ids.update(
        _evidence_id(item)
        for item in context.evidence_items
        if isinstance(item.provenance, SourceProvenance)
        and item.provenance.source_id in source_ids
    )
    return tuple(sorted(evidence_ids))


def _has_cited_literature(context: _ReviewContext) -> bool:
    return bool(_literature_evidence_ids(context))


def _literature_evidence_ids(context: _ReviewContext) -> tuple[str, ...]:
    referenced_evidence_ids = set(_decision_evidence_ids(context))
    selected: list[str] = []
    for item in context.evidence_items:
        evidence_id = _evidence_id(item)
        if evidence_id not in referenced_evidence_ids:
            continue
        if isinstance(item.observation, FunctionalObservation):
            citation = item.observation.assay_id
        elif isinstance(item.observation, CaseControlObservation):
            citation = item.observation.study_id
        else:
            continue
        if _is_citation_identifier(citation):
            selected.append(evidence_id)
    return tuple(sorted(selected))


def _full_explanation_requested(context: _ReviewContext) -> bool:
    return context.detail is ExplanationDetail.FULL


def _decision_evidence_ids(context: _ReviewContext) -> tuple[str, ...]:
    return tuple(
        sorted(
            {
                evidence_id
                for assessment in context.decision.assessments
                for evidence_id in assessment.evidence_ids
            }
        )
    )


def _is_citation_identifier(value: str) -> bool:
    return value.startswith(("PMID:", "DOI:", "https://"))


_REVIEW_TRIGGER_RULES: tuple[_ReviewTriggerRule, ...] = (
    _ReviewTriggerRule(
        task_type=ReviewTaskType.EVIDENCE_CONFLICT,
        reason=ReviewReason.UNRESOLVED_CONFLICT,
        requested_output_schema=ReviewOutputSchema.CONFLICT_SUMMARY_V1,
        applies=_has_conflict,
        evidence_ids=_conflict_evidence_ids,
    ),
    _ReviewTriggerRule(
        task_type=ReviewTaskType.LITERATURE_SYNTHESIS,
        reason=ReviewReason.CITED_STUDY_SYNTHESIS,
        requested_output_schema=ReviewOutputSchema.LITERATURE_SYNTHESIS_V1,
        applies=_has_cited_literature,
        evidence_ids=_literature_evidence_ids,
    ),
    _ReviewTriggerRule(
        task_type=ReviewTaskType.EXPLANATION_REVIEW,
        reason=ReviewReason.FULL_EXPLANATION_REQUESTED,
        requested_output_schema=ReviewOutputSchema.EXPLANATION_SUGGESTIONS_V1,
        applies=_full_explanation_requested,
        evidence_ids=_decision_evidence_ids,
    ),
)


def _validate_recommendation_shape(
    *,
    task_type: ReviewTaskType,
    reason: ReviewReason,
    requested_output_schema: ReviewOutputSchema,
    token_cap: int,
) -> None:
    expected = {
        ReviewTaskType.EVIDENCE_CONFLICT: (
            ReviewReason.UNRESOLVED_CONFLICT,
            ReviewOutputSchema.CONFLICT_SUMMARY_V1,
        ),
        ReviewTaskType.LITERATURE_SYNTHESIS: (
            ReviewReason.CITED_STUDY_SYNTHESIS,
            ReviewOutputSchema.LITERATURE_SYNTHESIS_V1,
        ),
        ReviewTaskType.EXPLANATION_REVIEW: (
            ReviewReason.FULL_EXPLANATION_REQUESTED,
            ReviewOutputSchema.EXPLANATION_SUGGESTIONS_V1,
        ),
    }[task_type]
    if (reason, requested_output_schema) != expected:
        raise ValueError("review task, reason, and output schema must agree")
    if not 1 <= token_cap <= DEFAULT_REVIEW_TOKEN_CAP:
        raise ValueError(
            f"review token cap must be between 1 and {DEFAULT_REVIEW_TOKEN_CAP}"
        )


def _evidence_id(item: EvidenceItem) -> str:
    if item.evidence_id is None:
        raise ValueError("review evidence item is missing its derived evidence ID")
    return item.evidence_id


def _normalized_evidence_items(
    evidence_items: Iterable[EvidenceItem],
) -> tuple[EvidenceItem, ...]:
    items = tuple(sorted(evidence_items, key=_evidence_id))
    evidence_ids = tuple(_evidence_id(item) for item in items)
    if len(evidence_ids) != len(set(evidence_ids)):
        raise ValueError("review evidence items must have unique evidence IDs")
    return items


def _normalized_evidence_ids(evidence_ids: Iterable[str]) -> tuple[str, ...]:
    normalized = tuple(sorted(set(evidence_ids)))
    if any(not evidence_id for evidence_id in normalized):
        raise ValueError("review evidence IDs must not be empty")
    return normalized


def _compact_observation(item: EvidenceItem) -> CompactEvidenceObservation:
    return CompactEvidenceObservation(
        evidence_id=_evidence_id(item),
        kind=item.kind,
        observation=item.observation,
        derivation=item.derivation,
        source_id=(
            item.provenance.source_id
            if isinstance(item.provenance, SourceProvenance)
            else None
        ),
        quality_flags=item.quality_flags,
    )


def _freeze_mapping(value: Mapping[str, object]) -> Mapping[str, object]:
    frozen: dict[str, object] = {}
    for key, item in value.items():
        if not isinstance(key, str):
            raise ValueError("review request mapping keys must be strings")
        frozen[key] = _freeze_value(item)
    return MappingProxyType(frozen)


def _freeze_value(value: object) -> object:
    if isinstance(value, Mapping):
        return _freeze_mapping(cast(Mapping[str, object], value))
    if isinstance(value, (list, tuple)):
        return tuple(_freeze_value(item) for item in value)
    return value


def _required_string(payload: Mapping[str, object], field: str) -> str:
    value = payload[field]
    if not isinstance(value, str) or not value:
        raise ValueError(f"review output field {field} must be a non-empty string")
    return value


def _validated_specialist_output(value: object) -> SpecialistReviewOutput:
    if not isinstance(value, dict):
        raise ValueError("review output content must be a JSON object")
    required_fields = {"conclusion", "uncertainty", "questions", "limitations"}
    if set(value) != required_fields:
        raise ValueError("review output content has an invalid field set")
    questions = _validated_text_list(value["questions"], field="questions")
    limitations = _validated_text_list(value["limitations"], field="limitations")
    return SpecialistReviewOutput(
        conclusion=_required_string(value, "conclusion"),
        uncertainty=_required_string(value, "uncertainty"),
        questions=questions,
        limitations=limitations,
    )


def _validated_text_list(value: object, *, field: str) -> tuple[str, ...]:
    if not isinstance(value, list) or not all(
        isinstance(item, str) and item for item in value
    ):
        raise ValueError(f"review output {field} must be a JSON string array")
    return tuple(value)


def _required_packet_classification_id(packet: ReviewPacket) -> str:
    if packet.classification_id is None:
        raise ValueError("review packet has no persisted classification ID")
    return packet.classification_id


def _validated_evidence_ids(value: object, packet: ReviewPacket) -> tuple[str, ...]:
    if not isinstance(value, list) or not all(isinstance(item, str) for item in value):
        raise ValueError("review output evidence IDs must be a JSON string array")
    evidence_ids = _normalized_evidence_ids(cast(list[str], value))
    if evidence_ids != packet.selected_evidence_ids:
        raise ValueError("review output evidence IDs do not exactly match packet")
    return evidence_ids


def _parse_review_timestamp(value: str) -> datetime:
    normalized = value.replace("Z", "+00:00")
    try:
        timestamp = datetime.fromisoformat(normalized)
    except ValueError as error:
        raise ValueError("review output timestamp must be ISO-8601") from error
    if timestamp.tzinfo is None:
        raise ValueError("review output timestamp must include an offset")
    return timestamp


def _reject_json_constant(value: str) -> None:
    raise ValueError(f"review output must not contain JSON constant {value}")


def _mutable_json_value(value: object) -> object:
    if isinstance(value, Mapping):
        return {key: _mutable_json_value(item) for key, item in value.items()}
    if isinstance(value, tuple):
        return [_mutable_json_value(item) for item in value]
    return value
