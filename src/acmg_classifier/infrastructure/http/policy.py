"""Fail-closed HTTP policy shared by official evidence-source adapters."""

from __future__ import annotations

import asyncio
import time
from collections.abc import AsyncIterator, Awaitable, Callable, Mapping
from dataclasses import dataclass, field
from datetime import UTC, datetime
from email.utils import parsedate_to_datetime
from enum import StrEnum
from typing import Protocol
from urllib.parse import urljoin, urlsplit

from acmg_classifier.domain.evidence import (
    EvidencePolicy,
    EvidencePolicyMode,
    SourceStatus,
    SourceStatusValue,
)
from acmg_classifier.infrastructure.storage.source_cache import (
    CacheLookup,
    SQLiteSourceCache,
)
from acmg_classifier.ports.evidence import SourceQuery


class HttpStatusKind(StrEnum):
    SUCCESS = "success"
    NO_RECORD = "no_record"
    RATE_LIMITED = "rate_limited"
    TIMEOUT = "timeout"
    UNAVAILABLE = "unavailable"
    RESPONSE_TOO_LARGE = "response_too_large"
    INVALID_QUERY = "invalid_query"


class CircuitState(StrEnum):
    CLOSED = "closed"
    OPEN = "open"
    HALF_OPEN = "half_open"


@dataclass(frozen=True, slots=True)
class HttpPolicy:
    source_id: str
    allowed_hosts: frozenset[str]
    timeout_seconds: float
    max_response_bytes: int
    max_retries: int
    deadline_seconds: float | None = None
    base_backoff_seconds: float = 0.25
    max_backoff_seconds: float = 5.0
    circuit_failure_threshold: int = 3
    circuit_cooldown_seconds: float = 30.0
    max_redirects: int = 3
    user_agent: str = "acmg-classifier/0.1"

    def __post_init__(self) -> None:
        if not self.source_id or not self.allowed_hosts:
            raise ValueError("source_id and allowed_hosts are required")
        if any(not host or host != host.lower() for host in self.allowed_hosts):
            raise ValueError("allowed_hosts must be lowercase host names")
        if self.timeout_seconds <= 0 or self.max_response_bytes < 1:
            raise ValueError("timeout_seconds and max_response_bytes must be positive")
        if self.deadline_seconds is not None and self.deadline_seconds <= 0:
            raise ValueError("deadline_seconds must be positive when provided")
        if (
            self.max_retries < 0
            or self.base_backoff_seconds < 0
            or self.max_backoff_seconds < 0
        ):
            raise ValueError("retry settings must be non-negative")
        if self.circuit_failure_threshold < 1 or self.circuit_cooldown_seconds < 0:
            raise ValueError("circuit settings are invalid")


@dataclass(frozen=True, slots=True)
class HttpRequest:
    method: str
    url: str
    headers: Mapping[str, str] = field(default_factory=dict)
    body: bytes | None = None
    read_only: bool = False


@dataclass(slots=True)
class HttpResponse:
    status_code: int
    headers: Mapping[str, str]
    body: AsyncIterator[bytes]
    close: Callable[[], Awaitable[None]] | None = None

    async def aclose(self) -> None:
        if self.close is not None:
            await self.close()


class AsyncHttpTransport(Protocol):
    """Minimal injectable transport; production uses HTTPXTransport."""

    async def request(self, request: HttpRequest) -> HttpResponse: ...


@dataclass(frozen=True, slots=True)
class HttpOutcome:
    kind: HttpStatusKind
    body: bytes = b""
    media_type: str | None = None
    retry_after_seconds: float | None = None
    attempts: int = 0
    circuit_open: bool = False


def source_status_from_outcome(
    *,
    source_id: str,
    outcome: HttpOutcome,
    checked_at: datetime,
    normalized_query_key: str | None = None,
    source_version: str | None = None,
) -> SourceStatus:
    """Map transport outcomes to auditable source status without inferring evidence."""
    mapping = {
        HttpStatusKind.SUCCESS: SourceStatusValue.FRESH,
        HttpStatusKind.NO_RECORD: SourceStatusValue.FRESH,
        HttpStatusKind.RATE_LIMITED: SourceStatusValue.RATE_LIMITED,
        HttpStatusKind.TIMEOUT: SourceStatusValue.TIMEOUT,
        HttpStatusKind.UNAVAILABLE: SourceStatusValue.UNAVAILABLE,
        HttpStatusKind.RESPONSE_TOO_LARGE: SourceStatusValue.UNAVAILABLE,
        HttpStatusKind.INVALID_QUERY: SourceStatusValue.UNAVAILABLE,
    }
    detail = {
        HttpStatusKind.NO_RECORD: "no_record",
        HttpStatusKind.RESPONSE_TOO_LARGE: "response_too_large",
        HttpStatusKind.INVALID_QUERY: "invalid_query",
    }.get(outcome.kind)
    return SourceStatus(
        source_id=source_id,
        status=mapping[outcome.kind],
        checked_at=checked_at,
        source_version=source_version,
        normalized_query_key=normalized_query_key,
        detail=detail,
    )


