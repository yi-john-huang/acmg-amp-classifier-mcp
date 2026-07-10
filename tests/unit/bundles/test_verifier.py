from __future__ import annotations

import hashlib
import stat
import unittest
import warnings
import zipfile
from pathlib import Path
from tempfile import TemporaryDirectory

from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey


def manifest_data(content: bytes) -> dict[str, object]:
    return {
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
                "transformation_version": "1.0.0",
            }
        ],
        "genome_builds": ["GRCh38"],
        "transcript_release": "MANE Select v1.5",
        "rulesets": [{"identifier": "acmg-amp", "version": "2015.1"}],
        "signer_key_id": "test-key",
        "signature_algorithm": "Ed25519",
    }


class BundleVerifierTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temporary_directory = TemporaryDirectory()
        self.addCleanup(self.temporary_directory.cleanup)
        self.root = Path(self.temporary_directory.name)
        self.archive_path = self.root / "bundle.zip"
        self.staging_path = self.root / "verified"
        self.content = b"verified bundle bytes"

        from acmg_classifier.infrastructure.bundles.manifest import BundleManifest

        self.manifest = BundleManifest.model_validate(manifest_data(self.content))
        self.private_key = Ed25519PrivateKey.generate()
        public_key = self.private_key.public_key().public_bytes(
            encoding=serialization.Encoding.Raw,
            format=serialization.PublicFormat.Raw,
        )
        self.keyring = {"test-key": public_key}

    def _signature(self) -> bytes:
        from acmg_classifier.infrastructure.bundles.verifier import (
            canonical_manifest_bytes,
        )

        return self.private_key.sign(canonical_manifest_bytes(self.manifest))

    def _write_archive(
        self, entries: list[tuple[str | zipfile.ZipInfo, bytes]]
    ) -> None:
        with (
            zipfile.ZipFile(self.archive_path, "w", zipfile.ZIP_DEFLATED) as archive,
            warnings.catch_warnings(),
        ):
            warnings.simplefilter("ignore", UserWarning)
            for name, content in entries:
                archive.writestr(name, content)

    def test_valid_signature_and_artifact_extract_to_verified_staging(self) -> None:
        from acmg_classifier.infrastructure.bundles.verifier import BundleVerifier

        self._write_archive([("data/knowledge.sqlite3", self.content)])

        result = BundleVerifier(self.keyring).verify_and_extract(
            archive_path=self.archive_path,
            manifest=self.manifest,
            signature=self._signature(),
            staging_path=self.staging_path,
        )

        self.assertEqual(result.staging_path, self.staging_path)
        self.assertEqual(result.bundle_version, "2026.7.1")
        self.assertEqual(
            (self.staging_path / "data/knowledge.sqlite3").read_bytes(),
            self.content,
        )

    def test_bad_signature_and_unknown_key_fail_before_extraction(self) -> None:
        from acmg_classifier.infrastructure.bundles.verifier import (
            BundleSignatureError,
            BundleVerifier,
            UnknownSigningKeyError,
        )

        self._write_archive([("data/knowledge.sqlite3", self.content)])

        with self.assertRaises(BundleSignatureError):
            BundleVerifier(self.keyring).verify_and_extract(
                self.archive_path,
                self.manifest,
                b"bad signature",
                self.staging_path,
            )
        with self.assertRaises(UnknownSigningKeyError):
            BundleVerifier({}).verify_and_extract(
                self.archive_path,
                self.manifest,
                self._signature(),
                self.staging_path,
            )
        self.assertFalse(self.staging_path.exists())

    def test_checksum_and_declared_size_mismatch_are_rejected(self) -> None:
        from acmg_classifier.infrastructure.bundles.verifier import (
            ArtifactVerificationError,
            BundleVerifier,
        )

        invalid_contents = (b"tampered", b"x" * len(self.content))
        for invalid_content in invalid_contents:
            with self.subTest(invalid_content=invalid_content):
                self._write_archive([("data/knowledge.sqlite3", invalid_content)])
                with self.assertRaises(ArtifactVerificationError):
                    BundleVerifier(self.keyring).verify_and_extract(
                        self.archive_path,
                        self.manifest,
                        self._signature(),
                        self.staging_path,
                    )
        self.assertFalse(self.staging_path.exists())

    def test_traversal_absolute_duplicate_and_unexpected_paths_are_rejected(
        self,
    ) -> None:
        from acmg_classifier.infrastructure.bundles.verifier import (
            ArchiveSafetyError,
            BundleVerifier,
        )

        unsafe_archives = (
            [("../escape", b"x")],
            [("/absolute", b"x")],
            [("data/knowledge.sqlite3", self.content), ("unexpected", b"x")],
            [
                ("data/knowledge.sqlite3", self.content),
                ("data/knowledge.sqlite3", self.content),
            ],
        )
        for entries in unsafe_archives:
            with self.subTest(entries=entries):
                self._write_archive(entries)
                with self.assertRaises(ArchiveSafetyError):
                    BundleVerifier(self.keyring).verify_and_extract(
                        self.archive_path,
                        self.manifest,
                        self._signature(),
                        self.staging_path,
                    )
                self.assertFalse(self.staging_path.exists())

    def test_symlink_and_expanded_size_limit_are_rejected(self) -> None:
        from acmg_classifier.infrastructure.bundles.verifier import (
            ArchiveSafetyError,
            BundleVerifier,
        )

        link = zipfile.ZipInfo("data/knowledge.sqlite3")
        link.create_system = 3
        link.external_attr = stat.S_IFLNK << 16
        self._write_archive([(link, b"target")])
        with self.assertRaises(ArchiveSafetyError):
            BundleVerifier(self.keyring).verify_and_extract(
                self.archive_path,
                self.manifest,
                self._signature(),
                self.staging_path,
            )

        self._write_archive([("data/knowledge.sqlite3", self.content)])
        with self.assertRaises(ArchiveSafetyError):
            BundleVerifier(self.keyring, max_expanded_bytes=3).verify_and_extract(
                self.archive_path,
                self.manifest,
                self._signature(),
                self.staging_path,
            )
        self.assertFalse(self.staging_path.exists())

    def test_malformed_archive_policy_and_existing_staging_are_rejected(self) -> None:
        from acmg_classifier.infrastructure.bundles.verifier import (
            ArchiveSafetyError,
            BundleVerifier,
        )

        with self.assertRaisesRegex(ValueError, "max_expanded_bytes"):
            BundleVerifier(self.keyring, max_expanded_bytes=0)

        self.archive_path.write_bytes(b"not a zip")
        with self.assertRaises(ArchiveSafetyError):
            BundleVerifier(self.keyring).verify_and_extract(
                self.archive_path,
                self.manifest,
                self._signature(),
                self.staging_path,
            )

        self._write_archive([("data/knowledge.sqlite3", self.content)])
        self.staging_path.mkdir()
        sentinel = self.staging_path / "keep"
        sentinel.write_text("existing")
        with self.assertRaises(ArchiveSafetyError):
            BundleVerifier(self.keyring).verify_and_extract(
                self.archive_path,
                self.manifest,
                self._signature(),
                self.staging_path,
            )
        self.assertEqual(sentinel.read_text(), "existing")


if __name__ == "__main__":
    unittest.main()
