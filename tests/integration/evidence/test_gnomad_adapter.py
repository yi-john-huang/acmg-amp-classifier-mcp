from __future__ import annotations

import json
import unittest
from dataclasses import dataclass
from datetime import UTC, datetime
from pathlib import Path
from tempfile import TemporaryDirectory
from acmg_classifier.domain.enums import GenomeBuild
from acmg_classifier.domain.evidence import EvidenceContextScope


FIXTURES = Path(__file__).parents[2] / "fixtures" / "evidence" / "gnomad"
NOW = datetime(2026, 7, 11, 12, 0, tzinfo=UTC)


async def chunks(content: bytes):
    yield content


@dataclass(frozen=True)
class Variant:
    variant_key: str
    genome_build: str
    genomic_hgvs: str | None = None

def scope_for(variant: Variant) -> EvidenceContextScope:
    return EvidenceContextScope(genome_build=GenomeBuild(variant.genome_build))


class RecordedTransport:
    def __init__(self, fixture_name: str) -> None:
        self.fixture_name = fixture_name
        self.calls: list[object] = []

    async def request(self, request: object) -> object:
        from acmg_classifier.infrastructure.http.policy import HttpResponse

        self.calls.append(request)
        return HttpResponse(
            status_code=200,
            headers={"Content-Type": "application/json"},
            body=chunks((FIXTURES / self.fixture_name).read_bytes()),
        )


class NeverCalledTransport:
    calls = 0

    async def request(self, _: object) -> object:
        self.calls += 1
        raise AssertionError("transport must not be called")


