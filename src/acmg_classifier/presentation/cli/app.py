"""Thin Typer CLI adapter over the shared classification application workflow."""

from __future__ import annotations

import asyncio
import json
import sys
from collections.abc import Coroutine, Mapping
from pathlib import Path
from typing import Annotated, Any, NoReturn, cast

import typer
from rich.console import Console

from acmg_classifier.application.classification import (
    ClassificationRequest,
    ClassificationWorkflowResponse,
    NeedsContextClassificationResponse,
)
from acmg_classifier.application.drafts import DraftAnswer, DraftAnswerState
from acmg_classifier.application.explanation import ExplanationDetail
from acmg_classifier.application.feedback import FeedbackSubmission
from acmg_classifier.domain.context import ContextField
from acmg_classifier.domain.enums import AnalysisIntent, GenomeBuild, InheritanceMode
from acmg_classifier.domain.errors import JsonValue
from acmg_classifier.domain.evidence import EvidencePolicy, EvidencePolicyMode
from acmg_classifier.domain.feedback import FeedbackRecord, FeedbackType
from acmg_classifier.domain.models import InterpretationContext
from acmg_classifier.presentation.serialization import (
    bootstrap_content,
    stored_explanation_content,
    workflow_content,
)
from acmg_classifier.presentation.services import (
    PresentationServices,
    RuntimeConfigurationError,
    default_services,
)

app = typer.Typer(
    name="acmg",
    no_args_is_help=True,
    add_completion=False,
    help="Research-use ACMG/AMP variant classification.",
)
data_app = typer.Typer(no_args_is_help=True, help="Signed data-bundle operations.")
app.add_typer(data_app, name="data")

VariantArgument = Annotated[
    str,
    typer.Argument(help="HGVS or supported gene-plus-variant input."),
]
GenomeBuildOption = Annotated[GenomeBuild | None, typer.Option("--genome-build")]
TranscriptOption = Annotated[str | None, typer.Option("--transcript")]
DiseaseIdOption = Annotated[str | None, typer.Option("--disease-id")]
DiseaseLabelOption = Annotated[str | None, typer.Option("--disease-label")]
InheritanceOption = Annotated[InheritanceMode | None, typer.Option("--inheritance")]
AnalysisIntentOption = Annotated[AnalysisIntent, typer.Option("--analysis-intent")]
OfflineOption = Annotated[bool, typer.Option("--offline")]
InteractiveOption = Annotated[
    bool,
    typer.Option("--interactive/--no-interactive"),
]
RepairOption = Annotated[bool, typer.Option("--repair")]
OutputFormatOption = Annotated[str, typer.Option("--format")]
FeedbackTypeOption = Annotated[FeedbackType, typer.Option("--type")]
RationaleOption = Annotated[str, typer.Option("--rationale")]
ActorIdOption = Annotated[str, typer.Option("--actor-id")]
ProposedCorrectionOption = Annotated[
    str | None,
    typer.Option("--proposed-correction"),
]
EvidenceReferencesOption = Annotated[
    list[str] | None,
    typer.Option("--evidence-reference"),
]
FeedbackExportPathOption = Annotated[Path | None, typer.Option("--output")]
FeedbackClassificationOption = Annotated[
    str | None,
    typer.Option("--classification-id"),
]
ExplanationDetailOption = Annotated[ExplanationDetail, typer.Option("--detail")]
_MAX_FEEDBACK_IMPORT_BYTES = 8 * 1024 * 1024


@app.command()
def classify(
    ctx: typer.Context,
    variant: VariantArgument,
    genome_build: GenomeBuildOption = None,
    transcript: TranscriptOption = None,
    disease_id: DiseaseIdOption = None,
    disease_label: DiseaseLabelOption = None,
    inheritance: InheritanceOption = None,
    analysis_intent: AnalysisIntentOption = AnalysisIntent.GERMLINE_MENDELIAN,
    offline: OfflineOption = False,
    interactive: InteractiveOption = True,
    output_format: OutputFormatOption = "text",
) -> None:
    """Classify one variant through the same application service as MCP."""
    services = _services(ctx)
    _validate_format(output_format)
    if (disease_id is None) != (disease_label is None):
        raise typer.BadParameter(
            "--disease-id and --disease-label must be supplied together"
        )
    request = ClassificationRequest(
        variant=variant,
        context=InterpretationContext(
            genome_build=genome_build,
            transcript=transcript,
            disease_id=disease_id,
            disease_label=disease_label,
            inheritance=inheritance,
        ),
        evidence_policy=EvidencePolicy(
            mode=EvidencePolicyMode.OFFLINE if offline else EvidencePolicyMode.LIVE
        ),
        analysis_intent=analysis_intent,
        interactive=interactive,
    )
    response = _run(services.classifier.classify(request))
    if interactive:
        response = _resolve_interactively(services, response)
    payload = workflow_content(response)
    _emit(payload, output_format)
    raise typer.Exit(_workflow_exit_code(response))


