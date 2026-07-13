"""Fail-closed validation for external release evidence metadata.

The validator consumes metadata only. It never fetches an evidence artifact,
opens a report, or handles a private signing key.
"""

from __future__ import annotations

import math
from collections.abc import Mapping, Sequence
from dataclasses import dataclass
from datetime import UTC, date, datetime
from enum import StrEnum
from typing import Any, Literal

from pydantic import (
    AnyHttpUrl,
    BaseModel,
    ConfigDict,
    Field,
    ValidationError,
    field_validator,
)

_CANONICAL_KINDS = (
    "scientific_validation",
    "controlled_catalog",
    "bundle_installation",
    "platform_usability",
    "security_assessment",
    "live_performance",
    "release_owner_approval",
)
_SECRET_KEY_PARTS = (
    "private",
    "secret",
    "credential",
    "password",
    "token",
    "api_key",
    "apikey",
    "access_key",
    "signing_key",
    "bearer",
)


class EvidenceKind(StrEnum):
    """Canonical evidence records required by the release contract."""

    SCIENTIFIC_VALIDATION = "scientific_validation"
    CONTROLLED_CATALOG = "controlled_catalog"
    BUNDLE_INSTALLATION = "bundle_installation"
    PLATFORM_USABILITY = "platform_usability"
    SECURITY_ASSESSMENT = "security_assessment"
    LIVE_PERFORMANCE = "live_performance"
    RELEASE_OWNER_APPROVAL = "release_owner_approval"


class EvidenceStatus(StrEnum):
    """Lifecycle state of an external evidence record."""

    PENDING = "pending"
    ACCEPTED = "accepted"
    REJECTED = "rejected"
    EXPIRED = "expired"


class EvidenceValidationStatus(StrEnum):
    """Stable validator result states."""

    COMPLETE = "complete"
    BLOCKED = "blocked"
    INVALID = "invalid"


class EvidenceReview(BaseModel):
    """Review identity and independence attestation."""

    model_config = ConfigDict(extra="forbid", frozen=True)

    status: Literal["pending", "approved", "rejected"]
    reviewed_by: str | None = None
    reviewed_at: datetime | None = None
    independent: bool

    @field_validator("reviewed_at")
    @classmethod
    def require_aware_timestamp(cls, value: datetime | None) -> datetime | None:
        if value is not None and value.tzinfo is None:
            raise ValueError("reviewed_at must include a timezone")
        return value

    @field_validator("reviewed_by")
    @classmethod
    def require_reviewer_identity(cls, value: str | None) -> str | None:
        if value is not None and not value.strip():
            raise ValueError("reviewed_by must be non-empty when supplied")
        return value


class EvidenceRecord(BaseModel):
    """One content-addressed external evidence record."""

    model_config = ConfigDict(extra="forbid", frozen=True)

    kind: EvidenceKind
    status: EvidenceStatus
    artifact_uri: AnyHttpUrl
    retention_uri: AnyHttpUrl
    artifact_sha256: str
    owner: str
    submitted_at: datetime
    review: EvidenceReview
    details: dict[str, Any]

    @field_validator("artifact_uri", "retention_uri")
    @classmethod
    def require_https(cls, value: AnyHttpUrl) -> AnyHttpUrl:
        if value.scheme != "https":
            raise ValueError("evidence references must use HTTPS")
        return value

    @field_validator("artifact_sha256")
    @classmethod
    def require_sha256(cls, value: str) -> str:
        if len(value) != 64 or any(
            character not in "0123456789abcdef" for character in value
        ):
            raise ValueError("artifact_sha256 must be a lowercase SHA-256 digest")
        return value

    @field_validator("owner")
    @classmethod
    def require_owner(cls, value: str) -> str:
        if not value.strip():
            raise ValueError("owner must be non-empty")
        return value

    @field_validator("submitted_at")
    @classmethod
    def require_aware_submission_time(cls, value: datetime) -> datetime:
        if value.tzinfo is None:
            raise ValueError("submitted_at must include a timezone")
        return value

    @field_validator("details")
    @classmethod
    def reject_secret_like_fields(cls, value: dict[str, Any]) -> dict[str, Any]:
        if _contains_secret_like_field(value):
            raise ValueError("secret-like metadata fields are not allowed")
        return value


