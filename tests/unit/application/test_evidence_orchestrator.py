from __future__ import annotations

import asyncio
import sqlite3
import unittest
from dataclasses import dataclass
from datetime import UTC, datetime
from pathlib import Path
from tempfile import TemporaryDirectory

from acmg_classifier.domain.enums import GenomeBuild, InheritanceMode
from acmg_classifier.domain.evidence import (
    EvidenceContextScope,
    EvidenceDerivation,
    EvidenceItem,
    EvidencePolicy,
    ObservationKind,
    PopulationObservation,
    SourceProvenance,
    SourceStatus,
    SourceStatusValue,
)
from acmg_classifier.infrastructure.storage.evidence import SQLiteEvidenceStore
from acmg_classifier.infrastructure.storage.sqlite import SQLiteStateStore
from acmg_classifier.ports.evidence import CacheState, EvidenceSourceResult

NOW = datetime(2026, 7, 11, 12, 0, tzinfo=UTC)
VARIANT_KEY = "ga4gh:VA.orchestrator"
SCOPE = EvidenceContextScope(genome_build=GenomeBuild.GRCH38)
COMPLETE_SCOPE = EvidenceContextScope(
    genome_build=GenomeBuild.GRCH38,
    transcript="NM_007294.4",
    disease_id="MONDO:0011450",
    inheritance=InheritanceMode.AUTOSOMAL_DOMINANT,
)
POLICY = EvidencePolicy(mode="live")


@dataclass(frozen=True, slots=True)
class FakeVariant:
    variant_key: str = VARIANT_KEY
    genome_build: str = "GRCh38"
    genomic_hgvs: str | None = "NC_000017.11:g.43071077G>A"


def item_for(
    source_id: str,
    record_id: str,
    *,
    context_scope: EvidenceContextScope = SCOPE,
) -> EvidenceItem:
    return EvidenceItem(
        variant_key=VARIANT_KEY,
        kind=ObservationKind.POPULATION,
        observation=PopulationObservation(
            kind=ObservationKind.POPULATION,
            source_release="test-1",
            ancestry=record_id,
            allele_count=0,
            allele_number=100,
            allele_frequency=0.0,
            filter_status="pass",
        ),
        context_scope=context_scope,
        provenance=SourceProvenance(
            kind=EvidenceDerivation.SOURCE,
            source_id=source_id,
            source_record_id=record_id,
            source_version="test-1",
            retrieved_at=NOW,
            normalized_query_key=VARIANT_KEY,
        ),
        raw_snapshot_ref="raw_" + "a" * 64,
        derivation=EvidenceDerivation.SOURCE,
    )


def status_for(
    source_id: str, value: SourceStatusValue = SourceStatusValue.FRESH
) -> SourceStatus:
    return SourceStatus(
        source_id=source_id,
        status=value,
        checked_at=NOW,
        normalized_query_key=VARIANT_KEY,
    )


class FakeAdapter:
    def __init__(
        self,
        source_id: str,
        result: EvidenceSourceResult | Exception,
        *,
        started: asyncio.Event | None = None,
        release: asyncio.Event | None = None,
        started_count: list[str] | None = None,
        all_started: asyncio.Event | None = None,
    ) -> None:
        self.source_id = source_id
        self._result = result
        self._started = started
        self._release = release
        self._started_count = started_count
        self._all_started = all_started
        self.calls = 0
        self.context_scopes: list[EvidenceContextScope] = []
        self.cancelled = False

    async def query(
        self,
        variant: FakeVariant,
        *,
        context_scope: EvidenceContextScope,
        policy: EvidencePolicy,
    ) -> EvidenceSourceResult:
        self.calls += 1
        self.context_scopes.append(context_scope)
        if self._started is not None:
            self._started.set()
        if self._started_count is not None:
            self._started_count.append(self.source_id)
            if len(self._started_count) == 2 and self._all_started is not None:
                self._all_started.set()
        try:
            if self._release is not None:
                await self._release.wait()
        except asyncio.CancelledError:
            self.cancelled = True
            raise
        if isinstance(self._result, Exception):
            raise self._result
        return self._result


def result_for(
    source_id: str,
    *,
    items: tuple[EvidenceItem, ...] = (),
    status: SourceStatusValue = SourceStatusValue.FRESH,
    cache_state: CacheState = CacheState.LIVE,
) -> EvidenceSourceResult:
    return EvidenceSourceResult(
        source_id=source_id,
        evidence_items=items,
        source_status=status_for(source_id, status),
        cache_state=cache_state,
    )


