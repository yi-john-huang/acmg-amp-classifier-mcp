from __future__ import annotations

import sqlite3
import unittest
from contextlib import closing
from pathlib import Path
from tempfile import TemporaryDirectory


class SQLiteStateStoreTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temporary_directory = TemporaryDirectory()
        self.addCleanup(self.temporary_directory.cleanup)
        self.database_path = (
            Path(self.temporary_directory.name) / "state" / "state.sqlite3"
        )

    def test_initialize_creates_application_database_with_safe_pragmas(self) -> None:
        from acmg_classifier.infrastructure.storage.sqlite import SQLiteStateStore

        store = SQLiteStateStore(self.database_path)

        first = store.initialize()
        second = store.initialize()

        self.assertTrue(self.database_path.is_file())
        expected_version = store.DEFAULT_MIGRATIONS[-1].version
        self.assertEqual(first.schema_version, expected_version)
        self.assertEqual(second.schema_version, expected_version)
        self.assertEqual(second.journal_mode, "wal")
        self.assertTrue(second.foreign_keys_enabled)
        with closing(sqlite3.connect(self.database_path)) as connection:
            migration_count = connection.execute(
                "SELECT COUNT(*) FROM schema_migrations"
            ).fetchone()[0]
        self.assertEqual(migration_count, len(store.DEFAULT_MIGRATIONS))

    def test_forward_migration_runs_once(self) -> None:
        from acmg_classifier.infrastructure.storage.migrations import Migration
        from acmg_classifier.infrastructure.storage.sqlite import SQLiteStateStore

        initial = Migration(
            version=1,
            name="initial",
            statements=("CREATE TABLE first_record (id TEXT PRIMARY KEY)",),
        )
        forward = Migration(
            version=2,
            name="forward",
            statements=("CREATE TABLE second_record (id TEXT PRIMARY KEY)",),
        )

        SQLiteStateStore(self.database_path, migrations=(initial,)).initialize()
        result = SQLiteStateStore(
            self.database_path,
            migrations=(initial, forward),
        ).initialize()

        self.assertEqual(result.schema_version, 2)
        with closing(sqlite3.connect(self.database_path)) as connection:
            names = {
                row[0]
                for row in connection.execute(
                    "SELECT name FROM sqlite_master WHERE type = 'table'"
                )
            }
            versions = connection.execute(
                "SELECT version FROM schema_migrations ORDER BY version"
            ).fetchall()
        self.assertIn("first_record", names)
        self.assertIn("second_record", names)
        self.assertEqual(versions, [(1,), (2,)])

    def test_failed_migration_rolls_back_every_statement_and_version(self) -> None:
        from acmg_classifier.infrastructure.storage.migrations import Migration
        from acmg_classifier.infrastructure.storage.sqlite import (
            MigrationError,
            SQLiteStateStore,
        )

        broken = Migration(
            version=SQLiteStateStore.DEFAULT_MIGRATIONS[-1].version + 1,
            name="broken",
            statements=(
                "CREATE TABLE must_rollback (id TEXT PRIMARY KEY)",
                "THIS IS NOT SQL",
            ),
        )
        SQLiteStateStore(self.database_path).initialize()

        with self.assertRaisesRegex(MigrationError, "broken"):
            SQLiteStateStore(
                self.database_path,
                migrations=(*SQLiteStateStore.DEFAULT_MIGRATIONS, broken),
            ).initialize()

        with closing(sqlite3.connect(self.database_path)) as connection:
            table = connection.execute(
                "SELECT name FROM sqlite_master WHERE name = 'must_rollback'"
            ).fetchone()
            versions = connection.execute(
                "SELECT version FROM schema_migrations ORDER BY version"
            ).fetchall()
        self.assertIsNone(table)
        self.assertEqual(
            versions,
            [(migration.version,) for migration in SQLiteStateStore.DEFAULT_MIGRATIONS],
        )

    def test_database_lock_returns_specific_retryable_error(self) -> None:
        from acmg_classifier.infrastructure.storage.sqlite import (
            SQLiteStateStore,
            StateStoreBusyError,
        )

        store = SQLiteStateStore(self.database_path, busy_timeout_ms=20)
        store.initialize()
        locking_connection = sqlite3.connect(self.database_path)
        self.addCleanup(locking_connection.close)
        locking_connection.execute("BEGIN EXCLUSIVE")

        with self.assertRaises(StateStoreBusyError):
            store.initialize()

    def test_invalid_migration_order_is_rejected_before_opening_database(self) -> None:
        from acmg_classifier.infrastructure.storage.migrations import Migration
        from acmg_classifier.infrastructure.storage.sqlite import SQLiteStateStore

        duplicate_versions = (
            Migration(version=1, name="one", statements=("SELECT 1",)),
            Migration(version=1, name="duplicate", statements=("SELECT 1",)),
        )

        with self.assertRaisesRegex(ValueError, "strictly increasing"):
            SQLiteStateStore(self.database_path, migrations=duplicate_versions)
        self.assertFalse(self.database_path.exists())

    def test_invalid_migration_definitions_and_timeout_are_rejected(self) -> None:
        from acmg_classifier.infrastructure.storage.migrations import Migration
        from acmg_classifier.infrastructure.storage.sqlite import SQLiteStateStore

        invalid_migrations = (
            {"version": 0, "name": "invalid", "statements": ("SELECT 1",)},
            {"version": 1, "name": "", "statements": ("SELECT 1",)},
            {"version": 1, "name": "empty", "statements": ()},
        )
        for invalid_migration in invalid_migrations:
            with (
                self.subTest(invalid_migration=invalid_migration),
                self.assertRaises(ValueError),
            ):
                Migration(**invalid_migration)

        with self.assertRaisesRegex(ValueError, "busy_timeout_ms"):
            SQLiteStateStore(self.database_path, busy_timeout_ms=0)

    def test_database_with_unknown_migration_is_rejected(self) -> None:
        from acmg_classifier.infrastructure.storage.sqlite import (
            MigrationError,
            SQLiteStateStore,
        )

        SQLiteStateStore(self.database_path).initialize()
        with closing(sqlite3.connect(self.database_path)) as connection:
            connection.execute(
                """
                INSERT INTO schema_migrations (version, name, applied_at)
                VALUES (99, 'future', '2026-07-11T00:00:00Z')
                """
            )
            connection.commit()

        with self.assertRaisesRegex(MigrationError, "unknown migrations: 99"):
            SQLiteStateStore(self.database_path).initialize()

    def test_unopenable_database_returns_state_store_error(self) -> None:
        from acmg_classifier.infrastructure.storage.sqlite import (
            SQLiteStateStore,
            StateStoreError,
        )

        self.database_path.mkdir(parents=True)

        with self.assertRaisesRegex(StateStoreError, "initialization failed"):
            SQLiteStateStore(self.database_path).initialize()


if __name__ == "__main__":
    unittest.main()