class EvidenceManifest(BaseModel):
    """Complete metadata manifest for one candidate release."""

    model_config = ConfigDict(extra="forbid", frozen=True)

    schema_version: Literal["1.0"]
    release_id: str
    candidate_bundle_version: str
    matrix_sha256: str
    records: tuple[EvidenceRecord, ...] = Field(min_length=1)

    @field_validator("release_id")
    @classmethod
    def require_release_id(cls, value: str) -> str:
        allowed = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789._-"
        if not value or any(character not in allowed for character in value):
            raise ValueError(
                "release_id must contain only letters, digits, '.', '_' or '-'"
            )
        return value

    @field_validator("candidate_bundle_version")
    @classmethod
    def require_bundle_version(cls, value: str) -> str:
        if not _is_numeric_version(value):
            raise ValueError("candidate_bundle_version must be numeric")
        return value

    @field_validator("matrix_sha256")
    @classmethod
    def require_matrix_sha256(cls, value: str) -> str:
        if len(value) != 64 or any(
            character not in "0123456789abcdef" for character in value
        ):
            raise ValueError("matrix_sha256 must be a lowercase SHA-256 digest")
        return value


@dataclass(frozen=True, slots=True)
class EvidenceValidationResult:
    """Safe, serializable result from metadata-only validation."""

    status: EvidenceValidationStatus
    blockers: tuple[str, ...]
    manifest: EvidenceManifest | None
    checked_at: str

    def as_dict(self) -> dict[str, Any]:
        return {
            "status": self.status.value,
            "release_id": self.manifest.release_id if self.manifest else None,
            "candidate_bundle_version": (
                self.manifest.candidate_bundle_version if self.manifest else None
            ),
            "blockers": list(self.blockers),
            "checked_at": self.checked_at,
        }


def validate_evidence_manifest(value: Mapping[str, Any]) -> EvidenceValidationResult:
    """Validate an evidence manifest without reading or fetching its artifacts."""
    checked_at = datetime.now(UTC).isoformat().replace("+00:00", "Z")
    if not isinstance(value, Mapping):
        return EvidenceValidationResult(
            EvidenceValidationStatus.INVALID,
            ("evidence manifest must be an object",),
            None,
            checked_at,
        )
    try:
        manifest = EvidenceManifest.model_validate(value)
    except ValidationError as error:
        return EvidenceValidationResult(
            EvidenceValidationStatus.INVALID,
            _safe_validation_messages(error),
            None,
            checked_at,
        )

    blockers: list[str] = []
    if _is_placeholder_sha256(manifest.matrix_sha256):
        blockers.append("matrix_sha256 is a placeholder digest")
    records_by_kind: dict[str, EvidenceRecord] = {}
    for record in manifest.records:
        kind = record.kind.value
        if kind in records_by_kind:
            blockers.append(f"duplicate evidence kind: {kind}")
        records_by_kind[kind] = record
    if len(records_by_kind) != len(manifest.records):
        return EvidenceValidationResult(
            EvidenceValidationStatus.INVALID,
            tuple(blockers),
            manifest,
            checked_at,
        )

    missing = [kind for kind in _CANONICAL_KINDS if kind not in records_by_kind]
    blockers.extend(f"missing evidence kind: {kind}" for kind in missing)
    for kind in _CANONICAL_KINDS:
        candidate_record = records_by_kind.get(kind)
        if candidate_record is None:
            continue
        blockers.extend(
            _record_blockers(candidate_record, manifest.candidate_bundle_version)
        )

    has_unknown_detail = any(
        " detail is unknown: " in blocker for blocker in blockers
    )
    status = (
        EvidenceValidationStatus.INVALID
        if has_unknown_detail
        else (
            EvidenceValidationStatus.COMPLETE
            if not blockers
            else EvidenceValidationStatus.BLOCKED
        )
    )
    return EvidenceValidationResult(
        status,
        tuple(dict.fromkeys(blockers)),
        manifest,
        checked_at,
    )


def canonical_evidence_kinds() -> tuple[str, ...]:
    """Return the stable evidence-kind vocabulary."""
    return _CANONICAL_KINDS


