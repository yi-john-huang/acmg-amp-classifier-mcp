package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	
	"github.com/acmg-amp-mcp-server/internal/mcp/protocol"
)

// EvidenceCache provides caching for external database queries
type EvidenceCache struct {
	logger *logrus.Logger
	cache  map[string]*CacheEntry
	mutex  sync.RWMutex
}

// CacheEntry represents a cached evidence result
type CacheEntry struct {
	Data      *QueryEvidenceResult
	Timestamp time.Time
	AccessCount int
	LastAccess  time.Time
}

// NewEvidenceCache creates a new evidence cache
func NewEvidenceCache(logger *logrus.Logger) *EvidenceCache {
	cache := &EvidenceCache{
		logger: logger,
		cache:  make(map[string]*CacheEntry),
	}

	// Start cache cleanup routine
	go cache.cleanupRoutine()

	return cache
}

// Get retrieves cached evidence if available and not expired
func (c *EvidenceCache) Get(key string, maxAge string) *QueryEvidenceResult {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	entry, exists := c.cache[key]
	if !exists {
		return nil
	}

	// Parse max age duration
	maxAgeDuration, err := time.ParseDuration(maxAge)
	if err != nil {
		c.logger.WithError(err).Warn("Failed to parse cache max age, using default")
		maxAgeDuration = 24 * time.Hour
	}

	// Check if entry is expired
	if time.Since(entry.Timestamp) > maxAgeDuration {
		c.logger.WithField("key", key).Debug("Cache entry expired")
		return nil
	}

	// Update access statistics
	entry.AccessCount++
	entry.LastAccess = time.Now()

	c.logger.WithFields(logrus.Fields{
		"key":          key,
		"access_count": entry.AccessCount,
		"age":          time.Since(entry.Timestamp).String(),
	}).Debug("Cache hit")

	return entry.Data
}

// Set stores evidence result in cache
func (c *EvidenceCache) Set(key string, data *QueryEvidenceResult) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	entry := &CacheEntry{
		Data:        data,
		Timestamp:   time.Now(),
		AccessCount: 1,
		LastAccess:  time.Now(),
	}

	c.cache[key] = entry

	c.logger.WithFields(logrus.Fields{
		"key":         key,
		"cache_size":  len(c.cache),
	}).Debug("Cached evidence result")
}

// Delete removes an entry from cache
func (c *EvidenceCache) Delete(key string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if _, exists := c.cache[key]; exists {
		delete(c.cache, key)
		c.logger.WithField("key", key).Debug("Deleted cache entry")
	}
}

// Clear removes all entries from cache
func (c *EvidenceCache) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	oldSize := len(c.cache)
	c.cache = make(map[string]*CacheEntry)
	
	c.logger.WithField("cleared_entries", oldSize).Info("Cleared evidence cache")
}

// GetStats returns cache statistics
func (c *EvidenceCache) GetStats() map[string]interface{} {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	totalAccess := 0
	oldestEntry := time.Now()
	newestEntry := time.Time{}

	for _, entry := range c.cache {
		totalAccess += entry.AccessCount
		if entry.Timestamp.Before(oldestEntry) {
			oldestEntry = entry.Timestamp
		}
		if entry.Timestamp.After(newestEntry) {
			newestEntry = entry.Timestamp
		}
	}

	stats := map[string]interface{}{
		"total_entries":   len(c.cache),
		"total_accesses":  totalAccess,
	}

	if len(c.cache) > 0 {
		stats["oldest_entry_age"] = time.Since(oldestEntry).String()
		stats["newest_entry_age"] = time.Since(newestEntry).String()
		stats["average_accesses"] = float64(totalAccess) / float64(len(c.cache))
	}

	return stats
}

// cleanupRoutine runs periodically to remove expired entries
func (c *EvidenceCache) cleanupRoutine() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		c.cleanup()
	}
}

// cleanup removes expired and infrequently accessed entries
func (c *EvidenceCache) cleanup() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	now := time.Now()
	maxAge := 48 * time.Hour // Default max age for cleanup
	minAccess := 2           // Minimum access count to retain
	
	keysToDelete := make([]string, 0)

	for key, entry := range c.cache {
		// Remove if too old or infrequently accessed
		age := now.Sub(entry.Timestamp)
		lastAccessAge := now.Sub(entry.LastAccess)
		
		if age > maxAge || (entry.AccessCount < minAccess && lastAccessAge > 6*time.Hour) {
			keysToDelete = append(keysToDelete, key)
		}
	}

	// Delete identified entries
	for _, key := range keysToDelete {
		delete(c.cache, key)
	}

	if len(keysToDelete) > 0 {
		c.logger.WithFields(logrus.Fields{
			"deleted_entries":  len(keysToDelete),
			"remaining_entries": len(c.cache),
		}).Info("Cache cleanup completed")
	}
}

