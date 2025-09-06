package resources

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResourceManager_RegisterProvider(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	
	manager := NewResourceManager(logger)
	
	// Create test provider
	provider := NewVariantResourceProvider(logger)
	
	// Register provider
	manager.RegisterProvider("variant", provider)
	
	// Verify provider is registered
	providers := manager.GetProviderInfo()
	require.Len(t, providers, 1)
	assert.Equal(t, "variant", providers[0].Name)
}

func TestResourceManager_GetResource(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	
	manager := NewResourceManager(logger)
	
	// Register providers
	manager.RegisterProvider("variant", NewVariantResourceProvider(logger))
	manager.RegisterProvider("interpretation", NewInterpretationResourceProvider(logger))
	manager.RegisterProvider("evidence", NewEvidenceResourceProvider(logger))
	manager.RegisterProvider("acmg_rules", NewACMGRulesResourceProvider(logger))
	
	tests := []struct {
		name        string
		uri         string
		expectError bool
	}{
		{
			name:        "Valid variant resource",
			uri:         "/variant/123",
			expectError: false,
		},
		{
			name:        "Valid interpretation resource",
			uri:         "/interpretation/456",
			expectError: false,
		},
		{
			name:        "Valid evidence resource",
			uri:         "/evidence/789/summary",
			expectError: false,
		},
		{
			name:        "Valid ACMG rules resource",
			uri:         "/acmg/rules",
			expectError: false,
		},
		{
			name:        "Invalid URI - no provider",
			uri:         "/unknown/resource",
			expectError: true,
		},
		{
			name:        "Invalid URI format",
			uri:         "invalid-uri",
			expectError: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			resource, err := manager.GetResource(ctx, tt.uri)
			
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, resource)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resource)
				assert.Equal(t, tt.uri, resource.URI)
				assert.NotEmpty(t, resource.Name)
				assert.NotEmpty(t, resource.Content)
				assert.Equal(t, "application/json", resource.MimeType)
			}
		})
	}
}

func TestResourceManager_Caching(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	
	manager := NewResourceManager(logger)
	manager.RegisterProvider("variant", NewVariantResourceProvider(logger))
	
	ctx := context.Background()
	uri := "/variant/test123"
	
	// First request - should hit provider
	resource1, err := manager.GetResource(ctx, uri)
	require.NoError(t, err)
	require.NotNil(t, resource1)
	
	// Second request - should hit cache
	resource2, err := manager.GetResource(ctx, uri)
	require.NoError(t, err)
	require.NotNil(t, resource2)
	
	// Should be same content
	assert.Equal(t, resource1.URI, resource2.URI)
	assert.Equal(t, resource1.Name, resource2.Name)
}

func TestResourceManager_ListResources(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	
	manager := NewResourceManager(logger)
	
	// Register all providers
	manager.RegisterProvider("variant", NewVariantResourceProvider(logger))
	manager.RegisterProvider("interpretation", NewInterpretationResourceProvider(logger))
	manager.RegisterProvider("evidence", NewEvidenceResourceProvider(logger))
	manager.RegisterProvider("acmg_rules", NewACMGRulesResourceProvider(logger))
	
	ctx := context.Background()
	resourceList, err := manager.ListResources(ctx, "")
	
	require.NoError(t, err)
	require.NotNil(t, resourceList)
	
	// Should have resources from all providers
	assert.Greater(t, len(resourceList.Resources), 20) // Each provider has multiple resources
	assert.Equal(t, len(resourceList.Resources), resourceList.Total)
	
	// Check that resources have required fields
	for _, resource := range resourceList.Resources {
		assert.NotEmpty(t, resource.URI)
		assert.NotEmpty(t, resource.Name)
		assert.NotEmpty(t, resource.MimeType)
		assert.NotZero(t, resource.LastModified)
	}
}