class GnomADAdapterTests(unittest.IsolatedAsyncioTestCase):
    def setUp(self) -> None:
        self.temporary_directory = TemporaryDirectory()
        self.addCleanup(self.temporary_directory.cleanup)
        root = Path(self.temporary_directory.name)
        self.database_path = root / "state.sqlite3"
        self.raw_root = root / "raw"
        from acmg_classifier.infrastructure.storage.sqlite import SQLiteStateStore

        SQLiteStateStore(self.database_path).initialize()

    def make_adapter(self, transport: object, **settings_overrides: object) -> object:
        from acmg_classifier.infrastructure.evidence.gnomad import (
            GnomADAdapter,
            GnomADSettings,
        )
        from acmg_classifier.infrastructure.http.policy import (
            HttpPolicy,
            SourceHttpClient,
        )
        from acmg_classifier.infrastructure.storage.evidence import SQLiteEvidenceStore
        from acmg_classifier.infrastructure.storage.source_cache import (
            SQLiteSourceCache,
        )

        settings = GnomADSettings(**settings_overrides)
        client = SourceHttpClient(
            transport=transport,
            policy=HttpPolicy(
                source_id="gnomad",
                allowed_hosts=frozenset({"gnomad.broadinstitute.org"}),
                timeout_seconds=1,
                max_response_bytes=100_000,
                max_retries=0,
            ),
        )
        evidence_store = SQLiteEvidenceStore(self.database_path, self.raw_root)
        return GnomADAdapter(
            client=client,
            cache=SQLiteSourceCache(self.database_path, evidence_store),
            evidence_store=evidence_store,
            clock=lambda: NOW,
            settings=settings,
        )

    def live_policy(self) -> object:
        from acmg_classifier.domain.evidence import EvidencePolicy, EvidencePolicyMode

        return EvidencePolicy(mode=EvidencePolicyMode.LIVE)

    async def test_zero_with_adequate_denominator_is_population_evidence(self) -> None:
        transport = RecordedTransport("variant_absent_with_coverage.json")
        result = await self.make_adapter(transport).query(Variant("GRCh38:1:55516888:G:GA", "GRCh38"), context_scope=scope_for(Variant("GRCh38:1:55516888:G:GA", "GRCh38")), policy=self.live_policy())

        from acmg_classifier.domain.evidence import SourceStatusValue
        from acmg_classifier.ports.evidence import CacheState

        self.assertEqual(result.source_status.status, SourceStatusValue.FRESH)
        self.assertEqual(result.source_status.detail, "record_found")
        self.assertEqual(result.cache_state, CacheState.LIVE)
        self.assertEqual(len(result.evidence_items), 1)
        observation = result.evidence_items[0].observation
        self.assertEqual(observation.allele_count, 0)
        self.assertEqual(observation.allele_number, 1000)
        self.assertEqual(observation.allele_frequency, 0.0)
        self.assertEqual(observation.coverage, 31.2)
        self.assertEqual(observation.filter_status, "PASS")
        self.assertEqual(
            result.evidence_items[0].provenance.normalized_query_key,
            "GRCh38:1:55516888:G:GA",
        )
        self.assertEqual(len(result.raw_snapshot_refs), 1)
        request = transport.calls[0]
        self.assertEqual(request.method, "POST")
        self.assertTrue(request.read_only)
        body = json.loads(request.body)
        self.assertEqual(
            body["variables"], {"dataset": "gnomad_r4", "variantId": "1-55516888-G-GA"}
        )
        self.assertEqual(
            body["query"].strip(),
            (FIXTURES / "population_evidence_query.graphql").read_text().strip(),
        )

    async def test_no_variant_without_denominator_is_no_record_not_zero_evidence(
        self,
    ) -> None:
        result = await self.make_adapter(
            RecordedTransport("variant_not_found.json")
        ).query(Variant("GRCh38:1:55516888:G:GA", "GRCh38"), context_scope=scope_for(Variant("GRCh38:1:55516888:G:GA", "GRCh38")), policy=self.live_policy())

        from acmg_classifier.domain.evidence import SourceStatusValue

        self.assertEqual(result.source_status.status, SourceStatusValue.FRESH)
        self.assertEqual(result.source_status.detail, "no_record")
        self.assertEqual(result.evidence_items, ())
        self.assertEqual(len(result.raw_snapshot_refs), 1)

    async def test_overall_and_ancestry_observations_preserve_genome_exome_split(
        self,
    ) -> None:
        result = await self.make_adapter(
            RecordedTransport("overall_and_ancestry_counts.json")
        ).query(Variant("GRCh38:1:55516888:G:GA", "GRCh38"), context_scope=scope_for(Variant("GRCh38:1:55516888:G:GA", "GRCh38")), policy=self.live_policy())

        observations = [item.observation for item in result.evidence_items]
        self.assertEqual(
            [
                (
                    item.source_release,
                    item.ancestry,
                    item.allele_count,
                    item.allele_number,
                )
                for item in observations
            ],
            [
                ("gnomAD v4.1.1 (gnomad_r4; genome)", None, 8, 1000),
                ("gnomAD v4.1.1 (gnomad_r4; genome)", "afr", 2, 200),
                ("gnomAD v4.1.1 (gnomad_r4; genome)", "nfe", 6, 800),
                ("gnomAD v4.1.1 (gnomad_r4; exome)", None, 4, 500),
                ("gnomAD v4.1.1 (gnomad_r4; exome)", "afr", 1, 100),
                ("gnomAD v4.1.1 (gnomad_r4; joint)", None, 12, 1500),
                ("gnomAD v4.1.1 (gnomad_r4; joint)", "nfe", 7, 900),
            ],
        )
        self.assertEqual(observations[0].homozygote_count, 1)
        self.assertEqual(observations[0].hemizygote_count, 2)
        self.assertEqual(observations[1].allele_frequency, 0.01)
        self.assertEqual(observations[4].hemizygote_count, None)

    async def test_filtered_and_hemizygous_values_are_preserved(self) -> None:
        filtered = await self.make_adapter(
            RecordedTransport("filtered_variant.json")
        ).query(Variant("GRCh38:1:55516888:G:GA", "GRCh38"), context_scope=scope_for(Variant("GRCh38:1:55516888:G:GA", "GRCh38")), policy=self.live_policy())
        hemizygous = await self.make_adapter(
            RecordedTransport("hemizygous_counts.json")
        ).query(Variant("GRCh38:X:154931044:C:T", "GRCh38"), context_scope=scope_for(Variant("GRCh38:X:154931044:C:T", "GRCh38")), policy=self.live_policy())

        from acmg_classifier.domain.evidence import QualityFlag

        self.assertEqual(filtered.evidence_items[0].observation.filter_status, "AC0")
        self.assertIn(QualityFlag.FILTERED, filtered.evidence_items[0].quality_flags)
        self.assertEqual(hemizygous.evidence_items[0].observation.homozygote_count, 3)
        self.assertEqual(hemizygous.evidence_items[0].observation.hemizygote_count, 5)
        self.assertIsNone(hemizygous.evidence_items[1].observation.hemizygote_count)

    async def test_schema_errors_and_missing_required_counts_are_not_no_record(
        self,
    ) -> None:
        graphql_error = await self.make_adapter(
            RecordedTransport("graphql_errors_schema.json")
        ).query(Variant("GRCh38:1:55516888:G:GA", "GRCh38"), context_scope=scope_for(Variant("GRCh38:1:55516888:G:GA", "GRCh38")), policy=self.live_policy())
        drift = await self.make_adapter(
            RecordedTransport("schema_drift_missing_an.json")
        ).query(Variant("GRCh38:1:55516888:G:GA", "GRCh38"), context_scope=scope_for(Variant("GRCh38:1:55516888:G:GA", "GRCh38")), policy=self.live_policy())

        from acmg_classifier.domain.evidence import SourceStatusValue

        self.assertEqual(
            graphql_error.source_status.status, SourceStatusValue.SCHEMA_CHANGED
        )
        self.assertEqual(drift.source_status.status, SourceStatusValue.SCHEMA_CHANGED)
        self.assertEqual(graphql_error.evidence_items, ())
        self.assertEqual(drift.evidence_items, ())

    async def test_build_release_mismatch_fails_before_http(self) -> None:
        transport = NeverCalledTransport()
        result = await self.make_adapter(transport).query(Variant("GRCh37:1:55516888:G:GA", "GRCh37"), context_scope=scope_for(Variant("GRCh37:1:55516888:G:GA", "GRCh37")), policy=self.live_policy())

        from acmg_classifier.domain.evidence import SourceStatusValue
        from acmg_classifier.ports.evidence import CacheState

        self.assertEqual(result.source_status.status, SourceStatusValue.UNAVAILABLE)
        self.assertEqual(result.source_status.detail, "build_release_mismatch")
        self.assertEqual(result.cache_state, CacheState.INELIGIBLE)
        self.assertEqual(transport.calls, 0)

    async def test_fresh_cache_replays_original_provenance_and_skips_http(self) -> None:
        transport = RecordedTransport("variant_absent_with_coverage.json")
        adapter = self.make_adapter(transport)
        variant = Variant("GRCh38:1:55516888:G:GA", "GRCh38")
        live = await adapter.query(variant, context_scope=scope_for(variant), policy=self.live_policy())
        cached = await adapter.query(variant, context_scope=scope_for(variant), policy=self.live_policy())

        from acmg_classifier.domain.evidence import SourceStatusValue
        from acmg_classifier.ports.evidence import CacheState

        self.assertEqual(len(transport.calls), 1)
        self.assertEqual(cached.cache_state, CacheState.HIT_FRESH)
        self.assertEqual(cached.source_status.status, SourceStatusValue.CACHED)
        self.assertEqual(cached.raw_snapshot_refs, live.raw_snapshot_refs)
        self.assertEqual(cached.evidence_items, live.evidence_items)
        self.assertEqual(
            cached.evidence_items[0].provenance.retrieved_at,
            live.evidence_items[0].provenance.retrieved_at,
        )


if __name__ == "__main__":
    unittest.main()
