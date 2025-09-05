package protocol

import (
	"context"
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"
)

// MessageRouter routes MCP messages to appropriate handlers based on message type
type MessageRouter struct {
	logger         *logrus.Logger
	toolHandlers   map[string]ToolHandler
	resourceHandlers map[string]ResourceHandler  
	promptHandlers map[string]PromptHandler
	systemHandlers map[string]SystemHandler
	mu             sync.RWMutex
}

// ToolHandler defines the interface for MCP tool handlers
type ToolHandler interface {
	HandleTool(ctx context.Context, req *JSONRPC2Request) *JSONRPC2Response
	GetToolInfo() ToolInfo
	ValidateParams(params interface{}) error
}

// ResourceHandler defines the interface for MCP resource handlers
type ResourceHandler interface {
	HandleResource(ctx context.Context, req *JSONRPC2Request) *JSONRPC2Response
	GetResourceInfo() ResourceInfo
	ValidateURI(uri string) error
}

// PromptHandler defines the interface for MCP prompt handlers
type PromptHandler interface {
	HandlePrompt(ctx context.Context, req *JSONRPC2Request) *JSONRPC2Response
	GetPromptInfo() PromptInfo
	ValidateParams(params interface{}) error
}

// SystemHandler defines the interface for MCP system handlers
type SystemHandler interface {
	HandleSystem(ctx context.Context, req *JSONRPC2Request) *JSONRPC2Response
	GetSystemInfo() SystemInfo
}

// ToolInfo contains metadata about a tool
type ToolInfo struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema,omitempty"`
}

