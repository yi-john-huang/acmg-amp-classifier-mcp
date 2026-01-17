package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultLiteConfig(t *testing.T) {
	cfg := DefaultLiteConfig()

	assert.NotEmpty(t, cfg.DataDir)
	assert.Equal(t, 1000, cfg.CacheMaxItems)
	assert.Equal(t, 24*time.Hour, cfg.CacheTTL)
	assert.Equal(t, "stdio", cfg.Transport)
	assert.Equal(t, 8080, cfg.HTTPPort)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, "json", cfg.LogFormat)
}

func TestLoadLiteConfig_Defaults(t *testing.T) {
	// Clear relevant env vars
	clearEnvVars(t)

	cfg := LoadLiteConfig()

	assert.NotEmpty(t, cfg.DataDir)
	assert.Equal(t, 1000, cfg.CacheMaxItems)
	assert.Equal(t, "stdio", cfg.Transport)
}

func TestLoadLiteConfig_EnvironmentOverrides(t *testing.T) {
	clearEnvVars(t)

	// Set environment variables
	os.Setenv("ACMG_DATA_DIR", "/tmp/test-acmg")
	os.Setenv("ACMG_CACHE_MAX_ITEMS", "500")
	os.Setenv("ACMG_CACHE_TTL", "12h")
	os.Setenv("ACMG_TRANSPORT", "http")
	os.Setenv("ACMG_HTTP_PORT", "9090")
	os.Setenv("ACMG_LOG_LEVEL", "debug")
	os.Setenv("CLINVAR_API_KEY", "test-key")

	defer clearEnvVars(t)

	cfg := LoadLiteConfig()

	assert.Equal(t, "/tmp/test-acmg", cfg.DataDir)
	assert.Equal(t, 500, cfg.CacheMaxItems)
	assert.Equal(t, 12*time.Hour, cfg.CacheTTL)
	assert.Equal(t, "http", cfg.Transport)
	assert.Equal(t, 9090, cfg.HTTPPort)
	assert.Equal(t, "debug", cfg.LogLevel)
	assert.Equal(t, "test-key", cfg.ClinVarAPIKey)
}

func TestLiteConfig_FeedbackDBPath(t *testing.T) {
	cfg := &LiteConfig{DataDir: "/home/user/.acmg-amp-mcp"}

	path := cfg.FeedbackDBPath()

	assert.Equal(t, "/home/user/.acmg-amp-mcp/feedback.db", path)
}

func TestLiteConfig_ExportDir(t *testing.T) {
	cfg := &LiteConfig{DataDir: "/home/user/.acmg-amp-mcp"}

	path := cfg.ExportDir()

	assert.Equal(t, "/home/user/.acmg-amp-mcp/exports", path)
}

func TestLiteConfig_EnsureDataDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "config-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	cfg := &LiteConfig{DataDir: filepath.Join(tmpDir, "acmg")}

	err = cfg.EnsureDataDir()
	require.NoError(t, err)

	// Verify directories exist
	_, err = os.Stat(cfg.DataDir)
	assert.NoError(t, err)

	_, err = os.Stat(cfg.ExportDir())
	assert.NoError(t, err)
}

func clearEnvVars(t *testing.T) {
	t.Helper()
	vars := []string{
		"ACMG_DATA_DIR",
		"ACMG_CACHE_MAX_ITEMS",
		"ACMG_CACHE_TTL",
		"ACMG_TRANSPORT",
		"ACMG_HTTP_PORT",
		"ACMG_LOG_LEVEL",
		"ACMG_LOG_FORMAT",
		"CLINVAR_API_KEY",
		"COSMIC_API_KEY",
	}
	for _, v := range vars {
		os.Unsetenv(v)
	}
}
