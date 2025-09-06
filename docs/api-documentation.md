# ACMG/AMP MCP Server - API Documentation

Complete API reference for all MCP tools, resources, and prompts provided by the ACMG/AMP classification server.

## Protocol Information

- **Protocol**: Model Context Protocol (MCP) 2024-11-05
- **Transport**: stdio, HTTP with Server-Sent Events, WebSocket
- **Message Format**: JSON-RPC 2.0
- **Authentication**: API key-based (for HTTP/WebSocket transports)

## Server Capabilities

### Supported MCP Features

```json
{
  "capabilities": {
    "tools": {
      "listChanged": true
    },
    "resources": {
      "subscribe": true,
      "listChanged": true
    },
    "prompts": {
      "listChanged": true
    },
    "logging": {
      "level": ["error", "warn", "info", "debug"]
    }
  },
  "serverInfo": {
    "name": "acmg-amp-mcp-server",
    "version": "1.0.0",
    "protocolVersion": "2024-11-05"
  }
}
```

---

## MCP Tools

Tools are active functions that AI agents can invoke to perform specific operations.

### Core Classification Tools

#### classify_variant

**Description**: Complete ACMG/AMP variant classification workflow including evidence gathering, criteria application, and confidence scoring.

**Input Schema**:
```json
{
  "type": "object",
  "properties": {
    "variant_data": {
      "type": "object",
      "properties": {
        "hgvs": {
          "type": "string",
          "description": "HGVS notation of the variant",
          "pattern": "^N[MR]_\\d+\\.\\d+:[cgmnpr]\\."
        },
        "gene": {
          "type": "string", 
          "description": "Gene symbol (optional but recommended)"
        },
        "chromosome": {
          "type": "string",
          "description": "Chromosome identifier"
        },
        "position": {
          "type": "integer",
          "description": "Genomic position"
        },
        "ref": {
          "type": "string",
          "description": "Reference allele"
        },
        "alt": {
          "type": "string", 
          "description": "Alternative allele"
        },
        "transcript": {
          "type": "string",
          "description": "Specific transcript identifier"
        }
      },
      "required": ["hgvs"]
    },
    "options": {
      "type": "object",
      "properties": {
        "include_evidence": {
          "type": "boolean",
          "default": true,
          "description": "Include detailed evidence in response"
        },
        "confidence_threshold": {
          "type": "number",
          "minimum": 0.0,
          "maximum": 1.0,
          "default": 0.8,
          "description": "Minimum confidence threshold for classification"
        },
        "clinical_context": {
          "type": "array",
          "items": {"type": "string"},
          "description": "Clinical phenotypes or conditions"
        },
        "population_ancestry": {
          "type": "string",
          "enum": ["AFR", "AMR", "ASJ", "EAS", "FIN", "NFE", "SAS", "all"],
          "default": "all",
          "description": "Population ancestry for frequency analysis"
        }
      }
    }
  },
  "required": ["variant_data"]
}
```

**Output Schema**:
```json
{
  "type": "object",
  "properties": {
    "classification": {
      "type": "string",
      "enum": ["Pathogenic", "Likely Pathogenic", "Uncertain Significance", "Likely Benign", "Benign"]
    },
    "confidence": {
      "type": "number",
      "minimum": 0.0,
      "maximum": 1.0,
      "description": "Classification confidence score"
    },
    "applied_criteria": {
      "type": "array",
      "items": {"type": "string"},
      "description": "List of ACMG/AMP criteria applied"
    },
    "evidence_summary": {
      "type": "object",
      "description": "Detailed evidence for each applied criterion"
    },
    "acmg_points": {
      "type": "object",
      "properties": {
        "pathogenic_very_strong": {"type": "integer"},
        "pathogenic_strong": {"type": "integer"},
        "pathogenic_moderate": {"type": "integer"},
        "pathogenic_supporting": {"type": "integer"},
        "benign_strong": {"type": "integer"},
        "benign_supporting": {"type": "integer"}
      }
    },
    "clinical_significance": {
      "type": "string",
      "description": "Clinical interpretation of the classification"
    },
    "recommendation": {
      "type": "string",
      "description": "Clinical recommendations based on classification"
    },
    "last_updated": {
      "type": "string",
      "format": "date-time"
    }
  }
}
```

