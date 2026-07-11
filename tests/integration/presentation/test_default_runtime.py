"""First-use composition contracts for the installed presentation runtime."""

from __future__ import annotations

from pathlib import Path

import pytest

from acmg_classifier.application.bootstrap import ApplicationPaths, BootstrapIssueCode
from acmg_classifier.application.classification import (
    ClassificationRequest,
    FailedClassificationResponse,
)
from acmg_classifier.domain.enums import WorkflowStatus
from acmg_classifier.presentation import runtime
from acmg_classifier.presentation.services import default_services


def _paths(root: Path) -> ApplicationPaths:
    return ApplicationPaths(
        config_directory=root / "config",
        state_directory=root / "state",
        cache_directory=root / "cache",
        bundle_directory=root / "bundles",
    )


@pytest.mark.asyncio
async def test_default_services_return_actionable_unavailable_bundle(
    monkeypatch: pytest.MonkeyPatch,
    tmp_path: Path,
) -> None:
    """Default first use requires neither service configuration nor fake data."""
    paths = _paths(tmp_path)
    monkeypatch.setattr(runtime, "default_application_paths", lambda: paths)

    services = default_services()

    report = services.bootstrap.ensure_ready()
    response = await services.classifier.classify(
        ClassificationRequest(variant="NM_000059.4(BRCA2):c.7008-1G>A")
    )

    assert report.ready is False
    assert report.issue is not None
    assert report.issue.code is BootstrapIssueCode.BUNDLE_UNAVAILABLE
    assert report.issue.repair_action == "acmg doctor --repair"
    assert paths.state_database_path.is_file()
    assert isinstance(response, FailedClassificationResponse)
    assert response.status is WorkflowStatus.FAILED
    assert response.error_code == "BUNDLE_UNAVAILABLE"
    assert "acmg doctor --repair" in response.limitations
