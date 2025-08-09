package external

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/acmg-amp-mcp-server/internal/domain"
)

// CacheClient wraps Redis client with caching functionality for external API responses
type CacheClient struct {
	redis      *redis.Client
	defaultTTL time.Duration
}

// NewCacheClient creates a new cache client
func NewCacheClient(config domain.CacheConfig) (*CacheClient, error) {
	opts, err := redis.ParseURL(config.RedisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URL: %w", err)
	}
	
	// Apply cache-specific configurations
	opts.PoolSize = config.PoolSize
	opts.PoolTimeout = config.PoolTimeout
	opts.MaxRetries = config.MaxRetries
	
	client := redis.NewClient(opts)
	
	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}
	
	return &CacheClient{
		redis:      client,
		defaultTTL: config.DefaultTTL,
	}, nil
}

// CachedClinVarData represents cached ClinVar data with metadata
type CachedClinVarData struct {
	Data      *domain.ClinVarData `json:"data"`
	CachedAt  time.Time           `json:"cached_at"`
	ExpiresAt time.Time           `json:"expires_at"`
}

// CachedPopulationData represents cached population data with metadata
type CachedPopulationData struct {
	Data      *domain.PopulationData `json:"data"`
	CachedAt  time.Time              `json:"cached_at"`
	ExpiresAt time.Time              `json:"expires_at"`
}

// CachedSomaticData represents cached somatic data with metadata
type CachedSomaticData struct {
	Data      *domain.SomaticData `json:"data"`
	CachedAt  time.Time           `json:"cached_at"`
	ExpiresAt time.Time           `json:"expires_at"`
}

// GetClinVarData retrieves cached ClinVar data
func (c *CacheClient) GetClinVarData(ctx context.Context, variant *domain.StandardizedVariant) (*domain.ClinVarData, bool, error) {
	key := c.generateClinVarKey(variant)
	
	val, err := c.redis.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, false, nil // Cache miss
	}
	if err != nil {
		return nil, false, fmt.Errorf("failed to get ClinVar cache: %w", err)
	}
	
	var cached CachedClinVarData
	if err := json.Unmarshal([]byte(val), &cached); err != nil {
		// Remove corrupted cache entry
		c.redis.Del(ctx, key)
		return nil, false, nil
	}
	
	// Check if expired
	if time.Now().After(cached.ExpiresAt) {
		c.redis.Del(ctx, key)
		return nil, false, nil
	}
	
	return cached.Data, true, nil
}

// SetClinVarData caches ClinVar data
func (c *CacheClient) SetClinVarData(ctx context.Context, variant *domain.StandardizedVariant, data *domain.ClinVarData, ttl time.Duration) error {
	if ttl == 0 {
		ttl = c.defaultTTL
	}
	
	key := c.generateClinVarKey(variant)
	
	cached := CachedClinVarData{
		Data:      data,
		CachedAt:  time.Now(),
		ExpiresAt: time.Now().Add(ttl),
	}
	
	jsonData, err := json.Marshal(cached)
	if err != nil {
		return fmt.Errorf("failed to marshal ClinVar cache data: %w", err)
	}
	
	return c.redis.Set(ctx, key, jsonData, ttl).Err()
}

// GetPopulationData retrieves cached population data
func (c *CacheClient) GetPopulationData(ctx context.Context, variant *domain.StandardizedVariant) (*domain.PopulationData, bool, error) {
	key := c.generatePopulationKey(variant)
	
	val, err := c.redis.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, false, nil // Cache miss
	}
	if err != nil {
		return nil, false, fmt.Errorf("failed to get population cache: %w", err)
	}
	
	var cached CachedPopulationData
	if err := json.Unmarshal([]byte(val), &cached); err != nil {
		// Remove corrupted cache entry
		c.redis.Del(ctx, key)
		return nil, false, nil
	}
	
	// Check if expired
	if time.Now().After(cached.ExpiresAt) {
		c.redis.Del(ctx, key)
		return nil, false, nil
	}
	
	return cached.Data, true, nil
}

// SetPopulationData caches population data
func (c *CacheClient) SetPopulationData(ctx context.Context, variant *domain.StandardizedVariant, data *domain.PopulationData, ttl time.Duration) error {
	if ttl == 0 {
		ttl = c.defaultTTL
	}
	
	key := c.generatePopulationKey(variant)
	
	cached := CachedPopulationData{
		Data:      data,
		CachedAt:  time.Now(),
		ExpiresAt: time.Now().Add(ttl),
	}
	
	jsonData, err := json.Marshal(cached)
	if err != nil {
		return fmt.Errorf("failed to marshal population cache data: %w", err)
	}
	
	return c.redis.Set(ctx, key, jsonData, ttl).Err()
}