**Example Usage**:
```json
{
  "tool": "classify_variant",
  "arguments": {
    "variant_data": {
      "hgvs": "NM_000492.3:c.1521_1523delCTT",
      "gene": "CFTR"
    },
    "options": {
      "include_evidence": true,
      "confidence_threshold": 0.9,
      "clinical_context": ["cystic_fibrosis"]
    }
  }
}
```

#### validate_hgvs

**Description**: Validate and normalize HGVS variant notation.

**Input Schema**:
```json
{
  "type": "object", 
  "properties": {
    "hgvs": {
      "type": "string",
      "description": "HGVS notation to validate"
    },
    "strict": {
      "type": "boolean",
      "default": true,
      "description": "Use strict validation rules"
    }
  },
  "required": ["hgvs"]
}
```

**Output Schema**:
```json
{
  "type": "object",
  "properties": {
    "valid": {
      "type": "boolean",
      "description": "Whether HGVS notation is valid"
    },
    "normalized": {
      "type": "string",
      "description": "Normalized HGVS notation"
    },
    "gene": {
      "type": "string",
      "description": "Gene symbol derived from transcript"
    },
    "transcript": {
      "type": "string", 
      "description": "Transcript identifier"
    },
    "variant_type": {
      "type": "string",
      "enum": ["substitution", "deletion", "insertion", "duplication", "inversion", "complex"]
    },
    "protein_change": {
      "type": "string",
      "description": "Predicted protein change (if applicable)"
    },
    "genomic_coordinates": {
      "type": "object",
      "properties": {
        "chromosome": {"type": "string"},
        "start": {"type": "integer"},
        "end": {"type": "integer"},
        "ref": {"type": "string"},
        "alt": {"type": "string"}
      }
    },
    "warnings": {
      "type": "array",
      "items": {"type": "string"},
      "description": "Validation warnings"
    },
    "errors": {
      "type": "array", 
      "items": {"type": "string"},
      "description": "Validation errors"
    }
  }
}
```

#### apply_rule

**Description**: Apply individual ACMG/AMP classification criterion to variant data.

**Input Schema**:
```json
{
  "type": "object",
  "properties": {
    "rule": {
      "type": "string",
      "enum": ["PVS1", "PS1", "PS2", "PS3", "PS4", "PM1", "PM2", "PM3", "PM4", "PM5", "PM6", "PP1", "PP2", "PP3", "PP4", "PP5", "BA1", "BS1", "BS2", "BS3", "BS4", "BP1", "BP2", "BP3", "BP4", "BP5", "BP6", "BP7"],
      "description": "ACMG/AMP criterion to evaluate"
    },
    "variant_data": {
      "type": "object",
      "description": "Same as classify_variant variant_data"
    },
    "evidence_data": {
      "type": "object",
      "description": "Pre-gathered evidence data (optional)"
    }
  },
  "required": ["rule", "variant_data"]
}
```

**Output Schema**:
```json
{
  "type": "object",
  "properties": {
    "rule": {
      "type": "string",
      "description": "Applied ACMG/AMP rule"
    },
    "applicable": {
      "type": "boolean",
      "description": "Whether rule applies to this variant"
    },
    "strength": {
      "type": "string",
      "enum": ["very_strong", "strong", "moderate", "supporting"],
      "description": "Evidence strength level"
    },
    "evidence": {
      "type": "object",
      "description": "Supporting evidence for rule application"
    },
    "rationale": {
      "type": "string", 
      "description": "Human-readable explanation"
    },
    "confidence": {
      "type": "number",
      "minimum": 0.0,
      "maximum": 1.0,
      "description": "Confidence in rule application"
    }
  }
}
```

#### combine_evidence

**Description**: Combine multiple ACMG/AMP criteria according to official combination rules.

**Input Schema**:
```json
{
  "type": "object",
  "properties": {
    "pathogenic_criteria": {
      "type": "object",
      "properties": {
        "very_strong": {"type": "array", "items": {"type": "string"}},
        "strong": {"type": "array", "items": {"type": "string"}},
        "moderate": {"type": "array", "items": {"type": "string"}},
        "supporting": {"type": "array", "items": {"type": "string"}}
      }
    },
    "benign_criteria": {
      "type": "object", 
      "properties": {
        "strong": {"type": "array", "items": {"type": "string"}},
        "supporting": {"type": "array", "items": {"type": "string"}}
      }
    },
    "combination_rules": {
      "type": "string",
      "enum": ["standard", "updated_2018", "custom"],
      "default": "standard",
      "description": "ACMG/AMP combination rules version"
    }
  },
  "required": ["pathogenic_criteria", "benign_criteria"]
}
```

