"""Upstream-format parsers kept separate from runtime table generation."""

import csv
import gzip
import io
import re
from dataclasses import dataclass
from pathlib import Path

HGNC = re.compile(r"^HGNC:\d+$")
GENE_ID = re.compile(r"^GeneID:\d+$")
MONDO = re.compile(r"^MONDO:\d+$")
REFSEQ_TRANSCRIPT = re.compile(r"^(?:NM|NR)_\d+\.\d+$")
ENSEMBL_TRANSCRIPT = re.compile(r"^ENST\d+\.\d+$")
GENOMIC_ACCESSION = re.compile(r"^(?:NC|NW|NT)_\d+\.\d+$")
GENE_SYMBOL = re.compile(r"^[A-Za-z0-9@._-]+$")


class SourceFormatError(ValueError):
    """An official source no longer satisfies its locked schema."""


@dataclass(frozen=True, slots=True)
class TranscriptMapping:
    gene_id: str
    hgnc_id: str
    gene_symbol: str
    refseq_transcript: str
    ensembl_transcript: str
    mane_status: str
    genomic_accession: str
    start: int
    end: int
    strand: str


@dataclass(frozen=True, slots=True)
class GeneDisease:
    gene_symbol: str
    hgnc_id: str
    disease_label: str
    mondo_id: str
    mode_of_inheritance: str
    sop: str
    classification: str
    report_url: str
    classification_date: str
    expert_panel: str


def _decoded_source(path: Path) -> str:
    content = path.read_bytes()
    if content.startswith(b"\x1f\x8b"):
        try:
            content = gzip.decompress(content)
        except OSError as error:
            raise SourceFormatError(f"invalid gzip source: {path.name}") from error
    try:
        return content.decode("utf-8-sig")
    except UnicodeDecodeError as error:
        raise SourceFormatError(f"source is not UTF-8: {path.name}") from error


def parse_mane_summary(path: Path) -> tuple[TranscriptMapping, ...]:
    """Parse the official MANE summary table and validate key identifiers."""
    reader = csv.DictReader(io.StringIO(_decoded_source(path)), delimiter="\t")
    required = {
        "#NCBI_GeneID",
        "HGNC_ID",
        "symbol",
        "RefSeq_nuc",
        "Ensembl_nuc",
        "MANE_status",
        "GRCh38_chr",
        "chr_start",
        "chr_end",
        "chr_strand",
    }
    if reader.fieldnames is None or not required.issubset(reader.fieldnames):
        raise SourceFormatError("MANE summary header does not match the locked schema")
    rows: list[TranscriptMapping] = []
    for line_number, row in enumerate(reader, start=2):
        gene_id = row["#NCBI_GeneID"]
        if not GENE_ID.fullmatch(gene_id):
            raise SourceFormatError(f"invalid GeneID on MANE line {line_number}")
        hgnc_id = row["HGNC_ID"]
        # MANE v1.5 contains one LOC record without an assigned HGNC identifier.
        # Runtime tables require stable HGNC identity, so that unsupported row is
        # intentionally excluded while malformed non-empty identifiers fail closed.
        if not hgnc_id:
            continue
        if not HGNC.fullmatch(hgnc_id):
            raise SourceFormatError(
                f"invalid HGNC identifier on MANE line {line_number}"
            )
        gene_symbol = row["symbol"]
        if not GENE_SYMBOL.fullmatch(gene_symbol):
            raise SourceFormatError(f"invalid gene symbol on MANE line {line_number}")
        refseq = row["RefSeq_nuc"]
        ensembl = row["Ensembl_nuc"]
        genomic = row["GRCh38_chr"]
        if not REFSEQ_TRANSCRIPT.fullmatch(refseq):
            raise SourceFormatError(
                f"invalid RefSeq transcript on MANE line {line_number}"
            )
        if not ENSEMBL_TRANSCRIPT.fullmatch(ensembl):
            raise SourceFormatError(
                f"invalid Ensembl transcript on MANE line {line_number}"
            )
        if not GENOMIC_ACCESSION.fullmatch(genomic):
            raise SourceFormatError(
                f"invalid genomic accession on MANE line {line_number}"
            )
        try:
            start, end = int(row["chr_start"]), int(row["chr_end"])
        except ValueError as error:
            raise SourceFormatError(
                f"invalid coordinates on MANE line {line_number}"
            ) from error
        if start < 0 or end < start:
            raise SourceFormatError(f"invalid coordinates on MANE line {line_number}")
        strand = row["chr_strand"]
        if strand not in {"+", "-"}:
            raise SourceFormatError(f"invalid strand on MANE line {line_number}")
        status = row["MANE_status"]
        if status not in {"MANE Select", "MANE Plus Clinical"}:
            raise SourceFormatError(f"invalid MANE status on line {line_number}")
        rows.append(
            TranscriptMapping(
                gene_id=gene_id,
                hgnc_id=hgnc_id,
                gene_symbol=gene_symbol,
                refseq_transcript=refseq,
                ensembl_transcript=ensembl,
                mane_status=status,
                genomic_accession=genomic,
                start=start,
                end=end,
                strand=strand,
            )
        )
    if not rows:
        raise SourceFormatError("MANE summary contains no records")
    return tuple(rows)


