// Package setup provides setup and configuration utilities for the ACMG-AMP MCP Server.
package setup

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// ClaudeDesktopConfig represents the Claude Desktop configuration file structure.
type ClaudeDesktopConfig struct {
	MCPServers map[string]MCPServerConfig `json:"mcpServers"`
}

// MCPServerConfig represents a single MCP server configuration.
type MCPServerConfig struct {
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

// SetupOptions contains options for the setup process.
type SetupOptions struct {
	ServerType  string // "lite" or "full"
	BinaryPath  string // Path to the server binary
	DataDir     string // Data directory for lite server
	AutoConfirm bool   // Skip confirmation prompts
}

// GetClaudeDesktopConfigPath returns the path to Claude Desktop's config file.
func GetClaudeDesktopConfigPath() (string, error) {
	var configDir string

	switch runtime.GOOS {
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		configDir = filepath.Join(home, "Library", "Application Support", "Claude")
	case "linux":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		// Try XDG config first, then fallback
		xdgConfig := os.Getenv("XDG_CONFIG_HOME")
		if xdgConfig != "" {
			configDir = filepath.Join(xdgConfig, "Claude")
		} else {
			configDir = filepath.Join(home, ".config", "Claude")
		}
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			return "", fmt.Errorf("APPDATA environment variable not set")
		}
		configDir = filepath.Join(appData, "Claude")
	default:
		return "", fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}

	return filepath.Join(configDir, "claude_desktop_config.json"), nil
}

// LoadClaudeDesktopConfig loads the existing Claude Desktop configuration.
func LoadClaudeDesktopConfig(configPath string) (*ClaudeDesktopConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty config if file doesn't exist
			return &ClaudeDesktopConfig{
				MCPServers: make(map[string]MCPServerConfig),
			}, nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config ClaudeDesktopConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if config.MCPServers == nil {
		config.MCPServers = make(map[string]MCPServerConfig)
	}

	return &config, nil
}

