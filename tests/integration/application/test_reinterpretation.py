from __future__ import annotations

import unittest
from datetime import UTC, datetime
from pathlib import Path
from tempfile import TemporaryDirectory
from unittest.mock import patch

from acmg_classifier.application.reinterpretation import ReinterpretationService
from acmg_classifier.infrastructure.storage.records import SQLiteRecordStore
from acmg_classifier.infrastructure.storage.sqlite import SQLiteStateStore


class ReinterpretationServiceIntegrationTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temporary_directory = TemporaryDirectory()
        self.addCleanup(self.temporary_directory.cleanup)
        database = Path(self.temporary_directory.name) / "state.sqlite3"
        SQLiteStateStore(database).initialize()
        self.store = SQLiteRecordStore(
            database,
            clock=lambda: datetime(2026, 7, 11, 14, 0, tzinfo=UTC),
        )
        self.service = ReinterpretationService(self.store)

    def test_replay_returns_stored_record_without_requerying_work(self) -> None:
        classification_id = self.store.finalize_classification(_record_content())

        with patch(
            "socket.create_connection",
            side_effect=AssertionError("replay must not open a network socket"),
        ) as network:
            replay = self.service.replay(classification_id)
        network.assert_not_called()

        self.assertEqual(replay.to_canonical_content(), _record_content())
        with self.assertRaises(TypeError):
            replay.content["bundle_version"] = "changed"  # type: ignore[index]
        with self.assertRaises(TypeError):
            replay.content["context"]["disease_id"] = "changed"  # type: ignore[index]

    def test_difference_reports_causal_evidence_context_ruleset_and_decision_changes(
        self,
    ) -> None:
        first_id = self.store.finalize_classification(_record_content())
        second_id = self.store.finalize_classification(
            _record_content(
                disease_id="MONDO:0000002",
                evidence_ids=["ev_b", "ev_c"],
                ruleset_version="2.0.0",
                bundle_version="core-test-2",
                classification="likely_pathogenic",
                pm2_status="not_applied",
            ),
            previous_classification_id=first_id,
        )

        difference = self.service.difference(first_id, second_id)

        self.assertEqual(difference.previous_classification_id, first_id)
        self.assertEqual(difference.classification_before, "uncertain_significance")
        self.assertEqual(difference.classification_after, "likely_pathogenic")
        self.assertEqual(difference.evidence_added, ("ev_c",))
        self.assertEqual(difference.evidence_removed, ("ev_a",))
        self.assertEqual(
            difference.context_changes["disease_id"],
            ("MONDO:0000001", "MONDO:0000002"),
        )
        self.assertTrue(difference.ruleset_changed)
        self.assertTrue(difference.bundle_changed)
        self.assertEqual(difference.criteria_changed, ("PM2",))
        self.assertEqual(
            difference.to_canonical_content()["context_changes"]["disease_id"],
            ["MONDO:0000001", "MONDO:0000002"],
        )


def _record_content(
    *,
    disease_id: str = "MONDO:0000001",
    evidence_ids: list[str] | None = None,
    ruleset_version: str = "1.0.0",
    bundle_version: str = "core-test-1",
    classification: str = "uncertain_significance",
    pm2_status: str = "applied",
) -> dict[str, object]:
    return {
        "schema_version": "1.0",
        "context": {
            "genome_build": "GRCh38",
            "transcript": "NM_000001.1",
            "disease_id": disease_id,
            "disease_label": "Test disease",
            "inheritance": "autosomal_dominant",
        },
        "evidence_snapshot": {
            "snapshot_id": "snap_test",
            "content": {"evidence_ids": evidence_ids or ["ev_a", "ev_b"]},
        },
        "decision": {
            "classification": classification,
            "assessments": [
                {
                    "code": "PM2",
                    "status": pm2_status,
                    "applied_strength": "moderate" if pm2_status == "applied" else None,
                }
            ],
        },
        "ruleset": {"id": "test", "version": ruleset_version},
        "bundle_version": bundle_version,
    }


if __name__ == "__main__":
    unittest.main()
