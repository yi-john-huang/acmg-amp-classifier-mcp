from __future__ import annotations

import asyncio
import unittest
from collections.abc import AsyncIterator
from datetime import UTC, datetime
from pathlib import Path
from tempfile import TemporaryDirectory

from acmg_classifier.domain.enums import GenomeBuild
from acmg_classifier.domain.evidence import (
    EvidenceContextScope,
    EvidencePolicy,
    EvidencePolicyMode,
    SourceStatusValue,
)
from acmg_classifier.infrastructure.http.policy import (
    HttpPolicy,
    HttpRequest,
    HttpResponse,
    SourceHttpClient,
)
from acmg_classifier.infrastructure.storage.evidence import SQLiteEvidenceStore
from acmg_classifier.infrastructure.storage.source_cache import SQLiteSourceCache
from acmg_classifier.infrastructure.storage.sqlite import SQLiteStateStore

FIXTURE_ROOT = Path(__file__).parents[2] / "fixtures" / "evidence" / "clinvar"
FIXED_NOW = datetime(2026, 7, 11, 12, 0, tzinfo=UTC)
ESEARCH_ONE_RECORD = (
    b"<?xml version='1.0'?><eSearchResult><Count>1</Count>"
    b"<IdList><Id>14206</Id></IdList></eSearchResult>"
)


class Variant:
    variant_key = "GRCh38:7:117559593:CTT:C"
    genome_build = "GRCh38"
    genomic_hgvs = "NC_000007.14:g.117559593_117559595del"




class RecordedTransport:
    def __init__(self, responses: list[bytes]) -> None:
        self.responses = responses
        self.requests: list[HttpRequest] = []

    async def request(self, request: HttpRequest) -> HttpResponse:
        self.requests.append(request)
        payload = self.responses.pop(0)

        async def chunks() -> AsyncIterator[bytes]:
            yield payload

        return HttpResponse(
            status_code=200, headers={"Content-Type": "application/xml"}, body=chunks()
        )


class StatusTransport:
    def __init__(self, status_code: int) -> None:
        self.status_code = status_code
        self.calls = 0

    async def request(self, _: HttpRequest) -> HttpResponse:
        self.calls += 1

        async def chunks() -> AsyncIterator[bytes]:
            if False:
                yield b""

        return HttpResponse(status_code=self.status_code, headers={}, body=chunks())


class TimeoutTransport:
    async def request(self, _: HttpRequest) -> HttpResponse:
        raise TimeoutError


class NeverCalledTransport:
    async def request(self, _: HttpRequest) -> HttpResponse:
        raise AssertionError("fresh ClinVar cache must avoid HTTP")


class ClinVarAdapterTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temporary_directory = TemporaryDirectory()
        self.addCleanup(self.temporary_directory.cleanup)
        root = Path(self.temporary_directory.name)
        self.database_path = root / "state.sqlite3"
        SQLiteStateStore(self.database_path).initialize()
        self.store = SQLiteEvidenceStore(self.database_path, root / "raw")
        self.cache = SQLiteSourceCache(self.database_path)
        self.policy = EvidencePolicy(mode=EvidencePolicyMode.LIVE)

    def adapter(self, transport: object) -> object:
        from acmg_classifier.infrastructure.evidence.clinvar import ClinVarAdapter

        return ClinVarAdapter(
            client=SourceHttpClient(
                transport=transport,
                policy=HttpPolicy(
                    source_id="clinvar",
                    allowed_hosts=frozenset({"eutils.ncbi.nlm.nih.gov"}),
                    timeout_seconds=1,
                    max_response_bytes=100_000,
                    max_retries=0,
                ),
            ),
            cache=self.cache,
            evidence_store=self.store,
            clock=lambda: FIXED_NOW,
        )

    def query(
        self,
        adapter: object,
        variant: object = Variant(),
        *,
        context_scope: EvidenceContextScope | None = None,
    ) -> object:
        return asyncio.run(
            adapter.query(
                variant,
                context_scope=context_scope
                or EvidenceContextScope(genome_build=GenomeBuild.GRCH38),
                policy=self.policy,
            )
        )

    def test_no_record_is_fresh_negative_result_not_evidence(self) -> None:
        transport = RecordedTransport(
            [(FIXTURE_ROOT / "esearch_no_record.xml").read_bytes()]
        )

        result = self.query(self.adapter(transport))

        self.assertEqual(result.source_status.status, SourceStatusValue.FRESH)
        self.assertEqual(result.source_status.detail, "no_record")
        self.assertEqual(result.evidence_items, ())
        self.assertEqual(len(transport.requests), 1)
        self.assertIn("esearch.fcgi", transport.requests[0].url)

    def test_maps_scv_assertion_with_vcv_version_provenance_and_raw_reference(
        self,
    ) -> None:
        transport = RecordedTransport(
            [
                ESEARCH_ONE_RECORD,
                (FIXTURE_ROOT / "efetch_single_assertion.xml").read_bytes(),
            ]
        )

        result = self.query(self.adapter(transport))

        self.assertEqual(len(transport.requests), 2)
        self.assertIn("efetch.fcgi", transport.requests[1].url)
        self.assertEqual(result.source_status.status, SourceStatusValue.FRESH)
        self.assertEqual(result.source_status.source_version, "VCV000014206.4")
        self.assertEqual(len(result.evidence_items), 1)
        item = result.evidence_items[0]
        observation = item.observation
        self.assertEqual(observation.accession, "SCV000000001.3")
        self.assertEqual(observation.submitter, "Example ClinVar Submitter")
        self.assertEqual(observation.condition_id, "MONDO:0009061")
        self.assertEqual(observation.condition_label, "Cystic fibrosis")
        self.assertEqual(
            observation.review_status, "criteria provided, single submitter"
        )
        self.assertEqual(observation.last_evaluated, datetime(2025, 12, 1, tzinfo=UTC))
        self.assertEqual(observation.citation_ids, ("PMID:23974870",))
        self.assertEqual(item.provenance.normalized_query_key, Variant.variant_key)
        self.assertEqual(item.provenance.source_version, "VCV000014206.4")
        self.assertTrue(item.raw_snapshot_ref.startswith("raw_"))
        self.assertIn(item.raw_snapshot_ref, result.raw_snapshot_refs)

    def test_conflicting_assertions_stay_separate_and_sorted(self) -> None:
        transport = RecordedTransport(
            [
                ESEARCH_ONE_RECORD,
                (FIXTURE_ROOT / "efetch_conflicting_assertions.xml").read_bytes(),
            ]
        )

        result = self.query(self.adapter(transport))

        self.assertEqual(
            [item.observation.accession for item in result.evidence_items],
            ["SCV000000002.1", "SCV000000003.2"],
        )
        self.assertEqual(
            [item.observation.clinical_significance for item in result.evidence_items],
            ["Benign", "Pathogenic"],
        )

    def test_requested_condition_mismatch_excludes_assertion_without_disease_inference(
        self,
    ) -> None:
        transport = RecordedTransport(
            [
                ESEARCH_ONE_RECORD,
                (FIXTURE_ROOT / "efetch_condition_mismatch.xml").read_bytes(),
            ]
        )

        result = self.query(
            self.adapter(transport),
            context_scope=EvidenceContextScope(
                genome_build=GenomeBuild.GRCH38, disease_id="MONDO:0009999"
            ),
        )

        self.assertEqual(result.evidence_items, ())
        self.assertEqual(result.source_status.status, SourceStatusValue.FRESH)
        self.assertEqual(result.source_status.detail, "condition_mismatch_excluded")

    def test_requested_disease_filters_evidence_and_separates_cache_identity(
        self,
    ) -> None:
        transport = RecordedTransport(
            [
                ESEARCH_ONE_RECORD,
                (FIXTURE_ROOT / "efetch_single_assertion.xml").read_bytes(),
                ESEARCH_ONE_RECORD,
                (FIXTURE_ROOT / "efetch_single_assertion.xml").read_bytes(),
            ]
        )
        adapter = self.adapter(transport)
        matching_scope = EvidenceContextScope(
            genome_build=GenomeBuild.GRCH38, disease_id="MONDO:0009061"
        )
        mismatching_scope = EvidenceContextScope(
            genome_build=GenomeBuild.GRCH38, disease_id="MONDO:0011450"
        )

        matching = self.query(adapter, context_scope=matching_scope)
        mismatching = self.query(adapter, context_scope=mismatching_scope)

        self.assertEqual(len(matching.evidence_items), 1)
        self.assertEqual(matching.evidence_items[0].context_scope, matching_scope)
        self.assertEqual(mismatching.evidence_items, ())
        self.assertEqual(
            mismatching.source_status.detail, "condition_mismatch_excluded"
        )
        self.assertEqual(len(transport.requests), 4)

    def test_schema_drift_is_not_misreported_as_no_record(self) -> None:
        transport = RecordedTransport(
            [
                ESEARCH_ONE_RECORD,
                (
                    FIXTURE_ROOT / "efetch_schema_drift_missing_accession.xml"
                ).read_bytes(),
            ]
        )

        result = self.query(self.adapter(transport))

        self.assertEqual(result.evidence_items, ())
        self.assertEqual(result.source_status.status, SourceStatusValue.SCHEMA_CHANGED)
        self.assertTrue(result.raw_snapshot_refs[0].startswith("raw_"))

    def test_rate_limit_status_is_preserved(self) -> None:
        transport = StatusTransport(429)

        result = self.query(self.adapter(transport))

        self.assertEqual(result.source_status.status, SourceStatusValue.RATE_LIMITED)
        self.assertEqual(result.evidence_items, ())
        self.assertEqual(transport.calls, 1)

    def test_timeout_and_outage_have_distinct_source_statuses(self) -> None:
        timeout = self.query(self.adapter(TimeoutTransport()))
        outage = self.query(self.adapter(StatusTransport(503)))

        self.assertEqual(timeout.source_status.status, SourceStatusValue.TIMEOUT)
        self.assertEqual(outage.source_status.status, SourceStatusValue.UNAVAILABLE)
        self.assertEqual(timeout.evidence_items, ())
        self.assertEqual(outage.evidence_items, ())

    def test_fresh_cache_replays_original_provenance_without_http(self) -> None:
        first_transport = RecordedTransport(
            [
                ESEARCH_ONE_RECORD,
                (FIXTURE_ROOT / "efetch_single_assertion.xml").read_bytes(),
            ]
        )
        initial = self.query(self.adapter(first_transport))

        replay = self.query(self.adapter(NeverCalledTransport()))

        self.assertEqual(replay.source_status.status, SourceStatusValue.CACHED)
        self.assertEqual(replay.evidence_items, initial.evidence_items)
        self.assertEqual(replay.raw_snapshot_refs, initial.raw_snapshot_refs)
        self.assertEqual(replay.evidence_items[0].provenance.retrieved_at, FIXED_NOW)


if __name__ == "__main__":
    unittest.main()
