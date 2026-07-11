"""Strict, provenance-aware ingestion for golden validation fixtures.

This module deliberately accepts a small JSON manifest vocabulary.  A fixture is not
an independently curated scientific dataset merely because it can be executed;
missing provenance is represented explicitly and blocks release eligibility.
"""

from __future__ import annotations

import json
import re
from collections.abc import Mapping, Sequence
from dataclasses import dataclass
from datetime import datetime
from pathlib import Path
from types import MappingProxyType
from typing import cast

from acmg_classifier.domain.enums import ClassificationTier, CriterionStatus
from acmg_classifier.domain.rules import CriterionCode


class GoldenFixtureError(ValueError):
    """Raised when an untrusted golden fixture is incomplete or ambiguous."""


_APPROVAL_STATUSES = frozenset({"approved", "missing"})
_DATASET_KINDS = frozenset({"independently_curated_clinical", "synthetic_smoke"})
_SOURCE_CLASSIFICATION_FIELDS = frozenset(
    {"classification", "clinicalsignificance", "germlineclassification"}
)
_SHA256 = re.compile(r"^[0-9a-f]{64}$")


def _require_mapping(value: object, location: str) -> Mapping[str, object]:
    if not isinstance(value, Mapping):
        raise GoldenFixtureError(f"{location} must be an object")
    return cast(Mapping[str, object], value)


def _require_list(value: object, location: str) -> Sequence[object]:
    if not isinstance(value, list):
        raise GoldenFixtureError(f"{location} must be an array")
    return value


def _require_exact_keys(
    value: Mapping[str, object], *, required: frozenset[str], location: str
) -> None:
    unknown = set(value) - required
    missing = required - set(value)
    if unknown:
        raise GoldenFixtureError(
            f"{location} has unknown field(s): {', '.join(sorted(unknown))}"
        )
    if missing:
        raise GoldenFixtureError(
            f"{location} is missing required field(s): {', '.join(sorted(missing))}"
        )


def _require_nonempty_text(value: object, location: str) -> str:
    if not isinstance(value, str) or not value.strip():
        raise GoldenFixtureError(f"{location} must be non-empty text")
    return value.strip()


def _require_nullable_text(value: object, location: str) -> str | None:
    if value is None:
        return None
    return _require_nonempty_text(value, location)


def _require_utc_timestamp(value: object, location: str) -> str:
    text = _require_nonempty_text(value, location)
    try:
        parsed = datetime.fromisoformat(text.replace("Z", "+00:00"))
    except ValueError as error:
        raise GoldenFixtureError(f"{location} must be an ISO-8601 timestamp") from error
    if parsed.tzinfo is None or parsed.utcoffset() is None:
        raise GoldenFixtureError(f"{location} must include a timezone")
    return text


def _freeze_json(value: object, location: str) -> object:
    if value is None or isinstance(value, (bool, int, float, str)):
        return value
    if isinstance(value, Mapping):
        frozen: dict[str, object] = {}
        for key, child in value.items():
            if not isinstance(key, str):
                raise GoldenFixtureError(f"{location} object keys must be text")
            frozen[key] = _freeze_json(child, f"{location}.{key}")
        return MappingProxyType(frozen)
    if isinstance(value, list):
        return tuple(
            _freeze_json(child, f"{location}[{index}]")
            for index, child in enumerate(value)
        )
    raise GoldenFixtureError(f"{location} must contain JSON-compatible values")


def _source_field_requires_mask(field: str) -> bool:
    normalized = re.sub(r"[^a-z0-9]", "", field.casefold())
    return normalized in _SOURCE_CLASSIFICATION_FIELDS or "classification" in normalized


