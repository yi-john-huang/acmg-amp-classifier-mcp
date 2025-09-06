package health

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Mock database for testing
type MockDB struct {
	mock.Mock
	pingError error
	stats     sql.DBStats
}

func (m *MockDB) PingContext(ctx context.Context) error {
	return m.pingError
}

func (m *MockDB) Stats() sql.DBStats {
	return m.stats
}

// Mock Redis client for testing
type MockRedisClient struct {
	mock.Mock
	pingError error
	info      string
}

func (m *MockRedisClient) Ping(ctx context.Context) *redis.StatusCmd {
	cmd := redis.NewStatusCmd(ctx)
	if m.pingError != nil {
		cmd.SetErr(m.pingError)
	} else {
		cmd.SetVal("PONG")
	}
	return cmd
}

func (m *MockRedisClient) Info(ctx context.Context, sections ...string) *redis.StringCmd {
	cmd := redis.NewStringCmd(ctx)
	cmd.SetVal(m.info)
	return cmd
}

func TestHealthChecker_NewHealthChecker(t *testing.T) {
	config := HealthConfig{
		CheckInterval:    30 * time.Second,
		Timeout:          10 * time.Second,
		EnabledChecks:    []string{"database", "redis"},
		AlertingEnabled:  true,
		DetailedResponse: true,
		Thresholds: HealthThresholds{
			DatabaseMaxLatency: 100 * time.Millisecond,
			RedisMaxLatency:    50 * time.Millisecond,
		},
	}

	hc := NewHealthChecker(config, nil, nil)

	assert.NotNil(t, hc)
	assert.Equal(t, config, hc.config)
	assert.NotNil(t, hc.checks)
	assert.NotNil(t, hc.status)
	assert.Equal(t, HealthStateUnknown, hc.status.Overall)
}

func TestHealthChecker_RegisterCheck(t *testing.T) {
	config := HealthConfig{}
	hc := NewHealthChecker(config, nil, nil)

	// Create a mock health check
	mockCheck := &MockHealthCheck{
		name: "test_check",
		result: ComponentHealth{
			Name:    "test_check",
			Status:  HealthStateHealthy,
			Message: "Test check is healthy",
		},
	}

	hc.RegisterCheck(mockCheck)

	assert.Contains(t, hc.checks, "test_check")
	assert.Equal(t, mockCheck, hc.checks["test_check"])
}

func TestHealthChecker_DatabaseHealthCheck_Healthy(t *testing.T) {
	mockDB := &MockDB{
		pingError: nil,
		stats: sql.DBStats{
			OpenConnections: 5,
			InUse:          2,
			Idle:           3,
			WaitCount:      10,
			WaitDuration:   time.Millisecond,
		},
	}

	check := &DatabaseHealthCheck{
		db:      mockDB,
		timeout: 5 * time.Second,
	}

	ctx := context.Background()
	result := check.Check(ctx)

	assert.Equal(t, "database", result.Name)
	assert.Equal(t, HealthStateHealthy, result.Status)
	assert.Contains(t, result.Message, "healthy")
	assert.NoError(t, result.Error)
	assert.NotNil(t, result.Metadata)
	assert.Equal(t, 5, result.Metadata["open_connections"])
}

func TestHealthChecker_DatabaseHealthCheck_Unhealthy(t *testing.T) {
	mockDB := &MockDB{
		pingError: assert.AnError,
	}

	check := &DatabaseHealthCheck{
		db:      mockDB,
		timeout: 5 * time.Second,
	}

	ctx := context.Background()
	result := check.Check(ctx)

	assert.Equal(t, "database", result.Name)
	assert.Equal(t, HealthStateUnhealthy, result.Status)
	assert.Contains(t, result.Message, "failed")
	assert.Contains(t, result.Error, assert.AnError.Error())
}

func TestHealthChecker_DatabaseHealthCheck_Warning(t *testing.T) {
	mockDB := &MockDB{
		pingError: nil,
		stats: sql.DBStats{
			OpenConnections: 5,
			InUse:          2,
			Idle:           3,
			WaitCount:      150, // High wait count should trigger warning
			WaitDuration:   time.Millisecond,
		},
	}

	check := &DatabaseHealthCheck{
		db:      mockDB,
		timeout: 5 * time.Second,
	}

	ctx := context.Background()
	result := check.Check(ctx)

	assert.Equal(t, "database", result.Name)
	assert.Equal(t, HealthStateWarning, result.Status)
	assert.Contains(t, result.Message, "High database connection wait count")
}

func TestHealthChecker_RedisHealthCheck_Healthy(t *testing.T) {
	mockRedis := &MockRedisClient{
		pingError: nil,
		info:      "redis_version:6.2.0\r\nconnected_clients:10\r\nused_memory:1024000",
	}

	check := &RedisHealthCheck{
		client:  mockRedis,
		timeout: 5 * time.Second,
	}

	ctx := context.Background()
	result := check.Check(ctx)

	assert.Equal(t, "redis", result.Name)
	assert.Equal(t, HealthStateHealthy, result.Status)
	assert.Contains(t, result.Message, "healthy")
	assert.Empty(t, result.Error)
	assert.NotNil(t, result.Metadata)
}