// BatchEvidenceTool implements batch evidence gathering for multiple variants
type BatchEvidenceTool struct {
	logger        *logrus.Logger
	evidenceTool  *QueryEvidenceTool
	maxBatchSize  int
	maxConcurrent int
}

// BatchEvidenceParams defines parameters for batch evidence queries
type BatchEvidenceParams struct {
	Variants        []VariantQuery `json:"variants" validate:"required"`
	Databases       []string       `json:"databases,omitempty"`
	IncludeRaw      bool           `json:"include_raw,omitempty"`
	MaxAge          string         `json:"max_age,omitempty"`
	MaxConcurrent   int            `json:"max_concurrent,omitempty"`
}

// VariantQuery represents a single variant to query
type VariantQuery struct {
	VariantID    string `json:"variant_id,omitempty"`
	HGVSNotation string `json:"hgvs_notation" validate:"required"`
	GeneSymbol   string `json:"gene_symbol,omitempty"`
	Priority     int    `json:"priority,omitempty"` // Higher numbers = higher priority
}

// BatchEvidenceResult contains results for multiple variants
type BatchEvidenceResult struct {
	TotalVariants    int                              `json:"total_variants"`
	ProcessedVariants int                             `json:"processed_variants"`
	FailedVariants   int                              `json:"failed_variants"`
	ProcessingTime   string                           `json:"processing_time"`
	Results          map[string]*QueryEvidenceResult  `json:"results"`
	Errors           map[string]string                `json:"errors"`
	BatchStats       BatchProcessingStats             `json:"batch_stats"`
}

// BatchProcessingStats contains batch processing statistics
type BatchProcessingStats struct {
	AverageProcessingTime string                 `json:"average_processing_time"`
	CacheHitRate         float64                `json:"cache_hit_rate"`
	DatabaseQueryCounts  map[string]int         `json:"database_query_counts"`
	QualityDistribution  map[string]int         `json:"quality_distribution"`
}

// NewBatchEvidenceTool creates a new batch evidence tool
func NewBatchEvidenceTool(logger *logrus.Logger) *BatchEvidenceTool {
	return &BatchEvidenceTool{
		logger:        logger,
		evidenceTool:  NewQueryEvidenceTool(logger),
		maxBatchSize:  100,
		maxConcurrent: 10,
	}
}

// HandleTool implements the ToolHandler interface for batch_query_evidence
func (t *BatchEvidenceTool) HandleTool(ctx context.Context, req *protocol.JSONRPC2Request) *protocol.JSONRPC2Response {
	startTime := time.Now()
	t.logger.WithField("tool", "batch_query_evidence").Info("Processing batch evidence query")

	// Parse and validate parameters
	var params BatchEvidenceParams
	if err := t.parseAndValidateParams(req.Params, &params); err != nil {
		return &protocol.JSONRPC2Response{
			Error: &protocol.RPCError{
				Code:    protocol.InvalidParams,
				Message: "Invalid parameters",
				Data:    err.Error(),
			},
		}
	}

	// Validate batch size
	if len(params.Variants) > t.maxBatchSize {
		return &protocol.JSONRPC2Response{
			Error: &protocol.RPCError{
				Code:    protocol.InvalidParams,
				Message: "Batch size too large",
				Data:    fmt.Sprintf("Maximum batch size is %d, received %d", t.maxBatchSize, len(params.Variants)),
			},
		}
	}

	// Process variants in batch
	result := t.processBatch(ctx, &params)
	result.ProcessingTime = time.Since(startTime).String()

	t.logger.WithFields(logrus.Fields{
		"total_variants":     result.TotalVariants,
		"processed_variants": result.ProcessedVariants,
		"failed_variants":    result.FailedVariants,
		"processing_time":    result.ProcessingTime,
	}).Info("Batch evidence query completed")

	return &protocol.JSONRPC2Response{
		Result: map[string]interface{}{
			"batch_evidence": result,
		},
	}
}

// GetToolInfo returns tool metadata for batch evidence queries
func (t *BatchEvidenceTool) GetToolInfo() protocol.ToolInfo {
	return protocol.ToolInfo{
		Name:        "batch_query_evidence",
		Description: "Query evidence for multiple variants simultaneously with optimized batching and caching",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"variants": map[string]interface{}{
					"type":        "array",
					"description": "Array of variants to query",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"hgvs_notation": map[string]interface{}{
								"type": "string",
								"description": "HGVS notation of the variant",
							},
							"variant_id": map[string]interface{}{
								"type": "string",
								"description": "Optional variant identifier",
							},
							"gene_symbol": map[string]interface{}{
								"type": "string",
								"description": "Optional gene symbol",
							},
							"priority": map[string]interface{}{
								"type": "integer",
								"description": "Processing priority (higher numbers first)",
								"default": 1,
							},
						},
						"required": []string{"hgvs_notation"},
					},
					"maxItems": 100,
				},
				"databases": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "string",
						"enum": []string{"clinvar", "gnomad", "cosmic", "lovd", "hgmd", "pubmed"},
					},
				},
				"max_concurrent": map[string]interface{}{
					"type": "integer",
					"description": "Maximum concurrent database queries",
					"default": 10,
					"maximum": 20,
				},
			},
			"required": []string{"variants"},
		},
	}
}

