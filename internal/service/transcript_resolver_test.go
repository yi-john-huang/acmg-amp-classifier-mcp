package service

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/golang-lru"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/acmg-amp-mcp-server/pkg/external"
)

// MockExternalGeneAPI is a mock implementation of the ExternalGeneAPI interface
type MockExternalGeneAPI struct {
	mock.Mock
}

func (m *MockExternalGeneAPI) GetCanonicalTranscript(ctx context.Context, geneSymbol string) (*external.TranscriptInfo, error) {
	args := m.Called(ctx, geneSymbol)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*external.TranscriptInfo), args.Error(1)
}

func (m *MockExternalGeneAPI) ValidateGeneSymbol(ctx context.Context, geneSymbol string) (*external.GeneValidationResult, error) {
	args := m.Called(ctx, geneSymbol)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*external.GeneValidationResult), args.Error(1)
}

func (m *MockExternalGeneAPI) SearchGeneVariants(ctx context.Context, geneSymbol string) ([]*external.VariantInfo, error) {
	args := m.Called(ctx, geneSymbol)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*external.VariantInfo), args.Error(1)
}

func TestCachedTranscriptResolver_ResolveGeneToTranscript(t *testing.T) {
	ctx := context.Background()
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel) // Suppress logs during testing

	t.Run("Successful_Resolution", func(t *testing.T) {
		mockAPI := new(MockExternalGeneAPI)
		
		expectedTranscript := &external.TranscriptInfo{
			RefSeqID:       "NM_007294.3",
			GeneSymbol:     "BRCA1",
			TranscriptType: external.TranscriptTypeCanonical,
			Source:         external.ServiceTypeHGNC,
			LastUpdated:    time.Now(),
		}

		mockAPI.On("GetCanonicalTranscript", ctx, "BRCA1").Return(expectedTranscript, nil)

		resolver := createTestResolver(t, mockAPI, logger)

		// First call should hit external API
		transcript, err := resolver.ResolveGeneToTranscript(ctx, "BRCA1")
		require.NoError(t, err)
		assert.Equal(t, "NM_007294.3", transcript.RefSeqID)
		assert.Equal(t, "BRCA1", transcript.GeneSymbol)

		// Second call should hit memory cache
		transcript2, err := resolver.ResolveGeneToTranscript(ctx, "BRCA1")
		require.NoError(t, err)
		assert.Equal(t, "NM_007294.3", transcript2.RefSeqID)

		// Verify external API was called only once
		mockAPI.AssertNumberOfCalls(t, "GetCanonicalTranscript", 1)

		// Check cache stats
		stats := resolver.GetCacheStats()
		assert.Equal(t, int64(2), stats.TotalRequests)
		assert.Equal(t, int64(1), stats.MemoryHits)
		assert.Equal(t, int64(1), stats.ExternalCalls)
	})

	t.Run("Gene_Symbol_Normalization", func(t *testing.T) {
		mockAPI := new(MockExternalGeneAPI)
		
		expectedTranscript := &external.TranscriptInfo{
			RefSeqID:   "NM_000492.3",
			GeneSymbol: "CFTR",
			Source:     external.ServiceTypeHGNC,
		}

		// Mock should be called with normalized (uppercase) gene symbol
		mockAPI.On("GetCanonicalTranscript", ctx, "CFTR").Return(expectedTranscript, nil)

		resolver := createTestResolver(t, mockAPI, logger)

		// Test with lowercase input
		transcript, err := resolver.ResolveGeneToTranscript(ctx, "cftr")
		require.NoError(t, err)
		assert.Equal(t, "NM_000492.3", transcript.RefSeqID)
		assert.Equal(t, "CFTR", transcript.GeneSymbol)

		mockAPI.AssertExpectations(t)
	})

	t.Run("Empty_Gene_Symbol", func(t *testing.T) {
		mockAPI := new(MockExternalGeneAPI)
		resolver := createTestResolver(t, mockAPI, logger)

		// Test empty string
		_, err := resolver.ResolveGeneToTranscript(ctx, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "gene symbol cannot be empty")

		// Test whitespace only
		_, err = resolver.ResolveGeneToTranscript(ctx, "   ")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "gene symbol cannot be empty")

		// Verify no external API calls were made
		mockAPI.AssertNotCalled(t, "GetCanonicalTranscript")
	})

	t.Run("External_API_Error", func(t *testing.T) {
		mockAPI := new(MockExternalGeneAPI)
		
		mockAPI.On("GetCanonicalTranscript", ctx, "INVALID").Return(nil, assert.AnError)

		resolver := createTestResolver(t, mockAPI, logger)

		_, err := resolver.ResolveGeneToTranscript(ctx, "INVALID")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to resolve transcript")

		// Check error count in stats
		stats := resolver.GetCacheStats()
		assert.Equal(t, int64(1), stats.ErrorCount)
		assert.Equal(t, int64(1), stats.ExternalCalls)
	})
}

