"""Pure parsing and provider-neutral canonical allele values for variant input."""

from __future__ import annotations

import re
from dataclasses import dataclass
from datetime import datetime
from enum import StrEnum
from typing import Literal

from acmg_classifier.domain.enums import GenomeBuild
from acmg_classifier.domain.errors import JsonValue


class ParseScope(StrEnum):
    """The parser's typed outcome."""

    SUPPORTED = "supported"
    UNSUPPORTED = "unsupported"
    INVALID = "invalid"


class VariantNotation(StrEnum):
    """Input notation recognized by the release-one parser."""

    TRANSCRIPT_HGVS = "transcript_hgvs"
    GENOMIC_HGVS = "genomic_hgvs"
    GENE_VARIANT = "gene_variant"


class VariantKind(StrEnum):
    """Small sequence variant forms accepted by this parser."""

    SUBSTITUTION = "substitution"
    DELETION = "deletion"
    INSERTION = "insertion"
    DUPLICATION = "duplication"
    FRAMESHIFT = "frameshift"


@dataclass(frozen=True, slots=True)
class ParsedVariant:
    """A syntactically valid, in-scope input before reference resolution."""

    original_input: str
    normalized_input: str
    notation: VariantNotation
    kind: VariantKind
    accession: str | None = None
    gene_symbol: str | None = None
    genome_build: None = None
    requires_context: bool = False
    scope: ParseScope = ParseScope.SUPPORTED
    reason: None = None


@dataclass(frozen=True, slots=True)
class UnsupportedVariant:
    """An explicit release-scope rejection, safe to show at the boundary."""

    original_input: str
    reason: str
    message: str
    scope: ParseScope = ParseScope.UNSUPPORTED


@dataclass(frozen=True, slots=True)
class InvalidVariant:
    """A non-empty input that is not valid supported notation."""

    original_input: str
    reason: str
    message: str
    scope: ParseScope = ParseScope.INVALID


type VariantParseResult = ParsedVariant | UnsupportedVariant | InvalidVariant


class NormalizationFailureCode(StrEnum):
    """Stable failure vocabulary for canonical allele normalization."""

    INVALID_VARIANT_SYNTAX = "INVALID_VARIANT_SYNTAX"
    UNSUPPORTED_VARIANT_SCOPE = "UNSUPPORTED_VARIANT_SCOPE"
    GENOME_BUILD_REQUIRED = "GENOME_BUILD_REQUIRED"
    REFERENCE_MISMATCH = "REFERENCE_MISMATCH"
    ALLELE_NORMALIZATION_FAILED = "ALLELE_NORMALIZATION_FAILED"
    ROUND_TRIP_MISMATCH = "ROUND_TRIP_MISMATCH"
    NORMALIZATION_UNAVAILABLE = "NORMALIZATION_UNAVAILABLE"
    NORMALIZATION_CONFLICT = "NORMALIZATION_CONFLICT"
    PROVIDER_SCHEMA_DRIFT = "PROVIDER_SCHEMA_DRIFT"
    PROVIDER_TIMEOUT = "PROVIDER_TIMEOUT"
    PROVIDER_RATE_LIMITED = "PROVIDER_RATE_LIMITED"
    PROVIDER_OUTAGE = "PROVIDER_OUTAGE"


_VERSIONED_SEQUENCE_ACCESSION = re.compile(r"^(?:NC|NG|NT|NW)_\d+\.\d+$")
_ALLELE_SEQUENCE = re.compile(r"^[ACGT]*$")


