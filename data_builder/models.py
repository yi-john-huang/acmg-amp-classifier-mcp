"""Strict source-lock and build-recipe models for the maintainer data pipeline."""

from datetime import date, datetime
from pathlib import PurePath
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

from acmg_classifier.infrastructure.bundles.manifest import VersionRange

type Sha256Hex = Annotated[str, StringConstraints(pattern=r"^[0-9a-f]{64}$")]
type NumericVersion = Annotated[str, StringConstraints(pattern=r"^\d+(?:\.\d+){1,2}$")]
type NonEmptyText = Annotated[str, StringConstraints(min_length=1)]

ALLOWED_LICENSES = frozenset({"CC0-1.0", "NCBI-MOLECULAR-DATA-TERMS"})


class RecipeModel(BaseModel):
    """Immutable recipe model that rejects undeclared build inputs."""

    model_config = ConfigDict(extra="forbid", frozen=True)


class SourceLock(RecipeModel):
    """One immutable, license-reviewed upstream input."""

    identifier: Annotated[str, StringConstraints(pattern=r"^[a-z0-9][a-z0-9.-]+$")]
    kind: Literal["mane_summary", "clingen_gene_disease"]
    name: NonEmptyText
    release: NonEmptyText
    url: AnyHttpUrl
    filename: NonEmptyText
    sha256: Sha256Hex
    retrieved_at: datetime
    license_id: NonEmptyText
    terms_url: AnyHttpUrl
    redistribution_allowed: bool
    license_reviewed_by: NonEmptyText
    license_reviewed_at: date
    transformation_version: NumericVersion

    @field_validator("url", "terms_url")
    @classmethod
    def require_https(cls, value: AnyHttpUrl) -> AnyHttpUrl:
        if value.scheme != "https":
            raise ValueError("source and license terms URLs must use HTTPS")
        return value

    @field_validator("filename")
    @classmethod
    def require_plain_filename(cls, value: str) -> str:
        if PurePath(value).name != value or value in {".", ".."}:
            raise ValueError("source filename must not contain a path")
        return value

    @model_validator(mode="after")
    def require_reviewed_redistributable_license(self) -> Self:
        if self.license_id not in ALLOWED_LICENSES:
            raise ValueError(f"license is not approved: {self.license_id}")
        if not self.redistribution_allowed:
            raise ValueError("license does not permit redistribution")
        return self


class RulesetLock(RecipeModel):
    """Reviewed local ruleset input and its primary citation."""

    identifier: NonEmptyText
    version: NumericVersion
    path: NonEmptyText
    sha256: Sha256Hex
    citation_url: AnyHttpUrl
    transformation_version: NumericVersion

    @field_validator("citation_url")
    @classmethod
    def require_https(cls, value: AnyHttpUrl) -> AnyHttpUrl:
        if value.scheme != "https":
            raise ValueError("ruleset citation URL must use HTTPS")
        return value


class BuildRecipe(RecipeModel):
    """All inputs and fixed metadata required for a reproducible build."""

    format_version: NumericVersion
    bundle_version: NumericVersion
    application_version: VersionRange
    schema_version: VersionRange
    created_at: datetime
    channel: Literal["candidate", "stable"]
    genome_builds: Annotated[tuple[Literal["GRCh38"], ...], Field(min_length=1)]
    transcript_release: NonEmptyText
    signer_key_id: NonEmptyText
    sources: Annotated[tuple[SourceLock, ...], Field(min_length=2)]
    ruleset: RulesetLock

    @model_validator(mode="after")
    def require_one_source_of_each_kind(self) -> Self:
        kinds = [source.kind for source in self.sources]
        if len(kinds) != len(set(kinds)):
            raise ValueError("source kinds must be unique")
        required = {"mane_summary", "clingen_gene_disease"}
        if set(kinds) != required:
            raise ValueError(f"sources must contain exactly {sorted(required)}")
        filenames = [source.filename for source in self.sources]
        if len(filenames) != len(set(filenames)):
            raise ValueError("source filenames must be unique")
        return self
