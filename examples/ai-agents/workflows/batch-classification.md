# Batch Variant Classification Workflow

This workflow demonstrates how to efficiently classify multiple genetic variants simultaneously using the ACMG/AMP MCP server.

## Overview

**Objective**: Classify multiple variants from sequencing data or variant lists
**Target Users**: Clinical laboratories, research teams, bioinformaticians
**Estimated Time**: 5-15 minutes for 10-100 variants

## Prerequisites

- ACMG/AMP MCP server with batch processing capabilities
- Variant list in standard format (VCF, CSV, or structured JSON)
- External database API keys with sufficient quota
- Understanding of batch processing limitations

## Input Formats

### CSV Format
```csv
variant_id,hgvs,gene,chromosome,position,ref,alt,clinical_context
VAR001,NM_000492.3:c.1521_1523delCTT,CFTR,7,117199644,CTT,-,cystic_fibrosis
VAR002,NM_000138.4:c.419G>A,FBN1,15,48755048,G,A,marfan_syndrome
VAR003,NM_000059.3:c.1138C>T,BRCA2,13,32913605,C,T,breast_cancer_risk
```

### JSON Format
```json
{
  "variants": [
    {
      "id": "VAR001",
      "hgvs": "NM_000492.3:c.1521_1523delCTT",
      "gene": "CFTR",
      "clinical_context": ["cystic_fibrosis"]
    },
    {
      "id": "VAR002", 
      "hgvs": "NM_000138.4:c.419G>A",
      "gene": "FBN1",
      "clinical_context": ["marfan_syndrome"]
    }
  ],
  "options": {
    "include_evidence": true,
    "confidence_threshold": 0.7,
    "output_format": "detailed"
  }
}
```

## Step-by-Step Workflow

### Step 1: Batch Input Preparation

**Human Input**:
```
"I have a list of 25 variants from exome sequencing that need ACMG/AMP classification. The variants are in this CSV file: [file attachment]. Please process them and provide a summary report."
```

**AI Agent Actions**:
1. Parse the input file format
2. Validate variant list structure
3. Check for required fields and data quality
4. Prepare batch processing strategy

### Step 2: Batch Validation

**Expected MCP Tool Calls**:
```json
{
  "tool": "validate_hgvs_batch",
  "arguments": {
    "variants": [
      "NM_000492.3:c.1521_1523delCTT",
      "NM_000138.4:c.419G>A",
      "NM_000059.3:c.1138C>T"
    ]
  }
}
```

**Expected Response**:
```json
{
  "validation_results": [
    {
      "input": "NM_000492.3:c.1521_1523delCTT",
      "valid": true,
      "normalized": "NM_000492.3:c.1521_1523del",
      "gene": "CFTR"
    },
    {
      "input": "NM_000138.4:c.419G>A", 
      "valid": true,
      "normalized": "NM_000138.4:c.419G>A",
      "gene": "FBN1"
    }
  ],
  "summary": {
    "total": 25,
    "valid": 23,
    "invalid": 2,
    "warnings": ["VAR015: Non-standard transcript", "VAR023: Ambiguous notation"]
  }
}
```

### Step 3: Batch Evidence Gathering

**Expected MCP Tool Calls**:
```json
{
  "tool": "query_evidence_batch",
  "arguments": {
    "variants": [
      "NM_000492.3:c.1521_1523delCTT",
      "NM_000138.4:c.419G>A",
      "NM_000059.3:c.1138C>T"
    ],
    "databases": ["clinvar", "gnomad"],
    "batch_size": 10,
    "parallel_requests": 3
  }
}
```

**Expected Response**:
```json
{
  "evidence_results": [
    {
      "variant": "NM_000492.3:c.1521_1523delCTT",
      "sources": ["clinvar", "gnomad"],
      "evidence": {
        "clinvar": {"clinical_significance": "Pathogenic"},
        "gnomad": {"allele_frequency": 0}
      }
    }
  ],
  "batch_summary": {
    "total_variants": 25,
    "successful": 23,
    "failed": 2,
    "rate_limited": 0,
    "processing_time": "2.5 minutes"
  }
}
```

### Step 4: Batch Classification

**Expected MCP Tool Calls**:
```json
{
  "tool": "classify_variants_batch",
  "arguments": {
    "variants": [
      {
        "id": "VAR001",
        "hgvs": "NM_000492.3:c.1521_1523delCTT",
        "gene": "CFTR"
      },
      {
        "id": "VAR002",
        "hgvs": "NM_000138.4:c.419G>A", 
        "gene": "FBN1"
      }
    ],
    "options": {
      "include_evidence": true,
      "parallel_processing": true,
      "confidence_threshold": 0.7
    }
  }
}
```

**Expected Response**:
```json
{
  "classification_results": [
    {
      "variant_id": "VAR001",
      "hgvs": "NM_000492.3:c.1521_1523delCTT",
      "classification": "Pathogenic",
      "confidence": 0.95,
      "applied_criteria": ["PVS1", "PS3", "PM2", "PP3", "PP5"]
    },
    {
      "variant_id": "VAR002", 
      "hgvs": "NM_000138.4:c.419G>A",
      "classification": "Likely Pathogenic",
      "confidence": 0.85,
      "applied_criteria": ["PM1", "PM2", "PP2", "PP3"]
    }
  ],
  "batch_summary": {
    "total_variants": 25,
    "pathogenic": 8,
    "likely_pathogenic": 6,
    "uncertain_significance": 9,
    "likely_benign": 2,
    "benign": 0,
    "processing_time": "4.2 minutes",
    "high_confidence": 18,
    "review_needed": 7
  }
}
```

### Step 5: Batch Report Generation

