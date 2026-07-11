"""Explicit SQLite migration definitions."""

from dataclasses import dataclass


@dataclass(frozen=True, slots=True)
class Migration:
    """One atomic, forward-only SQLite schema migration."""

    version: int
    name: str
    statements: tuple[str, ...]

    def __post_init__(self) -> None:
        if self.version < 1:
            raise ValueError("Migration versions must be positive")
        if not self.name:
            raise ValueError("Migration names must not be empty")
        if not self.statements:
            raise ValueError("Migrations must contain at least one statement")


DEFAULT_MIGRATIONS = (
    Migration(
        version=1,
        name="initialize_runtime_metadata",
        statements=(
            """
            CREATE TABLE runtime_metadata (
                key TEXT PRIMARY KEY,
                value TEXT NOT NULL
            )
            """,
        ),
    ),
    Migration(
        version=2,
        name="add_immutable_evidence_storage",
        statements=(
            """
            CREATE TABLE evidence_items (
                evidence_id TEXT PRIMARY KEY,
                canonical_json BLOB NOT NULL,
                created_at TEXT NOT NULL
            )
            """,
            """
            CREATE TRIGGER evidence_items_reject_update
            BEFORE UPDATE ON evidence_items
            BEGIN
                SELECT RAISE(ABORT, 'evidence_items are immutable');
            END
            """,
            """
            CREATE TRIGGER evidence_items_reject_delete
            BEFORE DELETE ON evidence_items
            BEGIN
                SELECT RAISE(ABORT, 'evidence_items are immutable');
            END
            """,
            """
            CREATE TABLE raw_snapshots (
                snapshot_hash TEXT PRIMARY KEY,
                media_type TEXT NOT NULL,
                byte_size INTEGER NOT NULL CHECK (byte_size >= 0),
                relative_path TEXT NOT NULL,
                created_at TEXT NOT NULL
            )
            """,
            """
            CREATE TABLE evidence_snapshots (
                snapshot_id TEXT PRIMARY KEY,
                canonical_json BLOB NOT NULL,
                source_status_json BLOB NOT NULL,
                created_at TEXT NOT NULL
            )
            """,
            """
            CREATE TRIGGER evidence_snapshots_reject_update
            BEFORE UPDATE ON evidence_snapshots
            BEGIN
                SELECT RAISE(ABORT, 'evidence_snapshots are immutable');
            END
            """,
            """
            CREATE TRIGGER evidence_snapshots_reject_delete
            BEFORE DELETE ON evidence_snapshots
            BEGIN
                SELECT RAISE(ABORT, 'evidence_snapshots are immutable');
            END
            """,
            """
            CREATE TABLE evidence_snapshot_items (
                snapshot_id TEXT NOT NULL REFERENCES evidence_snapshots(snapshot_id),
                evidence_id TEXT NOT NULL REFERENCES evidence_items(evidence_id),
                PRIMARY KEY (snapshot_id, evidence_id)
            )
            """,
        ),
    ),
    Migration(
        version=3,
        name="add_classification_and_audit_records",
        statements=(
            """
            CREATE TABLE classification_records (
                classification_id TEXT PRIMARY KEY,
                canonical_json BLOB NOT NULL,
                previous_classification_id TEXT
                    REFERENCES classification_records(classification_id),
                created_at TEXT NOT NULL
            )
            """,
            """
            CREATE TRIGGER classification_records_reject_update
            BEFORE UPDATE ON classification_records
            BEGIN
                SELECT RAISE(ABORT, 'classification_records are immutable');
            END
            """,
            """
            CREATE TRIGGER classification_records_reject_delete
            BEFORE DELETE ON classification_records
            BEGIN
                SELECT RAISE(ABORT, 'classification_records are immutable');
            END
            """,
            """
            CREATE TABLE draft_requests (
                draft_id TEXT PRIMARY KEY,
                request_json BLOB NOT NULL,
                expires_at TEXT NOT NULL,
                completed_classification_id TEXT
                    REFERENCES classification_records(classification_id),
                created_at TEXT NOT NULL,
                updated_at TEXT NOT NULL
            )
            """,
            """
            CREATE TABLE review_artifacts (
                review_id TEXT PRIMARY KEY,
                classification_id TEXT NOT NULL
                    REFERENCES classification_records(classification_id),
                canonical_json BLOB NOT NULL,
                created_at TEXT NOT NULL
            )
            """,
            """
            CREATE TABLE feedback (
                feedback_id TEXT PRIMARY KEY,
                classification_id TEXT NOT NULL
                    REFERENCES classification_records(classification_id),
                canonical_json BLOB NOT NULL,
                created_at TEXT NOT NULL
            )
            """,
            """
            CREATE TABLE audit_events (
                audit_id TEXT PRIMARY KEY,
                canonical_json BLOB NOT NULL,
                created_at TEXT NOT NULL
            )
            """,
            """
            CREATE TRIGGER review_artifacts_reject_update
            BEFORE UPDATE ON review_artifacts
            BEGIN
                SELECT RAISE(ABORT, 'review_artifacts are immutable');
            END
            """,
            """
            CREATE TRIGGER feedback_reject_update
            BEFORE UPDATE ON feedback
            BEGIN
                SELECT RAISE(ABORT, 'feedback is immutable');
            END
            """,
            """
            CREATE TRIGGER audit_events_reject_update
            BEFORE UPDATE ON audit_events
            BEGIN
                SELECT RAISE(ABORT, 'audit_events are immutable');
            END
            """,
        ),
    ),
    Migration(
        version=4,
        name="add_replaceable_source_cache_index",
        statements=(
            """
            CREATE TABLE source_cache (
                cache_key TEXT PRIMARY KEY,
                source_id TEXT NOT NULL,
                normalized_query_key TEXT NOT NULL,
                request_fingerprint TEXT NOT NULL,
                source_version TEXT,
                adapter_version TEXT NOT NULL,
                status TEXT NOT NULL
                    CHECK (status IN ('success', 'no_record', 'failure')),
                retrieved_at TEXT NOT NULL,
                expires_at TEXT,
                evidence_ids_json BLOB NOT NULL,
                raw_snapshot_ref TEXT,
                response_hash TEXT,
                media_type TEXT,
                byte_size INTEGER NOT NULL CHECK (byte_size >= 0),
                last_error_code TEXT,
                updated_at TEXT NOT NULL
            )
            """,
            """
            CREATE INDEX source_cache_lookup
            ON source_cache (source_id, normalized_query_key, source_version)
            """,
        ),
    ),
    Migration(
        version=5,
        name="add_draft_revision",
        statements=(
            """
            ALTER TABLE draft_requests
            ADD COLUMN revision INTEGER NOT NULL DEFAULT 0 CHECK (revision >= 0)
            """,
        ),
    ),
)
