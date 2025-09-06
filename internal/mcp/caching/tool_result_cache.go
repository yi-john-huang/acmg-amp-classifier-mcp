package caching

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
)

// CacheConfig defines configuration for tool result caching
type CacheConfig struct {
	// Redis client for distributed caching
	RedisClient *redis.Client
	// Default TTL for cached results
	DefaultTTL time.Duration
	// Maximum cache size for in-memory cache
	MaxMemorySize int
	// Enable/disable caching
	Enabled bool
	// Compression threshold (bytes)
	CompressionThreshold int
}

// CachedResult represents a cached tool execution result
type CachedResult struct {
	ToolName   string                 `json:"tool_name"`
	Parameters map[string]interface{} `json:"parameters"`
	Result     interface{}            `json:"result"`
	Metadata   CacheMetadata          `json:"metadata"`
	CreatedAt  time.Time              `json:"created_at"`
	ExpiresAt  time.Time              `json:"expires_at"`
	Compressed bool                   `json:"compressed"`
	Data       []byte                 `json:"data,omitempty"`
}

// CacheMetadata contains additional information about the cached result
type CacheMetadata struct {
	ExecutionTime time.Duration `json:"execution_time"`
	Success       bool          `json:"success"`
	ErrorCode     string        `json:"error_code,omitempty"`
	CacheHits     int64         `json:"cache_hits"`
	LastAccessed  time.Time     `json:"last_accessed"`
	Size          int           `json:"size"`
	Version       string        `json:"version"`
}

// ToolResultCache manages caching of tool execution results
type ToolResultCache struct {
	config      CacheConfig
	memoryCache map[string]*CachedResult
	memoryMutex sync.RWMutex
	stats       CacheStats
	statsMutex  sync.RWMutex
}

// CacheStats tracks cache performance metrics
type CacheStats struct {
	Hits        int64 `json:"hits"`
	Misses      int64 `json:"misses"`
	Evictions   int64 `json:"evictions"`
	MemoryUsage int64 `json:"memory_usage"`
	RedisUsage  int64 `json:"redis_usage"`
}

// NewToolResultCache creates a new tool result cache instance
func NewToolResultCache(config CacheConfig) *ToolResultCache {
	if config.DefaultTTL == 0 {
		config.DefaultTTL = 15 * time.Minute
	}
	if config.MaxMemorySize == 0 {
		config.MaxMemorySize = 100 * 1024 * 1024 // 100MB
	}
	if config.CompressionThreshold == 0 {
		config.CompressionThreshold = 1024 // 1KB
	}

	return &ToolResultCache{
		config:      config,
		memoryCache: make(map[string]*CachedResult),
		stats:       CacheStats{},
	}
}

// GenerateKey creates a unique cache key for tool parameters
func (trc *ToolResultCache) GenerateKey(toolName string, parameters map[string]interface{}) string {
	// Sort parameters for consistent key generation
	paramBytes, _ := json.Marshal(parameters)
	hash := sha256.Sum256(append([]byte(toolName+"::"), paramBytes...))
	return hex.EncodeToString(hash[:])
}

// Get retrieves a cached result if available
func (trc *ToolResultCache) Get(ctx context.Context, toolName string, parameters map[string]interface{}) (*CachedResult, bool) {
	if !trc.config.Enabled {
		return nil, false
	}

	key := trc.GenerateKey(toolName, parameters)

	// Check memory cache first
	trc.memoryMutex.RLock()
	if cached, exists := trc.memoryCache[key]; exists {
		if time.Now().Before(cached.ExpiresAt) {
			trc.memoryMutex.RUnlock()
			// Update access statistics
			cached.Metadata.CacheHits++
			cached.Metadata.LastAccessed = time.Now()
			trc.updateStats(true, false)
			return cached, true
		}
		// Expired entry, remove it
		delete(trc.memoryCache, key)
	}
	trc.memoryMutex.RUnlock()

	// Check Redis cache if available
	if trc.config.RedisClient != nil {
		data, err := trc.config.RedisClient.Get(ctx, "mcp:cache:tool:"+key).Bytes()
		if err == nil {
			var cached CachedResult
			if err := json.Unmarshal(data, &cached); err == nil {
				if time.Now().Before(cached.ExpiresAt) {
					// Update access statistics
					cached.Metadata.CacheHits++
					cached.Metadata.LastAccessed = time.Now()

					// Store in memory cache for faster access
					trc.memoryMutex.Lock()
					trc.memoryCache[key] = &cached
					trc.memoryMutex.Unlock()

					trc.updateStats(true, false)
					return &cached, true
				}
				// Remove expired entry from Redis
				trc.config.RedisClient.Del(ctx, "mcp:cache:tool:"+key)
			}
		}
	}

	trc.updateStats(false, false)
	return nil, false
}

