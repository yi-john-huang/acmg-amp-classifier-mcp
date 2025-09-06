package caching

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
)

// ResourceCacheConfig defines configuration for resource caching
type ResourceCacheConfig struct {
	// Redis client for distributed caching
	RedisClient *redis.Client
	// Default TTL for cached resources
	DefaultTTL time.Duration
	// Maximum cache size for in-memory cache
	MaxMemorySize int
	// Enable/disable caching
	Enabled bool
	// Resource-specific TTL overrides
	ResourceTTLs map[string]time.Duration
	// Preload patterns for frequent resources
	PreloadPatterns []string
}

// CachedResource represents a cached MCP resource
type CachedResource struct {
	URI          string                 `json:"uri"`
	ResourceType string                 `json:"resource_type"`
	Content      interface{}            `json:"content"`
	Metadata     ResourceMetadata       `json:"metadata"`
	Headers      map[string]string      `json:"headers"`
	CreatedAt    time.Time              `json:"created_at"`
	ExpiresAt    time.Time              `json:"expires_at"`
	LastModified time.Time              `json:"last_modified"`
	ETag         string                 `json:"etag"`
}

// ResourceMetadata contains additional information about the cached resource
type ResourceMetadata struct {
	Size         int           `json:"size"`
	AccessCount  int64         `json:"access_count"`
	LastAccessed time.Time     `json:"last_accessed"`
	LoadTime     time.Duration `json:"load_time"`
	Version      string        `json:"version"`
	CacheSource  string        `json:"cache_source"` // "memory", "redis", "origin"
	Compressed   bool          `json:"compressed"`
}

// ResourceCache manages caching of MCP resources
type ResourceCache struct {
	config      ResourceCacheConfig
	memoryCache map[string]*CachedResource
	memoryMutex sync.RWMutex
	stats       ResourceCacheStats
	statsMutex  sync.RWMutex
	preloadDone map[string]bool
	preloadMutex sync.RWMutex
}

// ResourceCacheStats tracks resource cache performance metrics
type ResourceCacheStats struct {
	Hits          int64            `json:"hits"`
	Misses        int64            `json:"misses"`
	Evictions     int64            `json:"evictions"`
	MemoryUsage   int64            `json:"memory_usage"`
	RedisUsage    int64            `json:"redis_usage"`
	PreloadHits   int64            `json:"preload_hits"`
	ResourceTypes map[string]int64 `json:"resource_types"`
}

// ResourcePattern defines patterns for resource preloading
type ResourcePattern struct {
	Pattern     string        `json:"pattern"`
	TTL         time.Duration `json:"ttl"`
	Priority    int           `json:"priority"`
	BatchSize   int           `json:"batch_size"`
	RefreshRate time.Duration `json:"refresh_rate"`
}

// NewResourceCache creates a new resource cache instance
func NewResourceCache(config ResourceCacheConfig) *ResourceCache {
	if config.DefaultTTL == 0 {
		config.DefaultTTL = 30 * time.Minute
	}
	if config.MaxMemorySize == 0 {
		config.MaxMemorySize = 200 * 1024 * 1024 // 200MB
	}
	if config.ResourceTTLs == nil {
		config.ResourceTTLs = make(map[string]time.Duration)
	}

	rc := &ResourceCache{
		config:       config,
		memoryCache:  make(map[string]*CachedResource),
		preloadDone:  make(map[string]bool),
		stats: ResourceCacheStats{
			ResourceTypes: make(map[string]int64),
		},
	}

	// Start preloading if patterns are configured
	if len(config.PreloadPatterns) > 0 {
		go rc.startPreloader()
	}

	return rc
}

// Get retrieves a cached resource
func (rc *ResourceCache) Get(ctx context.Context, uri string) (*CachedResource, bool) {
	if !rc.config.Enabled {
		return nil, false
	}

	// Check memory cache first
	rc.memoryMutex.RLock()
	if cached, exists := rc.memoryCache[uri]; exists {
		if time.Now().Before(cached.ExpiresAt) {
			rc.memoryMutex.RUnlock()
			// Update access statistics
			cached.Metadata.AccessCount++
			cached.Metadata.LastAccessed = time.Now()
			cached.Metadata.CacheSource = "memory"
			rc.updateStats(true, cached.ResourceType, false)
			return cached, true
		}
		// Expired entry, remove it
		delete(rc.memoryCache, uri)
	}
	rc.memoryMutex.RUnlock()

	// Check Redis cache if available
	if rc.config.RedisClient != nil {
		data, err := rc.config.RedisClient.Get(ctx, "mcp:cache:resource:"+uri).Bytes()
		if err == nil {
			var cached CachedResource
			if err := json.Unmarshal(data, &cached); err == nil {
				if time.Now().Before(cached.ExpiresAt) {
					// Update access statistics
					cached.Metadata.AccessCount++
					cached.Metadata.LastAccessed = time.Now()
					cached.Metadata.CacheSource = "redis"

					// Store in memory cache for faster access
					rc.memoryMutex.Lock()
					rc.memoryCache[uri] = &cached
					rc.memoryMutex.Unlock()

					rc.updateStats(true, cached.ResourceType, false)
					return &cached, true
				}
				// Remove expired entry from Redis
				rc.config.RedisClient.Del(ctx, "mcp:cache:resource:"+uri)
			}
		}
	}

	rc.updateStats(false, "", false)
	return nil, false
}

