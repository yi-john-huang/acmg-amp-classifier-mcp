"""Validated, versioned ACMG/AMP ruleset selection.

This module owns rule specification data and deterministic selection only. It has
no evidence acquisition or evaluator implementation dependencies, so a selected
ruleset can be reproduced from its recorded version and context.
"""

from __future__ import annotations

import re
from collections.abc import Mapping
from dataclasses import dataclass
from enum import StrEnum
from functools import cmp_to_key
from types import MappingProxyType
from typing import Self

from pydantic import Field, StringConstraints, field_validator, model_validator

from acmg_classifier.domain.enums import GenomeBuild, InheritanceMode
from acmg_classifier.domain.errors import JsonValue
from acmg_classifier.domain.evidence import EvidenceModel, NonEmptyText, OntologyId
from acmg_classifier.domain.models import InterpretationContext

RulesetId = StringConstraints(
    strip_whitespace=True,
    min_length=3,
    max_length=128,
    pattern=r"^[a-z][a-z0-9_-]*$",
)

_SEMANTIC_VERSION = re.compile(
    r"^(?P<major>0|[1-9]\d*)\."
    r"(?P<minor>0|[1-9]\d*)\."
    r"(?P<patch>0|[1-9]\d*)"
    r"(?:-(?P<prerelease>[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?"
    r"(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$"
)
_VERSION_CONSTRAINT = re.compile(r"^(?P<operator>>=|<=|>|<|=)?(?P<version>.+)$")


class CriterionCode(StrEnum):
    """The 28 ACMG/AMP 2015 evidence criterion codes."""

    PVS1 = "PVS1"
    PS1 = "PS1"
    PS2 = "PS2"
    PS3 = "PS3"
    PS4 = "PS4"
    PM1 = "PM1"
    PM2 = "PM2"
    PM3 = "PM3"
    PM4 = "PM4"
    PM5 = "PM5"
    PM6 = "PM6"
    PP1 = "PP1"
    PP2 = "PP2"
    PP3 = "PP3"
    PP4 = "PP4"
    PP5 = "PP5"
    BA1 = "BA1"
    BS1 = "BS1"
    BS2 = "BS2"
    BS3 = "BS3"
    BS4 = "BS4"
    BP1 = "BP1"
    BP2 = "BP2"
    BP3 = "BP3"
    BP4 = "BP4"
    BP5 = "BP5"
    BP6 = "BP6"
    BP7 = "BP7"

    @property
    def is_benign(self) -> bool:
        """Return whether the code contributes benign-direction evidence."""
        return self.value.startswith(("BA", "BS", "BP"))


class CriterionStrength(StrEnum):
    """Controlled ACMG/AMP strengths, including the standalone benign strength."""

    STANDALONE = "standalone"
    VERY_STRONG = "very_strong"
    STRONG = "strong"
    MODERATE = "moderate"
    SUPPORTING = "supporting"


class RulesetState(StrEnum):
    """Approval state of a published specification."""

    DRAFT = "draft"
    APPROVED = "approved"
    RETIRED = "retired"


class RulesetSelectionStatus(StrEnum):
    """Result of deterministic specification selection."""

    SELECTED = "selected"
    NEEDS_CONTEXT = "needs_context"
    NO_COMPATIBLE_RULESET = "no_compatible_ruleset"


class RejectionReason(StrEnum):
    """Auditable reason a candidate specification was not activated."""

    NOT_APPROVED = "not_approved"
    NOT_REQUESTED = "not_requested"
    CONTEXT_MISMATCH = "context_mismatch"
    MISSING_CONTEXT = "missing_context"
    EVALUATOR_MISSING = "evaluator_missing"
    EVALUATOR_INCOMPATIBLE = "evaluator_incompatible"
    LOWER_PRECEDENCE = "lower_precedence"


@dataclass(frozen=True, slots=True)
class _SemanticVersion:
    """Parsed SemVer 2.0.0 value used only for validated rule metadata."""

    major: int
    minor: int
    patch: int
    prerelease: tuple[str, ...] | None