def _record_blockers(record: EvidenceRecord, candidate_version: str) -> tuple[str, ...]:
    blockers: list[str] = []
    kind = record.kind.value
    if record.status is not EvidenceStatus.ACCEPTED:
        blockers.append(f"{kind} record is not accepted")
    if record.review.status != "approved":
        blockers.append(f"{kind} record has no approved review")
    if record.review.reviewed_by is None or record.review.reviewed_at is None:
        blockers.append(f"{kind} record review identity is incomplete")
    if (
        kind != EvidenceKind.RELEASE_OWNER_APPROVAL.value
        and not record.review.independent
    ):
        blockers.append(f"{kind} record lacks an independent review")
    if (
        record.status is EvidenceStatus.ACCEPTED
        and _is_placeholder_sha256(record.artifact_sha256)
    ):
        blockers.append(f"{kind} artifact digest is a placeholder")
    blockers.extend(_detail_blockers(kind, record.details, candidate_version))
    return tuple(blockers)


def _detail_blockers(
    kind: str, details: Mapping[str, Any], candidate_version: str
) -> tuple[str, ...]:
    requirements: dict[str, tuple[str, ...]] = {
        EvidenceKind.SCIENTIFIC_VALIDATION.value: (
            "dataset_id",
            "provenance_uris",
            "redistribution_permission",
            "evaluation_protocol",
            "result_summary",
            "independent_review",
            "synthetic_only",
        ),
        EvidenceKind.CONTROLLED_CATALOG.value: (
            "key_id",
            "algorithm",
            "public_key_sha256",
            "catalog_sha256",
            "archive_sha256",
            "manifest_sha256",
            "manifest_verified",
            "signature_verified",
            "published_uri",
            "revocation_uri",
            "retention_uri",
        ),
        EvidenceKind.BUNDLE_INSTALLATION.value: (
            "bundle_version",
            "platform",
            "python_version",
            "clean_install",
            "first_use_seconds",
            "installed_manifest_sha256",
            "cache_sha256",
            "runtime_smoke_status",
        ),
        EvidenceKind.PLATFORM_USABILITY.value: (
            "platforms",
            "python_versions",
            "clean_install_report_uri",
            "participant_record_uri",
            "participant_count",
            "first_use_max_seconds",
            "report_sha256",
            "automation_only",
        ),
        EvidenceKind.SECURITY_ASSESSMENT.value: (
            "assessor",
            "assessor_independent",
            "standard",
            "issued_at",
            "valid_until",
            "report_sha256",
            "findings_status",
            "scope",
        ),
        EvidenceKind.LIVE_PERFORMANCE.value: (
            "workload_id",
            "synthetic",
            "platform",
            "python_version",
            "sample_count",
            "p95_ms",
            "p99_ms",
            "collection_period",
            "report_uri",
        ),
        EvidenceKind.RELEASE_OWNER_APPROVAL.value: (
            "release_version",
            "compatibility_window_end",
            "decision_id",
            "bound_candidate_bundle_version",
            "candidate_bundle_sha256",
            "evidence_manifest_sha256",
            "approved_at",
            "approved",
        ),
    }
    allowed_keys = set(requirements[kind])
    unknown = sorted(set(details) - allowed_keys)
    blockers = [f"{kind} detail is unknown: {key}" for key in unknown]
    missing = [key for key in requirements[kind] if key not in details]
    blockers.extend(f"{kind} detail is missing: {key}" for key in missing)
    if unknown or missing:
        return tuple(blockers)
    text_fields = {
        EvidenceKind.SCIENTIFIC_VALIDATION.value: (
            "dataset_id",
            "redistribution_permission",
            "evaluation_protocol",
            "result_summary",
        ),
        EvidenceKind.CONTROLLED_CATALOG.value: ("key_id", "algorithm"),
        EvidenceKind.BUNDLE_INSTALLATION.value: (
            "bundle_version",
            "platform",
            "python_version",
            "runtime_smoke_status",
        ),
        EvidenceKind.PLATFORM_USABILITY.value: (
            "clean_install_report_uri",
            "participant_record_uri",
        ),
        EvidenceKind.SECURITY_ASSESSMENT.value: (
            "assessor",
            "standard",
            "issued_at",
            "valid_until",
            "findings_status",
            "scope",
        ),
        EvidenceKind.LIVE_PERFORMANCE.value: (
            "workload_id",
            "platform",
            "python_version",
            "collection_period",
        ),
        EvidenceKind.RELEASE_OWNER_APPROVAL.value: (
            "release_version",
            "compatibility_window_end",
            "decision_id",
            "bound_candidate_bundle_version",
            "approved_at",
        ),
    }
    for key in text_fields.get(kind, ()):
        if not _is_text(details[key]):
            blockers.append(f"{kind} detail must be non-empty text: {key}")

    if kind == EvidenceKind.SCIENTIFIC_VALIDATION.value:
        if not _is_https_list(details["provenance_uris"]):
            blockers.append("scientific_validation provenance must use HTTPS")
        if details["independent_review"] is not True:
            blockers.append("scientific_validation lacks independent review")
        if details["synthetic_only"] is not False:
            blockers.append("scientific_validation is synthetic-only")
    elif kind == EvidenceKind.CONTROLLED_CATALOG.value:
        if details["algorithm"] != "Ed25519":
            blockers.append("controlled_catalog must use Ed25519")
        for key in (
            "public_key_sha256",
            "catalog_sha256",
            "archive_sha256",
            "manifest_sha256",
        ):
            if not _is_nonplaceholder_sha256(details[key]):
                blockers.append(f"controlled_catalog {key} is not a SHA-256 digest")
        if details["manifest_verified"] is not True:
            blockers.append("controlled_catalog manifest is not verified")
        for key in ("published_uri", "revocation_uri", "retention_uri"):
            if not _is_https(details[key]):
                blockers.append(f"controlled_catalog {key} must use HTTPS")
    elif kind == EvidenceKind.BUNDLE_INSTALLATION.value:
        if details["bundle_version"] != candidate_version:
            blockers.append("bundle_installation version does not match candidate")
        if details["clean_install"] is not True:
            blockers.append("bundle_installation clean install is not recorded")
        first_use_seconds = details["first_use_seconds"]
        if not _is_nonnegative_number(first_use_seconds):
            blockers.append("bundle_installation first-use duration is invalid")
        elif first_use_seconds > 300:
            blockers.append("bundle_installation first-use exceeds five minutes")
        for key in ("installed_manifest_sha256", "cache_sha256"):
            if not _is_nonplaceholder_sha256(details[key]):
                blockers.append(f"bundle_installation {key} is not a SHA-256 digest")
        if details["runtime_smoke_status"] != "passed":
            blockers.append("bundle_installation runtime smoke did not pass")
    elif kind == EvidenceKind.PLATFORM_USABILITY.value:
        platforms = details["platforms"]
        if not _is_string_list(platforms):
            blockers.append("platform_usability platforms must be a list")
        else:
            lowered = {item.lower() for item in platforms}
            if not any("linux" in item or "ubuntu" in item for item in lowered):
                blockers.append("platform_usability lacks Linux evidence")
            if not any("windows" in item for item in lowered):
                blockers.append("platform_usability lacks Windows evidence")
        if not _is_string_list(details["python_versions"]):
            blockers.append("platform_usability Python versions must be a list")
        if not _is_nonplaceholder_sha256(details["report_sha256"]):
            blockers.append("platform_usability report digest is invalid")
        for key in ("clean_install_report_uri", "participant_record_uri"):
            if not _is_https(details[key]):
                blockers.append(f"platform_usability {key} must use HTTPS")
        if not _is_positive_int(details["participant_count"]):
            blockers.append("platform_usability participant count must be positive")
        first_use_max = details["first_use_max_seconds"]
        if not _is_nonnegative_number(first_use_max):
            blockers.append("platform_usability first-use duration is invalid")
        elif first_use_max > 300:
            blockers.append("platform_usability first-use exceeds five minutes")
        if details["automation_only"] is not False:
            blockers.append("platform_usability cannot be automation-only")
    elif kind == EvidenceKind.SECURITY_ASSESSMENT.value:
        if details["assessor_independent"] is not True:
            blockers.append("security_assessment lacks an independent assessor")
        if not _is_nonplaceholder_sha256(details["report_sha256"]):
            blockers.append("security_assessment report digest is invalid")
        issued_at = _parse_date_or_datetime(details["issued_at"])
        if issued_at is None:
            blockers.append("security_assessment issue date is invalid")
        valid_until = _parse_date_or_datetime(details["valid_until"])
        if valid_until is None:
            blockers.append("security_assessment validity date is invalid")
        elif valid_until.date() < datetime.now(UTC).date():
            blockers.append("security_assessment report is expired")
        if details["findings_status"] not in {"resolved", "accepted_risk"}:
            blockers.append("security_assessment findings are not dispositioned")
    elif kind == EvidenceKind.LIVE_PERFORMANCE.value:
        if details["synthetic"] is not False:
            blockers.append("live_performance is synthetic-only")
        if not _is_positive_int(details["sample_count"]):
            blockers.append("live_performance sample count must be positive")
        p95 = details["p95_ms"]
        p99 = details["p99_ms"]
        if not _is_nonnegative_number(p95) or not _is_nonnegative_number(p99):
            blockers.append("live_performance latency values are invalid")
        elif p99 < p95:
            blockers.append("live_performance p99 must not be below p95")
        if not _is_https(details["report_uri"]):
            blockers.append("live_performance report must use HTTPS")
    elif kind == EvidenceKind.RELEASE_OWNER_APPROVAL.value:
        if details["release_version"] != candidate_version:
            blockers.append("release_owner_approval version does not match candidate")
        if details["bound_candidate_bundle_version"] != candidate_version:
            blockers.append("release_owner_approval is not bound to candidate")
        for key in ("candidate_bundle_sha256", "evidence_manifest_sha256"):
            if not _is_nonplaceholder_sha256(details[key]):
                blockers.append(f"release_owner_approval {key} is not a SHA-256 digest")
        if details["approved"] is not True:
            blockers.append("release_owner_approval is not approved")
        approved_at = _parse_date_or_datetime(details["approved_at"])
        if approved_at is None:
            blockers.append("release_owner_approval timestamp is invalid")
        window_end = _parse_date_or_datetime(details["compatibility_window_end"])
        if window_end is None:
            blockers.append("release_owner_approval compatibility window is invalid")
        elif window_end.date() <= datetime.now(UTC).date():
            blockers.append("release_owner_approval compatibility window has ended")
    return tuple(blockers)