func TestCachedTranscriptResolver_GetCanonicalTranscript(t *testing.T) {
	ctx := context.Background()
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	mockAPI := new(MockExternalGeneAPI)
	
	expectedTranscript := &external.TranscriptInfo{
		RefSeqID:   "NM_000546.5",
		GeneSymbol: "TP53",
		Source:     external.ServiceTypeHGNC,
	}

	mockAPI.On("GetCanonicalTranscript", ctx, "TP53").Return(expectedTranscript, nil)

	resolver := createTestResolver(t, mockAPI, logger)

	refSeqID, err := resolver.GetCanonicalTranscript(ctx, "TP53")
	require.NoError(t, err)
	assert.Equal(t, "NM_000546.5", refSeqID)

	mockAPI.AssertExpectations(t)
}

func TestCachedTranscriptResolver_BatchResolve(t *testing.T) {
	ctx := context.Background()
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	mockAPI := new(MockExternalGeneAPI)
	
	// Setup mock responses for multiple genes
	mockAPI.On("GetCanonicalTranscript", ctx, "BRCA1").Return(&external.TranscriptInfo{
		RefSeqID: "NM_007294.3", GeneSymbol: "BRCA1", Source: external.ServiceTypeHGNC,
	}, nil)
	
	mockAPI.On("GetCanonicalTranscript", ctx, "TP53").Return(&external.TranscriptInfo{
		RefSeqID: "NM_000546.5", GeneSymbol: "TP53", Source: external.ServiceTypeHGNC,
	}, nil)
	
	mockAPI.On("GetCanonicalTranscript", ctx, "CFTR").Return(&external.TranscriptInfo{
		RefSeqID: "NM_000492.3", GeneSymbol: "CFTR", Source: external.ServiceTypeHGNC,
	}, nil)

	resolver := createTestResolver(t, mockAPI, logger)

	geneSymbols := []string{"BRCA1", "TP53", "CFTR"}
	results, err := resolver.BatchResolve(ctx, geneSymbols)
	
	require.NoError(t, err)
	assert.Len(t, results, 3)
	
	assert.Equal(t, "NM_007294.3", results["BRCA1"].RefSeqID)
	assert.Equal(t, "NM_000546.5", results["TP53"].RefSeqID)
	assert.Equal(t, "NM_000492.3", results["CFTR"].RefSeqID)

	// Verify all API calls were made
	mockAPI.AssertExpectations(t)
}

func TestCachedTranscriptResolver_BatchResolve_PartialSuccess(t *testing.T) {
	ctx := context.Background()
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	mockAPI := new(MockExternalGeneAPI)
	
	// Setup mock responses - one success, one failure
	mockAPI.On("GetCanonicalTranscript", ctx, "BRCA1").Return(&external.TranscriptInfo{
		RefSeqID: "NM_007294.3", GeneSymbol: "BRCA1", Source: external.ServiceTypeHGNC,
	}, nil)
	
	mockAPI.On("GetCanonicalTranscript", ctx, "INVALID").Return(nil, assert.AnError)

	resolver := createTestResolver(t, mockAPI, logger)

	geneSymbols := []string{"BRCA1", "INVALID"}
	results, err := resolver.BatchResolve(ctx, geneSymbols)
	
	// Should not return error for partial success
	require.NoError(t, err)
	assert.Len(t, results, 1) // Only successful result
	
	assert.Equal(t, "NM_007294.3", results["BRCA1"].RefSeqID)
	assert.NotContains(t, results, "INVALID")

	mockAPI.AssertExpectations(t)
}

