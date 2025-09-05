# Design Document

## Overview

The ACMG-AMP MCP Server is a high-performance Golang service that implements standardized genetic variant classification using ACMG/AMP guidelines through the Model Context Protocol (MCP). The system follows a tool-oriented architecture where AI agents can directly invoke genetic analysis capabilities via JSON-RPC 2.0, enabling seamless integration with Claude, ChatGPT, Gemini, and other MCP-compatible AI systems.

The service exposes genetic variant analysis through MCP tools, resources, and prompts, processing variants through a multi-stage pipeline: input validation and parsing, evidence aggregation from multiple databases, ACMG/AMP rule application, and structured report generation. The design emphasizes protocol compliance, clinical accuracy, and AI agent interoperability while maintaining the flexibility to support both stdio and HTTP transport mechanisms.

## Architecture

### System Architecture

The system follows an MCP-native architecture with JSON-RPC 2.0 message handling and tool-based functionality:

```mermaid
graph TB
    subgraph "MCP Client Layer"
        Claude[Claude AI Agent]
        ChatGPT[ChatGPT AI Agent]
        Gemini[Gemini AI Agent]
        Custom[Custom MCP Clients]
    end
    
    subgraph "MCP Transport Layer"
        Stdio[stdio Transport]
        HTTP_SSE[HTTP with SSE]
        Stream_HTTP[Streamable HTTP]
    end
    
    subgraph "MCP Server Core"
        JSONRPCHandler[JSON-RPC 2.0 Handler]
        CapabilityNegotiation[Capability Negotiation]
        MessageRouter[Message Router]
    end
    
    subgraph "MCP Capabilities"
        ToolRegistry[Tool Registry]
        ResourceProvider[Resource Provider]
        PromptManager[Prompt Manager]
    end
    
    subgraph "ACMG/AMP Tools"
        ClassifyTool[classify_variant]
        ValidateTool[validate_hgvs]
        EvidenceTool[query_evidence]
        ReportTool[generate_report]
    end
    
    subgraph "MCP Resources"
        VariantRes[variant/{id}]
        InterpRes[interpretation/{id}]
        EvidenceRes[evidence/{variant_id}]
        RuleRes[acmg/rules]
    end
    
    subgraph "Business Logic Layer"
        Parser[Input Parser]
        Engine[Interpretation Engine]
        KBAccess[Knowledge Base Access]
        Cache[Redis Cache]
    end
    
    subgraph "External Systems"
        ClinVar[(ClinVar)]
        gnomAD[(gnomAD)]
        COSMIC[(COSMIC)]
    end
    
    subgraph "Storage Layer"
        PostgreSQL[(PostgreSQL)]
        Logs[(Audit Logs)]
    end
    
    Claude --> Stdio
    ChatGPT --> HTTP_SSE
    Gemini --> Stream_HTTP
    Custom --> HTTP_SSE
    
    Stdio --> JSONRPCHandler
    HTTP_SSE --> JSONRPCHandler
    Stream_HTTP --> JSONRPCHandler
    
    JSONRPCHandler --> CapabilityNegotiation
    JSONRPCHandler --> MessageRouter
    
    MessageRouter --> ToolRegistry
    MessageRouter --> ResourceProvider
    MessageRouter --> PromptManager
    
    ToolRegistry --> ClassifyTool
    ToolRegistry --> ValidateTool
    ToolRegistry --> EvidenceTool
    ToolRegistry --> ReportTool
    
    ResourceProvider --> VariantRes
    ResourceProvider --> InterpRes
    ResourceProvider --> EvidenceRes
    ResourceProvider --> RuleRes
    
    ClassifyTool --> Parser
    ClassifyTool --> Engine
    EvidenceTool --> KBAccess
    
    Engine --> KBAccess
    KBAccess --> Cache
    KBAccess --> ClinVar
    KBAccess --> gnomAD
    KBAccess --> COSMIC
    
    Engine --> PostgreSQL
    JSONRPCHandler --> Logs
```

### Technology Stack

