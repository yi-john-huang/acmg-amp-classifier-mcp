# ACMG/AMP Guidelines Quick Reference

## Overview

The ACMG/AMP 2015 guidelines provide a standardized framework for classifying genetic variants. This reference covers all 28 evidence criteria used by the ACMG-AMP MCP Server.

## Classification Categories

| Category | Description | Clinical Action |
|----------|-------------|-----------------|
| **Pathogenic** | Causes disease | Report, clinical action indicated |
| **Likely Pathogenic** | >90% probability pathogenic | Report, clinical action may be indicated |
| **Uncertain Significance (VUS)** | Insufficient evidence | Report with caution, no clinical action |
| **Likely Benign** | >90% probability benign | May report, no clinical action |
| **Benign** | Does not cause disease | May report, no clinical action |

## Evidence Categories

### Very Strong Evidence

| Code | Name | Description |
|------|------|-------------|
| **PVS1** | Null variant | Null variant (nonsense, frameshift, canonical splice, initiation codon) in a gene where LOF is a known disease mechanism |

### Strong Pathogenic Evidence

| Code | Name | Description |
|------|------|-------------|
| **PS1** | Same AA change | Same amino acid change as established pathogenic variant |
| **PS2** | De novo (confirmed) | De novo variant with confirmed parentage |
| **PS3** | Functional studies | Well-established functional studies show deleterious effect |
| **PS4** | Prevalence | Significantly increased prevalence in affected vs controls |

### Moderate Pathogenic Evidence

| Code | Name | Description |
|------|------|-------------|
| **PM1** | Mutational hot spot | Located in critical/functional domain without benign variation |
| **PM2** | Absent in controls | Absent from controls (or extremely low frequency) |
| **PM3** | Trans with pathogenic | In trans with pathogenic variant (recessive disorders) |
| **PM4** | Protein length change | Protein length changes due to in-frame deletions/insertions |
| **PM5** | Novel missense | Novel missense at position of established pathogenic variant |
| **PM6** | De novo (assumed) | Assumed de novo, without confirmation |

### Supporting Pathogenic Evidence

| Code | Name | Description |
|------|------|-------------|
| **PP1** | Cosegregation | Cosegregation with disease in multiple family members |
| **PP2** | Missense constraint | Missense in gene with low rate of benign missense |
| **PP3** | Computational | Multiple computational tools support deleterious effect |
| **PP4** | Phenotype specific | Patient phenotype highly specific for gene |
| **PP5** | Reputable source | Reputable source reports pathogenic (without evidence) |

### Stand-alone Benign Evidence

| Code | Name | Description |
|------|------|-------------|
| **BA1** | High frequency | Allele frequency >5% in population databases |

### Strong Benign Evidence

| Code | Name | Description |
|------|------|-------------|
| **BS1** | High frequency | Allele frequency greater than expected for disorder |
| **BS2** | Observed healthy | Observed in healthy adult (full penetrance expected) |
| **BS3** | Functional studies | Functional studies show no damaging effect |
| **BS4** | Non-segregation | Lack of segregation in affected family members |

### Supporting Benign Evidence

| Code | Name | Description |
|------|------|-------------|
| **BP1** | Missense in truncating gene | Missense in gene where truncating causes disease |
| **BP2** | In trans/cis | Observed in trans with dominant or in cis with pathogenic |
| **BP3** | In-frame in repeat | In-frame in repetitive region without function |
| **BP4** | Computational benign | Computational evidence suggests no impact |
| **BP5** | Alternate mechanism | Variant in gene with alternate molecular basis |
| **BP6** | Reputable source benign | Reputable source reports benign (without evidence) |
| **BP7** | Synonymous | Synonymous with no splicing prediction |

## Evidence Combination Rules

### Pathogenic Classification

| Combination | Required Evidence |
|-------------|-------------------|
| **1** | 1 Very Strong (PVS1) + 1 Strong (PS1-PS4) |
| **2** | 1 Very Strong (PVS1) + 2 Moderate (PM1-PM6) |
| **3** | 1 Very Strong (PVS1) + 1 Moderate + 1 Supporting (PP1-PP5) |
| **4** | 1 Very Strong (PVS1) + 2 Supporting |
| **5** | 2 Strong |
| **6** | 1 Strong + 3 Moderate |
| **7** | 1 Strong + 2 Moderate + 2 Supporting |
| **8** | 1 Strong + 1 Moderate + 4 Supporting |

### Likely Pathogenic Classification

| Combination | Required Evidence |
|-------------|-------------------|
| **1** | 1 Very Strong + 1 Moderate |
| **2** | 1 Strong + 1-2 Moderate |
| **3** | 1 Strong + 2 Supporting |
| **4** | 3 Moderate |
| **5** | 2 Moderate + 2 Supporting |
| **6** | 1 Moderate + 4 Supporting |

### Benign Classification

| Combination | Required Evidence |
|-------------|-------------------|
| **1** | 1 Stand-alone (BA1) |
| **2** | 2 Strong (BS1-BS4) |

### Likely Benign Classification

| Combination | Required Evidence |
|-------------|-------------------|
| **1** | 1 Strong + 1 Supporting |
| **2** | 2 Supporting |

## Population Frequency Thresholds

| Database | BA1 Threshold | BS1 Threshold |
|----------|---------------|---------------|
| gnomAD (global) | >5% | >1% |
| gnomAD (subpopulation) | >5% | Disease-specific |

## MCP Tool Mapping

| ACMG Criterion | Primary MCP Tool |
|----------------|------------------|
| PVS1, PM4 | `classify_variant` (variant type analysis) |
| PS1, PM5 | `query_clinvar` (same AA change lookup) |
| PM2, BA1, BS1 | `query_gnomad` (population frequency) |
| PS3, BS3 | `query_evidence` (functional studies) |
| PP3, BP4 | `classify_variant` (computational predictors) |
| PS4 | `query_clinvar`, `query_cosmic` (prevalence) |

## References

- Richards S, et al. (2015) Standards and guidelines for the interpretation of sequence variants. Genet Med. 17(5):405-24.
- ClinGen Sequence Variant Interpretation Working Group recommendations
