package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/acmg-amp-mcp-server/internal/domain"
	"github.com/acmg-amp-mcp-server/internal/mcp/protocol"
	"github.com/acmg-amp-mcp-server/internal/service"
)

// ClassifyVariantTool implements the classify_variant MCP tool
type ClassifyVariantTool struct {
	logger            *logrus.Logger
	classifierService *service.ClassifierService
	inputParser       domain.InputParser
}

// ClassifyVariantParams defines parameters for the classify_variant tool
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
func NewClassifyVariantTool(logger *logrus.Logger, classifierService *service.ClassifierService, inputParser domain.InputParser) *ClassifyVariantTool {
	return &ClassifyVariantTool{
		logger:            logger,
		classifierService: classifierService,
		inputParser:       inputParser,
	}
}

// NewClassifyVariantToolLegacy creates a new classify_variant tool without input parser (for backward compatibility)
func NewClassifyVariantToolLegacy(logger *logrus.Logger, classifierService *service.ClassifierService) *ClassifyVariantTool {
	return &ClassifyVariantTool{
		logger:            logger,
		classifierService: classifierService,
		inputParser:       service.NewInputParserService(), // Use default input parser
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
		Description: "Classify a genetic variant using ACMG/AMP guidelines with comprehensive evidence evaluation. Supports both HGVS notation and gene symbol notation for flexible input.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"hgvs_notation": map[string]interface{}{
					"type":        "string",
					"description": "HGVS notation of the variant (e.g., 'NM_000492.3:c.1521_1523delCTT', 'NC_000017.11:g.43104261G>T')",
					"pattern":     "^(NC_|NM_|NP_|NG_|NR_|XM_|XR_).*",
					"examples":    []string{"NM_000492.3:c.1521_1523delCTT", "NC_000017.11:g.43104261G>T", "NP_000483.3:p.Phe508del"},
				},
				"gene_symbol_notation": map[string]interface{}{
					"type":        "string",
					"description": "Gene symbol notation in various supported formats: standalone gene (e.g., 'BRCA1'), gene with coding variant (e.g., 'TP53:c.273G>A'), or gene with protein variant (e.g., 'BRCA1 p.Cys61Gly')",
					"examples":    []string{"BRCA1", "TP53:c.273G>A", "BRCA1 p.Cys61Gly", "CFTR", "HLA-A"},
				},
				"variant_type": map[string]interface{}{
					"type":        "string",
					"description": "Type of variant (SNV, indel, CNV, etc.)",
					"enum":        []string{"SNV", "indel", "CNV", "SV", "fusion"},
				},
				"gene_symbol": map[string]interface{}{
					"type":        "string",
					"description": "Legacy HGNC gene symbol field (deprecated: use gene_symbol_notation instead)",
					"pattern":     "^[A-Z][A-Z0-9-]*$",
					"deprecated":  true,
				},
				"transcript_id": map[string]interface{}{
					"type":        "string",
					"description": "RefSeq transcript identifier (e.g., 'NM_000492.3')",
					"pattern":     "^(NM_|NR_|XM_|XR_).*",
					"examples":    []string{"NM_000492.3", "NM_000546.5"},
				},
				"preferred_isoform": map[string]interface{}{
					"type":        "string",
					"description": "Override transcript selection with preferred RefSeq isoform (e.g., 'NM_000492.3')",
					"pattern":     "^(NM_|NR_|XM_|XR_).*",
					"examples":    []string{"NM_000492.3", "NM_007294.4"},
				},
				"clinical_context": map[string]interface{}{
					"type":        "string",
					"description": "Clinical context or phenotype information for enhanced interpretation",
					"examples":    []string{"Breast cancer susceptibility", "Cystic fibrosis", "Cardiomyopathy"},
				},
				"include_evidence": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether to include detailed evidence summary in the response",
					"default":     false,
				},
			},
			"oneOf": []map[string]interface{}{
				{
					"required": []string{"hgvs_notation"},
					"title":    "HGVS Notation Input",
				},
				{
					"required": []string{"gene_symbol_notation"},
					"title":    "Gene Symbol Notation Input",
				},
				{
					"required": []string{"gene_symbol"},
					"title":    "Legacy Gene Symbol Input (deprecated)",
				},
			},
			"additionalProperties": false,
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
	if err := ParseParams(params, target); err != nil {
		return err
	}

	// Validate that at least one notation format is provided
	if err := t.validateNotationParameters(target); err != nil {
		return err
	}

	// Validate the provided notation formats
	if err := t.validateNotationFormats(target); err != nil {
		return err
	}

	// Validate other parameters
	if err := t.validateAdditionalParameters(target); err != nil {
		return err
	}

	return nil
}