@dataclass(frozen=True, slots=True)
class CanonicalAlleleKey:
    """A versioned, SPDI-like domain key using 0-based interbase coordinates."""

    assembly: GenomeBuild
    sequence_accession: str
    start: int
    end: int
    deleted_sequence: str
    inserted_sequence: str
    schema_version: Literal["1.0"] = "1.0"

    def __post_init__(self) -> None:
        if not isinstance(self.assembly, GenomeBuild):
            raise ValueError("assembly must be a supported GenomeBuild")
        if self.schema_version != "1.0":
            raise ValueError("unsupported canonical allele schema version")
        if not isinstance(self.sequence_accession, str):
            raise ValueError("sequence accession must be a string")
        if (
            not isinstance(self.start, int)
            or isinstance(self.start, bool)
            or not isinstance(self.end, int)
            or isinstance(self.end, bool)
        ):
            raise ValueError("coordinates must be integers")
        if not isinstance(self.deleted_sequence, str) or not isinstance(
            self.inserted_sequence, str
        ):
            raise ValueError("allele sequences must be strings")
        if not _VERSIONED_SEQUENCE_ACCESSION.fullmatch(self.sequence_accession):
            raise ValueError("sequence accession must be a versioned genomic accession")
        if self.start < 0 or self.end < self.start:
            raise ValueError("coordinates must satisfy 0 <= start <= end")
        if not _ALLELE_SEQUENCE.fullmatch(self.deleted_sequence):
            raise ValueError("deleted sequence must contain uppercase A, C, G, or T")
        if not _ALLELE_SEQUENCE.fullmatch(self.inserted_sequence):
            raise ValueError("inserted sequence must contain uppercase A, C, G, or T")
        if self.end - self.start != len(self.deleted_sequence):
            raise ValueError("deleted sequence length must equal coordinate span")
        if not self.deleted_sequence and self.start != self.end:
            raise ValueError("an insertion must use an empty interbase interval")
        if not self.deleted_sequence and not self.inserted_sequence:
            raise ValueError("canonical allele cannot be a no-op")

    @property
    def value(self) -> str:
        """Return the stable external-safe string form of this domain key."""
        return (
            f"cak1:{self.assembly}:{self.sequence_accession}:{self.start}:"
            f"{self.deleted_sequence}>{self.inserted_sequence}"
        )

    def __str__(self) -> str:
        return self.value

    def to_canonical_content(self) -> dict[str, JsonValue]:
        """Return the primitive content used for deterministic serialization."""
        return {
            "schema_version": self.schema_version,
            "assembly": self.assembly.value,
            "sequence_accession": self.sequence_accession,
            "start": self.start,
            "end": self.end,
            "deleted_sequence": self.deleted_sequence,
            "inserted_sequence": self.inserted_sequence,
        }


@dataclass(frozen=True, slots=True)
class HgvsAlias:
    """A source-preserved HGVS representation of a canonical allele."""

    expression: str
    notation: Literal["genomic", "transcript", "protein"]
    accession: str | None = None
    source: str | None = None

    def __post_init__(self) -> None:
        if not self.expression:
            raise ValueError("HGVS alias expression must not be empty")
        if self.notation not in {"genomic", "transcript", "protein"}:
            raise ValueError("HGVS alias notation is unsupported")


@dataclass(frozen=True, slots=True)
class ProviderProvenance:
    """The reproducible source record for a normalization provider result."""

    provider_id: str
    provider_version: str | None
    source_record_id: str | None
    retrieved_at: datetime | None
    bundle_version: str | None
    raw_snapshot_ref: str | None
    query_key: str

    def __post_init__(self) -> None:
        if not self.provider_id:
            raise ValueError("provider provenance requires a provider ID")
        if not self.query_key:
            raise ValueError("provider provenance requires a query key")
        if self.retrieved_at is not None and (
            self.retrieved_at.tzinfo is None or self.retrieved_at.utcoffset() is None
        ):
            raise ValueError("provider provenance timestamp must be timezone-aware")

    def sort_key(self) -> tuple[str, ...]:
        """Provide a provider-order-independent deterministic sort order."""
        return (
            self.provider_id,
            self.provider_version or "",
            self.source_record_id or "",
            self.retrieved_at.isoformat() if self.retrieved_at else "",
            self.bundle_version or "",
            self.raw_snapshot_ref or "",
            self.query_key,
        )


