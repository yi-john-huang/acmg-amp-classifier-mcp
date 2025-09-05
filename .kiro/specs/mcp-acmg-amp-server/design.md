# MCP ACMG-AMP Server Design

## Overview

The MCP ACMG-AMP Server is a purpose-built Model Context Protocol implementation that exposes genetic variant classification capabilities through standardized MCP tools, resources, and prompts. The design prioritizes AI agent integration, clinical accuracy, and protocol compliance while building upon the existing ACMG-AMP server foundation.

The system transforms genetic variant analysis into a tool-oriented architecture where AI agents can directly invoke classification operations, access variant data through resources, and receive guided prompts for clinical workflows. This enables seamless integration with Claude, ChatGPT, Gemini, and other MCP-compatible AI systems.

## Architecture

### MCP Server Architecture

```mermaid
graph TB
    subgraph "AI Agents"
        Claude[Claude Desktop]
        ChatGPT[ChatGPT MCP Client]
        Gemini[Gemini AI]
        Custom[Custom MCP Clients]
    end
    
    subgraph "MCP Transport Layer"
        StdioTransport[stdio Transport]
        HTTPTransport[HTTP + SSE Transport]
        AutoDetect[Transport Auto-Detection]
    end
    
    subgraph "MCP Protocol Core"
        JSONRPCServer[JSON-RPC 2.0 Server]
        CapabilityManager[Capability Manager]
        MessageRouter[Message Router]
        ClientManager[Client Manager]
    end
    
    subgraph "MCP Capabilities Registry"
        ToolRegistry[Tool Registry]
        ResourceRegistry[Resource Registry] 
        PromptRegistry[Prompt Registry]
    end
    
    subgraph "ACMG/AMP MCP Tools"
        ClassifyTool[classify_variant]
        ValidateTool[validate_hgvs]
        RulesTool[apply_rule]
        EvidenceTool[query_evidence]
        ReportTool[generate_report]
        FormatTool[format_report]
    end
    
    subgraph "MCP Resources"
        VariantResource[variant/{id}]
        InterpretationResource[interpretation/{id}]
        EvidenceResource[evidence/{variant_id}]
        RulesResource[acmg/rules]
    end
    
    subgraph "MCP Prompts"  
        ClinicalPrompt[clinical_interpretation]
        EvidencePrompt[evidence_review]
        ReportPrompt[report_generation]
        TrainingPrompt[acmg_training]
    end
    
    subgraph "Business Logic (Existing)"
        InputParser[Input Parser Service]
        InterpretationEngine[Interpretation Engine] 
        KnowledgeBase[Knowledge Base Access]
        ReportGenerator[Report Generator]
    end
    
    subgraph "Data Layer (Existing)"
        PostgreSQL[(PostgreSQL)]
        Redis[(Redis Cache)]
        ExternalAPIs[ClinVar/gnomAD/COSMIC]
    end
    
    Claude --> StdioTransport
    ChatGPT --> HTTPTransport
    Gemini --> HTTPTransport
    Custom --> AutoDetect
    
    StdioTransport --> JSONRPCServer
    HTTPTransport --> JSONRPCServer
    AutoDetect --> JSONRPCServer
    
    JSONRPCServer --> CapabilityManager
    JSONRPCServer --> MessageRouter
    JSONRPCServer --> ClientManager
    
    MessageRouter --> ToolRegistry
    MessageRouter --> ResourceRegistry
    MessageRouter --> PromptRegistry
    
    ToolRegistry --> ClassifyTool
    ToolRegistry --> ValidateTool
    ToolRegistry --> RulesTool
    ToolRegistry --> EvidenceTool
    ToolRegistry --> ReportTool
    ToolRegistry --> FormatTool
    
    ResourceRegistry --> VariantResource
    ResourceRegistry --> InterpretationResource
    ResourceRegistry --> EvidenceResource
    ResourceRegistry --> RulesResource
    
    PromptRegistry --> ClinicalPrompt
    PromptRegistry --> EvidencePrompt
    PromptRegistry --> ReportPrompt
    PromptRegistry --> TrainingPrompt
    
    ClassifyTool --> InputParser
    ClassifyTool --> InterpretationEngine
    EvidenceTool --> KnowledgeBase
    ReportTool --> ReportGenerator
    
    InterpretationEngine --> PostgreSQL
    KnowledgeBase --> Redis
    KnowledgeBase --> ExternalAPIs
```

## Technology Stack

- **MCP Protocol**: `github.com/modelcontextprotocol/go-sdk` (Official Go SDK)
- **JSON-RPC**: Built-in JSON-RPC 2.0 via MCP SDK
- **Transport Mechanisms**: stdio, HTTP with Server-Sent Events
- **Runtime**: Go 1.21+
- **Database**: PostgreSQL (existing foundation)
- **Cache**: Redis (existing foundation)
- **External APIs**: ClinVar, gnomAD, COSMIC (existing integrations)
- **Logging**: Structured logging with correlation IDs
- **Testing**: MCP compliance tests + clinical validation

