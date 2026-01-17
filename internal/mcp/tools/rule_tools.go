package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/acmg-amp-mcp-server/internal/mcp/protocol"
	"github.com/acmg-amp-mcp-server/internal/service"
)

// ApplyRuleTool implements the apply_rule MCP tool for individual ACMG/AMP criterion evaluation
type ApplyRuleTool struct {
	logger            *logrus.Logger
	classifierService *service.ClassifierService
}

// ApplyRuleParams defines parameters for the apply_rule tool
type ApplyRuleParams struct {
	RuleCode     string                 `json:"rule_code" validate:"required"`
	VariantData  VariantData            `json:"variant_data" validate:"required"`
	EvidenceData map[string]interface{} `json:"evidence_data,omitempty"`
}

// VariantData contains variant information for rule evaluation
type VariantData struct {
	HGVSNotation    string `json:"hgvs_notation"`
	GeneSymbol      string `json:"gene_symbol,omitempty"`
	TranscriptID    string `json:"transcript_id,omitempty"`
	VariantType     string `json:"variant_type,omitempty"`
	Position        int    `json:"position,omitempty"`
	ReferenceAllele string `json:"reference_allele,omitempty"`
	AlternateAllele string `json:"alternate_allele,omitempty"`
}

// ApplyRuleResult defines the result structure for apply_rule tool
type ApplyRuleResult struct {
	RuleCode        string                 `json:"rule_code"`
	RuleName        string                 `json:"rule_name"`
	Category        string                 `json:"category"`
	Strength        string                 `json:"strength"`
	Applied         bool                   `json:"applied"`
	Confidence      float64                `json:"confidence"`
	Evidence        map[string]interface{} `json:"evidence,omitempty"`
	Reasoning       string                 `json:"reasoning"`
	Requirements    []string               `json:"requirements,omitempty"`
	Recommendations []string               `json:"recommendations,omitempty"`
}

// CombineEvidenceTool implements the combine_evidence MCP tool
type CombineEvidenceTool struct {
	logger            *logrus.Logger
	classifierService *service.ClassifierService
}

// CombineEvidenceParams defines parameters for the combine_evidence tool
type CombineEvidenceParams struct {
	AppliedRules []ACMGAMPRuleResult `json:"applied_rules" validate:"required"`
	Guidelines   string              `json:"guidelines,omitempty"` // "ACMG2015", "ClinGen2020", etc.
}

// CombineEvidenceResult defines the result structure for combine_evidence tool  
type CombineEvidenceResult struct {
	Classification    string                      `json:"classification"`
	Confidence        string                      `json:"confidence"`
	CombinationLogic  CombinationLogicExplanation `json:"combination_logic"`
	AlternativeRules  []string                    `json:"alternative_rules,omitempty"`
	Recommendations   []string                    `json:"recommendations"`
}

// CombinationLogicExplanation explains how rules were combined
type CombinationLogicExplanation struct {
	PathogenicRules    RuleCombination `json:"pathogenic_rules"`
	BenignRules        RuleCombination `json:"benign_rules"`
	DecisionTree       []DecisionStep  `json:"decision_tree"`
	GuidelinesUsed     string          `json:"guidelines_used"`
}

// RuleCombination represents grouped rules by strength
type RuleCombination struct {
	VeryStrong  []string `json:"very_strong,omitempty"`
	Strong      []string `json:"strong,omitempty"`
	Moderate    []string `json:"moderate,omitempty"`
	Supporting  []string `json:"supporting,omitempty"`
	Standalone  []string `json:"standalone,omitempty"`
}

// DecisionStep represents a step in the classification decision process
type DecisionStep struct {
	Step        int    `json:"step"`
	Condition   string `json:"condition"`
	Met         bool   `json:"met"`
	Result      string `json:"result,omitempty"`
	Explanation string `json:"explanation"`
}

