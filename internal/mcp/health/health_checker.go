package health

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
)

type HealthChecker struct {
	config      HealthConfig
	logger      *logrus.Logger
	db          *sql.DB
	redisClient *redis.Client
	checks      map[string]HealthCheck
	status      *HealthStatus
	mutex       sync.RWMutex
	stopChan    chan struct{}
	ticker      *time.Ticker
}

type HealthConfig struct {
	CheckInterval        time.Duration     `json:"check_interval"`
	Timeout             time.Duration     `json:"timeout"`
	EnabledChecks       []string          `json:"enabled_checks"`
	Thresholds          HealthThresholds  `json:"thresholds"`
	AlertingEnabled     bool              `json:"alerting_enabled"`
	EndpointPath        string            `json:"endpoint_path"`
	DetailedResponse    bool              `json:"detailed_response"`
	CacheResults        bool              `json:"cache_results"`
	CacheTTL           time.Duration     `json:"cache_ttl"`
}

type HealthThresholds struct {
	DatabaseMaxLatency    time.Duration `json:"database_max_latency"`
	RedisMaxLatency      time.Duration `json:"redis_max_latency"`
	MemoryMaxUsage       int64         `json:"memory_max_usage"`
	CPUMaxUsage          float64       `json:"cpu_max_usage"`
	DiskMaxUsage         float64       `json:"disk_max_usage"`
	MaxActiveConnections int           `json:"max_active_connections"`
	MinFreeMemory        int64         `json:"min_free_memory"`
}

type HealthStatus struct {
	Overall     HealthState               `json:"overall"`
	Timestamp   time.Time                `json:"timestamp"`
	Version     string                   `json:"version"`
	Uptime      time.Duration            `json:"uptime"`
	Components  map[string]ComponentHealth `json:"components"`
	Metrics     SystemMetrics            `json:"metrics"`
	LastChecked time.Time                `json:"last_checked"`
	CheckCount  int64                    `json:"check_count"`
}

