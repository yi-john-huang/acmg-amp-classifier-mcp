"""Provider-neutral normalization boundary."""

from __future__ import annotations

from collections.abc import Mapping
from dataclasses import dataclass
from dataclasses import field as dataclass_field
from typing import Literal, Protocol

from acmg_classifier.domain.enums import GenomeBuild
from acmg_classifier.domain.errors import JsonValue
from acmg_classifier.domain.models import InterpretationContext
from acmg_classifier.domain.normalization import (
    CanonicalAlleleKey,
    NormalizationFailureCode,
    NormalizedVariant,
    ParsedVariant,
)


@dataclass(frozen=True, slots=True)
class NormalizationPolicy:
    """Controls which normalization providers may be invoked."""

    mode: Literal["live", "offline"] = "live"
    allow_remote: bool = True
    require_provider_agreement: bool = True
    genome_build: GenomeBuild | None = None


@dataclass(frozen=True, slots=True)
class NormalizationSuccess:
    """A provider-normalized allele, optionally verified by a round trip."""

    normalized: NormalizedVariant
    round_trip_key: CanonicalAlleleKey | None = None
    status: Literal["success"] = "success"


@dataclass(frozen=True, slots=True)
class NormalizationFailure:
    """A typed provider or normalization failure safe for application callers."""

    code: NormalizationFailureCode
    message: str
    retryable: bool
    provider_id: str | None = None
    field: str | None = None
    details: Mapping[str, JsonValue] = dataclass_field(default_factory=dict)
    status: Literal["failure"] = "failure"

    def __post_init__(self) -> None:
        object.__setattr__(self, "details", dict(self.details))


type NormalizationProviderResult = NormalizationSuccess | NormalizationFailure


class NormalizationProvider(Protocol):
    """A source of reference-validated, canonical alleles."""

    provider_id: str
    capabilities: frozenset[Literal["live", "offline"]]

    def normalize(
        self,
        parsed: ParsedVariant,
        context: InterpretationContext,
        policy: NormalizationPolicy,
    ) -> NormalizationProviderResult:
        """Normalize a parsed input or return a typed diagnostic."""
