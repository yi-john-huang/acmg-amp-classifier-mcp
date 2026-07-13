"""Strict signed release-catalog loading for controlled bundle distribution."""

from __future__ import annotations

import base64
import binascii
import json
from collections.abc import Mapping
from datetime import datetime
from pathlib import Path
from typing import Annotated, Literal

from cryptography.exceptions import InvalidSignature
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PublicKey
from pydantic import (
    AnyHttpUrl,
    BaseModel,
    ConfigDict,
    Field,
    StringConstraints,
    ValidationError,
    field_validator,
)

from acmg_classifier.domain.canonical import canonical_json_bytes
from acmg_classifier.infrastructure.bundles.manifest import BundleManifest
from acmg_classifier.infrastructure.bundles.provisioner import (
    BundleCandidate,
    BundleCatalog,
)
from acmg_classifier.infrastructure.bundles.verifier import (
    BundleVerificationError,
    BundleVerifier,
)

NumericVersion = Annotated[str, StringConstraints(pattern=r"^\d+(?:\.\d+){1,2}$")]
NonEmptyText = Annotated[str, StringConstraints(min_length=1)]


class CatalogVerificationError(RuntimeError):
    """Base class for release-catalog trust failures."""


class CatalogSignatureError(CatalogVerificationError):
    """The catalog signature or trusted key is invalid."""


class CatalogValidationError(CatalogVerificationError):
    """The catalog structure or candidate set is invalid."""


class CatalogCandidateError(CatalogVerificationError):
    """A catalog candidate has an invalid manifest signature."""


class CatalogCandidateModel(BaseModel):
    """One manifest and signed manifest signature offered by a catalog."""

    model_config = ConfigDict(extra="forbid", frozen=True)

    archive_url: AnyHttpUrl
    manifest: BundleManifest
    manifest_signature: str

    @field_validator("archive_url")
    @classmethod
    def require_https(cls, value: AnyHttpUrl) -> AnyHttpUrl:
        if value.scheme != "https":
            raise ValueError("catalog archive URLs require HTTPS")
        return value

    @field_validator("manifest_signature")
    @classmethod
    def validate_base64_signature(cls, value: str) -> str:
        if not value:
            raise ValueError("manifest signature must not be empty")
        try:
            decoded = base64.b64decode(value, validate=True)
        except (binascii.Error, ValueError) as error:
            raise ValueError("manifest signature must be base64") from error
        if not decoded:
            raise ValueError("manifest signature must not be empty")
        return value


class SignedReleaseCatalog(BaseModel):
    """Canonical catalog payload with a detached Ed25519 signature field."""

    model_config = ConfigDict(extra="forbid", frozen=True)

    schema_version: Literal["1.0"]
    catalog_version: NumericVersion
    created_at: datetime
    signer_key_id: NonEmptyText
    signature_algorithm: Literal["Ed25519"]
    candidates: tuple[CatalogCandidateModel, ...] = Field(min_length=1)
    signature: str = ""

    @field_validator("signature")
    @classmethod
    def validate_catalog_signature(cls, value: str) -> str:
        if not value:
            return value
        try:
            decoded = base64.b64decode(value, validate=True)
        except (binascii.Error, ValueError) as error:
            raise ValueError("catalog signature must be base64") from error
        if not decoded:
            raise ValueError("catalog signature must not be empty")
        return value


def canonical_catalog_bytes(catalog: SignedReleaseCatalog) -> bytes:
    """Return canonical unsigned catalog bytes covered by its signature."""
    return canonical_json_bytes(
        catalog.model_dump(mode="json", exclude={"signature"})
    )


class FileBundleCatalog(BundleCatalog):
    """Load and verify a retained signed catalog without network access."""

    def __init__(self, path: Path, public_keys: Mapping[str, bytes]) -> None:
        self.path = Path(path)
        self.public_keys = dict(public_keys)
        self._verifier = BundleVerifier(self.public_keys)

    def candidates(self) -> tuple[BundleCandidate, ...]:
        """Return candidates only after catalog and manifest verification."""
        catalog = self._load()
        self._verify_catalog_signature(catalog)
        versions: set[str] = set()
        candidates: list[BundleCandidate] = []
        for item in catalog.candidates:
            version = item.manifest.bundle_version
            if version in versions:
                raise CatalogValidationError(
                    f"duplicate catalog bundle version: {version}"
                )
            versions.add(version)
            signature = _decode_signature(item.manifest_signature, "manifest")
            try:
                self._verifier.verify_manifest_signature(item.manifest, signature)
            except BundleVerificationError as error:
                raise CatalogCandidateError(
                    f"candidate manifest signature is invalid: {version}"
                ) from error
            candidates.append(
                BundleCandidate(
                    archive_url=str(item.archive_url),
                    manifest=item.manifest,
                    signature=signature,
                )
            )
        return tuple(candidates)

    def _load(self) -> SignedReleaseCatalog:
        try:
            payload = json.loads(self.path.read_text(encoding="utf-8"))
            return SignedReleaseCatalog.model_validate(payload)
        except (OSError, json.JSONDecodeError, ValidationError, ValueError) as error:
            raise CatalogValidationError("release catalog is invalid") from error

    def _verify_catalog_signature(self, catalog: SignedReleaseCatalog) -> None:
        key_bytes = self.public_keys.get(catalog.signer_key_id)
        if key_bytes is None:
            raise CatalogSignatureError(
                f"Unknown catalog signing key: {catalog.signer_key_id}"
            )
        signature = _decode_signature(catalog.signature, "catalog")
        try:
            key = Ed25519PublicKey.from_public_bytes(key_bytes)
            key.verify(signature, canonical_catalog_bytes(catalog))
        except (InvalidSignature, ValueError) as error:
            raise CatalogSignatureError("catalog signature is invalid") from error


def _decode_signature(value: str, label: str) -> bytes:
    try:
        decoded = base64.b64decode(value, validate=True)
    except (binascii.Error, ValueError) as error:
        raise CatalogSignatureError(f"{label} signature is not valid base64") from error
    if not decoded:
        raise CatalogSignatureError(f"{label} signature is empty")
    return decoded
