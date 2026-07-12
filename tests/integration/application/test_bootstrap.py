from __future__ import annotations

import hashlib
import json
import sqlite3
import zipfile
from collections.abc import Callable
from contextlib import closing
from dataclasses import dataclass
from pathlib import Path


@dataclass
class _Provisioner:
    result: object
    ensure_calls: int = 0
    inspect_calls: int = 0
    repair_calls: int = 0

    def ensure_compatible(self, _requirements: object, _progress: object) -> object:
        self.ensure_calls += 1
        if isinstance(self.result, Exception):
            raise self.result
        return self.result

    def inspect(self, _requirements: object) -> object:
        self.inspect_calls += 1
        if isinstance(self.result, Exception):
            raise self.result
        return self.result

    def repair_compatible(self, _requirements: object, _progress: object) -> object:
        self.repair_calls += 1
        if isinstance(self.result, Exception):
            raise self.result
        return self.result


class _ImmediateTimer:
    def __init__(self, _seconds: float, callback: Callable[[], None]) -> None:
        self._callback = callback

    def start(self) -> None:
        self._callback()

    def cancel(self) -> None:
        return


@dataclass
class _LocalArchiveTransport:
    archive_path: Path
    download_calls: int = 0

    def download(
        self,
        _url: str,
        destination: Path,
        *,
        offset: int,
        progress: object,
    ) -> None:
        from acmg_classifier.infrastructure.bundles.transport import ProgressEvent

        assert offset == 0
        self.download_calls += 1
        destination.parent.mkdir(parents=True, exist_ok=True)
        payload = self.archive_path.read_bytes()
        destination.write_bytes(payload)
        progress(ProgressEvent("download", len(payload), len(payload)))


@dataclass
class _Catalog:
    candidate: object | None
    calls: int = 0

    def candidates(self) -> tuple[object, ...]:
        self.calls += 1
        return () if self.candidate is None else (self.candidate,)


