from __future__ import annotations

from datetime import UTC, datetime

import pytest

from acmg_classifier.domain.criteria import (
    CriteriaEngine,
    CriterionAssessment,
    EvaluatorRegistry,
    EvaluatorRegistryError,
)
from acmg_classifier.domain.enums import CriterionStatus, GenomeBuild
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


class _StaticEvaluator:
    evaluator_id = "test_evaluator"
    evaluator_version = "1.0.0"

    def __init__(self, code: CriterionCode) -> None:
        self.code = code

    def evaluate(
        self,
        facts: FactSet,
        context: InterpretationContext,
        specification: CriterionSpecification,
        ruleset: RulesetSpecification,
    ) -> CriterionAssessment:
        del facts, context
        return CriterionAssessment(
            code=specification.code,
            status=CriterionStatus.NOT_EVALUABLE,
            original_strength=specification.base_strength,
            applied_strength=None,
            evidence_ids=(),
            comparisons=(),
            rationale_template=specification.rationale_template,
            rationale_values={},
            limitations=("test evaluator deliberately has no facts",),
            ruleset_id=ruleset.ruleset_id,
            ruleset_version=ruleset.version,
            evaluator_id=self.evaluator_id,
            evaluator_version=self.evaluator_version,
        )


def _criterion(code: CriterionCode) -> CriterionSpecification:
    enabled = code is CriterionCode.PVS1
    return CriterionSpecification(
        code=code,
        enabled=enabled,
        base_strength=CriterionStrength.VERY_STRONG if enabled else None,
        allowed_strengths=(CriterionStrength.VERY_STRONG,) if enabled else (),
        evaluator_id="test_evaluator",
        evaluator_version_constraint=">=1.0.0,<2.0.0",
        rationale_template=f"{code.value} rationale",
    )


def _ruleset() -> RulesetSpecification:
    return RulesetSpecification(
        ruleset_id="acmg-general",
        version="1.0.0",
        state=RulesetState.APPROVED,
        publication_reference="PMID:25741868",
        scope=RulesetScope(),
        criteria=tuple(_criterion(code) for code in CriterionCode),
        combination_algorithm_id="acmg_2015",
        combination_algorithm_version="1.0.0",
    )


def _population_item(*, allele_count: int) -> EvidenceItem:
    return EvidenceItem(
        variant_key=f"ga4gh:VA.example:{allele_count}",
        kind=ObservationKind.POPULATION,
        observation=PopulationObservation(
            kind=ObservationKind.POPULATION,
            source_release="gnomad-r4.1",
            ancestry="global",
            allele_count=allele_count,
            allele_number=1000,
            allele_frequency=allele_count / 1000,
            homozygote_count=0,
            hemizygote_count=0,
            coverage=30.0,
            filter_status="PASS",
        ),
        context_scope=EvidenceContextScope(genome_build=GenomeBuild.GRCH38),
        provenance=SourceProvenance(
            kind=EvidenceDerivation.SOURCE,
            source_id="gnomad",
            source_record_id=f"1-100-A-G:{allele_count}",
            source_version="4.1",
            retrieved_at=datetime(2026, 7, 11, tzinfo=UTC),
            normalized_query_key=f"ga4gh:VA.example:{allele_count}",
        ),
        raw_snapshot_ref="raw_" + "a" * 64,
        derivation=EvidenceDerivation.SOURCE,
    )


def test_fact_set_indexes_items_deterministically_and_keeps_conflicts_explicit() -> (
    None
):
    first = _population_item(allele_count=1)
    second = _population_item(allele_count=2)

    facts = FactSet.from_evidence((second, first))

    assert facts.evidence_items == tuple(
        sorted((first, second), key=lambda item: item.evidence_id or "")
    )
    assert {observation.allele_count for observation in facts.population} == {1, 2}
    assert facts.evidence_ids_for(facts.population[0])


def test_fact_set_rejects_duplicate_evidence_identifiers() -> None:
    item = _population_item(allele_count=1)

    with pytest.raises(ValueError, match="duplicate evidence_id"):
        FactSet.from_evidence((item, item))


def test_evaluator_registry_requires_unique_codes_and_can_prove_completeness() -> None:
    duplicate = (_StaticEvaluator(CriterionCode.PVS1),) * 2
    with pytest.raises(EvaluatorRegistryError, match="duplicate evaluator"):
        EvaluatorRegistry(duplicate)

    partial = EvaluatorRegistry((_StaticEvaluator(CriterionCode.PVS1),))
    with pytest.raises(EvaluatorRegistryError, match="missing evaluators"):
        partial.assert_complete()

    complete = EvaluatorRegistry(
        tuple(_StaticEvaluator(code) for code in CriterionCode)
    )
    complete.assert_complete()
    assert complete.versions["test_evaluator"] == "1.0.0"


def test_registry_fails_closed_on_enabled_evaluator_version_mismatch() -> None:
    evaluator = _StaticEvaluator(CriterionCode.PVS1)
    evaluator.evaluator_version = "2.0.0"

    with pytest.raises(EvaluatorRegistryError, match="incompatible evaluator"):
        EvaluatorRegistry((evaluator,)).assert_compatible(_ruleset())


def test_engine_returns_code_keyed_assessments_and_explicit_disabled_states() -> None:
    ruleset = _ruleset()
    registry = EvaluatorRegistry((_StaticEvaluator(CriterionCode.PVS1),))

    assessments = CriteriaEngine(registry).evaluate(
        FactSet(), InterpretationContext(), ruleset
    )

    assert tuple(assessments) == tuple(
        sorted(CriterionCode, key=lambda code: code.value)
    )
    assert assessments[CriterionCode.PVS1].status is CriterionStatus.NOT_EVALUABLE
    assert assessments[CriterionCode.PS1].status is CriterionStatus.DISABLED
    assert assessments[CriterionCode.PS1].evaluator_id is None
