from __future__ import annotations

import hashlib
import json
import socket
import sqlite3
from pathlib import Path

import pytest
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey

from acmg_classifier.application.normalization import VariantNormalizationService
from acmg_classifier.domain.enums import GenomeBuild
from acmg_classifier.domain.models import InterpretationContext
from acmg_classifier.domain.normalization import (
    CanonicalAlleleKey,
    HgvsAlias,
    NormalizationFailureCode,
    NormalizedVariant,
    ProviderProvenance,
    VariantInputParser,
)
from acmg_classifier.infrastructure.bundles.manager import BundleManager
from acmg_classifier.infrastructure.bundles.manifest import BundleManifest
from acmg_classifier.infrastructure.bundles.verifier import (
    BundleVerifier,
    canonical_manifest_bytes,
)
from acmg_classifier.infrastructure.normalization.knowledge import (
    ActiveKnowledgeBundleLocator,
    KnowledgeBundleError,
    KnowledgeBundleRepository,
)
from acmg_classifier.infrastructure.normalization.local import (
    LocalBundleNormalizationProvider,
)
from acmg_classifier.ports.normalization import (
    NormalizationFailure,
    NormalizationPolicy,
    NormalizationSuccess,
)

FIXTURES = Path(__file__).parents[2] / "fixtures" / "normalization"


def _create_knowledge_database(path: Path, *, bundle_version: str) -> None:
    with sqlite3.connect(path) as database:
        database.executescript(
            """
            CREATE TABLE metadata (
                key TEXT PRIMARY KEY,
                value TEXT NOT NULL
            ) WITHOUT ROWID;
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
            ) WITHOUT ROWID;
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
                expert_panel TEXT NOT NULL,
                PRIMARY KEY (hgnc_id, mondo_id, mode_of_inheritance, classification)
            ) WITHOUT ROWID;
            """
        )
        database.executemany(
            "INSERT INTO metadata(key, value) VALUES (?, ?)",
            (("schema_version", "1.0"), ("bundle_version", bundle_version)),
        )
        database.execute(
            """
            INSERT INTO transcript_mappings VALUES (
                'NM_000492.4', 'ENST00000003084.11', 'GeneID:1080', 'HGNC:1884',
                'CFTR', 'MANE Select', 'GRCh38', 'NC_000007.14', 117120016,
                117308718, '+'
            )
            """
        )
        database.execute(
            """
            INSERT INTO gene_disease VALUES (
                'HGNC:1884', 'MONDO:0009061', 'CFTR', 'cystic fibrosis',
                'autosomal_recessive', 'SOP:8', 'Definitive',
                'https://example.test/cftr', '2026-07-11', 'CFTR expert panel'
            )
            """
        )


def _install_signed_bundle(
    tmp_path: Path,
    *,
    metadata_bundle_version: str = "2026.7.11",
    invalid_schema: bool = False,
) -> BundleManager:
    root = tmp_path / "bundles-root"
    bundle_path = root / "bundles" / "2026.7.11"
    bundle_path.mkdir(parents=True)
    knowledge_path = bundle_path / "data" / "knowledge.sqlite3"
    knowledge_path.parent.mkdir()
    _create_knowledge_database(knowledge_path, bundle_version=metadata_bundle_version)
    if invalid_schema:
        with sqlite3.connect(knowledge_path) as database:
            database.execute("DROP TABLE gene_disease")

    private_key = Ed25519PrivateKey.generate()
    public_key = private_key.public_key().public_bytes(
        serialization.Encoding.Raw, serialization.PublicFormat.Raw
    )
    manifest = BundleManifest.model_validate(
        {
            "format_version": "1.0",
            "bundle_version": "2026.7.11",
            "application_version": {"minimum": "0.1.0", "maximum": "0.9.9"},
            "schema_version": {"minimum": "1.0", "maximum": "1.0"},
            "created_at": "2026-07-11T08:00:00Z",
            "channel": "test",
            "artifacts": [
                {
                    "path": "data/knowledge.sqlite3",
                    "byte_size": knowledge_path.stat().st_size,
                    "sha256": hashlib.sha256(knowledge_path.read_bytes()).hexdigest(),
                    "media_type": "application/vnd.sqlite3",
                    "role": "knowledge",
                }
            ],
            "sources": [
                {
                    "name": "test source",
                    "release": "1.0",
                    "url": "https://example.test/source",
                    "retrieved_at": "2026-07-11T07:00:00Z",
                    "license": "test-only fixture",
                    "sha256": "b" * 64,
                    "terms_url": "https://example.test/terms",
                    "transformation_version": "1.0.0",
                }
            ],
            "genome_builds": ["GRCh38"],
            "transcript_release": "test",
            "rulesets": [{"identifier": "acmg-amp", "version": "2015.1"}],
            "signer_key_id": "test-key",
            "signature_algorithm": "Ed25519",
        }
    )
    (bundle_path / "manifest.json").write_bytes(canonical_manifest_bytes(manifest))
    (bundle_path / "manifest.sig").write_bytes(
        private_key.sign(canonical_manifest_bytes(manifest))
    )
    (root / "active-bundle.json").write_text(
        json.dumps(
            {
                "active_version": manifest.bundle_version,
                "previous_version": None,
                "pinned_version": None,
            }
        )
    )
    return BundleManager(root, verifier=BundleVerifier({"test-key": public_key}))