// SaveClaudeDesktopConfig saves the configuration to the Claude Desktop config file.
func SaveClaudeDesktopConfig(configPath string, config *ClaudeDesktopConfig) error {
	// Ensure directory exists
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// ConfigureClaudeDesktop adds or updates the ACMG-AMP MCP server in Claude Desktop config.
func ConfigureClaudeDesktop(opts SetupOptions) error {
	configPath, err := GetClaudeDesktopConfigPath()
	if err != nil {
		return err
	}

	config, err := LoadClaudeDesktopConfig(configPath)
	if err != nil {
		return err
	}

	// Determine binary path
	binaryPath := opts.BinaryPath
	if binaryPath == "" {
		// Try to find the binary in common locations
		binaryPath, err = findBinary(opts.ServerType)
		if err != nil {
			return fmt.Errorf("could not find server binary: %w", err)
		}
	}

	// Create server configuration
	serverConfig := MCPServerConfig{
		Command: binaryPath,
		Args:    []string{},
		Env:     make(map[string]string),
	}

	// Add environment variables
	if opts.DataDir != "" {
		serverConfig.Env["ACMG_DATA_DIR"] = opts.DataDir
	}

	// Add to config
	serverName := "acmg-amp-classifier"
	config.MCPServers[serverName] = serverConfig

	// Save config
	if err := SaveClaudeDesktopConfig(configPath, config); err != nil {
		return err
	}

	return nil
}

// findBinary attempts to find the server binary in common locations.
func findBinary(serverType string) (string, error) {
	binaryName := "mcp-server-lite"
	if serverType == "full" {
		binaryName = "mcp-server"
	}

	// Check common locations
	locations := []string{
		// Current directory
		"./" + binaryName,
		// Build directory
		"./build/" + binaryName,
		// User's local bin
		filepath.Join(os.Getenv("HOME"), ".local", "bin", binaryName),
		// System paths
		"/usr/local/bin/" + binaryName,
	}

	// Also check PATH
	if path, err := exec.LookPath(binaryName); err == nil {
		return path, nil
	}

	for _, loc := range locations {
		if _, err := os.Stat(loc); err == nil {
			absPath, err := filepath.Abs(loc)
			if err != nil {
				return loc, nil
			}
			return absPath, nil
		}
	}

	return "", fmt.Errorf("binary '%s' not found in common locations", binaryName)
}

// Status represents the current setup status.
type Status struct {
	ClaudeDesktopConfigured bool
	ClaudeDesktopPath       string
	ServerConfigured        bool
	ServerPath              string
	DataDir                 string
	DatabaseConfigured      bool
	Issues                  []string
}

// GetStatus checks the current setup status.
func GetStatus(serverType string) (*Status, error) {
	status := &Status{
		Issues: []string{},
	}

	// Check Claude Desktop config
	configPath, err := GetClaudeDesktopConfigPath()
	if err != nil {
		status.Issues = append(status.Issues, fmt.Sprintf("Could not determine Claude Desktop config path: %v", err))
	} else {
		status.ClaudeDesktopPath = configPath

		config, err := LoadClaudeDesktopConfig(configPath)
		if err != nil {
			status.Issues = append(status.Issues, fmt.Sprintf("Could not load Claude Desktop config: %v", err))
		} else {
			if serverConfig, ok := config.MCPServers["acmg-amp-classifier"]; ok {
				status.ClaudeDesktopConfigured = true
				status.ServerConfigured = true
				status.ServerPath = serverConfig.Command

				// Check if binary exists
				if _, err := os.Stat(serverConfig.Command); os.IsNotExist(err) {
					status.Issues = append(status.Issues, fmt.Sprintf("Server binary not found at: %s", serverConfig.Command))
				}

				// Get data dir from env
				if dataDir, ok := serverConfig.Env["ACMG_DATA_DIR"]; ok {
					status.DataDir = dataDir
				}
			}
		}
	}

	// Check default data directory
	if status.DataDir == "" {
		home, _ := os.UserHomeDir()
		status.DataDir = filepath.Join(home, ".acmg-amp-mcp")
	}

	// Check if data directory exists
	if _, err := os.Stat(status.DataDir); os.IsNotExist(err) {
		status.Issues = append(status.Issues, fmt.Sprintf("Data directory does not exist: %s", status.DataDir))
	}

	return status, nil
}

// Validate checks if the current setup is valid and functional.
func Validate(serverType string) (bool, []string) {
	var issues []string

	// Check Claude Desktop config
	configPath, err := GetClaudeDesktopConfigPath()
	if err != nil {
		issues = append(issues, fmt.Sprintf("Cannot find Claude Desktop config: %v", err))
		return false, issues
	}

	config, err := LoadClaudeDesktopConfig(configPath)
	if err != nil {
		issues = append(issues, fmt.Sprintf("Cannot load Claude Desktop config: %v", err))
		return false, issues
	}

	serverConfig, ok := config.MCPServers["acmg-amp-classifier"]
	if !ok {
		issues = append(issues, "ACMG-AMP classifier not configured in Claude Desktop")
		return false, issues
	}

	// Check binary exists and is executable
	if _, err := os.Stat(serverConfig.Command); os.IsNotExist(err) {
		issues = append(issues, fmt.Sprintf("Server binary not found: %s", serverConfig.Command))
	} else {
		// Try to execute with --version or --help
		cmd := exec.Command(serverConfig.Command, "--help")
		if err := cmd.Run(); err != nil {
			// This might fail if there's no --help flag, which is OK
			// Just check if the file is executable
			info, err := os.Stat(serverConfig.Command)
			if err == nil && info.Mode()&0111 == 0 {
				issues = append(issues, fmt.Sprintf("Server binary is not executable: %s", serverConfig.Command))
			}
		}
	}

	// Check data directory
	dataDir := serverConfig.Env["ACMG_DATA_DIR"]
	if dataDir == "" {
		home, _ := os.UserHomeDir()
		dataDir = filepath.Join(home, ".acmg-amp-mcp")
	}

	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		// Not a critical error - will be created on first run
		issues = append(issues, fmt.Sprintf("Data directory will be created on first run: %s", dataDir))
	}

	return len(issues) == 0 || allWarnings(issues), issues
}

// allWarnings returns true if all issues are just warnings (not errors).
func allWarnings(issues []string) bool {
	for _, issue := range issues {
		if !strings.Contains(issue, "will be created") {
			return false
		}
	}
	return true
}

// GetDefaultDataDir returns the default data directory path.
func GetDefaultDataDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".acmg-amp-mcp")
}

// EnsureDataDir creates the data directory if it doesn't exist.
func EnsureDataDir(dataDir string) error {
	if dataDir == "" {
		dataDir = GetDefaultDataDir()
	}

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// Create subdirectories
	subdirs := []string{"exports"}
	for _, subdir := range subdirs {
		path := filepath.Join(dataDir, subdir)
		if err := os.MkdirAll(path, 0755); err != nil {
			return fmt.Errorf("failed to create %s directory: %w", subdir, err)
		}
	}

	return nil
}
