"""Signed opaque draft continuation and context-answer merge workflow."""

from __future__ import annotations

import hashlib
import hmac
import json
import re
from collections.abc import Callable, Mapping
from dataclasses import dataclass
from datetime import UTC, datetime, timedelta
from enum import StrEnum
from types import MappingProxyType
from typing import Protocol, cast

from acmg_classifier.domain.context import ContextField, ContextQuestion
from acmg_classifier.domain.errors import JsonValue
from acmg_classifier.domain.normalization import NormalizedVariant

_TOKEN_PATTERN = re.compile(
    r"^resume_(?P<draft>[0-9a-f]{32})(?P<signature>[0-9a-f]{64})$"
)


class DraftAnswerState(StrEnum):
    """Scientist response states that never invent unavailable context."""

    PROVIDED = "provided"
    UNKNOWN = "unknown"
    UNAVAILABLE = "unavailable"
    NOT_APPLICABLE = "not_applicable"


class DraftResumeError(ValueError):
    """Stable non-secret failure for a continuation request."""

    def __init__(self, code: str) -> None:
        self.code = code
        super().__init__(code)


class DraftStoreRecord(Protocol):
    """The immutable read view required from a draft persistence adapter."""

    @property
    def draft_id(self) -> str: ...

    @property
    def request_json(self) -> bytes: ...

    @property
    def expires_at(self) -> datetime: ...

    @property
    def completed_classification_id(self) -> str | None: ...

    @property
    def revision(self) -> int: ...


class DraftStore(Protocol):
    """The minimal mutable-draft storage boundary used by this service."""

    def create_draft(self, request: JsonValue, *, expires_at: datetime) -> str: ...

    def get_draft(self, draft_id: str) -> DraftStoreRecord: ...

    def update_draft(
        self,
        draft_id: str,
        request: JsonValue,
        *,
        expected_revision: int,
    ) -> int: ...


@dataclass(frozen=True, slots=True)
class DraftAnswer:
    """One explicit answer to a previously issued context question."""

    field: ContextField
    state: DraftAnswerState
    value: JsonValue | None = None

    def __post_init__(self) -> None:
        if not isinstance(self.field, ContextField):
            raise ValueError("draft answer field must be a ContextField")
        if not isinstance(self.state, DraftAnswerState):
            raise ValueError("draft answer state must be a DraftAnswerState")
        if self.state is DraftAnswerState.PROVIDED and self.value is None:
            raise ValueError("provided draft answer requires a value")
        if self.state is not DraftAnswerState.PROVIDED and self.value is not None:
            raise ValueError("non-provided draft answer must not include a value")


@dataclass(frozen=True, slots=True)
class DraftContinuation:
    """Opaque continuation credentials for one persisted incomplete workflow."""

    draft_id: str
    resume_token: str


@dataclass(frozen=True, slots=True)
class ResumedWorkflowDraft:
    """Validated stored work and merged request ready for application resumption."""

    draft_id: str
    resume_token: str
    request: Mapping[str, JsonValue]
    normalized_variant: NormalizedVariant
    answer_states: Mapping[str, str]
    revision: int

    def __post_init__(self) -> None:
        object.__setattr__(self, "request", MappingProxyType(dict(self.request)))
        object.__setattr__(
            self,
            "answer_states",
            MappingProxyType(dict(sorted(self.answer_states.items()))),
        )


