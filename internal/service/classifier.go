package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/acmg-amp-mcp-server/internal/domain"
	"github.com/acmg-amp-mcp-server/pkg/external"
)

// ClassifierService implements ACMG/AMP variant classification
type ClassifierService struct {
	logger              *logrus.Logger
	knowledgeBaseService *external.KnowledgeBaseService
	inputParser         domain.InputParser
	transcriptResolver  domain.GeneTranscriptResolver
	ruleEngine          *ACMGAMPRuleEngine
}

// NewClassifierService creates a new classifier service
func NewClassifierService(
	logger *logrus.Logger,
	knowledgeBaseService *external.KnowledgeBaseService,
	inputParser domain.InputParser,
	transcriptResolver domain.GeneTranscriptResolver,
) *ClassifierService {
	return &ClassifierService{
		logger:              logger,
		knowledgeBaseService: knowledgeBaseService,
		inputParser:         inputParser,
		transcriptResolver:  transcriptResolver,
		ruleEngine:          NewACMGAMPRuleEngine(logger),
	}
}

// ClassifyVariant performs complete ACMG/AMP classification workflow
func (c *ClassifierService) ClassifyVariant(ctx context.Context, params *ClassifyVariantParams) (*ClassifyVariantResult, error) {
	startTime := time.Now()
	
	// Validate that at least one notation format is provided
	if err := c.validateNotationInput(params); err != nil {
		return nil, fmt.Errorf("invalid input parameters: %w", err)
	}
	
	// Determine input type and log accordingly
	inputType, inputValue := c.determineInputType(params)
	c.logger.WithFields(logrus.Fields{
		"input_type":  inputType,
		"input_value": inputValue,
	}).Info("Starting variant classification")

	// Step 1: Parse and standardize input notation to HGVS format
	variant, hgvsNotation, err := c.prepareVariantForClassification(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare variant for classification: %w", err)
	}

	// Step 2: Gather evidence from external databases
	evidence, err := c.knowledgeBaseService.GatherEvidence(ctx, variant)
	if err != nil {
		c.logger.WithError(err).Warn("Failed to gather complete evidence, proceeding with available data")
		// Continue with partial evidence
		evidence = &domain.AggregatedEvidence{}
	}

	// Step 3: Apply ACMG/AMP rules
	ruleResults, err := c.ruleEngine.EvaluateAllRules(ctx, variant, evidence)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate ACMG/AMP rules: %w", err)
	}

	// Step 4: Combine evidence according to ACMG/AMP guidelines
	classification, confidence := c.ruleEngine.CombineEvidence(ruleResults)

	// Step 5: Generate recommendations
	recommendations := c.generateRecommendations(classification, confidence, evidence)

	// Step 6: Create evidence summary
	evidenceSummary := c.generateEvidenceSummary(ruleResults, evidence)

	result := &ClassifyVariantResult{
		VariantID:       variant.ID,
		Classification:  classification.String(),
		Confidence:      confidence.String(),
		AppliedRules:    convertRuleResults(ruleResults),
		EvidenceSummary: evidenceSummary,
		Recommendations: recommendations,
		ProcessingTime:  time.Since(startTime),
		InputNotation:   hgvsNotation, // Store the final HGVS notation used
	}

	c.logger.WithFields(logrus.Fields{
		"variant_id":      result.VariantID,
		"classification":  result.Classification,
		"confidence":      result.Confidence,
		"processing_time": result.ProcessingTime,
		"rules_applied":   len(result.AppliedRules),
		"input_type":      inputType,
	}).Info("Variant classification completed")

	return result, nil
}

// ValidateHGVS validates HGVS notation and returns normalized form
func (c *ClassifierService) ValidateHGVS(hgvsNotation string) (*HGVSValidationResult, error) {
	c.logger.WithField("hgvs_notation", hgvsNotation).Debug("Validating HGVS notation")

	// Parse the variant
	variant, err := c.inputParser.ParseVariant(hgvsNotation)
	if err != nil {
		return &HGVSValidationResult{
			IsValid:      false,
			ErrorMessage: err.Error(),
		}, nil
	}

	return &HGVSValidationResult{
		IsValid:           true,
		NormalizedHGVS:    variant.HGVSCoding, // Use the parsed/normalized version
		VariantType:       variant.VariantType.String(),
		GeneSymbol:        variant.GeneSymbol,
		TranscriptID:      variant.TranscriptID,
		GenomicPosition:   fmt.Sprintf("chr%s:g.%d", variant.Chromosome, variant.Position),
		PredictedProtein:  variant.HGVSProtein,
	}, nil
}

