# ACMG-AMP Classifier Skills

This directory contains Claude Code skills for the ACMG-AMP MCP Server. Skills provide guided, multi-step workflows that orchestrate the underlying MCP tools.

## Architecture Philosophy

This project follows a lean architecture based on the YAGNI (You Aren't Gonna Need It) principle:

- **Skills are for multi-step workflows** that require orchestration
- **MCP tools are self-sufficient** with enhanced outputs (gene info, quality scores, ACMG hints)
- **No 1:1 wrapper skills** - use MCP tools directly for single operations

## Available Skills

| Skill | Command | Description |
|-------|---------|-------------|
| **Classify** | `/classify <variant>` | Full ACMG/AMP classification workflow |
| **Batch** | `/batch <variants>` | Process multiple variants with progress tracking |

## Quick Start

### Classify a Variant

```
/classify NM_000492.3:c.1521_1523delCTT
```

or with gene symbol:

```
/classify BRCA1:c.5266dupC
```

### Batch Processing

```
/batch CFTR:c.1521_1523del, BRCA1:c.5266dupC, TP53:p.R273H
```

## Skills vs MCP Tools

| Aspect | MCP Tools | Skills |
|--------|-----------|--------|
| **Invocation** | JSON-RPC protocol | Slash commands (`/classify`) |
| **Target** | AI agents (Claude API) | Claude Code CLI users |
| **Purpose** | Self-sufficient operations | Multi-step workflows |
| **Format** | Structured JSON with enhanced fields | Natural language + guidance |

### When to Use Each

**Use MCP Tools Directly:**
- Single validation: `validate_hgvs` returns gene_info, transcript_info, suggestions
- Single evidence query: `query_evidence` returns acmg_criteria_hints, synthesis, source_quality
- Single report: `generate_report` provides complete output

**Use Skills:**
- `/classify` - Multi-step workflow: validate → gather evidence → classify → report
- `/batch` - Iteration workflow: validate all → classify each → summarize results

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                   Claude Code CLI                            │
│  ┌─────────────────────────────────────────────────────┐   │
│  │ Skills: /classify, /batch                            │   │
│  │ (Multi-step workflow orchestration)                  │   │
│  └───────────────────────┬─────────────────────────────┘   │
│                          │ Orchestrates                      │
└──────────────────────────┼──────────────────────────────────┘
                           ▼
┌─────────────────────────────────────────────────────────────┐
│              ACMG-AMP MCP Server                            │
│  ┌──────────────────────────────────────────────────────┐  │
│  │ Enhanced MCP Tools (self-sufficient)                 │  │
│  │ • validate_hgvs  → gene_info, transcript_info        │  │
│  │ • query_evidence → acmg_hints, synthesis, quality    │  │
│  │ • classify_variant → full classification             │  │
│  │ • generate_report → complete clinical report         │  │
│  └──────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

## Directory Structure

```
.claude/skills/
├── README.md                    # This file
├── classify/
│   └── SKILL.md                 # /classify skill definition
├── batch/
│   └── SKILL.md                 # /batch skill definition
└── _shared/
    ├── acmg-guidelines.md       # ACMG/AMP rules reference
    ├── error-handling.md        # Error handling guide
    └── clinical-context.md      # Clinical interpretation guide
```

## Skill Details

### /classify

Full ACMG/AMP variant classification workflow.

**Workflow:**
1. Validate input notation (uses enhanced `validate_hgvs`)
2. Gather evidence from databases (uses enhanced `query_evidence`)
3. Apply ACMG/AMP classification rules
4. Generate optional clinical report

**Examples:**
```
/classify NM_000492.3:c.1521_1523delCTT
/classify BRCA1:c.5266dupC
/classify TP53 p.R273H
/classify chr17:g.43094692G>A --report
```

### /batch

Process multiple variants with progress tracking.

**Features:**
- Parallel validation with suggestions for invalid input
- Progress updates during processing
- Summary statistics with classification distribution
- Error handling with continue-on-error support

**Examples:**
```
/batch CFTR:c.1521_1523del, BRCA1:c.5266dupC
/batch --format json
/batch --detailed
```

## Enhanced MCP Tool Outputs

### validate_hgvs

Returns self-sufficient validation with enriched data:

```json
{
  "is_valid": true,
  "normalized_hgvs": "NM_000492.3:c.1521_1523del",
  "gene_info": {
    "symbol": "CFTR",
    "name": "CF transmembrane conductance regulator",
    "hgnc_id": "HGNC:1884"
  },
  "transcript_info": {
    "refseq": "NM_000492.3",
    "ensembl": "ENST00000003084",
    "is_canonical": true
  },
  "suggestions": []
}
```

### query_evidence

Returns evidence with quality assessment and ACMG mapping:

```json
{
  "hgvs_notation": "NM_000492.3:c.1521_1523del",
  "database_results": { ... },
  "source_quality": {
    "clinvar": { "quality": "high", "notes": "Expert panel review" },
    "gnomad": { "quality": "high", "notes": "Complete coverage" }
  },
  "acmg_criteria_hints": {
    "PS1": { "applicable": true, "note": "Same AA change as known pathogenic" },
    "PM2": { "applicable": true, "note": "Absent from gnomAD" }
  },
  "quality_scores": {
    "overall_quality": "high",
    "data_completeness": 0.85
  },
  "synthesis": "Population frequency: absent from gnomAD. Clinical significance: Pathogenic with expert panel review."
}
```

## Shared Resources

### ACMG Guidelines (`_shared/acmg-guidelines.md`)

Reference for all 28 ACMG/AMP evidence criteria:
- PVS1 (Very Strong Pathogenic)
- PS1-PS4 (Strong Pathogenic)
- PM1-PM6 (Moderate Pathogenic)
- PP1-PP5 (Supporting Pathogenic)
- BA1 (Stand-alone Benign)
- BS1-BS4 (Strong Benign)
- BP1-BP7 (Supporting Benign)

### Error Handling (`_shared/error-handling.md`)

Common error scenarios and solutions:
- Input validation errors
- Database connection issues
- Insufficient evidence handling

### Clinical Context (`_shared/clinical-context.md`)

Clinical interpretation guidance:
- Report components
- Gene-specific considerations
- Professional disclaimers

## MCP Tool Reference

### Core Classification Tools
| MCP Tool | Purpose | Enhanced Output |
|----------|---------|-----------------|
| `classify_variant` | Complete ACMG/AMP workflow | Full classification with confidence |
| `validate_hgvs` | Validate HGVS notation | gene_info, transcript_info, suggestions |
| `apply_rule` | Apply specific ACMG/AMP rule | Rule evaluation result |
| `combine_evidence` | Combine rule results | Final classification |

### Evidence Gathering Tools
| MCP Tool | Purpose | Enhanced Output |
|----------|---------|-----------------|
| `query_evidence` | Query all 6 databases | acmg_criteria_hints, synthesis, source_quality |
| `batch_query_evidence` | Batch query with caching | Multiple variant evidence |
| `query_clinvar` | Search ClinVar | Clinical significance data |
| `query_gnomad` | Search gnomAD | Population frequency data |
| `query_cosmic` | Search COSMIC | Somatic mutation data |

### Report Generation Tools
| MCP Tool | Purpose | Enhanced Output |
|----------|---------|-----------------|
| `generate_report` | Create clinical report | Complete formatted report |
| `format_report` | Export in multiple formats | JSON, text, PDF output |
| `validate_report` | Quality assurance | Validation results |

### Feedback Tools
| MCP Tool | Purpose | Enhanced Output |
|----------|---------|-----------------|
| `submit_feedback` | Save user feedback | Confirmation |
| `query_feedback` | Check previous feedback | Feedback record |
| `list_feedback` | List all feedback | Paginated results |
| `export_feedback` | Export to JSON | File path |
| `import_feedback` | Import from JSON | Import statistics |

## Disclaimers

**IMPORTANT: Research and Educational Use Only**

- These tools are NOT approved for clinical diagnostic use
- Results require professional interpretation
- Should not be the sole basis for medical decisions
- Clinical use requires regulatory approval

## Support

For issues or questions:
1. Check error handling guide: `_shared/error-handling.md`
2. Review ACMG guidelines: `_shared/acmg-guidelines.md`
3. Consult clinical context: `_shared/clinical-context.md`

## License

This software is released under a Non-Commercial License. See the main repository LICENSE file for details.
