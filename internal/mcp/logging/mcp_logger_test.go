package logging

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMCPLogger_NewLogger(t *testing.T) {
	config := MCPLoggingConfig{
		LogLevel:        "info",
		EnableJSON:      true,
		PrivacyMode:     true,
		CorrelationTTL:  time.Hour,
		MaxCorrelations: 1000,
		EnableMetrics:   true,
	}

	logger := NewMCPLogger(config)

	assert.NotNil(t, logger)
	assert.Equal(t, config, logger.config)
	assert.NotNil(t, logger.correlations)
	assert.NotNil(t, logger.metrics)
}

func TestMCPLogger_StartCorrelation(t *testing.T) {
	config := MCPLoggingConfig{
		LogLevel:        "info",
		EnableJSON:      true,
		CorrelationTTL:  time.Hour,
		MaxCorrelations: 1000,
	}

	logger := NewMCPLogger(config)

	clientInfo := ClientInfo{
		ID:        "test-client",
		Type:      "claude",
		Version:   "1.0.0",
		UserAgent: "Claude/1.0.0",
	}

	correlationID := logger.StartCorrelation("session123", clientInfo)

	assert.NotEmpty(t, correlationID)
	assert.Contains(t, correlationID, "session123")

	// Check that correlation was stored
	logger.mutex.RLock()
	correlation, exists := logger.correlations[correlationID]
	logger.mutex.RUnlock()

	assert.True(t, exists)
	assert.Equal(t, correlationID, correlation.ID)
	assert.Equal(t, "session123", correlation.SessionID)
	assert.Equal(t, clientInfo, correlation.ClientInfo)
	assert.Empty(t, correlation.Operations)
}

func TestMCPLogger_LogOperation(t *testing.T) {
	config := MCPLoggingConfig{
		LogLevel:        "info",
		EnableJSON:      true,
		PrivacyMode:     false,
		CorrelationTTL:  time.Hour,
		MaxCorrelations: 1000,
	}

	// Capture logger output
	var buf bytes.Buffer
	logger := NewMCPLogger(config)
	logger.logger.SetOutput(&buf)

	clientInfo := ClientInfo{ID: "test-client", Type: "claude"}
	correlationID := logger.StartCorrelation("session123", clientInfo)

	operation := OperationLog{
		Type:       "tool_call",
		Name:       "classify_variant",
		StartTime:  time.Now(),
		Parameters: map[string]interface{}{"variant": "NM_000492.3:c.1521_1523del"},
		Metadata:   map[string]interface{}{"source": "test"},
	}

	logger.LogOperation(correlationID, operation)

	// Check that operation was added to correlation
	logger.mutex.RLock()
	correlation := logger.correlations[correlationID]
	logger.mutex.RUnlock()

	assert.Len(t, correlation.Operations, 1)
	assert.Equal(t, operation.Type, correlation.Operations[0].Type)
	assert.Equal(t, operation.Name, correlation.Operations[0].Name)

	// Check log output
	logOutput := buf.String()
	assert.Contains(t, logOutput, "MCP operation logged")
	assert.Contains(t, logOutput, "classify_variant")
	assert.Contains(t, logOutput, correlationID)
}

func TestMCPLogger_PrivacyMode(t *testing.T) {
	config := MCPLoggingConfig{
		LogLevel:         "info",
		EnableJSON:       true,
		PrivacyMode:      true,
		SensitiveFields:  []string{"password", "token", "secret"},
		CorrelationTTL:   time.Hour,
		MaxCorrelations:  1000,
	}

	var buf bytes.Buffer
	logger := NewMCPLogger(config)
	logger.logger.SetOutput(&buf)

	clientInfo := ClientInfo{
		ID:   "sensitive-client-id",
		Type: "claude",
	}
	correlationID := logger.StartCorrelation("session123", clientInfo)

	operation := OperationLog{
		Type:      "tool_call",
		Name:      "test_tool",
		StartTime: time.Now(),
		Parameters: map[string]interface{}{
			"normal_param": "normal_value",
			"password":     "secret123",
			"token":        "abc123def456",
		},
	}

	logger.LogOperation(correlationID, operation)

	logOutput := buf.String()
	
	// Should not contain sensitive values
	assert.NotContains(t, logOutput, "secret123")
	assert.NotContains(t, logOutput, "abc123def456")
	assert.NotContains(t, logOutput, "sensitive-client-id")
	
	// Should contain redacted values or hashed IDs
	assert.Contains(t, logOutput, "[REDACTED]")
}