### Evidence Gathering Tools

#### query_evidence

**Description**: Gather evidence for variant from multiple external databases.

**Input Schema**:
```json
{
  "type": "object",
  "properties": {
    "variant": {
      "type": "string",
      "description": "Variant identifier (HGVS, dbSNP, etc.)"
    },
    "databases": {
      "type": "array",
      "items": {
        "type": "string",
        "enum": ["clinvar", "gnomad", "cosmic", "lovd", "hgmd", "all"]
      },
      "default": ["clinvar", "gnomad"],
      "description": "Databases to query"
    },
    "include_predictions": {
      "type": "boolean",
      "default": true,
      "description": "Include computational predictions"
    },
    "include_functional": {
      "type": "boolean", 
      "default": true,
      "description": "Include functional study data"
    }
  },
  "required": ["variant"]
}
```

**Output Schema**:
```json
{
  "type": "object",
  "properties": {
    "variant": {
      "type": "string",
      "description": "Queried variant"
    },
    "sources": {
      "type": "array",
      "items": {"type": "string"},
      "description": "Successfully queried databases"
    },
    "evidence": {
      "type": "object",
      "properties": {
        "clinvar": {
          "type": "object",
          "properties": {
            "variation_id": {"type": "string"},
            "clinical_significance": {"type": "string"},
            "review_status": {"type": "string"},
            "star_rating": {"type": "integer"},
            "last_evaluated": {"type": "string"},
            "submissions": {"type": "integer"},
            "conditions": {"type": "array", "items": {"type": "string"}}
          }
        },
        "gnomad": {
          "type": "object", 
          "properties": {
            "allele_frequency": {"type": "number"},
            "allele_count": {"type": "integer"},
            "allele_number": {"type": "integer"},
            "homozygote_count": {"type": "integer"},
            "populations": {"type": "object"},
            "quality_metrics": {"type": "object"}
          }
        },
        "predictions": {
          "type": "object",
          "properties": {
            "sift": {"type": "object"},
            "polyphen": {"type": "object"},
            "cadd": {"type": "object"},
            "revel": {"type": "object"},
            "spliceai": {"type": "object"}
          }
        },
        "functional_studies": {
          "type": "array",
          "items": {
            "type": "object",
            "properties": {
              "pmid": {"type": "string"},
              "study_type": {"type": "string"},
              "finding": {"type": "string"},
              "evidence_level": {"type": "string"}
            }
          }
        }
      }
    },
    "summary": {
      "type": "object",
      "properties": {
        "evidence_strength": {"type": "string"},
        "pathogenicity_prediction": {"type": "string"},
        "population_frequency": {"type": "number"},
        "functional_impact": {"type": "string"}
      }
    }
  }
}
```

#### query_clinvar

**Description**: Query ClinVar database specifically for variant information.

**Input Schema**:
```json
{
  "type": "object",
  "properties": {
    "variant": {
      "type": "string",
      "description": "Variant identifier"
    },
    "include_submissions": {
      "type": "boolean",
      "default": false,
      "description": "Include individual submissions"
    },
    "include_history": {
      "type": "boolean",
      "default": false, 
      "description": "Include classification history"
    }
  },
  "required": ["variant"]
}
```

#### query_gnomad

**Description**: Query gnomAD population database for allele frequencies.

**Input Schema**:
```json
{
  "type": "object",
  "properties": {
    "variant": {
      "type": "string",
      "description": "Variant identifier"
    },
    "dataset": {
      "type": "string",
      "enum": ["exomes", "genomes", "both"],
      "default": "both",
      "description": "gnomAD dataset to query"
    },
    "populations": {
      "type": "array",
      "items": {
        "type": "string", 
        "enum": ["AFR", "AMR", "ASJ", "EAS", "FIN", "NFE", "SAS"]
      },
      "description": "Specific populations to include"
    }
  },
  "required": ["variant"]
}
```

#### query_cosmic

**Description**: Query COSMIC cancer mutation database.

