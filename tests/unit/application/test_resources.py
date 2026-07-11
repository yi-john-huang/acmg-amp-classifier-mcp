from __future__ import annotations

from dataclasses import dataclass

import pytest

from acmg_classifier.application.resources import ResourceNotFoundError, ResourceService
from acmg_classifier.domain.errors import JsonValue


@dataclass(frozen=True)
class _Snapshot:
    snapshot_id: str
    evidence_ids: tuple[str, ...]
    source_status_json: bytes


class _EvidenceStore:
    def get_evidence_snapshot(self, snapshot_id: str) -> _Snapshot:
        if snapshot_id != "es_test":
            raise KeyError(snapshot_id)
        return _Snapshot(
            snapshot_id=snapshot_id,
            evidence_ids=("ev_a", "ev_b"),
            source_status_json=b'{"policy":{"mode":"offline"},"source_statuses":[]}',
        )

    def get_raw_snapshot(self, raw_snapshot_ref: str) -> bytes:
        if raw_snapshot_ref != "raw_test":
            raise KeyError(raw_snapshot_ref)
        return b'{"raw":true}'


class _Rulesets:
    def get_ruleset(self, ruleset_id: str, version: str) -> dict[str, JsonValue]:
        if (ruleset_id, version) != ("acmg-amp", "2015.1"):
            raise KeyError((ruleset_id, version))
        return {"id": ruleset_id, "version": version}


def test_resource_service_returns_identifier_scoped_immutable_content() -> None:
    service = ResourceService(_EvidenceStore(), _Rulesets())

    snapshot = service.get_evidence_snapshot("es_test")

    assert snapshot == {
        "snapshot_id": "es_test",
        "evidence_ids": ["ev_a", "ev_b"],
        "policy": {"mode": "offline"},
        "source_statuses": [],
    }
    assert service.get_ruleset("acmg-amp", "2015.1") == {
        "id": "acmg-amp",
        "version": "2015.1",
    }
    assert service.get_raw_snapshot("raw_test") == b'{"raw":true}'


def test_resource_service_hides_storage_parse_failures_behind_stable_errors() -> None:
    class _MalformedEvidenceStore(_EvidenceStore):
        def get_evidence_snapshot(self, snapshot_id: str) -> _Snapshot:
            return _Snapshot(snapshot_id, (), b"not-json")

    service = ResourceService(_MalformedEvidenceStore(), _Rulesets())

    with pytest.raises(ResourceNotFoundError, match="evidence snapshot unavailable"):
        service.get_evidence_snapshot("es_test")
