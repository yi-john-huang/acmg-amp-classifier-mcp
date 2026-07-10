from __future__ import annotations

from dataclasses import replace

import pytest
from pydantic import ValidationError

from acmg_classifier.domain.enums import GenomeBuild, InheritanceMode
from acmg_classifier.domain.models import InterpretationContext
from acmg_classifier.domain.rules import (
    CriterionCode,
    CriterionSpecification,
    CriterionStrength,
    RejectionReason,
    RulesetRegistry,
    RulesetScope,
    RulesetSelectionStatus,
    RulesetSpecification,
    RulesetState,
)


def _criterion(code: CriterionCode) -> CriterionSpecification:
    enabled = code is CriterionCode.PVS1
    return CriterionSpecification(
        code=code,
        enabled=enabled,
        base_strength=CriterionStrength.VERY_STRONG if enabled else None,
        allowed_strengths=(CriterionStrength.VERY_STRONG,) if enabled else (),
        evaluator_id=code.value,
        evaluator_version_constraint=">=1.0.0,<2.0.0",
        rationale_template=f"{code.value} rationale",
    )


def _ruleset(
    ruleset_id: str,
    version: str,
    *,
    state: RulesetState = RulesetState.APPROVED,
    scope: RulesetScope | None = None,
) -> RulesetSpecification:
    return RulesetSpecification(
        ruleset_id=ruleset_id,
        version=version,
        state=state,
        publication_reference="PMID:25741868",
        scope=scope or RulesetScope(),
        criteria=tuple(_criterion(code) for code in CriterionCode),
        combination_algorithm_id="acmg_2015",
        combination_algorithm_version="1.0.0",
    )


def _evaluator_versions(version: str = "1.0.0") -> dict[str, str]:
    return {code.value: version for code in CriterionCode}


def test_ruleset_requires_each_2015_code_once() -> None:
    ruleset = _ruleset("acmg-general", "1.0.0")

    assert len(CriterionCode) == 28
    assert {criterion.code for criterion in ruleset.criteria} == set(CriterionCode)

    with pytest.raises(ValidationError, match="every ACMG/AMP criterion"):
        RulesetSpecification(
            **ruleset.model_dump(exclude={"criteria"}),
            criteria=ruleset.criteria[:-1],
        )


def test_ruleset_rejects_duplicate_code_and_invalid_semantic_version() -> None:
    ruleset = _ruleset("acmg-general", "1.0.0")
    duplicate = (*ruleset.criteria, ruleset.criteria[0])

    with pytest.raises(ValidationError, match="exactly once"):
        RulesetSpecification(
            **ruleset.model_dump(exclude={"criteria"}), criteria=duplicate
        )
    with pytest.raises(ValidationError, match="semantic version"):
        _ruleset("acmg-general", "2026.7")


def test_selection_prefers_explicit_approved_ruleset_and_records_rejections() -> None:
    general = _ruleset("acmg-general", "1.0.0")
    disease = _ruleset(
        "cftr-cystic-fibrosis",
        "1.0.0",
        scope=RulesetScope(
            gene_symbol="CFTR",
            disease_id="MONDO:0009061",
            inheritance=InheritanceMode.AUTOSOMAL_RECESSIVE,
            genome_build=GenomeBuild.GRCH38,
        ),
    )
    selection = RulesetRegistry((general, disease)).select(
        InterpretationContext(
            genome_build=GenomeBuild.GRCH38,
            disease_id="MONDO:0009061",
            inheritance=InheritanceMode.AUTOSOMAL_RECESSIVE,
        ),
        gene_symbol="CFTR",
        requested_ruleset_id="acmg-general",
        evaluator_versions=_evaluator_versions(),
    )

    assert selection.status is RulesetSelectionStatus.SELECTED
    assert selection.ruleset == general
    assert any(
        rejection.ruleset_id == disease.ruleset_id
        and rejection.reason is RejectionReason.NOT_REQUESTED
        for rejection in selection.rejected
    )


def test_selection_prefers_highest_semantic_version_of_exact_scope() -> None:
    general = _ruleset("acmg-general", "1.0.0")
    lower = _ruleset(
        "cftr-cystic-fibrosis",
        "1.2.0",
        scope=RulesetScope(gene_symbol="CFTR", disease_id="MONDO:0009061"),
    )
    higher = _ruleset(
        "cftr-cystic-fibrosis",
        "1.10.0",
        scope=RulesetScope(gene_symbol="CFTR", disease_id="MONDO:0009061"),
    )

    selection = RulesetRegistry((general, lower, higher)).select(
        InterpretationContext(disease_id="MONDO:0009061"),
        gene_symbol="CFTR",
        evaluator_versions=_evaluator_versions(),
    )

    assert selection.status is RulesetSelectionStatus.SELECTED
    assert selection.ruleset == higher
    assert any(
        rejection.ruleset_id == lower.ruleset_id
        and rejection.version == lower.version
        and rejection.reason is RejectionReason.LOWER_PRECEDENCE
        for rejection in selection.rejected
    )


def test_ambiguous_gene_disease_rulesets_require_disease_context() -> None:
    first = _ruleset(
        "cftr-cystic-fibrosis",
        "1.0.0",
        scope=RulesetScope(gene_symbol="CFTR", disease_id="MONDO:0009061"),
    )
    second = _ruleset(
        "cftr-related-disorder",
        "1.0.0",
        scope=RulesetScope(gene_symbol="CFTR", disease_id="MONDO:0020137"),
    )

    selection = RulesetRegistry((first, second)).select(
        InterpretationContext(),
        gene_symbol="CFTR",
        evaluator_versions=_evaluator_versions(),
    )

    assert selection.status is RulesetSelectionStatus.NEEDS_CONTEXT
    assert selection.ruleset is None
    assert selection.required_context_fields == ("disease_id",)


def test_draft_retired_and_incompatible_evaluators_are_rejected() -> None:
    draft = _ruleset("draft", "1.0.0", state=RulesetState.DRAFT)
    retired = _ruleset("retired", "1.0.0", state=RulesetState.RETIRED)
    approved = _ruleset("approved", "1.0.0")

    selection = RulesetRegistry((draft, retired, approved)).select(
        InterpretationContext(),
        evaluator_versions=_evaluator_versions("2.0.0"),
    )

    assert selection.status is RulesetSelectionStatus.NO_COMPATIBLE_RULESET
    assert selection.ruleset is None
    assert {rejection.reason for rejection in selection.rejected} == {
        RejectionReason.NOT_APPROVED,
        RejectionReason.EVALUATOR_INCOMPATIBLE,
    }


def test_registry_is_immutable_and_rejects_duplicate_identity() -> None:
    first = _ruleset("acmg-general", "1.0.0")

    with pytest.raises(ValueError, match="duplicate ruleset"):
        RulesetRegistry((first, first))

    registry = RulesetRegistry((first,))
    assert registry.rulesets == (first,)
    with pytest.raises((AttributeError, TypeError)):
        registry.rulesets += (replace(first, version="1.0.1"),)  # type: ignore[misc]
