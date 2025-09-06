package monitoring

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
)

// MetricsCollector collects and aggregates MCP performance metrics
type MetricsCollector struct {
	logger      *logrus.Logger
	config      MetricsConfig
	metrics     *MCPMetrics
	collectors  map[string]MetricCollector
	aggregators map[string]*MetricAggregator
	mutex       sync.RWMutex
	stopChan    chan struct{}
}

// MetricsConfig configures metrics collection behavior
type MetricsConfig struct {
	EnableCollection    bool          `json:"enable_collection"`
	CollectionInterval  time.Duration `json:"collection_interval"`
	RetentionPeriod     time.Duration `json:"retention_period"`
	EnableHistograms    bool          `json:"enable_histograms"`
	EnableResourceUsage bool          `json:"enable_resource_usage"`
	MaxMetricSeries     int           `json:"max_metric_series"`
	AggregationWindow   time.Duration `json:"aggregation_window"`
}

// MCPMetrics contains all collected MCP metrics
type MCPMetrics struct {
	// Tool execution metrics
	ToolInvocations   *CounterMetric    `json:"tool_invocations"`
	ToolExecutionTime *HistogramMetric  `json:"tool_execution_time"`
	ToolErrors        *CounterMetric    `json:"tool_errors"`
	ToolCacheHits     *CounterMetric    `json:"tool_cache_hits"`
	
	// Resource access metrics
	ResourceAccesses   *CounterMetric    `json:"resource_accesses"`
	ResourceLoadTime   *HistogramMetric  `json:"resource_load_time"`
	ResourceCacheHits  *CounterMetric    `json:"resource_cache_hits"`
	ResourceErrors     *CounterMetric    `json:"resource_errors"`
	
	// Client connection metrics
	ActiveConnections *GaugeMetric      `json:"active_connections"`
	ConnectionEvents  *CounterMetric    `json:"connection_events"`
	ClientErrors      *CounterMetric    `json:"client_errors"`
	
	// System metrics
	MemoryUsage       *GaugeMetric      `json:"memory_usage"`
	CPUUsage          *GaugeMetric      `json:"cpu_usage"`
	GoroutineCount    *GaugeMetric      `json:"goroutine_count"`
	
	// Database metrics
	DatabaseQueries   *CounterMetric    `json:"database_queries"`
	DatabaseLatency   *HistogramMetric  `json:"database_latency"`
	DatabaseErrors    *CounterMetric    `json:"database_errors"`
	
	// External API metrics
	ExternalAPICalls  *CounterMetric    `json:"external_api_calls"`
	ExternalAPILatency *HistogramMetric `json:"external_api_latency"`
	ExternalAPIErrors *CounterMetric    `json:"external_api_errors"`
	
	mutex sync.RWMutex
}

// MetricCollector interface for different metric collection strategies
type MetricCollector interface {
	Collect(ctx context.Context) (map[string]interface{}, error)
	GetName() string
	GetType() string
}

// CounterMetric represents a monotonically increasing counter
type CounterMetric struct {
	Name        string            `json:"name"`
	Value       int64             `json:"value"`
	Labels      map[string]string `json:"labels,omitempty"`
	LastUpdated time.Time         `json:"last_updated"`
}

// GaugeMetric represents a gauge that can go up and down
type GaugeMetric struct {
	Name        string            `json:"name"`
	Value       float64           `json:"value"`
	Labels      map[string]string `json:"labels,omitempty"`
	LastUpdated time.Time         `json:"last_updated"`
}

// HistogramMetric represents a histogram of values
type HistogramMetric struct {
	Name        string            `json:"name"`
	Count       int64             `json:"count"`
	Sum         float64           `json:"sum"`
	Buckets     map[float64]int64 `json:"buckets"`
	Labels      map[string]string `json:"labels,omitempty"`
	LastUpdated time.Time         `json:"last_updated"`
}

// MetricAggregator aggregates metrics over time windows
type MetricAggregator struct {
	Name           string                    `json:"name"`
	Window         time.Duration             `json:"window"`
	DataPoints     []MetricDataPoint         `json:"data_points"`
	Aggregations   map[string]AggregatedValue `json:"aggregations"`
	mutex          sync.RWMutex
}

// MetricDataPoint represents a single metric data point
type MetricDataPoint struct {
	Timestamp time.Time   `json:"timestamp"`
	Value     float64     `json:"value"`
	Labels    map[string]string `json:"labels,omitempty"`
}

