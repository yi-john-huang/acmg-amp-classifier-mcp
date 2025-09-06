package logging

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// MCPLogger provides structured logging for MCP operations with correlation tracking
type MCPLogger struct {
	logger      *logrus.Logger
	config      MCPLoggingConfig
	correlations map[string]*CorrelationContext
	mutex       sync.RWMutex
}

// MCPLoggingConfig configures MCP logging behavior
type MCPLoggingConfig struct {
	Level                string        `json:"level"`                   // "trace", "debug", "info", "warn", "error", "fatal", "panic"
	Format               string        `json:"format"`                  // "json", "text"
	EnableCorrelation    bool          `json:"enable_correlation"`      // Track correlation across operations
	CorrelationTTL       time.Duration `json:"correlation_ttl"`         // TTL for correlation contexts
	EnablePrivacyMode    bool          `json:"enable_privacy_mode"`     // Scrub sensitive data
	EnablePerformanceLog bool          `json:"enable_performance_log"`  // Log performance metrics
	EnableAuditTrail     bool          `json:"enable_audit_trail"`      // Enable detailed audit logging
	MaxCorrelations      int           `json:"max_correlations"`        // Maximum correlations to track
	OutputPath           string        `json:"output_path,omitempty"`   // Optional file output
}

// CorrelationContext tracks related operations across an MCP interaction
type CorrelationContext struct {
	ID          string                 `json:"id"`
	SessionID   string                 `json:"session_id,omitempty"`
	ClientInfo  ClientInfo             `json:"client_info"`
	StartTime   time.Time              `json:"start_time"`
	LastAccess  time.Time              `json:"last_access"`
	Operations  []OperationLog         `json:"operations"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ClientInfo contains information about the MCP client
type ClientInfo struct {
	ID          string            `json:"id"`
	Name        string            `json:"name,omitempty"`
	Version     string            `json:"version,omitempty"`
	Capabilities []string         `json:"capabilities,omitempty"`
	Transport   string            `json:"transport"` // "stdio", "http", "websocket"
	UserAgent   string            `json:"user_agent,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// OperationLog represents a logged MCP operation
type OperationLog struct {
	ID            string                 `json:"id"`
	Type          string                 `json:"type"`          // "tool_call", "resource_access", "prompt_render"
	Name          string                 `json:"name"`          // Tool/resource/prompt name
	StartTime     time.Time              `json:"start_time"`
	EndTime       time.Time              `json:"end_time"`
	Duration      time.Duration          `json:"duration"`
	Success       bool                   `json:"success"`
	Error         string                 `json:"error,omitempty"`
	Parameters    map[string]interface{} `json:"parameters,omitempty"`
	ResultSize    int                    `json:"result_size,omitempty"`
	CacheHit      bool                   `json:"cache_hit,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// Performance metrics for operations
type PerformanceMetrics struct {
	OperationType     string        `json:"operation_type"`
	OperationName     string        `json:"operation_name"`
	ExecutionTime     time.Duration `json:"execution_time"`
	ParameterCount    int           `json:"parameter_count"`
	ResultSize        int           `json:"result_size"`
	MemoryUsage       int64         `json:"memory_usage,omitempty"`
	ExternalAPICalls  int           `json:"external_api_calls"`
	CacheHit          bool          `json:"cache_hit"`
	DatabaseQueries   int           `json:"database_queries,omitempty"`
	Timestamp         time.Time     `json:"timestamp"`
	CorrelationID     string        `json:"correlation_id"`
}

// AuditTrailEntry represents an entry in the audit trail
type AuditTrailEntry struct {
	ID            string                 `json:"id"`
	Timestamp     time.Time              `json:"timestamp"`
	CorrelationID string                 `json:"correlation_id"`
	Operation     string                 `json:"operation"`
	ClientID      string                 `json:"client_id"`
	UserContext   map[string]interface{} `json:"user_context,omitempty"`
	Parameters    map[string]interface{} `json:"parameters,omitempty"`
	Result        string                 `json:"result,omitempty"` // Sanitized result summary
	Success       bool                   `json:"success"`
	Duration      time.Duration          `json:"duration"`
	IPAddress     string                 `json:"ip_address,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// LogLevel constants
const (
	TraceLevel = "trace"
	DebugLevel = "debug"
	InfoLevel  = "info"
	WarnLevel  = "warn"
	ErrorLevel = "error"
	FatalLevel = "fatal"
	PanicLevel = "panic"
)

// Operation types
const (
	OperationToolCall      = "tool_call"
	OperationResourceAccess = "resource_access"
	OperationPromptRender  = "prompt_render"
	OperationHealthCheck   = "health_check"
	OperationClientConnect = "client_connect"
	OperationClientDisconnect = "client_disconnect"
)

// NewMCPLogger creates a new MCP-aware logger
func NewMCPLogger(config MCPLoggingConfig) *MCPLogger {
	logger := logrus.New()
	
	// Set log level
	level, err := logrus.ParseLevel(config.Level)
	if err != nil {
		level = logrus.InfoLevel
	}
	logger.SetLevel(level)
	
	// Set formatter
	if config.Format == "json" {
		logger.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: time.RFC3339Nano,
			FieldMap: logrus.FieldMap{
				logrus.FieldKeyTime:  "timestamp",
				logrus.FieldKeyLevel: "level",
				logrus.FieldKeyMsg:   "message",
			},
		})
	} else {
		logger.SetFormatter(&logrus.TextFormatter{
			TimestampFormat: time.RFC3339,
			FullTimestamp:   true,
		})
	}
	
	// Set defaults
	if config.CorrelationTTL == 0 {
		config.CorrelationTTL = 1 * time.Hour
	}
	if config.MaxCorrelations == 0 {
		config.MaxCorrelations = 10000
	}
	
	mcpLogger := &MCPLogger{
		logger:       logger,
		config:       config,
		correlations: make(map[string]*CorrelationContext),
	}
	
	// Start cleanup routine
	go mcpLogger.startCleanupRoutine()
	
	return mcpLogger
}