func TestResourceManager_GetResourceInfo(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	
	manager := NewResourceManager(logger)
	manager.RegisterProvider("variant", NewVariantResourceProvider(logger))
	
	ctx := context.Background()
	
	// Test valid URI
	info, err := manager.GetResourceInfo(ctx, "/variant/test123")
	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Equal(t, "/variant/test123", info.URI)
	assert.NotEmpty(t, info.Name)
	assert.Equal(t, "application/json", info.MimeType)
	
	// Test invalid URI
	info, err = manager.GetResourceInfo(ctx, "/unknown/resource")
	assert.Error(t, err)
	assert.Nil(t, info)
}

func TestResourceCache_SetGet(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	
	cache := NewResourceCache(logger)
	
	content := &ResourceContent{
		URI:         "/test/uri",
		Name:        "Test Resource",
		Description: "Test Description",
		MimeType:    "application/json",
		Content:     map[string]interface{}{"test": "data"},
		LastModified: time.Now(),
	}
	
	// Set in cache
	cache.Set("/test/uri", content, 5*time.Minute)
	
	// Get from cache
	retrieved := cache.Get("/test/uri")
	require.NotNil(t, retrieved)
	assert.Equal(t, content.URI, retrieved.URI)
	assert.Equal(t, content.Name, retrieved.Name)
}

func TestResourceCache_Expiration(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	
	cache := NewResourceCache(logger)
	
	content := &ResourceContent{
		URI:         "/test/uri",
		Name:        "Test Resource",
		MimeType:    "application/json",
		Content:     map[string]interface{}{"test": "data"},
		LastModified: time.Now(),
	}
	
	// Set with very short TTL
	cache.Set("/test/uri", content, 1*time.Millisecond)
	
	// Wait for expiration
	time.Sleep(10 * time.Millisecond)
	
	// Should be expired
	retrieved := cache.Get("/test/uri")
	assert.Nil(t, retrieved)
}

func TestResourceCache_LRUEviction(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	
	// Create cache with small max size for testing
	cache := &ResourceCache{
		cache:      make(map[string]*CacheEntry),
		maxSize:    2, // Very small for testing
		defaultTTL: 5 * time.Minute,
		logger:     logger,
	}
	
	content1 := &ResourceContent{URI: "/test/1", Name: "Test 1", MimeType: "application/json", Content: map[string]interface{}{}, LastModified: time.Now()}
	content2 := &ResourceContent{URI: "/test/2", Name: "Test 2", MimeType: "application/json", Content: map[string]interface{}{}, LastModified: time.Now()}
	content3 := &ResourceContent{URI: "/test/3", Name: "Test 3", MimeType: "application/json", Content: map[string]interface{}{}, LastModified: time.Now()}
	
	// Fill cache to capacity
	cache.Set("/test/1", content1, 5*time.Minute)
	cache.Set("/test/2", content2, 5*time.Minute)
	
	// Access first item to make it more recently used
	cache.Get("/test/1")
	
	// Add third item - should evict second item (LRU)
	cache.Set("/test/3", content3, 5*time.Minute)
	
	// First item should still be there
	assert.NotNil(t, cache.Get("/test/1"))
	
	// Second item should be evicted
	assert.Nil(t, cache.Get("/test/2"))
	
	// Third item should be there
	assert.NotNil(t, cache.Get("/test/3"))
}

func TestResourceCache_Stats(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	
	cache := NewResourceCache(logger)
	
	// Add some items
	content1 := &ResourceContent{URI: "/test/1", MimeType: "application/json", Content: map[string]interface{}{}, LastModified: time.Now()}
	content2 := &ResourceContent{URI: "/test/2", MimeType: "application/json", Content: map[string]interface{}{}, LastModified: time.Now()}
	
	cache.Set("/test/1", content1, 5*time.Minute)
	cache.Set("/test/2", content2, 5*time.Minute)
	
	// Access items
	cache.Get("/test/1")
	cache.Get("/test/1")
	cache.Get("/test/2")
	
	stats := cache.GetCacheStats()
	
	assert.Equal(t, 2, stats["total_entries"])
	assert.Equal(t, 1000, stats["max_size"])
	assert.Equal(t, 3, stats["total_accesses"])
	assert.Equal(t, 1.5, stats["average_accesses"])
}

