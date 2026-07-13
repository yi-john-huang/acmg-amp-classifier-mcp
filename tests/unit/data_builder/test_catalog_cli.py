from __future__ import annotations

from pathlib import Path

from data_builder.cli import _parser


def test_catalog_command_requires_public_key_and_metadata_arguments() -> None:
    args = _parser().parse_args(
        [
            "catalog",
            "--manifest",
            "manifest.json",
            "--manifest-signature",
            "manifest.sig",
            "--archive-url",
            "https://release.example/core.zip",
            "--catalog-version",
            "2026.7.1",
            "--signer-key-id",
            "catalog-key",
            "--private-key",
            "catalog.key",
            "--bundle-public-key",
            "bundle.pub",
            "--output",
            "catalog.json",
        ]
    )

    assert args.command == "catalog"
    assert args.manifest == Path("manifest.json")
    assert args.bundle_public_key == Path("bundle.pub")