@app.command()
def explain(
    ctx: typer.Context,
    classification_id: str,
    detail: ExplanationDetailOption = ExplanationDetail.STANDARD,
    output_format: OutputFormatOption = "text",
) -> None:
    """Render the explanation stored with an immutable classification record."""
    services = _services(ctx)
    _validate_format(output_format)
    try:
        replay = services.replay.replay(classification_id)
        content = replay.to_canonical_content()
    except Exception:
        _emit(
            {
                "schema_version": "1.0",
                "status": "failed",
                "error_code": "CLASSIFICATION_NOT_FOUND",
            },
            output_format,
        )
        raise typer.Exit(1) from None
    try:
        explanation = stored_explanation_content(content, detail=detail)
    except ValueError:
        _emit(
            {
                "schema_version": "1.0",
                "status": "failed",
                "error_code": "EXPLANATION_UNAVAILABLE",
            },
            output_format,
        )
        raise typer.Exit(1) from None
    _emit(
        {
            "schema_version": "1.0",
            "status": "completed",
            "classification_id": classification_id,
            "explanation": explanation,
        },
        output_format,
    )


@app.command("feedback")
def submit_feedback(
    ctx: typer.Context,
    classification_id: str,
    feedback_type: FeedbackTypeOption,
    rationale: RationaleOption,
    actor_id: ActorIdOption,
    proposed_correction: ProposedCorrectionOption = None,
    evidence_references: EvidenceReferencesOption = None,
    output_format: OutputFormatOption = "text",
) -> None:
    """Append a scientist agreement or correction without changing a result."""
    services = _services(ctx)
    _validate_format(output_format)
    if services.feedback is None:
        _emit(
            {
                "schema_version": "1.0",
                "status": "failed",
                "error_code": "FEEDBACK_UNAVAILABLE",
            },
            output_format,
        )
        raise typer.Exit(1)
    try:
        record = services.feedback.submit(
            FeedbackSubmission(
                classification_id=classification_id,
                feedback_type=feedback_type,
                proposed_correction=proposed_correction,
                rationale=rationale,
                evidence_references=tuple(evidence_references or ()),
                actor_id=actor_id,
            )
        )
    except ValueError as error:
        _emit(
            {
                "schema_version": "1.0",
                "status": "failed",
                "error_code": "INVALID_FEEDBACK",
                "limitations": [str(error)],
            },
            output_format,
        )
        raise typer.Exit(2) from None
    _emit(
        {
            "schema_version": "1.0",
            "status": "completed",
            "feedback_id": record.feedback_id,
            "feedback": cast(JsonValue, record.to_canonical_content()),
        },
        output_format,
    )


@app.command("feedback-export")
def export_feedback(
    ctx: typer.Context,
    classification_id: FeedbackClassificationOption = None,
    output_file: FeedbackExportPathOption = None,
    output_format: OutputFormatOption = "text",
) -> None:
    """Export immutable feedback records without changing their audit content."""
    services = _services(ctx)
    _validate_format(output_format)
    if services.feedback is None:
        _feedback_unavailable(output_format)
    records = services.feedback.export(classification_id)
    payload: dict[str, JsonValue] = {
        "schema_version": "1.0",
        "status": "completed",
        "feedback": [
            cast(JsonValue, record.to_canonical_content()) for record in records
        ],
    }
    if output_file is not None:
        output_file.write_text(
            json.dumps(payload, sort_keys=True, separators=(",", ":")) + "\n",
            encoding="utf-8",
        )
    _emit(payload, output_format)


@app.command("feedback-import")
def import_feedback(
    ctx: typer.Context,
    input_file: Path,
    output_format: OutputFormatOption = "text",
) -> None:
    """Append an export while preserving its identifiers and audit metadata."""
    services = _services(ctx)
    _validate_format(output_format)
    if services.feedback is None:
        _feedback_unavailable(output_format)
    try:
        if input_file.stat().st_size > _MAX_FEEDBACK_IMPORT_BYTES:
            raise ValueError("feedback export exceeds the 8 MiB import limit")
        payload = json.loads(input_file.read_text(encoding="utf-8"))
        values = payload["feedback"]
        if not isinstance(values, list):
            raise ValueError("feedback export must contain a feedback array")
        records = tuple(FeedbackRecord.model_validate(value) for value in values)
        imported_ids = services.feedback.import_records(records)
    except (OSError, TypeError, ValueError, json.JSONDecodeError) as error:
        _emit(
            {
                "schema_version": "1.0",
                "status": "failed",
                "error_code": "INVALID_FEEDBACK_IMPORT",
                "limitations": [str(error)],
            },
            output_format,
        )
        raise typer.Exit(2) from None
    _emit(
        {
            "schema_version": "1.0",
            "status": "completed",
            "feedback_ids": list(imported_ids),
        },
        output_format,
    )


def _feedback_unavailable(output_format: str) -> NoReturn:
    _emit(
        {
            "schema_version": "1.0",
            "status": "failed",
            "error_code": "FEEDBACK_UNAVAILABLE",
        },
        output_format,
    )
    raise typer.Exit(1)


