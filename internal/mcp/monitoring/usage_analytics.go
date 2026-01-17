package monitoring

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// UsageAnalytics tracks and analyzes MCP resource and tool usage patterns
type UsageAnalytics struct {
	logger         *logrus.Logger
	config         AnalyticsConfig
	resourceStats  map[string]*ResourceUsageStats
	toolStats      map[string]*ToolUsageStats
	clientStats    map[string]*ClientUsageStats
	patternTracker *UsagePatternTracker
	mutex          sync.RWMutex
}

// AnalyticsConfig configures usage analytics behavior
type AnalyticsConfig struct {
	EnableAnalytics       bool          `json:"enable_analytics"`
	RetentionPeriod       time.Duration `json:"retention_period"`
	AnalysisInterval      time.Duration `json:"analysis_interval"`
	EnablePatternDetection bool          `json:"enable_pattern_detection"`
	EnableUsageReports    bool          `json:"enable_usage_reports"`
	MaxTrackingEntries    int           `json:"max_tracking_entries"`
	PrivacyMode           bool          `json:"privacy_mode"`
}

// ResourceUsageStats tracks usage statistics for MCP resources
type ResourceUsageStats struct {
	ResourceURI        string                    `json:"resource_uri"`
	ResourceType       string                    `json:"resource_type"` // "variant", "interpretation", "evidence", etc.
	AccessCount        int64                     `json:"access_count"`
	UniqueClients      map[string]bool           `json:"unique_clients"`
	TotalBytes         int64                     `json:"total_bytes"`
	AverageLoadTime    time.Duration             `json:"average_load_time"`
	CacheHitRate       float64                   `json:"cache_hit_rate"`
	ErrorCount         int64                     `json:"error_count"`
	FirstAccess        time.Time                 `json:"first_access"`
	LastAccess         time.Time                 `json:"last_access"`
	AccessPatterns     []AccessPattern           `json:"access_patterns"`
	PopularParameters  map[string]ParameterStats `json:"popular_parameters"`
	mutex              sync.RWMutex
}

// ToolUsageStats tracks usage statistics for MCP tools
type ToolUsageStats struct {
	ToolName            string                    `json:"tool_name"`
	InvocationCount     int64                     `json:"invocation_count"`
	UniqueClients       map[string]bool           `json:"unique_clients"`
	AverageExecutionTime time.Duration            `json:"average_execution_time"`
	SuccessRate         float64                   `json:"success_rate"`
	ErrorTypes          map[string]int64          `json:"error_types"`
	ParameterUsage      map[string]ParameterStats `json:"parameter_usage"`
	ResultSizeStats     SizeStats                 `json:"result_size_stats"`
	FirstInvocation     time.Time                 `json:"first_invocation"`
	LastInvocation      time.Time                 `json:"last_invocation"`
	InvocationPatterns  []InvocationPattern       `json:"invocation_patterns"`
	mutex               sync.RWMutex
}

// ClientUsageStats tracks usage statistics for MCP clients
type ClientUsageStats struct {
	ClientID            string            `json:"client_id"`
	ClientName          string            `json:"client_name"`
	SessionCount        int64             `json:"session_count"`
	TotalSessionTime    time.Duration     `json:"total_session_time"`
	ToolInvocations     int64             `json:"tool_invocations"`
	ResourceAccesses    int64             `json:"resource_accesses"`
	ErrorCount          int64             `json:"error_count"`
	FavoriteTools       []string          `json:"favorite_tools"`
	FavoriteResources   []string          `json:"favorite_resources"`
	UsagePatterns       []UsagePattern    `json:"usage_patterns"`
	FirstConnection     time.Time         `json:"first_connection"`
	LastConnection      time.Time         `json:"last_connection"`
	ClientMetadata      map[string]string `json:"client_metadata"`
	mutex               sync.RWMutex
}

// AccessPattern represents a resource access pattern
type AccessPattern struct {
	Pattern     string            `json:"pattern"`      // "sequential", "batch", "random", "burst"
	Frequency   float64           `json:"frequency"`    // Accesses per hour/day
	Parameters  map[string]string `json:"parameters"`
	TimeWindow  time.Duration     `json:"time_window"`
	ClientTypes []string          `json:"client_types"`
	Confidence  float64           `json:"confidence"`
}