@dataclass(frozen=True, slots=True)
class NormalizedVariant:
    """A reference-validated canonical allele with its display aliases."""

    original_input: str
    parsed_input: str
    canonical_key: CanonicalAlleleKey
    genome_build: GenomeBuild
    genomic_accession: str
    genomic_start: int
    genomic_end: int
    reference_allele: str
    alternate_allele: str
    normalized_hgvs: str
    hgvs_aliases: tuple[HgvsAlias, ...]
    gene_symbol: str | None
    transcript: str | None
    transcript_hgvs: str | None
    provider_provenance: tuple[ProviderProvenance, ...]

    def __post_init__(self) -> None:
        if not isinstance(self.canonical_key, CanonicalAlleleKey):
            raise ValueError("normalized variant requires a canonical allele key")
        if not isinstance(self.genome_build, GenomeBuild):
            raise ValueError("normalized variant requires a supported GenomeBuild")
        key = self.canonical_key
        if (
            self.genome_build != key.assembly
            or self.genomic_accession != key.sequence_accession
            or self.genomic_start != key.start
            or self.genomic_end != key.end
            or self.reference_allele != key.deleted_sequence
            or self.alternate_allele != key.inserted_sequence
        ):
            raise ValueError(
                "normalized variant fields must duplicate its canonical key"
            )
        if not self.original_input or not self.parsed_input or not self.normalized_hgvs:
            raise ValueError(
                "normalized variant input and HGVS fields must not be empty"
            )
        if not self.hgvs_aliases:
            raise ValueError("normalized variant requires at least one HGVS alias")
        if not self.provider_provenance:
            raise ValueError("normalized variant requires provider provenance")
        object.__setattr__(
            self,
            "hgvs_aliases",
            tuple(
                sorted(
                    set(self.hgvs_aliases),
                    key=lambda alias: (
                        alias.expression,
                        alias.notation,
                        alias.accession or "",
                        alias.source or "",
                    ),
                )
            ),
        )
        object.__setattr__(
            self,
            "provider_provenance",
            tuple(
                sorted(set(self.provider_provenance), key=ProviderProvenance.sort_key)
            ),
        )

    def to_canonical_content(self) -> dict[str, JsonValue]:
        """Return deterministic content, including aliases and provenance."""
        return {
            "original_input": self.original_input,
            "parsed_input": self.parsed_input,
            "canonical_key": self.canonical_key.to_canonical_content(),
            "genome_build": self.genome_build.value,
            "genomic_accession": self.genomic_accession,
            "genomic_start": self.genomic_start,
            "genomic_end": self.genomic_end,
            "reference_allele": self.reference_allele,
            "alternate_allele": self.alternate_allele,
            "normalized_hgvs": self.normalized_hgvs,
            "hgvs_aliases": [
                {
                    "expression": alias.expression,
                    "notation": alias.notation,
                    "accession": alias.accession,
                    "source": alias.source,
                }
                for alias in self.hgvs_aliases
            ],
            "gene_symbol": self.gene_symbol,
            "transcript": self.transcript,
            "transcript_hgvs": self.transcript_hgvs,
            "provider_provenance": [
                {
                    "provider_id": item.provider_id,
                    "provider_version": item.provider_version,
                    "source_record_id": item.source_record_id,
                    "retrieved_at": (
                        item.retrieved_at.isoformat()
                        if item.retrieved_at is not None
                        else None
                    ),
                    "bundle_version": item.bundle_version,
                    "raw_snapshot_ref": item.raw_snapshot_ref,
                    "query_key": item.query_key,
                }
                for item in self.provider_provenance
            ],
        }