// ACMG/AMP rule definitions
var ACMGAMPRules = map[string]RuleDefinition{
	"PVS1": {
		Code:        "PVS1",
		Name:        "Null variant in a gene where LOF is a known mechanism",
		Category:    "pathogenic",
		Strength:    "very_strong",
		Description: "Predicted loss-of-function variant in a gene where LOF is a known mechanism of disease",
	},
	"PS1": {
		Code:        "PS1", 
		Name:        "Same amino acid change as established pathogenic variant",
		Category:    "pathogenic",
		Strength:    "strong",
		Description: "Same amino acid change as a previously established pathogenic variant regardless of nucleotide change",
	},
	"PS2": {
		Code:        "PS2",
		Name:        "De novo variant with confirmed paternity/maternity",
		Category:    "pathogenic",
		Strength:    "strong",
		Description: "De novo (both maternity and paternity confirmed) in a patient with the disease and no family history",
	},
	"PS3": {
		Code:        "PS3",
		Name:        "Functional studies show deleterious effect",
		Category:    "pathogenic",
		Strength:    "strong",
		Description: "Well-established in vitro or in vivo functional studies supportive of a damaging effect on the gene or gene product",
	},
	"PS4": {
		Code:        "PS4",
		Name:        "Prevalence increased in affected vs controls",
		Category:    "pathogenic",
		Strength:    "strong",
		Description: "The prevalence of the variant in affected individuals is significantly increased compared to the prevalence in controls",
	},
	"PM1": {
		Code:        "PM1",
		Name:        "Missense in critical functional domain",
		Category:    "pathogenic",
		Strength:    "moderate",
		Description: "Located in a mutational hot spot and/or critical and well-established functional domain without benign variation",
	},
	"PM2": {
		Code:        "PM2",
		Name:        "Absent from population databases",
		Category:    "pathogenic",
		Strength:    "moderate", 
		Description: "Absent from controls (or at extremely low frequency if recessive) in population databases",
	},
	"PP3": {
		Code:        "PP3",
		Name:        "In silico evidence supports deleterious effect",
		Category:    "pathogenic",
		Strength:    "supporting",
		Description: "Multiple lines of computational evidence support a deleterious effect on the gene or gene product",
	},
	"BA1": {
		Code:        "BA1",
		Name:        "Allele frequency >5% in population database",
		Category:    "benign",
		Strength:    "standalone",
		Description: "Allele frequency is >5% in population databases",
	},
	"BS1": {
		Code:        "BS1",
		Name:        "Allele frequency higher than expected for disorder",
		Category:    "benign",
		Strength:    "strong",
		Description: "Allele frequency is greater than expected for disorder",
	},
	"BP4": {
		Code:        "BP4",
		Name:        "In silico evidence suggests no impact",
		Category:    "benign",
		Strength:    "supporting",
		Description: "Multiple lines of computational evidence suggest no impact on gene or gene product",
	},
}

// RuleDefinition contains metadata about an ACMG/AMP rule
type RuleDefinition struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Strength    string `json:"strength"`
	Description string `json:"description"`
}

// NewApplyRuleTool creates a new apply_rule tool
func NewApplyRuleTool(logger *logrus.Logger, classifierService *service.ClassifierService) *ApplyRuleTool {
	return &ApplyRuleTool{
		logger:            logger,
		classifierService: classifierService,
	}
}

// HandleTool implements the ToolHandler interface for apply_rule
func (t *ApplyRuleTool) HandleTool(ctx context.Context, req *protocol.JSONRPC2Request) *protocol.JSONRPC2Response {
	t.logger.WithField("tool", "apply_rule").Info("Processing ACMG/AMP rule application request")

	// Parse and validate parameters
	var params ApplyRuleParams
	if err := t.parseAndValidateParams(req.Params, &params); err != nil {
		return &protocol.JSONRPC2Response{
			Error: &protocol.RPCError{
				Code:    protocol.InvalidParams,
				Message: "Invalid parameters",
				Data:    err.Error(),
			},
		}
	}

	// Apply the rule
	result, err := t.applyRule(ctx, &params)
	if err != nil {
		return &protocol.JSONRPC2Response{
			Error: &protocol.RPCError{
				Code:    protocol.MCPToolError,
				Message: "Rule application failed",
				Data:    err.Error(),
			},
		}
	}

	t.logger.WithFields(logrus.Fields{
		"rule_code": params.RuleCode,
		"applied":   result.Applied,
		"confidence": result.Confidence,
	}).Info("ACMG/AMP rule application completed")

	return &protocol.JSONRPC2Response{
		Result: map[string]interface{}{
			"rule_evaluation": result,
		},
	}
}

// GetToolInfo returns tool metadata for apply_rule
func (t *ApplyRuleTool) GetToolInfo() protocol.ToolInfo {
	return protocol.ToolInfo{
		Name:        "apply_rule",
		Description: "Evaluate a specific ACMG/AMP criterion for a genetic variant with detailed reasoning",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"rule_code": map[string]interface{}{
					"type":        "string",
					"description": "ACMG/AMP rule code (e.g., PVS1, PS1, PM2, PP3, BA1, BS1, BP4)",
					"enum": []string{"PVS1", "PS1", "PS2", "PS3", "PS4", "PM1", "PM2", "PM3", "PM4", "PM5", "PM6",
						"PP1", "PP2", "PP3", "PP4", "PP5", "BA1", "BS1", "BS2", "BS3", "BS4", "BP1", "BP2", "BP3", "BP4", "BP5", "BP6", "BP7"},
				},
				"variant_data": map[string]interface{}{
					"type":        "object",
					"description": "Variant information for rule evaluation",
					"properties": map[string]interface{}{
						"hgvs_notation": map[string]interface{}{
							"type": "string",
						},
						"gene_symbol": map[string]interface{}{
							"type": "string",
						},
						"variant_type": map[string]interface{}{
							"type": "string",
							"enum": []string{"SNV", "indel", "CNV", "SV"},
						},
					},
					"required": []string{"hgvs_notation"},
				},
				"evidence_data": map[string]interface{}{
					"type":        "object",
					"description": "Additional evidence data for rule evaluation",
				},
			},
			"required": []string{"rule_code", "variant_data"},
		},
	}
}

