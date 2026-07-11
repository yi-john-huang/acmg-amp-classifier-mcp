from __future__ import annotations

import json
from dataclasses import replace
from pathlib import Path

import pytest

from acmg_classifier.application.normalization import VariantNormalizationService
from acmg_classifier.domain.enums import GenomeBuild
from acmg_classifier.domain.models import InterpretationContext
from acmg_classifier.domain.normalization import (
    CanonicalAlleleKey,
    HgvsAlias,
    NormalizationFailureCode,
    NormalizedVariant,
    ProviderProvenance,
)
from acmg_classifier.ports.normalization import (
    NormalizationFailure,
    NormalizationPolicy,
    NormalizationSuccess,
)

_FIXTURES = Path(__file__).parents[2] / "fixtures" / "normalization"


class FakeProvider:
    def __init__(self, provider_id: str, result: object) -> None:
        self.provider_id = provider_id
        self.result = result
        self.calls = 0
        self.capabilities = frozenset({"live", "offline"})

    def normalize(self, parsed, context, policy):
        self.calls += 1
        return self.result


def _variant(
    provider_id: str,
    *,
    deleted: str = "A",
    inserted: str = "G",
    start: int = 100,
    end: int = 101,
    alias: str = "NC_000001.11:g.101A>G",
    transcript: str | None = "NM_000001.1",
) -> NormalizedVariant:
    canonical_key = CanonicalAlleleKey(
        assembly=GenomeBuild.GRCH38,
        sequence_accession="NC_000001.11",
        start=start,
        end=end,
        deleted_sequence=deleted,
        inserted_sequence=inserted,
    )
    return NormalizedVariant(
        original_input="NC_000001.11:g.101A>G",
        parsed_input="NC_000001.11:g.101A>G",
        canonical_key=canonical_key,
        genome_build=GenomeBuild.GRCH38,
        genomic_accession="NC_000001.11",
        genomic_start=start,
        genomic_end=end,
        reference_allele=deleted,
        alternate_allele=inserted,
        normalized_hgvs=alias,
        hgvs_aliases=(HgvsAlias(alias, "genomic", "NC_000001.11", provider_id),),
        gene_symbol="GENE1",
        transcript=transcript,
        transcript_hgvs="NM_000001.1:c.1A>G" if transcript else None,
        provider_provenance=(
            ProviderProvenance(provider_id, "1", None, None, None, None, "query"),
        ),
    )


def _success(provider_id: str, **kwargs: object) -> NormalizationSuccess:
    return NormalizationSuccess(_variant(provider_id, **kwargs))


def test_substitution_result_has_canonical_coordinates_and_input_alias() -> None:
    provider = FakeProvider("local", _success("local"))
    service = VariantNormalizationService(providers=(provider,))

    result = service.normalize("NC_000001.11:g.101A>G")

    assert isinstance(result, NormalizationSuccess)
    assert result.normalized.canonical_key.value == "cak1:GRCh38:NC_000001.11:100:A>G"
    assert {alias.expression for alias in result.normalized.hgvs_aliases} == {
        "NC_000001.11:g.101A>G"
    }


def test_duplication_normalizes_to_insertion_key_but_preserves_dup_alias() -> None:
    provider = FakeProvider(
        "local",
        _success(
            "local",
            deleted="",
            inserted="C",
            start=100,
            end=100,
            alias="NM_007294.4:c.5266dupC",
            transcript="NM_007294.4",
        ),
    )
    service = VariantNormalizationService(providers=(provider,))

    result = service.normalize("NM_007294.4:c.5266dupC")

    assert isinstance(result, NormalizationSuccess)
    assert result.normalized.canonical_key.deleted_sequence == ""
    assert result.normalized.canonical_key.inserted_sequence == "C"
    assert any("dupC" in alias.expression for alias in result.normalized.hgvs_aliases)


def test_reference_mismatch_fails_closed_before_evidence_query() -> None:
    fixture = json.loads((_FIXTURES / "reference_mismatch.json").read_text())
    provider = FakeProvider(
        fixture["provider_id"],
        NormalizationFailure(
            code=NormalizationFailureCode.REFERENCE_MISMATCH,
            message="supplied reference does not match resolved reference",
            retryable=False,
            provider_id=fixture["provider_id"],
            details=fixture["details"],
        ),
    )
    service = VariantNormalizationService(providers=(provider,))

    result = service.normalize(fixture["input"])

    assert isinstance(result, NormalizationFailure)
    assert result.code is NormalizationFailureCode.REFERENCE_MISMATCH
    assert provider.calls == 1


