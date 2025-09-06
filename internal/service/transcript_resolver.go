package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/golang-lru"
	"github.com/sirupsen/logrus"

	"github.com/acmg-amp-mcp-server/internal/mcp/caching"
	"github.com/acmg-amp-mcp-server/pkg/external"
)

// TranscriptResolver defines the interface for resolving gene symbols to RefSeq transcripts
type TranscriptResolver interface {
	// ResolveGeneToTranscript resolves a gene symbol to its canonical transcript
	ResolveGeneToTranscript(ctx context.Context, geneSymbol string) (*external.TranscriptInfo, error)
	
	// GetCanonicalTranscript gets the canonical transcript for a gene symbol
	GetCanonicalTranscript(ctx context.Context, geneSymbol string) (string, error)
	
	// GetAllTranscripts gets all available transcripts for a gene symbol
	GetAllTranscripts(ctx context.Context, geneSymbol string) ([]external.TranscriptInfo, error)
	
	// BatchResolve resolves multiple gene symbols concurrently
	BatchResolve(ctx context.Context, geneSymbols []string) (map[string]*external.TranscriptInfo, error)
	
	// InvalidateCache invalidates cached transcript information for a gene symbol
	InvalidateCache(geneSymbol string) error
	
	// GetCacheStats returns cache performance statistics
	GetCacheStats() CacheStats
}

// CachedTranscriptResolver implements the TranscriptResolver interface with multi-level caching
type CachedTranscriptResolver struct {
	// External API client with failover support
	externalAPI external.ExternalGeneAPI
	
	// Multi-level caching
	memoryCache *lru.Cache // Tier 1: In-memory LRU cache (hot data)
	redisCache  *caching.ToolResultCache // Tier 2: Redis distributed cache (warm data)
	
	// Caching configuration
	memoryCacheTTL time.Duration
	redisCacheTTL  time.Duration
	maxMemorySize  int
	
	// Concurrency control
	batchSemaphore chan struct{} // Limits concurrent external API calls
	maxConcurrency int
	
	// Metrics and logging
	logger *logrus.Logger
	stats  *CacheStats
	statsMu sync.RWMutex
}

// CacheStats represents cache performance statistics
type CacheStats struct {
	MemoryHits     int64 `json:"memory_hits"`
	MemoryMisses   int64 `json:"memory_misses"`
	RedisHits      int64 `json:"redis_hits"`
	RedisMisses    int64 `json:"redis_misses"`
	ExternalCalls  int64 `json:"external_calls"`
	TotalRequests  int64 `json:"total_requests"`
	ErrorCount     int64 `json:"error_count"`
	LastReset      time.Time `json:"last_reset"`
}

// TranscriptResolverConfig represents configuration for the transcript resolver
type TranscriptResolverConfig struct {
	// Cache configuration
	MemoryCacheTTL time.Duration `json:"memory_cache_ttl"`
	RedisCacheTTL  time.Duration `json:"redis_cache_ttl"`
	MaxMemorySize  int           `json:"max_memory_size"`
	
	// Concurrency configuration
	MaxConcurrency int `json:"max_concurrency"`
	
	// External API configuration
	ExternalAPIConfig external.UnifiedGeneAPIConfig `json:"external_api"`
}

// NewCachedTranscriptResolver creates a new cached transcript resolver
func NewCachedTranscriptResolver(
	config TranscriptResolverConfig,
	redisCache *caching.ToolResultCache,
	logger *logrus.Logger,
) (*CachedTranscriptResolver, error) {
	// Set default configuration values
	if config.MemoryCacheTTL == 0 {
		config.MemoryCacheTTL = 15 * time.Minute
	}
	if config.RedisCacheTTL == 0 {
		config.RedisCacheTTL = 24 * time.Hour
	}
	if config.MaxMemorySize == 0 {
		config.MaxMemorySize = 1000 // 1000 entries
	}
	if config.MaxConcurrency == 0 {
		config.MaxConcurrency = 5 // Max 5 concurrent external API calls
	}

	// Create in-memory LRU cache
	memoryCache, err := lru.New(config.MaxMemorySize)
	if err != nil {
		return nil, fmt.Errorf("failed to create memory cache: %w", err)
	}

	// Create external API client
	externalAPI := external.NewUnifiedGeneAPIClient(config.ExternalAPIConfig, logger)

	// Create concurrency control semaphore
	batchSemaphore := make(chan struct{}, config.MaxConcurrency)

	return &CachedTranscriptResolver{
		externalAPI:     externalAPI,
		memoryCache:     memoryCache,
		redisCache:      redisCache,
		memoryCacheTTL:  config.MemoryCacheTTL,
		redisCacheTTL:   config.RedisCacheTTL,
		maxMemorySize:   config.MaxMemorySize,
		batchSemaphore:  batchSemaphore,
		maxConcurrency:  config.MaxConcurrency,
		logger:          logger,
		stats: &CacheStats{
			LastReset: time.Now(),
		},
	}, nil
}

