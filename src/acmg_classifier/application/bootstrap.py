"""Presentation-neutral first-use readiness orchestration."""

from __future__ import annotations

import shutil
import threading
from collections.abc import Callable
from dataclasses import dataclass, field
from enum import StrEnum
from pathlib import Path
from typing import Protocol

from platformdirs import PlatformDirs

from acmg_classifier.domain.enums import GenomeBuild
from acmg_classifier.infrastructure.bundles.manager import (
    BundleInstallBusyError,
    BundleManagerError,
    BundlePinnedError,
    BundleStateError,
)
from acmg_classifier.infrastructure.bundles.resolver import BundleRequirements
from acmg_classifier.infrastructure.bundles.transport import (
    DownloadTransportError,
    ProgressEvent,
)
from acmg_classifier.infrastructure.bundles.verifier import BundleVerificationError
from acmg_classifier.infrastructure.storage.sqlite import (
    MigrationError,
    SQLiteStateStore,
    StateDatabaseInfo,
    StateStoreBusyError,
    StateStoreError,
)
from acmg_classifier.version import __version__


class BootstrapIssueCode(StrEnum):
    """Stable readiness failures suitable for a presentation boundary."""

    INSUFFICIENT_DISK = "INSUFFICIENT_DISK"
    STATE_UNAVAILABLE = "STATE_UNAVAILABLE"
    STATE_BUSY = "STATE_BUSY"
    MIGRATION_FAILED = "MIGRATION_FAILED"
    BUNDLE_UNAVAILABLE = "BUNDLE_UNAVAILABLE"
    BUNDLE_BUSY = "BUNDLE_BUSY"
    BUNDLE_CORRUPT = "BUNDLE_CORRUPT"
    BUNDLE_INCOMPATIBLE = "BUNDLE_INCOMPATIBLE"
    NETWORK_UNAVAILABLE = "NETWORK_UNAVAILABLE"
    BOOTSTRAP_FAILED = "BOOTSTRAP_FAILED"


class BootstrapPhase(StrEnum):
    """Stable, presentation-neutral bootstrap progress phases."""

    PREPARING_DIRECTORIES = "preparing_directories"
    CHECKING_DISK = "checking_disk"
    INITIALIZING_STATE = "initializing_state"
    WAITING_FOR_BUNDLE = "waiting_for_bundle"
    DOWNLOADING_BUNDLE = "downloading_bundle"
    VERIFYING_BUNDLE = "verifying_bundle"
    ACTIVATING_BUNDLE = "activating_bundle"
    INSPECTING_BUNDLE = "inspecting_bundle"
    READY = "ready"


@dataclass(frozen=True, slots=True)
class ApplicationPaths:
    """Directories solely owned by the application runtime."""

    config_directory: Path
    state_directory: Path
    cache_directory: Path
    bundle_directory: Path

    @property
    def state_database_path(self) -> Path:
        """Return the single application state database location."""
        return self.state_directory / "state.sqlite3"


def default_application_paths(app_name: str = "acmg-classifier") -> ApplicationPaths:
    """Return platform-appropriate locations without creating them."""
    directories = PlatformDirs(appname=app_name, appauthor=False)
    data_directory = Path(directories.user_data_dir)
    return ApplicationPaths(
        config_directory=Path(directories.user_config_dir),
        state_directory=data_directory / "state",
        cache_directory=Path(directories.user_cache_dir),
        bundle_directory=data_directory / "bundles",
    )


def default_bundle_requirements() -> BundleRequirements:
    """Return the release-one core bundle compatibility policy."""
    return BundleRequirements(
        application_version=__version__,
        schema_version="1.0",
        genome_build=GenomeBuild.GRCH38,
        ruleset_id="acmg-amp",
        ruleset_version="2015.1",
    )


@dataclass(frozen=True, slots=True)
class BootstrapSettings:
    """Injected paths and policy for one bootstrap operation."""

    paths: ApplicationPaths = field(default_factory=default_application_paths)
    bundle_requirements: BundleRequirements = field(
        default_factory=default_bundle_requirements
    )
    minimum_free_bytes: int = 256 * 1024 * 1024
    progress_after_seconds: float = 2.0
    repair_command: str = "acmg doctor --repair"

    def __post_init__(self) -> None:
        if self.minimum_free_bytes < 0:
            raise ValueError("minimum_free_bytes must not be negative")
        if self.progress_after_seconds < 0:
            raise ValueError("progress_after_seconds must not be negative")