class WorkflowDraftService:
    """Persist and securely resume context-limited classification work."""

    def __init__(
        self,
        *,
        store: DraftStore,
        signing_key: bytes,
        clock: Callable[[], datetime],
        lifetime: timedelta,
    ) -> None:
        if len(signing_key) < 32:
            raise ValueError("draft signing_key must contain at least 32 bytes")
        if lifetime <= timedelta(0):
            raise ValueError("draft lifetime must be positive")
        self._store = store
        self._signing_key = signing_key
        self._clock = clock
        self._lifetime = lifetime

    def create(
        self,
        *,
        request: Mapping[str, JsonValue],
        normalized_variant: NormalizedVariant,
        questions: tuple[ContextQuestion, ...],
    ) -> DraftContinuation:
        """Persist normalized work before asking the first context question."""
        if not questions:
            raise ValueError("draft requires at least one context question")
        expires_at = self._expires_at()
        payload = _draft_payload(
            request,
            normalized_variant,
            _question_payloads(questions),
            {},
        )
        try:
            draft_id = self._store.create_draft(payload, expires_at=expires_at)
        except Exception as error:
            raise DraftResumeError("DRAFT_PERSISTENCE_FAILED") from error
        return DraftContinuation(
            draft_id=draft_id,
            resume_token=self._sign(draft_id, expires_at),
        )

    def update(
        self,
        *,
        draft_id: str,
        request: Mapping[str, JsonValue],
        normalized_variant: NormalizedVariant,
        questions: tuple[ContextQuestion, ...],
        answer_states: Mapping[str, str],
    ) -> DraftContinuation:
        """Replace an active draft only with a canonical next question state."""
        if not questions:
            raise ValueError("draft update requires at least one context question")
        try:
            stored = self._store.get_draft(draft_id)
        except Exception as error:
            raise _store_error(error) from error
        if stored.completed_classification_id is not None:
            raise DraftResumeError("DRAFT_ALREADY_COMPLETED")
        payload = _draft_payload(
            request,
            normalized_variant,
            _question_payloads(questions),
            answer_states,
        )
        try:
            self._store.update_draft(
                draft_id,
                payload,
                expected_revision=stored.revision,
            )
        except Exception as error:
            raise _store_error(error) from error
        return DraftContinuation(
            draft_id=draft_id,
            resume_token=self._sign(draft_id, stored.expires_at),
        )

    def resume(
        self,
        resume_token: str,
        *,
        answers: tuple[DraftAnswer, ...],
    ) -> ResumedWorkflowDraft:
        """Verify a token, merge only asked fields, and return saved normalization."""
        draft_id = self._draft_id_from_token(resume_token)
        try:
            stored = self._store.get_draft(draft_id)
        except Exception as error:
            raise _store_error(error) from error
        if stored.completed_classification_id is not None:
            raise DraftResumeError("DRAFT_ALREADY_COMPLETED")
        if not hmac.compare_digest(
            resume_token, self._sign(draft_id, stored.expires_at)
        ):
            raise DraftResumeError("DRAFT_TOKEN_INVALID")
        payload = _parse_payload(stored.request_json)
        request = _payload_mapping(payload, "request")
        try:
            normalized = NormalizedVariant.from_canonical_content(
                _payload_mapping(payload, "normalized_variant")
            )
        except ValueError as error:
            raise DraftResumeError("DRAFT_CONTENT_INVALID") from error
        questions = _payload_questions(payload)
        answer_states = _payload_answer_states(payload)
        revision = stored.revision
        merged_request = _merge_answers(request, questions, answer_states, answers)
        if answers:
            payload = _draft_payload(
                merged_request,
                normalized,
                questions,
                answer_states,
            )
            try:
                revision = self._store.update_draft(
                    draft_id,
                    payload,
                    expected_revision=stored.revision,
                )
            except Exception as error:
                raise _store_error(error) from error
        return ResumedWorkflowDraft(
            draft_id=draft_id,
            resume_token=self._sign(draft_id, stored.expires_at),
            request=merged_request,
            normalized_variant=normalized,
            answer_states=answer_states,
            revision=revision,
        )

    def _expires_at(self) -> datetime:
        now = self._clock()
        if now.tzinfo is None or now.utcoffset() is None:
            raise ValueError("draft clock must return a timezone-aware datetime")
        return now.astimezone(UTC) + self._lifetime

    def _sign(self, draft_id: str, expires_at: datetime) -> str:
        if not draft_id.startswith("draft_"):
            raise DraftResumeError("DRAFT_TOKEN_INVALID")
        suffix = draft_id.removeprefix("draft_")
        if not re.fullmatch(r"[0-9a-f]{32}", suffix):
            raise DraftResumeError("DRAFT_TOKEN_INVALID")
        if expires_at.tzinfo is None or expires_at.utcoffset() is None:
            raise DraftResumeError("DRAFT_TOKEN_INVALID")
        message = f"{draft_id}\x00{expires_at.astimezone(UTC).isoformat()}".encode()
        signature = hmac.new(self._signing_key, message, hashlib.sha256).hexdigest()
        return f"resume_{suffix}{signature}"

    @staticmethod
    def _draft_id_from_token(resume_token: str) -> str:
        match = _TOKEN_PATTERN.fullmatch(resume_token)
        if match is None:
            raise DraftResumeError("DRAFT_TOKEN_INVALID")
        return f"draft_{match.group('draft')}"