// ResolveGeneToTranscript resolves a gene symbol to its canonical transcript with caching
func (r *CachedTranscriptResolver) ResolveGeneToTranscript(ctx context.Context, geneSymbol string) (*external.TranscriptInfo, error) {
	r.incrementStat("total_requests")
	
	// Normalize gene symbol
	geneSymbol = normalizeGeneSymbol(geneSymbol)
	if geneSymbol == "" {
		r.incrementStat("error_count")
		return nil, fmt.Errorf("gene symbol cannot be empty")
	}

	r.logger.WithField("gene_symbol", geneSymbol).Debug("Resolving gene symbol to transcript")

	// Try memory cache first (Tier 1)
	if transcript := r.getFromMemoryCache(geneSymbol); transcript != nil {
		r.incrementStat("memory_hits")
		r.logger.WithFields(logrus.Fields{
			"gene_symbol": geneSymbol,
			"transcript":  transcript.RefSeqID,
			"cache_tier":  "memory",
		}).Debug("Cache hit in memory")
		return transcript, nil
	}
	r.incrementStat("memory_misses")

	// Try Redis cache (Tier 2)
	if transcript := r.getFromRedisCache(ctx, geneSymbol); transcript != nil {
		r.incrementStat("redis_hits")
		r.logger.WithFields(logrus.Fields{
			"gene_symbol": geneSymbol,
			"transcript":  transcript.RefSeqID,
			"cache_tier":  "redis",
		}).Debug("Cache hit in Redis")
		
		// Populate memory cache for next time
		r.setInMemoryCache(geneSymbol, transcript)
		return transcript, nil
	}
	r.incrementStat("redis_misses")

	// Cache miss - fetch from external API
	r.incrementStat("external_calls")
	transcript, err := r.fetchFromExternalAPI(ctx, geneSymbol)
	if err != nil {
		r.incrementStat("error_count")
		return nil, fmt.Errorf("failed to resolve transcript for gene %s: %w", geneSymbol, err)
	}

	// Cache the result in both tiers
	r.setInMemoryCache(geneSymbol, transcript)
	r.setInRedisCache(ctx, geneSymbol, transcript)

	r.logger.WithFields(logrus.Fields{
		"gene_symbol": geneSymbol,
		"transcript":  transcript.RefSeqID,
		"source":      transcript.Source,
	}).Info("Successfully resolved transcript from external API")

	return transcript, nil
}

// GetCanonicalTranscript gets the canonical transcript RefSeq ID for a gene symbol
func (r *CachedTranscriptResolver) GetCanonicalTranscript(ctx context.Context, geneSymbol string) (string, error) {
	transcript, err := r.ResolveGeneToTranscript(ctx, geneSymbol)
	if err != nil {
		return "", err
	}
	return transcript.RefSeqID, nil
}

// GetAllTranscripts gets all available transcripts for a gene symbol
func (r *CachedTranscriptResolver) GetAllTranscripts(ctx context.Context, geneSymbol string) ([]external.TranscriptInfo, error) {
	// For now, we only return the canonical transcript
	// In a full implementation, this would query all available transcripts
	transcript, err := r.ResolveGeneToTranscript(ctx, geneSymbol)
	if err != nil {
		return nil, err
	}
	return []external.TranscriptInfo{*transcript}, nil
}

// BatchResolve resolves multiple gene symbols concurrently with controlled concurrency
func (r *CachedTranscriptResolver) BatchResolve(ctx context.Context, geneSymbols []string) (map[string]*external.TranscriptInfo, error) {
	if len(geneSymbols) == 0 {
		return make(map[string]*external.TranscriptInfo), nil
	}

	results := make(map[string]*external.TranscriptInfo)
	errors := make(map[string]error)
	var wg sync.WaitGroup
	var mu sync.Mutex

	r.logger.WithField("batch_size", len(geneSymbols)).Info("Starting batch transcript resolution")

	for _, symbol := range geneSymbols {
		wg.Add(1)
		go func(geneSymbol string) {
			defer wg.Done()

			// Acquire semaphore to limit concurrency
			select {
			case r.batchSemaphore <- struct{}{}:
				defer func() { <-r.batchSemaphore }()
			case <-ctx.Done():
				mu.Lock()
				errors[geneSymbol] = ctx.Err()
				mu.Unlock()
				return
			}

			transcript, err := r.ResolveGeneToTranscript(ctx, geneSymbol)
			
			mu.Lock()
			if err != nil {
				errors[geneSymbol] = err
			} else {
				results[geneSymbol] = transcript
			}
			mu.Unlock()
		}(symbol)
	}

	wg.Wait()

	// Log batch results
	r.logger.WithFields(logrus.Fields{
		"batch_size":      len(geneSymbols),
		"successful":      len(results),
		"failed":         len(errors),
	}).Info("Completed batch transcript resolution")

	// If we have partial success, return the results we have
	// Clients can check the results map for which symbols were successful
	return results, nil
}

