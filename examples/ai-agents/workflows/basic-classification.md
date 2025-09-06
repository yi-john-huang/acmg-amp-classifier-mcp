# Basic Variant Classification Workflow

This workflow demonstrates the fundamental process of classifying a genetic variant using ACMG/AMP guidelines through the MCP server.

## Overview

**Objective**: Classify a single genetic variant and provide clinical recommendations
**Target Users**: Clinical geneticists, genetic counselors, laboratory personnel
**Estimated Time**: 2-5 minutes per variant

## Prerequisites

- ACMG/AMP MCP server running and accessible
- External database API keys configured (ClinVar, gnomAD, COSMIC)
- AI agent connected to MCP server (Claude Desktop, custom client, etc.)

## Step-by-Step Workflow

### Step 1: Variant Input and Validation

**Human Input**: 
```
"I need to classify the variant NM_000492.3:c.1521_1523delCTT in the CFTR gene. This was found in a patient with cystic fibrosis symptoms."
```

**AI Agent Actions**:
1. Extract variant information from the request
2. Validate HGVS notation using `validate_hgvs` tool
3. Confirm gene symbol and transcript

**Expected MCP Tool Calls**:
```json
{
  "tool": "validate_hgvs",
  "arguments": {
    "hgvs": "NM_000492.3:c.1521_1523delCTT"
  }
}
```

**Expected Response**:
```json
{
  "valid": true,
  "normalized": "NM_000492.3:c.1521_1523del",
  "gene": "CFTR",
  "transcript": "NM_000492.3",
  "change": "c.1521_1523del",
  "type": "deletion"
}
```

### Step 2: Evidence Gathering

**AI Agent Actions**:
1. Query multiple databases for variant evidence
2. Gather population frequency data
3. Collect clinical annotations and pathogenicity predictions

**Expected MCP Tool Calls**:
```json
{
  "tool": "query_evidence",
  "arguments": {
    "variant": "NM_000492.3:c.1521_1523delCTT",
    "databases": ["clinvar", "gnomad", "cosmic"]
  }
}
```

**Expected Response**:
```json
{
  "sources": ["clinvar", "gnomad"],
  "evidence": {
    "clinvar": {
      "variation_id": "12345",
      "clinical_significance": "Pathogenic",
      "review_status": "criteria provided, multiple submitters, no conflicts",
      "submissions": 15,
      "condition": "Cystic fibrosis"
    },
    "gnomad": {
      "allele_count": 0,
      "allele_number": 251472,
      "allele_frequency": 0,
      "popmax_population": "NFE",
      "popmax_frequency": 0
    },
    "functional_predictions": {
      "consequence": "frameshift_variant",
      "impact": "HIGH",
      "protein_change": "p.Phe508del"
    }
  }
}
```

### Step 3: ACMG/AMP Classification

**AI Agent Actions**:
1. Apply ACMG/AMP criteria based on gathered evidence
2. Calculate final classification and confidence score
3. Document applied criteria and reasoning

**Expected MCP Tool Calls**:
```json
{
  "tool": "classify_variant",
  "arguments": {
    "variant_data": {
      "hgvs": "NM_000492.3:c.1521_1523delCTT",
      "gene": "CFTR",
      "transcript": "NM_000492.3"
    },
    "options": {
      "include_evidence": true,
      "confidence_threshold": 0.8
    }
  }
}
```

**Expected Response**:
```json
{
  "classification": "Pathogenic",
  "confidence": 0.95,
  "applied_criteria": {
    "pathogenic": ["PVS1", "PS3", "PM2"],
    "benign": [],
    "supporting": ["PP3", "PP5"]
  },
  "evidence_summary": {
    "PVS1": "Null variant in gene where LOF is established mechanism",
    "PS3": "Well-established functional studies show damaging effect",
    "PM2": "Absent from controls in population databases",
    "PP3": "Computational evidence supports deleterious effect",
    "PP5": "Reputable source reports variant as pathogenic"
  },
  "acmg_points": {
    "pathogenic_strong": 1,
    "pathogenic_moderate": 2,
    "pathogenic_supporting": 2,
    "benign_strong": 0,
    "benign_supporting": 0
  }
}
```

### Step 4: Clinical Report Generation

**AI Agent Actions**:
1. Generate comprehensive clinical report
2. Include evidence summary and recommendations
3. Format for clinical use

