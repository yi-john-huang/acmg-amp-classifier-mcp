from __future__ import annotations

import sqlite3
from contextlib import closing
from pathlib import Path
from types import SimpleNamespace

from acmg_classifier.application.context import ContextResolutionService
from acmg_classifier.domain.enums import GenomeBuild
from acmg_classifier.domain.normalization import (
    CanonicalAlleleKey,
    HgvsAlias,
    NormalizedVariant,
    ProviderProvenance,
)
from acmg_classifier.infrastructure.normalization.knowledge import (
    KnowledgeBundle,
    KnowledgeBundleRepository,
)


class _StaticKnowledgeLocator:
    def __init__(self, bundle: KnowledgeBundle) -> None:
        self._bundle = bundle

    def active_knowledge(self) -> KnowledgeBundle:
        return self._bundle


def _repository(path: Path) -> KnowledgeBundleRepository:
    with closing(sqlite3.connect(path)) as database, database:
        database.executescript(
            """
            CREATE TABLE metadata (
                key TEXT PRIMARY KEY,
                value TEXT NOT NULL
            );
            CREATE TABLE transcript_mappings (
                refseq_transcript TEXT PRIMARY KEY,
                ensembl_transcript TEXT NOT NULL,
                gene_id TEXT NOT NULL,
                hgnc_id TEXT NOT NULL,
                gene_symbol TEXT NOT NULL,
                mane_status TEXT NOT NULL,
                genome_build TEXT NOT NULL,
                genomic_accession TEXT NOT NULL,
                start INTEGER NOT NULL,
                end INTEGER NOT NULL,
                strand TEXT NOT NULL
            );
            CREATE TABLE gene_disease (
                hgnc_id TEXT NOT NULL,
                mondo_id TEXT NOT NULL,
                gene_symbol TEXT NOT NULL,
                disease_label TEXT NOT NULL,
                mode_of_inheritance TEXT NOT NULL,
                sop TEXT NOT NULL,
                classification TEXT NOT NULL,
                report_url TEXT NOT NULL,
                classification_date TEXT NOT NULL,
                expert_panel TEXT NOT NULL
            );
            INSERT INTO metadata VALUES ('schema_version', '1.0');
            INSERT INTO metadata VALUES ('bundle_version', '2026.7.11');
            INSERT INTO transcript_mappings VALUES (
                'NM_000492.4', 'ENST00000003084.11', '1080', 'HGNC:1884', 'CFTR',
                'MANE Select', 'GRCh38', 'NC_000007.14', 117120016, 117308718, '+'
            );
            INSERT INTO gene_disease VALUES (
                'HGNC:1884', 'MONDO:0009061', 'CFTR', 'cystic fibrosis',
                'autosomal_recessive', 'Definitive', 'Definitive',
                'https://example.invalid/cftr', '2026-01-01', 'Panel'
            );
            """
        )
    bundle = KnowledgeBundle(
        root=path.parent,
        manifest=SimpleNamespace(bundle_version="2026.7.11"),
        knowledge_path=path,
        artifact_path="data/knowledge.sqlite3",
    )
    return KnowledgeBundleRepository(_StaticKnowledgeLocator(bundle))


def _normalized_cftr() -> NormalizedVariant:
    key = CanonicalAlleleKey(
        assembly=GenomeBuild.GRCH38,
        sequence_accession="NC_000007.14",
        start=117199644,
        end=117199645,
        deleted_sequence="A",
        inserted_sequence="G",
    )
    return NormalizedVariant(
        original_input="NC_000007.14:g.117199645A>G",
        parsed_input="NC_000007.14:g.117199645A>G",
        canonical_key=key,
        genome_build=GenomeBuild.GRCH38,
        genomic_accession="NC_000007.14",
        genomic_start=117199644,
        genomic_end=117199645,
        reference_allele="A",
        alternate_allele="G",
        normalized_hgvs="NC_000007.14:g.117199645A>G",
        hgvs_aliases=(HgvsAlias("NC_000007.14:g.117199645A>G", "genomic"),),
        gene_symbol="CFTR",
        transcript=None,
        transcript_hgvs=None,
        provider_provenance=(
            ProviderProvenance(
                provider_id="local_bundle",
                provider_version="1.0",
                source_record_id=None,
                retrieved_at=None,
                bundle_version="2026.7.11",
                raw_snapshot_ref="data/knowledge.sqlite3",
                query_key="cftr",
            ),
        ),
    )


def test_context_service_consumes_read_only_knowledge_repository(
    tmp_path: Path,
) -> None:
    repository = _repository(tmp_path / "knowledge.sqlite3")

    resolution = ContextResolutionService(repository).resolve(_normalized_cftr())

    assert resolution.status == "complete"
    assert resolution.resolved.transcript == "NM_000492.4"
    assert resolution.resolved.disease_id == "MONDO:0009061"
    assert resolution.resolved.inheritance is not None
    assert resolution.resolved.inheritance.value == "autosomal_recessive"