func TestMCPLogger_EndOperation(t *testing.T) {
	config := MCPLoggingConfig{
		LogLevel:        "info",
		EnableJSON:      true,
		CorrelationTTL:  time.Hour,
		MaxCorrelations: 1000,
	}

	logger := NewMCPLogger(config)

	clientInfo := ClientInfo{ID: "test-client", Type: "claude"}
	correlationID := logger.StartCorrelation("session123", clientInfo)

	operation := OperationLog{
		Type:      "tool_call",
		Name:      "test_tool",
		StartTime: time.Now().Add(-100 * time.Millisecond),
	}

	logger.LogOperation(correlationID, operation)

	result := map[string]interface{}{"status": "success", "result": "test_result"}
	err := logger.EndOperation(correlationID, "test_tool", result, nil)

	assert.NoError(t, err)

	// Check that operation was updated
	logger.mutex.RLock()
	correlation := logger.correlations[correlationID]
	logger.mutex.RUnlock()

	assert.Len(t, correlation.Operations, 1)
	op := correlation.Operations[0]
	assert.NotNil(t, op.EndTime)
	assert.True(t, op.Duration > 0)
	assert.Equal(t, result, op.Result)
	assert.Nil(t, op.Error)
	assert.True(t, op.Success)
}

func TestMCPLogger_EndOperationWithError(t *testing.T) {
	config := MCPLoggingConfig{
		LogLevel:        "info",
		EnableJSON:      true,
		CorrelationTTL:  time.Hour,
		MaxCorrelations: 1000,
	}

	logger := NewMCPLogger(config)

	clientInfo := ClientInfo{ID: "test-client", Type: "claude"}
	correlationID := logger.StartCorrelation("session123", clientInfo)

	operation := OperationLog{
		Type:      "tool_call",
		Name:      "test_tool",
		StartTime: time.Now().Add(-100 * time.Millisecond),
	}

	logger.LogOperation(correlationID, operation)

	testErr := assert.AnError
	err := logger.EndOperation(correlationID, "test_tool", nil, testErr)

	assert.NoError(t, err)

	// Check that operation was updated with error
	logger.mutex.RLock()
	correlation := logger.correlations[correlationID]
	logger.mutex.RUnlock()

	assert.Len(t, correlation.Operations, 1)
	op := correlation.Operations[0]
	assert.NotNil(t, op.EndTime)
	assert.NotNil(t, op.Error)
	assert.Equal(t, testErr.Error(), op.Error.Message)
	assert.False(t, op.Success)
}

func TestMCPLogger_EndCorrelation(t *testing.T) {
	config := MCPLoggingConfig{
		LogLevel:        "info",
		EnableJSON:      true,
		CorrelationTTL:  time.Hour,
		MaxCorrelations: 1000,
		EnableMetrics:   true,
	}

	var buf bytes.Buffer
	logger := NewMCPLogger(config)
	logger.logger.SetOutput(&buf)

	clientInfo := ClientInfo{ID: "test-client", Type: "claude"}
	correlationID := logger.StartCorrelation("session123", clientInfo)

	operation := OperationLog{
		Type:      "tool_call",
		Name:      "test_tool",
		StartTime: time.Now().Add(-100 * time.Millisecond),
	}

	logger.LogOperation(correlationID, operation)
	logger.EndOperation(correlationID, "test_tool", map[string]interface{}{"result": "ok"}, nil)

	summary := logger.EndCorrelation(correlationID)

	assert.NotNil(t, summary)
	assert.Equal(t, correlationID, summary.CorrelationID)
	assert.Equal(t, "session123", summary.SessionID)
	assert.Equal(t, 1, summary.TotalOperations)
	assert.Equal(t, 1, summary.SuccessfulOperations)
	assert.Equal(t, 0, summary.FailedOperations)
	assert.True(t, summary.TotalDuration > 0)

	// Check that correlation was removed
	logger.mutex.RLock()
	_, exists := logger.correlations[correlationID]
	logger.mutex.RUnlock()

	assert.False(t, exists)

	// Check log output
	logOutput := buf.String()
	assert.Contains(t, logOutput, "Correlation ended")
	assert.Contains(t, logOutput, correlationID)
}

func TestMCPLogger_MaxCorrelations(t *testing.T) {
	config := MCPLoggingConfig{
		LogLevel:        "info",
		EnableJSON:      true,
		CorrelationTTL:  time.Hour,
		MaxCorrelations: 2, // Very small limit for testing
	}

	logger := NewMCPLogger(config)
	clientInfo := ClientInfo{ID: "test-client", Type: "claude"}

	// Create correlations up to the limit
	corr1 := logger.StartCorrelation("session1", clientInfo)
	corr2 := logger.StartCorrelation("session2", clientInfo)

	logger.mutex.RLock()
	assert.Len(t, logger.correlations, 2)
	logger.mutex.RUnlock()

	// Adding another should trigger cleanup
	corr3 := logger.StartCorrelation("session3", clientInfo)

	logger.mutex.RLock()
	correlationCount := len(logger.correlations)
	logger.mutex.RUnlock()

	// Should still be within limit (cleanup should have occurred)
	assert.LessOrEqual(t, correlationCount, 2)
	assert.NotEmpty(t, corr1)
	assert.NotEmpty(t, corr2)
	assert.NotEmpty(t, corr3)
}

