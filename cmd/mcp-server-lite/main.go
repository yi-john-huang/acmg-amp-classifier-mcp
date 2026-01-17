// Package main provides the lightweight entry point for the ACMG-AMP MCP Server.
// This version requires no external databases - uses in-memory caching and SQLite.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/acmg-amp-mcp-server/internal/config"
	"github.com/acmg-amp-mcp-server/internal/mcp"
	"github.com/acmg-amp-mcp-server/internal/setup"
)

func main() {
	// Check for setup subcommand
	if len(os.Args) > 1 && os.Args[1] == "setup" {
		cli := setup.NewCLI("lite")
		if err := cli.Run(os.Args[2:]); err != nil {
			log.Fatalf("Setup failed: %v", err)
		}
		return
	}

	// Load lightweight configuration
	cfg := config.LoadLiteConfig()

	log.Printf("Starting ACMG-AMP MCP Server (Lite) with transport: %s", cfg.Transport)
	log.Printf("Data directory: %s", cfg.DataDir)

	// Create lite MCP server
	server, err := mcp.NewLiteServer(cfg)
	if err != nil {
		log.Fatalf("Failed to create MCP server: %v", err)
	}
	defer server.Close()

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

	// Start MCP server
	if err := server.Start(ctx); err != nil {
		log.Fatalf("MCP server failed: %v", err)
	}

	log.Println("ACMG-AMP MCP Server (Lite) stopped")
}