// Set stores a resource in the cache
func (rc *ResourceCache) Set(ctx context.Context, uri string, resourceType string, content interface{}, headers map[string]string, loadTime time.Duration) error {
	if !rc.config.Enabled {
		return nil
	}

	// Determine TTL based on resource type
	ttl := rc.getTTLForResource(resourceType)

	contentBytes, err := json.Marshal(content)
	if err != nil {
		return fmt.Errorf("failed to marshal content: %w", err)
	}

	// Generate ETag for cache validation
	etag := rc.generateETag(contentBytes)

	cached := &CachedResource{
		URI:          uri,
		ResourceType: resourceType,
		Content:      content,
		Headers:      headers,
		CreatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(ttl),
		LastModified: time.Now(),
		ETag:         etag,
		Metadata: ResourceMetadata{
			Size:         len(contentBytes),
			AccessCount:  0,
			LastAccessed: time.Now(),
			LoadTime:     loadTime,
			Version:      "1.0",
			CacheSource:  "origin",
			Compressed:   false,
		},
	}

	// Store in memory cache
	rc.memoryMutex.Lock()
	rc.evictIfNeeded()
	rc.memoryCache[uri] = cached
	rc.memoryMutex.Unlock()

	// Store in Redis cache if available
	if rc.config.RedisClient != nil {
		cachedBytes, err := json.Marshal(cached)
		if err == nil {
			err = rc.config.RedisClient.Set(ctx, "mcp:cache:resource:"+uri, cachedBytes, ttl).Err()
			if err != nil {
				fmt.Printf("Failed to store resource in Redis cache: %v\n", err)
			}
		}
	}

	rc.updateStats(false, resourceType, false)
	return nil
}

// SetWithConditionalHeaders stores a resource with conditional caching headers
func (rc *ResourceCache) SetWithConditionalHeaders(ctx context.Context, uri string, resourceType string, content interface{}, headers map[string]string, loadTime time.Duration, ifModifiedSince time.Time, ifNoneMatch string) (bool, error) {
	if !rc.config.Enabled {
		return false, nil
	}

	// Check if resource has been modified
	if cached, found := rc.Get(ctx, uri); found {
		// Check ETag
		if ifNoneMatch != "" && cached.ETag == ifNoneMatch {
			return false, nil // Not modified
		}

		// Check Last-Modified
		if !ifModifiedSince.IsZero() && cached.LastModified.Before(ifModifiedSince.Add(time.Second)) {
			return false, nil // Not modified
		}
	}

	err := rc.Set(ctx, uri, resourceType, content, headers, loadTime)
	return true, err
}

// Invalidate removes a specific resource from cache
func (rc *ResourceCache) Invalidate(ctx context.Context, uri string) error {
	// Remove from memory cache
	rc.memoryMutex.Lock()
	delete(rc.memoryCache, uri)
	rc.memoryMutex.Unlock()

	// Remove from Redis cache if available
	if rc.config.RedisClient != nil {
		rc.config.RedisClient.Del(ctx, "mcp:cache:resource:"+uri)
	}

	return nil
}

// InvalidateByPattern removes resources matching a pattern
func (rc *ResourceCache) InvalidateByPattern(ctx context.Context, pattern string) error {
	// Remove from memory cache
	rc.memoryMutex.Lock()
	for uri := range rc.memoryCache {
		if rc.matchesPattern(uri, pattern) {
			delete(rc.memoryCache, uri)
		}
	}
	rc.memoryMutex.Unlock()

	// Remove from Redis cache if available
	if rc.config.RedisClient != nil {
		keys, err := rc.config.RedisClient.Keys(ctx, "mcp:cache:resource:*").Result()
		if err == nil {
			for _, key := range keys {
				uri := key[len("mcp:cache:resource:"):]
				if rc.matchesPattern(uri, pattern) {
					rc.config.RedisClient.Del(ctx, key)
				}
			}
		}
	}

	return nil
}

// Clear removes all cached resources
func (rc *ResourceCache) Clear(ctx context.Context) error {
	// Clear memory cache
	rc.memoryMutex.Lock()
	rc.memoryCache = make(map[string]*CachedResource)
	rc.memoryMutex.Unlock()

	// Clear Redis cache if available
	if rc.config.RedisClient != nil {
		keys, err := rc.config.RedisClient.Keys(ctx, "mcp:cache:resource:*").Result()
		if err == nil && len(keys) > 0 {
			rc.config.RedisClient.Del(ctx, keys...)
		}
	}

	// Reset stats
	rc.statsMutex.Lock()
	rc.stats = ResourceCacheStats{
		ResourceTypes: make(map[string]int64),
	}
	rc.statsMutex.Unlock()

	return nil
}

