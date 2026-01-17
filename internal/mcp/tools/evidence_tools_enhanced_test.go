package tools

import (
	"context"
	"testing"

	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"

	"github.com/acmg-amp-mcp-server/internal/mcp/protocol"
)

// =============================================================================
// RED Phase Tests: Enhanced query_evidence Output
// These tests verify the enhanced output schema per REQ-MCP-002
// Tests are expected to FAIL initially until implementation is complete
// =============================================================================

// TestQueryEvidence_EnhancedOutput_ReturnsACMGCriteriaHints tests ACMG criteria mapping
func TestQueryEvidence_EnhancedOutput_ReturnsACMGCriteriaHints(t *testing.T) {
	// Arrange
	logger, _ := test.NewNullLogger()
	tool := NewQueryEvidenceTool(logger)

	params := map[string]interface{}{
		"hgvs_notation": "NM_007294.4:c.5266dup",
		"databases":     []string{"clinvar", "gnomad"},
	}

	req := &protocol.JSONRPC2Request{
		JSONRPC: "2.0",
		Method:  "query_evidence",
		Params:  params,
		ID:      1,
	}

	// Act
	ctx := context.Background()
	response := tool.HandleTool(ctx, req)

	// Assert
	assert.Nil(t, response.Error, "Expected no error")

	resultMap, ok := response.Result.(map[string]interface{})
	assert.True(t, ok, "Result should be a map")

	evidence, ok := resultMap["evidence"].(*QueryEvidenceResult)
	assert.True(t, ok, "Evidence result should be present")

	// Enhanced output assertions - ACMG Criteria Hints
	assert.NotNil(t, evidence.ACMGCriteriaHints, "ACMG criteria hints should be present")
	if evidence.ACMGCriteriaHints != nil {
		// Should have at least some criteria mapped based on evidence
		t.Logf("ACMG Criteria Hints: %v", evidence.ACMGCriteriaHints)
	}
}

// TestQueryEvidence_EnhancedOutput_ReturnsSynthesis tests evidence synthesis text
func TestQueryEvidence_EnhancedOutput_ReturnsSynthesis(t *testing.T) {
	// Arrange
	logger, _ := test.NewNullLogger()
	tool := NewQueryEvidenceTool(logger)

	params := map[string]interface{}{
		"hgvs_notation": "NM_000492.3:c.1521_1523delCTT",
		"databases":     []string{"clinvar", "gnomad", "cosmic"},
	}

	req := &protocol.JSONRPC2Request{
		JSONRPC: "2.0",
		Method:  "query_evidence",
		Params:  params,
		ID:      1,
	}

	// Act
	ctx := context.Background()
	response := tool.HandleTool(ctx, req)

	// Assert
	assert.Nil(t, response.Error, "Expected no error")

	resultMap, ok := response.Result.(map[string]interface{})
	assert.True(t, ok, "Result should be a map")

	evidence, ok := resultMap["evidence"].(*QueryEvidenceResult)
	assert.True(t, ok, "Evidence result should be present")

	// Enhanced output assertions - Synthesis
	assert.NotEmpty(t, evidence.Synthesis, "Evidence synthesis text should be present")
	t.Logf("Synthesis: %s", evidence.Synthesis)
}

