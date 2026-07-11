"""Replaceable, provenance-aware SQLite index for volatile source retrievals."""

from __future__ import annotations

import json
import re
import sqlite3
from contextlib import closing
from dataclasses import dataclass
from datetime import UTC, datetime, timedelta
from enum import StrEnum
from pathlib import Path

from acmg_classifier.domain.evidence import EvidencePolicy, EvidencePolicyMode
from acmg_classifier.infrastructure.storage.evidence import (
    RawSnapshotIntegrityError,
    RawSnapshotMissingError,
    SQLiteEvidenceStore,
)
from acmg_classifier.ports.evidence import SourceQuery

_EVIDENCE_ID = re.compile(r"^ev_[0-9a-f]{64}$")
_RAW_REF = re.compile(r"^raw_[0-9a-f]{64}$")


class SourceCacheStatus(StrEnum):
    """Persisted source response meaning, never a clinical conclusion."""

    SUCCESS = "success"
    NO_RECORD = "no_record"
    FAILURE = "failure"


class CacheLookupState(StrEnum):
    """Eligibility outcome consumed before any source network request."""

    FRESH = "fresh"
    STALE = "stale"
    INELIGIBLE = "ineligible"
    MISS = "miss"
    OFFLINE_MISS = "offline_miss"


@dataclass(frozen=True, slots=True)
class SourceCacheEntry:
    """One replaceable index row; referenced evidence and raw bytes stay immutable."""

    cache_key: str
    source_id: str
    normalized_query_key: str
    request_fingerprint: str
    source_version: str | None
    adapter_version: str
    status: SourceCacheStatus
    retrieved_at: datetime
    expires_at: datetime | None
    evidence_ids: tuple[str, ...]
    raw_snapshot_ref: str | None
    response_hash: str | None
    media_type: str | None
    byte_size: int
    last_error_code: str | None


@dataclass(frozen=True, slots=True)
class CacheLookup:
    state: CacheLookupState
    entry: SourceCacheEntry | None = None

    @property
    def eligible(self) -> bool:
        return self.state is CacheLookupState.FRESH