**Input Schema**:
```json
{
  "type": "object",
  "properties": {
    "variant": {
      "type": "string", 
      "description": "Variant identifier"
    },
    "cancer_types": {
      "type": "array",
      "items": {"type": "string"},
      "description": "Specific cancer types to include"
    },
    "include_resistance": {
      "type": "boolean",
      "default": true,
      "description": "Include drug resistance data"
    }
  },
  "required": ["variant"]
}
```

### Report Generation Tools

#### generate_report

**Description**: Generate comprehensive clinical classification report.

**Input Schema**:
```json
{
  "type": "object",
  "properties": {
    "classification_data": {
      "type": "object",
      "description": "Output from classify_variant tool"
    },
    "format": {
      "type": "string",
      "enum": ["clinical", "research", "brief", "detailed"],
      "default": "clinical",
      "description": "Report format type"
    },
    "sections": {
      "type": "array",
      "items": {
        "type": "string",
        "enum": ["summary", "variant_details", "evidence", "classification", "recommendations", "references", "methodology"]
      },
      "default": ["summary", "classification", "evidence", "recommendations"],
      "description": "Report sections to include"
    },
    "patient_info": {
      "type": "object",
      "properties": {
        "age": {"type": "string"},
        "sex": {"type": "string"},
        "ethnicity": {"type": "string"},
        "phenotype": {"type": "array", "items": {"type": "string"}},
        "family_history": {"type": "string"}
      },
      "description": "Patient information (optional, de-identified)"
    }
  },
  "required": ["classification_data"]
}
```

**Output Schema**:
```json
{
  "type": "object",
  "properties": {
    "report_id": {"type": "string"},
    "generated_date": {"type": "string", "format": "date-time"},
    "report_type": {"type": "string"},
    "content": {
      "type": "object",
      "properties": {
        "summary": {"type": "string"},
        "variant_details": {"type": "object"},
        "classification": {"type": "object"},
        "evidence": {"type": "object"},
        "recommendations": {"type": "object"},
        "references": {"type": "array"}
      }
    },
    "metadata": {
      "type": "object",
      "properties": {
        "server_version": {"type": "string"},
        "classification_date": {"type": "string"},
        "guidelines_version": {"type": "string"}
      }
    }
  }
}
```

#### format_report

**Description**: Format classification report in various output formats.

**Input Schema**:
```json
{
  "type": "object",
  "properties": {
    "report_data": {
      "type": "object",
      "description": "Output from generate_report tool"
    },
    "output_format": {
      "type": "string",
      "enum": ["json", "xml", "html", "pdf", "docx", "txt"],
      "default": "json",
      "description": "Desired output format"
    },
    "template": {
      "type": "string",
      "description": "Custom template name (optional)"
    },
    "styling": {
      "type": "object",
      "properties": {
        "include_logo": {"type": "boolean", "default": true},
        "color_scheme": {"type": "string", "enum": ["standard", "minimal", "clinical"]},
        "font_family": {"type": "string"}
      }
    }
  },
  "required": ["report_data", "output_format"]
}
```

---

## MCP Resources

Resources provide structured access to data and information.

### Static Resources

#### acmg/rules

**URI**: `acmg/rules`
**Description**: Complete ACMG/AMP classification criteria definitions and guidelines.

**Content Type**: `application/json`

**Structure**:
```json
{
  "criteria": {
    "pathogenic": {
      "very_strong": {
        "PVS1": {
          "description": "Null variant in a gene where LOF is a known mechanism of disease",
          "examples": ["nonsense", "frameshift", "canonical_splice_sites"],
          "caveats": ["gene_specific_considerations", "transcript_considerations"],
          "references": ["PMID:25741868"]
        }
      },
      "strong": {
        "PS1": {
          "description": "Same amino acid change as established pathogenic variant",
          "examples": ["same_codon_different_nucleotide"],
          "caveats": ["functional_domain_considerations"]
        }
      }
    },
    "benign": {
      "standalone": {
        "BA1": {
          "description": "Allele frequency >5% in control population",
          "thresholds": {"general": 0.05, "recessive": 0.01}
        }
      }
    }
  },
  "combination_rules": {
    "pathogenic": [
      "1 Very strong + (≥1 Strong OR ≥2 Moderate OR ≥2 Supporting)",
      "≥2 Strong + (≥1 Moderate OR ≥2 Supporting)",
      "1 Strong + (≥3 Moderate OR ≥4 Supporting)",
      "≥5 Supporting"
    ],
    "likely_pathogenic": [
      "1 Very strong + 1 Moderate",
      "1 Strong + (1-2 Moderate OR ≥2 Supporting)", 
      "≥3 Moderate",
      "2 Moderate + ≥2 Supporting",
      "1 Moderate + ≥4 Supporting"
    ]
  }
}
```