func TestCachedTranscriptResolver_InvalidateCache(t *testing.T) {
	ctx := context.Background()
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	mockAPI := new(MockExternalGeneAPI)
	
	expectedTranscript := &external.TranscriptInfo{
		RefSeqID: "NM_007294.3", GeneSymbol: "BRCA1", Source: external.ServiceTypeHGNC,
	}

	mockAPI.On("GetCanonicalTranscript", ctx, "BRCA1").Return(expectedTranscript, nil)

	resolver := createTestResolver(t, mockAPI, logger)

	// First call - should hit external API and cache result
	transcript1, err := resolver.ResolveGeneToTranscript(ctx, "BRCA1")
	require.NoError(t, err)
	assert.Equal(t, "NM_007294.3", transcript1.RefSeqID)

	// Invalidate cache
	err = resolver.InvalidateCache("BRCA1")
	require.NoError(t, err)

	// Second call - should hit external API again due to cache invalidation
	transcript2, err := resolver.ResolveGeneToTranscript(ctx, "BRCA1")
	require.NoError(t, err)
	assert.Equal(t, "NM_007294.3", transcript2.RefSeqID)

	// Verify external API was called twice
	mockAPI.AssertNumberOfCalls(t, "GetCanonicalTranscript", 2)
}

func TestCachedTranscriptResolver_GetCacheStats(t *testing.T) {
	ctx := context.Background()
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	mockAPI := new(MockExternalGeneAPI)
	
	expectedTranscript := &external.TranscriptInfo{
		RefSeqID: "NM_007294.3", GeneSymbol: "BRCA1", Source: external.ServiceTypeHGNC,
	}

	mockAPI.On("GetCanonicalTranscript", ctx, "BRCA1").Return(expectedTranscript, nil)

	resolver := createTestResolver(t, mockAPI, logger)

	// Make some requests to generate stats
	resolver.ResolveGeneToTranscript(ctx, "BRCA1") // External call
	resolver.ResolveGeneToTranscript(ctx, "BRCA1") // Memory hit

	stats := resolver.GetCacheStats()
	
	assert.Equal(t, int64(2), stats.TotalRequests)
	assert.Equal(t, int64(1), stats.MemoryHits)
	assert.Equal(t, int64(1), stats.MemoryMisses)
	assert.Equal(t, int64(1), stats.ExternalCalls)
	assert.Equal(t, int64(0), stats.ErrorCount)
	assert.False(t, stats.LastReset.IsZero())
}

func BenchmarkCachedTranscriptResolver_ResolveGeneToTranscript(b *testing.B) {
	ctx := context.Background()
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	mockAPI := new(MockExternalGeneAPI)
	
	expectedTranscript := &external.TranscriptInfo{
		RefSeqID: "NM_007294.3", GeneSymbol: "BRCA1", Source: external.ServiceTypeHGNC,
	}

	mockAPI.On("GetCanonicalTranscript", ctx, "BRCA1").Return(expectedTranscript, nil)

	resolver := createTestResolver(b, mockAPI, logger)

	// Prime the cache
	resolver.ResolveGeneToTranscript(ctx, "BRCA1")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			resolver.ResolveGeneToTranscript(ctx, "BRCA1")
		}
	})
}

// Helper function to create a test resolver
func createTestResolver(t testing.TB, mockAPI external.ExternalGeneAPI, logger *logrus.Logger) *CachedTranscriptResolver {
	// Create a minimal resolver for testing (without Redis cache)
	resolver := &CachedTranscriptResolver{
		externalAPI:     mockAPI,
		memoryCacheTTL:  15 * time.Minute,
		redisCacheTTL:   24 * time.Hour,
		maxMemorySize:   100,
		maxConcurrency:  5,
		batchSemaphore:  make(chan struct{}, 5),
		logger:          logger,
		stats: &CacheStats{
			LastReset: time.Now(),
		},
	}

	// Create in-memory cache
	var err error
	resolver.memoryCache, err = lru.New(resolver.maxMemorySize)
	require.NoError(t, err)

	return resolver
}