// ApplyRule applies a specific ACMG/AMP rule to a variant
func (c *ClassifierService) ApplyRule(ctx context.Context, params *ApplyRuleParams) (*RuleEvaluationResult, error) {
	c.logger.WithFields(logrus.Fields{
		"rule_code":       params.RuleCode,
		"hgvs_notation":   params.HGVSNotation,
	}).Debug("Applying ACMG/AMP rule")

	// Parse variant
	variant, err := c.inputParser.ParseVariant(params.HGVSNotation)
	if err != nil {
		return nil, fmt.Errorf("failed to parse variant: %w", err)
	}

	// Gather evidence if not provided
	var evidence *domain.AggregatedEvidence
	if params.Evidence != nil {
		evidence = params.Evidence
	} else {
		evidence, err = c.knowledgeBaseService.GatherEvidence(ctx, variant)
		if err != nil {
			c.logger.WithError(err).Warn("Failed to gather evidence for rule evaluation")
			evidence = &domain.AggregatedEvidence{}
		}
	}

	// Apply specific rule
	ruleResult, err := c.ruleEngine.EvaluateRule(ctx, params.RuleCode, variant, evidence)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate rule %s: %w", params.RuleCode, err)
	}

	return &RuleEvaluationResult{
		RuleCode:    ruleResult.Code,
		RuleName:    ruleResult.Name,
		Category:    ruleResult.Category.String(),
		Strength:    ruleResult.Strength.String(),
		Applied:     ruleResult.Applied,
		Confidence:  ruleResult.Confidence,
		Evidence:    ruleResult.Evidence,
		Reasoning:   ruleResult.Reasoning,
		MetCriteria: ruleResult.MetCriteria,
	}, nil
}

// CombineEvidence combines evidence according to ACMG/AMP guidelines
func (c *ClassifierService) CombineEvidence(ruleResults []RuleResult) (*EvidenceCombinationResult, error) {
	c.logger.WithField("rule_count", len(ruleResults)).Debug("Combining evidence")

	// Convert to internal format
	internalRuleResults := make([]domain.ACMGAMPRuleResult, len(ruleResults))
	for i, rr := range ruleResults {
		internalRuleResults[i] = domain.ACMGAMPRuleResult{
			Code:        rr.RuleCode,
			Name:        rr.RuleName,
			Category:    domain.RuleCategory(rr.Category),
			Strength:    domain.RuleStrength(rr.Strength),
			Applied:     rr.Applied,
			Confidence:  rr.Confidence,
			Evidence:    rr.Evidence,
			Reasoning:   rr.Reasoning,
		}
	}

	// Use rule engine to combine evidence
	classification, confidence := c.ruleEngine.CombineEvidence(internalRuleResults)

	return &EvidenceCombinationResult{
		Classification:  classification.String(),
		Confidence:      confidence.String(),
		CombinationRule: c.determineCombinationRule(internalRuleResults),
		Summary:         c.generateCombinationSummary(internalRuleResults, classification),
	}, nil
}