// ResourceInfo contains metadata about a resource
type ResourceInfo struct {
	URI         string `json:"uri"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// PromptInfo contains metadata about a prompt
type PromptInfo struct {
	Name        string                   `json:"name"`
	Description string                   `json:"description"`
	Arguments   []PromptArgument         `json:"arguments,omitempty"`
}

// PromptArgument defines a prompt argument
type PromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// SystemInfo contains metadata about system handlers
type SystemInfo struct {
	Method      string `json:"method"`
	Description string `json:"description"`
}

// NewMessageRouter creates a new message router
func NewMessageRouter(logger *logrus.Logger) *MessageRouter {
	router := &MessageRouter{
		logger:           logger,
		toolHandlers:     make(map[string]ToolHandler),
		resourceHandlers: make(map[string]ResourceHandler),
		promptHandlers:   make(map[string]PromptHandler),
		systemHandlers:   make(map[string]SystemHandler),
	}

	// Register built-in system handlers
	router.registerSystemHandlers()

	return router
}

// registerSystemHandlers registers built-in MCP system message handlers
func (mr *MessageRouter) registerSystemHandlers() {
	// Initialize system handler
	mr.systemHandlers["initialize"] = &InitializeHandler{logger: mr.logger}
	
	// Tools list handler
	mr.systemHandlers["tools/list"] = &ToolsListHandler{
		logger: mr.logger,
		router: mr,
	}
	
	// Tools call handler
	mr.systemHandlers["tools/call"] = &ToolsCallHandler{
		logger: mr.logger,
		router: mr,
	}

	// Resources list handler
	mr.systemHandlers["resources/list"] = &ResourcesListHandler{
		logger: mr.logger,
		router: mr,
	}
	
	// Resources read handler
	mr.systemHandlers["resources/read"] = &ResourcesReadHandler{
		logger: mr.logger,
		router: mr,
	}

	// Prompts list handler
	mr.systemHandlers["prompts/list"] = &PromptsListHandler{
		logger: mr.logger,
		router: mr,
	}
	
	// Prompts get handler
	mr.systemHandlers["prompts/get"] = &PromptsGetHandler{
		logger: mr.logger,
		router: mr,
	}

	mr.logger.Debug("Registered system message handlers")
}

// HandleRequest implements MessageHandler interface for routing messages
func (mr *MessageRouter) HandleRequest(ctx context.Context, req *JSONRPC2Request) *JSONRPC2Response {
	mr.logger.WithField("method", req.Method).Debug("Routing message")

	// First check system handlers
	mr.mu.RLock()
	if handler, exists := mr.systemHandlers[req.Method]; exists {
		mr.mu.RUnlock()
		return handler.HandleSystem(ctx, req)
	}
	mr.mu.RUnlock()

	// If no system handler found, return method not found
	return &JSONRPC2Response{
		Error: &RPCError{
			Code:    MethodNotFound,
			Message: "Method not found",
			Data:    fmt.Sprintf("No handler found for method: %s", req.Method),
		},
	}
}

// GetSupportedMethods returns all supported methods
func (mr *MessageRouter) GetSupportedMethods() []string {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	methods := make([]string, 0)
	
	// Add system methods
	for method := range mr.systemHandlers {
		methods = append(methods, method)
	}
	
	return methods
}

// RegisterToolHandler registers a tool handler
func (mr *MessageRouter) RegisterToolHandler(name string, handler ToolHandler) {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	mr.toolHandlers[name] = handler
	mr.logger.WithField("tool_name", name).Debug("Registered tool handler")
}

// RegisterResourceHandler registers a resource handler
func (mr *MessageRouter) RegisterResourceHandler(uriPattern string, handler ResourceHandler) {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	mr.resourceHandlers[uriPattern] = handler
	mr.logger.WithField("uri_pattern", uriPattern).Debug("Registered resource handler")
}

// RegisterPromptHandler registers a prompt handler
func (mr *MessageRouter) RegisterPromptHandler(name string, handler PromptHandler) {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	mr.promptHandlers[name] = handler
	mr.logger.WithField("prompt_name", name).Debug("Registered prompt handler")
}

// GetToolHandlers returns all registered tool handlers
func (mr *MessageRouter) GetToolHandlers() map[string]ToolHandler {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	handlers := make(map[string]ToolHandler)
	for name, handler := range mr.toolHandlers {
		handlers[name] = handler
	}
	return handlers
}

// GetResourceHandlers returns all registered resource handlers  
func (mr *MessageRouter) GetResourceHandlers() map[string]ResourceHandler {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	handlers := make(map[string]ResourceHandler)
	for pattern, handler := range mr.resourceHandlers {
		handlers[pattern] = handler
	}
	return handlers
}

// GetPromptHandlers returns all registered prompt handlers
func (mr *MessageRouter) GetPromptHandlers() map[string]PromptHandler {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	handlers := make(map[string]PromptHandler)
	for name, handler := range mr.promptHandlers {
		handlers[name] = handler
	}
	return handlers
}

// GetToolHandler retrieves a specific tool handler
func (mr *MessageRouter) GetToolHandler(name string) (ToolHandler, bool) {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	handler, exists := mr.toolHandlers[name]
	return handler, exists
}

// GetResourceHandler retrieves a resource handler by URI pattern matching
func (mr *MessageRouter) GetResourceHandler(uri string) (ResourceHandler, bool) {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	// Simple exact match for now - could be enhanced with pattern matching
	for pattern, handler := range mr.resourceHandlers {
		if mr.matchesPattern(uri, pattern) {
			return handler, true
		}
	}
	return nil, false
}

// matchesPattern checks if a URI matches a pattern (simple implementation)
func (mr *MessageRouter) matchesPattern(uri, pattern string) bool {
	// For now, just do exact match - could be enhanced with regex or glob patterns
	return uri == pattern
}

// GetPromptHandler retrieves a specific prompt handler
func (mr *MessageRouter) GetPromptHandler(name string) (PromptHandler, bool) {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	handler, exists := mr.promptHandlers[name]
	return handler, exists
}

// GetStats returns router statistics
func (mr *MessageRouter) GetStats() map[string]interface{} {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	return map[string]interface{}{
		"registered_tools":     len(mr.toolHandlers),
		"registered_resources": len(mr.resourceHandlers),
		"registered_prompts":   len(mr.promptHandlers),
		"system_handlers":      len(mr.systemHandlers),
		"total_handlers":       len(mr.toolHandlers) + len(mr.resourceHandlers) + len(mr.promptHandlers) + len(mr.systemHandlers),
	}
}