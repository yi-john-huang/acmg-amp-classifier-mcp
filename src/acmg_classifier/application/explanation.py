"""Pure deterministic renderings of structured scientific decision content."""

from __future__ import annotations

import re
import string
from collections.abc import Mapping
from dataclasses import dataclass
from enum import StrEnum

from acmg_classifier.domain.canonical import canonical_json_bytes
from acmg_classifier.domain.combination import ClassificationDecision
from acmg_classifier.domain.enums import ClassificationTier, CriterionStatus
from acmg_classifier.domain.errors import JsonValue


class ExplanationDetail(StrEnum):
    """Presentation detail levels that never alter the scientific decision."""

    COMPACT = "compact"
    STANDARD = "standard"
    FULL = "full"


class ExplanationTemplateError(ValueError):
    """A rationale template attempts to reference unavailable or unsafe content."""


@dataclass(frozen=True, slots=True)
class ExplanationBlock:
    """One stable display block tied exclusively to decision facts."""

    kind: str
    title: str
    text: str
    criterion_code: str | None = None
    criterion_codes: tuple[str, ...] = ()
    evidence_ids: tuple[str, ...] = ()

    def __post_init__(self) -> None:
        if not self.kind or not self.title or not self.text:
            raise ValueError("explanation block kind, title, and text are required")
        object.__setattr__(
            self, "criterion_codes", tuple(sorted(set(self.criterion_codes)))
        )
        object.__setattr__(self, "evidence_ids", tuple(sorted(set(self.evidence_ids))))

    def to_canonical_content(self) -> dict[str, JsonValue]:
        """Return display-only content separate from decision hashing."""
        return {
            "kind": self.kind,
            "title": self.title,
            "text": self.text,
            "criterion_code": self.criterion_code,
            "criterion_codes": list(self.criterion_codes),
            "evidence_ids": list(self.evidence_ids),
        }


@dataclass(frozen=True, slots=True)
class Explanation:
    """A deterministic report whose statements originate in typed decision data."""

    detail: ExplanationDetail
    classification: str | None
    blocks: tuple[ExplanationBlock, ...]
    limitations: tuple[str, ...]
    disclaimer: str

    def __post_init__(self) -> None:
        if not self.blocks:
            raise ValueError("explanation requires a summary block")
        if not self.disclaimer:
            raise ValueError("explanation requires a research-use disclaimer")
        object.__setattr__(self, "limitations", tuple(sorted(set(self.limitations))))

    def to_canonical_content(self) -> dict[str, JsonValue]:
        """Return a deterministic display payload without changing decision hashes."""
        return {
            "detail": self.detail.value,
            "classification": self.classification,
            "blocks": [block.to_canonical_content() for block in self.blocks],
            "limitations": list(self.limitations),
            "disclaimer": self.disclaimer,
        }