// generateRecommendations creates actionable recommendations based on classification
func (c *ClassifierService) generateRecommendations(classification domain.Classification, confidence domain.ConfidenceLevel, evidence *domain.AggregatedEvidence) []string {
	recommendations := make([]string, 0)

	switch classification {
	case domain.PATHOGENIC, domain.LIKELY_PATHOGENIC:
		recommendations = append(recommendations, "Consider genetic counseling for the patient and family")
		recommendations = append(recommendations, "Evaluate for medical management based on associated condition")
		if classification == domain.PATHOGENIC {
			recommendations = append(recommendations, "Consider cascade testing for at-risk family members")
		}

	case domain.BENIGN, domain.LIKELY_BENIGN:
		recommendations = append(recommendations, "No specific follow-up required for this variant")
		if classification == domain.LIKELY_BENIGN {
			recommendations = append(recommendations, "Consider periodic re-evaluation as new evidence emerges")
		}

	case domain.VUS:
		recommendations = append(recommendations, "Consider functional studies if clinically indicated")
		recommendations = append(recommendations, "Evaluate family segregation if possible")
		recommendations = append(recommendations, "Periodic re-evaluation as new evidence becomes available")
		if evidence.PopulationData != nil && evidence.PopulationData.AlleleFrequency == 0 {
			recommendations = append(recommendations, "Consider population frequency studies in relevant ethnic groups")
		}
	}

	// Confidence-based recommendations
	if confidence == domain.LOW {
		recommendations = append(recommendations, "Low confidence classification - consider additional evidence")
	}

	return recommendations
}

// generateEvidenceSummary creates a human-readable evidence summary
func (c *ClassifierService) generateEvidenceSummary(ruleResults []domain.ACMGAMPRuleResult, evidence *domain.AggregatedEvidence) string {
	appliedRules := make([]string, 0)
	for _, rule := range ruleResults {
		if rule.Applied {
			appliedRules = append(appliedRules, rule.Code)
		}
	}

	summary := fmt.Sprintf("Applied ACMG/AMP criteria: %s", joinStrings(appliedRules))
	
	if evidence.ClinVarData != nil && evidence.ClinVarData.ClinicalSignificance != "" {
		summary += fmt.Sprintf(". ClinVar classification: %s", evidence.ClinVarData.ClinicalSignificance)
	}
	
	if evidence.PopulationData != nil {
		summary += fmt.Sprintf(". Population frequency: %.6f", evidence.PopulationData.AlleleFrequency)
	}

	return summary
}

// determineCombinationRule determines which ACMG/AMP combination rule was used
func (c *ClassifierService) determineCombinationRule(rules []domain.ACMGAMPRuleResult) string {
	pathogenicStrong := 0
	pathogenicModerate := 0
	pathogenicSupporting := 0
	veryStrong := false

	for _, rule := range rules {
		if rule.Applied && rule.Category == domain.PATHOGENIC_RULE {
			switch rule.Strength {
			case domain.VERY_STRONG:
				veryStrong = true
			case domain.STRONG:
				pathogenicStrong++
			case domain.MODERATE:
				pathogenicModerate++
			case domain.SUPPORTING:
				pathogenicSupporting++
			}
		}
	}

	if veryStrong {
		return "PVS1 + other criteria"
	} else if pathogenicStrong >= 2 {
		return "Two strong pathogenic criteria"
	} else if pathogenicStrong >= 1 && pathogenicModerate >= 1 {
		return "One strong + one moderate pathogenic criteria"
	}

	return "Standard ACMG/AMP combination rules"
}

// generateCombinationSummary creates summary of evidence combination
func (c *ClassifierService) generateCombinationSummary(rules []domain.ACMGAMPRuleResult, classification domain.Classification) string {
	appliedCount := 0
	for _, rule := range rules {
		if rule.Applied {
			appliedCount++
		}
	}

	return fmt.Sprintf("Classification '%s' based on %d applied ACMG/AMP criteria", 
		classification.String(), appliedCount)
}

// Helper function to join strings with proper formatting
func joinStrings(strs []string) string {
	if len(strs) == 0 {
		return "none"
	}
	return fmt.Sprintf("%v", strs)
}

// Convert internal rule results to API format
func convertRuleResults(results []domain.ACMGAMPRuleResult) []ACMGAMPRuleResult {
	converted := make([]ACMGAMPRuleResult, len(results))
	for i, r := range results {
		converted[i] = ACMGAMPRuleResult{
			RuleCode:    r.Code,
			RuleName:    r.Name,
			Category:    r.Category.String(),
			Strength:    r.Strength.String(),
			Applied:     r.Applied,
			Confidence:  r.Confidence,
			Evidence:    r.Evidence,
			Reasoning:   r.Reasoning,
		}
	}
	return converted
}

// Data structures for the service API