// AggregatedValue represents aggregated metric values
type AggregatedValue struct {
	Min         float64   `json:"min"`
	Max         float64   `json:"max"`
	Avg         float64   `json:"avg"`
	Sum         float64   `json:"sum"`
	Count       int64     `json:"count"`
	Percentiles map[int]float64 `json:"percentiles,omitempty"` // P50, P95, P99
	LastUpdated time.Time `json:"last_updated"`
}

// ToolExecutionCollector collects tool execution metrics
type ToolExecutionCollector struct {
	logger    *logrus.Logger
	startTime time.Time
	metrics   *MCPMetrics
}

// ResourceAccessCollector collects resource access metrics
type ResourceAccessCollector struct {
	logger    *logrus.Logger
	startTime time.Time
	metrics   *MCPMetrics
}

// SystemMetricsCollector collects system-level metrics
type SystemMetricsCollector struct {
	logger       *logrus.Logger
	startTime    time.Time
	lastCPUStats runtime.MemStats
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector(logger *logrus.Logger, config MetricsConfig) *MetricsCollector {
	// Set defaults
	if config.CollectionInterval == 0 {
		config.CollectionInterval = 30 * time.Second
	}
	if config.RetentionPeriod == 0 {
		config.RetentionPeriod = 24 * time.Hour
	}
	if config.AggregationWindow == 0 {
		config.AggregationWindow = 5 * time.Minute
	}
	if config.MaxMetricSeries == 0 {
		config.MaxMetricSeries = 10000
	}
	
	metrics := &MCPMetrics{
		ToolInvocations:    &CounterMetric{Name: "tool_invocations"},
		ToolExecutionTime:  &HistogramMetric{Name: "tool_execution_time", Buckets: createLatencyBuckets()},
		ToolErrors:         &CounterMetric{Name: "tool_errors"},
		ToolCacheHits:      &CounterMetric{Name: "tool_cache_hits"},
		
		ResourceAccesses:   &CounterMetric{Name: "resource_accesses"},
		ResourceLoadTime:   &HistogramMetric{Name: "resource_load_time", Buckets: createLatencyBuckets()},
		ResourceCacheHits:  &CounterMetric{Name: "resource_cache_hits"},
		ResourceErrors:     &CounterMetric{Name: "resource_errors"},
		
		ActiveConnections:  &GaugeMetric{Name: "active_connections"},
		ConnectionEvents:   &CounterMetric{Name: "connection_events"},
		ClientErrors:       &CounterMetric{Name: "client_errors"},
		
		MemoryUsage:        &GaugeMetric{Name: "memory_usage"},
		CPUUsage:          &GaugeMetric{Name: "cpu_usage"},
		GoroutineCount:     &GaugeMetric{Name: "goroutine_count"},
		
		DatabaseQueries:    &CounterMetric{Name: "database_queries"},
		DatabaseLatency:    &HistogramMetric{Name: "database_latency", Buckets: createLatencyBuckets()},
		DatabaseErrors:     &CounterMetric{Name: "database_errors"},
		
		ExternalAPICalls:   &CounterMetric{Name: "external_api_calls"},
		ExternalAPILatency: &HistogramMetric{Name: "external_api_latency", Buckets: createLatencyBuckets()},
		ExternalAPIErrors:  &CounterMetric{Name: "external_api_errors"},
	}
	
	collector := &MetricsCollector{
		logger:      logger,
		config:      config,
		metrics:     metrics,
		collectors:  make(map[string]MetricCollector),
		aggregators: make(map[string]*MetricAggregator),
		stopChan:    make(chan struct{}),
	}
	
	// Register default collectors
	collector.RegisterCollector(NewToolExecutionCollector(logger, metrics))
	collector.RegisterCollector(NewResourceAccessCollector(logger, metrics))
	collector.RegisterCollector(NewSystemMetricsCollector(logger))
	
	// Start collection routine
	if config.EnableCollection {
		go collector.startCollectionRoutine()
	}
	
	return collector
}

// RegisterCollector registers a metric collector
func (mc *MetricsCollector) RegisterCollector(collector MetricCollector) {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()
	
	mc.collectors[collector.GetName()] = collector
	mc.logger.WithField("collector", collector.GetName()).Debug("Registered metric collector")
}

// RecordToolInvocation records a tool invocation
func (mc *MetricsCollector) RecordToolInvocation(toolName string, duration time.Duration, success bool, cacheHit bool) {
	mc.metrics.mutex.Lock()
	defer mc.metrics.mutex.Unlock()
	
	// Increment tool invocations
	atomic.AddInt64(&mc.metrics.ToolInvocations.Value, 1)
	mc.metrics.ToolInvocations.LastUpdated = time.Now()
	
	// Record execution time
	if mc.config.EnableHistograms {
		mc.recordHistogram(mc.metrics.ToolExecutionTime, duration.Seconds())
	}
	
	// Record errors
	if !success {
		atomic.AddInt64(&mc.metrics.ToolErrors.Value, 1)
		mc.metrics.ToolErrors.LastUpdated = time.Now()
	}
	
	// Record cache hits
	if cacheHit {
		atomic.AddInt64(&mc.metrics.ToolCacheHits.Value, 1)
		mc.metrics.ToolCacheHits.LastUpdated = time.Now()
	}
	
	// Add to aggregator
	if aggregator, exists := mc.aggregators["tool_execution"]; exists {
		aggregator.AddDataPoint(duration.Seconds(), map[string]string{
			"tool": toolName,
			"success": fmt.Sprintf("%t", success),
		})
	}
}

// RecordResourceAccess records a resource access
func (mc *MetricsCollector) RecordResourceAccess(resourceURI string, duration time.Duration, success bool, cacheHit bool) {
	mc.metrics.mutex.Lock()
	defer mc.metrics.mutex.Unlock()
	
	// Increment resource accesses
	atomic.AddInt64(&mc.metrics.ResourceAccesses.Value, 1)
	mc.metrics.ResourceAccesses.LastUpdated = time.Now()
	
	// Record load time
	if mc.config.EnableHistograms {
		mc.recordHistogram(mc.metrics.ResourceLoadTime, duration.Seconds())
	}
	
	// Record errors
	if !success {
		atomic.AddInt64(&mc.metrics.ResourceErrors.Value, 1)
		mc.metrics.ResourceErrors.LastUpdated = time.Now()
	}
	
	// Record cache hits
	if cacheHit {
		atomic.AddInt64(&mc.metrics.ResourceCacheHits.Value, 1)
		mc.metrics.ResourceCacheHits.LastUpdated = time.Now()
	}
	
	// Add to aggregator
	if aggregator, exists := mc.aggregators["resource_access"]; exists {
		aggregator.AddDataPoint(duration.Seconds(), map[string]string{
			"resource": resourceURI,
			"success": fmt.Sprintf("%t", success),
		})
	}
}

// RecordDatabaseQuery records a database query
func (mc *MetricsCollector) RecordDatabaseQuery(operation string, duration time.Duration, success bool) {
	mc.metrics.mutex.Lock()
	defer mc.metrics.mutex.Unlock()
	
	// Increment database queries
	atomic.AddInt64(&mc.metrics.DatabaseQueries.Value, 1)
	mc.metrics.DatabaseQueries.LastUpdated = time.Now()
	
	// Record latency
	if mc.config.EnableHistograms {
		mc.recordHistogram(mc.metrics.DatabaseLatency, duration.Seconds())
	}
	
	// Record errors
	if !success {
		atomic.AddInt64(&mc.metrics.DatabaseErrors.Value, 1)
		mc.metrics.DatabaseErrors.LastUpdated = time.Now()
	}
}

// RecordExternalAPICall records an external API call
func (mc *MetricsCollector) RecordExternalAPICall(apiName string, duration time.Duration, success bool) {
	mc.metrics.mutex.Lock()
	defer mc.metrics.mutex.Unlock()
	
	// Increment API calls
	atomic.AddInt64(&mc.metrics.ExternalAPICalls.Value, 1)
	mc.metrics.ExternalAPICalls.LastUpdated = time.Now()
	
	// Record latency
	if mc.config.EnableHistograms {
		mc.recordHistogram(mc.metrics.ExternalAPILatency, duration.Seconds())
	}
	
	// Record errors
	if !success {
		atomic.AddInt64(&mc.metrics.ExternalAPIErrors.Value, 1)
		mc.metrics.ExternalAPIErrors.LastUpdated = time.Now()
	}
}

// SetActiveConnections sets the number of active connections
func (mc *MetricsCollector) SetActiveConnections(count int) {
	mc.metrics.mutex.Lock()
	defer mc.metrics.mutex.Unlock()
	
	mc.metrics.ActiveConnections.Value = float64(count)
	mc.metrics.ActiveConnections.LastUpdated = time.Now()
}

// RecordConnectionEvent records a connection event
func (mc *MetricsCollector) RecordConnectionEvent(eventType string) {
	mc.metrics.mutex.Lock()
	defer mc.metrics.mutex.Unlock()
	
	atomic.AddInt64(&mc.metrics.ConnectionEvents.Value, 1)
	mc.metrics.ConnectionEvents.LastUpdated = time.Now()
}

// recordHistogram records a value in a histogram
func (mc *MetricsCollector) recordHistogram(histogram *HistogramMetric, value float64) {
	histogram.Count++
	histogram.Sum += value
	histogram.LastUpdated = time.Now()
	
	// Find appropriate bucket
	for bucketLE, _ := range histogram.Buckets {
		if value <= bucketLE {
			histogram.Buckets[bucketLE]++
		}
	}
}

// GetMetrics returns current metrics snapshot
func (mc *MetricsCollector) GetMetrics() *MCPMetrics {
	mc.metrics.mutex.RLock()
	defer mc.metrics.mutex.RUnlock()
	
	// Return deep copy
	metricsCopy := *mc.metrics
	
	// Copy histograms
	if mc.metrics.ToolExecutionTime != nil {
		toolExecCopy := *mc.metrics.ToolExecutionTime
		toolExecCopy.Buckets = make(map[float64]int64)
		for k, v := range mc.metrics.ToolExecutionTime.Buckets {
			toolExecCopy.Buckets[k] = v
		}
		metricsCopy.ToolExecutionTime = &toolExecCopy
	}
	
	return &metricsCopy
}

// GetAggregatedMetrics returns aggregated metrics for a time period
func (mc *MetricsCollector) GetAggregatedMetrics(period time.Duration) map[string]AggregatedValue {
	mc.mutex.RLock()
	defer mc.mutex.RUnlock()
	
	result := make(map[string]AggregatedValue)
	cutoff := time.Now().Add(-period)
	
	for name, aggregator := range mc.aggregators {
		aggregated := aggregator.GetAggregatedValue(cutoff)
		if aggregated != nil {
			result[name] = *aggregated
		}
	}
	
	return result
}

// startCollectionRoutine starts the background metrics collection
func (mc *MetricsCollector) startCollectionRoutine() {
	ticker := time.NewTicker(mc.config.CollectionInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			mc.collectSystemMetrics()
			mc.aggregateMetrics()
			mc.cleanupOldMetrics()
		case <-mc.stopChan:
			return
		}
	}
}