// validateNotationParameters ensures either HGVS or gene symbol notation is provided
func (t *ClassifyVariantTool) validateNotationParameters(params *ClassifyVariantParams) error {
	hasHGVS := strings.TrimSpace(params.HGVSNotation) != ""
	hasGeneSymbol := strings.TrimSpace(params.GeneSymbolNotation) != ""
	hasLegacyGeneSymbol := strings.TrimSpace(params.GeneSymbol) != ""

	// Check if at least one notation format is provided
	if !hasHGVS && !hasGeneSymbol && !hasLegacyGeneSymbol {
		return fmt.Errorf("either 'hgvs_notation' or 'gene_symbol_notation' is required. Examples: " +
			"HGVS: 'NM_000492.3:c.1521_1523delCTT', " +
			"Gene Symbol: 'BRCA1', 'TP53:c.273G>A', 'BRCA1 p.Cys61Gly'")
	}

	// Handle legacy gene_symbol field for backward compatibility
	if hasLegacyGeneSymbol && !hasGeneSymbol {
		t.logger.Debug("Using legacy gene_symbol field for backward compatibility")
		// For legacy support, treat gene_symbol as gene_symbol_notation if it looks like a gene symbol
		if t.isGeneSymbolFormat(params.GeneSymbol) {
			params.GeneSymbolNotation = params.GeneSymbol
		}
	}

	return nil
}

// validateNotationFormats validates the format of provided notation strings
func (t *ClassifyVariantTool) validateNotationFormats(params *ClassifyVariantParams) error {
	// Validate HGVS notation if provided
	if params.HGVSNotation != "" {
		if !t.isValidHGVSFormat(params.HGVSNotation) {
			return fmt.Errorf("invalid HGVS notation format: %s. Expected format like 'NM_000492.3:c.1521_1523delCTT'", params.HGVSNotation)
		}
	}

	// Validate gene symbol notation if provided
	if params.GeneSymbolNotation != "" {
		if err := t.validateGeneSymbolNotation(params.GeneSymbolNotation); err != nil {
			return fmt.Errorf("invalid gene symbol notation: %w", err)
		}
	}

	// HGVS takes priority when both are provided
	if params.HGVSNotation != "" && params.GeneSymbolNotation != "" {
		t.logger.Debug("Both HGVS and gene symbol notation provided - HGVS takes priority")
	}

	return nil
}

// validateAdditionalParameters validates other optional parameters
func (t *ClassifyVariantTool) validateAdditionalParameters(params *ClassifyVariantParams) error {
	// Validate preferred isoform if provided
	if params.PreferredIsoform != "" {
		if !t.isValidTranscriptFormat(params.PreferredIsoform) {
			return fmt.Errorf("invalid preferred_isoform format: %s. Expected RefSeq format like 'NM_000492.3'", params.PreferredIsoform)
		}
	}

	// Validate variant type if provided
	if params.VariantType != "" {
		validTypes := []string{"SNV", "indel", "CNV", "SV", "fusion"}
		if !t.isValidVariantType(params.VariantType, validTypes) {
			return fmt.Errorf("invalid variant_type: %s. Valid types: %s", params.VariantType, strings.Join(validTypes, ", "))
		}
	}

	return nil
}