def _draft_payload(
    request: Mapping[str, JsonValue],
    normalized_variant: NormalizedVariant,
    questions: tuple[Mapping[str, JsonValue], ...],
    answer_states: Mapping[str, str],
) -> dict[str, JsonValue]:
    return {
        "schema_version": "1.0",
        "request": dict(request),
        "normalized_variant": normalized_variant.to_canonical_content(),
        "questions": [dict(question) for question in questions],
        "answer_states": dict(sorted(answer_states.items())),
    }


def _question_payloads(
    questions: tuple[ContextQuestion, ...],
) -> tuple[Mapping[str, JsonValue], ...]:
    payloads = tuple(question.to_canonical_content() for question in questions)
    fields = [payload["field"] for payload in payloads]
    if len(fields) != len(set(fields)):
        raise ValueError("draft questions must not repeat a context field")
    return payloads


def _parse_payload(raw: bytes) -> dict[str, JsonValue]:
    try:
        parsed = json.loads(raw)
    except (UnicodeDecodeError, json.JSONDecodeError) as error:
        raise DraftResumeError("DRAFT_CONTENT_INVALID") from error
    if not isinstance(parsed, dict) or parsed.get("schema_version") != "1.0":
        raise DraftResumeError("DRAFT_CONTENT_INVALID")
    return cast(dict[str, JsonValue], parsed)


def _payload_mapping(
    payload: Mapping[str, JsonValue], key: str
) -> dict[str, JsonValue]:
    value = payload.get(key)
    if not isinstance(value, dict):
        raise DraftResumeError("DRAFT_CONTENT_INVALID")
    return dict(value)


def _payload_questions(
    payload: Mapping[str, JsonValue],
) -> tuple[dict[str, JsonValue], ...]:
    value = payload.get("questions")
    if not isinstance(value, list):
        raise DraftResumeError("DRAFT_CONTENT_INVALID")
    questions: list[dict[str, JsonValue]] = []
    for item in value:
        if not isinstance(item, dict):
            raise DraftResumeError("DRAFT_CONTENT_INVALID")
        field = item.get("field")
        if not isinstance(field, str):
            raise DraftResumeError("DRAFT_CONTENT_INVALID")
        try:
            ContextField(field)
        except ValueError as error:
            raise DraftResumeError("DRAFT_CONTENT_INVALID") from error
        questions.append(dict(item))
    return tuple(questions)


def _payload_answer_states(payload: Mapping[str, JsonValue]) -> dict[str, str]:
    value = payload.get("answer_states", {})
    if not isinstance(value, dict):
        raise DraftResumeError("DRAFT_CONTENT_INVALID")
    states: dict[str, str] = {}
    for field, state in value.items():
        if not isinstance(field, str) or not isinstance(state, str):
            raise DraftResumeError("DRAFT_CONTENT_INVALID")
        try:
            ContextField(field)
            DraftAnswerState(state)
        except ValueError as error:
            raise DraftResumeError("DRAFT_CONTENT_INVALID") from error
        states[field] = state
    return states


def _merge_answers(
    request: dict[str, JsonValue],
    questions: tuple[dict[str, JsonValue], ...],
    answer_states: dict[str, str],
    answers: tuple[DraftAnswer, ...],
) -> dict[str, JsonValue]:
    questions_by_field = {
        field: question
        for question in questions
        if isinstance(field := question.get("field"), str)
    }
    if len({answer.field for answer in answers}) != len(answers):
        raise DraftResumeError("DRAFT_ANSWER_INVALID")
    context = request.get("context")
    if not isinstance(context, dict):
        raise DraftResumeError("DRAFT_CONTENT_INVALID")
    merged_context = dict(context)
    for answer in answers:
        field = answer.field.value
        question = questions_by_field.get(field)
        if question is None:
            raise DraftResumeError("DRAFT_ANSWER_INVALID")
        answer_states[field] = answer.state.value
        if answer.state is DraftAnswerState.PROVIDED:
            value = answer.value
            if value is None:
                raise DraftResumeError("DRAFT_ANSWER_INVALID")
            _validate_answer_schema(value, question)
            _apply_answer(merged_context, answer, question)
        else:
            _clear_answer(merged_context, answer.field)
    merged = dict(request)
    merged["context"] = merged_context
    return merged


