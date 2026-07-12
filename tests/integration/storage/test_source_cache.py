from __future__ import annotations

import unittest
from contextlib import closing
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

        return SQLiteSourceCache(self.database_path, self.evidence_store)

    def test_fresh_success_and_no_record_are_eligible_but_failure_is_not(self) -> None:
        from acmg_classifier.infrastructure.storage.source_cache import (
            CacheLookupState,
            SourceCacheStatus,
        )

        cache = self.cache()
        query = self.query()
        evidence_id = self.evidence_store.put_evidence({"source": "clinvar"})
        fresh_raw = self.evidence_store.put_raw_snapshot(b"{}", "application/json")
        cache.put_success(
            query,
            retrieved_at=self.now,
            expires_at=self.now + timedelta(hours=1),
            evidence_ids=(evidence_id,),
            raw_snapshot_ref=fresh_raw.snapshot_hash,
            response_hash=fresh_raw.snapshot_hash.removeprefix("raw_"),
            media_type=fresh_raw.media_type,
            byte_size=fresh_raw.byte_size,
        )
        fresh = cache.get_eligible(query, policy=self.policy(), now=self.now)
        self.assertEqual(fresh.state, CacheLookupState.FRESH)
        self.assertTrue(fresh.eligible)
        self.assertEqual(fresh.entry.evidence_ids, (evidence_id,))

        no_record_query = self.query().with_fingerprint("no-record")
        no_record_raw = self.evidence_store.put_raw_snapshot(
            b"[]", "application/json"
        )
        cache.put_negative(
            no_record_query,
            retrieved_at=self.now,
            expires_at=self.now + timedelta(hours=1),
            raw_snapshot_ref=no_record_raw.snapshot_hash,
            response_hash=no_record_raw.snapshot_hash.removeprefix("raw_"),
            media_type=no_record_raw.media_type,
            byte_size=no_record_raw.byte_size,
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

    def test_deleted_evidence_reference_makes_valid_raw_cache_ineligible(self) -> None:
        from acmg_classifier.infrastructure.storage.source_cache import CacheLookupState

        cache = self.cache()
        query = self.query()
        evidence_id = self.evidence_store.put_evidence({"source": "clinvar"})
        raw = self.evidence_store.put_raw_snapshot(b"{}", "application/json")
        cache.put_success(
            query,
            retrieved_at=self.now,
            expires_at=self.now + timedelta(hours=1),
            evidence_ids=(evidence_id,),
            raw_snapshot_ref=raw.snapshot_hash,
            response_hash=raw.snapshot_hash.removeprefix("raw_"),
            media_type=raw.media_type,
            byte_size=raw.byte_size,
        )

        with closing(self.evidence_store._connect()) as connection, connection:
            connection.execute("DROP TRIGGER evidence_items_reject_delete")
            connection.execute(
                "DELETE FROM evidence_items WHERE evidence_id = ?", (evidence_id,)
            )
            connection.commit()

        lookup = cache.get_eligible(query, policy=self.policy(), now=self.now)

        self.assertEqual(lookup.state, CacheLookupState.INELIGIBLE)
        self.assertFalse(lookup.eligible)

    def test_missing_or_mismatched_raw_cache_provenance_is_ineligible(self) -> None:
        from acmg_classifier.infrastructure.storage.source_cache import CacheLookupState

        cache = self.cache()

        def put_cache_entry(
            fingerprint: str,
            *,
            response_hash: str | None = None,
            media_type: str | None = None,
            byte_size: int | None = None,
        ) -> tuple[object, object]:
            raw = self.evidence_store.put_raw_snapshot(
                fingerprint.encode(), "application/json"
            )
            query = self.query().with_fingerprint(fingerprint)
            cache.put_success(
                query,
                retrieved_at=self.now,
                expires_at=self.now + timedelta(hours=1),
                evidence_ids=(),
                raw_snapshot_ref=raw.snapshot_hash,
                response_hash=(
                    raw.snapshot_hash.removeprefix("raw_")
                    if response_hash is None
                    else response_hash
                ),
                media_type=raw.media_type if media_type is None else media_type,
                byte_size=raw.byte_size if byte_size is None else byte_size,
            )
            return query, raw

        def assert_ineligible(query: object) -> None:
            lookup = cache.get_eligible(
                query, policy=self.policy("offline"), now=self.now
            )
            self.assertEqual(lookup.state, CacheLookupState.INELIGIBLE)
            self.assertFalse(lookup.eligible)

        deleted_query, deleted_raw = put_cache_entry("deleted-metadata")
        with closing(self.evidence_store._connect()) as connection, connection:
            connection.execute(
                "DELETE FROM raw_snapshots WHERE snapshot_hash = ?",
                (deleted_raw.snapshot_hash,),
            )
            connection.commit()
        assert_ineligible(deleted_query)

        missing_query, missing_raw = put_cache_entry("missing-payload")
        (self.root / "raw" / missing_raw.relative_path).unlink()
        assert_ineligible(missing_query)

        corrupt_query, corrupt_raw = put_cache_entry("corrupt-payload")
        (self.root / "raw" / corrupt_raw.relative_path).write_bytes(b"corrupt")
        assert_ineligible(corrupt_query)

        for fingerprint, metadata in (
            ("mismatched-hash", {"response_hash": "0" * 64}),
            ("mismatched-media-type", {"media_type": "text/plain"}),
            ("mismatched-byte-size", {"byte_size": 0}),
        ):
            with self.subTest(fingerprint=fingerprint):
                mismatched_query, _ = put_cache_entry(fingerprint, **metadata)
                assert_ineligible(mismatched_query)

    def test_stale_and_offline_miss_are_never_eligible(self) -> None:
        from acmg_classifier.infrastructure.storage.source_cache import CacheLookupState

        cache = self.cache()
        query = self.query()
        raw = self.evidence_store.put_raw_snapshot(b"stale", "application/json")
        cache.put_success(
            query,
            retrieved_at=self.now - timedelta(hours=2),
            expires_at=self.now - timedelta(hours=1),
            evidence_ids=(),
            raw_snapshot_ref=raw.snapshot_hash,
            response_hash=raw.snapshot_hash.removeprefix("raw_"),
            media_type=raw.media_type,
            byte_size=raw.byte_size,
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
        with closing(cache._connect()) as connection, connection:
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
