"""Pure, typed criterion-assessment and evaluator-registry contracts."""

from __future__ import annotations

from collections.abc import Mapping
from dataclasses import dataclass
from types import MappingProxyType
from typing import Literal, Protocol, Self

from pydantic import model_validator

from acmg_classifier.domain.enums import CriterionStatus
from acmg_classifier.domain.errors import JsonValue
from acmg_classifier.domain.evidence import (
    EvidenceId,
    EvidenceModel,
    FactSet,
    NonEmptyText,
)
from acmg_classifier.domain.models import InterpretationContext
from acmg_classifier.domain.rules import (
    CriterionCode,
    CriterionSpecification,
    CriterionStrength,
    RulesetSpecification,
    version_satisfies,
)

ComparisonOperator = Literal["<", "<=", "=", ">=", ">", "in", "matches"]


class EvaluatorRegistryError(ValueError):
    """Raised when evaluator availability cannot safely satisfy a ruleset."""


class CriteriaEngineError(ValueError):
    """Raised when an evaluator violates the immutable criterion contract."""


class CriterionComparison(EvidenceModel):
    """One auditable deterministic comparison used by a criterion evaluator."""

    input_name: NonEmptyText
    operator: ComparisonOperator
    observed: JsonValue
    expected: JsonValue
    matched: bool


class CriterionAssessment(EvidenceModel):
    """A complete machine-readable result for one criterion and ruleset version."""

    code: CriterionCode
    status: CriterionStatus
    original_strength: CriterionStrength | None
    applied_strength: CriterionStrength | None = None
    evidence_ids: tuple[EvidenceId, ...] = ()
    comparisons: tuple[CriterionComparison, ...] = ()
    rationale_template: NonEmptyText
    rationale_values: Mapping[str, JsonValue]
    limitations: tuple[NonEmptyText, ...] = ()
    ruleset_id: NonEmptyText
    ruleset_version: NonEmptyText
    evaluator_id: NonEmptyText | None
    evaluator_version: NonEmptyText | None

    @model_validator(mode="after")
    def validate_assessment(self) -> Self:
        evidence_ids = tuple(sorted(set(self.evidence_ids)))
        comparisons = tuple(
            sorted(
                self.comparisons,
                key=lambda comparison: (
                    comparison.input_name,
                    comparison.operator,
                    str(comparison.observed),
                    str(comparison.expected),
                ),
            )
        )
        if self.status is CriterionStatus.APPLIED:
            if self.original_strength is None or self.applied_strength is None:
                raise ValueError(
                    "applied criterion requires original and applied strengths"
                )
            if not evidence_ids:
                raise ValueError("applied criterion requires evidence_ids")
        elif self.applied_strength is not None:
            raise ValueError("non-applied criterion must not have an applied_strength")
        if self.status is CriterionStatus.DISABLED:
            if self.evaluator_id is not None or self.evaluator_version is not None:
                raise ValueError("disabled criterion must not claim an evaluator")
        elif self.evaluator_id is None or self.evaluator_version is None:
            raise ValueError("non-disabled criterion requires evaluator identity")
        object.__setattr__(self, "evidence_ids", evidence_ids)
        object.__setattr__(self, "comparisons", comparisons)
        object.__setattr__(
            self, "rationale_values", MappingProxyType(dict(self.rationale_values))
        )
        return self

    def to_canonical_content(self) -> dict[str, JsonValue]:
        """Return deterministic decision content without display-only prose."""
        return {
            "code": self.code.value,
            "status": self.status.value,
            "original_strength": (
                self.original_strength.value
                if self.original_strength is not None
                else None
            ),
            "applied_strength": (
                self.applied_strength.value
                if self.applied_strength is not None
                else None
            ),
            "evidence_ids": list(self.evidence_ids),
            "comparisons": [
                comparison.model_dump(mode="json") for comparison in self.comparisons
            ],
            "rationale_template": self.rationale_template,
            "rationale_values": dict(self.rationale_values),
            "limitations": list(self.limitations),
            "ruleset_id": self.ruleset_id,
            "ruleset_version": self.ruleset_version,
            "evaluator_id": self.evaluator_id,
            "evaluator_version": self.evaluator_version,
        }


class CriterionEvaluator(Protocol):
    """A pure evaluator selected by criterion code and specification version."""

    @property
    def code(self) -> CriterionCode: ...

    @property
    def evaluator_id(self) -> str: ...

    @property
    def evaluator_version(self) -> str: ...

    def evaluate(
        self,
        facts: FactSet,
        context: InterpretationContext,
        specification: CriterionSpecification,
        ruleset: RulesetSpecification,
    ) -> CriterionAssessment:
        """Evaluate one enabled criterion from typed facts and ruleset data."""


