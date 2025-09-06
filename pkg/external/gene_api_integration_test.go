package external

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGeneAPIIntegration tests the integration with real external APIs
// These tests can be disabled with build tags or environment variables for CI
func TestGeneAPIIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	ctx := context.Background()
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	// Test data - well-known genes that should exist in all databases
	testGenes := []string{"BRCA1", "TP53", "CFTR"}

	t.Run("HGNC_Integration", func(t *testing.T) {
		client := NewHGNCClient(HGNCConfig{
			RateLimit: 1, // Slower for integration tests
		})

		for _, gene := range testGenes {
			t.Run(gene, func(t *testing.T) {
				// Test gene validation
				validation, err := client.ValidateGeneSymbol(ctx, gene)
				require.NoError(t, err)
				assert.True(t, validation.IsValid, "Gene %s should be valid", gene)
				assert.Equal(t, ServiceTypeHGNC, validation.Source)

				// Test transcript retrieval
				transcript, err := client.GetCanonicalTranscript(ctx, gene)
				require.NoError(t, err)
				assert.NotEmpty(t, transcript.RefSeqID, "Should have RefSeq ID")
				assert.Equal(t, gene, transcript.GeneSymbol)
				assert.Equal(t, ServiceTypeHGNC, transcript.Source)
				assert.NotEmpty(t, transcript.Metadata.HGNCID)

				t.Logf("Gene %s -> RefSeq %s (HGNC ID: %s)", 
					gene, transcript.RefSeqID, transcript.Metadata.HGNCID)
			})
		}
	})

	t.Run("RefSeq_Integration", func(t *testing.T) {
		client := NewRefSeqClient(RefSeqConfig{
			RateLimit: 1, // Slower for integration tests
		})

		for _, gene := range testGenes {
			t.Run(gene, func(t *testing.T) {
				// Test gene validation
				validation, err := client.ValidateGeneSymbol(ctx, gene)
				require.NoError(t, err)
				assert.Equal(t, ServiceTypeRefSeq, validation.Source)

				// Test transcript retrieval
				transcript, err := client.GetCanonicalTranscript(ctx, gene)
				require.NoError(t, err)
				assert.NotEmpty(t, transcript.RefSeqID, "Should have RefSeq ID")
				assert.Equal(t, gene, transcript.GeneSymbol)
				assert.Equal(t, ServiceTypeRefSeq, transcript.Source)
				assert.Greater(t, transcript.Length, 0, "Should have positive length")

				t.Logf("Gene %s -> RefSeq %s (Length: %d)", 
					gene, transcript.RefSeqID, transcript.Length)
			})
		}
	})

	t.Run("Ensembl_Integration", func(t *testing.T) {
		client := NewEnsemblClient(EnsemblConfig{
			RateLimit: 5, // Ensembl allows higher rate
		})

		for _, gene := range testGenes {
			t.Run(gene, func(t *testing.T) {
				// Test gene validation
				validation, err := client.ValidateGeneSymbol(ctx, gene)
				require.NoError(t, err)
				assert.True(t, validation.IsValid, "Gene %s should be valid", gene)
				assert.Equal(t, ServiceTypeEnsembl, validation.Source)

				// Test transcript retrieval
				transcript, err := client.GetCanonicalTranscript(ctx, gene)
				require.NoError(t, err)
				assert.Equal(t, gene, transcript.GeneSymbol)
				assert.Equal(t, ServiceTypeEnsembl, transcript.Source)
				assert.Greater(t, transcript.Length, 0, "Should have positive length")
				assert.NotEmpty(t, transcript.Metadata.GenomicCoordinates)

				t.Logf("Gene %s -> Ensembl transcript (Length: %d, RefSeq: %s)", 
					gene, transcript.Length, transcript.RefSeqID)
			})
		}
	})

	t.Run("UnifiedAPI_Integration", func(t *testing.T) {
		config := UnifiedGeneAPIConfig{
			HGNCConfig:    HGNCConfig{RateLimit: 1},
			RefSeqConfig:  RefSeqConfig{RateLimit: 1},
			EnsemblConfig: EnsemblConfig{RateLimit: 5},
			CircuitBreaker: CircuitBreakerConfig{
				MaxRequests:      3,
				Interval:         10 * time.Second,
				Timeout:          5 * time.Second,
				FailureThreshold: 3,
			},
		}

		client := NewUnifiedGeneAPIClient(config, logger)

		for _, gene := range testGenes {
			t.Run(gene, func(t *testing.T) {
				// Test unified transcript retrieval
				transcript, err := client.GetCanonicalTranscript(ctx, gene)
				require.NoError(t, err)
				assert.Equal(t, gene, transcript.GeneSymbol)
				assert.NotEmpty(t, transcript.RefSeqID, "Should have RefSeq ID")

				// Test unified validation
				validation, err := client.ValidateGeneSymbol(ctx, gene)
				require.NoError(t, err)
				assert.True(t, validation.IsValid, "Gene %s should be valid", gene)

				t.Logf("Unified API - Gene %s -> RefSeq %s (Source: %s)", 
					gene, transcript.RefSeqID, transcript.Source)
			})
		}

		// Test service health
		health := client.GetServiceHealth(ctx)
		assert.Len(t, health, 3, "Should check 3 services")
		
		for _, h := range health {
			t.Logf("Service %s health: %v (Error: %s)", h.Service, h.Healthy, h.Error)
		}
	})

	t.Run("Invalid_Gene_Handling", func(t *testing.T) {
		client := NewHGNCClient(HGNCConfig{RateLimit: 1})

		// Test invalid gene symbol
		validation, err := client.ValidateGeneSymbol(ctx, "INVALIDGENE123")
		require.NoError(t, err)
		assert.False(t, validation.IsValid)
		assert.NotEmpty(t, validation.ValidationErrors)

		// Test transcript retrieval for invalid gene
		_, err = client.GetCanonicalTranscript(ctx, "INVALIDGENE123")
		assert.Error(t, err, "Should fail for invalid gene")
	})

	t.Run("Rate_Limiting", func(t *testing.T) {
		client := NewHGNCClient(HGNCConfig{RateLimit: 1}) // 1 request per second

		start := time.Now()
		
		// Make 3 requests - should take at least 2 seconds due to rate limiting
		for i := 0; i < 3; i++ {
			_, err := client.ValidateGeneSymbol(ctx, "BRCA1")
			require.NoError(t, err)
		}
		
		elapsed := time.Since(start)
		assert.GreaterOrEqual(t, elapsed, 2*time.Second, "Rate limiting should enforce delays")
		
		t.Logf("3 requests took %v (rate limited to 1/sec)", elapsed)
	})
}