_TRANSCRIPT = r"(?P<accession>(?:NM|NR)_\d+\.\d+|ENST\d+\.\d+)"
_GENOMIC = r"(?P<accession>(?:NC|NG|NT|NW)_\d+\.\d+|chr(?:[0-9]{1,2}|X|Y|M))"
_COORD = r"[0-9]+(?:[+-][0-9]+)?"
_CODING_SUBSTITUTION = re.compile(
    rf"^{_TRANSCRIPT}:c\.(?P<change>{_COORD}[ACGT]>[ACGT])$", re.IGNORECASE
)
_CODING_EVENT = re.compile(
    rf"^{_TRANSCRIPT}:c\.(?P<change>{_COORD}(?:_{_COORD})?"
    r"(?P<event>del|ins|dup)(?:[ACGT]+)?(?:fs(?:\*[0-9]+)?)?)$",
    re.IGNORECASE,
)
_CODING_FRAMESHIFT = re.compile(
    rf"^{_TRANSCRIPT}:c\.(?P<change>{_COORD}(?:_{_COORD})?"
    r"(?:del|ins|dup)?[ACGT]*fs(?:\*[0-9]+)?)$",
    re.IGNORECASE,
)
_GENOMIC_SUBSTITUTION = re.compile(
    rf"^{_GENOMIC}:g\.(?P<change>[0-9]+[ACGT]>[ACGT])$", re.IGNORECASE
)
_GENOMIC_EVENT = re.compile(
    rf"^{_GENOMIC}:g\.(?P<start>[0-9]+)(?:_(?P<end>[0-9]+))?"
    r"(?P<event>del|ins|dup)(?P<sequence>[ACGT]+)?$",
    re.IGNORECASE,
)
_GENE_VARIANT = re.compile(
    r"^(?P<gene>[A-Za-z][A-Za-z0-9-]{0,14}):c\.(?P<change>.+)$",
    re.IGNORECASE,
)
_PROTEIN_ONLY = re.compile(
    r"^(?:(?:NP|XP)_\d+\.\d+:)?p\..+$|^[A-Za-z][A-Za-z0-9-]{0,14}\s+p\..+$",
    re.IGNORECASE,
)


