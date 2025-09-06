package resources

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// ResourceManager manages MCP resources and their providers
type ResourceManager struct {
	logger    *logrus.Logger
	providers map[string]ResourceProvider
	cache     *ResourceCache
	mutex     sync.RWMutex
}

// ResourceProvider defines the interface for resource providers
type ResourceProvider interface {
	// GetResource retrieves a resource by URI
	GetResource(ctx context.Context, uri string) (*ResourceContent, error)
	
	// ListResources lists available resources with optional filtering
	ListResources(ctx context.Context, cursor string) (*ResourceList, error)
	
	// GetResourceInfo returns metadata about a resource
	GetResourceInfo(ctx context.Context, uri string) (*ResourceInfo, error)
	
	// SupportsURI checks if this provider can handle the given URI
	SupportsURI(uri string) bool
	
	// GetProviderInfo returns information about this provider
	GetProviderInfo() ProviderInfo
}

// ResourceContent represents the content of a resource
type ResourceContent struct {
	URI         string                 `json:"uri"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	MimeType    string                 `json:"mimeType"`
	Content     interface{}            `json:"content"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	LastModified time.Time             `json:"lastModified"`
	ETag        string                 `json:"etag,omitempty"`
}

// ResourceList represents a list of available resources
type ResourceList struct {
	Resources []ResourceInfo `json:"resources"`
	NextCursor string        `json:"nextCursor,omitempty"`
	Total      int           `json:"total"`
}

// ResourceInfo provides metadata about a resource
type ResourceInfo struct {
	URI         string                 `json:"uri"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	MimeType    string                 `json:"mimeType"`
	Size        int64                  `json:"size,omitempty"`
	LastModified time.Time             `json:"lastModified"`
	Tags        []string               `json:"tags,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ProviderInfo contains metadata about a resource provider
type ProviderInfo struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Version     string   `json:"version"`
	URIPatterns []string `json:"uriPatterns"`
}

// ResourceCache provides caching for resource content
type ResourceCache struct {
	cache      map[string]*CacheEntry
	mutex      sync.RWMutex
	maxSize    int
	defaultTTL time.Duration
	logger     *logrus.Logger
}

// CacheEntry represents a cached resource
type CacheEntry struct {
	Content    *ResourceContent
	Timestamp  time.Time
	AccessCount int
	TTL        time.Duration
}

// NewResourceManager creates a new resource manager
func NewResourceManager(logger *logrus.Logger) *ResourceManager {
	return &ResourceManager{
		logger:    logger,
		providers: make(map[string]ResourceProvider),
		cache:     NewResourceCache(logger),
	}
}

// NewResourceCache creates a new resource cache
func NewResourceCache(logger *logrus.Logger) *ResourceCache {
	cache := &ResourceCache{
		cache:      make(map[string]*CacheEntry),
		maxSize:    1000, // Maximum cached resources
		defaultTTL: 5 * time.Minute,
		logger:     logger,
	}
	
	// Start cleanup routine
	go cache.cleanupRoutine()
	
	return cache
}

// RegisterProvider registers a new resource provider
func (rm *ResourceManager) RegisterProvider(name string, provider ResourceProvider) {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()
	
	rm.providers[name] = provider
	rm.logger.WithFields(logrus.Fields{
		"provider": name,
		"patterns": provider.GetProviderInfo().URIPatterns,
	}).Info("Registered resource provider")
}

// GetResource retrieves a resource by URI
func (rm *ResourceManager) GetResource(ctx context.Context, uri string) (*ResourceContent, error) {
	rm.logger.WithField("uri", uri).Debug("Getting resource")
	
	// Check cache first
	if cached := rm.cache.Get(uri); cached != nil {
		rm.logger.WithField("uri", uri).Debug("Resource cache hit")
		return cached, nil
	}
	
	// Find appropriate provider
	provider := rm.findProvider(uri)
	if provider == nil {
		return nil, fmt.Errorf("no provider found for URI: %s", uri)
	}
	
	// Get resource from provider
	content, err := provider.GetResource(ctx, uri)
	if err != nil {
		return nil, fmt.Errorf("provider error for URI %s: %w", uri, err)
	}
	
	// Cache the result
	rm.cache.Set(uri, content, rm.cache.defaultTTL)
	
	rm.logger.WithFields(logrus.Fields{
		"uri":      uri,
		"provider": provider.GetProviderInfo().Name,
		"size":     len(fmt.Sprintf("%v", content.Content)),
	}).Info("Resource retrieved successfully")
	
	return content, nil
}

