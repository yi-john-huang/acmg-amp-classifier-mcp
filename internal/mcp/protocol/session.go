package protocol

import (
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// SessionManager handles client session management and authentication
type SessionManager struct {
	logger     *logrus.Logger
	sessions   map[string]*ClientSession
	authConfig *AuthConfig
	mu         sync.RWMutex
}

// ClientSession represents an active MCP client session
type ClientSession struct {
	ID               string                 `json:"id"`
	ClientName       string                 `json:"client_name,omitempty"`
	ClientVersion    string                 `json:"client_version,omitempty"`
	ConnectedAt      time.Time              `json:"connected_at"`
	LastActivity     time.Time              `json:"last_activity"`
	Capabilities     map[string]interface{} `json:"capabilities"`
	Authenticated    bool                   `json:"authenticated"`
	AuthMethod       string                 `json:"auth_method,omitempty"`
	Metadata         map[string]string      `json:"metadata,omitempty"`
	RequestCount     int64                  `json:"request_count"`
	ErrorCount       int64                  `json:"error_count"`
}

// AuthConfig contains authentication configuration
type AuthConfig struct {
	Enabled       bool                   `json:"enabled"`
	Methods       []string               `json:"methods"`
	RequireAuth   bool                   `json:"require_auth"`
	TokenExpiry   time.Duration          `json:"token_expiry"`
	APIKeys       map[string]string      `json:"api_keys,omitempty"`
	CustomAuth    map[string]interface{} `json:"custom_auth,omitempty"`
}

// NewSessionManager creates a new session manager
func NewSessionManager(logger *logrus.Logger) *SessionManager {
	return &SessionManager{
		logger:   logger,
		sessions: make(map[string]*ClientSession),
		authConfig: &AuthConfig{
			Enabled:     false, // Disabled by default for now
			Methods:     []string{"none"},
			RequireAuth: false,
			TokenExpiry: 24 * time.Hour,
		},
	}
}

// CreateSession creates a new client session
func (sm *SessionManager) CreateSession(clientID string, capabilities map[string]interface{}) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Check if session already exists
	if _, exists := sm.sessions[clientID]; exists {
		return fmt.Errorf("session already exists for client %s", clientID)
	}

	// Extract client information from capabilities
	clientName := "unknown"
	clientVersion := "unknown"
	
	if clientInfo, exists := capabilities["client"]; exists {
		if clientMap, ok := clientInfo.(map[string]interface{}); ok {
			if name, exists := clientMap["name"]; exists {
				if nameStr, ok := name.(string); ok {
					clientName = nameStr
				}
			}
			if version, exists := clientMap["version"]; exists {
				if versionStr, ok := version.(string); ok {
					clientVersion = versionStr
				}
			}
		}
	}

	now := time.Now()
	session := &ClientSession{
		ID:               clientID,
		ClientName:       clientName,
		ClientVersion:    clientVersion,
		ConnectedAt:      now,
		LastActivity:     now,
		Capabilities:     capabilities,
		Authenticated:    !sm.authConfig.RequireAuth, // Auto-authenticate if auth not required
		AuthMethod:       sm.getDefaultAuthMethod(),
		Metadata:         make(map[string]string),
		RequestCount:     0,
		ErrorCount:       0,
	}

	sm.sessions[clientID] = session

	sm.logger.WithFields(logrus.Fields{
		"client_id":      clientID,
		"client_name":    clientName,
		"client_version": clientVersion,
		"authenticated":  session.Authenticated,
	}).Info("Created new MCP client session")

	return nil
}

// GetSession retrieves a client session
func (sm *SessionManager) GetSession(clientID string) (*ClientSession, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, exists := sm.sessions[clientID]
	if exists {
		// Return a copy to prevent external modification
		sessionCopy := *session
		return &sessionCopy, true
	}
	return nil, false
}

// UpdateClientActivity updates the last activity time for a client
func (sm *SessionManager) UpdateClientActivity(clientID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if session, exists := sm.sessions[clientID]; exists {
		session.LastActivity = time.Now()
		session.RequestCount++
	}
}

// RecordError records an error for a client session
func (sm *SessionManager) RecordError(clientID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if session, exists := sm.sessions[clientID]; exists {
		session.ErrorCount++
	}
}

