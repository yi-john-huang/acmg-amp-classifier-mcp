"""Maintainer command line for acquiring and building signed core bundles."""

import argparse
import json
from pathlib import Path

from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey

from data_builder.builder import build_core_bundle, load_recipe
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
    return parser


def main(argv: list[str] | None = None) -> int:
    args = _parser().parse_args(argv)
    recipe = load_recipe(args.recipe)
    if args.command == "fetch":
        paths = acquire_sources(recipe, args.inputs)
        print(json.dumps({"sources": [str(path) for path in paths]}, sort_keys=True))
        return 0
    result = build_core_bundle(
        args.recipe,
        args.inputs,
        args.output,
        _private_key_bytes(args.private_key),
    )
    print(
        json.dumps(
            {
                "archive": str(result.archive),
                "archive_sha256": result.archive_sha256,
                "manifest": str(result.manifest),
                "signature": str(result.signature),
            },
            sort_keys=True,
        )
    )
    return 0
