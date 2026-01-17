package tools

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/acmg-amp-mcp-server/internal/mcp/protocol"
	"github.com/acmg-amp-mcp-server/internal/service"
)

// ValidateHGVSTool implements the validate_hgvs MCP tool
type ValidateHGVSTool struct {
	logger            *logrus.Logger
	classifierService *service.ClassifierService
}

// ValidateHGVSParams defines parameters for the validate_hgvs tool
type ValidateHGVSParams struct {
	HGVSNotation string `json:"hgvs_notation" validate:"required"`
	StrictMode   bool   `json:"strict_mode,omitempty"`
}

// ValidateHGVSResult defines the result structure for validate_hgvs tool
// Enhanced per REQ-MCP-001 to be self-sufficient with complete information
type ValidateHGVSResult struct {
	IsValid          bool              `json:"is_valid"`
	HGVSNotation     string            `json:"hgvs_notation"`
	NormalizedHGVS   string            `json:"normalized_hgvs,omitempty"`
	ValidationIssues []ValidationIssue `json:"validation_issues,omitempty"`
	ParsedComponents HGVSComponents    `json:"parsed_components,omitempty"`
	// Enhanced fields per REQ-MCP-001
	GeneInfo         *GeneInfo         `json:"gene_info,omitempty"`
	TranscriptInfo   *TranscriptInfo   `json:"transcript_info,omitempty"`
	Suggestions      []string          `json:"suggestions,omitempty"`
}

// GeneInfo contains gene-related information (REQ-MCP-001)
type GeneInfo struct {
	Symbol string `json:"symbol"`
	Name   string `json:"name"`
	HGNCID string `json:"hgnc_id,omitempty"`
}

// TranscriptInfo contains transcript-related information (REQ-MCP-001)
type TranscriptInfo struct {
	RefSeq      string `json:"refseq"`
	Ensembl     string `json:"ensembl,omitempty"`
	IsCanonical bool   `json:"is_canonical"`
}

// ValidationIssue represents a validation problem
type ValidationIssue struct {
	Severity    string `json:"severity"`    // "error", "warning", "info"
	Code        string `json:"code"`        // Error code
	Message     string `json:"message"`     // Human-readable message
	Position    int    `json:"position"`    // Character position in HGVS string
	Suggestion  string `json:"suggestion,omitempty"` // Suggested fix
}

// HGVSComponents represents parsed HGVS components
type HGVSComponents struct {
	Reference     string `json:"reference"`      // RefSeq accession
	Version       string `json:"version"`        // Version number
	Type          string `json:"type"`           // "g", "c", "p", "n", "r", "m"
	Position      string `json:"position"`       // Position or range
	ReferenceSeq  string `json:"reference_seq"`  // Reference sequence at position
	AlteredSeq    string `json:"altered_seq"`    // Altered sequence
	VariantType   string `json:"variant_type"`   // "substitution", "deletion", "insertion", etc.
	Description   string `json:"description"`    // Full variant description
}

// NewValidateHGVSTool creates a new validate_hgvs tool
func NewValidateHGVSTool(logger *logrus.Logger, classifierService *service.ClassifierService) *ValidateHGVSTool {
	return &ValidateHGVSTool{
		logger:            logger,
		classifierService: classifierService,
	}
}

// HandleTool implements the ToolHandler interface for validate_hgvs
func (t *ValidateHGVSTool) HandleTool(ctx context.Context, req *protocol.JSONRPC2Request) *protocol.JSONRPC2Response {
	t.logger.WithField("tool", "validate_hgvs").Info("Processing HGVS validation request")

	// Parse and validate parameters
	var params ValidateHGVSParams
	if err := t.parseAndValidateParams(req.Params, &params); err != nil {
		return &protocol.JSONRPC2Response{
			Error: &protocol.RPCError{
				Code:    protocol.InvalidParams,
				Message: "Invalid parameters",
				Data:    err.Error(),
			},
		}
	}

	// Perform HGVS validation
	result := t.validateHGVS(&params)

	t.logger.WithFields(logrus.Fields{
		"hgvs":      params.HGVSNotation,
		"is_valid":  result.IsValid,
		"issues":    len(result.ValidationIssues),
	}).Info("HGVS validation completed")

	return &protocol.JSONRPC2Response{
		Result: map[string]interface{}{
			"validation": result,
		},
	}
}

