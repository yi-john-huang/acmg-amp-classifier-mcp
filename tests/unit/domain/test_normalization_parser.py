from __future__ import annotations

import pytest


def _parser():
    from acmg_classifier.domain.normalization import VariantInputParser

    return VariantInputParser()


@pytest.mark.parametrize(
    ("value", "kind", "normalized"),
    [
        ("NM_000492.4:c.1521a>g", "substitution", "NM_000492.4:c.1521A>G"),
        (
            "NM_000492.4:c.1521_1522insat",
            "insertion",
            "NM_000492.4:c.1521_1522insAT",
        ),
        ("NM_007294.4:c.5266dupC", "duplication", "NM_007294.4:c.5266dupC"),
        (
            "NM_000492.4:c.1521_1523delctt",
            "deletion",
            "NM_000492.4:c.1521_1523delCTT",
        ),
        (
            "ENST00000357654.9:c.5266dupCfs*2",
            "frameshift",
            "ENST00000357654.9:c.5266dupCfs*2",
        ),
    ],
)
def test_parses_transcript_substitution_and_small_indels(
    value: str, kind: str, normalized: str
) -> None:
    parser = _parser()

    parsed = parser.parse(value)

    assert parsed.kind == kind
    assert parsed.notation == "transcript_hgvs"
    assert parsed.normalized_input == normalized


def test_parses_genomic_hgvs_without_claiming_a_build() -> None:
    parsed = _parser().parse("NC_000007.14:g.117199644A>G")

    assert parsed.notation == "genomic_hgvs"
    assert parsed.accession == "NC_000007.14"
    assert parsed.kind == "substitution"
    assert parsed.genome_build is None


def test_parses_genomic_small_indel_without_dropping_sequence() -> None:
    parsed = _parser().parse("NC_000007.14:g.117199644_117199645insat")

    assert parsed.kind == "insertion"
    assert parsed.normalized_input.endswith("insAT")


@pytest.mark.parametrize(
    "value",
    [
        "NM_000492.3:c.1A>GG",
        "NM_000492.3:c.1AA>G",
        "NC_000007.14:g.117199644A>GG",
        "NC_000007.14:g.117199644AA>G",
    ],
)
def test_rejects_multi_base_substitution_notation(value: str) -> None:
    parsed = _parser().parse(value)

    assert parsed.scope == "invalid"
    assert parsed.reason == "malformed_syntax"


@pytest.mark.parametrize(
    "value",
    [
        "NM_000492.3:c.1delinsGG",
        "NC_000007.14:g.117199644delinsGG",
        "BRCA1:c.1delinsGG",
    ],
)
def test_delins_is_explicitly_out_of_scope(value: str) -> None:
    parsed = _parser().parse(value)

    assert parsed.scope == "unsupported"
    assert parsed.reason == "delins_not_supported"


def test_parses_gene_plus_variant_and_normalizes_gene_symbol() -> None:
    parsed = _parser().parse("brca1:c.5266dupC")

    assert parsed.notation == "gene_variant"
    assert parsed.gene_symbol == "BRCA1"
    assert parsed.requires_context


@pytest.mark.parametrize(
    "value",
    [
        "NP_000546.5:p.Arg273His",
        "TP53 p.Arg273His",
        "p.Arg273His",
    ],
)
def test_protein_only_inputs_are_explicitly_unsupported(value: str) -> None:
    parsed = _parser().parse(value)

    assert parsed.reason == "protein_only_ambiguous"
    assert parsed.scope == "unsupported"


@pytest.mark.parametrize(
    "value",
    [
        "chrM:g.100A>G",
        "NC_012920.1:g.100A>G",
        "chr1:g.100_200CNV",
        "sample-somatic:NM_000492.4:c.1A>G",
        "NM_000492.4:c.1_100000del",
    ],
)
def test_out_of_scope_inputs_never_become_supported_variants(value: str) -> None:
    parsed = _parser().parse(value)

    assert parsed.scope == "unsupported"
    assert parsed.reason in {
        "mitochondrial",
        "copy_number_or_structural",
        "somatic_or_mosaic",
    }


def test_malformed_input_is_typed_as_invalid() -> None:
    parsed = _parser().parse("not an HGVS variant")

    assert parsed.scope == "invalid"
    assert parsed.reason == "malformed_syntax"


def test_empty_input_is_typed_as_invalid() -> None:
    parsed = _parser().parse("   ")

    assert parsed.scope == "invalid"
    assert parsed.reason == "empty_input"
