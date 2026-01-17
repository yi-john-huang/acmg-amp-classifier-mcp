# Error Handling Guide

## Overview

This guide covers common error scenarios when using the ACMG-AMP MCP Server skills and how to handle them.

## Error Categories

### 1. Input Validation Errors

#### Invalid HGVS Notation

**Symptoms:**
- `validate_hgvs` tool returns validation error
- "Invalid HGVS notation" message

**Common Causes:**
- Missing transcript reference (e.g., `c.123A>G` instead of `NM_000492.3:c.123A>G`)
- Incorrect nucleotide positions
- Invalid variant syntax

**Resolution Steps:**
1. Verify transcript reference exists and is valid
2. Check nucleotide positions match reference sequence
3. Use `/validate` skill to get detailed error information
4. Refer to HGVS nomenclature guidelines

**Example Fix:**
```
Invalid:  c.123A>G
Valid:    NM_000492.3:c.123A>G

Invalid:  BRCA1 123A>G
Valid:    BRCA1:c.123A>G
```

#### Unrecognized Gene Symbol

**Symptoms:**
- Gene symbol not found in databases
- Transcript resolution fails

**Resolution Steps:**
1. Verify gene symbol is HGNC-approved
2. Check for common aliases
3. Try using RefSeq transcript directly

### 2. Database Connection Errors

#### External Database Unavailable

**Symptoms:**
- `query_evidence` returns partial results
- Timeout errors for specific databases

**Affected Databases:**
- ClinVar (NCBI)
- gnomAD (Broad Institute)
- COSMIC (Sanger Institute)
- PubMed (NCBI)

**Resolution Steps:**
1. Check if specific database is experiencing downtime
2. Retry the query after a brief wait
3. Proceed with available evidence (classification may be less certain)
4. Note missing database in report

**Graceful Degradation:**
The MCP server implements circuit breakers. When a database is unavailable:
- Other databases continue to function
- Classification proceeds with available evidence
- Confidence scores are adjusted accordingly

### 3. Classification Errors

#### Insufficient Evidence

**Symptoms:**
- Classification returns "Uncertain Significance (VUS)"
- Low confidence score

**This is Not an Error** - VUS is a valid classification when:
- Population frequency data is unavailable
- No prior clinical assertions exist
- Functional studies are lacking

**Recommendations:**
1. Document the classification with available evidence
2. Consider additional testing or research
3. Use `/evidence` skill to see what data is available

#### Conflicting Evidence

**Symptoms:**
- Both pathogenic and benign evidence present
- Classification confidence is reduced

**Resolution:**
1. Review individual evidence items
2. Assess evidence quality and source reliability
3. Consider gene-specific guidelines if available
4. Document the conflict in the report

### 4. Report Generation Errors

#### Missing Classification Data

**Symptoms:**
- `/report` fails without prior classification
- "No classification found" error

**Resolution:**
1. Run `/classify` first to generate classification
2. Ensure variant notation matches exactly
3. Use session context to maintain classification state

### 5. Batch Processing Errors

#### Partial Batch Failure

**Symptoms:**
- Some variants in batch fail while others succeed
- Mixed results in output

**Handling:**
- Failed variants are reported individually
- Successful classifications are still returned
- Summary indicates which variants failed

**Resolution:**
1. Review failed variants separately
2. Check input format for each failed variant
3. Run `/validate` on problematic variants

## Error Response Format

MCP tools return structured error information:

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Invalid HGVS notation",
    "details": {
      "field": "hgvs_notation",
      "value": "invalid input",
      "suggestion": "Use format: NM_XXXXX.X:c.XXX"
    }
  }
}
```

## Error Codes Reference

| Code | Category | Description |
|------|----------|-------------|
| `VALIDATION_ERROR` | Input | Invalid input format |
| `NOT_FOUND` | Data | Requested resource not found |
| `DATABASE_ERROR` | External | External database error |
| `TIMEOUT` | Network | Request timed out |
| `RATE_LIMITED` | Access | Too many requests |
| `INTERNAL_ERROR` | Server | Server-side error |

## Retry Strategy

For transient errors, use this retry approach:

1. **Immediate retry**: For timeout errors
2. **Exponential backoff**: For rate limiting (wait 1s, 2s, 4s...)
3. **Skip and continue**: For batch processing failures
4. **Fail fast**: For validation errors (fix input first)

## Reporting Issues

If you encounter persistent errors:

1. Check server health: `curl http://localhost:8080/health`
2. Review server logs for detailed error information
3. Verify database connections are configured
4. Check API key validity for external services
