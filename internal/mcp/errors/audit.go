package errors

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// ErrorAuditTrail manages error correlation and audit logging
type ErrorAuditTrail struct {
	logger           *logrus.Logger
	correlationStore map[string]*CorrelationChain
	auditStore       map[string]*AuditEntry
	mutex            sync.RWMutex
	config           AuditConfig
	cleanupTicker    *time.Ticker
	stopChan         chan struct{}
}

// AuditConfig configures audit trail behavior
type AuditConfig struct {
	RetentionPeriod      time.Duration `json:"retention_period"`
	MaxEntriesPerChain   int           `json:"max_entries_per_chain"`
	MaxCorrelationChains int           `json:"max_correlation_chains"`
	EnableMetrics        bool          `json:"enable_metrics"`
	PersistToFile        bool          `json:"persist_to_file"`
	FilePath             string        `json:"file_path,omitempty"`
	CleanupInterval      time.Duration `json:"cleanup_interval"`
}

// CorrelationChain tracks related errors across requests
type CorrelationChain struct {
	ID            string       `json:"id"`
	RootRequestID string       `json:"root_request_id"`
	UserID        string       `json:"user_id,omitempty"`
	ServiceName   string       `json:"service_name"`
	StartTime     time.Time    `json:"start_time"`
	LastActivity  time.Time    `json:"last_activity"`
	Entries       []AuditEntry `json:"entries"`
	Pattern       string       `json:"pattern,omitempty"`
	Status        string       `json:"status"` // "active", "resolved", "escalated"
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// AuditEntry represents a single error event in the audit trail
type AuditEntry struct {
	ID            string                 `json:"id"`
	CorrelationID string                 `json:"correlation_id"`
	RequestID     string                 `json:"request_id"`
	UserID        string                 `json:"user_id,omitempty"`
	SessionID     string                 `json:"session_id,omitempty"`
	ServiceName   string                 `json:"service_name"`
	Operation     string                 `json:"operation"`
	ErrorCode     int                    `json:"error_code"`
	ErrorMessage  string                 `json:"error_message"`
	ErrorDetails  map[string]interface{} `json:"error_details,omitempty"`
	Severity      string                 `json:"severity"`
	Category      string                 `json:"category"`
	Stack         []StackFrame           `json:"stack,omitempty"`
	Context       ErrorEventContext      `json:"context"`
	Resolution    *ResolutionInfo        `json:"resolution,omitempty"`
	Timestamp     time.Time              `json:"timestamp"`
}

// ErrorEventContext captures context information for audit entries
type ErrorEventContext struct {
	ClientInfo    map[string]interface{} `json:"client_info,omitempty"`
	SystemState   map[string]interface{} `json:"system_state,omitempty"`
	RequestParams map[string]interface{} `json:"request_params,omitempty"`
	ResponseData  map[string]interface{} `json:"response_data,omitempty"`
	Timing        TimingInfo             `json:"timing"`
	Environment   EnvironmentInfo        `json:"environment"`
}

// StackFrame represents a frame in the error stack trace
type StackFrame struct {
	Function string `json:"function"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	Package  string `json:"package,omitempty"`
}

// TimingInfo captures timing-related context
type TimingInfo struct {
	RequestStartTime  time.Time     `json:"request_start_time"`
	ErrorTime         time.Time     `json:"error_time"`
	ProcessingTime    time.Duration `json:"processing_time"`
	ExternalCallTime  time.Duration `json:"external_call_time,omitempty"`
	TimeToFirstError  time.Duration `json:"time_to_first_error,omitempty"`
}

// EnvironmentInfo captures environment context
type EnvironmentInfo struct {
	ServiceVersion    string                 `json:"service_version,omitempty"`
	Environment       string                 `json:"environment"` // "dev", "staging", "prod"
	Region            string                 `json:"region,omitempty"`
	InstanceID        string                 `json:"instance_id,omitempty"`
	ResourceUsage     map[string]interface{} `json:"resource_usage,omitempty"`
	Dependencies      []DependencyInfo       `json:"dependencies,omitempty"`
}

// DependencyInfo captures dependency status
type DependencyInfo struct {
	Name         string        `json:"name"`
	Version      string        `json:"version,omitempty"`
	Status       string        `json:"status"` // "healthy", "degraded", "unhealthy"
	ResponseTime time.Duration `json:"response_time,omitempty"`
	ErrorRate    float64       `json:"error_rate,omitempty"`
}

// ResolutionInfo tracks error resolution details
type ResolutionInfo struct {
	ResolvedAt      time.Time              `json:"resolved_at"`
	ResolvedBy      string                 `json:"resolved_by"` // "auto", "manual", "user"
	Method          string                 `json:"method"`      // "retry", "fallback", "manual_fix"
	Success         bool                   `json:"success"`
	AttemptCount    int                    `json:"attempt_count"`
	ResolutionTime  time.Duration          `json:"resolution_time"`
	Description     string                 `json:"description,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// ErrorPatternAnalyzer analyzes error patterns in audit trails
type ErrorPatternAnalyzer struct {
	logger   *logrus.Logger
	patterns map[string]PatternDefinition
}

// PatternDefinition defines how to detect error patterns
type PatternDefinition struct {
	ID              string        `json:"id"`
	Name            string        `json:"name"`
	Description     string        `json:"description"`
	TimeWindow      time.Duration `json:"time_window"`
	MinOccurrences  int           `json:"min_occurrences"`
	MatchCriteria   []MatchCriterion `json:"match_criteria"`
	Severity        string        `json:"severity"`
	AutoEscalate    bool          `json:"auto_escalate"`
	NotificationChannels []string `json:"notification_channels,omitempty"`
}

// MatchCriterion defines criteria for pattern matching
type MatchCriterion struct {
	Field    string      `json:"field"`    // "error_code", "service_name", "user_id", etc.
	Operator string      `json:"operator"` // "equals", "contains", "regex"
	Value    interface{} `json:"value"`
	Weight   float64     `json:"weight,omitempty"` // Importance of this criterion
}

// PatternMatch represents a detected error pattern
type PatternMatch struct {
	PatternID       string        `json:"pattern_id"`
	PatternName     string        `json:"pattern_name"`
	MatchedEntries  []string      `json:"matched_entries"` // AuditEntry IDs
	FirstOccurrence time.Time     `json:"first_occurrence"`
	LastOccurrence  time.Time     `json:"last_occurrence"`
	Frequency       int           `json:"frequency"`
	TimeSpan        time.Duration `json:"time_span"`
	Confidence      float64       `json:"confidence"`
	Impact          string        `json:"impact"` // "low", "medium", "high", "critical"
	Trends          []TrendData   `json:"trends,omitempty"`
}

// TrendData represents trend information for patterns
type TrendData struct {
	Timestamp   time.Time `json:"timestamp"`
	Count       int       `json:"count"`
	AverageRate float64   `json:"average_rate"`
}

// NewErrorAuditTrail creates a new error audit trail
func NewErrorAuditTrail(logger *logrus.Logger, config AuditConfig) *ErrorAuditTrail {
	if config.RetentionPeriod == 0 {
		config.RetentionPeriod = 7 * 24 * time.Hour // 7 days default
	}
	if config.MaxEntriesPerChain == 0 {
		config.MaxEntriesPerChain = 100
	}
	if config.MaxCorrelationChains == 0 {
		config.MaxCorrelationChains = 10000
	}
	if config.CleanupInterval == 0 {
		config.CleanupInterval = 1 * time.Hour
	}

	audit := &ErrorAuditTrail{
		logger:           logger,
		correlationStore: make(map[string]*CorrelationChain),
		auditStore:       make(map[string]*AuditEntry),
		config:           config,
		stopChan:         make(chan struct{}),
	}

	// Start cleanup routine
	audit.startCleanupRoutine()

	return audit
}

// LogError logs an error to the audit trail with correlation
func (eat *ErrorAuditTrail) LogError(ctx context.Context, correlationID string, err error, context ErrorEventContext) (*AuditEntry, error) {
	entry := &AuditEntry{
		ID:            generateAuditEntryID(),
		CorrelationID: correlationID,
		ErrorMessage:  err.Error(),
		Context:       context,
		Timestamp:     time.Now(),
	}
	
	// Safely extract request ID
	if requestID, ok := context.RequestParams["request_id"].(string); ok {
		entry.RequestID = requestID
	} else {
		entry.RequestID = "unknown"
	}
	
	// Safely extract service name
	if context.Environment.ServiceVersion != "" {
		entry.ServiceName = context.Environment.ServiceVersion
	} else {
		entry.ServiceName = "unknown"
	}

	// Extract error details if it's an MCPError
	if mcpErr, ok := err.(*MCPError); ok {
		entry.ErrorCode = mcpErr.Code
		entry.ErrorDetails = mcpErr.Data
		entry.Severity = mcpErr.Severity
		entry.Category = mcpErr.Category
	} else {
		entry.ErrorCode = ErrorInternalError
		entry.Severity = SeverityMedium
		entry.Category = CategorySystem
	}

	// Add to correlation chain
	eat.addToCorrelationChain(correlationID, entry)

	// Store audit entry
	eat.mutex.Lock()
	eat.auditStore[entry.ID] = entry
	eat.mutex.Unlock()

	// Log structured entry
	eat.logger.WithFields(logrus.Fields{
		"correlation_id": correlationID,
		"request_id":     entry.RequestID,
		"error_code":     entry.ErrorCode,
		"service":        entry.ServiceName,
		"operation":      entry.Operation,
		"severity":       entry.Severity,
		"category":       entry.Category,
		"user_id":        entry.UserID,
	}).Error("Error logged to audit trail")

	return entry, nil
}

// addToCorrelationChain adds an entry to the appropriate correlation chain
func (eat *ErrorAuditTrail) addToCorrelationChain(correlationID string, entry *AuditEntry) {
	eat.mutex.Lock()
	defer eat.mutex.Unlock()

	chain, exists := eat.correlationStore[correlationID]
	if !exists {
		// Create new correlation chain
		chain = &CorrelationChain{
			ID:            correlationID,
			RootRequestID: entry.RequestID,
			UserID:        entry.UserID,
			ServiceName:   entry.ServiceName,
			StartTime:     time.Now(),
			LastActivity:  time.Now(),
			Entries:       make([]AuditEntry, 0),
			Status:        "active",
			Metadata:      make(map[string]interface{}),
		}
		eat.correlationStore[correlationID] = chain
	}

	// Add entry to chain
	chain.Entries = append(chain.Entries, *entry)
	chain.LastActivity = time.Now()

	// Trim chain if it gets too long
	if len(chain.Entries) > eat.config.MaxEntriesPerChain {
		chain.Entries = chain.Entries[len(chain.Entries)-eat.config.MaxEntriesPerChain:]
	}

	// Update chain status based on error patterns
	eat.updateChainStatus(chain)
}

// updateChainStatus updates the status of a correlation chain
func (eat *ErrorAuditTrail) updateChainStatus(chain *CorrelationChain) {
	if len(chain.Entries) == 0 {
		return
	}

	recentErrors := 0
	now := time.Now()
	
	// Count errors in the last 5 minutes
	for _, entry := range chain.Entries {
		if now.Sub(entry.Timestamp) <= 5*time.Minute {
			recentErrors++
		}
	}

	// Update status based on error frequency
	if recentErrors >= 10 {
		chain.Status = "escalated"
	} else if recentErrors >= 5 {
		chain.Status = "degraded"
	} else if recentErrors == 0 && now.Sub(chain.LastActivity) > 10*time.Minute {
		chain.Status = "resolved"
	}
}

// GetCorrelationChain retrieves a correlation chain by ID
func (eat *ErrorAuditTrail) GetCorrelationChain(correlationID string) (*CorrelationChain, error) {
	eat.mutex.RLock()
	defer eat.mutex.RUnlock()

	chain, exists := eat.correlationStore[correlationID]
	if !exists {
		return nil, fmt.Errorf("correlation chain %s not found", correlationID)
	}

	// Return a copy to prevent external modifications
	chainCopy := *chain
	chainCopy.Entries = make([]AuditEntry, len(chain.Entries))
	copy(chainCopy.Entries, chain.Entries)

	return &chainCopy, nil
}

// SearchAuditEntries searches audit entries by criteria
func (eat *ErrorAuditTrail) SearchAuditEntries(ctx context.Context, criteria SearchCriteria) ([]*AuditEntry, error) {
	eat.mutex.RLock()
	defer eat.mutex.RUnlock()

	var results []*AuditEntry
	
	for _, entry := range eat.auditStore {
		if eat.matchesCriteria(entry, criteria) {
			results = append(results, entry)
		}
	}

	// Sort by timestamp (newest first)
	// Implementation would sort results here

	// Apply limit
	if criteria.Limit > 0 && len(results) > criteria.Limit {
		results = results[:criteria.Limit]
	}

	return results, nil
}

// SearchCriteria defines search parameters for audit entries
type SearchCriteria struct {
	StartTime     *time.Time             `json:"start_time,omitempty"`
	EndTime       *time.Time             `json:"end_time,omitempty"`
	ServiceName   string                 `json:"service_name,omitempty"`
	UserID        string                 `json:"user_id,omitempty"`
	ErrorCodes    []int                  `json:"error_codes,omitempty"`
	Severity      string                 `json:"severity,omitempty"`
	Category      string                 `json:"category,omitempty"`
	CorrelationID string                 `json:"correlation_id,omitempty"`
	Limit         int                    `json:"limit,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// matchesCriteria checks if an audit entry matches search criteria
func (eat *ErrorAuditTrail) matchesCriteria(entry *AuditEntry, criteria SearchCriteria) bool {
	// Time range check
	if criteria.StartTime != nil && entry.Timestamp.Before(*criteria.StartTime) {
		return false
	}
	if criteria.EndTime != nil && entry.Timestamp.After(*criteria.EndTime) {
		return false
	}

	// Service name check
	if criteria.ServiceName != "" && entry.ServiceName != criteria.ServiceName {
		return false
	}

	// User ID check
	if criteria.UserID != "" && entry.UserID != criteria.UserID {
		return false
	}

	// Error codes check
	if len(criteria.ErrorCodes) > 0 {
		found := false
		for _, code := range criteria.ErrorCodes {
			if entry.ErrorCode == code {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Severity check
	if criteria.Severity != "" && entry.Severity != criteria.Severity {
		return false
	}

	// Category check
	if criteria.Category != "" && entry.Category != criteria.Category {
		return false
	}

	// Correlation ID check
	if criteria.CorrelationID != "" && entry.CorrelationID != criteria.CorrelationID {
		return false
	}

	return true
}

// MarkResolved marks an error as resolved in the audit trail
func (eat *ErrorAuditTrail) MarkResolved(entryID, method, resolvedBy, description string) error {
	eat.mutex.Lock()
	defer eat.mutex.Unlock()

	entry, exists := eat.auditStore[entryID]
	if !exists {
		return fmt.Errorf("audit entry %s not found", entryID)
	}

	resolution := &ResolutionInfo{
		ResolvedAt:     time.Now(),
		ResolvedBy:     resolvedBy,
		Method:         method,
		Success:        true,
		ResolutionTime: time.Since(entry.Timestamp),
		Description:    description,
	}

	entry.Resolution = resolution

	eat.logger.WithFields(logrus.Fields{
		"entry_id":        entryID,
		"correlation_id":  entry.CorrelationID,
		"method":          method,
		"resolved_by":     resolvedBy,
		"resolution_time": resolution.ResolutionTime,
	}).Info("Error marked as resolved in audit trail")

	return nil
}

// GetAuditStats returns statistics about the audit trail
func (eat *ErrorAuditTrail) GetAuditStats() map[string]interface{} {
	eat.mutex.RLock()
	defer eat.mutex.RUnlock()

	stats := map[string]interface{}{
		"total_entries":         len(eat.auditStore),
		"total_correlations":    len(eat.correlationStore),
		"retention_period":      eat.config.RetentionPeriod.String(),
		"cleanup_interval":      eat.config.CleanupInterval.String(),
	}

	// Count by severity
	severityCounts := make(map[string]int)
	categoryCounts := make(map[string]int)
	resolvedCount := 0

	for _, entry := range eat.auditStore {
		severityCounts[entry.Severity]++
		categoryCounts[entry.Category]++
		if entry.Resolution != nil {
			resolvedCount++
		}
	}

	stats["by_severity"] = severityCounts
	stats["by_category"] = categoryCounts
	stats["resolved_count"] = resolvedCount
	stats["resolution_rate"] = float64(resolvedCount) / float64(len(eat.auditStore))

	return stats
}

// startCleanupRoutine starts the background cleanup routine
func (eat *ErrorAuditTrail) startCleanupRoutine() {
	eat.cleanupTicker = time.NewTicker(eat.config.CleanupInterval)
	
	go func() {
		for {
			select {
			case <-eat.cleanupTicker.C:
				eat.cleanup()
			case <-eat.stopChan:
				return
			}
		}
	}()
}

// cleanup removes old entries from the audit trail
func (eat *ErrorAuditTrail) cleanup() {
	eat.mutex.Lock()
	defer eat.mutex.Unlock()

	now := time.Now()
	cutoff := now.Add(-eat.config.RetentionPeriod)

	// Clean audit entries
	removedEntries := 0
	for id, entry := range eat.auditStore {
		if entry.Timestamp.Before(cutoff) {
			delete(eat.auditStore, id)
			removedEntries++
		}
	}

	// Clean correlation chains
	removedChains := 0
	for id, chain := range eat.correlationStore {
		if chain.LastActivity.Before(cutoff) {
			delete(eat.correlationStore, id)
			removedChains++
		}
	}

	if removedEntries > 0 || removedChains > 0 {
		eat.logger.WithFields(logrus.Fields{
			"removed_entries": removedEntries,
			"removed_chains":  removedChains,
			"cutoff_time":     cutoff,
		}).Info("Cleaned up expired audit trail entries")
	}
}

// Stop stops the audit trail and cleanup routine
func (eat *ErrorAuditTrail) Stop() {
	if eat.cleanupTicker != nil {
		eat.cleanupTicker.Stop()
	}
	close(eat.stopChan)
	
	eat.logger.Info("Error audit trail stopped")
}

// ExportAuditTrail exports audit trail data for external analysis
func (eat *ErrorAuditTrail) ExportAuditTrail(ctx context.Context, criteria SearchCriteria, format string) ([]byte, error) {
	entries, err := eat.SearchAuditEntries(ctx, criteria)
	if err != nil {
		return nil, fmt.Errorf("failed to search audit entries: %w", err)
	}

	switch format {
	case "json":
		return json.MarshalIndent(entries, "", "  ")
	default:
		return nil, fmt.Errorf("unsupported export format: %s", format)
	}
}

// Helper functions

// generateAuditEntryID generates a unique ID for an audit entry
func generateAuditEntryID() string {
	return fmt.Sprintf("audit_%d_%s", time.Now().UnixNano(), generateCorrelationID()[:8])
}