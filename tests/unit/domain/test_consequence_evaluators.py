from __future__ import annotations

from datetime import UTC, datetime
from typing import Literal

from acmg_classifier.domain.criteria import CriterionAssessment
from acmg_classifier.domain.enums import CriterionStatus, GenomeBuild, InheritanceMode
from acmg_classifier.domain.errors import JsonValue
from acmg_classifier.domain.evaluators.consequence import ConsequenceCriterionEvaluator
from acmg_classifier.domain.evidence import (
    ConsequenceObservation,
    EvidenceContextScope,
    EvidenceDerivation,
    EvidenceItem,
    FactSet,
    GeneMechanismObservation,
    Observation,
    ObservationKind,
    SourceProvenance,
    VariantLocationObservation,
)
from acmg_classifier.domain.models import InterpretationContext
from acmg_classifier.domain.rules import (
    CriterionCode,
    CriterionSpecification,
    CriterionStrength,
    RulesetScope,
    RulesetSpecification,
    RulesetState,
)

_CONTEXT = InterpretationContext(
    genome_build=GenomeBuild.GRCH38,
    disease_id="MONDO:0009061",
    inheritance=InheritanceMode.AUTOSOMAL_RECESSIVE,
)


def _item(
    observation: Observation,
    kind: ObservationKind,
    identifier: str,
) -> EvidenceItem:
    return EvidenceItem(
        variant_key="ga4gh:VA.consequence-example",
        kind=kind,
        observation=observation,
        context_scope=EvidenceContextScope(
            genome_build=GenomeBuild.GRCH38,
            disease_id="MONDO:0009061",
            inheritance=InheritanceMode.AUTOSOMAL_RECESSIVE,
        ),
        provenance=SourceProvenance(
            kind=EvidenceDerivation.SOURCE,
            source_id="consequence_source",
            source_record_id=identifier,
            source_version="1.0",
            retrieved_at=datetime(2026, 7, 11, tzinfo=UTC),
            normalized_query_key="ga4gh:VA.consequence-example",
        ),
        raw_snapshot_ref="raw_" + "c" * 64,
        derivation=EvidenceDerivation.SOURCE,
    )


def _consequence(
    consequence: Literal[
        "nonsense",
        "frameshift",
        "canonical_splice",
        "missense",
        "inframe_indel",
        "synonymous",
        "splice_region",
        "start_lost",
        "stop_lost",
        "unknown",
    ],
    *,
    nmd_predicted: bool | None = None,
    same_amino_acid_change: bool | None = None,
    same_amino_acid_splice_difference: bool | None = None,
    same_residue_different_amino_acid: bool | None = None,
    inframe_length: int | None = None,
    splice_impact: Literal["none", "predicted", "confirmed", "unknown"] = "unknown",
) -> EvidenceItem:
    return _item(
        ConsequenceObservation(
            kind=ObservationKind.CONSEQUENCE,
            transcript="NM_000492.4",
            consequence=consequence,
            protein_change="p.Arg1Ter",
            nmd_predicted=nmd_predicted,
            same_amino_acid_change=same_amino_acid_change,
            same_amino_acid_splice_difference=same_amino_acid_splice_difference,
            same_residue_different_amino_acid=same_residue_different_amino_acid,
            inframe_length=inframe_length,
            splice_impact=splice_impact,
        ),
        ObservationKind.CONSEQUENCE,
        f"consequence-{consequence}-{nmd_predicted}-{inframe_length}",
    )


def _mechanism() -> EvidenceItem:
    return _item(
        GeneMechanismObservation(
            kind=ObservationKind.GENE_MECHANISM,
            gene_id="HGNC:1884",
            disease_id="MONDO:0009061",
            mechanism="loss_of_function",
            validity="definitive",
            inheritance=InheritanceMode.AUTOSOMAL_RECESSIVE,
        ),
        ObservationKind.GENE_MECHANISM,
        "mechanism",
    )


def _location(
    *,
    region_type: str = "critical_domain",
    is_critical: bool | None = True,
    distance_to_splice_site: int | None = None,
) -> EvidenceItem:
    return _item(
        VariantLocationObservation(
            kind=ObservationKind.VARIANT_LOCATION,
            region_type=region_type,
            region_id="region-1",
            is_critical=is_critical,
            distance_to_splice_site=distance_to_splice_site,
        ),
        ObservationKind.VARIANT_LOCATION,
        f"location-{region_type}-{distance_to_splice_site}",
    )


