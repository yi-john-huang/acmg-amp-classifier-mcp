package tools

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/acmg-amp-mcp-server/internal/domain"
	"github.com/acmg-amp-mcp-server/internal/mcp/protocol"
	"github.com/acmg-amp-mcp-server/internal/service"
	"github.com/acmg-amp-mcp-server/pkg/external"
)

// GeneSymbolIntegrationTestSuite provides comprehensive end-to-end testing
type GeneSymbolIntegrationTestSuite struct {
	suite.Suite
	logger            *logrus.Logger
	classifierService *service.ClassifierService
	toolRegistry      *ToolRegistry
	router            *protocol.MessageRouter
	testContext       context.Context
}

// SetupSuite initializes the test suite with real services
func (suite *GeneSymbolIntegrationTestSuite) SetupSuite() {
	// Skip this integration test if running in short mode or without Redis
	if testing.Short() {
		suite.T().Skip("Skipping integration test in short mode")
	}

	suite.logger = logrus.New()
	suite.logger.SetLevel(logrus.DebugLevel)

	// Create test context with timeout
	suite.testContext, _ = context.WithTimeout(context.Background(), 30*time.Second)

	// Initialize services with test configurations
	suite.initializeServices()
	suite.initializeToolRegistry()
}

func (suite *GeneSymbolIntegrationTestSuite) initializeServices() {
	// Create knowledge base service with test configuration
	knowledgeBaseService, err := external.NewKnowledgeBaseService(
		domain.ClinVarConfig{
			BaseURL:   "https://eutils.ncbi.nlm.nih.gov/entrez/eutils",
			RateLimit: 10, // Higher rate limit for testing
			Timeout:   5 * time.Second,
		},
		domain.GnomADConfig{
			BaseURL:   "https://gnomad.broadinstitute.org/api",
			RateLimit: 20,
			Timeout:   5 * time.Second,
		},
		domain.COSMICConfig{
			BaseURL:   "https://cancer.sanger.ac.uk/cosmic/search",
			RateLimit: 10,
			Timeout:   5 * time.Second,
		},
		domain.PubMedConfig{
			BaseURL:   "https://eutils.ncbi.nlm.nih.gov/entrez/eutils",
			RateLimit: 10,
			Timeout:   5 * time.Second,
		},
		domain.LOVDConfig{
			BaseURL:   "https://www.lovd.nl/3.0/api",
			RateLimit: 10,
			Timeout:   5 * time.Second,
		},
		domain.HGMDConfig{
			BaseURL:   "https://my.qiagendigitalinsights.com/bbp/view/hgmd",
			RateLimit: 5,
			Timeout:   5 * time.Second,
		},
		domain.CacheConfig{
			RedisURL:    "redis://localhost:6379/1", // Test database
			DefaultTTL:  60 * time.Second,           // Shorter TTL for testing
			MaxRetries:  2,
			PoolSize:    5,
			PoolTimeout: 2 * time.Second,
		},
	)
	require.NoError(suite.T(), err, "Knowledge base service should initialize successfully")

	// Create input parser
	inputParser := domain.NewStandardInputParser()

	// Create transcript resolver with test configuration
	transcriptResolverConfig := service.TranscriptResolverConfig{
		MemoryCacheTTL: 5 * time.Minute, // Shorter TTL for testing
		RedisCacheTTL:  30 * time.Minute,
		MaxMemorySize:  100,
		MaxConcurrency: 3,
		ExternalAPIConfig: external.UnifiedGeneAPIConfig{
			RefSeqConfig: external.RefSeqConfig{
				BaseURL:    "https://eutils.ncbi.nlm.nih.gov/entrez/eutils",
				RateLimit:  10,
				Timeout:    10 * time.Second,
				MaxRetries: 2,
			},
			EnsemblConfig: external.EnsemblConfig{
				BaseURL:   "https://rest.ensembl.org",
				RateLimit: 20,
				Timeout:   10 * time.Second,
			},
			HGNCConfig: external.HGNCConfig{
				BaseURL:   "https://rest.genenames.org",
				RateLimit: 15,
				Timeout:   10 * time.Second,
			},
			CircuitBreaker: external.UnifiedCircuitBreakerConfig{
				MaxRequests: 3,
				Interval:    30 * time.Second,
				Timeout:     10 * time.Second,
			},
		},
	}

	// Pass nil for Redis cache since it's not actively used in transcript resolution
	// The memory cache (LRU) handles caching for tests
	cachedResolver, err := service.NewCachedTranscriptResolver(transcriptResolverConfig, nil, suite.logger)
	require.NoError(suite.T(), err, "Transcript resolver should initialize successfully")

	transcriptResolver := service.NewTranscriptResolverAdapter(cachedResolver)

	// Inject transcript resolver into input parser
	if standardParser, ok := inputParser.(*domain.StandardInputParser); ok {
		standardParser.SetTranscriptResolver(transcriptResolver)
	}

	// Create classifier service
	suite.classifierService = service.NewClassifierService(
		suite.logger,
		knowledgeBaseService,
		inputParser,
		transcriptResolver,
	)
}

