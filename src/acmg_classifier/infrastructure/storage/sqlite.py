"""Zero-service SQLite state initialization and migration."""

import sqlite3
from contextlib import closing
from dataclasses import dataclass
from pathlib import Path

from acmg_classifier.infrastructure.storage.migrations import (
    DEFAULT_MIGRATIONS,
    Migration,
)


class StateStoreError(RuntimeError):
    """Base class for expected SQLite state-store failures."""


class StateStoreBusyError(StateStoreError):
    """The state database is temporarily locked by another writer."""


class MigrationError(StateStoreError):
    """A schema migration could not be applied atomically."""


@dataclass(frozen=True, slots=True)
class StateDatabaseInfo:
    """Verified state database configuration."""

    schema_version: int
    journal_mode: str
    foreign_keys_enabled: bool


class SQLiteStateStore:
    """Initialize and migrate one application-owned SQLite database."""

    DEFAULT_MIGRATIONS = DEFAULT_MIGRATIONS

    def __init__(
        self,
        database_path: Path,
        *,
        migrations: tuple[Migration, ...] = DEFAULT_MIGRATIONS,
        busy_timeout_ms: int = 5_000,
    ) -> None:
        _validate_migrations(migrations)
        if busy_timeout_ms < 1:
            raise ValueError("busy_timeout_ms must be positive")
        self.database_path = database_path
        self.migrations = migrations
        self.busy_timeout_ms = busy_timeout_ms

    def initialize(self) -> StateDatabaseInfo:
        """Create and migrate the database, returning verified settings."""
        self.database_path.parent.mkdir(parents=True, exist_ok=True)
        try:
            with closing(self._connect()) as connection:
                journal_mode = self._configure(connection)
                self._ensure_migration_table(connection)
                schema_version = self._apply_pending_migrations(connection)
                self._verify_write_access(connection)
                foreign_keys_enabled = bool(
                    connection.execute("PRAGMA foreign_keys").fetchone()[0]
                )
        except sqlite3.Error as error:
            raise _state_store_error(error) from error
        return StateDatabaseInfo(
            schema_version=schema_version,
            journal_mode=journal_mode,
            foreign_keys_enabled=foreign_keys_enabled,
        )

    def _connect(self) -> sqlite3.Connection:
        return sqlite3.connect(
            self.database_path,
            timeout=self.busy_timeout_ms / 1_000,
            isolation_level=None,
        )

    def _configure(self, connection: sqlite3.Connection) -> str:
        connection.execute(f"PRAGMA busy_timeout={self.busy_timeout_ms}")
        journal_mode = connection.execute("PRAGMA journal_mode=WAL").fetchone()[0]
        connection.execute("PRAGMA foreign_keys=ON")
        return str(journal_mode).lower()

    def _ensure_migration_table(self, connection: sqlite3.Connection) -> None:
        connection.execute(
            """
            CREATE TABLE IF NOT EXISTS schema_migrations (
                version INTEGER PRIMARY KEY,
                name TEXT NOT NULL,
                applied_at TEXT NOT NULL
            )
            """
        )

    def _apply_pending_migrations(self, connection: sqlite3.Connection) -> int:
        applied_versions = {
            int(row[0])
            for row in connection.execute("SELECT version FROM schema_migrations")
        }
        known_versions = {migration.version for migration in self.migrations}
        unknown_versions = applied_versions - known_versions
        if unknown_versions:
            versions = ", ".join(str(version) for version in sorted(unknown_versions))
            raise MigrationError(f"Database contains unknown migrations: {versions}")
        for migration in self.migrations:
            if migration.version not in applied_versions:
                self._apply_migration(connection, migration)
                applied_versions.add(migration.version)
        return max(applied_versions, default=0)

    def _apply_migration(
        self,
        connection: sqlite3.Connection,
        migration: Migration,
    ) -> None:
        try:
            connection.execute("BEGIN IMMEDIATE")
            for statement in migration.statements:
                connection.execute(statement)
            connection.execute(
                """
                INSERT INTO schema_migrations (version, name, applied_at)
                VALUES (?, ?, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
                """,
                (migration.version, migration.name),
            )
            connection.commit()
        except sqlite3.Error as error:
            connection.rollback()
            if _is_busy(error):
                raise StateStoreBusyError("State database is busy") from error
            raise MigrationError(
                f"Failed to apply migration {migration.version} ({migration.name})"
            ) from error

    def _verify_write_access(self, connection: sqlite3.Connection) -> None:
        connection.execute("BEGIN IMMEDIATE")
        connection.commit()


def _validate_migrations(migrations: tuple[Migration, ...]) -> None:
    versions = [migration.version for migration in migrations]
    if versions != sorted(set(versions)):
        raise ValueError("Migration versions must be strictly increasing")


def _is_busy(error: sqlite3.Error) -> bool:
    message = str(error).lower()
    return "locked" in message or "busy" in message


def _state_store_error(error: sqlite3.Error) -> StateStoreError:
    if _is_busy(error):
        return StateStoreBusyError("State database is busy")
    return StateStoreError("State database initialization failed")
