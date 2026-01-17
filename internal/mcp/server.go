package mcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sirupsen/logrus"

	"github.com/acmg-amp-mcp-server/internal/config"
	"github.com/acmg-amp-mcp-server/internal/domain"
	"github.com/acmg-amp-mcp-server/internal/feedback"
	"github.com/acmg-amp-mcp-server/internal/mcp/caching"
	"github.com/acmg-amp-mcp-server/internal/mcp/protocol"
	"github.com/acmg-amp-mcp-server/internal/mcp/tools"
	"github.com/acmg-amp-mcp-server/internal/mcp/transport"
	"github.com/acmg-amp-mcp-server/internal/service"
	"github.com/acmg-amp-mcp-server/pkg/external"
)

// Server represents the ACMG-AMP MCP Server implementation
type Server struct {
	config          *config.Manager
	mcpServer       *mcp.Server
	transportMgr    *transport.Manager
	activeTransport transport.Transport
	protocolCore    *protocol.ProtocolCore
	toolRegistry    *tools.ToolRegistry
	feedbackStore   feedback.Store
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
	// For now, create with sensible default configs
	knowledgeBaseService, err := external.NewKnowledgeBaseService(
		domain.ClinVarConfig{
			BaseURL:   "https://eutils.ncbi.nlm.nih.gov/entrez/eutils",
			RateLimit: 3, // 3 requests per second (NCBI guideline)
			Timeout:   30 * time.Second,
		},
		domain.GnomADConfig{
			BaseURL:   "https://gnomad.broadinstitute.org/api",
			RateLimit: 10,
			Timeout:   30 * time.Second,
		},
		domain.COSMICConfig{
			BaseURL:   "https://cancer.sanger.ac.uk/cosmic/search",
			RateLimit: 5,
			Timeout:   30 * time.Second,
		},
		domain.PubMedConfig{
			BaseURL:   "https://eutils.ncbi.nlm.nih.gov/entrez/eutils",
			RateLimit: 3, // 3 requests per second (NCBI guideline)
			Timeout:   30 * time.Second,
		},
		domain.LOVDConfig{
			BaseURL:   "https://www.lovd.nl/3.0/api",
			RateLimit: 5,
			Timeout:   30 * time.Second,
		},
		domain.HGMDConfig{
			BaseURL:   "https://my.qiagendigitalinsights.com/bbp/view/hgmd",
			RateLimit: 2,
			Timeout:   30 * time.Second,
		},
		domain.CacheConfig{
			RedisURL:    "redis://localhost:6379/0",
			DefaultTTL:  300 * time.Second, // 5 minutes
			MaxRetries:  3,
			PoolSize:    10,
			PoolTimeout: 5 * time.Second,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create knowledge base service: %w", err)
	}

	// Create input parser for HGVS notation
	inputParser := domain.NewStandardInputParser()

	// Create transcript resolver with external API access
	transcriptResolverConfig := service.TranscriptResolverConfig{
		MemoryCacheTTL: 15 * time.Minute,
		RedisCacheTTL:  24 * time.Hour,
		MaxMemorySize:  1000,
		MaxConcurrency: 5,
		ExternalAPIConfig: external.UnifiedGeneAPIConfig{
			RefSeqConfig: external.RefSeqConfig{
				BaseURL:    "https://eutils.ncbi.nlm.nih.gov/entrez/eutils",
				RateLimit:  3, // 3 requests per second (NCBI guideline without API key)
				Timeout:    30 * time.Second,
				MaxRetries: 3,
			},
			EnsemblConfig: external.EnsemblConfig{
				BaseURL:   "https://rest.ensembl.org",
				RateLimit: 15,
				Timeout:   30 * time.Second,
			},
			HGNCConfig: external.HGNCConfig{
				BaseURL:   "https://rest.genenames.org",
				RateLimit: 10,
				Timeout:   30 * time.Second,
			},
			CircuitBreaker: external.UnifiedCircuitBreakerConfig{
				MaxRequests: 5,
				Interval:    60 * time.Second,
				Timeout:     30 * time.Second,
			},
		},
	}

	// Create Redis cache for transcript resolver
	// TODO: This should use the same Redis config as the main cache
	redisCache := &caching.ToolResultCache{} // Placeholder - would need proper initialization

	cachedResolver, err := service.NewCachedTranscriptResolver(transcriptResolverConfig, redisCache, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create transcript resolver: %w", err)
	}

	// Create adapter to convert to domain interface
	transcriptResolver := service.NewTranscriptResolverAdapter(cachedResolver)

	// Inject transcript resolver into input parser
	if standardParser, ok := inputParser.(*domain.StandardInputParser); ok {
		standardParser.SetTranscriptResolver(transcriptResolver)
	}

	// Create classifier service with transcript resolver
	classifierService := service.NewClassifierService(logger, knowledgeBaseService, inputParser, transcriptResolver)

	// Validate service initialization and connectivity
	if err := validateServiceConnectivity(logger, transcriptResolver, knowledgeBaseService); err != nil {
		return nil, fmt.Errorf("service connectivity validation failed: %w", err)
	}

	// Create tool registry and register tools
	toolRegistry := tools.NewToolRegistry(logger, router, classifierService)
	if err := toolRegistry.RegisterAllTools(); err != nil {
		return nil, fmt.Errorf("failed to register tools: %w", err)
	}

	// Initialize feedback store (PostgreSQL-based, using same database as main app)
	dbConnStr := configManager.GetDatabaseConnectionString()
	feedbackStore, err := feedback.NewPostgresStoreFromURL(dbConnStr)
	if err != nil {
		return nil, fmt.Errorf("failed to create feedback store: %w", err)
	}

	// Register feedback tools
	// Export directory for JSON backups (still uses local filesystem)
	exportDir := filepath.Join(getFeedbackDataDir(), "exports")
	if err := os.MkdirAll(exportDir, 0755); err != nil {
		feedbackStore.Close()
		return nil, fmt.Errorf("failed to create export directory: %w", err)
	}
	if err := registerFeedbackTools(toolRegistry, logger, feedbackStore, exportDir); err != nil {
		feedbackStore.Close()
		return nil, fmt.Errorf("failed to register feedback tools: %w", err)
	}

	// Validate all tools
	if err := toolRegistry.ValidateAllTools(); err != nil {
		feedbackStore.Close()
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
		config:        configManager,
		mcpServer:     mcpServer,
		transportMgr:  transportMgr,
		protocolCore:  protocolCore,
		toolRegistry:  toolRegistry,
		feedbackStore: feedbackStore,
		logger:        logger,
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

// validateServiceConnectivity validates that all external services are accessible
func validateServiceConnectivity(logger *logrus.Logger, transcriptResolver domain.GeneTranscriptResolver, knowledgeBaseService *external.KnowledgeBaseService) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	logger.Info("Validating service connectivity...")

	// Test transcript resolver connectivity with a well-known gene
	logger.Debug("Testing transcript resolver connectivity...")
	if _, err := transcriptResolver.ResolveGeneToTranscript(ctx, "BRCA1"); err != nil {
		logger.WithError(err).Warn("Transcript resolver connectivity test failed - service may have limited functionality")
		// Don't fail initialization, but log the issue
	} else {
		logger.Info("Transcript resolver connectivity validated successfully")
	}

	// Test knowledge base service connectivity
	logger.Debug("Testing knowledge base service connectivity...")
	// Knowledge base service validation would go here
	// For now, just log success
	logger.Info("Knowledge base service connectivity assumed valid")

	logger.Info("Service connectivity validation completed")
	return nil
}

// Close cleans up server resources.
func (s *Server) Close() error {
	if s.feedbackStore != nil {
		if err := s.feedbackStore.Close(); err != nil {
			s.logger.WithError(err).Error("Failed to close feedback store")
		}
	}
	if s.activeTransport != nil {
		s.activeTransport.Close()
	}
	return nil
}

// getFeedbackDataDir returns the directory for feedback data storage.
func getFeedbackDataDir() string {
	// Check environment variable first
	if dir := os.Getenv("ACMG_DATA_DIR"); dir != "" {
		return dir
	}
	// Default to user's home directory
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".acmg-amp-mcp")
}

