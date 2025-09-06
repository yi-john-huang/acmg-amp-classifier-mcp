package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/acmg-amp-mcp-server/internal/mcp/protocol"
)

// TestGenerateReportTool tests the generate_report tool functionality
func TestGenerateReportTool(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	tool := NewGenerateReportTool(logger)

	tests := []struct {
		name          string
		params        GenerateReportParams
		expectError   bool
		expectedSections []string
	}{
		{
			name: "Basic clinical report",
			params: GenerateReportParams{
				HGVSNotation: "NM_000492.3:c.1521_1523delCTT",
				GeneSymbol:   "CFTR",
				Classification: ClassifyVariantResult{
					Classification: "Pathogenic",
					Confidence:    "high",
					AppliedRules:   []ACMGAMPRuleResult{
						{RuleCode: "PVS1", RuleName: "Null variant", Applied: true},
					},
				},
				ReportTemplate: "clinical",
			},
			expectError: false,
			expectedSections: []string{"executive_summary", "variant_details", "classification"},
		},
		{
			name: "Research report with evidence",
			params: GenerateReportParams{
				HGVSNotation: "NM_000492.3:c.1521_1523delCTT",
				GeneSymbol:   "CFTR", 
				Classification: ClassifyVariantResult{
					Classification: "Pathogenic",
					Confidence:    "high",
					AppliedRules:   []ACMGAMPRuleResult{},
				},
				Evidence: &QueryEvidenceResult{
					HGVSNotation: "NM_000492.3:c.1521_1523delCTT",
					QualityScores: EvidenceQualityScores{
						OverallQuality:   "high",
						DataCompleteness: 0.9,
					},
					DatabaseResults: map[string]interface{}{
						"clinvar": map[string]interface{}{"significance": "pathogenic"},
					},
				},
				ReportTemplate: "research",
			},
			expectError: false,
			expectedSections: []string{"variant_details", "classification"},
		},
		{
			name: "Report with clinical context",
			params: GenerateReportParams{
				HGVSNotation: "NM_000492.3:c.1521_1523delCTT",
				GeneSymbol:   "CFTR",
				Classification: ClassifyVariantResult{
					Classification: "VUS",
					Confidence:    "moderate",
					AppliedRules:   []ACMGAMPRuleResult{},
				},
				ClinicalContext: &ClinicalContext{
					PatientID:          "P123456",
					ClinicalIndication: "Cystic fibrosis screening",
					FamilyHistory:      "No known family history",
					Ethnicity:          "European",
				},
				ReportTemplate: "clinical",
			},
			expectError: false,
			expectedSections: []string{"executive_summary", "variant_details"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &protocol.JSONRPC2Request{
				Method: "generate_report",
				Params: tt.params,
			}

			ctx := context.Background()
			response := tool.HandleTool(ctx, req)

			if tt.expectError {
				assert.NotNil(t, response.Error, "Expected error response")
				return
			}

			require.Nil(t, response.Error, "Expected no error: %v", response.Error)
			require.NotNil(t, response.Result, "Expected result")

			// Validate result structure
			resultMap, ok := response.Result.(map[string]interface{})
			require.True(t, ok, "Result should be a map")

			reportInterface, exists := resultMap["report"]
			require.True(t, exists, "Result should contain report")
			require.NotNil(t, reportInterface, "Report should not be nil")

			// Convert to ReportResult for validation
			reportBytes, err := json.Marshal(reportInterface)
			require.NoError(t, err)

			var report ReportResult
			err = json.Unmarshal(reportBytes, &report)
			require.NoError(t, err)

			// Validate basic report structure
			assert.NotEmpty(t, report.ReportID, "Report should have an ID")
			assert.Equal(t, tt.params.HGVSNotation, report.HGVSNotation, "HGVS notation should match")
			assert.Equal(t, tt.params.GeneSymbol, report.GeneSymbol, "Gene symbol should match")
			assert.NotEmpty(t, report.GenerationDate, "Generation date should be set")
			assert.Equal(t, tt.params.ReportTemplate, report.Template, "Template should match")

			// Check expected sections are present
			for _, expectedSection := range tt.expectedSections {
				_, exists := report.Sections[expectedSection]
				assert.True(t, exists, "Expected section %s should be present", expectedSection)
			}

			// Validate summary
			assert.Equal(t, tt.params.Classification.Classification, report.Summary.Classification)
			assert.NotEmpty(t, report.Summary.KeyFindings, "Should have key findings")

			// Validate quality metrics
			assert.GreaterOrEqual(t, report.QualityMetrics.CompletenessScore, 0.0)
			assert.LessOrEqual(t, report.QualityMetrics.CompletenessScore, 1.0)

			// Validate recommendations
			assert.NotEmpty(t, report.Recommendations, "Should have recommendations")
			assert.NotEmpty(t, report.Disclaimers, "Should have disclaimers")
		})
	}
}