class RulesetScope(EvidenceModel):
    """Optional selectors that bound a specification's biological applicability."""

    gene_symbol: NonEmptyText | None = None
    disease_id: OntologyId | None = None
    inheritance: InheritanceMode | None = None
    transcript: NonEmptyText | None = None
    genome_build: GenomeBuild | None = None

    def assess(
        self,
        context: InterpretationContext,
        *,
        gene_symbol: str | None,
    ) -> tuple[tuple[str, ...], tuple[str, ...]]:
        """Return missing and mismatched selector field names in stable order."""
        candidates = (
            ("gene_symbol", self.gene_symbol, gene_symbol),
            ("disease_id", self.disease_id, context.disease_id),
            ("inheritance", self.inheritance, context.inheritance),
            ("transcript", self.transcript, context.transcript),
            ("genome_build", self.genome_build, context.genome_build),
        )
        missing: list[str] = []
        mismatched: list[str] = []
        for field, expected, actual in candidates:
            if expected is None:
                continue
            if actual is None:
                missing.append(field)
            elif actual != expected:
                mismatched.append(field)
        return tuple(missing), tuple(mismatched)

    @property
    def specificity(self) -> int:
        """Return the count of explicit selectors for deterministic precedence."""
        return sum(
            value is not None
            for value in (
                self.gene_symbol,
                self.disease_id,
                self.inheritance,
                self.transcript,
                self.genome_build,
            )
        )


class CriterionSpecification(EvidenceModel):
    """Data-owned enablement, strength, and evaluator compatibility for one code."""

    code: CriterionCode
    enabled: bool
    base_strength: CriterionStrength | None = None
    allowed_strengths: tuple[CriterionStrength, ...] = ()
    evaluator_id: NonEmptyText
    evaluator_version_constraint: NonEmptyText
    rationale_template: NonEmptyText
    parameters: Mapping[str, JsonValue] = Field(default_factory=dict)

    @field_validator("evaluator_version_constraint")
    @classmethod
    def validate_evaluator_constraint(cls, value: str) -> str:
        _parse_version_constraint(value)
        return value

    @model_validator(mode="after")
    def validate_strength_configuration(self) -> Self:
        if self.enabled:
            if self.base_strength is None:
                raise ValueError("enabled criterion requires a base_strength")
            if self.base_strength not in self.allowed_strengths:
                raise ValueError("allowed_strengths must include base_strength")
        elif self.base_strength is not None or self.allowed_strengths:
            raise ValueError("disabled criterion must not declare strengths")
        if len(set(self.allowed_strengths)) != len(self.allowed_strengths):
            raise ValueError("allowed_strengths must not contain duplicates")
        object.__setattr__(self, "parameters", MappingProxyType(dict(self.parameters)))
        return self


class RulesetSpecification(EvidenceModel):
    """An immutable, complete, versioned ACMG/AMP automation specification."""

    ruleset_id: str
    version: str
    state: RulesetState
    publication_reference: NonEmptyText
    scope: RulesetScope
    criteria: tuple[CriterionSpecification, ...]
    combination_algorithm_id: NonEmptyText
    combination_algorithm_version: NonEmptyText
    supersedes: tuple[NonEmptyText, ...] = ()

    @field_validator("ruleset_id")
    @classmethod
    def validate_ruleset_id(cls, value: str) -> str:
        cleaned = value.strip()
        if not re.fullmatch(r"[a-z][a-z0-9_-]{2,127}", cleaned):
            raise ValueError("ruleset_id must be a stable lowercase identifier")
        return cleaned

    @field_validator("version", "combination_algorithm_version")
    @classmethod
    def validate_semantic_version(cls, value: str) -> str:
        _parse_semantic_version(value)
        return value

    @model_validator(mode="after")
    def validate_complete_criteria(self) -> Self:
        codes = tuple(criterion.code for criterion in self.criteria)
        if len(codes) != len(set(codes)):
            raise ValueError("each ACMG/AMP criterion must occur exactly once")
        if set(codes) != set(CriterionCode):
            raise ValueError(
                "ruleset must define every ACMG/AMP criterion exactly once"
            )
        return self