def _real_bundle_provisioner(tmp_path: Path):
    from cryptography.hazmat.primitives import serialization
    from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey

    from acmg_classifier.infrastructure.bundles.manager import BundleManager
    from acmg_classifier.infrastructure.bundles.manifest import BundleManifest
    from acmg_classifier.infrastructure.bundles.provisioner import (
        BundleCandidate,
        BundleManagerProvisioner,
    )
    from acmg_classifier.infrastructure.bundles.verifier import (
        BundleVerifier,
        canonical_manifest_bytes,
    )

    knowledge_path = tmp_path / "knowledge.sqlite3"
    with closing(sqlite3.connect(knowledge_path)) as connection, connection:
        connection.execute("CREATE TABLE knowledge (identifier TEXT PRIMARY KEY)")
        connection.execute("INSERT INTO knowledge VALUES ('fixture')")
    knowledge_bytes = knowledge_path.read_bytes()
    archive_path = tmp_path / "bundle.zip"
    with zipfile.ZipFile(archive_path, "w", zipfile.ZIP_DEFLATED) as archive:
        archive.writestr("knowledge.sqlite3", knowledge_bytes)
    private_key = Ed25519PrivateKey.generate()
    public_key = private_key.public_key().public_bytes(
        encoding=serialization.Encoding.Raw,
        format=serialization.PublicFormat.Raw,
    )
    manifest = BundleManifest.model_validate(
        {
            "format_version": "1.0",
            "bundle_version": "2026.7.10",
            "application_version": {"minimum": "0.1.0", "maximum": "0.9.9"},
            "schema_version": {"minimum": "1.0", "maximum": "1.2"},
            "created_at": "2026-07-11T08:00:00Z",
            "channel": "test",
            "artifacts": [
                {
                    "path": "knowledge.sqlite3",
                    "byte_size": len(knowledge_bytes),
                    "sha256": hashlib.sha256(knowledge_bytes).hexdigest(),
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
                    "sha256": "b" * 64,
                    "terms_url": "https://example.test/terms",
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
    transport = _LocalArchiveTransport(archive_path)
    manager = BundleManager(
        tmp_path / "bundles",
        verifier=BundleVerifier({"test-key": public_key}),
        transport=transport,
    )
    candidate = BundleCandidate(
        archive_url="offline-test://bundle.zip",
        manifest=manifest,
        signature=private_key.sign(canonical_manifest_bytes(manifest)),
    )
    catalog = _Catalog(candidate)
    return BundleManagerProvisioner(manager, catalog), manager, transport, catalog


def _settings(tmp_path: Path):
    from acmg_classifier.application.bootstrap import (
        ApplicationPaths,
        BootstrapSettings,
    )

    return BootstrapSettings(
        paths=ApplicationPaths(
            config_directory=tmp_path / "config",
            state_directory=tmp_path / "state",
            cache_directory=tmp_path / "cache",
            bundle_directory=tmp_path / "bundles",
        ),
        minimum_free_bytes=1,
    )


def _ready_bundle() -> object:
    from acmg_classifier.application.bootstrap import BundleReadiness

    return BundleReadiness.ready("2026.7.10")


def test_first_run_creates_app_owned_state_and_becomes_ready(tmp_path: Path) -> None:
    from acmg_classifier.application.bootstrap import BootstrapService

    provisioner = _Provisioner(_ready_bundle())
    service = BootstrapService(_settings(tmp_path), provisioner)

    report = service.ensure_ready()

    assert report.ready
    assert report.bundle_version == "2026.7.10"
    assert report.state_database is not None
    assert report.state_database.schema_version >= 1
    assert provisioner.ensure_calls == 1
    assert (tmp_path / "state" / "state.sqlite3").is_file()
    for directory in ("config", "state", "cache", "bundles"):
        assert (tmp_path / directory).is_dir()


def test_repeat_first_use_readiness_is_idempotent(tmp_path: Path) -> None:
    from acmg_classifier.application.bootstrap import BootstrapService

    provisioner = _Provisioner(_ready_bundle())
    service = BootstrapService(_settings(tmp_path), provisioner)

    first = service.ensure_ready()
    second = service.ensure_ready()

    assert first.ready and second.ready
    assert first.state_database == second.state_database
    assert provisioner.ensure_calls == 2


def test_bootstrap_failure_preserves_created_state_and_has_repair_action(
    tmp_path: Path,
) -> None:
    from acmg_classifier.application.bootstrap import (
        BootstrapService,
        BundleProvisioningError,
    )

    service = BootstrapService(
        _settings(tmp_path),
        _Provisioner(BundleProvisioningError("network unavailable")),
    )

    report = service.ensure_ready()

    assert not report.ready
    assert report.issue is not None
    assert report.issue.code == "BOOTSTRAP_FAILED"
    assert report.issue.repair_action == "acmg doctor --repair"
    assert (tmp_path / "state" / "state.sqlite3").is_file()


def test_low_disk_fails_before_attempting_bundle_provisioning(tmp_path: Path) -> None:
    from acmg_classifier.application.bootstrap import BootstrapService

    provisioner = _Provisioner(_ready_bundle())
    service = BootstrapService(
        _settings(tmp_path),
        provisioner,
        free_space_bytes=lambda _path: 0,
    )

    report = service.ensure_ready()

    assert not report.ready
    assert report.issue is not None
    assert report.issue.code == "INSUFFICIENT_DISK"
    assert provisioner.ensure_calls == 0


def test_doctor_reports_corrupt_bundle_and_repair_uses_same_policy(
    tmp_path: Path,
) -> None:
    from acmg_classifier.application.bootstrap import (
        BootstrapService,
        BundleReadiness,
    )

    provisioner = _Provisioner(BundleReadiness.corrupt("manifest signature is invalid"))
    service = BootstrapService(_settings(tmp_path), provisioner)

    unhealthy = service.doctor()
    provisioner.result = _ready_bundle()
    repaired = service.doctor(repair=True)

    assert not unhealthy.ready
    assert unhealthy.issue is not None
    assert unhealthy.issue.code == "BUNDLE_CORRUPT"
    assert unhealthy.issue.repair_action == "acmg doctor --repair"
    assert repaired.ready
    assert provisioner.inspect_calls == 1
    assert provisioner.repair_calls == 1


def test_bootstrap_emits_a_waiting_progress_event_without_sleeping(
    tmp_path: Path,
) -> None:
    from acmg_classifier.application.bootstrap import BootstrapService

    progress = []
    service = BootstrapService(
        _settings(tmp_path),
        _Provisioner(_ready_bundle()),
        timer_factory=_ImmediateTimer,
    )

    report = service.ensure_ready(progress.append)

    assert report.ready
    assert any(event.phase == "waiting_for_bundle" for event in progress)


def test_bootstrap_runs_real_sqlite_migrations(tmp_path: Path) -> None:
    from acmg_classifier.application.bootstrap import BootstrapService
    from acmg_classifier.infrastructure.storage.sqlite import SQLiteStateStore

    report = BootstrapService(
        _settings(tmp_path), _Provisioner(_ready_bundle())
    ).ensure_ready()

    assert report.ready
    assert report.state_database is not None
    assert (
        report.state_database.schema_version
        == SQLiteStateStore.DEFAULT_MIGRATIONS[-1].version
    )
    assert report.state_database.journal_mode == "wal"
    assert report.state_database.foreign_keys_enabled
    with closing(sqlite3.connect(tmp_path / "state" / "state.sqlite3")) as connection:
        migration_count = connection.execute(
            "SELECT COUNT(*) FROM schema_migrations"
        ).fetchone()[0]
    assert migration_count == len(SQLiteStateStore.DEFAULT_MIGRATIONS)


def test_bootstrap_installs_and_reuses_a_real_signed_bundle_manager_bundle(
    tmp_path: Path,
) -> None:
    from acmg_classifier.application.bootstrap import BootstrapService

    provisioner, manager, transport, _catalog = _real_bundle_provisioner(tmp_path)
    service = BootstrapService(_settings(tmp_path), provisioner)

    first = service.ensure_ready()
    second = service.ensure_ready()

    assert first.ready and second.ready
    assert first.bundle_version == "2026.7.10"
    assert transport.download_calls == 1
    assert (
        json.loads((manager.root / "active-bundle.json").read_text())["active_version"]
        == "2026.7.10"
    )
    installed_knowledge = manager.root / "bundles" / "2026.7.10" / "knowledge.sqlite3"
    with closing(
        sqlite3.connect(f"file:{installed_knowledge}?mode=ro", uri=True)
    ) as connection:
        assert connection.execute("SELECT identifier FROM knowledge").fetchone() == (
            "fixture",
        )


def test_doctor_inspection_is_non_mutating_and_never_uses_catalog(
    tmp_path: Path,
) -> None:
    from acmg_classifier.application.bootstrap import BootstrapService

    provisioner, manager, transport, catalog = _real_bundle_provisioner(tmp_path)
    service = BootstrapService(_settings(tmp_path), provisioner)
    assert service.ensure_ready().ready
    pointer = manager.root / "active-bundle.json"
    pointer_before = pointer.read_bytes()
    catalog_calls_before = catalog.calls
    downloads_before = transport.download_calls

    report = service.doctor()

    assert report.ready
    assert pointer.read_bytes() == pointer_before
    assert catalog.calls == catalog_calls_before
    assert transport.download_calls == downloads_before


def test_offline_catalog_injection_cannot_download_or_open_a_socket(
    tmp_path: Path,
) -> None:
    from acmg_classifier.application.bootstrap import BootstrapService
    from acmg_classifier.infrastructure.bundles.manager import BundleManager
    from acmg_classifier.infrastructure.bundles.provisioner import (
        BundleManagerProvisioner,
    )
    from acmg_classifier.infrastructure.bundles.verifier import BundleVerifier

    class _NoNetworkCatalog:
        calls = 0

        def candidates(self) -> tuple[object, ...]:
            self.calls += 1
            return ()

    catalog = _NoNetworkCatalog()
    manager = BundleManager(
        tmp_path / "bundles",
        verifier=BundleVerifier({}),
        transport=_LocalArchiveTransport(tmp_path / "must-not-exist.zip"),
    )
    report = BootstrapService(
        _settings(tmp_path),
        BundleManagerProvisioner(manager, catalog),
    ).ensure_ready()

    assert not report.ready
    assert report.issue is not None
    assert report.issue.code == "BUNDLE_UNAVAILABLE"
    assert catalog.calls == 1
    assert not (tmp_path / "must-not-exist.zip").exists()


def test_real_corrupt_bundle_doctor_reports_then_repairs_with_same_policy(
    tmp_path: Path,
) -> None:
    from acmg_classifier.application.bootstrap import BootstrapService

    provisioner, manager, _transport, _catalog = _real_bundle_provisioner(tmp_path)
    service = BootstrapService(_settings(tmp_path), provisioner)
    assert service.ensure_ready().ready
    artifact = manager.root / "bundles" / "2026.7.10" / "knowledge.sqlite3"
    artifact.write_bytes(b"tampered")

    unhealthy = service.doctor()
    repaired = service.doctor(repair=True)

    assert not unhealthy.ready
    assert unhealthy.issue is not None
    assert unhealthy.issue.code == "BUNDLE_CORRUPT"
    assert unhealthy.issue.repair_action == "acmg doctor --repair"
    assert repaired.ready


def test_real_tampered_bundle_signature_is_rejected_and_repaired(
    tmp_path: Path,
) -> None:
    from acmg_classifier.application.bootstrap import BootstrapService

    provisioner, manager, _transport, _catalog = _real_bundle_provisioner(tmp_path)
    service = BootstrapService(_settings(tmp_path), provisioner)
    assert service.ensure_ready().ready
    signature = manager.root / "bundles" / "2026.7.10" / "manifest.sig"
    signature.write_bytes(b"tampered signature")

    unhealthy = service.doctor()
    repaired = service.doctor(repair=True)

    assert not unhealthy.ready
    assert unhealthy.issue is not None
    assert unhealthy.issue.code == "BUNDLE_CORRUPT"
    assert repaired.ready
