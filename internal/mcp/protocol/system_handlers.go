package protocol

import (
	"context"
	"encoding/json"

	"github.com/sirupsen/logrus"
)

// InitializeHandler handles the MCP initialize request
type InitializeHandler struct {
	logger *logrus.Logger
}

// HandleSystem implements the initialize handler
func (h *InitializeHandler) HandleSystem(ctx context.Context, req *JSONRPC2Request) *JSONRPC2Response {
	h.logger.Info("Handling MCP initialize request")

	// Parse initialize parameters
	var params map[string]interface{}
	if req.Params != nil {
		if paramsMap, ok := req.Params.(map[string]interface{}); ok {
			params = paramsMap
		}
	}

	// Extract client info
	clientInfo := map[string]interface{}{
		"name":    "unknown",
		"version": "unknown",
	}

	if params != nil {
		if clientName, exists := params["clientInfo"]; exists {
			if clientMap, ok := clientName.(map[string]interface{}); ok {
				clientInfo = clientMap
			}
		}
	}

	h.logger.WithFields(logrus.Fields{
		"client_name":    clientInfo["name"],
		"client_version": clientInfo["version"],
	}).Info("MCP client initialized")

	// Return server capabilities
	return &JSONRPC2Response{
		Result: map[string]interface{}{
			"protocolVersion": "2025-01-01",
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{
					"listChanged": true,
				},
				"resources": map[string]interface{}{
					"subscribe":   true,
					"listChanged": true,
				},
				"prompts": map[string]interface{}{
					"listChanged": true,
				},
				"logging": map[string]interface{}{},
			},
			"serverInfo": map[string]interface{}{
				"name":    "acmg-amp-mcp-server",
				"version": "v0.1.0",
			},
		},
	}
}

// GetSystemInfo returns system handler info
func (h *InitializeHandler) GetSystemInfo() SystemInfo {
	return SystemInfo{
		Method:      "initialize",
		Description: "Initialize MCP connection and negotiate capabilities",
	}
}

// ToolsListHandler handles tools/list requests
type ToolsListHandler struct {
	logger *logrus.Logger
	router *MessageRouter
}

// HandleSystem implements the tools/list handler
func (h *ToolsListHandler) HandleSystem(ctx context.Context, req *JSONRPC2Request) *JSONRPC2Response {
	h.logger.Debug("Handling tools/list request")

	tools := make([]map[string]interface{}, 0)
	
	// Get all registered tool handlers
	toolHandlers := h.router.GetToolHandlers()
	for _, handler := range toolHandlers {
		toolInfo := handler.GetToolInfo()
		tool := map[string]interface{}{
			"name":        toolInfo.Name,
			"description": toolInfo.Description,
		}
		if toolInfo.InputSchema != nil {
			tool["inputSchema"] = toolInfo.InputSchema
		}
		tools = append(tools, tool)
	}

	return &JSONRPC2Response{
		Result: map[string]interface{}{
			"tools": tools,
		},
	}
}

// GetSystemInfo returns system handler info
func (h *ToolsListHandler) GetSystemInfo() SystemInfo {
	return SystemInfo{
		Method:      "tools/list",
		Description: "List available MCP tools",
	}
}

// ToolsCallHandler handles tools/call requests
type ToolsCallHandler struct {
	logger *logrus.Logger
	router *MessageRouter
}

// HandleSystem implements the tools/call handler
func (h *ToolsCallHandler) HandleSystem(ctx context.Context, req *JSONRPC2Request) *JSONRPC2Response {
	h.logger.Debug("Handling tools/call request")

	// Parse call parameters
	var params struct {
		Name      string      `json:"name"`
		Arguments interface{} `json:"arguments"`
	}

	if req.Params != nil {
		if paramsData, err := json.Marshal(req.Params); err == nil {
			json.Unmarshal(paramsData, &params)
		}
	}

	if params.Name == "" {
		return &JSONRPC2Response{
			Error: &RPCError{
				Code:    InvalidParams,
				Message: "Missing required parameter 'name'",
			},
		}
	}

	// Find tool handler
	toolHandler, exists := h.router.GetToolHandler(params.Name)
	if !exists {
		return &JSONRPC2Response{
			Error: &RPCError{
				Code:    InvalidParams,
				Message: "Tool not found",
				Data:    params.Name,
			},
		}
	}

	// Create new request for tool handler
	toolReq := &JSONRPC2Request{
		JSONRPC: req.JSONRPC,
		Method:  "tool_call",
		Params:  params.Arguments,
		ID:      req.ID,
	}

	// Delegate to tool handler
	return toolHandler.HandleTool(ctx, toolReq)
}

// GetSystemInfo returns system handler info
func (h *ToolsCallHandler) GetSystemInfo() SystemInfo {
	return SystemInfo{
		Method:      "tools/call",
		Description: "Call a specific MCP tool",
	}
}

// ResourcesListHandler handles resources/list requests
type ResourcesListHandler struct {
	logger *logrus.Logger
	router *MessageRouter
}