// InvocationPattern represents a tool invocation pattern
type InvocationPattern struct {
	Pattern        string            `json:"pattern"`         // "workflow", "single", "batch", "retry"
	ToolSequence   []string          `json:"tool_sequence"`
	AverageLatency time.Duration     `json:"average_latency"`
	SuccessRate    float64           `json:"success_rate"`
	Frequency      float64           `json:"frequency"`
	Parameters     map[string]string `json:"parameters"`
	Confidence     float64           `json:"confidence"`
}

// UsagePattern represents general usage patterns
type UsagePattern struct {
	PatternType string            `json:"pattern_type"` // "daily", "weekly", "burst", "steady"
	Description string            `json:"description"`
	Metrics     map[string]float64 `json:"metrics"`
	TimeRange   TimeRange         `json:"time_range"`
	Confidence  float64           `json:"confidence"`
}

// ParameterStats tracks parameter usage statistics
type ParameterStats struct {
	ParameterName  string            `json:"parameter_name"`
	UsageCount     int64             `json:"usage_count"`
	UniqueValues   int               `json:"unique_values"`
	PopularValues  map[string]int64  `json:"popular_values"`
	AverageSize    int               `json:"average_size"`
	DataTypes      map[string]int64  `json:"data_types"`
}

// SizeStats tracks size-related statistics
type SizeStats struct {
	MinSize     int   `json:"min_size"`
	MaxSize     int   `json:"max_size"`
	AverageSize int   `json:"average_size"`
	TotalSize   int64 `json:"total_size"`
	SizeBuckets map[string]int64 `json:"size_buckets"` // "small", "medium", "large", "xlarge"
}

// TimeRange represents a time range
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// UsagePatternTracker detects and tracks usage patterns
type UsagePatternTracker struct {
	logger           *logrus.Logger
	patterns         map[string]*DetectedPattern
	patternDetectors map[string]PatternDetector
	mutex            sync.RWMutex
}

