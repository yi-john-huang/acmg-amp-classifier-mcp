// Package mcp provides the MCP server implementation.
// This file contains shared feedback tool registration logic.
package mcp

import (
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/acmg-amp-mcp-server/internal/feedback"
	"github.com/acmg-amp-mcp-server/internal/mcp/tools"
)

// registerFeedbackTools registers feedback-related MCP tools.
func registerFeedbackTools(registry *tools.ToolRegistry, logger *logrus.Logger, store feedback.Store, exportDir string) error {
	// Create feedback tools
	submitTool := tools.NewSubmitFeedbackTool(logger, store)
	queryTool := tools.NewQueryFeedbackTool(logger, store)
	listTool := tools.NewListFeedbackTool(logger, store)
	exportTool := tools.NewExportFeedbackTool(logger, store, exportDir)
	importTool := tools.NewImportFeedbackTool(logger, store)

	// Register with the registry
	feedbackTools := []tools.Tool{
		submitTool,
		queryTool,
		listTool,
		exportTool,
		importTool,
	}

	for _, tool := range feedbackTools {
		if err := registry.RegisterTool(tool); err != nil {
			return fmt.Errorf("failed to register %s: %w", tool.GetToolInfo().Name, err)
		}
		logger.WithField("tool_name", tool.GetToolInfo().Name).Debug("Registered feedback tool")
	}

	return nil
}
