package tools

import (
	"context"
	"encoding/json"
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
type ValidateHGVSResult struct {
	IsValid         bool              `json:"is_valid"`
	HGVSNotation    string            `json:"hgvs_notation"`
	NormalizedHGVS  string            `json:"normalized_hgvs,omitempty"`
	ValidationIssues []ValidationIssue `json:"validation_issues,omitempty"`
	ParsedComponents HGVSComponents    `json:"parsed_components,omitempty"`
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

	return nil
}

// validateHGVS performs comprehensive HGVS validation using the classifier service
func (t *ValidateHGVSTool) validateHGVS(params *ValidateHGVSParams) *ValidateHGVSResult {
	hgvs := strings.TrimSpace(params.HGVSNotation)

	// Check if classifier service is available
	if t.classifierService == nil {
		return &ValidateHGVSResult{
			IsValid:      false,
			HGVSNotation: hgvs,
			ValidationIssues: []ValidationIssue{{
				Severity: "error",
				Code:     "SERVICE_NOT_CONFIGURED",
				Message:  "Validation service not configured",
				Position: 0,
			}},
		}
	}

	// Call the real validation service
	serviceResult, err := t.classifierService.ValidateHGVS(hgvs)
	if err != nil {
		// If service validation fails, fall back to basic validation
		return &ValidateHGVSResult{
			IsValid:      false,
			HGVSNotation: hgvs,
			ValidationIssues: []ValidationIssue{{
				Severity: "error",
				Code:     "VALIDATION_SERVICE_ERROR",
				Message:  fmt.Sprintf("Validation service error: %v", err),
				Position: 0,
			}},
		}
	}

	// Convert service result to MCP tool result
	result := &ValidateHGVSResult{
		IsValid:         serviceResult.IsValid,
		HGVSNotation:    hgvs,
		NormalizedHGVS:  serviceResult.NormalizedHGVS,
		ValidationIssues: make([]ValidationIssue, 0),
		ParsedComponents: HGVSComponents{
			Reference:    extractReference(serviceResult.NormalizedHGVS),
			Type:         serviceResult.VariantType,
			Position:     extractPosition(serviceResult.GenomicPosition),
			VariantType:  serviceResult.VariantType,
			Description:  serviceResult.PredictedProtein,
		},
	}

	// Add validation issue if not valid
	if !serviceResult.IsValid {
		result.ValidationIssues = append(result.ValidationIssues, ValidationIssue{
			Severity: "error",
			Code:     "HGVS_INVALID",
			Message:  serviceResult.ErrorMessage,
			Position: 0,
		})
	}

	return result
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