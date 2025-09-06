package errors

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestErrorManager_HandleError(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	
	config := ErrorManagerConfig{
		CorrelationTTL:    time.Hour,
		MaxCorrelations:   1000,
		AuditRetention:    24 * time.Hour,
		EnableRecovery:    true,
		EnableDegradation: true,
	}
	
	manager := NewErrorManager(logger, config)

	tests := []struct {
		name        string
		err         error
		context     map[string]interface{}
		expectType  string
		expectCode  int
	}{
		{
			name: "Standard error",
			err:  fmt.Errorf("test error"),
			context: map[string]interface{}{
				"service": "test_service",
				"operation": "test_operation",
			},
			expectType: "*errors.MCPError",
			expectCode: ErrorInternalError,
		},
		{
			name: "MCP error",
			err: &MCPError{
				Code:    ErrorInvalidParams,
				Message: "Invalid parameters",
			},
			context: map[string]interface{}{
				"service": "test_service",
			},
			expectType: "*errors.MCPError",
			expectCode: ErrorInvalidParams,
		},
		{
			name: "Nil error",
			err:  nil,
			context: map[string]interface{}{
				"service": "test_service",
			},
			expectType: "*errors.MCPError",
			expectCode: ErrorInternalError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := manager.HandleError(context.Background(), tt.err, tt.context)
			
			assert.NotNil(t, result)
			assert.Equal(t, tt.expectCode, result.Code)
			assert.NotEmpty(t, result.CorrelationID)
			
			// Verify correlation was created
			correlations := manager.GetActiveCorrelations()
			found := false
			for _, corr := range correlations {
				if corr.ID == result.CorrelationID {
					found = true
					break
				}
			}
			assert.True(t, found, "Correlation should be created")
		})
	}
}

func TestCircuitBreakerManager_Operations(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	
	config := CircuitBreakerConfig{
		DefaultThreshold:     3,
		DefaultTimeout:       5 * time.Second,
		DefaultHalfOpenLimit: 2,
	}
	
	manager := NewCircuitBreakerManager(config)

	t.Run("Create and retrieve circuit breaker", func(t *testing.T) {
		breaker := manager.GetOrCreateCircuitBreaker("test_service")
		
		assert.NotNil(t, breaker)
		assert.Equal(t, "test_service", breaker.Name)
		assert.Equal(t, CircuitBreakerClosed, breaker.State)
		assert.Equal(t, 3, breaker.Threshold)
		
		// Should return same instance
		breaker2 := manager.GetOrCreateCircuitBreaker("test_service")
		assert.Equal(t, breaker, breaker2)
	})

	t.Run("Circuit breaker state transitions", func(t *testing.T) {
		breaker := manager.GetOrCreateCircuitBreaker("state_test_service")
		
		// Initially closed and allows execution
		result := breaker.CanExecute()
		assert.True(t, result.Allowed)
		assert.Equal(t, CircuitBreakerClosed, result.State)
		
		// Simulate failures to trigger open state
		for i := 0; i < 3; i++ {
			breaker.recordResult(false, time.Millisecond)
		}
		
		assert.Equal(t, CircuitBreakerOpen, breaker.State)
		
		result = breaker.CanExecute()
		assert.False(t, result.Allowed)
		assert.Equal(t, CircuitBreakerOpen, result.State)
	})

	t.Run("Circuit breaker call with operation", func(t *testing.T) {
		breaker := manager.GetOrCreateCircuitBreaker("operation_test")
		
		// Successful operation
		err := breaker.Call(context.Background(), func(ctx context.Context) error {
			return nil
		})
		assert.NoError(t, err)
		
		// Failed operation
		err = breaker.Call(context.Background(), func(ctx context.Context) error {
			return fmt.Errorf("operation failed")
		})
		assert.Error(t, err)
		
		// Should not be circuit breaker error yet (threshold not reached)
		mcpErr, ok := err.(*MCPError)
		assert.False(t, ok || (ok && mcpErr.Code == ErrorServiceUnavailable))
	})

	t.Run("Get metrics", func(t *testing.T) {
		breaker := manager.GetOrCreateCircuitBreaker("metrics_test")
		
		// Execute some operations
		breaker.Call(context.Background(), func(ctx context.Context) error { return nil })
		breaker.Call(context.Background(), func(ctx context.Context) error { return fmt.Errorf("error") })
		
		metrics := breaker.GetMetrics()
		assert.Equal(t, "metrics_test", metrics.Name)
		assert.True(t, metrics.TotalRequests > 0)
	})
}