def _validate_answer_schema(
    value: JsonValue,
    question: Mapping[str, JsonValue],
) -> None:
    schema = question.get("answer_schema")
    if not isinstance(schema, dict):
        raise DraftResumeError("DRAFT_CONTENT_INVALID")
    expected_type = schema.get("type")
    if expected_type == "string":
        if not isinstance(value, str):
            raise DraftResumeError("DRAFT_ANSWER_INVALID")
        minimum = schema.get("minLength")
        if not isinstance(minimum, int) or isinstance(minimum, bool) or minimum < 0:
            if minimum is not None:
                raise DraftResumeError("DRAFT_CONTENT_INVALID")
        elif len(value) < minimum:
            raise DraftResumeError("DRAFT_ANSWER_INVALID")
    elif expected_type == "object":
        if not isinstance(value, dict):
            raise DraftResumeError("DRAFT_ANSWER_INVALID")
        required = schema.get("required")
        if required is not None:
            if not isinstance(required, list) or not all(
                isinstance(key, str) for key in required
            ):
                raise DraftResumeError("DRAFT_CONTENT_INVALID")
            if any(key not in value for key in required):
                raise DraftResumeError("DRAFT_ANSWER_INVALID")
    else:
        raise DraftResumeError("DRAFT_CONTENT_INVALID")
    allowed = schema.get("enum")
    if allowed is not None:
        if not isinstance(allowed, list):
            raise DraftResumeError("DRAFT_CONTENT_INVALID")
        if value not in allowed:
            raise DraftResumeError("DRAFT_ANSWER_INVALID")


def _apply_answer(
    context: dict[str, JsonValue],
    answer: DraftAnswer,
    question: Mapping[str, JsonValue],
) -> None:
    value = answer.value
    if answer.field is ContextField.DISEASE:
        if isinstance(value, str) and value:
            context["disease_id"] = value
            context["disease_label"] = _choice_label(question, value)
            return
        if not isinstance(value, dict):
            raise DraftResumeError("DRAFT_ANSWER_INVALID")
        identifier = value.get("identifier")
        label = value.get("label")
        if not isinstance(identifier, str) or not isinstance(label, str):
            raise DraftResumeError("DRAFT_ANSWER_INVALID")
        context["disease_id"] = identifier
        context["disease_label"] = label
        return
    if not isinstance(value, str) or not value:
        raise DraftResumeError("DRAFT_ANSWER_INVALID")
    if answer.field is ContextField.GENOME_BUILD:
        context["genome_build"] = value
    elif answer.field is ContextField.TRANSCRIPT:
        context["transcript"] = value
    elif answer.field is ContextField.INHERITANCE:
        context["inheritance"] = value
    else:
        raise DraftResumeError("DRAFT_ANSWER_INVALID")


def _choice_label(
    question: Mapping[str, JsonValue],
    selected_value: str,
) -> str | None:
    choices = question.get("choices")
    if not isinstance(choices, list):
        raise DraftResumeError("DRAFT_CONTENT_INVALID")
    for choice in choices:
        if not isinstance(choice, dict):
            raise DraftResumeError("DRAFT_CONTENT_INVALID")
        if choice.get("value") == selected_value:
            label = choice.get("label")
            if label is not None and not isinstance(label, str):
                raise DraftResumeError("DRAFT_CONTENT_INVALID")
            return label
    return None


def _clear_answer(context: dict[str, JsonValue], field: ContextField) -> None:
    if field is ContextField.DISEASE:
        context["disease_id"] = None
        context["disease_label"] = None
    elif field is ContextField.GENOME_BUILD:
        context["genome_build"] = None
    elif field is ContextField.TRANSCRIPT:
        context["transcript"] = None
    elif field is ContextField.INHERITANCE:
        context["inheritance"] = None


def _store_error(error: Exception) -> DraftResumeError:
    name = type(error).__name__
    if name == "DraftExpiredError":
        return DraftResumeError("DRAFT_EXPIRED")
    if name == "DraftCompletedError":
        return DraftResumeError("DRAFT_ALREADY_COMPLETED")
    if name == "DraftRevisionConflictError":
        return DraftResumeError("DRAFT_REVISION_CONFLICT")
    if name == "RecordNotFoundError":
        return DraftResumeError("DRAFT_TOKEN_INVALID")
    return DraftResumeError("DRAFT_PERSISTENCE_FAILED")
