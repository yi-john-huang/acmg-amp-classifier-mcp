package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/acmg-amp-mcp-server/internal/mcp/protocol"
	"github.com/acmg-amp-mcp-server/internal/service"
)

// ClassifyVariantTool implements the classify_variant MCP tool
type ClassifyVariantTool struct {
	logger            *logrus.Logger
	classifierService *service.ClassifierService
}

// ClassifyVariantParams defines parameters for the classify_variant tool
type ClassifyVariantParams struct {
	HGVSNotation    string `json:"hgvs_notation" validate:"required"`
	VariantType     string `json:"variant_type,omitempty"`
	GeneSymbol      string `json:"gene_symbol,omitempty"`
	TranscriptID    string `json:"transcript_id,omitempty"`
	ClinicalContext string `json:"clinical_context,omitempty"`
	IncludeEvidence bool   `json:"include_evidence,omitempty"`
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

// ACMGAMPRuleResult represents a single ACMG/AMP rule evaluation result
type ACMGAMPRuleResult struct {
	RuleCode    string  `json:"rule_code"`
	RuleName    string  `json:"rule_name"`
	Category    string  `json:"category"` // "pathogenic", "benign", "other"
	Strength    string  `json:"strength"` // "very_strong", "strong", "moderate", "supporting"
	Applied     bool    `json:"applied"`
	Confidence  float64 `json:"confidence"`
	Evidence    string  `json:"evidence,omitempty"`
	Reasoning   string  `json:"reasoning,omitempty"`
}

// NewClassifyVariantTool creates a new classify_variant tool
func NewClassifyVariantTool(logger *logrus.Logger, classifierService *service.ClassifierService) *ClassifyVariantTool {
	return &ClassifyVariantTool{
		logger:            logger,
		classifierService: classifierService,
	}
}

// HandleTool implements the ToolHandler interface for classify_variant
func (t *ClassifyVariantTool) HandleTool(ctx context.Context, req *protocol.JSONRPC2Request) *protocol.JSONRPC2Response {
	startTime := time.Now()
	t.logger.WithField("tool", "classify_variant").Info("Processing variant classification request")

	// Parse and validate parameters
	var params ClassifyVariantParams
	if err := t.parseAndValidateParams(req.Params, &params); err != nil {
		return &protocol.JSONRPC2Response{
			Error: &protocol.RPCError{
				Code:    protocol.InvalidParams,
				Message: "Invalid parameters",
				Data:    err.Error(),
			},
		}
	}

	// Perform variant classification
	result, err := t.classifyVariant(ctx, &params)
	if err != nil {
		return &protocol.JSONRPC2Response{
			Error: &protocol.RPCError{
				Code:    protocol.MCPToolError,
				Message: "Classification failed",
				Data:    err.Error(),
			},
		}
	}

	result.ProcessingTime = time.Since(startTime).String()

	t.logger.WithFields(logrus.Fields{
		"variant_id":      result.VariantID,
		"classification":  result.Classification,
		"processing_time": result.ProcessingTime,
	}).Info("Variant classification completed")

	return &protocol.JSONRPC2Response{
		Result: map[string]interface{}{
			"classification": result,
		},
	}
}

// GetToolInfo returns tool metadata
func (t *ClassifyVariantTool) GetToolInfo() protocol.ToolInfo {
	return protocol.ToolInfo{
		Name:        "classify_variant",
		Description: "Classify a genetic variant using ACMG/AMP guidelines with comprehensive evidence evaluation",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"hgvs_notation": map[string]interface{}{
					"type":        "string",
					"description": "HGVS notation of the variant (e.g., 'NM_000492.3:c.1521_1523delCTT')",
					"pattern":     "^(NC_|NM_|NP_|NG_|NR_|XM_|XR_).*",
				},
				"variant_type": map[string]interface{}{
					"type":        "string",
					"description": "Type of variant (SNV, indel, CNV, etc.)",
					"enum":        []string{"SNV", "indel", "CNV", "SV", "fusion"},
				},
				"gene_symbol": map[string]interface{}{
					"type":        "string",
					"description": "HGNC gene symbol",
					"pattern":     "^[A-Z][A-Z0-9-]*$",
				},
				"transcript_id": map[string]interface{}{
					"type":        "string",
					"description": "RefSeq transcript identifier",
				},
				"clinical_context": map[string]interface{}{
					"type":        "string",
					"description": "Clinical context or phenotype information",
				},
				"include_evidence": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether to include detailed evidence in the response",
					"default":     false,
				},
			},
			"required": []string{"hgvs_notation"},
		},
	}
}

