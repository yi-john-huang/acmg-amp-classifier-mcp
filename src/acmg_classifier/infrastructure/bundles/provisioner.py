"""BundleManager adapter for the application bootstrap provisioning port."""

from __future__ import annotations

import hashlib
import shutil
import sqlite3
from collections.abc import Callable
from dataclasses import dataclass
from pathlib import Path
from typing import Protocol

from pydantic import ValidationError

from acmg_classifier.application.bootstrap import (
    BundleProvisioningError,
    BundleReadiness,
)
from acmg_classifier.infrastructure.bundles.manager import (
    BundleManager,
    BundleStateError,
)
from acmg_classifier.infrastructure.bundles.manifest import BundleManifest
from acmg_classifier.infrastructure.bundles.resolver import (
    BundleRequirements,
    select_bundle,
)
from acmg_classifier.infrastructure.bundles.transport import ProgressEvent
from acmg_classifier.infrastructure.bundles.verifier import BundleVerificationError


@dataclass(frozen=True, slots=True)
class BundleCandidate:
    """One signed, installable release offered by an injected catalog."""

    archive_url: str
    manifest: BundleManifest
    signature: bytes


class BundleCatalog(Protocol):
    """Release source boundary; implementations choose their network policy."""

    def candidates(self) -> tuple[BundleCandidate, ...]: ...


class BundleManagerProvisioner:
    """Select, install, and inspect bundles through an existing BundleManager."""

    def __init__(self, manager: BundleManager, catalog: BundleCatalog) -> None:
        self._manager = manager
        self._catalog = catalog

    def ensure_compatible(
        self,
        requirements: BundleRequirements,
        progress: Callable[[ProgressEvent], None],
    ) -> BundleReadiness:
        """Reuse a valid active bundle or select and install one candidate."""
        readiness = self.inspect(requirements)
        if readiness.is_ready:
            return readiness
        return self._install_selected(
            requirements,
            progress,
            remove_corrupt=readiness.corrupt_reason is not None,
        )

    def inspect(self, requirements: BundleRequirements) -> BundleReadiness:
        """Inspect local state only; this method never lists a catalog or downloads."""
        try:
            state = self._manager.status()
        except BundleStateError:
            return BundleReadiness.corrupt("active bundle pointer is invalid")
        if state.active_version is None:
            return BundleReadiness.unavailable("no active bundle is installed")
        bundle_path = self._manager.root / "bundles" / state.active_version
        try:
            manifest = self._load_and_validate_bundle(bundle_path)
        except BundleProvisioningError as error:
            return BundleReadiness.corrupt(str(error))
        selection = select_bundle((manifest,), requirements)
        if selection.selected is None:
            reasons = selection.rejected[0].reasons if selection.rejected else ()
            return BundleReadiness.incompatible(reasons)
        return BundleReadiness.ready(manifest.bundle_version)

    def repair_compatible(
        self,
        requirements: BundleRequirements,
        progress: Callable[[ProgressEvent], None],
    ) -> BundleReadiness:
        """Apply the same compatibility policy, replacing only a corrupt bundle."""
        readiness = self.inspect(requirements)
        if readiness.is_ready:
            return readiness
        return self._install_selected(
            requirements,
            progress,
            remove_corrupt=readiness.corrupt_reason is not None,
        )

    def _install_selected(
        self,
        requirements: BundleRequirements,
        progress: Callable[[ProgressEvent], None],
        *,
        remove_corrupt: bool,
    ) -> BundleReadiness:
        try:
            candidates = self._catalog.candidates()
            selection = select_bundle(
                tuple(candidate.manifest for candidate in candidates), requirements
            )
            if selection.selected is None:
                reasons = tuple(
                    reason
                    for rejected in selection.rejected
                    for reason in rejected.reasons
                )
                return (
                    BundleReadiness.incompatible(reasons)
                    if reasons
                    else BundleReadiness.unavailable(
                        "no bundle candidates are available"
                    )
                )
            candidate = next(
                item for item in candidates if item.manifest == selection.selected
            )
            if remove_corrupt:
                self._remove_corrupt_active_bundle(selection.selected.bundle_version)
            state = self._manager.install(
                candidate.archive_url,
                candidate.manifest,
                candidate.signature,
                progress,
            )
        except StopIteration as error:
            raise BundleProvisioningError(
                "Selected bundle is absent from catalog"
            ) from error
        if state.active_version != selection.selected.bundle_version:
            raise BundleProvisioningError("Selected bundle was not activated")
        return self.inspect(requirements)

    def _remove_corrupt_active_bundle(self, selected_version: str) -> None:
        """Remove only the known-corrupt active directory before verified reinstall."""
        state = self._manager.status()
        if state.active_version != selected_version:
            return
        bundle_path = self._manager.root / "bundles" / selected_version
        if bundle_path.is_dir():
            shutil.rmtree(bundle_path)

    def _load_and_validate_bundle(self, bundle_path: Path) -> BundleManifest:
        manifest_path = bundle_path / "manifest.json"
        signature_path = bundle_path / "manifest.sig"
        if (
            not bundle_path.is_dir()
            or not manifest_path.is_file()
            or not signature_path.is_file()
        ):
            raise BundleProvisioningError("active bundle metadata is missing")
        try:
            manifest = BundleManifest.model_validate_json(manifest_path.read_bytes())
        except (OSError, ValidationError, ValueError) as error:
            raise BundleProvisioningError(
                "active bundle manifest is invalid"
            ) from error
        try:
            signature = signature_path.read_bytes()
            if not signature:
                raise BundleProvisioningError("active bundle signature is missing")
            self._manager.verifier.verify_manifest_signature(manifest, signature)
            for artifact in manifest.artifacts:
                artifact_path = bundle_path / artifact.path
                if not artifact_path.is_file():
                    raise BundleProvisioningError(
                        f"active bundle artifact is missing: {artifact.path}"
                    )
                if artifact_path.stat().st_size != artifact.byte_size:
                    raise BundleProvisioningError(
                        f"active bundle artifact size is invalid: {artifact.path}"
                    )
                if _sha256(artifact_path) != artifact.sha256:
                    raise BundleProvisioningError(
                        f"active bundle artifact checksum is invalid: {artifact.path}"
                    )
                if artifact.role == "knowledge":
                    _verify_read_only_sqlite(artifact_path)
        except BundleVerificationError as error:
            raise BundleProvisioningError(
                "active bundle signature is invalid"
            ) from error
        except OSError as error:
            raise BundleProvisioningError("active bundle cannot be read") from error
        return manifest


def _sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        while chunk := source.read(1024 * 1024):
            digest.update(chunk)
    return digest.hexdigest()


def _verify_read_only_sqlite(path: Path) -> None:
    try:
        connection = sqlite3.connect(
            f"file:{path.as_posix()}?mode=ro&immutable=1", uri=True
        )
        try:
            connection.execute("SELECT name FROM sqlite_master LIMIT 1").fetchone()
        finally:
            connection.close()
    except sqlite3.Error as error:
        raise BundleProvisioningError(
            "knowledge artifact is not readable SQLite"
        ) from error