@dataclass(frozen=True, slots=True)
class SourceArtifact:
    """Pinned provenance for an independently curated fixture source."""

    artifact_id: str
    release: str
    url: str
    retrieved_at: str
    sha256: str
    transformation_version: str

    @classmethod
    def from_mapping(cls, value: object, location: str) -> SourceArtifact:
        data = _require_mapping(value, location)
        required = frozenset(
            {
                "artifact_id",
                "release",
                "url",
                "retrieved_at",
                "sha256",
                "transformation_version",
            }
        )
        _require_exact_keys(data, required=required, location=location)
        url = _require_nonempty_text(data["url"], f"{location}.url")
        if not url.startswith("https://"):
            raise GoldenFixtureError(f"{location}.url must use HTTPS")
        sha256 = _require_nonempty_text(data["sha256"], f"{location}.sha256")
        if not _SHA256.fullmatch(sha256):
            raise GoldenFixtureError(f"{location}.sha256 must be a lowercase SHA-256")
        return cls(
            artifact_id=_require_nonempty_text(
                data["artifact_id"], f"{location}.artifact_id"
            ),
            release=_require_nonempty_text(data["release"], f"{location}.release"),
            url=url,
            retrieved_at=_require_utc_timestamp(
                data["retrieved_at"], f"{location}.retrieved_at"
            ),
            sha256=sha256,
            transformation_version=_require_nonempty_text(
                data["transformation_version"], f"{location}.transformation_version"
            ),
        )

    def to_dict(self) -> dict[str, str]:
        return {
            "artifact_id": self.artifact_id,
            "release": self.release,
            "url": self.url,
            "retrieved_at": self.retrieved_at,
            "sha256": self.sha256,
            "transformation_version": self.transformation_version,
        }


@dataclass(frozen=True, slots=True)
class DatasetProvenance:
    """Approval and source traceability required for scientific validation."""

    approval_status: str
    approval_id: str | None
    protocol_id: str | None
    protocol_version: str | None
    approved_by: str | None
    approved_at: str | None
    license: str | None
    independent_label_basis: str | None
    source_artifacts: tuple[SourceArtifact, ...]
    missing_prerequisite: str | None

    @property
    def is_approved(self) -> bool:
        return self.approval_status == "approved"

    @classmethod
    def from_mapping(cls, value: object) -> DatasetProvenance:
        data = _require_mapping(value, "provenance")
        required = frozenset(
            {
                "approval_status",
                "approval_id",
                "protocol_id",
                "protocol_version",
                "approved_by",
                "approved_at",
                "license",
                "independent_label_basis",
                "source_artifacts",
                "missing_prerequisite",
            }
        )
        _require_exact_keys(data, required=required, location="provenance")
        approval_status = _require_nonempty_text(
            data["approval_status"], "provenance.approval_status"
        )
        if approval_status not in _APPROVAL_STATUSES:
            raise GoldenFixtureError(
                "provenance.approval_status must be approved or missing"
            )
        artifacts = tuple(
            SourceArtifact.from_mapping(item, f"provenance.source_artifacts[{index}]")
            for index, item in enumerate(
                _require_list(data["source_artifacts"], "provenance.source_artifacts")
            )
        )
        provenance = cls(
            approval_status=approval_status,
            approval_id=_require_nullable_text(
                data["approval_id"], "provenance.approval_id"
            ),
            protocol_id=_require_nullable_text(
                data["protocol_id"], "provenance.protocol_id"
            ),
            protocol_version=_require_nullable_text(
                data["protocol_version"], "provenance.protocol_version"
            ),
            approved_by=_require_nullable_text(
                data["approved_by"], "provenance.approved_by"
            ),
            approved_at=(
                _require_utc_timestamp(data["approved_at"], "provenance.approved_at")
                if data["approved_at"] is not None
                else None
            ),
            license=_require_nullable_text(data["license"], "provenance.license"),
            independent_label_basis=_require_nullable_text(
                data["independent_label_basis"], "provenance.independent_label_basis"
            ),
            source_artifacts=artifacts,
            missing_prerequisite=_require_nullable_text(
                data["missing_prerequisite"], "provenance.missing_prerequisite"
            ),
        )
        if provenance.is_approved:
            required_values = {
                "approval_id": provenance.approval_id,
                "protocol_id": provenance.protocol_id,
                "protocol_version": provenance.protocol_version,
                "approved_by": provenance.approved_by,
                "approved_at": provenance.approved_at,
                "license": provenance.license,
                "independent_label_basis": provenance.independent_label_basis,
            }
            missing = [name for name, item in required_values.items() if item is None]
            if missing:
                raise GoldenFixtureError(
                    "approved provenance requires " + ", ".join(missing)
                )
            if not provenance.source_artifacts:
                raise GoldenFixtureError(
                    "approved provenance requires source_artifacts"
                )
            if provenance.missing_prerequisite is not None:
                raise GoldenFixtureError(
                    "approved provenance must not name a missing_prerequisite"
                )
        else:
            if provenance.missing_prerequisite is None:
                raise GoldenFixtureError(
                    "missing provenance requires missing_prerequisite"
                )
            if provenance.source_artifacts:
                raise GoldenFixtureError(
                    "missing provenance must not claim source_artifacts"
                )
        return provenance

    def to_dict(self) -> dict[str, object]:
        return {
            "approval_status": self.approval_status,
            "approval_id": self.approval_id,
            "protocol_id": self.protocol_id,
            "protocol_version": self.protocol_version,
            "approved_by": self.approved_by,
            "approved_at": self.approved_at,
            "license": self.license,
            "independent_label_basis": self.independent_label_basis,
            "source_artifacts": [
                artifact.to_dict() for artifact in self.source_artifacts
            ],
            "missing_prerequisite": self.missing_prerequisite,
        }