@dataclass(frozen=True, slots=True)
class EvaluatorRegistry:
    """Immutable code-keyed evaluator map with fail-closed compatibility checks."""

    evaluators: tuple[CriterionEvaluator, ...]

    def __post_init__(self) -> None:
        values = tuple(self.evaluators)
        by_code: dict[CriterionCode, CriterionEvaluator] = {}
        versions: dict[str, str] = {}
        for evaluator in values:
            if not isinstance(evaluator.code, CriterionCode):
                raise EvaluatorRegistryError("evaluator code must be a CriterionCode")
            if evaluator.code in by_code:
                raise EvaluatorRegistryError(
                    f"duplicate evaluator for criterion {evaluator.code.value}"
                )
            if not evaluator.evaluator_id:
                raise EvaluatorRegistryError("evaluator_id must not be empty")
            try:
                version_satisfies(evaluator.evaluator_version, "*")
            except ValueError as error:
                raise EvaluatorRegistryError(
                    "evaluator_version must be semantic"
                ) from error
            existing_version = versions.get(evaluator.evaluator_id)
            if (
                existing_version is not None
                and existing_version != evaluator.evaluator_version
            ):
                raise EvaluatorRegistryError(
                    "one evaluator_id cannot report conflicting evaluator versions"
                )
            by_code[evaluator.code] = evaluator
            versions[evaluator.evaluator_id] = evaluator.evaluator_version
        object.__setattr__(
            self, "evaluators", tuple(by_code[code] for code in sorted(by_code))
        )

    @property
    def versions(self) -> Mapping[str, str]:
        """Return an immutable evaluator implementation-to-version map."""
        versions = {
            evaluator.evaluator_id: evaluator.evaluator_version
            for evaluator in self.evaluators
        }
        return MappingProxyType(versions)

    def evaluator_for(self, code: CriterionCode) -> CriterionEvaluator | None:
        """Return a registered evaluator by code without any fallback behavior."""
        return next((value for value in self.evaluators if value.code is code), None)

    def assert_complete(self) -> None:
        """Fail closed unless every 2015 criterion has exactly one evaluator."""
        registered = {evaluator.code for evaluator in self.evaluators}
        missing = tuple(sorted(set(CriterionCode) - registered))
        if missing:
            rendered = ", ".join(code.value for code in missing)
            raise EvaluatorRegistryError(f"missing evaluators for {rendered}")

    def assert_compatible(self, ruleset: RulesetSpecification) -> None:
        """Fail closed if an enabled specification lacks a matching evaluator."""
        for specification in ruleset.criteria:
            if not specification.enabled:
                continue
            evaluator = self.evaluator_for(specification.code)
            if evaluator is None:
                raise EvaluatorRegistryError(
                    f"missing evaluator for enabled {specification.code.value}"
                )
            if evaluator.evaluator_id != specification.evaluator_id:
                raise EvaluatorRegistryError(
                    f"incompatible evaluator for {specification.code.value}"
                )
            if not version_satisfies(
                evaluator.evaluator_version,
                specification.evaluator_version_constraint,
            ):
                raise EvaluatorRegistryError(
                    f"incompatible evaluator version for {specification.code.value}"
                )


@dataclass(frozen=True, slots=True)
class CriteriaEngine:
    """Pure code-keyed evaluation over one selected immutable ruleset."""

    registry: EvaluatorRegistry

    def evaluate(
        self,
        facts: FactSet,
        context: InterpretationContext,
        ruleset: RulesetSpecification,
    ) -> dict[CriterionCode, CriterionAssessment]:
        """Evaluate every definition in stable code order without silent fallback."""
        self.registry.assert_compatible(ruleset)
        assessments: dict[CriterionCode, CriterionAssessment] = {}
        for specification in sorted(
            ruleset.criteria, key=lambda value: value.code.value
        ):
            if not specification.enabled:
                assessments[specification.code] = _disabled_assessment(
                    specification, ruleset
                )
                continue
            evaluator = self.registry.evaluator_for(specification.code)
            if evaluator is None:
                raise EvaluatorRegistryError(
                    f"missing evaluator for enabled {specification.code.value}"
                )
            assessment = evaluator.evaluate(facts, context, specification, ruleset)
            _validate_assessment(assessment, specification, ruleset, evaluator)
            assessments[specification.code] = assessment
        return assessments


def _disabled_assessment(
    specification: CriterionSpecification,
    ruleset: RulesetSpecification,
) -> CriterionAssessment:
    return CriterionAssessment(
        code=specification.code,
        status=CriterionStatus.DISABLED,
        original_strength=None,
        applied_strength=None,
        evidence_ids=(),
        comparisons=(),
        rationale_template=specification.rationale_template,
        rationale_values={},
        limitations=("disabled by selected ruleset",),
        ruleset_id=ruleset.ruleset_id,
        ruleset_version=ruleset.version,
        evaluator_id=None,
        evaluator_version=None,
    )


def _validate_assessment(
    assessment: CriterionAssessment,
    specification: CriterionSpecification,
    ruleset: RulesetSpecification,
    evaluator: CriterionEvaluator,
) -> None:
    if assessment.code is not specification.code:
        raise CriteriaEngineError("evaluator returned an assessment for another code")
    if assessment.status is CriterionStatus.DISABLED:
        raise CriteriaEngineError("enabled criterion evaluator returned disabled")
    if assessment.original_strength is not specification.base_strength:
        raise CriteriaEngineError("assessment original_strength differs from ruleset")
    if assessment.applied_strength not in (None, *specification.allowed_strengths):
        raise CriteriaEngineError(
            "assessment applied_strength is not allowed by ruleset"
        )
    if (
        assessment.ruleset_id != ruleset.ruleset_id
        or assessment.ruleset_version != ruleset.version
    ):
        raise CriteriaEngineError("assessment ruleset reference differs from selection")
    if (
        assessment.evaluator_id != evaluator.evaluator_id
        or assessment.evaluator_version != evaluator.evaluator_version
    ):
        raise CriteriaEngineError(
            "assessment evaluator reference differs from registry"
        )
