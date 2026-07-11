from __future__ import annotations

import unittest
from datetime import UTC, datetime, timedelta
from pathlib import Path
from tempfile import TemporaryDirectory


class SQLiteSourceCacheTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temporary_directory = TemporaryDirectory()
        self.addCleanup(self.temporary_directory.cleanup)
        self.root = Path(self.temporary_directory.name)
        self.database_path = self.root / "state.sqlite3"
        from acmg_classifier.infrastructure.storage.sqlite import SQLiteStateStore

        SQLiteStateStore(self.database_path).initialize()
        from acmg_classifier.infrastructure.storage.evidence import SQLiteEvidenceStore

        self.evidence_store = SQLiteEvidenceStore(self.database_path, self.root / "raw")
        self.now = datetime(2026, 1, 1, tzinfo=UTC)

    def query(self) -> object:
        from acmg_classifier.ports.evidence import SourceQuery

        return SourceQuery(
            source_id="clinvar",
            normalized_query_key="GRCh38:1:100:A:G",
            request_fingerprint="esearch-v1",
            source_version="2026-01",
        )

    def policy(self, mode: str = "live", max_age_seconds: int | None = None) -> object:
        from acmg_classifier.domain.evidence import EvidencePolicy, EvidencePolicyMode

        return EvidencePolicy(
            mode=EvidencePolicyMode(mode), max_age_seconds=max_age_seconds
        )

    def cache(self) -> object:
        from acmg_classifier.infrastructure.storage.source_cache import (
            SQLiteSourceCache,
        )

        return SQLiteSourceCache(self.database_path)

    def test_fresh_success_and_no_record_are_eligible_but_failure_is_not(self) -> None:
        from acmg_classifier.infrastructure.storage.source_cache import (
            CacheLookupState,
            SourceCacheStatus,
        )

        cache = self.cache()
        query = self.query()
        evidence_id = self.evidence_store.put_evidence({"source": "clinvar"})
        cache.put_success(
            query,
            retrieved_at=self.now,
            expires_at=self.now + timedelta(hours=1),
            evidence_ids=(evidence_id,),
            raw_snapshot_ref="raw_" + "b" * 64,
            response_hash="c" * 64,
            media_type="application/json",
            byte_size=2,
        )
        fresh = cache.get_eligible(query, policy=self.policy(), now=self.now)
        self.assertEqual(fresh.state, CacheLookupState.FRESH)
        self.assertTrue(fresh.eligible)
        self.assertEqual(fresh.entry.evidence_ids, (evidence_id,))

        no_record_query = self.query().with_fingerprint("no-record")
        cache.put_negative(
            no_record_query,
            retrieved_at=self.now,
            expires_at=self.now + timedelta(hours=1),
            raw_snapshot_ref="raw_" + "c" * 64,
            response_hash="d" * 64,
            media_type="application/json",
            byte_size=2,
        )
        no_record = cache.get_eligible(
            no_record_query, policy=self.policy(), now=self.now
        )
        self.assertEqual(no_record.state, CacheLookupState.FRESH)
        self.assertEqual(no_record.entry.status, SourceCacheStatus.NO_RECORD)
        self.assertEqual(no_record.entry.evidence_ids, ())

        failed_query = self.query().with_fingerprint("outage")
        cache.put_failure_metadata(
            failed_query,
            retrieved_at=self.now,
            error_code="SOURCE_UNAVAILABLE",
        )
        failure = cache.get_eligible(failed_query, policy=self.policy(), now=self.now)
        self.assertEqual(failure.state, CacheLookupState.INELIGIBLE)
        self.assertFalse(failure.eligible)

    def test_stale_and_offline_miss_are_never_eligible(self) -> None:
        from acmg_classifier.infrastructure.storage.source_cache import CacheLookupState

        cache = self.cache()
        query = self.query()
        cache.put_success(
            query,
            retrieved_at=self.now - timedelta(hours=2),
            expires_at=self.now - timedelta(hours=1),
            evidence_ids=(),
            raw_snapshot_ref=None,
            response_hash=None,
            media_type=None,
            byte_size=0,
        )

        live = cache.get_eligible(query, policy=self.policy("live"), now=self.now)
        offline = cache.get_eligible(query, policy=self.policy("offline"), now=self.now)
        miss = cache.get_eligible(
            query.with_fingerprint("missing"),
            policy=self.policy("offline"),
            now=self.now,
        )

        self.assertEqual(live.state, CacheLookupState.STALE)
        self.assertEqual(offline.state, CacheLookupState.INELIGIBLE)
        self.assertEqual(miss.state, CacheLookupState.OFFLINE_MISS)
        self.assertFalse(live.eligible)
        self.assertFalse(offline.eligible)
        self.assertFalse(miss.eligible)

    def test_offline_miss_does_not_call_transport(self) -> None:
        import asyncio

        from acmg_classifier.infrastructure.http.policy import (
            HttpPolicy,
            HttpRequest,
            SourceHttpClient,
            SourceRequestExecutor,
        )

        class NeverCalledTransport:
            calls = 0

            async def request(self, _: object) -> object:
                self.calls += 1
                raise AssertionError("offline mode must not use transport")

        async def run() -> None:
            transport = NeverCalledTransport()
            client = SourceHttpClient(
                transport=transport,
                policy=HttpPolicy(
                    source_id="clinvar",
                    allowed_hosts=frozenset({"api.example.org"}),
                    timeout_seconds=1,
                    max_response_bytes=10,
                    max_retries=0,
                ),
            )
            result = await SourceRequestExecutor(
                cache=self.cache(), client=client, clock=lambda: self.now
            ).request(
                self.query().with_fingerprint("offline-miss"),
                HttpRequest(method="GET", url="https://api.example.org/x"),
                self.policy("offline"),
            )
            self.assertIsNone(result.outcome)
            self.assertEqual(transport.calls, 0)

        asyncio.run(run())

    def test_corrupt_cache_row_is_ineligible_not_false_success(self) -> None:
        from acmg_classifier.infrastructure.storage.source_cache import CacheLookupState

        cache = self.cache()
        query = self.query()
        with cache._connect() as connection:
            connection.execute(
                """
                INSERT INTO source_cache (
                    cache_key, source_id, normalized_query_key, request_fingerprint,
                    source_version, adapter_version, status, retrieved_at, expires_at,
                    evidence_ids_json, byte_size, updated_at
                ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                """,
                (
                    query.cache_key,
                    query.source_id,
                    query.normalized_query_key,
                    query.request_fingerprint,
                    query.source_version,
                    query.adapter_version,
                    "success",
                    self.now.isoformat(),
                    (self.now + timedelta(hours=1)).isoformat(),
                    b"not-json",
                    0,
                    self.now.isoformat(),
                ),
            )
        lookup = cache.get_eligible(query, policy=self.policy(), now=self.now)
        self.assertEqual(lookup.state, CacheLookupState.INELIGIBLE)
        self.assertIsNone(lookup.entry)

    def test_cache_write_transaction_is_short_and_network_is_not_held_by_request_gate(
        self,
    ) -> None:
        import asyncio

        from acmg_classifier.infrastructure.http.policy import (
            HttpPolicy,
            HttpRequest,
            HttpResponse,
            SourceHttpClient,
            SourceRequestExecutor,
        )

        class WaitingTransport:
            def __init__(self) -> None:
                self.started = asyncio.Event()
                self.release = asyncio.Event()

            async def request(self, _: object) -> object:
                self.started.set()
                await self.release.wait()

                async def body() -> object:
                    yield b"ok"

                return HttpResponse(status_code=200, headers={}, body=body())

        async def run() -> None:
            transport = WaitingTransport()
            client = SourceHttpClient(
                transport=transport,
                policy=HttpPolicy(
                    source_id="clinvar",
                    allowed_hosts=frozenset({"api.example.org"}),
                    timeout_seconds=5,
                    max_response_bytes=10,
                    max_retries=0,
                ),
            )
            executor = SourceRequestExecutor(
                cache=self.cache(), client=client, clock=lambda: self.now
            )
            pending = asyncio.create_task(
                executor.request(
                    self.query(),
                    HttpRequest(method="GET", url="https://api.example.org/x"),
                    self.policy(),
                )
            )
            await transport.started.wait()
            self.cache().put_failure_metadata(
                self.query().with_fingerprint("parallel-write"),
                retrieved_at=self.now,
                error_code="SOURCE_UNAVAILABLE",
            )
            transport.release.set()
            await pending

        asyncio.run(run())


if __name__ == "__main__":
    unittest.main()