// GetCorrelationID extracts or creates correlation ID from context
func (ml *MCPLogger) GetCorrelationID(ctx context.Context) string {
	if correlationID, ok := ctx.Value("correlation_id").(string); ok {
		return correlationID
	}
	
	// Generate new correlation ID
	correlationID := uuid.New().String()
	return correlationID
}

// WithCorrelation creates a new context with correlation ID
func (ml *MCPLogger) WithCorrelation(ctx context.Context, correlationID string) context.Context {
	return context.WithValue(ctx, "correlation_id", correlationID)
}

// StartOperation begins tracking an MCP operation
func (ml *MCPLogger) StartOperation(ctx context.Context, operationType, operationName string, params map[string]interface{}) (context.Context, string) {
	correlationID := ml.GetCorrelationID(ctx)
	operationID := uuid.New().String()
	
	// Create operation log entry
	operation := OperationLog{
		ID:         operationID,
		Type:       operationType,
		Name:       operationName,
		StartTime:  time.Now(),
		Parameters: ml.sanitizeParameters(params),
		Metadata:   make(map[string]interface{}),
	}
	
	// Update correlation context
	ml.updateCorrelationContext(correlationID, operation)
	
	// Log operation start
	ml.logger.WithFields(logrus.Fields{
		"correlation_id":   correlationID,
		"operation_id":     operationID,
		"operation_type":   operationType,
		"operation_name":   operationName,
		"parameter_count":  len(params),
	}).Info("MCP operation started")
	
	// Add operation ID to context
	newCtx := context.WithValue(ctx, "operation_id", operationID)
	return newCtx, operationID
}

// EndOperation completes tracking an MCP operation
func (ml *MCPLogger) EndOperation(ctx context.Context, operationID string, success bool, resultSize int, err error) {
	correlationID := ml.GetCorrelationID(ctx)
	endTime := time.Now()
	
	ml.mutex.Lock()
	defer ml.mutex.Unlock()
	
	// Find and update the operation
	if correlation, exists := ml.correlations[correlationID]; exists {
		for i, op := range correlation.Operations {
			if op.ID == operationID {
				correlation.Operations[i].EndTime = endTime
				correlation.Operations[i].Duration = endTime.Sub(op.StartTime)
				correlation.Operations[i].Success = success
				correlation.Operations[i].ResultSize = resultSize
				
				if err != nil {
					correlation.Operations[i].Error = ml.sanitizeError(err)
				}
				
				correlation.LastAccess = endTime
				break
			}
		}
	}
	
	// Create log entry
	logEntry := ml.logger.WithFields(logrus.Fields{
		"correlation_id": correlationID,
		"operation_id":   operationID,
		"success":        success,
		"result_size":    resultSize,
	})
	
	if err != nil {
		logEntry = logEntry.WithError(err)
		logEntry.Error("MCP operation failed")
	} else {
		logEntry.Info("MCP operation completed")
	}
	
	// Log performance metrics if enabled
	if ml.config.EnablePerformanceLog {
		ml.logPerformanceMetrics(correlationID, operationID)
	}
	
	// Log audit trail if enabled
	if ml.config.EnableAuditTrail {
		ml.logAuditTrail(correlationID, operationID, success, err)
	}
}