// GetToolInfo returns tool metadata
func (t *ValidateHGVSTool) GetToolInfo() protocol.ToolInfo {
	return protocol.ToolInfo{
		Name:        "validate_hgvs",
		Description: "Validate HGVS notation format and parse variant components according to HGVS nomenclature standards",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"hgvs_notation": map[string]interface{}{
					"type":        "string",
					"description": "HGVS notation string to validate",
					"examples":    []string{"NM_000492.3:c.1521_1523delCTT", "NC_000007.14:g.117199644_117199645insA"},
				},
				"strict_mode": map[string]interface{}{
					"type":        "boolean",
					"description": "Enable strict validation mode with additional checks",
					"default":     false,
				},
			},
			"required": []string{"hgvs_notation"},
		},
	}
}

// ValidateParams validates tool parameters
func (t *ValidateHGVSTool) ValidateParams(params interface{}) error {
	var validateParams ValidateHGVSParams
	return t.parseAndValidateParams(params, &validateParams)
}

// parseAndValidateParams parses and validates input parameters
func (t *ValidateHGVSTool) parseAndValidateParams(params interface{}, target *ValidateHGVSParams) error {
	if err := ParseParams(params, target); err != nil {
		return err
	}

	// Validate required fields
	if target.HGVSNotation == "" {
		return fmt.Errorf("hgvs_notation is required")
	}

	return nil
}

// validateHGVS performs comprehensive HGVS validation using the classifier service
// Enhanced per REQ-MCP-001 to return self-sufficient results with gene and transcript info
func (t *ValidateHGVSTool) validateHGVS(params *ValidateHGVSParams) *ValidateHGVSResult {
	hgvs := strings.TrimSpace(params.HGVSNotation)

	// Check if classifier service is available
	if t.classifierService == nil {
		// Fall back to basic parsing for enhanced output
		return t.validateHGVSBasic(hgvs)
	}

	// Call the real validation service
	serviceResult, err := t.classifierService.ValidateHGVS(hgvs)
	if err != nil {
		// If service validation fails, fall back to basic validation with suggestions
		result := t.validateHGVSBasic(hgvs)
		result.ValidationIssues = append(result.ValidationIssues, ValidationIssue{
			Severity: "error",
			Code:     "VALIDATION_SERVICE_ERROR",
			Message:  fmt.Sprintf("Validation service error: %v", err),
			Position: 0,
		})
		return result
	}

	// Convert service result to MCP tool result with enhanced output
	result := &ValidateHGVSResult{
		IsValid:          serviceResult.IsValid,
		HGVSNotation:     hgvs,
		NormalizedHGVS:   serviceResult.NormalizedHGVS,
		ValidationIssues: make([]ValidationIssue, 0),
		ParsedComponents: HGVSComponents{
			Reference:   extractReference(serviceResult.NormalizedHGVS),
			Type:        serviceResult.VariantType,
			Position:    extractPosition(serviceResult.GenomicPosition),
			VariantType: serviceResult.VariantType,
			Description: serviceResult.PredictedProtein,
		},
		Suggestions: make([]string, 0),
	}

	// Populate enhanced GeneInfo (REQ-MCP-001)
	if serviceResult.GeneSymbol != "" {
		result.GeneInfo = &GeneInfo{
			Symbol: serviceResult.GeneSymbol,
			Name:   t.getGeneName(serviceResult.GeneSymbol),
			HGNCID: t.getHGNCID(serviceResult.GeneSymbol),
		}
	}

	// Populate enhanced TranscriptInfo (REQ-MCP-001)
	if serviceResult.TranscriptID != "" {
		result.TranscriptInfo = &TranscriptInfo{
			RefSeq:      serviceResult.TranscriptID,
			Ensembl:     t.getEnsemblID(serviceResult.TranscriptID),
			IsCanonical: t.isCanonicalTranscript(serviceResult.TranscriptID, serviceResult.GeneSymbol),
		}
	}

	// Add validation issue and suggestions if not valid
	if !serviceResult.IsValid {
		result.ValidationIssues = append(result.ValidationIssues, ValidationIssue{
			Severity: "error",
			Code:     "HGVS_INVALID",
			Message:  serviceResult.ErrorMessage,
			Position: 0,
		})
		// Generate suggestions for invalid input
		result.Suggestions = t.generateSuggestions(hgvs, serviceResult.ErrorMessage)
	}

	return result
}