// TestFormatReportTool tests the format_report tool functionality
func TestFormatReportTool(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	tool := NewFormatReportTool(logger)

	// Create a sample report for testing
	sampleReport := ReportResult{
		ReportID:       "RPT_TEST_123",
		HGVSNotation:   "NM_000492.3:c.1521_1523delCTT",
		GeneSymbol:     "CFTR",
		GenerationDate: "2023-01-01T00:00:00Z",
		Template:       "clinical",
		Sections: map[string]interface{}{
			"executive_summary": map[string]interface{}{
				"variant": "NM_000492.3:c.1521_1523delCTT",
				"classification": "Pathogenic",
			},
			"variant_details": map[string]interface{}{
				"gene": "CFTR",
				"type": "deletion",
			},
		},
		Summary: ReportSummary{
			Classification: "Pathogenic",
			Confidence:     0.9,
			ClinicalSig:    "Pathogenic",
			KeyFindings:    []string{"Variant classified as Pathogenic"},
		},
		QualityMetrics: ReportQualityMetrics{
			CompletenessScore: 0.9,
			EvidenceQuality:   "high",
			DataSources:       3,
		},
		Recommendations: []string{"Genetic counseling recommended"},
		Disclaimers:     []string{"This report is for clinical use"},
	}

	tests := []struct {
		name         string
		outputFormat string
		expectError  bool
		validateFunc func(t *testing.T, content string)
	}{
		{
			name:         "JSON format",
			outputFormat: "json",
			expectError:  false,
			validateFunc: func(t *testing.T, content string) {
				var jsonData interface{}
				err := json.Unmarshal([]byte(content), &jsonData)
				assert.NoError(t, err, "Should be valid JSON")
			},
		},
		{
			name:         "Text format",
			outputFormat: "text",
			expectError:  false,
			validateFunc: func(t *testing.T, content string) {
				assert.Contains(t, content, "EXECUTIVE SUMMARY", "Should contain executive summary header")
				assert.Contains(t, content, "CFTR", "Should contain gene symbol")
				assert.Contains(t, content, "Pathogenic", "Should contain classification")
				assert.Contains(t, content, "RECOMMENDATIONS", "Should contain recommendations section")
			},
		},
		{
			name:         "HTML format",
			outputFormat: "html",
			expectError:  false,
			validateFunc: func(t *testing.T, content string) {
				assert.Contains(t, content, "<!DOCTYPE html>", "Should be valid HTML")
				assert.Contains(t, content, "<title>", "Should have title")
				assert.Contains(t, content, "executive-summary", "Should contain executive summary div")
				assert.Contains(t, content, "</html>", "Should close HTML tag")
			},
		},
		{
			name:         "Markdown format",
			outputFormat: "markdown",
			expectError:  false,
			validateFunc: func(t *testing.T, content string) {
				assert.Contains(t, content, "# Executive Summary", "Should have markdown headers")
				assert.Contains(t, content, "**Variant:**", "Should have bold formatting")
				assert.Contains(t, content, "## Recommendations", "Should have section headers")
			},
		},
		{
			name:         "XML format",
			outputFormat: "xml",
			expectError:  false,
			validateFunc: func(t *testing.T, content string) {
				assert.Contains(t, content, "<?xml version=\"1.0\"", "Should have XML declaration")
				assert.Contains(t, content, "<clinical_report>", "Should have root element")
				assert.Contains(t, content, "</clinical_report>", "Should close root element")
			},
		},
		{
			name:         "PDF format (HTML conversion)",
			outputFormat: "pdf",
			expectError:  false,
			validateFunc: func(t *testing.T, content string) {
				assert.Contains(t, content, "<!-- PDF Content", "Should contain PDF comment")
				assert.Contains(t, content, "<!DOCTYPE html>", "Should contain HTML for conversion")
			},
		},
		{
			name:         "Invalid format",
			outputFormat: "invalid",
			expectError:  true,
			validateFunc: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := FormatReportParams{
				Report:        sampleReport,
				OutputFormat:  tt.outputFormat,
				IncludeHeader: true,
				IncludeFooter: true,
			}

			req := &protocol.JSONRPC2Request{
				Method: "format_report",
				Params: params,
			}

			ctx := context.Background()
			response := tool.HandleTool(ctx, req)

			if tt.expectError {
				assert.NotNil(t, response.Error, "Expected error response")
				return
			}

			require.Nil(t, response.Error, "Expected no error: %v", response.Error)
			require.NotNil(t, response.Result, "Expected result")

			// Validate result structure
			resultMap, ok := response.Result.(map[string]interface{})
			require.True(t, ok, "Result should be a map")

			formattedReportInterface, exists := resultMap["formatted_report"]
			require.True(t, exists, "Result should contain formatted_report")

			// Convert to FormatReportResult for validation
			formattedBytes, err := json.Marshal(formattedReportInterface)
			require.NoError(t, err)

			var formattedResult FormatReportResult
			err = json.Unmarshal(formattedBytes, &formattedResult)
			require.NoError(t, err)

			// Validate basic structure
			assert.Equal(t, tt.outputFormat, formattedResult.Format, "Format should match")
			assert.NotEmpty(t, formattedResult.FormattedContent, "Should have formatted content")
			assert.Greater(t, formattedResult.Size, 0, "Size should be positive")
			assert.Equal(t, "UTF-8", formattedResult.Encoding, "Should use UTF-8 encoding")

			// Validate metadata
			assert.NotNil(t, formattedResult.Metadata, "Should have metadata")
			assert.Equal(t, sampleReport.ReportID, formattedResult.Metadata["report_id"], "Should preserve report ID")

			// Run format-specific validation
			if tt.validateFunc != nil {
				tt.validateFunc(t, formattedResult.FormattedContent)
			}
		})
	}
}