// ValidateParams validates tool parameters for apply_rule
func (t *ApplyRuleTool) ValidateParams(params interface{}) error {
	var applyParams ApplyRuleParams
	return t.parseAndValidateParams(params, &applyParams)
}

// parseAndValidateParams parses and validates input parameters for apply_rule
func (t *ApplyRuleTool) parseAndValidateParams(params interface{}, target *ApplyRuleParams) error {
	if params == nil {
		return fmt.Errorf("missing required parameters")
	}

	paramsBytes, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("failed to marshal parameters: %w", err)
	}

	if err := json.Unmarshal(paramsBytes, target); err != nil {
		return fmt.Errorf("failed to parse parameters: %w", err)
	}

	if target.RuleCode == "" {
		return fmt.Errorf("rule_code is required")
	}

	if _, exists := ACMGAMPRules[target.RuleCode]; !exists {
		return fmt.Errorf("unknown rule code: %s", target.RuleCode)
	}

	if target.VariantData.HGVSNotation == "" {
		return fmt.Errorf("variant_data.hgvs_notation is required")
	}

	return nil
}

// applyRule applies a specific ACMG/AMP rule
func (t *ApplyRuleTool) applyRule(ctx context.Context, params *ApplyRuleParams) (*ApplyRuleResult, error) {
	// Validate that classifier service is available
	if t.classifierService == nil {
		return nil, fmt.Errorf("classification service not configured")
	}

	// Convert MCP tool params to service params
	serviceParams := &service.ApplyRuleParams{
		RuleCode:     params.RuleCode,
		HGVSNotation: params.VariantData.HGVSNotation,
		Evidence:     nil, // TODO: Convert evidence data if needed
	}

	// Call the real rule evaluation service
	serviceResult, err := t.classifierService.ApplyRule(ctx, serviceParams)
	if err != nil {
		return nil, fmt.Errorf("rule evaluation service failed: %w", err)
	}

	// Convert service result to MCP tool result
	result := &ApplyRuleResult{
		RuleCode:        serviceResult.RuleCode,
		RuleName:        serviceResult.RuleName,
		Category:        serviceResult.Category,
		Strength:        serviceResult.Strength,
		Applied:         serviceResult.Applied,
		Confidence:      serviceResult.Confidence,
		Evidence:        map[string]interface{}{"details": serviceResult.Evidence},
		Reasoning:       serviceResult.Reasoning,
		Requirements:    []string{}, // Could be enhanced with specific requirements
		Recommendations: []string{}, // Could be enhanced with specific recommendations
	}

	return result, nil
}


// NewCombineEvidenceTool creates a new combine_evidence tool
func NewCombineEvidenceTool(logger *logrus.Logger, classifierService *service.ClassifierService) *CombineEvidenceTool {
	return &CombineEvidenceTool{
		logger:            logger,
		classifierService: classifierService,
	}
}

// HandleTool implements the ToolHandler interface for combine_evidence
func (t *CombineEvidenceTool) HandleTool(ctx context.Context, req *protocol.JSONRPC2Request) *protocol.JSONRPC2Response {
	t.logger.WithField("tool", "combine_evidence").Info("Processing evidence combination request")

	// Parse and validate parameters
	var params CombineEvidenceParams
	if err := t.parseAndValidateParams(req.Params, &params); err != nil {
		return &protocol.JSONRPC2Response{
			Error: &protocol.RPCError{
				Code:    protocol.InvalidParams,
				Message: "Invalid parameters",
				Data:    err.Error(),
			},
		}
	}

	// Combine evidence
	result := t.combineEvidence(&params)

	t.logger.WithFields(logrus.Fields{
		"classification":  result.Classification,
		"applied_rules":   len(params.AppliedRules),
		"guidelines":      result.CombinationLogic.GuidelinesUsed,
	}).Info("Evidence combination completed")

	return &protocol.JSONRPC2Response{
		Result: map[string]interface{}{
			"combination_result": result,
		},
	}
}

