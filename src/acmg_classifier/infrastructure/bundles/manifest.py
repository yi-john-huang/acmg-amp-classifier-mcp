"""Strict manifest for immutable reference-data bundles."""

from datetime import datetime
from pathlib import PurePosixPath
from typing import Annotated, Literal, Self

from pydantic import (
    AnyHttpUrl,
    BaseModel,
    ConfigDict,
    Field,
    StringConstraints,
    field_validator,
    model_validator,
)

from acmg_classifier.domain.enums import GenomeBuild

type NumericVersion = Annotated[
    str,
    StringConstraints(pattern=r"^\d+(?:\.\d+){1,2}$"),
]
type NonEmptyText = Annotated[str, StringConstraints(min_length=1)]
type Sha256Hex = Annotated[str, StringConstraints(pattern=r"^[0-9a-f]{64}$")]


class BundleModel(BaseModel):
    """Immutable manifest model that rejects unknown fields."""

    model_config = ConfigDict(extra="forbid", frozen=True)


class VersionRange(BundleModel):
    """Inclusive compatibility interval."""

    minimum: NumericVersion
    maximum: NumericVersion

    @model_validator(mode="after")
    def validate_order(self) -> Self:
        if _version_tuple(self.minimum) > _version_tuple(self.maximum):
            raise ValueError("minimum version must not exceed maximum version")
        return self


class BundleArtifact(BundleModel):
    """One hashed file contained by the bundle."""

    path: NonEmptyText
    byte_size: Annotated[int, Field(ge=0)]
    sha256: Sha256Hex
    media_type: NonEmptyText
    role: NonEmptyText

    @field_validator("path")
    @classmethod
    def validate_relative_path(cls, value: str) -> str:
        path = PurePosixPath(value)
        if path.is_absolute() or ".." in path.parts or "\\" in value:
            raise ValueError("artifact path must be a safe relative POSIX path")
        return value


class BundleSource(BundleModel):
    """Provenance for one upstream dataset."""

    name: NonEmptyText
    release: NonEmptyText
    url: AnyHttpUrl
    retrieved_at: datetime
    license: NonEmptyText
    transformation_version: NumericVersion

    @field_validator("url")
    @classmethod
    def require_https(cls, value: AnyHttpUrl) -> AnyHttpUrl:
        if value.scheme != "https":
            raise ValueError("source URL must use HTTPS")
        return value


class RulesetReference(BundleModel):
    """Ruleset identity included by a bundle."""

    identifier: NonEmptyText
    version: NumericVersion


class BundleManifest(BundleModel):
    """Signed bundle metadata used for compatibility and integrity checks."""

    format_version: NumericVersion
    bundle_version: NumericVersion
    application_version: VersionRange
    schema_version: VersionRange
    created_at: datetime
    channel: NonEmptyText
    artifacts: Annotated[tuple[BundleArtifact, ...], Field(min_length=1)]
    sources: Annotated[tuple[BundleSource, ...], Field(min_length=1)]
    genome_builds: Annotated[tuple[GenomeBuild, ...], Field(min_length=1)]
    transcript_release: NonEmptyText
    rulesets: Annotated[tuple[RulesetReference, ...], Field(min_length=1)]
    signer_key_id: NonEmptyText
    signature_algorithm: Literal["Ed25519"]


def _version_tuple(value: str) -> tuple[int, int, int]:
    parts = [int(part) for part in value.split(".")]
    padded = [*parts, 0, 0, 0][:3]
    return padded[0], padded[1], padded[2]