@dataclass(frozen=True, slots=True)
class Exclusion:
    """A disclosed case omission; it is never silently removed from a report."""

    case_id: str
    reason: str

    @classmethod
    def from_mapping(cls, value: object, location: str) -> Exclusion:
        data = _require_mapping(value, location)
        required = frozenset({"case_id", "reason"})
        _require_exact_keys(data, required=required, location=location)
        return cls(
            case_id=_require_nonempty_text(data["case_id"], f"{location}.case_id"),
            reason=_require_nonempty_text(data["reason"], f"{location}.reason"),
        )

    def to_dict(self) -> dict[str, str]:
        return {"case_id": self.case_id, "reason": self.reason}


@dataclass(frozen=True, slots=True)
class SourceObservation:
    """Fixture source data, retained only after configured label masking."""

    source_id: str
    fields: Mapping[str, object]

    @classmethod
    def from_mapping(cls, value: object, location: str) -> SourceObservation:
        data = _require_mapping(value, location)
        required = frozenset({"source_id", "fields"})
        _require_exact_keys(data, required=required, location=location)
        fields = _require_mapping(data["fields"], f"{location}.fields")
        return cls(
            source_id=_require_nonempty_text(
                data["source_id"], f"{location}.source_id"
            ),
            fields=cast(
                Mapping[str, object], _freeze_json(fields, f"{location}.fields")
            ),
        )


@dataclass(frozen=True, slots=True)
class MaskedSourceField:
    """One direct source-label field that must not reach the classifier."""

    source_id: str
    field: str

    @classmethod
    def from_mapping(cls, value: object, location: str) -> MaskedSourceField:
        data = _require_mapping(value, location)
        required = frozenset({"source_id", "field"})
        _require_exact_keys(data, required=required, location=location)
        return cls(
            source_id=_require_nonempty_text(
                data["source_id"], f"{location}.source_id"
            ),
            field=_require_nonempty_text(data["field"], f"{location}.field"),
        )

    def to_dict(self) -> dict[str, str]:
        return {"source_id": self.source_id, "field": self.field}


@dataclass(frozen=True, slots=True)
class GoldenCaseRequest:
    """Safe, source-label-masked request handed to a real application client."""

    variant: str
    context: Mapping[str, object]
    source_observations: tuple[SourceObservation, ...]


@dataclass(frozen=True, slots=True)
class GoldenCaseExpectation:
    """Independent labels used solely for deterministic comparison."""

    classification: ClassificationTier | None
    criterion_statuses: Mapping[str, CriterionStatus]
    normalization_failure: bool


