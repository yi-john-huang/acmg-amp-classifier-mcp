package caching

import (
	"context"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewToolResultCache(t *testing.T) {
	config := CacheConfig{
		Enabled: true,
	}
	
	cache := NewToolResultCache(config)
	
	assert.NotNil(t, cache)
	assert.Equal(t, 15*time.Minute, cache.config.DefaultTTL)
	assert.Equal(t, 100*1024*1024, cache.config.MaxMemorySize)
	assert.Equal(t, 1024, cache.config.CompressionThreshold)
}

func TestGenerateKey(t *testing.T) {
	cache := NewToolResultCache(CacheConfig{Enabled: true})
	
	params1 := map[string]interface{}{
		"variant": "NM_000492.3:c.1521_1523delCTT",
		"mode":    "classify",
	}
	
	params2 := map[string]interface{}{
		"mode":    "classify",
		"variant": "NM_000492.3:c.1521_1523delCTT",
	}
	
	key1 := cache.GenerateKey("classify_variant", params1)
	key2 := cache.GenerateKey("classify_variant", params2)
	
	// Keys should be identical regardless of parameter order
	assert.Equal(t, key1, key2)
	assert.Len(t, key1, 64) // SHA-256 hex string length
}

func TestCacheSetAndGet(t *testing.T) {
	cache := NewToolResultCache(CacheConfig{
		Enabled:    true,
		DefaultTTL: time.Minute,
	})
	
	ctx := context.Background()
	toolName := "classify_variant"
	params := map[string]interface{}{
		"variant": "NM_000492.3:c.1521_1523delCTT",
	}
	
	result := map[string]interface{}{
		"classification": "Pathogenic",
		"confidence":     0.95,
	}
	
	// Cache miss initially
	cached, found := cache.Get(ctx, toolName, params)
	assert.False(t, found)
	assert.Nil(t, cached)
	
	// Set result
	err := cache.Set(ctx, toolName, params, result, 100*time.Millisecond, 0)
	require.NoError(t, err)
	
	// Cache hit
	cached, found = cache.Get(ctx, toolName, params)
	assert.True(t, found)
	assert.NotNil(t, cached)
	assert.Equal(t, toolName, cached.ToolName)
	assert.Equal(t, result, cached.Result)
	assert.True(t, cached.Metadata.Success)
	assert.Equal(t, 100*time.Millisecond, cached.Metadata.ExecutionTime)
}

func TestCacheExpiration(t *testing.T) {
	cache := NewToolResultCache(CacheConfig{
		Enabled:    true,
		DefaultTTL: 50 * time.Millisecond,
	})
	
	ctx := context.Background()
	toolName := "test_tool"
	params := map[string]interface{}{"key": "value"}
	result := "test_result"
	
	// Set with short TTL
	err := cache.Set(ctx, toolName, params, result, time.Millisecond, 50*time.Millisecond)
	require.NoError(t, err)
	
	// Should be available immediately
	cached, found := cache.Get(ctx, toolName, params)
	assert.True(t, found)
	assert.Equal(t, result, cached.Result)
	
	// Wait for expiration
	time.Sleep(60 * time.Millisecond)
	
	// Should be expired
	cached, found = cache.Get(ctx, toolName, params)
	assert.False(t, found)
	assert.Nil(t, cached)
}

func TestCacheError(t *testing.T) {
	cache := NewToolResultCache(CacheConfig{
		Enabled:    true,
		DefaultTTL: time.Minute,
	})
	
	ctx := context.Background()
	toolName := "failing_tool"
	params := map[string]interface{}{"invalid": true}
	
	// Set error result
	err := cache.SetError(ctx, toolName, params, "INVALID_PARAMETERS", 50*time.Millisecond)
	require.NoError(t, err)
	
	// Retrieve error result
	cached, found := cache.Get(ctx, toolName, params)
	assert.True(t, found)
	assert.NotNil(t, cached)
	assert.False(t, cached.Metadata.Success)
	assert.Equal(t, "INVALID_PARAMETERS", cached.Metadata.ErrorCode)
	assert.Equal(t, 50*time.Millisecond, cached.Metadata.ExecutionTime)
}

func TestCacheStats(t *testing.T) {
	cache := NewToolResultCache(CacheConfig{Enabled: true})
	
	ctx := context.Background()
	params := map[string]interface{}{"key": "value"}
	
	// Initial stats
	stats := cache.GetStats()
	assert.Equal(t, int64(0), stats.Hits)
	assert.Equal(t, int64(0), stats.Misses)
	
	// Cache miss
	cache.Get(ctx, "tool1", params)
	stats = cache.GetStats()
	assert.Equal(t, int64(0), stats.Hits)
	assert.Equal(t, int64(1), stats.Misses)
	
	// Set and hit
	cache.Set(ctx, "tool1", params, "result", time.Millisecond, 0)
	cache.Get(ctx, "tool1", params)
	stats = cache.GetStats()
	assert.Equal(t, int64(1), stats.Hits)
	assert.Equal(t, int64(1), stats.Misses)
	
	// Hit ratio
	ratio := cache.GetHitRatio()
	assert.Equal(t, 0.5, ratio)
}

func TestInvalidateByTool(t *testing.T) {
	cache := NewToolResultCache(CacheConfig{Enabled: true})
	
	ctx := context.Background()
	params1 := map[string]interface{}{"variant": "variant1"}
	params2 := map[string]interface{}{"variant": "variant2"}
	
	// Set results for two tools
	cache.Set(ctx, "classify_variant", params1, "result1", time.Millisecond, 0)
	cache.Set(ctx, "validate_hgvs", params2, "result2", time.Millisecond, 0)
	
	// Verify both are cached
	cached1, found1 := cache.Get(ctx, "classify_variant", params1)
	cached2, found2 := cache.Get(ctx, "validate_hgvs", params2)
	assert.True(t, found1)
	assert.True(t, found2)
	
	// Invalidate one tool
	err := cache.InvalidateByTool(ctx, "classify_variant")
	require.NoError(t, err)
	
	// Verify only the targeted tool was invalidated
	cached1, found1 = cache.Get(ctx, "classify_variant", params1)
	cached2, found2 = cache.Get(ctx, "validate_hgvs", params2)
	assert.False(t, found1)
	assert.True(t, found2)
	assert.Equal(t, "result2", cached2.Result)
}

func TestCacheClear(t *testing.T) {
	cache := NewToolResultCache(CacheConfig{Enabled: true})
	
	ctx := context.Background()
	params := map[string]interface{}{"key": "value"}
	
	// Set multiple results
	cache.Set(ctx, "tool1", params, "result1", time.Millisecond, 0)
	cache.Set(ctx, "tool2", params, "result2", time.Millisecond, 0)
	
	// Verify they're cached
	_, found1 := cache.Get(ctx, "tool1", params)
	_, found2 := cache.Get(ctx, "tool2", params)
	assert.True(t, found1)
	assert.True(t, found2)
	
	// Clear cache
	err := cache.Clear(ctx)
	require.NoError(t, err)
	
	// Verify cache is empty
	_, found1 = cache.Get(ctx, "tool1", params)
	_, found2 = cache.Get(ctx, "tool2", params)
	assert.False(t, found1)
	assert.False(t, found2)
	
	// Stats should be reset
	stats := cache.GetStats()
	assert.Equal(t, int64(0), stats.Hits)
	assert.Equal(t, int64(0), stats.Misses)
}

func TestCacheDisabled(t *testing.T) {
	cache := NewToolResultCache(CacheConfig{Enabled: false})
	
	ctx := context.Background()
	params := map[string]interface{}{"key": "value"}
	
	// Operations should not error but should not cache
	err := cache.Set(ctx, "tool1", params, "result", time.Millisecond, 0)
	assert.NoError(t, err)
	
	cached, found := cache.Get(ctx, "tool1", params)
	assert.False(t, found)
	assert.Nil(t, cached)
}

func TestCacheHealth(t *testing.T) {
	// Enabled cache should be healthy
	cache := NewToolResultCache(CacheConfig{Enabled: true})
	assert.True(t, cache.IsHealthy(context.Background()))
	
	// Disabled cache should also be healthy
	disabledCache := NewToolResultCache(CacheConfig{Enabled: false})
	assert.True(t, disabledCache.IsHealthy(context.Background()))
}

func TestCacheWithRedis(t *testing.T) {
	// Skip if no Redis available (integration test)
	t.Skip("Redis integration test - requires Redis instance")
	
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	
	cache := NewToolResultCache(CacheConfig{
		Enabled:     true,
		RedisClient: redisClient,
	})
	
	ctx := context.Background()
	params := map[string]interface{}{"key": "value"}
	
	// Test Redis storage and retrieval
	err := cache.Set(ctx, "test_tool", params, "test_result", time.Millisecond, time.Minute)
	require.NoError(t, err)
	
	// Clear memory cache to test Redis retrieval
	cache.memoryCache = make(map[string]*CachedResult)
	
	cached, found := cache.Get(ctx, "test_tool", params)
	assert.True(t, found)
	assert.Equal(t, "test_result", cached.Result)
	
	// Cleanup
	redisClient.Close()
}

func TestCacheEviction(t *testing.T) {
	cache := NewToolResultCache(CacheConfig{
		Enabled:       true,
		MaxMemorySize: 100, // Very small limit to trigger eviction
	})
	
	ctx := context.Background()
	
	// Add multiple entries to trigger eviction
	for i := 0; i < 10; i++ {
		params := map[string]interface{}{"index": i}
		result := map[string]interface{}{"data": string(make([]byte, 50))} // 50 bytes each
		cache.Set(ctx, "test_tool", params, result, time.Millisecond, 0)
		time.Sleep(time.Millisecond) // Ensure different timestamps
	}
	
	stats := cache.GetStats()
	assert.Greater(t, stats.Evictions, int64(0))
}

func TestCacheAccessMetrics(t *testing.T) {
	cache := NewToolResultCache(CacheConfig{Enabled: true})
	
	ctx := context.Background()
	params := map[string]interface{}{"key": "value"}
	
	// Set result
	cache.Set(ctx, "test_tool", params, "result", time.Millisecond, 0)
	
	// Access multiple times
	for i := 0; i < 3; i++ {
		cached, found := cache.Get(ctx, "test_tool", params)
		assert.True(t, found)
		assert.Equal(t, int64(i+1), cached.Metadata.CacheHits)
	}
}