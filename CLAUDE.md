# AI Agents Integration Guide

## Purpose
This document defines how AI agents should interact with the ACMG-AMP MCP Server and provides guidelines for effective variant classification workflows.

## ACMG-AMP Classifier Skills

The ACMG-AMP MCP Server provides Claude Code skills for streamlined variant classification workflows. Skills orchestrate multi-step workflows, while MCP tools are self-sufficient for single operations.

### Architecture Philosophy

This project follows a lean architecture based on YAGNI (You Aren't Gonna Need It):
- **Skills are for multi-step workflows** requiring orchestration
- **MCP tools are self-sufficient** with enhanced outputs (gene info, quality scores, ACMG hints)
- **No 1:1 wrapper skills** - use MCP tools directly for single operations

### Available Skills

| Skill | Command | Description |
|-------|---------|-------------|
| **Classify** | `/classify <variant>` | Full ACMG/AMP classification workflow |
| **Batch** | `/batch <variants>` | Process multiple variants with progress tracking |

### Quick Start Examples

```bash
# Multi-step classification workflow
/classify NM_000492.3:c.1521_1523delCTT
/classify BRCA1:c.5266dupC
/classify TP53:p.R273H --report

# Batch processing multiple variants
/batch CFTR:c.1521_1523del, BRCA1:c.5266dupC, TP53:p.R273H
```

### Skills vs MCP Tools

| Aspect | MCP Tools | Skills |
|--------|-----------|--------|
| **Invocation** | JSON-RPC protocol | Slash commands (`/classify`) |
| **Target** | AI agents (Claude API) | Claude Code CLI users |
| **Purpose** | Self-sufficient operations | Multi-step workflows |
| **Format** | Structured JSON with enhanced fields | Natural language + guidance |

**Use MCP Tools Directly For:**
- Single validation: `validate_hgvs` returns gene_info, transcript_info, suggestions
- Single evidence query: `query_evidence` returns acmg_criteria_hints, synthesis, source_quality
- Single report: `generate_report` provides complete output

### Available MCP Tools (17 total)

**Core Classification:** `classify_variant`, `validate_hgvs`, `apply_rule`, `combine_evidence`

**Evidence Gathering:** `query_evidence`, `batch_query_evidence`, `query_clinvar`, `query_gnomad`, `query_cosmic`

**Report Generation:** `generate_report`, `format_report`, `validate_report`

**Feedback:** `submit_feedback`, `query_feedback`, `list_feedback`, `export_feedback`, `import_feedback`

### Skill Documentation

Full documentation for each skill is available in `.claude/skills/`:
- `.claude/skills/classify/SKILL.md` - Classification workflow
- `.claude/skills/batch/SKILL.md` - Batch processing

### Shared Resources

Reference materials in `.claude/skills/_shared/`:
- `acmg-guidelines.md` - ACMG/AMP rules quick reference
- `error-handling.md` - Error handling guide
- `clinical-context.md` - Clinical interpretation guidance

---

## Best Practices for AI Agents

### 1. Variant Classification Workflow

When classifying variants, follow this recommended workflow:

```
1. Validate Input → validate_hgvs
2. Gather Evidence → query_evidence (or individual database tools)
3. Apply Rules → apply_rule (for specific criteria)
4. Combine Evidence → combine_evidence
5. Generate Report → generate_report
```

Or use the `/classify` skill which orchestrates all steps automatically.

### 2. Input Formats

The classifier accepts multiple input formats:
- **HGVS notation**: `NM_000492.3:c.1521_1523delCTT`
- **Gene symbol + variant**: `BRCA1:c.5266dupC`
- **Protein change**: `TP53:p.R273H`
- **Genomic coordinates**: `chr17:g.43094692G>A`

### 3. Error Handling

- Always validate input before classification
- Check for suggestions when validation fails
- Handle database connectivity issues gracefully
- Provide meaningful error messages to users

### 4. User Feedback

Collect user feedback to improve future classifications:
- Use `submit_feedback` when users agree or disagree with classifications
- Query previous feedback with `query_feedback` for consistency
- Export feedback regularly for backup with `export_feedback`

---

## Code Quality Standards

### Testing Requirements
- Generate unit tests for all new functions
- Create integration tests for workflows
- Ensure test coverage meets project standards

### Security Considerations
- Never commit sensitive data (API keys, credentials)
- Validate all user input
- Follow OWASP Top 10 guidelines
- Use environment variables for configuration

### Documentation
- Update relevant documentation with changes
- Maintain clear commit messages
- Document design decisions and trade-offs

---

## Server Configurations

### Lite Server (Recommended for most users)
- No external dependencies
- SQLite for feedback storage
- In-memory caching
- Single binary deployment

### Full Server (Production deployments)
- PostgreSQL for all data storage
- Redis for distributed caching
- Docker/Kubernetes ready
- Health monitoring and metrics

---

## Disclaimers

**IMPORTANT: Research and Educational Use Only**

- These tools are NOT approved for clinical diagnostic use
- Results require professional interpretation
- Should not be the sole basis for medical decisions
- Clinical use requires regulatory approval

---

## Contact

For issues or questions, refer to:
1. Error handling guide: `.claude/skills/_shared/error-handling.md`
2. ACMG guidelines: `.claude/skills/_shared/acmg-guidelines.md`
3. Clinical context: `.claude/skills/_shared/clinical-context.md`