- **Runtime**: Go 1.21+
- **MCP Protocol**: `github.com/modelcontextprotocol/go-sdk` (Official MCP Go SDK)
- **JSON-RPC**: Built-in JSON-RPC 2.0 implementation via MCP SDK
- **Transport**: stdio, HTTP with Server-Sent Events, Streamable HTTP
- **Database**: PostgreSQL (for persistent storage)
- **Cache**: Redis (for external API response caching)
- **Configuration**: Viper (for configuration management)
- **Logging**: Logrus with structured logging
- **Testing**: Go standard testing + Testify + MCP integration tests
- **Documentation**: MCP tool/resource/prompt definitions (JSON schema-based)
- **Containerization**: Docker with multi-stage builds

## Components and Interfaces

### 1. MCP Server Core

**Purpose**: Handles MCP protocol communication, message routing, and capability negotiation using JSON-RPC 2.0.

**Key Interfaces**:
```go
// Core MCP Server using official Go SDK
type MCPServer struct {
    server     *mcp.Server
    tools      map[string]mcp.Tool
    resources  map[string]mcp.Resource
    prompts    map[string]mcp.Prompt
    transport  mcp.Transport
}

// MCP Tool Parameters for variant classification
type ClassifyVariantParams struct {
    HGVSNotation string      `json:"hgvs_notation" jsonschema:"description=HGVS notation of the variant,required"`
    VariantType  VariantType `json:"variant_type,omitempty" jsonschema:"description=Variant type (GERMLINE or SOMATIC)"`
    GeneSymbol   string      `json:"gene_symbol,omitempty" jsonschema:"description=Gene symbol if known"`
    TranscriptID string      `json:"transcript_id,omitempty" jsonschema:"description=Transcript ID for coding variants"`
}

// MCP Tool Result for variant classification
type ClassifyVariantResult struct {
    VariantID      string                 `json:"variant_id"`
    Classification Classification          `json:"classification"`
    Confidence     ConfidenceLevel        `json:"confidence"`
    AppliedRules   []ACMGAMPRule         `json:"applied_rules"`
    Evidence       []EvidenceItem        `json:"evidence"`
    Report         InterpretationReport  `json:"report"`
    ProcessingTime string                `json:"processing_time"`
}

// MCP Resource URI patterns
const (
    VariantResourceURI       = "variant/{id}"
    InterpretationResourceURI = "interpretation/{id}" 
    EvidenceResourceURI      = "evidence/{variant_id}"
    ACMGRulesResourceURI     = "acmg/rules"
)
```

### 2. Input Parser Component

**Purpose**: Validates and standardizes variant nomenclature, ensuring consistent input format.

**Key Interfaces**:
```go
type InputParser interface {
    ParseVariant(input string) (*StandardizedVariant, error)
    ValidateHGVS(hgvs string) error
    NormalizeVariant(variant *StandardizedVariant) error
}

type StandardizedVariant struct {
    Chromosome    string      `json:"chromosome"`
    Position      int64       `json:"position"`
    Reference     string      `json:"reference"`
    Alternative   string      `json:"alternative"`
    HGVSGenomic   string      `json:"hgvs_genomic"`
    HGVSCoding    string      `json:"hgvs_coding,omitempty"`
    HGVSProtein   string      `json:"hgvs_protein,omitempty"`
    GeneSymbol    string      `json:"gene_symbol"`
    TranscriptID  string      `json:"transcript_id,omitempty"`
    VariantType   VariantType `json:"variant_type"`
}
```

### 3. MCP Tool Definitions

**Purpose**: Expose ACMG/AMP functionality as callable MCP tools that AI agents can invoke directly.