// LogClientConnection logs a client connection event
func (ml *MCPLogger) LogClientConnection(clientInfo ClientInfo) {
	correlationID := uuid.New().String()
	
	// Create correlation context for client
	ml.mutex.Lock()
	ml.correlations[correlationID] = &CorrelationContext{
		ID:         correlationID,
		ClientInfo: clientInfo,
		StartTime:  time.Now(),
		LastAccess: time.Now(),
		Operations: make([]OperationLog, 0),
		Metadata:   make(map[string]interface{}),
	}
	ml.mutex.Unlock()
	
	ml.logger.WithFields(logrus.Fields{
		"correlation_id": correlationID,
		"client_id":      clientInfo.ID,
		"client_name":    clientInfo.Name,
		"transport":      clientInfo.Transport,
		"capabilities":   len(clientInfo.Capabilities),
	}).Info("MCP client connected")
}

// LogClientDisconnection logs a client disconnection event
func (ml *MCPLogger) LogClientDisconnection(clientID string, reason string) {
	ml.logger.WithFields(logrus.Fields{
		"client_id": clientID,
		"reason":    reason,
	}).Info("MCP client disconnected")
	
	// Clean up correlations for this client
	ml.mutex.Lock()
	defer ml.mutex.Unlock()
	
	for id, correlation := range ml.correlations {
		if correlation.ClientInfo.ID == clientID {
			// Log final correlation summary
			ml.logger.WithFields(logrus.Fields{
				"correlation_id":    id,
				"client_id":         clientID,
				"session_duration":  time.Since(correlation.StartTime),
				"operations_count":  len(correlation.Operations),
			}).Info("Client correlation summary")
			
			delete(ml.correlations, id)
		}
	}
}

// LogError logs an error with MCP context
func (ml *MCPLogger) LogError(ctx context.Context, err error, message string, fields map[string]interface{}) {
	correlationID := ml.GetCorrelationID(ctx)
	
	logFields := logrus.Fields{
		"correlation_id": correlationID,
	}
	
	// Add additional fields
	for k, v := range fields {
		logFields[k] = ml.sanitizeField(k, v)
	}
	
	ml.logger.WithFields(logFields).WithError(err).Error(message)
}

// LogInfo logs an info message with MCP context
func (ml *MCPLogger) LogInfo(ctx context.Context, message string, fields map[string]interface{}) {
	correlationID := ml.GetCorrelationID(ctx)
	
	logFields := logrus.Fields{
		"correlation_id": correlationID,
	}
	
	// Add additional fields
	for k, v := range fields {
		logFields[k] = ml.sanitizeField(k, v)
	}
	
	ml.logger.WithFields(logFields).Info(message)
}

// LogDebug logs a debug message with MCP context
func (ml *MCPLogger) LogDebug(ctx context.Context, message string, fields map[string]interface{}) {
	correlationID := ml.GetCorrelationID(ctx)
	
	logFields := logrus.Fields{
		"correlation_id": correlationID,
	}
	
	// Add additional fields
	for k, v := range fields {
		logFields[k] = ml.sanitizeField(k, v)
	}
	
	ml.logger.WithFields(logFields).Debug(message)
}

// GetCorrelationContext retrieves correlation context by ID
func (ml *MCPLogger) GetCorrelationContext(correlationID string) (*CorrelationContext, bool) {
	ml.mutex.RLock()
	defer ml.mutex.RUnlock()
	
	correlation, exists := ml.correlations[correlationID]
	if !exists {
		return nil, false
	}
	
	// Return copy to prevent external mutations
	correlationCopy := *correlation
	correlationCopy.Operations = make([]OperationLog, len(correlation.Operations))
	copy(correlationCopy.Operations, correlation.Operations)
	
	return &correlationCopy, true
}

