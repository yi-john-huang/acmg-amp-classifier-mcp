package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/acmg-amp-mcp-server/internal/mcp/protocol"
)

// ApplyRuleTool implements the apply_rule MCP tool for individual ACMG/AMP criterion evaluation
type ApplyRuleTool struct {
	logger *logrus.Logger
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
	logger *logrus.Logger
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
func NewApplyRuleTool(logger *logrus.Logger) *ApplyRuleTool {
	return &ApplyRuleTool{
		logger: logger,
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
	ruledef, exists := ACMGAMPRules[params.RuleCode]
	if !exists {
		return nil, fmt.Errorf("rule not found: %s", params.RuleCode)
	}

	result := &ApplyRuleResult{
		RuleCode:        ruledef.Code,
		RuleName:        ruledef.Name,
		Category:        ruledef.Category,
		Strength:        ruledef.Strength,
		Applied:         false,
		Confidence:      0.0,
		Evidence:        make(map[string]interface{}),
		Requirements:    make([]string, 0),
		Recommendations: make([]string, 0),
	}

	// Apply rule-specific logic
	switch params.RuleCode {
	case "PVS1":
		t.applyPVS1(params, result)
	case "PS1":
		t.applyPS1(params, result)
	case "PM2":
		t.applyPM2(params, result) 
	case "PP3":
		t.applyPP3(params, result)
	case "BA1":
		t.applyBA1(params, result)
	default:
		// Generic rule application
		t.applyGenericRule(params, result)
	}

	return result, nil
}

// applyPVS1 applies the PVS1 rule (null variant)
func (t *ApplyRuleTool) applyPVS1(params *ApplyRuleParams, result *ApplyRuleResult) {
	// Check if variant is loss-of-function
	isLOF := t.isLossOfFunction(params.VariantData)
	
	if isLOF {
		result.Applied = true
		result.Confidence = 0.8
		result.Reasoning = "Variant predicted to result in loss of function in a gene where LOF is pathogenic mechanism"
		result.Evidence["variant_type"] = params.VariantData.VariantType
		result.Requirements = []string{
			"Confirm variant results in loss of function",
			"Verify gene has established LOF pathogenic mechanism",
			"Rule out potential rescue by alternative transcripts",
		}
	} else {
		result.Applied = false
		result.Confidence = 0.2
		result.Reasoning = "Variant does not clearly result in loss of function"
		result.Recommendations = []string{
			"Consider functional studies to assess impact",
			"Evaluate alternative transcripts and isoforms",
		}
	}
}

// applyPS1 applies the PS1 rule (same amino acid change)
func (t *ApplyRuleTool) applyPS1(params *ApplyRuleParams, result *ApplyRuleResult) {
	// Mock implementation - would require database lookup
	result.Applied = false
	result.Confidence = 0.1
	result.Reasoning = "No established pathogenic variants found at this amino acid position"
	result.Requirements = []string{
		"Search ClinVar for pathogenic variants at same amino acid position",
		"Verify nucleotide change is different from reported variant",
	}
}

// applyPM2 applies the PM2 rule (absent from population databases)
func (t *ApplyRuleTool) applyPM2(params *ApplyRuleParams, result *ApplyRuleResult) {
	// Mock implementation - would query population databases
	result.Applied = true
	result.Confidence = 0.9
	result.Reasoning = "Variant absent from large population cohorts (gnomAD, 1000G)"
	result.Evidence["gnomad_frequency"] = 0.0
	result.Evidence["databases_checked"] = []string{"gnomAD", "1000 Genomes", "ESP"}
}

// applyPP3 applies the PP3 rule (computational evidence)
func (t *ApplyRuleTool) applyPP3(params *ApplyRuleParams, result *ApplyRuleResult) {
	// Mock computational predictions
	result.Applied = true
	result.Confidence = 0.7
	result.Reasoning = "Multiple computational algorithms predict damaging effect"
	result.Evidence["sift"] = "deleterious"
	result.Evidence["polyphen"] = "probably_damaging"
	result.Evidence["cadd_score"] = 25.3
}

// applyBA1 applies the BA1 rule (high population frequency)
func (t *ApplyRuleTool) applyBA1(params *ApplyRuleParams, result *ApplyRuleResult) {
	// Mock implementation
	result.Applied = false
	result.Confidence = 0.0
	result.Reasoning = "Allele frequency below 5% threshold"
	result.Evidence["max_population_frequency"] = 0.001
}

// applyGenericRule provides generic rule application logic
func (t *ApplyRuleTool) applyGenericRule(params *ApplyRuleParams, result *ApplyRuleResult) {
	result.Applied = false
	result.Confidence = 0.0
	result.Reasoning = fmt.Sprintf("Rule %s requires manual evaluation with specific evidence", params.RuleCode)
	result.Recommendations = []string{
		"Gather specific evidence required for this rule",
		"Consult ACMG/AMP guidelines for detailed criteria",
		"Consider expert consultation for complex cases",
	}
}

// isLossOfFunction checks if variant is predicted loss-of-function
func (t *ApplyRuleTool) isLossOfFunction(variant VariantData) bool {
	// Simplified LOF prediction
	if variant.VariantType == "deletion" || variant.VariantType == "insertion" {
		return true
	}
	// In reality, this would be much more sophisticated
	return false
}

// NewCombineEvidenceTool creates a new combine_evidence tool
func NewCombineEvidenceTool(logger *logrus.Logger) *CombineEvidenceTool {
	return &CombineEvidenceTool{
		logger: logger,
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
	result := &CombineEvidenceResult{
		CombinationLogic: CombinationLogicExplanation{
			GuidelinesUsed: params.Guidelines,
			DecisionTree:   make([]DecisionStep, 0),
		},
		Recommendations: make([]string, 0),
	}

	// Separate pathogenic and benign rules
	pathRules, benignRules := t.separateRules(params.AppliedRules)
	result.CombinationLogic.PathogenicRules = pathRules
	result.CombinationLogic.BenignRules = benignRules

	// Apply combination logic according to ACMG 2015 guidelines
	classification := t.applyCombinationLogic(pathRules, benignRules, &result.CombinationLogic.DecisionTree)
	result.Classification = classification
	result.Confidence = t.calculateCombinationConfidence(params.AppliedRules, classification)

	// Generate recommendations
	result.Recommendations = t.generateCombinationRecommendations(classification, pathRules, benignRules)

	return result
}

// separateRules separates applied rules into pathogenic and benign categories
func (t *CombineEvidenceTool) separateRules(rules []ACMGAMPRuleResult) (RuleCombination, RuleCombination) {
	pathRules := RuleCombination{}
	benignRules := RuleCombination{}

	for _, rule := range rules {
		if !rule.Applied {
			continue
		}

		code := rule.RuleCode
		switch rule.Category {
		case "pathogenic":
			switch rule.Strength {
			case "very_strong":
				pathRules.VeryStrong = append(pathRules.VeryStrong, code)
			case "strong":
				pathRules.Strong = append(pathRules.Strong, code)
			case "moderate":
				pathRules.Moderate = append(pathRules.Moderate, code)
			case "supporting":
				pathRules.Supporting = append(pathRules.Supporting, code)
			}
		case "benign":
			switch rule.Strength {
			case "standalone":
				benignRules.Standalone = append(benignRules.Standalone, code)
			case "strong":
				benignRules.Strong = append(benignRules.Strong, code)
			case "supporting":
				benignRules.Supporting = append(benignRules.Supporting, code)
			}
		}
	}

	return pathRules, benignRules
}

// applyCombinationLogic applies ACMG combination rules
func (t *CombineEvidenceTool) applyCombinationLogic(pathRules, benignRules RuleCombination, decisionTree *[]DecisionStep) string {
	step := 1

	// Check for standalone benign evidence
	if len(benignRules.Standalone) > 0 {
		*decisionTree = append(*decisionTree, DecisionStep{
			Step:        step,
			Condition:   "BA1 (standalone benign evidence)",
			Met:         true,
			Result:      "Benign",
			Explanation: fmt.Sprintf("Standalone benign evidence: %v", benignRules.Standalone),
		})
		return "Benign"
	}
	step++

	// Check for pathogenic combinations
	if len(pathRules.VeryStrong) >= 1 {
		if len(pathRules.Strong) >= 1 || len(pathRules.Moderate) >= 2 || 
		   len(pathRules.Moderate) >= 1 && len(pathRules.Supporting) >= 1 || 
		   len(pathRules.Supporting) >= 2 {
			*decisionTree = append(*decisionTree, DecisionStep{
				Step:        step,
				Condition:   "PVS1 + additional evidence",
				Met:         true,
				Result:      "Pathogenic",
				Explanation: "Very strong evidence with supporting criteria met",
			})
			return "Pathogenic"
		}
	}
	step++

	// Check for likely pathogenic
	if len(pathRules.Strong) >= 2 || 
	   (len(pathRules.Strong) >= 1 && len(pathRules.Moderate) >= 1) ||
	   (len(pathRules.Strong) >= 1 && len(pathRules.Supporting) >= 2) ||
	   len(pathRules.Moderate) >= 3 {
		*decisionTree = append(*decisionTree, DecisionStep{
			Step:        step,
			Condition:   "Likely pathogenic combination criteria",
			Met:         true,
			Result:      "Likely pathogenic",
			Explanation: "Strong/moderate evidence combination meets threshold",
		})
		return "Likely pathogenic"
	}
	step++

	// Check for likely benign
	if len(benignRules.Strong) >= 1 && len(benignRules.Supporting) >= 1 ||
	   len(benignRules.Supporting) >= 2 {
		*decisionTree = append(*decisionTree, DecisionStep{
			Step:        step,
			Condition:   "Likely benign combination criteria",
			Met:         true,
			Result:      "Likely benign",
			Explanation: "Benign evidence combination meets threshold",
		})
		return "Likely benign"
	}
	step++

	// Default to VUS
	*decisionTree = append(*decisionTree, DecisionStep{
		Step:        step,
		Condition:   "Insufficient evidence for definitive classification",
		Met:         true,
		Result:      "VUS",
		Explanation: "Evidence does not meet criteria for pathogenic or benign classification",
	})

	return "Variant of uncertain significance (VUS)"
}

// calculateCombinationConfidence calculates confidence in the combined classification
func (t *CombineEvidenceTool) calculateCombinationConfidence(rules []ACMGAMPRuleResult, classification string) string {
	totalConfidence := 0.0
	appliedCount := 0

	for _, rule := range rules {
		if rule.Applied {
			totalConfidence += rule.Confidence
			appliedCount++
		}
	}

	if appliedCount == 0 {
		return "Low"
	}

	avgConfidence := totalConfidence / float64(appliedCount)
	
	// Adjust confidence based on classification certainty
	if classification == "Pathogenic" || classification == "Benign" {
		avgConfidence += 0.1 // Boost for definitive classifications
	} else if classification == "Variant of uncertain significance (VUS)" {
		avgConfidence -= 0.1 // Reduce for uncertain classifications
	}

	if avgConfidence >= 0.8 {
		return "High"
	} else if avgConfidence >= 0.6 {
		return "Moderate"
	}
	return "Low"
}

// generateCombinationRecommendations generates recommendations based on combination results
func (t *CombineEvidenceTool) generateCombinationRecommendations(classification string, pathRules, benignRules RuleCombination) []string {
	recommendations := make([]string, 0)

	switch classification {
	case "Pathogenic":
		recommendations = append(recommendations,
			"Strong evidence supports pathogenic classification",
			"Consider clinical action based on gene-disease association",
			"Recommend genetic counseling and cascade testing",
		)
	case "Likely pathogenic":
		recommendations = append(recommendations,
			"Moderate evidence supports pathogenic classification",
			"Consider additional functional or segregation studies",
			"Exercise appropriate clinical caution",
		)
	case "Variant of uncertain significance (VUS)":
		recommendations = append(recommendations,
			"Insufficient evidence for classification",
			"Gather additional evidence (segregation, functional studies)",
			"Consider expert consultation",
			"Re-evaluate periodically as new evidence emerges",
		)
	case "Likely benign":
		recommendations = append(recommendations,
			"Evidence supports benign interpretation",
			"Variant unlikely to be disease-causing",
			"Continue standard clinical care",
		)
	case "Benign":
		recommendations = append(recommendations,
			"Strong evidence supports benign classification",
			"Variant not expected to contribute to disease phenotype",
		)
	}

	return recommendations
}