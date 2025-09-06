package caching

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewResourceCache(t *testing.T) {
	config := ResourceCacheConfig{
		Enabled: true,
	}
	
	cache := NewResourceCache(config)
	
	assert.NotNil(t, cache)
	assert.Equal(t, 30*time.Minute, cache.config.DefaultTTL)
	assert.Equal(t, 200*1024*1024, cache.config.MaxMemorySize)
	assert.NotNil(t, cache.config.ResourceTTLs)
}

func TestResourceCacheSetAndGet(t *testing.T) {
	cache := NewResourceCache(ResourceCacheConfig{
		Enabled:    true,
		DefaultTTL: time.Hour,
	})
	
	ctx := context.Background()
	uri := "variant/NM_000492.3:c.1521_1523delCTT"
	resourceType := "variant"
	content := map[string]interface{}{
		"id":             "NM_000492.3:c.1521_1523delCTT",
		"classification": "Pathogenic",
		"confidence":     0.95,
	}
	headers := map[string]string{
		"Content-Type": "application/json",
	}
	
	// Cache miss initially
	cached, found := cache.Get(ctx, uri)
	assert.False(t, found)
	assert.Nil(t, cached)
	
	// Set resource
	err := cache.Set(ctx, uri, resourceType, content, headers, 50*time.Millisecond)
	require.NoError(t, err)
	
	// Cache hit
	cached, found = cache.Get(ctx, uri)
	assert.True(t, found)
	assert.NotNil(t, cached)
	assert.Equal(t, uri, cached.URI)
	assert.Equal(t, resourceType, cached.ResourceType)
	assert.Equal(t, content, cached.Content)
	assert.Equal(t, headers, cached.Headers)
	assert.Equal(t, 50*time.Millisecond, cached.Metadata.LoadTime)
}

func TestResourceCacheExpiration(t *testing.T) {
	cache := NewResourceCache(ResourceCacheConfig{
		Enabled:    true,
		DefaultTTL: 50 * time.Millisecond,
	})
	
	ctx := context.Background()
	uri := "test/resource"
	content := "test_content"
	
	// Set with short TTL
	err := cache.Set(ctx, uri, "test", content, nil, time.Millisecond)
	require.NoError(t, err)
	
	// Should be available immediately
	cached, found := cache.Get(ctx, uri)
	assert.True(t, found)
	assert.Equal(t, content, cached.Content)
	
	// Wait for expiration
	time.Sleep(60 * time.Millisecond)
	
	// Should be expired
	cached, found = cache.Get(ctx, uri)
	assert.False(t, found)
	assert.Nil(t, cached)
}

func TestResourceCacheWithCustomTTL(t *testing.T) {
	customTTLs := map[string]time.Duration{
		"variant": 2 * time.Hour,
		"evidence": 30 * time.Minute,
	}
	
	cache := NewResourceCache(ResourceCacheConfig{
		Enabled:      true,
		DefaultTTL:   time.Hour,
		ResourceTTLs: customTTLs,
	})
	
	ctx := context.Background()
	
	// Set variant resource (should use custom TTL)
	err := cache.Set(ctx, "variant/test", "variant", "data", nil, time.Millisecond)
	require.NoError(t, err)
	
	cached, found := cache.Get(ctx, "variant/test")
	assert.True(t, found)
	
	// Check that TTL is approximately 2 hours (allowing for small timing differences)
	expectedExpiry := time.Now().Add(2 * time.Hour)
	timeDiff := cached.ExpiresAt.Sub(expectedExpiry)
	assert.Less(t, timeDiff.Abs(), time.Minute)
}

func TestResourceCacheStats(t *testing.T) {
	cache := NewResourceCache(ResourceCacheConfig{Enabled: true})
	
	ctx := context.Background()
	
	// Initial stats
	stats := cache.GetStats()
	assert.Equal(t, int64(0), stats.Hits)
	assert.Equal(t, int64(0), stats.Misses)
	
	// Cache miss
	cache.Get(ctx, "test/resource")
	stats = cache.GetStats()
	assert.Equal(t, int64(0), stats.Hits)
	assert.Equal(t, int64(1), stats.Misses)
	
	// Set and hit
	cache.Set(ctx, "test/resource", "test", "data", nil, time.Millisecond)
	cache.Get(ctx, "test/resource")
	stats = cache.GetStats()
	assert.Equal(t, int64(1), stats.Hits)
	assert.Equal(t, int64(1), stats.Misses)
	assert.Equal(t, int64(1), stats.ResourceTypes["test"])
	
	// Hit ratio
	ratio := cache.GetHitRatio()
	assert.Equal(t, 0.5, ratio)
}

