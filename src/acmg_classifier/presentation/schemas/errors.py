"""Public error response schemas."""

from typing import Self

from pydantic import BaseModel, ConfigDict

from acmg_classifier.domain.errors import ApplicationError, ErrorCode, JsonValue
from acmg_classifier.domain.identifiers import RequestId


class ErrorBody(BaseModel):
    """Machine-readable details for an expected application error."""

    model_config = ConfigDict(extra="forbid", frozen=True)

    code: ErrorCode
    message: str
    retryable: bool
    source: str | None
    field: str | None
    details: dict[str, JsonValue]
    next_action: str | None
    request_id: RequestId


class ErrorEnvelope(BaseModel):
    """Stable public wrapper for expected errors."""

    model_config = ConfigDict(extra="forbid", frozen=True)

    error: ErrorBody

    @classmethod
    def from_error(cls, error: ApplicationError, *, request_id: RequestId) -> Self:
        """Convert an application error without leaking exception internals."""
        return cls(
            error=ErrorBody(
                code=error.code,
                message=error.message,
                retryable=error.retryable,
                source=error.source,
                field=error.field,
                details=error.details,
                next_action=error.next_action,
                request_id=request_id,
            )
        )
