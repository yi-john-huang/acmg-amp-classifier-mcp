from __future__ import annotations

import asyncio
import sqlite3
import unittest
from datetime import UTC, datetime
from pathlib import Path
from tempfile import TemporaryDirectory

from acmg_classifier.domain.enums import GenomeBuild
from acmg_classifier.domain.evidence import (
    EvidenceContextScope,
    EvidencePolicy,
    SourceStatus,
    SourceStatusValue,
)
from acmg_classifier.infrastructure.storage.evidence import SQLiteEvidenceStore
from acmg_classifier.infrastructure.storage.sqlite import SQLiteStateStore
from acmg_classifier.ports.evidence import CacheState, EvidenceSourceResult


class _Variant:
    variant_key = "ga4gh:VA.transaction"
    genome_build = "GRCh38"
    genomic_hgvs = "NC_000017.11:g.43071077G>A"


class _NetworkWaitingAdapter:
    source_id = "clinvar"

    def __init__(self, started: asyncio.Event, release: asyncio.Event) -> None:
        self._started = started
        self._release = release

    async def query(
        self, variant: _Variant, *, policy: EvidencePolicy
    ) -> EvidenceSourceResult:
        self._started.set()
        await self._release.wait()
        return EvidenceSourceResult(
            source_id=self.source_id,
            evidence_items=(),
            source_status=SourceStatus(
                source_id=self.source_id,
                status=SourceStatusValue.UNAVAILABLE,
                checked_at=datetime(2026, 7, 11, tzinfo=UTC),
                detail="source_down",
            ),
            cache_state=CacheState.LIVE,
        )


class EvidenceOrchestratorStorageIntegrationTests(unittest.IsolatedAsyncioTestCase):
    async def test_network_wait_does_not_hold_a_sqlite_write_transaction(self) -> None:
        from acmg_classifier.application.evidence_orchestrator import (
            EvidenceOrchestrator,
        )

        with TemporaryDirectory() as temporary_directory:
            root = Path(temporary_directory)
            database = root / "state.sqlite3"
            SQLiteStateStore(database).initialize()
            store = SQLiteEvidenceStore(database, root / "raw")
            started = asyncio.Event()
            release = asyncio.Event()
            task = asyncio.create_task(
                EvidenceOrchestrator(
                    adapters=(_NetworkWaitingAdapter(started, release),),
                    evidence_store=store,
                    clock=lambda: datetime(2026, 7, 11, tzinfo=UTC),
                    total_deadline_seconds=1.0,
                ).gather(
                    _Variant(),
                    EvidenceContextScope(genome_build=GenomeBuild.GRCH38),
                    EvidencePolicy(mode="live"),
                )
            )
            await asyncio.wait_for(started.wait(), timeout=0.5)
            with sqlite3.connect(database, timeout=0) as writer:
                writer.execute("BEGIN IMMEDIATE")
                writer.execute("ROLLBACK")
            release.set()
            result = await task
            self.assertEqual(result.snapshot.evidence_ids, ())
            self.assertEqual(
                result.snapshot.source_statuses[0].status, SourceStatusValue.UNAVAILABLE
            )


if __name__ == "__main__":
    unittest.main()
