# Clinical Workflows for ACMG/AMP MCP Server

This directory contains comprehensive clinical workflow examples demonstrating how to use the ACMG/AMP MCP Server with AI agents for genetic variant interpretation.

## Available Workflows

### Core Classification Workflows

#### 1. [Basic Classification](./basic-classification.md) ⭐
**Most Common Workflow**
- Single variant classification using ACMG/AMP guidelines
- Step-by-step evidence gathering and criteria application
- Clinical report generation
- **Use Case**: Routine clinical variant interpretation
- **Time**: 2-5 minutes per variant

#### 2. [Batch Classification](./batch-classification.md) 
**High-Throughput Processing**
- Multiple variant processing from sequencing data
- Optimized batch processing strategies
- Quality control and error handling
- **Use Case**: Laboratory processing, research studies
- **Time**: 5-15 minutes for 10-100 variants

### Specialized Workflows

#### 3. Family Analysis (Coming Soon)
- Segregation analysis with family pedigrees
- Co-segregation evidence evaluation
- Inherited vs. de novo variant assessment

#### 4. Pharmacogenomics (Coming Soon)
- Drug response variant interpretation
- Dosing recommendations based on genotype
- Drug-drug-gene interaction analysis

#### 5. Complex Cases (Coming Soon)
- Challenging interpretation scenarios
- Conflicting evidence resolution
- Novel variant assessment strategies

#### 6. Quality Control (Coming Soon)
- Result validation procedures
- Inter-laboratory comparison
- Classification audit workflows

## Workflow Categories

### By User Type

**Clinical Geneticists**
- Basic Classification (routine cases)
- Family Analysis (pedigree analysis)
- Complex Cases (challenging interpretations)

**Laboratory Personnel**
- Batch Classification (high throughput)
- Quality Control (result validation)
- Basic Classification (standard processing)

**Genetic Counselors**
- Basic Classification (patient consultation)
- Family Analysis (inheritance patterns)
- Pharmacogenomics (treatment guidance)

**Researchers**
- Batch Classification (population studies)
- Complex Cases (novel variants)
- Quality Control (method validation)

### By Complexity Level

**Beginner** 🟢
- Basic Classification
- Standard batch processing

**Intermediate** 🟡  
- Family Analysis
- Quality Control procedures
- Pharmacogenomics

**Advanced** 🔴
- Complex Cases
- Custom workflow development
- Research applications

### By Processing Time

**Quick (< 5 minutes)** ⚡
- Basic Classification
- Simple batch processing (< 10 variants)

**Standard (5-15 minutes)** ⏱️
- Medium batch processing (10-100 variants)
- Family Analysis
- Quality Control

**Extended (> 15 minutes)** 🕐
- Large batch processing (> 100 variants)
- Complex research workflows
- Comprehensive quality audits

## Getting Started

### 1. Choose Your Workflow

Select the appropriate workflow based on:
- **Number of variants**: Single vs. batch
- **Clinical context**: Diagnostic vs. research
- **Available time**: Quick vs. comprehensive
- **User experience**: Beginner vs. advanced

### 2. Prerequisites Check

Ensure you have:
- ✅ ACMG/AMP MCP server running
- ✅ AI agent connected (Claude Desktop, custom client)
- ✅ External API keys configured
- ✅ Input data in correct format
- ✅ Clinical context information

### 3. Follow the Workflow

Each workflow provides:
- **Step-by-step instructions**
- **Expected MCP tool calls**
- **Sample inputs and outputs**
- **Quality check procedures**
- **Troubleshooting guidance**

## Common Elements Across Workflows

### Standard Input Formats
```json
{
  "variant_data": {
    "hgvs": "NM_000492.3:c.1521_1523delCTT",
    "gene": "CFTR", 
    "transcript": "NM_000492.3",
    "clinical_context": ["cystic_fibrosis"]
  },
  "options": {
    "include_evidence": true,
    "confidence_threshold": 0.8,
    "report_format": "clinical"
  }
}
```

### Standard Output Structure
```json
{
  "classification": "Pathogenic",
  "confidence": 0.95,
  "applied_criteria": ["PVS1", "PS3", "PM2"],
  "evidence_summary": {},
  "clinical_recommendations": [],
  "report": {}
}
```

### Quality Assurance Steps

All workflows include:

1. **Input Validation**
   - HGVS notation verification
   - Gene symbol standardization
   - Clinical context review

2. **Evidence Assessment**
   - Multiple database queries
   - Evidence quality scoring
   - Source reliability evaluation

3. **Classification Review**
   - ACMG/AMP criteria application
   - Confidence score calculation
   - Classification consistency check

