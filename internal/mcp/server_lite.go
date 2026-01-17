// Package mcp provides the MCP server implementation.
// This file contains the lightweight server that requires no external databases.
package mcp

import (
	"context"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sirupsen/logrus"

	"github.com/acmg-amp-mcp-server/internal/cache"
	litecfg "github.com/acmg-amp-mcp-server/internal/config"
	"github.com/acmg-amp-mcp-server/internal/domain"
	"github.com/acmg-amp-mcp-server/internal/feedback"
	"github.com/acmg-amp-mcp-server/internal/mcp/protocol"
	"github.com/acmg-amp-mcp-server/internal/mcp/tools"
	"github.com/acmg-amp-mcp-server/internal/mcp/transport"
	"github.com/acmg-amp-mcp-server/internal/service"
	"github.com/acmg-amp-mcp-server/pkg/external"
)

// LiteServer is a lightweight MCP server that requires no external databases.
// It uses in-memory caching and SQLite for persistence.
type LiteServer struct {
	config          *litecfg.LiteConfig
	mcpServer       *mcp.Server
	transportMgr    *transport.Manager
	activeTransport transport.Transport
	toolRegistry    *tools.ToolRegistry
	feedbackStore   feedback.Store
	cache           *cache.MemoryCache
	logger          *logrus.Logger
}

// LiteServerOption is a functional option for LiteServer.
type LiteServerOption func(*LiteServer) error

// WithFeedbackStore sets a custom feedback store.
func WithFeedbackStore(store feedback.Store) LiteServerOption {
	return func(s *LiteServer) error {
		s.feedbackStore = store
		return nil
	}
}

// WithLogger sets a custom logger.
func WithLogger(logger *logrus.Logger) LiteServerOption {
	return func(s *LiteServer) error {
		s.logger = logger
		return nil
	}
}

// NewLiteServer creates a new lightweight MCP server instance.
// It requires no external databases - uses in-memory cache and SQLite.
func NewLiteServer(cfg *litecfg.LiteConfig, opts ...LiteServerOption) (*LiteServer, error) {
	// Create server with default logger
	server := &LiteServer{
		config: cfg,
		logger: logrus.New(),
	}

	// Configure default logger
	if cfg.LogFormat == "text" {
		server.logger.SetFormatter(&logrus.TextFormatter{})
	} else {
		server.logger.SetFormatter(&logrus.JSONFormatter{})
	}
	level, _ := logrus.ParseLevel(cfg.LogLevel)
	server.logger.SetLevel(level)

	// Apply options
	for _, opt := range opts {
		if err := opt(server); err != nil {
			return nil, fmt.Errorf("failed to apply option: %w", err)
		}
	}

	// Ensure data directory exists
	if err := cfg.EnsureDataDir(); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	// Initialize memory cache
	memCache, err := cache.NewMemoryCache(cfg.CacheMaxItems, cfg.CacheTTL)
	if err != nil {
		return nil, fmt.Errorf("failed to create memory cache: %w", err)
	}
	server.cache = memCache

	// Initialize feedback store if not provided
	if server.feedbackStore == nil {
		store, err := feedback.NewSQLiteStore(cfg.FeedbackDBPath())
		if err != nil {
			return nil, fmt.Errorf("failed to create feedback store: %w", err)
		}
		server.feedbackStore = store
	}

	// Create MCP configuration for transport
	mcpConfig := &domain.MCPConfig{
		TransportType: cfg.Transport,
		HTTPPort:      cfg.HTTPPort,
	}

	// Create transport manager and message router
	transportMgr := transport.NewManager(server.logger, mcpConfig)
	router := protocol.NewMessageRouter(server.logger)

	// Create external services for evidence gathering (no Redis cache)
	knowledgeBaseService, err := createKnowledgeBaseService(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create knowledge base service: %w", err)
	}

	// Create input parser for HGVS notation
	inputParser := domain.NewStandardInputParser()

	// Create transcript resolver with in-memory caching only
	transcriptResolver, err := createLiteTranscriptResolver(server.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create transcript resolver: %w", err)
	}

	// Inject transcript resolver into input parser
	if standardParser, ok := inputParser.(*domain.StandardInputParser); ok {
		standardParser.SetTranscriptResolver(transcriptResolver)
	}

	// Create classifier service
	classifierService := service.NewClassifierService(server.logger, knowledgeBaseService, inputParser, transcriptResolver)

	// Create tool registry and register tools
	toolRegistry := tools.NewToolRegistry(server.logger, router, classifierService)
	if err := toolRegistry.RegisterAllTools(); err != nil {
		return nil, fmt.Errorf("failed to register tools: %w", err)
	}

	// Register feedback tools
	if err := registerFeedbackTools(toolRegistry, server.logger, server.feedbackStore, cfg.ExportDir()); err != nil {
		return nil, fmt.Errorf("failed to register feedback tools: %w", err)
	}

	// Validate all tools
	if err := toolRegistry.ValidateAllTools(); err != nil {
		return nil, fmt.Errorf("tool validation failed: %w", err)
	}

	// Create server info
	serverInfo := &mcp.Implementation{
		Name:    "acmg-amp-mcp-server-lite",
		Version: "v0.1.0",
	}

	// Create MCP server
	mcpServer := mcp.NewServer(serverInfo, nil)

	// Complete server setup
	server.mcpServer = mcpServer
	server.transportMgr = transportMgr
	server.toolRegistry = toolRegistry

	// Register MCP tools
	if err := server.registerMCPTools(mcpServer, toolRegistry); err != nil {
		return nil, fmt.Errorf("failed to register MCP tools: %w", err)
	}

	server.logger.Info("Lite server initialized successfully")
	return server, nil
}