## MCP Tool Definitions

### 1. Classification Tools

#### classify_variant Tool
```go
type ClassifyVariantParams struct {
    HGVSNotation  string      `json:"hgvs_notation" jsonschema:"description=HGVS notation of the variant,required"`
    VariantType   string      `json:"variant_type,omitempty" jsonschema:"description=Variant type (GERMLINE or SOMATIC),enum=GERMLINE,enum=SOMATIC"`
    GeneSymbol    string      `json:"gene_symbol,omitempty" jsonschema:"description=Gene symbol if known"`
    TranscriptID  string      `json:"transcript_id,omitempty" jsonschema:"description=Transcript ID for coding variants"`
    ClinicalContext string   `json:"clinical_context,omitempty" jsonschema:"description=Clinical context for interpretation"`
}

type ClassifyVariantResult struct {
    VariantID       string                `json:"variant_id"`
    Classification  string                `json:"classification"`
    Confidence      string                `json:"confidence"`
    AppliedRules    []ACMGAMPRuleResult  `json:"applied_rules"`
    EvidenceSummary string               `json:"evidence_summary"`
    Recommendations []string             `json:"recommendations"`
    ProcessingTime  string               `json:"processing_time"`
}
```

#### validate_hgvs Tool
```go
type ValidateHGVSParams struct {
    HGVSNotation string `json:"hgvs_notation" jsonschema:"description=HGVS notation to validate,required"`
}

type ValidateHGVSResult struct {
    IsValid         bool   `json:"is_valid"`
    ParsedVariant   *StandardizedVariant `json:"parsed_variant,omitempty"`
    ValidationErrors []string `json:"validation_errors,omitempty"`
    Suggestions     []string `json:"suggestions,omitempty"`
}
```

#### apply_rule Tool
```go
type ApplyRuleParams struct {
    VariantID    string            `json:"variant_id" jsonschema:"description=Variant ID to evaluate,required"`
    RuleCode     string            `json:"rule_code" jsonschema:"description=ACMG/AMP rule code (e.g. PVS1 PS1),required"`
    Evidence     map[string]interface{} `json:"evidence,omitempty" jsonschema:"description=Additional evidence for rule evaluation"`
}

type ApplyRuleResult struct {
    RuleCode    string `json:"rule_code"`
    Met         bool   `json:"met"`
    Strength    string `json:"strength"`
    Rationale   string `json:"rationale"`
    Evidence    string `json:"evidence"`
    Confidence  string `json:"confidence"`
}
```

### 2. Evidence Tools

#### query_evidence Tool
```go
type QueryEvidenceParams struct {
    VariantID   string   `json:"variant_id" jsonschema:"description=Variant ID to query evidence for,required"`
    Databases   []string `json:"databases,omitempty" jsonschema:"description=Specific databases to query,items=enum=clinvar,items=enum=gnomad,items=enum=cosmic"`
    EvidenceTypes []string `json:"evidence_types,omitempty" jsonschema:"description=Types of evidence to gather"`
}

type QueryEvidenceResult struct {
    VariantID        string           `json:"variant_id"`
    ClinVarData      *ClinVarEvidence `json:"clinvar_data,omitempty"`
    GnomADData       *PopulationData  `json:"gnomad_data,omitempty"`
    COSMICData       *SomaticData     `json:"cosmic_data,omitempty"`
    EvidenceSummary  string          `json:"evidence_summary"`
    QueryTime        string          `json:"query_time"`
}
```

### 3. Report Tools

#### generate_report Tool
```go
type GenerateReportParams struct {
    VariantID        string   `json:"variant_id" jsonschema:"description=Variant ID for report generation,required"`
    ReportType       string   `json:"report_type,omitempty" jsonschema:"description=Type of report to generate,enum=clinical,enum=research,enum=summary"`
    IncludeEvidence  bool     `json:"include_evidence,omitempty" jsonschema:"description=Include detailed evidence in report"`
    IncludeRules     bool     `json:"include_rules,omitempty" jsonschema:"description=Include ACMG/AMP rule details"`
    ClinicalContext  string   `json:"clinical_context,omitempty" jsonschema:"description=Clinical context for report customization"`
}

type GenerateReportResult struct {
    ReportID         string                  `json:"report_id"`
    VariantID        string                  `json:"variant_id"`
    Report           *InterpretationReport   `json:"report"`
    GeneratedAt      string                  `json:"generated_at"`
}
```

## MCP Resource Definitions

### 1. Variant Resources