4. **Output Verification**
   - Report completeness
   - Recommendation appropriateness
   - Clinical relevance assessment

## Workflow Customization

### Custom Prompts

Use MCP prompts to customize AI agent behavior:

```json
{
  "tool": "prompts/get",
  "arguments": {
    "name": "clinical_interpretation",
    "arguments": {
      "specialty": "cardiology",
      "patient_age": "pediatric",
      "analysis_depth": "comprehensive"
    }
  }
}
```

### Parameter Tuning

Adjust workflow parameters:

```json
{
  "classification_options": {
    "confidence_threshold": 0.7,
    "require_functional_evidence": true,
    "population_frequency_cutoff": 0.01,
    "prioritize_recent_evidence": true
  }
}
```

### Custom Tools Integration

Extend workflows with custom tools:

```python
# Python example
async def custom_workflow_step(variant_data):
    # Custom analysis logic
    result = await custom_analysis(variant_data)
    
    # Integrate with MCP workflow
    classification = await acmg_client.classify_variant(
        variant_data, 
        custom_evidence=result
    )
    
    return classification
```

## Error Handling Patterns

### Common Error Scenarios

1. **Invalid Input Data**
   - Malformed HGVS notation
   - Unrecognized gene symbols
   - Missing required fields

2. **External API Issues**
   - Rate limit exceeded
   - Service unavailability
   - Authentication failures

3. **Classification Uncertainties**
   - Conflicting evidence
   - Insufficient data
   - Novel variants

### Standard Error Responses

```json
{
  "error": {
    "code": "INVALID_HGVS",
    "message": "Invalid HGVS notation format",
    "suggestions": ["Check transcript version", "Verify nucleotide positions"],
    "retry_possible": false
  }
}
```

## Performance Guidelines

### Optimization Tips

1. **Batch Processing**
   - Group similar variants together
   - Use optimal batch sizes (10-50)
   - Implement parallel processing

2. **Caching Strategy**
   - Enable evidence caching
   - Reuse recent classifications
   - Cache frequently accessed data

3. **Resource Management**
   - Monitor API quotas
   - Implement rate limiting
   - Use connection pooling

### Performance Metrics

Track these metrics:
- **Processing Time**: Average time per variant
- **Success Rate**: Percentage of successful classifications
- **API Usage**: External database query counts
- **Cache Hit Rate**: Percentage of cached responses used

## Integration Examples

### Electronic Health Records (EHR)
```python
# EHR integration example
patient_variants = ehr_system.get_patient_variants(patient_id)
for variant in patient_variants:
    classification = await acmg_client.classify_variant(variant)
    ehr_system.update_patient_record(patient_id, classification)
```

### Laboratory Information Systems (LIS)
```javascript
// LIS integration example
const pendingVariants = await lis.getPendingVariants();
const results = await mcpClient.batchClassify(pendingVariants);
await lis.updateVariantResults(results);
```

### Research Databases
```bash
# Research pipeline integration
./mcp-batch-processor variants.vcf > classifications.json
python3 update_research_db.py classifications.json
```

## Compliance and Documentation

### Clinical Documentation Requirements

Each workflow ensures:
- **Traceability**: Complete audit trail of decisions
- **Reproducibility**: Consistent results with same inputs  
- **Transparency**: Clear explanation of applied criteria
- **Validation**: Quality checks and verification steps

### Regulatory Compliance

Workflows support:
- **CAP/CLIA Requirements**: Laboratory quality standards
- **ACMG/AMP Guidelines**: Current classification standards
- **HIPAA Compliance**: Patient data protection
- **FDA Guidance**: Clinical interpretation best practices

## Contributing New Workflows

To add new workflows:

1. **Create workflow document** following the established template
2. **Include practical examples** with real variant data
3. **Add quality control steps** for validation
4. **Provide troubleshooting guidance** for common issues
5. **Test with multiple AI agents** to ensure compatibility

### Workflow Template Structure

```markdown
# Workflow Name

## Overview
- Objective
- Target Users  
- Estimated Time

## Prerequisites
- Technical requirements
- Data requirements

## Step-by-Step Process
- Detailed instructions
- Expected tool calls
- Sample outputs

## Quality Control
- Validation steps
- Error handling

## Examples
- Usage scenarios
- Integration samples

## Troubleshooting
- Common issues
- Solutions
```

## Support and Resources

- **Documentation**: Detailed guides for each workflow
- **Examples**: Complete working code samples
- **Community**: Discussion forum for questions
- **Training**: Video tutorials and workshops

For additional support:
- Check the troubleshooting guides
- Review the FAQ section
- Contact the development team
- Join the user community forum