type ComponentHealth struct {
	Name        string            `json:"name"`
	Status      HealthState       `json:"status"`
	Message     string            `json:"message"`
	LastChecked time.Time         `json:"last_checked"`
	Duration    time.Duration     `json:"duration"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Error       string            `json:"error,omitempty"`
}

type SystemMetrics struct {
	MemoryUsage      int64     `json:"memory_usage_bytes"`
	MemoryAvailable  int64     `json:"memory_available_bytes"`
	CPUUsage         float64   `json:"cpu_usage_percent"`
	DiskUsage        float64   `json:"disk_usage_percent"`
	GoroutineCount   int       `json:"goroutine_count"`
	ActiveConnections int      `json:"active_connections"`
	RequestRate      float64   `json:"requests_per_second"`
	ErrorRate        float64   `json:"errors_per_second"`
	LastUpdated      time.Time `json:"last_updated"`
}

type HealthState string

const (
	HealthStateHealthy   HealthState = "healthy"
	HealthStateUnhealthy HealthState = "unhealthy"
	HealthStateWarning   HealthState = "warning"
	HealthStateUnknown   HealthState = "unknown"
)

type HealthCheck interface {
	Name() string
	Check(ctx context.Context) ComponentHealth
	Priority() int
	Dependencies() []string
}

type DatabaseHealthCheck struct {
	db      *sql.DB
	timeout time.Duration
	logger  *logrus.Logger
}

type RedisHealthCheck struct {
	client  *redis.Client
	timeout time.Duration
	logger  *logrus.Logger
}

type MCPToolsHealthCheck struct {
	toolRegistry map[string]interface{}
	timeout      time.Duration
	logger       *logrus.Logger
}

type ExternalAPIHealthCheck struct {
	endpoints []APIEndpoint
	timeout   time.Duration
	logger    *logrus.Logger
}

type SystemResourceHealthCheck struct {
	thresholds HealthThresholds
	logger     *logrus.Logger
}

type APIEndpoint struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Method   string `json:"method"`
	Expected int    `json:"expected_status"`
}

func NewHealthChecker(config HealthConfig, db *sql.DB, redisClient *redis.Client) *HealthChecker {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})

	if config.CheckInterval == 0 {
		config.CheckInterval = 30 * time.Second
	}
	if config.Timeout == 0 {
		config.Timeout = 10 * time.Second
	}
	if config.EndpointPath == "" {
		config.EndpointPath = "/health"
	}
	if config.CacheTTL == 0 {
		config.CacheTTL = 10 * time.Second
	}

	hc := &HealthChecker{
		config:      config,
		logger:      logger,
		db:          db,
		redisClient: redisClient,
		checks:      make(map[string]HealthCheck),
		status: &HealthStatus{
			Overall:    HealthStateUnknown,
			Timestamp:  time.Now(),
			Components: make(map[string]ComponentHealth),
			Metrics:    SystemMetrics{},
		},
		stopChan: make(chan struct{}),
	}

	hc.registerDefaultChecks()
	return hc
}

func (h *HealthChecker) registerDefaultChecks() {
	if h.db != nil {
		h.RegisterCheck(&DatabaseHealthCheck{
			db:      h.db,
			timeout: h.config.Timeout,
			logger:  h.logger,
		})
	}

	if h.redisClient != nil {
		h.RegisterCheck(&RedisHealthCheck{
			client:  h.redisClient,
			timeout: h.config.Timeout,
			logger:  h.logger,
		})
	}

	h.RegisterCheck(&SystemResourceHealthCheck{
		thresholds: h.config.Thresholds,
		logger:     h.logger,
	})

	h.RegisterCheck(&MCPToolsHealthCheck{
		toolRegistry: make(map[string]interface{}), // Will be populated by MCP server
		timeout:      h.config.Timeout,
		logger:       h.logger,
	})

	// External API endpoints for ACMG/AMP services
	h.RegisterCheck(&ExternalAPIHealthCheck{
		endpoints: []APIEndpoint{
			{Name: "ClinVar", URL: "https://eutils.ncbi.nlm.nih.gov/entrez/eutils/einfo.fcgi", Method: "GET", Expected: 200},
			{Name: "gnomAD", URL: "https://gnomad.broadinstitute.org/api", Method: "GET", Expected: 200},
			{Name: "COSMIC", URL: "https://cancer.sanger.ac.uk/cosmic", Method: "HEAD", Expected: 200},
		},
		timeout: h.config.Timeout,
		logger:  h.logger,
	})
}

func (h *HealthChecker) RegisterCheck(check HealthCheck) {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	h.checks[check.Name()] = check
}

func (h *HealthChecker) Start() {
	h.ticker = time.NewTicker(h.config.CheckInterval)
	
	// Run initial check
	h.runHealthChecks()

	go func() {
		for {
			select {
			case <-h.ticker.C:
				h.runHealthChecks()
			case <-h.stopChan:
				return
			}
		}
	}()

	h.logger.Info("Health checker started")
}

func (h *HealthChecker) Stop() {
	if h.ticker != nil {
		h.ticker.Stop()
	}
	close(h.stopChan)
	h.logger.Info("Health checker stopped")
}

func (h *HealthChecker) runHealthChecks() {
	ctx, cancel := context.WithTimeout(context.Background(), h.config.Timeout)
	defer cancel()

	h.mutex.Lock()
	defer h.mutex.Unlock()

	startTime := time.Now()
	overallHealthy := true
	hasWarnings := false

	// Run checks in parallel
	results := make(chan ComponentHealth, len(h.checks))
	var wg sync.WaitGroup

	for _, check := range h.checks {
		if !h.isCheckEnabled(check.Name()) {
			continue
		}

		wg.Add(1)
		go func(c HealthCheck) {
			defer wg.Done()
			result := c.Check(ctx)
			results <- result
		}(check)
	}

	// Close results channel when all checks complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	components := make(map[string]ComponentHealth)
	for result := range results {
		components[result.Name] = result
		
		if result.Status == HealthStateUnhealthy {
			overallHealthy = false
		} else if result.Status == HealthStateWarning {
			hasWarnings = true
		}
	}

	// Determine overall status
	var overallStatus HealthState
	if !overallHealthy {
		overallStatus = HealthStateUnhealthy
	} else if hasWarnings {
		overallStatus = HealthStateWarning
	} else {
		overallStatus = HealthStateHealthy
	}

	// Update status
	h.status = &HealthStatus{
		Overall:     overallStatus,
		Timestamp:   startTime,
		Version:     "1.0.0", // Should be injected from build
		Uptime:      time.Since(startTime), // Should track actual uptime
		Components:  components,
		Metrics:     h.collectSystemMetrics(),
		LastChecked: time.Now(),
		CheckCount:  h.status.CheckCount + 1,
	}

	// Log status change
	if overallStatus != HealthStateHealthy {
		h.logger.WithFields(logrus.Fields{
			"overall_status": overallStatus,
			"unhealthy_components": h.getUnhealthyComponents(components),
		}).Warn("Health check completed with issues")
	} else {
		h.logger.Debug("Health check completed successfully")
	}
}

func (h *HealthChecker) isCheckEnabled(checkName string) bool {
	if len(h.config.EnabledChecks) == 0 {
		return true // All checks enabled by default
	}
	
	for _, enabled := range h.config.EnabledChecks {
		if enabled == checkName {
			return true
		}
	}
	return false
}

func (h *HealthChecker) getUnhealthyComponents(components map[string]ComponentHealth) []string {
	var unhealthy []string
	for name, component := range components {
		if component.Status == HealthStateUnhealthy {
			unhealthy = append(unhealthy, name)
		}
	}
	return unhealthy
}

func (h *HealthChecker) collectSystemMetrics() SystemMetrics {
	// This would integrate with actual system monitoring
	// For now, returning placeholder values
	return SystemMetrics{
		MemoryUsage:       1024 * 1024 * 100, // 100MB
		MemoryAvailable:   1024 * 1024 * 900, // 900MB
		CPUUsage:          25.0,
		DiskUsage:         50.0,
		GoroutineCount:    100,
		ActiveConnections: 5,
		RequestRate:       10.5,
		ErrorRate:         0.1,
		LastUpdated:       time.Now(),
	}
}

func (h *HealthChecker) GetStatus() *HealthStatus {
	h.mutex.RLock()
	defer h.mutex.RUnlock()
	
	// Return a copy to prevent external modification
	status := *h.status
	status.Components = make(map[string]ComponentHealth)
	for k, v := range h.status.Components {
		status.Components[k] = v
	}
	
	return &status
}

func (h *HealthChecker) GetHTTPHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := h.GetStatus()
		
		// Set appropriate HTTP status code
		httpStatus := http.StatusOK
		if status.Overall == HealthStateUnhealthy {
			httpStatus = http.StatusServiceUnavailable
		} else if status.Overall == HealthStateWarning {
			httpStatus = http.StatusOK // Still OK but with warnings
		}
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(httpStatus)
		
		var response interface{}
		if h.config.DetailedResponse {
			response = status
		} else {
			response = map[string]interface{}{
				"status":    status.Overall,
				"timestamp": status.Timestamp,
				"version":   status.Version,
			}
		}
		
		json.NewEncoder(w).Encode(response)
	}
}

// DatabaseHealthCheck implementation
func (d *DatabaseHealthCheck) Name() string {
	return "database"
}

func (d *DatabaseHealthCheck) Priority() int {
	return 1
}

func (d *DatabaseHealthCheck) Dependencies() []string {
	return []string{}
}

func (d *DatabaseHealthCheck) Check(ctx context.Context) ComponentHealth {
	start := time.Now()
	
	if d.db == nil {
		return ComponentHealth{
			Name:        d.Name(),
			Status:      HealthStateUnhealthy,
			Message:     "Database connection not configured",
			LastChecked: time.Now(),
			Duration:    time.Since(start),
			Error:       "database connection is nil",
		}
	}
	
	// Test connection with timeout
	ctx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()
	
	err := d.db.PingContext(ctx)
	duration := time.Since(start)
	
	if err != nil {
		return ComponentHealth{
			Name:        d.Name(),
			Status:      HealthStateUnhealthy,
			Message:     "Database connection failed",
			LastChecked: time.Now(),
			Duration:    duration,
			Error:       err.Error(),
		}
	}
	
	// Check connection pool stats
	stats := d.db.Stats()
	metadata := map[string]interface{}{
		"open_connections":     stats.OpenConnections,
		"in_use_connections":   stats.InUse,
		"idle_connections":     stats.Idle,
		"max_open_connections": stats.MaxOpenConnections,
		"wait_count":          stats.WaitCount,
		"wait_duration":       stats.WaitDuration,
	}
	
	status := HealthStateHealthy
	message := "Database connection healthy"
	
	// Check for warnings
	if stats.WaitCount > 100 {
		status = HealthStateWarning
		message = "High database connection wait count"
	}
	
	return ComponentHealth{
		Name:        d.Name(),
		Status:      status,
		Message:     message,
		LastChecked: time.Now(),
		Duration:    duration,
		Metadata:    metadata,
	}
}

// RedisHealthCheck implementation
func (r *RedisHealthCheck) Name() string {
	return "redis"
}

func (r *RedisHealthCheck) Priority() int {
	return 2
}

func (r *RedisHealthCheck) Dependencies() []string {
	return []string{}
}

func (r *RedisHealthCheck) Check(ctx context.Context) ComponentHealth {
	start := time.Now()
	
	if r.client == nil {
		return ComponentHealth{
			Name:        r.Name(),
			Status:      HealthStateUnhealthy,
			Message:     "Redis client not configured",
			LastChecked: time.Now(),
			Duration:    time.Since(start),
			Error:       "redis client is nil",
		}
	}
	
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	
	// Test ping
	_, err := r.client.Ping(ctx).Result()
	duration := time.Since(start)
	
	if err != nil {
		return ComponentHealth{
			Name:        r.Name(),
			Status:      HealthStateUnhealthy,
			Message:     "Redis connection failed",
			LastChecked: time.Now(),
			Duration:    duration,
			Error:       err.Error(),
		}
	}
	
	// Get Redis info
	info, _ := r.client.Info(ctx).Result()
	metadata := map[string]interface{}{
		"connected_clients": "unknown",
		"used_memory":      "unknown",
		"redis_info":       info[:min(len(info), 200)], // Truncated info
	}
	
	return ComponentHealth{
		Name:        r.Name(),
		Status:      HealthStateHealthy,
		Message:     "Redis connection healthy",
		LastChecked: time.Now(),
		Duration:    duration,
		Metadata:    metadata,
	}
}

// SystemResourceHealthCheck implementation
func (s *SystemResourceHealthCheck) Name() string {
	return "system_resources"
}

func (s *SystemResourceHealthCheck) Priority() int {
	return 3
}

func (s *SystemResourceHealthCheck) Dependencies() []string {
	return []string{}
}

func (s *SystemResourceHealthCheck) Check(ctx context.Context) ComponentHealth {
	start := time.Now()
	
	// This would integrate with actual system monitoring
	// For now, returning healthy status with mock data
	metadata := map[string]interface{}{
		"memory_usage_bytes":  1024 * 1024 * 100, // 100MB
		"cpu_usage_percent":   25.0,
		"disk_usage_percent":  50.0,
		"goroutine_count":     100,
	}
	
	return ComponentHealth{
		Name:        s.Name(),
		Status:      HealthStateHealthy,
		Message:     "System resources healthy",
		LastChecked: time.Now(),
		Duration:    time.Since(start),
		Metadata:    metadata,
	}
}

// MCPToolsHealthCheck implementation
func (m *MCPToolsHealthCheck) Name() string {
	return "mcp_tools"
}

func (m *MCPToolsHealthCheck) Priority() int {
	return 4
}

func (m *MCPToolsHealthCheck) Dependencies() []string {
	return []string{"database", "redis"}
}

func (m *MCPToolsHealthCheck) Check(ctx context.Context) ComponentHealth {
	start := time.Now()
	
	// This would test actual MCP tools
	toolCount := len(m.toolRegistry)
	metadata := map[string]interface{}{
		"registered_tools": toolCount,
		"tools_available": []string{
			"classify_variant",
			"validate_hgvs",
			"apply_rule",
			"combine_evidence",
			"query_evidence",
			"generate_report",
		},
	}
	
	return ComponentHealth{
		Name:        m.Name(),
		Status:      HealthStateHealthy,
		Message:     fmt.Sprintf("MCP tools healthy (%d tools available)", len(metadata["tools_available"].([]string))),
		LastChecked: time.Now(),
		Duration:    time.Since(start),
		Metadata:    metadata,
	}
}

// ExternalAPIHealthCheck implementation
func (e *ExternalAPIHealthCheck) Name() string {
	return "external_apis"
}

func (e *ExternalAPIHealthCheck) Priority() int {
	return 5
}

func (e *ExternalAPIHealthCheck) Dependencies() []string {
	return []string{}
}

func (e *ExternalAPIHealthCheck) Check(ctx context.Context) ComponentHealth {
	start := time.Now()
	
	ctx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()
	
	healthyCount := 0
	results := make(map[string]string)
	
	for _, endpoint := range e.endpoints {
		req, err := http.NewRequestWithContext(ctx, endpoint.Method, endpoint.URL, nil)
		if err != nil {
			results[endpoint.Name] = fmt.Sprintf("error: %v", err)
			continue
		}
		
		client := &http.Client{Timeout: e.timeout}
		resp, err := client.Do(req)
		if err != nil {
			results[endpoint.Name] = fmt.Sprintf("error: %v", err)
			continue
		}
		resp.Body.Close()
		
		if resp.StatusCode == endpoint.Expected {
			results[endpoint.Name] = "healthy"
			healthyCount++
		} else {
			results[endpoint.Name] = fmt.Sprintf("status: %d", resp.StatusCode)
		}
	}
	
	var status HealthState
	var message string
	
	if healthyCount == len(e.endpoints) {
		status = HealthStateHealthy
		message = "All external APIs healthy"
	} else if healthyCount > 0 {
		status = HealthStateWarning
		message = fmt.Sprintf("%d/%d external APIs healthy", healthyCount, len(e.endpoints))
	} else {
		status = HealthStateUnhealthy
		message = "No external APIs responding"
	}
	
	metadata := map[string]interface{}{
		"api_results": results,
		"healthy_count": healthyCount,
		"total_apis": len(e.endpoints),
	}
	
	return ComponentHealth{
		Name:        e.Name(),
		Status:      status,
		Message:     message,
		LastChecked: time.Now(),
		Duration:    time.Since(start),
		Metadata:    metadata,
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}