def parse_clingen_gene_disease(path: Path) -> tuple[GeneDisease, ...]:
    """Parse the official ClinGen gene-disease CSV after its metadata preamble."""
    all_rows = list(csv.reader(io.StringIO(_decoded_source(path))))
    header_index = next(
        (
            index
            for index, row in enumerate(all_rows)
            if row and row[0] == "GENE SYMBOL"
        ),
        None,
    )
    if header_index is None:
        raise SourceFormatError("ClinGen gene-disease header is missing")
    header = all_rows[header_index]
    required = {
        "GENE SYMBOL",
        "GENE ID (HGNC)",
        "DISEASE LABEL",
        "DISEASE ID (MONDO)",
        "MOI",
        "SOP",
        "CLASSIFICATION",
        "ONLINE REPORT",
        "CLASSIFICATION DATE",
        "GCEP",
    }
    if not required.issubset(header):
        raise SourceFormatError("ClinGen gene-disease header changed")
    parsed: list[GeneDisease] = []
    for line_number, values in enumerate(
        all_rows[header_index + 1 :], header_index + 2
    ):
        if not values or not any(values):
            continue
        if values[0].startswith("+++"):
            continue
        if len(values) != len(header):
            raise SourceFormatError(f"malformed ClinGen CSV line {line_number}")
        row = dict(zip(header, values, strict=True))
        hgnc_id = row["GENE ID (HGNC)"]
        mondo_id = row["DISEASE ID (MONDO)"]
        if not HGNC.fullmatch(hgnc_id):
            raise SourceFormatError(
                f"invalid HGNC identifier on ClinGen line {line_number}"
            )
        if not MONDO.fullmatch(mondo_id):
            raise SourceFormatError(
                f"invalid MONDO identifier on ClinGen line {line_number}"
            )
        gene_symbol = row["GENE SYMBOL"]
        if not GENE_SYMBOL.fullmatch(gene_symbol):
            raise SourceFormatError(
                f"invalid gene symbol on ClinGen line {line_number}"
            )
        report_url = row["ONLINE REPORT"]
        if not report_url.startswith("https://"):
            raise SourceFormatError(f"invalid report URL on ClinGen line {line_number}")
        required_values = (row["DISEASE LABEL"], row["CLASSIFICATION"], row["GCEP"])
        if not all(required_values):
            raise SourceFormatError(f"missing ClinGen value on line {line_number}")
        parsed.append(
            GeneDisease(
                gene_symbol=gene_symbol,
                hgnc_id=hgnc_id,
                disease_label=row["DISEASE LABEL"],
                mondo_id=mondo_id,
                mode_of_inheritance=row["MOI"],
                sop=row["SOP"],
                classification=row["CLASSIFICATION"],
                report_url=report_url,
                classification_date=row["CLASSIFICATION DATE"],
                expert_panel=row["GCEP"],
            )
        )
    if not parsed:
        raise SourceFormatError("ClinGen gene-disease source contains no records")
    return tuple(parsed)