// TestQueryEvidence_EnhancedOutput_ReturnsSourceQualityScores tests per-source quality scores
func TestQueryEvidence_EnhancedOutput_ReturnsSourceQualityScores(t *testing.T) {
	// Arrange
	logger, _ := test.NewNullLogger()
	tool := NewQueryEvidenceTool(logger)

	params := map[string]interface{}{
		"hgvs_notation": "NM_007294.4:c.5266dup",
		"databases":     []string{"clinvar", "gnomad"},
	}

	req := &protocol.JSONRPC2Request{
		JSONRPC: "2.0",
		Method:  "query_evidence",
		Params:  params,
		ID:      1,
	}

	// Act
	ctx := context.Background()
	response := tool.HandleTool(ctx, req)

	// Assert
	assert.Nil(t, response.Error, "Expected no error")

	resultMap, ok := response.Result.(map[string]interface{})
	assert.True(t, ok, "Result should be a map")

	evidence, ok := resultMap["evidence"].(*QueryEvidenceResult)
	assert.True(t, ok, "Evidence result should be present")

	// Enhanced output assertions - Source Quality
	assert.NotNil(t, evidence.SourceQuality, "Source quality should be present")
	if evidence.SourceQuality != nil {
		// Check ClinVar quality
		if clinvarQuality, ok := evidence.SourceQuality["clinvar"]; ok {
			assert.NotEmpty(t, clinvarQuality.Quality, "ClinVar quality rating should be present")
			assert.Contains(t, []string{"high", "medium", "low"}, clinvarQuality.Quality,
				"Quality should be one of: high, medium, low")
		}
	}
}

// TestQueryEvidence_EnhancedOutput_ReturnsOverallQualityAssessment tests overall quality assessment
func TestQueryEvidence_EnhancedOutput_ReturnsOverallQualityAssessment(t *testing.T) {
	// Arrange
	logger, _ := test.NewNullLogger()
	tool := NewQueryEvidenceTool(logger)

	params := map[string]interface{}{
		"hgvs_notation": "NM_000492.3:c.1521_1523delCTT",
		"databases":     []string{"clinvar", "gnomad", "cosmic"},
	}

	req := &protocol.JSONRPC2Request{
		JSONRPC: "2.0",
		Method:  "query_evidence",
		Params:  params,
		ID:      1,
	}

	// Act
	ctx := context.Background()
	response := tool.HandleTool(ctx, req)

	// Assert
	assert.Nil(t, response.Error, "Expected no error")

	resultMap, ok := response.Result.(map[string]interface{})
	assert.True(t, ok, "Result should be a map")

	evidence, ok := resultMap["evidence"].(*QueryEvidenceResult)
	assert.True(t, ok, "Evidence result should be present")

	// Enhanced output assertions - Quality Assessment
	assert.NotEmpty(t, evidence.QualityScores.OverallQuality, "Overall quality should be present")
	assert.GreaterOrEqual(t, evidence.QualityScores.DataCompleteness, 0.0, "Completeness should be >= 0")
	assert.LessOrEqual(t, evidence.QualityScores.DataCompleteness, 1.0, "Completeness should be <= 1")
}

// TestQueryEvidence_EnhancedOutput_FullSchema tests the complete enhanced output schema
func TestQueryEvidence_EnhancedOutput_FullSchema(t *testing.T) {
	// Arrange
	logger, _ := test.NewNullLogger()
	tool := NewQueryEvidenceTool(logger)

	params := map[string]interface{}{
		"hgvs_notation": "NM_000492.3:c.1521_1523delCTT",
		"databases":     []string{"clinvar", "gnomad", "cosmic", "pubmed"},
	}

	req := &protocol.JSONRPC2Request{
		JSONRPC: "2.0",
		Method:  "query_evidence",
		Params:  params,
		ID:      1,
	}

	// Act
	ctx := context.Background()
	response := tool.HandleTool(ctx, req)

	// Assert
	assert.Nil(t, response.Error, "Expected no error")

	resultMap, ok := response.Result.(map[string]interface{})
	assert.True(t, ok, "Result should be a map")

	evidence, ok := resultMap["evidence"].(*QueryEvidenceResult)
	assert.True(t, ok, "Evidence result should be present")

	// Verify all enhanced fields are present according to REQ-MCP-002

	// 1. Variant identifier
	assert.NotEmpty(t, evidence.HGVSNotation, "HGVS notation should be present")

	// 2. Database results (already exists)
	assert.NotNil(t, evidence.DatabaseResults, "Database results should be present")

	// 3. Source Quality (per-source quality ratings)
	assert.NotNil(t, evidence.SourceQuality, "Source quality should be present")

	// 4. Overall Quality Assessment
	assert.NotEmpty(t, evidence.QualityScores.OverallQuality, "Overall quality should be present")
	assert.GreaterOrEqual(t, evidence.QualityScores.DataCompleteness, 0.0, "Completeness should be valid")

	// 5. ACMG Criteria Hints
	assert.NotNil(t, evidence.ACMGCriteriaHints, "ACMG criteria hints should be present")

	// 6. Evidence Synthesis
	assert.NotEmpty(t, evidence.Synthesis, "Evidence synthesis should be present")
}