// HandleSystem implements the resources/list handler
func (h *ResourcesListHandler) HandleSystem(ctx context.Context, req *JSONRPC2Request) *JSONRPC2Response {
	h.logger.Debug("Handling resources/list request")

	resources := make([]map[string]interface{}, 0)
	
	// Get all registered resource handlers
	resourceHandlers := h.router.GetResourceHandlers()
	for pattern, handler := range resourceHandlers {
		resourceInfo := handler.GetResourceInfo()
		resource := map[string]interface{}{
			"uri":  resourceInfo.URI,
			"name": resourceInfo.Name,
		}
		if resourceInfo.Description != "" {
			resource["description"] = resourceInfo.Description
		}
		if resourceInfo.MimeType != "" {
			resource["mimeType"] = resourceInfo.MimeType
		}
		resources = append(resources, resource)
		
		h.logger.WithField("pattern", pattern).Debug("Added resource")
	}

	return &JSONRPC2Response{
		Result: map[string]interface{}{
			"resources": resources,
		},
	}
}

// GetSystemInfo returns system handler info
func (h *ResourcesListHandler) GetSystemInfo() SystemInfo {
	return SystemInfo{
		Method:      "resources/list",
		Description: "List available MCP resources",
	}
}

// ResourcesReadHandler handles resources/read requests
type ResourcesReadHandler struct {
	logger *logrus.Logger
	router *MessageRouter
}

// HandleSystem implements the resources/read handler
func (h *ResourcesReadHandler) HandleSystem(ctx context.Context, req *JSONRPC2Request) *JSONRPC2Response {
	h.logger.Debug("Handling resources/read request")

	// Parse read parameters
	var params struct {
		URI string `json:"uri"`
	}

	if req.Params != nil {
		if paramsData, err := json.Marshal(req.Params); err == nil {
			json.Unmarshal(paramsData, &params)
		}
	}

	if params.URI == "" {
		return &JSONRPC2Response{
			Error: &RPCError{
				Code:    InvalidParams,
				Message: "Missing required parameter 'uri'",
			},
		}
	}

	// Find resource handler
	resourceHandler, exists := h.router.GetResourceHandler(params.URI)
	if !exists {
		return &JSONRPC2Response{
			Error: &RPCError{
				Code:    InvalidParams,
				Message: "Resource not found",
				Data:    params.URI,
			},
		}
	}

	// Create new request for resource handler
	resourceReq := &JSONRPC2Request{
		JSONRPC: req.JSONRPC,
		Method:  "resource_read",
		Params:  req.Params,
		ID:      req.ID,
	}

	// Delegate to resource handler
	return resourceHandler.HandleResource(ctx, resourceReq)
}

// GetSystemInfo returns system handler info
func (h *ResourcesReadHandler) GetSystemInfo() SystemInfo {
	return SystemInfo{
		Method:      "resources/read",
		Description: "Read a specific MCP resource",
	}
}

// PromptsListHandler handles prompts/list requests
type PromptsListHandler struct {
	logger *logrus.Logger
	router *MessageRouter
}

// HandleSystem implements the prompts/list handler
func (h *PromptsListHandler) HandleSystem(ctx context.Context, req *JSONRPC2Request) *JSONRPC2Response {
	h.logger.Debug("Handling prompts/list request")

	prompts := make([]map[string]interface{}, 0)
	
	// Get all registered prompt handlers
	promptHandlers := h.router.GetPromptHandlers()
	for _, handler := range promptHandlers {
		promptInfo := handler.GetPromptInfo()
		prompt := map[string]interface{}{
			"name":        promptInfo.Name,
			"description": promptInfo.Description,
		}
		if len(promptInfo.Arguments) > 0 {
			prompt["arguments"] = promptInfo.Arguments
		}
		prompts = append(prompts, prompt)
	}

	return &JSONRPC2Response{
		Result: map[string]interface{}{
			"prompts": prompts,
		},
	}
}

// GetSystemInfo returns system handler info
func (h *PromptsListHandler) GetSystemInfo() SystemInfo {
	return SystemInfo{
		Method:      "prompts/list",
		Description: "List available MCP prompts",
	}
}

// PromptsGetHandler handles prompts/get requests
type PromptsGetHandler struct {
	logger *logrus.Logger
	router *MessageRouter
}

// HandleSystem implements the prompts/get handler
func (h *PromptsGetHandler) HandleSystem(ctx context.Context, req *JSONRPC2Request) *JSONRPC2Response {
	h.logger.Debug("Handling prompts/get request")

	// Parse get parameters
	var params struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	}

	if req.Params != nil {
		if paramsData, err := json.Marshal(req.Params); err == nil {
			json.Unmarshal(paramsData, &params)
		}
	}

	if params.Name == "" {
		return &JSONRPC2Response{
			Error: &RPCError{
				Code:    InvalidParams,
				Message: "Missing required parameter 'name'",
			},
		}
	}

	// Find prompt handler
	promptHandler, exists := h.router.GetPromptHandler(params.Name)
	if !exists {
		return &JSONRPC2Response{
			Error: &RPCError{
				Code:    InvalidParams,
				Message: "Prompt not found",
				Data:    params.Name,
			},
		}
	}

	// Create new request for prompt handler
	promptReq := &JSONRPC2Request{
		JSONRPC: req.JSONRPC,
		Method:  "prompt_get",
		Params:  params.Arguments,
		ID:      req.ID,
	}

	// Delegate to prompt handler
	return promptHandler.HandlePrompt(ctx, promptReq)
}

// GetSystemInfo returns system handler info
func (h *PromptsGetHandler) GetSystemInfo() SystemInfo {
	return SystemInfo{
		Method:      "prompts/get",
		Description: "Get a specific MCP prompt",
	}
}