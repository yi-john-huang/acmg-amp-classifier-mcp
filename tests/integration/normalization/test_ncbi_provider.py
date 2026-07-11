from __future__ import annotations

import asyncio
import unittest
from collections.abc import AsyncIterator
from pathlib import Path

from acmg_classifier.application.normalization import VariantNormalizationService
from acmg_classifier.domain.models import InterpretationContext
from acmg_classifier.domain.normalization import (
    NormalizationFailureCode,
    VariantInputParser,
)
from acmg_classifier.infrastructure.http.policy import (
    HttpPolicy,
    HttpRequest,
    HttpResponse,
    SourceHttpClient,
)
from acmg_classifier.ports.normalization import NormalizationPolicy


async def _no_sleep(_: float) -> None:
    return None


FIXTURE_ROOT = Path(__file__).parents[2] / "fixtures" / "normalization" / "ncbi"


class RecordedTransport:
    def __init__(self, responses: list[bytes], *, status_code: int = 200) -> None:
        self._responses = responses
        self._status_code = status_code
        self.requests: list[HttpRequest] = []

    async def request(self, request: HttpRequest) -> HttpResponse:
        self.requests.append(request)
        payload = self._responses.pop(0)

        async def chunks() -> AsyncIterator[bytes]:
            yield payload

        return HttpResponse(
            status_code=self._status_code,
            headers={"Content-Type": "application/json"},
            body=chunks(),
        )


class StatusTransport:
    def __init__(self, status_code: int) -> None:
        self._status_code = status_code
        self.calls = 0

    async def request(self, _: HttpRequest) -> HttpResponse:
        self.calls += 1

        async def chunks() -> AsyncIterator[bytes]:
            if False:
                yield b""

        return HttpResponse(status_code=self._status_code, headers={}, body=chunks())


class TimeoutTransport:
    async def request(self, _: HttpRequest) -> HttpResponse:
        raise TimeoutError


class NeverCalledTransport:
    def __init__(self) -> None:
        self.calls = 0

    async def request(self, _: HttpRequest) -> HttpResponse:
        self.calls += 1
        raise AssertionError("offline policy must not open an NCBI connection")


class BlockingTransport:
    def __init__(self) -> None:
        self.started = asyncio.Event()

    async def request(self, _: HttpRequest) -> HttpResponse:
        self.started.set()
        await asyncio.Event().wait()
        raise AssertionError("unreachable")


