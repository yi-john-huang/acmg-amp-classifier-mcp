from __future__ import annotations

import json
from datetime import UTC, datetime
from pathlib import Path
from typing import Never, cast

import pytest
from typer.testing import CliRunner

from acmg_classifier.application.bootstrap import (
    ApplicationPaths,
    BootstrapIssue,
    BootstrapIssueCode,
    BootstrapReport,
)
from acmg_classifier.application.classification import (
    ClassificationRequest,
    FailedClassificationResponse,
    NeedsContextClassificationResponse,
)
from acmg_classifier.application.drafts import DraftAnswer
from acmg_classifier.application.feedback import FeedbackSubmission
from acmg_classifier.application.reinterpretation import (
    FrozenJsonValue,
    ReplayedClassification,
)
from acmg_classifier.domain.combination import ClassificationDecision
from acmg_classifier.domain.context import (
    ContextField,
    ContextIssueCode,
    ContextQuestion,
)
from acmg_classifier.domain.enums import ClassificationTier, GenomeBuild, WorkflowStatus
from acmg_classifier.domain.feedback import (
    FeedbackContext,
    FeedbackRecord,
    FeedbackType,
)
from acmg_classifier.domain.normalization import (
    CanonicalAlleleKey,
    HgvsAlias,
    NormalizedVariant,
    ProviderProvenance,
)
from acmg_classifier.presentation.cli.app import app
from acmg_classifier.presentation.services import PresentationServices


class _Classifier:
    def __init__(self) -> None:
        self.requests: list[ClassificationRequest] = []

    async def classify(
        self,
        request: ClassificationRequest,
    ) -> NeedsContextClassificationResponse:
        self.requests.append(request)
        return NeedsContextClassificationResponse(
            status=WorkflowStatus.NEEDS_CONTEXT,
            normalized_variant=_normalized(),
            questions=(
                ContextQuestion(
                    field=ContextField.DISEASE,
                    code=ContextIssueCode.DISEASE_REQUIRED,
                    prompt="Select a disease context.",
                    reason="The selected ruleset is disease-specific.",
                    answer_schema={"type": "string"},
                ),
            ),
            draft_id="draft_" + "a" * 32,
            resume_token="resume_" + "b" * 96,
            limitations=("classification is pending required context",),
        )

    async def resume(
        self,
        resume_token: str,
        *,
        answers: tuple[DraftAnswer, ...],
    ) -> FailedClassificationResponse:
        raise AssertionError("non-interactive test double must not resume")


class _InteractiveClassifier(_Classifier):
    def __init__(self) -> None:
        super().__init__()
        self.resume_tokens: list[str] = []
        self.answers: list[DraftAnswer] = []

    async def resume(
        self,
        resume_token: str,
        *,
        answers: tuple[DraftAnswer, ...],
    ) -> FailedClassificationResponse:
        self.resume_tokens.append(resume_token)
        self.answers.extend(answers)
        return FailedClassificationResponse(
            status=WorkflowStatus.FAILED,
            error_code="EVIDENCE_ACQUISITION_FAILED",
        )


class _Bootstrap:
    def __init__(self) -> None:
        self.repair = False

    def doctor(self, *, repair: bool = False) -> BootstrapReport:
        self.repair = repair
        return BootstrapReport(
            ready=False,
            paths=ApplicationPaths(
                config_directory=Path("/tmp/config"),
                state_directory=Path("/tmp/state"),
                cache_directory=Path("/tmp/cache"),
                bundle_directory=Path("/tmp/bundles"),
            ),
            state_database=None,
            bundle_version=None,
            issue=BootstrapIssue(
                code=BootstrapIssueCode.BUNDLE_UNAVAILABLE,
                message="No compatible data bundle is available",
                repair_action="acmg doctor --repair",
            ),
        )

    def ensure_ready(self) -> BootstrapReport:
        return self.doctor()


class _Replay:
    def replay(self, classification_id: str) -> Never:
        raise AssertionError("explain must not run in this test")


class _StoredReplay:
    def replay(self, classification_id: str) -> ReplayedClassification:
        decision = ClassificationDecision(
            algorithm_id="acmg-amp",
            algorithm_version="2015.1",
            classification=ClassificationTier.LIKELY_PATHOGENIC,
            matched_rule_id="pathogenic_ps1_moderate_supporting",
        )
        return ReplayedClassification(
            classification_id=classification_id,
            content=cast(
                dict[str, FrozenJsonValue],
                {"decision": decision.to_canonical_content()},
            ),
            previous_classification_id=None,
        )


