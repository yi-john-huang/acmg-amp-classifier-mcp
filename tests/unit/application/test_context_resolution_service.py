from __future__ import annotations

from dataclasses import dataclass

from acmg_classifier.application.context import ContextResolutionService
from acmg_classifier.domain.context import (
    ContextField,
    ContextIssueCode,
    ContextSpecification,
    ContextValueSource,
)
from acmg_classifier.domain.enums import GenomeBuild, InheritanceMode
from acmg_classifier.domain.models import InterpretationContext
from acmg_classifier.domain.normalization import (
    CanonicalAlleleKey,
    HgvsAlias,
    NormalizedVariant,
    ProviderProvenance,
)
from acmg_classifier.ports.context import (
    GeneDiseaseKnowledgeRecord,
    TranscriptKnowledgeRecord,
)


@dataclass(frozen=True)
class FakeKnowledge:
    transcripts: tuple[TranscriptKnowledgeRecord, ...]
    diseases: tuple[GeneDiseaseKnowledgeRecord, ...]

    def transcript_by_accession(
        self, accession: str
    ) -> TranscriptKnowledgeRecord | None:
        return next(
            (row for row in self.transcripts if row.refseq_transcript == accession),
            None,
        )

    def transcripts_for_gene(
        self, gene_symbol: str
    ) -> tuple[TranscriptKnowledgeRecord, ...]:
        return tuple(row for row in self.transcripts if row.gene_symbol == gene_symbol)

    def gene_diseases(self, gene_symbol: str) -> tuple[GeneDiseaseKnowledgeRecord, ...]:
        return tuple(row for row in self.diseases if row.gene_symbol == gene_symbol)


def _transcript(
    accession: str,
    *,
    gene: str = "BRCA1",
    status: str = "MANE Select",
    build: GenomeBuild = GenomeBuild.GRCH38,
) -> TranscriptKnowledgeRecord:
    return TranscriptKnowledgeRecord(
        refseq_transcript=accession,
        ensembl_transcript=f"ENST{accession[3:].replace('.', '')}",
        gene_id="672",
        hgnc_id="HGNC:1100",
        gene_symbol=gene,
        mane_status=status,
        genome_build=build,
        genomic_accession="NC_000017.11",
        start=43044295,
        end=43170245,
        strand="-",
    )


def _disease(
    mondo_id: str,
    inheritance: str,
    *,
    gene: str = "BRCA1",
    label: str = "Hereditary cancer",
) -> GeneDiseaseKnowledgeRecord:
    return GeneDiseaseKnowledgeRecord(
        hgnc_id="HGNC:1100",
        mondo_id=mondo_id,
        gene_symbol=gene,
        disease_label=label,
        mode_of_inheritance=inheritance,
        sop="Definitive",
        classification="Definitive",
        report_url="https://example.invalid/report",
        classification_date="2026-01-01",
        expert_panel="Panel",
    )


def _normalized(
    *,
    gene: str = "BRCA1",
    transcript: str | None = None,
    source_record_id: str | None = None,
) -> NormalizedVariant:
    key = CanonicalAlleleKey(
        assembly=GenomeBuild.GRCH38,
        sequence_accession="NC_000017.11",
        start=43071077,
        end=43071078,
        deleted_sequence="A",
        inserted_sequence="G",
    )
    return NormalizedVariant(
        original_input="NC_000017.11:g.43071078A>G",
        parsed_input="NC_000017.11:g.43071078A>G",
        canonical_key=key,
        genome_build=GenomeBuild.GRCH38,
        genomic_accession="NC_000017.11",
        genomic_start=43071077,
        genomic_end=43071078,
        reference_allele="A",
        alternate_allele="G",
        normalized_hgvs="NC_000017.11:g.43071078A>G",
        hgvs_aliases=(HgvsAlias("NC_000017.11:g.43071078A>G", "genomic"),),
        gene_symbol=gene,
        transcript=transcript,
        transcript_hgvs=None,
        provider_provenance=(
            ProviderProvenance(
                provider_id="local_bundle",
                provider_version="1.0",
                source_record_id=source_record_id,
                retrieved_at=None,
                bundle_version="2026.7.11",
                raw_snapshot_ref="data/knowledge.sqlite3",
                query_key="test",
            ),
        ),
    )


def _service(
    transcripts: tuple[TranscriptKnowledgeRecord, ...],
    diseases: tuple[GeneDiseaseKnowledgeRecord, ...] = (),
) -> ContextResolutionService:
    return ContextResolutionService(FakeKnowledge(transcripts, diseases))


def test_user_confirmed_transcript_wins_when_compatible() -> None:
    select = _transcript("NM_007294.4")
    plus = _transcript("NM_007299.5", status="MANE Plus Clinical")

    result = _service((select, plus)).resolve(
        _normalized(), InterpretationContext(transcript=select.refseq_transcript)
    )

    transcript = result.value_for(ContextField.TRANSCRIPT)
    assert result.resolved.transcript == select.refseq_transcript
    assert transcript.provenance.source is ContextValueSource.USER_CONFIRMED
    assert not [
        question
        for question in result.questions
        if question.field is ContextField.TRANSCRIPT
    ]


def test_ruleset_specification_transcript_wins_over_mane_rank() -> None:
    select = _transcript("NM_007294.4")
    plus = _transcript("NM_007299.5", status="MANE Plus Clinical")

    result = _service((select, plus)).resolve(
        _normalized(),
        specification=ContextSpecification(
            ruleset_id="brca-guidance",
            version="1.2",
            transcript=select.refseq_transcript,
        ),
    )

    transcript = result.value_for(ContextField.TRANSCRIPT)
    assert result.resolved.transcript == select.refseq_transcript
    assert transcript.provenance.source is ContextValueSource.RULESET_SPECIFICATION


