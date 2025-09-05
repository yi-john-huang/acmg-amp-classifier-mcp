package mcp

import (
	"context"
	"fmt"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sirupsen/logrus"

	"github.com/acmg-amp-mcp-server/internal/config"
)

// Server represents the ACMG-AMP MCP Server implementation
type Server struct {
	config     *config.Manager
	mcpServer  *mcp.Server
	transport  mcp.Transport
	logger     *logrus.Logger
}

// ServerInfo contains MCP server metadata
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// NewServer creates a new MCP server instance
func NewServer(configManager *config.Manager) (*Server, error) {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	logger.SetFormatter(&logrus.JSONFormatter{})

	// Create server info
	serverInfo := &mcp.Implementation{
		Name:    "acmg-amp-mcp-server",
		Version: "v0.1.0",
	}

	// Create MCP server
	mcpServer := mcp.NewServer(serverInfo, nil)

	server := &Server{
		config:    configManager,
		mcpServer: mcpServer,
		logger:    logger,
	}

	// Register capabilities
	if err := server.registerCapabilities(); err != nil {
		return nil, fmt.Errorf("failed to register capabilities: %w", err)
	}

	return server, nil
}

// Start starts the MCP server with the appropriate transport
func (s *Server) Start(ctx context.Context) error {
	s.logger.Info("Starting ACMG-AMP MCP Server...")

	// Determine transport type
	transport, err := s.detectTransport()
	if err != nil {
		return fmt.Errorf("failed to detect transport: %w", err)
	}

	s.transport = transport
	s.logger.WithField("transport", s.getTransportName()).Info("Using transport")

	// Run the MCP server
	if err := s.mcpServer.Run(ctx, transport); err != nil {
		return fmt.Errorf("MCP server failed: %w", err)
	}

	return nil
}

// detectTransport determines the appropriate transport based on environment
func (s *Server) detectTransport() (mcp.Transport, error) {
	// Check if running in stdio mode (default for MCP servers)
	if len(os.Args) > 1 && os.Args[1] == "--stdio" {
		return &mcp.StdioTransport{}, nil
	}

	// Check for environment variables indicating transport type
	if transportType := os.Getenv("MCP_TRANSPORT"); transportType == "http" {
		// HTTP transport would be implemented here
		// For now, fallback to stdio
		s.logger.Warn("HTTP transport requested but not yet implemented, using stdio")
		return &mcp.StdioTransport{}, nil
	}

	// Default to stdio transport
	return &mcp.StdioTransport{}, nil
}

// getTransportName returns a human-readable transport name
func (s *Server) getTransportName() string {
	switch s.transport.(type) {
	case *mcp.StdioTransport:
		return "stdio"
	default:
		return "unknown"
	}
}

// registerCapabilities registers all MCP tools, resources, and prompts
func (s *Server) registerCapabilities() error {
	s.logger.Info("Registering MCP capabilities...")

	// Register classification tools
	if err := s.registerClassificationTools(); err != nil {
		return fmt.Errorf("failed to register classification tools: %w", err)
	}

	// Register evidence tools
	if err := s.registerEvidenceTools(); err != nil {
		return fmt.Errorf("failed to register evidence tools: %w", err)
	}

	// Register report tools
	if err := s.registerReportTools(); err != nil {
		return fmt.Errorf("failed to register report tools: %w", err)
	}

	// Register resources
	if err := s.registerResources(); err != nil {
		return fmt.Errorf("failed to register resources: %w", err)
	}

	// Register prompts
	if err := s.registerPrompts(); err != nil {
		return fmt.Errorf("failed to register prompts: %w", err)
	}

	s.logger.Info("Successfully registered all MCP capabilities")
	return nil
}

// registerClassificationTools registers ACMG/AMP classification tools
func (s *Server) registerClassificationTools() error {
	// classify_variant tool
	classifyTool := mcp.NewTool("classify_variant",
		mcp.WithToolDescription("Classify a genetic variant using ACMG/AMP guidelines"),
	)

	s.mcpServer.AddTool(classifyTool, s.handleClassifyVariantSimple)

	// validate_hgvs tool
	validateTool := mcp.NewTool("validate_hgvs",
		mcp.WithToolDescription("Validate HGVS notation and parse variant components"),
	)

	s.mcpServer.AddTool(validateTool, s.handleValidateHGVSSimple)

	s.logger.Debug("Registered classification tools")
	return nil
}

// registerEvidenceTools registers evidence gathering tools
func (s *Server) registerEvidenceTools() error {
	// query_evidence tool
	evidenceTool := mcp.NewTool("query_evidence",
		mcp.WithToolDescription("Query external databases for variant evidence"),
	)

	s.mcpServer.AddTool(evidenceTool, s.handleQueryEvidenceSimple)

	s.logger.Debug("Registered evidence tools")
	return nil
}

// registerReportTools registers report generation tools
func (s *Server) registerReportTools() error {
	// generate_report tool
	reportTool := mcp.NewTool("generate_report",
		mcp.WithToolDescription("Generate clinical interpretation report"),
	)

	s.mcpServer.AddTool(reportTool, s.handleGenerateReportSimple)

	s.logger.Debug("Registered report tools")
	return nil
}

// registerResources registers MCP resources
func (s *Server) registerResources() error {
	// For now, we'll register placeholder resource templates
	s.logger.Debug("Resource registration placeholder - to be implemented")
	return nil
}

// registerPrompts registers MCP prompts
func (s *Server) registerPrompts() error {
	// For now, we'll register placeholder prompts
	s.logger.Debug("Prompt registration placeholder - to be implemented")
	return nil
}