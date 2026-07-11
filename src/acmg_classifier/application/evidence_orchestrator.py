"""Concurrent, adapter-agnostic evidence acquisition and durable snapshots."""

from __future__ import annotations

import asyncio
from collections.abc import Callable, Iterable
from dataclasses import dataclass
from datetime import UTC, datetime
from typing import Protocol

from acmg_classifier.domain.evidence import (
    EvidenceContextScope,
    EvidenceItem,
    EvidencePolicy,
    EvidenceSnapshot,
    SourceStatus,
    SourceStatusValue,
)
from acmg_classifier.ports.evidence import (
    CacheState,
    EvidenceAdapter,
    EvidenceSourceResult,
    NormalizedVariantQuery,
)

_SUCCESS_STATUSES = frozenset((SourceStatusValue.FRESH, SourceStatusValue.CACHED))


class EvidenceSnapshotStore(Protocol):
    """The narrow persistence surface needed after all adapter work completes."""

    def put_evidence(self, content: object) -> str: ...

    def put_domain_evidence_snapshot(self, snapshot: EvidenceSnapshot) -> str: ...


@dataclass(frozen=True, slots=True)
class EvidenceAcquisitionResult:
    """A complete, auditable acquisition outcome with no inferred completeness."""

    snapshot: EvidenceSnapshot
    evidence_items: tuple[EvidenceItem, ...]
    source_results: tuple[EvidenceSourceResult, ...]
    degraded: bool
    unavailable_sources: tuple[str, ...]


