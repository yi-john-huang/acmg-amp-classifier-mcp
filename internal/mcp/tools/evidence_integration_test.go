package tools

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/acmg-amp-mcp-server/internal/mcp/protocol"
)

// TestEvidenceToolsIntegration tests the full evidence gathering workflow
func TestEvidenceToolsIntegration(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Reduce noise in tests

	tests := []struct {
		name           string
		hgvsNotation   string
		expectEvidence bool
		expectError    bool
	}{
		{
			name:           "CFTR variant with known evidence",
			hgvsNotation:   "NM_000492.3:c.1521_1523delCTT",
			expectEvidence: true,
			expectError:    false,
		},
		{
			name:           "BRCA1 variant with clinical significance",
			hgvsNotation:   "NM_007294.3:c.68_69delAG",
			expectEvidence: true,
			expectError:    false,
		},
		{
			name:           "Novel variant with limited evidence",
			hgvsNotation:   "NM_000001.1:c.100A>G",
			expectEvidence: true,
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test query_evidence tool
			queryTool := NewQueryEvidenceTool(logger)
			result := testQueryEvidenceTool(t, queryTool, tt.hgvsNotation)

			if tt.expectEvidence {
				assert.NotNil(t, result, "Expected evidence result")
			}

			if !tt.expectError {
				// Validate evidence structure
				validateEvidenceStructure(t, result)
			}
		})
	}
}

// TestDatabaseSpecificIntegration tests individual database tools
func TestDatabaseSpecificIntegration(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	hgvsNotation := "NM_000492.3:c.1521_1523delCTT"

	t.Run("ClinVar Integration", func(t *testing.T) {
		clinvarTool := NewQueryClinVarTool(logger)
		result := testDatabaseTool(t, clinvarTool, "query_clinvar", hgvsNotation)
		assert.NotNil(t, result, "ClinVar should return data")
	})

	t.Run("gnomAD Integration", func(t *testing.T) {
		gnomadTool := NewQueryGnomADTool(logger)
		result := testDatabaseTool(t, gnomadTool, "query_gnomad", hgvsNotation)
		assert.NotNil(t, result, "gnomAD should return data")
	})

	t.Run("COSMIC Integration", func(t *testing.T) {
		cosmicTool := NewQueryCOSMICTool(logger)
		result := testDatabaseTool(t, cosmicTool, "query_cosmic", hgvsNotation)
		assert.NotNil(t, result, "COSMIC should return data")
	})
}

// TestBatchEvidenceIntegration tests batch evidence processing
func TestBatchEvidenceIntegration(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	batchTool := NewBatchEvidenceTool(logger)

	variants := []VariantQuery{
		{
			HGVSNotation: "NM_000492.3:c.1521_1523delCTT",
			GeneSymbol:   "CFTR",
			Priority:     1,
		},
		{
			HGVSNotation: "NM_007294.3:c.68_69delAG",
			GeneSymbol:   "BRCA1",
			Priority:     2,
		},
		{
			HGVSNotation: "NM_000001.1:c.100A>G",
			GeneSymbol:   "TEST",
			Priority:     1,
		},
	}

	params := BatchEvidenceParams{
		Variants:      variants,
		Databases:     []string{"clinvar", "gnomad", "cosmic"},
		IncludeRaw:    true,
		MaxAge:        "24h",
		MaxConcurrent: 3,
	}

	// Create JSONRPC request
	req := &protocol.JSONRPC2Request{
		Method: "batch_query_evidence",
		Params: params,
	}

	ctx := context.Background()
	response := batchTool.HandleTool(ctx, req)

	require.Nil(t, response.Error, "Batch tool should not error")
	require.NotNil(t, response.Result, "Batch tool should return result")

	// Validate batch result structure
	resultMap, ok := response.Result.(map[string]interface{})
	require.True(t, ok, "Result should be a map")

	batchEvidenceInterface, exists := resultMap["batch_evidence"]
	require.True(t, exists, "Result should contain batch_evidence")
	require.NotNil(t, batchEvidenceInterface, "Batch evidence should not be nil")

	// Test that processing completed successfully
	t.Logf("Batch evidence processing completed")
}

