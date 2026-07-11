from __future__ import annotations

from datetime import UTC, datetime

from acmg_classifier.domain.criteria import CriterionAssessment
from acmg_classifier.domain.enums import CriterionStatus, GenomeBuild
from acmg_classifier.domain.errors import JsonValue
from acmg_classifier.domain.evaluators.population import PopulationCriterionEvaluator
from acmg_classifier.domain.evidence import (
    EvidenceContextScope,
    EvidenceDerivation,
    EvidenceItem,
    FactSet,
    ObservationKind,
    PopulationObservation,
    SourceProvenance,
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
    *,
    ancestry: str,
    allele_frequency: float,
    allele_number: int = 1000,
    coverage: float = 30.0,
    homozygote_count: int = 0,
    hemizygote_count: int = 0,
    filter_status: str = "PASS",
) -> EvidenceItem:
    allele_count = round(allele_frequency * allele_number)
    identifier = f"{ancestry}-{allele_count}-{allele_number}-{coverage}"
    return EvidenceItem(
        variant_key="ga4gh:VA.population-example",
        kind=ObservationKind.POPULATION,
        observation=PopulationObservation(
            kind=ObservationKind.POPULATION,
            source_release="gnomad-r4.1",
            ancestry=ancestry,
            allele_count=allele_count,
            allele_number=allele_number,
            allele_frequency=allele_frequency,
            homozygote_count=homozygote_count,
            hemizygote_count=hemizygote_count,
            coverage=coverage,
            filter_status=filter_status,
        ),
        context_scope=EvidenceContextScope(genome_build=GenomeBuild.GRCH38),
        provenance=SourceProvenance(
            kind=EvidenceDerivation.SOURCE,
            source_id="gnomad",
            source_record_id=identifier,
            source_version="4.1",
            retrieved_at=datetime(2026, 7, 11, tzinfo=UTC),
            normalized_query_key="ga4gh:VA.population-example",
        ),
        raw_snapshot_ref="raw_" + "b" * 64,
        derivation=EvidenceDerivation.SOURCE,
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
            evaluator_id="population" if enabled else criterion_code.value,
            evaluator_version_constraint=">=1.0.0,<2.0.0",
            rationale_template=f"{criterion_code.value} rationale",
            parameters=parameters if enabled else {},
        )
        criteria.append(criterion)
        if enabled:
            selected = criterion
    assert selected is not None
    ruleset = RulesetSpecification(
        ruleset_id="population-test",
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
    return PopulationCriterionEvaluator(code).evaluate(
        FactSet.from_evidence(items), InterpretationContext(), specification, ruleset
    )


def test_ba1_uses_maximum_eligible_ancestry_frequency_at_boundary() -> None:
    global_population = _item(ancestry="global", allele_frequency=0.01)
    ancestry_population = _item(ancestry="afr", allele_frequency=0.05)

    assessment = _evaluate(
        CriterionCode.BA1,
        {
            "minimum_allele_number": 1000,
            "minimum_coverage": 20.0,
            "required_filter_status": "PASS",
            "minimum_allele_frequency": 0.05,
        },
        global_population,
        ancestry_population,
    )

    assert assessment.status is CriterionStatus.APPLIED
    assert assessment.evidence_ids == (ancestry_population.evidence_id,)
    assert assessment.applied_strength is CriterionStrength.STRONG
    assert assessment.comparisons[0].matched is True


def test_pm2_requires_adequate_coverage_before_zero_observation_can_apply() -> None:
    parameters: dict[str, JsonValue] = {
        "minimum_allele_number": 1000,
        "minimum_coverage": 20.0,
        "required_filter_status": "PASS",
        "maximum_allele_frequency": 0.0,
    }

    low_coverage = _evaluate(
        CriterionCode.PM2,
        parameters,
        _item(ancestry="global", allele_frequency=0.0, coverage=5.0),
    )
    adequate_coverage = _evaluate(
        CriterionCode.PM2,
        parameters,
        _item(ancestry="global", allele_frequency=0.0, coverage=20.0),
    )

    assert low_coverage.status is CriterionStatus.NOT_EVALUABLE
    assert adequate_coverage.status is CriterionStatus.APPLIED


def test_bs1_below_frequency_threshold_is_not_applied() -> None:
    assessment = _evaluate(
        CriterionCode.BS1,
        {
            "minimum_allele_number": 1000,
            "minimum_coverage": 20.0,
            "required_filter_status": "PASS",
            "minimum_allele_frequency": 0.02,
        },
        _item(ancestry="global", allele_frequency=0.019),
    )

    assert assessment.status is CriterionStatus.NOT_APPLIED
    assert assessment.evidence_ids == ()


def test_bs2_accepts_homozygous_or_hemizygous_boundary_counts() -> None:
    item = _item(
        ancestry="global",
        allele_frequency=0.002,
        homozygote_count=0,
        hemizygote_count=2,
    )

    assessment = _evaluate(
        CriterionCode.BS2,
        {
            "minimum_allele_number": 1000,
            "minimum_coverage": 20.0,
            "required_filter_status": "PASS",
            "minimum_homozygote_count": 1,
            "minimum_hemizygote_count": 2,
        },
        item,
    )

    assert assessment.status is CriterionStatus.APPLIED
    assert assessment.evidence_ids == (item.evidence_id,)


def test_population_spread_is_explicit_conflict_when_ruleset_declares_limit() -> None:
    assessment = _evaluate(
        CriterionCode.BA1,
        {
            "minimum_allele_number": 1000,
            "minimum_coverage": 20.0,
            "required_filter_status": "PASS",
            "minimum_allele_frequency": 0.05,
            "maximum_frequency_spread": 0.01,
        },
        _item(ancestry="global", allele_frequency=0.01),
        _item(ancestry="global", allele_frequency=0.05),
    )

    assert assessment.status is CriterionStatus.CONFLICTING
    assert len(assessment.evidence_ids) == 2


def test_missing_threshold_configuration_is_not_evaluable_not_a_universal_default() -> (
    None
):
    assessment = _evaluate(
        CriterionCode.PM2,
        {
            "minimum_allele_number": 1000,
            "minimum_coverage": 20.0,
            "required_filter_status": "PASS",
        },
        _item(ancestry="global", allele_frequency=0.0),
    )

    assert assessment.status is CriterionStatus.NOT_EVALUABLE
    assert "ruleset" in assessment.limitations[0]