class ExplanationService:
    """Render a decision using only approved rationale templates and facts."""

    _DISCLAIMER = (
        "Research use only; not for clinical diagnosis. Interpretation requires "
        "qualified professional review and may change as evidence changes."
    )

    def render_decision(
        self,
        decision: ClassificationDecision,
        *,
        detail: ExplanationDetail = ExplanationDetail.STANDARD,
    ) -> Explanation:
        """Render compact, standard, or full content without adding a new fact."""
        classification = _classification_label(decision.classification)
        limitations = tuple(
            sorted(
                set(
                    decision.limitations
                    + (
                        decision.conflict.limitations
                        if decision.conflict is not None
                        else ()
                    )
                    + tuple(
                        limitation
                        for assessment in decision.assessments
                        for limitation in assessment.limitations
                    )
                )
            )
        )
        blocks = [self._summary_block(decision, classification)]
        if detail is not ExplanationDetail.COMPACT:
            for assessment in decision.assessments:
                blocks.append(self._criterion_block(assessment))
                if detail is ExplanationDetail.FULL:
                    blocks.extend(self._comparison_blocks(assessment))
        return Explanation(
            detail=detail,
            classification=classification,
            blocks=tuple(blocks),
            limitations=limitations,
            disclaimer=self._DISCLAIMER,
        )

    @staticmethod
    def _summary_block(
        decision: ClassificationDecision,
        classification: str | None,
    ) -> ExplanationBlock:
        if decision.conflict is not None:
            conflict = decision.conflict
            criteria = tuple(code.value for code in conflict.criterion_codes)
            sources = tuple(conflict.source_ids)
            details = []
            if criteria:
                details.append(f"criteria={', '.join(criteria)}")
            if sources:
                details.append(f"sources={', '.join(sources)}")
            suffix = f" ({'; '.join(details)})" if details else ""
            return ExplanationBlock(
                kind="summary",
                title="Conflict",
                text=f"Conflict prevents a five-tier classification{suffix}.",
                criterion_codes=criteria,
            )
        assert classification is not None
        return ExplanationBlock(
            kind="summary",
            title="Classification",
            text=(
                f"Classification: {classification}. Rule: {decision.matched_rule_id}."
            ),
        )

    @staticmethod
    def _criterion_block(assessment: object) -> ExplanationBlock:
        from acmg_classifier.domain.criteria import CriterionAssessment

        if not isinstance(assessment, CriterionAssessment):
            raise TypeError("decision assessment must be a CriterionAssessment")
        rationale = _render_template(
            assessment.rationale_template,
            assessment.rationale_values,
        )
        status = _status_label(assessment.status)
        strength = (
            f"; applied strength={assessment.applied_strength.value}"
            if assessment.applied_strength is not None
            else ""
        )
        return ExplanationBlock(
            kind="criterion",
            title=assessment.code.value,
            text=f"{assessment.code.value}: {status}{strength}. {rationale}",
            criterion_code=assessment.code.value,
            criterion_codes=(assessment.code.value,),
            evidence_ids=tuple(assessment.evidence_ids),
        )

    @staticmethod
    def _comparison_blocks(assessment: object) -> tuple[ExplanationBlock, ...]:
        from acmg_classifier.domain.criteria import CriterionAssessment

        if not isinstance(assessment, CriterionAssessment):
            raise TypeError("decision assessment must be a CriterionAssessment")
        return tuple(
            ExplanationBlock(
                kind="comparison",
                title=f"{assessment.code.value} comparison",
                text=(
                    f"{comparison.input_name} {comparison.operator} "
                    f"{_display_value(comparison.expected)}; observed "
                    f"{_display_value(comparison.observed)}; "
                    f"matched={str(comparison.matched).lower()}."
                ),
                criterion_code=assessment.code.value,
                criterion_codes=(assessment.code.value,),
                evidence_ids=tuple(assessment.evidence_ids),
            )
            for comparison in assessment.comparisons
        )


def _render_template(template: str, values: Mapping[str, JsonValue]) -> str:
    formatter = string.Formatter()
    rendered_values: dict[str, str] = {}
    try:
        parsed = tuple(formatter.parse(template))
    except ValueError as error:
        raise ExplanationTemplateError("invalid rationale template") from error
    for _, field_name, format_spec, conversion in parsed:
        if field_name is None:
            continue
        if (
            not re.fullmatch(r"[A-Za-z_][A-Za-z0-9_]*", field_name)
            or format_spec
            or conversion is not None
            or field_name not in values
        ):
            raise ExplanationTemplateError("unsafe or unavailable rationale field")
        rendered_values[field_name] = _display_value(values[field_name])
    try:
        return template.format_map(rendered_values)
    except (KeyError, ValueError) as error:
        raise ExplanationTemplateError("invalid rationale template") from error


def _display_value(value: JsonValue) -> str:
    if isinstance(value, str):
        return value
    return canonical_json_bytes(value).decode("utf-8")


def _classification_label(value: ClassificationTier | None) -> str | None:
    if value is None:
        return None
    return {
        ClassificationTier.PATHOGENIC: "Pathogenic",
        ClassificationTier.LIKELY_PATHOGENIC: "Likely Pathogenic",
        ClassificationTier.UNCERTAIN_SIGNIFICANCE: "Uncertain Significance",
        ClassificationTier.LIKELY_BENIGN: "Likely Benign",
        ClassificationTier.BENIGN: "Benign",
    }[value]


def _status_label(status: CriterionStatus) -> str:
    return {
        CriterionStatus.APPLIED: "applied",
        CriterionStatus.NOT_APPLIED: "not applied",
        CriterionStatus.NOT_EVALUABLE: "not evaluable",
        CriterionStatus.DISABLED: "disabled",
    }[status]