def test_round_trip_mismatch_is_terminal() -> None:
    variant = _variant("local")
    provider = FakeProvider(
        "local",
        NormalizationSuccess(
            variant,
            round_trip_key=replace(variant.canonical_key, inserted_sequence="T"),
        ),
    )

    result = VariantNormalizationService(providers=(provider,)).normalize(
        "NC_000001.11:g.101A>G"
    )

    assert isinstance(result, NormalizationFailure)
    assert result.code is NormalizationFailureCode.ROUND_TRIP_MISMATCH


def test_provider_order_does_not_change_normalized_variant_content() -> None:
    local = FakeProvider("local", _success("local", alias="NC_000001.11:g.101A>G"))
    remote = FakeProvider("remote", _success("remote", alias="NM_000001.1:c.1A>G"))

    first = VariantNormalizationService(providers=(local, remote)).normalize(
        "NC_000001.11:g.101A>G"
    )
    second = VariantNormalizationService(providers=(remote, local)).normalize(
        "NC_000001.11:g.101A>G"
    )

    assert isinstance(first, NormalizationSuccess)
    assert isinstance(second, NormalizationSuccess)
    assert first.normalized == second.normalized
    assert tuple(item.provider_id for item in first.normalized.provider_provenance) == (
        "local",
        "remote",
    )


def test_successful_providers_disagree_returns_normalization_conflict() -> None:
    fixture = json.loads((_FIXTURES / "provider_conflict.json").read_text())
    local = FakeProvider("local", _success("local"))
    remote = FakeProvider(
        "remote", _success("remote", inserted=fixture["remote"]["inserted"])
    )

    result = VariantNormalizationService(providers=(local, remote)).normalize(
        fixture["input"]
    )

    assert isinstance(result, NormalizationFailure)
    assert result.code.value == fixture["expected_code"]


def test_no_provider_success_returns_unavailable_with_diagnostics() -> None:
    local = FakeProvider(
        "local",
        NormalizationFailure(
            NormalizationFailureCode.ALLELE_NORMALIZATION_FAILED,
            "no local record",
            False,
            "local",
        ),
    )
    remote = FakeProvider(
        "remote",
        NormalizationFailure(
            NormalizationFailureCode.PROVIDER_TIMEOUT,
            "remote timed out",
            True,
            "remote",
        ),
    )

    result = VariantNormalizationService(providers=(local, remote)).normalize(
        "NC_000001.11:g.101A>G"
    )

    assert isinstance(result, NormalizationFailure)
    assert result.code is NormalizationFailureCode.NORMALIZATION_UNAVAILABLE
    assert result.details["failures"] == [
        {
            "code": "ALLELE_NORMALIZATION_FAILED",
            "provider_id": "local",
            "retryable": False,
        },
        {"code": "PROVIDER_TIMEOUT", "provider_id": "remote", "retryable": True},
    ]


def test_no_provider_returns_unavailable_without_fabricating_normalization() -> None:
    result = VariantNormalizationService().normalize("NC_000001.11:g.101A>G")

    assert isinstance(result, NormalizationFailure)
    assert result.code is NormalizationFailureCode.NORMALIZATION_UNAVAILABLE
    assert result.details == {"failures": []}


def test_unsupported_parse_never_calls_provider() -> None:
    provider = FakeProvider("local", _success("local"))

    result = VariantNormalizationService(providers=(provider,)).normalize("p.Arg273His")

    assert isinstance(result, NormalizationFailure)
    assert result.code is NormalizationFailureCode.UNSUPPORTED_VARIANT_SCOPE
    assert provider.calls == 0


@pytest.mark.asyncio
@pytest.mark.parametrize(
    ("value", "expected_code"),
    [
        ("NM_000492.3:c.1A>GG", NormalizationFailureCode.INVALID_VARIANT_SYNTAX),
        ("p.Arg273His", NormalizationFailureCode.UNSUPPORTED_VARIANT_SCOPE),
    ],
)
async def test_async_rejected_parse_never_calls_provider(
    value: str, expected_code: NormalizationFailureCode
) -> None:
    provider = FakeProvider("local", _success("local"))
    service = VariantNormalizationService(providers=(provider,))

    result = await service.normalize_async(value)
    expected = service.normalize(value)

    assert isinstance(result, NormalizationFailure)
    assert result == expected
    assert result.code is expected_code
    assert provider.calls == 0


def test_policy_skips_live_only_provider_offline() -> None:
    provider = FakeProvider("remote", _success("remote"))
    provider.capabilities = frozenset({"live"})

    result = VariantNormalizationService(providers=(provider,)).normalize(
        "NC_000001.11:g.101A>G",
        context=InterpretationContext(),
        policy=NormalizationPolicy(mode="offline", allow_remote=False),
    )

    assert isinstance(result, NormalizationFailure)
    assert result.code is NormalizationFailureCode.NORMALIZATION_UNAVAILABLE
    assert provider.calls == 0
