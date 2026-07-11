"""Deterministic serialization for evidence and decision content."""

import hashlib
import json
import math
from collections.abc import Mapping
from datetime import UTC, datetime

from acmg_classifier.domain.errors import JsonValue


class CanonicalizationError(ValueError):
    """Raised when a value cannot be represented canonically."""


def canonical_json_bytes(
    value: object,
    *,
    excluded_keys: set[str] | frozenset[str] = frozenset(),
) -> bytes:
    """Return deterministic UTF-8 JSON while preserving ordered sequences."""
    normalized = _normalize(value, excluded_keys=frozenset(excluded_keys))
    return _encode(normalized)


def canonical_hash(
    value: object,
    *,
    excluded_keys: set[str] | frozenset[str] = frozenset(),
) -> str:
    """Return the SHA-256 hex digest of canonical JSON bytes."""
    return hashlib.sha256(
        canonical_json_bytes(value, excluded_keys=excluded_keys)
    ).hexdigest()


def _normalize(value: object, *, excluded_keys: frozenset[str]) -> JsonValue:
    if value is None or isinstance(value, (bool, str, int)):
        return value
    if isinstance(value, float):
        if not math.isfinite(value):
            raise CanonicalizationError("Canonical numbers must be finite")
        return value
    if isinstance(value, datetime):
        if value.tzinfo is None or value.utcoffset() is None:
            raise CanonicalizationError("Canonical timestamps must be timezone-aware")
        return (
            value.astimezone(UTC)
            .isoformat(timespec="microseconds")
            .replace("+00:00", "Z")
        )
    if isinstance(value, Mapping):
        normalized: dict[str, JsonValue] = {}
        for key, item in value.items():
            if not isinstance(key, str):
                raise CanonicalizationError("Canonical mapping keys must be strings")
            if key not in excluded_keys:
                normalized[key] = _normalize(item, excluded_keys=excluded_keys)
        return normalized
    if isinstance(value, (list, tuple)):
        return [_normalize(item, excluded_keys=excluded_keys) for item in value]
    if isinstance(value, (set, frozenset)):
        items = [_normalize(item, excluded_keys=excluded_keys) for item in value]
        return sorted(items, key=_encode)
    raise CanonicalizationError(
        f"Unsupported canonical value type: {type(value).__name__}"
    )


def _encode(value: JsonValue) -> bytes:
    return json.dumps(
        value,
        allow_nan=False,
        ensure_ascii=False,
        separators=(",", ":"),
        sort_keys=True,
    ).encode("utf-8")
