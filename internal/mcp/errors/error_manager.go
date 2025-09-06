package errors

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// ErrorManager manages comprehensive error handling for the MCP server
type ErrorManager struct {
	logger            *logrus.Logger
	correlations      map[string]*ErrorCorrelation
	mutex             sync.RWMutex
	circuitManager    *CircuitBreakerManager
	auditTrail        *ErrorAuditTrail
	toolHandler       *ToolErrorHandler
	degradationMgr    *GracefulDegradationManager
	recoveryManager   *RecoveryGuidanceManager
	config            ErrorManagerConfig
}

// ErrorManagerConfig configures error handling behavior
type ErrorManagerConfig struct {
	CorrelationTTL         time.Duration `json:"correlation_ttl"`
	MaxCorrelations        int           `json:"max_correlations"`
	AuditRetention         time.Duration `json:"audit_retention"`
	EnableRecovery         bool          `json:"enable_recovery"`
	EnableDegradation      bool          `json:"enable_degradation"`
	EnableCircuitBreaker   bool          `json:"enable_circuit_breaker"`
	DetailedErrorMessages  bool          `json:"detailed_error_messages"`
}

// MCPError represents a comprehensive MCP error with JSON-RPC 2.0 compliance
type MCPError struct {
	Code         int                    `json:"code"`
	Message      string                 `json:"message"`
	Data         map[string]interface{} `json:"data,omitempty"`
	CorrelationID string                `json:"correlation_id,omitempty"`
	Timestamp    time.Time              `json:"timestamp"`
	Source       string                 `json:"source,omitempty"`
	Category     string                 `json:"category,omitempty"`
	Severity     string                 `json:"severity,omitempty"`
	Recoverable  bool                   `json:"recoverable,omitempty"`
	RetryAfter   *time.Duration         `json:"retry_after,omitempty"`
	Suggestions  []string               `json:"suggestions,omitempty"`
	Context      map[string]interface{} `json:"context,omitempty"`
}

// Error implements the error interface
func (e *MCPError) Error() string {
	if e.CorrelationID != "" {
		return fmt.Sprintf("[%s] Code %d: %s", e.CorrelationID, e.Code, e.Message)
	}
	return fmt.Sprintf("Code %d: %s", e.Code, e.Message)
}

// ErrorCorrelation tracks related errors and their resolution attempts
type ErrorCorrelation struct {
	ID                 string                 `json:"id"`
	ServiceName        string                 `json:"service_name"`
	RequestID          string                 `json:"request_id"`
	OriginalError      *MCPError              `json:"original_error"`
	RelatedErrors      []*MCPError            `json:"related_errors"`
	ResolutionAttempts []ResolutionAttempt    `json:"resolution_attempts"`
	Status             string                 `json:"status"`
	CreatedAt          time.Time              `json:"created_at"`
	UpdatedAt          time.Time              `json:"updated_at"`
	Context            map[string]interface{} `json:"context,omitempty"`
}

