"""Immutable core request and interpretation context values."""

from dataclasses import dataclass

from acmg_classifier.domain.enums import GenomeBuild, InheritanceMode


@dataclass(frozen=True, slots=True)
class VariantInput:
    """The scientist's original variant expression."""

    value: str


@dataclass(frozen=True, slots=True)
class InterpretationContext:
    """Context supplied with a classification request."""

    genome_build: GenomeBuild | None = None
    transcript: str | None = None
    disease_id: str | None = None
    disease_label: str | None = None
    inheritance: InheritanceMode | None = None