// validateHGVSBasic performs basic HGVS validation without the classifier service
// Used as fallback when service is not available, still provides enhanced output
func (t *ValidateHGVSTool) validateHGVSBasic(hgvs string) *ValidateHGVSResult {
	result := &ValidateHGVSResult{
		IsValid:          false,
		HGVSNotation:     hgvs,
		ValidationIssues: make([]ValidationIssue, 0),
		Suggestions:      make([]string, 0),
	}

	// Try to parse components even without service
	components, issues := t.parseHGVSComponents(hgvs)
	result.ParsedComponents = components
	result.ValidationIssues = issues

	// Extract gene info from parsed components if available
	if components.Reference != "" {
		result.TranscriptInfo = &TranscriptInfo{
			RefSeq: components.Reference + "." + components.Version,
		}
		// Try to get gene info from transcript
		if geneSymbol := t.getGeneFromTranscript(components.Reference); geneSymbol != "" {
			result.GeneInfo = &GeneInfo{
				Symbol: geneSymbol,
				Name:   t.getGeneName(geneSymbol),
				HGNCID: t.getHGNCID(geneSymbol),
			}
		}
	}

	// Check if notation is a gene symbol format (e.g., "BRCA1:c.5266dup")
	if strings.Contains(hgvs, ":") {
		parts := strings.Split(hgvs, ":")
		if len(parts) == 2 && !strings.HasPrefix(parts[0], "NM_") && !strings.HasPrefix(parts[0], "NC_") {
			// Looks like gene symbol notation
			geneSymbol := parts[0]
			result.GeneInfo = &GeneInfo{
				Symbol: geneSymbol,
				Name:   t.getGeneName(geneSymbol),
				HGNCID: t.getHGNCID(geneSymbol),
			}
			// Add suggestion to use transcript notation
			result.Suggestions = append(result.Suggestions,
				fmt.Sprintf("Consider using transcript notation: NM_XXXXX:c.%s", parts[1]),
				"Use a specific transcript for precise variant representation",
			)
		}
	}

	// Set validity based on parsing results
	result.IsValid = t.hasNoErrors(result.ValidationIssues)
	if result.IsValid {
		result.NormalizedHGVS = t.normalizeHGVS(hgvs, components)
	}

	return result
}

// getGeneName returns the full gene name for a symbol
// In production, this would query a gene database
func (t *ValidateHGVSTool) getGeneName(symbol string) string {
	// Common gene name mappings (mock - would be database lookup in production)
	geneNames := map[string]string{
		"BRCA1": "BRCA1 DNA repair associated",
		"BRCA2": "BRCA2 DNA repair associated",
		"CFTR":  "CF transmembrane conductance regulator",
		"TP53":  "tumor protein p53",
		"KRAS":  "KRAS proto-oncogene, GTPase",
		"EGFR":  "epidermal growth factor receptor",
		"MLH1":  "mutL homolog 1",
		"MSH2":  "mutS homolog 2",
	}
	if name, exists := geneNames[symbol]; exists {
		return name
	}
	return ""
}

