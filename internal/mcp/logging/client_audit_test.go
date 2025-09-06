package logging

import (
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientAuditLogger_NewLogger(t *testing.T) {
	config := ClientAuditConfig{
		EnableAuditTrail:   true,
		RetentionPeriod:    24 * time.Hour,
		PrivacyMode:        true,
		HashClientData:     true,
		MaxTrailSize:       1000,
		SensitiveFields:    []string{"password", "token"},
		CleanupInterval:    time.Hour,
		CompressOldEntries: true,
	}

	auditor := NewClientAuditLogger(config)

	assert.NotNil(t, auditor)
	assert.Equal(t, config, auditor.config)
	assert.NotNil(t, auditor.sessions)
	assert.NotNil(t, auditor.auditTrails)
	assert.NotNil(t, auditor.cleanupTimer)
}

func TestClientAuditLogger_StartSession(t *testing.T) {
	config := ClientAuditConfig{
		EnableAuditTrail: true,
		PrivacyMode:      false,
		MaxTrailSize:     1000,
	}

	auditor := NewClientAuditLogger(config)
	auditor.logger.SetOutput(io.Discard) // Suppress log output

	connectionInfo := ConnectionInfo{
		RemoteAddr:  "192.168.1.100",
		UserAgent:   "Claude/1.0.0",
		Protocol:    "HTTP/1.1",
		ConnectedAt: time.Now(),
	}

	sessionID := "session123"
	clientID := "client456"
	clientType := "claude"

	auditor.StartSession(sessionID, clientID, clientType, connectionInfo)

	// Check session was created
	session, err := auditor.GetSessionSummary(sessionID)
	assert.NoError(t, err)
	assert.Equal(t, sessionID, session.SessionID)
	assert.Equal(t, clientID, session.ClientID)
	assert.Equal(t, clientType, session.ClientType)
	assert.Equal(t, connectionInfo.RemoteAddr, session.ConnectionInfo.RemoteAddr)

	// Check audit trail was created
	trail, err := auditor.GetAuditTrail(sessionID)
	assert.NoError(t, err)
	assert.Equal(t, sessionID, trail.SessionID)
	assert.Equal(t, clientID, trail.ClientID)
	assert.NotEmpty(t, trail.Entries)

	// Check first entry is session start
	firstEntry := trail.Entries[0]
	assert.Equal(t, EventClientConnect, firstEntry.EventType)
	assert.Equal(t, "session_start", firstEntry.Action)
	assert.True(t, firstEntry.Success)
}

func TestClientAuditLogger_StartSessionWithPrivacy(t *testing.T) {
	config := ClientAuditConfig{
		EnableAuditTrail: true,
		PrivacyMode:      true,
		MaxTrailSize:     1000,
	}

	auditor := NewClientAuditLogger(config)
	auditor.logger.SetOutput(io.Discard)

	connectionInfo := ConnectionInfo{
		RemoteAddr: "192.168.1.100",
		UserAgent:  "Claude/1.0.0 (specific version info)",
		Protocol:   "HTTP/1.1",
	}

	auditor.StartSession("session123", "sensitive-client-id", "claude", connectionInfo)

	session, err := auditor.GetSessionSummary("session123")
	assert.NoError(t, err)

	// In privacy mode, client ID should be hashed
	assert.NotEqual(t, "sensitive-client-id", session.ClientID)
	assert.NotContains(t, session.ClientID, "sensitive-client-id")

	// User agent should be sanitized
	assert.Contains(t, session.ConnectionInfo.UserAgent, "Claude/***")

	// IP should be sanitized
	assert.Contains(t, session.ConnectionInfo.RemoteAddr, "hash_")
}

func TestClientAuditLogger_LogToolInvocation(t *testing.T) {
	config := ClientAuditConfig{
		EnableAuditTrail: true,
		PrivacyMode:      false,
		MaxTrailSize:     1000,
		HashClientData:   true,
	}

	auditor := NewClientAuditLogger(config)
	auditor.logger.SetOutput(io.Discard)

	sessionID := "session123"
	auditor.StartSession(sessionID, "client456", "claude", ConnectionInfo{})

	parameters := map[string]interface{}{
		"variant": "NM_000492.3:c.1521_1523del",
		"gene":    "CFTR",
	}

	response := map[string]interface{}{
		"classification": "pathogenic",
		"confidence":     0.95,
	}

	duration := 150 * time.Millisecond

	auditor.LogToolInvocation(sessionID, "classify_variant", parameters, response, duration, nil)

	trail, err := auditor.GetAuditTrail(sessionID)
	assert.NoError(t, err)

	// Should have session start + tool invocation entries
	assert.Len(t, trail.Entries, 2)

	toolEntry := trail.Entries[1]
	assert.Equal(t, EventToolInvocation, toolEntry.EventType)
	assert.Equal(t, "tool_invoke", toolEntry.Action)
	assert.Equal(t, "classify_variant", toolEntry.Resource)
	assert.Equal(t, parameters, toolEntry.Parameters)
	assert.Equal(t, duration, toolEntry.Duration)
	assert.True(t, toolEntry.Success)
	assert.Empty(t, toolEntry.ErrorMessage)

	// Check response summary
	assert.Greater(t, toolEntry.Response.ResponseSize, 0)
	assert.True(t, toolEntry.Response.DataSummary["has_data"].(bool))

	// Check data hash (when HashClientData is enabled)
	assert.NotEmpty(t, toolEntry.DataHash)
}

func TestClientAuditLogger_LogToolInvocationWithError(t *testing.T) {
	config := ClientAuditConfig{
		EnableAuditTrail: true,
		PrivacyMode:      false,
		MaxTrailSize:     1000,
	}

	auditor := NewClientAuditLogger(config)
	auditor.logger.SetOutput(io.Discard)

	sessionID := "session123"
	auditor.StartSession(sessionID, "client456", "claude", ConnectionInfo{})

	parameters := map[string]interface{}{
		"variant": "invalid_hgvs",
	}

	testError := assert.AnError
	duration := 50 * time.Millisecond

	auditor.LogToolInvocation(sessionID, "classify_variant", parameters, nil, duration, testError)

	trail, err := auditor.GetAuditTrail(sessionID)
	assert.NoError(t, err)

	toolEntry := trail.Entries[1]
	assert.Equal(t, EventToolInvocation, toolEntry.EventType)
	assert.False(t, toolEntry.Success)
	assert.Equal(t, "TOOL_ERROR", toolEntry.ErrorCode)
	assert.Contains(t, toolEntry.ErrorMessage, testError.Error())
}

func TestClientAuditLogger_LogResourceAccess(t *testing.T) {
	config := ClientAuditConfig{
		EnableAuditTrail: true,
		PrivacyMode:      false,
		MaxTrailSize:     1000,
	}

	auditor := NewClientAuditLogger(config)
	auditor.logger.SetOutput(io.Discard)

	sessionID := "session123"
	auditor.StartSession(sessionID, "client456", "claude", ConnectionInfo{})

	resourceURI := "variant/NM_000492.3:c.1521_1523del"
	method := "GET"
	response := map[string]interface{}{
		"id":          "var123",
		"gene":        "CFTR",
		"consequence": "frameshift",
	}
	duration := 25 * time.Millisecond

	auditor.LogResourceAccess(sessionID, resourceURI, method, response, duration, nil)

	trail, err := auditor.GetAuditTrail(sessionID)
	assert.NoError(t, err)

	resourceEntry := trail.Entries[1]
	assert.Equal(t, EventResourceAccess, resourceEntry.EventType)
	assert.Equal(t, "resource_access", resourceEntry.Action)
	assert.Equal(t, resourceURI, resourceEntry.Resource)
	assert.Equal(t, method, resourceEntry.Method)
	assert.Equal(t, duration, resourceEntry.Duration)
	assert.True(t, resourceEntry.Success)
}

func TestClientAuditLogger_LogSecurityViolation(t *testing.T) {
	config := ClientAuditConfig{
		EnableAuditTrail: true,
		PrivacyMode:      false,
		MaxTrailSize:     1000,
	}

	auditor := NewClientAuditLogger(config)
	auditor.logger.SetOutput(io.Discard)

	sessionID := "session123"
	auditor.StartSession(sessionID, "client456", "claude", ConnectionInfo{})

	violationType := "rate_limit_exceeded"
	description := "Client exceeded rate limit of 100 requests per minute"
	severity := "high"

	auditor.LogSecurityViolation(sessionID, violationType, description, severity)

	trail, err := auditor.GetAuditTrail(sessionID)
	assert.NoError(t, err)

	violationEntry := trail.Entries[1]
	assert.Equal(t, EventSecurityViolation, violationEntry.EventType)
	assert.Equal(t, "security_violation", violationEntry.Action)
	assert.Equal(t, violationType, violationEntry.Resource)
	assert.False(t, violationEntry.Success)
	assert.Equal(t, "SECURITY_VIOLATION", violationEntry.ErrorCode)
	assert.Equal(t, description, violationEntry.ErrorMessage)

	// Check session security context was updated
	session, err := auditor.GetSessionSummary(sessionID)
	assert.NoError(t, err)
	assert.Len(t, session.SecurityContext.Violations, 1)

	violation := session.SecurityContext.Violations[0]
	assert.Equal(t, violationType, violation.Type)
	assert.Equal(t, description, violation.Description)
	assert.Equal(t, severity, violation.Severity)
	assert.False(t, violation.Resolved)

	// Risk score should be increased
	assert.Greater(t, session.SecurityContext.RiskScore, 0.0)
}

func TestClientAuditLogger_EndSession(t *testing.T) {
	config := ClientAuditConfig{
		EnableAuditTrail: true,
		PrivacyMode:      false,
		MaxTrailSize:     1000,
	}

	auditor := NewClientAuditLogger(config)
	auditor.logger.SetOutput(io.Discard)

	sessionID := "session123"
	auditor.StartSession(sessionID, "client456", "claude", ConnectionInfo{})

	// Add some activity
	time.Sleep(10 * time.Millisecond) // Small delay to see duration
	auditor.LogToolInvocation(sessionID, "test_tool", map[string]interface{}{}, nil, time.Millisecond, nil)

	auditor.EndSession(sessionID)

	// Session should be removed from active sessions
	_, err := auditor.GetSessionSummary(sessionID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session not found")

	// But audit trail should still exist
	trail, err := auditor.GetAuditTrail(sessionID)
	assert.NoError(t, err)

	// Should have session start, tool invocation, and session end entries
	assert.Len(t, trail.Entries, 3)

	endEntry := trail.Entries[2]
	assert.Equal(t, EventClientDisconnect, endEntry.EventType)
	assert.Equal(t, "session_end", endEntry.Action)
	assert.True(t, endEntry.Success)
	assert.Greater(t, endEntry.Duration, time.Duration(0))
}

func TestClientAuditLogger_MaxTrailSize(t *testing.T) {
	config := ClientAuditConfig{
		EnableAuditTrail: true,
		PrivacyMode:      false,
		MaxTrailSize:     3, // Very small for testing
	}

	auditor := NewClientAuditLogger(config)
	auditor.logger.SetOutput(io.Discard)

	sessionID := "session123"
	auditor.StartSession(sessionID, "client456", "claude", ConnectionInfo{})

	// Add more entries than the limit
	for i := 0; i < 5; i++ {
		auditor.LogToolInvocation(sessionID, "test_tool", map[string]interface{}{}, nil, time.Millisecond, nil)
	}

	trail, err := auditor.GetAuditTrail(sessionID)
	assert.NoError(t, err)

	// Should be limited to MaxTrailSize
	assert.LessOrEqual(t, len(trail.Entries), config.MaxTrailSize)
	assert.Equal(t, 6, trail.TotalEntries) // Original count should be preserved
}

func TestClientAuditLogger_SensitiveFieldRedaction(t *testing.T) {
	config := ClientAuditConfig{
		EnableAuditTrail: true,
		PrivacyMode:      true,
		SensitiveFields:  []string{"password", "token", "secret"},
		MaxTrailSize:     1000,
	}

	auditor := NewClientAuditLogger(config)
	auditor.logger.SetOutput(io.Discard)

	sessionID := "session123"
	auditor.StartSession(sessionID, "client456", "claude", ConnectionInfo{})

	parameters := map[string]interface{}{
		"username":     "testuser",
		"password":     "secret123",
		"api_token":    "abc123def456",
		"client_secret": "supersecret",
		"normal_field": "normal_value",
	}

	auditor.LogToolInvocation(sessionID, "auth_tool", parameters, nil, time.Millisecond, nil)

	trail, err := auditor.GetAuditTrail(sessionID)
	assert.NoError(t, err)

	toolEntry := trail.Entries[1]
	
	// Sensitive fields should be redacted
	assert.Equal(t, "[REDACTED]", toolEntry.Parameters["password"])
	assert.Equal(t, "[REDACTED]", toolEntry.Parameters["api_token"])
	assert.Equal(t, "[REDACTED]", toolEntry.Parameters["client_secret"])
	
	// Normal fields should be preserved
	assert.Equal(t, "testuser", toolEntry.Parameters["username"])
	assert.Equal(t, "normal_value", toolEntry.Parameters["normal_field"])
}

func TestClientAuditLogger_ExportAuditData(t *testing.T) {
	config := ClientAuditConfig{
		EnableAuditTrail: true,
		PrivacyMode:      false,
		MaxTrailSize:     1000,
	}

	auditor := NewClientAuditLogger(config)
	auditor.logger.SetOutput(io.Discard)

	// Create multiple sessions
	session1 := "session123"
	session2 := "session456"

	auditor.StartSession(session1, "client1", "claude", ConnectionInfo{})
	auditor.StartSession(session2, "client2", "chatgpt", ConnectionInfo{})

	auditor.LogToolInvocation(session1, "tool1", map[string]interface{}{}, nil, time.Millisecond, nil)
	auditor.LogToolInvocation(session2, "tool2", map[string]interface{}{}, nil, time.Millisecond, nil)

	// Export specific sessions
	exportData, err := auditor.ExportAuditData([]string{session1, session2})
	assert.NoError(t, err)
	assert.NotEmpty(t, exportData)

	// Parse exported JSON
	var export map[string]interface{}
	err = json.Unmarshal(exportData, &export)
	assert.NoError(t, err)

	// Check structure
	assert.Contains(t, export, "export_timestamp")
	assert.Contains(t, export, "sessions")
	assert.Contains(t, export, "audit_trails")

	sessions := export["sessions"].(map[string]interface{})
	trails := export["audit_trails"].(map[string]interface{})

	assert.Len(t, sessions, 2)
	assert.Len(t, trails, 2)
	assert.Contains(t, sessions, session1)
	assert.Contains(t, sessions, session2)
	assert.Contains(t, trails, session1)
	assert.Contains(t, trails, session2)
}

func TestClientAuditLogger_CleanupExpiredData(t *testing.T) {
	config := ClientAuditConfig{
		EnableAuditTrail: true,
		RetentionPeriod:  50 * time.Millisecond, // Very short for testing
		PrivacyMode:      false,
		MaxTrailSize:     1000,
		CleanupInterval:  10 * time.Millisecond,
	}

	auditor := NewClientAuditLogger(config)
	auditor.logger.SetOutput(io.Discard)

	sessionID := "session123"
	auditor.StartSession(sessionID, "client456", "claude", ConnectionInfo{})

	// Verify session and trail exist
	_, err := auditor.GetSessionSummary(sessionID)
	assert.NoError(t, err)

	_, err = auditor.GetAuditTrail(sessionID)
	assert.NoError(t, err)

	// Wait for retention period to expire
	time.Sleep(100 * time.Millisecond)

	// Trigger cleanup by calling it directly
	auditor.cleanupExpiredData()

	// Session and trail should be cleaned up
	_, err = auditor.GetSessionSummary(sessionID)
	assert.Error(t, err)

	_, err = auditor.GetAuditTrail(sessionID)
	assert.Error(t, err)
}

func TestClientAuditLogger_CorrelationIDGeneration(t *testing.T) {
	config := ClientAuditConfig{
		EnableAuditTrail: true,
		MaxTrailSize:     1000,
	}

	auditor := NewClientAuditLogger(config)

	sessionID := "session123"
	resource := "test_resource"

	// Generate multiple correlation IDs
	id1 := auditor.generateCorrelationID(sessionID, resource)
	id2 := auditor.generateCorrelationID(sessionID, resource)

	// Should be different (due to timestamp)
	assert.NotEqual(t, id1, id2)

	// Should contain session ID
	assert.Contains(t, id1, sessionID[:8])
	assert.Contains(t, id2, sessionID[:8])

	// Should have expected format
	parts1 := strings.Split(id1, "_")
	parts2 := strings.Split(id2, "_")

	assert.Len(t, parts1, 3)
	assert.Len(t, parts2, 3)
}

// Benchmark tests
func BenchmarkClientAuditLogger_LogToolInvocation(b *testing.B) {
	config := ClientAuditConfig{
		EnableAuditTrail: true,
		PrivacyMode:      false,
		MaxTrailSize:     10000,
	}

	auditor := NewClientAuditLogger(config)
	auditor.logger.SetOutput(io.Discard)

	sessionID := "session123"
	auditor.StartSession(sessionID, "client456", "claude", ConnectionInfo{})

	parameters := map[string]interface{}{
		"variant": "NM_000492.3:c.1521_1523del",
		"gene":    "CFTR",
	}

	response := map[string]interface{}{
		"classification": "pathogenic",
		"confidence":     0.95,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		auditor.LogToolInvocation(sessionID, "classify_variant", parameters, response, time.Millisecond, nil)
	}
}