// validateGeneSymbolNotation validates gene symbol notation using the input parser
func (t *ClassifyVariantTool) validateGeneSymbolNotation(notation string) error {
	if t.inputParser == nil {
		// Fallback validation if no input parser available
		return t.basicGeneSymbolValidation(notation)
	}

	// Use the enhanced input parser to validate gene symbol
	_, err := t.inputParser.ParseGeneSymbol(notation)
	if err != nil {
		return fmt.Errorf("gene symbol validation failed: %w. Supported formats: 'BRCA1', 'TP53:c.273G>A', 'BRCA1 p.Cys61Gly'", err)
	}

	return nil
}

// basicGeneSymbolValidation provides fallback validation when input parser is not available
func (t *ClassifyVariantTool) basicGeneSymbolValidation(notation string) error {
	notation = strings.TrimSpace(notation)
	if notation == "" {
		return fmt.Errorf("gene symbol notation cannot be empty")
	}

	// Basic validation for common formats
	if t.isGeneSymbolFormat(notation) {
		return nil
	}

	// Check for gene:variant format
	if strings.Contains(notation, ":") {
		parts := strings.Split(notation, ":")
		if len(parts) == 2 && t.isGeneSymbolFormat(parts[0]) {
			return nil
		}
	}

	// Check for gene protein format
	if strings.Contains(notation, " p.") {
		parts := strings.Split(notation, " ")
		if len(parts) >= 2 && t.isGeneSymbolFormat(parts[0]) && strings.HasPrefix(parts[1], "p.") {
			return nil
		}
	}

	return fmt.Errorf("unrecognized gene symbol format")
}

// isGeneSymbolFormat checks if a string looks like a valid gene symbol
func (t *ClassifyVariantTool) isGeneSymbolFormat(symbol string) bool {
	if len(symbol) == 0 || len(symbol) > 15 {
		return false
	}
	
	// Basic HUGO pattern: starts with letter, contains only letters, numbers, and hyphens
	if !strings.ContainsAny(symbol[:1], "ABCDEFGHIJKLMNOPQRSTUVWXYZ") {
		return false
	}
	
	for _, r := range symbol {
		if !strings.ContainsRune("ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-", r) {
			return false
		}
	}
	
	// Cannot start or end with hyphen
	if strings.HasPrefix(symbol, "-") || strings.HasSuffix(symbol, "-") {
		return false
	}
	
	return true
}

// isValidTranscriptFormat validates transcript identifier format
func (t *ClassifyVariantTool) isValidTranscriptFormat(transcript string) bool {
	// Check for RefSeq format (NM_, NR_, XM_, XR_)
	prefixes := []string{"NM_", "NR_", "XM_", "XR_"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(transcript, prefix) {
			return len(transcript) > len(prefix)
		}
	}
	return false
}