**Key Tool Implementations**:
```go
// Tool: classify_variant - Full ACMG/AMP variant classification
func RegisterClassifyVariantTool(server *mcp.Server, engine *InterpretationEngine) {
    tool := &mcp.Tool{
        Name:        "classify_variant",
        Description: "Classify a genetic variant using ACMG/AMP guidelines",
        InputSchema: ClassifyVariantParams{},
    }
    
    handler := func(ctx context.Context, req *mcp.CallToolRequest, params ClassifyVariantParams) (*mcp.CallToolResult, any, error) {
        result, err := engine.ClassifyVariant(ctx, params)
        if err != nil {
            return nil, nil, err
        }
        
        return &mcp.CallToolResult{
            Content: []mcp.Content{
                &mcp.TextContent{Text: fmt.Sprintf("Classification: %s", result.Classification)},
                &mcp.TextContent{Text: fmt.Sprintf("Confidence: %s", result.Confidence)},
            },
        }, result, nil
    }
    
    mcp.AddTool(server, tool, handler)
}

// Tool: validate_hgvs - HGVS notation validation
func RegisterValidateHGVSTool(server *mcp.Server, parser *InputParser) {
    tool := &mcp.Tool{
        Name:        "validate_hgvs",
        Description: "Validate HGVS notation and parse variant components",
        InputSchema: struct {
            HGVSNotation string `json:"hgvs_notation" jsonschema:"description=HGVS notation to validate,required"`
        }{},
    }
    
    handler := func(ctx context.Context, req *mcp.CallToolRequest, params struct{ HGVSNotation string }) (*mcp.CallToolResult, any, error) {
        result, err := parser.ParseVariant(params.HGVSNotation)
        if err != nil {
            return &mcp.CallToolResult{
                Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Invalid HGVS: %s", err.Error())}},
            }, nil, nil
        }
        
        return &mcp.CallToolResult{
            Content: []mcp.Content{&mcp.TextContent{Text: "Valid HGVS notation"}},
        }, result, nil
    }
    
    mcp.AddTool(server, tool, handler)
}

// Tool: query_evidence - Query external databases for evidence
func RegisterQueryEvidenceTool(server *mcp.Server, kbAccess *KnowledgeBaseAccess) {
    tool := &mcp.Tool{
        Name:        "query_evidence",
        Description: "Query external databases (ClinVar, gnomAD, COSMIC) for variant evidence",
        InputSchema: struct {
            VariantID string `json:"variant_id" jsonschema:"description=Variant ID to query,required"`
            Databases []string `json:"databases,omitempty" jsonschema:"description=Specific databases to query (clinvar, gnomad, cosmic)"`
        }{},
    }
    
    mcp.AddTool(server, tool, func(ctx context.Context, req *mcp.CallToolRequest, params struct{
        VariantID string
        Databases []string
    }) (*mcp.CallToolResult, any, error) {
        evidence, err := kbAccess.GatherEvidence(ctx, params.VariantID, params.Databases)
        return &mcp.CallToolResult{
            Content: []mcp.Content{&mcp.TextContent{Text: "Evidence gathered successfully"}},
        }, evidence, err
    })
}
```

### 4. Knowledge Base Access Component

**Purpose**: Manages connections to external databases and aggregates evidence for variant interpretation.

**Key Interfaces**:
```go
type KnowledgeBaseAccess interface {
    GatherEvidence(ctx context.Context, variant *StandardizedVariant) (*AggregatedEvidence, error)
    QueryClinVar(variant *StandardizedVariant) (*ClinVarData, error)
    QueryGnomAD(variant *StandardizedVariant) (*PopulationData, error)
    QueryCOSMIC(variant *StandardizedVariant) (*SomaticData, error)
}

type AggregatedEvidence struct {
    ClinicalSignificance *ClinVarData     `json:"clinical_significance,omitempty"`
    PopulationFrequency  *PopulationData  `json:"population_frequency,omitempty"`
    SomaticEvidence      *SomaticData     `json:"somatic_evidence,omitempty"`
    ComputationalData    *PredictionData  `json:"computational_data,omitempty"`
    FunctionalData       *FunctionalData  `json:"functional_data,omitempty"`
    SegregationData      *SegregationData `json:"segregation_data,omitempty"`
}
```

### 5. MCP Resource Definitions

**Purpose**: Expose variant data and results as queryable MCP resources that AI agents can access without tool execution overhead.

