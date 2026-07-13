from __future__ import annotations

import base64
import json
from pathlib import Path

import pytest
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey

from acmg_classifier.infrastructure.bundles.catalog import (
    CatalogSignatureError,
    FileBundleCatalog,
    SignedReleaseCatalog,
    canonical_catalog_bytes,
)
from acmg_classifier.infrastructure.bundles.manifest import BundleManifest
from acmg_classifier.infrastructure.bundles.verifier import canonical_manifest_bytes


def _manifest() -> BundleManifest:
    content = b"catalog test knowledge"
    import hashlib

    return BundleManifest.model_validate(
        {
            "format_version": "1.0",
            "bundle_version": "2026.7.1",
            "application_version": {"minimum": "0.1.0", "maximum": "0.9.9"},
            "schema_version": {"minimum": "1.0", "maximum": "1.2"},
            "created_at": "2026-07-11T08:00:00Z",
            "channel": "stable",
            "artifacts": [
                {
                    "path": "data/knowledge.sqlite3",
                    "byte_size": len(content),
                    "sha256": hashlib.sha256(content).hexdigest(),
                    "media_type": "application/vnd.sqlite3",
                    "role": "knowledge",
                }
            ],
            "sources": [
                {
                    "name": "NCBI MANE",
                    "release": "1.5",
                    "url": "https://www.ncbi.nlm.nih.gov/refseq/MANE/",
                    "retrieved_at": "2026-07-11T07:00:00Z",
                    "license": "United States government work",
                    "sha256": "b" * 64,
                    "terms_url": "https://www.ncbi.nlm.nih.gov/home/about/policies/",
                    "transformation_version": "1.0.0",
                }
            ],
            "genome_builds": ["GRCh38"],
            "transcript_release": "MANE Select v1.5",
            "rulesets": [{"identifier": "acmg-amp", "version": "2015.1"}],
            "signer_key_id": "bundle-key",
            "signature_algorithm": "Ed25519",
        }
    )


def _raw_public_key(key: Ed25519PrivateKey) -> bytes:
    return key.public_key().public_bytes(
        encoding=serialization.Encoding.Raw,
        format=serialization.PublicFormat.Raw,
    )


def _catalog_payload(
    manifest: BundleManifest,
    manifest_signature: bytes,
    *,
    archive_url: str = "https://release.example/core.zip",
    signer_key_id: str = "catalog-key",
) -> dict[str, object]:
    return {
        "schema_version": "1.0",
        "catalog_version": "2026.7.1",
        "created_at": "2026-07-13T12:00:00Z",
        "signer_key_id": signer_key_id,
        "signature_algorithm": "Ed25519",
        "candidates": [
            {
                "archive_url": archive_url,
                "manifest": manifest.model_dump(mode="json"),
                "manifest_signature": base64.b64encode(manifest_signature).decode(
                    "ascii"
                ),
            }
        ],
        "signature": "",
    }


def _write_catalog(
    path: Path,
    manifest: BundleManifest,
    bundle_key: Ed25519PrivateKey,
    catalog_key: Ed25519PrivateKey,
    *,
    archive_url: str = "https://release.example/core.zip",
) -> dict[str, bytes]:
    manifest_signature = bundle_key.sign(canonical_manifest_bytes(manifest))
    unsigned = SignedReleaseCatalog.model_validate(
        _catalog_payload(manifest, manifest_signature, archive_url=archive_url)
    )
    catalog_signature = catalog_key.sign(canonical_catalog_bytes(unsigned))
    payload = unsigned.model_dump(mode="json")
    payload["signature"] = base64.b64encode(catalog_signature).decode("ascii")
    path.write_text(json.dumps(payload), encoding="utf-8")
    return {"manifest": manifest_signature, "catalog": catalog_signature}


def test_signed_catalog_returns_verified_candidates(tmp_path: Path) -> None:
    manifest = _manifest()
    bundle_key = Ed25519PrivateKey.generate()
    catalog_key = Ed25519PrivateKey.generate()
    path = tmp_path / "catalog.json"
    _write_catalog(path, manifest, bundle_key, catalog_key)

    catalog = FileBundleCatalog(
        path,
        {
            "catalog-key": _raw_public_key(catalog_key),
            "bundle-key": _raw_public_key(bundle_key),
        },
    )

    candidates = catalog.candidates()

    assert len(candidates) == 1
    assert candidates[0].manifest.bundle_version == "2026.7.1"
    assert candidates[0].archive_url == "https://release.example/core.zip"


def test_catalog_signature_tampering_fails_before_candidates(tmp_path: Path) -> None:
    manifest = _manifest()
    bundle_key = Ed25519PrivateKey.generate()
    catalog_key = Ed25519PrivateKey.generate()
    path = tmp_path / "catalog.json"
    _write_catalog(path, manifest, bundle_key, catalog_key)
    payload = json.loads(path.read_text(encoding="utf-8"))
    payload["candidates"][0]["archive_url"] = "https://evil.example/core.zip"
    path.write_text(json.dumps(payload), encoding="utf-8")

    catalog = FileBundleCatalog(path, {"catalog-key": _raw_public_key(catalog_key)})

    with pytest.raises(CatalogSignatureError):
        catalog.candidates()


def test_unknown_catalog_key_fails_closed(tmp_path: Path) -> None:
    manifest = _manifest()
    bundle_key = Ed25519PrivateKey.generate()
    catalog_key = Ed25519PrivateKey.generate()
    path = tmp_path / "catalog.json"
    _write_catalog(path, manifest, bundle_key, catalog_key)

    catalog = FileBundleCatalog(path, {"bundle-key": _raw_public_key(bundle_key)})

    with pytest.raises(CatalogSignatureError, match="Unknown"):
        catalog.candidates()


def test_candidate_manifest_signature_tampering_fails_closed(tmp_path: Path) -> None:
    manifest = _manifest()
    bundle_key = Ed25519PrivateKey.generate()
    catalog_key = Ed25519PrivateKey.generate()
    path = tmp_path / "catalog.json"
    _write_catalog(path, manifest, bundle_key, catalog_key)
    payload = json.loads(path.read_text(encoding="utf-8"))
    payload["candidates"][0]["manifest_signature"] = base64.b64encode(
        b"bad"
    ).decode("ascii")
    unsigned = SignedReleaseCatalog.model_validate({**payload, "signature": ""})
    payload["signature"] = base64.b64encode(
        catalog_key.sign(canonical_catalog_bytes(unsigned))
    ).decode("ascii")
    path.write_text(json.dumps(payload), encoding="utf-8")

    catalog = FileBundleCatalog(
        path,
        {
            "catalog-key": _raw_public_key(catalog_key),
            "bundle-key": _raw_public_key(bundle_key),
        },
    )

    with pytest.raises(Exception, match="invalid"):
        catalog.candidates()


def test_non_https_candidate_is_rejected(tmp_path: Path) -> None:
    manifest = _manifest()
    bundle_key = Ed25519PrivateKey.generate()
    catalog_key = Ed25519PrivateKey.generate()
    path = tmp_path / "catalog.json"
    with pytest.raises(ValueError, match="HTTPS"):
        _write_catalog(
            path,
            manifest,
            bundle_key,
            catalog_key,
            archive_url="http://release.example/core.zip",
        )