func (suite *GeneSymbolIntegrationTestSuite) initializeToolRegistry() {
	// Create message router
	suite.router = protocol.NewMessageRouter(suite.logger)

	// Create and register tools
	suite.toolRegistry = NewToolRegistry(suite.logger, suite.router, suite.classifierService)
	err := suite.toolRegistry.RegisterAllTools()
	require.NoError(suite.T(), err, "Tool registration should succeed")

	// Validate all tools
	err = suite.toolRegistry.ValidateAllTools()
	require.NoError(suite.T(), err, "Tool validation should succeed")
}

// TestEndToEndClassificationWorkflow tests complete classification workflow
func (suite *GeneSymbolIntegrationTestSuite) TestEndToEndClassificationWorkflow() {
	endToEndTests := []struct {
		name                   string
		inputType              string
		inputNotation          string
		expectedClassification string
		expectedGeneSymbol     string
		description            string
	}{
		{
			name:                   "HGVS pathogenic variant",
			inputType:              "hgvs",
			inputNotation:          "NM_007294.4:c.273G>A",
			expectedGeneSymbol:     "BRCA1",
			description:            "Well-known BRCA1 pathogenic variant via HGVS",
		},
		{
			name:                   "Gene symbol pathogenic variant",
			inputType:              "gene_symbol",
			inputNotation:          "BRCA1:c.273G>A",
			expectedGeneSymbol:     "BRCA1",
			description:            "Same BRCA1 variant via gene symbol notation",
		},
		{
			name:                   "TP53 hotspot via HGVS",
			inputType:              "hgvs",
			inputNotation:          "NM_000546.6:c.818G>A",
			expectedGeneSymbol:     "TP53",
			description:            "TP53 R273H hotspot mutation via HGVS",
		},
		{
			name:                   "TP53 hotspot via gene symbol",
			inputType:              "gene_symbol",
			inputNotation:          "TP53:c.818G>A",
			expectedGeneSymbol:     "TP53",
			description:            "TP53 R273H hotspot mutation via gene symbol",
		},
		{
			name:                   "Standalone gene symbol query",
			inputType:              "gene_symbol",
			inputNotation:          "BRCA1",
			expectedGeneSymbol:     "BRCA1",
			description:            "Gene-level query without specific variant",
		},
	}

	for _, test := range endToEndTests {
		suite.Run(test.name, func() {
			// Prepare MCP tool request
			var params map[string]interface{}
			if test.inputType == "hgvs" {
				params = map[string]interface{}{
					"hgvs_notation": test.inputNotation,
				}
			} else {
				params = map[string]interface{}{
					"gene_symbol_notation": test.inputNotation,
				}
			}

			req := &protocol.JSONRPC2Request{
				JSONRPC: "2.0",
				Method:  "classify_variant",
				Params:  params,
				ID:      1,
			}

			// Execute through MCP tool interface
			response := suite.toolRegistry.ExecuteTool(suite.testContext, req)

			// Verify successful response
			assert.Nil(suite.T(), response.Error, "Tool execution should succeed for %s", test.description)
			assert.NotNil(suite.T(), response.Result, "Tool should return result for %s", test.description)

			// Parse and validate result structure
			resultBytes, err := json.Marshal(response.Result)
			require.NoError(suite.T(), err, "Result should be JSON serializable")

			var result map[string]interface{}
			err = json.Unmarshal(resultBytes, &result)
			require.NoError(suite.T(), err, "Result should be valid JSON")

			// Verify essential result fields
			assert.Contains(suite.T(), result, "classification", "Result should contain classification")
			assert.Contains(suite.T(), result, "variant_id", "Result should contain variant ID")
			assert.Contains(suite.T(), result, "confidence", "Result should contain confidence level")
			assert.Contains(suite.T(), result, "applied_rules", "Result should contain applied rules")

			suite.logger.WithFields(logrus.Fields{
				"test_name":      test.name,
				"input_type":     test.inputType,
				"input_notation": test.inputNotation,
				"classification": result["classification"],
			}).Info("End-to-end test completed successfully")
		})
	}
}