@dataclass(frozen=True, slots=True)
class GoldenValidationCase:
    """One provenance-bound case and its explicit source-label masking policy."""

    case_id: str
    request: GoldenCaseRequest
    masked_source_fields: tuple[MaskedSourceField, ...]
    expected: GoldenCaseExpectation

    @classmethod
    def from_mapping(cls, value: object, location: str) -> GoldenValidationCase:
        data = _require_mapping(value, location)
        required = frozenset({"case_id", "request", "masked_source_fields", "expected"})
        _require_exact_keys(data, required=required, location=location)
        request_data = _require_mapping(data["request"], f"{location}.request")
        _require_exact_keys(
            request_data,
            required=frozenset({"variant", "context", "source_observations"}),
            location=f"{location}.request",
        )
        observations = tuple(
            SourceObservation.from_mapping(
                item, f"{location}.request.source_observations[{index}]"
            )
            for index, item in enumerate(
                _require_list(
                    request_data["source_observations"],
                    f"{location}.request.source_observations",
                )
            )
        )
        source_ids = [observation.source_id for observation in observations]
        if len(set(source_ids)) != len(source_ids):
            raise GoldenFixtureError(f"{location}.request has duplicate source IDs")
        masked_fields = tuple(
            MaskedSourceField.from_mapping(
                item, f"{location}.masked_source_fields[{index}]"
            )
            for index, item in enumerate(
                _require_list(
                    data["masked_source_fields"], f"{location}.masked_source_fields"
                )
            )
        )
        mask_pairs = {(item.source_id, item.field) for item in masked_fields}
        if len(mask_pairs) != len(masked_fields):
            raise GoldenFixtureError(
                f"{location}.masked_source_fields has duplicate entries"
            )
        observed_fields = {
            (observation.source_id, field)
            for observation in observations
            for field in observation.fields
        }
        unknown_masks = mask_pairs - observed_fields
        if unknown_masks:
            source_id, field = sorted(unknown_masks)[0]
            raise GoldenFixtureError(
                f"{location}.masked_source_fields references absent field "
                f"{source_id}.{field}"
            )
        for source_id, field in observed_fields:
            if (
                _source_field_requires_mask(field)
                and (source_id, field) not in mask_pairs
            ):
                raise GoldenFixtureError(
                    f"{location} source classification field {source_id}.{field} "
                    "must be masked"
                )
        expected_data = _require_mapping(data["expected"], f"{location}.expected")
        _require_exact_keys(
            expected_data,
            required=frozenset({"classification", "criteria", "normalization_failure"}),
            location=f"{location}.expected",
        )
        classification_value = expected_data["classification"]
        if classification_value is None:
            classification = None
        else:
            try:
                classification = ClassificationTier(
                    _require_nonempty_text(
                        classification_value, f"{location}.expected.classification"
                    )
                )
            except ValueError as error:
                raise GoldenFixtureError(
                    f"{location}.expected.classification is not a five-tier value"
                ) from error
        criteria_data = _require_mapping(
            expected_data["criteria"], f"{location}.expected.criteria"
        )
        criterion_statuses: dict[str, CriterionStatus] = {}
        for code, status_value in criteria_data.items():
            try:
                criterion_code = CriterionCode(code)
                criterion_statuses[criterion_code.value] = CriterionStatus(
                    _require_nonempty_text(
                        status_value, f"{location}.expected.criteria.{code}"
                    )
                )
            except ValueError as error:
                raise GoldenFixtureError(
                    f"{location}.expected.criteria.{code} is invalid"
                ) from error
        normalization_failure = expected_data["normalization_failure"]
        if not isinstance(normalization_failure, bool):
            raise GoldenFixtureError(
                f"{location}.expected.normalization_failure must be boolean"
            )
        if normalization_failure and classification is not None:
            raise GoldenFixtureError(
                f"{location} normalization-failure expectation must not have a "
                "classification"
            )
        return cls(
            case_id=_require_nonempty_text(data["case_id"], f"{location}.case_id"),
            request=GoldenCaseRequest(
                variant=_require_nonempty_text(
                    request_data["variant"], f"{location}.request.variant"
                ),
                context=cast(
                    Mapping[str, object],
                    _freeze_json(
                        _require_mapping(
                            request_data["context"], f"{location}.request.context"
                        ),
                        f"{location}.request.context",
                    ),
                ),
                source_observations=observations,
            ),
            masked_source_fields=tuple(
                sorted(masked_fields, key=lambda item: (item.source_id, item.field))
            ),
            expected=GoldenCaseExpectation(
                classification=classification,
                criterion_statuses=MappingProxyType(
                    dict(sorted(criterion_statuses.items()))
                ),
                normalization_failure=normalization_failure,
            ),
        )

    def masked_request(self) -> GoldenCaseRequest:
        """Return a copy with direct source classifications removed before execution."""
        masks = {(mask.source_id, mask.field) for mask in self.masked_source_fields}
        observations = tuple(
            SourceObservation(
                source_id=observation.source_id,
                fields=MappingProxyType(
                    {
                        field: value
                        for field, value in observation.fields.items()
                        if (observation.source_id, field) not in masks
                    }
                ),
            )
            for observation in self.request.source_observations
        )
        return GoldenCaseRequest(
            variant=self.request.variant,
            context=self.request.context,
            source_observations=observations,
        )