// getHGNCID returns the HGNC ID for a gene symbol
// In production, this would query the HGNC database
func (t *ValidateHGVSTool) getHGNCID(symbol string) string {
	// Common HGNC ID mappings (mock - would be database lookup in production)
	hgncIDs := map[string]string{
		"BRCA1": "HGNC:1100",
		"BRCA2": "HGNC:1101",
		"CFTR":  "HGNC:1884",
		"TP53":  "HGNC:11998",
		"KRAS":  "HGNC:6407",
		"EGFR":  "HGNC:3236",
		"MLH1":  "HGNC:7127",
		"MSH2":  "HGNC:7325",
	}
	if id, exists := hgncIDs[symbol]; exists {
		return id
	}
	return ""
}

// getEnsemblID returns the Ensembl transcript ID for a RefSeq ID
// In production, this would query the Ensembl database
func (t *ValidateHGVSTool) getEnsemblID(refseqID string) string {
	// Common RefSeq to Ensembl mappings (mock - would be database lookup in production)
	ensemblIDs := map[string]string{
		"NM_007294": "ENST00000357654",
		"NM_000059": "ENST00000380152",
		"NM_000492": "ENST00000003084",
		"NM_000546": "ENST00000269305",
	}
	// Strip version number for lookup
	base := strings.Split(refseqID, ".")[0]
	if id, exists := ensemblIDs[base]; exists {
		return id
	}
	return ""
}

// isCanonicalTranscript checks if a transcript is the canonical one for a gene
// In production, this would query the transcript database
func (t *ValidateHGVSTool) isCanonicalTranscript(transcriptID, geneSymbol string) bool {
	// Common canonical transcript mappings (mock - would be database lookup in production)
	canonicalTranscripts := map[string]string{
		"BRCA1": "NM_007294",
		"BRCA2": "NM_000059",
		"CFTR":  "NM_000492",
		"TP53":  "NM_000546",
	}
	if canonical, exists := canonicalTranscripts[geneSymbol]; exists {
		base := strings.Split(transcriptID, ".")[0]
		return base == canonical
	}
	return false
}

// getGeneFromTranscript returns the gene symbol for a transcript ID
// In production, this would query the transcript database
func (t *ValidateHGVSTool) getGeneFromTranscript(transcriptID string) string {
	// Common transcript to gene mappings (mock - would be database lookup in production)
	transcriptGenes := map[string]string{
		"NM_007294": "BRCA1",
		"NM_000059": "BRCA2",
		"NM_000492": "CFTR",
		"NM_000546": "TP53",
	}
	base := strings.Split(transcriptID, ".")[0]
	if gene, exists := transcriptGenes[base]; exists {
		return gene
	}
	return ""
}

// generateSuggestions generates helpful suggestions for invalid HGVS input
func (t *ValidateHGVSTool) generateSuggestions(hgvs string, errorMessage string) []string {
	suggestions := make([]string, 0)

	// Check for common issues and suggest fixes
	if strings.Contains(strings.ToLower(errorMessage), "invalid reference") ||
		!strings.Contains(hgvs, "_") && !strings.HasPrefix(hgvs, "NM_") && !strings.HasPrefix(hgvs, "NC_") {
		suggestions = append(suggestions, "Ensure notation starts with a valid RefSeq accession (e.g., NM_000492.3)")
	}

	if !strings.Contains(hgvs, ":") {
		suggestions = append(suggestions, "HGVS notation requires a colon separator (e.g., NM_000492.3:c.1521_1523delCTT)")
	}

	if strings.Contains(hgvs, ":") {
		parts := strings.Split(hgvs, ":")
		if len(parts) == 2 && !strings.Contains(parts[0], ".") {
			suggestions = append(suggestions, "Include version number in reference (e.g., NM_000492.3 instead of NM_000492)")
		}
	}

	if strings.Contains(hgvs, " ") {
		suggestions = append(suggestions, "Remove spaces from HGVS notation")
	}

	if len(suggestions) == 0 {
		suggestions = append(suggestions, "Check HGVS format at: https://varnomen.hgvs.org/")
	}

	return suggestions
}

