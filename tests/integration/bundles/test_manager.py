from __future__ import annotations

import hashlib
import json
import multiprocessing
import threading
import unittest
import zipfile
from collections.abc import Iterator
from contextlib import contextmanager
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from tempfile import TemporaryDirectory
from typing import ClassVar

from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey


def _hold_file_lock(
    lock_path: str,
    ready: multiprocessing.synchronize.Event,
    release: multiprocessing.synchronize.Event,
) -> None:
    from filelock import FileLock

    with FileLock(lock_path):
        ready.set()
        release.wait(10)


class RangeHandler(BaseHTTPRequestHandler):
    content = b""
    ranges: ClassVar[list[str | None]] = []
    ignore_range = False
    wrong_content_range = False

    def do_GET(self) -> None:
        range_header = self.headers.get("Range")
        type(self).ranges.append(range_header)
        start = 0
        status = 200
        if range_header and not type(self).ignore_range:
            start = int(range_header.removeprefix("bytes=").removesuffix("-"))
            status = 206
        payload = type(self).content[start:]
        self.send_response(status)
        self.send_header("Content-Length", str(len(payload)))
        if status == 206:
            total_size = len(type(self).content)
            reported_start = start + 1 if type(self).wrong_content_range else start
            self.send_header(
                "Content-Range",
                f"bytes {reported_start}-{total_size - 1}/{total_size}",
            )
        self.end_headers()
        self.wfile.write(payload)

    def log_message(self, format: str, *args: object) -> None:
        return


class BundleManagerTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temporary_directory = TemporaryDirectory()
        self.addCleanup(self.temporary_directory.cleanup)
        self.root = Path(self.temporary_directory.name)
        self.bundle_root = self.root / "data"
        self.content = b"bundle knowledge"
        self.archive_bytes = self._archive_bytes(self.content)
        self.private_key = Ed25519PrivateKey.generate()
        self.public_key = self.private_key.public_key().public_bytes(
            encoding=serialization.Encoding.Raw,
            format=serialization.PublicFormat.Raw,
        )
        self.manifest = self._manifest("2026.7.1", self.content)

    @staticmethod
    def _archive_bytes(content: bytes) -> bytes:
        import io

        buffer = io.BytesIO()
        with zipfile.ZipFile(buffer, "w", zipfile.ZIP_DEFLATED) as archive:
            archive.writestr("knowledge.sqlite3", content)
        return buffer.getvalue()

    @staticmethod
    def _manifest(version: str, content: bytes):
        from acmg_classifier.infrastructure.bundles.manifest import BundleManifest

        return BundleManifest.model_validate(
            {
                "format_version": "1.0",
                "bundle_version": version,
                "application_version": {"minimum": "0.1.0", "maximum": "0.9.9"},
                "schema_version": {"minimum": "1.0", "maximum": "1.2"},
                "created_at": "2026-07-11T08:00:00Z",
                "channel": "stable",
                "artifacts": [
                    {
                        "path": "knowledge.sqlite3",
                        "byte_size": len(content),
                        "sha256": hashlib.sha256(content).hexdigest(),
                        "media_type": "application/vnd.sqlite3",
                        "role": "knowledge",
                    }
                ],
                "sources": [
                    {
                        "name": "test source",
                        "release": "1.0",
                        "url": "https://example.test/source",
                        "retrieved_at": "2026-07-11T07:00:00Z",
                        "license": "test-only fixture",
                        "transformation_version": "1.0.0",
                    }
                ],
                "genome_builds": ["GRCh38"],
                "transcript_release": "test",
                "rulesets": [{"identifier": "acmg-amp", "version": "2015.1"}],
                "signer_key_id": "test-key",
                "signature_algorithm": "Ed25519",
            }
        )

    def _signature(self, manifest=None) -> bytes:
        from acmg_classifier.infrastructure.bundles.verifier import (
            canonical_manifest_bytes,
        )

        selected = manifest or self.manifest
        return self.private_key.sign(canonical_manifest_bytes(selected))

    @contextmanager
    def _server(self, content: bytes) -> Iterator[str]:
        RangeHandler.content = content
        RangeHandler.ranges = []
        RangeHandler.ignore_range = False
        RangeHandler.wrong_content_range = False
        server = ThreadingHTTPServer(("127.0.0.1", 0), RangeHandler)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        try:
            yield f"http://127.0.0.1:{server.server_port}/bundle.zip"
        finally:
            server.shutdown()
            server.server_close()
            thread.join(timeout=2)

    def _manager(self, *, lock_timeout: float = 2):
        return self._manager_for_root(self.bundle_root, lock_timeout=lock_timeout)

    def _manager_for_root(
        self,
        root: Path,
        *,
        lock_timeout: float = 2,
        transport=None,
    ):
        from acmg_classifier.infrastructure.bundles.manager import BundleManager
        from acmg_classifier.infrastructure.bundles.verifier import BundleVerifier

        return BundleManager(
            root,
            verifier=BundleVerifier({"test-key": self.public_key}),
            transport=transport,
            lock_timeout=lock_timeout,
        )

    def test_first_install_is_verified_activated_and_idempotent(self) -> None:
        progress = []
        with self._server(self.archive_bytes) as url:
            first = self._manager().install(
                archive_url=url,
                manifest=self.manifest,
                signature=self._signature(),
                progress=progress.append,
            )
            second = self._manager().install(
                archive_url=url,
                manifest=self.manifest,
                signature=self._signature(),
            )

        self.assertEqual(first, second)
        self.assertEqual(first.active_version, "2026.7.1")
        self.assertEqual(
            (self.bundle_root / "bundles/2026.7.1/knowledge.sqlite3").read_bytes(),
            self.content,
        )
        pointer = json.loads((self.bundle_root / "active-bundle.json").read_text())
        self.assertEqual(pointer["active_version"], "2026.7.1")
        self.assertTrue(progress)
        self.assertEqual(RangeHandler.ranges, [None])

    def test_partial_download_resumes_with_http_range(self) -> None:
        partial = self.bundle_root / "downloads/2026.7.1.zip.part"
        partial.parent.mkdir(parents=True)
        split = len(self.archive_bytes) // 2
        partial.write_bytes(self.archive_bytes[:split])

        with self._server(self.archive_bytes) as url:
            result = self._manager().install(
                archive_url=url,
                manifest=self.manifest,
                signature=self._signature(),
            )

        self.assertEqual(result.active_version, "2026.7.1")
        self.assertEqual(RangeHandler.ranges, [f"bytes={split}-"])
        self.assertFalse(partial.exists())

    def test_range_fallback_protocol_error_and_download_limit_are_safe(self) -> None:
        from acmg_classifier.infrastructure.bundles.transport import (
            DownloadProtocolError,
            DownloadTooLargeError,
            HttpDownloadTransport,
        )

        partial = self.bundle_root / "downloads/2026.7.1.zip.part"
        partial.parent.mkdir(parents=True)
        split = len(self.archive_bytes) // 2
        partial.write_bytes(self.archive_bytes[:split])

        with self._server(self.archive_bytes) as url:
            RangeHandler.ignore_range = True
            result = self._manager().install(url, self.manifest, self._signature())
        self.assertEqual(result.active_version, "2026.7.1")

        shutil_root = self.bundle_root / "second"
        manager = self._manager_for_root(shutil_root)
        second_partial = shutil_root / "downloads/2026.7.1.zip.part"
        second_partial.parent.mkdir(parents=True)
        second_partial.write_bytes(self.archive_bytes[:split])
        with self._server(self.archive_bytes) as url:
            RangeHandler.wrong_content_range = True
            with self.assertRaises(DownloadProtocolError):
                manager.install(url, self.manifest, self._signature())
        self.assertEqual(second_partial.stat().st_size, split)

        limited_root = self.bundle_root / "limited"
        limited = self._manager_for_root(
            limited_root,
            transport=HttpDownloadTransport(max_download_bytes=3),
        )
        with (
            self._server(self.archive_bytes) as url,
            self.assertRaises(DownloadTooLargeError),
        ):
            limited.install(url, self.manifest, self._signature())
        self.assertIsNone(limited.status().active_version)

    def test_failed_update_preserves_previous_active_bundle(self) -> None:
        from acmg_classifier.infrastructure.bundles.verifier import ArchiveSafetyError

        with self._server(self.archive_bytes) as url:
            self._manager().install(url, self.manifest, self._signature())

        next_manifest = self._manifest("2026.8.0", b"new content")
        with (
            self._server(b"corrupt archive") as url,
            self.assertRaises(ArchiveSafetyError),
        ):
            self._manager().install(url, next_manifest, self._signature(next_manifest))

        status = self._manager().status()
        self.assertEqual(status.active_version, "2026.7.1")
        self.assertFalse((self.bundle_root / "bundles/2026.8.0").exists())
        self.assertFalse((self.bundle_root / "downloads/2026.8.0.zip.part").exists())

    def test_pin_blocks_other_version_and_rollback_switches_atomically(self) -> None:
        from acmg_classifier.infrastructure.bundles.manager import BundlePinnedError

        with self._server(self.archive_bytes) as url:
            manager = self._manager()
            manager.install(url, self.manifest, self._signature())
        manager.pin("2026.7.1")

        next_content = b"next bundle"
        next_manifest = self._manifest("2026.8.0", next_content)
        with self._server(self._archive_bytes(next_content)) as url:
            with self.assertRaises(BundlePinnedError):
                manager.install(url, next_manifest, self._signature(next_manifest))
            manager.unpin()
            manager.install(url, next_manifest, self._signature(next_manifest))

        self.assertEqual(manager.status().active_version, "2026.8.0")
        rolled_back = manager.rollback()
        self.assertEqual(rolled_back.active_version, "2026.7.1")

    def test_cross_process_lock_returns_specific_busy_error(self) -> None:
        from acmg_classifier.infrastructure.bundles.manager import (
            BundleInstallBusyError,
        )

        ready = multiprocessing.Event()
        release = multiprocessing.Event()
        self.bundle_root.mkdir(parents=True)
        process = multiprocessing.Process(
            target=_hold_file_lock,
            args=(str(self.bundle_root / "install.lock"), ready, release),
        )
        process.start()
        self.addCleanup(lambda: process.kill() if process.is_alive() else None)
        self.assertTrue(ready.wait(5))
        try:
            with (
                self._server(self.archive_bytes) as url,
                self.assertRaises(BundleInstallBusyError),
            ):
                self._manager(lock_timeout=0.05).install(
                    url,
                    self.manifest,
                    self._signature(),
                )
        finally:
            release.set()
            process.join(timeout=5)


if __name__ == "__main__":
    unittest.main()
