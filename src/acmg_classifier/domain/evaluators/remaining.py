"""Remaining explicit criterion behavior and deprecated-criterion handling."""

from __future__ import annotations

from collections.abc import Iterable, Mapping
from dataclasses import dataclass

from acmg_classifier.domain.criteria import CriterionAssessment
from acmg_classifier.domain.enums import CriterionStatus
from acmg_classifier.domain.errors import JsonValue
from acmg_classifier.domain.evidence import (
    FactSet,
    GeneMechanismObservation,
    Observation,
)
from acmg_classifier.domain.models import InterpretationContext
from acmg_classifier.domain.rules import (
    CriterionCode,
    CriterionSpecification,
    CriterionStrength,
    RulesetSpecification,
)

_SUPPORTED_CODES = frozenset((CriterionCode.PP2, CriterionCode.PP5, CriterionCode.BP6))
_VALIDITY_ORDER = {
    "unknown": 0,
    "refuted": 0,
    "disputed": 0,
    "limited": 1,
    "moderate": 2,
    "strong": 3,
    "definitive": 4,
}
_DEPRECATED_CODES = frozenset((CriterionCode.PP5, CriterionCode.BP6))


@dataclass(frozen=True, slots=True)
class RemainingCriterionEvaluator:
    """Evaluate PP2 and make retired PP5/BP6 behavior explicit and fail-closed."""

    code: CriterionCode
    evaluator_id: str = "remaining"
    evaluator_version: str = "1.0.0"

    def __post_init__(self) -> None:
        if self.code not in _SUPPORTED_CODES:
            raise ValueError("unsupported remaining criterion code")

    def evaluate(
        self,
        facts: FactSet,
        context: InterpretationContext,
        specification: CriterionSpecification,
        ruleset: RulesetSpecification,
    ) -> CriterionAssessment:
        """Return one structured assessment without relying on source assertions."""
        if specification.code is not self.code:
            raise ValueError("remaining evaluator specification code does not match")
        if self.code in _DEPRECATED_CODES:
            if not specification.enabled:
                return CriterionAssessment(
                    code=specification.code,
                    status=CriterionStatus.DISABLED,
                    original_strength=None,
                    applied_strength=None,
                    evidence_ids=(),
                    comparisons=(),
                    rationale_template=specification.rationale_template,
                    rationale_values={},
                    limitations=("criterion is disabled by the selected ruleset",),
                    ruleset_id=ruleset.ruleset_id,
                    ruleset_version=ruleset.version,
                    evaluator_id=None,
                    evaluator_version=None,
                )
            return self._not_evaluable(
                specification,
                ruleset,
                "criterion is deprecated and cannot be automatically applied",
            )
        return self._pp2(facts, context, specification, ruleset)

    def _pp2(
        self,
        facts: FactSet,
        context: InterpretationContext,
        specification: CriterionSpecification,
        ruleset: RulesetSpecification,
    ) -> CriterionAssessment:
        required_consequence, consequence_error = _text(
            specification.parameters, "required_consequence"
        )
        required_mechanism, mechanism_error = _text(
            specification.parameters, "required_gene_mechanism"
        )
        minimum_validity, validity_error = _text(
            specification.parameters, "minimum_gene_validity"
        )
        require_missense, missense_error = _bool(
            specification.parameters, "require_established_missense_mechanism"
        )
        require_low_rate, rate_error = _bool(
            specification.parameters, "require_low_benign_missense_rate"
        )
        if (
            any(
                error is not None
                for error in (
                    consequence_error,
                    mechanism_error,
                    validity_error,
                    missense_error,
                    rate_error,
                )
            )
            or minimum_validity not in _VALIDITY_ORDER
        ):
            return self._not_evaluable(
                specification, ruleset, "ruleset PP2 configuration is invalid"
            )
        assert required_consequence is not None
        assert required_mechanism is not None
        assert minimum_validity is not None
        assert require_missense is not None
        assert require_low_rate is not None
        if not facts.consequence or not facts.gene_mechanism:
            return self._not_evaluable(
                specification,
                ruleset,
                "missing structured consequence or gene-mechanism evidence",
            )
        consequences = tuple(
            observation
            for observation in facts.consequence
            if observation.consequence == required_consequence
        )
        if not consequences:
            return self._not_applied(
                specification, ruleset, "configured missense consequence is absent"
            )
        mechanisms = tuple(
            observation
            for observation in facts.gene_mechanism
            if _mechanism_matches(
                observation,
                context,
                required_mechanism,
                minimum_validity,
                require_missense,
                require_low_rate,
            )
        )
        if not mechanisms:
            return self._not_applied(
                specification,
                ruleset,
                "gene mechanism or missense-constraint requirements are not met",
            )
        return self._applied(
            facts, specification, ruleset, (*consequences, *mechanisms)
        )

    def _applied(
        self,
        facts: FactSet,
        specification: CriterionSpecification,
        ruleset: RulesetSpecification,
        observations: Iterable[Observation],
    ) -> CriterionAssessment:
        evidence_ids = tuple(
            sorted(
                {
                    evidence_id
                    for observation in observations
                    for evidence_id in facts.evidence_ids_for(observation)
                }
            )
        )
        if not evidence_ids:
            return self._not_evaluable(
                specification,
                ruleset,
                "decisive observations lack evidence identifiers",
            )
        strength, error = _applied_strength(specification)
        if error is not None or strength is None:
            return self._not_evaluable(
                specification, ruleset, "ruleset PP2 strength configuration is invalid"
            )
        return self._assessment(
            status=CriterionStatus.APPLIED,
            specification=specification,
            ruleset=ruleset,
            evidence_ids=evidence_ids,
            applied_strength=strength,
        )

    def _not_applied(
        self,
        specification: CriterionSpecification,
        ruleset: RulesetSpecification,
        limitation: str,
    ) -> CriterionAssessment:
        return self._assessment(
            status=CriterionStatus.NOT_APPLIED,
            specification=specification,
            ruleset=ruleset,
            limitations=(limitation,),
        )

    def _not_evaluable(
        self,
        specification: CriterionSpecification,
        ruleset: RulesetSpecification,
        limitation: str,
    ) -> CriterionAssessment:
        return self._assessment(
            status=CriterionStatus.NOT_EVALUABLE,
            specification=specification,
            ruleset=ruleset,
            limitations=(limitation,),
        )

    def _assessment(
        self,
        *,
        status: CriterionStatus,
        specification: CriterionSpecification,
        ruleset: RulesetSpecification,
        evidence_ids: tuple[str, ...] = (),
        limitations: tuple[str, ...] = (),
        applied_strength: CriterionStrength | None = None,
    ) -> CriterionAssessment:
        return CriterionAssessment(
            code=specification.code,
            status=status,
            original_strength=specification.base_strength,
            applied_strength=applied_strength,
            evidence_ids=evidence_ids,
            comparisons=(),
            rationale_template=specification.rationale_template,
            rationale_values={},
            limitations=limitations,
            ruleset_id=ruleset.ruleset_id,
            ruleset_version=ruleset.version,
            evaluator_id=self.evaluator_id,
            evaluator_version=self.evaluator_version,
        )