**Expected MCP Tool Calls**:
```json
{
  "tool": "generate_batch_report",
  "arguments": {
    "classification_results": "...",
    "report_format": "summary_with_details",
    "include_sections": [
      "executive_summary",
      "classification_distribution", 
      "high_priority_variants",
      "variants_needing_review",
      "methodology"
    ]
  }
}
```

## AI Agent Response Structure

**Executive Summary**:
```
## Batch Variant Classification Report

**Analysis Date**: 2024-01-15
**Total Variants Processed**: 25
**Processing Time**: 6.8 minutes
**Success Rate**: 92% (23/25 variants successfully classified)

### Classification Distribution
- **Pathogenic**: 8 variants (32%)
- **Likely Pathogenic**: 6 variants (24%) 
- **Uncertain Significance**: 9 variants (36%)
- **Likely Benign**: 2 variants (8%)
- **Benign**: 0 variants (0%)

### High-Priority Findings
Three variants require immediate clinical attention:

1. **VAR001** (CFTR c.1521_1523delCTT): Pathogenic - Known CF mutation
2. **VAR007** (BRCA1 c.185delA): Pathogenic - High breast cancer risk  
3. **VAR012** (TP53 c.742C>T): Pathogenic - Li-Fraumeni syndrome risk
```

**Detailed Results Table**:
```
| Variant ID | Gene | HGVS | Classification | Confidence | Priority |
|------------|------|------|---------------|------------|----------|
| VAR001     | CFTR | c.1521_1523delCTT | Pathogenic | 95% | High |
| VAR002     | FBN1 | c.419G>A | Likely Pathogenic | 85% | Medium |
| VAR003     | BRCA2 | c.1138C>T | Uncertain Significance | 65% | Review |
```

## Optimization Strategies

### Performance Optimization

1. **Batch Size Tuning**:
   ```json
   "batch_options": {
     "optimal_batch_size": 10,
     "max_parallel_requests": 5,
     "rate_limit_buffer": "30s"
   }
   ```

2. **Caching Strategy**:
   ```json
   "caching_options": {
     "enable_evidence_cache": true,
     "cache_duration": "24h",
     "reuse_recent_classifications": true
   }
   ```

3. **Prioritization**:
   ```json
   "processing_priority": {
     "known_pathogenic_first": true,
     "clinical_context_priority": true,
     "gene_importance_weighting": true
   }
   ```

### Error Handling

**Failed Variants Processing**:
```json
{
  "failed_variants": [
    {
      "variant_id": "VAR015",
      "error": "Invalid HGVS notation", 
      "suggestion": "Check transcript version"
    },
    {
      "variant_id": "VAR023",
      "error": "API rate limit exceeded",
      "suggestion": "Retry in 5 minutes"
    }
  ],
  "retry_strategy": {
    "auto_retry": true,
    "max_attempts": 3,
    "backoff_strategy": "exponential"
  }
}
```

## Quality Control

### Pre-processing Checks
- Variant format validation
- Gene symbol standardization
- Transcript version verification
- Duplicate variant detection

### Post-processing Validation
- Classification consistency checks
- Confidence score distribution analysis
- Evidence quality assessment
- Known variant benchmark comparison

## Output Formats

### CSV Output
```csv
variant_id,hgvs,gene,classification,confidence,criteria,clinical_significance
VAR001,NM_000492.3:c.1521_1523delCTT,CFTR,Pathogenic,0.95,"PVS1,PS3,PM2","Cystic fibrosis causing"
```

### Excel Report
- Summary sheet with overview statistics
- Detailed results with hyperlinks to evidence
- Visualization charts for classification distribution
- Separate sheets for high-priority variants

### JSON Export
```json
{
  "batch_id": "BATCH_20240115_001",
  "metadata": {
    "processing_date": "2024-01-15T10:30:00Z",
    "total_variants": 25,
    "success_rate": 0.92
  },
  "results": [...],
  "quality_metrics": {...}
}
```

## Integration Examples

### Laboratory LIMS Integration
```python
# Python example for LIMS integration
import pandas as pd

# Read variants from LIMS
variants_df = lims_client.get_pending_variants()

# Convert to MCP format
mcp_batch = {
    "variants": variants_df.to_dict('records'),
    "options": {"include_evidence": True}
}

# Process batch
results = await acmg_client.classify_variants_batch(mcp_batch)

# Update LIMS with results
for result in results['classification_results']:
    lims_client.update_variant_classification(
        result['variant_id'], 
        result['classification'],
        result['confidence']
    )
```

### Research Pipeline Integration
```bash
# Bash script for research pipeline
#!/bin/bash

# Convert VCF to batch format
python3 vcf_to_batch.py input.vcf > variants_batch.json

# Process with MCP server
curl -X POST http://localhost:8080/batch/classify \
  -H "Content-Type: application/json" \
  -d @variants_batch.json > results.json

# Generate research report
python3 generate_research_report.py results.json output_dir/
```

## Best Practices

1. **Batch Size Management**: Optimal batch size is 10-50 variants
2. **Rate Limit Awareness**: Monitor API quotas and implement backoff
3. **Quality Thresholds**: Set appropriate confidence thresholds for batch processing
4. **Error Recovery**: Implement robust retry mechanisms
5. **Progress Monitoring**: Provide progress updates for long-running batches
6. **Result Validation**: Always validate batch results against known benchmarks

## Troubleshooting

### Common Issues
- API rate limits exceeded
- Memory issues with large batches
- Network timeouts for slow databases
- Inconsistent variant formats

### Solutions
- Implement exponential backoff
- Process in smaller chunks
- Increase timeout settings
- Standardize input validation

## Related Workflows

- [Basic Classification](./basic-classification.md) - Single variant workflow
- [Complex Cases](./complex-cases.md) - Difficult interpretation scenarios
- [Quality Control](./quality-control.md) - Result validation procedures