// registerMCPTools registers tools with the MCP SDK.
func (s *LiteServer) registerMCPTools(mcpServer *mcp.Server, toolRegistry *tools.ToolRegistry) error {
	s.logger.Info("Registering tools with MCP SDK...")

	toolsInfo := toolRegistry.GetRegisteredToolsInfo()

	for _, toolInfo := range toolsInfo {
		toolDef := &mcp.Tool{
			Name:        toolInfo.Name,
			Description: toolInfo.Description,
		}

		handler := NewMCPToolHandler(toolRegistry, toolInfo.Name, s.logger)
		mcpServer.AddTool(toolDef, handler)

		s.logger.WithField("tool_name", toolInfo.Name).Debug("Registered MCP tool")
	}

	s.logger.WithField("tool_count", len(toolsInfo)).Info("Successfully registered all tools")
	return nil
}

// Start starts the lite MCP server.
func (s *LiteServer) Start(ctx context.Context) error {
	s.logger.Info("Starting ACMG-AMP MCP Server (Lite)...")

	// Start transport
	activeTransport, err := s.transportMgr.StartTransport(ctx)
	if err != nil {
		return fmt.Errorf("failed to start transport: %w", err)
	}

	s.activeTransport = activeTransport
	s.logger.WithField("transport_type", activeTransport.GetType()).Info("Transport initialized")

	// Create bridge between transport and MCP SDK
	mcpTransport := NewMCPTransportBridge(activeTransport, s.logger)

	// Run the MCP server
	if err := s.mcpServer.Run(ctx, mcpTransport); err != nil {
		s.activeTransport.Close()
		return fmt.Errorf("MCP server failed: %w", err)
	}

	return nil
}

// Close cleans up server resources.
func (s *LiteServer) Close() error {
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

// GetFeedbackStore returns the feedback store for external access.
func (s *LiteServer) GetFeedbackStore() feedback.Store {
	return s.feedbackStore
}

// GetCache returns the memory cache for external access.
func (s *LiteServer) GetCache() *cache.MemoryCache {
	return s.cache
}

// createKnowledgeBaseService creates the knowledge base service with no Redis cache.
func createKnowledgeBaseService(cfg *litecfg.LiteConfig) (*external.KnowledgeBaseService, error) {
	return external.NewKnowledgeBaseService(
		domain.ClinVarConfig{
			BaseURL:   "https://eutils.ncbi.nlm.nih.gov/entrez/eutils",
			RateLimit: 3,
			Timeout:   30 * time.Second,
			APIKey:    cfg.ClinVarAPIKey,
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
			APIKey:    cfg.COSMICAPIKey,
		},
		domain.PubMedConfig{
			BaseURL:   "https://eutils.ncbi.nlm.nih.gov/entrez/eutils",
			RateLimit: 3,
			Timeout:   30 * time.Second,
			APIKey:    cfg.ClinVarAPIKey, // PubMed uses same NCBI API key
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
			// Empty Redis URL - service will work without caching
			RedisURL:   "",
			DefaultTTL: 5 * time.Minute,
		},
	)
}

// createLiteTranscriptResolver creates a transcript resolver with in-memory caching only.
func createLiteTranscriptResolver(logger *logrus.Logger) (domain.GeneTranscriptResolver, error) {
	config := service.TranscriptResolverConfig{
		MemoryCacheTTL: 15 * time.Minute,
		RedisCacheTTL:  0, // No Redis cache
		MaxMemorySize:  1000,
		MaxConcurrency: 5,
		ExternalAPIConfig: external.UnifiedGeneAPIConfig{
			RefSeqConfig: external.RefSeqConfig{
				BaseURL:    "https://eutils.ncbi.nlm.nih.gov/entrez/eutils",
				RateLimit:  3,
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

	// Create resolver without Redis cache (pass nil)
	cachedResolver, err := service.NewCachedTranscriptResolver(config, nil, logger)
	if err != nil {
		return nil, err
	}

	return service.NewTranscriptResolverAdapter(cachedResolver), nil
}

