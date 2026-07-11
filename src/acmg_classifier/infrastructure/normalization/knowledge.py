"""Read-only access to the signed active knowledge-bundle artifact."""

from __future__ import annotations

import hashlib
import sqlite3
from collections.abc import Iterator, Mapping
from contextlib import contextmanager
from dataclasses import dataclass
from pathlib import Path
from types import MappingProxyType
from typing import Protocol

from pydantic import ValidationError

from acmg_classifier.domain.enums import GenomeBuild
from acmg_classifier.infrastructure.bundles.manager import (
    BundleManager,
    BundleStateError,
)
from acmg_classifier.infrastructure.bundles.manifest import BundleManifest
from acmg_classifier.infrastructure.bundles.verifier import BundleVerificationError

_KNOWLEDGE_ARTIFACT_ROLE = "knowledge"
_KNOWLEDGE_MEDIA_TYPE = "application/vnd.sqlite3"
_KNOWLEDGE_SCHEMA_VERSION = "1.0"
_REQUIRED_TABLE_COLUMNS: Mapping[str, frozenset[str]] = MappingProxyType(
    {
        "metadata": frozenset({"key", "value"}),
        "transcript_mappings": frozenset(
            {
                "refseq_transcript",
                "ensembl_transcript",
                "gene_id",
                "hgnc_id",
                "gene_symbol",
                "mane_status",
                "genome_build",
                "genomic_accession",
                "start",
                "end",
                "strand",
            }
        ),
        "gene_disease": frozenset(
            {
                "hgnc_id",
                "mondo_id",
                "gene_symbol",
                "disease_label",
                "mode_of_inheritance",
                "sop",
                "classification",
                "report_url",
                "classification_date",
                "expert_panel",
            }
        ),
    }
)


class KnowledgeBundleError(RuntimeError):
    """A signed bundle or its knowledge artifact cannot be safely consumed."""


@dataclass(frozen=True, slots=True)
class KnowledgeBundle:
    """The verified knowledge artifact and signed manifest that describe it."""

    root: Path
    manifest: BundleManifest
    knowledge_path: Path
    artifact_path: str


class KnowledgeBundleLocator(Protocol):
    """Locates a previously verified, active immutable knowledge bundle."""

    def active_knowledge(self) -> KnowledgeBundle:
        """Return the active verified knowledge artifact."""


class ActiveKnowledgeBundleLocator:
    """Re-verify the active manager artifact before exposing its SQLite path."""

    def __init__(self, manager: BundleManager) -> None:
        self._manager = manager

    def active_knowledge(self) -> KnowledgeBundle:
        """Locate the signed active knowledge artifact selected by its manifest role."""
        try:
            state = self._manager.status()
        except BundleStateError as error:
            raise KnowledgeBundleError("active bundle pointer is invalid") from error
        if state.active_version is None:
            raise KnowledgeBundleError("no active bundle is installed")

        bundle_parent = (self._manager.root / "bundles").resolve()
        root = (bundle_parent / state.active_version).resolve()
        try:
            root.relative_to(bundle_parent)
        except ValueError as error:
            raise KnowledgeBundleError(
                "active bundle path escapes bundle storage"
            ) from error
        manifest_path = root / "manifest.json"
        signature_path = root / "manifest.sig"
        if (
            not root.is_dir()
            or not manifest_path.is_file()
            or not signature_path.is_file()
        ):
            raise KnowledgeBundleError("active bundle metadata is missing")
        try:
            manifest = BundleManifest.model_validate_json(manifest_path.read_bytes())
            signature = signature_path.read_bytes()
            if not signature:
                raise KnowledgeBundleError("active bundle signature is missing")
            self._manager.verifier.verify_manifest_signature(manifest, signature)
        except KnowledgeBundleError:
            raise
        except (OSError, ValidationError, ValueError, BundleVerificationError) as error:
            raise KnowledgeBundleError(
                "active bundle manifest or signature is invalid"
            ) from error
        if manifest.bundle_version != state.active_version:
            raise KnowledgeBundleError("active bundle version does not match manifest")

        artifacts = tuple(
            artifact
            for artifact in manifest.artifacts
            if artifact.role == _KNOWLEDGE_ARTIFACT_ROLE
        )
        if len(artifacts) != 1:
            raise KnowledgeBundleError(
                "active bundle must contain exactly one knowledge artifact"
            )
        artifact = artifacts[0]
        if artifact.media_type != _KNOWLEDGE_MEDIA_TYPE:
            raise KnowledgeBundleError(
                "knowledge artifact has an unsupported media type"
            )
        knowledge_path = (root / artifact.path).resolve()
        try:
            knowledge_path.relative_to(root)
        except ValueError as error:
            raise KnowledgeBundleError(
                "knowledge artifact resolves outside its bundle"
            ) from error
        try:
            if not knowledge_path.is_file():
                raise KnowledgeBundleError("knowledge artifact is missing")
            if knowledge_path.stat().st_size != artifact.byte_size:
                raise KnowledgeBundleError("knowledge artifact size is invalid")
            if _sha256(knowledge_path) != artifact.sha256:
                raise KnowledgeBundleError("knowledge artifact checksum is invalid")
        except KnowledgeBundleError:
            raise
        except OSError as error:
            raise KnowledgeBundleError("knowledge artifact cannot be read") from error
        return KnowledgeBundle(root, manifest, knowledge_path, artifact.path)


@dataclass(frozen=True, slots=True)
class TranscriptMappingRow:
    """Runtime-owned projection of one signed transcript mapping row."""

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
class GeneDiseaseRow:
    """Runtime-owned projection of one signed gene-disease knowledge row."""

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


