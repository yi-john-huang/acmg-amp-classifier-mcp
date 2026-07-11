"""Deterministic transform, validate, package, and sign stages."""

import hashlib
import json
import os
import sqlite3
import stat
import tempfile
import zipfile
from contextlib import closing
from dataclasses import dataclass
from pathlib import Path
from typing import Any

from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
from pydantic import ValidationError

from acmg_classifier.domain.canonical import canonical_json_bytes
from acmg_classifier.domain.enums import GenomeBuild
from acmg_classifier.infrastructure.bundles.manifest import (
    BundleArtifact,
    BundleManifest,
    BundleSource,
    RulesetReference,
)
from acmg_classifier.infrastructure.bundles.verifier import canonical_manifest_bytes
from data_builder.models import BuildRecipe
from data_builder.parsers import (
    GeneDisease,
    SourceFormatError,
    TranscriptMapping,
    parse_clingen_gene_disease,
    parse_mane_summary,
)
from data_builder.sources import file_sha256


class BuildValidationError(RuntimeError):
    """A locked input or its transformed data failed closed."""


@dataclass(frozen=True, slots=True)
class BuildResult:
    """Paths and verified metadata emitted by one deterministic build."""

    archive: Path
    manifest: Path
    signature: Path
    build_report: Path
    archive_sha256: str
    manifest_model: BundleManifest


def load_recipe(path: Path) -> BuildRecipe:
    """Load a strict build recipe and translate validation errors."""
    try:
        return BuildRecipe.model_validate_json(path.read_bytes())
    except (OSError, ValidationError, ValueError) as error:
        raise BuildValidationError(
            f"invalid build recipe or license review: {error}"
        ) from error


def _load_ruleset(
    recipe_path: Path, recipe: BuildRecipe
) -> tuple[Path, dict[str, Any]]:
    path = Path(recipe.ruleset.path)
    if not path.is_absolute():
        path = recipe_path.parent / path
    if not path.is_file():
        raise BuildValidationError(f"ruleset input does not exist: {path}")
    if file_sha256(path) != recipe.ruleset.sha256:
        raise BuildValidationError("ruleset checksum mismatch")
    try:
        value = json.loads(path.read_bytes())
    except (OSError, json.JSONDecodeError) as error:
        raise BuildValidationError("ruleset is not valid JSON") from error
    if not isinstance(value, dict):
        raise BuildValidationError("ruleset root must be an object")
    if value.get("identifier") != recipe.ruleset.identifier:
        raise BuildValidationError("ruleset identifier differs from recipe")
    if value.get("version") != recipe.ruleset.version:
        raise BuildValidationError("ruleset version differs from recipe")
    if value.get("review_status") != "approved_for_candidate":
        raise BuildValidationError("ruleset is not approved for a candidate build")
    if not value.get("approved_by") or not value.get("approved_at"):
        raise BuildValidationError("ruleset approval metadata is incomplete")
    criteria = value.get("criteria")
    if (
        not isinstance(criteria, list)
        or not criteria
        or len(criteria) != len(set(criteria))
    ):
        raise BuildValidationError("ruleset criteria must be a non-empty unique list")
    return path, value


def _source_paths(recipe: BuildRecipe, input_directory: Path) -> dict[str, Path]:
    result: dict[str, Path] = {}
    for source in recipe.sources:
        path = input_directory / source.filename
        if not path.is_file():
            raise BuildValidationError(
                f"source input does not exist: {source.filename}"
            )
        if file_sha256(path) != source.sha256:
            raise BuildValidationError(f"source checksum mismatch: {source.identifier}")
        result[source.kind] = path
    return result


def _validate_clingen_mane_references(
    transcripts: tuple[TranscriptMapping, ...],
    gene_diseases: tuple[GeneDisease, ...],
) -> None:
    mane_gene_identities = frozenset(
        (row.hgnc_id, row.gene_symbol) for row in transcripts
    )
    for row in gene_diseases:
        if (row.hgnc_id, row.gene_symbol) not in mane_gene_identities:
            raise BuildValidationError(
                "ClinGen row has no matching MANE mapping for "
                f"{row.hgnc_id} and {row.gene_symbol}"
            )


