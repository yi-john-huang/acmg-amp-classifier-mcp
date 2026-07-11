"""Draft, immutable classification, review, feedback, and audit records."""

import secrets
import sqlite3
from collections.abc import Callable
from contextlib import closing
from dataclasses import dataclass
from datetime import UTC, datetime
from pathlib import Path

from acmg_classifier.domain.canonical import canonical_json_bytes
from acmg_classifier.domain.errors import JsonValue
from acmg_classifier.domain.feedback import FeedbackRecord


class RecordStoreError(RuntimeError):
    """Base class for expected record-store failures."""


class RecordNotFoundError(RecordStoreError):
    """A referenced draft or classification does not exist."""


class DraftExpiredError(RecordStoreError):
    """A draft can no longer be resumed because it expired."""


class DraftCompletedError(RecordStoreError):
    """A completed draft cannot be changed."""


class DraftRevisionConflictError(RecordStoreError):
    """A draft update was based on an obsolete revision."""


@dataclass(frozen=True, slots=True)
class StoredDraft:
    """A resumable request and its lifecycle state."""

    draft_id: str
    request_json: bytes
    expires_at: datetime
    completed_classification_id: str | None
    revision: int

@dataclass(frozen=True, slots=True)
class StoredClassification:
    """An immutable classification record."""

    classification_id: str
    canonical_json: bytes
    previous_classification_id: str | None


@dataclass(frozen=True, slots=True)
class StoredFeedback:
    """An immutable feedback artifact and its durable audit timestamp."""

    feedback_id: str
    classification_id: str
    canonical_json: bytes
    created_at: datetime