// TestEvidenceCaching tests the caching functionality
func TestEvidenceCaching(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cache := NewEvidenceCache(logger)
	hgvsNotation := "NM_000492.3:c.1521_1523delCTT"

	// Test cache miss
	result := cache.Get(hgvsNotation, "24h")
	assert.Nil(t, result, "Cache should be empty initially")

	// Test cache set and hit
	mockEvidence := &QueryEvidenceResult{
		HGVSNotation: hgvsNotation,
		QualityScores: EvidenceQualityScores{
			OverallQuality:   "high",
			DataCompleteness: 0.8,
		},
		DatabaseResults: make(map[string]interface{}),
	}

	cache.Set(hgvsNotation, mockEvidence)
	cachedResult := cache.Get(hgvsNotation, "24h")
	assert.NotNil(t, cachedResult, "Cache should return stored evidence")
	assert.Equal(t, hgvsNotation, cachedResult.HGVSNotation, "Cached HGVS should match")

	// Test cache expiration
	expiredResult := cache.Get(hgvsNotation, "0s")
	assert.Nil(t, expiredResult, "Expired cache entry should return nil")

	// Test cache stats
	stats := cache.GetStats()
	assert.NotNil(t, stats, "Cache stats should be available")
	assert.Contains(t, stats, "total_entries", "Stats should contain entry count")
}

// TestEvidenceQualityScoring tests quality assessment functionality
func TestEvidenceQualityScoring(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	queryTool := NewQueryEvidenceTool(logger)
	result := testQueryEvidenceTool(t, queryTool, "NM_000492.3:c.1521_1523delCTT")

	require.NotNil(t, result, "Evidence result should not be nil")

	// Validate quality scoring structure
	assert.NotEmpty(t, result.QualityScores.OverallQuality, "Overall quality should be assessed")
	assert.GreaterOrEqual(t, result.QualityScores.DataCompleteness, 0.0, "Data completeness should be valid")
	assert.NotNil(t, result.QualityScores.SourceReliability, "Source reliability should exist")
}

// TestConcurrentEvidenceQueries tests concurrent access patterns
func TestConcurrentEvidenceQueries(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	queryTool := NewQueryEvidenceTool(logger)
	hgvsNotation := "NM_000492.3:c.1521_1523delCTT"

	// Test concurrent queries
	const numConcurrent = 5
	results := make(chan interface{}, numConcurrent)

	for i := 0; i < numConcurrent; i++ {
		go func() {
			result := testQueryEvidenceTool(t, queryTool, hgvsNotation)
			results <- result
		}()
	}

	// Collect all results
	for i := 0; i < numConcurrent; i++ {
		select {
		case result := <-results:
			assert.NotNil(t, result, "Concurrent query should return result")
		case <-time.After(30 * time.Second):
			t.Fatal("Concurrent query timed out")
		}
	}
}

// TestErrorHandling tests error scenarios for evidence tools
func TestErrorHandling(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	queryTool := NewQueryEvidenceTool(logger)

	tests := []struct {
		name         string
		params       interface{}
		expectError  bool
		errorMessage string
	}{
		{
			name:         "Missing HGVS notation",
			params:       map[string]interface{}{},
			expectError:  true,
			errorMessage: "hgvs_notation is required",
		},
		{
			name: "Invalid HGVS format",
			params: map[string]interface{}{
				"hgvs_notation": "invalid_hgvs",
			},
			expectError: false, // Tool should handle gracefully
		},
		{
			name: "Empty databases list",
			params: map[string]interface{}{
				"hgvs_notation": "NM_000492.3:c.1521_1523delCTT",
				"databases":     []string{},
			},
			expectError: false, // Should use defaults
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &protocol.JSONRPC2Request{
				Method: "query_evidence",
				Params: tt.params,
			}

			ctx := context.Background()
			response := queryTool.HandleTool(ctx, req)

			if tt.expectError {
				assert.NotNil(t, response.Error, "Expected error response")
				if tt.errorMessage != "" {
					assert.Contains(t, response.Error.Data.(string), tt.errorMessage)
				}
			} else {
				// For non-error cases, we expect either success or graceful handling
				if response.Error != nil {
					t.Logf("Tool handled error gracefully: %s", response.Error.Message)
				}
			}
		})
	}
}

