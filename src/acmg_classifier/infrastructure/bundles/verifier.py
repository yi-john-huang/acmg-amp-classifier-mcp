"""Ed25519 bundle signature and bounded archive verification."""

import hashlib
import os
import shutil
import stat
import struct
import tempfile
import zipfile
from collections.abc import Mapping
from dataclasses import dataclass
from pathlib import Path, PurePosixPath

from cryptography.exceptions import InvalidSignature
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PublicKey

from acmg_classifier.domain.canonical import canonical_json_bytes
from acmg_classifier.infrastructure.bundles.manifest import (
    BundleArtifact,
    BundleManifest,
)


class BundleVerificationError(RuntimeError):
    """Base class for bundle trust or integrity failures."""


class UnknownSigningKeyError(BundleVerificationError):
    """The manifest references a key not shipped by this application."""


class BundleSignatureError(BundleVerificationError):
    """The manifest signature is invalid."""


class ArchiveSafetyError(BundleVerificationError):
    """The archive has unsafe structure or expansion characteristics."""


class ArtifactVerificationError(BundleVerificationError):
    """An extracted artifact differs from its signed manifest metadata."""


@dataclass(frozen=True, slots=True)
class VerifiedBundle:
    """A verified but not yet activated bundle staging directory."""

    bundle_version: str
    staging_path: Path


def canonical_manifest_bytes(manifest: BundleManifest) -> bytes:
    """Return the exact canonical bytes covered by the release signature."""
    return canonical_json_bytes(manifest.model_dump(mode="json"))


