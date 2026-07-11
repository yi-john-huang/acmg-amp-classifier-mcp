"""Default local composition for the installed research-use application."""

from __future__ import annotations

from dataclasses import dataclass
from typing import TYPE_CHECKING

from acmg_classifier.application.bootstrap import (
    ApplicationPaths,
    BootstrapReport,
    BootstrapService,
    BootstrapSettings,
    default_application_paths,
)
from acmg_classifier.application.classification import (
    ClassificationRequest,
    FailedClassificationResponse,
)
from acmg_classifier.application.drafts import DraftAnswer
from acmg_classifier.application.feedback import FeedbackService
from acmg_classifier.application.reinterpretation import ReinterpretationService
from acmg_classifier.domain.enums import WorkflowStatus
from acmg_classifier.infrastructure.bundles.manager import BundleManager
from acmg_classifier.infrastructure.bundles.provisioner import (
    BundleCandidate,
    BundleCatalog,
    BundleManagerProvisioner,
)
from acmg_classifier.infrastructure.bundles.verifier import BundleVerifier
from acmg_classifier.infrastructure.storage.records import SQLiteRecordStore

if TYPE_CHECKING:
    from acmg_classifier.presentation.services import PresentationServices


class _NoConfiguredReleaseCatalog(BundleCatalog):
    """Explicitly represent an installed app with no configured release catalog."""

    def candidates(self) -> tuple[BundleCandidate, ...]:
        return ()


@dataclass(frozen=True, slots=True)
class _FirstUseClassifier:
    """Return a typed readiness failure until a signed release is configured.

    The application distribution deliberately ships neither scientific data nor signing
    keys.  It must therefore never synthesize a classification on first use.
    """

    bootstrap: BootstrapService

    async def classify(
        self,
        request: ClassificationRequest,
    ) -> FailedClassificationResponse:
        del request
        return self._readiness_failure()

    async def resume(
        self,
        resume_token: str,
        *,
        answers: tuple[DraftAnswer, ...],
    ) -> FailedClassificationResponse:
        del resume_token, answers
        return self._readiness_failure()

    def _readiness_failure(self) -> FailedClassificationResponse:
        report = self.bootstrap.ensure_ready()
        return _classification_readiness_failure(report)


def compose_default_services(
    paths: ApplicationPaths | None = None,
) -> PresentationServices:
    """Compose local state, bootstrap, and truthful first-use workflow services.

    A release catalog is intentionally absent from the wheel.  ``BootstrapService``
    still owns the state database and application paths, while the classifier reports
    the resulting typed readiness state rather than claiming a scientific result.
    """
    application_paths = paths or default_application_paths()
    bootstrap = BootstrapService(
        BootstrapSettings(paths=application_paths),
        BundleManagerProvisioner(
            BundleManager(
                application_paths.bundle_directory,
                verifier=BundleVerifier({}),
            ),
            _NoConfiguredReleaseCatalog(),
        ),
    )
    from acmg_classifier.presentation.services import PresentationServices

    records = SQLiteRecordStore(application_paths.state_database_path)
    return PresentationServices(
        classifier=_FirstUseClassifier(bootstrap),
        bootstrap=bootstrap,
        replay=ReinterpretationService(records),
        feedback=FeedbackService(records),
    )


def _classification_readiness_failure(
    report: BootstrapReport,
) -> FailedClassificationResponse:
    """Project bootstrap state into the stable classification failure envelope."""
    if report.issue is None:
        return FailedClassificationResponse(
            status=WorkflowStatus.FAILED,
            error_code="CLASSIFICATION_RUNTIME_UNAVAILABLE",
            limitations=(
                "A compatible bundle is installed, but this wheel has no configured "
                "classification runtime.",
            ),
        )
    limitation_values = (
        report.issue.message,
        *report.issue.details,
        report.issue.repair_action,
    )
    return FailedClassificationResponse(
        status=WorkflowStatus.FAILED,
        error_code=report.issue.code.value,
        limitations=tuple(value for value in limitation_values if value),
    )