func TestMCPLogger_JSONFormat(t *testing.T) {
	config := MCPLoggingConfig{
		LogLevel:        "info",
		EnableJSON:      true,
		CorrelationTTL:  time.Hour,
		MaxCorrelations: 1000,
	}

	var buf bytes.Buffer
	logger := NewMCPLogger(config)
	logger.logger.SetOutput(&buf)

	clientInfo := ClientInfo{ID: "test-client", Type: "claude"}
	correlationID := logger.StartCorrelation("session123", clientInfo)

	operation := OperationLog{
		Type:      "tool_call",
		Name:      "test_tool",
		StartTime: time.Now(),
	}

	logger.LogOperation(correlationID, operation)

	logOutput := buf.String()
	
	// Should be valid JSON
	lines := strings.Split(strings.TrimSpace(logOutput), "\n")
	for _, line := range lines {
		if line != "" {
			var logEntry map[string]interface{}
			err := json.Unmarshal([]byte(line), &logEntry)
			assert.NoError(t, err, "Log line should be valid JSON: %s", line)
		}
	}
}

func TestMCPLogger_Cleanup(t *testing.T) {
	config := MCPLoggingConfig{
		LogLevel:        "info",
		EnableJSON:      true,
		CorrelationTTL:  100 * time.Millisecond, // Very short TTL for testing
		MaxCorrelations: 1000,
	}

	logger := NewMCPLogger(config)
	clientInfo := ClientInfo{ID: "test-client", Type: "claude"}

	correlationID := logger.StartCorrelation("session123", clientInfo)

	logger.mutex.RLock()
	assert.Len(t, logger.correlations, 1)
	logger.mutex.RUnlock()

	// Wait for TTL to expire
	time.Sleep(200 * time.Millisecond)

	// Trigger cleanup by starting another correlation
	logger.StartCorrelation("session456", clientInfo)

	logger.mutex.RLock()
	_, exists := logger.correlations[correlationID]
	correlationCount := len(logger.correlations)
	logger.mutex.RUnlock()

	// Expired correlation should be cleaned up
	assert.False(t, exists)
	assert.Equal(t, 1, correlationCount) // Only the new one should remain
}

func TestMCPLogger_GetCorrelation(t *testing.T) {
	config := MCPLoggingConfig{
		LogLevel:        "info",
		EnableJSON:      true,
		CorrelationTTL:  time.Hour,
		MaxCorrelations: 1000,
	}

	logger := NewMCPLogger(config)
	clientInfo := ClientInfo{ID: "test-client", Type: "claude"}

	correlationID := logger.StartCorrelation("session123", clientInfo)

	// Test getting existing correlation
	correlation, err := logger.GetCorrelation(correlationID)
	assert.NoError(t, err)
	assert.NotNil(t, correlation)
	assert.Equal(t, correlationID, correlation.ID)

	// Test getting non-existent correlation
	_, err = logger.GetCorrelation("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "correlation not found")
}

func TestMCPLogger_Stop(t *testing.T) {
	config := MCPLoggingConfig{
		LogLevel:        "info",
		EnableJSON:      true,
		CorrelationTTL:  time.Hour,
		MaxCorrelations: 1000,
		CleanupInterval: 100 * time.Millisecond,
	}

	logger := NewMCPLogger(config)
	
	// Start the logger
	logger.Start()
	
	// Verify cleanup timer is running
	assert.NotNil(t, logger.cleanupTimer)
	
	// Stop the logger
	logger.Stop()
	
	// Timer should be stopped (we can't easily test this, but ensure no panic)
	time.Sleep(50 * time.Millisecond)
}

// Benchmark tests
func BenchmarkMCPLogger_StartCorrelation(b *testing.B) {
	config := MCPLoggingConfig{
		LogLevel:        "info",
		EnableJSON:      true,
		CorrelationTTL:  time.Hour,
		MaxCorrelations: 10000,
	}

	logger := NewMCPLogger(config)
	logger.logger.SetOutput(io.Discard) // Don't actually log during benchmark

	clientInfo := ClientInfo{ID: "test-client", Type: "claude"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sessionID := "session" + string(rune(i))
		logger.StartCorrelation(sessionID, clientInfo)
	}
}

func BenchmarkMCPLogger_LogOperation(b *testing.B) {
	config := MCPLoggingConfig{
		LogLevel:        "info",
		EnableJSON:      true,
		CorrelationTTL:  time.Hour,
		MaxCorrelations: 10000,
	}

	logger := NewMCPLogger(config)
	logger.logger.SetOutput(io.Discard) // Don't actually log during benchmark

	clientInfo := ClientInfo{ID: "test-client", Type: "claude"}
	correlationID := logger.StartCorrelation("session123", clientInfo)

	operation := OperationLog{
		Type:       "tool_call",
		Name:       "test_tool",
		StartTime:  time.Now(),
		Parameters: map[string]interface{}{"param": "value"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.LogOperation(correlationID, operation)
	}
}