// GetToolInfo returns tool metadata for combine_evidence
func (t *CombineEvidenceTool) GetToolInfo() protocol.ToolInfo {
	return protocol.ToolInfo{
		Name:        "combine_evidence",
		Description: "Combine multiple ACMG/AMP rule evaluations to determine final variant classification",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"applied_rules": map[string]interface{}{
					"type":        "array",
					"description": "Array of applied ACMG/AMP rule results",
					"items": map[string]interface{}{
						"type": "object",
					},
				},
				"guidelines": map[string]interface{}{
					"type":        "string", 
					"description": "Guidelines version to use for combination logic",
					"enum":        []string{"ACMG2015", "ClinGen2020"},
					"default":     "ACMG2015",
				},
			},
			"required": []string{"applied_rules"},
		},
	}
}

// ValidateParams validates tool parameters for combine_evidence
func (t *CombineEvidenceTool) ValidateParams(params interface{}) error {
	var combineParams CombineEvidenceParams
	return t.parseAndValidateParams(params, &combineParams)
}

// parseAndValidateParams parses and validates input parameters for combine_evidence
func (t *CombineEvidenceTool) parseAndValidateParams(params interface{}, target *CombineEvidenceParams) error {
	if params == nil {
		return fmt.Errorf("missing required parameters")
	}

	paramsBytes, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("failed to marshal parameters: %w", err)
	}

	if err := json.Unmarshal(paramsBytes, target); err != nil {
		return fmt.Errorf("failed to parse parameters: %w", err)
	}

	if len(target.AppliedRules) == 0 {
		return fmt.Errorf("applied_rules is required and must not be empty")
	}

	// Set default guidelines
	if target.Guidelines == "" {
		target.Guidelines = "ACMG2015"
	}

	return nil
}

// combineEvidence combines ACMG/AMP rules according to guidelines
func (t *CombineEvidenceTool) combineEvidence(params *CombineEvidenceParams) *CombineEvidenceResult {
	// Check if classifier service is available
	if t.classifierService == nil {
		return &CombineEvidenceResult{
			Classification: "VUS",
			Confidence:     "Low",
			CombinationLogic: CombinationLogicExplanation{
				GuidelinesUsed: params.Guidelines,
				DecisionTree: []DecisionStep{{
					Step:        1,
					Condition:   "Service availability check",
					Met:         false,
					Result:      "VUS",
					Explanation: "Classification service not configured - unable to combine evidence",
				}},
			},
		}
	}

	// Convert MCP rule results to service format
	serviceRules := make([]service.RuleResult, len(params.AppliedRules))
	for i, rule := range params.AppliedRules {
		serviceRules[i] = service.RuleResult{
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

	// Call the real evidence combination service
	serviceResult, err := t.classifierService.CombineEvidence(serviceRules)
	if err != nil {
		// Fallback to basic result in case of error
		return &CombineEvidenceResult{
			Classification: "VUS",
			Confidence:     "Low",
			CombinationLogic: CombinationLogicExplanation{
				GuidelinesUsed: params.Guidelines,
				DecisionTree:   []DecisionStep{},
			},
			Recommendations: []string{"Evidence combination failed - manual review required"},
		}
	}

	// Convert service result to MCP tool result
	result := &CombineEvidenceResult{
		Classification: serviceResult.Classification,
		Confidence:     serviceResult.Confidence,
		CombinationLogic: CombinationLogicExplanation{
			GuidelinesUsed: params.Guidelines,
			DecisionTree: []DecisionStep{{
				Step:        1,
				Condition:   serviceResult.CombinationRule,
				Met:         true,
				Result:      serviceResult.Classification,
				Explanation: serviceResult.Summary,
			}},
		},
		Recommendations: t.generateSimpleRecommendations(serviceResult.Classification),
	}

	return result
}

// generateSimpleRecommendations creates basic recommendations based on classification
func (t *CombineEvidenceTool) generateSimpleRecommendations(classification string) []string {
	switch classification {
	case "PATHOGENIC":
		return []string{
			"Strong evidence supports pathogenic classification",
			"Consider clinical action and genetic counseling",
		}
	case "LIKELY_PATHOGENIC":
		return []string{
			"Moderate evidence supports pathogenic classification",
			"Consider additional studies if clinically warranted",
		}
	case "VUS":
		return []string{
			"Insufficient evidence for definitive classification",
			"Gather additional evidence and re-evaluate periodically",
		}
	case "LIKELY_BENIGN":
		return []string{
			"Evidence supports benign interpretation",
			"Variant unlikely to be disease-causing",
		}
	case "BENIGN":
		return []string{
			"Strong evidence supports benign classification",
			"Variant not expected to contribute to disease",
		}
	default:
		return []string{"Classification uncertain - expert review recommended"}
	}
}