**Key Resource Implementations**:
```go
// Resource: variant/{id} - Individual variant details
func RegisterVariantResource(server *mcp.Server, repository *VariantRepository) {
    template := mcp.NewResourceTemplate(
        "variant/{id}",
        "Genetic Variant Information",
        mcp.WithTemplateDescription("Detailed information about a genetic variant"),
        mcp.WithTemplateMIMEType("application/json"),
    )
    
    handler := func(ctx context.Context, req *mcp.ReadResourceRequest, uri string) (*mcp.ReadResourceResult, error) {
        variantID := extractIDFromURI(uri)
        variant, err := repository.GetVariant(ctx, variantID)
        if err != nil {
            return nil, err
        }
        
        return &mcp.ReadResourceResult{
            Contents: []mcp.ResourceContent{
                {
                    URI:      uri,
                    MIMEType: "application/json",
                    Text:     toJSON(variant),
                },
            },
        }, nil
    }
    
    server.AddResourceTemplate(template, handler)
}

// Resource: interpretation/{id} - Classification results  
func RegisterInterpretationResource(server *mcp.Server, repository *InterpretationRepository) {
    template := mcp.NewResourceTemplate(
        "interpretation/{id}",
        "Variant Classification Result",
        mcp.WithTemplateDescription("ACMG/AMP classification result with evidence"),
        mcp.WithTemplateMIMEType("application/json"),
    )
    
    handler := func(ctx context.Context, req *mcp.ReadResourceRequest, uri string) (*mcp.ReadResourceResult, error) {
        interpretationID := extractIDFromURI(uri)
        result, err := repository.GetInterpretation(ctx, interpretationID)
        if err != nil {
            return nil, err
        }
        
        return &mcp.ReadResourceResult{
            Contents: []mcp.ResourceContent{
                {
                    URI:      uri,
                    MIMEType: "application/json", 
                    Text:     toJSON(result),
                },
            },
        }, nil
    }
    
    server.AddResourceTemplate(template, handler)
}

// Resource: acmg/rules - ACMG/AMP rule definitions
func RegisterACMGRulesResource(server *mcp.Server) {
    resource := mcp.NewResource(
        "acmg/rules",
        "ACMG/AMP Guidelines",
        mcp.WithDescription("Complete ACMG/AMP evidence criteria definitions"),
        mcp.WithMIMEType("application/json"),
    )
    
    handler := func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
        rules := GetACMGAMPRuleDefinitions() // Load all 28 rules
        
        return &mcp.ReadResourceResult{
            Contents: []mcp.ResourceContent{
                {
                    URI:      "acmg/rules",
                    MIMEType: "application/json",
                    Text:     toJSON(rules),
                },
            },
        }, nil
    }
    
    server.AddResource(resource, handler)
}
```

### 6. MCP Prompt Templates

**Purpose**: Provide structured prompts to guide AI agents through clinical variant interpretation workflows.

**Key Prompt Implementations**:
```go
// Prompt: clinical_interpretation - Guide through variant interpretation
func RegisterClinicalInterpretationPrompt(server *mcp.Server) {
    prompt := mcp.NewPrompt(
        "clinical_interpretation",
        mcp.WithPromptDescription("Guide AI agent through ACMG/AMP variant interpretation workflow"),
        mcp.WithArgument("variant_id", 
            mcp.ArgumentDescription("ID of variant to interpret"),
            mcp.RequiredArgument(),
        ),
        mcp.WithArgument("clinical_context",
            mcp.ArgumentDescription("Clinical context (germline/somatic, patient info)"),
        ),
    )
    
    handler := func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
        variantID := req.Arguments["variant_id"]
        clinicalContext := req.Arguments["clinical_context"]
        
        promptText := fmt.Sprintf(`
You are interpreting variant %s for clinical significance using ACMG/AMP guidelines.

Clinical Context: %s

Follow this systematic approach:
1. Use validate_hgvs tool to confirm variant notation
2. Use query_evidence tool to gather data from ClinVar, gnomAD, COSMIC
3. Apply ACMG/AMP criteria based on evidence:
   - PVS1: null variant in gene where LOF causes disease
   - PS1: same amino acid change as established pathogenic variant
   - PM1: located in mutational hotspot or functional domain
   - [Continue with all 28 criteria...]
4. Use classify_variant tool for final classification
5. Generate clinical report with recommendations