def _mechanism_matches(
    observation: GeneMechanismObservation,
    context: InterpretationContext,
    required_mechanism: str,
    minimum_validity: str,
    require_missense: bool,
    require_low_rate: bool,
) -> bool:
    if (
        observation.mechanism != required_mechanism
        or _VALIDITY_ORDER[observation.validity] < _VALIDITY_ORDER[minimum_validity]
    ):
        return False
    if context.disease_id is not None and observation.disease_id != context.disease_id:
        return False
    if require_missense and observation.missense_mechanism != "established":
        return False
    return not require_low_rate or observation.benign_missense_rate == "low"


def _text(
    parameters: Mapping[str, JsonValue], key: str
) -> tuple[str | None, str | None]:
    value = parameters.get(key)
    if not isinstance(value, str) or not value.strip():
        return None, f"{key} must be non-empty text"
    return value.strip(), None


def _bool(
    parameters: Mapping[str, JsonValue], key: str
) -> tuple[bool | None, str | None]:
    value = parameters.get(key)
    if not isinstance(value, bool):
        return None, f"{key} must be boolean"
    return value, None


def _applied_strength(
    specification: CriterionSpecification,
) -> tuple[CriterionStrength | None, str | None]:
    configured = specification.parameters.get("applied_strength")
    if configured is None:
        return specification.base_strength, None
    if not isinstance(configured, str):
        return None, "applied_strength must be a controlled strength"
    try:
        strength = CriterionStrength(configured)
    except ValueError:
        return None, "applied_strength must be a controlled strength"
    if strength not in specification.allowed_strengths:
        return None, "applied_strength is not allowed by this criterion"
    return strength, None