def _ruleset(
    code: CriterionCode,
    parameters: dict[str, JsonValue],
) -> tuple[RulesetSpecification, CriterionSpecification]:
    criteria: list[CriterionSpecification] = []
    selected: CriterionSpecification | None = None
    for criterion_code in CriterionCode:
        enabled = criterion_code is code
        criterion = CriterionSpecification(
            code=criterion_code,
            enabled=enabled,
            base_strength=CriterionStrength.STRONG if enabled else None,
            allowed_strengths=(CriterionStrength.STRONG,) if enabled else (),
            evaluator_id="consequence" if enabled else criterion_code.value,
            evaluator_version_constraint=">=1.0.0,<2.0.0",
            rationale_template=f"{criterion_code.value} rationale",
            parameters=parameters if enabled else {},
        )
        criteria.append(criterion)
        if enabled:
            selected = criterion
    assert selected is not None
    ruleset = RulesetSpecification(
        ruleset_id="consequence-test",
        version="1.0.0",
        state=RulesetState.APPROVED,
        publication_reference="PMID:25741868",
        scope=RulesetScope(),
        criteria=tuple(criteria),
        combination_algorithm_id="acmg_2015",
        combination_algorithm_version="1.0.0",
    )
    return ruleset, selected


def _evaluate(
    code: CriterionCode,
    parameters: dict[str, JsonValue],
    *items: EvidenceItem,
) -> CriterionAssessment:
    ruleset, specification = _ruleset(code, parameters)
    return ConsequenceCriterionEvaluator(code).evaluate(
        FactSet.from_evidence(items), _CONTEXT, specification, ruleset
    )


def test_pvs1_requires_consequence_mechanism_and_nmd_decision_data() -> None:
    consequence = _consequence("nonsense", nmd_predicted=True)
    parameters: dict[str, JsonValue] = {
        "allowed_consequences": ["nonsense", "frameshift", "canonical_splice"],
        "required_gene_mechanism": "loss_of_function",
        "minimum_gene_validity": "strong",
        "require_nmd": True,
    }

    applied = _evaluate(CriterionCode.PVS1, parameters, consequence, _mechanism())
    missing_mechanism = _evaluate(CriterionCode.PVS1, parameters, consequence)
    nmd_escape = _evaluate(
        CriterionCode.PVS1,
        parameters,
        _consequence("nonsense", nmd_predicted=False),
        _mechanism(),
    )

    assert applied.status is CriterionStatus.APPLIED
    assert set(applied.evidence_ids) == {
        consequence.evidence_id,
        _mechanism().evidence_id,
    }
    assert missing_mechanism.status is CriterionStatus.NOT_EVALUABLE
    assert nmd_escape.status is CriterionStatus.NOT_APPLIED


def test_ps1_requires_same_amino_acid_and_no_splice_difference() -> None:
    applied = _evaluate(
        CriterionCode.PS1,
        {"require_same_amino_acid_change": True},
        _consequence(
            "missense",
            same_amino_acid_change=True,
            same_amino_acid_splice_difference=False,
        ),
    )
    splice_difference = _evaluate(
        CriterionCode.PS1,
        {"require_same_amino_acid_change": True},
        _consequence(
            "missense",
            same_amino_acid_change=True,
            same_amino_acid_splice_difference=True,
        ),
    )

    assert applied.status is CriterionStatus.APPLIED
    assert splice_difference.status is CriterionStatus.NOT_APPLIED


def test_location_and_inframe_criteria_require_structured_fields() -> None:
    pm1 = _evaluate(
        CriterionCode.PM1,
        {"required_region_type": "critical_domain", "require_critical_region": True},
        _location(),
    )
    pm4 = _evaluate(
        CriterionCode.PM4,
        {"allowed_consequences": ["inframe_indel"], "minimum_inframe_length": 3},
        _consequence("inframe_indel", inframe_length=3),
    )
    bp3 = _evaluate(
        CriterionCode.BP3,
        {"required_region_type": "repeat_region", "minimum_inframe_length": 3},
        _consequence("inframe_indel", inframe_length=3),
        _location(region_type="repeat_region", is_critical=False),
    )

    assert pm1.status is CriterionStatus.APPLIED
    assert pm4.status is CriterionStatus.APPLIED
    assert bp3.status is CriterionStatus.APPLIED


def test_pm5_and_bp1_require_structured_same_residue_and_mechanism_facts() -> None:
    pm5 = _evaluate(
        CriterionCode.PM5,
        {"require_same_residue_different_amino_acid": True},
        _consequence(
            "missense",
            same_amino_acid_change=False,
            same_residue_different_amino_acid=True,
        ),
    )
    bp1 = _evaluate(
        CriterionCode.BP1,
        {
            "required_consequence": "missense",
            "required_gene_mechanism": "loss_of_function",
        },
        _consequence("missense"),
        _mechanism(),
    )

    assert pm5.status is CriterionStatus.APPLIED
    assert bp1.status is CriterionStatus.APPLIED


def test_bp7_requires_synonymous_consequence_and_splice_distance() -> None:
    applied = _evaluate(
        CriterionCode.BP7,
        {"required_consequence": "synonymous", "minimum_splice_distance": 20},
        _consequence("synonymous", splice_impact="none"),
        _location(distance_to_splice_site=20),
    )
    splice_risk = _evaluate(
        CriterionCode.BP7,
        {"required_consequence": "synonymous", "minimum_splice_distance": 20},
        _consequence("synonymous", splice_impact="predicted"),
        _location(distance_to_splice_site=20),
    )

    assert applied.status is CriterionStatus.APPLIED
    assert splice_risk.status is CriterionStatus.NOT_APPLIED
