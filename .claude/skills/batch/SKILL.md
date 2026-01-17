---
name: batch
description: Process multiple variants for classification with progress tracking and summary generation. Invoked via /batch <variants>.
---

# Batch Classification Workflow

Process multiple variants for ACMG/AMP classification with progress tracking and summary generation.

## Usage

```
/batch NM_000492.3:c.1521_1523delCTT, BRCA1:c.5266dupC, TP53:p.R273H

/batch
NM_000492.3:c.1521_1523delCTT
BRCA1:c.5266dupC
TP53:p.R273H
```

## Input Formats

### Comma-Separated

```
/batch variant1, variant2, variant3
```

### Newline-Separated

```
/batch
variant1
variant2
variant3
```

### From File Reference

```
/batch --file variants.txt
```

## Workflow Steps

### Step 1: Parse Input

Extract individual variants from input:
- Split by commas or newlines
- Trim whitespace
- Remove empty entries
- Count total variants

### Step 2: Validate All Variants First

Before processing, validate all variants:

```
For each variant:
  Tool: validate_hgvs
  Parameters:
    - hgvs_notation: <variant>
    - strict_mode: false
```

**Enhanced validation output includes:**
- `is_valid`: Validation result
- `normalized_hgvs`: Standardized notation for processing
- `gene_info`: Gene symbol, name, HGNC ID
- `transcript_info`: RefSeq, Ensembl IDs, canonical status
- `suggestions`: Fix suggestions for invalid input

Report validation results:
- Count valid variants
- List invalid variants with errors and suggestions
- Ask user whether to proceed with valid variants only

### Step 3: Process Each Variant

For each valid variant, run classification:

```
Tool: classify_variant
Parameters:
  - hgvs_notation: <normalized_hgvs from validation>
```

**Classification output includes:**
- `classification`: Pathogenic/Likely Pathogenic/VUS/Likely Benign/Benign
- `confidence`: High/Medium/Low
- `criteria_met`: List of ACMG/AMP criteria applied
- `evidence_summary`: Brief explanation from evidence synthesis

Provide progress updates:
```
Processing variant 1/10: NM_000492.3:c.1521_1523delCTT... Done
Processing variant 2/10: BRCA1:c.5266dupC... Done
Processing variant 3/10: TP53:p.R273H... Error (see details)
```

### Step 4: Handle Errors

For failed classifications:
- Log the error
- Continue with remaining variants
- Include in error summary

### Step 5: Generate Summary

Produce comprehensive summary table:

```markdown
## Batch Classification Summary

**Total Variants:** [count]
**Successfully Classified:** [count]
**Failed:** [count]
**Processing Time:** [duration]

---

### Classification Results

| # | Variant | Gene | Classification | Confidence | Key Evidence |
|---|---------|------|----------------|------------|--------------|
| 1 | [HGVS] | [Gene] | [Class] | [Conf] | [Summary] |
| 2 | [HGVS] | [Gene] | [Class] | [Conf] | [Summary] |
| ... | ... | ... | ... | ... | ... |

---

### Classification Distribution

| Classification | Count | Percentage |
|----------------|-------|------------|
| Pathogenic | [n] | [%] |
| Likely Pathogenic | [n] | [%] |
| VUS | [n] | [%] |
| Likely Benign | [n] | [%] |
| Benign | [n] | [%] |

---

### Failed Variants

| Variant | Error |
|---------|-------|
| [variant] | [error message] |

---

### Detailed Results

[Expandable section with full classification details for each variant]
```

## Output Formats

### Summary Table (Default)

Condensed tabular format for quick review.

### Detailed Report

```
/batch <variants> --detailed
```

Full classification details for each variant.

### JSON Export

```
/batch <variants> --format json
```

Machine-readable output:

```json
{
  "batch_id": "uuid",
  "processed_at": "timestamp",
  "total": 10,
  "successful": 9,
  "failed": 1,
  "results": [
    {
      "variant": "NM_000492.3:c.1521_1523delCTT",
      "gene": "CFTR",
      "classification": "Pathogenic",
      "confidence": "High",
      "criteria": ["PVS1", "PS1", "PP5"]
    }
  ],
  "errors": [
    {
      "variant": "invalid:variant",
      "error": "Validation failed"
    }
  ]
}
```

### CSV Export

```
/batch <variants> --format csv
```

## Options

### Validation Only

Validate without classifying:

```
/batch <variants> --validate-only
```

### Skip Validation

Trust input and skip validation step:

```
/batch <variants> --skip-validation
```

### Continue on Error

Don't stop on individual failures (default behavior):