Always explain your reasoning for each criterion applied.
        `, variantID, clinicalContext)
        
        return &mcp.GetPromptResult{
            Description: "ACMG/AMP Clinical Interpretation Workflow",
            Messages: []mcp.PromptMessage{
                {
                    Role:    mcp.RoleUser,
                    Content: mcp.TextContent{Text: promptText},
                },
            },
        }, nil
    }
    
    server.AddPrompt(prompt, handler)
}
```

## Data Models

### Core Enums and Types

```go
type Classification string
const (
    PATHOGENIC        Classification = "PATHOGENIC"
    LIKELY_PATHOGENIC Classification = "LIKELY_PATHOGENIC"
    VUS              Classification = "VUS"
    LIKELY_BENIGN    Classification = "LIKELY_BENIGN"
    BENIGN           Classification = "BENIGN"
)

type VariantType string
const (
    GERMLINE VariantType = "GERMLINE"
    SOMATIC  VariantType = "SOMATIC"
)

type RuleStrength string
const (
    VERY_STRONG RuleStrength = "VERY_STRONG"
    STRONG      RuleStrength = "STRONG"
    MODERATE    RuleStrength = "MODERATE"
    SUPPORTING  RuleStrength = "SUPPORTING"
)
```

### Database Schema

**variants table**:
```sql
CREATE TABLE variants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    hgvs_notation VARCHAR(255) NOT NULL,
    chromosome VARCHAR(10) NOT NULL,
    position BIGINT NOT NULL,
    reference VARCHAR(1000) NOT NULL,
    alternative VARCHAR(1000) NOT NULL,
    gene_symbol VARCHAR(50),
    variant_type VARCHAR(20) NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
```

**interpretations table**:
```sql
CREATE TABLE interpretations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    variant_id UUID REFERENCES variants(id),
    classification VARCHAR(20) NOT NULL,
    confidence_level VARCHAR(20) NOT NULL,
    applied_rules JSONB NOT NULL,
    evidence_summary JSONB NOT NULL,
    report_data JSONB NOT NULL,
    processing_time_ms INTEGER NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);
```

## Error Handling

### MCP Error Types and Handling Strategy

```go
// MCP-compliant error handling using JSON-RPC 2.0 error format
type MCPError struct {
    Code    int         `json:"code"`    // JSON-RPC error codes
    Message string      `json:"message"`
    Data    interface{} `json:"data,omitempty"`
}

// MCP Error Categories (JSON-RPC 2.0 compliant)
const (
    ErrParseError     = -32700 // Invalid JSON was received
    ErrInvalidRequest = -32600 // The JSON sent is not a valid Request object
    ErrMethodNotFound = -32601 // The method does not exist
    ErrInvalidParams  = -32602 // Invalid method parameter(s)
    ErrInternalError  = -32603 // Internal JSON-RPC error
    
    // Application-specific errors (range -32000 to -32099)
    ErrInvalidHGVS       = -32000 // Invalid HGVS notation
    ErrDatabaseError     = -32001 // Database operation failed
    ErrExternalAPIError  = -32002 // External API unavailable
    ErrClassificationError = -32003 // Classification logic error
    ErrRateLimitExceeded = -32004 // Too many requests
    ErrAuthenticationError = -32005 // Authentication failed
)

// MCP Tool Error Response
func NewMCPToolError(code int, message string, details interface{}) *mcp.CallToolResult {
    return &mcp.CallToolResult{
        Content: []mcp.Content{
            &mcp.TextContent{Text: fmt.Sprintf("Error: %s", message)},
        },
        IsError: true,
    }
}
```

### Error Handling Patterns

1. **JSON-RPC Validation Errors**: Return standard JSON-RPC error responses with appropriate error codes
2. **Tool Execution Errors**: Return MCP tool error results with structured error information
3. **External API Failures**: Implement circuit breaker pattern with fallback to cached data, report as tool warnings
4. **Classification Uncertainties**: Return successful tool results with VUS classification and uncertainty explanations
5. **Resource Access Errors**: Return MCP resource not found errors with helpful alternative suggestions

## Testing Strategy

### Unit Testing
- **Coverage Target**: 90%+ code coverage for MCP tools and business logic
- **Test Structure**: Table-driven tests for ACMG/AMP rule application
- **MCP Mocking**: Mock MCP client interactions and JSON-RPC messages
- **Test Data**: Curated set of known variants with expected classifications

