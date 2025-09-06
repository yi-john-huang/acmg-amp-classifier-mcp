package logging

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

type ClientAuditLogger struct {
	logger       *logrus.Logger
	config       ClientAuditConfig
	sessions     map[string]*ClientSession
	auditTrails  map[string]*AuditTrail
	mutex        sync.RWMutex
	retention    time.Duration
	privacyMode  bool
	cleanupTimer *time.Timer
}

type ClientAuditConfig struct {
	EnableAuditTrail    bool          `json:"enable_audit_trail"`
	RetentionPeriod     time.Duration `json:"retention_period"`
	PrivacyMode         bool          `json:"privacy_mode"`
	HashClientData      bool          `json:"hash_client_data"`
	MaxTrailSize        int           `json:"max_trail_size"`
	SensitiveFields     []string      `json:"sensitive_fields"`
	CleanupInterval     time.Duration `json:"cleanup_interval"`
	CompressOldEntries  bool          `json:"compress_old_entries"`
}

type ClientSession struct {
	SessionID       string                 `json:"session_id"`
	ClientID        string                 `json:"client_id"`
	ClientType      string                 `json:"client_type"`
	StartTime       time.Time              `json:"start_time"`
	LastActivity    time.Time              `json:"last_activity"`
	TotalRequests   int                    `json:"total_requests"`
	TotalResponses  int                    `json:"total_responses"`
	Capabilities    []string               `json:"capabilities"`
	ConnectionInfo  ConnectionInfo         `json:"connection_info"`
	SecurityContext SecurityContext        `json:"security_context"`
	Metadata        map[string]interface{} `json:"metadata"`
}

type AuditTrail struct {
	ClientID       string       `json:"client_id"`
	SessionID      string       `json:"session_id"`
	Entries        []AuditEntry `json:"entries"`
	CreatedAt      time.Time    `json:"created_at"`
	LastUpdated    time.Time    `json:"last_updated"`
	TotalEntries   int          `json:"total_entries"`
	CompressedSize int          `json:"compressed_size"`
}

type AuditEntry struct {
	Timestamp     time.Time              `json:"timestamp"`
	EventType     AuditEventType         `json:"event_type"`
	Action        string                 `json:"action"`
	Resource      string                 `json:"resource"`
	Method        string                 `json:"method"`
	Parameters    map[string]interface{} `json:"parameters"`
	Response      AuditResponse          `json:"response"`
	Duration      time.Duration          `json:"duration"`
	ClientIP      string                 `json:"client_ip"`
	UserAgent     string                 `json:"user_agent"`
	CorrelationID string                 `json:"correlation_id"`
	Success       bool                   `json:"success"`
	ErrorCode     string                 `json:"error_code,omitempty"`
	ErrorMessage  string                 `json:"error_message,omitempty"`
	DataHash      string                 `json:"data_hash,omitempty"`
}

type AuditResponse struct {
	StatusCode   int                    `json:"status_code"`
	ResponseSize int                    `json:"response_size"`
	ContentType  string                 `json:"content_type"`
	Headers      map[string]string      `json:"headers,omitempty"`
	DataSummary  map[string]interface{} `json:"data_summary"`
}

type ConnectionInfo struct {
	RemoteAddr    string    `json:"remote_addr"`
	UserAgent     string    `json:"user_agent"`
	Protocol      string    `json:"protocol"`
	TLSVersion    string    `json:"tls_version,omitempty"`
	ConnectedAt   time.Time `json:"connected_at"`
	BytesSent     int64     `json:"bytes_sent"`
	BytesReceived int64     `json:"bytes_received"`
}

type SecurityContext struct {
	AuthMethod    string            `json:"auth_method,omitempty"`
	Permissions   []string          `json:"permissions"`
	RiskScore     float64           `json:"risk_score"`
	Violations    []SecurityViolation `json:"violations,omitempty"`
	LastValidated time.Time         `json:"last_validated"`
}

type SecurityViolation struct {
	Type        string    `json:"type"`
	Description string    `json:"description"`
	Severity    string    `json:"severity"`
	Timestamp   time.Time `json:"timestamp"`
	Resolved    bool      `json:"resolved"`
}

type AuditEventType string

const (
	EventClientConnect    AuditEventType = "client_connect"
	EventClientDisconnect AuditEventType = "client_disconnect"
	EventToolInvocation   AuditEventType = "tool_invocation"
	EventResourceAccess   AuditEventType = "resource_access"
	EventPromptRequest    AuditEventType = "prompt_request"
	EventCapabilityQuery  AuditEventType = "capability_query"
	EventAuthAttempt      AuditEventType = "auth_attempt"
	EventSecurityViolation AuditEventType = "security_violation"
	EventDataExport       AuditEventType = "data_export"
	EventConfigChange     AuditEventType = "config_change"
)

