package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/acmg-amp-mcp-server/internal/config"
	"github.com/acmg-amp-mcp-server/internal/mcp"
)

func main() {
	// Load configuration
	configManager, err := config.NewManager()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Validate configuration
	if err := configManager.Validate(); err != nil {
		log.Fatalf("Configuration validation failed: %v", err)
	}

	cfg := configManager.GetConfig()
	log.Printf("Starting ACMG-AMP MCP Server with protocol version %s", "2025-01-01")

	// Create MCP server
	mcpServer, err := mcp.NewServer(configManager)
	if err != nil {
		log.Fatalf("Failed to create MCP server: %v", err)
	}

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutdown signal received, gracefully shutting down MCP server...")
		cancel()
	}()

	// Start MCP server
	if err := mcpServer.Start(ctx); err != nil {
		log.Fatalf("MCP server failed to start: %v", err)
	}

	log.Println("ACMG-AMP MCP Server stopped")
}