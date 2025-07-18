package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/acmg-amp-mcp-server/internal/domain"
	"github.com/gin-gonic/gin"
)

// Server represents the HTTP server
type Server struct {
	configManager domain.ConfigManager
	router        *gin.Engine
	server        *http.Server
}

// NewServer creates a new HTTP server instance
func NewServer(configManager domain.ConfigManager) *Server {
	cfg := configManager.GetConfig()

	// Set Gin mode based on environment
	if cfg.Logging.Level == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	// Add middleware
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.Use(corsMiddleware())
	router.Use(requestIDMiddleware())

	server := &Server{
		configManager: configManager,
		router:        router,
	}

	// Setup routes
	server.setupRoutes()

	return server
}

// Start starts the HTTP server
func (s *Server) Start(ctx context.Context) error {
	cfg := s.configManager.GetServerConfig()
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)

	s.server = &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	// Start server in a goroutine
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			panic(fmt.Sprintf("Failed to start server: %v", err))
		}
	}()

	// Wait for context cancellation
	<-ctx.Done()

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return s.server.Shutdown(shutdownCtx)
}

// setupRoutes configures the API routes
func (s *Server) setupRoutes() {
	// Health check endpoint
	s.router.GET("/health", s.handleHealth)

	// API v1 routes
	v1 := s.router.Group("/api/v1")
	{
		v1.POST("/interpret", s.handleVariantInterpretation)
		v1.GET("/variant/:id", s.handleGetVariant)
		v1.GET("/interpretation/:id", s.handleGetInterpretation)
	}
}

// handleHealth handles health check requests
func (s *Server) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": time.Now(),
		"version":   "1.0.0",
	})
}

// handleVariantInterpretation handles variant interpretation requests
func (s *Server) handleVariantInterpretation(c *gin.Context) {
	// TODO: Implement variant interpretation logic
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "Variant interpretation not yet implemented",
	})
}

// handleGetVariant handles variant retrieval requests
func (s *Server) handleGetVariant(c *gin.Context) {
	// TODO: Implement variant retrieval logic
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "Variant retrieval not yet implemented",
	})
}

// handleGetInterpretation handles interpretation retrieval requests
func (s *Server) handleGetInterpretation(c *gin.Context) {
	// TODO: Implement interpretation retrieval logic
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "Interpretation retrieval not yet implemented",
	})
}

// corsMiddleware adds CORS headers to responses
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, X-API-Key")
		c.Header("Access-Control-Expose-Headers", "Content-Length")
		c.Header("Access-Control-Allow-Credentials", "true")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// requestIDMiddleware adds a unique request ID to each request
func requestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}
		c.Header("X-Request-ID", requestID)
		c.Set("request_id", requestID)
		c.Next()
	}
}

// generateRequestID generates a simple request ID
func generateRequestID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