func TestToolErrorHandler_Validation(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	
	handler := NewToolErrorHandler(logger)

	t.Run("Valid tool call", func(t *testing.T) {
		params := map[string]interface{}{
			"variant": "NM_000314.6:c.1A>G",
			"gene":    "PTEN",
			"classification_level": "pathogenic",
		}
		
		err := handler.ValidateToolCall("classify_variant", params)
		assert.NoError(t, err)
	})

	t.Run("Missing required parameter", func(t *testing.T) {
		params := map[string]interface{}{
			"gene": "PTEN",
		}
		
		err := handler.ValidateToolCall("classify_variant", params)
		assert.Error(t, err)
		
		toolErr, ok := err.(*ToolError)
		assert.True(t, ok)
		assert.Equal(t, "classify_variant", toolErr.ToolName)
		assert.True(t, len(toolErr.ValidationErrors) > 0)
		
		// Should have error for missing 'variant' parameter
		hasVariantError := false
		for _, valErr := range toolErr.ValidationErrors {
			if valErr.Parameter == "variant" && valErr.Rule == "required" {
				hasVariantError = true
				break
			}
		}
		assert.True(t, hasVariantError)
	})

	t.Run("Invalid parameter type", func(t *testing.T) {
		params := map[string]interface{}{
			"variant": 123, // Should be string
			"gene":    "PTEN",
		}
		
		err := handler.ValidateToolCall("classify_variant", params)
		assert.Error(t, err)
		
		toolErr, ok := err.(*ToolError)
		assert.True(t, ok)
		assert.True(t, len(toolErr.ValidationErrors) > 0)
	})

	t.Run("Invalid enum value", func(t *testing.T) {
		params := map[string]interface{}{
			"variant": "NM_000314.6:c.1A>G",
			"gene":    "PTEN",
			"classification_level": "invalid_level",
		}
		
		err := handler.ValidateToolCall("classify_variant", params)
		assert.Error(t, err)
		
		toolErr, ok := err.(*ToolError)
		assert.True(t, ok)
		
		// Should have enum validation error
		hasEnumError := false
		for _, valErr := range toolErr.ValidationErrors {
			if valErr.Parameter == "classification_level" && valErr.Rule == "enum" {
				hasEnumError = true
				break
			}
		}
		assert.True(t, hasEnumError)
	})

	t.Run("Unknown tool", func(t *testing.T) {
		params := map[string]interface{}{
			"param1": "value1",
		}
		
		err := handler.ValidateToolCall("unknown_tool", params)
		assert.Error(t, err)
		
		toolErr, ok := err.(*ToolError)
		assert.True(t, ok)
		assert.Equal(t, ErrorToolNotFound, toolErr.Code)
	})
}

