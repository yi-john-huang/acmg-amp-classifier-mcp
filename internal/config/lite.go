// Package config provides configuration management for the MCP server.
// This file contains the lightweight configuration for standalone operation.
package config

import (
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// LiteConfig is a simplified configuration for standalone operation.
// It requires no external databases and uses sensible defaults.
type LiteConfig struct {
	// Data storage
	DataDir string // Base directory for data files

	// Cache settings
	CacheMaxItems int           // Maximum items in memory cache
	CacheTTL      time.Duration // Default cache TTL

	// API settings
	ClinVarAPIKey string // Optional: NCBI API key for higher rate limits
	COSMICAPIKey  string // Optional: COSMIC API key

	// Transport settings
	Transport string // Transport type: stdio, http
	HTTPPort  int    // HTTP port (if transport is http)

	// Logging
	LogLevel  string // Log level: debug, info, warn, error
	LogFormat string // Log format: json, text
}

// DefaultLiteConfig returns a configuration with sensible defaults.
func DefaultLiteConfig() *LiteConfig {
	homeDir, _ := os.UserHomeDir()
	dataDir := filepath.Join(homeDir, ".acmg-amp-mcp")

	return &LiteConfig{
		DataDir:       dataDir,
		CacheMaxItems: 1000,
		CacheTTL:      24 * time.Hour,
		Transport:     "stdio",
		HTTPPort:      8080,
		LogLevel:      "info",
		LogFormat:     "json",
	}
}

// LoadLiteConfig loads configuration from environment variables.
// Falls back to defaults if not set.
func LoadLiteConfig() *LiteConfig {
	cfg := DefaultLiteConfig()

	// Data directory
	if v := os.Getenv("ACMG_DATA_DIR"); v != "" {
		cfg.DataDir = v
	}

	// Cache settings
	if v := os.Getenv("ACMG_CACHE_MAX_ITEMS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.CacheMaxItems = n
		}
	}
	if v := os.Getenv("ACMG_CACHE_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.CacheTTL = d
		}
	}

	// API keys
	cfg.ClinVarAPIKey = os.Getenv("CLINVAR_API_KEY")
	cfg.COSMICAPIKey = os.Getenv("COSMIC_API_KEY")

	// Transport
	if v := os.Getenv("ACMG_TRANSPORT"); v != "" {
		cfg.Transport = v
	}
	if v := os.Getenv("ACMG_HTTP_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.HTTPPort = n
		}
	}

	// Logging
	if v := os.Getenv("ACMG_LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}
	if v := os.Getenv("ACMG_LOG_FORMAT"); v != "" {
		cfg.LogFormat = v
	}

	return cfg
}

// FeedbackDBPath returns the path to the feedback SQLite database.
func (c *LiteConfig) FeedbackDBPath() string {
	return filepath.Join(c.DataDir, "feedback.db")
}

// ExportDir returns the directory for JSON exports.
func (c *LiteConfig) ExportDir() string {
	return filepath.Join(c.DataDir, "exports")
}

// EnsureDataDir creates the data directory if it doesn't exist.
func (c *LiteConfig) EnsureDataDir() error {
	if err := os.MkdirAll(c.DataDir, 0755); err != nil {
		return err
	}
	return os.MkdirAll(c.ExportDir(), 0755)
}
