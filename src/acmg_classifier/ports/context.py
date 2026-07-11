"""Narrow read-only transcript and gene-disease knowledge boundary."""

from __future__ import annotations

from dataclasses import dataclass
from typing import Protocol

from acmg_classifier.domain.enums import GenomeBuild


@dataclass(frozen=True, slots=True)
class TranscriptKnowledgeRecord:
    """One transcript-mapping row supplied by a trusted knowledge bundle."""

    refseq_transcript: str
    ensembl_transcript: str
    gene_id: str
    hgnc_id: str
    gene_symbol: str
    mane_status: str
    genome_build: GenomeBuild
    genomic_accession: str
    start: int
    end: int
    strand: str


@dataclass(frozen=True, slots=True)
class GeneDiseaseKnowledgeRecord:
    """One gene-disease metadata row supplied by a trusted knowledge bundle."""

    hgnc_id: str
    mondo_id: str
    gene_symbol: str
    disease_label: str
    mode_of_inheritance: str
    sop: str
    classification: str
    report_url: str
    classification_date: str
    expert_panel: str


class TranscriptKnowledgeRepository(Protocol):
    """Read-only lookup required by context resolution and nothing more."""

    def transcript_by_accession(
        self, accession: str
    ) -> TranscriptKnowledgeRecord | None:
        """Return an exact, versioned RefSeq transcript mapping when present."""

    def transcripts_for_gene(
        self, gene_symbol: str
    ) -> tuple[TranscriptKnowledgeRecord, ...]:
        """Return mappings for the requested gene."""

    def gene_diseases(self, gene_symbol: str) -> tuple[GeneDiseaseKnowledgeRecord, ...]:
        """Return curated gene-disease metadata for the requested gene."""