// TestValidateReportTool tests the validate_report tool functionality
func TestValidateReportTool(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	tool := NewValidateReportTool(logger)

	// Create test reports with different quality levels
	goodReport := ReportResult{
		ReportID:       "RPT_GOOD_123",
		HGVSNotation:   "NM_000492.3:c.1521_1523delCTT",
		GeneSymbol:     "CFTR",
		GenerationDate: "2023-01-01T00:00:00Z",
		Sections: map[string]interface{}{
			"executive_summary":  map[string]interface{}{"summary": "Complete"},
			"variant_details":    map[string]interface{}{"details": "Present"},
			"classification":     map[string]interface{}{"class": "Pathogenic"},
			"evidence_summary":   map[string]interface{}{"evidence": "Strong"},
			"recommendations":    map[string]interface{}{"recs": "Clear"},
			"methodology":        map[string]interface{}{"method": "ACMG"},
		},
		Summary: ReportSummary{
			Classification: "Pathogenic",
			Confidence:     0.9,
			KeyFindings:    []string{"Strong pathogenic evidence"},
		},
		QualityMetrics: ReportQualityMetrics{
			CompletenessScore: 0.95,
			EvidenceQuality:   "high",
			DataSources:       5,
		},
		Recommendations: []string{"Genetic counseling", "Family screening"},
		Disclaimers:     []string{"Clinical use only", "Subject to change"},
	}

	poorReport := ReportResult{
		ReportID:     "",  // Missing required field
		HGVSNotation: "NM_000492.3:c.1521_1523delCTT",
		GeneSymbol:   "",  // Missing gene symbol
		Sections:     map[string]interface{}{}, // No sections
		Summary: ReportSummary{
			Classification: "Unknown",  // Invalid classification
			Confidence:     0.2,        // Low confidence
		},
		QualityMetrics: ReportQualityMetrics{
			DataSources: 1,  // Insufficient sources
		},
		Recommendations: []string{}, // No recommendations
		Disclaimers:     []string{}, // No disclaimers
	}

	tests := []struct {
		name            string
		params          ValidateReportParams
		expectValid     bool
		expectedIssues  int
		minOverallScore float64
		maxOverallScore float64
	}{
		{
			name: "Good report - basic validation",
			params: ValidateReportParams{
				Report:          goodReport,
				ValidationLevel: "basic",
			},
			expectValid:     true,
			expectedIssues:  0,
			minOverallScore: 0.8,
			maxOverallScore: 1.0,
		},
		{
			name: "Good report - comprehensive validation",
			params: ValidateReportParams{
				Report:          goodReport,
				ValidationLevel: "comprehensive",
				ComplianceRules: []string{"ACMG", "CLIA"},
			},
			expectValid:     true,
			expectedIssues:  2, // May have some minor issues
			minOverallScore: 0.7,
			maxOverallScore: 1.0,
		},
		{
			name: "Poor report - basic validation",
			params: ValidateReportParams{
				Report:          poorReport,
				ValidationLevel: "basic",
			},
			expectValid:     false,
			expectedIssues:  3, // Missing report ID, gene symbol, classification
			minOverallScore: 0.0,
			maxOverallScore: 0.5,
		},
		{
			name: "Poor report - strict validation",
			params: ValidateReportParams{
				Report:          poorReport,
				ValidationLevel: "strict",
			},
			expectValid:     false,
			expectedIssues:  15, // Many more issues in strict mode
			minOverallScore: 0.0,
			maxOverallScore: 0.6,
		},
		{
			name: "Custom validation checks",
			params: ValidateReportParams{
				Report:          goodReport,
				ValidationLevel: "standard",
				CustomChecks: []CustomValidationCheck{
					{
						Name:        "Custom Test",
						Description: "Test custom validation",
						Rule:        "test_rule",
						Severity:    "warning",
					},
				},
			},
			expectValid:     true,
			expectedIssues:  1, // Custom check adds one issue
			minOverallScore: 0.6,
			maxOverallScore: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &protocol.JSONRPC2Request{
				Method: "validate_report",
				Params: tt.params,
			}

			ctx := context.Background()
			response := tool.HandleTool(ctx, req)

			require.Nil(t, response.Error, "Expected no error: %v", response.Error)
			require.NotNil(t, response.Result, "Expected result")

			// Validate result structure
			resultMap, ok := response.Result.(map[string]interface{})
			require.True(t, ok, "Result should be a map")

			validationInterface, exists := resultMap["validation"]
			require.True(t, exists, "Result should contain validation")

			// Convert to ReportValidationResult
			validationBytes, err := json.Marshal(validationInterface)
			require.NoError(t, err)

			var validation ReportValidationResult
			err = json.Unmarshal(validationBytes, &validation)
			require.NoError(t, err)

			// Validate basic structure
			assert.Equal(t, tt.expectValid, validation.IsValid, "Validity should match expectation")
			assert.GreaterOrEqual(t, len(validation.ValidationIssues), tt.expectedIssues-2, "Should have expected issues (within range)")
			assert.LessOrEqual(t, len(validation.ValidationIssues), tt.expectedIssues+2, "Should not have too many issues")
			
			assert.GreaterOrEqual(t, validation.OverallScore, tt.minOverallScore, "Overall score should be above minimum")
			assert.LessOrEqual(t, validation.OverallScore, tt.maxOverallScore, "Overall score should be below maximum")

			// Validate validation summary
			assert.NotEmpty(t, validation.ValidationSummary, "Should have validation summary")

			// Validate quality assessment
			assert.GreaterOrEqual(t, validation.QualityAssessment.CompletenessScore, 0.0)
			assert.LessOrEqual(t, validation.QualityAssessment.CompletenessScore, 1.0)
			assert.GreaterOrEqual(t, validation.QualityAssessment.AccuracyScore, 0.0)
			assert.LessOrEqual(t, validation.QualityAssessment.AccuracyScore, 1.0)

			// Validate compliance status
			assert.NotNil(t, validation.ComplianceStatus.RequiredElements, "Should have required elements check")

			// Validate improvement suggestions
			if !validation.IsValid {
				assert.NotEmpty(t, validation.ImprovementSuggestions, "Invalid reports should have improvement suggestions")
			}

			// Validate metadata
			assert.NotEmpty(t, validation.ValidationMetadata.ValidationDate, "Should have validation date")
			assert.NotEmpty(t, validation.ValidationMetadata.ValidatorVersion, "Should have validator version")
		})
	}
}