class SQLiteSourceCache:
    """Short-transaction cache index; it deliberately never performs network I/O."""

    def __init__(
        self, database_path: Path, raw_snapshot_store: SQLiteEvidenceStore
    ) -> None:
        self.database_path = database_path
        self.raw_snapshot_store = raw_snapshot_store

    def get_eligible(
        self,
        query: SourceQuery,
        *,
        policy: EvidencePolicy,
        now: datetime,
    ) -> CacheLookup:
        now = _as_utc(now)
        with closing(self._connect()) as connection:
            row = connection.execute(
                """
                SELECT
                    cache_key,
                    source_id,
                    normalized_query_key,
                    request_fingerprint,
                    source_version,
                    adapter_version,
                    status,
                    retrieved_at,
                    expires_at,
                    evidence_ids_json,
                    raw_snapshot_ref,
                    response_hash,
                    media_type,
                    byte_size,
                    last_error_code
                FROM source_cache WHERE cache_key = ?
                """,
                (query.cache_key,),
            ).fetchone()
        if row is None:
            state = (
                CacheLookupState.OFFLINE_MISS
                if policy.mode is EvidencePolicyMode.OFFLINE
                else CacheLookupState.MISS
            )
            return CacheLookup(state)
        try:
            entry = _entry_from_row(row)
        except (TypeError, ValueError, json.JSONDecodeError):
            return CacheLookup(CacheLookupState.INELIGIBLE)
        if not self._provenance_references_match(entry):
            return CacheLookup(CacheLookupState.INELIGIBLE, entry)
        if entry.status is SourceCacheStatus.FAILURE:
            return CacheLookup(CacheLookupState.INELIGIBLE, entry)
        if _is_fresh(entry, policy, now):
            return CacheLookup(CacheLookupState.FRESH, entry)
        if policy.mode is EvidencePolicyMode.OFFLINE:
            return CacheLookup(CacheLookupState.INELIGIBLE, entry)
        return CacheLookup(CacheLookupState.STALE, entry)

    def put_success(
        self,
        query: SourceQuery,
        *,
        retrieved_at: datetime,
        expires_at: datetime | None,
        evidence_ids: tuple[str, ...],
        raw_snapshot_ref: str | None,
        response_hash: str | None,
        media_type: str | None,
        byte_size: int,
    ) -> None:
        self._put(
            query,
            status=SourceCacheStatus.SUCCESS,
            retrieved_at=retrieved_at,
            expires_at=expires_at,
            evidence_ids=evidence_ids,
            raw_snapshot_ref=raw_snapshot_ref,
            response_hash=response_hash,
            media_type=media_type,
            byte_size=byte_size,
            last_error_code=None,
        )

    def put_negative(
        self,
        query: SourceQuery,
        *,
        retrieved_at: datetime,
        expires_at: datetime | None,
        raw_snapshot_ref: str | None,
        response_hash: str | None,
        media_type: str | None,
        byte_size: int,
    ) -> None:
        self._put(
            query,
            status=SourceCacheStatus.NO_RECORD,
            retrieved_at=retrieved_at,
            expires_at=expires_at,
            evidence_ids=(),
            raw_snapshot_ref=raw_snapshot_ref,
            response_hash=response_hash,
            media_type=media_type,
            byte_size=byte_size,
            last_error_code=None,
        )

    def put_failure_metadata(
        self, query: SourceQuery, *, retrieved_at: datetime, error_code: str
    ) -> None:
        if not re.fullmatch(r"[A-Z][A-Z_]{2,63}", error_code):
            raise ValueError("error_code must be a stable non-secret code")
        self._put(
            query,
            status=SourceCacheStatus.FAILURE,
            retrieved_at=retrieved_at,
            expires_at=None,
            evidence_ids=(),
            raw_snapshot_ref=None,
            response_hash=None,
            media_type=None,
            byte_size=0,
            last_error_code=error_code,
        )

    def _put(
        self,
        query: SourceQuery,
        *,
        status: SourceCacheStatus,
        retrieved_at: datetime,
        expires_at: datetime | None,
        evidence_ids: tuple[str, ...],
        raw_snapshot_ref: str | None,
        response_hash: str | None,
        media_type: str | None,
        byte_size: int,
        last_error_code: str | None,
    ) -> None:
        retrieved_at = _as_utc(retrieved_at)
        if expires_at is not None:
            expires_at = _as_utc(expires_at)
        evidence_ids = tuple(sorted(set(evidence_ids)))
        if any(_EVIDENCE_ID.fullmatch(item) is None for item in evidence_ids):
            raise ValueError("evidence_ids must be content-addressed evidence IDs")
        if (
            raw_snapshot_ref is not None
            and _RAW_REF.fullmatch(raw_snapshot_ref) is None
        ):
            raise ValueError("raw_snapshot_ref must be a content-addressed raw ID")
        if byte_size < 0:
            raise ValueError("byte_size must be non-negative")
        with closing(self._connect()) as connection:
            connection.execute(
                """
                INSERT INTO source_cache (
                    cache_key, source_id, normalized_query_key, request_fingerprint,
                    source_version, adapter_version, status, retrieved_at, expires_at,
                    evidence_ids_json, raw_snapshot_ref, response_hash, media_type,
                    byte_size, last_error_code, updated_at
                ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                ON CONFLICT(cache_key) DO UPDATE SET
                    status=excluded.status,
                    retrieved_at=excluded.retrieved_at,
                    expires_at=excluded.expires_at,
                    evidence_ids_json=excluded.evidence_ids_json,
                    raw_snapshot_ref=excluded.raw_snapshot_ref,
                    response_hash=excluded.response_hash,
                    media_type=excluded.media_type,
                    byte_size=excluded.byte_size,
                    last_error_code=excluded.last_error_code,
                    updated_at=excluded.updated_at
                """,
                (
                    query.cache_key,
                    query.source_id,
                    query.normalized_query_key,
                    query.request_fingerprint,
                    query.source_version,
                    query.adapter_version,
                    status.value,
                    retrieved_at.isoformat(),
                    None if expires_at is None else expires_at.isoformat(),
                    json.dumps(evidence_ids, separators=(",", ":")),
                    raw_snapshot_ref,
                    response_hash,
                    media_type,
                    byte_size,
                    last_error_code,
                    retrieved_at.isoformat(),
                ),
            )
            connection.commit()

    def _evidence_references_exist(self, evidence_ids: tuple[str, ...]) -> bool:
        if not evidence_ids:
            return True
        placeholders = ",".join("?" for _ in evidence_ids)
        with closing(self._connect()) as connection:
            statement = (
                "SELECT COUNT(*) FROM evidence_items "
                f"WHERE evidence_id IN ({placeholders})"
            )
            count = connection.execute(statement, evidence_ids).fetchone()[0]
        return int(count) == len(evidence_ids)

    def _provenance_references_match(self, entry: SourceCacheEntry) -> bool:
        if entry.raw_snapshot_ref is None:
            return False
        try:
            raw = self.raw_snapshot_store.verify_raw_snapshot(entry.raw_snapshot_ref)
        except (RawSnapshotMissingError, RawSnapshotIntegrityError):
            return False
        return (
            entry.response_hash == raw.snapshot_hash.removeprefix("raw_")
            and entry.media_type == raw.media_type
            and entry.byte_size == raw.byte_size
        )

    def _connect(self) -> sqlite3.Connection:
        connection = sqlite3.connect(self.database_path)
        connection.execute("PRAGMA foreign_keys=ON")
        return connection


