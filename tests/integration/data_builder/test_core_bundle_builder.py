from __future__ import annotations

import gc
import hashlib
import json
import sqlite3
from contextlib import closing
from pathlib import Path

import pytest
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey

FIXTURES = Path(__file__).parents[2] / "fixtures" / "data_builder"


def _sha256(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def _ruleset(path: Path) -> None:
    path.write_text(
        json.dumps(
            {
                "identifier": "acmg-amp",
                "version": "2015.1",
                "review_status": "approved_for_candidate",
                "approved_by": "project-maintainer",
                "approved_at": "2026-07-11",
                "criteria": ["PVS1", "PS1", "PM2", "PP3", "BA1", "BS1", "BP4"],
            },
            sort_keys=True,
            separators=(",", ":"),
        )
        + "\n"
    )


def _recipe(tmp_path: Path, *, license_id: str = "CC0-1.0") -> Path:
    inputs = tmp_path / "inputs"
    inputs.mkdir()
    mane = inputs / "mane.tsv"
    clingen = inputs / "clingen.csv"
    mane.write_bytes((FIXTURES / "mane_summary.tsv").read_bytes())
    clingen.write_bytes((FIXTURES / "clingen_gene_disease.csv").read_bytes())
    ruleset = tmp_path / "ruleset.json"
    _ruleset(ruleset)
    recipe = {
        "format_version": "1.0",
        "bundle_version": "2026.7.10",
        "application_version": {"minimum": "0.1.0", "maximum": "0.9.9"},
        "schema_version": {"minimum": "1.0", "maximum": "1.0"},
        "created_at": "2026-07-10T17:00:00Z",
        "channel": "candidate",
        "genome_builds": ["GRCh38"],
        "transcript_release": "MANE Select v1.5",
        "signer_key_id": "test-candidate-key",
        "sources": [
            {
                "identifier": "mane-v1.5",
                "kind": "mane_summary",
                "name": "NCBI/EMBL-EBI MANE",
                "release": "1.5",
                "url": "https://ftp.ncbi.nlm.nih.gov/refseq/MANE/MANE_human/release_1.5/MANE.GRCh38.v1.5.summary.txt.gz",
                "filename": "mane.tsv",
                "sha256": _sha256(mane),
                "retrieved_at": "2026-07-10T17:00:00Z",
                "license_id": "NCBI-MOLECULAR-DATA-TERMS",
                "terms_url": "https://www.ncbi.nlm.nih.gov/home/about/policies/",
                "redistribution_allowed": True,
                "license_reviewed_by": "project-maintainer",
                "license_reviewed_at": "2026-07-11",
                "transformation_version": "1.0.0",
            },
            {
                "identifier": "clingen-gdv-2026-07-10",
                "kind": "clingen_gene_disease",
                "name": "ClinGen Gene-Disease Validity",
                "release": "2026-07-10",
                "url": "https://search.clinicalgenome.org/kb/gene-validity/download",
                "filename": "clingen.csv",
                "sha256": _sha256(clingen),
                "retrieved_at": "2026-07-10T17:00:00Z",
                "license_id": license_id,
                "terms_url": "https://www.clinicalgenome.org/docs/terms-of-use/",
                "redistribution_allowed": True,
                "license_reviewed_by": "project-maintainer",
                "license_reviewed_at": "2026-07-11",
                "transformation_version": "1.0.0",
            },
        ],
        "ruleset": {
            "identifier": "acmg-amp",
            "version": "2015.1",
            "path": str(ruleset),
            "sha256": _sha256(ruleset),
            "citation_url": "https://pubmed.ncbi.nlm.nih.gov/25741868/",
            "transformation_version": "1.0.0",
        },
    }
    path = tmp_path / "recipe.json"
    path.write_text(json.dumps(recipe, indent=2) + "\n")
    return path


def _raw_private_key() -> tuple[Ed25519PrivateKey, bytes]:
    key = Ed25519PrivateKey.generate()
    raw = key.private_bytes(
        serialization.Encoding.Raw,
        serialization.PrivateFormat.Raw,
        serialization.NoEncryption(),
    )
    return key, raw


def test_build_is_reproducible_and_runtime_verifiable(tmp_path: Path) -> None:
    from data_builder.builder import build_core_bundle

    from acmg_classifier.infrastructure.bundles.verifier import BundleVerifier

    recipe = _recipe(tmp_path)
    private_key, private_bytes = _raw_private_key()
    first = build_core_bundle(
        recipe, tmp_path / "inputs", tmp_path / "one", private_bytes
    )
    second = build_core_bundle(
        recipe, tmp_path / "inputs", tmp_path / "two", private_bytes
    )

    assert first.archive.read_bytes() == second.archive.read_bytes()
    assert first.manifest.read_bytes() == second.manifest.read_bytes()
    assert first.signature.read_bytes() == second.signature.read_bytes()
    assert first.archive_sha256 == second.archive_sha256
    public_key = private_key.public_key().public_bytes(
        serialization.Encoding.Raw, serialization.PublicFormat.Raw
    )
    verified = BundleVerifier({"test-candidate-key": public_key}).verify_and_extract(
        first.archive,
        first.manifest_model,
        first.signature.read_bytes(),
        tmp_path / "verified",
    )
    knowledge_database = verified.staging_path / "data" / "knowledge.sqlite3"
    with closing(sqlite3.connect(knowledge_database)) as db:
        assert db.execute("SELECT COUNT(*) FROM transcript_mappings").fetchone() == (3,)
        assert db.execute("SELECT COUNT(*) FROM gene_disease").fetchone() == (3,)
        assert db.execute(
            "SELECT refseq_transcript FROM transcript_mappings WHERE gene_symbol = ?",
            ("CFTR",),
        ).fetchone() == ("NM_000492.4",)

    sources = first.manifest_model.sources
    assert {source.transformation_version for source in sources} == {"1.0.0"}
    assert all(source.sha256 and source.terms_url for source in sources)


@pytest.mark.parametrize(
    ("mutation", "message"),
    [
        ("bad_hgnc", "HGNC"),
        ("bad_coordinates", "coordinates"),
        ("bad_gene_id", "GeneID"),
        ("bad_license", "license"),
        ("bad_checksum", "checksum"),
    ],
)
def test_invalid_official_inputs_fail_closed(
    tmp_path: Path, mutation: str, message: str
) -> None:
    from data_builder.builder import BuildValidationError, build_core_bundle

    recipe_path = _recipe(
        tmp_path, license_id="unknown" if mutation == "bad_license" else "CC0-1.0"
    )
    recipe = json.loads(recipe_path.read_text())
    if mutation == "bad_gene_id":
        mane = tmp_path / "inputs" / "mane.tsv"
        mane.write_text(mane.read_text().replace("GeneID:672", "672", 1))
        recipe["sources"][0]["sha256"] = _sha256(mane)
    elif mutation in {"bad_hgnc", "bad_coordinates"}:
        mane = tmp_path / "inputs" / "mane.tsv"
        content = mane.read_text()
        if mutation == "bad_hgnc":
            content = content.replace("HGNC:1100", "HGNC:BRCA1", 1)
        else:
            content = content.replace("43044295\t43125364", "43125364\t43044295", 1)
        mane.write_text(content)
        recipe["sources"][0]["sha256"] = _sha256(mane)
    elif mutation == "bad_checksum":
        recipe["sources"][0]["sha256"] = "0" * 64
    recipe_path.write_text(json.dumps(recipe))

    with pytest.raises(BuildValidationError, match=message):
        build_core_bundle(
            recipe_path, tmp_path / "inputs", tmp_path / "output", _raw_private_key()[1]
        )


@pytest.mark.parametrize(
    ("replacement", "message"),
    [
        ('"BRCA1","HGNC:1100"', '"BRCA1","HGNC:999999"'),
        ('"BRCA1","HGNC:1100"', '"CFTR","HGNC:1100"'),
    ],
)
def test_clingen_rows_require_a_matching_mane_hgnc_symbol_pair(
    tmp_path: Path, replacement: str, message: str
) -> None:
    from data_builder.builder import BuildValidationError, build_core_bundle

    recipe_path = _recipe(tmp_path)
    recipe = json.loads(recipe_path.read_text())
    clingen = tmp_path / "inputs" / "clingen.csv"
    clingen.write_text(clingen.read_text().replace(replacement, message, 1))
    recipe["sources"][1]["sha256"] = _sha256(clingen)
    recipe_path.write_text(json.dumps(recipe))

    with pytest.raises(BuildValidationError, match="matching MANE mapping"):
        build_core_bundle(
            recipe_path, tmp_path / "inputs", tmp_path / "output", _raw_private_key()[1]
        )


@pytest.mark.filterwarnings("error::ResourceWarning")
@pytest.mark.filterwarnings("error::pytest.PytestUnraisableExceptionWarning")
def test_build_closes_sqlite_connections(tmp_path: Path) -> None:
    from data_builder.builder import build_core_bundle

    recipe = _recipe(tmp_path)
    build_core_bundle(
        recipe, tmp_path / "inputs", tmp_path / "output", _raw_private_key()[1]
    )
    gc.collect()


def test_production_recipe_pins_official_release_and_reviewed_terms() -> None:
    recipe = json.loads((Path("data_builder/recipes/core-2026.7.10.json")).read_text())
    sources = {source["identifier"]: source for source in recipe["sources"]}

    mane = sources["mane-v1.5"]
    assert "/release_1.5/" in mane["url"]
    assert (
        mane["sha256"]
        == "d10ace2720681a3b2e0eefd9da4f551274a6b4141ac9bfd6a2565dfb6e9ad55c"
    )
    clingen = sources["clingen-gdv-2026-07-10"]
    assert clingen["release"] == "2026-07-10"
    assert (
        clingen["sha256"]
        == "446f2609932cf8fcae1169b16485ba9ba8c2ec89c5ec4d9a42d19b99969062d4"
    )
    assert clingen["license_id"] == "CC0-1.0"
    assert all(source["redistribution_allowed"] for source in sources.values())
    release = json.loads(Path("data_builder/releases/core-2026.7.10.json").read_text())
    assert release["candidate_status"] == "reproducible"
    assert release["transcript_count"] == 19385
    assert release["gene_disease_count"] == 3646
    assert release["scientific_release_gate"] == "pending_task_10.2"