@dataclass(frozen=True, slots=True)
class SourceRequestExecution:
    """A generic pre-parser result: eligible cache metadata or bounded HTTP bytes."""

    cache_lookup: CacheLookup
    outcome: HttpOutcome | None

    @property
    def should_fetch(self) -> bool:
        return self.outcome is not None


class SourceRequestExecutor:
    """Check cache eligibility before HTTP without holding SQLite transactions."""

    def __init__(
        self,
        *,
        cache: SQLiteSourceCache,
        client: SourceHttpClient,
        clock: Callable[[], datetime],
    ) -> None:
        self.cache = cache
        self.client = client
        self.clock = clock

    async def request(
        self, query: SourceQuery, request: HttpRequest, policy: EvidencePolicy
    ) -> SourceRequestExecution:
        lookup = self.cache.get_eligible(query, policy=policy, now=self.clock())
        if lookup.eligible or policy.mode is EvidencePolicyMode.OFFLINE:
            return SourceRequestExecution(cache_lookup=lookup, outcome=None)
        return SourceRequestExecution(
            cache_lookup=lookup, outcome=await self.client.request(request)
        )


class SourceHttpClient:
    """TLS-only host-bound client with bounded streaming, retry, and circuit state."""

    def __init__(
        self,
        *,
        transport: AsyncHttpTransport,
        policy: HttpPolicy,
        sleep: Callable[[float], Awaitable[None]] = asyncio.sleep,
        monotonic: Callable[[], float] = time.monotonic,
    ) -> None:
        self.transport = transport
        self.policy = policy
        self._sleep = sleep
        self._monotonic = monotonic
        self._failure_count = 0
        self._opened_at: float | None = None

    @property
    def circuit_state(self) -> CircuitState:
        if self._opened_at is None:
            return CircuitState.CLOSED
        if self._monotonic() - self._opened_at < self.policy.circuit_cooldown_seconds:
            return CircuitState.OPEN
        return CircuitState.HALF_OPEN

    async def request(self, request: HttpRequest) -> HttpOutcome:
        if not _is_allowed_url(request.url, self.policy.allowed_hosts):
            return HttpOutcome(HttpStatusKind.INVALID_QUERY)
        if self.circuit_state is CircuitState.OPEN:
            return HttpOutcome(HttpStatusKind.UNAVAILABLE, circuit_open=True)
        started_at = self._monotonic()
        attempts = 0
        headers = dict(request.headers)
        if not any(key.lower() == "user-agent" for key in headers):
            headers["User-Agent"] = self.policy.user_agent
        current = HttpRequest(
            method=request.method,
            url=request.url,
            headers=headers,
            body=request.body,
            read_only=request.read_only,
        )
        redirects = 0
        while True:
            attempts += 1
            timeout = self.policy.timeout_seconds
            if self.policy.deadline_seconds is not None:
                remaining = self.policy.deadline_seconds - (
                    self._monotonic() - started_at
                )
                if remaining <= 0:
                    outcome = HttpOutcome(HttpStatusKind.TIMEOUT)
                    self._update_circuit(outcome)
                    return HttpOutcome(HttpStatusKind.TIMEOUT, attempts=attempts - 1)
                timeout = min(timeout, remaining)
            try:
                response = await asyncio.wait_for(
                    self.transport.request(current), timeout=timeout
                )
                outcome, redirect = await self._consume_response(response, current.url)
            except TimeoutError:
                outcome, redirect = HttpOutcome(HttpStatusKind.TIMEOUT), None
            except Exception:
                outcome, redirect = HttpOutcome(HttpStatusKind.UNAVAILABLE), None
            if redirect is not None:
                redirects += 1
                if redirects > self.policy.max_redirects or not _is_allowed_url(
                    redirect, self.policy.allowed_hosts
                ):
                    return HttpOutcome(HttpStatusKind.INVALID_QUERY, attempts=attempts)
                current = HttpRequest(
                    method=current.method,
                    url=redirect,
                    headers=current.headers,
                    body=current.body,
                    read_only=current.read_only,
                )
                continue
            outcome = HttpOutcome(
                kind=outcome.kind,
                body=outcome.body,
                media_type=outcome.media_type,
                retry_after_seconds=outcome.retry_after_seconds,
                attempts=attempts,
                circuit_open=False,
            )
            if (
                self._retryable(outcome, current)
                and attempts <= self.policy.max_retries
            ):
                delay = _retry_delay(outcome, attempts, self.policy)
                if (
                    self.policy.deadline_seconds is not None
                    and self._monotonic() - started_at + delay
                    >= self.policy.deadline_seconds
                ):
                    timeout_outcome = HttpOutcome(
                        HttpStatusKind.TIMEOUT, attempts=attempts
                    )
                    self._update_circuit(timeout_outcome)
                    return timeout_outcome
                await self._sleep(delay)
                continue
            self._update_circuit(outcome)
            return outcome

    async def _consume_response(
        self, response: HttpResponse, base_url: str
    ) -> tuple[HttpOutcome, str | None]:
        headers = {key.lower(): value for key, value in response.headers.items()}
        try:
            if response.status_code in {301, 302, 303, 307, 308}:
                location = headers.get("location")
                return HttpOutcome(HttpStatusKind.UNAVAILABLE), (
                    None if location is None else urljoin(base_url, location)
                )
            if response.status_code == 429:
                return HttpOutcome(
                    HttpStatusKind.RATE_LIMITED,
                    retry_after_seconds=_retry_after(headers.get("retry-after")),
                ), None
            if response.status_code in {204, 404}:
                return HttpOutcome(HttpStatusKind.NO_RECORD), None
            if not 200 <= response.status_code < 300:
                return HttpOutcome(HttpStatusKind.UNAVAILABLE), None
            body = bytearray()
            async for chunk in response.body:
                if len(body) + len(chunk) > self.policy.max_response_bytes:
                    return HttpOutcome(HttpStatusKind.RESPONSE_TOO_LARGE), None
                body.extend(chunk)
            return HttpOutcome(
                HttpStatusKind.SUCCESS,
                bytes(body),
                headers.get("content-type"),
            ), None
        finally:
            await response.aclose()

    def _retryable(self, outcome: HttpOutcome, request: HttpRequest) -> bool:
        safe = request.method.upper() in {"GET", "HEAD", "OPTIONS"} or request.read_only
        return safe and outcome.kind in {
            HttpStatusKind.RATE_LIMITED,
            HttpStatusKind.TIMEOUT,
            HttpStatusKind.UNAVAILABLE,
        }

    def _update_circuit(self, outcome: HttpOutcome) -> None:
        if outcome.kind in {HttpStatusKind.SUCCESS, HttpStatusKind.NO_RECORD}:
            self._failure_count = 0
            self._opened_at = None
            return
        if outcome.kind in {
            HttpStatusKind.INVALID_QUERY,
            HttpStatusKind.RESPONSE_TOO_LARGE,
        }:
            return
        self._failure_count += 1
        if self._failure_count >= self.policy.circuit_failure_threshold:
            self._opened_at = self._monotonic()


def _is_allowed_url(url: str, allowed_hosts: frozenset[str]) -> bool:
    parsed = urlsplit(url)
    return (
        parsed.scheme == "https"
        and parsed.hostname is not None
        and parsed.hostname.lower() in allowed_hosts
        and parsed.username is None
        and parsed.password is None
    )


def _retry_after(value: str | None) -> float | None:
    if value is None:
        return None
    try:
        seconds = float(value)
    except ValueError:
        try:
            parsed = parsedate_to_datetime(value)
        except (TypeError, ValueError):
            return None
        if parsed.tzinfo is None:
            parsed = parsed.replace(tzinfo=UTC)
        seconds = (parsed - datetime.now(UTC)).total_seconds()
    return max(0.0, seconds)


def _retry_delay(outcome: HttpOutcome, attempt: int, policy: HttpPolicy) -> float:
    if outcome.retry_after_seconds is not None:
        return outcome.retry_after_seconds
    exponential_delay = policy.base_backoff_seconds * 2.0 ** (attempt - 1)
    return min(policy.max_backoff_seconds, exponential_delay)