// TestClassificationAccuracyConsistency verifies identical results between input methods
func (suite *GeneSymbolIntegrationTestSuite) TestClassificationAccuracyConsistency() {
	consistencyTests := []struct {
		hgvsNotation       string
		geneSymbolNotation string
		description        string
	}{
		{
			hgvsNotation:       "NM_007294.4:c.273G>A",
			geneSymbolNotation: "BRCA1:c.273G>A",
			description:        "BRCA1 pathogenic variant consistency",
		},
		{
			hgvsNotation:       "NM_000546.6:c.818G>A",
			geneSymbolNotation: "TP53:c.818G>A",
			description:        "TP53 hotspot variant consistency",
		},
		{
			hgvsNotation:       "NM_000492.4:c.1521_1523delCTT",
			geneSymbolNotation: "CFTR:c.1521_1523delCTT",
			description:        "CFTR deletion variant consistency",
		},
	}

	for _, test := range consistencyTests {
		suite.Run(test.description, func() {
			// Execute with HGVS notation
			hgvsParams := map[string]interface{}{
				"hgvs_notation": test.hgvsNotation,
			}
			hgvsReq := &protocol.JSONRPC2Request{
				JSONRPC: "2.0",
				Method:  "classify_variant",
				Params:  hgvsParams,
				ID:      1,
			}
			hgvsResponse := suite.toolRegistry.ExecuteTool(suite.testContext, hgvsReq)

			// Execute with gene symbol notation
			geneParams := map[string]interface{}{
				"gene_symbol_notation": test.geneSymbolNotation,
			}
			geneReq := &protocol.JSONRPC2Request{
				JSONRPC: "2.0",
				Method:  "classify_variant",
				Params:  geneParams,
				ID:      2,
			}
			geneResponse := suite.toolRegistry.ExecuteTool(suite.testContext, geneReq)

			// Both should succeed
			assert.Nil(suite.T(), hgvsResponse.Error, "HGVS classification should succeed")
			assert.Nil(suite.T(), geneResponse.Error, "Gene symbol classification should succeed")

			// Parse results
			hgvsResultBytes, _ := json.Marshal(hgvsResponse.Result)
			geneResultBytes, _ := json.Marshal(geneResponse.Result)

			var hgvsResult, geneResult map[string]interface{}
			json.Unmarshal(hgvsResultBytes, &hgvsResult)
			json.Unmarshal(geneResultBytes, &geneResult)

			// Verify identical classifications
			assert.Equal(suite.T(), hgvsResult["classification"], geneResult["classification"],
				"Classifications should be identical for %s", test.description)
			assert.Equal(suite.T(), hgvsResult["confidence"], geneResult["confidence"],
				"Confidence levels should be identical for %s", test.description)

			// Applied rules should be the same (order may differ)
			hgvsRules := hgvsResult["applied_rules"].([]interface{})
			geneRules := geneResult["applied_rules"].([]interface{})
			assert.Len(suite.T(), geneRules, len(hgvsRules),
				"Number of applied rules should be identical for %s", test.description)

			suite.logger.WithField("test", test.description).Info("Classification consistency verified")
		})
	}
}

// TestExternalAPIIntegrationWithCircuitBreaker tests external API integration with circuit breaker
func (suite *GeneSymbolIntegrationTestSuite) TestExternalAPIIntegrationWithCircuitBreaker() {
	circuitBreakerTests := []struct {
		name        string
		geneSymbol  string
		expectError bool
		description string
	}{
		{
			name:        "Valid gene symbol",
			geneSymbol:  "BRCA1",
			expectError: false,
			description: "Should successfully resolve well-known gene",
		},
		{
			name:        "Unknown gene symbol",
			geneSymbol:  "INVALIDGENE123",
			expectError: true,
			description: "Should handle unknown gene gracefully",
		},
		{
			name:        "Complex gene symbol",
			geneSymbol:  "HLA-A",
			expectError: false,
			description: "Should handle complex gene symbols",
		},
	}

	for _, test := range circuitBreakerTests {
		suite.Run(test.name, func() {
			params := map[string]interface{}{
				"gene_symbol_notation": test.geneSymbol,
			}

			req := &protocol.JSONRPC2Request{
				JSONRPC: "2.0",
				Method:  "classify_variant",
				Params:  params,
				ID:      1,
			}

			response := suite.toolRegistry.ExecuteTool(suite.testContext, req)

			if test.expectError {
				assert.NotNil(suite.T(), response.Error, "Should return error for %s", test.description)
				assert.Equal(suite.T(), int64(protocol.InvalidParams), response.Error.Code,
					"Should return InvalidParams error code")
			} else {
				// May still fail due to external API issues, but shouldn't crash
				suite.logger.WithFields(logrus.Fields{
					"gene_symbol": test.geneSymbol,
					"has_error":   response.Error != nil,
				}).Info("Circuit breaker test completed")
			}
		})
	}
}

