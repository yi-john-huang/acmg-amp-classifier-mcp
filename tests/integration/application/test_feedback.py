from __future__ import annotations

import json
import sqlite3
from contextlib import closing
from datetime import UTC, datetime
from pathlib import Path

import pytest

from acmg_classifier.application.feedback import FeedbackService, FeedbackSubmission
from acmg_classifier.domain.errors import JsonValue
from acmg_classifier.domain.feedback import FeedbackType
from acmg_classifier.infrastructure.storage.records import SQLiteRecordStore
from acmg_classifier.infrastructure.storage.sqlite import SQLiteStateStore


@pytest.fixture
def feedback_store(tmp_path: Path) -> SQLiteRecordStore:
    database_path = tmp_path / "state.sqlite3"
    SQLiteStateStore(database_path).initialize()
    return SQLiteRecordStore(
        database_path,
        clock=lambda: datetime(2026, 7, 11, 12, 0, tzinfo=UTC),
    )


def test_feedback_is_append_only_and_queryable_by_canonical_variant_context(
    feedback_store: SQLiteRecordStore,
) -> None:
    classification_id = feedback_store.finalize_classification(
        _classification_content()
    )
    service = FeedbackService(
        feedback_store,
        clock=lambda: datetime(2026, 7, 11, 12, 0, tzinfo=UTC),
    )

    record = service.submit(
        FeedbackSubmission(
            classification_id=classification_id,
            feedback_type=FeedbackType.CORRECTION,
            proposed_correction="Likely Pathogenic",
            rationale="Validated segregation evidence changes expert review.",
            evidence_references=("PMID:12345",),
            actor_id="usr_scientist-1",
        )
    )

    assert record.feedback_id.startswith("fb_")
    assert record.classification_id == classification_id
    assert record.variant_key == "cak1:GRCh38:NC_000001.11:100:A>G"
    assert record.context.disease_id == "MONDO:0000001"
    assert (
        json.loads(feedback_store.get_classification(classification_id).canonical_json)
        == _classification_content()
    )
    assert service.query(
        variant_key="cak1:GRCh38:NC_000001.11:100:A>G",
        context=record.context,
    ) == (record,)

    with (
        closing(sqlite3.connect(feedback_store.database_path)) as connection,
        pytest.raises(
            sqlite3.IntegrityError,
            match="immutable",
        ),
    ):
        connection.execute(
            "UPDATE feedback SET canonical_json = ? WHERE feedback_id = ?",
            (b"{}", record.feedback_id),
        )


def test_feedback_export_import_preserves_ids_and_audit_content(
    feedback_store: SQLiteRecordStore,
    tmp_path: Path,
) -> None:
    classification_id = feedback_store.finalize_classification(
        _classification_content()
    )
    source = FeedbackService(
        feedback_store,
        clock=lambda: datetime(2026, 7, 11, 12, 0, tzinfo=UTC),
    )
    submitted = source.submit(
        FeedbackSubmission(
            classification_id=classification_id,
            feedback_type=FeedbackType.AGREEMENT,
            rationale="Reviewed against the cited evidence.",
            evidence_references=("PMID:12345",),
            actor_id="usr_scientist-1",
        )
    )
    exported = source.export()

    target_path = tmp_path / "target.sqlite3"
    SQLiteStateStore(target_path).initialize()
    with closing(sqlite3.connect(target_path)) as connection, connection:
        connection.execute(
            """
            INSERT INTO classification_records
                (
                    classification_id, canonical_json, previous_classification_id,
                    created_at
                )
            VALUES (?, ?, ?, ?)
            """,
            (
                classification_id,
                json.dumps(_classification_content()).encode(),
                None,
                "2026-07-11T12:00:00+00:00",
            ),
        )
        connection.commit()
    target_store = SQLiteRecordStore(target_path)
    target = FeedbackService(target_store)

    imported_ids = target.import_records(exported)

    assert imported_ids == (submitted.feedback_id,)
    assert target.export() == exported


def test_feedback_import_is_atomic_when_any_referenced_record_is_missing(
    feedback_store: SQLiteRecordStore,
    tmp_path: Path,
) -> None:
    classification_id = feedback_store.finalize_classification(
        _classification_content()
    )
    source = FeedbackService(feedback_store)
    submitted = source.submit(
        FeedbackSubmission(
            classification_id=classification_id,
            feedback_type=FeedbackType.AGREEMENT,
            rationale="Reviewed against primary evidence.",
            actor_id="usr_scientist-1",
        )
    )
    invalid = submitted.model_copy(
        update={
            "feedback_id": "fb_" + "d" * 32,
            "classification_id": "cls_" + "e" * 32,
        }
    )

    target_path = tmp_path / "atomic-target.sqlite3"
    SQLiteStateStore(target_path).initialize()
    with closing(sqlite3.connect(target_path)) as connection, connection:
        connection.execute(
            """
            INSERT INTO classification_records
                (
                    classification_id, canonical_json, previous_classification_id,
                    created_at
                )
            VALUES (?, ?, ?, ?)
            """,
            (
                classification_id,
                json.dumps(_classification_content()).encode(),
                None,
                "2026-07-11T12:00:00+00:00",
            ),
        )
        connection.commit()
    target = FeedbackService(SQLiteRecordStore(target_path))

    with pytest.raises(Exception, match="feedback import failed"):
        target.import_records((submitted, invalid))

    assert target.export() == ()


def _classification_content() -> dict[str, JsonValue]:
    return {
        "schema_version": "1.0",
        "normalized_variant": {
            "canonical_key": {
                "schema_version": "1.0",
                "assembly": "GRCh38",
                "sequence_accession": "NC_000001.11",
                "start": 100,
                "end": 101,
                "deleted_sequence": "A",
                "inserted_sequence": "G",
            }
        },
        "context": {
            "genome_build": "GRCh38",
            "transcript": "NM_000001.1",
            "disease_id": "MONDO:0000001",
            "disease_label": "Test disease",
            "inheritance": "autosomal_dominant",
        },
    }