// collectSystemMetrics collects system-level metrics
func (mc *MetricsCollector) collectSystemMetrics() {
	if !mc.config.EnableResourceUsage {
		return
	}
	
	// Collect memory metrics
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	
	mc.metrics.mutex.Lock()
	mc.metrics.MemoryUsage.Value = float64(memStats.Alloc)
	mc.metrics.MemoryUsage.LastUpdated = time.Now()
	
	mc.metrics.GoroutineCount.Value = float64(runtime.NumGoroutine())
	mc.metrics.GoroutineCount.LastUpdated = time.Now()
	mc.metrics.mutex.Unlock()
}

// aggregateMetrics performs metric aggregation
func (mc *MetricsCollector) aggregateMetrics() {
	// Implementation would aggregate metrics over time windows
	// This is a simplified version
	for name, aggregator := range mc.aggregators {
		aggregator.Aggregate()
		mc.logger.WithField("aggregator", name).Debug("Aggregated metrics")
	}
}

// cleanupOldMetrics removes old metric data points
func (mc *MetricsCollector) cleanupOldMetrics() {
	cutoff := time.Now().Add(-mc.config.RetentionPeriod)
	
	for _, aggregator := range mc.aggregators {
		aggregator.CleanupOldData(cutoff)
	}
}

// Stop stops the metrics collector
func (mc *MetricsCollector) Stop() {
	close(mc.stopChan)
	mc.logger.Info("Metrics collector stopped")
}