// GetActiveCorrelations returns all active correlation contexts
func (ml *MCPLogger) GetActiveCorrelations() map[string]*CorrelationContext {
	ml.mutex.RLock()
	defer ml.mutex.RUnlock()
	
	result := make(map[string]*CorrelationContext)
	for id, correlation := range ml.correlations {
		correlationCopy := *correlation
		correlationCopy.Operations = make([]OperationLog, len(correlation.Operations))
		copy(correlationCopy.Operations, correlation.Operations)
		result[id] = &correlationCopy
	}
	
	return result
}

// updateCorrelationContext updates correlation with new operation
func (ml *MCPLogger) updateCorrelationContext(correlationID string, operation OperationLog) {
	ml.mutex.Lock()
	defer ml.mutex.Unlock()
	
	correlation, exists := ml.correlations[correlationID]
	if !exists {
		// Create new correlation if it doesn't exist
		correlation = &CorrelationContext{
			ID:         correlationID,
			StartTime:  time.Now(),
			LastAccess: time.Now(),
			Operations: make([]OperationLog, 0),
			Metadata:   make(map[string]interface{}),
		}
		ml.correlations[correlationID] = correlation
	}
	
	correlation.Operations = append(correlation.Operations, operation)
	correlation.LastAccess = time.Now()
}

// logPerformanceMetrics logs performance metrics for an operation
func (ml *MCPLogger) logPerformanceMetrics(correlationID, operationID string) {
	correlation, exists := ml.correlations[correlationID]
	if !exists {
		return
	}
	
	for _, op := range correlation.Operations {
		if op.ID == operationID {
			metrics := PerformanceMetrics{
				OperationType:     op.Type,
				OperationName:     op.Name,
				ExecutionTime:     op.Duration,
				ParameterCount:    len(op.Parameters),
				ResultSize:        op.ResultSize,
				CacheHit:          op.CacheHit,
				Timestamp:         op.EndTime,
				CorrelationID:     correlationID,
			}
			
			ml.logger.WithFields(logrus.Fields{
				"metric_type":      "performance",
				"correlation_id":   correlationID,
				"operation_type":   metrics.OperationType,
				"operation_name":   metrics.OperationName,
				"execution_time":   metrics.ExecutionTime,
				"parameter_count":  metrics.ParameterCount,
				"result_size":      metrics.ResultSize,
				"cache_hit":        metrics.CacheHit,
			}).Info("Performance metrics")
			
			break
		}
	}
}

// logAuditTrail logs audit trail entry for an operation
func (ml *MCPLogger) logAuditTrail(correlationID, operationID string, success bool, err error) {
	correlation, exists := ml.correlations[correlationID]
	if !exists {
		return
	}
	
	for _, op := range correlation.Operations {
		if op.ID == operationID {
			auditEntry := AuditTrailEntry{
				ID:            uuid.New().String(),
				Timestamp:     op.EndTime,
				CorrelationID: correlationID,
				Operation:     fmt.Sprintf("%s:%s", op.Type, op.Name),
				ClientID:      correlation.ClientInfo.ID,
				Success:       success,
				Duration:      op.Duration,
			}
			
			// Sanitize and add result summary
			if success {
				auditEntry.Result = fmt.Sprintf("Success: %d bytes", op.ResultSize)
			} else if err != nil {
				auditEntry.Result = fmt.Sprintf("Error: %s", ml.sanitizeError(err))
			}
			
			ml.logger.WithFields(logrus.Fields{
				"audit_type":     "operation",
				"audit_id":       auditEntry.ID,
				"correlation_id": auditEntry.CorrelationID,
				"operation":      auditEntry.Operation,
				"client_id":      auditEntry.ClientID,
				"success":        auditEntry.Success,
				"duration":       auditEntry.Duration,
			}).Info("Audit trail entry")
			
			break
		}
	}
}

// sanitizeParameters removes sensitive data from parameters
func (ml *MCPLogger) sanitizeParameters(params map[string]interface{}) map[string]interface{} {
	if !ml.config.EnablePrivacyMode {
		return params
	}
	
	sanitized := make(map[string]interface{})
	for k, v := range params {
		sanitized[k] = ml.sanitizeField(k, v)
	}
	
	return sanitized
}