// Helper functions to extract information from service results
func extractReference(normalizedHGVS string) string {
	if normalizedHGVS == "" {
		return ""
	}
	parts := strings.Split(normalizedHGVS, ":")
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

func extractPosition(genomicPosition string) string {
	if genomicPosition == "" {
		return ""
	}
	// Extract position from format like "chr1:g.12345"
	parts := strings.Split(genomicPosition, ".")
	if len(parts) > 1 {
		return parts[1]
	}
	return genomicPosition
}

// parseHGVSComponents parses HGVS string into components
func (t *ValidateHGVSTool) parseHGVSComponents(hgvs string) (HGVSComponents, []ValidationIssue) {
	components := HGVSComponents{}
	issues := make([]ValidationIssue, 0)

	// Basic HGVS format: Reference:Type.Variant
	// e.g., NM_000492.3:c.1521_1523delCTT
	
	// Split by colon
	parts := strings.Split(hgvs, ":")
	if len(parts) != 2 {
		issues = append(issues, ValidationIssue{
			Severity: "error",
			Code:     "INVALID_FORMAT",
			Message:  "HGVS must contain exactly one colon separator",
			Position: strings.Index(hgvs, ":"),
		})
		return components, issues
	}

	// Parse reference part
	refPart := parts[0]
	varPart := parts[1]

	// Parse reference accession and version
	if err := t.parseReference(refPart, &components); err != nil {
		issues = append(issues, ValidationIssue{
			Severity: "error", 
			Code:     "INVALID_REFERENCE",
			Message:  err.Error(),
			Position: 0,
		})
	}

	// Parse variant part
	if err := t.parseVariant(varPart, &components); err != nil {
		issues = append(issues, ValidationIssue{
			Severity: "error",
			Code:     "INVALID_VARIANT",
			Message:  err.Error(),
			Position: len(refPart) + 1,
		})
	}

	return components, issues
}

// parseReference parses the reference accession part
func (t *ValidateHGVSTool) parseReference(refPart string, components *HGVSComponents) error {
	// Match RefSeq accession patterns (include NP_ for proteins)
	refPattern := regexp.MustCompile(`^(N[CMGRP]_|X[MR]_|N[TW]_)(\d+)\.(\d+)$`)
	matches := refPattern.FindStringSubmatch(refPart)
	
	if matches == nil {
		return fmt.Errorf("invalid RefSeq accession format: %s", refPart)
	}

	components.Reference = matches[1] + matches[2]
	components.Version = matches[3]

	return nil
}

// parseVariant parses the variant description part
func (t *ValidateHGVSTool) parseVariant(varPart string, components *HGVSComponents) error {
	if len(varPart) < 2 {
		return fmt.Errorf("variant part too short: %s", varPart)
	}

	// Extract sequence type (g, c, p, n, r, m)
	components.Type = string(varPart[0])
	if components.Type != "g" && components.Type != "c" && components.Type != "p" && 
	   components.Type != "n" && components.Type != "r" && components.Type != "m" {
		return fmt.Errorf("invalid sequence type: %s", components.Type)
	}

	// Check for period separator
	if varPart[1] != '.' {
		return fmt.Errorf("missing period after sequence type")
	}

	description := varPart[2:]
	components.Description = description

	// Parse variant type and position
	if err := t.parseVariantDescription(description, components); err != nil {
		return fmt.Errorf("invalid variant description: %w", err)
	}

	return nil
}

// parseVariantDescription parses the variant description
func (t *ValidateHGVSTool) parseVariantDescription(desc string, components *HGVSComponents) error {
	// Common HGVS variant patterns - support both DNA and protein descriptions
	patterns := map[string]*regexp.Regexp{
		"substitution":     regexp.MustCompile(`^(\d+)([ACGT])>([ACGT])$`),
		"deletion":         regexp.MustCompile(`^(\d+(?:_\d+)?)del([ACGT]*)$`),
		"insertion":        regexp.MustCompile(`^(\d+_\d+)ins([ACGT]+)$`),
		"duplication":      regexp.MustCompile(`^(\d+(?:_\d+)?)dup([ACGT]*)$`),
		"delins":           regexp.MustCompile(`^(\d+(?:_\d+)?)delins([ACGT]+)$`),
		"protein_deletion": regexp.MustCompile(`^([A-Za-z]{3}\d+)del$`),  // e.g., Phe508del
		"protein_subst":    regexp.MustCompile(`^([A-Za-z]{3}\d+)([A-Za-z]{3})$`),  // e.g., Phe508Cys
	}

	for varType, pattern := range patterns {
		if matches := pattern.FindStringSubmatch(desc); matches != nil {
			components.VariantType = varType
			components.Position = matches[1]
			
			if len(matches) > 1 {
				if varType == "substitution" && len(matches) > 3 {
					components.ReferenceSeq = matches[2]
					components.AlteredSeq = matches[3]
				} else if varType == "insertion" || varType == "delins" {
					if len(matches) > 2 {
						components.AlteredSeq = matches[2]
					}
				} else if varType == "deletion" || varType == "duplication" {
					if len(matches) > 2 {
						components.ReferenceSeq = matches[2]
					}
				} else if varType == "protein_deletion" {
					components.ReferenceSeq = matches[1]
				} else if varType == "protein_subst" && len(matches) > 2 {
					components.ReferenceSeq = matches[1]
					components.AlteredSeq = matches[2]
				}
			}
			
			return nil
		}
	}

	return fmt.Errorf("unrecognized variant description pattern: %s", desc)
}

// performStrictValidation performs additional strict validation checks
func (t *ValidateHGVSTool) performStrictValidation(hgvs string, components HGVSComponents) []ValidationIssue {
	issues := make([]ValidationIssue, 0)

	// Check for deprecated formats
	if strings.Contains(hgvs, "IVS") {
		issues = append(issues, ValidationIssue{
			Severity:   "warning",
			Code:       "DEPRECATED_FORMAT",
			Message:    "IVS notation is deprecated, use c. notation instead",
			Suggestion: "Convert to standard c. notation",
		})
	}

	// Check for common formatting issues
	if strings.Contains(hgvs, " ") {
		issues = append(issues, ValidationIssue{
			Severity:   "warning",
			Code:       "FORMATTING_ISSUE",
			Message:    "HGVS notation should not contain spaces",
			Suggestion: "Remove all spaces from HGVS notation",
		})
	}

	// Validate sequence type context
	if components.Type == "p" && !strings.HasPrefix(components.Reference, "NP_") {
		issues = append(issues, ValidationIssue{
			Severity: "warning",
			Code:     "SEQUENCE_TYPE_MISMATCH", 
			Message:  "Protein notation (p.) should use protein reference (NP_)",
		})
	}

	return issues
}

// hasNoErrors checks if validation issues contain any errors
func (t *ValidateHGVSTool) hasNoErrors(issues []ValidationIssue) bool {
	for _, issue := range issues {
		if issue.Severity == "error" {
			return false
		}
	}
	return true
}

// normalizeHGVS returns a normalized version of the HGVS notation
func (t *ValidateHGVSTool) normalizeHGVS(hgvs string, components HGVSComponents) string {
	// Simple normalization - remove extra spaces, ensure proper case
	normalized := strings.TrimSpace(hgvs)
	normalized = strings.ReplaceAll(normalized, " ", "")
	
	// In a real implementation, this would perform more sophisticated normalization
	return normalized
}