// TestReportToolsIntegration tests the full workflow of generate -> format -> validate
func TestReportToolsIntegration(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	generateTool := NewGenerateReportTool(logger)
	formatTool := NewFormatReportTool(logger)
	validateTool := NewValidateReportTool(logger)

	// Step 1: Generate a report
	generateParams := GenerateReportParams{
		HGVSNotation: "NM_000492.3:c.1521_1523delCTT",
		GeneSymbol:   "CFTR",
		Classification: ClassifyVariantResult{
			Classification: "Pathogenic",
			Confidence:    "high",
			AppliedRules: []ACMGAMPRuleResult{
				{RuleCode: "PVS1", RuleName: "Null variant", Applied: true},
			},
		},
		ReportTemplate: "clinical",
	}

	generateReq := &protocol.JSONRPC2Request{
		Method: "generate_report",
		Params: generateParams,
	}

	ctx := context.Background()
	generateResponse := generateTool.HandleTool(ctx, generateReq)

	require.Nil(t, generateResponse.Error, "Generate should succeed")
	require.NotNil(t, generateResponse.Result, "Generate should return result")

	// Extract the generated report
	generateResultMap := generateResponse.Result.(map[string]interface{})
	reportInterface := generateResultMap["report"]

	reportBytes, err := json.Marshal(reportInterface)
	require.NoError(t, err)

	var generatedReport ReportResult
	err = json.Unmarshal(reportBytes, &generatedReport)
	require.NoError(t, err)

	// Step 2: Format the report as HTML
	formatParams := FormatReportParams{
		Report:        generatedReport,
		OutputFormat:  "html",
		IncludeHeader: true,
		IncludeFooter: true,
	}

	formatReq := &protocol.JSONRPC2Request{
		Method: "format_report",
		Params: formatParams,
	}

	formatResponse := formatTool.HandleTool(ctx, formatReq)

	require.Nil(t, formatResponse.Error, "Format should succeed")
	require.NotNil(t, formatResponse.Result, "Format should return result")

	// Validate the formatted report
	formatResultMap := formatResponse.Result.(map[string]interface{})
	formattedReportInterface := formatResultMap["formatted_report"]

	formattedBytes, err := json.Marshal(formattedReportInterface)
	require.NoError(t, err)

	var formattedResult FormatReportResult
	err = json.Unmarshal(formattedBytes, &formattedResult)
	require.NoError(t, err)

	assert.Equal(t, "html", formattedResult.Format)
	assert.Contains(t, formattedResult.FormattedContent, "<!DOCTYPE html>")
	assert.Contains(t, formattedResult.FormattedContent, "CFTR")
	assert.Contains(t, formattedResult.FormattedContent, "Pathogenic")

	// Step 3: Validate the original report
	validateParams := ValidateReportParams{
		Report:          generatedReport,
		ValidationLevel: "comprehensive",
		ComplianceRules: []string{"ACMG"},
	}

	validateReq := &protocol.JSONRPC2Request{
		Method: "validate_report",
		Params: validateParams,
	}

	validateResponse := validateTool.HandleTool(ctx, validateReq)

	require.Nil(t, validateResponse.Error, "Validate should succeed")
	require.NotNil(t, validateResponse.Result, "Validate should return result")

	// Check validation results
	validateResultMap := validateResponse.Result.(map[string]interface{})
	validationInterface := validateResultMap["validation"]

	validationBytes, err := json.Marshal(validationInterface)
	require.NoError(t, err)

	var validationResult ReportValidationResult
	err = json.Unmarshal(validationBytes, &validationResult)
	require.NoError(t, err)

	// The generated report should be reasonably valid
	assert.True(t, validationResult.IsValid, "Generated report should be valid")
	assert.GreaterOrEqual(t, validationResult.OverallScore, 0.6, "Should have reasonable quality score")
	assert.NotEmpty(t, validationResult.ValidationSummary, "Should have validation summary")

	t.Logf("Integration test completed successfully:")
	t.Logf("- Generated report: %s", generatedReport.ReportID)
	t.Logf("- Formatted as HTML: %d bytes", formattedResult.Size)
	t.Logf("- Validation score: %.2f", validationResult.OverallScore)
	t.Logf("- Validation issues: %d", len(validationResult.ValidationIssues))
}