class BundleVerifier:
    """Verify signed ZIP contents before exposing a staging directory."""

    def __init__(
        self,
        public_keys: Mapping[str, bytes],
        *,
        max_expanded_bytes: int = 250 * 1024 * 1024,
        max_members: int = 4_096,
        max_metadata_bytes: int = 16 * 1024 * 1024,
    ) -> None:
        if max_expanded_bytes < 1:
            raise ValueError("max_expanded_bytes must be positive")
        if max_members < 1:
            raise ValueError("max_members must be positive")
        if max_metadata_bytes < 1:
            raise ValueError("max_metadata_bytes must be positive")
        self.public_keys = dict(public_keys)
        self.max_expanded_bytes = max_expanded_bytes
        self.max_members = max_members
        self.max_metadata_bytes = max_metadata_bytes

    def verify_and_extract(
        self,
        archive_path: Path,
        manifest: BundleManifest,
        signature: bytes,
        staging_path: Path,
    ) -> VerifiedBundle:
        """Verify signature and artifacts, leaving only trusted staged files."""
        self.verify_manifest_signature(manifest, signature)
        if staging_path.exists():
            raise ArchiveSafetyError(f"Staging path already exists: {staging_path}")
        staging_path.parent.mkdir(parents=True, exist_ok=True)
        temporary_path = Path(
            tempfile.mkdtemp(dir=staging_path.parent, prefix=".untrusted-")
        )
        try:
            self._extract_archive(archive_path, manifest, temporary_path)
            os.replace(temporary_path, staging_path)
        except Exception:
            shutil.rmtree(temporary_path, ignore_errors=True)
            raise
        return VerifiedBundle(
            bundle_version=manifest.bundle_version,
            staging_path=staging_path,
        )

    def verify_manifest_signature(
        self, manifest: BundleManifest, signature: bytes
    ) -> None:
        """Verify an installed manifest without extracting its archive."""
        self._verify_signature(manifest, signature)

    def _verify_signature(self, manifest: BundleManifest, signature: bytes) -> None:
        key_bytes = self.public_keys.get(manifest.signer_key_id)
        if key_bytes is None:
            raise UnknownSigningKeyError(
                f"Unknown bundle signing key: {manifest.signer_key_id}"
            )
        try:
            key = Ed25519PublicKey.from_public_bytes(key_bytes)
            key.verify(signature, canonical_manifest_bytes(manifest))
        except (InvalidSignature, ValueError) as error:
            raise BundleSignatureError(
                "Bundle manifest signature is invalid"
            ) from error

    def _extract_archive(
        self,
        archive_path: Path,
        manifest: BundleManifest,
        temporary_path: Path,
    ) -> None:
        artifacts = {artifact.path: artifact for artifact in manifest.artifacts}
        self._preflight_metadata(archive_path)
        try:
            with zipfile.ZipFile(archive_path) as archive:
                members = archive.infolist()
                self._validate_members(members, artifacts)
                for member in members:
                    self._extract_artifact(
                        archive, member, artifacts[member.filename], temporary_path
                    )
        except zipfile.BadZipFile as error:
            raise ArchiveSafetyError("Bundle is not a valid ZIP archive") from error

    def _preflight_metadata(self, archive_path: Path) -> None:
        """Bound central-directory metadata before ``ZipFile`` parses it."""
        end_of_central_directory = b"PK\x05\x06"
        end_record_size = 22
        maximum_comment_size = 0xFFFF
        try:
            archive_size = archive_path.stat().st_size
            with archive_path.open("rb") as archive:
                archive.seek(
                    -min(
                        archive_size,
                        end_record_size + maximum_comment_size,
                    ),
                    2,
                )
                trailer = archive.read(end_record_size + maximum_comment_size)
        except OSError as error:
            raise ArchiveSafetyError("Bundle archive cannot be read") from error

        record_offset = trailer.rfind(end_of_central_directory)
        while record_offset >= 0:
            if record_offset + end_record_size <= len(trailer):
                (
                    _,
                    disk_number,
                    central_directory_disk,
                    members_on_disk,
                    member_count,
                    central_directory_size,
                    _,
                    comment_size,
                ) = struct.unpack_from("<4s4H2LH", trailer, record_offset)
                if record_offset + end_record_size + comment_size == len(trailer):
                    break
            record_offset = trailer.rfind(end_of_central_directory, 0, record_offset)
        else:
            raise ArchiveSafetyError("Bundle archive has no valid ZIP directory record")

        if (
            disk_number != 0
            or central_directory_disk != 0
            or members_on_disk != member_count
        ):
            raise ArchiveSafetyError("Multi-disk ZIP archives are not allowed")
        if member_count > self.max_members:
            raise ArchiveSafetyError(
                f"Bundle archive has {member_count} members, above {self.max_members}"
            )
        if central_directory_size > self.max_metadata_bytes:
            raise ArchiveSafetyError(
                "Bundle archive metadata exceeds "
                f"{self.max_metadata_bytes} bytes"
            )
        if central_directory_size > archive_size - len(trailer) + record_offset:
            raise ArchiveSafetyError("Bundle archive has an invalid central directory")

    def _validate_members(
        self,
        members: list[zipfile.ZipInfo],
        artifacts: dict[str, BundleArtifact],
    ) -> None:
        names = [member.filename for member in members]
        if len(names) != len(set(names)):
            raise ArchiveSafetyError("Bundle archive contains duplicate paths")
        for member in members:
            self._validate_member_path(member)
        member_names = set(names)
        expected_names = set(artifacts)
        if member_names != expected_names:
            unexpected = sorted(member_names - expected_names)
            missing = sorted(expected_names - member_names)
            raise ArchiveSafetyError(
                f"Bundle archive paths differ from manifest; unexpected={unexpected}, "
                f"missing={missing}"
            )
        total_size = sum(member.file_size for member in members)
        if total_size > self.max_expanded_bytes:
            raise ArchiveSafetyError(
                f"Bundle expands to {total_size} bytes, above {self.max_expanded_bytes}"
            )

    def _validate_member_path(self, member: zipfile.ZipInfo) -> None:
        path = PurePosixPath(member.filename)
        if (
            not member.filename
            or path.is_absolute()
            or ".." in path.parts
            or "\\" in member.filename
        ):
            raise ArchiveSafetyError(f"Unsafe archive path: {member.filename}")
        if member.is_dir():
            raise ArchiveSafetyError(
                f"Explicit directory entries are not allowed: {member.filename}"
            )
        mode = (member.external_attr >> 16) & 0xFFFF
        file_type = stat.S_IFMT(mode)
        if file_type not in (0, stat.S_IFREG):
            raise ArchiveSafetyError(
                f"Archive member is not a regular file: {member.filename}"
            )

    def _extract_artifact(
        self,
        archive: zipfile.ZipFile,
        member: zipfile.ZipInfo,
        artifact: BundleArtifact,
        temporary_path: Path,
    ) -> None:
        if member.file_size != artifact.byte_size:
            raise ArtifactVerificationError(
                f"Artifact size differs for {artifact.path}: "
                f"{member.file_size} != {artifact.byte_size}"
            )
        with archive.open(member) as source:
            content = source.read(artifact.byte_size + 1)
        if len(content) != artifact.byte_size:
            raise ArtifactVerificationError(
                f"Artifact size differs for {artifact.path}"
            )
        digest = hashlib.sha256(content).hexdigest()
        if digest != artifact.sha256:
            raise ArtifactVerificationError(
                f"Artifact checksum differs for {artifact.path}"
            )
        target = temporary_path / PurePosixPath(artifact.path)
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_bytes(content)
