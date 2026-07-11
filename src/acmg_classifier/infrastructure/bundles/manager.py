"""Resumable, locked, atomic bundle installation and activation."""

import json
import os
import re
import shutil
from collections.abc import Iterator
from contextlib import contextmanager
from dataclasses import dataclass
from pathlib import Path

from filelock import FileLock, Timeout

from acmg_classifier.domain.canonical import canonical_json_bytes
from acmg_classifier.infrastructure.bundles.manifest import BundleManifest
from acmg_classifier.infrastructure.bundles.transport import (
    DownloadTransport,
    HttpDownloadTransport,
    ProgressEvent,
    ProgressSink,
)
from acmg_classifier.infrastructure.bundles.verifier import (
    BundleVerifier,
    canonical_manifest_bytes,
)


class BundleManagerError(RuntimeError):
    """Base class for expected bundle lifecycle failures."""


class BundleInstallBusyError(BundleManagerError):
    """Another process currently owns the installation lock."""


class BundlePinnedError(BundleManagerError):
    """A pin prevents activation of the requested bundle version."""


class BundleNotInstalledError(BundleManagerError):
    """The requested active, pinned, or rollback bundle is unavailable."""


class BundleStateError(BundleManagerError):
    """The active-bundle pointer is malformed."""


@dataclass(frozen=True, slots=True)
class BundleState:
    """Atomically persisted active, previous, and pinned versions."""

    active_version: str | None = None
    previous_version: str | None = None
    pinned_version: str | None = None


class BundleManager:
    """Install only verified bundles and atomically manage activation."""

    def __init__(
        self,
        root: Path,
        *,
        verifier: BundleVerifier,
        transport: DownloadTransport | None = None,
        lock_timeout: float = 30,
    ) -> None:
        if lock_timeout < 0:
            raise ValueError("lock_timeout must not be negative")
        self.root = root
        self.root.mkdir(parents=True, exist_ok=True)
        self.verifier = verifier
        self.transport = transport or HttpDownloadTransport()
        self.lock_timeout = lock_timeout

    def install(
        self,
        archive_url: str,
        manifest: BundleManifest,
        signature: bytes,
        progress: ProgressSink | None = None,
    ) -> BundleState:
        """Resume, verify, install, and activate one bundle under a file lock."""
        sink = progress or _ignore_progress
        with self._lock():
            return self._install_locked(archive_url, manifest, signature, sink)

    def status(self) -> BundleState:
        """Return the current atomic activation state."""
        pointer = self.root / "active-bundle.json"
        if not pointer.exists():
            return BundleState()
        try:
            data = json.loads(pointer.read_text(encoding="utf-8"))
            if set(data) != {"active_version", "previous_version", "pinned_version"}:
                raise ValueError("unexpected pointer fields")
            return BundleState(**data)
        except (OSError, TypeError, ValueError, json.JSONDecodeError) as error:
            raise BundleStateError("active-bundle.json is invalid") from error

    def pin(self, version: str) -> BundleState:
        """Pin future activation to one already installed version."""
        with self._lock():
            self._require_installed(version)
            state = self.status()
            updated = BundleState(state.active_version, state.previous_version, version)
            self._write_state(updated)
            return updated

    def unpin(self) -> BundleState:
        """Remove the bundle activation pin."""
        with self._lock():
            state = self.status()
            updated = BundleState(state.active_version, state.previous_version, None)
            self._write_state(updated)
            return updated

    def rollback(self) -> BundleState:
        """Atomically swap active and previous installed bundles."""
        with self._lock():
            return self._rollback_locked()

    def _rollback_locked(self) -> BundleState:
        state = self.status()
        if state.previous_version is None:
            raise BundleNotInstalledError("No previous bundle is available")
        self._require_installed(state.previous_version)
        if state.pinned_version not in (None, state.previous_version):
            raise BundlePinnedError(
                f"Bundle is pinned to {state.pinned_version}, "
                f"not {state.previous_version}"
            )
        updated = BundleState(
            active_version=state.previous_version,
            previous_version=state.active_version,
            pinned_version=state.pinned_version,
        )
        self._write_state(updated)
        return updated

    @contextmanager
    def _lock(self) -> Iterator[None]:
        try:
            with FileLock(self.root / "install.lock", timeout=self.lock_timeout):
                yield
        except Timeout as error:
            raise BundleInstallBusyError(
                "Another bundle installation is active"
            ) from error

    def _install_locked(
        self,
        archive_url: str,
        manifest: BundleManifest,
        signature: bytes,
        progress: ProgressSink,
    ) -> BundleState:
        state = self.status()
        version = manifest.bundle_version
        if state.pinned_version not in (None, version):
            raise BundlePinnedError(
                f"Bundle is pinned to {state.pinned_version}, not {version}"
            )
        installed_path = self._bundle_path(version)
        if installed_path.is_dir():
            return self._activate(version, state)

        partial_path = self.root / "downloads" / f"{version}.zip.part"
        offset = partial_path.stat().st_size if partial_path.exists() else 0
        try:
            self.transport.download(
                archive_url,
                partial_path,
                offset=offset,
                progress=progress,
            )
        except Exception:
            partial_path.unlink(missing_ok=True)
            raise

        staging_path = self.root / "staging" / version
        shutil.rmtree(staging_path, ignore_errors=True)
        try:
            verified = self.verifier.verify_and_extract(
                partial_path,
                manifest,
                signature,
                staging_path,
            )
            (verified.staging_path / "manifest.json").write_bytes(
                canonical_manifest_bytes(manifest)
            )
            (verified.staging_path / "manifest.sig").write_bytes(signature)
            installed_path.parent.mkdir(parents=True, exist_ok=True)
            os.replace(verified.staging_path, installed_path)
        except Exception:
            partial_path.unlink(missing_ok=True)
            shutil.rmtree(staging_path, ignore_errors=True)
            raise
        partial_path.unlink(missing_ok=True)
        progress(ProgressEvent("activate", 1, 1))
        return self._activate(version, state)

    def _activate(self, version: str, state: BundleState) -> BundleState:
        previous = state.previous_version
        if state.active_version not in (None, version):
            previous = state.active_version
        updated = BundleState(version, previous, state.pinned_version)
        self._write_state(updated)
        return updated

    def _write_state(self, state: BundleState) -> None:
        pointer = self.root / "active-bundle.json"
        temporary = pointer.with_suffix(".json.tmp")
        content = canonical_json_bytes(
            {
                "active_version": state.active_version,
                "previous_version": state.previous_version,
                "pinned_version": state.pinned_version,
            }
        )
        with temporary.open("wb") as output:
            output.write(content)
            output.flush()
            os.fsync(output.fileno())
        os.replace(temporary, pointer)

    def _bundle_path(self, version: str) -> Path:
        if not re.fullmatch(r"\d+(?:\.\d+){1,2}", version):
            raise ValueError("Bundle version must be numeric")
        return self.root / "bundles" / version

    def _require_installed(self, version: str) -> None:
        if not self._bundle_path(version).is_dir():
            raise BundleNotInstalledError(f"Bundle is not installed: {version}")


def _ignore_progress(event: ProgressEvent) -> None:
    del event
    return