func NewClientAuditLogger(config ClientAuditConfig) *ClientAuditLogger {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: time.RFC3339Nano,
		FieldMap: logrus.FieldMap{
			logrus.FieldKeyTime:  "audit_timestamp",
			logrus.FieldKeyLevel: "audit_level",
			logrus.FieldKeyMsg:   "audit_message",
		},
	})

	if config.RetentionPeriod == 0 {
		config.RetentionPeriod = 30 * 24 * time.Hour // 30 days default
	}
	if config.MaxTrailSize == 0 {
		config.MaxTrailSize = 10000 // 10k entries default
	}
	if config.CleanupInterval == 0 {
		config.CleanupInterval = time.Hour // 1 hour default
	}

	auditor := &ClientAuditLogger{
		logger:      logger,
		config:      config,
		sessions:    make(map[string]*ClientSession),
		auditTrails: make(map[string]*AuditTrail),
		retention:   config.RetentionPeriod,
		privacyMode: config.PrivacyMode,
	}

	auditor.startCleanupRoutine()
	return auditor
}

func (c *ClientAuditLogger) StartSession(sessionID, clientID, clientType string, connectionInfo ConnectionInfo) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	session := &ClientSession{
		SessionID:       sessionID,
		ClientID:        c.sanitizeClientID(clientID),
		ClientType:      clientType,
		StartTime:       time.Now(),
		LastActivity:    time.Now(),
		ConnectionInfo:  c.sanitizeConnectionInfo(connectionInfo),
		SecurityContext: SecurityContext{Permissions: []string{}},
		Metadata:        make(map[string]interface{}),
	}

	c.sessions[sessionID] = session

	// Create audit trail
	c.auditTrails[sessionID] = &AuditTrail{
		ClientID:    session.ClientID,
		SessionID:   sessionID,
		Entries:     make([]AuditEntry, 0),
		CreatedAt:   time.Now(),
		LastUpdated: time.Now(),
	}

	// Log session start
	c.logAuditEvent(sessionID, AuditEntry{
		Timestamp:     time.Now(),
		EventType:     EventClientConnect,
		Action:        "session_start",
		Success:       true,
		ClientIP:      connectionInfo.RemoteAddr,
		UserAgent:     connectionInfo.UserAgent,
		CorrelationID: sessionID,
	})

	c.logger.WithFields(logrus.Fields{
		"session_id":  sessionID,
		"client_id":   session.ClientID,
		"client_type": clientType,
		"remote_addr": c.sanitizeIP(connectionInfo.RemoteAddr),
	}).Info("Client session started")
}

func (c *ClientAuditLogger) EndSession(sessionID string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	session, exists := c.sessions[sessionID]
	if !exists {
		return
	}

	// Log session end
	duration := time.Since(session.StartTime)
	c.logAuditEvent(sessionID, AuditEntry{
		Timestamp:     time.Now(),
		EventType:     EventClientDisconnect,
		Action:        "session_end",
		Success:       true,
		Duration:      duration,
		ClientIP:      session.ConnectionInfo.RemoteAddr,
		CorrelationID: sessionID,
	})

	c.logger.WithFields(logrus.Fields{
		"session_id":     sessionID,
		"client_id":      session.ClientID,
		"duration":       duration,
		"total_requests": session.TotalRequests,
		"bytes_sent":     session.ConnectionInfo.BytesSent,
		"bytes_received": session.ConnectionInfo.BytesReceived,
	}).Info("Client session ended")

	delete(c.sessions, sessionID)
}

func (c *ClientAuditLogger) LogToolInvocation(sessionID, toolName string, parameters map[string]interface{}, response interface{}, duration time.Duration, err error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	correlationID := c.generateCorrelationID(sessionID, toolName)
	
	entry := AuditEntry{
		Timestamp:     time.Now(),
		EventType:     EventToolInvocation,
		Action:        "tool_invoke",
		Resource:      toolName,
		Method:        "invoke",
		Parameters:    c.sanitizeParameters(parameters),
		Duration:      duration,
		CorrelationID: correlationID,
		Success:       err == nil,
	}

	if err != nil {
		entry.ErrorMessage = c.sanitizeErrorMessage(err.Error())
		entry.ErrorCode = "TOOL_ERROR"
	}

	// Create response summary
	entry.Response = c.createResponseSummary(response)
	
	// Hash sensitive data if configured
	if c.config.HashClientData {
		entry.DataHash = c.hashData(parameters)
	}

	c.logAuditEvent(sessionID, entry)
	c.updateSessionActivity(sessionID)

	c.logger.WithFields(logrus.Fields{
		"session_id":     sessionID,
		"tool_name":      toolName,
		"duration":       duration,
		"success":        err == nil,
		"correlation_id": correlationID,
	}).Info("Tool invocation audited")
}