class RulesetCandidateRejection(EvidenceModel):
    """A stable audit record for a candidate not activated by selection."""

    ruleset_id: str
    version: str
    reason: RejectionReason
    fields: tuple[str, ...] = ()


class RulesetSelection(EvidenceModel):
    """Selection result with candidate audit data and no hidden fallback."""

    status: RulesetSelectionStatus
    ruleset: RulesetSpecification | None = None
    rejected: tuple[RulesetCandidateRejection, ...] = ()
    required_context_fields: tuple[str, ...] = ()

    @model_validator(mode="after")
    def validate_result_shape(self) -> Self:
        if self.status is RulesetSelectionStatus.SELECTED:
            if self.ruleset is None or self.required_context_fields:
                raise ValueError(
                    "selected result requires a ruleset and no missing context"
                )
        elif self.ruleset is not None:
            raise ValueError("non-selected result must not contain a ruleset")
        return self


@dataclass(frozen=True, slots=True)
class RulesetRegistry:
    """Immutable registry that selects only approved compatible specifications."""

    rulesets: tuple[RulesetSpecification, ...]

    def __post_init__(self) -> None:
        candidates = tuple(self.rulesets)
        if not all(
            isinstance(candidate, RulesetSpecification) for candidate in candidates
        ):
            raise TypeError("rulesets must contain RulesetSpecification values")
        identities = tuple(
            (candidate.ruleset_id, candidate.version) for candidate in candidates
        )
        if len(identities) != len(set(identities)):
            raise ValueError("duplicate ruleset identity and version")
        object.__setattr__(
            self,
            "rulesets",
            tuple(
                sorted(
                    candidates,
                    key=cmp_to_key(_compare_registry_candidates),
                )
            ),
        )

    def select(
        self,
        context: InterpretationContext,
        *,
        gene_symbol: str | None = None,
        requested_ruleset_id: str | None = None,
        evaluator_versions: Mapping[str, str] | None = None,
    ) -> RulesetSelection:
        """Select one approved compatible ruleset or return an auditable blocker."""
        rejected: list[RulesetCandidateRejection] = []
        matching: list[RulesetSpecification] = []
        pending: list[tuple[RulesetSpecification, tuple[str, ...]]] = []

        for candidate in self.rulesets:
            if candidate.state is not RulesetState.APPROVED:
                rejected.append(_rejected(candidate, RejectionReason.NOT_APPROVED))
                continue
            if (
                requested_ruleset_id is not None
                and candidate.ruleset_id != requested_ruleset_id
            ):
                rejected.append(_rejected(candidate, RejectionReason.NOT_REQUESTED))
                continue
            compatibility = _evaluator_compatibility(candidate, evaluator_versions)
            if compatibility is not None:
                rejected.append(_rejected(candidate, compatibility))
                continue
            missing, mismatched = candidate.scope.assess(
                context, gene_symbol=gene_symbol
            )
            if mismatched:
                rejected.append(
                    _rejected(
                        candidate,
                        RejectionReason.CONTEXT_MISMATCH,
                        fields=mismatched,
                    )
                )
                continue
            if missing:
                pending.append((candidate, missing))
                rejected.append(
                    _rejected(
                        candidate,
                        RejectionReason.MISSING_CONTEXT,
                        fields=missing,
                    )
                )
                continue
            matching.append(candidate)

        if requested_ruleset_id is not None:
            return self._select_requested(matching, pending, rejected)
        return self._select_default(matching, pending, rejected)

    @staticmethod
    def _select_requested(
        matching: list[RulesetSpecification],
        pending: list[tuple[RulesetSpecification, tuple[str, ...]]],
        rejected: list[RulesetCandidateRejection],
    ) -> RulesetSelection:
        if matching:
            selected = _highest_version(matching)
            rejected.extend(
                _rejected(candidate, RejectionReason.LOWER_PRECEDENCE)
                for candidate in matching
                if candidate is not selected
            )
            return _selected(selected, rejected)
        if pending:
            return _needs_context(pending, rejected)
        return _no_compatible(rejected)

    @staticmethod
    def _select_default(
        matching: list[RulesetSpecification],
        pending: list[tuple[RulesetSpecification, tuple[str, ...]]],
        rejected: list[RulesetCandidateRejection],
    ) -> RulesetSelection:
        if not matching:
            return (
                _needs_context(pending, rejected)
                if pending
                else _no_compatible(rejected)
            )

        highest_specificity = max(candidate.scope.specificity for candidate in matching)
        if any(
            candidate.scope.specificity >= highest_specificity
            for candidate, _ in pending
        ):
            return _needs_context(pending, rejected)

        most_specific = [
            candidate
            for candidate in matching
            if candidate.scope.specificity == highest_specificity
        ]
        ruleset_ids = {candidate.ruleset_id for candidate in most_specific}
        if len(ruleset_ids) != 1:
            rejected.extend(
                _rejected(candidate, RejectionReason.LOWER_PRECEDENCE)
                for candidate in matching
                if candidate.scope.specificity < highest_specificity
            )
            return RulesetSelection(
                status=RulesetSelectionStatus.NEEDS_CONTEXT,
                rejected=_sorted_rejections(rejected),
                required_context_fields=("ruleset_id",),
            )

        selected = _highest_version(most_specific)
        rejected.extend(
            _rejected(candidate, RejectionReason.LOWER_PRECEDENCE)
            for candidate in matching
            if candidate is not selected
        )
        return _selected(selected, rejected)