#### variant/{id} Resource Template
```go
type VariantResource struct {
    URI          string                `json:"uri"`
    Name         string                `json:"name"`
    Description  string                `json:"description"`  
    MIMEType     string                `json:"mime_type"`
    
    // Resource content
    Variant      *StandardizedVariant  `json:"variant"`
    Status       string                `json:"status"`
    LastUpdated  string                `json:"last_updated"`
}
```

#### interpretation/{id} Resource Template
```go
type InterpretationResource struct {
    URI             string                     `json:"uri"`
    Name            string                     `json:"name"`
    Description     string                     `json:"description"`
    MIMEType        string                     `json:"mime_type"`
    
    // Resource content  
    Interpretation  *ClassificationResult      `json:"interpretation"`
    VariantID       string                     `json:"variant_id"`
    CreatedAt       string                     `json:"created_at"`
    Version         string                     `json:"version"`
}
```

### 2. Reference Resources

#### acmg/rules Resource
```go
type ACMGRulesResource struct {
    URI         string              `json:"uri"`
    Name        string              `json:"name"`
    Description string              `json:"description"`
    MIMEType    string              `json:"mime_type"`
    
    // Resource content
    Rules       []ACMGAMPRuleDefinition `json:"rules"`
    Version     string              `json:"version"`
    LastUpdated string              `json:"last_updated"`
}
```

## MCP Prompt Definitions

### 1. Clinical Workflow Prompts

#### clinical_interpretation Prompt
```go
type ClinicalInterpretationPrompt struct {
    Name         string                    `json:"name"`
    Description  string                    `json:"description"`
    Arguments    []PromptArgument         `json:"arguments"`
    
    // Prompt template generates workflow guidance
    PromptTemplate string                  `json:"prompt_template"`
}
```

Sample prompt template:
```
You are performing clinical genetic variant interpretation using ACMG/AMP guidelines for variant {{variant_id}}.

Clinical Context: {{clinical_context}}

Systematic Workflow:
1. First, use the validate_hgvs tool to confirm the variant notation is correct
2. Use query_evidence tool to gather comprehensive evidence from databases
3. Apply ACMG/AMP criteria systematically:
   - Pathogenic criteria (PVS1, PS1-4, PM1-6, PP1-5)
   - Benign criteria (BA1, BS1-4, BP1-7)
4. Use apply_rule tool for each relevant criterion
5. Use classify_variant tool for final classification
6. Generate comprehensive report with clinical recommendations

Remember to:
- Document rationale for each criterion
- Consider population frequency data carefully
- Account for clinical context in interpretation
- Provide clear recommendations for clinical action
```

## Implementation Strategy

### Phase 1: MCP Foundation (Week 1)
1. **MCP SDK Integration**
   - Add official Go MCP SDK dependency
   - Create basic MCP server structure
   - Implement transport layer (stdio + HTTP)
   - Set up capability negotiation

2. **Core Protocol Implementation**
   - JSON-RPC 2.0 message handling
   - Client connection management
   - Basic tool/resource/prompt registration
   - Error handling framework

### Phase 2: Tool Implementation (Weeks 2-3)
1. **Classification Tools**
   - Implement classify_variant tool
   - Create validate_hgvs tool
   - Build apply_rule tool for individual criteria
   - Add combine_evidence tool

2. **Evidence Tools**
   - Implement query_evidence tool
   - Integrate with existing external API layer
   - Add evidence caching and optimization
   - Create structured evidence responses

### Phase 3: Resources & Prompts (Week 4)
1. **Resource Implementation**
   - Create resource templates for variants and interpretations
   - Implement dynamic resource URI resolution
   - Add ACMG rules reference resources
   - Optimize resource caching

2. **Prompt Templates**
   - Build clinical workflow prompts
   - Create educational prompts
   - Add evidence review guidance
   - Implement prompt customization

### Phase 4: Integration & Testing (Week 5)
1. **MCP Compliance Testing**
   - Test with real AI agent clients
   - Validate JSON-RPC 2.0 compliance
   - Verify tool/resource/prompt discovery
   - Performance testing and optimization

2. **Clinical Validation**
   - Test with known variant datasets
   - Validate ACMG/AMP rule application
   - Verify report accuracy
   - Document AI agent interaction patterns

## Security and Compliance

### MCP Protocol Security
- **Client Authentication**: MCP client identification and rate limiting
- **Tool Execution Safety**: Clear tool descriptions and parameter validation
- **Audit Logging**: Complete audit trail of all AI agent interactions
- **Error Handling**: Secure error responses without information disclosure

### Clinical Data Protection
- **No PHI Storage**: Strictly no patient-identifiable information
- **Secure Transport**: TLS 1.3 for HTTP transports
- **Access Control**: Resource-level access restrictions
- **Compliance Logging**: Healthcare-grade audit trails

This MCP-native design enables AI agents to seamlessly access ACMG/AMP genetic variant classification capabilities while maintaining clinical accuracy, security, and protocol compliance.