func TestURIParser_ParseURI(t *testing.T) {
	parser := NewURIParser()
	
	// Add test patterns
	err := parser.AddPattern("variant", `^/variant/(?P<id>[^/]+)$`)
	require.NoError(t, err)
	
	err = parser.AddPattern("variant_transcripts", `^/variant/(?P<id>[^/]+)/transcripts$`)
	require.NoError(t, err)
	
	tests := []struct {
		name           string
		uri            string
		expectedPattern string
		expectedParams map[string]string
		expectError    bool
	}{
		{
			name:           "Basic variant URI",
			uri:            "/variant/123",
			expectedPattern: "variant",
			expectedParams: map[string]string{"id": "123"},
			expectError:    false,
		},
		{
			name:           "Variant transcripts URI",
			uri:            "/variant/456/transcripts",
			expectedPattern: "variant_transcripts",
			expectedParams: map[string]string{"id": "456"},
			expectError:    false,
		},
		{
			name:           "URI with special characters",
			uri:            "/variant/NM_000001.3:c.123A>G",
			expectedPattern: "variant",
			expectedParams: map[string]string{"id": "NM_000001.3:c.123A>G"},
			expectError:    false,
		},
		{
			name:        "No matching pattern",
			uri:         "/unknown/path",
			expectError: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patternName, params, err := parser.ParseURI(tt.uri)
			
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedPattern, patternName)
				assert.Equal(t, tt.expectedParams, params)
			}
		})
	}
}