func TestHealthChecker_RedisHealthCheck_Unhealthy(t *testing.T) {
	mockRedis := &MockRedisClient{
		pingError: assert.AnError,
	}

	check := &RedisHealthCheck{
		client:  mockRedis,
		timeout: 5 * time.Second,
	}

	ctx := context.Background()
	result := check.Check(ctx)

	assert.Equal(t, "redis", result.Name)
	assert.Equal(t, HealthStateUnhealthy, result.Status)
	assert.Contains(t, result.Message, "failed")
	assert.Contains(t, result.Error, assert.AnError.Error())
}

func TestHealthChecker_SystemResourceHealthCheck(t *testing.T) {
	thresholds := HealthThresholds{
		MemoryMaxUsage: 1024 * 1024 * 1024, // 1GB
		CPUMaxUsage:    80.0,
	}

	check := &SystemResourceHealthCheck{
		thresholds: thresholds,
	}

	ctx := context.Background()
	result := check.Check(ctx)

	assert.Equal(t, "system_resources", result.Name)
	assert.Equal(t, HealthStateHealthy, result.Status) // Mock always returns healthy
	assert.Contains(t, result.Message, "healthy")
	assert.NotNil(t, result.Metadata)
}

func TestHealthChecker_MCPToolsHealthCheck(t *testing.T) {
	toolRegistry := map[string]interface{}{
		"classify_variant": struct{}{},
		"validate_hgvs":    struct{}{},
		"apply_rule":       struct{}{},
	}

	check := &MCPToolsHealthCheck{
		toolRegistry: toolRegistry,
		timeout:      5 * time.Second,
	}

	ctx := context.Background()
	result := check.Check(ctx)

	assert.Equal(t, "mcp_tools", result.Name)
	assert.Equal(t, HealthStateHealthy, result.Status)
	assert.Contains(t, result.Message, "healthy")
	assert.Contains(t, result.Message, "tools available")
	assert.NotNil(t, result.Metadata)

	toolsAvailable := result.Metadata["tools_available"].([]string)
	assert.Contains(t, toolsAvailable, "classify_variant")
	assert.Contains(t, toolsAvailable, "validate_hgvs")
}

func TestHealthChecker_ExternalAPIHealthCheck(t *testing.T) {
	// Create test servers
	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server1.Close()

	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server2.Close()

	endpoints := []APIEndpoint{
		{Name: "API1", URL: server1.URL, Method: "GET", Expected: 200},
		{Name: "API2", URL: server2.URL, Method: "GET", Expected: 200},
	}

	check := &ExternalAPIHealthCheck{
		endpoints: endpoints,
		timeout:   5 * time.Second,
	}

	ctx := context.Background()
	result := check.Check(ctx)

	assert.Equal(t, "external_apis", result.Name)
	assert.Equal(t, HealthStateWarning, result.Status) // 1/2 APIs healthy
	assert.Contains(t, result.Message, "1/2 external APIs healthy")
	
	metadata := result.Metadata
	apiResults := metadata["api_results"].(map[string]string)
	assert.Equal(t, "healthy", apiResults["API1"])
	assert.Contains(t, apiResults["API2"], "status: 500")
	assert.Equal(t, 1, metadata["healthy_count"])
	assert.Equal(t, 2, metadata["total_apis"])
}

func TestHealthChecker_StartAndRunChecks(t *testing.T) {
	config := HealthConfig{
		CheckInterval: 50 * time.Millisecond, // Very fast for testing
		Timeout:       1 * time.Second,
		EnabledChecks: []string{"test_check"},
	}

	hc := NewHealthChecker(config, nil, nil)

	// Add a mock check
	mockCheck := &MockHealthCheck{
		name: "test_check",
		result: ComponentHealth{
			Name:    "test_check",
			Status:  HealthStateHealthy,
			Message: "Test is healthy",
		},
	}
	hc.RegisterCheck(mockCheck)

	// Start health checker
	hc.Start()
	defer hc.Stop()

	// Wait for at least one check cycle
	time.Sleep(100 * time.Millisecond)

	status := hc.GetStatus()
	assert.Equal(t, HealthStateHealthy, status.Overall)
	assert.Contains(t, status.Components, "test_check")
	assert.Equal(t, HealthStateHealthy, status.Components["test_check"].Status)
}

func TestHealthChecker_HTTPHandler_Healthy(t *testing.T) {
	config := HealthConfig{
		DetailedResponse: true,
	}

	hc := NewHealthChecker(config, nil, nil)

	// Set up healthy status
	hc.status = &HealthStatus{
		Overall:   HealthStateHealthy,
		Timestamp: time.Now(),
		Version:   "1.0.0",
		Components: map[string]ComponentHealth{
			"test": {
				Name:    "test",
				Status:  HealthStateHealthy,
				Message: "OK",
			},
		},
	}

	handler := hc.GetHTTPHandler()

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	// Response should contain detailed health info
	body := w.Body.String()
	assert.Contains(t, body, "healthy")
	assert.Contains(t, body, "1.0.0")
	assert.Contains(t, body, "test")
}

