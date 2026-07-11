"""Immutable stored-record replay and causal reinterpretation differences."""

from __future__ import annotations

import json
from collections.abc import Mapping
from dataclasses import dataclass
from types import MappingProxyType
from typing import Protocol, cast

from acmg_classifier.domain.errors import JsonValue

type FrozenJsonValue = (
    None
    | bool
    | int
    | float
    | str
    | tuple[FrozenJsonValue, ...]
    | Mapping[str, FrozenJsonValue]
)


class ReinterpretationError(ValueError):
    """A stable error when an immutable record cannot be replayed safely."""


class StoredClassificationRecord(Protocol):
    """The narrow immutable classification view required for replay."""

    @property
    def classification_id(self) -> str: ...

    @property
    def canonical_json(self) -> bytes: ...

    @property
    def previous_classification_id(self) -> str | None: ...


class ClassificationRecordReader(Protocol):
    """Read-only record boundary; replay must never requery scientific sources."""

    def get_classification(
        self, classification_id: str
    ) -> StoredClassificationRecord: ...


@dataclass(frozen=True, slots=True)
class ReplayedClassification:
    """A read-only view of exactly one persisted interpretation."""

    classification_id: str
    content: Mapping[str, FrozenJsonValue]
    previous_classification_id: str | None

    def __post_init__(self) -> None:
        object.__setattr__(
            self,
            "content",
            _freeze_mapping(cast(Mapping[str, JsonValue], self.content)),
        )

    def to_canonical_content(self) -> dict[str, JsonValue]:
        """Return a mutable caller copy of the record's canonical content."""
        return cast(dict[str, JsonValue], _thaw_json(self.content))


@dataclass(frozen=True, slots=True)
class ReinterpretationDifference:
    """Semantic changes between immutable records, excluding presentation metadata."""

    previous_classification_id: str
    classification_id: str
    evidence_added: tuple[str, ...]
    evidence_removed: tuple[str, ...]
    context_changes: Mapping[str, tuple[JsonValue, JsonValue]]
    ruleset_changed: bool
    bundle_changed: bool
    criteria_changed: tuple[str, ...]
    classification_before: str | None
    classification_after: str | None

    def __post_init__(self) -> None:
        object.__setattr__(
            self,
            "evidence_added",
            tuple(sorted(set(self.evidence_added))),
        )
        object.__setattr__(
            self,
            "evidence_removed",
            tuple(sorted(set(self.evidence_removed))),
        )
        object.__setattr__(
            self,
            "criteria_changed",
            tuple(sorted(set(self.criteria_changed))),
        )
        object.__setattr__(
            self,
            "context_changes",
            MappingProxyType(dict(sorted(self.context_changes.items()))),
        )

    def to_canonical_content(self) -> dict[str, JsonValue]:
        """Return deterministic semantic differences without display timestamps."""
        return {
            "previous_classification_id": self.previous_classification_id,
            "classification_id": self.classification_id,
            "evidence_added": list(self.evidence_added),
            "evidence_removed": list(self.evidence_removed),
            "context_changes": {
                field: [before, after]
                for field, (before, after) in self.context_changes.items()
            },
            "ruleset_changed": self.ruleset_changed,
            "bundle_changed": self.bundle_changed,
            "criteria_changed": list(self.criteria_changed),
            "classification_before": self.classification_before,
            "classification_after": self.classification_after,
        }


