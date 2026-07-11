"""Pure, ruleset-configured consequence and location criterion evaluators."""

from __future__ import annotations

from collections.abc import Iterable, Mapping
from dataclasses import dataclass

from acmg_classifier.domain.criteria import CriterionAssessment, CriterionComparison
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

_CONSEQUENCE_CODES = frozenset(
    (
        CriterionCode.PVS1,
        CriterionCode.PS1,
        CriterionCode.PM1,
        CriterionCode.PM4,
        CriterionCode.PM5,
        CriterionCode.BP1,
        CriterionCode.BP3,
        CriterionCode.BP7,
    )
)
_VALIDITY_ORDER = {
    "unknown": 0,
    "refuted": 0,
    "disputed": 0,
    "limited": 1,
    "moderate": 2,
    "strong": 3,
    "definitive": 4,
}


@dataclass(frozen=True, slots=True)
class ConsequenceCriterionEvaluator:
    """Evaluate structured consequence and location facts without label shortcuts."""

    code: CriterionCode
    evaluator_id: str = "consequence"
    evaluator_version: str = "1.0.0"

    def __post_init__(self) -> None:
        if self.code not in _CONSEQUENCE_CODES:
            raise ValueError("unsupported consequence criterion code")

    def evaluate(
        self,
        facts: FactSet,
        context: InterpretationContext,
        specification: CriterionSpecification,
        ruleset: RulesetSpecification,
    ) -> CriterionAssessment:
        """Return a typed, data-only assessment for one configured criterion."""
        if specification.code is not self.code:
            raise ValueError("consequence evaluator specification code does not match")
        if self.code is CriterionCode.PVS1:
            return self._pvs1(facts, context, specification, ruleset)
        if self.code is CriterionCode.PS1:
            return self._ps1(facts, specification, ruleset)
        if self.code is CriterionCode.PM1:
            return self._pm1(facts, specification, ruleset)
        if self.code is CriterionCode.PM4:
            return self._pm4(facts, specification, ruleset)
        if self.code is CriterionCode.PM5:
            return self._pm5(facts, specification, ruleset)
        if self.code is CriterionCode.BP1:
            return self._bp1(facts, context, specification, ruleset)
        if self.code is CriterionCode.BP3:
            return self._bp3(facts, specification, ruleset)
        return self._bp7(facts, specification, ruleset)

    def _pvs1(
        self,
        facts: FactSet,
        context: InterpretationContext,
        specification: CriterionSpecification,
        ruleset: RulesetSpecification,
    ) -> CriterionAssessment:
        allowed, error = _string_set(specification.parameters, "allowed_consequences")
        required_mechanism, mechanism_error = _text(
            specification.parameters, "required_gene_mechanism"
        )
        minimum_validity, validity_error = _text(
            specification.parameters, "minimum_gene_validity"
        )
        require_nmd, nmd_error = _bool(specification.parameters, "require_nmd")
        if error or mechanism_error or validity_error or nmd_error:
            return self._invalid_configuration(specification, ruleset)
        assert allowed is not None
        assert required_mechanism is not None
        assert minimum_validity is not None
        assert require_nmd is not None
        if minimum_validity not in _VALIDITY_ORDER:
            return self._invalid_configuration(specification, ruleset)
        if not facts.consequence:
            return self._not_evaluable(
                specification, ruleset, "missing structured consequence evidence"
            )
        matching_consequences = tuple(
            observation
            for observation in facts.consequence
            if observation.consequence in allowed
        )
        if not matching_consequences:
            return self._not_applied(
                specification,
                ruleset,
                "configured loss-of-function consequence not present",
            )
        if require_nmd:
            nmd_values = tuple(
                observation.nmd_predicted for observation in matching_consequences
            )
            if all(value is None for value in nmd_values):
                return self._not_evaluable(
                    specification,
                    ruleset,
                    "missing nonsense-mediated decay decision data",
                )
            matching_consequences = tuple(
                observation
                for observation in matching_consequences
                if observation.nmd_predicted is True
            )
            if not matching_consequences:
                return self._not_applied(
                    specification,
                    ruleset,
                    "loss-of-function consequence is predicted to escape NMD",
                )
        if not facts.gene_mechanism:
            return self._not_evaluable(
                specification, ruleset, "missing gene-disease mechanism evidence"
            )
        mechanisms = tuple(
            observation
            for observation in facts.gene_mechanism
            if _mechanism_matches(
                observation,
                context,
                required_mechanism,
                minimum_validity,
            )
        )
        if not mechanisms:
            return self._not_applied(
                specification,
                ruleset,
                "configured gene-disease loss-of-function mechanism is not established",
            )
        observations: tuple[Observation, ...] = (*matching_consequences, *mechanisms)
        return self._applied(facts, specification, ruleset, observations)

    def _ps1(
        self,
        facts: FactSet,
        specification: CriterionSpecification,
        ruleset: RulesetSpecification,
    ) -> CriterionAssessment:
        required, error = _bool(
            specification.parameters, "require_same_amino_acid_change"
        )
        if error or required is not True:
            return self._invalid_configuration(specification, ruleset)
        values = tuple(
            observation
            for observation in facts.consequence
            if observation.same_amino_acid_change is not None
        )
        if not values:
            return self._not_evaluable(
                specification, ruleset, "missing same-amino-acid comparison evidence"
            )
        matching = tuple(
            observation
            for observation in values
            if observation.same_amino_acid_change is True
            and observation.same_amino_acid_splice_difference is False
        )
        if not matching:
            return self._not_applied(
                specification,
                ruleset,
                "same amino-acid change is absent or has a splice difference",
            )
        return self._applied(facts, specification, ruleset, matching)

    def _pm1(
        self,
        facts: FactSet,
        specification: CriterionSpecification,
        ruleset: RulesetSpecification,
    ) -> CriterionAssessment:
        region_type, type_error = _text(
            specification.parameters, "required_region_type"
        )
        critical, critical_error = _bool(
            specification.parameters, "require_critical_region"
        )
        if type_error or critical_error or region_type is None or critical is not True:
            return self._invalid_configuration(specification, ruleset)
        if not facts.variant_location:
            return self._not_evaluable(
                specification, ruleset, "missing structured variant-location evidence"
            )
        matching = tuple(
            location
            for location in facts.variant_location
            if location.region_type == region_type and location.is_critical is True
        )
        if not matching:
            return self._not_applied(
                specification, ruleset, "configured critical region is not present"
            )
        return self._applied(facts, specification, ruleset, matching)

    def _pm4(
        self,
        facts: FactSet,
        specification: CriterionSpecification,
        ruleset: RulesetSpecification,
    ) -> CriterionAssessment:
        allowed, allowed_error = _string_set(
            specification.parameters, "allowed_consequences"
        )
        minimum_length, length_error = _integer(
            specification.parameters, "minimum_inframe_length", minimum=1
        )
        if allowed_error or length_error or allowed is None or minimum_length is None:
            return self._invalid_configuration(specification, ruleset)
        if not facts.consequence:
            return self._not_evaluable(
                specification, ruleset, "missing structured consequence evidence"
            )
        candidates = tuple(
            observation
            for observation in facts.consequence
            if observation.consequence in allowed
        )
        if not candidates:
            return self._not_applied(
                specification, ruleset, "configured in-frame consequence is not present"
            )
        if all(observation.inframe_length is None for observation in candidates):
            return self._not_evaluable(
                specification, ruleset, "missing in-frame length evidence"
            )
        matching = tuple(
            observation
            for observation in candidates
            if observation.inframe_length is not None
            and observation.inframe_length >= minimum_length
        )
        if not matching:
            return self._not_applied(
                specification,
                ruleset,
                "in-frame length does not meet the ruleset minimum",
            )
        return self._applied(facts, specification, ruleset, matching)

    def _pm5(
        self,
        facts: FactSet,
        specification: CriterionSpecification,
        ruleset: RulesetSpecification,
    ) -> CriterionAssessment:
        required, error = _bool(
            specification.parameters, "require_same_residue_different_amino_acid"
        )
        if error or required is not True:
            return self._invalid_configuration(specification, ruleset)
        values = tuple(
            observation
            for observation in facts.consequence
            if observation.same_residue_different_amino_acid is not None
        )
        if not values:
            return self._not_evaluable(
                specification, ruleset, "missing same-residue comparison evidence"
            )
        matching = tuple(
            observation
            for observation in values
            if observation.same_residue_different_amino_acid is True
            and observation.same_amino_acid_change is False
        )
        if not matching:
            return self._not_applied(
                specification,
                ruleset,
                "different same-residue amino-acid change is absent",
            )
        return self._applied(facts, specification, ruleset, matching)

    def _bp1(
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
        if consequence_error or mechanism_error:
            return self._invalid_configuration(specification, ruleset)
        assert required_consequence is not None
        assert required_mechanism is not None
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
        mechanisms = tuple(
            observation
            for observation in facts.gene_mechanism
            if _mechanism_matches(
                observation,
                context,
                required_mechanism,
                minimum_validity="limited",
            )
        )
        if not consequences or not mechanisms:
            return self._not_applied(
                specification,
                ruleset,
                "configured consequence and gene-mechanism combination is absent",
            )
        return self._applied(
            facts, specification, ruleset, (*consequences, *mechanisms)
        )

    def _bp3(
        self,
        facts: FactSet,
        specification: CriterionSpecification,
        ruleset: RulesetSpecification,
    ) -> CriterionAssessment:
        region_type, type_error = _text(
            specification.parameters, "required_region_type"
        )
        minimum_length, length_error = _integer(
            specification.parameters, "minimum_inframe_length", minimum=1
        )
        if type_error or length_error or region_type is None or minimum_length is None:
            return self._invalid_configuration(specification, ruleset)
        if not facts.consequence or not facts.variant_location:
            return self._not_evaluable(
                specification,
                ruleset,
                "missing structured consequence or variant-location evidence",
            )
        consequences = tuple(
            observation
            for observation in facts.consequence
            if observation.consequence == "inframe_indel"
            and observation.inframe_length is not None
            and observation.inframe_length >= minimum_length
        )
        locations = tuple(
            observation
            for observation in facts.variant_location
            if observation.region_type == region_type
        )
        if not consequences or not locations:
            return self._not_applied(
                specification,
                ruleset,
                "configured repetitive-region in-frame evidence is absent",
            )
        return self._applied(facts, specification, ruleset, (*consequences, *locations))

    def _bp7(
        self,
        facts: FactSet,
        specification: CriterionSpecification,
        ruleset: RulesetSpecification,
    ) -> CriterionAssessment:
        required_consequence, consequence_error = _text(
            specification.parameters, "required_consequence"
        )
        minimum_distance, distance_error = _integer(
            specification.parameters, "minimum_splice_distance", minimum=0
        )
        if consequence_error or distance_error:
            return self._invalid_configuration(specification, ruleset)
        assert required_consequence is not None
        assert minimum_distance is not None
        if not facts.consequence or not facts.variant_location:
            return self._not_evaluable(
                specification,
                ruleset,
                "missing structured consequence or splice-distance evidence",
            )
        consequences = tuple(
            observation
            for observation in facts.consequence
            if observation.consequence == required_consequence
        )
        if not consequences:
            return self._not_applied(
                specification, ruleset, "configured synonymous consequence is absent"
            )
        if any(observation.splice_impact == "unknown" for observation in consequences):
            return self._not_evaluable(
                specification, ruleset, "missing splice-impact decision data"
            )
        if any(
            observation.splice_impact in ("predicted", "confirmed")
            for observation in consequences
        ):
            return self._not_applied(
                specification, ruleset, "synonymous change has splice-impact evidence"
            )
        locations = tuple(
            observation
            for observation in facts.variant_location
            if observation.distance_to_splice_site is not None
            and observation.distance_to_splice_site >= minimum_distance
        )
        if not locations:
            return self._not_applied(
                specification,
                ruleset,
                "splice distance does not meet the ruleset minimum",
            )
        return self._applied(facts, specification, ruleset, (*consequences, *locations))

    def _applied(
        self,
        facts: FactSet,
        specification: CriterionSpecification,
        ruleset: RulesetSpecification,
        observations: Iterable[Observation],
    ) -> CriterionAssessment:
        evidence_ids = _evidence_ids(facts, observations)
        if not evidence_ids:
            return self._not_evaluable(
                specification,
                ruleset,
                "decisive observations lack evidence identifiers",
            )
        strength, error = _applied_strength(specification)
        if error is not None:
            return self._invalid_configuration(specification, ruleset)
        assert strength is not None
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

    def _invalid_configuration(
        self,
        specification: CriterionSpecification,
        ruleset: RulesetSpecification,
    ) -> CriterionAssessment:
        return self._not_evaluable(
            specification, ruleset, "ruleset consequence configuration is invalid"
        )

    def _assessment(
        self,
        *,
        status: CriterionStatus,
        specification: CriterionSpecification,
        ruleset: RulesetSpecification,
        evidence_ids: tuple[str, ...] = (),
        comparisons: tuple[CriterionComparison, ...] = (),
        rationale_values: Mapping[str, JsonValue] | None = None,
        limitations: tuple[str, ...] = (),
        applied_strength: CriterionStrength | None = None,
    ) -> CriterionAssessment:
        return CriterionAssessment(
            code=specification.code,
            status=status,
            original_strength=specification.base_strength,
            applied_strength=applied_strength,
            evidence_ids=evidence_ids,
            comparisons=comparisons,
            rationale_template=specification.rationale_template,
            rationale_values={} if rationale_values is None else rationale_values,
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
) -> bool:
    if observation.mechanism != required_mechanism:
        return False
    if _VALIDITY_ORDER[observation.validity] < _VALIDITY_ORDER[minimum_validity]:
        return False
    if context.disease_id is not None and observation.disease_id != context.disease_id:
        return False
    return (
        context.inheritance is None
        or observation.inheritance is None
        or observation.inheritance is context.inheritance
    )


def _string_set(
    parameters: Mapping[str, JsonValue], key: str
) -> tuple[frozenset[str] | None, str | None]:
    value = parameters.get(key)
    if isinstance(value, str) and value:
        return frozenset((value,)), None
    if isinstance(value, list) and value:
        strings = tuple(item for item in value if isinstance(item, str) and item)
        if len(strings) == len(value):
            return frozenset(strings), None
    return None, f"{key} must be non-empty text or a non-empty text list"


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


def _integer(
    parameters: Mapping[str, JsonValue], key: str, *, minimum: int
) -> tuple[int | None, str | None]:
    value = parameters.get(key)
    if not isinstance(value, int) or isinstance(value, bool) or value < minimum:
        return None, f"{key} must be an integer >= {minimum}"
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


def _evidence_ids(
    facts: FactSet, observations: Iterable[Observation]
) -> tuple[str, ...]:
    return tuple(
        sorted(
            {
                evidence_id
                for observation in observations
                for evidence_id in facts.evidence_ids_for(observation)
            }
        )
    )