func TestGracefulDegradationManager_ServiceFailure(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	
	cbManager := NewCircuitBreakerManager(CircuitBreakerConfig{})
	manager := NewGracefulDegradationManager(logger, cbManager)

	t.Run("Handle service failure with fallback", func(t *testing.T) {
		originalError := fmt.Errorf("service unavailable")
		
		result, err := manager.HandleServiceFailure(context.Background(), "variant_classification", originalError)
		
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Success)
		assert.NotEmpty(t, result.Source)
		assert.NotEmpty(t, result.Quality)
		assert.True(t, result.ExecutionTime > 0)
	})

	t.Run("Service status tracking", func(t *testing.T) {
		status := manager.GetServiceStatus()
		
		assert.NotNil(t, status)
		
		// Should have default services
		variantStatus, exists := status["variant_classification"]
		assert.True(t, exists)
		
		variantStatusMap, ok := variantStatus.(map[string]interface{})
		assert.True(t, ok)
		assert.Equal(t, 1, variantStatusMap["priority"])
		assert.Equal(t, true, variantStatusMap["fallback_enabled"])
	})

	t.Run("Unknown service failure", func(t *testing.T) {
		originalError := fmt.Errorf("service error")
		
		result, err := manager.HandleServiceFailure(context.Background(), "unknown_service", originalError)
		
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestRecoveryGuidanceManager_RecoveryPlan(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	
	manager := NewRecoveryGuidanceManager(logger)

	t.Run("Generate recovery plan for timeout error", func(t *testing.T) {
		errorCtx := &ErrorContext{
			Error: &MCPError{
				Code:    ErrorServiceUnavailable,
				Message: "Connection timeout",
			},
			ServiceName:   "test_service",
			OperationName: "test_operation",
			RequestID:     "req_123",
			Timestamp:     time.Now(),
		}
		
		plan, err := manager.GenerateRecoveryPlan(context.Background(), errorCtx)
		
		assert.NoError(t, err)
		assert.NotNil(t, plan)
		assert.Equal(t, errorCtx, plan.ErrorContext)
		assert.True(t, len(plan.RecommendedActions) > 0)
		assert.True(t, plan.EstimatedTime > 0)
		assert.True(t, plan.SuccessRate > 0)
		
		// Should have retry action for timeout
		hasRetryAction := false
		for _, action := range plan.RecommendedActions {
			if action.Action.Type == "retry" {
				hasRetryAction = true
				break
			}
		}
		assert.True(t, hasRetryAction)
	})

	t.Run("Generate recovery plan for validation error", func(t *testing.T) {
		errorCtx := &ErrorContext{
			Error: &MCPError{
				Code:    ErrorInvalidParams,
				Message: "Invalid parameters provided",
			},
			ServiceName:   "test_service",
			OperationName: "validate_input",
			RequestID:     "req_456",
			Timestamp:     time.Now(),
		}
		
		plan, err := manager.GenerateRecoveryPlan(context.Background(), errorCtx)
		
		assert.NoError(t, err)
		assert.NotNil(t, plan)
		assert.True(t, len(plan.RecommendedActions) > 0)
		
		// Should have manual fix action for validation errors
		hasManualAction := false
		for _, action := range plan.RecommendedActions {
			if action.Action.Type == "manual" {
				hasManualAction = true
				break
			}
		}
		assert.True(t, hasManualAction)
	})
}

func TestErrorAuditTrail_Logging(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	
	config := AuditConfig{
		RetentionPeriod:    time.Hour,
		MaxEntriesPerChain: 10,
		CleanupInterval:    time.Minute,
	}
	
	audit := NewErrorAuditTrail(logger, config)
	defer audit.Stop()

	t.Run("Log error to audit trail", func(t *testing.T) {
		correlationID := "test_correlation_123"
		testError := fmt.Errorf("test audit error")
		
		errorCtx := ErrorEventContext{
			RequestParams: map[string]interface{}{
				"request_id": "req_789",
			},
			Environment: EnvironmentInfo{
				ServiceVersion: "test_service_v1",
				Environment:    "test",
			},
			Timing: TimingInfo{
				RequestStartTime: time.Now(),
				ErrorTime:        time.Now(),
			},
		}
		
		entry, err := audit.LogError(context.Background(), correlationID, testError, errorCtx)
		
		assert.NoError(t, err)
		assert.NotNil(t, entry)
		assert.Equal(t, correlationID, entry.CorrelationID)
		assert.Equal(t, "test audit error", entry.ErrorMessage)
		assert.Equal(t, ErrorInternalError, entry.ErrorCode)
		assert.NotEmpty(t, entry.ID)
	})

	t.Run("Get correlation chain", func(t *testing.T) {
		correlationID := "test_correlation_456"
		testError := fmt.Errorf("chain test error")
		
		errorCtx := ErrorEventContext{
			RequestParams: map[string]interface{}{
				"request_id": "req_chain_test",
			},
			Environment: EnvironmentInfo{
				ServiceVersion: "chain_test_service",
			},
		}
		
		// Log multiple errors with same correlation ID
		for i := 0; i < 3; i++ {
			_, err := audit.LogError(context.Background(), correlationID, testError, errorCtx)
			assert.NoError(t, err)
		}
		
		chain, err := audit.GetCorrelationChain(correlationID)
		assert.NoError(t, err)
		assert.NotNil(t, chain)
		assert.Equal(t, correlationID, chain.ID)
		assert.Equal(t, 3, len(chain.Entries))
		assert.Equal(t, "active", chain.Status)
	})

	t.Run("Search audit entries", func(t *testing.T) {
		// Log some test entries
		correlationID := "search_test_correlation"
		serviceName := "search_test_service"
		
		errorCtx := ErrorEventContext{
			RequestParams: map[string]interface{}{
				"request_id": "search_req_123",
			},
			Environment: EnvironmentInfo{
				ServiceVersion: serviceName,
			},
		}
		
		_, err := audit.LogError(context.Background(), correlationID, fmt.Errorf("search test error"), errorCtx)
		assert.NoError(t, err)
		
		// Search for entries
		criteria := SearchCriteria{
			ServiceName: serviceName,
			Limit:       10,
		}
		
		entries, err := audit.SearchAuditEntries(context.Background(), criteria)
		assert.NoError(t, err)
		assert.True(t, len(entries) > 0)
		
		// Verify we found the right entry
		found := false
		for _, entry := range entries {
			if entry.CorrelationID == correlationID {
				found = true
				break
			}
		}
		assert.True(t, found)
	})

	t.Run("Mark error as resolved", func(t *testing.T) {
		correlationID := "resolve_test_correlation"
		testError := fmt.Errorf("resolve test error")
		
		errorCtx := ErrorEventContext{
			RequestParams: map[string]interface{}{
				"request_id": "resolve_req_123",
			},
			Environment: EnvironmentInfo{
				ServiceVersion: "resolve_test_service",
			},
		}
		
		entry, err := audit.LogError(context.Background(), correlationID, testError, errorCtx)
		assert.NoError(t, err)
		
		// Mark as resolved
		err = audit.MarkResolved(entry.ID, "retry", "auto", "Issue resolved automatically")
		assert.NoError(t, err)
		
		// Verify resolution was recorded
		criteria := SearchCriteria{
			CorrelationID: correlationID,
		}
		
		entries, err := audit.SearchAuditEntries(context.Background(), criteria)
		assert.NoError(t, err)
		assert.True(t, len(entries) > 0)
		assert.NotNil(t, entries[0].Resolution)
		assert.True(t, entries[0].Resolution.Success)
		assert.Equal(t, "auto", entries[0].Resolution.ResolvedBy)
	})

	t.Run("Get audit statistics", func(t *testing.T) {
		stats := audit.GetAuditStats()
		
		assert.NotNil(t, stats)
		assert.Contains(t, stats, "total_entries")
		assert.Contains(t, stats, "total_correlations")
		assert.Contains(t, stats, "by_severity")
		assert.Contains(t, stats, "by_category")
		assert.Contains(t, stats, "resolution_rate")
		
		totalEntries, ok := stats["total_entries"].(int)
		assert.True(t, ok)
		assert.True(t, totalEntries > 0)
	})
}

func TestIntegrationErrorHandling(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	
	// Create integrated error handling system
	config := ErrorManagerConfig{
		CorrelationTTL:      time.Hour,
		MaxCorrelations:     1000,
		AuditRetention:      24 * time.Hour,
		EnableRecovery:      true,
		EnableDegradation:   true,
		EnableCircuitBreaker: true,
	}
	
	manager := NewErrorManager(logger, config)

	t.Run("End-to-end error handling flow", func(t *testing.T) {
		// Simulate a service failure
		originalError := &MCPError{
			Code:    ErrorServiceUnavailable,
			Message: "Database connection timeout",
			Data: map[string]interface{}{
				"database": "variant_db",
				"host":     "localhost:5432",
			},
		}
		
		ctxData := map[string]interface{}{
			"service":    "variant_classification",
			"operation":  "classify_variant",
			"request_id": "integration_test_123",
			"user_id":    "user_789",
		}
		
		// Handle the error
		result := manager.HandleError(context.Background(), originalError, ctxData)
		
		// Verify error was processed correctly
		assert.NotNil(t, result)
		assert.Equal(t, ErrorServiceUnavailable, result.Code)
		assert.NotEmpty(t, result.CorrelationID)
		assert.NotEmpty(t, result.Suggestions)
		assert.True(t, result.Recoverable)
		
		// Verify correlation was created
		correlations := manager.GetActiveCorrelations()
		assert.True(t, len(correlations) > 0)
		
		foundCorrelation := false
		for _, corr := range correlations {
			if corr.ID == result.CorrelationID {
				foundCorrelation = true
				assert.Equal(t, "variant_classification", corr.ServiceName)
				assert.Equal(t, "integration_test_123", corr.RequestID)
				break
			}
		}
		assert.True(t, foundCorrelation)
		
		// Test recovery plan generation
		if manager.recoveryManager != nil {
			errorCtx := &ErrorContext{
				Error:         result,
				ServiceName:   ctxData["service"].(string),
				OperationName: ctxData["operation"].(string),
				RequestID:     ctxData["request_id"].(string),
				UserID:        ctxData["user_id"].(string),
				Timestamp:     time.Now(),
			}
			
			plan, err := manager.recoveryManager.GenerateRecoveryPlan(context.Background(), errorCtx)
			assert.NoError(t, err)
			assert.NotNil(t, plan)
			assert.True(t, len(plan.RecommendedActions) > 0)
		}
	})
}

// Benchmarks for performance testing

func BenchmarkErrorManager_HandleError(b *testing.B) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	
	config := ErrorManagerConfig{
		CorrelationTTL:  time.Hour,
		MaxCorrelations: 10000,
	}
	
	manager := NewErrorManager(logger, config)
	testError := fmt.Errorf("benchmark test error")
	ctxData := map[string]interface{}{
		"service":    "benchmark_service",
		"request_id": "bench_123",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.HandleError(context.Background(), testError, ctxData)
	}
}

func BenchmarkCircuitBreaker_Call(b *testing.B) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	
	config := CircuitBreakerConfig{
		DefaultThreshold: 5,
		DefaultTimeout:   time.Second,
	}
	
	manager := NewCircuitBreakerManager(config)
	breaker := manager.GetOrCreateCircuitBreaker("benchmark_service")

	operation := func(ctx context.Context) error {
		return nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		breaker.Call(context.Background(), operation)
	}
}

func BenchmarkToolErrorHandler_ValidateToolCall(b *testing.B) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	
	handler := NewToolErrorHandler(logger)
	params := map[string]interface{}{
		"variant": "NM_000314.6:c.1A>G",
		"gene":    "PTEN",
		"classification_level": "pathogenic",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		handler.ValidateToolCall("classify_variant", params)
	}
}