```
/batch <variants> --continue-on-error
```

### Stop on First Error

Halt processing on first failure:

```
/batch <variants> --stop-on-error
```

### Generate Reports

Generate individual reports for each variant:

```
/batch <variants> --with-reports
```

## Progress Tracking

For large batches, provide regular progress updates:

```
Batch Classification Progress
=============================
[████████████░░░░░░░░] 60% (6/10)

Current: TP53:p.R273H
Elapsed: 45s
Estimated remaining: 30s

Completed:
✓ NM_000492.3:c.1521_1523delCTT - Pathogenic
✓ BRCA1:c.5266dupC - Pathogenic
✓ MLH1:c.350C>T - VUS
✓ MSH2:c.942+3A>T - Likely Pathogenic
✓ APC:c.3927_3931del - Pathogenic
✓ TP53:p.R175H - Pathogenic

In Progress:
⋯ TP53:p.R273H

Pending:
○ BRCA2:c.5946del
○ PTEN:c.388C>T
○ RB1:c.2359C>T
```

## Examples

### Example 1: Basic Batch

```
User: /batch CFTR:c.1521_1523del, BRCA1:c.5266dupC, TP53:p.R273H

Claude:
## Batch Classification Summary

**Total Variants:** 3
**Successfully Classified:** 3
**Failed:** 0

### Classification Results

| # | Variant | Gene | Classification | Confidence |
|---|---------|------|----------------|------------|
| 1 | NM_000492.3:c.1521_1523del | CFTR | Pathogenic | High |
| 2 | NM_007294.4:c.5266dup | BRCA1 | Pathogenic | High |
| 3 | NM_000546.6:c.817C>A | TP53 | Pathogenic | High |

### Classification Distribution

| Classification | Count | Percentage |
|----------------|-------|------------|
| Pathogenic | 3 | 100% |
```

### Example 2: Mixed Results

```
User: /batch
NM_000492.3:c.1521_1523del
BRCA1:c.1234A>G
MLH1:c.350C>T
invalid_variant

Claude:
## Validation Results

**Valid:** 3 variants
**Invalid:** 1 variant

### Invalid Variants

| Variant | Error |
|---------|-------|
| invalid_variant | Invalid HGVS syntax |

Proceeding with 3 valid variants...

## Batch Classification Summary

**Total Variants:** 3
**Successfully Classified:** 3

### Classification Results

| # | Variant | Gene | Classification | Confidence |
|---|---------|------|----------------|------------|
| 1 | NM_000492.3:c.1521_1523del | CFTR | Pathogenic | High |
| 2 | NM_007294.4:c.1234A>G | BRCA1 | VUS | Medium |
| 3 | NM_000249.4:c.350C>T | MLH1 | VUS | Medium |

### Classification Distribution

| Classification | Count | Percentage |
|----------------|-------|------------|
| Pathogenic | 1 | 33% |
| VUS | 2 | 67% |
```

### Example 3: JSON Export

```
User: /batch CFTR:c.1521_1523del, BRCA1:c.5266dupC --format json

Claude:
{
  "batch_id": "550e8400-e29b-41d4-a716-446655440000",
  "processed_at": "2026-01-17T10:30:00Z",
  "total": 2,
  "successful": 2,
  "failed": 0,
  "results": [
    {
      "variant": "NM_000492.3:c.1521_1523del",
      "gene": "CFTR",
      "classification": "Pathogenic",
      "confidence": "High",
      "criteria_met": ["PS1", "PS4", "PP5"]
    },
    {
      "variant": "NM_007294.4:c.5266dup",
      "gene": "BRCA1",
      "classification": "Pathogenic",
      "confidence": "High",
      "criteria_met": ["PVS1", "PS4", "PP5"]
    }
  ]
}
```

## Limitations

- Maximum recommended batch size: 50 variants
- Large batches may take several minutes
- Some database rate limits may apply
- Complex variants may require individual analysis

## Error Handling

See `.claude/skills/_shared/error-handling.md` for:
- Partial batch failure handling
- Database timeout strategies
- Retry recommendations

## Related Skills

- `/classify` - Single variant classification (uses same MCP tools)

## MCP Tools Used

| Tool | Purpose |
|------|---------|
| `validate_hgvs` | Validate input notation (returns gene_info, transcript_info, suggestions) |
| `classify_variant` | Apply ACMG/AMP classification rules |
| `query_evidence` | Gather evidence (returns acmg_criteria_hints, synthesis, source_quality) |
| `generate_report` | Generate clinical report (with --with-reports flag) |