// sanitizeField sanitizes individual fields
func (ml *MCPLogger) sanitizeField(key string, value interface{}) interface{} {
	if !ml.config.EnablePrivacyMode {
		return value
	}
	
	// List of sensitive field patterns
	sensitivePatterns := []string{
		"password", "token", "secret", "key", "auth",
		"patient", "user", "email", "phone", "address",
	}
	
	lowerKey := strings.ToLower(key)
	for _, pattern := range sensitivePatterns {
		if strings.Contains(lowerKey, pattern) {
			return "[REDACTED]"
		}
	}
	
	// Truncate very long values
	if str, ok := value.(string); ok && len(str) > 1000 {
		return str[:1000] + "... [TRUNCATED]"
	}
	
	return value
}

// sanitizeError sanitizes error messages
func (ml *MCPLogger) sanitizeError(err error) string {
	if err == nil {
		return ""
	}
	
	errorMsg := err.Error()
	if ml.config.EnablePrivacyMode {
		// Remove potential sensitive information from error messages
		if len(errorMsg) > 500 {
			errorMsg = errorMsg[:500] + "... [TRUNCATED]"
		}
	}
	
	return errorMsg
}

// startCleanupRoutine starts background cleanup of expired correlations
func (ml *MCPLogger) startCleanupRoutine() {
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()
	
	for range ticker.C {
		ml.cleanupExpiredCorrelations()
	}
}

// cleanupExpiredCorrelations removes expired correlation contexts
func (ml *MCPLogger) cleanupExpiredCorrelations() {
	ml.mutex.Lock()
	defer ml.mutex.Unlock()
	
	now := time.Now()
	removed := 0
	
	for id, correlation := range ml.correlations {
		if now.Sub(correlation.LastAccess) > ml.config.CorrelationTTL {
			delete(ml.correlations, id)
			removed++
		}
	}
	
	// Also limit max correlations
	if len(ml.correlations) > ml.config.MaxCorrelations {
		// Remove oldest correlations
		type correlationAge struct {
			id       string
			lastTime time.Time
		}
		
		correlations := make([]correlationAge, 0, len(ml.correlations))
		for id, correlation := range ml.correlations {
			correlations = append(correlations, correlationAge{
				id:       id,
				lastTime: correlation.LastAccess,
			})
		}
		
		// Sort by last access time (oldest first)
		for i := 0; i < len(correlations)-1; i++ {
			for j := i + 1; j < len(correlations); j++ {
				if correlations[i].lastTime.After(correlations[j].lastTime) {
					correlations[i], correlations[j] = correlations[j], correlations[i]
				}
			}
		}
		
		// Remove oldest correlations until we're under the limit
		toRemove := len(ml.correlations) - ml.config.MaxCorrelations
		for i := 0; i < toRemove && i < len(correlations); i++ {
			delete(ml.correlations, correlations[i].id)
			removed++
		}
	}
	
	if removed > 0 {
		ml.logger.WithField("removed_correlations", removed).Debug("Cleaned up expired correlations")
	}
}

// GetLoggingStats returns logging statistics
func (ml *MCPLogger) GetLoggingStats() map[string]interface{} {
	ml.mutex.RLock()
	defer ml.mutex.RUnlock()
	
	totalOperations := 0
	successfulOperations := 0
	failedOperations := 0
	
	for _, correlation := range ml.correlations {
		totalOperations += len(correlation.Operations)
		for _, op := range correlation.Operations {
			if op.Success {
				successfulOperations++
			} else {
				failedOperations++
			}
		}
	}
	
	stats := map[string]interface{}{
		"active_correlations":    len(ml.correlations),
		"total_operations":       totalOperations,
		"successful_operations":  successfulOperations,
		"failed_operations":      failedOperations,
		"correlation_ttl":        ml.config.CorrelationTTL.String(),
		"privacy_mode_enabled":   ml.config.EnablePrivacyMode,
		"performance_log_enabled": ml.config.EnablePerformanceLog,
		"audit_trail_enabled":    ml.config.EnableAuditTrail,
	}
	
	if totalOperations > 0 {
		stats["success_rate"] = float64(successfulOperations) / float64(totalOperations)
	}
	
	return stats
}