// isValidVariantType checks if variant type is in the allowed list
func (t *ClassifyVariantTool) isValidVariantType(variantType string, validTypes []string) bool {
	for _, validType := range validTypes {
		if strings.EqualFold(variantType, validType) {
			return true
		}
	}
	return false
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
	// Validate that classifier service is available
	if t.classifierService == nil {
		return nil, fmt.Errorf("classification service not configured")
	}

	// Determine the input notation and prepare for classification
	hgvsNotation, geneSymbol, err := t.prepareNotationForClassification(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare notation for classification: %w", err)
	}

	t.logger.WithFields(logrus.Fields{
		"hgvs_notation": hgvsNotation,
		"gene_symbol":   geneSymbol,
	}).Debug("Starting variant classification")

	// Convert MCP tool params to service params
	serviceParams := &service.ClassifyVariantParams{
		HGVSNotation:    hgvsNotation,
		VariantType:     params.VariantType,
		GeneSymbol:      geneSymbol,
		TranscriptID:    params.TranscriptID,
		ClinicalContext: params.ClinicalContext,
		IncludeEvidence: params.IncludeEvidence,
	}

	// Add preferred isoform if specified
	if params.PreferredIsoform != "" {
		serviceParams.TranscriptID = params.PreferredIsoform
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

// prepareNotationForClassification determines the appropriate notation to use for classification
func (t *ClassifyVariantTool) prepareNotationForClassification(ctx context.Context, params *ClassifyVariantParams) (hgvs, geneSymbol string, err error) {
	// HGVS takes priority when both are provided
	if params.HGVSNotation != "" {
		t.logger.Debug("Using provided HGVS notation for classification")
		
		// Try to extract gene symbol from HGVS if not already provided
		if params.GeneSymbol == "" && t.inputParser != nil {
			if variant, parseErr := t.inputParser.ParseVariant(params.HGVSNotation); parseErr == nil && variant.GeneSymbol != "" {
				geneSymbol = variant.GeneSymbol
				t.logger.WithField("extracted_gene", geneSymbol).Debug("Extracted gene symbol from HGVS")
			}
		} else {
			geneSymbol = params.GeneSymbol
		}
		
		return params.HGVSNotation, geneSymbol, nil
	}

	// Use gene symbol notation - need to convert to HGVS for classification service
	if params.GeneSymbolNotation != "" {
		t.logger.WithField("gene_notation", params.GeneSymbolNotation).Debug("Converting gene symbol notation to HGVS for classification")
		
		// Try to parse the gene symbol notation and convert to HGVS
		if t.inputParser != nil {
			variant, parseErr := t.inputParser.ParseGeneSymbol(params.GeneSymbolNotation)
			if parseErr != nil {
				return "", "", fmt.Errorf("failed to parse gene symbol notation: %w", parseErr)
			}
			
			// Use the best available HGVS representation
			hgvsResult := t.selectBestHGVS(variant)
			if hgvsResult == "" {
				return "", "", fmt.Errorf("could not generate HGVS notation from gene symbol: %s", params.GeneSymbolNotation)
			}
			
			return hgvsResult, variant.GeneSymbol, nil
		}
		
		// Fallback: return gene symbol notation as-is if no parser available
		// This relies on the classification service to handle gene symbols
		return params.GeneSymbolNotation, t.extractGeneFromNotation(params.GeneSymbolNotation), nil
	}

	// Should not reach here due to validation, but handle legacy case
	if params.GeneSymbol != "" {
		return "", params.GeneSymbol, nil
	}

	return "", "", fmt.Errorf("no valid notation provided")
}

// selectBestHGVS selects the best HGVS representation from a parsed variant
func (t *ClassifyVariantTool) selectBestHGVS(variant *domain.StandardizedVariant) string {
	// Prefer coding notation, then genomic, then protein
	if variant.HGVSCoding != "" {
		return variant.HGVSCoding
	}
	if variant.HGVSGenomic != "" {
		return variant.HGVSGenomic
	}
	if variant.HGVSProtein != "" {
		return variant.HGVSProtein
	}
	return ""
}

// extractGeneFromNotation extracts gene symbol from various notation formats
func (t *ClassifyVariantTool) extractGeneFromNotation(notation string) string {
	// Handle gene:variant format
	if strings.Contains(notation, ":") {
		parts := strings.Split(notation, ":")
		if len(parts) >= 1 && t.isGeneSymbolFormat(parts[0]) {
			return parts[0]
		}
	}
	
	// Handle gene protein format
	if strings.Contains(notation, " p.") {
		parts := strings.Split(notation, " ")
		if len(parts) >= 1 && t.isGeneSymbolFormat(parts[0]) {
			return parts[0]
		}
	}
	
	// Handle standalone gene symbol
	if t.isGeneSymbolFormat(notation) {
		return notation
	}
	
	return ""
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

