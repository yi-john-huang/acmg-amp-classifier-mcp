from __future__ import annotations

import sqlite3
import unittest
from contextlib import closing
from datetime import UTC, datetime, timedelta
from pathlib import Path
from tempfile import TemporaryDirectory


class SQLiteRecordStoreTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temporary_directory = TemporaryDirectory()
        self.addCleanup(self.temporary_directory.cleanup)
        self.database_path = Path(self.temporary_directory.name) / "state.sqlite3"
        self.now = datetime(2026, 7, 11, 8, 0, tzinfo=UTC)

        from acmg_classifier.infrastructure.storage.sqlite import SQLiteStateStore

        SQLiteStateStore(self.database_path).initialize()

    def _store(self):
        from acmg_classifier.infrastructure.storage.records import SQLiteRecordStore

        return SQLiteRecordStore(self.database_path, clock=lambda: self.now)

    def test_draft_can_be_updated_until_it_expires(self) -> None:
        from acmg_classifier.infrastructure.storage.records import DraftExpiredError

        store = self._store()
        draft_id = store.create_draft(
            {"variant": "NM_007294.4:c.5266dupC"},
            expires_at=self.now + timedelta(hours=1),
        )

        initial = store.get_draft(draft_id)
        store.update_draft(
            draft_id,
            {"variant": "NM_007294.4:c.5266dupC", "build": "GRCh38"},
            expected_revision=initial.revision,
        )
        draft = store.get_draft(draft_id)

        self.assertTrue(draft_id.startswith("draft_"))
        self.assertEqual(
            draft.request_json,
            b'{"build":"GRCh38","variant":"NM_007294.4:c.5266dupC"}',
        )
        self.now += timedelta(hours=2)
        with self.assertRaises(DraftExpiredError):
            store.get_draft(draft_id)
        with self.assertRaises(DraftExpiredError):
            store.update_draft(
                draft_id,
                {"variant": "changed"},
                expected_revision=initial.revision + 1,
            )

    def test_draft_update_rejects_a_stale_revision(self) -> None:
        from acmg_classifier.infrastructure.storage.records import (
            DraftRevisionConflictError,
        )

        store = self._store()
        draft_id = store.create_draft(
            {"variant": "NM_007294.4:c.5266dupC"},
            expires_at=self.now + timedelta(hours=1),
        )
        initial = store.get_draft(draft_id)
        store.update_draft(
            draft_id,
            {"variant": "NM_007294.4:c.5266dupC", "build": "GRCh38"},
            expected_revision=initial.revision,
        )

        with self.assertRaises(DraftRevisionConflictError):
            store.update_draft(
                draft_id,
                {"variant": "NM_007294.4:c.5266dupC", "build": "GRCh37"},
                expected_revision=initial.revision,
            )

        draft = store.get_draft(draft_id)
        self.assertEqual(draft.revision, initial.revision + 1)
        self.assertEqual(
            draft.request_json,
            b'{"build":"GRCh38","variant":"NM_007294.4:c.5266dupC"}',
        )

    def test_finalization_rejects_a_stale_draft_revision(self) -> None:
        from acmg_classifier.infrastructure.storage.records import (
            DraftRevisionConflictError,
        )

        store = self._store()
        draft_id = store.create_draft(
            {"variant": "NM_007294.4:c.5266dupC"},
            expires_at=self.now + timedelta(hours=1),
        )
        initial = store.get_draft(draft_id)
        store.update_draft(
            draft_id,
            {"variant": "NM_007294.4:c.5266dupC", "build": "GRCh38"},
            expected_revision=initial.revision,
        )

        with self.assertRaises(DraftRevisionConflictError):
            store.finalize_classification(
                {"classification": "pathogenic"},
                draft_id=draft_id,
                expected_draft_revision=initial.revision,
            )

        self.assertIsNone(store.get_draft(draft_id).completed_classification_id)
        self.assertEqual(
            store.get_draft(draft_id).request_json,
            b'{"build":"GRCh38","variant":"NM_007294.4:c.5266dupC"}',
        )

    def test_finalization_completes_draft_and_record_is_immutable(self) -> None:
        from acmg_classifier.infrastructure.storage.records import (
            DraftCompletedError,
        )

        store = self._store()
        draft_id = store.create_draft(
            {"variant": "NM_007294.4:c.5266dupC"},
            expires_at=self.now + timedelta(hours=1),
        )

        classification_id = store.finalize_classification(
            {"classification": "pathogenic", "ruleset": "acmg-2015"},
            draft_id=draft_id,
            expected_draft_revision=0,
        )

        self.assertTrue(classification_id.startswith("cls_"))
        self.assertEqual(
            store.get_draft(draft_id).completed_classification_id,
            classification_id,
        )
        with self.assertRaises(DraftCompletedError):
            store.update_draft(
                draft_id,
                {"variant": "changed"},
                expected_revision=0,
            )
        with (
            closing(sqlite3.connect(self.database_path)) as connection,
            self.assertRaisesRegex(sqlite3.IntegrityError, "immutable"),
        ):
            connection.execute(
                """
                UPDATE classification_records
                SET canonical_json = ?
                WHERE classification_id = ?
                """,
                (b"{}", classification_id),
            )

    def test_reinterpretation_creates_linked_new_record(self) -> None:
        store = self._store()
        original_id = store.finalize_classification({"classification": "vus"})
        reinterpretation_id = store.finalize_classification(
            {"classification": "likely_pathogenic"},
            previous_classification_id=original_id,
        )

        self.assertNotEqual(original_id, reinterpretation_id)
        self.assertEqual(
            store.get_classification(reinterpretation_id).previous_classification_id,
            original_id,
        )
        self.assertEqual(
            store.get_classification(original_id).canonical_json,
            b'{"classification":"vus"}',
        )

    def test_invalid_draft_or_previous_record_rolls_back_classification(self) -> None:
        from acmg_classifier.infrastructure.storage.records import RecordNotFoundError

        store = self._store()

        with self.assertRaisesRegex(RecordNotFoundError, "draft_missing"):
            store.finalize_classification(
                {"classification": "vus"},
                draft_id="draft_missing",
                expected_draft_revision=0,
            )
        with self.assertRaisesRegex(RecordNotFoundError, "cls_missing"):
            store.finalize_classification(
                {"classification": "vus"},
                previous_classification_id="cls_missing",
            )

        with closing(sqlite3.connect(self.database_path)) as connection:
            count = connection.execute(
                "SELECT COUNT(*) FROM classification_records"
            ).fetchone()[0]
        self.assertEqual(count, 0)

    def test_review_feedback_and_audit_are_append_only(self) -> None:
        store = self._store()
        classification_id = store.finalize_classification({"classification": "vus"})

        first_feedback = store.append_feedback(
            classification_id,
            {"type": "agree", "rationale": "reviewed"},
        )
        second_feedback = store.append_feedback(
            classification_id,
            {"type": "agree", "rationale": "reviewed"},
        )
        review_id = store.append_review(
            classification_id,
            {"type": "conflict_review", "finding": "manual review needed"},
        )
        audit_id = store.append_audit({"event": "classification_created"})

        self.assertNotEqual(first_feedback, second_feedback)
        self.assertTrue(review_id.startswith("review_"))
        self.assertTrue(audit_id.startswith("audit_"))
        with (
            closing(sqlite3.connect(self.database_path)) as connection,
            self.assertRaisesRegex(sqlite3.IntegrityError, "immutable"),
        ):
            connection.execute(
                "UPDATE feedback SET canonical_json = ? WHERE feedback_id = ?",
                (b"{}", first_feedback),
            )

    def test_missing_records_and_naive_expiry_are_rejected(self) -> None:
        from acmg_classifier.infrastructure.storage.records import (
            RecordNotFoundError,
        )

        store = self._store()
        with self.assertRaisesRegex(ValueError, "timezone-aware"):
            store.create_draft({}, expires_at=datetime(2026, 7, 11))
        with self.assertRaisesRegex(RecordNotFoundError, "draft_missing"):
            store.get_draft("draft_missing")
        with self.assertRaisesRegex(RecordNotFoundError, "cls_missing"):
            store.get_classification("cls_missing")
        with self.assertRaisesRegex(RecordNotFoundError, "cls_missing"):
            store.append_feedback("cls_missing", {"type": "agree"})


if __name__ == "__main__":
    unittest.main()