// AuthenticateClient authenticates a client session
func (sm *SessionManager) AuthenticateClient(clientID, authMethod, credentials string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[clientID]
	if !exists {
		return fmt.Errorf("session not found for client %s", clientID)
	}

	if !sm.authConfig.Enabled {
		// Auth is disabled, mark as authenticated
		session.Authenticated = true
		session.AuthMethod = "none"
		sm.logger.WithField("client_id", clientID).Debug("Authentication disabled, marking client as authenticated")
		return nil
	}

	// Validate authentication method
	validMethod := false
	for _, method := range sm.authConfig.Methods {
		if method == authMethod {
			validMethod = true
			break
		}
	}

	if !validMethod {
		return fmt.Errorf("unsupported authentication method: %s", authMethod)
	}

	// Perform authentication based on method
	switch authMethod {
	case "api_key":
		if err := sm.authenticateAPIKey(credentials); err != nil {
			return fmt.Errorf("API key authentication failed: %w", err)
		}
	case "none":
		// No authentication required
	default:
		return fmt.Errorf("authentication method %s not implemented", authMethod)
	}

	session.Authenticated = true
	session.AuthMethod = authMethod

	sm.logger.WithFields(logrus.Fields{
		"client_id":   clientID,
		"auth_method": authMethod,
	}).Info("Client authenticated successfully")

	return nil
}

// authenticateAPIKey validates an API key
func (sm *SessionManager) authenticateAPIKey(apiKey string) error {
	if sm.authConfig.APIKeys == nil {
		return fmt.Errorf("API key authentication not configured")
	}

	// In a real implementation, you would validate against stored API keys
	// For now, we'll just check if it's not empty
	if apiKey == "" {
		return fmt.Errorf("empty API key")
	}

	// TODO: Implement proper API key validation
	return nil
}

// getDefaultAuthMethod returns the default authentication method
func (sm *SessionManager) getDefaultAuthMethod() string {
	if len(sm.authConfig.Methods) > 0 {
		return sm.authConfig.Methods[0]
	}
	return "none"
}

// RemoveSession removes a client session
func (sm *SessionManager) RemoveSession(clientID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if session, exists := sm.sessions[clientID]; exists {
		duration := time.Since(session.ConnectedAt)
		sm.logger.WithFields(logrus.Fields{
			"client_id":      clientID,
			"duration":       duration.String(),
			"request_count":  session.RequestCount,
			"error_count":    session.ErrorCount,
		}).Info("Removed MCP client session")

		delete(sm.sessions, clientID)
	}
}

// GetSessionCount returns the number of active sessions
func (sm *SessionManager) GetSessionCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.sessions)
}

// GetAllSessions returns all active sessions (for monitoring/debugging)
func (sm *SessionManager) GetAllSessions() map[string]*ClientSession {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	sessions := make(map[string]*ClientSession)
	for id, session := range sm.sessions {
		// Return copies to prevent external modification
		sessionCopy := *session
		sessions[id] = &sessionCopy
	}
	return sessions
}

// CleanupExpiredSessions removes sessions that have been inactive for too long
func (sm *SessionManager) CleanupExpiredSessions(maxInactivity time.Duration) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	now := time.Now()
	expiredSessions := make([]string, 0)

	for clientID, session := range sm.sessions {
		if now.Sub(session.LastActivity) > maxInactivity {
			expiredSessions = append(expiredSessions, clientID)
		}
	}

	for _, clientID := range expiredSessions {
		session := sm.sessions[clientID]
		sm.logger.WithFields(logrus.Fields{
			"client_id":      clientID,
			"last_activity":  session.LastActivity,
			"inactive_time":  now.Sub(session.LastActivity).String(),
		}).Info("Removed expired MCP client session")

		delete(sm.sessions, clientID)
	}

	if len(expiredSessions) > 0 {
		sm.logger.WithField("expired_count", len(expiredSessions)).Info("Cleaned up expired sessions")
	}
}

// UpdateAuthConfig updates the authentication configuration
func (sm *SessionManager) UpdateAuthConfig(config *AuthConfig) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.authConfig = config
	sm.logger.WithFields(logrus.Fields{
		"auth_enabled":    config.Enabled,
		"require_auth":    config.RequireAuth,
		"auth_methods":    config.Methods,
	}).Info("Updated authentication configuration")
}

// GetStats returns session manager statistics
func (sm *SessionManager) GetStats() map[string]interface{} {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	totalRequests := int64(0)
	totalErrors := int64(0)
	authenticatedSessions := 0

	for _, session := range sm.sessions {
		totalRequests += session.RequestCount
		totalErrors += session.ErrorCount
		if session.Authenticated {
			authenticatedSessions++
		}
	}

	return map[string]interface{}{
		"active_sessions":        len(sm.sessions),
		"authenticated_sessions": authenticatedSessions,
		"total_requests":         totalRequests,
		"total_errors":           totalErrors,
		"auth_enabled":           sm.authConfig.Enabled,
		"auth_required":          sm.authConfig.RequireAuth,
	}
}