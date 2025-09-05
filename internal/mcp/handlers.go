package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ClassifyVariantParams defines parameters for classify_variant tool
type ClassifyVariantParams struct {
	HGVSNotation    string `json:"hgvs_notation"`
	VariantType     string `json:"variant_type,omitempty"`
	GeneSymbol      string `json:"gene_symbol,omitempty"`
	TranscriptID    string `json:"transcript_id,omitempty"`
	ClinicalContext string `json:"clinical_context,omitempty"`
}

// ClassifyVariantResult defines the result structure for classify_variant tool
type ClassifyVariantResult struct {
	VariantID       string                 `json:"variant_id"`
	Classification  string                 `json:"classification"`
	Confidence      string                 `json:"confidence"`
	AppliedRules    []ACMGAMPRuleResult    `json:"applied_rules"`
	EvidenceSummary string                 `json:"evidence_summary"`
	Recommendations []string               `json:"recommendations"`
	ProcessingTime  string                 `json:"processing_time"`
}

// ACMGAMPRuleResult represents a single ACMG/AMP rule evaluation
type ACMGAMPRuleResult struct {
	Code      string `json:"code"`
	Met       bool   `json:"met"`
	Strength  string `json:"strength"`
	Rationale string `json:"rationale"`
	Evidence  string `json:"evidence"`
}

// ValidateHGVSParams defines parameters for validate_hgvs tool
type ValidateHGVSParams struct {
	HGVSNotation string `json:"hgvs_notation"`
}

// ValidateHGVSResult defines the result structure for validate_hgvs tool
type ValidateHGVSResult struct {
	IsValid          bool     `json:"is_valid"`
	ParsedVariant    interface{} `json:"parsed_variant,omitempty"`
	ValidationErrors []string `json:"validation_errors,omitempty"`
	Suggestions      []string `json:"suggestions,omitempty"`
}

// QueryEvidenceParams defines parameters for query_evidence tool
type QueryEvidenceParams struct {
	VariantID string   `json:"variant_id"`
	Databases []string `json:"databases,omitempty"`
}

// QueryEvidenceResult defines the result structure for query_evidence tool
type QueryEvidenceResult struct {
	VariantID       string      `json:"variant_id"`
	ClinVarData     interface{} `json:"clinvar_data,omitempty"`
	GnomADData      interface{} `json:"gnomad_data,omitempty"`
	COSMICData      interface{} `json:"cosmic_data,omitempty"`
	EvidenceSummary string      `json:"evidence_summary"`
	QueryTime       string      `json:"query_time"`
}

// GenerateReportParams defines parameters for generate_report tool
type GenerateReportParams struct {
	VariantID       string `json:"variant_id"`
	ReportType      string `json:"report_type,omitempty"`
	IncludeEvidence bool   `json:"include_evidence,omitempty"`
}

// GenerateReportResult defines the result structure for generate_report tool
type GenerateReportResult struct {
	ReportID    string      `json:"report_id"`
	VariantID   string      `json:"variant_id"`
	Report      interface{} `json:"report"`
	GeneratedAt string      `json:"generated_at"`
}

// handleClassifyVariant handles the classify_variant tool invocation
func (s *Server) handleClassifyVariant(ctx context.Context, req *mcp.CallToolRequest, params ClassifyVariantParams) (*mcp.CallToolResult, any, error) {
	s.logger.WithField("tool", "classify_variant").Info("Tool invoked")

	// Validate required parameters
	if params.HGVSNotation == "" {
		return s.createErrorResult("Missing required parameter", fmt.Errorf("hgvs_notation is required")), nil, nil
	}

	// TODO: Implement actual classification logic using existing services
	// For now, return a mock result
	result := ClassifyVariantResult{
		VariantID:      "mock-variant-id-001",
		Classification: "VUS", // Variant of Uncertain Significance
		Confidence:     "MODERATE",
		AppliedRules: []ACMGAMPRuleResult{
			{
				Code:      "PM1",
				Met:       true,
				Strength:  "MODERATE",
				Rationale: "Located in mutational hotspot",
				Evidence:  "Domain analysis shows critical functional region",
			},
		},
		EvidenceSummary: fmt.Sprintf("Classification for variant %s completed using available evidence", params.HGVSNotation),
		Recommendations: []string{
			"Consider additional functional studies",
			"Review family history and segregation data",
		},
		ProcessingTime: "1.2s",
	}

	// Create tool result
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf("Classification completed for %s: %s (%s confidence)", 
					params.HGVSNotation, result.Classification, result.Confidence),
			},
		},
	}, result, nil
}