class EvidenceOrchestratorTests(unittest.IsolatedAsyncioTestCase):
    def setUp(self) -> None:
        self.temporary_directory = TemporaryDirectory()
        self.addCleanup(self.temporary_directory.cleanup)
        root = Path(self.temporary_directory.name)
        database = root / "state.sqlite3"
        self.database_path = database
        SQLiteStateStore(database).initialize()
        self.store = SQLiteEvidenceStore(database, root / "raw")

    def orchestrator(self, *adapters: FakeAdapter, deadline: float = 1.0):
        from acmg_classifier.application.evidence_orchestrator import (
            EvidenceOrchestrator,
        )

        return EvidenceOrchestrator(
            adapters=adapters,
            evidence_store=self.store,
            clock=lambda: NOW,
            total_deadline_seconds=deadline,
        )

    async def test_complete_context_reaches_adapters_with_source_specific_scopes(
        self,
    ) -> None:
        clinvar_scope = EvidenceContextScope(
            genome_build=GenomeBuild.GRCH38, disease_id="MONDO:0011450"
        )
        clinvar = FakeAdapter(
            "clinvar",
            result_for(
                "clinvar",
                items=(
                    item_for(
                        "clinvar", "clinical", context_scope=clinvar_scope
                    ),
                ),
            ),
        )
        gnomad = FakeAdapter(
            "gnomad",
            result_for(
                "gnomad",
                items=(item_for("gnomad", "population", context_scope=SCOPE),),
            ),
        )

        result = await self.orchestrator(clinvar, gnomad).gather(
            FakeVariant(), COMPLETE_SCOPE, POLICY
        )

        self.assertFalse(result.degraded)
        self.assertEqual(len(result.evidence_items), 2)
        self.assertEqual(clinvar.context_scopes, [COMPLETE_SCOPE])
        self.assertEqual(gnomad.context_scopes, [COMPLETE_SCOPE])

    async def test_adapters_start_concurrently_without_completion_timing(self) -> None:
        release = asyncio.Event()
        all_started = asyncio.Event()
        started: list[str] = []
        clinvar = FakeAdapter(
            "clinvar",
            result_for("clinvar", items=(item_for("clinvar", "clinvar"),)),
            release=release,
            started_count=started,
            all_started=all_started,
        )
        gnomad = FakeAdapter(
            "gnomad",
            result_for("gnomad", items=(item_for("gnomad", "gnomad"),)),
            release=release,
            started_count=started,
            all_started=all_started,
        )
        task = asyncio.create_task(
            self.orchestrator(clinvar, gnomad).gather(FakeVariant(), SCOPE, POLICY)
        )
        await asyncio.wait_for(all_started.wait(), timeout=0.5)
        self.assertCountEqual(started, ("clinvar", "gnomad"))
        self.assertFalse(task.done())
        release.set()
        result = await task
        self.assertFalse(result.degraded)

    async def test_duplicate_items_are_deterministic_across_completion_order(
        self,
    ) -> None:
        shared = item_for("clinvar", "shared")
        first_release = asyncio.Event()
        second_release = asyncio.Event()
        first_started = asyncio.Event()
        second_started = asyncio.Event()
        first = FakeAdapter(
            "clinvar",
            result_for(
                "clinvar",
                items=(shared,),
                status=SourceStatusValue.CACHED,
                cache_state=CacheState.HIT_FRESH,
            ),
            started=first_started,
            release=first_release,
        )
        second = FakeAdapter(
            "gnomad",
            result_for("gnomad", items=(shared, item_for("gnomad", "unique"))),
            started=second_started,
            release=second_release,
        )
        task = asyncio.create_task(
            self.orchestrator(first, second).gather(FakeVariant(), SCOPE, POLICY)
        )
        await asyncio.wait_for(
            asyncio.gather(first_started.wait(), second_started.wait()), timeout=0.5
        )
        second_release.set()
        await asyncio.sleep(0)
        first_release.set()
        reversed_result = await task

        normal_result = await self.orchestrator(
            FakeAdapter(
                "clinvar",
                result_for(
                    "clinvar",
                    items=(shared,),
                    status=SourceStatusValue.CACHED,
                    cache_state=CacheState.HIT_FRESH,
                ),
            ),
            FakeAdapter(
                "gnomad",
                result_for("gnomad", items=(shared, item_for("gnomad", "unique"))),
            ),
        ).gather(FakeVariant(), SCOPE, POLICY)

        self.assertEqual(
            reversed_result.snapshot.snapshot_id, normal_result.snapshot.snapshot_id
        )
        self.assertEqual(
            reversed_result.snapshot.evidence_ids, normal_result.snapshot.evidence_ids
        )
        self.assertEqual(
            tuple(
                status.source_id for status in reversed_result.snapshot.source_statuses
            ),
            ("clinvar", "gnomad"),
        )
        self.assertEqual(len(reversed_result.evidence_items), 2)
        self.assertEqual(
            reversed_result.source_results[0].cache_state, CacheState.HIT_FRESH
        )
        self.assertEqual(
            reversed_result.snapshot.source_statuses[0].status,
            SourceStatusValue.CACHED,
        )
        stored = self.store.get_evidence_snapshot(reversed_result.snapshot.snapshot_id)
        self.assertEqual(stored.evidence_ids, reversed_result.snapshot.evidence_ids)

    async def test_adapter_failures_are_truthful_and_persisted(
        self,
    ) -> None:
        success = FakeAdapter(
            "clinvar", result_for("clinvar", items=(item_for("clinvar", "ok"),))
        )
        failed = FakeAdapter("gnomad", RuntimeError("network unavailable"))
        mixed = await self.orchestrator(success, failed).gather(
            FakeVariant(), SCOPE, POLICY
        )
        self.assertTrue(mixed.degraded)
        self.assertEqual(mixed.unavailable_sources, ("gnomad",))
        self.assertEqual(
            mixed.snapshot.source_statuses[1].status, SourceStatusValue.UNAVAILABLE
        )
        self.assertEqual(len(mixed.evidence_items), 1)

        all_failed = await self.orchestrator(
            FakeAdapter("clinvar", RuntimeError("down")),
            FakeAdapter("gnomad", RuntimeError("down")),
        ).gather(FakeVariant(), SCOPE, POLICY)
        self.assertTrue(all_failed.degraded)
        self.assertEqual(all_failed.snapshot.evidence_ids, ())
        self.assertEqual(all_failed.unavailable_sources, ("clinvar", "gnomad"))
        self.assertEqual(
            self.store.get_evidence_snapshot(
                all_failed.snapshot.snapshot_id
            ).evidence_ids,
            (),
        )

    async def test_deadline_retains_fast_result_and_marks_slow_source_timeout(
        self,
    ) -> None:
        release = asyncio.Event()
        slow = FakeAdapter("gnomad", result_for("gnomad"), release=release)
        result = await self.orchestrator(
            FakeAdapter(
                "clinvar", result_for("clinvar", items=(item_for("clinvar", "fast"),))
            ),
            slow,
            deadline=0.02,
        ).gather(FakeVariant(), SCOPE, POLICY)
        self.assertTrue(slow.cancelled)
        self.assertEqual(
            result.snapshot.evidence_ids, (item_for("clinvar", "fast").evidence_id,)
        )
        self.assertEqual(
            result.snapshot.source_statuses[1].status, SourceStatusValue.TIMEOUT
        )

    async def test_caller_cancellation_propagates_without_snapshot(self) -> None:
        started = asyncio.Event()
        release = asyncio.Event()
        adapter = FakeAdapter(
            "clinvar", result_for("clinvar"), started=started, release=release
        )
        task = asyncio.create_task(
            self.orchestrator(adapter).gather(FakeVariant(), SCOPE, POLICY)
        )
        await asyncio.wait_for(started.wait(), timeout=0.5)
        task.cancel()
        with self.assertRaises(asyncio.CancelledError):
            await task
        self.assertTrue(adapter.cancelled)

        with sqlite3.connect(self.database_path) as connection:
            snapshot_count = connection.execute(
                "SELECT COUNT(*) FROM evidence_snapshots"
            ).fetchone()[0]
        self.assertEqual(snapshot_count, 0)

    async def test_invalid_scope_or_variant_rejects_source_output_without_persisting_it(
        self,
    ) -> None:
        from acmg_classifier.infrastructure.storage.evidence import (
            EvidenceNotFoundError,
        )

        original = item_for("clinvar", "invalid")
        invalid_items = (
            original.model_copy(update={"variant_key": "ga4gh:VA.other"}),
            original.model_copy(
                update={
                    "context_scope": EvidenceContextScope(
                        genome_build=GenomeBuild.GRCH38,
                        disease_id="MONDO:0011450",
                    )
                }
            ),
        )
        for invalid in invalid_items:
            with self.subTest(invalid=invalid):
                result = await self.orchestrator(
                    FakeAdapter("clinvar", result_for("clinvar", items=(invalid,)))
                ).gather(FakeVariant(), SCOPE, POLICY)
                self.assertEqual(result.evidence_items, ())
                self.assertEqual(
                    result.snapshot.source_statuses[0].status,
                    SourceStatusValue.SCHEMA_CHANGED,
                )
                with self.assertRaises(EvidenceNotFoundError):
                    self.store.get_evidence(invalid.evidence_id)

    async def test_storage_failure_is_not_reported_as_a_completed_snapshot(
        self,
    ) -> None:
        class FailingStore:
            def put_evidence(self, content: object) -> str:
                raise OSError("disk full")

            def put_domain_evidence_snapshot(self, snapshot: object) -> str:
                raise AssertionError("snapshot must not be attempted")

        from acmg_classifier.application.evidence_orchestrator import (
            EvidenceOrchestrator,
        )

        orchestrator = EvidenceOrchestrator(
            adapters=(
                FakeAdapter(
                    "clinvar",
                    result_for("clinvar", items=(item_for("clinvar", "write"),)),
                ),
            ),
            evidence_store=FailingStore(),
            clock=lambda: NOW,
            total_deadline_seconds=1.0,
        )
        with self.assertRaisesRegex(OSError, "disk full"):
            await orchestrator.gather(FakeVariant(), SCOPE, POLICY)


if __name__ == "__main__":
    unittest.main()