def _selected(
    selected: RulesetSpecification,
    rejected: list[RulesetCandidateRejection],
) -> RulesetSelection:
    return RulesetSelection(
        status=RulesetSelectionStatus.SELECTED,
        ruleset=selected,
        rejected=_sorted_rejections(rejected),
    )


def _needs_context(
    pending: list[tuple[RulesetSpecification, tuple[str, ...]]],
    rejected: list[RulesetCandidateRejection],
) -> RulesetSelection:
    fields = tuple(sorted({field for _, values in pending for field in values}))
    return RulesetSelection(
        status=RulesetSelectionStatus.NEEDS_CONTEXT,
        rejected=_sorted_rejections(rejected),
        required_context_fields=fields,
    )


def _no_compatible(
    rejected: list[RulesetCandidateRejection],
) -> RulesetSelection:
    return RulesetSelection(
        status=RulesetSelectionStatus.NO_COMPATIBLE_RULESET,
        rejected=_sorted_rejections(rejected),
    )


def _rejected(
    candidate: RulesetSpecification,
    reason: RejectionReason,
    *,
    fields: tuple[str, ...] = (),
) -> RulesetCandidateRejection:
    return RulesetCandidateRejection(
        ruleset_id=candidate.ruleset_id,
        version=candidate.version,
        reason=reason,
        fields=tuple(sorted(fields)),
    )


def _sorted_rejections(
    values: list[RulesetCandidateRejection],
) -> tuple[RulesetCandidateRejection, ...]:
    return tuple(
        sorted(
            values,
            key=lambda value: (
                value.ruleset_id,
                _semantic_version_sort_key(value.version),
                value.reason.value,
                value.fields,
            ),
        )
    )


def _evaluator_compatibility(
    candidate: RulesetSpecification,
    versions: Mapping[str, str] | None,
) -> RejectionReason | None:
    if versions is None:
        return None
    for criterion in candidate.criteria:
        if not criterion.enabled:
            continue
        version = versions.get(criterion.evaluator_id)
        if version is None:
            return RejectionReason.EVALUATOR_MISSING
        if not version_satisfies(version, criterion.evaluator_version_constraint):
            return RejectionReason.EVALUATOR_INCOMPATIBLE
    return None


