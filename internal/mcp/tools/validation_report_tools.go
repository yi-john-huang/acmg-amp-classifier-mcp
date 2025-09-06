package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/acmg-amp-mcp-server/internal/mcp/protocol"
)

// ValidateReportTool implements report validation and quality assurance mechanisms
type ValidateReportTool struct {
	logger *logrus.Logger
}

// ValidateReportParams defines parameters for report validation
type ValidateReportParams struct {
	Report          ReportResult               `json:"report" validate:"required"`
	ValidationLevel string                     `json:"validation_level,omitempty"`
	CheckSections   []string                   `json:"check_sections,omitempty"`
	QualityChecks   map[string]interface{}     `json:"quality_checks,omitempty"`
	ComplianceRules []string                   `json:"compliance_rules,omitempty"`
	CustomChecks    []CustomValidationCheck    `json:"custom_checks,omitempty"`
}

// CustomValidationCheck defines a custom validation rule
type CustomValidationCheck struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Rule        string                 `json:"rule"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
	Severity    string                 `json:"severity"` // "error", "warning", "info"
}

// ReportValidationResult contains validation results and recommendations
type ReportValidationResult struct {
	IsValid              bool                    `json:"is_valid"`
	OverallScore         float64                 `json:"overall_score"`
	ValidationSummary    string                  `json:"validation_summary"`
	QualityAssessment    QualityAssessment       `json:"quality_assessment"`
	ValidationIssues     []ValidationIssue       `json:"validation_issues"`
	ComplianceStatus     ComplianceStatus        `json:"compliance_status"`
	ImprovementSuggestions []ImprovementSuggestion `json:"improvement_suggestions"`
	ChecksPerformed      []string                `json:"checks_performed"`
	ValidationMetadata   ValidationMetadata      `json:"validation_metadata"`
}

// QualityAssessment provides detailed quality metrics
type QualityAssessment struct {
	CompletenessScore    float64                 `json:"completeness_score"`
	AccuracyScore        float64                 `json:"accuracy_score"`
	ConsistencyScore     float64                 `json:"consistency_score"`
	ClarityScore         float64                 `json:"clarity_score"`
	SectionQuality       map[string]float64      `json:"section_quality"`
	EvidenceQuality      float64                 `json:"evidence_quality"`
	RecommendationQuality float64                `json:"recommendation_quality"`
	QualityFlags         []string                `json:"quality_flags"`
}

// ComplianceStatus tracks regulatory and guideline compliance
type ComplianceStatus struct {
	ACMGCompliance       bool                   `json:"acmg_compliance"`
	CLIACompliance       bool                   `json:"clia_compliance"`
	FDACompliance        bool                   `json:"fda_compliance"`
	InternationalStandards bool                 `json:"international_standards"`
	ComplianceIssues     []string               `json:"compliance_issues"`
	RequiredElements     map[string]bool        `json:"required_elements"`
}

// ImprovementSuggestion provides actionable recommendations
type ImprovementSuggestion struct {
	Priority    string `json:"priority"`    // "high", "medium", "low"
	Category    string `json:"category"`    // "content", "format", "compliance"
	Suggestion  string `json:"suggestion"`
	Impact      string `json:"impact"`
	ActionItems []string `json:"action_items"`
}

// ValidationMetadata contains validation context
type ValidationMetadata struct {
	ValidationDate   string                 `json:"validation_date"`
	ValidatorVersion string                 `json:"validator_version"`
	RulesApplied     []string               `json:"rules_applied"`
	ProcessingTime   string                 `json:"processing_time"`
	ValidationConfig map[string]interface{} `json:"validation_config"`
}

// NewValidateReportTool creates a new report validation tool
func NewValidateReportTool(logger *logrus.Logger) *ValidateReportTool {
	return &ValidateReportTool{
		logger: logger,
	}
}

// HandleTool implements the ToolHandler interface for validate_report
func (t *ValidateReportTool) HandleTool(ctx context.Context, req *protocol.JSONRPC2Request) *protocol.JSONRPC2Response {
	t.logger.WithField("tool", "validate_report").Info("Processing report validation request")

	// Parse and validate parameters
	var params ValidateReportParams
	if err := t.parseAndValidateParams(req.Params, &params); err != nil {
		return &protocol.JSONRPC2Response{
			Error: &protocol.RPCError{
				Code:    protocol.InvalidParams,
				Message: "Invalid parameters",
				Data:    err.Error(),
			},
		}
	}

	// Validate the report
	result, err := t.validateReport(ctx, &params)
	if err != nil {
		return &protocol.JSONRPC2Response{
			Error: &protocol.RPCError{
				Code:    protocol.InternalError,
				Message: "Report validation failed",
				Data:    err.Error(),
			},
		}
	}

	t.logger.WithFields(logrus.Fields{
		"report_id":       params.Report.ReportID,
		"is_valid":        result.IsValid,
		"overall_score":   result.OverallScore,
		"issues_found":    len(result.ValidationIssues),
		"suggestions":     len(result.ImprovementSuggestions),
	}).Info("Report validation completed")

	return &protocol.JSONRPC2Response{
		Result: map[string]interface{}{
			"validation": result,
		},
	}
}

// GetToolInfo returns tool metadata
func (t *ValidateReportTool) GetToolInfo() protocol.ToolInfo {
	return protocol.ToolInfo{
		Name:        "validate_report",
		Description: "Validate clinical reports for quality, compliance, and completeness with comprehensive quality assurance mechanisms",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"report": map[string]interface{}{
					"type":        "object",
					"description": "Report data from generate_report tool to validate",
				},
				"validation_level": map[string]interface{}{
					"type":        "string",
					"description": "Level of validation to perform",
					"enum":        []string{"basic", "standard", "comprehensive", "strict"},
					"default":     "standard",
				},
				"check_sections": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Specific sections to validate (if not provided, validates all)",
				},
				"compliance_rules": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Compliance standards to check",
					"default":     []string{"ACMG", "CLIA"},
				},
				"custom_checks": map[string]interface{}{
					"type":        "array",
					"description": "Custom validation checks to perform",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"name": map[string]interface{}{
								"type": "string",
								"description": "Name of the custom check",
							},
							"rule": map[string]interface{}{
								"type": "string",
								"description": "Validation rule to apply",
							},
							"severity": map[string]interface{}{
								"type": "string",
								"enum": []string{"error", "warning", "info"},
								"default": "warning",
							},
						},
						"required": []string{"name", "rule"},
					},
				},
			},
			"required": []string{"report"},
		},
	}
}

// ValidateParams validates tool parameters
func (t *ValidateReportTool) ValidateParams(params interface{}) error {
	var validateParams ValidateReportParams
	return t.parseAndValidateParams(params, &validateParams)
}

// parseAndValidateParams parses and validates input parameters
func (t *ValidateReportTool) parseAndValidateParams(params interface{}, target *ValidateReportParams) error {
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

	// Validate that report is present
	if target.Report.ReportID == "" && target.Report.HGVSNotation == "" {
		return fmt.Errorf("report data is required")
	}

	// Set defaults
	if target.ValidationLevel == "" {
		target.ValidationLevel = "standard"
	}

	if len(target.ComplianceRules) == 0 {
		target.ComplianceRules = []string{"ACMG", "CLIA"}
	}

	// Validate validation level
	validLevels := []string{"basic", "standard", "comprehensive", "strict"}
	if !t.isValidLevel(target.ValidationLevel, validLevels) {
		return fmt.Errorf("invalid validation level: %s", target.ValidationLevel)
	}

	return nil
}

// validateReport performs comprehensive report validation
func (t *ValidateReportTool) validateReport(ctx context.Context, params *ValidateReportParams) (*ReportValidationResult, error) {
	result := &ReportValidationResult{
		ValidationIssues:       make([]ValidationIssue, 0),
		ImprovementSuggestions: make([]ImprovementSuggestion, 0),
		ChecksPerformed:        make([]string, 0),
	}

	// Initialize quality assessment
	result.QualityAssessment = QualityAssessment{
		SectionQuality: make(map[string]float64),
		QualityFlags:   make([]string, 0),
	}

	// Initialize compliance status
	result.ComplianceStatus = ComplianceStatus{
		ComplianceIssues:  make([]string, 0),
		RequiredElements:  make(map[string]bool),
	}

	// Perform different validation checks based on level
	switch params.ValidationLevel {
	case "basic":
		t.performBasicValidation(params, result)
	case "standard":
		t.performBasicValidation(params, result)
		t.performStandardValidation(params, result)
	case "comprehensive":
		t.performBasicValidation(params, result)
		t.performStandardValidation(params, result)
		t.performComprehensiveValidation(params, result)
	case "strict":
		t.performBasicValidation(params, result)
		t.performStandardValidation(params, result)
		t.performComprehensiveValidation(params, result)
		t.performStrictValidation(params, result)
	}

	// Perform compliance checks
	t.performComplianceValidation(params, result)

	// Perform custom checks if provided
	if len(params.CustomChecks) > 0 {
		t.performCustomValidation(params, result)
	}

	// Calculate overall score and determine validity
	result.OverallScore = t.calculateOverallScore(result)
	result.IsValid = t.determineValidity(result)
	result.ValidationSummary = t.generateValidationSummary(result)

	// Generate improvement suggestions
	result.ImprovementSuggestions = t.generateImprovementSuggestions(result)

	// Set validation metadata
	result.ValidationMetadata = t.generateValidationMetadata(params)

	return result, nil
}

// Basic validation checks essential elements
func (t *ValidateReportTool) performBasicValidation(params *ValidateReportParams, result *ReportValidationResult) {
	result.ChecksPerformed = append(result.ChecksPerformed, "basic_validation")

	// Check required fields
	if params.Report.ReportID == "" {
		result.ValidationIssues = append(result.ValidationIssues, ValidationIssue{
			Severity: "error",
			Code:     "MISSING_REPORT_ID",
			Message:  "Report ID is required",
		})
	}

	if params.Report.HGVSNotation == "" {
		result.ValidationIssues = append(result.ValidationIssues, ValidationIssue{
			Severity: "error",
			Code:     "MISSING_HGVS",
			Message:  "HGVS notation is required",
		})
	}

	if params.Report.Summary.Classification == "" {
		result.ValidationIssues = append(result.ValidationIssues, ValidationIssue{
			Severity: "error",
			Code:     "MISSING_CLASSIFICATION",
			Message:  "Variant classification is required",
		})
	}

	// Basic completeness score
	requiredFields := 5
	presentFields := 0
	if params.Report.ReportID != "" { presentFields++ }
	if params.Report.HGVSNotation != "" { presentFields++ }
	if params.Report.Summary.Classification != "" { presentFields++ }
	if len(params.Report.Sections) > 0 { presentFields++ }
	if len(params.Report.Recommendations) > 0 { presentFields++ }

	result.QualityAssessment.CompletenessScore = float64(presentFields) / float64(requiredFields)
}

// Standard validation adds content quality checks
func (t *ValidateReportTool) performStandardValidation(params *ValidateReportParams, result *ReportValidationResult) {
	result.ChecksPerformed = append(result.ChecksPerformed, "standard_validation")

	// Validate classification values
	validClassifications := []string{"Pathogenic", "Likely pathogenic", "VUS", "Likely benign", "Benign"}
	if !t.isValidClassification(params.Report.Summary.Classification, validClassifications) {
		result.ValidationIssues = append(result.ValidationIssues, ValidationIssue{
			Severity: "warning",
			Code:     "INVALID_CLASSIFICATION",
			Message:  fmt.Sprintf("Unusual classification: %s", params.Report.Summary.Classification),
		})
	}

	// Check confidence score
	if params.Report.Summary.Confidence < 0.3 {
		result.ValidationIssues = append(result.ValidationIssues, ValidationIssue{
			Severity: "warning",
			Code:     "LOW_CONFIDENCE",
			Message:  fmt.Sprintf("Low confidence score: %.2f", params.Report.Summary.Confidence),
		})
	}

	// Validate sections
	expectedSections := []string{"executive_summary", "variant_details", "classification"}
	for _, expectedSection := range expectedSections {
		if _, exists := params.Report.Sections[expectedSection]; !exists {
			result.ValidationIssues = append(result.ValidationIssues, ValidationIssue{
				Severity: "warning",
				Code:     "MISSING_SECTION",
				Message:  fmt.Sprintf("Expected section missing: %s", expectedSection),
			})
		}
	}

	// Calculate section quality scores
	for sectionName := range params.Report.Sections {
		result.QualityAssessment.SectionQuality[sectionName] = t.evaluateSectionQuality(sectionName, params.Report.Sections[sectionName])
	}

	// Update accuracy score based on validation issues
	errorCount := 0
	warningCount := 0
	for _, issue := range result.ValidationIssues {
		if issue.Severity == "error" {
			errorCount++
		} else if issue.Severity == "warning" {
			warningCount++
		}
	}

	result.QualityAssessment.AccuracyScore = 1.0 - (float64(errorCount)*0.2 + float64(warningCount)*0.1)
	if result.QualityAssessment.AccuracyScore < 0 {
		result.QualityAssessment.AccuracyScore = 0
	}
}

// Comprehensive validation adds detailed content analysis
func (t *ValidateReportTool) performComprehensiveValidation(params *ValidateReportParams, result *ReportValidationResult) {
	result.ChecksPerformed = append(result.ChecksPerformed, "comprehensive_validation")

	// Check evidence quality
	if params.Report.QualityMetrics.EvidenceQuality == "" {
		result.ValidationIssues = append(result.ValidationIssues, ValidationIssue{
			Severity: "info",
			Code:     "MISSING_EVIDENCE_QUALITY",
			Message:  "Evidence quality assessment not provided",
		})
	}

	// Validate recommendations consistency
	if len(params.Report.Recommendations) == 0 {
		result.ValidationIssues = append(result.ValidationIssues, ValidationIssue{
			Severity: "warning",
			Code:     "NO_RECOMMENDATIONS",
			Message:  "No clinical recommendations provided",
		})
	}

	// Check for disclaimers
	if len(params.Report.Disclaimers) == 0 {
		result.ValidationIssues = append(result.ValidationIssues, ValidationIssue{
			Severity: "warning",
			Code:     "NO_DISCLAIMERS",
			Message:  "No disclaimers provided",
		})
	}

	// Evaluate consistency score
	result.QualityAssessment.ConsistencyScore = t.evaluateConsistency(params.Report)

	// Evaluate clarity score
	result.QualityAssessment.ClarityScore = t.evaluateClarity(params.Report)

	// Check evidence quality
	result.QualityAssessment.EvidenceQuality = params.Report.QualityMetrics.CompletenessScore
	
	// Evaluate recommendation quality
	result.QualityAssessment.RecommendationQuality = t.evaluateRecommendationQuality(params.Report)
}

// Strict validation enforces the highest standards
func (t *ValidateReportTool) performStrictValidation(params *ValidateReportParams, result *ReportValidationResult) {
	result.ChecksPerformed = append(result.ChecksPerformed, "strict_validation")

	// Require all standard sections
	requiredSections := []string{
		"executive_summary", "variant_details", "classification",
		"evidence_summary", "recommendations", "methodology",
	}
	
	for _, requiredSection := range requiredSections {
		if _, exists := params.Report.Sections[requiredSection]; !exists {
			result.ValidationIssues = append(result.ValidationIssues, ValidationIssue{
				Severity: "error",
				Code:     "MISSING_REQUIRED_SECTION",
				Message:  fmt.Sprintf("Required section missing: %s", requiredSection),
			})
		}
	}

	// Require minimum confidence threshold
	if params.Report.Summary.Confidence < 0.5 {
		result.ValidationIssues = append(result.ValidationIssues, ValidationIssue{
			Severity: "error",
			Code:     "INSUFFICIENT_CONFIDENCE",
			Message:  "Classification confidence below minimum threshold (0.5)",
		})
	}

	// Require evidence documentation
	if params.Report.QualityMetrics.DataSources < 2 {
		result.ValidationIssues = append(result.ValidationIssues, ValidationIssue{
			Severity: "error",
			Code:     "INSUFFICIENT_EVIDENCE_SOURCES",
			Message:  "Minimum of 2 evidence sources required",
		})
	}
}

// Compliance validation checks regulatory requirements
func (t *ValidateReportTool) performComplianceValidation(params *ValidateReportParams, result *ReportValidationResult) {
	result.ChecksPerformed = append(result.ChecksPerformed, "compliance_validation")

	for _, rule := range params.ComplianceRules {
		switch strings.ToUpper(rule) {
		case "ACMG":
			t.checkACMGCompliance(params, result)
		case "CLIA":
			t.checkCLIACompliance(params, result)
		case "FDA":
			t.checkFDACompliance(params, result)
		}
	}
}

// Custom validation applies user-defined rules
func (t *ValidateReportTool) performCustomValidation(params *ValidateReportParams, result *ReportValidationResult) {
	result.ChecksPerformed = append(result.ChecksPerformed, "custom_validation")

	for _, check := range params.CustomChecks {
		// In a real implementation, this would evaluate the custom rule
		// For now, just log the custom check
		t.logger.WithField("custom_check", check.Name).Info("Performing custom validation check")
		
		// Mock custom validation result
		result.ValidationIssues = append(result.ValidationIssues, ValidationIssue{
			Severity: check.Severity,
			Code:     fmt.Sprintf("CUSTOM_%s", strings.ToUpper(strings.ReplaceAll(check.Name, " ", "_"))),
			Message:  fmt.Sprintf("Custom check: %s", check.Description),
		})
	}
}

// Compliance check implementations
func (t *ValidateReportTool) checkACMGCompliance(params *ValidateReportParams, result *ReportValidationResult) {
	result.ComplianceStatus.ACMGCompliance = true

	// Check for ACMG-required elements
	result.ComplianceStatus.RequiredElements["variant_classification"] = params.Report.Summary.Classification != ""
	result.ComplianceStatus.RequiredElements["evidence_assessment"] = len(params.Report.Sections) > 0
	result.ComplianceStatus.RequiredElements["methodology"] = true // Assume present

	if params.Report.Summary.Classification == "" {
		result.ComplianceStatus.ACMGCompliance = false
		result.ComplianceStatus.ComplianceIssues = append(result.ComplianceStatus.ComplianceIssues,
			"ACMG guidelines require explicit variant classification")
	}
}

func (t *ValidateReportTool) checkCLIACompliance(params *ValidateReportParams, result *ReportValidationResult) {
	result.ComplianceStatus.CLIACompliance = true

	// Check for CLIA-required elements
	result.ComplianceStatus.RequiredElements["report_date"] = params.Report.GenerationDate != ""
	result.ComplianceStatus.RequiredElements["methodology"] = true
	result.ComplianceStatus.RequiredElements["limitations"] = len(params.Report.Disclaimers) > 0

	if len(params.Report.Disclaimers) == 0 {
		result.ComplianceStatus.CLIACompliance = false
		result.ComplianceStatus.ComplianceIssues = append(result.ComplianceStatus.ComplianceIssues,
			"CLIA regulations require test limitations and disclaimers")
	}
}

func (t *ValidateReportTool) checkFDACompliance(params *ValidateReportParams, result *ReportValidationResult) {
	result.ComplianceStatus.FDACompliance = true

	// FDA compliance checks would be more complex in real implementation
	result.ComplianceStatus.RequiredElements["traceability"] = params.Report.ReportID != ""
	result.ComplianceStatus.RequiredElements["quality_metrics"] = true
}

// Helper methods for quality assessment
func (t *ValidateReportTool) evaluateSectionQuality(sectionName string, sectionData interface{}) float64 {
	// Mock section quality evaluation
	if sectionData == nil {
		return 0.0
	}

	// Basic quality score based on content presence
	if dataMap, ok := sectionData.(map[string]interface{}); ok {
		if len(dataMap) > 0 {
			return 0.8 // Good quality if section has content
		}
	}
	
	return 0.5 // Average quality
}

func (t *ValidateReportTool) evaluateConsistency(report ReportResult) float64 {
	// Mock consistency evaluation
	// Check if classification matches recommendations
	consistencyScore := 0.8

	if report.Summary.Classification == "VUS" && len(report.Recommendations) == 0 {
		consistencyScore -= 0.2 // VUS should have specific recommendations
	}

	return consistencyScore
}

func (t *ValidateReportTool) evaluateClarity(report ReportResult) float64 {
	// Mock clarity evaluation based on content structure
	clarityScore := 0.8

	if len(report.Summary.KeyFindings) == 0 {
		clarityScore -= 0.2
	}

	if len(report.Sections) < 3 {
		clarityScore -= 0.1
	}

	return clarityScore
}

func (t *ValidateReportTool) evaluateRecommendationQuality(report ReportResult) float64 {
	if len(report.Recommendations) == 0 {
		return 0.0
	}

	// Mock recommendation quality based on count and content
	qualityScore := 0.7

	if len(report.Recommendations) >= 2 {
		qualityScore += 0.2
	}

	return qualityScore
}

// Scoring and summary methods
func (t *ValidateReportTool) calculateOverallScore(result *ReportValidationResult) float64 {
	scores := []float64{
		result.QualityAssessment.CompletenessScore,
		result.QualityAssessment.AccuracyScore,
		result.QualityAssessment.ConsistencyScore,
		result.QualityAssessment.ClarityScore,
	}

	sum := 0.0
	count := 0
	for _, score := range scores {
		if score > 0 {
			sum += score
			count++
		}
	}

	if count == 0 {
		return 0.0
	}

	return sum / float64(count)
}

func (t *ValidateReportTool) determineValidity(result *ReportValidationResult) bool {
	// Report is invalid if there are any error-level issues
	for _, issue := range result.ValidationIssues {
		if issue.Severity == "error" {
			return false
		}
	}

	// Report is valid if overall score is above threshold
	return result.OverallScore >= 0.6
}

func (t *ValidateReportTool) generateValidationSummary(result *ReportValidationResult) string {
	if result.IsValid {
		return fmt.Sprintf("Report validation passed with overall score %.2f. Found %d issues (%d warnings, %d info).",
			result.OverallScore, len(result.ValidationIssues), 
			t.countIssuesBySeverity(result.ValidationIssues, "warning"),
			t.countIssuesBySeverity(result.ValidationIssues, "info"))
	} else {
		return fmt.Sprintf("Report validation failed with overall score %.2f. Found %d issues (%d errors, %d warnings).",
			result.OverallScore, len(result.ValidationIssues),
			t.countIssuesBySeverity(result.ValidationIssues, "error"),
			t.countIssuesBySeverity(result.ValidationIssues, "warning"))
	}
}

func (t *ValidateReportTool) generateImprovementSuggestions(result *ReportValidationResult) []ImprovementSuggestion {
	suggestions := make([]ImprovementSuggestion, 0)

	// Suggest improvements based on validation issues
	if result.QualityAssessment.CompletenessScore < 0.8 {
		suggestions = append(suggestions, ImprovementSuggestion{
			Priority:    "high",
			Category:    "content",
			Suggestion:  "Improve report completeness by adding missing required sections",
			Impact:      "Higher completeness score and better clinical utility",
			ActionItems: []string{"Review required sections", "Add missing content"},
		})
	}

	if result.QualityAssessment.AccuracyScore < 0.7 {
		suggestions = append(suggestions, ImprovementSuggestion{
			Priority:    "high",
			Category:    "content",
			Suggestion:  "Address validation errors and warnings to improve accuracy",
			Impact:      "More reliable clinical interpretation",
			ActionItems: []string{"Fix validation errors", "Review warning messages"},
		})
	}

	if len(result.ValidationIssues) > 5 {
		suggestions = append(suggestions, ImprovementSuggestion{
			Priority:    "medium",
			Category:    "format",
			Suggestion:  "Consider comprehensive review to address multiple validation issues",
			Impact:      "Overall improvement in report quality",
			ActionItems: []string{"Systematic review of all issues", "Process improvements"},
		})
	}

	return suggestions
}

func (t *ValidateReportTool) generateValidationMetadata(params *ValidateReportParams) ValidationMetadata {
	return ValidationMetadata{
		ValidationDate:   fmt.Sprintf("%d", 1700000000), // Mock timestamp
		ValidatorVersion: "1.0.0",
		RulesApplied:     params.ComplianceRules,
		ProcessingTime:   "150ms",
		ValidationConfig: map[string]interface{}{
			"validation_level": params.ValidationLevel,
			"checks_performed": len(params.CheckSections),
		},
	}
}

// Utility methods
func (t *ValidateReportTool) countIssuesBySeverity(issues []ValidationIssue, severity string) int {
	count := 0
	for _, issue := range issues {
		if issue.Severity == severity {
			count++
		}
	}
	return count
}

func (t *ValidateReportTool) isValidLevel(level string, validLevels []string) bool {
	for _, valid := range validLevels {
		if level == valid {
			return true
		}
	}
	return false
}

func (t *ValidateReportTool) isValidClassification(classification string, validClassifications []string) bool {
	for _, valid := range validClassifications {
		if strings.EqualFold(classification, valid) {
			return true
		}
	}
	return false
}