// handleValidateHGVS handles the validate_hgvs tool invocation
func (s *Server) handleValidateHGVS(ctx context.Context, req *mcp.CallToolRequest, params ValidateHGVSParams) (*mcp.CallToolResult, any, error) {
	s.logger.WithField("tool", "validate_hgvs").Info("Tool invoked")

	// Validate required parameters
	if params.HGVSNotation == "" {
		return s.createErrorResult("Missing required parameter", fmt.Errorf("hgvs_notation is required")), nil, nil
	}

	// TODO: Implement actual HGVS validation using existing input parser
	// For now, return a mock result
	result := ValidateHGVSResult{
		IsValid: true,
		ParsedVariant: map[string]interface{}{
			"chromosome":    "17",
			"position":      43044295,
			"reference":     "C",
			"alternative":   "T",
			"gene_symbol":   "BRCA1",
			"transcript_id": "NM_007294.3",
		},
		ValidationErrors: []string{},
		Suggestions:      []string{},
	}

	// Handle invalid cases
	if len(params.HGVSNotation) < 5 {
		result.IsValid = false
		result.ValidationErrors = []string{"HGVS notation too short"}
		result.Suggestions = []string{"Provide complete HGVS notation (e.g., NM_007294.3:c.1234C>T)"}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf("HGVS validation result: %t for %s", result.IsValid, params.HGVSNotation),
			},
		},
		Meta: map[string]interface{}{
			"result": result,
		},
	}, nil
}

// handleQueryEvidence handles the query_evidence tool invocation
func (s *Server) handleQueryEvidence(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	s.logger.WithField("tool", "query_evidence").Info("Tool invoked")

	// Parse parameters
	var params QueryEvidenceParams
	if err := json.Unmarshal(req.Params.Arguments, &params); err != nil {
		return s.createErrorResult("Invalid parameters", err), nil
	}

	// Validate required parameters
	if params.VariantID == "" {
		return s.createErrorResult("Missing required parameter", fmt.Errorf("variant_id is required")), nil
	}

	// TODO: Implement actual evidence querying using existing external API services
	// For now, return a mock result
	result := QueryEvidenceResult{
		VariantID: params.VariantID,
		ClinVarData: map[string]interface{}{
			"clinical_significance": "Uncertain significance",
			"review_status":        "criteria provided, single submitter",
			"last_evaluated":       "2024-01-15",
		},
		GnomADData: map[string]interface{}{
			"allele_frequency": 0.0001,
			"population":      "NFE",
			"quality_metrics": "PASS",
		},
		COSMICData: map[string]interface{}{
			"mutation_id":    "COSM12345",
			"cancer_types":   []string{"Breast carcinoma", "Ovarian carcinoma"},
			"samples_tested": 1250,
		},
		EvidenceSummary: fmt.Sprintf("Evidence gathered from %d databases for variant %s", len(params.Databases), params.VariantID),
		QueryTime:      "0.8s",
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf("Evidence query completed for variant %s", params.VariantID),
			},
		},
		Meta: map[string]interface{}{
			"result": result,
		},
	}, nil
}

// handleGenerateReport handles the generate_report tool invocation
func (s *Server) handleGenerateReport(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	s.logger.WithField("tool", "generate_report").Info("Tool invoked")

	// Parse parameters
	var params GenerateReportParams
	if err := json.Unmarshal(req.Params.Arguments, &params); err != nil {
		return s.createErrorResult("Invalid parameters", err), nil
	}

	// Validate required parameters
	if params.VariantID == "" {
		return s.createErrorResult("Missing required parameter", fmt.Errorf("variant_id is required")), nil
	}

	// Set default report type
	if params.ReportType == "" {
		params.ReportType = "clinical"
	}

	// TODO: Implement actual report generation using existing report service
	// For now, return a mock result
	result := GenerateReportResult{
		ReportID:  fmt.Sprintf("report-%s-001", params.VariantID),
		VariantID: params.VariantID,
		Report: map[string]interface{}{
			"summary":        "Variant classification and clinical interpretation report",
			"classification": "VUS",
			"evidence_summary": []string{
				"Population frequency within normal range",
				"No strong pathogenic evidence identified",
				"Requires additional functional studies",
			},
			"recommendations": []string{
				"Consider genetic counseling",
				"Monitor for additional family history",
			},
			"report_type": params.ReportType,
		},
		GeneratedAt: "2025-01-05T12:00:00Z",
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf("Clinical report generated for variant %s (Report ID: %s)", 
					params.VariantID, result.ReportID),
			},
		},
		Meta: map[string]interface{}{
			"result": result,
		},
	}, nil
}

// createErrorResult creates a standardized error result for tool calls
func (s *Server) createErrorResult(message string, err error) *mcp.CallToolResult {
	errorText := fmt.Sprintf("Error: %s", message)
	if err != nil {
		errorText += fmt.Sprintf(" - %v", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: errorText},
		},
		IsError: true,
	}
}