// ValidateParams validates batch tool parameters
func (t *BatchEvidenceTool) ValidateParams(params interface{}) error {
	var batchParams BatchEvidenceParams
	return t.parseAndValidateParams(params, &batchParams)
}

// parseAndValidateParams parses and validates batch parameters
func (t *BatchEvidenceTool) parseAndValidateParams(params interface{}, target *BatchEvidenceParams) error {
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

	if len(target.Variants) == 0 {
		return fmt.Errorf("variants array cannot be empty")
	}

	// Validate each variant
	for i, variant := range target.Variants {
		if variant.HGVSNotation == "" {
			return fmt.Errorf("variant %d missing hgvs_notation", i)
		}
	}

	// Set defaults
	if target.MaxAge == "" {
		target.MaxAge = "24h"
	}

	if target.MaxConcurrent == 0 {
		target.MaxConcurrent = t.maxConcurrent
	} else if target.MaxConcurrent > 20 {
		target.MaxConcurrent = 20
	}

	if len(target.Databases) == 0 {
		target.Databases = []string{"clinvar", "gnomad", "cosmic"}
	}

	return nil
}

// processBatch processes multiple variants with concurrent execution
func (t *BatchEvidenceTool) processBatch(ctx context.Context, params *BatchEvidenceParams) *BatchEvidenceResult {
	result := &BatchEvidenceResult{
		TotalVariants:   len(params.Variants),
		Results:        make(map[string]*QueryEvidenceResult),
		Errors:         make(map[string]string),
		BatchStats: BatchProcessingStats{
			DatabaseQueryCounts: make(map[string]int),
			QualityDistribution: make(map[string]int),
		},
	}

	// Create semaphore for concurrent processing
	semaphore := make(chan struct{}, params.MaxConcurrent)
	resultChan := make(chan VariantResult, len(params.Variants))

	// Process variants concurrently
	for _, variant := range params.Variants {
		go func(v VariantQuery) {
			semaphore <- struct{}{} // Acquire semaphore
			defer func() { <-semaphore }() // Release semaphore

			// Create individual query parameters
			queryParams := QueryEvidenceParams{
				HGVSNotation: v.HGVSNotation,
				GeneSymbol:   v.GeneSymbol,
				Databases:    params.Databases,
				IncludeRaw:   params.IncludeRaw,
				MaxAge:       params.MaxAge,
			}

			// Query evidence for this variant
			evidence, err := t.evidenceTool.gatherEvidence(ctx, &queryParams)
			
			resultChan <- VariantResult{
				VariantQuery: v,
				Evidence:     evidence,
				Error:        err,
			}
		}(variant)
	}

	// Collect results
	processedCount := 0
	failedCount := 0
	cacheHits := 0
	totalProcessingTime := time.Duration(0)

	for i := 0; i < len(params.Variants); i++ {
		varResult := <-resultChan
		processedCount++

		key := varResult.VariantQuery.HGVSNotation
		if varResult.VariantQuery.VariantID != "" {
			key = varResult.VariantQuery.VariantID
		}

		if varResult.Error != nil {
			result.Errors[key] = varResult.Error.Error()
			failedCount++
		} else {
			result.Results[key] = varResult.Evidence
			
			// Update statistics
			if varResult.Evidence.QualityScores.OverallQuality != "" {
				result.BatchStats.QualityDistribution[varResult.Evidence.QualityScores.OverallQuality]++
			}
			
			for dbName := range varResult.Evidence.DatabaseResults {
				result.BatchStats.DatabaseQueryCounts[dbName]++
			}
		}
	}

	result.ProcessedVariants = processedCount
	result.FailedVariants = failedCount
	result.BatchStats.CacheHitRate = float64(cacheHits) / float64(processedCount)

	if processedCount > 0 {
		result.BatchStats.AverageProcessingTime = (totalProcessingTime / time.Duration(processedCount)).String()
	}

	return result
}

// VariantResult represents the result for a single variant in batch processing
type VariantResult struct {
	VariantQuery VariantQuery
	Evidence     *QueryEvidenceResult
	Error        error
}