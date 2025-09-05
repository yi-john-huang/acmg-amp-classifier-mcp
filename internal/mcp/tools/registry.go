package tools

import (
	"github.com/sirupsen/logrus"

	"github.com/acmg-amp-mcp-server/internal/mcp/protocol"
)

// ToolRegistry manages registration of all MCP tools
type ToolRegistry struct {
	logger *logrus.Logger
	router *protocol.MessageRouter
}

// NewToolRegistry creates a new tool registry
func NewToolRegistry(logger *logrus.Logger, router *protocol.MessageRouter) *ToolRegistry {
	return &ToolRegistry{
		logger: logger,
		router: router,
	}
}

// RegisterAllTools registers all ACMG/AMP classification tools with the MCP router
func (tr *ToolRegistry) RegisterAllTools() error {
	tr.logger.Info("Registering ACMG/AMP classification tools")

	// Register classify_variant tool
	classifyTool := NewClassifyVariantTool(tr.logger)
	tr.router.RegisterToolHandler("classify_variant", classifyTool)
	tr.logger.Debug("Registered classify_variant tool")

	// Register validate_hgvs tool
	validateTool := NewValidateHGVSTool(tr.logger)
	tr.router.RegisterToolHandler("validate_hgvs", validateTool)
	tr.logger.Debug("Registered validate_hgvs tool")

	// Register apply_rule tool
	applyRuleTool := NewApplyRuleTool(tr.logger)
	tr.router.RegisterToolHandler("apply_rule", applyRuleTool)
	tr.logger.Debug("Registered apply_rule tool")

	// Register combine_evidence tool
	combineEvidenceTool := NewCombineEvidenceTool(tr.logger)
	tr.router.RegisterToolHandler("combine_evidence", combineEvidenceTool)
	tr.logger.Debug("Registered combine_evidence tool")

	tr.logger.Info("Successfully registered all ACMG/AMP classification tools")
	return nil
}

// GetRegisteredToolsInfo returns information about all registered tools
func (tr *ToolRegistry) GetRegisteredToolsInfo() []protocol.ToolInfo {
	toolHandlers := tr.router.GetToolHandlers()
	toolsInfo := make([]protocol.ToolInfo, 0, len(toolHandlers))

	for _, handler := range toolHandlers {
		toolsInfo = append(toolsInfo, handler.GetToolInfo())
	}

	return toolsInfo
}

// ValidateAllTools validates all registered tools can handle their schemas
func (tr *ToolRegistry) ValidateAllTools() error {
	tr.logger.Info("Validating all registered tools")

	toolHandlers := tr.router.GetToolHandlers()
	
	for name, handler := range toolHandlers {
		tr.logger.WithField("tool", name).Debug("Validating tool")
		
		// Basic validation - check if tool info is complete
		toolInfo := handler.GetToolInfo()
		if toolInfo.Name == "" {
			tr.logger.WithField("tool", name).Error("Tool missing name")
			continue
		}
		
		if toolInfo.Description == "" {
			tr.logger.WithField("tool", name).Warn("Tool missing description")
		}
		
		if toolInfo.InputSchema == nil {
			tr.logger.WithField("tool", name).Warn("Tool missing input schema")
		}
		
		tr.logger.WithField("tool", name).Debug("Tool validation completed")
	}

	tr.logger.Info("Tool validation completed")
	return nil
}