def test_mane_plus_clinical_ranks_before_mane_select() -> None:
    select = _transcript("NM_007294.4")
    plus = _transcript("NM_007299.5", status="MANE Plus Clinical")

    result = _service((select, plus)).resolve(_normalized())

    assert result.resolved.transcript == plus.refseq_transcript
    assert [
        candidate.refseq_transcript for candidate in result.transcript_candidates
    ] == [
        plus.refseq_transcript,
        select.refseq_transcript,
    ]
    assert (
        result.value_for(ContextField.TRANSCRIPT).provenance.source
        is ContextValueSource.BUNDLE_METADATA
    )


def test_unique_mane_select_resolves_with_bundle_metadata_provenance() -> None:
    select = _transcript("NM_000492.4", gene="CFTR")

    result = _service((select,)).resolve(_normalized(gene="CFTR"))

    assert result.resolved.transcript == select.refseq_transcript
    assert (
        result.value_for(ContextField.TRANSCRIPT).provenance.source
        is ContextValueSource.BUNDLE_METADATA
    )


def test_multiple_equal_rank_transcripts_are_ranked_but_not_selected() -> None:
    later = _transcript("NM_000002.3", gene="GENE2", status="MANE Plus Clinical")
    earlier = _transcript("NM_000001.2", gene="GENE2", status="MANE Plus Clinical")

    result = _service((later, earlier)).resolve(_normalized(gene="GENE2"))

    assert result.resolved.transcript is None
    assert (
        result.value_for(ContextField.TRANSCRIPT).provenance.source
        is ContextValueSource.UNRESOLVED
    )
    question = next(
        question
        for question in result.questions
        if question.field is ContextField.TRANSCRIPT
    )
    assert question.code is ContextIssueCode.TRANSCRIPT_AMBIGUOUS
    assert [choice["value"] for choice in question.choices] == [
        earlier.refseq_transcript,
        later.refseq_transcript,
    ]


def test_missing_disease_questions_when_transcript_resolved() -> None:
    result = _service((_transcript("NM_000492.4", gene="CFTR"),)).resolve(
        _normalized(gene="CFTR")
    )

    assert [question.field for question in result.questions] == [ContextField.DISEASE]
    assert result.questions[0].code is ContextIssueCode.DISEASE_REQUIRED


def test_clinvar_source_assertion_is_not_used_to_resolve_disease() -> None:
    result = _service((_transcript("NM_000492.4", gene="CFTR"),)).resolve(
        _normalized(gene="CFTR", source_record_id="ClinVar:VCV000000001")
    )

    disease = result.value_for(ContextField.DISEASE)
    assert disease.value is None
    assert disease.provenance.source is ContextValueSource.UNRESOLVED
    assert result.questions[0].code is ContextIssueCode.DISEASE_REQUIRED


def test_multiple_disease_and_inheritance_questions_are_stably_ordered() -> None:
    transcript = _transcript("NM_000001.2", gene="GENE3")
    result = _service(
        (transcript,),
        (
            _disease("MONDO:0002", "autosomal_recessive", gene="GENE3", label="Beta"),
            _disease("MONDO:0001", "autosomal_dominant", gene="GENE3", label="Alpha"),
        ),
    ).resolve(_normalized(gene="GENE3"))

    assert [question.field for question in result.questions] == [
        ContextField.DISEASE,
        ContextField.INHERITANCE,
    ]
    assert [choice["value"] for choice in result.questions[0].choices] == [
        "MONDO:0001",
        "MONDO:0002",
    ]


def test_user_confirmed_unknown_inheritance_is_not_inferred() -> None:
    transcript = _transcript("NM_000492.4", gene="CFTR")
    result = _service(
        (transcript,), (_disease("MONDO:0001", "autosomal_recessive", gene="CFTR"),)
    ).resolve(
        _normalized(gene="CFTR"),
        InterpretationContext(inheritance=InheritanceMode.UNKNOWN),
    )

    inheritance = result.value_for(ContextField.INHERITANCE)
    assert result.resolved.inheritance is InheritanceMode.UNKNOWN
    assert inheritance.provenance.source is ContextValueSource.USER_CONFIRMED


def test_supplied_transcript_not_found_is_explicit_conflict() -> None:
    result = _service((_transcript("NM_007294.4"),)).resolve(
        _normalized(), InterpretationContext(transcript="NM_999999.1")
    )

    assert result.status == "conflict"
    assert result.issues[0].code is ContextIssueCode.TRANSCRIPT_NOT_FOUND
    assert result.questions == ()


def test_conflicting_transcript_gene_is_explicit_conflict() -> None:
    wrong_gene = _transcript("NM_000492.4", gene="CFTR")
    result = _service((wrong_gene,)).resolve(
        _normalized(gene="BRCA1"),
        InterpretationContext(transcript=wrong_gene.refseq_transcript),
    )

    assert result.status == "conflict"
    assert result.issues[0].code is ContextIssueCode.TRANSCRIPT_CONFLICT
    assert result.questions == ()


def test_resolution_provenance_and_content_are_deterministic() -> None:
    select = _transcript("NM_007294.4")
    plus = _transcript("NM_007299.5", status="MANE Plus Clinical")
    diseases = (
        _disease("MONDO:0002", "autosomal_dominant"),
        _disease("MONDO:0001", "autosomal_dominant"),
    )

    first = _service((select, plus), diseases).resolve(_normalized())
    second = _service((plus, select), tuple(reversed(diseases))).resolve(_normalized())

    assert first.to_canonical_content() == second.to_canonical_content()