@dataclass(frozen=True, slots=True)
class BundleReadiness:
    """A provisioner's non-presentation readiness result."""

    is_ready: bool
    bundle_version: str | None
    corrupt_reason: str | None = None
    incompatible_reasons: tuple[str, ...] = ()
    unavailable_reason: str | None = None

    @classmethod
    def ready(cls, bundle_version: str) -> BundleReadiness:
        """Build a successful compatible-bundle result."""
        return cls(is_ready=True, bundle_version=bundle_version)

    @classmethod
    def corrupt(cls, reason: str) -> BundleReadiness:
        """Build a fail-closed integrity result."""
        return cls(is_ready=False, bundle_version=None, corrupt_reason=reason)

    @classmethod
    def incompatible(cls, reasons: tuple[str, ...]) -> BundleReadiness:
        """Build a compatibility-policy failure result."""
        return cls(is_ready=False, bundle_version=None, incompatible_reasons=reasons)

    @classmethod
    def unavailable(cls, reason: str) -> BundleReadiness:
        """Build an unavailable-bundle result without implying corruption."""
        return cls(is_ready=False, bundle_version=None, unavailable_reason=reason)


class BundleProvisioningError(RuntimeError):
    """Expected bundle provisioning failure safe to report to users."""


class BundleProvisioner(Protocol):
    """Narrow application port for compatible core-bundle lifecycle work."""

    def ensure_compatible(
        self,
        requirements: BundleRequirements,
        progress: Callable[[ProgressEvent], None],
    ) -> BundleReadiness: ...

    def inspect(self, requirements: BundleRequirements) -> BundleReadiness: ...

    def repair_compatible(
        self,
        requirements: BundleRequirements,
        progress: Callable[[ProgressEvent], None],
    ) -> BundleReadiness: ...


class StateStore(Protocol):
    """Minimal state initialization dependency."""

    def initialize(self) -> StateDatabaseInfo: ...


class TimerLike(Protocol):
    """Minimal delayed-progress dependency."""

    def start(self) -> None: ...

    def cancel(self) -> None: ...


@dataclass(frozen=True, slots=True)
class BootstrapIssue:
    """Actionable typed readiness diagnosis."""

    code: BootstrapIssueCode
    message: str
    repair_action: str | None = None
    retryable: bool = False
    component: str | None = None
    details: tuple[str, ...] = ()


@dataclass(frozen=True, slots=True)
class BootstrapProgressEvent:
    """Progress data independent of CLI, MCP, or logging frameworks."""

    phase: BootstrapPhase
    message: str
    completed_bytes: int | None = None
    total_bytes: int | None = None


@dataclass(frozen=True, slots=True)
class BootstrapReport:
    """Readiness result after initializing state and inspecting a core bundle."""

    ready: bool
    paths: ApplicationPaths
    state_database: StateDatabaseInfo | None
    bundle_version: str | None
    issue: BootstrapIssue | None = None


