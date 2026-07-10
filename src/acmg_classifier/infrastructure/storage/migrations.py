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
)