// ListResources lists all available resources
func (rm *ResourceManager) ListResources(ctx context.Context, cursor string) (*ResourceList, error) {
	rm.logger.WithField("cursor", cursor).Debug("Listing resources")
	
	allResources := make([]ResourceInfo, 0)
	
	rm.mutex.RLock()
	for _, provider := range rm.providers {
		list, err := provider.ListResources(ctx, cursor)
		if err != nil {
			rm.logger.WithError(err).WithField("provider", provider.GetProviderInfo().Name).
				Warn("Failed to list resources from provider")
			continue
		}
		
		allResources = append(allResources, list.Resources...)
	}
	rm.mutex.RUnlock()
	
	result := &ResourceList{
		Resources: allResources,
		Total:     len(allResources),
	}
	
	rm.logger.WithField("count", len(allResources)).Info("Listed resources")
	return result, nil
}

// GetResourceInfo returns metadata about a resource
func (rm *ResourceManager) GetResourceInfo(ctx context.Context, uri string) (*ResourceInfo, error) {
	provider := rm.findProvider(uri)
	if provider == nil {
		return nil, fmt.Errorf("no provider found for URI: %s", uri)
	}
	
	return provider.GetResourceInfo(ctx, uri)
}

// findProvider finds the appropriate provider for a URI
func (rm *ResourceManager) findProvider(uri string) ResourceProvider {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()
	
	for _, provider := range rm.providers {
		if provider.SupportsURI(uri) {
			return provider
		}
	}
	
	return nil
}

// GetProviderInfo returns information about all registered providers
func (rm *ResourceManager) GetProviderInfo() []ProviderInfo {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()
	
	info := make([]ProviderInfo, 0, len(rm.providers))
	for _, provider := range rm.providers {
		info = append(info, provider.GetProviderInfo())
	}
	
	return info
}

// Cache methods

// Get retrieves a resource from cache
func (rc *ResourceCache) Get(uri string) *ResourceContent {
	rc.mutex.RLock()
	defer rc.mutex.RUnlock()
	
	entry, exists := rc.cache[uri]
	if !exists {
		return nil
	}
	
	// Check if expired
	if time.Since(entry.Timestamp) > entry.TTL {
		delete(rc.cache, uri)
		return nil
	}
	
	// Update access statistics
	entry.AccessCount++
	
	rc.logger.WithFields(logrus.Fields{
		"uri":          uri,
		"access_count": entry.AccessCount,
		"age":          time.Since(entry.Timestamp).String(),
	}).Debug("Resource cache hit")
	
	return entry.Content
}

// Set stores a resource in cache
func (rc *ResourceCache) Set(uri string, content *ResourceContent, ttl time.Duration) {
	rc.mutex.Lock()
	defer rc.mutex.Unlock()
	
	// Check cache size limit
	if len(rc.cache) >= rc.maxSize {
		rc.evictLRU()
	}
	
	entry := &CacheEntry{
		Content:     content,
		Timestamp:   time.Now(),
		AccessCount: 1,
		TTL:         ttl,
	}
	
	rc.cache[uri] = entry
	
	rc.logger.WithFields(logrus.Fields{
		"uri":        uri,
		"cache_size": len(rc.cache),
		"ttl":        ttl.String(),
	}).Debug("Cached resource")
}

