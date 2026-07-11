from __future__ import annotations

from datetime import UTC, datetime

from acmg_classifier.domain.enums import CriterionStatus, GenomeBuild
from acmg_classifier.domain.errors import JsonValue
from acmg_classifier.domain.evaluators.registry import build_default_evaluator_registry
from acmg_classifier.domain.evaluators.remaining import RemainingCriterionEvaluator
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
        variant_key="ga4gh:VA.remaining-example",
        kind=kind,
        observation=observation,
        context_scope=EvidenceContextScope(genome_build=GenomeBuild.GRCH38),
        provenance=SourceProvenance(
            kind=EvidenceDerivation.SOURCE,
            source_id="remaining_source",
            source_record_id=identifier,
            source_version="1.0",
            retrieved_at=datetime(2026, 7, 11, tzinfo=UTC),
            normalized_query_key="ga4gh:VA.remaining-example",
        ),
        raw_snapshot_ref="raw_" + "e" * 64,
        derivation=EvidenceDerivation.SOURCE,
    )


def _ruleset(
    code: CriterionCode,
    parameters: dict[str, JsonValue],
    *,
    enabled: bool = True,
) -> tuple[RulesetSpecification, CriterionSpecification]:
    criteria: list[CriterionSpecification] = []
    selected: CriterionSpecification | None = None
    for criterion_code in CriterionCode:
        selected_code = criterion_code is code
        criterion_enabled = selected_code and enabled
        criterion = CriterionSpecification(
            code=criterion_code,
            enabled=criterion_enabled,
            base_strength=CriterionStrength.SUPPORTING if criterion_enabled else None,
            allowed_strengths=(CriterionStrength.SUPPORTING,)
            if criterion_enabled
            else (),
            evaluator_id="remaining" if selected_code else criterion_code.value,
            evaluator_version_constraint=">=1.0.0,<2.0.0",
            rationale_template=f"{criterion_code.value} rationale",
            parameters=parameters if selected_code else {},
        )
        criteria.append(criterion)
        if selected_code:
            selected = criterion
    assert selected is not None
    ruleset = RulesetSpecification(
        ruleset_id="remaining-test",
        version="1.0.0",
        state=RulesetState.APPROVED,
        publication_reference="PMID:25741868",
        scope=RulesetScope(),
        criteria=tuple(criteria),
        combination_algorithm_id="acmg_2015",
        combination_algorithm_version="1.0.0",
    )
    return ruleset, selected


def test_default_registry_has_exactly_one_evaluator_for_all_28_criteria() -> None:
    registry = build_default_evaluator_registry()

    registry.assert_complete()
    assert {evaluator.code for evaluator in registry.evaluators} == set(CriterionCode)
    assert len(registry.evaluators) == 28


def test_pp2_requires_structured_missense_mechanism_and_constraint_facts() -> None:
    consequence = _item(
        ConsequenceObservation(
            kind=ObservationKind.CONSEQUENCE,
            transcript="NM_000492.4",
            consequence="missense",
            protein_change="p.Arg1Gly",
        ),
        ObservationKind.CONSEQUENCE,
        "consequence",
    )
    mechanism = _item(
        GeneMechanismObservation(
            kind=ObservationKind.GENE_MECHANISM,
            gene_id="HGNC:1884",
            disease_id="MONDO:0009061",
            mechanism="gain_of_function",
            validity="strong",
            missense_mechanism="established",
            benign_missense_rate="low",
        ),
        ObservationKind.GENE_MECHANISM,
        "mechanism",
    )
    ruleset, specification = _ruleset(
        CriterionCode.PP2,
        {
            "required_consequence": "missense",
            "required_gene_mechanism": "gain_of_function",
            "minimum_gene_validity": "strong",
            "require_established_missense_mechanism": True,
            "require_low_benign_missense_rate": True,
        },
    )

    assessment = RemainingCriterionEvaluator(CriterionCode.PP2).evaluate(
        FactSet.from_evidence((consequence, mechanism)),
        InterpretationContext(disease_id="MONDO:0009061"),
        specification,
        ruleset,
    )

    assert assessment.status is CriterionStatus.APPLIED
    assert set(assessment.evidence_ids) == {
        consequence.evidence_id,
        mechanism.evidence_id,
    }


def test_pp5_and_bp6_are_explicitly_disabled_or_not_evaluable_never_applied() -> None:
    for code in (CriterionCode.PP5, CriterionCode.BP6):
        ruleset, specification = _ruleset(code, {}, enabled=False)
        disabled = RemainingCriterionEvaluator(code).evaluate(
            FactSet(), InterpretationContext(), specification, ruleset
        )
        enabled_ruleset, enabled_specification = _ruleset(code, {}, enabled=True)
        attempted = RemainingCriterionEvaluator(code).evaluate(
            FactSet(), InterpretationContext(), enabled_specification, enabled_ruleset
        )

        assert disabled.status is CriterionStatus.DISABLED
        assert attempted.status is CriterionStatus.NOT_EVALUABLE