@dataclass(frozen=True, slots=True)
class GoldenValidationManifest:
    """The complete, strict fixture contract for one validation dataset."""

    schema_version: int
    dataset_id: str
    dataset_version: str
    dataset_kind: str
    provenance: DatasetProvenance
    exclusions: tuple[Exclusion, ...]
    cases: tuple[GoldenValidationCase, ...]

    @classmethod
    def from_mapping(cls, value: object) -> GoldenValidationManifest:
        data = _require_mapping(value, "manifest")
        required = frozenset(
            {
                "schema_version",
                "dataset_id",
                "dataset_version",
                "dataset_kind",
                "provenance",
                "exclusions",
                "cases",
            }
        )
        _require_exact_keys(data, required=required, location="manifest")
        schema_version = data["schema_version"]
        if schema_version != 1:
            raise GoldenFixtureError("manifest.schema_version must be 1")
        dataset_kind = _require_nonempty_text(
            data["dataset_kind"], "manifest.dataset_kind"
        )
        if dataset_kind not in _DATASET_KINDS:
            raise GoldenFixtureError("manifest.dataset_kind is not supported")
        provenance = DatasetProvenance.from_mapping(data["provenance"])
        if (
            dataset_kind == "independently_curated_clinical"
            and not provenance.is_approved
        ):
            raise GoldenFixtureError(
                "independently curated clinical fixtures require approved provenance"
            )
        if dataset_kind == "synthetic_smoke" and provenance.is_approved:
            raise GoldenFixtureError(
                "synthetic smoke fixtures must not claim approved provenance"
            )
        exclusions = tuple(
            Exclusion.from_mapping(item, f"manifest.exclusions[{index}]")
            for index, item in enumerate(
                _require_list(data["exclusions"], "manifest.exclusions")
            )
        )
        cases = tuple(
            GoldenValidationCase.from_mapping(item, f"manifest.cases[{index}]")
            for index, item in enumerate(_require_list(data["cases"], "manifest.cases"))
        )
        if not cases:
            raise GoldenFixtureError("manifest.cases must not be empty")
        case_ids = [case.case_id for case in cases]
        if len(set(case_ids)) != len(case_ids):
            raise GoldenFixtureError("manifest.cases has duplicate case IDs")
        exclusion_ids = [exclusion.case_id for exclusion in exclusions]
        if len(set(exclusion_ids)) != len(exclusion_ids):
            raise GoldenFixtureError("manifest.exclusions has duplicate case IDs")
        if set(case_ids) & set(exclusion_ids):
            raise GoldenFixtureError("a case cannot be both executed and excluded")
        return cls(
            schema_version=schema_version,
            dataset_id=_require_nonempty_text(
                data["dataset_id"], "manifest.dataset_id"
            ),
            dataset_version=_require_nonempty_text(
                data["dataset_version"], "manifest.dataset_version"
            ),
            dataset_kind=dataset_kind,
            provenance=provenance,
            exclusions=tuple(
                sorted(exclusions, key=lambda exclusion: exclusion.case_id)
            ),
            cases=tuple(sorted(cases, key=lambda case: case.case_id)),
        )


def load_golden_manifest(path: Path) -> GoldenValidationManifest:
    """Load one JSON manifest without silently accepting invalid fixture content."""
    try:
        payload = json.loads(path.read_text(encoding="utf-8"))
    except OSError as error:
        raise GoldenFixtureError(f"could not read golden manifest {path}") from error
    except json.JSONDecodeError as error:
        raise GoldenFixtureError(f"golden manifest {path} is not valid JSON") from error
    return GoldenValidationManifest.from_mapping(payload)