def _write_knowledge_database(
    path: Path,
    transcripts: tuple[TranscriptMapping, ...],
    gene_diseases: tuple[GeneDisease, ...],
    recipe: BuildRecipe,
) -> None:
    with closing(sqlite3.connect(path)) as database:
        database.executescript(
            """
            PRAGMA page_size = 4096;
            PRAGMA auto_vacuum = NONE;
            PRAGMA journal_mode = DELETE;
            PRAGMA synchronous = FULL;
            PRAGMA application_id = 1094929735;
            PRAGMA user_version = 1;
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
                genome_build TEXT NOT NULL CHECK (genome_build = 'GRCh38'),
                genomic_accession TEXT NOT NULL,
                start INTEGER NOT NULL CHECK (start >= 0),
                end INTEGER NOT NULL CHECK (end >= start),
                strand TEXT NOT NULL CHECK (strand IN ('+', '-'))
            ) WITHOUT ROWID;
            CREATE INDEX transcript_mappings_gene
                ON transcript_mappings(gene_symbol, mane_status);
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
            CREATE INDEX gene_disease_symbol ON gene_disease(gene_symbol);
            """
        )
        metadata = {
            "bundle_version": recipe.bundle_version,
            "created_at": recipe.created_at.isoformat().replace("+00:00", "Z"),
            "schema_version": "1.0",
            **{
                f"source.{source.identifier}.sha256": source.sha256
                for source in recipe.sources
            },
        }
        database.executemany(
            "INSERT INTO metadata(key, value) VALUES (?, ?)", sorted(metadata.items())
        )
        database.executemany(
            """
            INSERT INTO transcript_mappings(
                refseq_transcript, ensembl_transcript, gene_id, hgnc_id,
                gene_symbol, mane_status, genome_build, genomic_accession,
                start, end, strand
            ) VALUES (?, ?, ?, ?, ?, ?, 'GRCh38', ?, ?, ?, ?)
            """,
            [
                (
                    row.refseq_transcript,
                    row.ensembl_transcript,
                    row.gene_id,
                    row.hgnc_id,
                    row.gene_symbol,
                    row.mane_status,
                    row.genomic_accession,
                    row.start,
                    row.end,
                    row.strand,
                )
                for row in sorted(transcripts, key=lambda item: item.refseq_transcript)
            ],
        )
        database.executemany(
            """
            INSERT INTO gene_disease(
                hgnc_id, mondo_id, gene_symbol, disease_label,
                mode_of_inheritance, sop, classification, report_url,
                classification_date, expert_panel
            ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
            """,
            [
                (
                    row.hgnc_id,
                    row.mondo_id,
                    row.gene_symbol,
                    row.disease_label,
                    row.mode_of_inheritance,
                    row.sop,
                    row.classification,
                    row.report_url,
                    row.classification_date,
                    row.expert_panel,
                )
                for row in sorted(
                    gene_diseases,
                    key=lambda item: (
                        item.hgnc_id,
                        item.mondo_id,
                        item.mode_of_inheritance,
                        item.classification,
                    ),
                )
            ],
        )
        database.commit()
        database.execute("VACUUM")


def _artifact(path: str, content: bytes, media_type: str, role: str) -> BundleArtifact:
    return BundleArtifact(
        path=path,
        byte_size=len(content),
        sha256=hashlib.sha256(content).hexdigest(),
        media_type=media_type,
        role=role,
    )


def _write_deterministic_zip(path: Path, artifacts: dict[str, bytes]) -> None:
    with zipfile.ZipFile(path, "w", compression=zipfile.ZIP_STORED) as archive:
        for name, content in sorted(artifacts.items()):
            info = zipfile.ZipInfo(name, date_time=(1980, 1, 1, 0, 0, 0))
            info.create_system = 3
            info.external_attr = (stat.S_IFREG | 0o644) << 16
            info.compress_type = zipfile.ZIP_STORED
            archive.writestr(info, content)