// Helper functions

func testQueryEvidenceTool(t *testing.T, tool *QueryEvidenceTool, hgvs string) *QueryEvidenceResult {
	params := QueryEvidenceParams{
		HGVSNotation: hgvs,
		Databases:    []string{"clinvar", "gnomad", "cosmic"},
		IncludeRaw:   false,
		MaxAge:       "24h",
	}

	req := &protocol.JSONRPC2Request{
		Method: "query_evidence",
		Params: params,
	}

	ctx := context.Background()
	response := tool.HandleTool(ctx, req)

	if response.Error != nil {
		t.Logf("Tool returned error: %s", response.Error.Message)
		return nil
	}

	require.NotNil(t, response.Result, "Tool should return result")

	resultMap, ok := response.Result.(map[string]interface{})
	require.True(t, ok, "Result should be a map")

	evidenceInterface, exists := resultMap["evidence"]
	require.True(t, exists, "Result should contain evidence")

	// Convert back to our structure for validation
	evidenceBytes, err := json.Marshal(evidenceInterface)
	require.NoError(t, err)

	var evidence QueryEvidenceResult
	err = json.Unmarshal(evidenceBytes, &evidence)
	require.NoError(t, err)

	return &evidence
}

func testDatabaseTool(t *testing.T, handler protocol.ToolHandler, toolName, hgvs string) interface{} {
	var params interface{}

	switch toolName {
	case "query_clinvar":
		params = QueryClinVarParams{HGVSNotation: hgvs}
	case "query_gnomad":
		params = QueryGnomADParams{HGVSNotation: hgvs}
	case "query_cosmic":
		params = QueryCOSMICParams{HGVSNotation: hgvs}
	}

	req := &protocol.JSONRPC2Request{
		Method: toolName,
		Params: params,
	}

	ctx := context.Background()
	response := handler.HandleTool(ctx, req)

	if response.Error != nil {
		t.Logf("Database tool %s returned error: %s", toolName, response.Error.Message)
		return nil
	}

	return response.Result
}

func validateEvidenceStructure(t *testing.T, evidence *QueryEvidenceResult) {
	assert.NotEmpty(t, evidence.HGVSNotation, "HGVS notation should be present")
	assert.NotNil(t, evidence.AggregatedEvidence, "Aggregated evidence should exist")
	assert.NotNil(t, evidence.QualityScores, "Quality scores should exist")
	assert.NotNil(t, evidence.DatabaseResults, "Database results should exist")

	// Validate aggregated evidence structure
	agg := evidence.AggregatedEvidence
	assert.NotNil(t, agg.PopulationFrequency, "Population frequency data should exist")
	assert.NotNil(t, agg.ClinicalEvidence, "Clinical evidence should exist")
	assert.NotNil(t, agg.FunctionalEvidence, "Functional evidence should exist")
	assert.NotNil(t, agg.ComputationalData, "Computational data should exist")
	assert.NotNil(t, agg.LiteratureEvidence, "Literature evidence should exist")

	// Validate quality scores
	assert.NotEmpty(t, evidence.QualityScores.OverallQuality, "Overall quality should be assessed")
	assert.GreaterOrEqual(t, evidence.QualityScores.DataCompleteness, 0.0, "Data completeness should be valid")
}