// TestQueryEvidence_EnhancedOutput_LowQualityEvidence tests quality assessment for limited data
func TestQueryEvidence_EnhancedOutput_LowQualityEvidence(t *testing.T) {
	// Arrange
	logger, _ := test.NewNullLogger()
	tool := NewQueryEvidenceTool(logger)

	// Query only one database to get limited evidence
	params := map[string]interface{}{
		"hgvs_notation": "NM_007294.4:c.9999A>T", // Novel/rare variant
		"databases":     []string{"gnomad"},      // Only population data
	}

	req := &protocol.JSONRPC2Request{
		JSONRPC: "2.0",
		Method:  "query_evidence",
		Params:  params,
		ID:      1,
	}

	// Act
	ctx := context.Background()
	response := tool.HandleTool(ctx, req)

	// Assert
	assert.Nil(t, response.Error, "Expected no error")

	resultMap, ok := response.Result.(map[string]interface{})
	assert.True(t, ok, "Result should be a map")

	evidence, ok := resultMap["evidence"].(*QueryEvidenceResult)
	assert.True(t, ok, "Evidence result should be present")

	// With only one database, quality should reflect limited data
	assert.LessOrEqual(t, evidence.QualityScores.DataCompleteness, 0.5,
		"Completeness should be low for single-database query")

	// Synthesis should mention limited evidence
	if evidence.Synthesis != "" {
		t.Logf("Synthesis for limited evidence: %s", evidence.Synthesis)
	}
}

// TestQueryEvidence_EnhancedOutput_ACMGCriteriaMapping tests specific ACMG criteria mapping
func TestQueryEvidence_EnhancedOutput_ACMGCriteriaMapping(t *testing.T) {
	// Arrange
	logger, _ := test.NewNullLogger()
	tool := NewQueryEvidenceTool(logger)

	// Well-characterized pathogenic variant
	params := map[string]interface{}{
		"hgvs_notation": "NM_000492.3:c.1521_1523delCTT",
		"databases":     []string{"clinvar", "gnomad"},
	}

	req := &protocol.JSONRPC2Request{
		JSONRPC: "2.0",
		Method:  "query_evidence",
		Params:  params,
		ID:      1,
	}

	// Act
	ctx := context.Background()
	response := tool.HandleTool(ctx, req)

	// Assert
	assert.Nil(t, response.Error, "Expected no error")

	resultMap, ok := response.Result.(map[string]interface{})
	assert.True(t, ok, "Result should be a map")

	evidence, ok := resultMap["evidence"].(*QueryEvidenceResult)
	assert.True(t, ok, "Evidence result should be present")

	// ACMG criteria mapping should include:
	// PM2: Absent from controls (or at extremely low frequency) in gnomAD
	// PS1/PS4: ClinVar pathogenic assertion with good review status
	if evidence.ACMGCriteriaHints != nil {
		// Check for PM2 (population frequency criterion)
		if pm2, exists := evidence.ACMGCriteriaHints["PM2"]; exists {
			assert.NotNil(t, pm2, "PM2 criteria hint should be present")
			t.Logf("PM2 hint: applicable=%v, note=%s", pm2.Applicable, pm2.Note)
		}

		// Log all available hints
		for code, hint := range evidence.ACMGCriteriaHints {
			t.Logf("ACMG %s: applicable=%v, note=%s", code, hint.Applicable, hint.Note)
		}
	}
}