// GetSomaticData retrieves cached somatic data
func (c *CacheClient) GetSomaticData(ctx context.Context, variant *domain.StandardizedVariant) (*domain.SomaticData, bool, error) {
	key := c.generateSomaticKey(variant)
	
	val, err := c.redis.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, false, nil // Cache miss
	}
	if err != nil {
		return nil, false, fmt.Errorf("failed to get somatic cache: %w", err)
	}
	
	var cached CachedSomaticData
	if err := json.Unmarshal([]byte(val), &cached); err != nil {
		// Remove corrupted cache entry
		c.redis.Del(ctx, key)
		return nil, false, nil
	}
	
	// Check if expired
	if time.Now().After(cached.ExpiresAt) {
		c.redis.Del(ctx, key)
		return nil, false, nil
	}
	
	return cached.Data, true, nil
}

// SetSomaticData caches somatic data
func (c *CacheClient) SetSomaticData(ctx context.Context, variant *domain.StandardizedVariant, data *domain.SomaticData, ttl time.Duration) error {
	if ttl == 0 {
		ttl = c.defaultTTL
	}
	
	key := c.generateSomaticKey(variant)
	
	cached := CachedSomaticData{
		Data:      data,
		CachedAt:  time.Now(),
		ExpiresAt: time.Now().Add(ttl),
	}
	
	jsonData, err := json.Marshal(cached)
	if err != nil {
		return fmt.Errorf("failed to marshal somatic cache data: %w", err)
	}
	
	return c.redis.Set(ctx, key, jsonData, ttl).Err()
}

// InvalidateVariant removes all cached data for a specific variant
func (c *CacheClient) InvalidateVariant(ctx context.Context, variant *domain.StandardizedVariant) error {
	keys := []string{
		c.generateClinVarKey(variant),
		c.generatePopulationKey(variant),
		c.generateSomaticKey(variant),
	}
	
	return c.redis.Del(ctx, keys...).Err()
}

// InvalidatePattern removes all cached data matching a pattern
func (c *CacheClient) InvalidatePattern(ctx context.Context, pattern string) error {
	keys, err := c.redis.Keys(ctx, pattern).Result()
	if err != nil {
		return fmt.Errorf("failed to get keys for pattern %s: %w", pattern, err)
	}
	
	if len(keys) == 0 {
		return nil
	}
	
	return c.redis.Del(ctx, keys...).Err()
}

// GetStats returns cache statistics
func (c *CacheClient) GetStats(ctx context.Context) (map[string]interface{}, error) {
	info, err := c.redis.Info(ctx, "memory", "stats").Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get Redis info: %w", err)
	}
	
	keyspace, err := c.redis.Info(ctx, "keyspace").Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get Redis keyspace: %w", err)
	}
	
	stats := map[string]interface{}{
		"memory_info": info,
		"keyspace":    keyspace,
		"client_info": map[string]interface{}{
			"pool_stats": c.redis.PoolStats(),
		},
	}
	
	return stats, nil
}

// Close closes the Redis connection
func (c *CacheClient) Close() error {
	return c.redis.Close()
}

// generateClinVarKey creates a cache key for ClinVar data
func (c *CacheClient) generateClinVarKey(variant *domain.StandardizedVariant) string {
	return c.generateVariantKey("clinvar", variant)
}

// generatePopulationKey creates a cache key for population data
func (c *CacheClient) generatePopulationKey(variant *domain.StandardizedVariant) string {
	return c.generateVariantKey("population", variant)
}

// generateSomaticKey creates a cache key for somatic data
func (c *CacheClient) generateSomaticKey(variant *domain.StandardizedVariant) string {
	return c.generateVariantKey("somatic", variant)
}

// generateVariantKey creates a standardized cache key for a variant
func (c *CacheClient) generateVariantKey(prefix string, variant *domain.StandardizedVariant) string {
	// Create a hash of variant identifying information
	data := fmt.Sprintf("%s:%d:%s:%s:%s:%s:%s", 
		variant.Chromosome, variant.Position, variant.Reference, variant.Alternative,
		variant.HGVSGenomic, variant.HGVSCoding, variant.HGVSProtein)
	
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%s:variant:%x", prefix, hash[:8]) // Use first 8 bytes of hash
}

// Ping checks if Redis connection is alive
func (c *CacheClient) Ping(ctx context.Context) error {
	return c.redis.Ping(ctx).Err()
}

// FlushAll removes all cache entries (use with caution!)
func (c *CacheClient) FlushAll(ctx context.Context) error {
	return c.redis.FlushAll(ctx).Err()
}