class _Feedback:
    def __init__(self) -> None:
        self.submissions: list[FeedbackSubmission] = []
        self.records: list[FeedbackRecord] = []

    def submit(self, submission: FeedbackSubmission) -> FeedbackRecord:
        self.submissions.append(submission)
        record = FeedbackRecord(
            feedback_id="fb_" + "c" * 32,
            submitted_at=datetime(2026, 7, 11, 12, 0, tzinfo=UTC),
            variant_key="cak1:GRCh38:NC_000001.11:100:A>G",
            context=FeedbackContext(genome_build=GenomeBuild.GRCH38),
            **submission.model_dump(),
        )
        self.records.append(record)
        return record

    def export(
        self, classification_id: str | None = None
    ) -> tuple[FeedbackRecord, ...]:
        if classification_id is None:
            return tuple(self.records)
        return tuple(
            record
            for record in self.records
            if record.classification_id == classification_id
        )

    def import_records(self, records: tuple[FeedbackRecord, ...]) -> tuple[str, ...]:
        self.records.extend(records)
        return tuple(record.feedback_id for record in records)


class _FailingFeedback:
    def submit(self, submission: FeedbackSubmission) -> FeedbackRecord:
        del submission
        raise ValueError("database password=do-not-disclose")


def test_noninteractive_classify_emits_only_shared_json_contract() -> None:
    classifier = _Classifier()
    runner = CliRunner()

    result = runner.invoke(
        app,
        [
            "classify",
            "NC_000001.11:g.101A>G",
            "--genome-build",
            "GRCh38",
            "--format",
            "json",
            "--no-interactive",
        ],
        obj={
            "services": PresentationServices(
                classifier=classifier,
                bootstrap=_Bootstrap(),
                replay=_Replay(),
            )
        },
    )

    assert result.exit_code == 2
    assert result.stderr == ""
    payload = json.loads(result.stdout)
    assert payload["schema_version"] == "1.0"
    assert payload["status"] == "needs_context"
    assert payload["draft_id"] == "draft_" + "a" * 32
    assert payload["questions"][0]["field"] == "disease"
    assert classifier.requests[0].context.genome_build is GenomeBuild.GRCH38
    assert classifier.requests[0].interactive is False


def test_interactive_classify_resumes_with_prompted_context_answer() -> None:
    classifier = _InteractiveClassifier()
    runner = CliRunner()

    result = runner.invoke(
        app,
        ["classify", "NC_000001.11:g.101A>G"],
        input="MONDO:0000001\n",
        obj={
            "services": PresentationServices(
                classifier=classifier,
                bootstrap=_Bootstrap(),
                replay=_Replay(),
            )
        },
    )

    assert result.exit_code == 1
    assert "Select a disease context." in result.stdout
    assert classifier.resume_tokens == ["resume_" + "b" * 96]
    assert classifier.answers[0].field is ContextField.DISEASE
    assert classifier.answers[0].value == "MONDO:0000001"


def test_explain_renders_requested_detail_from_stored_decision() -> None:
    runner = CliRunner()

    result = runner.invoke(
        app,
        [
            "explain",
            "cls_" + "b" * 32,
            "--detail",
            "compact",
            "--format",
            "json",
        ],
        obj={
            "services": PresentationServices(
                classifier=_Classifier(),
                bootstrap=_Bootstrap(),
                replay=_StoredReplay(),
            )
        },
    )

    assert result.exit_code == 0
    payload = json.loads(result.stdout)
    assert payload["explanation"]["detail"] == "compact"
    assert payload["explanation"]["classification"] == "Likely Pathogenic"

    text_result = runner.invoke(
        app,
        ["explain", "cls_" + "b" * 32, "--detail", "compact"],
        obj={
            "services": PresentationServices(
                classifier=_Classifier(),
                bootstrap=_Bootstrap(),
                replay=_StoredReplay(),
            )
        },
    )
    assert "Classification: Likely Pathogenic." in text_result.stdout


def test_feedback_command_uses_append_only_service_and_emits_json() -> None:
    feedback = _Feedback()
    runner = CliRunner()

    result = runner.invoke(
        app,
        [
            "feedback",
            "cls_" + "b" * 32,
            "--type",
            "correction",
            "--proposed-correction",
            "Likely Pathogenic",
            "--rationale",
            "Expert review supports this correction.",
            "--evidence-reference",
            "PMID:12345",
            "--actor-id",
            "usr_scientist-1",
            "--format",
            "json",
        ],
        obj={
            "services": PresentationServices(
                classifier=_Classifier(),
                bootstrap=_Bootstrap(),
                replay=_Replay(),
                feedback=feedback,
            )
        },
    )

    assert result.exit_code == 0
    assert result.stderr == ""
    payload = json.loads(result.stdout)
    assert payload["feedback_id"] == "fb_" + "c" * 32
    assert feedback.submissions[0].feedback_type is FeedbackType.CORRECTION


def test_feedback_command_redacts_service_error_details() -> None:
    runner = CliRunner()

    result = runner.invoke(
        app,
        [
            "feedback",
            "cls_" + "b" * 32,
            "--type",
            "agreement",
            "--rationale",
            "Reviewed.",
            "--actor-id",
            "usr_scientist-1",
            "--format",
            "json",
        ],
        obj={
            "services": PresentationServices(
                classifier=_Classifier(),
                bootstrap=_Bootstrap(),
                replay=_Replay(),
                feedback=cast(object, _FailingFeedback()),
            )
        },
    )

    assert result.exit_code == 2
    assert "database password" not in result.stdout
    assert json.loads(result.stdout)["error_code"] == "INVALID_FEEDBACK"


