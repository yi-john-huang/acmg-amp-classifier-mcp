package transport

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/acmg-amp-mcp-server/internal/domain"
)

// Manager handles transport creation, auto-detection, and lifecycle management
type Manager struct {
	logger    *logrus.Logger
	config    *domain.MCPConfig
	transport Transport
	clients   map[string]*ClientInfo
	clientsMu sync.RWMutex
	mu        sync.RWMutex
}

// NewManager creates a new transport manager
func NewManager(logger *logrus.Logger, config *domain.MCPConfig) *Manager {
	return &Manager{
		logger:  logger,
		config:  config,
		clients: make(map[string]*ClientInfo),
	}
}

// AutoDetectTransport automatically detects the appropriate transport type
func (m *Manager) AutoDetectTransport() (TransportType, error) {
	m.logger.Debug("Auto-detecting MCP transport type")

	// Check command line arguments first
	if len(os.Args) > 1 {
		for _, arg := range os.Args[1:] {
			switch arg {
			case "--stdio", "-stdio":
				m.logger.Info("Detected stdio transport via command line argument")
				return TransportStdio, nil
			case "--http", "-http":
				m.logger.Info("Detected HTTP transport via command line argument")
				return TransportHTTPSSE, nil
			}
		}
	}

	// Check environment variables
	if transportType := os.Getenv("MCP_TRANSPORT"); transportType != "" {
		switch transportType {
		case "stdio":
			m.logger.Info("Detected stdio transport via MCP_TRANSPORT environment variable")
			return TransportStdio, nil
		case "http", "http-sse":
			m.logger.Info("Detected HTTP SSE transport via MCP_TRANSPORT environment variable")
			return TransportHTTPSSE, nil
		default:
			m.logger.WithField("transport_type", transportType).Warn("Unknown transport type in MCP_TRANSPORT")
		}
	}

	// Check configuration
	if m.config != nil && m.config.TransportType != "" {
		switch m.config.TransportType {
		case "stdio":
			m.logger.Info("Using stdio transport from configuration")
			return TransportStdio, nil
		case "http", "http-sse":
			m.logger.Info("Using HTTP SSE transport from configuration")
			return TransportHTTPSSE, nil
		default:
			m.logger.WithField("transport_type", m.config.TransportType).Warn("Unknown transport type in configuration")
		}
	}

	// Check if running in a terminal (stdio is likely)
	if isTerminal() {
		m.logger.Info("Detected terminal environment, defaulting to stdio transport")
		return TransportStdio, nil
	}

	// Default to stdio for MCP servers
	m.logger.Info("No specific transport detected, defaulting to stdio")
	return TransportStdio, nil
}

// CreateTransport creates a transport instance based on the specified type
func (m *Manager) CreateTransport(transportType TransportType) (Transport, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch transportType {
	case TransportStdio:
		m.logger.Info("Creating stdio transport")
		return NewStdioTransport(m.logger), nil
	
	case TransportHTTPSSE:
		host := "localhost"
		port := 8080
		
		// Get host/port from config or environment
		if m.config != nil {
			if m.config.HTTPHost != "" {
				host = m.config.HTTPHost
			}
			if m.config.HTTPPort > 0 {
				port = m.config.HTTPPort
			}
		}
		
		if envPort := os.Getenv("MCP_HTTP_PORT"); envPort != "" {
			if p, err := strconv.Atoi(envPort); err == nil {
				port = p
			}
		}
		
		if envHost := os.Getenv("MCP_HTTP_HOST"); envHost != "" {
			host = envHost
		}
		
		m.logger.WithFields(logrus.Fields{
			"host": host,
			"port": port,
		}).Info("Creating HTTP SSE transport")
		
		return NewHTTPSSETransport(m.logger, host, port), nil
	
	default:
		return nil, fmt.Errorf("unsupported transport type: %s", transportType)
	}
}

// StartTransport auto-detects and starts the appropriate transport
func (m *Manager) StartTransport(ctx context.Context) (Transport, error) {
	// Auto-detect transport type
	transportType, err := m.AutoDetectTransport()
	if err != nil {
		return nil, fmt.Errorf("failed to detect transport: %w", err)
	}

	// Create transport
	transport, err := m.CreateTransport(transportType)
	if err != nil {
		return nil, fmt.Errorf("failed to create transport: %w", err)
	}

	// Start transport
	if err := transport.Start(ctx); err != nil {
		transport.Close()
		return nil, fmt.Errorf("failed to start transport: %w", err)
	}

	m.transport = transport
	m.logger.WithField("transport_type", transport.GetType()).Info("Transport started successfully")
	
	return transport, nil
}

// RegisterClient registers a new MCP client connection
func (m *Manager) RegisterClient(clientID string, transportType string, metadata map[string]string) {
	m.clientsMu.Lock()
	defer m.clientsMu.Unlock()

	now := time.Now().Format(time.RFC3339)
	client := &ClientInfo{
		ID:            clientID,
		TransportType: transportType,
		ConnectedAt:   now,
		LastActivity:  now,
		Metadata:      metadata,
	}

	m.clients[clientID] = client
	m.logger.WithFields(logrus.Fields{
		"client_id":      clientID,
		"transport_type": transportType,
	}).Info("MCP client registered")
}

// UnregisterClient removes a client registration
func (m *Manager) UnregisterClient(clientID string) {
	m.clientsMu.Lock()
	defer m.clientsMu.Unlock()

	if client, exists := m.clients[clientID]; exists {
		delete(m.clients, clientID)
		m.logger.WithFields(logrus.Fields{
			"client_id":      clientID,
			"transport_type": client.TransportType,
			"duration":       time.Since(parseTime(client.ConnectedAt)).String(),
		}).Info("MCP client unregistered")
	}
}

// UpdateClientActivity updates the last activity time for a client
func (m *Manager) UpdateClientActivity(clientID string) {
	m.clientsMu.Lock()
	defer m.clientsMu.Unlock()

	if client, exists := m.clients[clientID]; exists {
		client.LastActivity = time.Now().Format(time.RFC3339)
	}
}

// GetClients returns information about all registered clients
func (m *Manager) GetClients() []ClientInfo {
	m.clientsMu.RLock()
	defer m.clientsMu.RUnlock()

	clients := make([]ClientInfo, 0, len(m.clients))
	for _, client := range m.clients {
		clients = append(clients, *client)
	}
	return clients
}

// GetActiveTransport returns the currently active transport
func (m *Manager) GetActiveTransport() Transport {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.transport
}

// Shutdown gracefully shuts down all transports and client connections
func (m *Manager) Shutdown(ctx context.Context) error {
	m.logger.Info("Shutting down transport manager")

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.transport != nil {
		if err := m.transport.Close(); err != nil {
			m.logger.WithError(err).Error("Error closing transport")
			return err
		}
		m.transport = nil
	}

	// Clear all client registrations
	m.clientsMu.Lock()
	m.clients = make(map[string]*ClientInfo)
	m.clientsMu.Unlock()

	m.logger.Info("Transport manager shutdown complete")
	return nil
}

// isTerminal checks if the process is running in a terminal
func isTerminal() bool {
	// Simple heuristic: check if stdin is a terminal
	stat, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	
	// If stdin is a character device (terminal), not a pipe or regular file
	return (stat.Mode() & os.ModeCharDevice) != 0
}

// parseTime parses RFC3339 time string
func parseTime(timeStr string) time.Time {
	t, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		return time.Now()
	}
	return t
}