def _as_utc(value: object) -> datetime:
    if (
        not isinstance(value, datetime)
        or value.tzinfo is None
        or value.utcoffset() is None
    ):
        raise ValueError("timestamps must be timezone-aware")
    return value.astimezone(UTC)


def _is_fresh(entry: SourceCacheEntry, policy: EvidencePolicy, now: datetime) -> bool:
    if entry.expires_at is None or now > entry.expires_at:
        return False
    if policy.max_age_seconds is not None:
        return now <= entry.retrieved_at + timedelta(seconds=policy.max_age_seconds)
    return True


def _entry_from_row(row: tuple[object, ...]) -> SourceCacheEntry:
    raw_evidence_ids = row[9]
    if not isinstance(raw_evidence_ids, (bytes, str)):
        raise ValueError("invalid cached evidence ID encoding")
    evidence_ids_raw = json.loads(raw_evidence_ids)
    if not isinstance(evidence_ids_raw, list) or any(
        not isinstance(item, str) or _EVIDENCE_ID.fullmatch(item) is None
        for item in evidence_ids_raw
    ):
        raise ValueError("invalid cached evidence IDs")
    status = SourceCacheStatus(str(row[6]))
    raw_snapshot_ref = None if row[10] is None else str(row[10])
    if raw_snapshot_ref is not None and _RAW_REF.fullmatch(raw_snapshot_ref) is None:
        raise ValueError("invalid cached raw reference")
    raw_byte_size = row[13]
    if not isinstance(raw_byte_size, int) or isinstance(raw_byte_size, bool):
        raise ValueError("invalid cached byte size")
    byte_size = raw_byte_size
    if byte_size < 0:
        raise ValueError("invalid cached byte size")
    return SourceCacheEntry(
        cache_key=str(row[0]),
        source_id=str(row[1]),
        normalized_query_key=str(row[2]),
        request_fingerprint=str(row[3]),
        source_version=None if row[4] is None else str(row[4]),
        adapter_version=str(row[5]),
        status=status,
        retrieved_at=_as_utc(datetime.fromisoformat(str(row[7]))),
        expires_at=None
        if row[8] is None
        else _as_utc(datetime.fromisoformat(str(row[8]))),
        evidence_ids=tuple(sorted(set(evidence_ids_raw))),
        raw_snapshot_ref=raw_snapshot_ref,
        response_hash=None if row[11] is None else str(row[11]),
        media_type=None if row[12] is None else str(row[12]),
        byte_size=byte_size,
        last_error_code=None if row[14] is None else str(row[14]),
    )