// InvalidateCache invalidates cached transcript information for a gene symbol
func (r *CachedTranscriptResolver) InvalidateCache(geneSymbol string) error {
	geneSymbol = normalizeGeneSymbol(geneSymbol)
	if geneSymbol == "" {
		return fmt.Errorf("gene symbol cannot be empty")
	}

	// Remove from memory cache
	r.memoryCache.Remove(geneSymbol)

	// Remove from Redis cache
	// Note: This assumes the Redis cache has a method to remove specific keys
	// The actual implementation would depend on the cache interface
	r.logger.WithField("gene_symbol", geneSymbol).Info("Invalidated cache for gene symbol")

	return nil
}

// GetCacheStats returns cache performance statistics
func (r *CachedTranscriptResolver) GetCacheStats() CacheStats {
	r.statsMu.RLock()
	defer r.statsMu.RUnlock()
	
	stats := *r.stats
	stats.LastReset = r.stats.LastReset
	
	// Calculate hit ratios
	totalMemoryRequests := stats.MemoryHits + stats.MemoryMisses
	totalRedisRequests := stats.RedisHits + stats.RedisMisses
	
	r.logger.WithFields(logrus.Fields{
		"total_requests":      stats.TotalRequests,
		"memory_hit_ratio":    fmt.Sprintf("%.2f%%", float64(stats.MemoryHits)/float64(totalMemoryRequests)*100),
		"redis_hit_ratio":     fmt.Sprintf("%.2f%%", float64(stats.RedisHits)/float64(totalRedisRequests)*100),
		"external_calls":      stats.ExternalCalls,
		"error_count":        stats.ErrorCount,
	}).Debug("Cache statistics")
	
	return stats
}

// Private helper methods

func (r *CachedTranscriptResolver) getFromMemoryCache(geneSymbol string) *external.TranscriptInfo {
	if value, ok := r.memoryCache.Get(geneSymbol); ok {
		if entry, ok := value.(*cacheEntry); ok && !entry.isExpired() {
			return entry.transcript
		}
		// Remove expired entry
		r.memoryCache.Remove(geneSymbol)
	}
	return nil
}

func (r *CachedTranscriptResolver) getFromRedisCache(ctx context.Context, geneSymbol string) *external.TranscriptInfo {
	// This would use the actual Redis cache implementation
	// For now, returning nil as placeholder
	return nil
}

func (r *CachedTranscriptResolver) setInMemoryCache(geneSymbol string, transcript *external.TranscriptInfo) {
	entry := &cacheEntry{
		transcript: transcript,
		expiry:     time.Now().Add(r.memoryCacheTTL),
	}
	r.memoryCache.Add(geneSymbol, entry)
}

func (r *CachedTranscriptResolver) setInRedisCache(ctx context.Context, geneSymbol string, transcript *external.TranscriptInfo) {
	// This would use the actual Redis cache implementation
	// For now, this is a placeholder
}

func (r *CachedTranscriptResolver) fetchFromExternalAPI(ctx context.Context, geneSymbol string) (*external.TranscriptInfo, error) {
	return r.externalAPI.GetCanonicalTranscript(ctx, geneSymbol)
}

func (r *CachedTranscriptResolver) incrementStat(statName string) {
	r.statsMu.Lock()
	defer r.statsMu.Unlock()
	
	switch statName {
	case "memory_hits":
		r.stats.MemoryHits++
	case "memory_misses":
		r.stats.MemoryMisses++
	case "redis_hits":
		r.stats.RedisHits++
	case "redis_misses":
		r.stats.RedisMisses++
	case "external_calls":
		r.stats.ExternalCalls++
	case "total_requests":
		r.stats.TotalRequests++
	case "error_count":
		r.stats.ErrorCount++
	}
}

// Helper types

type cacheEntry struct {
	transcript *external.TranscriptInfo
	expiry     time.Time
}

func (e *cacheEntry) isExpired() bool {
	return time.Now().After(e.expiry)
}

// Helper functions

func normalizeGeneSymbol(geneSymbol string) string {
	// Normalize gene symbol to uppercase and trim whitespace
	return strings.TrimSpace(strings.ToUpper(geneSymbol))
}