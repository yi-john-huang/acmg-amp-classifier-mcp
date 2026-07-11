from __future__ import annotations

import json
from dataclasses import replace
from pathlib import Path

import pytest

from acmg_classifier.domain.canonical import canonical_hash
from acmg_classifier.domain.enums import GenomeBuild
from acmg_classifier.domain.normalization import (
    CanonicalAlleleKey,
    HgvsAlias,
    NormalizedVariant,
    ProviderProvenance,
)

_FIXTURES = Path(__file__).parents[2] / "fixtures" / "normalization"


def _allele(
    *,
    start: int = 100,
    end: int = 101,
    deleted: str = "A",
    inserted: str = "G",
) -> CanonicalAlleleKey:
    return CanonicalAlleleKey(
        assembly=GenomeBuild.GRCH38,
        sequence_accession="NC_000001.11",
        start=start,
        end=end,
        deleted_sequence=deleted,
        inserted_sequence=inserted,
    )


def _normalized(
    *,
    aliases: tuple[HgvsAlias, ...] = (),
    provenance: tuple[ProviderProvenance, ...] = (),
) -> NormalizedVariant:
    allele = _allele()
    return NormalizedVariant(
        original_input="NC_000001.11:g.101A>G",
        parsed_input="NC_000001.11:g.101A>G",
        canonical_key=allele,
        genome_build=allele.assembly,
        genomic_accession=allele.sequence_accession,
        genomic_start=allele.start,
        genomic_end=allele.end,
        reference_allele=allele.deleted_sequence,
        alternate_allele=allele.inserted_sequence,
        normalized_hgvs="NC_000001.11:g.101A>G",
        hgvs_aliases=aliases or (HgvsAlias("NC_000001.11:g.101A>G", "genomic"),),
        gene_symbol="GENE1",
        transcript="NM_000001.1",
        transcript_hgvs="NM_000001.1:c.1A>G",
        provider_provenance=provenance
        or (ProviderProvenance("local", None, None, None, "bundle-1", None, "query"),),
    )


def test_canonical_allele_uses_zero_based_interbase_coordinates() -> None:
    fixture = json.loads((_FIXTURES / "canonical_equivalents.json").read_text())[
        "substitution"
    ]

    allele = CanonicalAlleleKey(
        assembly=GenomeBuild(fixture["assembly"]),
        sequence_accession=fixture["accession"],
        start=fixture["start"],
        end=fixture["end"],
        deleted_sequence=fixture["deleted"],
        inserted_sequence=fixture["inserted"],
    )

    assert allele.value == "cak1:GRCh38:NC_000001.11:100:A>G"


def test_insertion_and_deletion_use_allele_length_coordinate_invariants() -> None:
    insertion = _allele(start=101, end=101, deleted="", inserted="T")
    deletion = _allele(start=101, end=103, deleted="AT", inserted="")

    assert insertion.value.endswith(":101:>T")
    assert deletion.value.endswith(":101:AT>")


@pytest.mark.parametrize(
    ("start", "end", "deleted", "inserted"),
    [
        (-1, 0, "", "A"),
        (2, 1, "", "A"),
        (1, 2, "", "A"),
        (1, 1, "A", "G"),
        (1, 2, "a", "G"),
        (1, 2, "A", "R"),
        (1, 1, "", ""),
    ],
)
def test_canonical_allele_rejects_invalid_coordinates_and_sequences(
    start: int, end: int, deleted: str, inserted: str
) -> None:
    with pytest.raises(ValueError):
        _allele(start=start, end=end, deleted=deleted, inserted=inserted)


def test_equivalent_deletion_representations_share_canonical_key() -> None:
    fixture = json.loads((_FIXTURES / "canonical_equivalents.json").read_text())[
        "deletion_equivalents"
    ]

    keys = {
        CanonicalAlleleKey(
            assembly=GenomeBuild.GRCH38,
            sequence_accession="NC_000001.11",
            start=item["start"],
            end=item["end"],
            deleted_sequence=item["deleted"],
            inserted_sequence=item["inserted"],
        )
        for item in fixture
    }

    assert len(keys) == 1


def test_normalized_variant_hash_is_stable_under_alias_and_provenance_ordering() -> (
    None
):
    aliases = (
        HgvsAlias("NM_000001.1:c.1A>G", "transcript"),
        HgvsAlias("NC_000001.11:g.101A>G", "genomic"),
    )
    provenance = (
        ProviderProvenance("remote", "1", "id-2", None, None, None, "query"),
        ProviderProvenance("local", "1", "id-1", None, "bundle-1", None, "query"),
    )

    first = _normalized(aliases=aliases, provenance=provenance)
    second = _normalized(
        aliases=tuple(reversed(aliases)), provenance=tuple(reversed(provenance))
    )

    assert first == second
    assert canonical_hash(first.to_canonical_content()) == canonical_hash(
        second.to_canonical_content()
    )


def test_normalized_variant_exposes_the_evidence_query_view() -> None:
    variant = _normalized(
        aliases=(
            HgvsAlias("NM_000001.1:c.1A>G", "transcript"),
            HgvsAlias("NC_000001.11:g.101A>G", "genomic"),
        )
    )
    transcript_only = _normalized(
        aliases=(HgvsAlias("NM_000001.1:c.1A>G", "transcript"),)
    )

    assert variant.variant_key == "cak1:GRCh38:NC_000001.11:100:A>G"
    assert variant.genomic_hgvs == "NC_000001.11:g.101A>G"
    assert transcript_only.genomic_hgvs is None


def test_normalized_variant_round_trips_through_canonical_draft_content() -> None:
    original = _normalized(
        aliases=(
            HgvsAlias("NM_000001.1:c.1A>G", "transcript"),
            HgvsAlias("NC_000001.11:g.101A>G", "genomic"),
        ),
        provenance=(
            ProviderProvenance("remote", "1", "id-2", None, None, None, "query"),
            ProviderProvenance("local", "1", "id-1", None, "bundle-1", None, "query"),
        ),
    )

    restored = NormalizedVariant.from_canonical_content(original.to_canonical_content())

    assert restored == original


def test_normalized_variant_rejects_malformed_canonical_draft_content() -> None:
    with pytest.raises(ValueError, match="normalized variant content"):
        NormalizedVariant.from_canonical_content({"canonical_key": {}})


def test_normalized_variant_rejects_noncanonical_allele_duplicates() -> None:
    normalized = _normalized()

    with pytest.raises(ValueError):
        replace(normalized, genomic_start=99)
