package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
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

// Connect implements the mcp.Transport interface
func (b *MCPTransportBridge) Connect(ctx context.Context) (mcp.Connection, error) {
	b.logger.Debug("Connecting through transport bridge")
	
	// Start our custom transport
	if err := b.customTransport.Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to start custom transport: %w", err)
	}
	
	// Return a connection that bridges our transport to MCP SDK
	return &MCPConnectionBridge{
		customTransport: b.customTransport,
		logger:          b.logger,
	}, nil
}

// MCPConnectionBridge bridges our custom transport to MCP SDK Connection
type MCPConnectionBridge struct {
	customTransport transport.Transport
	logger          *logrus.Logger
}

// Read implements the mcp.Connection interface
func (c *MCPConnectionBridge) Read(ctx context.Context) (jsonrpc.Message, error) {
	c.logger.Debug("Reading message through connection bridge")
	
	// Read raw bytes from our custom transport
	data, err := c.customTransport.ReadMessage()
	if err != nil {
		if err == io.EOF {
			return nil, err // Pass through EOF as-is
		}
		return nil, fmt.Errorf("failed to read from custom transport: %w", err)
	}
	
	if len(data) == 0 {
		return nil, io.EOF
	}
	
	// Parse JSON-RPC message
	var rawMsg json.RawMessage = data
	msg, err := parseJSONRPCMessage(rawMsg)
	if err != nil {
		c.logger.WithError(err).WithField("data", string(data)).Error("Failed to parse JSON-RPC message")
		return nil, fmt.Errorf("failed to parse JSON-RPC message: %w", err)
	}
	
	return msg, nil
}

// Write implements the mcp.Connection interface  
func (c *MCPConnectionBridge) Write(ctx context.Context, msg jsonrpc.Message) error {
	c.logger.Debug("Writing message through connection bridge")
	
	// Encode JSON-RPC message to bytes
	data, err := json.Marshal(msg)
	if err != nil {
		c.logger.WithError(err).Error("Failed to marshal JSON-RPC message")
		return fmt.Errorf("failed to marshal JSON-RPC message: %w", err)
	}
	
	// Write to our custom transport
	return c.customTransport.WriteMessage(data)
}

// Close implements the mcp.Connection interface
func (c *MCPConnectionBridge) Close() error {
	c.logger.Debug("Closing connection through bridge")
	return c.customTransport.Close()
}

// SessionID implements the mcp.Connection interface
func (c *MCPConnectionBridge) SessionID() string {
	// Generate or return a session ID - for now return a simple identifier
	return "acmg-amp-session"
}

// parseJSONRPCMessage parses raw JSON into a jsonrpc.Message
func parseJSONRPCMessage(raw json.RawMessage) (jsonrpc.Message, error) {
	// First try to determine if it's a request, response, or notification
	var base struct {
		JSONRPC string          `json:"jsonrpc"`
		Method  string          `json:"method,omitempty"`
		ID      json.RawMessage `json:"id,omitempty"`
		Result  json.RawMessage `json:"result,omitempty"`
		Error   json.RawMessage `json:"error,omitempty"`
	}
	
	if err := json.Unmarshal(raw, &base); err != nil {
		return nil, fmt.Errorf("invalid JSON-RPC message: %w", err)
	}
	
	// If it has a method, it's a request or notification
	if base.Method != "" {
		var req jsonrpc.Request
		if err := json.Unmarshal(raw, &req); err != nil {
			return nil, fmt.Errorf("invalid JSON-RPC request: %w", err)
		}
		return &req, nil
	}
	
	// Otherwise, it's a response
	var resp jsonrpc.Response
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("invalid JSON-RPC response: %w", err)
	}
	return &resp, nil
}

// NewMCPToolHandler creates a new MCP tool handler function
func NewMCPToolHandler(toolRegistry *tools.ToolRegistry, toolName string, logger *logrus.Logger) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger.WithField("tool", toolName).Debug("Handling MCP tool call")
		
		// Convert MCP call to our internal protocol
		internalReq := &protocol.JSONRPC2Request{
			Method: toolName,
			Params: req.Params.Arguments,
		}
		
		// Execute through our tool registry
		response := toolRegistry.ExecuteTool(ctx, internalReq)
		
		// Convert internal response to MCP CallToolResult
		var result *mcp.CallToolResult
		
		if response.Error != nil {
			// Return error as tool result with isError flag
			result = &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{
						Text: fmt.Sprintf("Tool execution failed: %s", response.Error.Message),
					},
				},
				IsError: true,
			}
		} else {
			// Convert successful result
			result = &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{
						Text: fmt.Sprintf("Tool %s executed successfully", toolName),
					},
				},
				StructuredContent: response.Result,
			}
		}
		
		return result, nil
	}
}