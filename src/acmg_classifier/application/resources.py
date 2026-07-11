"""Identifier-scoped immutable resource retrieval for presentation adapters."""

from __future__ import annotations

import json
from collections.abc import Mapping
from dataclasses import dataclass
from typing import Protocol, cast

from acmg_classifier.domain.errors import JsonValue


class ResourceNotFoundError(ValueError):
    """A resource is absent, corrupt, or not safe to expose at this boundary."""


class StoredEvidenceSnapshot(Protocol):
    """The immutable evidence snapshot view required by resource presentation."""

    @property
    def snapshot_id(self) -> str: ...

    @property
    def evidence_ids(self) -> tuple[str, ...]: ...

    @property
    def source_status_json(self) -> bytes: ...


class EvidenceResourceStore(Protocol):
    """Read-only evidence and raw snapshot storage boundary."""

    def get_evidence_snapshot(self, snapshot_id: str) -> StoredEvidenceSnapshot: ...

    def get_raw_snapshot(self, raw_snapshot_ref: str) -> bytes: ...


class RulesetResourceStore(Protocol):
    """Read-only versioned ruleset retrieval boundary."""

    def get_ruleset(self, ruleset_id: str, version: str) -> Mapping[str, JsonValue]: ...


@dataclass(frozen=True, slots=True)
class ResourceService:
    """Expose immutable stored content by ID without implicit source discovery."""

    evidence_store: EvidenceResourceStore
    rulesets: RulesetResourceStore

    def get_evidence_snapshot(self, snapshot_id: str) -> dict[str, JsonValue]:
        """Return snapshot IDs and source state without raw payloads."""
        try:
            stored = self.evidence_store.get_evidence_snapshot(snapshot_id)
            source_state = _source_state(stored.source_status_json)
        except Exception as error:
            raise ResourceNotFoundError("evidence snapshot unavailable") from error
        return {
            "snapshot_id": stored.snapshot_id,
            "evidence_ids": list(stored.evidence_ids),
            "policy": source_state["policy"],
            "source_statuses": source_state["source_statuses"],
        }

    def get_ruleset(self, ruleset_id: str, version: str) -> dict[str, JsonValue]:
        """Return one versioned ruleset only when both identity fields match."""
        try:
            content = self.rulesets.get_ruleset(ruleset_id, version)
        except Exception as error:
            raise ResourceNotFoundError("ruleset unavailable") from error
        return dict(content)

    def get_raw_snapshot(self, raw_snapshot_ref: str) -> bytes:
        """Return raw bytes only after presentation explicitly names their reference."""
        try:
            return self.evidence_store.get_raw_snapshot(raw_snapshot_ref)
        except Exception as error:
            raise ResourceNotFoundError("raw snapshot unavailable") from error


def _source_state(content: bytes) -> Mapping[str, JsonValue]:
    try:
        value = json.loads(content)
    except (TypeError, UnicodeDecodeError, json.JSONDecodeError) as error:
        raise ValueError("source status content is invalid") from error
    if not isinstance(value, Mapping):
        raise ValueError("source status content is invalid")
    policy = value.get("policy")
    statuses = value.get("source_statuses")
    if not isinstance(policy, Mapping) or not isinstance(statuses, list):
        raise ValueError("source status content is invalid")
    return cast(Mapping[str, JsonValue], value)
