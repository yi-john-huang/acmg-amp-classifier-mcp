"""Append-only feedback workflow isolated from evidence and classification logic."""

from __future__ import annotations

import json
import secrets
from collections.abc import Callable, Mapping
from dataclasses import dataclass
from datetime import UTC, datetime
from typing import Protocol

from acmg_classifier.domain.errors import JsonValue
from acmg_classifier.domain.feedback import (
    FeedbackContext,
    feedback_context_from_classification,
)
from acmg_classifier.domain.feedback import (
    FeedbackRecord as FeedbackRecord,
)
from acmg_classifier.domain.feedback import (
    FeedbackSubmission as FeedbackSubmission,
)


class FeedbackError(ValueError):
    """Stable feedback workflow failure that never exposes storage internals."""


class FeedbackImportError(FeedbackError):
    """An imported append-only record cannot be preserved safely."""


class StoredClassificationRecord(Protocol):
    """The narrow immutable classification view required by feedback."""

    @property
    def classification_id(self) -> str: ...

    @property
    def canonical_json(self) -> bytes: ...


class StoredFeedbackRecord(Protocol):
    """The narrow immutable feedback view used for query and export."""

    @property
    def feedback_id(self) -> str: ...

    @property
    def classification_id(self) -> str: ...

    @property
    def canonical_json(self) -> bytes: ...


class FeedbackStore(Protocol):
    """Persistence boundary that cannot mutate classifications or feedback."""

    def get_classification(
        self, classification_id: str
    ) -> StoredClassificationRecord: ...

    def append_feedback(
        self,
        classification_id: str,
        feedback: JsonValue,
        *,
        feedback_id: str,
        created_at: datetime,
    ) -> str: ...

    def list_feedback(
        self,
        classification_id: str | None = None,
    ) -> tuple[StoredFeedbackRecord, ...]: ...

    def append_feedback_records(
        self,
        records: tuple[FeedbackRecord, ...],
    ) -> tuple[str, ...]: ...


@dataclass(frozen=True, slots=True)
class FeedbackService:
    """Persist feedback as a distinct immutable artifact, never as evidence."""

    store: FeedbackStore
    clock: Callable[[], datetime] = lambda: datetime.now(UTC)

    def submit(self, submission: FeedbackSubmission) -> FeedbackRecord:
        """Append one validated agreement or correction to an existing record."""
        content = _classification_content(
            self.store.get_classification(submission.classification_id)
        )
        variant_key, context = feedback_context_from_classification(content)
        record = FeedbackRecord(
            feedback_id=_new_feedback_id(),
            submitted_at=_utc_now(self.clock),
            variant_key=variant_key,
            context=context,
            **submission.model_dump(),
        )
        stored_id = self.store.append_feedback(
            record.classification_id,
            record.to_canonical_content(),
            feedback_id=record.feedback_id,
            created_at=record.submitted_at,
        )
        if stored_id != record.feedback_id:
            raise FeedbackError("feedback store returned a mismatched identifier")
        return record

    def query(
        self,
        *,
        variant_key: str,
        context: FeedbackContext,
    ) -> tuple[FeedbackRecord, ...]:
        """Return feedback matching exactly one canonical allele and context."""
        if not variant_key:
            raise FeedbackError("variant key must not be empty")
        records = (_feedback_record(stored) for stored in self.store.list_feedback())
        return tuple(
            record
            for record in records
            if record.variant_key == variant_key and record.context == context
        )

    def export(
        self,
        classification_id: str | None = None,
    ) -> tuple[FeedbackRecord, ...]:
        """Return portable immutable artifacts with their IDs and audit fields."""
        records = tuple(
            _feedback_record(stored)
            for stored in self.store.list_feedback(classification_id)
        )
        return tuple(
            sorted(
                records,
                key=lambda record: (record.submitted_at, record.feedback_id),
            )
        )

    def import_records(self, records: tuple[FeedbackRecord, ...]) -> tuple[str, ...]:
        """Append one export atomically, preserving each ID and audit timestamp."""
        try:
            imported = self.store.append_feedback_records(records)
        except Exception as error:
            identifiers = ",".join(record.feedback_id for record in records)
            raise FeedbackImportError(
                f"feedback import failed for {identifiers or 'empty export'}"
            ) from error
        expected = tuple(record.feedback_id for record in records)
        if imported != expected:
            raise FeedbackImportError("feedback import changed record identifiers")
        return imported


def _classification_content(stored: StoredClassificationRecord) -> Mapping[str, object]:
    try:
        content = json.loads(stored.canonical_json)
    except (TypeError, UnicodeDecodeError, json.JSONDecodeError) as error:
        raise FeedbackError("classification record content is invalid") from error
    if not isinstance(content, Mapping):
        raise FeedbackError("classification record content is invalid")
    return content


def _feedback_record(stored: StoredFeedbackRecord) -> FeedbackRecord:
    try:
        content = json.loads(stored.canonical_json)
        if not isinstance(content, Mapping):
            raise TypeError("feedback content is not an object")
        record = FeedbackRecord.model_validate(content)
    except (TypeError, UnicodeDecodeError, json.JSONDecodeError, ValueError) as error:
        raise FeedbackError("stored feedback content is invalid") from error
    if (
        record.feedback_id != stored.feedback_id
        or record.classification_id != stored.classification_id
    ):
        raise FeedbackError("stored feedback identifier does not match its content")
    return record


def _new_feedback_id() -> str:
    return f"fb_{secrets.token_hex(16)}"


def _utc_now(clock: Callable[[], datetime]) -> datetime:
    value = clock()
    if value.tzinfo is None or value.utcoffset() is None:
        raise FeedbackError("feedback clock must return a timezone-aware datetime")
    return value.astimezone(UTC)