class EvidenceOrchestrator:
    """Run adapters concurrently, validate outputs, and persist deterministically."""

    def __init__(
        self,
        *,
        adapters: Iterable[EvidenceAdapter],
        evidence_store: EvidenceSnapshotStore,
        clock: Callable[[], datetime],
        total_deadline_seconds: float,
    ) -> None:
        adapter_tuple = tuple(adapters)
        source_ids = tuple(adapter.source_id for adapter in adapter_tuple)
        if len(source_ids) != len(set(source_ids)):
            raise ValueError("EvidenceOrchestrator requires one adapter per source_id")
        if any(not source_id for source_id in source_ids):
            raise ValueError("adapter source_id must not be empty")
        if total_deadline_seconds <= 0:
            raise ValueError("total_deadline_seconds must be positive")
        self._adapters = tuple(
            sorted(adapter_tuple, key=lambda adapter: adapter.source_id)
        )
        self._evidence_store = evidence_store
        self._clock = clock
        self._total_deadline_seconds = total_deadline_seconds

    async def gather(
        self,
        variant: NormalizedVariantQuery,
        context_scope: EvidenceContextScope,
        policy: EvidencePolicy,
    ) -> EvidenceAcquisitionResult:
        """Acquire source results under one deadline.

        Caller cancellation deliberately is not converted to a source outcome.  A
        total deadline, in contrast, cancels unfinished adapter work and records
        a timeout for each unfinished source while retaining completed results.
        """
        tasks: dict[str, asyncio.Task[EvidenceSourceResult]] = {}
        deadline_expired = False
        try:
            async with asyncio.timeout(self._total_deadline_seconds):
                async with asyncio.TaskGroup() as group:
                    for adapter in self._adapters:
                        tasks[adapter.source_id] = group.create_task(
                            self._query_adapter(
                                adapter, variant, context_scope, policy
                            )
                        )
        except TimeoutError:
            deadline_expired = True

        source_results = tuple(
            self._validated_result(
                source_id=source_id,
                result=self._result_after_deadline(source_id, task, deadline_expired),
                variant_key=variant.variant_key,
                context_scope=context_scope,
            )
            for source_id, task in sorted(tasks.items())
        )
        evidence_items = self._validated_evidence(source_results)

        # Every network task has completed or been cancelled before these short,
        # deterministic storage calls begin.  Storage failures intentionally
        # escape: no acquisition result is claimed unless its snapshot is durable.
        for item in evidence_items:
            stored_id = self._evidence_store.put_evidence(item.canonical_content())
            if stored_id != item.evidence_id:
                raise RuntimeError("evidence store returned a mismatched evidence ID")

        evidence_ids = tuple(self._require_evidence_id(item) for item in evidence_items)
        snapshot = EvidenceSnapshot(
            evidence_ids=evidence_ids,
            source_statuses=tuple(result.source_status for result in source_results),
            policy=policy,
            created_at=self._clock(),
        )
        stored_snapshot_id = self._evidence_store.put_domain_evidence_snapshot(snapshot)
        if stored_snapshot_id != snapshot.snapshot_id:
            raise RuntimeError("evidence store returned a mismatched snapshot ID")

        unavailable_sources = tuple(
            result.source_id
            for result in source_results
            if result.source_status.status not in _SUCCESS_STATUSES
        )
        return EvidenceAcquisitionResult(
            snapshot=snapshot,
            evidence_items=evidence_items,
            source_results=source_results,
            degraded=bool(unavailable_sources),
            unavailable_sources=unavailable_sources,
        )

    async def _query_adapter(
        self,
        adapter: EvidenceAdapter,
        variant: NormalizedVariantQuery,
        context_scope: EvidenceContextScope,
        policy: EvidencePolicy,
    ) -> EvidenceSourceResult:
        try:
            return await adapter.query(
                variant, context_scope=context_scope, policy=policy
            )
        except TimeoutError:
            return self._failure_result(adapter.source_id, SourceStatusValue.TIMEOUT)
        except Exception as error:
            return self._failure_result(
                adapter.source_id,
                SourceStatusValue.UNAVAILABLE,
                detail=f"adapter_error:{type(error).__name__}",
            )

    def _result_after_deadline(
        self,
        source_id: str,
        task: asyncio.Task[EvidenceSourceResult],
        deadline_expired: bool,
    ) -> EvidenceSourceResult:
        if task.cancelled():
            if deadline_expired:
                return self._failure_result(source_id, SourceStatusValue.TIMEOUT)
            raise RuntimeError(
                "adapter task was cancelled outside the acquisition deadline"
            )
        return task.result()

    def _validated_result(
        self,
        *,
        source_id: str,
        result: EvidenceSourceResult,
        variant_key: str,
        context_scope: EvidenceContextScope,
    ) -> EvidenceSourceResult:
        if result.source_id != source_id or result.source_status.source_id != source_id:
            return self._failure_result(
                source_id,
                SourceStatusValue.SCHEMA_CHANGED,
                detail="adapter_source_id_mismatch",
            )
        try:
            for item in result.evidence_items:
                item.assert_applies_to(
                    variant_key=variant_key,
                    context_scope=context_scope,
                )
        except ValueError:
            return self._failure_result(
                source_id,
                SourceStatusValue.SCHEMA_CHANGED,
                detail="adapter_evidence_scope_mismatch",
            )
        return result

    def _validated_evidence(
        self, source_results: tuple[EvidenceSourceResult, ...]
    ) -> tuple[EvidenceItem, ...]:
        by_id: dict[str, EvidenceItem] = {}
        for result in source_results:
            for item in result.evidence_items:
                evidence_id = self._require_evidence_id(item)
                by_id[evidence_id] = item
        return tuple(by_id[evidence_id] for evidence_id in sorted(by_id))

    def _failure_result(
        self,
        source_id: str,
        status: SourceStatusValue,
        *,
        detail: str | None = None,
    ) -> EvidenceSourceResult:
        return EvidenceSourceResult(
            source_id=source_id,
            evidence_items=(),
            source_status=SourceStatus(
                source_id=source_id,
                status=status,
                checked_at=self._as_utc(self._clock()),
                detail=detail,
            ),
            cache_state=CacheState.LIVE,
        )

    @staticmethod
    def _require_evidence_id(item: EvidenceItem) -> str:
        if item.evidence_id is None:
            raise RuntimeError("validated EvidenceItem must have an evidence_id")
        return item.evidence_id

    @staticmethod
    def _as_utc(value: datetime) -> datetime:
        if value.tzinfo is None or value.utcoffset() is None:
            raise ValueError("clock must return a timezone-aware datetime")
        return value.astimezone(UTC)
