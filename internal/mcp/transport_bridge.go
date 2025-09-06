package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sirupsen/logrus"

	"github.com/acmg-amp-mcp-server/internal/mcp/transport"
	"github.com/acmg-amp-mcp-server/internal/mcp/protocol"
	"github.com/acmg-amp-mcp-server/internal/mcp/tools"
)

// MCPTransportBridge bridges our custom transport interface with MCP SDK Transport
type MCPTransportBridge struct {
	customTransport transport.Transport
	logger          *logrus.Logger
}

// NewMCPTransportBridge creates a new transport bridge
func NewMCPTransportBridge(customTransport transport.Transport, logger *logrus.Logger) mcp.Transport {
	return &MCPTransportBridge{
		customTransport: customTransport,
		logger:          logger,
	}
}

// ReadMessage implements mcp.Transport interface
func (b *MCPTransportBridge) ReadMessage() ([]byte, error) {
	b.logger.Debug("Reading message through transport bridge")
	return b.customTransport.ReadMessage()
}

// WriteMessage implements mcp.Transport interface
func (b *MCPTransportBridge) WriteMessage(data []byte) error {
	b.logger.Debug("Writing message through transport bridge")
	return b.customTransport.WriteMessage(data)
}

// Close implements mcp.Transport interface
func (b *MCPTransportBridge) Close() error {
	b.logger.Debug("Closing transport through bridge")
	return b.customTransport.Close()
}

// ReadJSONMessage reads and unmarshals a JSON message
func (b *MCPTransportBridge) ReadJSONMessage(v interface{}) error {
	data, err := b.ReadMessage()
	if err != nil {
		if err == io.EOF {
			return err // Pass through EOF as-is for proper handling
		}
		return fmt.Errorf("failed to read message: %w", err)
	}
	
	if len(data) == 0 {
		return io.EOF
	}
	
	if err := json.Unmarshal(data, v); err != nil {
		b.logger.WithError(err).WithField("data", string(data)).Error("Failed to unmarshal JSON message")
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}
	
	return nil
}

// WriteJSONMessage marshals and writes a JSON message
func (b *MCPTransportBridge) WriteJSONMessage(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		b.logger.WithError(err).Error("Failed to marshal JSON message")
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	
	return b.WriteMessage(data)
}

// Start starts the underlying transport
func (b *MCPTransportBridge) Start(ctx context.Context) error {
	b.logger.Info("Starting transport bridge")
	return b.customTransport.Start(ctx)
}

// IsClosed returns whether the transport is closed
func (b *MCPTransportBridge) IsClosed() bool {
	return b.customTransport.IsClosed()
}

// MCPToolHandler bridges MCP SDK tool calls to our internal tool registry
type MCPToolHandler struct {
	toolRegistry *tools.ToolRegistry
	toolName     string
	logger       *logrus.Logger
}

// NewMCPToolHandler creates a new MCP tool handler
func NewMCPToolHandler(toolRegistry *tools.ToolRegistry, toolName string, logger *logrus.Logger) mcp.ToolHandler {
	return &MCPToolHandler{
		toolRegistry: toolRegistry,
		toolName:     toolName,
		logger:       logger,
	}
}

// Handle implements mcp.ToolHandler interface
func (h *MCPToolHandler) Handle(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	h.logger.WithField("tool", h.toolName).Debug("Handling MCP tool call")
	
	// Convert MCP call to our internal protocol
	req := &protocol.JSONRPC2Request{
		Method: h.toolName,
		Params: params,
	}
	
	// Execute through our tool registry
	response := h.toolRegistry.ExecuteTool(ctx, req)
	
	if response.Error != nil {
		return nil, fmt.Errorf("tool execution failed: %s", response.Error.Message)
	}
	
	return response.Result, nil
}