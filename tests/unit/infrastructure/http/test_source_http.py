from __future__ import annotations

import unittest
from collections.abc import AsyncIterator
from datetime import UTC, datetime


async def chunks(*parts: bytes) -> AsyncIterator[bytes]:
    for part in parts:
        yield part


class ScriptedTransport:
    def __init__(self, outcomes: list[object]) -> None:
        self.outcomes = outcomes
        self.calls: list[object] = []

    async def request(self, request: object) -> object:
        self.calls.append(request)
        outcome = self.outcomes.pop(0)
        if isinstance(outcome, BaseException):
            raise outcome
        return outcome


class SourceHttpClientTests(unittest.IsolatedAsyncioTestCase):
    def make_policy(self, **overrides: object) -> object:
        from acmg_classifier.infrastructure.http.policy import HttpPolicy

        defaults: dict[str, object] = {
            "source_id": "clinvar",
            "allowed_hosts": frozenset({"api.example.org"}),
            "timeout_seconds": 1,
            "max_response_bytes": 8,
            "max_retries": 1,
            "base_backoff_seconds": 0.1,
            "max_backoff_seconds": 1,
            "circuit_failure_threshold": 2,
            "circuit_cooldown_seconds": 30,
        }
        defaults.update(overrides)
        return HttpPolicy(**defaults)

    def response(
        self,
        status_code: int,
        *body: bytes,
        headers: dict[str, str] | None = None,
    ) -> object:
        from acmg_classifier.infrastructure.http.policy import HttpResponse

        return HttpResponse(
            status_code=status_code,
            headers=headers or {},
            body=chunks(*body),
        )

    async def test_retry_after_is_honored_for_safe_transient_request(self) -> None:
        from acmg_classifier.infrastructure.http.policy import (
            HttpRequest,
            HttpStatusKind,
            SourceHttpClient,
        )

        transport = ScriptedTransport(
            [
                self.response(429, headers={"Retry-After": "30"}),
                self.response(200, b"ok"),
            ]
        )
        delays: list[float] = []

        async def sleep(delay: float) -> None:
            delays.append(delay)

        client = SourceHttpClient(
            transport=transport,
            policy=self.make_policy(),
            sleep=sleep,
        )
        result = await client.request(
            HttpRequest(method="GET", url="https://api.example.org/x")
        )

        self.assertEqual(result.kind, HttpStatusKind.SUCCESS)
        self.assertEqual(result.body, b"ok")
        self.assertEqual(result.attempts, 2)
        self.assertEqual(delays, [30])
        self.assertEqual(len(transport.calls), 2)

    async def test_transient_retries_are_bounded_and_open_observable_circuit(
        self,
    ) -> None:
        from acmg_classifier.infrastructure.http.policy import (
            CircuitState,
            HttpRequest,
            HttpStatusKind,
            SourceHttpClient,
        )

        transport = ScriptedTransport(
            [
                self.response(503),
                self.response(503),
                self.response(503),
                self.response(503),
            ]
        )

        async def sleep(_: float) -> None:
            return None

        client = SourceHttpClient(
            transport=transport,
            policy=self.make_policy(max_retries=1, circuit_failure_threshold=1),
            sleep=sleep,
        )
        first = await client.request(
            HttpRequest(method="GET", url="https://api.example.org/x")
        )
        second = await client.request(
            HttpRequest(method="GET", url="https://api.example.org/x")
        )

        self.assertEqual(first.kind, HttpStatusKind.UNAVAILABLE)
        self.assertEqual(first.attempts, 2)
        self.assertEqual(second.kind, HttpStatusKind.UNAVAILABLE)
        self.assertTrue(second.circuit_open)
        self.assertEqual(client.circuit_state, CircuitState.OPEN)
        self.assertEqual(len(transport.calls), 2)

    async def test_oversized_stream_is_closed_and_not_returned_as_partial_data(
        self,
    ) -> None:
        from acmg_classifier.infrastructure.http.policy import (
            HttpRequest,
            HttpResponse,
            HttpStatusKind,
            SourceHttpClient,
        )

        closed = False

        async def close() -> None:
            nonlocal closed
            closed = True

        response = HttpResponse(
            status_code=200,
            headers={},
            body=chunks(b"four", b"more"),
            close=close,
        )
        client = SourceHttpClient(
            transport=ScriptedTransport([response]),
            policy=self.make_policy(max_response_bytes=7),
        )
        result = await client.request(
            HttpRequest(method="GET", url="https://api.example.org/x")
        )

        self.assertEqual(result.kind, HttpStatusKind.RESPONSE_TOO_LARGE)
        self.assertEqual(result.body, b"")
        self.assertTrue(closed)

    async def test_non_https_and_unapproved_redirect_are_rejected_before_following(
        self,
    ) -> None:
        from acmg_classifier.infrastructure.http.policy import (
            HttpRequest,
            HttpStatusKind,
            SourceHttpClient,
        )

        transport = ScriptedTransport(
            [
                self.response(302, headers={"Location": "https://evil.example/x"}),
                self.response(302, headers={"Location": "http://api.example.org/x"}),
            ]
        )
        client = SourceHttpClient(transport=transport, policy=self.make_policy())

        insecure = await client.request(
            HttpRequest(method="GET", url="http://api.example.org/x")
        )
        redirected = await client.request(
            HttpRequest(method="GET", url="https://api.example.org/x")
        )
        downgraded = await client.request(
            HttpRequest(method="GET", url="https://api.example.org/x")
        )

        self.assertEqual(insecure.kind, HttpStatusKind.INVALID_QUERY)
        self.assertEqual(redirected.kind, HttpStatusKind.INVALID_QUERY)
        self.assertEqual(downgraded.kind, HttpStatusKind.INVALID_QUERY)
        self.assertEqual(len(transport.calls), 2)

    async def test_unsafe_method_does_not_retry_without_explicit_read_only_marker(
        self,
    ) -> None:
        from acmg_classifier.infrastructure.http.policy import (
            HttpRequest,
            HttpStatusKind,
            SourceHttpClient,
        )

        transport = ScriptedTransport([self.response(503)])
        client = SourceHttpClient(transport=transport, policy=self.make_policy())
        result = await client.request(
            HttpRequest(
                method="POST", url="https://api.example.org/graphql", body=b"{}"
            )
        )

        self.assertEqual(result.kind, HttpStatusKind.UNAVAILABLE)
        self.assertEqual(result.attempts, 1)
        self.assertEqual(len(transport.calls), 1)

    async def test_no_record_status_remains_distinct_from_source_outage(self) -> None:
        from acmg_classifier.domain.evidence import SourceStatusValue
        from acmg_classifier.infrastructure.http.policy import (
            HttpOutcome,
            HttpStatusKind,
            source_status_from_outcome,
        )

        no_record = source_status_from_outcome(
            source_id="clinvar",
            outcome=HttpOutcome(HttpStatusKind.NO_RECORD),
            checked_at=datetime(2026, 1, 1, tzinfo=UTC),
            normalized_query_key="GRCh38:1:100:A:G",
        )
        outage = source_status_from_outcome(
            source_id="clinvar",
            outcome=HttpOutcome(HttpStatusKind.UNAVAILABLE),
            checked_at=datetime(2026, 1, 1, tzinfo=UTC),
        )

        self.assertEqual(no_record.status, SourceStatusValue.FRESH)
        self.assertEqual(no_record.detail, "no_record")
        self.assertEqual(outage.status, SourceStatusValue.UNAVAILABLE)
        self.assertIsNone(outage.detail)


if __name__ == "__main__":
    unittest.main()
