package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/acmg-amp-mcp-server/internal/api"
	"github.com/acmg-amp-mcp-server/internal/config"
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
	log.Printf("Starting ACMG-AMP MCP Server on %s:%d", cfg.Server.Host, cfg.Server.Port)

	// Create server
	server := api.NewServer(configManager)

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutdown signal received, gracefully shutting down...")
		cancel()
	}()

	// Start server
	if err := server.Start(ctx); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}

	log.Println("Server stopped")
}
