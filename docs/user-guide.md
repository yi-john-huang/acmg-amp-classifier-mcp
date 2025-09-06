# ACMG/AMP MCP Server - User Guide

Complete user guide for clinicians, genetic counselors, and laboratory personnel using the ACMG/AMP MCP Server with AI agents.

## Table of Contents

1. [Getting Started](#getting-started)
2. [Basic Usage](#basic-usage)
3. [Clinical Workflows](#clinical-workflows)
4. [Advanced Features](#advanced-features)
5. [Best Practices](#best-practices)
6. [Troubleshooting](#troubleshooting)

---

## Getting Started

### What is the ACMG/AMP MCP Server?

The ACMG/AMP MCP Server transforms genetic variant classification into a natural conversation with AI agents. Instead of navigating complex interfaces, you can simply describe your case and receive comprehensive ACMG/AMP classifications with supporting evidence.

### Prerequisites

**For Clinical Users**:
- Basic understanding of ACMG/AMP classification guidelines
- Access to genetic variant data (HGVS notation)
- AI agent with MCP support (Claude Desktop recommended)

**System Requirements**:
- Internet connection for external database access
- Modern web browser (for web-based clients)
- Claude Desktop application (for optimal experience)

### Initial Setup

#### Option 1: Claude Desktop (Recommended)

1. **Install Claude Desktop** from Anthropic's website
2. **Configure MCP Server** by adding to your Claude Desktop settings:

```json
{
  "mcpServers": {
    "acmg-amp-classifier": {
      "command": "/path/to/mcp-server",
      "args": ["--config", "/path/to/config.yaml"],
      "env": {
        "DATABASE_URL": "your-database-url",
        "CLINVAR_API_KEY": "your-api-key",
        "GNOMAD_API_KEY": "your-api-key"
      }
    }
  }
}
```

3. **Restart Claude Desktop**
4. **Test the connection**:
   ```
   "Hello! Can you help me classify genetic variants?"
   ```

#### Option 2: Web Interface

1. **Access the web interface** at your organization's ACMG/AMP server URL
2. **Log in** with your clinical credentials
3. **Complete initial setup** by verifying your role and permissions

#### Option 3: API Integration

For custom implementations, see the [API Documentation](./api-documentation.md).

---

## Basic Usage

### Your First Variant Classification

Let's classify the well-known F508del CFTR variant:

**You**: 
> "I need to classify the variant NM_000492.3:c.1521_1523delCTT found in a 6-month-old patient with failure to thrive and recurrent respiratory infections."

**AI Response**:
> I'll help you classify this CFTR variant using ACMG/AMP guidelines. This appears to be the F508del mutation, which is the most common cause of cystic fibrosis.
>
> Let me gather evidence and perform the classification...
>
> **Classification Result**: Pathogenic (High Confidence: 98%)
>
> **Applied ACMG/AMP Criteria**:
> - **PVS1**: Loss of function variant in gene where LOF causes disease
> - **PS3**: Well-established functional studies show damaging effect  
> - **PM2**: Absent from population databases (0/251,472 alleles in gnomAD)
> - **PP3**: Computational predictions support deleterious effect
> - **PP5**: Reputable sources report as pathogenic (ClinVar: 4-star rating)
>
> **Clinical Significance**: This variant confirms the diagnosis of cystic fibrosis. The patient is eligible for CFTR modulator therapy (Trikafta®) which can be started as early as 6 months of age.

### Understanding the Response

Every classification includes:

1. **Classification** (Pathogenic, Likely Pathogenic, Uncertain Significance, Likely Benign, Benign)
2. **Confidence Score** (0-100%)
3. **Applied Criteria** (ACMG/AMP evidence codes)
4. **Clinical Significance** (Real-world implications)
5. **Recommendations** (Next steps and treatment considerations)

### Basic Commands

| What to Ask | Example |
|-------------|---------|
| **Simple Classification** | "Classify the variant NM_007294.3:c.185delA" |
| **With Clinical Context** | "Classify BRCA1 c.185delA in a 35-year-old woman with family history of breast cancer" |
| **HGVS Validation** | "Is this HGVS notation correct: NM_000492.3:c.1521_1523delCTT?" |
| **Evidence Gathering** | "What evidence is available for the CFTR F508del variant?" |
| **Report Generation** | "Generate a clinical report for this classification" |

---

## Clinical Workflows

### Diagnostic Genetics Workflow

#### Step 1: Case Presentation
**You**:
> "I have a 2-year-old patient with developmental delay, seizures, and dysmorphic features. Exome sequencing identified a variant in SCN1A: NM_001165963.1:c.3184A>T. Can you help classify this variant?"

#### Step 2: AI Analysis
The AI agent will:
1. Validate the HGVS notation
2. Gather evidence from multiple databases
3. Apply ACMG/AMP criteria systematically
4. Calculate confidence scores
5. Provide clinical interpretation

#### Step 3: Results Review
**AI Response**:
> **Variant**: SCN1A NM_001165963.1:c.3184A>T (p.Ile1062Phe)
> **Classification**: Likely Pathogenic (Confidence: 87%)
> 
> **Evidence Summary**:
> - Located in critical sodium channel domain
> - Absent from population databases
> - Computational predictions strongly support pathogenicity
> - Similar variants reported as pathogenic in literature
>
> **Clinical Implications**: 
> - Consistent with Dravet syndrome phenotype
> - Recommend genetic counseling
> - Consider antiepileptic drug selection based on SCN1A status
> - Family cascade screening indicated

#### Step 4: Documentation
**You**:
> "Please generate a formal clinical report for the medical record"

### Hereditary Cancer Risk Assessment

#### Family History Integration
**You**:
> "I'm evaluating a 45-year-old woman with breast cancer at age 40. Her mother had ovarian cancer at 50, and her maternal grandmother had breast cancer at 45. We found BRCA1 c.5266dupC. Please classify and provide family counseling recommendations."

**AI Analysis Process**:
1. **Variant Classification**: Using population data, functional studies, and clinical observations
2. **Penetrance Assessment**: Age-specific risk calculations
3. **Family Risk Analysis**: Inheritance patterns and cascade testing recommendations
4. **Management Guidelines**: Screening and prevention strategies

### Pediatric Genetics Consultation

#### Developmental Delay Cases
**You**:
> "8-month-old infant with global developmental delay and feeding difficulties. Array CGH was normal, but trio exome found a de novo variant in MECP2: NM_004992.3:c.916C>T in a male patient. What's the significance?"

**AI Considerations**:
- X-linked inheritance patterns
- Gender-specific penetrance
- Phenotype correlation
- Recurrence risk counseling
- Early intervention recommendations

### Laboratory Quality Control

#### Batch Variant Processing
**You**:
> "I have a batch of 25 variants from our weekly sequencing run that need ACMG/AMP classification. Can you process them systematically?"

**AI Workflow**:
1. **Input Validation**: Check all HGVS notations
2. **Batch Processing**: Classify all variants efficiently
3. **Quality Review**: Flag uncertain or conflicting classifications
4. **Summary Report**: Provide overview of all classifications
5. **Priority Cases**: Highlight variants needing manual review

---

## Advanced Features

### Custom Classification Criteria

#### Modifying Evidence Strength
**You**:
> "For this BRCA1 variant, I want to consider the family history as stronger evidence than typical. The patient has 3 first-degree relatives with breast cancer under age 45."

**AI Response**:
> I understand you want to weight the family segregation evidence more heavily. Let me recalculate using PP1 (cosegregation) as moderate evidence rather than supporting, given the strong family history...

#### Gene-Specific Considerations
**You**:
> "This is a truncating variant in BRCA2 exon 27, which is in the ovarian cancer cluster region. How does this affect the classification?"

### Population-Specific Analysis

#### Ancestry Considerations
**You**:
> "The patient is of Ashkenazi Jewish ancestry. Please consider founder effects and population-specific frequencies for this BRCA1 variant."

**AI Analysis**:
- Uses Ashkenazi-specific allele frequencies
- Considers founder mutations
- Adjusts pathogenicity assessment accordingly
- Provides ancestry-specific risk estimates

### Research Applications

#### Novel Variant Assessment
**You**:
> "We've identified a novel missense variant in a known disease gene. It's not in any databases. How should we approach the classification?"

**AI Guidance**:
1. **Computational Predictions**: Multiple algorithm consensus
2. **Functional Domain Analysis**: Critical region assessment  
3. **Conservation Analysis**: Cross-species comparison
4. **Literature Mining**: Related variants and studies
5. **Functional Study Recommendations**: Suggested experimental approaches

### Multi-Gene Panel Interpretation

#### Comprehensive Analysis
**You**:
> "Our hereditary cancer panel identified variants in three genes: BRCA1 c.185delA, CHEK2 c.1100delC, and ATM c.7271T>G. Please provide a comprehensive risk assessment."

**AI Integration**:
- Individual variant classifications
- Combined risk assessment
- Gene interaction considerations
- Management recommendation synthesis
- Family counseling implications

---

## Best Practices

### Effective Communication with AI Agents

#### Provide Context
```
✅ Good: "45-year-old woman with strong family history of breast cancer, found to have BRCA1 c.185delA"

❌ Poor: "BRCA1 c.185delA"
```

#### Specify Your Needs
```
✅ Good: "Please classify this variant and generate a report suitable for genetic counseling"

❌ Poor: "What about this variant?"
```

#### Ask Follow-up Questions
```
✅ Good: "Can you explain why PM2 was applied? What was the population frequency data?"

❌ Poor: "OK" (missing opportunity to understand the reasoning)
```

### Clinical Documentation

#### Standard Information to Include
1. **Patient Demographics**: Age, sex, ancestry (when relevant)
2. **Clinical Presentation**: Symptoms, family history, indication for testing
3. **Variant Details**: HGVS notation, gene, transcript used
4. **Classification Results**: Category, confidence, criteria applied
5. **Clinical Interpretation**: Disease association, prognosis, management
6. **Recommendations**: Follow-up, family testing, treatment considerations

#### Sample Documentation
```
Patient: 35-year-old female of European ancestry
Indication: Personal history of breast cancer at age 33, family history of ovarian cancer
Variant: BRCA1 NM_007294.3:c.185delA (p.Gln62HisfsX19)
Classification: Pathogenic (Confidence: 99%)
Applied Criteria: PVS1, PM2, PP5
Clinical Interpretation: Pathogenic variant associated with hereditary breast and ovarian cancer syndrome
Recommendations: Enhanced screening per NCCN guidelines, discuss risk-reducing surgery, cascade family testing
```

### Quality Assurance

#### Double-Check Critical Results
- Verify HGVS notation accuracy
- Confirm gene-disease associations
- Review population frequency data
- Cross-reference with ClinVar when available

#### When to Seek Additional Review
- Uncertain Significance classifications
- Conflicting evidence sources
- Novel or extremely rare variants
- Complex inheritance patterns
- Discordant clinical and molecular findings

### Staying Current

#### Regular Updates
The ACMG/AMP server automatically incorporates:
- Updated database information
- New literature findings
- Revised classification guidelines
- Population frequency updates

#### Continuing Education
- Review reclassified variants in your patient population
- Stay informed about gene-disease associations
- Participate in proficiency testing programs
- Engage with professional genetics societies

---

## Common Use Cases

### Case 1: Confirmatory Testing

**Scenario**: Patient with clinical diagnosis needs genetic confirmation

**Approach**:
1. Describe clinical presentation
2. Provide family history context
3. Request classification with clinical correlation
4. Ask for management recommendations

**Example**:
> "Patient with classic cystic fibrosis presentation - failure to thrive, pancreatic insufficiency, elevated sweat chloride. Found to have CFTR F508del homozygous. Please confirm pathogenicity and discuss treatment options."

### Case 2: Incidental Findings

**Scenario**: Unexpected variant found during broad genetic testing

**Approach**:
1. Classify the variant independently
2. Assess clinical relevance
3. Determine disclosure appropriateness
4. Plan follow-up if needed

**Example**:
> "During exome sequencing for developmental delay, we found a BRCA2 variant c.8851G>T in a 10-year-old patient. Please classify and advise on disclosure considerations."

### Case 3: Family Cascade Testing

**Scenario**: Testing relatives of known variant carriers

**Approach**:
1. Confirm the familial variant classification
2. Assess penetrance and risk
3. Provide age-appropriate counseling
4. Plan surveillance recommendations

**Example**:
> "Testing the 25-year-old daughter of a woman with pathogenic BRCA1 c.185delA. The daughter tested positive. Please provide age-specific risk assessment and screening recommendations."

### Case 4: Prenatal Diagnosis

**Scenario**: Fetal testing for known familial variants

**Approach**:
1. Confirm variant pathogenicity
2. Assess penetrance and severity
3. Provide prenatal counseling information
4. Discuss reproductive options

**Example**:
> "Prenatal testing for Huntington disease. Fetus positive for HTT c.52_54CAG[42]. Please confirm pathogenicity and expected age of onset for counseling."

### Case 5: Pharmacogenomics

**Scenario**: Drug response variants affecting treatment

**Approach**:
1. Classify pharmacogenomic significance
2. Assess drug interactions
3. Provide dosing recommendations
4. Monitor for adverse effects

**Example**:
> "Patient with depression needs antipsychotic medication. Genotyping shows CYP2D6*4/*4 (poor metabolizer). Please advise on medication selection and dosing."

---

## Troubleshooting

### Common Issues and Solutions

#### Issue: "Invalid HGVS notation"
**Problem**: The variant format is not recognized
**Solution**: 
1. Use the HGVS validation tool first
2. Ensure transcript version is included
3. Check for typos in the notation
4. Try alternative transcript references

**Example Fix**:
```
❌ "c.185delA" 
✅ "NM_007294.3:c.185delA"
```

#### Issue: "No evidence found"
**Problem**: AI reports insufficient data for classification
**Solution**:
1. Try alternative variant representations
2. Search by protein change instead
3. Check gene symbol spelling
4. Consider using broader search terms

**Example Fix**:
```
❌ "NM_000492.2:c.1521_1523delCTT" (old transcript)
✅ "NM_000492.3:c.1521_1523delCTT" (current transcript)
```

#### Issue: "Conflicting classifications"
**Problem**: Different sources show different classifications
**Solution**:
1. Review the evidence quality
2. Check submission dates
3. Consider reviewer expertise
4. Focus on expert panel reviews when available

#### Issue: "Low confidence score"
**Problem**: Classification has low confidence
**Solution**:
1. Gather additional evidence
2. Consider functional studies
3. Review family segregation
4. Seek expert consultation

#### Issue: "Server timeout or slow response"
**Problem**: Long delays in getting results
**Solution**:
1. Check internet connection
2. Try simpler queries first
3. Break complex requests into parts
4. Contact system administrator if persistent

### Error Messages

| Error | Meaning | Solution |
|-------|---------|----------|
| `HGVS_INVALID` | Variant notation is malformed | Check HGVS syntax |
| `GENE_NOT_FOUND` | Gene symbol not recognized | Verify gene name spelling |
| `NO_TRANSCRIPT` | Transcript not found | Use current RefSeq transcript |
| `API_LIMIT_EXCEEDED` | Too many requests | Wait and retry |
| `DATABASE_ERROR` | External database unavailable | Try again later |

### Getting Help

#### Self-Service Resources
1. **Validation Tools**: Use built-in HGVS checker
2. **Example Queries**: Review provided examples
3. **Documentation**: Check user guide and API docs
4. **FAQ**: Common questions and answers

#### Professional Support
1. **Genetics Consultation**: For complex cases
2. **Technical Support**: For system issues
3. **Training Resources**: Educational materials
4. **User Community**: Peer discussions and tips

#### Contact Information
- **Technical Support**: support@acmg-amp-server.org
- **Clinical Questions**: genetics-support@acmg-amp-server.org
- **Training**: training@acmg-amp-server.org
- **Emergency**: 24/7 hotline for critical issues

---

## Appendices

### Appendix A: ACMG/AMP Criteria Quick Reference

#### Pathogenic Criteria
- **PVS1**: Null variant (LOF) in disease gene
- **PS1-4**: Strong evidence (functional, segregation, etc.)
- **PM1-6**: Moderate evidence (domain, frequency, etc.)
- **PP1-5**: Supporting evidence (predictions, literature, etc.)

#### Benign Criteria  
- **BA1**: High frequency in populations
- **BS1-4**: Strong benign evidence
- **BP1-7**: Supporting benign evidence

### Appendix B: Common Gene-Disease Associations

| Gene | Disease | Inheritance | Key Features |
|------|---------|-------------|--------------|
| BRCA1/2 | Hereditary breast/ovarian cancer | AD | Early onset, family history |
| CFTR | Cystic fibrosis | AR | Respiratory, GI symptoms |
| HTT | Huntington disease | AD | Progressive neurodegeneration |
| SCN1A | Dravet syndrome | AD | Severe epilepsy, developmental delay |
| FBN1 | Marfan syndrome | AD | Connective tissue, cardiac |

### Appendix C: Population Frequency Thresholds

| Inheritance | BA1 Threshold | BS1 Threshold |
|-------------|---------------|---------------|
| **Autosomal Dominant** | >0.1% | >0.01% |
| **Autosomal Recessive** | >1% | >0.1% |
| **X-linked** | >0.01% (males) | >0.001% (males) |

### Appendix D: External Resources

- **ClinVar**: https://www.ncbi.nlm.nih.gov/clinvar/
- **gnomAD**: https://gnomad.broadinstitute.org/
- **ACMG Guidelines**: https://www.acmg.net/
- **ClinGen**: https://clinicalgenome.org/
- **HGVS Nomenclature**: https://varnomen.hgvs.org/

---

**Document Version**: 1.0  
**Last Updated**: January 15, 2024  
**Next Review**: July 15, 2024

For the latest updates and additional resources, visit the [ACMG/AMP MCP Server Documentation Portal](./README.md).