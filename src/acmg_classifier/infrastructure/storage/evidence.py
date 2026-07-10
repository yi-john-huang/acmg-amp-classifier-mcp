"""Content-addressed evidence and raw source snapshot persistence."""

import hashlib
import os
import sqlite3
import tempfile
from contextlib import closing
from dataclasses import dataclass
from pathlib import Path

from acmg_classifier.domain.canonical import canonical_hash, canonical_json_bytes
from acmg_classifier.domain.errors import JsonValue
from acmg_classifier.domain.evidence import EvidenceSnapshot


class EvidenceStoreError(RuntimeError):
    """Base class for expected evidence persistence failures."""


class EvidenceNotFoundError(EvidenceStoreError):
    """Referenced evidence does not exist."""


class RawSnapshotMissingError(EvidenceStoreError):
    """Raw snapshot metadata or content is missing."""


class RawSnapshotTooLargeError(EvidenceStoreError):
    """Raw source content exceeds the configured safety limit."""


class RawSnapshotIntegrityError(EvidenceStoreError):
    """Raw source content no longer matches its recorded hash."""


class ContentHashCollisionError(EvidenceStoreError):
    """Stored content does not match an existing content identifier."""


@dataclass(frozen=True, slots=True)
class RawSnapshotReference:
    """Metadata required to retrieve a content-addressed raw response."""

    snapshot_hash: str
    media_type: str
    byte_size: int
    relative_path: Path


@dataclass(frozen=True, slots=True)
class StoredEvidenceSnapshot:
    """An immutable set of normalized evidence and source statuses."""

    snapshot_id: str
    evidence_ids: tuple[str, ...]
    source_status_json: bytes