def _highest_version(
    values: list[RulesetSpecification],
) -> RulesetSpecification:
    return max(values, key=cmp_to_key(_compare_ruleset_versions))


def _compare_registry_candidates(
    left: RulesetSpecification,
    right: RulesetSpecification,
) -> int:
    identity_comparison = (left.ruleset_id > right.ruleset_id) - (
        left.ruleset_id < right.ruleset_id
    )
    if identity_comparison:
        return identity_comparison
    return _compare_semantic_versions(
        _parse_semantic_version(left.version),
        _parse_semantic_version(right.version),
    )


def _compare_ruleset_versions(
    left: RulesetSpecification,
    right: RulesetSpecification,
) -> int:
    return _compare_semantic_versions(
        _parse_semantic_version(left.version),
        _parse_semantic_version(right.version),
    )


def _semantic_version_sort_key(
    value: str,
) -> tuple[int, int, int, int, tuple[str, ...]]:
    parsed = _parse_semantic_version(value)
    return (
        parsed.major,
        parsed.minor,
        parsed.patch,
        1 if parsed.prerelease is None else 0,
        parsed.prerelease or (),
    )


def _parse_version_constraint(value: str) -> tuple[tuple[str, _SemanticVersion], ...]:
    if value == "*":
        return ()
    constraints: list[tuple[str, _SemanticVersion]] = []
    for fragment in value.split(","):
        match = _VERSION_CONSTRAINT.fullmatch(fragment.strip())
        if match is None:
            raise ValueError("invalid evaluator semantic-version constraint")
        operator = match.group("operator") or "="
        constraints.append((operator, _parse_semantic_version(match.group("version"))))
    if not constraints:
        raise ValueError("evaluator semantic-version constraint is required")
    return tuple(constraints)


def version_satisfies(version: str, constraint: str) -> bool:
    candidate = _parse_semantic_version(version)
    for operator, required in _parse_version_constraint(constraint):
        comparison = _compare_semantic_versions(candidate, required)
        if not {
            ">=": comparison >= 0,
            "<=": comparison <= 0,
            ">": comparison > 0,
            "<": comparison < 0,
            "=": comparison == 0,
        }[operator]:
            return False
    return True


def _parse_semantic_version(value: str) -> _SemanticVersion:
    match = _SEMANTIC_VERSION.fullmatch(value)
    if match is None:
        raise ValueError("value must be a semantic version")
    prerelease = match.group("prerelease")
    return _SemanticVersion(
        major=int(match.group("major")),
        minor=int(match.group("minor")),
        patch=int(match.group("patch")),
        prerelease=None if prerelease is None else tuple(prerelease.split(".")),
    )


def _compare_semantic_versions(left: _SemanticVersion, right: _SemanticVersion) -> int:
    core = (left.major, left.minor, left.patch)
    other_core = (right.major, right.minor, right.patch)
    if core != other_core:
        return (core > other_core) - (core < other_core)
    if left.prerelease is None:
        return 0 if right.prerelease is None else 1
    if right.prerelease is None:
        return -1
    for left_identifier, right_identifier in zip(
        left.prerelease, right.prerelease, strict=False
    ):
        if left_identifier == right_identifier:
            continue
        left_numeric = left_identifier.isdecimal()
        right_numeric = right_identifier.isdecimal()
        if left_numeric and right_numeric:
            return (int(left_identifier) > int(right_identifier)) - (
                int(left_identifier) < int(right_identifier)
            )
        if left_numeric != right_numeric:
            return -1 if left_numeric else 1
        return (left_identifier > right_identifier) - (
            left_identifier < right_identifier
        )
    return (len(left.prerelease) > len(right.prerelease)) - (
        len(left.prerelease) < len(right.prerelease)
    )