def test_feedback_export_and_import_preserve_feedback_content(tmp_path: Path) -> None:
    feedback = _Feedback()
    feedback.submit(
        FeedbackSubmission(
            classification_id="cls_" + "b" * 32,
            feedback_type=FeedbackType.AGREEMENT,
            rationale="Reviewed against primary evidence.",
            actor_id="usr_scientist-1",
        )
    )
    services = PresentationServices(
        classifier=_Classifier(),
        bootstrap=_Bootstrap(),
        replay=_Replay(),
        feedback=feedback,
    )
    runner = CliRunner()

    export_path = tmp_path / "feedback.json"
    exported = runner.invoke(
        app,
        ["feedback-export", "--output", str(export_path), "--format", "json"],
        obj={"services": services},
    )
    imported = runner.invoke(
        app,
        ["feedback-import", str(export_path), "--format", "json"],
        obj={"services": services},
    )

    assert exported.exit_code == 0
    assert json.loads(exported.stdout)["feedback"][0]["feedback_id"] == "fb_" + "c" * 32
    assert imported.exit_code == 0
    assert json.loads(imported.stdout)["feedback_ids"] == ["fb_" + "c" * 32]


def test_feedback_import_rejects_oversized_file_before_parsing(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    import acmg_classifier.presentation.cli.app as cli_module

    monkeypatch.setattr(cli_module, "_MAX_FEEDBACK_IMPORT_BYTES", 1)
    input_path = tmp_path / "oversized-feedback.json"
    input_path.write_text("{}", encoding="utf-8")
    runner = CliRunner()

    result = runner.invoke(
        app,
        ["feedback-import", str(input_path), "--format", "json"],
        obj={
            "services": PresentationServices(
                classifier=_Classifier(),
                bootstrap=_Bootstrap(),
                replay=_Replay(),
                feedback=_Feedback(),
            )
        },
    )

    assert result.exit_code == 2
    assert json.loads(result.stdout)["error_code"] == "INVALID_FEEDBACK_IMPORT"


def test_feedback_import_redacts_rejected_path_value(tmp_path: Path) -> None:
    input_path = tmp_path / "api_key=secret-marker.json"
    runner = CliRunner()

    result = runner.invoke(
        app,
        ["feedback-import", str(input_path), "--format", "json"],
        obj={
            "services": PresentationServices(
                classifier=_Classifier(),
                bootstrap=_Bootstrap(),
                replay=_Replay(),
                feedback=_Feedback(),
            )
        },
    )

    assert result.exit_code == 2
    assert "api_key=secret-marker" not in result.stdout
    assert json.loads(result.stdout)["error_code"] == "INVALID_FEEDBACK_IMPORT"

def test_doctor_writes_json_to_stdout_and_honors_repair() -> None:
    bootstrap = _Bootstrap()
    runner = CliRunner()

    result = runner.invoke(
        app,
        ["doctor", "--repair", "--format", "json"],
        obj={
            "services": PresentationServices(
                classifier=_Classifier(),
                bootstrap=bootstrap,
                replay=_Replay(),
            )
        },
    )

    assert result.exit_code == 1
    assert result.stderr == ""
    assert bootstrap.repair
    payload = json.loads(result.stdout)
    assert payload["ready"] is False
    assert payload["issue"]["code"] == "BUNDLE_UNAVAILABLE"


def _normalized() -> NormalizedVariant:
    key = CanonicalAlleleKey(
        assembly=GenomeBuild.GRCH38,
        sequence_accession="NC_000001.11",
        start=100,
        end=101,
        deleted_sequence="A",
        inserted_sequence="G",
    )
    return NormalizedVariant(
        original_input="NC_000001.11:g.101A>G",
        parsed_input="NC_000001.11:g.101A>G",
        canonical_key=key,
        genome_build=GenomeBuild.GRCH38,
        genomic_accession="NC_000001.11",
        genomic_start=100,
        genomic_end=101,
        reference_allele="A",
        alternate_allele="G",
        normalized_hgvs="NC_000001.11:g.101A>G",
        hgvs_aliases=(HgvsAlias("NC_000001.11:g.101A>G", "genomic"),),
        gene_symbol="TEST1",
        transcript="NM_000001.1",
        transcript_hgvs="NM_000001.1:c.1A>G",
        provider_provenance=(
            ProviderProvenance(
                provider_id="test-normalizer",
                provider_version="1.0.0",
                source_record_id="record-1",
                retrieved_at=datetime(2026, 7, 11, 12, 0, tzinfo=UTC),
                bundle_version="core-test-1",
                raw_snapshot_ref=None,
                query_key=key.value,
            ),
        ),
    )
