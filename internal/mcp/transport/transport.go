package transport

import (
	"context"
)

// Transport defines the interface for MCP transport mechanisms
type Transport interface {
	// Start initializes the transport
	Start(ctx context.Context) error
	
	// ReadMessage reads a message from the transport
	ReadMessage() ([]byte, error)
	
	// WriteMessage sends a message via the transport
	WriteMessage(message []byte) error
	
	// WriteJSONMessage sends a JSON object as a message
	WriteJSONMessage(obj interface{}) error
	
	// Close closes the transport and cleans up resources
	Close() error
	
	// IsClosed returns whether the transport is closed
	IsClosed() bool
	
	// GetType returns the transport type identifier
	GetType() string
}

// TransportType represents the type of transport
type TransportType string

const (
	TransportStdio   TransportType = "stdio"
	TransportHTTPSSE TransportType = "http-sse"
)

// TransportConfig holds configuration for transport creation
type TransportConfig struct {
	Type     TransportType `json:"transport_type"`
	HTTPHost string        `json:"http_host,omitempty"`
	HTTPPort int           `json:"http_port,omitempty"`
}

// ClientInfo represents information about a connected MCP client
type ClientInfo struct {
	ID            string            `json:"id"`
	TransportType string            `json:"transport_type"`
	ConnectedAt   string            `json:"connected_at"`
	LastActivity  string            `json:"last_activity"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}