// BenchmarkGeneAPIPerformance benchmarks the performance of gene API clients
func BenchmarkGeneAPIPerformance(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping benchmarks in short mode")
	}

	ctx := context.Background()
	testGene := "BRCA1"

	b.Run("HGNC_Transcript_Lookup", func(b *testing.B) {
		client := NewHGNCClient(HGNCConfig{RateLimit: 10})
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_, err := client.GetCanonicalTranscript(ctx, testGene)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("RefSeq_Transcript_Lookup", func(b *testing.B) {
		client := NewRefSeqClient(RefSeqConfig{RateLimit: 10})
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_, err := client.GetCanonicalTranscript(ctx, testGene)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("Ensembl_Transcript_Lookup", func(b *testing.B) {
		client := NewEnsemblClient(EnsemblConfig{RateLimit: 15})
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_, err := client.GetCanonicalTranscript(ctx, testGene)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("Unified_API_Transcript_Lookup", func(b *testing.B) {
		config := UnifiedGeneAPIConfig{
			HGNCConfig:    HGNCConfig{RateLimit: 10},
			RefSeqConfig:  RefSeqConfig{RateLimit: 10},
			EnsemblConfig: EnsemblConfig{RateLimit: 15},
		}
		client := NewUnifiedGeneAPIClient(config, logrus.New())
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_, err := client.GetCanonicalTranscript(ctx, testGene)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}