class VariantInputParser:
    """Recognize supported notation and reject known out-of-scope forms."""

    def parse(self, value: str) -> VariantParseResult:
        original = value
        text = value.strip()
        if not text:
            return InvalidVariant(original, "empty_input", "variant input is empty")

        unsupported = self._scope_rejection(text)
        if unsupported is not None:
            return UnsupportedVariant(original, *unsupported)

        parsed = self._parse_transcript(text)
        if parsed is not None:
            return ParsedVariant(
                original,
                parsed[0],
                VariantNotation.TRANSCRIPT_HGVS,
                parsed[1],
                accession=parsed[2],
            )

        parsed_genomic = self._parse_genomic(text)
        if parsed_genomic is not None:
            return ParsedVariant(
                original,
                parsed_genomic[0],
                VariantNotation.GENOMIC_HGVS,
                parsed_genomic[1],
                accession=parsed_genomic[2],
            )

        gene_match = _GENE_VARIANT.fullmatch(text)
        if gene_match is not None:
            change = gene_match.group("change")
            kind = self._coding_change_kind(change)
            if kind is not None:
                gene = gene_match.group("gene").upper()
                canonical_change = (
                    change.upper()
                    if kind is VariantKind.SUBSTITUTION
                    else self._canonical_event_change(change)
                )
                return ParsedVariant(
                    original,
                    f"{gene}:c.{canonical_change}",
                    VariantNotation.GENE_VARIANT,
                    kind,
                    gene_symbol=gene,
                    requires_context=True,
                )

        if _PROTEIN_ONLY.fullmatch(text) is not None:
            return UnsupportedVariant(
                original,
                "protein_only_ambiguous",
                "protein-only input cannot identify a unique nucleotide allele",
            )

        return InvalidVariant(
            original,
            "malformed_syntax",
            "input is not supported transcript, genomic, or gene-plus-variant HGVS",
        )

    @staticmethod
    def _scope_rejection(value: str) -> tuple[str, str] | None:
        lowered = value.lower()
        if "somatic" in lowered or "mosaic" in lowered:
            return ("somatic_or_mosaic", "somatic and mosaic analysis is out of scope")
        if re.search(r"(?:^|:)chrM:", value, re.IGNORECASE) or re.search(
            r"(?:^|:)NC_012920\.\d+:", value, re.IGNORECASE
        ):
            return ("mitochondrial", "mitochondrial variants are out of scope")
        if re.search(
            r"(?:cnv|sv|bnd|fusion|translocation)|<(?:DEL|DUP|INV)>",
            value,
            re.IGNORECASE,
        ):
            return (
                "copy_number_or_structural",
                "copy-number and structural variants are out of scope",
            )
        if re.search(r":(?:c|g)\.[^:\s]*delins", value, re.IGNORECASE):
            return (
                "delins_not_supported",
                "deletion-insertion variants are out of scope",
            )
        # A very large interval is not a small indel, even when written with
        # ordinary HGVS ``del`` syntax.
        interval = re.search(r"(\d+)_([0-9]+)(?:del|dup|ins)", value, re.IGNORECASE)
        if interval and abs(int(interval.group(2)) - int(interval.group(1))) > 1_000:
            return (
                "copy_number_or_structural",
                "only small insertions and deletions are supported",
            )
        return None

    @staticmethod
    def _coding_change_kind(change: str) -> VariantKind | None:
        if re.fullmatch(
            rf"{_COORD}(?:_{_COORD})?(?:del|ins|dup)?[ACGT]*fs(?:\*[0-9]+)?",
            change,
            re.IGNORECASE,
        ):
            return VariantKind.FRAMESHIFT
        if re.fullmatch(rf"{_COORD}[ACGT]>[ACGT]", change, re.IGNORECASE):
            return VariantKind.SUBSTITUTION
        event = re.fullmatch(
            rf"{_COORD}(?:_{_COORD})?(?P<event>del|ins|dup)[ACGT]*",
            change,
            re.IGNORECASE,
        )
        if event is None:
            return None
        return {
            "del": VariantKind.DELETION,
            "ins": VariantKind.INSERTION,
            "dup": VariantKind.DUPLICATION,
        }[event.group("event").lower()]

    def _parse_transcript(self, value: str) -> tuple[str, VariantKind, str] | None:
        match = _CODING_SUBSTITUTION.fullmatch(value)
        if match:
            accession = match.group("accession").upper()
            change = match.group("change").upper()
            return (f"{accession}:c.{change}", VariantKind.SUBSTITUTION, accession)
        match = _CODING_FRAMESHIFT.fullmatch(value)
        if match:
            accession = match.group("accession").upper()
            change = self._canonical_event_change(match.group("change"))
            return (f"{accession}:c.{change}", VariantKind.FRAMESHIFT, accession)
        match = _CODING_EVENT.fullmatch(value)
        if match:
            accession = match.group("accession").upper()
            change = self._canonical_event_change(match.group("change"))
            event = re.search(r"(del|ins|dup)", change, re.IGNORECASE)
            if event is None:
                return None
            kind = {
                "del": VariantKind.DELETION,
                "ins": VariantKind.INSERTION,
                "dup": VariantKind.DUPLICATION,
            }[event.group(1).lower()]
            return (f"{accession}:c.{change}", kind, accession)
        return None

    @staticmethod
    def _canonical_event_change(change: str) -> str:
        """Uppercase nucleotide payloads while retaining HGVS event keywords."""
        match = re.fullmatch(
            rf"(?P<coordinates>{_COORD}(?:_{_COORD})?)"
            r"(?P<event>del|ins|dup)?(?P<sequence>[ACGT]*)"
            r"(?P<frameshift>fs(?:\*[0-9]+)?)?",
            change,
            re.IGNORECASE,
        )
        if match is None:
            return change
        event = (match.group("event") or "").lower()
        sequence = (match.group("sequence") or "").upper()
        frameshift = (match.group("frameshift") or "").lower()
        return f"{match.group('coordinates')}{event}{sequence}{frameshift}"

    def _parse_genomic(self, value: str) -> tuple[str, VariantKind, str] | None:
        match = _GENOMIC_SUBSTITUTION.fullmatch(value)
        if match:
            accession = self._canonical_accession(match.group("accession"))
            return (
                f"{accession}:g.{match.group('change').upper()}",
                VariantKind.SUBSTITUTION,
                accession,
            )
        match = _GENOMIC_EVENT.fullmatch(value)
        if match:
            accession = self._canonical_accession(match.group("accession"))
            event = match.group("event").lower()
            kind = {
                "del": VariantKind.DELETION,
                "ins": VariantKind.INSERTION,
                "dup": VariantKind.DUPLICATION,
            }[event]
            start = match.group("start")
            end = match.group("end")
            coordinate = f"{start}_{end}" if end else start
            sequence = match.group("sequence") or ""
            return (
                f"{accession}:g.{coordinate}{event}{sequence.upper()}",
                kind,
                accession,
            )
        return None

    @staticmethod
    def _canonical_accession(accession: str) -> str:
        if accession.lower().startswith("chr"):
            return "chr" + accession[3:].upper()
        return accession.upper()