func TestHealthChecker_HTTPHandler_Unhealthy(t *testing.T) {
	config := HealthConfig{
		DetailedResponse: false, // Simple response
	}

	hc := NewHealthChecker(config, nil, nil)

	// Set up unhealthy status
	hc.status = &HealthStatus{
		Overall:   HealthStateUnhealthy,
		Timestamp: time.Now(),
		Version:   "1.0.0",
		Components: map[string]ComponentHealth{
			"test": {
				Name:    "test",
				Status:  HealthStateUnhealthy,
				Message: "Failed",
			},
		},
	}

	handler := hc.GetHTTPHandler()

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	// Simple response should not contain component details
	body := w.Body.String()
	assert.Contains(t, body, "unhealthy")
	assert.Contains(t, body, "1.0.0")
	assert.NotContains(t, body, "components") // Not in simple response
}

func TestHealthChecker_EnabledChecks(t *testing.T) {
	config := HealthConfig{
		EnabledChecks: []string{"check1", "check3"}, // Only these should run
	}

	hc := NewHealthChecker(config, nil, nil)

	// Test enabled checks
	assert.True(t, hc.isCheckEnabled("check1"))
	assert.False(t, hc.isCheckEnabled("check2"))
	assert.True(t, hc.isCheckEnabled("check3"))

	// Test with empty enabled checks (all should be enabled)
	config.EnabledChecks = []string{}
	hc.config = config

	assert.True(t, hc.isCheckEnabled("check1"))
	assert.True(t, hc.isCheckEnabled("check2"))
	assert.True(t, hc.isCheckEnabled("check3"))
}

func TestHealthChecker_Stop(t *testing.T) {
	config := HealthConfig{
		CheckInterval: 100 * time.Millisecond,
	}

	hc := NewHealthChecker(config, nil, nil)

	hc.Start()
	time.Sleep(50 * time.Millisecond)

	// Should not panic
	hc.Stop()

	// Multiple stops should not panic
	hc.Stop()
}

func TestHealthChecker_GetUnhealthyComponents(t *testing.T) {
	hc := &HealthChecker{}

	components := map[string]ComponentHealth{
		"healthy1": {Status: HealthStateHealthy},
		"unhealthy1": {Status: HealthStateUnhealthy},
		"warning1": {Status: HealthStateWarning},
		"unhealthy2": {Status: HealthStateUnhealthy},
	}

	unhealthy := hc.getUnhealthyComponents(components)

	assert.Len(t, unhealthy, 2)
	assert.Contains(t, unhealthy, "unhealthy1")
	assert.Contains(t, unhealthy, "unhealthy2")
	assert.NotContains(t, unhealthy, "healthy1")
	assert.NotContains(t, unhealthy, "warning1")
}

// Mock health check for testing
type MockHealthCheck struct {
	name   string
	result ComponentHealth
}

func (m *MockHealthCheck) Name() string {
	return m.name
}

func (m *MockHealthCheck) Check(ctx context.Context) ComponentHealth {
	return m.result
}

func (m *MockHealthCheck) Priority() int {
	return 1
}

func (m *MockHealthCheck) Dependencies() []string {
	return []string{}
}

// Integration test with real database (requires test database)
func TestHealthChecker_DatabaseIntegration(t *testing.T) {
	t.Skip("Requires test database setup - enable for integration testing")

	// This would test with a real database connection
	// db, err := sql.Open("postgres", "postgresql://test:test@localhost/test_db?sslmode=disable")
	// require.NoError(t, err)
	// defer db.Close()
	
	// config := HealthConfig{
	// 	Timeout: 5 * time.Second,
	// }
	
	// hc := NewHealthChecker(config, db, nil)
	// // ... test with real DB
}

// Benchmark tests
func BenchmarkHealthChecker_RunChecks(b *testing.B) {
	config := HealthConfig{
		Timeout: 1 * time.Second,
	}

	hc := NewHealthChecker(config, nil, nil)

	// Add multiple mock checks
	for i := 0; i < 10; i++ {
		mockCheck := &MockHealthCheck{
			name: fmt.Sprintf("check_%d", i),
			result: ComponentHealth{
				Name:   fmt.Sprintf("check_%d", i),
				Status: HealthStateHealthy,
			},
		}
		hc.RegisterCheck(mockCheck)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hc.runHealthChecks()
	}
}

func BenchmarkHealthChecker_GetStatus(b *testing.B) {
	config := HealthConfig{}
	hc := NewHealthChecker(config, nil, nil)

	// Set up status with many components
	components := make(map[string]ComponentHealth)
	for i := 0; i < 100; i++ {
		components[fmt.Sprintf("component_%d", i)] = ComponentHealth{
			Name:   fmt.Sprintf("component_%d", i),
			Status: HealthStateHealthy,
		}
	}

	hc.status = &HealthStatus{
		Overall:    HealthStateHealthy,
		Components: components,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hc.GetStatus()
	}
}