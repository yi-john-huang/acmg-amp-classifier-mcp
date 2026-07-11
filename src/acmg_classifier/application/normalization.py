"""Application orchestration for parsing and provider-backed normalization."""

from __future__ import annotations

from dataclasses import replace
from typing import Literal

from acmg_classifier.domain.errors import JsonValue
from acmg_classifier.domain.models import InterpretationContext
from acmg_classifier.domain.normalization import (
    HgvsAlias,
    InvalidVariant,
    NormalizationFailureCode,
    NormalizedVariant,
    ParsedVariant,
    UnsupportedVariant,
    VariantInputParser,
    VariantNotation,
    VariantParseResult,
)
from acmg_classifier.ports.normalization import (
    NormalizationFailure,
    NormalizationPolicy,
    NormalizationProvider,
    NormalizationProviderResult,
    NormalizationSuccess,
)


class VariantNormalizationService:
    """Parse input purely, then aggregate canonical results from injected providers."""

    def __init__(
        self,
        parser: VariantInputParser | None = None,
        providers: tuple[NormalizationProvider, ...] = (),
    ) -> None:
        self._parser = parser or VariantInputParser()
        self._providers = tuple(providers)

    def parse(self, value: str) -> VariantParseResult:
        """Return a typed supported, unsupported, or invalid parse outcome."""
        return self._parser.parse(value)

    def normalize(
        self,
        value: str | ParsedVariant,
        *,
        context: InterpretationContext | None = None,
        policy: NormalizationPolicy | None = None,
    ) -> NormalizationProviderResult:
        """Return a provider-validated canonical allele or a typed failure."""
        parsed = value if isinstance(value, ParsedVariant) else self.parse(value)
        if isinstance(parsed, InvalidVariant):
            return NormalizationFailure(
                NormalizationFailureCode.INVALID_VARIANT_SYNTAX,
                parsed.message,
                False,
                field="variant",
                details={"reason": parsed.reason},
            )
        if isinstance(parsed, UnsupportedVariant):
            return NormalizationFailure(
                NormalizationFailureCode.UNSUPPORTED_VARIANT_SCOPE,
                parsed.message,
                False,
                field="variant",
                details={"reason": parsed.reason},
            )

        resolved_context = context or InterpretationContext()
        resolved_policy = policy or NormalizationPolicy()
        failures: list[NormalizationFailure] = []
        successes: list[tuple[str, NormalizationSuccess]] = []
        for provider in self._eligible_providers(resolved_policy):
            provider_id = provider.provider_id
            try:
                result = provider.normalize(parsed, resolved_context, resolved_policy)
            except Exception:
                failures.append(
                    NormalizationFailure(
                        NormalizationFailureCode.PROVIDER_OUTAGE,
                        "normalization provider failed unexpectedly",
                        True,
                        provider_id,
                    )
                )
                continue
            if isinstance(result, NormalizationFailure):
                failures.append(
                    result
                    if result.provider_id is not None
                    else replace(result, provider_id=provider_id)
                )
                continue
            if not isinstance(result, NormalizationSuccess):
                failures.append(
                    NormalizationFailure(
                        NormalizationFailureCode.PROVIDER_SCHEMA_DRIFT,
                        "normalization provider returned an invalid result",
                        False,
                        provider_id,
                    )
                )
                continue
            if (
                result.round_trip_key is not None
                and result.round_trip_key != result.normalized.canonical_key
            ):
                failures.append(
                    NormalizationFailure(
                        NormalizationFailureCode.ROUND_TRIP_MISMATCH,
                        (
                            "provider round-trip allele does not match its "
                            "canonical allele"
                        ),
                        False,
                        provider_id,
                        details={
                            "canonical_key": result.normalized.canonical_key.value,
                            "round_trip_key": result.round_trip_key.value,
                        },
                    )
                )
                continue
            successes.append((provider_id, result))

        terminal = self._terminal_failure(failures)
        if terminal is not None:
            return terminal
        if not successes:
            return NormalizationFailure(
                NormalizationFailureCode.NORMALIZATION_UNAVAILABLE,
                "no normalization provider returned a canonical allele",
                any(failure.retryable for failure in failures),
                details={"failures": self._failure_diagnostics(failures)},
            )
        if not self._agree_on_allele(successes):
            provider_ids: list[JsonValue] = []
            provider_ids.extend(sorted(provider_id for provider_id, _ in successes))
            canonical_keys: list[JsonValue] = []
            canonical_keys.extend(
                sorted(
                    {success.normalized.canonical_key.value for _, success in successes}
                )
            )
            details: dict[str, JsonValue] = {
                "providers": provider_ids,
                "canonical_keys": canonical_keys,
            }
            return NormalizationFailure(
                NormalizationFailureCode.NORMALIZATION_CONFLICT,
                "successful normalization providers disagree on the canonical allele",
                False,
                details=details,
            )
        return NormalizationSuccess(self._merge_successes(parsed, successes))

    def _eligible_providers(
        self, policy: NormalizationPolicy
    ) -> tuple[NormalizationProvider, ...]:
        eligible: list[NormalizationProvider] = []
        for provider in self._providers:
            capabilities = getattr(
                provider, "capabilities", frozenset({"live", "offline"})
            )
            if policy.mode == "offline" or not policy.allow_remote:
                if "offline" not in capabilities:
                    continue
            elif "live" not in capabilities:
                continue
            eligible.append(provider)
        return tuple(eligible)

    @staticmethod
    def _terminal_failure(
        failures: list[NormalizationFailure],
    ) -> NormalizationFailure | None:
        terminal_codes = {
            NormalizationFailureCode.REFERENCE_MISMATCH,
            NormalizationFailureCode.ROUND_TRIP_MISMATCH,
        }
        terminal = [failure for failure in failures if failure.code in terminal_codes]
        if not terminal:
            return None
        return min(
            terminal,
            key=lambda failure: (
                failure.code.value,
                failure.provider_id or "",
                failure.message,
            ),
        )

    @staticmethod
    def _failure_diagnostics(
        failures: list[NormalizationFailure],
    ) -> list[JsonValue]:
        diagnostics: list[JsonValue] = []
        for failure in sorted(
            failures,
            key=lambda failure: (
                failure.provider_id or "",
                failure.code.value,
                failure.message,
            ),
        ):
            diagnostics.append(
                {
                    "code": failure.code.value,
                    "provider_id": failure.provider_id,
                    "retryable": failure.retryable,
                }
            )
        return diagnostics

    @staticmethod
    def _agree_on_allele(
        successes: list[tuple[str, NormalizationSuccess]],
    ) -> bool:
        normalized = [success.normalized for _, success in successes]
        if len({candidate.canonical_key for candidate in normalized}) != 1:
            return False
        return (
            len(
                {
                    candidate.transcript
                    for candidate in normalized
                    if candidate.transcript is not None
                }
            )
            <= 1
        )

    @staticmethod
    def _merge_successes(
        parsed: ParsedVariant,
        successes: list[tuple[str, NormalizationSuccess]],
    ) -> NormalizedVariant:
        normalized = [success.normalized for _, success in successes]
        alias_notations: dict[VariantNotation, Literal["genomic", "transcript"]] = {
            VariantNotation.GENOMIC_HGVS: "genomic",
            VariantNotation.TRANSCRIPT_HGVS: "transcript",
            VariantNotation.GENE_VARIANT: "transcript",
        }
        aliases = {
            HgvsAlias(
                parsed.normalized_input,
                alias_notations[parsed.notation],
                parsed.accession,
                "input",
            )
        }
        aliases.update(
            alias for candidate in normalized for alias in candidate.hgvs_aliases
        )
        ordered_aliases = tuple(
            sorted(
                aliases,
                key=lambda alias: (
                    alias.expression,
                    alias.notation,
                    alias.accession or "",
                    alias.source or "",
                ),
            )
        )
        provenance = tuple(
            item for candidate in normalized for item in candidate.provider_provenance
        )
        gene_symbols = sorted(
            {candidate.gene_symbol for candidate in normalized if candidate.gene_symbol}
        )
        transcripts = sorted(
            {candidate.transcript for candidate in normalized if candidate.transcript}
        )
        transcript_hgvs = sorted(
            {
                candidate.transcript_hgvs
                for candidate in normalized
                if candidate.transcript_hgvs
            }
        )
        preferred_hgvs = min(
            ordered_aliases,
            key=lambda alias: (
                alias.notation != "genomic",
                alias.expression,
                alias.accession or "",
            ),
        ).expression
        first = normalized[0]
        return NormalizedVariant(
            original_input=parsed.original_input,
            parsed_input=parsed.normalized_input,
            canonical_key=first.canonical_key,
            genome_build=first.genome_build,
            genomic_accession=first.genomic_accession,
            genomic_start=first.genomic_start,
            genomic_end=first.genomic_end,
            reference_allele=first.reference_allele,
            alternate_allele=first.alternate_allele,
            normalized_hgvs=preferred_hgvs,
            hgvs_aliases=ordered_aliases,
            gene_symbol=gene_symbols[0] if gene_symbols else None,
            transcript=transcripts[0] if transcripts else None,
            transcript_hgvs=transcript_hgvs[0] if transcript_hgvs else None,
            provider_provenance=provenance,
        )