def build_core_bundle(
    recipe_path: Path,
    input_directory: Path,
    output_directory: Path,
    private_key_bytes: bytes,
) -> BuildResult:
    """Build and sign a core bundle exclusively from checksum-locked inputs."""
    recipe_path = Path(recipe_path)
    recipe = load_recipe(recipe_path)
    sources = _source_paths(recipe, Path(input_directory))
    _, ruleset = _load_ruleset(recipe_path, recipe)
    try:
        transcripts = parse_mane_summary(sources["mane_summary"])
        gene_diseases = parse_clingen_gene_disease(sources["clingen_gene_disease"])
    except SourceFormatError as error:
        raise BuildValidationError(str(error)) from error
    _validate_clingen_mane_references(transcripts, gene_diseases)
    try:
        private_key = Ed25519PrivateKey.from_private_bytes(private_key_bytes)
    except ValueError as error:
        raise BuildValidationError(
            "signing key must be a raw 32-byte Ed25519 key"
        ) from error

    output_directory = Path(output_directory)
    output_directory.mkdir(parents=True, exist_ok=True)
    stem = f"core-{recipe.bundle_version}"
    archive_path = output_directory / f"{stem}.zip"
    manifest_path = output_directory / f"{stem}.manifest.json"
    signature_path = output_directory / f"{stem}.manifest.sig"
    report_path = output_directory / f"{stem}.build.json"

    with tempfile.TemporaryDirectory(
        dir=output_directory, prefix=".build-"
    ) as temporary:
        database_path = Path(temporary) / "knowledge.sqlite3"
        _write_knowledge_database(database_path, transcripts, gene_diseases, recipe)
        ruleset_bytes = canonical_json_bytes(ruleset) + b"\n"
        notice = {
            "bundle_version": recipe.bundle_version,
            "research_use_only": True,
            "sources": [
                {
                    "name": source.name,
                    "release": source.release,
                    "url": str(source.url),
                    "sha256": source.sha256,
                    "license": source.license_id,
                    "terms_url": str(source.terms_url),
                    "retrieved_at": source.retrieved_at,
                    "license_reviewed_by": source.license_reviewed_by,
                    "license_reviewed_at": source.license_reviewed_at.isoformat(),
                    "transformation_version": source.transformation_version,
                }
                for source in recipe.sources
            ],
            "ruleset": {
                "identifier": recipe.ruleset.identifier,
                "version": recipe.ruleset.version,
                "sha256": recipe.ruleset.sha256,
                "citation_url": str(recipe.ruleset.citation_url),
                "transformation_version": recipe.ruleset.transformation_version,
            },
            "transformations": [
                {
                    "included_records": len(transcripts),
                    "input_kind": "mane_summary",
                    "output": "data/knowledge.sqlite3:transcript_mappings",
                    "policy": (
                        "require stable HGNC, RefSeq, Ensembl, and GRCh38 identifiers"
                    ),
                    "version": next(
                        source.transformation_version
                        for source in recipe.sources
                        if source.kind == "mane_summary"
                    ),
                },
                {
                    "included_records": len(gene_diseases),
                    "input_kind": "clingen_gene_disease",
                    "output": "data/knowledge.sqlite3:gene_disease",
                    "policy": "require stable HGNC and MONDO identifiers",
                    "version": next(
                        source.transformation_version
                        for source in recipe.sources
                        if source.kind == "clingen_gene_disease"
                    ),
                },
                {
                    "input_kind": "curated_ruleset",
                    "output": (
                        f"rules/{recipe.ruleset.identifier}-"
                        f"{recipe.ruleset.version}.json"
                    ),
                    "policy": "require approved_for_candidate review metadata",
                    "version": recipe.ruleset.transformation_version,
                },
            ],
        }
        ruleset_path = (
            f"rules/{recipe.ruleset.identifier}-{recipe.ruleset.version}.json"
        )
        artifacts = {
            "NOTICE.json": canonical_json_bytes(notice) + b"\n",
            "data/knowledge.sqlite3": database_path.read_bytes(),
            ruleset_path: ruleset_bytes,
        }
        artifact_models = (
            _artifact(
                "NOTICE.json", artifacts["NOTICE.json"], "application/json", "notice"
            ),
            _artifact(
                "data/knowledge.sqlite3",
                artifacts["data/knowledge.sqlite3"],
                "application/vnd.sqlite3",
                "knowledge",
            ),
            _artifact(ruleset_path, ruleset_bytes, "application/json", "ruleset"),
        )
        manifest = BundleManifest(
            format_version=recipe.format_version,
            bundle_version=recipe.bundle_version,
            application_version=recipe.application_version,
            schema_version=recipe.schema_version,
            created_at=recipe.created_at,
            channel=recipe.channel,
            artifacts=artifact_models,
            sources=tuple(
                BundleSource(
                    name=source.name,
                    release=source.release,
                    url=source.url,
                    retrieved_at=source.retrieved_at,
                    license=source.license_id,
                    sha256=source.sha256,
                    terms_url=source.terms_url,
                    transformation_version=source.transformation_version,
                )
                for source in recipe.sources
            ),
            genome_builds=tuple(GenomeBuild(build) for build in recipe.genome_builds),
            transcript_release=recipe.transcript_release,
            rulesets=(
                RulesetReference(
                    identifier=recipe.ruleset.identifier, version=recipe.ruleset.version
                ),
            ),
            signer_key_id=recipe.signer_key_id,
            signature_algorithm="Ed25519",
        )
        temporary_archive = Path(temporary) / archive_path.name
        _write_deterministic_zip(temporary_archive, artifacts)
        manifest_bytes = canonical_manifest_bytes(manifest)
        signature = private_key.sign(manifest_bytes)
        temporary_manifest = Path(temporary) / manifest_path.name
        temporary_signature = Path(temporary) / signature_path.name
        temporary_manifest.write_bytes(manifest_bytes + b"\n")
        temporary_signature.write_bytes(signature)
        archive_sha256 = file_sha256(temporary_archive)
        report = {
            "archive_sha256": archive_sha256,
            "artifact_count": len(artifacts),
            "bundle_version": recipe.bundle_version,
            "gene_disease_count": len(gene_diseases),
            "manifest_sha256": hashlib.sha256(manifest_bytes).hexdigest(),
            "recipe_sha256": file_sha256(recipe_path),
            "source_sha256": {
                source.identifier: source.sha256 for source in recipe.sources
            },
            "transcript_count": len(transcripts),
        }
        temporary_report = Path(temporary) / report_path.name
        temporary_report.write_bytes(canonical_json_bytes(report) + b"\n")
        for source, destination in (
            (temporary_archive, archive_path),
            (temporary_manifest, manifest_path),
            (temporary_signature, signature_path),
            (temporary_report, report_path),
        ):
            os.replace(source, destination)
    return BuildResult(
        archive=archive_path,
        manifest=manifest_path,
        signature=signature_path,
        build_report=report_path,
        archive_sha256=archive_sha256,
        manifest_model=manifest,
    )
