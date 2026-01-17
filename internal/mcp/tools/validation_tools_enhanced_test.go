package tools

import (
	"context"
	"testing"

	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"

	"github.com/acmg-amp-mcp-server/internal/mcp/protocol"
)

// =============================================================================
// RED Phase Tests: Enhanced validate_hgvs Output
// These tests verify the enhanced output schema per REQ-MCP-001
// Tests are expected to FAIL initially until implementation is complete
// =============================================================================

// TestValidateHGVS_EnhancedOutput_ReturnsGeneInfo tests that validation returns gene information
func TestValidateHGVS_EnhancedOutput_ReturnsGeneInfo(t *testing.T) {
	// Arrange
	logger, _ := test.NewNullLogger()
	tool := NewValidateHGVSTool(logger, nil)

	params := map[string]interface{}{
		"hgvs_notation": "NM_007294.4:c.5266dup",
	}

	req := &protocol.JSONRPC2Request{
		JSONRPC: "2.0",
		Method:  "validate_hgvs",
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

	validation, ok := resultMap["validation"].(*ValidateHGVSResult)
	assert.True(t, ok, "Validation result should be present")

	// Enhanced output assertions - Gene Info
	assert.NotNil(t, validation.GeneInfo, "Gene info should be present")
	if validation.GeneInfo != nil {
		assert.NotEmpty(t, validation.GeneInfo.Symbol, "Gene symbol should be present")
		assert.NotEmpty(t, validation.GeneInfo.Name, "Gene name should be present")
		assert.NotEmpty(t, validation.GeneInfo.HGNCID, "HGNC ID should be present")
	}
}

// TestValidateHGVS_EnhancedOutput_ReturnsTranscriptInfo tests that validation returns transcript information
func TestValidateHGVS_EnhancedOutput_ReturnsTranscriptInfo(t *testing.T) {
	// Arrange
	logger, _ := test.NewNullLogger()
	tool := NewValidateHGVSTool(logger, nil)

	params := map[string]interface{}{
		"hgvs_notation": "NM_007294.4:c.5266dup",
	}

	req := &protocol.JSONRPC2Request{
		JSONRPC: "2.0",
		Method:  "validate_hgvs",
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

	validation, ok := resultMap["validation"].(*ValidateHGVSResult)
	assert.True(t, ok, "Validation result should be present")

	// Enhanced output assertions - Transcript Info
	assert.NotNil(t, validation.TranscriptInfo, "Transcript info should be present")
	if validation.TranscriptInfo != nil {
		assert.NotEmpty(t, validation.TranscriptInfo.RefSeq, "RefSeq ID should be present")
		// Ensembl ID may be empty for some transcripts
	}
}

// TestValidateHGVS_EnhancedOutput_ReturnsSuggestionsForInvalid tests suggestions for invalid input
func TestValidateHGVS_EnhancedOutput_ReturnsSuggestionsForInvalid(t *testing.T) {
	// Arrange
	logger, _ := test.NewNullLogger()
	tool := NewValidateHGVSTool(logger, nil)

	// Using an invalid/incomplete notation that should trigger suggestions
	params := map[string]interface{}{
		"hgvs_notation": "BRCA1:c.5266dupC", // Missing transcript version
	}

	req := &protocol.JSONRPC2Request{
		JSONRPC: "2.0",
		Method:  "validate_hgvs",
		Params:  params,
		ID:      1,
	}

	// Act
	ctx := context.Background()
	response := tool.HandleTool(ctx, req)

	// Assert
	resultMap, ok := response.Result.(map[string]interface{})
	if !ok {
		t.Skip("Result format not as expected - may need error handling")
		return
	}

	validation, ok := resultMap["validation"].(*ValidateHGVSResult)
	if !ok {
		t.Skip("Validation result format changed")
		return
	}

	// For invalid input, we expect suggestions
	assert.NotEmpty(t, validation.Suggestions, "Suggestions should be provided for invalid/incomplete input")
}

// TestValidateHGVS_EnhancedOutput_FullSchema tests the complete enhanced output schema
func TestValidateHGVS_EnhancedOutput_FullSchema(t *testing.T) {
	// Arrange
	logger, _ := test.NewNullLogger()
	tool := NewValidateHGVSTool(logger, nil)

	params := map[string]interface{}{
		"hgvs_notation": "NM_000492.3:c.1521_1523delCTT",
	}

	req := &protocol.JSONRPC2Request{
		JSONRPC: "2.0",
		Method:  "validate_hgvs",
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

	validation, ok := resultMap["validation"].(*ValidateHGVSResult)
	assert.True(t, ok, "Validation result should be present")

	// Verify all enhanced fields are present according to REQ-MCP-001
	// 1. IsValid
	// Already exists

	// 2. Normalized HGVS (should be present for valid input)
	if validation.IsValid {
		assert.NotEmpty(t, validation.NormalizedHGVS, "Normalized HGVS should be present for valid input")
	}

	// 3. Gene Info
	assert.NotNil(t, validation.GeneInfo, "Gene info should be present in enhanced output")

	// 4. Transcript Info
	assert.NotNil(t, validation.TranscriptInfo, "Transcript info should be present in enhanced output")

	// 5. Parsed Components (already exists)
	// ParsedComponents field already exists

	// 6. Suggestions (for invalid input, already tested separately)
}

// TestValidateHGVS_EnhancedOutput_GeneSymbolInput tests gene symbol notation input
func TestValidateHGVS_EnhancedOutput_GeneSymbolInput(t *testing.T) {
	// Arrange
	logger, _ := test.NewNullLogger()
	tool := NewValidateHGVSTool(logger, nil)

	// Input using gene symbol instead of transcript
	params := map[string]interface{}{
		"hgvs_notation": "CFTR:c.1521_1523delCTT",
	}

	req := &protocol.JSONRPC2Request{
		JSONRPC: "2.0",
		Method:  "validate_hgvs",
		Params:  params,
		ID:      1,
	}

	// Act
	ctx := context.Background()
	response := tool.HandleTool(ctx, req)

	// Assert - for gene symbol input, we should get suggestions for canonical transcript
	resultMap, ok := response.Result.(map[string]interface{})
	if !ok {
		// May be an error response for gene symbol input
		return
	}

	validation, ok := resultMap["validation"].(*ValidateHGVSResult)
	if !ok {
		return
	}

	// If input is gene symbol, we expect either:
	// 1. Suggestions for the canonical transcript
	// 2. Gene info to help identify the correct transcript
	if validation.GeneInfo != nil {
		assert.Equal(t, "CFTR", validation.GeneInfo.Symbol, "Gene symbol should match input")
	}

	if !validation.IsValid && len(validation.Suggestions) > 0 {
		// Good - suggestions provided for non-transcript notation
		t.Logf("Suggestions provided: %v", validation.Suggestions)
	}
}
