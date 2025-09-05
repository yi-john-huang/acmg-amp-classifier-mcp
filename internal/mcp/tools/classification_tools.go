package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/acmg-amp-mcp-server/internal/mcp/protocol"
)

// ClassifyVariantTool implements the classify_variant MCP tool
type ClassifyVariantTool struct {
	logger *logrus.Logger
	// TODO: Add classifier service dependency when available
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
func NewClassifyVariantTool(logger *logrus.Logger) *ClassifyVariantTool {
	return &ClassifyVariantTool{
		logger: logger,
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

	// TODO: Integrate with actual classification service
	// For now, return a mock classification result
	
	// Generate variant ID
	variantID := t.generateVariantID(params.HGVSNotation)

	// Mock classification logic
	appliedRules := t.mockACMGAMPRules(params)
	classification := t.determineClassification(appliedRules)
	confidence := t.calculateConfidence(appliedRules)

	result := &ClassifyVariantResult{
		VariantID:      variantID,
		Classification: classification,
		Confidence:     confidence,
		AppliedRules:   appliedRules,
		EvidenceSummary: t.generateEvidenceSummary(appliedRules),
		Recommendations: t.generateRecommendations(classification, confidence),
	}

	return result, nil
}

// generateVariantID creates a unique variant identifier
func (t *ClassifyVariantTool) generateVariantID(hgvs string) string {
	// Simple hash-based ID generation - in production, use proper variant normalization
	return fmt.Sprintf("VAR_%d", time.Now().Unix())
}

// mockACMGAMPRules generates mock ACMG/AMP rule evaluations
func (t *ClassifyVariantTool) mockACMGAMPRules(params *ClassifyVariantParams) []ACMGAMPRuleResult {
	// Mock some common ACMG/AMP rules
	rules := []ACMGAMPRuleResult{
		{
			RuleCode:    "PVS1",
			RuleName:    "Null variant in a gene where LOF is a known mechanism",
			Category:    "pathogenic",
			Strength:    "very_strong",
			Applied:     false,
			Confidence:  0.3,
			Reasoning:   "Variant does not clearly result in loss of function",
		},
		{
			RuleCode:    "PS1",
			RuleName:    "Same amino acid change as established pathogenic variant",
			Category:    "pathogenic", 
			Strength:    "strong",
			Applied:     false,
			Confidence:  0.1,
			Reasoning:   "No known pathogenic variants at this position",
		},
		{
			RuleCode:    "PM2",
			RuleName:    "Absent from population databases",
			Category:    "pathogenic",
			Strength:    "moderate", 
			Applied:     true,
			Confidence:  0.8,
			Evidence:    "Absent from gnomAD v3.1.2",
			Reasoning:   "Not observed in large population cohorts",
		},
		{
			RuleCode:    "PP3",
			RuleName:    "In silico evidence supports deleterious effect",
			Category:    "pathogenic",
			Strength:    "supporting",
			Applied:     true,
			Confidence:  0.7,
			Evidence:    "CADD: 25.3, SIFT: deleterious, PolyPhen: probably damaging",
			Reasoning:   "Multiple algorithms predict damaging effect",
		},
		{
			RuleCode:    "BA1",
			RuleName:    "Allele frequency >5% in population database",
			Category:    "benign",
			Strength:    "standalone",
			Applied:     false,
			Confidence:  0.0,
			Reasoning:   "Variant frequency below threshold",
		},
	}

	return rules
}

// determineClassification determines final classification based on applied rules
func (t *ClassifyVariantTool) determineClassification(rules []ACMGAMPRuleResult) string {
	pathogenicScore := 0.0
	benignScore := 0.0

	for _, rule := range rules {
		if !rule.Applied {
			continue
		}

		weight := t.getRuleWeight(rule.Strength)
		if rule.Category == "pathogenic" {
			pathogenicScore += weight * rule.Confidence
		} else if rule.Category == "benign" {
			benignScore += weight * rule.Confidence
		}
	}

	// Simplified classification logic
	if pathogenicScore >= 5.0 {
		return "Pathogenic"
	} else if pathogenicScore >= 3.0 {
		return "Likely pathogenic"
	} else if benignScore >= 3.0 {
		return "Likely benign"
	} else if benignScore >= 5.0 {
		return "Benign"
	}

	return "Variant of uncertain significance (VUS)"
}

// getRuleWeight returns numeric weight for rule strength
func (t *ClassifyVariantTool) getRuleWeight(strength string) float64 {
	switch strength {
	case "very_strong":
		return 4.0
	case "strong":
		return 3.0
	case "moderate":
		return 2.0
	case "supporting":
		return 1.0
	case "standalone":
		return 5.0
	default:
		return 0.0
	}
}

// calculateConfidence calculates overall confidence in classification
func (t *ClassifyVariantTool) calculateConfidence(rules []ACMGAMPRuleResult) string {
	totalConfidence := 0.0
	appliedRules := 0

	for _, rule := range rules {
		if rule.Applied {
			totalConfidence += rule.Confidence
			appliedRules++
		}
	}

	if appliedRules == 0 {
		return "Low"
	}

	avgConfidence := totalConfidence / float64(appliedRules)
	if avgConfidence >= 0.8 {
		return "High"
	} else if avgConfidence >= 0.6 {
		return "Moderate"
	}
	return "Low"
}

// generateEvidenceSummary creates a human-readable evidence summary
func (t *ClassifyVariantTool) generateEvidenceSummary(rules []ACMGAMPRuleResult) string {
	appliedRules := make([]string, 0)
	for _, rule := range rules {
		if rule.Applied {
			appliedRules = append(appliedRules, rule.RuleCode)
		}
	}

	if len(appliedRules) == 0 {
		return "No ACMG/AMP criteria met"
	}

	return fmt.Sprintf("Applied ACMG/AMP criteria: %v", appliedRules)
}

// generateRecommendations creates actionable recommendations
func (t *ClassifyVariantTool) generateRecommendations(classification, confidence string) []string {
	recommendations := make([]string, 0)

	switch classification {
	case "Pathogenic", "Likely pathogenic":
		recommendations = append(recommendations, 
			"Consider genetic counseling",
			"Evaluate family history and consider cascade screening",
			"Review clinical management guidelines for this gene",
		)
	case "Variant of uncertain significance (VUS)":
		recommendations = append(recommendations,
			"Gather additional family history and segregation data",
			"Consider functional studies if available",
			"Re-evaluate as new evidence becomes available",
		)
	case "Likely benign", "Benign":
		recommendations = append(recommendations,
			"Variant unlikely to contribute to disease phenotype",
			"Continue standard clinical care",
		)
	}

	if confidence == "Low" {
		recommendations = append(recommendations,
			"Low confidence classification - seek expert consultation",
			"Consider additional evidence gathering",
		)
	}

	return recommendations
}