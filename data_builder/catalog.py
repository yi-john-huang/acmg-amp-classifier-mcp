"""Build deterministic signed release catalogs from verified bundle metadata."""

from __future__ import annotations

import base64
import os
from pathlib import Path

from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
from pydantic import ValidationError

from acmg_classifier.domain.canonical import canonical_json_bytes
from acmg_classifier.infrastructure.bundles.catalog import (
    SignedReleaseCatalog,
    canonical_catalog_bytes,
)
from acmg_classifier.infrastructure.bundles.manifest import BundleManifest
from acmg_classifier.infrastructure.bundles.verifier import (
    BundleVerificationError,
    BundleVerifier,
)


class CatalogBuildError(RuntimeError):
    """A controlled catalog input or signing operation failed."""


def build_signed_catalog(
    manifest_path: Path,
    signature_path: Path,
    archive_url: str,
    *,
    catalog_version: str,
    signer_key_id: str,
    private_key_bytes: bytes,
    bundle_public_key_bytes: bytes,
    output_path: Path,
) -> SignedReleaseCatalog:
    """Verify a bundle manifest and write one deterministic signed catalog.

    ``private_key_bytes`` is supplied by the caller and is never written to
    disk or included in the returned model.
    """
    manifest = _load_manifest(Path(manifest_path))
    manifest_signature = _read_signature(Path(signature_path))
    _verify_manifest(manifest, manifest_signature, bundle_public_key_bytes)
    private_key = _load_private_key(private_key_bytes)
    try:
        unsigned = SignedReleaseCatalog.model_validate(
            {
                "schema_version": "1.0",
                "catalog_version": catalog_version,
                "created_at": manifest.created_at.isoformat().replace("+00:00", "Z"),
                "signer_key_id": signer_key_id,
                "signature_algorithm": "Ed25519",
                "candidates": [
                    {
                        "archive_url": archive_url,
                        "manifest": manifest.model_dump(mode="json"),
                        "manifest_signature": base64.b64encode(
                            manifest_signature
                        ).decode("ascii"),
                    }
                ],
                "signature": "",
            }
        )
    except (ValidationError, ValueError) as error:
        raise CatalogBuildError(f"invalid catalog metadata: {error}") from error
    catalog_signature = private_key.sign(canonical_catalog_bytes(unsigned))
    catalog = unsigned.model_copy(
        update={"signature": base64.b64encode(catalog_signature).decode("ascii")}
    )
    _write_catalog(Path(output_path), catalog)
    return catalog


def _load_manifest(path: Path) -> BundleManifest:
    try:
        return BundleManifest.model_validate_json(path.read_bytes())
    except (OSError, ValidationError, ValueError) as error:
        raise CatalogBuildError("bundle manifest is invalid") from error


def _read_signature(path: Path) -> bytes:
    try:
        signature = path.read_bytes()
    except OSError as error:
        raise CatalogBuildError("bundle manifest signature cannot be read") from error
    if not signature:
        raise CatalogBuildError("bundle manifest signature is empty")
    return signature


def _verify_manifest(
    manifest: BundleManifest,
    signature: bytes,
    public_key_bytes: bytes,
) -> None:
    try:
        BundleVerifier(
            {manifest.signer_key_id: public_key_bytes}
        ).verify_manifest_signature(manifest, signature)
    except (BundleVerificationError, ValueError) as error:
        raise CatalogBuildError("bundle manifest signature is invalid") from error


def _load_private_key(private_key_bytes: bytes) -> Ed25519PrivateKey:
    try:
        return Ed25519PrivateKey.from_private_bytes(private_key_bytes)
    except ValueError as error:
        try:
            key = serialization.load_pem_private_key(private_key_bytes, password=None)
        except (ValueError, TypeError) as pem_error:
            raise CatalogBuildError(
                "catalog signing key must be a raw or PEM Ed25519 key"
            ) from pem_error
        if not isinstance(key, Ed25519PrivateKey):
            raise CatalogBuildError("catalog signing key must be Ed25519") from error
        return key


def _write_catalog(path: Path, catalog: SignedReleaseCatalog) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    temporary = path.with_suffix(path.suffix + ".tmp")
    try:
        content = canonical_json_bytes(catalog.model_dump(mode="json")) + b"\n"
        temporary.write_bytes(content)
        os.replace(temporary, path)
    except OSError as error:
        temporary.unlink(missing_ok=True)
        raise CatalogBuildError("signed catalog cannot be written") from error