**Expected MCP Tool Calls**:
```json
{
  "tool": "generate_report",
  "arguments": {
    "classification_data": {
      "variant": "NM_000492.3:c.1521_1523delCTT",
      "classification": "Pathogenic",
      "confidence": 0.95
    },
    "format": "clinical",
    "include_sections": ["summary", "evidence", "recommendation", "references"]
  }
}
```

### Step 5: AI Agent Response

**Final Response to Human**:
```
## Variant Classification Report

**Variant**: NM_000492.3:c.1521_1523delCTT (CFTR gene)
**Classification**: Pathogenic (High Confidence: 95%)

### Summary
This is a well-characterized pathogenic variant in the CFTR gene, commonly known as F508del, which is the most frequent mutation causing cystic fibrosis.

### Evidence Applied (ACMG/AMP Criteria)
- **PVS1**: Frameshift variant resulting in loss of function in a gene where LOF is the established disease mechanism
- **PS3**: Well-established functional studies demonstrate severe impact on CFTR protein function
- **PM2**: Variant is absent in population databases (gnomAD), indicating it's not a benign polymorphism
- **PP3**: Computational predictions consistently predict deleterious effect
- **PP5**: Variant is reported as pathogenic in reputable databases (ClinVar)

### Clinical Recommendations
1. **Diagnosis**: Confirms cystic fibrosis diagnosis in symptomatic patients
2. **Family Testing**: Recommend cascade screening for family members
3. **Treatment**: Patient eligible for CFTR modulators (ivacaftor/lumacaftor combination)
4. **Genetic Counseling**: Discuss inheritance pattern, family planning implications
5. **Follow-up**: Regular pulmonary and nutritional monitoring as per CF guidelines

### Clinical Significance
This variant has been extensively studied and is definitively associated with cystic fibrosis. The F508del mutation affects protein folding and trafficking, leading to reduced CFTR function at the cell surface.
```

## Alternative Scenarios

### Scenario A: Uncertain Significance Result

**If classification returns "Uncertain Significance"**:
- AI agent should request additional clinical information
- Suggest functional studies or family segregation analysis
- Recommend periodic re-evaluation as new evidence becomes available

### Scenario B: Conflicting Evidence

**If databases show conflicting interpretations**:
- AI agent should highlight the conflicts
- Analyze the quality of evidence from each source
- Provide reasoning for final classification choice

### Scenario C: Novel Variant

**If variant is not found in databases**:
- Focus on computational predictions and gene-specific information
- Emphasize the limitations of classification
- Recommend functional studies if clinically warranted

## Usage Examples

### Claude Desktop
```
"Please classify the CFTR variant c.1521_1523delCTT found in a patient with pancreatic insufficiency and elevated sweat chloride."
```

### Custom Python Client
```python
result = await acmg_client.classify_variant({
    "hgvs": "NM_000492.3:c.1521_1523delCTT",
    "gene": "CFTR",
    "clinical_context": {
        "phenotype": ["cystic_fibrosis", "pancreatic_insufficiency"],
        "family_history": "positive"
    }
})
```

### HTTP API (via bridge)
```bash
curl -X POST http://localhost:8080/classify \
  -H "Content-Type: application/json" \
  -d '{
    "variant": "NM_000492.3:c.1521_1523delCTT",
    "gene": "CFTR",
    "include_report": true
  }'
```

## Quality Checks

Before finalizing classification, verify:

1. **HGVS Notation**: Correctly formatted and validated
2. **Gene Context**: Appropriate transcript and gene symbol used  
3. **Evidence Quality**: Multiple high-quality sources consulted
4. **Criteria Application**: ACMG/AMP guidelines followed correctly
5. **Clinical Relevance**: Classification makes sense given patient phenotype

## Common Pitfalls

1. **Incomplete Evidence Gathering**: Always query multiple databases
2. **Outdated Information**: Ensure latest classifications are used
3. **Population-Specific Issues**: Consider patient ancestry in frequency analysis
4. **Transcript Mismatches**: Verify correct reference transcript is used
5. **Criteria Misapplication**: Double-check ACMG/AMP rule interpretation

## Related Workflows

- [Batch Classification](./batch-classification.md) - Multiple variants at once
- [Family Analysis](./family-analysis.md) - Segregation analysis workflow
- [Pharmacogenomics](./pharmacogenomics.md) - Drug response variants
- [Complex Cases](./complex-cases.md) - Challenging interpretation scenarios