// ClassifyVariantParams parameters for variant classification
type ClassifyVariantParams struct {
	// Either HGVS notation OR gene symbol notation is required
	HGVSNotation        string `json:"hgvs_notation,omitempty"`
	GeneSymbolNotation  string `json:"gene_symbol_notation,omitempty"`
	
	// Optional parameters
	VariantType        string `json:"variant_type,omitempty"`
	GeneSymbol         string `json:"gene_symbol,omitempty"`         // Legacy field for backward compatibility
	TranscriptID       string `json:"transcript_id,omitempty"`
	PreferredIsoform   string `json:"preferred_isoform,omitempty"`   // Override transcript selection
	ClinicalContext    string `json:"clinical_context,omitempty"`
	IncludeEvidence    bool   `json:"include_evidence,omitempty"`
}

// ClassifyVariantResult result of variant classification
type ClassifyVariantResult struct {
	VariantID       string                 `json:"variant_id"`
	Classification  string                 `json:"classification"`
	Confidence      string                 `json:"confidence"`
	AppliedRules    []ACMGAMPRuleResult    `json:"applied_rules"`
	EvidenceSummary string                 `json:"evidence_summary"`
	Recommendations []string               `json:"recommendations"`
	ProcessingTime  time.Duration          `json:"processing_time"`
	InputNotation   string                 `json:"input_notation,omitempty"` // Final HGVS notation used
}

// HGVSValidationResult result of HGVS validation
type HGVSValidationResult struct {
	IsValid           bool   `json:"is_valid"`
	NormalizedHGVS    string `json:"normalized_hgvs,omitempty"`
	VariantType       string `json:"variant_type,omitempty"`
	GeneSymbol        string `json:"gene_symbol,omitempty"`
	TranscriptID      string `json:"transcript_id,omitempty"`
	GenomicPosition   string `json:"genomic_position,omitempty"`
	PredictedProtein  string `json:"predicted_protein,omitempty"`
	ErrorMessage      string `json:"error_message,omitempty"`
}

// ApplyRuleParams parameters for applying specific rule
type ApplyRuleParams struct {
	RuleCode     string                     `json:"rule_code" validate:"required"`
	HGVSNotation string                     `json:"hgvs_notation" validate:"required"`
	Evidence     *domain.AggregatedEvidence `json:"evidence,omitempty"`
}

// RuleEvaluationResult result of rule evaluation
type RuleEvaluationResult struct {
	RuleCode    string  `json:"rule_code"`
	RuleName    string  `json:"rule_name"`
	Category    string  `json:"category"`
	Strength    string  `json:"strength"`
	Applied     bool    `json:"applied"`
	Confidence  float64 `json:"confidence"`
	Evidence    string  `json:"evidence,omitempty"`
	Reasoning   string  `json:"reasoning,omitempty"`
	MetCriteria []string `json:"met_criteria,omitempty"`
}

// RuleResult for evidence combination
type RuleResult struct {
	RuleCode    string  `json:"rule_code"`
	RuleName    string  `json:"rule_name"`
	Category    string  `json:"category"`
	Strength    string  `json:"strength"`
	Applied     bool    `json:"applied"`
	Confidence  float64 `json:"confidence"`
	Evidence    string  `json:"evidence,omitempty"`
	Reasoning   string  `json:"reasoning,omitempty"`
}

// EvidenceCombinationResult result of evidence combination
type EvidenceCombinationResult struct {
	Classification  string `json:"classification"`
	Confidence      string `json:"confidence"`
	CombinationRule string `json:"combination_rule"`
	Summary         string `json:"summary"`
}

// ACMGAMPRuleResult represents a single ACMG/AMP rule evaluation result for API
type ACMGAMPRuleResult struct {
	RuleCode    string  `json:"rule_code"`
	RuleName    string  `json:"rule_name"`
	Category    string  `json:"category"`
	Strength    string  `json:"strength"`
	Applied     bool    `json:"applied"`
	Confidence  float64 `json:"confidence"`
	Evidence    string  `json:"evidence,omitempty"`
	Reasoning   string  `json:"reasoning,omitempty"`
}

// Helper methods for enhanced ClassifyVariant functionality

