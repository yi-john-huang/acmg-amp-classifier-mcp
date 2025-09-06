package mcp

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sirupsen/logrus"

	"github.com/acmg-amp-mcp-server/internal/config"
	"github.com/acmg-amp-mcp-server/internal/mcp/protocol"
	"github.com/acmg-amp-mcp-server/internal/mcp/tools"
	"github.com/acmg-amp-mcp-server/internal/mcp/transport"
	"github.com/acmg-amp-mcp-server/internal/service"
	"github.com/acmg-amp-mcp-server/pkg/external"
	"github.com/acmg-amp-mcp-server/internal/domain"
)

// Server represents the ACMG-AMP MCP Server implementation
type Server struct {
	config          *config.Manager
	mcpServer       *mcp.Server
	transportMgr    *transport.Manager
	activeTransport transport.Transport
	protocolCore    *protocol.ProtocolCore
	toolRegistry    *tools.ToolRegistry
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

	// Create protocol core
	protocolCore := protocol.NewProtocolCore(logger)

	// Create message router
	router := protocol.NewMessageRouter(logger)

	// The protocol core will route messages through its built-in system handlers
	// and the message router handles tool-specific routing

	// Create external services for evidence gathering
	// TODO: Extract individual configs from mcpConfig once available
	// For now, create with default/empty configs
	knowledgeBaseService, err := external.NewKnowledgeBaseService(
		domain.ClinVarConfig{},
		domain.GnomADConfig{},
		domain.COSMICConfig{},
		domain.PubMedConfig{},
		domain.LOVDConfig{},
		domain.HGMDConfig{},
		domain.CacheConfig{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create knowledge base service: %w", err)
	}

	// Create input parser for HGVS notation
	inputParser := domain.NewStandardInputParser()

	// Create classifier service
	classifierService := service.NewClassifierService(logger, knowledgeBaseService, inputParser)

	// Create tool registry and register tools
	toolRegistry := tools.NewToolRegistry(logger, router, classifierService)
	if err := toolRegistry.RegisterAllTools(); err != nil {
		return nil, fmt.Errorf("failed to register tools: %w", err)
	}

	// Validate all tools
	if err := toolRegistry.ValidateAllTools(); err != nil {
		return nil, fmt.Errorf("tool validation failed: %w", err)
	}

	// Create server info
	serverInfo := &mcp.Implementation{
		Name:    "acmg-amp-mcp-server",
		Version: "v0.1.0",
	}

	// Create MCP server with tool handlers
	mcpServer := mcp.NewServer(serverInfo, nil)
	
	// Create server instance first
	server := &Server{
		config:       configManager,
		mcpServer:    mcpServer,
		transportMgr: transportMgr,
		protocolCore: protocolCore,
		toolRegistry: toolRegistry,
		logger:       logger,
	}

	// Register MCP tools from our tool registry
	if err := server.registerMCPTools(mcpServer, toolRegistry); err != nil {
		return nil, fmt.Errorf("failed to register MCP tools: %w", err)
	}

	// Register capabilities
	if err := server.registerCapabilities(); err != nil {
		return nil, fmt.Errorf("failed to register capabilities: %w", err)
	}

	return server, nil
}

// registerMCPTools registers our tools with the MCP SDK
func (s *Server) registerMCPTools(mcpServer *mcp.Server, toolRegistry *tools.ToolRegistry) error {
	s.logger.Info("Registering tools with MCP SDK...")
	
	// Get all registered tool info from our registry
	toolsInfo := toolRegistry.GetRegisteredToolsInfo()
	
	for _, toolInfo := range toolsInfo {
		// Create MCP tool definition
		toolDef := &mcp.Tool{
			Name:        toolInfo.Name,
			Description: toolInfo.Description,
			// TODO: Add proper input schema from tool info
		}
		
		// Create handler bridge
		handler := NewMCPToolHandler(toolRegistry, toolInfo.Name, s.logger)
		
		// Register with MCP server
		mcpServer.AddTool(toolDef, handler)
		
		s.logger.WithField("tool_name", toolInfo.Name).Debug("Registered MCP tool")
	}
	
	s.logger.WithField("tool_count", len(toolsInfo)).Info("Successfully registered all tools with MCP SDK")
	return nil
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

	// Create bridge between our transport and MCP SDK
	mcpTransport := NewMCPTransportBridge(activeTransport, s.logger)
	
	// Run the MCP server with our bridged transport
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
	// Tools are now registered via the ToolRegistry during server initialization
	toolsInfo := s.toolRegistry.GetRegisteredToolsInfo()
	s.logger.WithField("tool_count", len(toolsInfo)).Debug("Classification tools registered")
	
	// Log registered tools
	for _, toolInfo := range toolsInfo {
		s.logger.WithField("tool_name", toolInfo.Name).Debug("Tool available")
	}
	
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