func TestURIParser_ValidateURI(t *testing.T) {
	parser := NewURIParser()
	
	tests := []struct {
		name        string
		uri         string
		expectError bool
	}{
		{
			name:        "Valid URI",
			uri:         "/variant/123",
			expectError: false,
		},
		{
			name:        "Empty URI",
			uri:         "",
			expectError: true,
		},
		{
			name:        "URI not starting with /",
			uri:         "variant/123",
			expectError: true,
		},
		{
			name:        "Valid complex URI",
			uri:         "/evidence/NM_000001.3:c.123A>G/population",
			expectError: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := parser.ValidateURI(tt.uri)
			
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestURIParser_ExpandURITemplate(t *testing.T) {
	parser := NewURIParser()
	
	template := "/variant/{id}/analysis/{type}"
	params := map[string]string{
		"id":   "NM_000001.3:c.123A>G",
		"type": "functional",
	}
	
	result := parser.ExpandURITemplate(template, params)
	expected := "/variant/NM_000001.3:c.123A>G/analysis/functional"
	
	assert.Equal(t, expected, result)
}

// Integration tests for all resource providers
func TestAllResourceProviders_Integration(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	
	ctx := context.Background()
	
	// Test all providers
	providers := map[string]ResourceProvider{
		"variant":      NewVariantResourceProvider(logger),
		"interpretation": NewInterpretationResourceProvider(logger),
		"evidence":     NewEvidenceResourceProvider(logger),
		"acmg_rules":   NewACMGRulesResourceProvider(logger),
	}
	
	// Test URIs for each provider
	testURIs := map[string][]string{
		"variant": {
			"/variant/123",
			"/variant/hgvs/NM_000001.3:c.123A>G",
			"/variant/123/transcripts",
			"/variant/123/clinical",
		},
		"interpretation": {
			"/interpretation/456",
			"/interpretation/456/classification",
			"/interpretation/456/evidence",
			"/interpretation/456/rules",
		},
		"evidence": {
			"/evidence/789",
			"/evidence/789/summary",
			"/evidence/789/population",
			"/evidence/789/clinical",
		},
		"acmg_rules": {
			"/acmg/rules",
			"/acmg/rules/pathogenic",
			"/acmg/rules/benign",
			"/acmg/rules/combinations",
		},
	}
	
	for providerName, provider := range providers {
		t.Run(providerName, func(t *testing.T) {
			// Test provider info
			info := provider.GetProviderInfo()
			assert.NotEmpty(t, info.Name)
			assert.NotEmpty(t, info.Description)
			assert.NotEmpty(t, info.Version)
			assert.NotEmpty(t, info.URIPatterns)
			
			// Test list resources
			list, err := provider.ListResources(ctx, "")
			assert.NoError(t, err)
			assert.NotNil(t, list)
			assert.Greater(t, len(list.Resources), 0)
			assert.Equal(t, len(list.Resources), list.Total)
			
			// Test each URI
			for _, uri := range testURIs[providerName] {
				t.Run(uri, func(t *testing.T) {
					// Test SupportsURI
					supported := provider.SupportsURI(uri)
					assert.True(t, supported, "Provider should support URI: %s", uri)
					
					// Test GetResource
					resource, err := provider.GetResource(ctx, uri)
					assert.NoError(t, err)
					assert.NotNil(t, resource)
					assert.Equal(t, uri, resource.URI)
					assert.NotEmpty(t, resource.Name)
					assert.NotEmpty(t, resource.Description)
					assert.Equal(t, "application/json", resource.MimeType)
					assert.NotNil(t, resource.Content)
					assert.NotZero(t, resource.LastModified)
					assert.NotEmpty(t, resource.ETag)
					assert.NotNil(t, resource.Metadata)
					
					// Test GetResourceInfo
					info, err := provider.GetResourceInfo(ctx, uri)
					assert.NoError(t, err)
					assert.NotNil(t, info)
					assert.Equal(t, uri, info.URI)
					assert.NotEmpty(t, info.Name)
					assert.Equal(t, "application/json", info.MimeType)
				})
			}
		})
	}
}

func TestResourceManager_ConcurrentAccess(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	
	manager := NewResourceManager(logger)
	manager.RegisterProvider("variant", NewVariantResourceProvider(logger))
	
	ctx := context.Background()
	uri := "/variant/concurrent-test"
	
	// Test concurrent access
	const numGoroutines = 10
	const requestsPerGoroutine = 5
	
	done := make(chan bool, numGoroutines)
	
	for i := 0; i < numGoroutines; i++ {
		go func(routineID int) {
			defer func() { done <- true }()
			
			for j := 0; j < requestsPerGoroutine; j++ {
				resource, err := manager.GetResource(ctx, uri)
				assert.NoError(t, err)
				assert.NotNil(t, resource)
				assert.Equal(t, uri, resource.URI)
			}
		}(i)
	}
	
	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}
	
	// Verify cache still works after concurrent access
	resource, err := manager.GetResource(ctx, uri)
	assert.NoError(t, err)
	assert.NotNil(t, resource)
}

func TestResourceProviders_ErrorHandling(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	
	ctx := context.Background()
	providers := []ResourceProvider{
		NewVariantResourceProvider(logger),
		NewInterpretationResourceProvider(logger),
		NewEvidenceResourceProvider(logger),
		NewACMGRulesResourceProvider(logger),
	}
	
	// Test invalid URIs
	invalidURIs := []string{
		"", // Empty URI
		"invalid-uri", // No leading slash
		"/variant/", // Missing parameter
		"/unknown/provider", // Unsupported provider
	}
	
	for _, provider := range providers {
		t.Run(provider.GetProviderInfo().Name, func(t *testing.T) {
			for _, uri := range invalidURIs {
				t.Run(uri, func(t *testing.T) {
					// SupportsURI should handle invalid URIs gracefully
					supported := provider.SupportsURI(uri)
					
					if supported {
						// If provider claims support, GetResource should handle gracefully
						_, _ = provider.GetResource(ctx, uri)
						// We expect either success or a descriptive error, not a panic
						// The specific error type depends on the provider implementation
					}
				})
			}
		})
	}
}