// Set stores a result in the cache
func (trc *ToolResultCache) Set(ctx context.Context, toolName string, parameters map[string]interface{}, result interface{}, executionTime time.Duration, ttl time.Duration) error {
	if !trc.config.Enabled {
		return nil
	}

	if ttl == 0 {
		ttl = trc.config.DefaultTTL
	}

	key := trc.GenerateKey(toolName, parameters)
	
	resultBytes, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}

	cached := &CachedResult{
		ToolName:   toolName,
		Parameters: parameters,
		Result:     result,
		CreatedAt:  time.Now(),
		ExpiresAt:  time.Now().Add(ttl),
		Compressed: false,
		Metadata: CacheMetadata{
			ExecutionTime: executionTime,
			Success:       true,
			CacheHits:     0,
			LastAccessed:  time.Now(),
			Size:          len(resultBytes),
			Version:       "1.0",
		},
	}

	// Compress large results
	if len(resultBytes) > trc.config.CompressionThreshold {
		compressed, err := trc.compressData(resultBytes)
		if err == nil && len(compressed) < len(resultBytes) {
			cached.Data = compressed
			cached.Compressed = true
			cached.Metadata.Size = len(compressed)
		}
	}

	// Store in memory cache
	trc.memoryMutex.Lock()
	// Check if we need to evict entries
	trc.evictIfNeeded()
	trc.memoryCache[key] = cached
	trc.memoryMutex.Unlock()

	// Store in Redis cache if available
	if trc.config.RedisClient != nil {
		cachedBytes, err := json.Marshal(cached)
		if err == nil {
			err = trc.config.RedisClient.Set(ctx, "mcp:cache:tool:"+key, cachedBytes, ttl).Err()
			if err != nil {
				// Log error but don't fail the operation
				fmt.Printf("Failed to store in Redis cache: %v\n", err)
			}
		}
	}

	return nil
}

// SetError stores an error result in the cache with shorter TTL
func (trc *ToolResultCache) SetError(ctx context.Context, toolName string, parameters map[string]interface{}, errorCode string, executionTime time.Duration) error {
	if !trc.config.Enabled {
		return nil
	}

	key := trc.GenerateKey(toolName, parameters)
	
	// Use shorter TTL for errors
	errorTTL := trc.config.DefaultTTL / 4

	cached := &CachedResult{
		ToolName:   toolName,
		Parameters: parameters,
		Result:     nil,
		CreatedAt:  time.Now(),
		ExpiresAt:  time.Now().Add(errorTTL),
		Metadata: CacheMetadata{
			ExecutionTime: executionTime,
			Success:       false,
			ErrorCode:     errorCode,
			CacheHits:     0,
			LastAccessed:  time.Now(),
			Size:          0,
			Version:       "1.0",
		},
	}

	// Store in memory cache
	trc.memoryMutex.Lock()
	trc.evictIfNeeded()
	trc.memoryCache[key] = cached
	trc.memoryMutex.Unlock()

	// Store in Redis cache if available
	if trc.config.RedisClient != nil {
		cachedBytes, err := json.Marshal(cached)
		if err == nil {
			trc.config.RedisClient.Set(ctx, "mcp:cache:tool:"+key, cachedBytes, errorTTL)
		}
	}

	return nil
}

// InvalidateByTool removes all cached results for a specific tool
func (trc *ToolResultCache) InvalidateByTool(ctx context.Context, toolName string) error {
	// Remove from memory cache
	trc.memoryMutex.Lock()
	for key, cached := range trc.memoryCache {
		if cached.ToolName == toolName {
			delete(trc.memoryCache, key)
		}
	}
	trc.memoryMutex.Unlock()

	// Remove from Redis cache if available
	if trc.config.RedisClient != nil {
		keys, err := trc.config.RedisClient.Keys(ctx, "mcp:cache:tool:*").Result()
		if err == nil {
			for _, key := range keys {
				data, err := trc.config.RedisClient.Get(ctx, key).Bytes()
				if err == nil {
					var cached CachedResult
					if json.Unmarshal(data, &cached) == nil && cached.ToolName == toolName {
						trc.config.RedisClient.Del(ctx, key)
					}
				}
			}
		}
	}

	return nil
}

