"""Deterministic transcript ranking and interpretation-context resolution."""

from __future__ import annotations

from collections.abc import Iterable

from acmg_classifier.domain.context import (
    ContextField,
    ContextIssue,
    ContextIssueCode,
    ContextQuestion,
    ContextResolution,
    ContextSpecification,
    ContextValueSource,
    ResolutionProvenance,
    ResolvedValue,
    TranscriptCandidate,
)
from acmg_classifier.domain.enums import GenomeBuild, InheritanceMode
from acmg_classifier.domain.errors import JsonValue
from acmg_classifier.domain.models import InterpretationContext
from acmg_classifier.domain.normalization import NormalizedVariant
from acmg_classifier.ports.context import (
    GeneDiseaseKnowledgeRecord,
    TranscriptKnowledgeRecord,
    TranscriptKnowledgeRepository,
)

_MANE_RANKS = {"MANE Plus Clinical": 1, "MANE Select": 2}


class ContextResolutionService:
    """Resolve only trusted context; return questions rather than arbitrary choices."""

    def __init__(self, knowledge: TranscriptKnowledgeRepository) -> None:
        self._knowledge = knowledge

    def resolve(
        self,
        normalized: NormalizedVariant,
        context: InterpretationContext | None = None,
        *,
        specification: ContextSpecification | None = None,
    ) -> ContextResolution:
        """Resolve context from explicit, trusted, or unresolved values."""
        supplied = context or InterpretationContext()
        issues: list[ContextIssue] = []
        questions: list[ContextQuestion] = []

        build_value = self._resolve_build(normalized, supplied)
        gene_symbol, supplied_transcript = self._gene_and_supplied_transcript(
            normalized, supplied
        )
        candidates = self._candidates(normalized, gene_symbol)
        transcript_value, transcript_issue, transcript_question = (
            self._resolve_transcript(
                normalized,
                supplied_transcript,
                candidates,
                specification,
            )
        )
        if transcript_issue is not None:
            issues.append(transcript_issue)
        if transcript_question is not None and not issues:
            questions.append(transcript_question)

        diseases = (
            self._sorted_diseases(self._knowledge.gene_diseases(gene_symbol))
            if gene_symbol is not None
            else ()
        )
        disease_value, disease_question = self._resolve_disease(
            supplied, specification, diseases
        )
        if disease_question is not None and not issues:
            questions.append(disease_question)

        inheritance_value, inheritance_question, inheritance_issue = (
            self._resolve_inheritance(
                supplied,
                specification,
                diseases,
                disease_value.value,
            )
        )
        if inheritance_issue is not None:
            issues.append(inheritance_issue)
        if inheritance_question is not None and not issues:
            questions.append(inheritance_question)

        resolved = InterpretationContext(
            genome_build=build_value.value,
            transcript=transcript_value.value,
            disease_id=disease_value.value[0]
            if disease_value.value is not None
            else None,
            disease_label=disease_value.value[1]
            if disease_value.value is not None
            else None,
            inheritance=inheritance_value.value,
        )
        values: tuple[ResolvedValue[object], ...] = (
            build_value,
            transcript_value,
            disease_value,
            inheritance_value,
        )
        status = "conflict" if issues else "needs_context" if questions else "complete"
        return ContextResolution(
            resolved=resolved,
            values=values,
            transcript_candidates=candidates,
            questions=tuple(questions),
            issues=tuple(issues),
            status=status,
        )

    @staticmethod
    def _resolve_build(
        normalized: NormalizedVariant,
        supplied: InterpretationContext,
    ) -> ResolvedValue[GenomeBuild]:
        if supplied.genome_build is not None:
            return ResolvedValue(
                ContextField.GENOME_BUILD,
                supplied.genome_build,
                _provenance(
                    ContextValueSource.USER_CONFIRMED,
                    record_id=supplied.genome_build.value,
                    rationale="The requester confirmed the genome build.",
                ),
            )
        return ResolvedValue(
            ContextField.GENOME_BUILD,
            normalized.genome_build,
            _provenance(
                ContextValueSource.NORMALIZATION_PROVIDER,
                record_id=normalized.genome_build.value,
                rationale="The normalized allele carries the resolved genome build.",
            ),
        )

    def _gene_and_supplied_transcript(
        self,
        normalized: NormalizedVariant,
        supplied: InterpretationContext,
    ) -> tuple[str | None, str | None]:
        if supplied.transcript is not None:
            row = self._knowledge.transcript_by_accession(supplied.transcript)
            return normalized.gene_symbol or (
                row.gene_symbol if row is not None else None
            ), supplied.transcript
        if normalized.gene_symbol is not None:
            return normalized.gene_symbol, None
        if normalized.transcript is not None:
            row = self._knowledge.transcript_by_accession(normalized.transcript)
            return (row.gene_symbol if row is not None else None), normalized.transcript
        return None, None

    def _candidates(
        self, normalized: NormalizedVariant, gene_symbol: str | None
    ) -> tuple[TranscriptCandidate, ...]:
        if gene_symbol is None:
            return ()
        compatible_rows = (
            row
            for row in self._knowledge.transcripts_for_gene(gene_symbol)
            if self._compatible(normalized, row) and row.mane_status in _MANE_RANKS
        )
        candidates = [self._candidate(row) for row in compatible_rows]
        return tuple(
            sorted(
                candidates,
                key=lambda item: (
                    item.rank,
                    item.refseq_transcript,
                    item.ensembl_transcript,
                ),
            )
        )

    @staticmethod
    def _compatible(
        normalized: NormalizedVariant, row: TranscriptKnowledgeRecord
    ) -> bool:
        return (
            (
                normalized.gene_symbol is None
                or row.gene_symbol == normalized.gene_symbol
            )
            and row.genome_build == normalized.genome_build
            and row.genomic_accession == normalized.genomic_accession
        )

    @staticmethod
    def _candidate(row: TranscriptKnowledgeRecord) -> TranscriptCandidate:
        rank = _MANE_RANKS[row.mane_status]
        return TranscriptCandidate(
            refseq_transcript=row.refseq_transcript,
            ensembl_transcript=row.ensembl_transcript,
            gene_symbol=row.gene_symbol,
            hgnc_id=row.hgnc_id,
            mane_status=row.mane_status,
            genome_build=row.genome_build,
            genomic_accession=row.genomic_accession,
            rank=rank,
            rank_reason=(
                "MANE Plus Clinical is preferred over MANE Select."
                if rank == 1
                else "MANE Select is the compatible MANE transcript."
            ),
            provenance=_provenance(
                ContextValueSource.BUNDLE_METADATA,
                record_id=row.refseq_transcript,
                rationale=(
                    "The signed knowledge bundle records this compatible transcript."
                ),
            ),
        )

    def _resolve_transcript(
        self,
        normalized: NormalizedVariant,
        supplied_transcript: str | None,
        candidates: tuple[TranscriptCandidate, ...],
        specification: ContextSpecification | None,
    ) -> tuple[ResolvedValue[str], ContextIssue | None, ContextQuestion | None]:
        if supplied_transcript is not None:
            row = self._knowledge.transcript_by_accession(supplied_transcript)
            if row is None:
                return (
                    _unresolved_transcript(),
                    ContextIssue(
                        ContextIssueCode.TRANSCRIPT_NOT_FOUND,
                        (
                            f"The supplied transcript {supplied_transcript} "
                            "is not in the trusted bundle."
                        ),
                    ),
                    None,
                )
            if not self._compatible(normalized, row):
                return (
                    _unresolved_transcript(),
                    ContextIssue(
                        ContextIssueCode.TRANSCRIPT_CONFLICT,
                        (
                            f"The supplied transcript {supplied_transcript} "
                            "is incompatible with the normalized allele."
                        ),
                    ),
                    None,
                )
            return (
                ResolvedValue(
                    ContextField.TRANSCRIPT,
                    supplied_transcript,
                    _provenance(
                        ContextValueSource.USER_CONFIRMED,
                        record_id=supplied_transcript,
                        rationale=(
                            "The requester supplied an exact compatible transcript."
                        ),
                    ),
                ),
                None,
                None,
            )

        if specification is not None and specification.transcript is not None:
            row = self._knowledge.transcript_by_accession(specification.transcript)
            if row is None or not self._compatible(normalized, row):
                return (
                    _unresolved_transcript(),
                    ContextIssue(
                        ContextIssueCode.CONTEXT_CONFLICT,
                        (
                            "The selected ruleset transcript is not compatible "
                            "with the normalized allele."
                        ),
                    ),
                    None,
                )
            return (
                ResolvedValue(
                    ContextField.TRANSCRIPT,
                    specification.transcript,
                    _provenance(
                        ContextValueSource.RULESET_SPECIFICATION,
                        source_id=specification.source_id,
                        record_id=specification.transcript,
                        rationale=(
                            "The selected ruleset explicitly specifies this transcript."
                        ),
                    ),
                ),
                None,
                None,
            )

        if not candidates:
            return (
                _unresolved_transcript(),
                None,
                ContextQuestion(
                    field=ContextField.TRANSCRIPT,
                    code=ContextIssueCode.GENE_NOT_FOUND,
                    prompt="Select a compatible transcript.",
                    reason=(
                        "No compatible MANE transcript is present in the "
                        "trusted bundle."
                    ),
                    answer_schema={"type": "string", "minLength": 1},
                ),
            )

        top_rank = candidates[0].rank
        top_candidates = tuple(item for item in candidates if item.rank == top_rank)
        if len(top_candidates) > 1:
            return (
                _unresolved_transcript(),
                None,
                ContextQuestion(
                    field=ContextField.TRANSCRIPT,
                    code=ContextIssueCode.TRANSCRIPT_AMBIGUOUS,
                    prompt="Select the transcript to use for interpretation.",
                    reason=(
                        "Multiple equally ranked compatible transcripts are available."
                    ),
                    answer_schema={
                        "type": "string",
                        "enum": [item.refseq_transcript for item in top_candidates],
                    },
                    choices=tuple(
                        {
                            "value": item.refseq_transcript,
                            "label": item.refseq_transcript,
                            "mane_status": item.mane_status,
                        }
                        for item in top_candidates
                    ),
                    provenance=tuple(item.provenance for item in top_candidates),
                ),
            )

        selected = top_candidates[0]
        return (
            ResolvedValue(
                ContextField.TRANSCRIPT,
                selected.refseq_transcript,
                selected.provenance,
            ),
            None,
            None,
        )

    def _resolve_disease(
        self,
        supplied: InterpretationContext,
        specification: ContextSpecification | None,
        diseases: tuple[GeneDiseaseKnowledgeRecord, ...],
    ) -> tuple[ResolvedValue[tuple[str, str]], ContextQuestion | None]:
        if supplied.disease_id is not None:
            return (
                ResolvedValue(
                    ContextField.DISEASE,
                    (
                        supplied.disease_id,
                        supplied.disease_label or supplied.disease_id,
                    ),
                    _provenance(
                        ContextValueSource.USER_CONFIRMED,
                        record_id=supplied.disease_id,
                        rationale="The requester confirmed the disease context.",
                    ),
                ),
                None,
            )
        if specification is not None and specification.disease_id is not None:
            return (
                ResolvedValue(
                    ContextField.DISEASE,
                    (
                        specification.disease_id,
                        specification.disease_label or specification.disease_id,
                    ),
                    _provenance(
                        ContextValueSource.RULESET_SPECIFICATION,
                        source_id=specification.source_id,
                        record_id=specification.disease_id,
                        rationale=(
                            "The selected ruleset explicitly scopes this disease."
                        ),
                    ),
                ),
                None,
            )

        distinct = _distinct_diseases(diseases)
        if len(distinct) == 1:
            record = distinct[0]
            return (
                ResolvedValue(
                    ContextField.DISEASE,
                    (record.mondo_id, record.disease_label),
                    _provenance(
                        ContextValueSource.BUNDLE_METADATA,
                        record_id=record.mondo_id,
                        rationale=(
                            "The signed knowledge bundle has one disease "
                            "context for this gene."
                        ),
                    ),
                ),
                None,
            )
        code = (
            ContextIssueCode.DISEASE_REQUIRED
            if not distinct
            else ContextIssueCode.DISEASE_AMBIGUOUS
        )
        reason = (
            "No disease metadata is available for this gene."
            if not distinct
            else "Multiple curated disease contexts are available for this gene."
        )
        choices = tuple(
            {"value": record.mondo_id, "label": record.disease_label}
            for record in distinct
        )
        answer_schema: dict[str, JsonValue] = {"type": "string", "minLength": 1}
        if choices:
            disease_ids: list[JsonValue] = []
            disease_ids.extend(record.mondo_id for record in distinct)
            answer_schema["enum"] = disease_ids
        return (
            ResolvedValue(
                ContextField.DISEASE,
                None,
                _provenance(
                    ContextValueSource.UNRESOLVED,
                    rationale=reason,
                ),
            ),
            ContextQuestion(
                field=ContextField.DISEASE,
                code=code,
                prompt="Select the disease context for interpretation.",
                reason=reason,
                answer_schema=answer_schema,
                choices=choices,
            ),
        )

    def _resolve_inheritance(
        self,
        supplied: InterpretationContext,
        specification: ContextSpecification | None,
        diseases: tuple[GeneDiseaseKnowledgeRecord, ...],
        resolved_disease: tuple[str, str] | None,
    ) -> tuple[
        ResolvedValue[InheritanceMode], ContextQuestion | None, ContextIssue | None
    ]:
        if supplied.inheritance is not None:
            return (
                ResolvedValue(
                    ContextField.INHERITANCE,
                    supplied.inheritance,
                    _provenance(
                        ContextValueSource.USER_CONFIRMED,
                        record_id=supplied.inheritance.value,
                        rationale="The requester confirmed the inheritance mode.",
                    ),
                ),
                None,
                None,
            )
        if specification is not None and specification.inheritance is not None:
            mode = _as_inheritance(specification.inheritance)
            if mode is None:
                return (
                    _unresolved_inheritance(),
                    None,
                    ContextIssue(
                        ContextIssueCode.CONTEXT_CONFLICT,
                        "The selected ruleset has an unsupported inheritance value.",
                    ),
                )
            return (
                ResolvedValue(
                    ContextField.INHERITANCE,
                    mode,
                    _provenance(
                        ContextValueSource.RULESET_SPECIFICATION,
                        source_id=specification.source_id,
                        record_id=mode.value,
                        rationale=(
                            "The selected ruleset explicitly specifies inheritance."
                        ),
                    ),
                ),
                None,
                None,
            )

        applicable = diseases
        if resolved_disease is not None:
            applicable = tuple(
                record for record in diseases if record.mondo_id == resolved_disease[0]
            )
        modes = _inheritance_modes(applicable)
        if len(modes) == 1:
            mode = modes[0]
            return (
                ResolvedValue(
                    ContextField.INHERITANCE,
                    mode,
                    _provenance(
                        ContextValueSource.BUNDLE_METADATA,
                        record_id=mode.value,
                        rationale=(
                            "The signed knowledge bundle has one applicable "
                            "inheritance mode."
                        ),
                    ),
                ),
                None,
                None,
            )
        if not applicable and resolved_disease is None:
            return _unresolved_inheritance(), None, None
        reason = (
            "No applicable inheritance metadata is available for the selected disease."
            if not modes
            else "Multiple applicable inheritance modes are available."
        )
        choices = tuple(
            {"value": mode.value, "label": mode.value.replace("_", " ")}
            for mode in modes
        )
        return (
            _unresolved_inheritance(reason),
            ContextQuestion(
                field=ContextField.INHERITANCE,
                code=ContextIssueCode.INHERITANCE_REQUIRED,
                prompt="Select the inheritance mode for interpretation.",
                reason=reason,
                answer_schema={
                    "type": "string",
                    "enum": [
                        *(mode.value for mode in modes),
                        InheritanceMode.UNKNOWN.value,
                    ],
                },
                choices=(
                    *choices,
                    {"value": InheritanceMode.UNKNOWN.value, "label": "unknown"},
                ),
            ),
            None,
        )

    @staticmethod
    def _sorted_diseases(
        diseases: Iterable[GeneDiseaseKnowledgeRecord],
    ) -> tuple[GeneDiseaseKnowledgeRecord, ...]:
        return tuple(
            sorted(
                diseases,
                key=lambda record: (
                    record.mondo_id,
                    record.disease_label,
                    record.mode_of_inheritance,
                    record.classification,
                ),
            )
        )