// Helper functions for metric collectors

// NewToolExecutionCollector creates a tool execution collector
func NewToolExecutionCollector(logger *logrus.Logger, metrics *MCPMetrics) *ToolExecutionCollector {
	return &ToolExecutionCollector{
		logger:    logger,
		startTime: time.Now(),
		metrics:   metrics,
	}
}

// Collect implements MetricCollector interface
func (tec *ToolExecutionCollector) Collect(ctx context.Context) (map[string]interface{}, error) {
	return map[string]interface{}{
		"tool_invocations": atomic.LoadInt64(&tec.metrics.ToolInvocations.Value),
		"tool_errors":      atomic.LoadInt64(&tec.metrics.ToolErrors.Value),
		"tool_cache_hits":  atomic.LoadInt64(&tec.metrics.ToolCacheHits.Value),
	}, nil
}

func (tec *ToolExecutionCollector) GetName() string { return "tool_execution" }
func (tec *ToolExecutionCollector) GetType() string { return "counter" }

// NewResourceAccessCollector creates a resource access collector
func NewResourceAccessCollector(logger *logrus.Logger, metrics *MCPMetrics) *ResourceAccessCollector {
	return &ResourceAccessCollector{
		logger:    logger,
		startTime: time.Now(),
		metrics:   metrics,
	}
}