// Clear removes all cached results
func (trc *ToolResultCache) Clear(ctx context.Context) error {
	// Clear memory cache
	trc.memoryMutex.Lock()
	trc.memoryCache = make(map[string]*CachedResult)
	trc.memoryMutex.Unlock()

	// Clear Redis cache if available
	if trc.config.RedisClient != nil {
		keys, err := trc.config.RedisClient.Keys(ctx, "mcp:cache:tool:*").Result()
		if err == nil && len(keys) > 0 {
			trc.config.RedisClient.Del(ctx, keys...)
		}
	}

	// Reset stats
	trc.statsMutex.Lock()
	trc.stats = CacheStats{}
	trc.statsMutex.Unlock()

	return nil
}

// GetStats returns cache performance statistics
func (trc *ToolResultCache) GetStats() CacheStats {
	trc.statsMutex.RLock()
	defer trc.statsMutex.RUnlock()
	
	// Update memory usage
	trc.stats.MemoryUsage = trc.calculateMemoryUsage()
	
	return trc.stats
}

// evictIfNeeded removes old entries if memory usage is too high
func (trc *ToolResultCache) evictIfNeeded() {
	currentUsage := trc.calculateMemoryUsage()
	if int(currentUsage) <= trc.config.MaxMemorySize {
		return
	}

	// Find oldest entries to evict
	var oldestKey string
	var oldestTime time.Time = time.Now()

	for key, cached := range trc.memoryCache {
		if cached.Metadata.LastAccessed.Before(oldestTime) {
			oldestTime = cached.Metadata.LastAccessed
			oldestKey = key
		}
	}

	if oldestKey != "" {
		delete(trc.memoryCache, oldestKey)
		trc.updateStats(false, true)
	}
}

// calculateMemoryUsage estimates current memory usage
func (trc *ToolResultCache) calculateMemoryUsage() int64 {
	var usage int64
	for _, cached := range trc.memoryCache {
		usage += int64(cached.Metadata.Size)
	}
	return usage
}

// updateStats updates cache performance statistics
func (trc *ToolResultCache) updateStats(hit bool, eviction bool) {
	trc.statsMutex.Lock()
	defer trc.statsMutex.Unlock()

	if hit {
		trc.stats.Hits++
	} else {
		trc.stats.Misses++
	}

	if eviction {
		trc.stats.Evictions++
	}
}

// compressData compresses data using gzip (simplified implementation)
func (trc *ToolResultCache) compressData(data []byte) ([]byte, error) {
	// This is a placeholder for compression logic
	// In a real implementation, you would use gzip or another compression algorithm
	return data, nil
}

// GetHitRatio calculates the cache hit ratio
func (trc *ToolResultCache) GetHitRatio() float64 {
	trc.statsMutex.RLock()
	defer trc.statsMutex.RUnlock()

	total := trc.stats.Hits + trc.stats.Misses
	if total == 0 {
		return 0.0
	}
	return float64(trc.stats.Hits) / float64(total)
}

// IsHealthy checks if the cache is functioning properly
func (trc *ToolResultCache) IsHealthy(ctx context.Context) bool {
	if !trc.config.Enabled {
		return true
	}

	// Test memory cache
	testKey := "health_check_" + time.Now().Format("20060102150405")
	testCached := &CachedResult{
		ToolName:  "health_check",
		Result:    "ok",
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Minute),
		Metadata: CacheMetadata{
			Success: true,
			Size:    2,
			Version: "1.0",
		},
	}

	trc.memoryMutex.Lock()
	trc.memoryCache[testKey] = testCached
	_, exists := trc.memoryCache[testKey]
	delete(trc.memoryCache, testKey)
	trc.memoryMutex.Unlock()

	if !exists {
		return false
	}

	// Test Redis if available
	if trc.config.RedisClient != nil {
		err := trc.config.RedisClient.Ping(ctx).Err()
		if err != nil {
			return false
		}
	}

	return true
}