def _provenance(
    source: ContextValueSource,
    *,
    source_id: str | None = None,
    record_id: str | None = None,
    rationale: str,
) -> ResolutionProvenance:
    confidence = {
        ContextValueSource.USER_CONFIRMED: "confirmed",
        ContextValueSource.RULESET_SPECIFICATION: "specified",
        ContextValueSource.BUNDLE_METADATA: "unambiguous",
        ContextValueSource.NORMALIZATION_PROVIDER: "resolved",
        ContextValueSource.UNRESOLVED: "unresolved",
    }[source]
    return ResolutionProvenance(source, source_id, record_id, confidence, rationale)


def _unresolved_transcript() -> ResolvedValue[str]:
    return ResolvedValue(
        ContextField.TRANSCRIPT,
        None,
        _provenance(
            ContextValueSource.UNRESOLVED,
            rationale="No transcript has been safely selected.",
        ),
    )


def _unresolved_inheritance(
    rationale: str = "Inheritance is not resolved.",
) -> ResolvedValue[InheritanceMode]:
    return ResolvedValue(
        ContextField.INHERITANCE,
        None,
        _provenance(ContextValueSource.UNRESOLVED, rationale=rationale),
    )


def _distinct_diseases(
    records: tuple[GeneDiseaseKnowledgeRecord, ...],
) -> tuple[GeneDiseaseKnowledgeRecord, ...]:
    unique: dict[tuple[str, str], GeneDiseaseKnowledgeRecord] = {}
    for record in records:
        unique.setdefault((record.mondo_id, record.disease_label), record)
    return tuple(unique[key] for key in sorted(unique))


def _as_inheritance(value: str) -> InheritanceMode | None:
    try:
        return InheritanceMode(value)
    except ValueError:
        return None


def _inheritance_modes(
    records: Iterable[GeneDiseaseKnowledgeRecord],
) -> tuple[InheritanceMode, ...]:
    return tuple(
        sorted(
            {
                mode
                for record in records
                if (mode := _as_inheritance(record.mode_of_inheritance)) is not None
            },
            key=lambda mode: mode.value,
        )
    )
