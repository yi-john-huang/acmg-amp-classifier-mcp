from __future__ import annotations

from acmg_classifier.application.normalization import VariantNormalizationService
from acmg_classifier.domain.enums import GenomeBuild
from acmg_classifier.domain.normalization import (
    CanonicalAlleleKey,
    HgvsAlias,
    NormalizedVariant,
    ProviderProvenance,
)
from acmg_classifier.ports.normalization import (
    NormalizationPolicy,
    NormalizationSuccess,
)


class ContractProvider:
    provider_id = "contract-provider"
    capabilities = frozenset({"live", "offline"})

    def __init__(self) -> None:
        self.query = None

    def normalize(self, parsed, context, policy):
        self.query = (parsed, context, policy)
        key = CanonicalAlleleKey(
            assembly=GenomeBuild.GRCH38,
            sequence_accession="NC_000001.11",
            start=100,
            end=101,
            deleted_sequence="A",
            inserted_sequence="G",
        )
        return NormalizationSuccess(
            NormalizedVariant(
                original_input=parsed.original_input,
                parsed_input=parsed.normalized_input,
                canonical_key=key,
                genome_build=key.assembly,
                genomic_accession=key.sequence_accession,
                genomic_start=key.start,
                genomic_end=key.end,
                reference_allele=key.deleted_sequence,
                alternate_allele=key.inserted_sequence,
                normalized_hgvs="NC_000001.11:g.101A>G",
                hgvs_aliases=(HgvsAlias("NC_000001.11:g.101A>G", "genomic"),),
                gene_symbol=None,
                transcript=None,
                transcript_hgvs=None,
                provider_provenance=(
                    ProviderProvenance(
                        self.provider_id,
                        "1.0",
                        "synthetic-1",
                        None,
                        None,
                        "fixture:normalization",
                        parsed.normalized_input,
                    ),
                ),
            ),
            round_trip_key=key,
        )


def test_normalization_provider_contract_receives_pure_parse_context_and_policy() -> (
    None
):
    provider = ContractProvider()
    policy = NormalizationPolicy(mode="offline", allow_remote=False)

    result = VariantNormalizationService(providers=(provider,)).normalize(
        "NC_000001.11:g.101A>G", policy=policy
    )

    assert isinstance(result, NormalizationSuccess)
    parsed, context, received_policy = provider.query
    assert parsed.normalized_input == "NC_000001.11:g.101A>G"
    assert context.genome_build is None
    assert received_policy is policy
    assert (
        result.normalized.provider_provenance[0].raw_snapshot_ref
        == "fixture:normalization"
    )