### Dynamic Resources

#### variant/{id}

**URI Template**: `variant/{id}`
**Description**: Detailed information about a specific variant.

**Parameters**:
- `id`: Variant identifier (HGVS, dbSNP, internal ID)

**Content Type**: `application/json`

**Example**: `variant/NM_000492.3:c.1521_1523delCTT`

#### interpretation/{id}

**URI Template**: `interpretation/{id}`
**Description**: Classification result for a specific variant interpretation.

**Parameters**:
- `id`: Interpretation ID or variant identifier

**Content Type**: `application/json`

#### evidence/{variant_id}

**URI Template**: `evidence/{variant_id}`
**Description**: Aggregated evidence data for a variant.

**Parameters**:
- `variant_id`: Variant identifier

**Content Type**: `application/json`

---

## MCP Prompts

Prompts provide structured guidance for AI agent interactions.

### Clinical Workflow Prompts

#### clinical_interpretation

**Description**: Systematic workflow guidance for clinical variant interpretation.

**Arguments**:
```json
{
  "type": "object",
  "properties": {
    "variant": {
      "type": "string",
      "description": "Variant to interpret"
    },
    "patient_phenotype": {
      "type": "array",
      "items": {"type": "string"},
      "description": "Patient clinical features"
    },
    "urgency": {
      "type": "string",
      "enum": ["routine", "urgent", "emergent"],
      "default": "routine"
    },
    "specialty": {
      "type": "string",
      "enum": ["genetics", "oncology", "cardiology", "neurology", "other"],
      "default": "genetics"
    }
  },
  "required": ["variant"]
}
```

**Generated Messages**:
```json
{
  "messages": [
    {
      "role": "system",
      "content": "You are a clinical genetics specialist performing systematic variant interpretation..."
    },
    {
      "role": "user", 
      "content": "Please classify the variant {variant} found in a patient with {patient_phenotype}. Follow ACMG/AMP guidelines systematically..."
    }
  ]
}
```

#### evidence_review

**Description**: Structured guidance for evidence evaluation.

**Arguments**:
```json
{
  "type": "object",
  "properties": {
    "evidence_sources": {
      "type": "array",
      "items": {"type": "string"},
      "description": "Available evidence sources"
    },
    "focus_areas": {
      "type": "array",
      "items": {
        "type": "string",
        "enum": ["population_frequency", "functional_studies", "segregation", "computational_predictions"]
      }
    }
  }
}
```

#### report_generation

**Description**: Guidance for generating clinical reports.

**Arguments**:
```json
{
  "type": "object",
  "properties": {
    "report_type": {
      "type": "string",
      "enum": ["diagnostic", "research", "family_screening", "prenatal"]
    },
    "audience": {
      "type": "string",
      "enum": ["clinician", "patient", "family", "researcher"]
    },
    "complexity_level": {
      "type": "string", 
      "enum": ["basic", "detailed", "comprehensive"]
    }
  }
}
```

#### acmg_training

**Description**: Educational prompts for learning ACMG/AMP guidelines.

**Arguments**:
```json
{
  "type": "object",
  "properties": {
    "topic": {
      "type": "string",
      "enum": ["criteria_overview", "evidence_types", "combination_rules", "special_cases"]
    },
    "level": {
      "type": "string",
      "enum": ["beginner", "intermediate", "advanced"]
    },
    "interactive": {
      "type": "boolean",
      "default": false,
      "description": "Include interactive examples"
    }
  }
}
```

---

## Error Handling

### Error Response Format