class SQLiteRecordStore:
    """Persist mutable drafts and append-only scientific records."""

    def __init__(
        self,
        database_path: Path,
        *,
        clock: Callable[[], datetime] | None = None,
    ) -> None:
        self.database_path = database_path
        self.clock = clock or (lambda: datetime.now(UTC))

    def create_draft(self, request: JsonValue, *, expires_at: datetime) -> str:
        """Create a resumable request with an explicit UTC expiry."""
        expiry = _utc_text(expires_at)
        draft_id = _new_id("draft")
        now = _utc_text(self._now())
        with closing(self._connect()) as connection:
            connection.execute(
                """
                INSERT INTO draft_requests
                    (draft_id, request_json, expires_at, created_at, updated_at)
                VALUES (?, ?, ?, ?, ?)
                """,
                (draft_id, canonical_json_bytes(request), expiry, now, now),
            )
            connection.commit()
        return draft_id

    def get_draft(self, draft_id: str) -> StoredDraft:
        """Return an unexpired draft, including completion state."""
        with closing(self._connect()) as connection:
            draft = self._load_draft(connection, draft_id)
        self._ensure_not_expired(draft)
        return draft

    def update_draft(
        self,
        draft_id: str,
        request: JsonValue,
        *,
        expected_revision: int,
    ) -> int:
        """Replace active draft content only at its expected revision."""
        if (
            not isinstance(expected_revision, int)
            or isinstance(expected_revision, bool)
            or expected_revision < 0
        ):
            raise ValueError("expected_revision must be a non-negative integer")
        with closing(self._connect()) as connection:
            connection.execute("BEGIN IMMEDIATE")
            try:
                draft = self._load_draft(connection, draft_id)
                self._ensure_draft_mutable(draft)
                result = connection.execute(
                    """
                    UPDATE draft_requests
                    SET request_json = ?, revision = revision + 1, updated_at = ?
                    WHERE draft_id = ? AND revision = ?
                    """,
                    (
                        canonical_json_bytes(request),
                        _utc_text(self._now()),
                        draft_id,
                        expected_revision,
                    ),
                )
                if result.rowcount != 1:
                    raise DraftRevisionConflictError(
                        f"Draft revision conflict: {draft_id}"
                    )
                connection.commit()
            except Exception:
                connection.rollback()
                raise
        return expected_revision + 1

    def finalize_classification(
        self,
        record: JsonValue,
        *,
        draft_id: str | None = None,
        expected_draft_revision: int | None = None,
        previous_classification_id: str | None = None,
    ) -> str:
        """Atomically create a classification and complete its draft."""
        if draft_id is None:
            if expected_draft_revision is not None:
                raise ValueError(
                    "expected_draft_revision requires a draft_id"
                )
        elif (
            not isinstance(expected_draft_revision, int)
            or isinstance(expected_draft_revision, bool)
            or expected_draft_revision < 0
        ):
            raise ValueError(
                "draft finalization requires a non-negative expected_draft_revision"
            )
        classification_id = _new_id("cls")
        with closing(self._connect()) as connection:
            connection.execute("BEGIN IMMEDIATE")
            try:
                if previous_classification_id is not None:
                    self._require_classification(connection, previous_classification_id)
                connection.execute(
                    """
                    INSERT INTO classification_records
                        (
                            classification_id,
                            canonical_json,
                            previous_classification_id,
                            created_at
                        )
                    VALUES (?, ?, ?, ?)
                    """,
                    (
                        classification_id,
                        canonical_json_bytes(record),
                        previous_classification_id,
                        _utc_text(self._now()),
                    ),
                )
                if draft_id is not None:
                    assert expected_draft_revision is not None
                    draft = self._load_draft(connection, draft_id)
                    self._ensure_draft_mutable(draft)
                    result = connection.execute(
                        """
                        UPDATE draft_requests
                        SET completed_classification_id = ?, updated_at = ?
                        WHERE draft_id = ?
                          AND revision = ?
                          AND completed_classification_id IS NULL
                        """,
                        (
                            classification_id,
                            _utc_text(self._now()),
                            draft_id,
                            expected_draft_revision,
                        ),
                    )
                    if result.rowcount != 1:
                        raise DraftRevisionConflictError(
                            f"Draft revision conflict: {draft_id}"
                        )
                connection.commit()
            except Exception:
                connection.rollback()
                raise
        return classification_id

    def get_classification(self, classification_id: str) -> StoredClassification:
        """Return an immutable classification by ID."""
        with closing(self._connect()) as connection:
            row = connection.execute(
                """
                SELECT canonical_json, previous_classification_id
                FROM classification_records
                WHERE classification_id = ?
                """,
                (classification_id,),
            ).fetchone()
        if row is None:
            raise RecordNotFoundError(f"Classification not found: {classification_id}")
        return StoredClassification(
            classification_id=classification_id,
            canonical_json=bytes(row[0]),
            previous_classification_id=str(row[1]) if row[1] is not None else None,
        )

    def append_feedback(
        self,
        classification_id: str,
        feedback: JsonValue,
        *,
        feedback_id: str | None = None,
        created_at: datetime | None = None,
    ) -> str:
        """Append user feedback without changing its classification."""
        return self._append_classification_artifact(
            table="feedback",
            id_column="feedback_id",
            prefix="fb",
            classification_id=classification_id,
            content=feedback,
            artifact_id=feedback_id,
            created_at=created_at,
        )

    def append_review(self, classification_id: str, review: JsonValue) -> str:
        """Append a review artifact without changing its classification."""
        return self._append_classification_artifact(
            table="review_artifacts",
            id_column="review_id",
            prefix="review",
            classification_id=classification_id,
            content=review,
        )

    def list_feedback(
        self,
        classification_id: str | None = None,
    ) -> tuple[StoredFeedback, ...]:
        """Return immutable feedback in a deterministic audit order."""
        statement = """
            SELECT feedback_id, classification_id, canonical_json, created_at
            FROM feedback
        """
        parameters: tuple[str, ...] = ()
        if classification_id is not None:
            statement += " WHERE classification_id = ?"
            parameters = (classification_id,)
        statement += " ORDER BY created_at, feedback_id"
        with closing(self._connect()) as connection:
            rows = tuple(connection.execute(statement, parameters))
        return tuple(
            StoredFeedback(
                feedback_id=str(row[0]),
                classification_id=str(row[1]),
                canonical_json=bytes(row[2]),
                created_at=datetime.fromisoformat(str(row[3]).replace("Z", "+00:00")),
            )
            for row in rows
        )

    def append_feedback_records(
        self,
        records: tuple[FeedbackRecord, ...],
    ) -> tuple[str, ...]:
        """Append an import batch atomically while preserving all audit fields."""
        with closing(self._connect()) as connection:
            connection.execute("BEGIN IMMEDIATE")
            try:
                for record in records:
                    self._require_classification(connection, record.classification_id)
                    connection.execute(
                        """
                        INSERT INTO feedback
                            (feedback_id, classification_id, canonical_json, created_at)
                        VALUES (?, ?, ?, ?)
                        """,
                        (
                            record.feedback_id,
                            record.classification_id,
                            canonical_json_bytes(record.to_canonical_content()),
                            _utc_text(record.submitted_at),
                        ),
                    )
                connection.commit()
            except Exception:
                connection.rollback()
                raise
        return tuple(record.feedback_id for record in records)

    def append_audit(self, event: JsonValue) -> str:
        """Append an independent audit event."""
        audit_id = _new_id("audit")
        with closing(self._connect()) as connection:
            connection.execute(
                """
                INSERT INTO audit_events (audit_id, canonical_json, created_at)
                VALUES (?, ?, ?)
                """,
                (audit_id, canonical_json_bytes(event), _utc_text(self._now())),
            )
            connection.commit()
        return audit_id

    def _append_classification_artifact(
        self,
        *,
        table: str,
        id_column: str,
        prefix: str,
        classification_id: str,
        content: JsonValue,
        artifact_id: str | None = None,
        created_at: datetime | None = None,
    ) -> str:
        artifact_id = artifact_id or _new_id(prefix)
        if not artifact_id.startswith(f"{prefix}_"):
            raise ValueError(f"artifact ID must use {prefix}_ prefix")
        created_at_text = _utc_text(created_at or self._now())
        if (table, id_column) not in {
            ("feedback", "feedback_id"),
            ("review_artifacts", "review_id"),
        }:
            raise ValueError("Unsupported artifact table")
        with closing(self._connect()) as connection:
            self._require_classification(connection, classification_id)
            connection.execute(
                f"""
                INSERT INTO {table}
                    ({id_column}, classification_id, canonical_json, created_at)
                VALUES (?, ?, ?, ?)
                """,
                (
                    artifact_id,
                    classification_id,
                    canonical_json_bytes(content),
                    created_at_text,
                ),
            )
            connection.commit()
        return artifact_id

    def _load_draft(
        self,
        connection: sqlite3.Connection,
        draft_id: str,
    ) -> StoredDraft:
        row = connection.execute(
            """
            SELECT request_json, expires_at, completed_classification_id, revision
            FROM draft_requests
            WHERE draft_id = ?
            """,
            (draft_id,),
        ).fetchone()
        if row is None:
            raise RecordNotFoundError(f"Draft not found: {draft_id}")
        return StoredDraft(
            draft_id=draft_id,
            request_json=bytes(row[0]),
            expires_at=datetime.fromisoformat(str(row[1]).replace("Z", "+00:00")),
            completed_classification_id=str(row[2]) if row[2] is not None else None,
            revision=int(row[3]),
        )

    def _ensure_draft_mutable(self, draft: StoredDraft) -> None:
        if draft.completed_classification_id is not None:
            raise DraftCompletedError(f"Draft already completed: {draft.draft_id}")
        self._ensure_not_expired(draft)

    def _ensure_not_expired(self, draft: StoredDraft) -> None:
        if draft.expires_at <= self._now():
            raise DraftExpiredError(f"Draft expired: {draft.draft_id}")

    def _require_classification(
        self,
        connection: sqlite3.Connection,
        classification_id: str,
    ) -> None:
        exists = connection.execute(
            "SELECT 1 FROM classification_records WHERE classification_id = ?",
            (classification_id,),
        ).fetchone()
        if exists is None:
            raise RecordNotFoundError(f"Classification not found: {classification_id}")

    def _connect(self) -> sqlite3.Connection:
        connection = sqlite3.connect(self.database_path, isolation_level=None)
        connection.execute("PRAGMA foreign_keys=ON")
        return connection

    def _now(self) -> datetime:
        now = self.clock()
        _utc_text(now)
        return now.astimezone(UTC)


def _new_id(prefix: str) -> str:
    return f"{prefix}_{secrets.token_hex(16)}"


def _utc_text(value: datetime) -> str:
    if value.tzinfo is None or value.utcoffset() is None:
        raise ValueError("Record timestamps must be timezone-aware")
    return (
        value.astimezone(UTC).isoformat(timespec="microseconds").replace("+00:00", "Z")
    )
