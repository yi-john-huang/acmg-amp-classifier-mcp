"""Maintainer command line for acquiring and building signed core bundles."""

import argparse
import json
from pathlib import Path

from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric.ed25519 import (
    Ed25519PrivateKey,
    Ed25519PublicKey,
)

from data_builder.builder import build_core_bundle, load_recipe
from data_builder.catalog import build_signed_catalog
from data_builder.sources import acquire_sources


def _private_key_bytes(path: Path) -> bytes:
    content = path.read_bytes()
    if len(content) == 32:
        return content
    key = serialization.load_pem_private_key(content, password=None)
    if not isinstance(key, Ed25519PrivateKey):
        raise ValueError("signing key is not Ed25519")
    return key.private_bytes(
        serialization.Encoding.Raw,
        serialization.PrivateFormat.Raw,
        serialization.NoEncryption(),
    )


def _public_key_bytes(path: Path) -> bytes:
    content = path.read_bytes()
    if len(content) == 32:
        Ed25519PublicKey.from_public_bytes(content)
        return content
    key = serialization.load_pem_public_key(content)
    if not isinstance(key, Ed25519PublicKey):
        raise ValueError("bundle public key is not Ed25519")
    return key.public_bytes(
        serialization.Encoding.Raw,
        serialization.PublicFormat.Raw,
    )


def _parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    fetch = commands.add_parser("fetch", help="download and checksum source locks")
    fetch.add_argument("--recipe", type=Path, required=True)
    fetch.add_argument("--inputs", type=Path, required=True)
    build = commands.add_parser("build", help="transform, package, and sign a bundle")
    build.add_argument("--recipe", type=Path, required=True)
    build.add_argument("--inputs", type=Path, required=True)
    build.add_argument("--output", type=Path, required=True)
    build.add_argument("--private-key", type=Path, required=True)
    catalog = commands.add_parser(
        "catalog", help="create a signed catalog from a verified bundle"
    )
    catalog.add_argument("--manifest", type=Path, required=True)
    catalog.add_argument("--manifest-signature", type=Path, required=True)
    catalog.add_argument("--archive-url", required=True)
    catalog.add_argument("--catalog-version", required=True)
    catalog.add_argument("--signer-key-id", required=True)
    catalog.add_argument("--private-key", type=Path, required=True)
    catalog.add_argument("--bundle-public-key", type=Path, required=True)
    catalog.add_argument("--output", type=Path, required=True)
    return parser


def main(argv: list[str] | None = None) -> int:
    args = _parser().parse_args(argv)
    if args.command == "catalog":
        catalog_result = build_signed_catalog(
            args.manifest,
            args.manifest_signature,
            args.archive_url,
            catalog_version=args.catalog_version,
            signer_key_id=args.signer_key_id,
            private_key_bytes=_private_key_bytes(args.private_key),
            bundle_public_key_bytes=_public_key_bytes(args.bundle_public_key),
            output_path=args.output,
        )
        print(
            json.dumps(
                {
                    "catalog": str(args.output),
                    "catalog_version": catalog_result.catalog_version,
                    "signer_key_id": catalog_result.signer_key_id,
                },
                sort_keys=True,
            )
        )
        return 0

    recipe = load_recipe(args.recipe)
    if args.command == "fetch":
        paths = acquire_sources(recipe, args.inputs)
        print(json.dumps({"sources": [str(path) for path in paths]}, sort_keys=True))
        return 0
    bundle_result = build_core_bundle(
        args.recipe,
        args.inputs,
        args.output,
        _private_key_bytes(args.private_key),
    )
    print(
        json.dumps(
            {
                "archive": str(bundle_result.archive),
                "archive_sha256": bundle_result.archive_sha256,
                "manifest": str(bundle_result.manifest),
                "signature": str(bundle_result.signature),
            },
            sort_keys=True,
        )
    )
    return 0
