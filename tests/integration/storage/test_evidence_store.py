from __future__ import annotations

import sqlite3
import unittest
from contextlib import closing
from datetime import UTC, datetime
from pathlib import Path
from tempfile import TemporaryDirectory


class SQLiteEvidenceStoreTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temporary_directory = TemporaryDirectory()
        self.addCleanup(self.temporary_directory.cleanup)
        root = Path(self.temporary_directory.name)
        self.database_path = root / "state.sqlite3"
        self.raw_root = root / "raw"

        from acmg_classifier.infrastructure.storage.sqlite import SQLiteStateStore

        SQLiteStateStore(self.database_path).initialize()

    def test_evidence_is_content_addressed_and_deduplicated(self) -> None:
        from acmg_classifier.infrastructure.storage.evidence import SQLiteEvidenceStore

        store = SQLiteEvidenceStore(self.database_path, self.raw_root)

        first = store.put_evidence({"source": "clinvar", "record_id": "VCV000012345"})
        second = store.put_evidence({"record_id": "VCV000012345", "source": "clinvar"})

        self.assertEqual(first, second)
        self.assertTrue(first.startswith("ev_"))
        self.assertEqual(
            store.get_evidence(first),
            b'{"record_id":"VCV000012345","source":"clinvar"}',
        )
        with closing(sqlite3.connect(self.database_path)) as connection:
            count = connection.execute(
                "SELECT COUNT(*) FROM evidence_items"
            ).fetchone()[0]
        self.assertEqual(count, 1)

    def test_evidence_rows_cannot_be_updated_or_deleted(self) -> None:
        from acmg_classifier.infrastructure.storage.evidence import SQLiteEvidenceStore

        store = SQLiteEvidenceStore(self.database_path, self.raw_root)
        evidence_id = store.put_evidence({"source": "gnomad", "allele_count": 0})

        with closing(sqlite3.connect(self.database_path)) as connection:
            with self.assertRaisesRegex(sqlite3.IntegrityError, "immutable"):
                connection.execute(
                    """
                    UPDATE evidence_items
                    SET canonical_json = ?
                    WHERE evidence_id = ?
                    """,
                    (b"{}", evidence_id),
                )
            with self.assertRaisesRegex(sqlite3.IntegrityError, "immutable"):
                connection.execute(
                    "DELETE FROM evidence_items WHERE evidence_id = ?",
                    (evidence_id,),
                )

    def test_missing_evidence_and_snapshot_are_explicit(self) -> None:
        from acmg_classifier.infrastructure.storage.evidence import (
            EvidenceNotFoundError,
            SQLiteEvidenceStore,
        )

        store = SQLiteEvidenceStore(self.database_path, self.raw_root)

        with self.assertRaisesRegex(EvidenceNotFoundError, "ev_missing"):
            store.get_evidence("ev_missing")
        with self.assertRaisesRegex(EvidenceNotFoundError, "es_missing"):
            store.get_evidence_snapshot("es_missing")

    def test_raw_snapshot_is_hashed_stored_and_verified_on_read(self) -> None:
        from acmg_classifier.infrastructure.storage.evidence import (
            RawSnapshotMissingError,
            SQLiteEvidenceStore,
        )

        store = SQLiteEvidenceStore(self.database_path, self.raw_root)

        reference = store.put_raw_snapshot(b'{"result":"ok"}', "application/json")

        self.assertTrue(reference.snapshot_hash.startswith("raw_"))
        self.assertEqual(reference.byte_size, 15)
        self.assertEqual(
            store.get_raw_snapshot(reference.snapshot_hash), b'{"result":"ok"}'
        )
        (self.raw_root / reference.relative_path).unlink()
        with self.assertRaises(RawSnapshotMissingError):
            store.get_raw_snapshot(reference.snapshot_hash)

    def test_raw_snapshot_reuse_and_corruption_are_detected(self) -> None:
        from acmg_classifier.infrastructure.storage.evidence import (
            ContentHashCollisionError,
            RawSnapshotIntegrityError,
            SQLiteEvidenceStore,
        )

        store = SQLiteEvidenceStore(self.database_path, self.raw_root)
        first = store.put_raw_snapshot(b"source bytes", "application/octet-stream")
        second = store.put_raw_snapshot(b"source bytes", "application/octet-stream")

        self.assertEqual(first, second)
        with self.assertRaises(ContentHashCollisionError):
            store.put_raw_snapshot(b"source bytes", "text/plain")
        path = self.raw_root / first.relative_path
        path.write_bytes(b"corrupted")
        with self.assertRaises(RawSnapshotIntegrityError):
            store.get_raw_snapshot(first.snapshot_hash)
        with self.assertRaises(RawSnapshotIntegrityError):
            store.put_raw_snapshot(b"source bytes", "application/octet-stream")

    def test_raw_snapshot_size_is_bounded_before_writing(self) -> None:
        from acmg_classifier.infrastructure.storage.evidence import (
            RawSnapshotTooLargeError,
            SQLiteEvidenceStore,
        )

        store = SQLiteEvidenceStore(self.database_path, self.raw_root, max_raw_bytes=3)

        with self.assertRaisesRegex(RawSnapshotTooLargeError, "3 bytes"):
            store.put_raw_snapshot(b"four", "text/plain")
        self.assertFalse(self.raw_root.exists())

        with self.assertRaisesRegex(ValueError, "max_raw_bytes"):
            SQLiteEvidenceStore(self.database_path, self.raw_root, max_raw_bytes=0)
        with self.assertRaisesRegex(ValueError, "media_type"):
            SQLiteEvidenceStore(self.database_path, self.raw_root).put_raw_snapshot(
                b"ok", ""
            )

    def test_evidence_snapshot_is_order_independent_and_retrievable(self) -> None:
        from acmg_classifier.infrastructure.storage.evidence import SQLiteEvidenceStore

        store = SQLiteEvidenceStore(self.database_path, self.raw_root)
        first = store.put_evidence({"source": "clinvar"})
        second = store.put_evidence({"source": "gnomad"})

        snapshot_a = store.put_evidence_snapshot(
            evidence_ids=(first, second),
            source_status={"clinvar": "fresh", "gnomad": "fresh"},
        )
        snapshot_b = store.put_evidence_snapshot(
            evidence_ids=(second, first, second),
            source_status={"gnomad": "fresh", "clinvar": "fresh"},
        )

        self.assertEqual(snapshot_a, snapshot_b)
        snapshot = store.get_evidence_snapshot(snapshot_a)
        self.assertEqual(snapshot.evidence_ids, tuple(sorted((first, second))))
        self.assertEqual(
            snapshot.source_status_json,
            b'{"clinvar":"fresh","gnomad":"fresh"}',
        )
        with closing(sqlite3.connect(self.database_path)) as connection:
            with self.assertRaisesRegex(sqlite3.IntegrityError, "immutable"):
                connection.execute(
                    """
                    UPDATE evidence_snapshots
                    SET source_status_json = ?
                    WHERE snapshot_id = ?
                    """,
                    (b"{}", snapshot_a),
                )
            with self.assertRaisesRegex(sqlite3.IntegrityError, "immutable"):
                connection.execute(
                    "DELETE FROM evidence_snapshots WHERE snapshot_id = ?",
                    (snapshot_a,),
                )

    def test_domain_degraded_empty_snapshot_is_persisted_without_placeholder(
        self,
    ) -> None:
        from acmg_classifier.domain.evidence import (
            EvidencePolicy,
            EvidenceSnapshot,
            SourceStatus,
            SourceStatusValue,
        )
        from acmg_classifier.infrastructure.storage.evidence import SQLiteEvidenceStore

        store = SQLiteEvidenceStore(self.database_path, self.raw_root)
        snapshot = EvidenceSnapshot(
            evidence_ids=(),
            source_statuses=(
                SourceStatus(
                    source_id="clinvar",
                    status=SourceStatusValue.UNAVAILABLE,
                    checked_at=datetime(2026, 7, 11, tzinfo=UTC),
                    detail="offline_no_eligible_cache",
                ),
            ),
            policy=EvidencePolicy(mode="offline"),
            created_at=datetime(2026, 7, 11, tzinfo=UTC),
        )

        self.assertEqual(
            store.put_domain_evidence_snapshot(snapshot), snapshot.snapshot_id
        )
        stored = store.get_evidence_snapshot(snapshot.snapshot_id)
        self.assertEqual(stored.evidence_ids, ())
        self.assertEqual(
            stored.source_status_json,
            b'{"policy":{"max_age_seconds":null,"mode":"offline"},"source_statuses":[{"checked_at":"2026-07-11T00:00:00Z","detail":"offline_no_eligible_cache","normalized_query_key":null,"source_id":"clinvar","source_version":null,"status":"unavailable"}]}',
        )

    def test_snapshot_rejects_unknown_evidence(self) -> None:
        from acmg_classifier.infrastructure.storage.evidence import (
            EvidenceNotFoundError,
            SQLiteEvidenceStore,
        )

        store = SQLiteEvidenceStore(self.database_path, self.raw_root)

        with self.assertRaisesRegex(EvidenceNotFoundError, "ev_missing"):
            store.put_evidence_snapshot(
                evidence_ids=("ev_missing",),
                source_status={"clinvar": "unavailable"},
            )


if __name__ == "__main__":
    unittest.main()