class KnowledgeBundleRepository:
    """Query the active signed knowledge database without ever mutating it."""

    def __init__(self, locator: KnowledgeBundleLocator) -> None:
        self._bundle = locator.active_knowledge()
        self._metadata = self._validate_schema()

    @property
    def bundle(self) -> KnowledgeBundle:
        """Return the verified artifact provenance used by this repository."""
        return self._bundle

    @property
    def metadata(self) -> Mapping[str, str]:
        """Return validated immutable knowledge metadata."""
        return self._metadata

    @contextmanager
    def connection(self) -> Iterator[sqlite3.Connection]:
        """Yield a SQLite connection forced into immutable read-only mode."""
        try:
            database = sqlite3.connect(
                f"{self._bundle.knowledge_path.resolve().as_uri()}?mode=ro&immutable=1",
                uri=True,
            )
            database.execute("PRAGMA query_only = ON")
        except (OSError, sqlite3.Error) as error:
            raise KnowledgeBundleError(
                "knowledge artifact cannot be opened read-only"
            ) from error
        try:
            database.row_factory = sqlite3.Row
            yield database
        finally:
            database.close()

    def transcript_by_accession(self, accession: str) -> TranscriptMappingRow | None:
        """Return an exact signed RefSeq transcript mapping, if present."""
        with self.connection() as database:
            row = database.execute(
                """
                SELECT refseq_transcript, ensembl_transcript, gene_id, hgnc_id,
                       gene_symbol, mane_status, genome_build, genomic_accession,
                       start, end, strand
                FROM transcript_mappings
                WHERE refseq_transcript = ?
                """,
                (accession,),
            ).fetchone()
        return self._transcript_row(row) if row is not None else None

    def transcripts_for_gene(
        self, gene_symbol: str
    ) -> tuple[TranscriptMappingRow, ...]:
        """Return deterministically ordered signed transcript mappings for a gene."""
        with self.connection() as database:
            rows = database.execute(
                """
                SELECT refseq_transcript, ensembl_transcript, gene_id, hgnc_id,
                       gene_symbol, mane_status, genome_build, genomic_accession,
                       start, end, strand
                FROM transcript_mappings
                WHERE gene_symbol = ?
                ORDER BY mane_status, refseq_transcript, ensembl_transcript
                """,
                (gene_symbol,),
            ).fetchall()
        return tuple(self._transcript_row(row) for row in rows)

    def gene_diseases(self, gene_symbol: str) -> tuple[GeneDiseaseRow, ...]:
        """Return deterministically ordered signed gene-disease records."""
        with self.connection() as database:
            rows = database.execute(
                """
                SELECT hgnc_id, mondo_id, gene_symbol, disease_label,
                       mode_of_inheritance, sop, classification, report_url,
                       classification_date, expert_panel
                FROM gene_disease
                WHERE gene_symbol = ?
                ORDER BY mondo_id, mode_of_inheritance, classification
                """,
                (gene_symbol,),
            ).fetchall()
        return tuple(
            GeneDiseaseRow(
                hgnc_id=row["hgnc_id"],
                mondo_id=row["mondo_id"],
                gene_symbol=row["gene_symbol"],
                disease_label=row["disease_label"],
                mode_of_inheritance=row["mode_of_inheritance"],
                sop=row["sop"],
                classification=row["classification"],
                report_url=row["report_url"],
                classification_date=row["classification_date"],
                expert_panel=row["expert_panel"],
            )
            for row in rows
        )

    def _validate_schema(self) -> Mapping[str, str]:
        try:
            with self.connection() as database:
                for table, expected_columns in _REQUIRED_TABLE_COLUMNS.items():
                    actual_columns = {
                        row["name"]
                        for row in database.execute(f"PRAGMA table_info({table})")
                    }
                    if not expected_columns.issubset(actual_columns):
                        raise KnowledgeBundleError(
                            f"knowledge schema is missing required columns for {table}"
                        )
                metadata = {
                    row["key"]: row["value"]
                    for row in database.execute("SELECT key, value FROM metadata")
                }
        except KnowledgeBundleError:
            raise
        except sqlite3.Error as error:
            raise KnowledgeBundleError(
                "knowledge artifact has an invalid schema"
            ) from error
        if metadata.get("schema_version") != _KNOWLEDGE_SCHEMA_VERSION:
            raise KnowledgeBundleError("knowledge schema_version is unsupported")
        if metadata.get("bundle_version") != self._bundle.manifest.bundle_version:
            raise KnowledgeBundleError(
                "knowledge bundle_version does not match manifest"
            )
        return MappingProxyType(metadata)

    @staticmethod
    def _transcript_row(row: sqlite3.Row) -> TranscriptMappingRow:
        try:
            return TranscriptMappingRow(
                refseq_transcript=row["refseq_transcript"],
                ensembl_transcript=row["ensembl_transcript"],
                gene_id=row["gene_id"],
                hgnc_id=row["hgnc_id"],
                gene_symbol=row["gene_symbol"],
                mane_status=row["mane_status"],
                genome_build=GenomeBuild(row["genome_build"]),
                genomic_accession=row["genomic_accession"],
                start=row["start"],
                end=row["end"],
                strand=row["strand"],
            )
        except (KeyError, TypeError, ValueError) as error:
            raise KnowledgeBundleError("knowledge transcript row is invalid") from error


def _sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        while chunk := source.read(1024 * 1024):
            digest.update(chunk)
    return digest.hexdigest()