@app.command()
def doctor(
    ctx: typer.Context,
    repair: RepairOption = False,
    output_format: OutputFormatOption = "text",
) -> None:
    """Inspect local readiness or repair a compatible signed data bundle."""
    services = _services(ctx)
    _validate_format(output_format)
    report = services.bootstrap.doctor(repair=repair)
    _emit(bootstrap_content(report), output_format)
    raise typer.Exit(0 if report.ready else 1)


@data_app.command("status")
def data_status(
    ctx: typer.Context,
    output_format: OutputFormatOption = "text",
) -> None:
    """Show non-mutating local data-bundle diagnostics."""
    doctor(ctx, repair=False, output_format=output_format)


@data_app.command("update")
def data_update(
    ctx: typer.Context,
    output_format: OutputFormatOption = "text",
) -> None:
    """Install or reuse the selected compatible data bundle."""
    services = _services(ctx)
    _validate_format(output_format)
    report = services.bootstrap.ensure_ready()
    _emit(bootstrap_content(report), output_format)
    raise typer.Exit(0 if report.ready else 1)


@data_app.command("install")
def data_install(
    ctx: typer.Context,
    offline: OfflineOption = False,
    output_format: OutputFormatOption = "text",
) -> None:
    """Reject unsupported manual installation rather than fabricate a data state."""
    _services(ctx)
    _validate_format(output_format)
    error_code = (
        "OFFLINE_INSTALL_REQUIRES_SIGNED_ARCHIVE"
        if offline
        else "DATA_INSTALL_UNAVAILABLE"
    )
    _emit(
        {
            "schema_version": "1.0",
            "status": "failed",
            "error_code": error_code,
        },
        output_format,
    )
    raise typer.Exit(1)


def main() -> None:
    """Run the console entry point with default real application composition."""
    app()


def _services(ctx: typer.Context) -> PresentationServices:
    services = ctx.obj.get("services") if isinstance(ctx.obj, dict) else None
    if isinstance(services, PresentationServices):
        return services
    try:
        return default_services()
    except RuntimeConfigurationError as error:
        typer.echo(str(error), err=True)
        raise typer.Exit(1) from None


def _resolve_interactively(
    services: PresentationServices,
    response: ClassificationWorkflowResponse,
) -> ClassificationWorkflowResponse:
    while isinstance(response, NeedsContextClassificationResponse):
        question = response.questions[0]
        answer_text = typer.prompt(question.prompt)
        answer = _prompt_answer(question.field, answer_text)
        response = _run(
            services.classifier.resume(
                response.resume_token,
                answers=(answer,),
            )
        )
    return response


def _prompt_answer(field: ContextField, value: str) -> DraftAnswer:
    normalized = value.strip()
    state = {
        "unknown": DraftAnswerState.UNKNOWN,
        "unavailable": DraftAnswerState.UNAVAILABLE,
        "not applicable": DraftAnswerState.NOT_APPLICABLE,
        "not_applicable": DraftAnswerState.NOT_APPLICABLE,
    }.get(normalized.lower())
    if state is not None:
        return DraftAnswer(field=field, state=state)
    return DraftAnswer(
        field=field,
        state=DraftAnswerState.PROVIDED,
        value=normalized,
    )


def _run[ResultT](coroutine: Coroutine[Any, Any, ResultT]) -> ResultT:
    return asyncio.run(coroutine)


def _validate_format(output_format: str) -> None:
    if output_format not in {"text", "json"}:
        raise typer.BadParameter("--format must be text or json")


def _emit(payload: Mapping[str, JsonValue], output_format: str) -> None:
    if output_format == "json":
        typer.echo(json.dumps(payload, sort_keys=True, separators=(",", ":")))
        return
    console = Console(file=sys.stdout, color_system=None, force_terminal=False)
    console.print(_text(payload))


def _text(payload: Mapping[str, JsonValue]) -> str:
    status = str(payload.get("status", "unknown"))
    lines = [f"status: {status}"]
    classification = payload.get("classification")
    if classification is not None:
        lines.append(f"classification: {classification}")
    explanation = payload.get("explanation")
    if isinstance(explanation, Mapping):
        blocks = explanation.get("blocks")
        if isinstance(blocks, list):
            for block in blocks:
                if not isinstance(block, Mapping):
                    continue
                title = block.get("title")
                text = block.get("text")
                if isinstance(title, str) and isinstance(text, str):
                    lines.append(f"{title}: {text}")
        disclaimer = explanation.get("disclaimer")
        if isinstance(disclaimer, str):
            lines.append(disclaimer)
    error_code = payload.get("error_code")
    if error_code is not None:
        lines.append(f"error: {error_code}")
    issue = payload.get("issue")
    if isinstance(issue, dict):
        lines.append(f"issue: {issue.get('code', 'UNKNOWN')}")
        lines.append(str(issue.get("message", "")))
    limitations = payload.get("limitations")
    if isinstance(limitations, list):
        lines.extend(f"limitation: {limitation}" for limitation in limitations)
    return "\n".join(lines)


def _workflow_exit_code(response: ClassificationWorkflowResponse) -> int:
    if response.status.value == "completed":
        return 0
    if response.status.value == "needs_context":
        return 2
    return 1
