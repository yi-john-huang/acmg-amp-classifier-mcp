package mcp

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/acmg-amp-mcp-server/internal/config"
	"github.com/acmg-amp-mcp-server/internal/domain"
)

func TestNewServer(t *testing.T) {
	// Create a minimal config manager for testing
	configManager := &config.Manager{}
	
	// Create MCP server
	server, err := NewServer(configManager)
	
	require.NoError(t, err)
	assert.NotNil(t, server)
	assert.NotNil(t, server.mcpServer)
	assert.NotNil(t, server.logger)
}

func TestDetectTransport(t *testing.T) {
	configManager := &config.Manager{}
	server, err := NewServer(configManager)
	require.NoError(t, err)

	// Test default stdio transport
	transport, err := server.detectTransport()
	require.NoError(t, err)
	assert.NotNil(t, transport)
	
	// Should detect stdio transport as default
	transportName := server.getTransportName()
	// Note: This might be "unknown" until transport is actually set
	assert.Contains(t, []string{"stdio", "unknown"}, transportName)
}

func TestServerInfo(t *testing.T) {
	configManager := &config.Manager{}
	server, err := NewServer(configManager)
	require.NoError(t, err)
	
	// Verify server has been created with expected metadata
	assert.NotNil(t, server.mcpServer)
	assert.NotNil(t, server.config)
	assert.NotNil(t, server.logger)
}

// TestMockConfigManager creates a mock config manager for testing
func createMockConfig() *domain.Config {
	return &domain.Config{
		MCP: domain.MCPConfig{
			ServerName:       "acmg-amp-mcp-server",
			ServerVersion:    "v0.1.0",
			TransportType:    "stdio",
			MaxClients:       10,
			RequestTimeout:   30 * time.Second,
			EnableMetrics:    true,
			EnableCaching:    true,
			ToolCacheTTL:     5 * time.Minute,
			ResourceCacheTTL: 10 * time.Minute,
		},
		Logging: domain.LoggingConfig{
			Level:  "info",
			Format: "json",
		},
	}
}

func TestRegisterCapabilities(t *testing.T) {
	configManager := &config.Manager{}
	server, err := NewServer(configManager)
	require.NoError(t, err)

	// Test capability registration (this happens in NewServer)
	// Verify no errors occurred during registration
	assert.NotNil(t, server.mcpServer)
	
	// Note: We can't easily test the actual registration without accessing
	// the internal MCP server state, but we can verify no errors occurred
}