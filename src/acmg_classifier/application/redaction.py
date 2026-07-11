"""Bounded, privacy-safe values for newly introduced diagnostic reports."""

from __future__ import annotations

import math
import re
from collections.abc import Mapping, Sequence
from urllib.parse import parse_qsl, urlsplit

REDACTED = "[REDACTED]"
TRUNCATED = "[TRUNCATED]"
_BINARY_DATA = "[BINARY_DATA]"

type ReportValue = (
    str | int | float | bool | None | list[ReportValue] | dict[str, ReportValue]
)

_SENSITIVE_FIELD_NAMES = frozenset(
    {
        "accesstoken",
        "apikey",
        "authorization",
        "clientsecret",
        "cookie",
        "credential",
        "credentials",
        "dateofbirth",
        "dob",
        "email",
        "firstname",
        "fullname",
        "idtoken",
        "lastname",
        "medicalrecordnumber",
        "mrn",
        "password",
        "patientid",
        "patientidentifier",
        "patientname",
        "phone",
        "proxyauthorization",
        "refreshtoken",
        "secret",
        "setcookie",
        "ssn",
        "token",
        "xapikey",
    }
)
_CREDENTIAL_ASSIGNMENT = re.compile(
    r"\b(?:api[ _-]?key|access[ _-]?token|auth(?:orization)?|"
    r"credential|password|secret|token)\s*[:=]\s*\S+",
    re.IGNORECASE,
)
_BEARER_TOKEN = re.compile(r"\bbearer\s+\S+", re.IGNORECASE)


class ResourceTooLargeError(ValueError):
    """An untrusted resource exceeds its explicit boundary."""


def enforce_resource_size(
    content: bytes | bytearray | memoryview,
    *,
    max_bytes: int,
) -> None:
    """Fail without echoing untrusted data when a resource exceeds its limit."""
    if max_bytes < 1:
        raise ValueError("max_bytes must be positive")
    if len(content) > max_bytes:
        raise ResourceTooLargeError("untrusted resource exceeds configured size limit")


def redact_for_report(
    value: object,
    *,
    max_depth: int = 8,
    max_items: int = 100,
    max_string_chars: int = 4_096,
) -> ReportValue:
    """Return a JSON-safe report value with secrets, PHI, and excess data removed.

    This boundary intentionally returns placeholders instead of object representations:
    ``repr`` can itself disclose credentials or identifiers.  Limits apply before a
    value can become a diagnostic/logging payload and do not mutate the input.
    """
    if max_depth < 1:
        raise ValueError("max_depth must be positive")
    if max_items < 1:
        raise ValueError("max_items must be positive")
    if max_string_chars < 1:
        raise ValueError("max_string_chars must be positive")
    return _redact(
        value,
        depth=0,
        max_depth=max_depth,
        max_items=max_items,
        max_string_chars=max_string_chars,
    )


def _redact(
    value: object,
    *,
    depth: int,
    max_depth: int,
    max_items: int,
    max_string_chars: int,
) -> ReportValue:
    if depth >= max_depth and (
        isinstance(value, Mapping)
        or (
            isinstance(value, Sequence)
            and not isinstance(value, (bytes, bytearray, str))
        )
    ):
        return TRUNCATED
    if isinstance(value, Mapping):
        result: dict[str, ReportValue] = {}
        for index, (key, nested_value) in enumerate(value.items()):
            if index >= max_items:
                result["_truncated_items"] = TRUNCATED
                break
            field_name = _safe_field_name(key, max_string_chars=max_string_chars)
            result[field_name] = (
                REDACTED
                if _is_sensitive_field(key)
                else _redact(
                    nested_value,
                    depth=depth + 1,
                    max_depth=max_depth,
                    max_items=max_items,
                    max_string_chars=max_string_chars,
                )
            )
        return result
    if isinstance(value, Sequence) and not isinstance(value, (bytes, bytearray, str)):
        items: list[ReportValue] = []
        for index, item in enumerate(value):
            if index >= max_items:
                items.append(TRUNCATED)
                break
            items.append(
                _redact(
                    item,
                    depth=depth + 1,
                    max_depth=max_depth,
                    max_items=max_items,
                    max_string_chars=max_string_chars,
                )
            )
        return items
    if isinstance(value, str):
        return _safe_text(value, max_string_chars=max_string_chars)
    if isinstance(value, (bytes, bytearray, memoryview)):
        return _BINARY_DATA
    if value is None or isinstance(value, (bool, int)):
        return value
    if isinstance(value, float):
        return value if math.isfinite(value) else TRUNCATED
    return TRUNCATED


def _safe_field_name(key: object, *, max_string_chars: int) -> str:
    if not isinstance(key, str):
        return "[NON_STRING_KEY]"
    if _is_sensitive_text(key):
        return REDACTED
    return _truncate(key, max_string_chars=max_string_chars)


def _is_sensitive_field(key: object) -> bool:
    if not isinstance(key, str):
        return False
    normalized = re.sub(r"[^a-z0-9]", "", key.casefold())
    if normalized in _SENSITIVE_FIELD_NAMES:
        return True
    return normalized.startswith(
        ("apikey", "authorization", "credential", "password", "secret")
    ) or normalized.endswith(("apikey", "credential", "password", "secret"))


def _safe_text(value: str, *, max_string_chars: int) -> str:
    bounded_value = value[:max_string_chars]
    if _is_sensitive_text(bounded_value):
        return REDACTED
    if len(value) <= max_string_chars:
        return bounded_value
    return bounded_value + TRUNCATED


def _is_sensitive_text(value: str) -> bool:
    if _CREDENTIAL_ASSIGNMENT.search(value) or _BEARER_TOKEN.search(value):
        return True
    try:
        parsed = urlsplit(value)
    except ValueError:
        return True
    if not parsed.scheme or not parsed.netloc:
        return False
    if "@" in parsed.netloc:
        return True
    return any(_is_sensitive_field(name) for name, _ in parse_qsl(parsed.query))


def _truncate(value: str, *, max_string_chars: int) -> str:
    if len(value) <= max_string_chars:
        return value
    return value[:max_string_chars] + TRUNCATED