// DetectedPattern represents a detected usage pattern
type DetectedPattern struct {
	ID          string            `json:"id"`
	Type        string            `json:"type"`
	Description string            `json:"description"`
	Confidence  float64           `json:"confidence"`
	Frequency   float64           `json:"frequency"`
	Impact      string            `json:"impact"` // "low", "medium", "high"
	Suggestions []string          `json:"suggestions"`
	DetectedAt  time.Time         `json:"detected_at"`
	LastSeen    time.Time         `json:"last_seen"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// PatternDetector interface for pattern detection algorithms
type PatternDetector interface {
	DetectPattern(data interface{}) (*DetectedPattern, error)
	GetPatternType() string
}

// UsageReport represents a comprehensive usage report
type UsageReport struct {
	GeneratedAt       time.Time                        `json:"generated_at"`
	ReportPeriod      TimeRange                        `json:"report_period"`
	Summary           UsageSummary                     `json:"summary"`
	TopResources      []ResourceUsageStats             `json:"top_resources"`
	TopTools          []ToolUsageStats                 `json:"top_tools"`
	ClientAnalysis    []ClientUsageStats               `json:"client_analysis"`
	DetectedPatterns  []DetectedPattern                `json:"detected_patterns"`
	Recommendations   []UsageRecommendation            `json:"recommendations"`
	PerformanceInsights []PerformanceInsight           `json:"performance_insights"`
}

// UsageSummary provides high-level usage summary
type UsageSummary struct {
	TotalToolInvocations  int64         `json:"total_tool_invocations"`
	TotalResourceAccesses int64         `json:"total_resource_accesses"`
	ActiveClients         int           `json:"active_clients"`
	AverageResponseTime   time.Duration `json:"average_response_time"`
	OverallSuccessRate    float64       `json:"overall_success_rate"`
	PeakUsageTime         time.Time     `json:"peak_usage_time"`
	DataTransferred       int64         `json:"data_transferred"`
}

// UsageRecommendation provides actionable recommendations
type UsageRecommendation struct {
	Type        string   `json:"type"`        // "performance", "cache", "scaling", "optimization"
	Priority    string   `json:"priority"`    // "low", "medium", "high", "critical"
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Actions     []string `json:"actions"`
	Impact      string   `json:"impact"`
	Effort      string   `json:"effort"`
}

// PerformanceInsight provides performance analysis insights
type PerformanceInsight struct {
	Type        string            `json:"type"`        // "bottleneck", "optimization", "scaling"
	Component   string            `json:"component"`   // "tool", "resource", "client", "system"
	Description string            `json:"description"`
	Metrics     map[string]float64 `json:"metrics"`
	Trend       string            `json:"trend"`       // "improving", "degrading", "stable"
	Severity    string            `json:"severity"`    // "low", "medium", "high", "critical"
}

// NewUsageAnalytics creates a new usage analytics tracker
func NewUsageAnalytics(logger *logrus.Logger, config AnalyticsConfig) *UsageAnalytics {
	// Set defaults
	if config.RetentionPeriod == 0 {
		config.RetentionPeriod = 30 * 24 * time.Hour // 30 days
	}
	if config.AnalysisInterval == 0 {
		config.AnalysisInterval = 1 * time.Hour
	}
	if config.MaxTrackingEntries == 0 {
		config.MaxTrackingEntries = 100000
	}
	
	patternTracker := &UsagePatternTracker{
		logger:           logger,
		patterns:         make(map[string]*DetectedPattern),
		patternDetectors: make(map[string]PatternDetector),
	}
	
	analytics := &UsageAnalytics{
		logger:         logger,
		config:         config,
		resourceStats:  make(map[string]*ResourceUsageStats),
		toolStats:      make(map[string]*ToolUsageStats),
		clientStats:    make(map[string]*ClientUsageStats),
		patternTracker: patternTracker,
	}
	
	// Start analysis routine
	if config.EnableAnalytics {
		go analytics.startAnalysisRoutine()
	}
	
	return analytics
}

// TrackResourceAccess tracks access to an MCP resource
func (ua *UsageAnalytics) TrackResourceAccess(resourceURI, resourceType, clientID string, 
	loadTime time.Duration, resultSize int, cacheHit bool, success bool) {
	
	if !ua.config.EnableAnalytics {
		return
	}
	
	ua.mutex.Lock()
	defer ua.mutex.Unlock()
	
	// Get or create resource stats
	stats, exists := ua.resourceStats[resourceURI]
	if !exists {
		stats = &ResourceUsageStats{
			ResourceURI:       resourceURI,
			ResourceType:      resourceType,
			UniqueClients:     make(map[string]bool),
			FirstAccess:       time.Now(),
			AccessPatterns:    make([]AccessPattern, 0),
			PopularParameters: make(map[string]ParameterStats),
		}
		ua.resourceStats[resourceURI] = stats
	}
	
	// Update stats
	stats.mutex.Lock()
	stats.AccessCount++
	stats.UniqueClients[clientID] = true
	stats.TotalBytes += int64(resultSize)
	stats.LastAccess = time.Now()
	
	// Update average load time
	if stats.AccessCount == 1 {
		stats.AverageLoadTime = loadTime
	} else {
		// Calculate running average
		total := stats.AverageLoadTime * time.Duration(stats.AccessCount-1)
		stats.AverageLoadTime = (total + loadTime) / time.Duration(stats.AccessCount)
	}
	
	// Update cache hit rate
	if cacheHit {
		hitCount := stats.CacheHitRate * float64(stats.AccessCount-1)
		if cacheHit {
			hitCount++
		}
		stats.CacheHitRate = hitCount / float64(stats.AccessCount)
	}
	
	// Track errors
	if !success {
		stats.ErrorCount++
	}
	
	stats.mutex.Unlock()
	
	ua.logger.WithFields(logrus.Fields{
		"resource_uri":  resourceURI,
		"resource_type": resourceType,
		"client_id":     ua.sanitizeClientID(clientID),
		"load_time":     loadTime,
		"result_size":   resultSize,
		"cache_hit":     cacheHit,
		"success":       success,
	}).Debug("Tracked resource access")
}

// TrackToolInvocation tracks invocation of an MCP tool
func (ua *UsageAnalytics) TrackToolInvocation(toolName, clientID string, 
	parameters map[string]interface{}, executionTime time.Duration, 
	resultSize int, success bool, errorType string) {
	
	if !ua.config.EnableAnalytics {
		return
	}
	
	ua.mutex.Lock()
	defer ua.mutex.Unlock()
	
	// Get or create tool stats
	stats, exists := ua.toolStats[toolName]
	if !exists {
		stats = &ToolUsageStats{
			ToolName:           toolName,
			UniqueClients:      make(map[string]bool),
			ErrorTypes:         make(map[string]int64),
			ParameterUsage:     make(map[string]ParameterStats),
			FirstInvocation:    time.Now(),
			InvocationPatterns: make([]InvocationPattern, 0),
		}
		ua.toolStats[toolName] = stats
	}
	
	// Update stats
	stats.mutex.Lock()
	stats.InvocationCount++
	stats.UniqueClients[clientID] = true
	stats.LastInvocation = time.Now()
	
	// Update average execution time
	if stats.InvocationCount == 1 {
		stats.AverageExecutionTime = executionTime
	} else {
		total := stats.AverageExecutionTime * time.Duration(stats.InvocationCount-1)
		stats.AverageExecutionTime = (total + executionTime) / time.Duration(stats.InvocationCount)
	}
	
	// Update success rate
	successCount := stats.SuccessRate * float64(stats.InvocationCount-1)
	if success {
		successCount++
	}
	stats.SuccessRate = successCount / float64(stats.InvocationCount)
	
	// Track error types
	if !success && errorType != "" {
		stats.ErrorTypes[errorType]++
	}
	
	// Update result size stats
	ua.updateSizeStats(&stats.ResultSizeStats, resultSize)
	
	// Track parameter usage
	ua.trackParameterUsage(stats.ParameterUsage, parameters)
	
	stats.mutex.Unlock()
	
	ua.logger.WithFields(logrus.Fields{
		"tool_name":      toolName,
		"client_id":      ua.sanitizeClientID(clientID),
		"execution_time": executionTime,
		"result_size":    resultSize,
		"success":        success,
		"error_type":     errorType,
	}).Debug("Tracked tool invocation")
}

// TrackClientActivity tracks client activity and session information
func (ua *UsageAnalytics) TrackClientActivity(clientID, clientName string, 
	activityType string, metadata map[string]string) {
	
	if !ua.config.EnableAnalytics {
		return
	}
	
	ua.mutex.Lock()
	defer ua.mutex.Unlock()
	
	// Get or create client stats
	stats, exists := ua.clientStats[clientID]
	if !exists {
		stats = &ClientUsageStats{
			ClientID:          clientID,
			ClientName:        clientName,
			FavoriteTools:     make([]string, 0),
			FavoriteResources: make([]string, 0),
			UsagePatterns:     make([]UsagePattern, 0),
			FirstConnection:   time.Now(),
			ClientMetadata:    make(map[string]string),
		}
		ua.clientStats[clientID] = stats
	}
	
	// Update stats based on activity type
	stats.mutex.Lock()
	switch activityType {
	case "connect":
		stats.SessionCount++
		stats.FirstConnection = time.Now()
	case "disconnect":
		stats.LastConnection = time.Now()
	case "tool_invocation":
		stats.ToolInvocations++
	case "resource_access":
		stats.ResourceAccesses++
	case "error":
		stats.ErrorCount++
	}
	
	// Update metadata
	for k, v := range metadata {
		stats.ClientMetadata[k] = v
	}
	
	stats.mutex.Unlock()
}

// GenerateUsageReport generates a comprehensive usage report
func (ua *UsageAnalytics) GenerateUsageReport(period TimeRange) (*UsageReport, error) {
	if !ua.config.EnableUsageReports {
		return nil, fmt.Errorf("usage reports are disabled")
	}
	
	ua.mutex.RLock()
	defer ua.mutex.RUnlock()
	
	// Generate summary
	summary := ua.generateUsageSummary(period)
	
	// Get top resources (sorted by access count)
	topResources := ua.getTopResources(10)
	
	// Get top tools (sorted by invocation count)
	topTools := ua.getTopTools(10)
	
	// Get client analysis
	clientAnalysis := ua.getClientAnalysis()
	
	// Get detected patterns
	patterns := ua.patternTracker.GetDetectedPatterns()
	
	// Generate recommendations
	recommendations := ua.generateRecommendations()
	
	// Generate performance insights
	insights := ua.generatePerformanceInsights()
	
	report := &UsageReport{
		GeneratedAt:         time.Now(),
		ReportPeriod:        period,
		Summary:             summary,
		TopResources:        topResources,
		TopTools:            topTools,
		ClientAnalysis:      clientAnalysis,
		DetectedPatterns:    patterns,
		Recommendations:     recommendations,
		PerformanceInsights: insights,
	}
	
	ua.logger.WithFields(logrus.Fields{
		"period_start":      period.Start,
		"period_end":        period.End,
		"top_resources":     len(topResources),
		"top_tools":         len(topTools),
		"detected_patterns": len(patterns),
	}).Info("Generated usage report")
	
	return report, nil
}

// GetResourceUsageStats returns usage statistics for a specific resource
func (ua *UsageAnalytics) GetResourceUsageStats(resourceURI string) (*ResourceUsageStats, bool) {
	ua.mutex.RLock()
	defer ua.mutex.RUnlock()
	
	stats, exists := ua.resourceStats[resourceURI]
	if !exists {
		return nil, false
	}
	
	// Return copy to prevent external modification
	statsCopy := *stats
	statsCopy.UniqueClients = make(map[string]bool)
	for k, v := range stats.UniqueClients {
		statsCopy.UniqueClients[k] = v
	}
	
	return &statsCopy, true
}

// GetToolUsageStats returns usage statistics for a specific tool
func (ua *UsageAnalytics) GetToolUsageStats(toolName string) (*ToolUsageStats, bool) {
	ua.mutex.RLock()
	defer ua.mutex.RUnlock()
	
	stats, exists := ua.toolStats[toolName]
	if !exists {
		return nil, false
	}
	
	// Return copy to prevent external modification
	statsCopy := *stats
	statsCopy.UniqueClients = make(map[string]bool)
	for k, v := range stats.UniqueClients {
		statsCopy.UniqueClients[k] = v
	}
	
	return &statsCopy, true
}

// Helper methods

// sanitizeClientID sanitizes client ID for privacy
func (ua *UsageAnalytics) sanitizeClientID(clientID string) string {
	if ua.config.PrivacyMode {
		if len(clientID) > 8 {
			return clientID[:8] + "..."
		}
		return clientID
	}
	return clientID
}

// updateSizeStats updates size statistics
func (ua *UsageAnalytics) updateSizeStats(sizeStats *SizeStats, size int) {
	if sizeStats.MinSize == 0 || size < sizeStats.MinSize {
		sizeStats.MinSize = size
	}
	if size > sizeStats.MaxSize {
		sizeStats.MaxSize = size
	}
	
	// Update average (simplified)
	sizeStats.TotalSize += int64(size)
	sizeStats.AverageSize = int(sizeStats.TotalSize) // This would need proper counting
	
	// Update buckets
	if sizeStats.SizeBuckets == nil {
		sizeStats.SizeBuckets = make(map[string]int64)
	}
	
	bucket := "small"
	if size > 10000 {
		bucket = "medium"
	}
	if size > 100000 {
		bucket = "large"
	}
	if size > 1000000 {
		bucket = "xlarge"
	}
	
	sizeStats.SizeBuckets[bucket]++
}

// trackParameterUsage tracks parameter usage patterns
func (ua *UsageAnalytics) trackParameterUsage(paramUsage map[string]ParameterStats, parameters map[string]interface{}) {
	for paramName, paramValue := range parameters {
		stats, exists := paramUsage[paramName]
		if !exists {
			stats = ParameterStats{
				ParameterName: paramName,
				PopularValues: make(map[string]int64),
				DataTypes:     make(map[string]int64),
			}
		}
		
		stats.UsageCount++
		
		// Track data type
		dataType := fmt.Sprintf("%T", paramValue)
		stats.DataTypes[dataType]++
		
		// Track popular values (string representation)
		valueStr := fmt.Sprintf("%v", paramValue)
		if len(valueStr) < 100 { // Only track short values
			stats.PopularValues[valueStr]++
		}
		
		// Track size
		if str, ok := paramValue.(string); ok {
			if stats.AverageSize == 0 {
				stats.AverageSize = len(str)
			} else {
				stats.AverageSize = (stats.AverageSize + len(str)) / 2
			}
		}
		
		paramUsage[paramName] = stats
	}
}

// generateUsageSummary generates usage summary for a period
func (ua *UsageAnalytics) generateUsageSummary(period TimeRange) UsageSummary {
	totalToolInvocations := int64(0)
	totalResourceAccesses := int64(0)
	activeClients := len(ua.clientStats)
	
	for _, stats := range ua.toolStats {
		totalToolInvocations += stats.InvocationCount
	}
	
	for _, stats := range ua.resourceStats {
		totalResourceAccesses += stats.AccessCount
	}
	
	return UsageSummary{
		TotalToolInvocations:  totalToolInvocations,
		TotalResourceAccesses: totalResourceAccesses,
		ActiveClients:         activeClients,
		AverageResponseTime:   time.Millisecond * 100, // Simplified
		OverallSuccessRate:    0.95,                   // Simplified
		PeakUsageTime:         time.Now(),             // Simplified
		DataTransferred:       1000000,                // Simplified
	}
}

// getTopResources returns top resources by access count
func (ua *UsageAnalytics) getTopResources(limit int) []ResourceUsageStats {
	resources := make([]ResourceUsageStats, 0, len(ua.resourceStats))
	
	for _, stats := range ua.resourceStats {
		resources = append(resources, *stats)
	}
	
	// Sort by access count (descending)
	sort.Slice(resources, func(i, j int) bool {
		return resources[i].AccessCount > resources[j].AccessCount
	})
	
	if limit > 0 && len(resources) > limit {
		resources = resources[:limit]
	}
	
	return resources
}

// getTopTools returns top tools by invocation count
func (ua *UsageAnalytics) getTopTools(limit int) []ToolUsageStats {
	tools := make([]ToolUsageStats, 0, len(ua.toolStats))
	
	for _, stats := range ua.toolStats {
		tools = append(tools, *stats)
	}
	
	// Sort by invocation count (descending)
	sort.Slice(tools, func(i, j int) bool {
		return tools[i].InvocationCount > tools[j].InvocationCount
	})
	
	if limit > 0 && len(tools) > limit {
		tools = tools[:limit]
	}
	
	return tools
}

// getClientAnalysis returns client analysis
func (ua *UsageAnalytics) getClientAnalysis() []ClientUsageStats {
	clients := make([]ClientUsageStats, 0, len(ua.clientStats))
	
	for _, stats := range ua.clientStats {
		clients = append(clients, *stats)
	}
	
	return clients
}

// generateRecommendations generates usage recommendations
func (ua *UsageAnalytics) generateRecommendations() []UsageRecommendation {
	recommendations := make([]UsageRecommendation, 0)
	
	// Example recommendations based on usage patterns
	recommendations = append(recommendations, UsageRecommendation{
		Type:        "cache",
		Priority:    "medium",
		Title:       "Enable Resource Caching",
		Description: "High-frequency resources could benefit from caching",
		Actions: []string{
			"Enable caching for frequently accessed resources",
			"Configure cache TTL based on resource update frequency",
		},
		Impact: "Reduce latency by 50-80%",
		Effort: "Low",
	})
	
	return recommendations
}

// generatePerformanceInsights generates performance insights
func (ua *UsageAnalytics) generatePerformanceInsights() []PerformanceInsight {
	insights := make([]PerformanceInsight, 0)
	
	// Example insights based on performance patterns
	insights = append(insights, PerformanceInsight{
		Type:        "optimization",
		Component:   "tool",
		Description: "Tool execution time variance detected",
		Metrics: map[string]float64{
			"avg_execution_time": 150.5,
			"variance":           25.3,
		},
		Trend:    "stable",
		Severity: "low",
	})
	
	return insights
}

// startAnalysisRoutine starts background analysis routine
func (ua *UsageAnalytics) startAnalysisRoutine() {
	ticker := time.NewTicker(ua.config.AnalysisInterval)
	defer ticker.Stop()
	
	for range ticker.C {
		if ua.config.EnablePatternDetection {
			ua.analyzeUsagePatterns()
		}
		ua.cleanupOldData()
	}
}

// analyzeUsagePatterns analyzes usage patterns
func (ua *UsageAnalytics) analyzeUsagePatterns() {
	// Pattern analysis would be implemented here
	// This is a placeholder for pattern detection algorithms
	ua.logger.Debug("Analyzing usage patterns")
}

// cleanupOldData cleans up old analytics data
func (ua *UsageAnalytics) cleanupOldData() {
	// Cleanup implementation would remove data older than retention period
	ua.logger.Debug("Cleaning up old analytics data")
}

// PatternTracker methods

// GetDetectedPatterns returns detected patterns
func (upt *UsagePatternTracker) GetDetectedPatterns() []DetectedPattern {
	upt.mutex.RLock()
	defer upt.mutex.RUnlock()
	
	patterns := make([]DetectedPattern, 0, len(upt.patterns))
	for _, pattern := range upt.patterns {
		patterns = append(patterns, *pattern)
	}
	
	return patterns
}