// GetStats returns cache performance statistics
func (rc *ResourceCache) GetStats() ResourceCacheStats {
	rc.statsMutex.RLock()
	defer rc.statsMutex.RUnlock()

	// Update memory usage
	rc.stats.MemoryUsage = rc.calculateMemoryUsage()

	return rc.stats
}

// GetHitRatio calculates the cache hit ratio
func (rc *ResourceCache) GetHitRatio() float64 {
	rc.statsMutex.RLock()
	defer rc.statsMutex.RUnlock()

	total := rc.stats.Hits + rc.stats.Misses
	if total == 0 {
		return 0.0
	}
	return float64(rc.stats.Hits) / float64(total)
}

// Preload loads frequently accessed resources into cache
func (rc *ResourceCache) Preload(ctx context.Context, resources []string) error {
	if !rc.config.Enabled {
		return nil
	}

	for _, uri := range resources {
		// Check if already preloaded
		rc.preloadMutex.RLock()
		done := rc.preloadDone[uri]
		rc.preloadMutex.RUnlock()

		if !done {
			// Mark as preloaded (even if it fails to avoid repeated attempts)
			rc.preloadMutex.Lock()
			rc.preloadDone[uri] = true
			rc.preloadMutex.Unlock()

			// This would normally fetch the resource from origin
			// For now, we just mark it as preloaded
			fmt.Printf("Preloading resource: %s\n", uri)
		}
	}

	return nil
}

// ListCachedResources returns a list of cached resource URIs
func (rc *ResourceCache) ListCachedResources() []string {
	rc.memoryMutex.RLock()
	defer rc.memoryMutex.RUnlock()

	uris := make([]string, 0, len(rc.memoryCache))
	for uri := range rc.memoryCache {
		uris = append(uris, uri)
	}
	return uris
}

// IsHealthy checks if the cache is functioning properly
func (rc *ResourceCache) IsHealthy(ctx context.Context) bool {
	if !rc.config.Enabled {
		return true
	}

	// Test memory cache
	testURI := "health_check_" + time.Now().Format("20060102150405")
	testCached := &CachedResource{
		URI:          testURI,
		ResourceType: "health_check",
		Content:      "ok",
		CreatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(time.Minute),
		Metadata: ResourceMetadata{
			Size:        2,
			Version:     "1.0",
			CacheSource: "memory",
		},
	}

	rc.memoryMutex.Lock()
	rc.memoryCache[testURI] = testCached
	_, exists := rc.memoryCache[testURI]
	delete(rc.memoryCache, testURI)
	rc.memoryMutex.Unlock()

	if !exists {
		return false
	}

	// Test Redis if available
	if rc.config.RedisClient != nil {
		err := rc.config.RedisClient.Ping(ctx).Err()
		if err != nil {
			return false
		}
	}

	return true
}

// Private helper methods

func (rc *ResourceCache) getTTLForResource(resourceType string) time.Duration {
	if ttl, exists := rc.config.ResourceTTLs[resourceType]; exists {
		return ttl
	}
	return rc.config.DefaultTTL
}

func (rc *ResourceCache) generateETag(data []byte) string {
	// Simple ETag generation - in production, use more robust method
	return fmt.Sprintf(`"%x"`, len(data))
}

func (rc *ResourceCache) evictIfNeeded() {
	currentUsage := rc.calculateMemoryUsage()
	if int(currentUsage) <= rc.config.MaxMemorySize {
		return
	}

	// Find least recently used entry
	var lruURI string
	var lruTime time.Time = time.Now()

	for uri, cached := range rc.memoryCache {
		if cached.Metadata.LastAccessed.Before(lruTime) {
			lruTime = cached.Metadata.LastAccessed
			lruURI = uri
		}
	}

	if lruURI != "" {
		delete(rc.memoryCache, lruURI)
		rc.updateStats(false, "", true)
	}
}

func (rc *ResourceCache) calculateMemoryUsage() int64 {
	var usage int64
	for _, cached := range rc.memoryCache {
		usage += int64(cached.Metadata.Size)
	}
	return usage
}

func (rc *ResourceCache) updateStats(hit bool, resourceType string, eviction bool) {
	rc.statsMutex.Lock()
	defer rc.statsMutex.Unlock()

	if hit {
		rc.stats.Hits++
		if resourceType != "" {
			rc.stats.ResourceTypes[resourceType]++
		}
	} else {
		rc.stats.Misses++
	}

	if eviction {
		rc.stats.Evictions++
	}
}

func (rc *ResourceCache) matchesPattern(uri, pattern string) bool {
	// Simple pattern matching - in production, use more sophisticated matching
	return uri == pattern
}

func (rc *ResourceCache) startPreloader() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ctx := context.Background()
			rc.Preload(ctx, rc.config.PreloadPatterns)
		}
	}
}