// TestReportToolErrors tests error handling in report tools
func TestReportToolErrors(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	tests := []struct {
		name     string
		tool     protocol.ToolHandler
		method   string
		params   interface{}
		errorCode int
	}{
		{
			name:      "Generate report - missing HGVS",
			tool:      NewGenerateReportTool(logger),
			method:    "generate_report",
			params:    map[string]interface{}{},
			errorCode: -32602,
		},
		{
			name:   "Generate report - invalid template",
			tool:   NewGenerateReportTool(logger),
			method: "generate_report",
			params: GenerateReportParams{
				HGVSNotation: "NM_000492.3:c.1521_1523delCTT",
				Classification: ClassifyVariantResult{
					Classification: "Pathogenic",
					Confidence:    "high",
				},
				ReportTemplate: "invalid_template",
			},
			errorCode: -32602,
		},
		{
			name:      "Format report - missing report",
			tool:      NewFormatReportTool(logger),
			method:    "format_report",
			params:    map[string]interface{}{"output_format": "json"},
			errorCode: -32602,
		},
		{
			name:   "Format report - invalid format",
			tool:   NewFormatReportTool(logger),
			method: "format_report",
			params: FormatReportParams{
				Report:       ReportResult{ReportID: "test"},
				OutputFormat: "invalid_format",
			},
			errorCode: -32602,
		},
		{
			name:      "Validate report - missing report",
			tool:      NewValidateReportTool(logger),
			method:    "validate_report",
			params:    map[string]interface{}{},
			errorCode: -32602,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &protocol.JSONRPC2Request{
				Method: tt.method,
				Params: tt.params,
			}

			ctx := context.Background()
			response := tt.tool.HandleTool(ctx, req)

			require.NotNil(t, response.Error, "Expected error response")
			assert.Equal(t, tt.errorCode, int(response.Error.Code), "Expected specific error code")
			assert.NotEmpty(t, response.Error.Message, "Error should have message")
		})
	}
}

// Helper function to test tool info
func TestReportToolsInfo(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	tools := []struct {
		name string
		tool protocol.ToolHandler
	}{
		{"generate_report", NewGenerateReportTool(logger)},
		{"format_report", NewFormatReportTool(logger)},
		{"validate_report", NewValidateReportTool(logger)},
	}

	for _, tt := range tools {
		t.Run(tt.name, func(t *testing.T) {
			info := tt.tool.GetToolInfo()
			
			assert.Equal(t, tt.name, info.Name, "Tool name should match")
			assert.NotEmpty(t, info.Description, "Should have description")
			assert.NotNil(t, info.InputSchema, "Should have input schema")

			// Validate schema structure
			schema := info.InputSchema
			assert.Equal(t, "object", schema["type"], "Schema should be object type")
			assert.NotNil(t, schema["properties"], "Should have properties")
			assert.NotNil(t, schema["required"], "Should have required fields")
		})
	}
}