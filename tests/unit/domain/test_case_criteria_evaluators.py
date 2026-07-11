from __future__ import annotations

from datetime import UTC, datetime
from typing import Literal

from acmg_classifier.domain.criteria import CriterionAssessment
from acmg_classifier.domain.enums import CriterionStatus, GenomeBuild
from acmg_classifier.domain.errors import JsonValue
from acmg_classifier.domain.evaluators.case import CaseCriterionEvaluator
from acmg_classifier.domain.evidence import (
    AllelicObservation,
    CaseControlObservation,
    DeNovoObservation,
    EvidenceContextScope,
    EvidenceDerivation,
    EvidenceItem,
    FactSet,
    Observation,
    ObservationKind,
    PhenotypeObservation,
    SegregationObservation,
    UserProvenance,
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


def _item(
    observation: Observation,
    kind: ObservationKind,
    identifier: str,
) -> EvidenceItem:
    return EvidenceItem(
        variant_key="ga4gh:VA.case-example",
        kind=kind,
        observation=observation,
        context_scope=EvidenceContextScope(genome_build=GenomeBuild.GRCH38),
        provenance=UserProvenance(
            kind=EvidenceDerivation.USER,
            actor_id="usr_researcher_001",
            submitted_at=datetime(2026, 7, 11, tzinfo=UTC),
            confirmation_method=identifier,
            citation_ids=("PMID:12345678",),
        ),
        derivation=EvidenceDerivation.USER,
    )


def _de_novo(
    confirmation: Literal["confirmed", "assumed", "unknown", "not_applicable"],
    occurrences: int = 1,
) -> EvidenceItem:
    return _item(
        DeNovoObservation(
            kind=ObservationKind.DE_NOVO,
            confirmation=confirmation,
            occurrence_count=occurrences,
            paternity_confirmed=confirmation == "confirmed",
            maternity_confirmed=confirmation == "confirmed",
            phenotype_consistent=True,
        ),
        ObservationKind.DE_NOVO,
        f"de-novo-{confirmation}-{occurrences}",
    )


def _segregation(*, lod_score: float, non_segregations: int = 0) -> EvidenceItem:
    return _item(
        SegregationObservation(
            kind=ObservationKind.SEGREGATION,
            family_count=2,
            informative_meioses=5,
            lod_score=lod_score,
            co_segregations=5,
            non_segregations=non_segregations,
            phenotype_defined=True,
        ),
        ObservationKind.SEGREGATION,
        f"segregation-{lod_score}-{non_segregations}",
    )


def _phenotype(
    term_id: str,
    specificity: Literal["highly_specific", "consistent", "nonspecific", "unknown"] = (
        "highly_specific"
    ),
) -> EvidenceItem:
    return _item(
        PhenotypeObservation(
            kind=ObservationKind.PHENOTYPE,
            term_id=term_id,
            state="present",
            specificity=specificity,
        ),
        ObservationKind.PHENOTYPE,
        f"phenotype-{term_id}",
    )


def _allelic(
    phase: Literal["cis", "trans", "unknown"],
    count: int = 1,
) -> EvidenceItem:
    return _item(
        AllelicObservation(
            kind=ObservationKind.ALLELIC,
            other_variant_key="ga4gh:VA.other-example",
            phase=phase,
            occurrence_count=count,
            observed_in_affected=True,
        ),
        ObservationKind.ALLELIC,
        f"allelic-{phase}-{count}",
    )


def _case_control() -> EvidenceItem:
    return _item(
        CaseControlObservation(
            kind=ObservationKind.CASE_CONTROL,
            study_id="PMID:23456789",
            case_count=100,
            control_count=1000,
            case_allele_count=10,
            control_allele_count=1,
            odds_ratio=5.0,
            p_value=0.001,
        ),
        ObservationKind.CASE_CONTROL,
        "case-control",
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
            evaluator_id="case" if enabled else criterion_code.value,
            evaluator_version_constraint=">=1.0.0,<2.0.0",
            rationale_template=f"{criterion_code.value} rationale",
            parameters=parameters if enabled else {},
        )
        criteria.append(criterion)
        if enabled:
            selected = criterion
    assert selected is not None
    ruleset = RulesetSpecification(
        ruleset_id="case-test",
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
    return CaseCriterionEvaluator(code).evaluate(
        FactSet.from_evidence(items), InterpretationContext(), specification, ruleset
    )


def test_ps2_and_pm6_require_distinct_confirmed_and_assumed_de_novo_states() -> None:
    confirmed = _de_novo("confirmed")
    assumed = _de_novo("assumed")

    ps2 = _evaluate(
        CriterionCode.PS2,
        {
            "required_confirmation": "confirmed",
            "minimum_occurrence_count": 1,
            "require_parentage_confirmation": True,
            "require_consistent_phenotype": True,
        },
        confirmed,
    )
    pm6 = _evaluate(
        CriterionCode.PM6,
        {
            "required_confirmation": "assumed",
            "minimum_occurrence_count": 1,
            "require_parentage_confirmation": False,
            "require_consistent_phenotype": True,
        },
        assumed,
    )

    assert ps2.status is CriterionStatus.APPLIED
    assert pm6.status is CriterionStatus.APPLIED


def test_allelic_phase_and_segregation_require_configured_quantitative_evidence() -> (
    None
):
    pm3 = _evaluate(
        CriterionCode.PM3,
        {
            "required_phase": "trans",
            "minimum_occurrence_count": 2,
            "require_affected": True,
        },
        _allelic("trans", 2),
    )
    pp1 = _evaluate(
        CriterionCode.PP1,
        {
            "minimum_lod_score": 3.0,
            "require_defined_phenotype": True,
        },
        _segregation(lod_score=3.0),
    )
    bs4 = _evaluate(
        CriterionCode.BS4,
        {
            "minimum_non_segregations": 1,
            "require_defined_phenotype": True,
        },
        _segregation(lod_score=0.0, non_segregations=1),
    )

    assert pm3.status is CriterionStatus.APPLIED
    assert pp1.status is CriterionStatus.APPLIED
    assert bs4.status is CriterionStatus.APPLIED


def test_pp4_bp2_and_ps4_require_typed_case_facts() -> None:
    pp4 = _evaluate(
        CriterionCode.PP4,
        {
            "required_specificity": "highly_specific",
            "minimum_present_terms": 2,
        },
        _phenotype("HP:0001250"),
        _phenotype("HP:0004322"),
    )
    bp2 = _evaluate(
        CriterionCode.BP2,
        {
            "required_phase": "cis",
            "minimum_occurrence_count": 1,
        },
        _allelic("cis"),
    )
    ps4 = _evaluate(
        CriterionCode.PS4,
        {
            "minimum_odds_ratio": 5.0,
            "maximum_p_value": 0.001,
            "minimum_case_count": 100,
            "minimum_control_count": 1000,
        },
        _case_control(),
    )

    assert pp4.status is CriterionStatus.APPLIED
    assert bp2.status is CriterionStatus.APPLIED
    assert ps4.status is CriterionStatus.APPLIED


def test_bp5_is_honestly_not_evaluable_until_structured_alternative_cause_exists() -> (
    None
):
    assessment = _evaluate(CriterionCode.BP5, {}, _phenotype("HP:0001250"))

    assert assessment.status is CriterionStatus.NOT_EVALUABLE
    assert "alternative molecular cause" in assessment.limitations[0]


def test_incomplete_case_facts_do_not_apply_criteria() -> None:
    assessment = _evaluate(
        CriterionCode.PS2,
        {
            "required_confirmation": "confirmed",
            "minimum_occurrence_count": 1,
            "require_parentage_confirmation": True,
            "require_consistent_phenotype": True,
        },
        _de_novo("assumed"),
    )

    assert assessment.status is CriterionStatus.NOT_APPLIED