func (c *ClientAuditLogger) LogResourceAccess(sessionID, resourceURI string, method string, response interface{}, duration time.Duration, err error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	correlationID := c.generateCorrelationID(sessionID, resourceURI)
	
	entry := AuditEntry{
		Timestamp:     time.Now(),
		EventType:     EventResourceAccess,
		Action:        "resource_access",
		Resource:      resourceURI,
		Method:        method,
		Duration:      duration,
		CorrelationID: correlationID,
		Success:       err == nil,
	}

	if err != nil {
		entry.ErrorMessage = c.sanitizeErrorMessage(err.Error())
		entry.ErrorCode = "RESOURCE_ERROR"
	}

	entry.Response = c.createResponseSummary(response)

	c.logAuditEvent(sessionID, entry)
	c.updateSessionActivity(sessionID)
}

func (c *ClientAuditLogger) LogSecurityViolation(sessionID, violationType, description, severity string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	violation := SecurityViolation{
		Type:        violationType,
		Description: description,
		Severity:    severity,
		Timestamp:   time.Now(),
		Resolved:    false,
	}

	if session, exists := c.sessions[sessionID]; exists {
		session.SecurityContext.Violations = append(session.SecurityContext.Violations, violation)
		session.SecurityContext.RiskScore += c.calculateRiskImpact(severity)
	}

	entry := AuditEntry{
		Timestamp:     time.Now(),
		EventType:     EventSecurityViolation,
		Action:        "security_violation",
		Resource:      violationType,
		Success:       false,
		ErrorCode:     "SECURITY_VIOLATION",
		ErrorMessage:  description,
		CorrelationID: c.generateCorrelationID(sessionID, violationType),
	}

	c.logAuditEvent(sessionID, entry)

	c.logger.WithFields(logrus.Fields{
		"session_id":      sessionID,
		"violation_type":  violationType,
		"severity":        severity,
		"description":     description,
	}).Warn("Security violation logged")
}

func (c *ClientAuditLogger) GetAuditTrail(sessionID string) (*AuditTrail, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	trail, exists := c.auditTrails[sessionID]
	if !exists {
		return nil, fmt.Errorf("audit trail not found for session %s", sessionID)
	}

	// Return a copy to prevent external modification
	trailCopy := *trail
	trailCopy.Entries = make([]AuditEntry, len(trail.Entries))
	copy(trailCopy.Entries, trail.Entries)

	return &trailCopy, nil
}

func (c *ClientAuditLogger) GetSessionSummary(sessionID string) (*ClientSession, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	session, exists := c.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	// Return a copy
	sessionCopy := *session
	return &sessionCopy, nil
}

func (c *ClientAuditLogger) ExportAuditData(sessionIDs []string) ([]byte, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	exportData := make(map[string]interface{})
	exportData["export_timestamp"] = time.Now()
	exportData["sessions"] = make(map[string]*ClientSession)
	exportData["audit_trails"] = make(map[string]*AuditTrail)

	for _, sessionID := range sessionIDs {
		if session, exists := c.sessions[sessionID]; exists {
			exportData["sessions"].(map[string]*ClientSession)[sessionID] = session
		}
		if trail, exists := c.auditTrails[sessionID]; exists {
			exportData["audit_trails"].(map[string]*AuditTrail)[sessionID] = trail
		}
	}

	return json.MarshalIndent(exportData, "", "  ")
}

func (c *ClientAuditLogger) logAuditEvent(sessionID string, entry AuditEntry) {
	trail, exists := c.auditTrails[sessionID]
	if !exists {
		return
	}

	// Add entry to trail
	trail.Entries = append(trail.Entries, entry)
	trail.LastUpdated = time.Now()
	trail.TotalEntries++

	// Enforce max trail size
	if len(trail.Entries) > c.config.MaxTrailSize {
		// Keep most recent entries
		trail.Entries = trail.Entries[len(trail.Entries)-c.config.MaxTrailSize:]
	}

	// Log to structured logger
	c.logger.WithFields(logrus.Fields{
		"session_id":     sessionID,
		"event_type":     entry.EventType,
		"action":         entry.Action,
		"resource":       entry.Resource,
		"success":        entry.Success,
		"duration":       entry.Duration,
		"correlation_id": entry.CorrelationID,
	}).Info("Audit event recorded")
}

func (c *ClientAuditLogger) updateSessionActivity(sessionID string) {
	if session, exists := c.sessions[sessionID]; exists {
		session.LastActivity = time.Now()
		session.TotalRequests++
	}
}

func (c *ClientAuditLogger) sanitizeClientID(clientID string) string {
	if !c.privacyMode {
		return clientID
	}
	return c.hashString(clientID)
}

