from __future__ import annotations

from acmg_classifier.domain.canonical import canonical_hash
from acmg_classifier.domain.context import (
    ContextField,
    ContextIssueCode,
    ContextQuestion,
    ContextResolution,
    ContextValueSource,
    ResolutionProvenance,
    ResolvedValue,
)
from acmg_classifier.domain.enums import GenomeBuild
from acmg_classifier.domain.models import InterpretationContext


def _provenance(source: ContextValueSource) -> ResolutionProvenance:
    return ResolutionProvenance(
        source=source,
        source_id="source-1",
        record_id="record-1",
        confidence="confirmed",
        rationale="A deterministic reason.",
    )


def test_context_question_copies_mutable_json_inputs_for_stable_serialization() -> None:
    schema = {"type": "string", "enum": ["NM_000001.2"]}
    choice = {"value": "NM_000001.2", "label": "Transcript"}
    question = ContextQuestion(
        field=ContextField.TRANSCRIPT,
        code=ContextIssueCode.TRANSCRIPT_AMBIGUOUS,
        prompt="Select a transcript.",
        reason="Two candidates are equally ranked.",
        answer_schema=schema,
        choices=(choice,),
    )
    before = canonical_hash(question.to_canonical_content())

    schema["type"] = "number"
    choice["value"] = "changed"

    assert canonical_hash(question.to_canonical_content()) == before
    assert question.answer_schema["type"] == "string"
    assert question.choices[0]["value"] == "NM_000001.2"


def test_resolution_keeps_one_provenance_bearing_value_per_context_field() -> None:
    provenance = _provenance(ContextValueSource.UNRESOLVED)
    resolution = ContextResolution(
        resolved=InterpretationContext(genome_build=GenomeBuild.GRCH38),
        values=(
            ResolvedValue(ContextField.TRANSCRIPT, None, provenance),
            ResolvedValue(ContextField.INHERITANCE, None, provenance),
            ResolvedValue(ContextField.GENOME_BUILD, GenomeBuild.GRCH38, provenance),
            ResolvedValue(ContextField.DISEASE, None, provenance),
        ),
        transcript_candidates=(),
        questions=(
            ContextQuestion(
                field=ContextField.DISEASE,
                code=ContextIssueCode.DISEASE_REQUIRED,
                prompt="Select disease.",
                reason="No disease is resolved.",
                answer_schema={"type": "string"},
            ),
        ),
        status="needs_context",
    )

    assert [value.field for value in resolution.values] == sorted(
        ContextField, key=lambda item: item.value
    )
    assert (
        resolution.value_for(ContextField.GENOME_BUILD).provenance.source
        is ContextValueSource.UNRESOLVED
    )