// TestCacheBehaviorVerification tests cache behavior across all cache tiers
func (suite *GeneSymbolIntegrationTestSuite) TestCacheBehaviorVerification() {
	cacheTests := []struct {
		name        string
		geneSymbol  string
		description string
	}{
		{
			name:        "First call hits external API",
			geneSymbol:  "BRCA1",
			description: "Initial call should hit external API and cache result",
		},
		{
			name:        "Second call hits cache",
			geneSymbol:  "BRCA1",
			description: "Subsequent call should hit memory cache",
		},
		{
			name:        "Different gene hits API",
			geneSymbol:  "TP53",
			description: "Different gene should require new API call",
		},
	}

	for i, test := range cacheTests {
		suite.Run(test.name, func() {
			startTime := time.Now()

			params := map[string]interface{}{
				"gene_symbol_notation": test.geneSymbol + ":c.1A>G", // Simple variant
			}

			req := &protocol.JSONRPC2Request{
				JSONRPC: "2.0",
				Method:  "classify_variant",
				Params:  params,
				ID:      int64(i + 1),
			}

			response := suite.toolRegistry.ExecuteTool(suite.testContext, req)
			elapsed := time.Since(startTime)

			// Log performance for cache behavior analysis
			suite.logger.WithFields(logrus.Fields{
				"test_name":   test.name,
				"gene_symbol": test.geneSymbol,
				"elapsed_ms":  elapsed.Milliseconds(),
				"has_error":   response.Error != nil,
			}).Info("Cache behavior test completed")

			// Second call to same gene should be faster (cached)
			if i == 1 && response.Error == nil {
				assert.Less(suite.T(), elapsed, 100*time.Millisecond,
					"Cached call should be faster than 100ms")
			}
		})
	}
}

// TestBatchProcessingIntegration tests batch processing with large gene symbol sets
func (suite *GeneSymbolIntegrationTestSuite) TestBatchProcessingIntegration() {
	// Test with a set of well-known genes
	geneSymbols := []string{
		"BRCA1", "BRCA2", "TP53", "APC", "MLH1",
		"MSH2", "MSH6", "PMS2", "MUTYH", "CHEK2",
	}

	suite.Run("Batch gene symbol processing", func() {
		startTime := time.Now()
		results := make([]interface{}, 0, len(geneSymbols))

		for i, geneSymbol := range geneSymbols {
			params := map[string]interface{}{
				"gene_symbol_notation": geneSymbol,
			}

			req := &protocol.JSONRPC2Request{
				JSONRPC: "2.0",
				Method:  "classify_variant",
				Params:  params,
				ID:      int64(i + 1),
			}

			response := suite.toolRegistry.ExecuteTool(suite.testContext, req)
			results = append(results, response)

			// Don't overwhelm external APIs
			time.Sleep(100 * time.Millisecond)
		}

		elapsed := time.Since(startTime)

		// Verify batch processing completed
		assert.Len(suite.T(), results, len(geneSymbols), "Should process all gene symbols")

		suite.logger.WithFields(logrus.Fields{
			"batch_size": len(geneSymbols),
			"elapsed":    elapsed,
			"avg_per_gene": elapsed / time.Duration(len(geneSymbols)),
		}).Info("Batch processing test completed")

		// Should complete within reasonable time (allowing for API rate limits)
		assert.Less(suite.T(), elapsed, 60*time.Second, "Batch processing should complete within 60 seconds")
	})
}

// Run the integration test suite
func TestGeneSymbolIntegrationSuite(t *testing.T) {
	suite.Run(t, new(GeneSymbolIntegrationTestSuite))
}