def _contains_secret_like_field(value: Any) -> bool:
    if isinstance(value, Mapping):
        for key, nested in value.items():
            lowered = str(key).lower()
            if any(part in lowered for part in _SECRET_KEY_PARTS):
                return True
            if _contains_secret_like_field(nested):
                return True
    elif isinstance(value, Sequence) and not isinstance(value, (str, bytes, bytearray)):
        return any(_contains_secret_like_field(item) for item in value)
    return False


def _is_numeric_version(value: Any) -> bool:
    if not isinstance(value, str):
        return False
    parts = value.split(".")
    return 1 < len(parts) <= 3 and all(part.isdigit() for part in parts)


def _is_sha256(value: Any) -> bool:
    return isinstance(value, str) and len(value) == 64 and all(
        character in "0123456789abcdef" for character in value
    )


def _is_placeholder_sha256(value: Any) -> bool:
    return _is_sha256(value) and value == "0" * 64


def _is_nonplaceholder_sha256(value: Any) -> bool:
    return _is_sha256(value) and not _is_placeholder_sha256(value)


def _is_https(value: Any) -> bool:
    return isinstance(value, str) and value.startswith("https://") and len(value) > 8


def _is_https_list(value: Any) -> bool:
    return _is_string_list(value) and all(_is_https(item) for item in value)