// Collect implements MetricCollector interface
func (rac *ResourceAccessCollector) Collect(ctx context.Context) (map[string]interface{}, error) {
	return map[string]interface{}{
		"resource_accesses":   atomic.LoadInt64(&rac.metrics.ResourceAccesses.Value),
		"resource_errors":     atomic.LoadInt64(&rac.metrics.ResourceErrors.Value),
		"resource_cache_hits": atomic.LoadInt64(&rac.metrics.ResourceCacheHits.Value),
	}, nil
}

func (rac *ResourceAccessCollector) GetName() string { return "resource_access" }
func (rac *ResourceAccessCollector) GetType() string { return "counter" }

// NewSystemMetricsCollector creates a system metrics collector
func NewSystemMetricsCollector(logger *logrus.Logger) *SystemMetricsCollector {
	return &SystemMetricsCollector{
		logger:    logger,
		startTime: time.Now(),
	}
}

// Collect implements MetricCollector interface
func (smc *SystemMetricsCollector) Collect(ctx context.Context) (map[string]interface{}, error) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	
	return map[string]interface{}{
		"memory_alloc":     memStats.Alloc,
		"memory_sys":       memStats.Sys,
		"gc_runs":          memStats.NumGC,
		"goroutines":       runtime.NumGoroutine(),
		"heap_objects":     memStats.HeapObjects,
	}, nil
}

func (smc *SystemMetricsCollector) GetName() string { return "system_metrics" }
func (smc *SystemMetricsCollector) GetType() string { return "gauge" }

// MetricAggregator methods

// AddDataPoint adds a data point to the aggregator
func (ma *MetricAggregator) AddDataPoint(value float64, labels map[string]string) {
	ma.mutex.Lock()
	defer ma.mutex.Unlock()
	
	dataPoint := MetricDataPoint{
		Timestamp: time.Now(),
		Value:     value,
		Labels:    labels,
	}
	
	ma.DataPoints = append(ma.DataPoints, dataPoint)
	
	// Limit data points to prevent memory growth
	if len(ma.DataPoints) > 10000 {
		ma.DataPoints = ma.DataPoints[len(ma.DataPoints)-5000:]
	}
}

// GetAggregatedValue returns aggregated value for a time period
func (ma *MetricAggregator) GetAggregatedValue(since time.Time) *AggregatedValue {
	ma.mutex.RLock()
	defer ma.mutex.RUnlock()
	
	var values []float64
	for _, dp := range ma.DataPoints {
		if dp.Timestamp.After(since) {
			values = append(values, dp.Value)
		}
	}
	
	if len(values) == 0 {
		return nil
	}
	
	// Calculate aggregated values
	sum := 0.0
	min := values[0]
	max := values[0]
	
	for _, v := range values {
		sum += v
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	
	avg := sum / float64(len(values))
	
	return &AggregatedValue{
		Min:         min,
		Max:         max,
		Avg:         avg,
		Sum:         sum,
		Count:       int64(len(values)),
		LastUpdated: time.Now(),
	}
}

// Aggregate performs aggregation
func (ma *MetricAggregator) Aggregate() {
	// Simplified aggregation - in production, this would be more sophisticated
	ma.mutex.Lock()
	defer ma.mutex.Unlock()
	
	if len(ma.DataPoints) == 0 {
		return
	}
	
	// Group by time windows and aggregate
	// Implementation would group data points by time windows and calculate statistics
}

// CleanupOldData removes data points older than the cutoff time
func (ma *MetricAggregator) CleanupOldData(cutoff time.Time) {
	ma.mutex.Lock()
	defer ma.mutex.Unlock()
	
	var filtered []MetricDataPoint
	for _, dp := range ma.DataPoints {
		if dp.Timestamp.After(cutoff) {
			filtered = append(filtered, dp)
		}
	}
	
	ma.DataPoints = filtered
}

// createLatencyBuckets creates histogram buckets for latency metrics
func createLatencyBuckets() map[float64]int64 {
	buckets := make(map[float64]int64)
	
	// Latency buckets in seconds: 1ms, 5ms, 10ms, 25ms, 50ms, 100ms, 250ms, 500ms, 1s, 2.5s, 5s, 10s
	latencyBuckets := []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0}
	
	for _, bucket := range latencyBuckets {
		buckets[bucket] = 0
	}
	
	return buckets
}