// ResolutionAttempt represents an attempt to resolve an error
type ResolutionAttempt struct {
	AttemptID    string                 `json:"attempt_id"`
	Strategy     string                 `json:"strategy"`
	Timestamp    time.Time              `json:"timestamp"`
	Success      bool                   `json:"success"`
	Details      string                 `json:"details,omitempty"`
	NextAction   string                 `json:"next_action,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}


// Standard JSON-RPC 2.0 error codes
const (
	// JSON-RPC 2.0 standard error codes
	ErrorParseError     = -32700
	ErrorInvalidRequest = -32600
	ErrorMethodNotFound = -32601
	ErrorInvalidParams  = -32602
	ErrorInternalError  = -32603
	
	// MCP-specific error codes (application layer)
	ErrorToolNotFound        = -32001
	ErrorToolExecutionFailed = -32002
	ErrorResourceNotFound    = -32003
	ErrorResourceAccessDenied = -32004
	ErrorPromptNotFound      = -32005
	ErrorPromptRenderFailed  = -32006
	ErrorValidationFailed    = -32007
	ErrorTimeout             = -32008
	ErrorRateLimited         = -32009
	ErrorServiceUnavailable  = -32010
	ErrorExternalAPIFailed   = -32011
	ErrorDatabaseError       = -32012
	ErrorAuthenticationFailed = -32013
	ErrorAuthorizationFailed = -32014
	ErrorQuotaExceeded       = -32015
)

// Error severity levels
const (
	SeverityCritical = "critical"
	SeverityHigh     = "high"
	SeverityMedium   = "medium"
	SeverityLow      = "low"
	SeverityInfo     = "info"
)

// Error categories
const (
	CategorySystem      = "system"
	CategoryValidation  = "validation"
	CategoryAuth        = "authentication"
	CategoryResource    = "resource"
	CategoryExternal    = "external"
	CategoryNetwork     = "network"
	CategoryDatabase    = "database"
	CategoryBusiness    = "business"
)

// Circuit breaker states
const (
	CircuitBreakerClosed     = "closed"
	CircuitBreakerOpen       = "open"
	CircuitBreakerHalfOpen   = "half-open"
)

// NewErrorManager creates a new error manager
func NewErrorManager(logger *logrus.Logger, config ErrorManagerConfig) *ErrorManager {
	// Set defaults
	if config.CorrelationTTL == 0 {
		config.CorrelationTTL = time.Hour
	}
	if config.MaxCorrelations == 0 {
		config.MaxCorrelations = 1000
	}
	if config.AuditRetention == 0 {
		config.AuditRetention = 24 * time.Hour
	}
	
	manager := &ErrorManager{
		logger:       logger,
		correlations: make(map[string]*ErrorCorrelation),
		config:       config,
	}
	
	// Initialize components based on configuration
	if config.EnableCircuitBreaker {
		cbConfig := CircuitBreakerConfig{
			DefaultThreshold:       5,
			DefaultTimeout:         60 * time.Second,
			DefaultHalfOpenLimit:   3,
			MonitoringInterval:     30 * time.Second,
			MetricsRetentionPeriod: 24 * time.Hour,
		}
		manager.circuitManager = NewCircuitBreakerManager(cbConfig)
	}
	
	// Initialize audit trail
	auditConfig := AuditConfig{
		RetentionPeriod:      config.AuditRetention,
		MaxEntriesPerChain:   100,
		MaxCorrelationChains: config.MaxCorrelations,
		EnableMetrics:        true,
		CleanupInterval:      time.Hour,
	}
	manager.auditTrail = NewErrorAuditTrail(logger, auditConfig)
	
	// Initialize tool error handler
	manager.toolHandler = NewToolErrorHandler(logger)
	
	// Initialize degradation manager if enabled
	if config.EnableDegradation {
		manager.degradationMgr = NewGracefulDegradationManager(logger, manager.circuitManager)
	}
	
	// Initialize recovery manager if enabled
	if config.EnableRecovery {
		manager.recoveryManager = NewRecoveryGuidanceManager(logger)
	}
	
	return manager
}

// generateCorrelationID generates a unique correlation ID
func generateCorrelationID() string {
	return uuid.New().String()
}

// HandleError handles an error and returns an MCP-compliant error
func (em *ErrorManager) HandleError(ctx context.Context, err error, context map[string]interface{}) *MCPError {
	if err == nil {
		return em.CreateError(ctx, ErrorInternalError, "Nil error encountered")
	}
	
	// If it's already an MCP error, enhance it with correlation
	if mcpErr, ok := err.(*MCPError); ok {
		if mcpErr.CorrelationID == "" {
			mcpErr.CorrelationID = generateCorrelationID()
		}
		
		// Add context information
		if context != nil {
			if mcpErr.Context == nil {
				mcpErr.Context = make(map[string]interface{})
			}
			for k, v := range context {
				mcpErr.Context[k] = v
			}
		}
		
		// Ensure suggestions are populated
		if len(mcpErr.Suggestions) == 0 {
			mcpErr.Suggestions = em.generateSuggestions(mcpErr.Code)
		}
		
		// Update recovery status if not set
		mcpErr.Recoverable = em.isRecoverableError(mcpErr.Code)
		
		// Update severity and category if not set
		if mcpErr.Severity == "" {
			mcpErr.Severity = em.inferSeverity(mcpErr.Code)
		}
		if mcpErr.Category == "" {
			mcpErr.Category = em.inferCategory(mcpErr.Code)
		}
		
		em.updateErrorCorrelation(mcpErr)
		em.addToAuditTrail(mcpErr, ctx)
		return mcpErr
	}
	
	// Convert to MCP error
	options := []ErrorOption{
		WithSource("error_manager"),
	}
	
	if context != nil {
		for k, v := range context {
			options = append(options, WithContext(k, v))
		}
	}
	
	return em.CreateError(ctx, ErrorInternalError, err.Error(), options...)
}

// GetActiveCorrelations returns active error correlations
func (em *ErrorManager) GetActiveCorrelations() []*ErrorCorrelation {
	em.mutex.RLock()
	defer em.mutex.RUnlock()
	
	result := make([]*ErrorCorrelation, 0, len(em.correlations))
	for _, corr := range em.correlations {
		result = append(result, corr)
	}
	
	return result
}

// generateSuggestions generates default suggestions based on error code
func (em *ErrorManager) generateSuggestions(code int) []string {
	switch code {
	case ErrorServiceUnavailable:
		return []string{
			"Retry the request after a brief delay",
			"Check service health and connectivity", 
			"Consider using cached data if available",
		}
	case ErrorInvalidParams:
		return []string{
			"Check parameter names and types",
			"Refer to API documentation",
			"Validate input data before calling",
		}
	case ErrorTimeout:
		return []string{
			"Increase timeout duration",
			"Retry with exponential backoff",
			"Check network connectivity",
		}
	case ErrorInternalError:
		return []string{
			"Contact support if the problem persists",
			"Check system logs for more details",
			"Retry the operation",
		}
	default:
		return []string{
			"Refer to error documentation",
			"Contact support if needed",
		}
	}
}

// CreateError creates a new MCP error with context
func (em *ErrorManager) CreateError(ctx context.Context, code int, message string, options ...ErrorOption) *MCPError {
	correlationID := em.getOrCreateCorrelationID(ctx)
	
	mcpError := &MCPError{
		Code:          code,
		Message:       message,
		CorrelationID: correlationID,
		Timestamp:     time.Now(),
		Data:          make(map[string]interface{}),
		Context:       make(map[string]interface{}),
		Recoverable:   em.isRecoverableError(code),
		Severity:      em.inferSeverity(code),
		Category:      em.inferCategory(code),
		Suggestions:   em.generateSuggestions(code),
	}
	
	// Apply options
	for _, option := range options {
		option(mcpError)
	}
	
	// Add stack trace for internal errors if detailed messages enabled
	if em.config.DetailedErrorMessages && code == ErrorInternalError {
		mcpError.Context["stack_trace"] = em.getStackTrace()
	}
	
	// Log the error
	em.logError(mcpError)
	
	// Add to audit trail
	em.addToAuditTrail(mcpError, ctx)
	
	// Update error correlation
	em.updateErrorCorrelation(mcpError)
	
	return mcpError
}

// ErrorOption defines functional options for error creation
type ErrorOption func(*MCPError)

// WithData adds data to the error
func WithData(key string, value interface{}) ErrorOption {
	return func(e *MCPError) {
		if e.Data == nil {
			e.Data = make(map[string]interface{})
		}
		e.Data[key] = value
	}
}

// WithContext adds context to the error
func WithContext(key string, value interface{}) ErrorOption {
	return func(e *MCPError) {
		if e.Context == nil {
			e.Context = make(map[string]interface{})
		}
		e.Context[key] = value
	}
}

// WithSource sets the error source
func WithSource(source string) ErrorOption {
	return func(e *MCPError) {
		e.Source = source
	}
}

// WithSuggestions adds recovery suggestions
func WithSuggestions(suggestions ...string) ErrorOption {
	return func(e *MCPError) {
		e.Suggestions = suggestions
	}
}

// WithRetryAfter sets retry timing
func WithRetryAfter(duration time.Duration) ErrorOption {
	return func(e *MCPError) {
		e.RetryAfter = &duration
	}
}

// WithSeverity overrides inferred severity
func WithSeverity(severity string) ErrorOption {
	return func(e *MCPError) {
		e.Severity = severity
	}
}

// WithCategory overrides inferred category
func WithCategory(category string) ErrorOption {
	return func(e *MCPError) {
		e.Category = category
	}
}

// getOrCreateCorrelationID gets or creates a correlation ID from context
func (em *ErrorManager) getOrCreateCorrelationID(ctx context.Context) string {
	if ctx != nil {
		if id, ok := ctx.Value("correlation_id").(string); ok && id != "" {
			return id
		}
	}
	return uuid.New().String()
}

// isRecoverableError determines if an error is recoverable
func (em *ErrorManager) isRecoverableError(code int) bool {
	recoverableCodes := map[int]bool{
		ErrorTimeout:           true,
		ErrorRateLimited:      true,
		ErrorServiceUnavailable: true,
		ErrorExternalAPIFailed: true,
		ErrorDatabaseError:    true,
		ErrorInvalidParams:    true,
		ErrorValidationFailed: true,
	}
	
	return recoverableCodes[code]
}

// inferSeverity infers error severity from code
func (em *ErrorManager) inferSeverity(code int) string {
	switch {
	case code == ErrorInternalError:
		return SeverityCritical
	case code >= ErrorAuthenticationFailed && code <= ErrorAuthorizationFailed:
		return SeverityHigh
	case code >= ErrorToolNotFound && code <= ErrorPromptRenderFailed:
		return SeverityMedium
	case code >= ErrorValidationFailed && code <= ErrorRateLimited:
		return SeverityLow
	case code >= ErrorParseError && code <= ErrorInvalidParams:
		return SeverityMedium
	default:
		return SeverityMedium
	}
}

// inferCategory infers error category from code
func (em *ErrorManager) inferCategory(code int) string {
	switch {
	case code >= ErrorParseError && code <= ErrorInternalError:
		return CategorySystem
	case code >= ErrorAuthenticationFailed && code <= ErrorAuthorizationFailed:
		return CategoryAuth
	case code >= ErrorToolNotFound && code <= ErrorResourceAccessDenied:
		return CategoryResource
	case code >= ErrorPromptNotFound && code <= ErrorPromptRenderFailed:
		return CategoryResource
	case code == ErrorValidationFailed:
		return CategoryValidation
	case code >= ErrorTimeout && code <= ErrorServiceUnavailable:
		return CategorySystem
	case code == ErrorExternalAPIFailed:
		return CategoryExternal
	case code == ErrorDatabaseError:
		return CategoryDatabase
	default:
		return CategorySystem
	}
}

// getStackTrace captures current stack trace
func (em *ErrorManager) getStackTrace() []string {
	trace := make([]string, 0)
	for i := 1; ; i++ {
		pc, file, line, ok := runtime.Caller(i)
		if !ok {
			break
		}
		fn := runtime.FuncForPC(pc)
		if fn == nil {
			continue
		}
		trace = append(trace, fmt.Sprintf("%s:%d %s", file, line, fn.Name()))
		if len(trace) >= 10 { // Limit stack trace depth
			break
		}
	}
	return trace
}

// logError logs the error with appropriate level
func (em *ErrorManager) logError(mcpError *MCPError) {
	fields := logrus.Fields{
		"error_code":      mcpError.Code,
		"correlation_id":  mcpError.CorrelationID,
		"source":          mcpError.Source,
		"category":        mcpError.Category,
		"severity":        mcpError.Severity,
		"recoverable":     mcpError.Recoverable,
	}
	
	// Add data fields
	for key, value := range mcpError.Data {
		fields[fmt.Sprintf("data_%s", key)] = value
	}
	
	entry := em.logger.WithFields(fields)
	
	switch mcpError.Severity {
	case SeverityCritical:
		entry.Error(mcpError.Message)
	case SeverityHigh:
		entry.Error(mcpError.Message)
	case SeverityMedium:
		entry.Warn(mcpError.Message)
	case SeverityLow:
		entry.Info(mcpError.Message)
	case SeverityInfo:
		entry.Debug(mcpError.Message)
	default:
		entry.Warn(mcpError.Message)
	}
}

// addToAuditTrail adds error to audit trail
func (em *ErrorManager) addToAuditTrail(mcpError *MCPError, ctx context.Context) {
	if em.auditTrail == nil {
		return
	}
	
	// Create error event context
	errorCtx := ErrorEventContext{
		RequestParams: make(map[string]interface{}),
		Environment: EnvironmentInfo{
			Environment: "production",
		},
		Timing: TimingInfo{
			RequestStartTime: time.Now(),
			ErrorTime:        mcpError.Timestamp,
		},
	}
	
	// Extract context information
	if ctx != nil {
		if userID, ok := ctx.Value("user_id").(string); ok {
			errorCtx.RequestParams["user_id"] = userID
		}
		if requestID, ok := ctx.Value("request_id").(string); ok {
			errorCtx.RequestParams["request_id"] = requestID
		} else {
			errorCtx.RequestParams["request_id"] = "unknown"
		}
	} else {
		errorCtx.RequestParams["request_id"] = "unknown"
	}
	
	// Add error data to context
	if mcpError.Data != nil {
		errorCtx.RequestParams["error_data"] = mcpError.Data
	}
	if mcpError.Context != nil {
		errorCtx.SystemState = mcpError.Context
	}
	
	// Log to audit trail (ignore errors)
	em.auditTrail.LogError(ctx, mcpError.CorrelationID, mcpError, errorCtx)
}

// updateErrorCorrelation updates error correlation tracking
func (em *ErrorManager) updateErrorCorrelation(mcpError *MCPError) {
	em.mutex.Lock()
	defer em.mutex.Unlock()
	
	correlationID := mcpError.CorrelationID
	if correlationID == "" {
		return
	}
	
	now := time.Now()
	
	if correlation, exists := em.correlations[correlationID]; exists {
		// Add to existing correlation
		correlation.RelatedErrors = append(correlation.RelatedErrors, mcpError)
		correlation.UpdatedAt = now
	} else {
		// Extract service and request info from error context
		serviceName := "unknown"
		requestID := "unknown"
		if mcpError.Context != nil {
			if svc, ok := mcpError.Context["service"].(string); ok {
				serviceName = svc
			}
			if req, ok := mcpError.Context["request_id"].(string); ok {
				requestID = req
			}
		}
		
		// Create new correlation
		em.correlations[correlationID] = &ErrorCorrelation{
			ID:                 correlationID,
			ServiceName:        serviceName,
			RequestID:          requestID,
			OriginalError:      mcpError,
			RelatedErrors:      []*MCPError{},
			ResolutionAttempts: []ResolutionAttempt{},
			Status:             "active",
			CreatedAt:          now,
			UpdatedAt:          now,
			Context:            make(map[string]interface{}),
		}
	}
}

// GetErrorCorrelation retrieves error correlation by ID
func (em *ErrorManager) GetErrorCorrelation(correlationID string) (*ErrorCorrelation, bool) {
	em.mutex.RLock()
	defer em.mutex.RUnlock()
	
	correlation, exists := em.correlations[correlationID]
	return correlation, exists
}

// AddResolutionAttempt adds a resolution attempt to error correlation
func (em *ErrorManager) AddResolutionAttempt(correlationID, strategy, details string, success bool) error {
	em.mutex.Lock()
	defer em.mutex.Unlock()
	
	correlation, exists := em.correlations[correlationID]
	if !exists {
		return fmt.Errorf("correlation %s not found", correlationID)
	}
	
	attempt := ResolutionAttempt{
		AttemptID: uuid.New().String(),
		Strategy:  strategy,
		Timestamp: time.Now(),
		Success:   success,
		Details:   details,
		Metadata:  make(map[string]interface{}),
	}
	
	if success {
		correlation.Status = "resolved"
		attempt.NextAction = "completed"
	} else {
		attempt.NextAction = "retry_or_escalate"
	}
	
	correlation.ResolutionAttempts = append(correlation.ResolutionAttempts, attempt)
	correlation.UpdatedAt = time.Now()
	
	return nil
}

// CleanupExpiredCorrelations removes expired error correlations
func (em *ErrorManager) CleanupExpiredCorrelations() int {
	em.mutex.Lock()
	defer em.mutex.Unlock()
	
	cutoff := time.Now().Add(-em.config.CorrelationTTL)
	removed := 0
	
	for id, correlation := range em.correlations {
		if correlation.UpdatedAt.Before(cutoff) {
			delete(em.correlations, id)
			removed++
		}
	}
	
	if removed > 0 {
		em.logger.WithField("removed_correlations", removed).Info("Cleaned up expired error correlations")
	}
	
	return removed
}

// GetAuditTrail returns recent audit trail entries
func (em *ErrorManager) GetAuditTrail(limit int) []*AuditEntry {
	if em.auditTrail == nil {
		return []*AuditEntry{}
	}
	
	criteria := SearchCriteria{
		Limit: limit,
	}
	
	entries, err := em.auditTrail.SearchAuditEntries(context.Background(), criteria)
	if err != nil {
		em.logger.WithError(err).Warn("Failed to retrieve audit trail entries")
		return []*AuditEntry{}
	}
	
	return entries
}

// GetErrorStats returns error statistics
func (em *ErrorManager) GetErrorStats() map[string]interface{} {
	em.mutex.RLock()
	defer em.mutex.RUnlock()
	
	stats := map[string]interface{}{
		"total_correlations":    len(em.correlations),
		"correlation_ttl_hours": em.config.CorrelationTTL.Hours(),
	}
	
	// Add audit trail stats if available
	if em.auditTrail != nil {
		auditStats := em.auditTrail.GetAuditStats()
		for k, v := range auditStats {
			stats["audit_"+k] = v
		}
	}
	
	// Add circuit breaker stats if available
	if em.circuitManager != nil {
		cbStats := em.circuitManager.GetCircuitBreakerStatus()
		for k, v := range cbStats {
			stats["circuit_breaker_"+k] = v
		}
	}
	
	return stats
}

// ToJSONRPCError converts MCPError to JSON-RPC 2.0 error format
func (em *ErrorManager) ToJSONRPCError(mcpError *MCPError) map[string]interface{} {
	jsonRPCError := map[string]interface{}{
		"code":    mcpError.Code,
		"message": mcpError.Message,
	}
	
	// Add data if present and detailed errors enabled
	if em.config.DetailedErrorMessages && len(mcpError.Data) > 0 {
		jsonRPCError["data"] = mcpError.Data
	}
	
	return jsonRPCError
}

// StartCleanupRoutine starts background cleanup of expired correlations
func (em *ErrorManager) StartCleanupRoutine(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				em.CleanupExpiredCorrelations()
			}
		}
	}()
}