// ValidateParams validates tool parameters
func (t *ClassifyVariantTool) ValidateParams(params interface{}) error {
	var classifyParams ClassifyVariantParams
	return t.parseAndValidateParams(params, &classifyParams)
}

// parseAndValidateParams parses and validates input parameters
func (t *ClassifyVariantTool) parseAndValidateParams(params interface{}, target *ClassifyVariantParams) error {
	if params == nil {
		return fmt.Errorf("missing required parameters")
	}

	// Convert params to JSON and back to properly parse
	paramsBytes, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("failed to marshal parameters: %w", err)
	}

	if err := json.Unmarshal(paramsBytes, target); err != nil {
		return fmt.Errorf("failed to parse parameters: %w", err)
	}

	// Validate required fields
	if target.HGVSNotation == "" {
		return fmt.Errorf("hgvs_notation is required")
	}

	// Basic HGVS format validation
	if !t.isValidHGVSFormat(target.HGVSNotation) {
		return fmt.Errorf("invalid HGVS notation format: %s", target.HGVSNotation)
	}

	return nil
}

// isValidHGVSFormat performs basic HGVS format validation
func (t *ClassifyVariantTool) isValidHGVSFormat(hgvs string) bool {
	// Basic validation - in a real implementation, this would be more comprehensive
	if len(hgvs) < 10 {
		return false
	}
	
	// Check for common HGVS prefixes
	prefixes := []string{"NC_", "NM_", "NP_", "NG_", "NR_", "XM_", "XR_"}
	for _, prefix := range prefixes {
		if len(hgvs) >= len(prefix) && hgvs[:len(prefix)] == prefix {
			return true
		}
	}
	
	return false
}

// classifyVariant performs the actual variant classification
func (t *ClassifyVariantTool) classifyVariant(ctx context.Context, params *ClassifyVariantParams) (*ClassifyVariantResult, error) {
	t.logger.WithField("hgvs", params.HGVSNotation).Debug("Starting variant classification")

	// Convert MCP tool params to service params
	serviceParams := &service.ClassifyVariantParams{
		HGVSNotation:    params.HGVSNotation,
		VariantType:     params.VariantType,
		GeneSymbol:      params.GeneSymbol,
		TranscriptID:    params.TranscriptID,
		ClinicalContext: params.ClinicalContext,
		IncludeEvidence: params.IncludeEvidence,
	}

	// Call the real classification service
	serviceResult, err := t.classifierService.ClassifyVariant(ctx, serviceParams)
	if err != nil {
		return nil, fmt.Errorf("classification service failed: %w", err)
	}

	// Convert service result to MCP tool result
	result := &ClassifyVariantResult{
		VariantID:       serviceResult.VariantID,
		Classification:  serviceResult.Classification,
		Confidence:      serviceResult.Confidence,
		AppliedRules:    t.convertRuleResults(serviceResult.AppliedRules),
		EvidenceSummary: serviceResult.EvidenceSummary,
		Recommendations: serviceResult.Recommendations,
		ProcessingTime:  serviceResult.ProcessingTime.String(),
	}

	return result, nil
}

// convertRuleResults converts service rule results to MCP tool format
func (t *ClassifyVariantTool) convertRuleResults(serviceRules []service.ACMGAMPRuleResult) []ACMGAMPRuleResult {
	results := make([]ACMGAMPRuleResult, len(serviceRules))
	for i, rule := range serviceRules {
		results[i] = ACMGAMPRuleResult{
			RuleCode:   rule.RuleCode,
			RuleName:   rule.RuleName,
			Category:   rule.Category,
			Strength:   rule.Strength,
			Applied:    rule.Applied,
			Confidence: rule.Confidence,
			Evidence:   rule.Evidence,
			Reasoning:  rule.Reasoning,
		}
	}
	return results
}