class SQLiteEvidenceStore:
    """Persist immutable evidence in SQLite and large raw bytes on disk."""

    def __init__(
        self,
        database_path: Path,
        raw_root: Path,
        *,
        max_raw_bytes: int = 10 * 1024 * 1024,
    ) -> None:
        if max_raw_bytes < 1:
            raise ValueError("max_raw_bytes must be positive")
        self.database_path = database_path
        self.raw_root = raw_root
        self.max_raw_bytes = max_raw_bytes

    def put_evidence(self, content: object) -> str:
        """Insert canonical evidence once and return its stable identifier."""
        canonical = canonical_json_bytes(content)
        evidence_id = f"ev_{canonical_hash(content)}"
        with closing(self._connect()) as connection:
            connection.execute(
                """
                INSERT OR IGNORE INTO evidence_items
                    (evidence_id, canonical_json, created_at)
                VALUES (?, ?, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
                """,
                (evidence_id, canonical),
            )
            existing = connection.execute(
                "SELECT canonical_json FROM evidence_items WHERE evidence_id = ?",
                (evidence_id,),
            ).fetchone()[0]
            connection.commit()
        if bytes(existing) != canonical:
            raise ContentHashCollisionError(f"Content differs for {evidence_id}")
        return evidence_id

    def get_evidence(self, evidence_id: str) -> bytes:
        """Return canonical evidence JSON by identifier."""
        with closing(self._connect()) as connection:
            row = connection.execute(
                "SELECT canonical_json FROM evidence_items WHERE evidence_id = ?",
                (evidence_id,),
            ).fetchone()
        if row is None:
            raise EvidenceNotFoundError(f"Evidence not found: {evidence_id}")
        return bytes(row[0])

    def put_raw_snapshot(self, content: bytes, media_type: str) -> RawSnapshotReference:
        """Atomically persist bounded raw source content by SHA-256."""
        if len(content) > self.max_raw_bytes:
            raise RawSnapshotTooLargeError(
                f"Raw snapshots are limited to {self.max_raw_bytes} bytes"
            )
        if not media_type:
            raise ValueError("media_type must not be empty")
        digest = hashlib.sha256(content).hexdigest()
        snapshot_hash = f"raw_{digest}"
        relative_path = Path(digest[:2]) / f"{digest}.bin"
        target = self.raw_root / relative_path
        created = self._write_raw_once(target, content)
        try:
            with closing(self._connect()) as connection:
                connection.execute(
                    """
                    INSERT OR IGNORE INTO raw_snapshots
                        (
                            snapshot_hash,
                            media_type,
                            byte_size,
                            relative_path,
                            created_at
                        )
                    VALUES (?, ?, ?, ?, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
                    """,
                    (snapshot_hash, media_type, len(content), relative_path.as_posix()),
                )
                metadata = connection.execute(
                    """
                    SELECT media_type, byte_size, relative_path
                    FROM raw_snapshots
                    WHERE snapshot_hash = ?
                    """,
                    (snapshot_hash,),
                ).fetchone()
                connection.commit()
        except sqlite3.Error:
            if created:
                target.unlink(missing_ok=True)
            raise
        expected_metadata = (media_type, len(content), relative_path.as_posix())
        if metadata != expected_metadata:
            raise ContentHashCollisionError(f"Metadata differs for {snapshot_hash}")
        return RawSnapshotReference(
            snapshot_hash=snapshot_hash,
            media_type=media_type,
            byte_size=len(content),
            relative_path=relative_path,
        )

    def get_raw_snapshot(self, snapshot_hash: str) -> bytes:
        """Read raw content and verify its content address."""
        with closing(self._connect()) as connection:
            row = connection.execute(
                "SELECT relative_path FROM raw_snapshots WHERE snapshot_hash = ?",
                (snapshot_hash,),
            ).fetchone()
        if row is None:
            raise RawSnapshotMissingError(f"Raw snapshot not found: {snapshot_hash}")
        path = self.raw_root / str(row[0])
        try:
            content = path.read_bytes()
        except FileNotFoundError as error:
            raise RawSnapshotMissingError(
                f"Raw snapshot file is missing: {snapshot_hash}"
            ) from error
        actual_hash = f"raw_{hashlib.sha256(content).hexdigest()}"
        if actual_hash != snapshot_hash:
            raise RawSnapshotIntegrityError(
                f"Raw snapshot hash mismatch: {snapshot_hash}"
            )
        return content

    def put_evidence_snapshot(
        self,
        *,
        evidence_ids: tuple[str, ...],
        source_status: JsonValue,
    ) -> str:
        """Persist an order-independent set of evidence references."""
        unique_ids = tuple(sorted(set(evidence_ids)))
        with closing(self._connect()) as connection:
            self._verify_evidence_ids(connection, unique_ids)
            source_status_json = canonical_json_bytes(source_status)
            content: JsonValue = {
                "evidence_ids": list(unique_ids),
                "source_status": source_status,
            }
            canonical = canonical_json_bytes(content)
            snapshot_id = f"es_{canonical_hash(content)}"
            connection.execute(
                """
                INSERT OR IGNORE INTO evidence_snapshots
                    (snapshot_id, canonical_json, source_status_json, created_at)
                VALUES (?, ?, ?, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
                """,
                (snapshot_id, canonical, source_status_json),
            )
            connection.executemany(
                """
                INSERT OR IGNORE INTO evidence_snapshot_items
                    (snapshot_id, evidence_id)
                VALUES (?, ?)
                """,
                ((snapshot_id, evidence_id) for evidence_id in unique_ids),
            )
            connection.commit()
        return snapshot_id

    def put_domain_evidence_snapshot(self, snapshot: EvidenceSnapshot) -> str:
        """Persist a typed domain snapshot without changing its canonical ID."""
        snapshot_id = snapshot.snapshot_id
        if snapshot_id is None:
            raise RuntimeError("validated EvidenceSnapshot must have a snapshot_id")
        evidence_ids = snapshot.evidence_ids
        canonical_content = snapshot.canonical_content()
        source_status: JsonValue = {
            "source_statuses": [
                status.model_dump(mode="json") for status in snapshot.source_statuses
            ],
            "policy": snapshot.policy.model_dump(mode="json"),
        }
        canonical = canonical_json_bytes(canonical_content)
        source_status_json = canonical_json_bytes(source_status)
        with closing(self._connect()) as connection:
            self._verify_evidence_ids(connection, evidence_ids)
            connection.execute(
                """
                INSERT OR IGNORE INTO evidence_snapshots
                    (snapshot_id, canonical_json, source_status_json, created_at)
                VALUES (?, ?, ?, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
                """,
                (snapshot_id, canonical, source_status_json),
            )
            connection.executemany(
                """
                INSERT OR IGNORE INTO evidence_snapshot_items
                    (snapshot_id, evidence_id)
                VALUES (?, ?)
                """,
                ((snapshot_id, evidence_id) for evidence_id in evidence_ids),
            )
            connection.commit()
        return snapshot_id

    def get_evidence_snapshot(self, snapshot_id: str) -> StoredEvidenceSnapshot:
        """Return an immutable evidence-set record by identifier."""
        with closing(self._connect()) as connection:
            row = connection.execute(
                """
                SELECT source_status_json
                FROM evidence_snapshots
                WHERE snapshot_id = ?
                """,
                (snapshot_id,),
            ).fetchone()
            evidence_ids = tuple(
                str(item[0])
                for item in connection.execute(
                    """
                    SELECT evidence_id
                    FROM evidence_snapshot_items
                    WHERE snapshot_id = ?
                    ORDER BY evidence_id
                    """,
                    (snapshot_id,),
                )
            )
        if row is None:
            raise EvidenceNotFoundError(f"Evidence snapshot not found: {snapshot_id}")
        return StoredEvidenceSnapshot(
            snapshot_id=snapshot_id,
            evidence_ids=evidence_ids,
            source_status_json=bytes(row[0]),
        )

    def _connect(self) -> sqlite3.Connection:
        connection = sqlite3.connect(self.database_path)
        connection.execute("PRAGMA foreign_keys=ON")
        return connection

    def _verify_evidence_ids(
        self,
        connection: sqlite3.Connection,
        evidence_ids: tuple[str, ...],
    ) -> None:
        for evidence_id in evidence_ids:
            exists = connection.execute(
                "SELECT 1 FROM evidence_items WHERE evidence_id = ?",
                (evidence_id,),
            ).fetchone()
            if exists is None:
                raise EvidenceNotFoundError(f"Evidence not found: {evidence_id}")

    def _write_raw_once(self, target: Path, content: bytes) -> bool:
        if target.exists():
            if target.read_bytes() != content:
                raise RawSnapshotIntegrityError(
                    f"Raw snapshot file is corrupt: {target.name}"
                )
            return False
        target.parent.mkdir(parents=True, exist_ok=True)
        descriptor, temporary_name = tempfile.mkstemp(
            dir=target.parent, prefix=".staging-"
        )
        temporary_path = Path(temporary_name)
        try:
            with os.fdopen(descriptor, "wb") as temporary_file:
                temporary_file.write(content)
                temporary_file.flush()
                os.fsync(temporary_file.fileno())
            os.replace(temporary_path, target)
        finally:
            temporary_path.unlink(missing_ok=True)
        return True