def _repository(tmp_path: Path, **kwargs: object) -> KnowledgeBundleRepository:
    manager = _install_signed_bundle(tmp_path, **kwargs)
    return KnowledgeBundleRepository(ActiveKnowledgeBundleLocator(manager))


def _cached_variant() -> NormalizedVariant:
    fixture = json.loads(
        (FIXTURES / "local_bundle_cached_normalization.json").read_text()
    )
    key = CanonicalAlleleKey(
        assembly=GenomeBuild(fixture["canonical_key"]["assembly"]),
        sequence_accession=fixture["canonical_key"]["sequence_accession"],
        start=fixture["canonical_key"]["start"],
        end=fixture["canonical_key"]["end"],
        deleted_sequence=fixture["canonical_key"]["deleted_sequence"],
        inserted_sequence=fixture["canonical_key"]["inserted_sequence"],
    )
    return NormalizedVariant(
        original_input=fixture["query"],
        parsed_input=fixture["query"],
        canonical_key=key,
        genome_build=key.assembly,
        genomic_accession=key.sequence_accession,
        genomic_start=key.start,
        genomic_end=key.end,
        reference_allele=key.deleted_sequence,
        alternate_allele=key.inserted_sequence,
        normalized_hgvs=fixture["normalized_hgvs"],
        hgvs_aliases=(HgvsAlias(fixture["normalized_hgvs"], "genomic"),),
        gene_symbol=None,
        transcript=None,
        transcript_hgvs=None,
        provider_provenance=(
            ProviderProvenance(
                "fixture-cache",
                "1.0",
                "fixture-allele-1",
                None,
                None,
                "fixture:local-cache",
                fixture["query"],
            ),
        ),
    )


def test_repository_verifies_signed_active_bundle_and_is_read_only(
    tmp_path: Path,
) -> None:
    repository = _repository(tmp_path)

    assert repository.bundle.manifest.bundle_version == "2026.7.11"
    assert repository.transcript_by_accession("NM_000492.4").gene_symbol == "CFTR"
    assert repository.gene_diseases("CFTR")[0].mondo_id == "MONDO:0009061"
    with repository.connection() as database, pytest.raises(sqlite3.OperationalError):
        database.execute(
            "INSERT INTO metadata(key, value) VALUES ('write', 'forbidden')"
        )


def test_repository_rejects_knowledge_metadata_version_mismatch(tmp_path: Path) -> None:
    manager = _install_signed_bundle(tmp_path, metadata_bundle_version="2026.7.10")

    with pytest.raises(KnowledgeBundleError, match="bundle_version"):
        KnowledgeBundleRepository(ActiveKnowledgeBundleLocator(manager))


def test_repository_rejects_missing_required_knowledge_table(tmp_path: Path) -> None:
    manager = _install_signed_bundle(tmp_path, invalid_schema=True)

    with pytest.raises(KnowledgeBundleError, match="gene_disease"):
        KnowledgeBundleRepository(ActiveKnowledgeBundleLocator(manager))


def test_local_provider_replays_cached_normalization_with_bundle_provenance(
    tmp_path: Path,
) -> None:
    provider = LocalBundleNormalizationProvider(
        _repository(tmp_path),
        cached_results={"NC_000001.11:g.101A>G": _cached_variant()},
    )
    parsed = VariantInputParser().parse("NC_000001.11:g.101A>G")

    result = provider.normalize(
        parsed,
        context=InterpretationContext(),
        policy=NormalizationPolicy(mode="offline", allow_remote=False),
    )

    assert isinstance(result, NormalizationSuccess)
    assert result.normalized.canonical_key.value == "cak1:GRCh38:NC_000001.11:100:A>G"
    provenance = result.normalized.provider_provenance
    assert [item.provider_id for item in provenance] == [
        "fixture-cache",
        "local_bundle",
    ]
    assert provenance[-1].bundle_version == "2026.7.11"
    assert provenance[-1].raw_snapshot_ref == "data/knowledge.sqlite3"


def test_core_metadata_bundle_miss_is_typed_unavailable_without_socket(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    provider = LocalBundleNormalizationProvider(_repository(tmp_path))
    parsed = VariantInputParser().parse("NC_000001.11:g.101A>G")

    def no_socket(*args: object, **kwargs: object) -> object:
        raise AssertionError("offline local provider must not open sockets")

    monkeypatch.setattr(socket, "socket", no_socket)
    result = provider.normalize(
        parsed,
        context=InterpretationContext(),
        policy=NormalizationPolicy(mode="offline", allow_remote=False),
    )

    assert isinstance(result, NormalizationFailure)
    assert result.code is NormalizationFailureCode.NORMALIZATION_UNAVAILABLE
    assert result.details["reason"] == "no_reference_data"
    assert result.details["bundle_version"] == "2026.7.11"


def test_local_provider_agrees_with_application_service_offline(tmp_path: Path) -> None:
    provider = LocalBundleNormalizationProvider(
        _repository(tmp_path),
        cached_results={"NC_000001.11:g.101A>G": _cached_variant()},
    )

    result = VariantNormalizationService(providers=(provider,)).normalize(
        "NC_000001.11:g.101A>G",
        policy=NormalizationPolicy(mode="offline", allow_remote=False),
    )

    assert isinstance(result, NormalizationSuccess)
    assert result.normalized.provider_provenance[-1].provider_id == "local_bundle"