func TestResourceCacheConditionalHeaders(t *testing.T) {
	cache := NewResourceCache(ResourceCacheConfig{Enabled: true})
	
	ctx := context.Background()
	uri := "test/conditional"
	content := "test_content"
	
	// Initial set
	modified, err := cache.SetWithConditionalHeaders(ctx, uri, "test", content, nil, time.Millisecond, time.Time{}, "")
	require.NoError(t, err)
	assert.True(t, modified)
	
	// Get the cached resource to obtain ETag
	cached, found := cache.Get(ctx, uri)
	require.True(t, found)
	etag := cached.ETag
	
	// Try to set again with matching ETag (should not modify)
	modified, err = cache.SetWithConditionalHeaders(ctx, uri, "test", "new_content", nil, time.Millisecond, time.Time{}, etag)
	require.NoError(t, err)
	assert.False(t, modified)
	
	// Verify content wasn't changed
	cached, found = cache.Get(ctx, uri)
	require.True(t, found)
	assert.Equal(t, content, cached.Content)
}

func TestResourceCacheInvalidation(t *testing.T) {
	cache := NewResourceCache(ResourceCacheConfig{Enabled: true})
	
	ctx := context.Background()
	uri1 := "test/resource1"
	uri2 := "test/resource2"
	
	// Set resources
	cache.Set(ctx, uri1, "test", "data1", nil, time.Millisecond)
	cache.Set(ctx, uri2, "test", "data2", nil, time.Millisecond)
	
	// Verify both are cached
	cached1, found1 := cache.Get(ctx, uri1)
	cached2, found2 := cache.Get(ctx, uri2)
	assert.True(t, found1)
	assert.True(t, found2)
	
	// Invalidate one resource
	err := cache.Invalidate(ctx, uri1)
	require.NoError(t, err)
	
	// Verify only the targeted resource was invalidated
	cached1, found1 = cache.Get(ctx, uri1)
	cached2, found2 = cache.Get(ctx, uri2)
	assert.False(t, found1)
	assert.True(t, found2)
	assert.Equal(t, "data2", cached2.Content)
}

func TestResourceCacheInvalidateByPattern(t *testing.T) {
	cache := NewResourceCache(ResourceCacheConfig{Enabled: true})
	
	ctx := context.Background()
	
	// Set resources with different patterns
	cache.Set(ctx, "variant/test1", "variant", "data1", nil, time.Millisecond)
	cache.Set(ctx, "evidence/test1", "evidence", "data2", nil, time.Millisecond)
	cache.Set(ctx, "variant/test2", "variant", "data3", nil, time.Millisecond)
	
	// Verify all are cached
	_, found1 := cache.Get(ctx, "variant/test1")
	_, found2 := cache.Get(ctx, "evidence/test1")
	_, found3 := cache.Get(ctx, "variant/test2")
	assert.True(t, found1)
	assert.True(t, found2)
	assert.True(t, found3)
	
	// Invalidate by pattern (simple matching)
	err := cache.InvalidateByPattern(ctx, "variant/test1")
	require.NoError(t, err)
	
	// Verify pattern-based invalidation
	_, found1 = cache.Get(ctx, "variant/test1")
	_, found2 = cache.Get(ctx, "evidence/test1")
	_, found3 = cache.Get(ctx, "variant/test2")
	assert.False(t, found1)
	assert.True(t, found2)
	assert.True(t, found3)
}

func TestResourceCacheClear(t *testing.T) {
	cache := NewResourceCache(ResourceCacheConfig{Enabled: true})
	
	ctx := context.Background()
	
	// Set multiple resources
	cache.Set(ctx, "resource1", "test", "data1", nil, time.Millisecond)
	cache.Set(ctx, "resource2", "test", "data2", nil, time.Millisecond)
	
	// Verify they're cached
	_, found1 := cache.Get(ctx, "resource1")
	_, found2 := cache.Get(ctx, "resource2")
	assert.True(t, found1)
	assert.True(t, found2)
	
	// Clear cache
	err := cache.Clear(ctx)
	require.NoError(t, err)
	
	// Verify cache is empty
	_, found1 = cache.Get(ctx, "resource1")
	_, found2 = cache.Get(ctx, "resource2")
	assert.False(t, found1)
	assert.False(t, found2)
	
	// Stats should be reset
	stats := cache.GetStats()
	assert.Equal(t, int64(0), stats.Hits)
	assert.Equal(t, int64(0), stats.Misses)
}