All errors follow JSON-RPC 2.0 error response format:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -32600,
    "message": "Invalid Request",
    "data": {
      "details": "Additional error information",
      "suggestions": ["Try correcting the HGVS notation", "Check gene symbol spelling"]
    }
  }
}
```

### Standard Error Codes

| Code | Name | Description |
|------|------|-------------|
| -32700 | Parse error | Invalid JSON |
| -32600 | Invalid Request | Invalid request object |
| -32601 | Method not found | Method does not exist |
| -32602 | Invalid params | Invalid method parameters |
| -32603 | Internal error | Internal server error |
| -32000 | Tool error | Tool execution failed |
| -32001 | Resource error | Resource access failed |
| -32002 | Validation error | Input validation failed |
| -32003 | External API error | External service unavailable |
| -32004 | Classification error | Variant classification failed |
| -32005 | Database error | Database operation failed |

### Tool-Specific Errors

#### classify_variant Errors

| Code | Description | Resolution |
|------|-------------|------------|
| -32002 | Invalid HGVS notation | Use validate_hgvs tool first |
| -32003 | External API unavailable | Retry or use cached data |
| -32004 | Insufficient evidence | Review available data sources |
| -32005 | Classification conflict | Manual review required |

#### validate_hgvs Errors

| Code | Description | Resolution |
|------|-------------|------------|
| -32002 | Malformed HGVS | Check notation format |
| -32004 | Unknown transcript | Verify transcript version |
| -32005 | Invalid genomic coordinates | Check reference sequence |

---

## Rate Limits and Quotas

### Default Limits

| Transport | Requests/Minute | Concurrent Connections |
|-----------|----------------|----------------------|
| stdio | Unlimited | 1 |
| HTTP | 1000 | 10 |
| WebSocket | 1000 | 5 |

### External API Limits

| Database | Requests/Hour | Batch Size |
|----------|---------------|------------|
| ClinVar | 600 | 50 |
| gnomAD | 3000 | 100 |
| COSMIC | 1000 | 20 |

### Rate Limit Headers (HTTP Transport)

```
X-RateLimit-Limit: 1000
X-RateLimit-Remaining: 999
X-RateLimit-Reset: 1640995200
```

---

## Authentication & Security

### API Key Authentication (HTTP/WebSocket)

```http
Authorization: Bearer your-api-key-here
```

### Request Signing (Optional)

For enhanced security, requests can be signed using HMAC-SHA256:

```http
X-Signature: sha256=calculated-signature
X-Timestamp: 1640995200
```

### CORS Configuration

Default CORS policy for HTTP transport:
```
Access-Control-Allow-Origin: https://yourdomain.com
Access-Control-Allow-Methods: POST, GET, OPTIONS
Access-Control-Allow-Headers: Authorization, Content-Type
```

---

## Versioning

### API Versioning

- **Current Version**: 1.0.0
- **Protocol Version**: 2024-11-05
- **Backward Compatibility**: Maintained for 1 major version

### Version Headers

```http
X-API-Version: 1.0.0
X-Protocol-Version: 2024-11-05
```

### Deprecation Notices

Deprecated features are announced 6 months before removal:

```json
{
  "warnings": [
    {
      "type": "deprecation",
      "message": "Feature XYZ will be removed in version 2.0.0",
      "removal_date": "2024-12-01"
    }
  ]
}
```

---

## Performance Considerations

### Response Times

| Operation | Target | Acceptable |
|-----------|--------|------------|
| validate_hgvs | < 100ms | < 500ms |
| classify_variant | < 2s | < 5s |
| query_evidence | < 3s | < 10s |
| generate_report | < 1s | < 3s |

### Caching

- **Tool Results**: 24 hours (configurable)
- **Evidence Data**: 1 hour (ClinVar), 24 hours (gnomAD)
- **Static Resources**: Indefinite (version-based)

### Optimization Tips

1. **Batch Operations**: Use batch-capable tools when available
2. **Caching**: Enable client-side caching for static resources
3. **Compression**: Use gzip compression for HTTP transport
4. **Connection Reuse**: Maintain persistent connections for WebSocket

---

## SDK and Client Libraries

### Official SDKs

- **Python**: `pip install acmg-amp-mcp-client`
- **JavaScript/TypeScript**: `npm install @acmg-amp/mcp-client`
- **Go**: `go get github.com/acmg-amp/mcp-client-go`

### Community Libraries

- **Java**: `acmg-amp-mcp-java`
- **C#**: `AcmgAmp.McpClient`
- **R**: `acmgamp.mcp`

### Example Client Usage

```python
from acmg_amp_mcp import Client

client = Client("./bin/mcp-server")
await client.connect()

result = await client.classify_variant({
    "hgvs": "NM_000492.3:c.1521_1523delCTT",
    "gene": "CFTR"
})

print(f"Classification: {result['classification']}")
```

---

For additional information, examples, and troubleshooting, see the [main documentation](./README.md).