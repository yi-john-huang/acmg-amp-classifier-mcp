package mcp

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sirupsen/logrus"

	"github.com/acmg-amp-mcp-server/internal/config"
	"github.com/acmg-amp-mcp-server/internal/mcp/transport"
)

// Server represents the ACMG-AMP MCP Server implementation
type Server struct {
	config          *config.Manager
	mcpServer       *mcp.Server
	transportMgr    *transport.Manager
	activeTransport transport.Transport
	logger          *logrus.Logger
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

	// Get MCP configuration
	cfg := configManager.GetConfig()
	mcpConfig := &cfg.MCP

	// Create transport manager
	transportMgr := transport.NewManager(logger, mcpConfig)

	// Create server info
	serverInfo := &mcp.Implementation{
		Name:    "acmg-amp-mcp-server",
		Version: "v0.1.0",
	}

	// Create MCP server
	mcpServer := mcp.NewServer(serverInfo, nil)

	server := &Server{
		config:       configManager,
		mcpServer:    mcpServer,
		transportMgr: transportMgr,
		logger:       logger,
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

	// Start transport using transport manager
	activeTransport, err := s.transportMgr.StartTransport(ctx)
	if err != nil {
		return fmt.Errorf("failed to start transport: %w", err)
	}

	s.activeTransport = activeTransport
	s.logger.WithField("transport_type", activeTransport.GetType()).Info("Transport initialized")

	// For now, we'll use the MCP SDK's built-in transport
	// TODO: Integrate our custom transport with the MCP SDK
	var mcpTransport mcp.Transport
	switch activeTransport.GetType() {
	case "stdio":
		mcpTransport = &mcp.StdioTransport{}
	default:
		// Fallback to stdio for unsupported transports
		s.logger.Warn("Using fallback stdio transport for MCP SDK")
		mcpTransport = &mcp.StdioTransport{}
	}

	// Run the MCP server
	if err := s.mcpServer.Run(ctx, mcpTransport); err != nil {
		s.activeTransport.Close()
		return fmt.Errorf("MCP server failed: %w", err)
	}

	return nil
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
	// TODO: Implement tool registration when MCP SDK API is finalized
	// For now, return success to allow building
	s.logger.Debug("Registered classification tools (placeholder)")
	return nil
}

// registerEvidenceTools registers evidence gathering tools
func (s *Server) registerEvidenceTools() error {
	// TODO: Implement tool registration when MCP SDK API is finalized
	// For now, return success to allow building
	s.logger.Debug("Registered evidence tools (placeholder)")
	return nil
}

// registerReportTools registers report generation tools
func (s *Server) registerReportTools() error {
	// TODO: Implement tool registration when MCP SDK API is finalized
	// For now, return success to allow building
	s.logger.Debug("Registered report tools (placeholder)")
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