// evictLRU removes the least recently used entry
func (rc *ResourceCache) evictLRU() {
	var oldestURI string
	var oldestTime time.Time = time.Now()
	var lowestAccess int = int(^uint(0) >> 1) // Max int
	
	for uri, entry := range rc.cache {
		if entry.Timestamp.Before(oldestTime) || 
		   (entry.Timestamp.Equal(oldestTime) && entry.AccessCount < lowestAccess) {
			oldestTime = entry.Timestamp
			lowestAccess = entry.AccessCount
			oldestURI = uri
		}
	}
	
	if oldestURI != "" {
		delete(rc.cache, oldestURI)
		rc.logger.WithField("uri", oldestURI).Debug("Evicted resource from cache")
	}
}

// cleanupRoutine periodically removes expired entries
func (rc *ResourceCache) cleanupRoutine() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	
	for range ticker.C {
		rc.cleanup()
	}
}

// cleanup removes expired entries
func (rc *ResourceCache) cleanup() {
	rc.mutex.Lock()
	defer rc.mutex.Unlock()
	
	now := time.Now()
	expired := make([]string, 0)
	
	for uri, entry := range rc.cache {
		if now.Sub(entry.Timestamp) > entry.TTL {
			expired = append(expired, uri)
		}
	}
	
	for _, uri := range expired {
		delete(rc.cache, uri)
	}
	
	if len(expired) > 0 {
		rc.logger.WithFields(logrus.Fields{
			"expired_count": len(expired),
			"cache_size":    len(rc.cache),
		}).Debug("Cleaned up expired cache entries")
	}
}

// GetCacheStats returns cache statistics
func (rc *ResourceCache) GetCacheStats() map[string]interface{} {
	rc.mutex.RLock()
	defer rc.mutex.RUnlock()
	
	totalAccess := 0
	for _, entry := range rc.cache {
		totalAccess += entry.AccessCount
	}
	
	stats := map[string]interface{}{
		"total_entries":   len(rc.cache),
		"max_size":        rc.maxSize,
		"total_accesses":  totalAccess,
		"default_ttl":     rc.defaultTTL.String(),
	}
	
	if len(rc.cache) > 0 {
		stats["average_accesses"] = float64(totalAccess) / float64(len(rc.cache))
	}
	
	return stats
}

// URIParser provides utilities for parsing resource URIs
type URIParser struct {
	patterns map[string]*regexp.Regexp
}

// NewURIParser creates a new URI parser
func NewURIParser() *URIParser {
	return &URIParser{
		patterns: make(map[string]*regexp.Regexp),
	}
}

// AddPattern adds a URI pattern for matching
func (up *URIParser) AddPattern(name, pattern string) error {
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("invalid regex pattern %s: %w", pattern, err)
	}
	
	up.patterns[name] = regex
	return nil
}

// ParseURI parses a URI and extracts parameters
func (up *URIParser) ParseURI(uri string) (string, map[string]string, error) {
	// Decode URI first
	decoded, err := url.QueryUnescape(uri)
	if err != nil {
		return "", nil, fmt.Errorf("failed to decode URI: %w", err)
	}
	
	for name, pattern := range up.patterns {
		if matches := pattern.FindStringSubmatch(decoded); matches != nil {
			params := make(map[string]string)
			
			// Extract named groups
			for i, subname := range pattern.SubexpNames() {
				if i > 0 && i < len(matches) && subname != "" {
					params[subname] = matches[i]
				}
			}
			
			return name, params, nil
		}
	}
	
	return "", nil, fmt.Errorf("no pattern matches URI: %s", uri)
}

// ValidateURI validates a URI format
func (up *URIParser) ValidateURI(uri string) error {
	if uri == "" {
		return fmt.Errorf("URI cannot be empty")
	}
	
	if !strings.HasPrefix(uri, "/") {
		return fmt.Errorf("URI must start with /")
	}
	
	// Parse as URL to check validity
	if _, err := url.Parse(uri); err != nil {
		return fmt.Errorf("invalid URI format: %w", err)
	}
	
	return nil
}

// ExpandURITemplate expands a URI template with parameters
func (up *URIParser) ExpandURITemplate(template string, params map[string]string) string {
	result := template
	for key, value := range params {
		placeholder := fmt.Sprintf("{%s}", key)
		result = strings.ReplaceAll(result, placeholder, value)
	}
	return result
}