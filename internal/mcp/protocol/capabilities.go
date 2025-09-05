package protocol

import (
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"
)

// CapabilityManager handles MCP capability negotiation and management
type CapabilityManager struct {
	logger               *logrus.Logger
	serverCapabilities   map[string]interface{}
	clientCapabilities   map[string]map[string]interface{}
	protocolVersions     []string
	mu                   sync.RWMutex
}

// ServerCapability represents a server capability
type ServerCapability struct {
	Name        string      `json:"name"`
	Version     string      `json:"version,omitempty"`
	Description string      `json:"description,omitempty"`
	Config      interface{} `json:"config,omitempty"`
}

// NewCapabilityManager creates a new capability manager
func NewCapabilityManager(logger *logrus.Logger) *CapabilityManager {
	cm := &CapabilityManager{
		logger:             logger,
		serverCapabilities: make(map[string]interface{}),
		clientCapabilities: make(map[string]map[string]interface{}),
		protocolVersions:   []string{"2025-01-01", "2024-11-05", "2024-09-25"},
	}
	
	// Initialize default server capabilities
	cm.initializeServerCapabilities()
	
	return cm
}

// initializeServerCapabilities sets up the default server capabilities
func (cm *CapabilityManager) initializeServerCapabilities() {
	cm.serverCapabilities = map[string]interface{}{
		"protocol": map[string]interface{}{
			"supportedVersions": cm.protocolVersions,
			"preferredVersion":  cm.protocolVersions[0], // Latest version
		},
		"tools": map[string]interface{}{
			"listChanged":    true,
			"supportsSchema": true,
			"capabilities": []string{
				"classify_variant",
				"validate_hgvs", 
				"apply_rule",
				"combine_evidence",
			},
		},
		"resources": map[string]interface{}{
			"subscribe":      true,
			"listChanged":    true,
			"supportsSchema": true,
			"capabilities": []string{
				"variant",
				"interpretation",
				"evidence",
				"acmg_rules",
			},
		},
		"prompts": map[string]interface{}{
			"listChanged":    true,
			"supportsSchema": true,
			"capabilities": []string{
				"classification_workflow",
				"evidence_analysis",
				"report_generation",
			},
		},
		"logging": map[string]interface{}{
			"enabled":      true,
			"level":        "info",
			"capabilities": []string{"structured", "contextual"},
		},
		"sampling": map[string]interface{}{
			"enabled": false, // Disabled by default for clinical data
		},
		"experimental": map[string]interface{}{
			"batchProcessing": true,
			"streamingTools":  false,
		},
	}

	cm.logger.Info("Initialized server capabilities")
}

// NegotiateCapabilities performs capability negotiation with a client
func (cm *CapabilityManager) NegotiateCapabilities(clientID string, clientCaps map[string]interface{}) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.logger.WithField("client_id", clientID).Info("Starting capability negotiation")

	// Store client capabilities
	cm.clientCapabilities[clientID] = clientCaps

	// Validate protocol version compatibility
	if err := cm.validateProtocolVersion(clientCaps); err != nil {
		return fmt.Errorf("protocol version negotiation failed: %w", err)
	}

	// Negotiate tool capabilities
	if err := cm.negotiateToolCapabilities(clientID, clientCaps); err != nil {
		return fmt.Errorf("tool capability negotiation failed: %w", err)
	}

	// Negotiate resource capabilities
	if err := cm.negotiateResourceCapabilities(clientID, clientCaps); err != nil {
		return fmt.Errorf("resource capability negotiation failed: %w", err)
	}

	// Negotiate prompt capabilities
	if err := cm.negotiatePromptCapabilities(clientID, clientCaps); err != nil {
		return fmt.Errorf("prompt capability negotiation failed: %w", err)
	}

	cm.logger.WithField("client_id", clientID).Info("Capability negotiation completed successfully")
	return nil
}

// validateProtocolVersion ensures client and server support compatible protocol versions
func (cm *CapabilityManager) validateProtocolVersion(clientCaps map[string]interface{}) error {
	clientProtocol, exists := clientCaps["protocol"]
	if !exists {
		// Default to oldest supported version if not specified
		cm.logger.Warn("Client did not specify protocol capabilities, using default")
		return nil
	}

	protocolMap, ok := clientProtocol.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid protocol capability format")
	}

	// Check if client supports any of our protocol versions
	clientVersionsRaw, exists := protocolMap["supportedVersions"]
	if !exists {
		// Check for single version
		if version, exists := protocolMap["version"]; exists {
			if versionStr, ok := version.(string); ok {
				clientVersionsRaw = []interface{}{versionStr}
			}
		}
	}

	clientVersions, ok := clientVersionsRaw.([]interface{})
	if !ok {
		return fmt.Errorf("invalid client protocol versions format")
	}

	// Find common protocol version
	for _, serverVersion := range cm.protocolVersions {
		for _, clientVersionRaw := range clientVersions {
			if clientVersion, ok := clientVersionRaw.(string); ok && clientVersion == serverVersion {
				cm.logger.WithFields(logrus.Fields{
					"negotiated_version": serverVersion,
					"server_versions":    cm.protocolVersions,
					"client_versions":    clientVersions,
				}).Info("Protocol version negotiated")
				return nil
			}
		}
	}

	return fmt.Errorf("no compatible protocol version found. Server supports: %v, Client supports: %v", 
		cm.protocolVersions, clientVersions)
}