class NcbiVariationNormalizationProviderTests(unittest.TestCase):
    def provider(self, transport: object) -> object:
        from acmg_classifier.infrastructure.normalization.ncbi import (
            NCBIVariationNormalizationProvider,
        )

        return NCBIVariationNormalizationProvider(
            client=SourceHttpClient(
                transport=transport,
                policy=HttpPolicy(
                    source_id="ncbi_variation",
                    allowed_hosts=frozenset({"api.ncbi.nlm.nih.gov"}),
                    timeout_seconds=0.01,
                    max_response_bytes=100_000,
                    max_retries=0,
                ),
            ),
            async_sleep=_no_sleep,
            monotonic=lambda: 0.0,
        )

    @staticmethod
    def parsed(value: str) -> object:
        return VariantInputParser().parse(value)

    def normalize(self, provider: object, value: str) -> object:
        return provider.normalize(
            self.parsed(value), InterpretationContext(), NormalizationPolicy()
        )

    def test_maps_official_substitution_fixture_to_normalized_variant(self) -> None:
        transport = RecordedTransport(
            [
                (FIXTURE_ROOT / "substitution_contextuals.json").read_bytes(),
                (FIXTURE_ROOT / "substitution_canonical.json").read_bytes(),
            ]
        )

        result = self.normalize(self.provider(transport), "NC_000007.14:g.117559593C>T")

        self.assertEqual(result.status, "success")
        self.assertEqual(
            result.normalized.canonical_key.value,
            "cak1:GRCh38:NC_000007.14:117559592:C>T",
        )
        self.assertEqual(result.round_trip_key, result.normalized.canonical_key)
        self.assertEqual(
            result.normalized.provider_provenance[0].provider_id, "ncbi_variation"
        )
        self.assertTrue(
            result.normalized.provider_provenance[0].raw_snapshot_ref.startswith(
                "sha256:"
            )
        )
        self.assertEqual(len(transport.requests), 2)
        self.assertTrue(
            all(
                request.url.startswith("https://api.ncbi.nlm.nih.gov/")
                for request in transport.requests
            )
        )
        self.assertIn("/hgvs/", transport.requests[0].url)
        self.assertIn("/canonical_representative", transport.requests[1].url)

    def test_maps_official_deletion_fixture_to_normalized_variant(self) -> None:
        transport = RecordedTransport(
            [
                (FIXTURE_ROOT / "deletion_contextuals.json").read_bytes(),
                (FIXTURE_ROOT / "deletion_canonical.json").read_bytes(),
            ]
        )

        result = self.normalize(
            self.provider(transport), "NC_000007.14:g.117559593_117559595delCTT"
        )

        self.assertEqual(result.status, "success")
        self.assertEqual(result.normalized.reference_allele, "CTT")
        self.assertEqual(result.normalized.alternate_allele, "")

    def test_schema_drift_is_not_normalization_success(self) -> None:
        transport = RecordedTransport(
            [(FIXTURE_ROOT / "schema_drift_missing_allele.json").read_bytes()]
        )

        result = self.normalize(self.provider(transport), "NC_000007.14:g.117559593C>T")

        self.assertEqual(result.code, NormalizationFailureCode.PROVIDER_SCHEMA_DRIFT)
        self.assertFalse(result.retryable)

    def test_invalid_query_is_distinct_from_no_record(self) -> None:
        invalid = self.normalize(
            self.provider(
                RecordedTransport([(FIXTURE_ROOT / "invalid_query.json").read_bytes()])
            ),
            "NC_000007.14:g.117559593C>T",
        )
        no_record = self.normalize(
            self.provider(StatusTransport(404)), "NC_000007.14:g.117559593C>T"
        )

        self.assertEqual(
            invalid.code, NormalizationFailureCode.ALLELE_NORMALIZATION_FAILED
        )
        self.assertEqual(invalid.details["reason"], "invalid_query")
        self.assertEqual(
            no_record.code, NormalizationFailureCode.NORMALIZATION_UNAVAILABLE
        )
        self.assertEqual(no_record.details["reason"], "no_record")

    def test_explicit_reference_mismatch_is_terminal(self) -> None:
        result = self.normalize(
            self.provider(
                RecordedTransport(
                    [(FIXTURE_ROOT / "reference_mismatch.json").read_bytes()]
                )
            ),
            "NC_000007.14:g.117559593C>T",
        )

        self.assertEqual(result.code, NormalizationFailureCode.REFERENCE_MISMATCH)
        self.assertFalse(result.retryable)

    def test_timeout_rate_limit_and_outage_are_distinct(self) -> None:
        timeout = self.normalize(
            self.provider(TimeoutTransport()), "NC_000007.14:g.117559593C>T"
        )
        rate_limited = self.normalize(
            self.provider(StatusTransport(429)), "NC_000007.14:g.117559593C>T"
        )
        outage = self.normalize(
            self.provider(StatusTransport(503)), "NC_000007.14:g.117559593C>T"
        )

        self.assertEqual(timeout.code, NormalizationFailureCode.PROVIDER_TIMEOUT)
        self.assertEqual(
            rate_limited.code, NormalizationFailureCode.PROVIDER_RATE_LIMITED
        )
        self.assertEqual(outage.code, NormalizationFailureCode.PROVIDER_OUTAGE)
        self.assertTrue(timeout.retryable)
        self.assertTrue(rate_limited.retryable)
        self.assertTrue(outage.retryable)

    def test_offline_or_no_remote_policy_never_calls_client(self) -> None:
        transport = NeverCalledTransport()
        provider = self.provider(transport)
        parsed = self.parsed("NC_000007.14:g.117559593C>T")

        offline = provider.normalize(
            parsed,
            InterpretationContext(),
            NormalizationPolicy(mode="offline"),
        )
        no_remote = provider.normalize(
            parsed,
            InterpretationContext(),
            NormalizationPolicy(allow_remote=False),
        )

        self.assertEqual(
            offline.code, NormalizationFailureCode.NORMALIZATION_UNAVAILABLE
        )
        self.assertEqual(offline.details["reason"], "offline_policy")
        self.assertEqual(
            no_remote.code, NormalizationFailureCode.NORMALIZATION_UNAVAILABLE
        )
        self.assertEqual(no_remote.details["reason"], "remote_disabled")
        self.assertEqual(transport.calls, 0)

    def test_application_service_skips_remote_provider_offline(self) -> None:
        transport = NeverCalledTransport()
        service = VariantNormalizationService(providers=(self.provider(transport),))

        result = service.normalize(
            "NC_000007.14:g.117559593C>T",
            policy=NormalizationPolicy(mode="offline"),
        )

        self.assertEqual(
            result.code, NormalizationFailureCode.NORMALIZATION_UNAVAILABLE
        )
        self.assertEqual(transport.calls, 0)


class AsyncNcbiVariationNormalizationProviderTests(unittest.IsolatedAsyncioTestCase):
    async def test_live_provider_normalizes_through_async_service_path(self) -> None:
        transport = RecordedTransport(
            [
                (FIXTURE_ROOT / "substitution_contextuals.json").read_bytes(),
                (FIXTURE_ROOT / "substitution_canonical.json").read_bytes(),
            ]
        )
        provider = NcbiVariationNormalizationProviderTests().provider(transport)
        service = VariantNormalizationService(providers=(provider,))

        result = await service.normalize_async("NC_000007.14:g.117559593C>T")

        self.assertEqual(result.status, "success")
        self.assertEqual(
            result.normalized.canonical_key.value,
            "cak1:GRCh38:NC_000007.14:117559592:C>T",
        )
        self.assertEqual(len(transport.requests), 2)

    async def test_async_normalization_propagates_cancellation(self) -> None:
        transport = BlockingTransport()
        provider = NcbiVariationNormalizationProviderTests().provider(transport)
        service = VariantNormalizationService(providers=(provider,))

        task = asyncio.create_task(
            service.normalize_async("NC_000007.14:g.117559593C>T")
        )
        await transport.started.wait()
        task.cancel()

        with self.assertRaises(asyncio.CancelledError):
            await task


if __name__ == "__main__":
    unittest.main()