class ReinterpretationService:
    """Replay stored results and compare only decision-relevant scientific inputs."""

    def __init__(self, records: ClassificationRecordReader) -> None:
        self._records = records

    def replay(self, classification_id: str) -> ReplayedClassification:
        """Read one immutable record without invoking a source, evaluator, or rule."""
        stored = self._load(classification_id)
        return ReplayedClassification(
            classification_id=stored.classification_id,
            content=cast(
                Mapping[str, FrozenJsonValue],
                _record_content(stored.canonical_json),
            ),
            previous_classification_id=stored.previous_classification_id,
        )

    def difference(
        self,
        previous_classification_id: str,
        classification_id: str,
    ) -> ReinterpretationDifference:
        """Report causal scientific changes, never timestamps or display prose."""
        previous = self.replay(previous_classification_id)
        current = self.replay(classification_id)
        previous_content = previous.to_canonical_content()
        current_content = current.to_canonical_content()
        before_context = _required_mapping(previous_content, "context")
        after_context = _required_mapping(current_content, "context")
        context_changes = {
            key: (before_context.get(key), after_context.get(key))
            for key in sorted(set(before_context) | set(after_context))
            if before_context.get(key) != after_context.get(key)
        }
        before_evidence = _evidence_ids(previous_content)
        after_evidence = _evidence_ids(current_content)
        before_decision = _required_mapping(previous_content, "decision")
        after_decision = _required_mapping(current_content, "decision")
        return ReinterpretationDifference(
            previous_classification_id=previous_classification_id,
            classification_id=classification_id,
            evidence_added=tuple(after_evidence - before_evidence),
            evidence_removed=tuple(before_evidence - after_evidence),
            context_changes=context_changes,
            ruleset_changed=_required_mapping(previous_content, "ruleset")
            != _required_mapping(current_content, "ruleset"),
            bundle_changed=previous_content.get("bundle_version")
            != current_content.get("bundle_version"),
            criteria_changed=_criteria_changes(before_decision, after_decision),
            classification_before=_optional_text(before_decision, "classification"),
            classification_after=_optional_text(after_decision, "classification"),
        )

    def _load(self, classification_id: str) -> StoredClassificationRecord:
        if not classification_id:
            raise ReinterpretationError("classification ID must not be empty")
        try:
            return self._records.get_classification(classification_id)
        except Exception as error:
            raise ReinterpretationError(
                "classification record is unavailable"
            ) from error


def _record_content(raw: bytes) -> dict[str, JsonValue]:
    try:
        parsed = json.loads(raw)
    except (UnicodeDecodeError, json.JSONDecodeError) as error:
        raise ReinterpretationError(
            "classification record content is invalid"
        ) from error
    if not isinstance(parsed, dict):
        raise ReinterpretationError("classification record content is invalid")
    return cast(dict[str, JsonValue], parsed)


def _freeze_mapping(
    content: Mapping[str, JsonValue],
) -> Mapping[str, FrozenJsonValue]:
    return MappingProxyType(
        {key: _freeze_json(value) for key, value in content.items()}
    )


def _freeze_json(value: JsonValue) -> FrozenJsonValue:
    if isinstance(value, dict):
        return _freeze_mapping(value)
    if isinstance(value, list):
        return tuple(_freeze_json(item) for item in value)
    return value


def _thaw_json(value: FrozenJsonValue) -> JsonValue:
    if isinstance(value, Mapping):
        return {key: _thaw_json(item) for key, item in value.items()}
    if isinstance(value, tuple):
        return [_thaw_json(item) for item in value]
    return value


def _required_mapping(
    content: Mapping[str, JsonValue],
    key: str,
) -> Mapping[str, JsonValue]:
    value = content.get(key)
    if not isinstance(value, dict):
        raise ReinterpretationError(f"classification record {key} is invalid")
    return value


def _evidence_ids(content: Mapping[str, JsonValue]) -> set[str]:
    snapshot = _required_mapping(content, "evidence_snapshot")
    snapshot_content = _required_mapping(snapshot, "content")
    values = snapshot_content.get("evidence_ids")
    if not isinstance(values, list) or not all(
        isinstance(value, str) for value in values
    ):
        raise ReinterpretationError("classification record evidence IDs are invalid")
    return set(cast(list[str], values))


def _criteria_changes(
    before: Mapping[str, JsonValue],
    after: Mapping[str, JsonValue],
) -> tuple[str, ...]:
    before_assessments = _assessments_by_code(before)
    after_assessments = _assessments_by_code(after)
    return tuple(
        code
        for code in sorted(set(before_assessments) | set(after_assessments))
        if before_assessments.get(code) != after_assessments.get(code)
    )


def _assessments_by_code(
    decision: Mapping[str, JsonValue],
) -> dict[str, Mapping[str, JsonValue]]:
    assessments = decision.get("assessments")
    if not isinstance(assessments, list):
        raise ReinterpretationError("classification record assessments are invalid")
    result: dict[str, Mapping[str, JsonValue]] = {}
    for assessment in assessments:
        if not isinstance(assessment, dict):
            raise ReinterpretationError("classification record assessment is invalid")
        code = assessment.get("code")
        if not isinstance(code, str) or not code or code in result:
            raise ReinterpretationError(
                "classification record criterion code is invalid"
            )
        result[code] = assessment
    return result


def _optional_text(content: Mapping[str, JsonValue], key: str) -> str | None:
    value = content.get(key)
    if value is not None and not isinstance(value, str):
        raise ReinterpretationError(f"classification record {key} is invalid")
    return value