// validateNotationInput ensures at least one notation format is provided
func (c *ClassifierService) validateNotationInput(params *ClassifyVariantParams) error {
	hasHGVS := params.HGVSNotation != ""
	hasGeneSymbol := params.GeneSymbolNotation != ""
	hasLegacyGene := params.GeneSymbol != ""

	if !hasHGVS && !hasGeneSymbol && !hasLegacyGene {
		return fmt.Errorf("either HGVS notation or gene symbol notation is required. Examples: 'NM_000492.3:c.1521T>G' or 'BRCA1:c.273G>A'")
	}

	return nil
}

// determineInputType identifies the type of input notation provided
func (c *ClassifierService) determineInputType(params *ClassifyVariantParams) (string, string) {
	if params.HGVSNotation != "" {
		return "hgvs", params.HGVSNotation
	}
	if params.GeneSymbolNotation != "" {
		return "gene_symbol", params.GeneSymbolNotation
	}
	if params.GeneSymbol != "" {
		return "legacy_gene_symbol", params.GeneSymbol
	}
	return "unknown", ""
}

// prepareVariantForClassification handles both HGVS and gene symbol inputs
func (c *ClassifierService) prepareVariantForClassification(ctx context.Context, params *ClassifyVariantParams) (*domain.StandardizedVariant, string, error) {
	// If HGVS notation is provided, use it directly (takes priority)
	if params.HGVSNotation != "" {
		c.logger.WithField("hgvs_notation", params.HGVSNotation).Debug("Processing HGVS notation input")
		
		variant, err := c.inputParser.ParseVariant(params.HGVSNotation)
		if err != nil {
			return nil, "", fmt.Errorf("failed to parse HGVS notation: %w", err)
		}
		
		return variant, params.HGVSNotation, nil
	}

	// Handle gene symbol notation
	var geneSymbolInput string
	if params.GeneSymbolNotation != "" {
		geneSymbolInput = params.GeneSymbolNotation
	} else if params.GeneSymbol != "" {
		// Legacy support
		geneSymbolInput = params.GeneSymbol
	}

	c.logger.WithField("gene_symbol_input", geneSymbolInput).Debug("Processing gene symbol notation input")

	// Use InputParser to handle gene symbol parsing
	variant, err := c.inputParser.ParseGeneSymbol(geneSymbolInput)
	if err != nil {
		return nil, "", fmt.Errorf("failed to parse gene symbol notation: %w", err)
	}

	// Generate HGVS notation if possible and if it's a variant (not just gene)
	var finalHGVS string
	if variant.HGVSCoding != "" {
		finalHGVS = variant.HGVSCoding
	} else if variant.HGVSGenomic != "" {
		finalHGVS = variant.HGVSGenomic
	} else {
		// For standalone gene symbols, try to resolve to a canonical transcript
		if c.transcriptResolver != nil && variant.GeneSymbol != "" {
			if transcript, err := c.transcriptResolver.ResolveGeneToTranscript(ctx, variant.GeneSymbol); err == nil {
				variant.TranscriptID = transcript.RefSeqID
				finalHGVS = fmt.Sprintf("%s:c.?", transcript.RefSeqID) // Placeholder for unknown variant
				
				c.logger.WithFields(logrus.Fields{
					"gene_symbol": variant.GeneSymbol,
					"transcript_id": transcript.RefSeqID,
				}).Debug("Resolved gene symbol to transcript")
			}
		}
		
		if finalHGVS == "" {
			finalHGVS = geneSymbolInput // Fallback to original input
		}
	}

	// Apply preferred isoform override if specified
	if params.PreferredIsoform != "" {
		c.logger.WithField("preferred_isoform", params.PreferredIsoform).Debug("Applying preferred isoform override")
		variant.TranscriptID = params.PreferredIsoform
		
		// Update HGVS notation if we have variant information
		if strings.Contains(geneSymbolInput, ":") && strings.Contains(geneSymbolInput, ".") {
			parts := strings.Split(geneSymbolInput, ":")
			if len(parts) == 2 {
				finalHGVS = fmt.Sprintf("%s:%s", params.PreferredIsoform, parts[1])
			}
		}
	}

	return variant, finalHGVS, nil
}