### MCP Integration Testing
- **Tool Testing**: Test all MCP tools with various parameter combinations
- **Resource Testing**: Validate MCP resource URI templates and content delivery
- **Prompt Testing**: Verify prompt generation with different clinical contexts
- **Protocol Testing**: Test JSON-RPC 2.0 compliance and capability negotiation

### AI Agent Integration Testing
- **Claude Integration**: Test with Claude Desktop MCP client
- **ChatGPT Integration**: Test with OpenAI MCP implementation
- **Custom Client Testing**: Test with mock MCP clients simulating different behaviors
- **Error Handling Testing**: Verify proper JSON-RPC error responses

### Performance Testing
- **Concurrent Tool Calls**: Support for 100 concurrent MCP tool invocations with <2s response time
- **Resource Access Load**: Test rapid resource access patterns
- **Transport Performance**: Compare stdio vs HTTP transport performance
- **Database Performance**: Query optimization for large variant datasets

### Clinical Validation Testing
- **Known Variant Testing**: Validate against ClinVar expert-reviewed variants using MCP tools
- **Benchmark Datasets**: Test against published ACMG/AMP validation sets
- **Edge Case Testing**: Handle ambiguous and complex variant scenarios through AI agent interactions

## Security Considerations

### MCP Protocol Security
- **Client Authentication**: Support for MCP client identification and capability negotiation
- **Tool Execution Consent**: Implement user consent mechanisms for tool invocation as required by MCP specification
- **Rate Limiting**: JSON-RPC request rate limiting per MCP client to prevent abuse
- **Audit Logging**: Comprehensive logging of all MCP tool calls, resource access, and client interactions

### Data Protection
- **No Patient Data Storage**: Strictly no storage of patient-identifiable information
- **Transport Security**: TLS 1.3 for HTTP-based MCP transports, secure stdio handling for local connections
- **Configuration Security**: Secure management of database credentials and API keys
- **Resource Access Control**: Restricted access to sensitive resources based on client permissions

### Input Validation and Sanitization
- **JSON-RPC Validation**: Strict validation of JSON-RPC 2.0 message format and parameters
- **HGVS Validation**: Comprehensive HGVS notation validation to prevent injection attacks
- **Parameter Sanitization**: Sanitization of all tool parameters before processing
- **Resource URI Validation**: Validation of resource URI patterns to prevent directory traversal

### MCP-Specific Security Measures
- **Tool Safety**: Implementation of MCP tool safety guidelines with clear descriptions of tool capabilities and risks
- **Prompt Injection Protection**: Safeguards against prompt injection attacks through structured prompt templates
- **Resource Enumeration Protection**: Limits on resource listing to prevent unauthorized discovery of sensitive data

## Performance and Scalability

### MCP-Optimized Caching Strategy
- **External API Caching**: Redis caching for ClinVar, gnomAD, COSMIC responses (TTL: 24 hours)
- **Resource Caching**: In-memory caching of frequently accessed MCP resources
- **Tool Result Caching**: Caching of identical tool invocation results
- **Rule Definition Caching**: In-memory caching for ACMG/AMP rule definitions

### Database Optimization
- **Indexed Queries**: Optimized indexes on chromosome, position, and gene symbol
- **Connection Pooling**: Configurable PostgreSQL connection pools per MCP transport
- **Read Replicas**: Separate read replicas for resource queries vs tool execution
- **JSONB Optimization**: GIN indexes for efficient evidence data queries

### MCP Transport Optimization
- **stdio Performance**: Optimized JSON message parsing and serialization
- **HTTP Transport**: Connection keep-alive and request batching for HTTP-based clients
- **Concurrent Handling**: Support for multiple concurrent MCP clients and tool invocations
- **Resource Streaming**: Efficient streaming for large resource content

### Horizontal Scaling
- **Stateless Design**: MCP server instances are fully stateless for horizontal scaling
- **Load Balancing**: Support for load balancing across multiple MCP server instances
- **Transport Distribution**: Distribute stdio and HTTP transports across different instances
- **Container Ready**: Full Kubernetes compatibility with health checks and graceful shutdown