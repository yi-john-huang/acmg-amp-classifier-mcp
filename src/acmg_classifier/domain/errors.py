"""Stable application errors without transport concerns."""

from collections.abc import Mapping
from enum import StrEnum

type JsonValue = (
    None | bool | int | float | str | list["JsonValue"] | dict[str, "JsonValue"]
)


class ErrorCode(StrEnum):
    """Machine-readable error codes."""

    SOURCE_RATE_LIMITED = "SOURCE_RATE_LIMITED"


class ApplicationError(Exception):
    """An expected error that can safely cross an application boundary."""

    def __init__(
        self,
        *,
        code: ErrorCode,
        message: str,
        retryable: bool = False,
        source: str | None = None,
        field: str | None = None,
        details: Mapping[str, JsonValue] | None = None,
        next_action: str | None = None,
    ) -> None:
        super().__init__(message)
        self.code = code
        self.message = message
        self.retryable = retryable
        self.source = source
        self.field = field
        self.details = dict(details or {})
        self.next_action = next_action