def _is_text(value: Any) -> bool:
    return isinstance(value, str) and bool(value.strip())


def _is_string_list(value: Any) -> bool:
    return isinstance(value, list) and bool(value) and all(
        isinstance(item, str) and bool(item.strip()) for item in value
    )


def _is_positive_int(value: Any) -> bool:
    return isinstance(value, int) and not isinstance(value, bool) and value > 0


def _is_nonnegative_number(value: Any) -> bool:
    return (
        isinstance(value, (int, float))
        and not isinstance(value, bool)
        and math.isfinite(float(value))
        and value >= 0
    )


def _parse_date_or_datetime(value: Any) -> datetime | None:
    if not isinstance(value, str):
        return None
    try:
        parsed = datetime.fromisoformat(value.replace("Z", "+00:00"))
    except ValueError:
        try:
            parsed_date = date.fromisoformat(value)
        except ValueError:
            return None
        return datetime.combine(parsed_date, datetime.min.time(), tzinfo=UTC)
    if parsed.tzinfo is None:
        return parsed.replace(tzinfo=UTC)
    return parsed.astimezone(UTC)


def _safe_validation_messages(error: ValidationError) -> tuple[str, ...]:
    messages: list[str] = []
    for item in error.errors():
        location = ".".join(str(part) for part in item.get("loc", ())) or "manifest"
        message = str(item.get("msg", "invalid value"))
        messages.append(f"{location}: {message}")
    return tuple(dict.fromkeys(messages)) or ("evidence manifest is invalid",)
