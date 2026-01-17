---
name: classify
description: Full ACMG/AMP variant classification workflow. Validates input, gathers evidence, applies rules, and generates classification. Invoked via /classify <variant>.
---

# Variant Classification Workflow

Perform a complete ACMG/AMP variant classification using the MCP server tools.

## Usage

```
/classify NM_000492.3:c.1521_1523delCTT
/classify BRCA1:c.5266dupC
/classify TP53 p.R273H
/classify chr17:g.43094692G>A --report
```

## Supported Input Formats

| Format | Example | Description |
|--------|---------|-------------|
| HGVS (coding) | `NM_000492.3:c.1521_1523delCTT` | Transcript with coding change |
| Gene symbol | `BRCA1:c.5266dupC` | Gene symbol with variant |
| Protein change | `TP53 p.R273H` | Gene with protein notation |
| Genomic | `chr17:g.43094692G>A` | Chromosomal coordinates |

## Workflow Steps

### Step 1: Input Validation

Use the `validate_hgvs` MCP tool to validate and enrich the input:

```
Tool: validate_hgvs
Parameters:
  - hgvs_notation: <user input>
  - strict_mode: false
```

**Enhanced output includes:**
- `is_valid`: Validation result
- `normalized_hgvs`: Standardized notation
- `gene_info`: Gene symbol, name, and HGNC ID
- `transcript_info`: RefSeq, Ensembl IDs, canonical status
- `suggestions`: Fix suggestions for invalid input

**If validation fails:**
1. Report the specific validation error
2. Present suggestions from the tool response
3. Reference `.claude/skills/_shared/error-handling.md` for common issues

**If validation succeeds:**
- Use `normalized_hgvs` for subsequent steps
- Display `gene_info` and `transcript_info` to user
- Proceed to evidence gathering

### Step 2: Evidence Gathering

Use the `query_evidence` MCP tool to gather comprehensive evidence:

```
Tool: query_evidence
Parameters:
  - hgvs_notation: <normalized HGVS>
  - databases: ["clinvar", "gnomad", "cosmic", "pubmed"]
```

**Enhanced output includes:**
- `database_results`: Raw data from each source
- `source_quality`: Per-source quality assessment (high/medium/low)
- `quality_scores`: Overall evidence quality with completeness score
- `acmg_criteria_hints`: Pre-mapped ACMG criteria suggestions
- `synthesis`: Human-readable evidence summary

**Present evidence summary:**
- Use the `synthesis` field for a quick overview
- Highlight `acmg_criteria_hints` that apply
- Note data quality from `source_quality`

### Step 3: Variant Classification

Use the `classify_variant` MCP tool for full classification:

```
Tool: classify_variant
Parameters:
  - hgvs_notation: <normalized HGVS> (or)
  - gene_symbol_notation: <gene:variant>
  - clinical_context: <if provided by user>
```

**Classification analysis:**
1. Review each ACMG/AMP rule applied
2. Cross-reference with `acmg_criteria_hints` from evidence
3. Explain evidence combination logic
4. Report final classification with confidence

### Step 4: Results Summary

Present results in this format:

```markdown
## Classification Result

**Variant:** [Normalized HGVS notation]
**Gene:** [Gene symbol] ([Transcript]) - from validate_hgvs gene_info
**Classification:** [Pathogenic/Likely Pathogenic/VUS/Likely Benign/Benign]
**Confidence:** [High/Medium/Low]

### Evidence Summary

[Include synthesis text from query_evidence]

| Criterion | Met | Strength | Evidence |
|-----------|-----|----------|----------|
| PVS1 | Yes/No | Very Strong | [Brief explanation] |
| PS1 | Yes/No | Strong | [Brief explanation] |
| ... | ... | ... | ... |

### Key Findings

- **Population Frequency:** [from query_evidence]
- **Clinical Assertions:** [from query_evidence]
- **Functional Impact:** [Computational predictions]

### Classification Rationale

[Explanation of how evidence was combined to reach classification]
```

### Step 5 (Optional): Report Generation

If `--report` flag is provided, generate a clinical report:

```
Tool: generate_report
Parameters:
  - variant: <normalized HGVS>
  - template: "clinical"
  - include_evidence: true
```

## Options

| Flag | Description |
|------|-------------|
| `--report` | Generate clinical report after classification |
| `--context "text"` | Provide clinical context for targeted analysis |
| `--format json` | Output structured JSON instead of markdown |

## Error Handling

Refer to `.claude/skills/_shared/error-handling.md` for:
- Input validation errors
- Database connection issues
- Insufficient evidence scenarios

## Clinical Guidelines

Refer to `.claude/skills/_shared/acmg-guidelines.md` for:
- ACMG/AMP rule definitions
- Evidence combination tables
- Classification thresholds

## Examples

### Example 1: CFTR Variant

```
User: /classify NM_000492.3:c.1521_1523delCTT

Claude:
## Classification Result

**Variant:** NM_000492.3:c.1521_1523delCTT (p.Phe508del)
**Gene:** CFTR (CF transmembrane conductance regulator)
**Transcript:** NM_000492.3 (canonical)
**Classification:** Pathogenic
**Confidence:** High

### Evidence Summary

Population frequency: absent/very rare - supports pathogenicity.
Clinical significance: Pathogenic with expert panel review.
Evidence quality: Excellent (95% complete).

| Criterion | Met | Strength | Evidence |
|-----------|-----|----------|----------|
| PS1 | Yes | Strong | Well-established pathogenic variant |
| PS4 | Yes | Strong | Significantly increased in CF patients |
| PP5 | Yes | Supporting | Multiple ClinVar pathogenic assertions |

### Classification Rationale

This variant meets criteria for Pathogenic classification based on:
2 Strong criteria (PS1, PS4) + 1 Supporting (PP5)
```

### Example 2: Novel VUS

```
User: /classify BRCA1:c.1234A>G

Claude:
## Classification Result

**Variant:** NM_007294.4:c.1234A>G (p.Asn412Asp)
**Gene:** BRCA1 (BRCA1 DNA repair associated)
**Classification:** Uncertain Significance (VUS)
**Confidence:** Medium

### Evidence Summary

Population frequency: absent from gnomAD.
Computational prediction: damaging (score: 0.72).
Evidence quality: Moderate (65% complete).

| Criterion | Met | Strength | Evidence |
|-----------|-----|----------|----------|
| PM2 | Yes | Moderate | Absent from gnomAD |
| PP3 | Yes | Supporting | REVEL score 0.72 (damaging) |

### Classification Rationale

This variant is classified as VUS due to:
- Only 1 Moderate (PM2) + 1 Supporting (PP3) criteria met
- Insufficient evidence to reach Likely Pathogenic threshold
```

## MCP Tools Used

| Tool | Purpose |
|------|---------|
| `validate_hgvs` | Validate and enrich input notation |
| `query_evidence` | Gather evidence with quality assessment |
| `classify_variant` | Apply ACMG/AMP classification rules |
| `generate_report` | Generate clinical report (optional) |

## References

- `.claude/skills/_shared/acmg-guidelines.md` - ACMG/AMP criteria reference
- `.claude/skills/_shared/error-handling.md` - Error handling guide
- `.claude/skills/_shared/clinical-context.md` - Clinical interpretation guidance
