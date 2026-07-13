from __future__ import annotations

import base64
import hashlib
import json
from pathlib import Path

import pytest
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
from data_builder.catalog import CatalogBuildError, build_signed_catalog

from acmg_classifier.infrastructure.bundles.manifest import BundleManifest
from acmg_classifier.infrastructure.bundles.verifier import canonical_manifest_bytes


def _manifest() -> BundleManifest:
    content = b"catalog builder knowledge"
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


def _raw_private_key(key: Ed25519PrivateKey) -> bytes:
    return key.private_bytes(
        serialization.Encoding.Raw,
        serialization.PrivateFormat.Raw,
        serialization.NoEncryption(),
    )



def _raw_public_key(key: Ed25519PrivateKey) -> bytes:
    return key.public_key().public_bytes(
        serialization.Encoding.Raw,
        serialization.PublicFormat.Raw,
    )

def test_builder_writes_deterministic_signed_catalog(tmp_path: Path) -> None:
    manifest = _manifest()
    bundle_key = Ed25519PrivateKey.generate()
    catalog_key = Ed25519PrivateKey.generate()
    manifest_path = tmp_path / "manifest.json"
    signature_path = tmp_path / "manifest.sig"
    output_one = tmp_path / "catalog-one.json"
    output_two = tmp_path / "catalog-two.json"
    manifest_path.write_bytes(canonical_manifest_bytes(manifest) + b"\n")
    signature_path.write_bytes(bundle_key.sign(canonical_manifest_bytes(manifest)))

    first = build_signed_catalog(
        manifest_path,
        signature_path,
        "https://release.example/core.zip",
        catalog_version="2026.7.1",
        signer_key_id="catalog-key",
        private_key_bytes=_raw_private_key(catalog_key),
        bundle_public_key_bytes=_raw_public_key(bundle_key),
        output_path=output_one,
    )
    second = build_signed_catalog(
        manifest_path,
        signature_path,
        "https://release.example/core.zip",
        catalog_version="2026.7.1",
        signer_key_id="catalog-key",
        private_key_bytes=_raw_private_key(catalog_key),
        bundle_public_key_bytes=_raw_public_key(bundle_key),
        output_path=output_two,
    )

    assert first == second
    assert output_one.read_bytes() == output_two.read_bytes()
    assert base64.b64decode(first.signature, validate=True)
    payload = json.loads(output_one.read_text())
    assert payload["candidates"][0]["archive_url"].startswith("https://")


def test_builder_rejects_manifest_signature_mismatch(tmp_path: Path) -> None:
    manifest = _manifest()
    catalog_key = Ed25519PrivateKey.generate()
    manifest_path = tmp_path / "manifest.json"
    signature_path = tmp_path / "manifest.sig"
    manifest_path.write_bytes(canonical_manifest_bytes(manifest))
    signature_path.write_bytes(b"wrong")

    with pytest.raises(CatalogBuildError, match="manifest signature"):
        build_signed_catalog(
            manifest_path,
            signature_path,
            "https://release.example/core.zip",
            catalog_version="2026.7.1",
            signer_key_id="catalog-key",
            private_key_bytes=_raw_private_key(catalog_key),
            bundle_public_key_bytes=b"invalid",
            output_path=tmp_path / "catalog.json",
        )


def test_builder_rejects_invalid_private_key(tmp_path: Path) -> None:
    manifest = _manifest()
    bundle_key = Ed25519PrivateKey.generate()
    manifest_path = tmp_path / "manifest.json"
    signature_path = tmp_path / "manifest.sig"
    manifest_path.write_bytes(canonical_manifest_bytes(manifest))
    signature_path.write_bytes(bundle_key.sign(canonical_manifest_bytes(manifest)))

    with pytest.raises(CatalogBuildError, match="Ed25519"):
        build_signed_catalog(
            manifest_path,
            signature_path,
            "https://release.example/core.zip",
            catalog_version="2026.7.1",
            signer_key_id="catalog-key",
            private_key_bytes=b"short",
            bundle_public_key_bytes=_raw_public_key(bundle_key),
            output_path=tmp_path / "catalog.json",
        )


def test_builder_rejects_non_https_archive_url(tmp_path: Path) -> None:
    manifest = _manifest()
    bundle_key = Ed25519PrivateKey.generate()
    catalog_key = Ed25519PrivateKey.generate()
    manifest_path = tmp_path / "manifest.json"
    signature_path = tmp_path / "manifest.sig"
    manifest_path.write_bytes(canonical_manifest_bytes(manifest))
    signature_path.write_bytes(bundle_key.sign(canonical_manifest_bytes(manifest)))

    with pytest.raises(CatalogBuildError, match="HTTPS"):
        build_signed_catalog(
            manifest_path,
            signature_path,
            "http://release.example/core.zip",
            catalog_version="2026.7.1",
            signer_key_id="catalog-key",
            private_key_bytes=_raw_private_key(catalog_key),
            bundle_public_key_bytes=_raw_public_key(bundle_key),
            output_path=tmp_path / "catalog.json",
        )