func (c *ClientAuditLogger) sanitizeConnectionInfo(info ConnectionInfo) ConnectionInfo {
	if !c.privacyMode {
		return info
	}
	
	info.RemoteAddr = c.sanitizeIP(info.RemoteAddr)
	info.UserAgent = c.sanitizeUserAgent(info.UserAgent)
	return info
}

func (c *ClientAuditLogger) sanitizeIP(ip string) string {
	if !c.privacyMode {
		return ip
	}
	
	// Hash IP while preserving format
	parts := strings.Split(ip, ".")
	if len(parts) == 4 {
		return fmt.Sprintf("hash_%s", c.hashString(ip)[:8])
	}
	return c.hashString(ip)[:16]
}

func (c *ClientAuditLogger) sanitizeUserAgent(userAgent string) string {
	if !c.privacyMode {
		return userAgent
	}
	
	// Extract browser/client type but hash specific version
	if strings.Contains(userAgent, "Claude") {
		return "Claude/***"
	}
	if strings.Contains(userAgent, "ChatGPT") {
		return "ChatGPT/***"
	}
	return "Unknown/***"
}

func (c *ClientAuditLogger) sanitizeParameters(params map[string]interface{}) map[string]interface{} {
	if !c.privacyMode {
		return params
	}
	
	sanitized := make(map[string]interface{})
	for key, value := range params {
		if c.isSensitiveField(key) {
			sanitized[key] = "[REDACTED]"
		} else {
			sanitized[key] = value
		}
	}
	return sanitized
}

func (c *ClientAuditLogger) sanitizeErrorMessage(message string) string {
	if !c.privacyMode {
		return message
	}
	
	// Remove potentially sensitive information from error messages
	sensitivePatterns := []string{
		`password=\w+`,
		`token=\w+`,
		`key=\w+`,
		`secret=\w+`,
	}
	
	sanitized := message
	for _, pattern := range sensitivePatterns {
		sanitized = strings.ReplaceAll(sanitized, pattern, "[REDACTED]")
	}
	return sanitized
}

func (c *ClientAuditLogger) isSensitiveField(field string) bool {
	for _, sensitive := range c.config.SensitiveFields {
		if strings.Contains(strings.ToLower(field), strings.ToLower(sensitive)) {
			return true
		}
	}
	return false
}

func (c *ClientAuditLogger) hashString(input string) string {
	hash := sha256.Sum256([]byte(input))
	return hex.EncodeToString(hash[:])[:16] // First 16 chars for brevity
}

func (c *ClientAuditLogger) hashData(data interface{}) string {
	jsonData, _ := json.Marshal(data)
	hash := sha256.Sum256(jsonData)
	return hex.EncodeToString(hash[:])[:16]
}

func (c *ClientAuditLogger) generateCorrelationID(sessionID, resource string) string {
	return fmt.Sprintf("%s_%s_%d", sessionID[:8], c.hashString(resource)[:8], time.Now().UnixNano())
}

func (c *ClientAuditLogger) createResponseSummary(response interface{}) AuditResponse {
	summary := AuditResponse{
		StatusCode:  200,
		ContentType: "application/json",
		DataSummary: make(map[string]interface{}),
	}
	
	if response != nil {
		// Create summary without sensitive data
		jsonData, _ := json.Marshal(response)
		summary.ResponseSize = len(jsonData)
		
		// Add basic summary info
		if len(jsonData) > 0 {
			summary.DataSummary["has_data"] = true
			summary.DataSummary["data_type"] = fmt.Sprintf("%T", response)
		}
	}
	
	return summary
}

func (c *ClientAuditLogger) calculateRiskImpact(severity string) float64 {
	switch strings.ToLower(severity) {
	case "critical":
		return 50.0
	case "high":
		return 25.0
	case "medium":
		return 10.0
	case "low":
		return 2.0
	default:
		return 1.0
	}
}

func (c *ClientAuditLogger) startCleanupRoutine() {
	c.cleanupTimer = time.AfterFunc(c.config.CleanupInterval, func() {
		c.cleanupExpiredData()
		c.startCleanupRoutine() // Reschedule
	})
}

func (c *ClientAuditLogger) cleanupExpiredData() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	cutoff := time.Now().Add(-c.retention)
	
	// Clean up old audit trails
	for sessionID, trail := range c.auditTrails {
		if trail.CreatedAt.Before(cutoff) {
			delete(c.auditTrails, sessionID)
			c.logger.WithField("session_id", sessionID).Info("Expired audit trail cleaned up")
		}
	}
	
	// Clean up inactive sessions
	for sessionID, session := range c.sessions {
		if session.LastActivity.Before(cutoff) {
			delete(c.sessions, sessionID)
			c.logger.WithField("session_id", sessionID).Info("Inactive session cleaned up")
		}
	}
}

func (c *ClientAuditLogger) Stop() {
	if c.cleanupTimer != nil {
		c.cleanupTimer.Stop()
	}
}