class BootstrapService:
    """Prepare local state and delegate bundle lifecycle to a narrow port."""

    def __init__(
        self,
        settings: BootstrapSettings,
        provisioner: BundleProvisioner,
        *,
        state_store_factory: Callable[[Path], StateStore] = SQLiteStateStore,
        free_space_bytes: Callable[[Path], int] | None = None,
        timer_factory: Callable[
            [float, Callable[[], None]], TimerLike
        ] = threading.Timer,
    ) -> None:
        self._settings = settings
        self._provisioner = provisioner
        self._state_store_factory = state_store_factory
        self._free_space_bytes = free_space_bytes or _free_space_bytes
        self._timer_factory = timer_factory

    def ensure_ready(
        self,
        progress: Callable[[BootstrapProgressEvent], None] | None = None,
    ) -> BootstrapReport:
        """Initialize state then install or reuse a compatible core bundle."""
        state, failure = self._prepare(progress)
        if failure is not None:
            return failure
        assert state is not None
        return self._with_waiting_progress(
            progress,
            lambda bundle_progress: self._bundle_report(
                state,
                self._provisioner.ensure_compatible,
                bundle_progress,
            ),
        )

    def doctor(
        self,
        *,
        repair: bool = False,
        progress: Callable[[BootstrapProgressEvent], None] | None = None,
    ) -> BootstrapReport:
        """Inspect readiness without mutation, or repair with identical policy."""
        state, failure = self._prepare(progress)
        if failure is not None:
            return failure
        assert state is not None
        if not repair:
            self._emit(
                progress, BootstrapPhase.INSPECTING_BUNDLE, "Inspecting data bundle"
            )
            try:
                readiness = self._provisioner.inspect(
                    self._settings.bundle_requirements
                )
            except Exception as error:  # mapped into the typed doctor boundary
                return self._provisioning_exception_report(state, error)
            return self._readiness_report(state, readiness)
        return self._with_waiting_progress(
            progress,
            lambda bundle_progress: self._bundle_report(
                state,
                self._provisioner.repair_compatible,
                bundle_progress,
            ),
        )

    def _prepare(
        self,
        progress: Callable[[BootstrapProgressEvent], None] | None,
    ) -> tuple[StateDatabaseInfo | None, BootstrapReport | None]:
        self._emit(
            progress,
            BootstrapPhase.PREPARING_DIRECTORIES,
            "Preparing application directories",
        )
        try:
            self._create_owned_directories()
        except OSError as error:
            return None, self._failure(
                BootstrapIssueCode.STATE_UNAVAILABLE,
                "Application directories are unavailable",
                component="state",
                details=(str(error),),
            )
        self._emit(
            progress, BootstrapPhase.CHECKING_DISK, "Checking available disk space"
        )
        try:
            free_bytes = self._free_space_bytes(self._settings.paths.state_directory)
        except OSError as error:
            return None, self._failure(
                BootstrapIssueCode.STATE_UNAVAILABLE,
                "Available disk space could not be determined",
                component="state",
                details=(str(error),),
            )
        if free_bytes < self._settings.minimum_free_bytes:
            return None, self._failure(
                BootstrapIssueCode.INSUFFICIENT_DISK,
                "Insufficient disk space for application data",
                component="state",
                details=(f"required={self._settings.minimum_free_bytes}",),
            )
        self._emit(
            progress, BootstrapPhase.INITIALIZING_STATE, "Initializing local state"
        )
        try:
            return self._state_store_factory(
                self._settings.paths.state_database_path
            ).initialize(), None
        except StateStoreBusyError as error:
            return None, self._failure(
                BootstrapIssueCode.STATE_BUSY,
                "Local state is busy",
                retryable=True,
                component="state",
                details=(str(error),),
            )
        except MigrationError as error:
            return None, self._failure(
                BootstrapIssueCode.MIGRATION_FAILED,
                "Local state migration failed",
                component="state",
                details=(str(error),),
            )
        except (StateStoreError, OSError) as error:
            return None, self._failure(
                BootstrapIssueCode.STATE_UNAVAILABLE,
                "Local state is unavailable",
                component="state",
                details=(str(error),),
            )

    def _bundle_report(
        self,
        state: StateDatabaseInfo,
        operation: Callable[
            [BundleRequirements, Callable[[ProgressEvent], None]], BundleReadiness
        ],
        progress: Callable[[ProgressEvent], None],
    ) -> BootstrapReport:
        try:
            readiness = operation(self._settings.bundle_requirements, progress)
        except Exception as error:  # mapped at the application error boundary
            return self._provisioning_exception_report(state, error)
        return self._readiness_report(state, readiness)

    def _readiness_report(
        self,
        state: StateDatabaseInfo,
        readiness: BundleReadiness,
    ) -> BootstrapReport:
        if readiness.is_ready and readiness.bundle_version is not None:
            return BootstrapReport(
                ready=True,
                paths=self._settings.paths,
                state_database=state,
                bundle_version=readiness.bundle_version,
            )
        if readiness.corrupt_reason is not None:
            return self._failure(
                BootstrapIssueCode.BUNDLE_CORRUPT,
                "Installed data bundle is corrupt",
                state=state,
                component="bundle",
                details=(readiness.corrupt_reason,),
            )
        if readiness.incompatible_reasons:
            return self._failure(
                BootstrapIssueCode.BUNDLE_INCOMPATIBLE,
                "No installed data bundle satisfies this application",
                state=state,
                component="bundle",
                details=readiness.incompatible_reasons,
            )
        return self._failure(
            BootstrapIssueCode.BUNDLE_UNAVAILABLE,
            "No compatible data bundle is available",
            state=state,
            component="bundle",
            details=(
                (readiness.unavailable_reason,)
                if readiness.unavailable_reason is not None
                else ()
            ),
        )

    def _provisioning_exception_report(
        self,
        state: StateDatabaseInfo,
        error: Exception,
    ) -> BootstrapReport:
        if isinstance(error, BundleInstallBusyError):
            return self._failure(
                BootstrapIssueCode.BUNDLE_BUSY,
                "Data bundle installation is busy",
                state=state,
                retryable=True,
                component="bundle",
            )
        if isinstance(error, BundlePinnedError):
            return self._failure(
                BootstrapIssueCode.BUNDLE_INCOMPATIBLE,
                "Data bundle pin prevents activation",
                state=state,
                component="bundle",
            )
        if isinstance(error, (BundleStateError, BundleVerificationError)):
            return self._failure(
                BootstrapIssueCode.BUNDLE_CORRUPT,
                "Data bundle integrity verification failed",
                state=state,
                component="bundle",
            )
        if isinstance(error, BundleManagerError):
            return self._failure(
                BootstrapIssueCode.BUNDLE_UNAVAILABLE,
                "Data bundle is unavailable",
                state=state,
                component="bundle",
            )
        if isinstance(error, DownloadTransportError):
            return self._failure(
                BootstrapIssueCode.NETWORK_UNAVAILABLE,
                "Data bundle could not be downloaded",
                state=state,
                retryable=True,
                component="bundle",
            )
        if isinstance(error, BundleProvisioningError):
            return self._failure(
                BootstrapIssueCode.BOOTSTRAP_FAILED,
                "Data bundle provisioning failed",
                state=state,
                component="bundle",
            )
        if isinstance(error, OSError):
            return self._failure(
                BootstrapIssueCode.NETWORK_UNAVAILABLE,
                "Data bundle could not be reached",
                state=state,
                retryable=True,
                component="bundle",
            )
        return self._failure(
            BootstrapIssueCode.BOOTSTRAP_FAILED,
            "Bootstrap failed while preparing the data bundle",
            state=state,
            component="bundle",
        )

    def _failure(
        self,
        code: BootstrapIssueCode,
        message: str,
        *,
        state: StateDatabaseInfo | None = None,
        retryable: bool = False,
        component: str | None = None,
        details: tuple[str, ...] = (),
    ) -> BootstrapReport:
        return BootstrapReport(
            ready=False,
            paths=self._settings.paths,
            state_database=state,
            bundle_version=None,
            issue=BootstrapIssue(
                code=code,
                message=message,
                repair_action=self._settings.repair_command,
                retryable=retryable,
                component=component,
                details=details,
            ),
        )

    def _create_owned_directories(self) -> None:
        for directory in (
            self._settings.paths.config_directory,
            self._settings.paths.state_directory,
            self._settings.paths.cache_directory,
            self._settings.paths.bundle_directory,
        ):
            directory.mkdir(parents=True, exist_ok=True)

    def _with_waiting_progress(
        self,
        progress: Callable[[BootstrapProgressEvent], None] | None,
        operation: Callable[[Callable[[ProgressEvent], None]], BootstrapReport],
    ) -> BootstrapReport:
        timer = self._timer_factory(
            self._settings.progress_after_seconds,
            lambda: self._emit(
                progress,
                BootstrapPhase.WAITING_FOR_BUNDLE,
                "Preparing data bundle",
            ),
        )
        timer.start()
        try:
            return operation(lambda event: self._adapt_bundle_progress(progress, event))
        finally:
            timer.cancel()

    def _adapt_bundle_progress(
        self,
        progress: Callable[[BootstrapProgressEvent], None] | None,
        event: ProgressEvent,
    ) -> None:
        phase = {
            "download": BootstrapPhase.DOWNLOADING_BUNDLE,
            "verify": BootstrapPhase.VERIFYING_BUNDLE,
            "activate": BootstrapPhase.ACTIVATING_BUNDLE,
        }.get(event.phase, BootstrapPhase.DOWNLOADING_BUNDLE)
        self._emit(
            progress,
            phase,
            "Preparing data bundle",
            completed_bytes=event.completed_bytes,
            total_bytes=event.total_bytes,
        )

    @staticmethod
    def _emit(
        progress: Callable[[BootstrapProgressEvent], None] | None,
        phase: BootstrapPhase,
        message: str,
        *,
        completed_bytes: int | None = None,
        total_bytes: int | None = None,
    ) -> None:
        if progress is not None:
            progress(
                BootstrapProgressEvent(
                    phase=phase,
                    message=message,
                    completed_bytes=completed_bytes,
                    total_bytes=total_bytes,
                )
            )


def _free_space_bytes(path: Path) -> int:
    return shutil.disk_usage(path).free
