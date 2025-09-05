package protocol

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// JSONRPC2Request represents a JSON-RPC 2.0 request message
type JSONRPC2Request struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
	ID      interface{} `json:"id,omitempty"`
}

// JSONRPC2Response represents a JSON-RPC 2.0 response message
type JSONRPC2Response struct {
	JSONRPC string      `json:"jsonrpc"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
	ID      interface{} `json:"id"`
}

// RPCError represents a JSON-RPC 2.0 error object
type RPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Standard JSON-RPC 2.0 error codes
const (
	ParseError     = -32700
	InvalidRequest = -32600
	MethodNotFound = -32601
	InvalidParams  = -32602
	InternalError  = -32603
	
	// MCP-specific error codes
	MCPUnauthorized   = -32000
	MCPRateLimited    = -32001
	MCPResourceError  = -32002
	MCPToolError      = -32003
)

// MessageHandler defines the interface for handling JSON-RPC messages
type MessageHandler interface {
	HandleRequest(ctx context.Context, req *JSONRPC2Request) *JSONRPC2Response
	GetSupportedMethods() []string
}

// ProtocolCore handles JSON-RPC 2.0 protocol operations
type ProtocolCore struct {
	logger        *logrus.Logger
	handlers      map[string]MessageHandler
	sessionMgr    *SessionManager
	rateLimiter   *RateLimiter
	capabilities  *CapabilityManager
	mu            sync.RWMutex
}

// NewProtocolCore creates a new JSON-RPC 2.0 protocol core
func NewProtocolCore(logger *logrus.Logger) *ProtocolCore {
	return &ProtocolCore{
		logger:       logger,
		handlers:     make(map[string]MessageHandler),
		sessionMgr:   NewSessionManager(logger),
		rateLimiter:  NewRateLimiter(logger),
		capabilities: NewCapabilityManager(logger),
	}
}

// RegisterHandler registers a message handler for specific methods
func (p *ProtocolCore) RegisterHandler(method string, handler MessageHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	p.handlers[method] = handler
	p.logger.WithField("method", method).Debug("Registered JSON-RPC handler")
}

// ProcessMessage processes an incoming JSON-RPC 2.0 message
func (p *ProtocolCore) ProcessMessage(ctx context.Context, clientID string, rawMessage []byte) ([]byte, error) {
	p.logger.WithFields(logrus.Fields{
		"client_id":      clientID,
		"message_length": len(rawMessage),
	}).Debug("Processing JSON-RPC message")

	// Parse JSON-RPC request
	var req JSONRPC2Request
	if err := json.Unmarshal(rawMessage, &req); err != nil {
		response := &JSONRPC2Response{
			JSONRPC: "2.0",
			Error: &RPCError{
				Code:    ParseError,
				Message: "Parse error",
				Data:    err.Error(),
			},
			ID: nil,
		}
		return json.Marshal(response)
	}

	// Validate JSON-RPC 2.0 format
	if req.JSONRPC != "2.0" {
		response := &JSONRPC2Response{
			JSONRPC: "2.0",
			Error: &RPCError{
				Code:    InvalidRequest,
				Message: "Invalid Request",
				Data:    "JSON-RPC version must be 2.0",
			},
			ID: req.ID,
		}
		return json.Marshal(response)
	}

	// Check rate limiting
	if !p.rateLimiter.AllowRequest(clientID) {
		response := &JSONRPC2Response{
			JSONRPC: "2.0",
			Error: &RPCError{
				Code:    MCPRateLimited,
				Message: "Rate limit exceeded",
				Data:    fmt.Sprintf("Client %s has exceeded rate limit", clientID),
			},
			ID: req.ID,
		}
		return json.Marshal(response)
	}

	// Update client activity
	p.sessionMgr.UpdateClientActivity(clientID)

	// Handle the request
	response := p.handleRequest(ctx, &req)
	
	return json.Marshal(response)
}

// handleRequest processes a validated JSON-RPC request
func (p *ProtocolCore) handleRequest(ctx context.Context, req *JSONRPC2Request) *JSONRPC2Response {
	p.mu.RLock()
	handler, exists := p.handlers[req.Method]
	p.mu.RUnlock()

	if !exists {
		return &JSONRPC2Response{
			JSONRPC: "2.0",
			Error: &RPCError{
				Code:    MethodNotFound,
				Message: "Method not found",
				Data:    fmt.Sprintf("Method '%s' not found", req.Method),
			},
			ID: req.ID,
		}
	}

	// Delegate to specific handler
	response := handler.HandleRequest(ctx, req)
	response.JSONRPC = "2.0"
	response.ID = req.ID

	return response
}

// GetCapabilities returns current protocol capabilities
func (p *ProtocolCore) GetCapabilities() map[string]interface{} {
	return p.capabilities.GetCapabilities()
}

// InitializeClient performs initial client setup and capability negotiation
func (p *ProtocolCore) InitializeClient(clientID string, capabilities map[string]interface{}) error {
	p.logger.WithField("client_id", clientID).Info("Initializing MCP client")
	
	// Create client session
	if err := p.sessionMgr.CreateSession(clientID, capabilities); err != nil {
		return fmt.Errorf("failed to create client session: %w", err)
	}

	// Initialize rate limiter for client
	p.rateLimiter.InitializeClient(clientID)

	// Perform capability negotiation
	if err := p.capabilities.NegotiateCapabilities(clientID, capabilities); err != nil {
		p.sessionMgr.RemoveSession(clientID)
		return fmt.Errorf("capability negotiation failed: %w", err)
	}

	p.logger.WithField("client_id", clientID).Info("MCP client initialized successfully")
	return nil
}

// CleanupClient removes client session and associated resources
func (p *ProtocolCore) CleanupClient(clientID string) {
	p.logger.WithField("client_id", clientID).Info("Cleaning up MCP client")
	
	p.sessionMgr.RemoveSession(clientID)
	p.rateLimiter.RemoveClient(clientID)
	p.capabilities.RemoveClient(clientID)
	
	p.logger.WithField("client_id", clientID).Info("MCP client cleanup complete")
}

// GetStats returns protocol statistics
func (p *ProtocolCore) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"active_sessions":   p.sessionMgr.GetSessionCount(),
		"registered_methods": len(p.handlers),
		"rate_limit_stats":   p.rateLimiter.GetStats(),
		"capability_stats":   p.capabilities.GetStats(),
		"uptime":            time.Now().Format(time.RFC3339),
	}
}