// negotiateToolCapabilities negotiates tool-related capabilities
func (cm *CapabilityManager) negotiateToolCapabilities(clientID string, clientCaps map[string]interface{}) error {
	clientTools, exists := clientCaps["tools"]
	if !exists {
		cm.logger.WithField("client_id", clientID).Debug("Client does not specify tool capabilities")
		return nil
	}

	toolsMap, ok := clientTools.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid tool capabilities format")
	}

	// Check if client supports schema validation
	if supportsSchema, exists := toolsMap["supportsSchema"]; exists {
		if supports, ok := supportsSchema.(bool); ok && supports {
			cm.logger.WithField("client_id", clientID).Debug("Client supports tool schema validation")
		}
	}

	// Check if client can handle tool list changes
	if listChanged, exists := toolsMap["listChanged"]; exists {
		if supports, ok := listChanged.(bool); ok && supports {
			cm.logger.WithField("client_id", clientID).Debug("Client supports dynamic tool list changes")
		}
	}

	return nil
}

// negotiateResourceCapabilities negotiates resource-related capabilities
func (cm *CapabilityManager) negotiateResourceCapabilities(clientID string, clientCaps map[string]interface{}) error {
	clientResources, exists := clientCaps["resources"]
	if !exists {
		cm.logger.WithField("client_id", clientID).Debug("Client does not specify resource capabilities")
		return nil
	}

	resourcesMap, ok := clientResources.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid resource capabilities format")
	}

	// Check subscription support
	if subscribe, exists := resourcesMap["subscribe"]; exists {
		if supports, ok := subscribe.(bool); ok && supports {
			cm.logger.WithField("client_id", clientID).Debug("Client supports resource subscriptions")
		}
	}

	return nil
}

// negotiatePromptCapabilities negotiates prompt-related capabilities
func (cm *CapabilityManager) negotiatePromptCapabilities(clientID string, clientCaps map[string]interface{}) error {
	clientPrompts, exists := clientCaps["prompts"]
	if !exists {
		cm.logger.WithField("client_id", clientID).Debug("Client does not specify prompt capabilities")
		return nil
	}

	promptsMap, ok := clientPrompts.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid prompt capabilities format")
	}

	// Check if client supports schema validation for prompts
	if supportsSchema, exists := promptsMap["supportsSchema"]; exists {
		if supports, ok := supportsSchema.(bool); ok && supports {
			cm.logger.WithField("client_id", clientID).Debug("Client supports prompt schema validation")
		}
	}

	return nil
}

// GetCapabilities returns the server capabilities
func (cm *CapabilityManager) GetCapabilities() map[string]interface{} {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	// Deep copy to prevent external modification
	capabilities := make(map[string]interface{})
	for k, v := range cm.serverCapabilities {
		capabilities[k] = v
	}
	
	return capabilities
}

// GetClientCapabilities returns capabilities for a specific client
func (cm *CapabilityManager) GetClientCapabilities(clientID string) map[string]interface{} {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	if caps, exists := cm.clientCapabilities[clientID]; exists {
		// Deep copy to prevent external modification
		clientCaps := make(map[string]interface{})
		for k, v := range caps {
			clientCaps[k] = v
		}
		return clientCaps
	}
	
	return nil
}

// RemoveClient removes a client's capability information
func (cm *CapabilityManager) RemoveClient(clientID string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	delete(cm.clientCapabilities, clientID)
	cm.logger.WithField("client_id", clientID).Debug("Removed client capability information")
}

// UpdateServerCapability updates or adds a server capability
func (cm *CapabilityManager) UpdateServerCapability(capability string, config interface{}) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	cm.serverCapabilities[capability] = config
	cm.logger.WithField("capability", capability).Debug("Updated server capability")
}

// GetStats returns capability manager statistics
func (cm *CapabilityManager) GetStats() map[string]interface{} {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	return map[string]interface{}{
		"active_clients":        len(cm.clientCapabilities),
		"server_capabilities":   len(cm.serverCapabilities),
		"supported_versions":    cm.protocolVersions,
		"preferred_version":     cm.protocolVersions[0],
	}
}