func TestResourceCacheDisabled(t *testing.T) {
	cache := NewResourceCache(ResourceCacheConfig{Enabled: false})
	
	ctx := context.Background()
	uri := "test/resource"
	
	// Operations should not error but should not cache
	err := cache.Set(ctx, uri, "test", "data", nil, time.Millisecond)
	assert.NoError(t, err)
	
	cached, found := cache.Get(ctx, uri)
	assert.False(t, found)
	assert.Nil(t, cached)
}

func TestResourceCachePreload(t *testing.T) {
	cache := NewResourceCache(ResourceCacheConfig{
		Enabled:         true,
		PreloadPatterns: []string{"variant/common", "evidence/frequent"},
	})
	
	ctx := context.Background()
	resources := []string{"variant/common", "evidence/frequent"}
	
	err := cache.Preload(ctx, resources)
	assert.NoError(t, err)
	
	// Verify preload tracking
	cache.preloadMutex.RLock()
	for _, uri := range resources {
		assert.True(t, cache.preloadDone[uri])
	}
	cache.preloadMutex.RUnlock()
}

func TestResourceCacheListCached(t *testing.T) {
	cache := NewResourceCache(ResourceCacheConfig{Enabled: true})
	
	ctx := context.Background()
	
	// Initially empty
	uris := cache.ListCachedResources()
	assert.Empty(t, uris)
	
	// Set some resources
	cache.Set(ctx, "resource1", "test", "data1", nil, time.Millisecond)
	cache.Set(ctx, "resource2", "test", "data2", nil, time.Millisecond)
	
	uris = cache.ListCachedResources()
	assert.Len(t, uris, 2)
	assert.Contains(t, uris, "resource1")
	assert.Contains(t, uris, "resource2")
}

func TestResourceCacheHealth(t *testing.T) {
	// Enabled cache should be healthy
	cache := NewResourceCache(ResourceCacheConfig{Enabled: true})
	assert.True(t, cache.IsHealthy(context.Background()))
	
	// Disabled cache should also be healthy
	disabledCache := NewResourceCache(ResourceCacheConfig{Enabled: false})
	assert.True(t, disabledCache.IsHealthy(context.Background()))
}

func TestResourceCacheEviction(t *testing.T) {
	cache := NewResourceCache(ResourceCacheConfig{
		Enabled:       true,
		MaxMemorySize: 100, // Very small limit to trigger eviction
	})
	
	ctx := context.Background()
	
	// Add multiple resources to trigger eviction
	for i := 0; i < 10; i++ {
		uri := fmt.Sprintf("resource%d", i)
		content := map[string]interface{}{"data": string(make([]byte, 50))} // 50 bytes each
		cache.Set(ctx, uri, "test", content, nil, time.Millisecond)
		time.Sleep(time.Millisecond) // Ensure different timestamps
	}
	
	stats := cache.GetStats()
	assert.Greater(t, stats.Evictions, int64(0))
}

func TestResourceCacheAccessMetrics(t *testing.T) {
	cache := NewResourceCache(ResourceCacheConfig{Enabled: true})
	
	ctx := context.Background()
	uri := "test/resource"
	
	// Set resource
	cache.Set(ctx, uri, "test", "data", nil, time.Millisecond)
	
	// Access multiple times
	for i := 0; i < 3; i++ {
		cached, found := cache.Get(ctx, uri)
		assert.True(t, found)
		assert.Equal(t, int64(i+1), cached.Metadata.AccessCount)
	}
}

func TestResourceCacheETagGeneration(t *testing.T) {
	cache := NewResourceCache(ResourceCacheConfig{Enabled: true})
	
	data1 := []byte("test data")
	data2 := []byte("different test data")
	
	etag1 := cache.generateETag(data1)
	etag2 := cache.generateETag(data2)
	
	assert.NotEqual(t, etag1, etag2)
	assert.Contains(t, etag1, `"`)
	assert.Contains(t, etag2, `"`)
}