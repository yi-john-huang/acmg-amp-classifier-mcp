# Clinical Context Guide

## Overview

This guide provides clinical context for interpreting ACMG/AMP variant classifications. Understanding clinical context is essential for appropriate use of classification results.

## Important Disclaimers

### Research Use Only

**This software is for research and educational purposes only.**

- NOT approved for clinical diagnostic use
- NOT a medical device
- Results require professional interpretation
- Should not be the sole basis for medical decisions
- Clinical use requires regulatory approval

### Professional Oversight Required

All variant classifications should be reviewed by:
- Board-certified clinical geneticists
- Molecular pathologists
- Genetic counselors (for patient communication)

## Clinical Interpretation Framework

### 1. Patient Context Matters

Classification should consider:
- **Phenotype specificity** (PP4 criterion)
- **Family history** (PP1, PS2 criteria)
- **Inheritance pattern** (dominant vs recessive)
- **Penetrance expectations** (BS2 criterion)
- **Age of onset** for the condition

### 2. Gene-Disease Relationship

Consider:
- **Gene-disease validity** (ClinGen assertions)
- **Mechanism of disease** (loss-of-function vs gain-of-function)
- **Allelic requirements** (monoallelic vs biallelic)

### 3. Population-Specific Considerations

Population frequency thresholds may vary:
- Some variants are population-specific
- Founder mutations may be common in specific populations
- gnomAD subpopulation data should be reviewed

## Classification Confidence

### High Confidence Scenarios

- Established pathogenic variants in ClinVar (multiple submitters)
- Null variants in genes with clear LOF mechanism
- Very high or absent population frequency

### Lower Confidence Scenarios

- Novel variants without prior reports
- Conflicting computational predictions
- Limited population frequency data
- VUS with partial evidence

## Using Classifications

### For Pathogenic/Likely Pathogenic

1. **Confirm gene-disease relationship** for patient's phenotype
2. **Consider penetrance** - not all carriers develop disease
3. **Family testing** may be indicated for at-risk relatives
4. **Genetic counseling** is essential before disclosure

### For VUS (Uncertain Significance)

1. **Do not use** for clinical decision-making
2. **Document** in patient record with uncertainty noted
3. **Consider re-classification** as evidence accumulates
4. **Avoid cascade testing** of family members
5. **Periodic re-evaluation** recommended (1-2 years)

### For Benign/Likely Benign

1. **Generally exclude** from diagnostic consideration
2. **Document** classification basis
3. **Consider alternative diagnoses** if phenotype persists

## Report Components

### Essential Elements

Every clinical report should include:

1. **Variant identification**
   - HGVS notation (genomic and coding)
   - Gene symbol and transcript
   - Genomic coordinates (GRCh37/38)

2. **Classification**
   - ACMG/AMP category
   - Evidence summary (rules applied)
   - Confidence level

3. **Clinical significance**
   - Relationship to patient phenotype
   - Inheritance pattern
   - Recommendations

4. **Limitations**
   - Databases queried and versions
   - Any missing evidence
   - Appropriate disclaimers

### Template Sections

```markdown
## Variant Classification Report

### Variant Information
- Gene: [GENE]
- Transcript: [NM_XXXXX.X]
- HGVS (coding): [c.XXX]
- HGVS (protein): [p.XXX]
- Classification: [Pathogenic/Likely Pathogenic/VUS/Likely Benign/Benign]

### Evidence Summary
[Rule-by-rule breakdown]

### Clinical Interpretation
[Context-specific interpretation]

### Recommendations
[Clinical next steps]

### Limitations and Disclaimers
[Standard disclaimers]
```

## Gene-Specific Considerations

### High-Penetrance Cancer Genes (BRCA1, BRCA2, TP53, etc.)

- Pathogenic variants have significant clinical implications
- Established management guidelines exist (NCCN)
- Cascade testing of relatives is standard of care
- Risk-reducing interventions may be indicated

### Cardiac Genes (MYBPC3, SCN5A, etc.)

- Penetrance is often reduced
- Multiple family members may need clinical screening
- Variants may have variable expressivity

### Pharmacogenomic Variants

- Classification may differ from disease-causing assessment
- Consider drug-gene interactions
- CPIC guidelines provide dosing recommendations

## Evidence Quality Assessment

### Stronger Evidence

- Multiple independent submitters in ClinVar
- Functional studies in peer-reviewed literature
- Large, well-powered case-control studies
- Confirmed de novo occurrence

### Weaker Evidence

- Single submitter reports
- In silico predictions only
- Small sample sizes
- Assumed (not confirmed) de novo

## Re-classification Considerations

Variants should be re-evaluated when:

- New population frequency data becomes available
- Functional studies are published
- Gene-disease relationships are updated
- ACMG/AMP guidelines are modified
- Patient phenotype changes or new information emerges

## Resources

### Guidelines

- ACMG/AMP 2015 guidelines (PMID: 25741868)
- ClinGen gene-disease validity: https://clinicalgenome.org
- NCCN guidelines: https://www.nccn.org

### Databases

- ClinVar: https://www.ncbi.nlm.nih.gov/clinvar/
- gnomAD: https://gnomad.broadinstitute.org
- OMIM: https://omim.org

### Professional Organizations

- ACMG: https://www.acmg.net
- NSGC (Genetic Counselors): https://www.nsgc.org
- AMP: https://www.amp.org
