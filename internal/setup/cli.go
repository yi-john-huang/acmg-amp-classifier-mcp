package setup

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// CLI provides command-line interface for setup operations.
type CLI struct {
	ServerType string // "lite" or "full"
	reader     *bufio.Reader
}

// NewCLI creates a new setup CLI instance.
func NewCLI(serverType string) *CLI {
	return &CLI{
		ServerType: serverType,
		reader:     bufio.NewReader(os.Stdin),
	}
}

// Run executes the setup command based on the provided arguments.
func (c *CLI) Run(args []string) error {
	if len(args) == 0 {
		return c.showHelp()
	}

	switch args[0] {
	case "claude-desktop":
		return c.setupClaudeDesktop(args[1:])
	case "status":
		return c.showStatus()
	case "validate":
		return c.validate()
	case "wizard":
		return c.runWizard()
	case "help", "--help", "-h":
		return c.showHelp()
	default:
		fmt.Printf("Unknown command: %s\n\n", args[0])
		return c.showHelp()
	}
}

// showHelp displays usage information.
func (c *CLI) showHelp() error {
	help := `
ACMG-AMP MCP Server Setup

Usage:
  mcp-server-lite setup <command> [options]

Commands:
  wizard          Interactive setup wizard (recommended for new users)
  claude-desktop  Configure Claude Desktop integration
  status          Show current setup status
  validate        Validate current configuration

Examples:
  # Run interactive setup wizard
  mcp-server-lite setup wizard

  # Configure Claude Desktop with auto-detection
  mcp-server-lite setup claude-desktop

  # Configure with specific binary path
  mcp-server-lite setup claude-desktop --binary /path/to/mcp-server-lite

  # Check current setup status
  mcp-server-lite setup status

  # Validate configuration
  mcp-server-lite setup validate
`
	fmt.Println(help)
	return nil
}

// setupClaudeDesktop configures Claude Desktop integration.
func (c *CLI) setupClaudeDesktop(args []string) error {
	opts := SetupOptions{
		ServerType: c.ServerType,
	}

	// Parse arguments
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--binary", "-b":
			if i+1 < len(args) {
				opts.BinaryPath = args[i+1]
				i++
			}
		case "--data-dir", "-d":
			if i+1 < len(args) {
				opts.DataDir = args[i+1]
				i++
			}
		case "--auto", "-y":
			opts.AutoConfirm = true
		}
	}

	// Get current executable path if not specified
	if opts.BinaryPath == "" {
		execPath, err := os.Executable()
		if err == nil {
			opts.BinaryPath = execPath
		}
	}

	// Show what will be configured
	configPath, _ := GetClaudeDesktopConfigPath()
	fmt.Println("Claude Desktop Configuration")
	fmt.Println("============================")
	fmt.Printf("Config file: %s\n", configPath)
	fmt.Printf("Server binary: %s\n", opts.BinaryPath)
	if opts.DataDir != "" {
		fmt.Printf("Data directory: %s\n", opts.DataDir)
	}
	fmt.Println()

	// Confirm unless auto
	if !opts.AutoConfirm {
		fmt.Print("Proceed with configuration? [Y/n]: ")
		response, _ := c.reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "" && response != "y" && response != "yes" {
			fmt.Println("Configuration cancelled.")
			return nil
		}
	}

	// Configure
	if err := ConfigureClaudeDesktop(opts); err != nil {
		return fmt.Errorf("failed to configure Claude Desktop: %w", err)
	}

	fmt.Println()
	fmt.Println("✓ Claude Desktop configured successfully!")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Restart Claude Desktop to load the new configuration")
	fmt.Println("  2. Ask Claude: \"What MCP tools do you have available?\"")
	fmt.Println("  3. Try: \"Classify the variant BRCA1:c.5266dupC\"")
	fmt.Println()

	return nil
}

// showStatus displays the current setup status.
func (c *CLI) showStatus() error {
	status, err := GetStatus(c.ServerType)
	if err != nil {
		return err
	}

	fmt.Println("ACMG-AMP MCP Server Status")
	fmt.Println("==========================")
	fmt.Println()

	// Claude Desktop
	fmt.Println("Claude Desktop:")
	fmt.Printf("  Config path: %s\n", status.ClaudeDesktopPath)
	if status.ClaudeDesktopConfigured {
		fmt.Println("  Status: ✓ Configured")
	} else {
		fmt.Println("  Status: ✗ Not configured")
	}
	fmt.Println()

	// Server
	fmt.Println("Server:")
	if status.ServerConfigured {
		fmt.Printf("  Binary: %s\n", status.ServerPath)
		if _, err := os.Stat(status.ServerPath); err == nil {
			fmt.Println("  Status: ✓ Found")
		} else {
			fmt.Println("  Status: ✗ Binary not found")
		}
	} else {
		fmt.Println("  Status: ✗ Not configured")
	}
	fmt.Println()

	// Data directory
	fmt.Println("Data Directory:")
	fmt.Printf("  Path: %s\n", status.DataDir)
	if _, err := os.Stat(status.DataDir); err == nil {
		fmt.Println("  Status: ✓ Exists")

		// Check for feedback database
		feedbackDB := filepath.Join(status.DataDir, "feedback.db")
		if _, err := os.Stat(feedbackDB); err == nil {
			fmt.Println("  Feedback DB: ✓ Present")
		} else {
			fmt.Println("  Feedback DB: - Not created yet")
		}
	} else {
		fmt.Println("  Status: - Will be created on first run")
	}
	fmt.Println()

	// Issues
	if len(status.Issues) > 0 {
		fmt.Println("Issues:")
		for _, issue := range status.Issues {
			fmt.Printf("  ⚠ %s\n", issue)
		}
		fmt.Println()
	}

	return nil
}

// validate checks the current configuration.
func (c *CLI) validate() error {
	fmt.Println("Validating configuration...")
	fmt.Println()

	valid, issues := Validate(c.ServerType)

	if valid {
		fmt.Println("✓ Configuration is valid!")
	} else {
		fmt.Println("✗ Configuration has issues:")
		for _, issue := range issues {
			fmt.Printf("  - %s\n", issue)
		}
	}

	return nil
}

// runWizard runs the interactive setup wizard.
func (c *CLI) runWizard() error {
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════╗")
	fmt.Println("║     ACMG-AMP MCP Server - Interactive Setup Wizard       ║")
	fmt.Println("╚══════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Step 1: Check current status
	fmt.Println("Step 1: Checking current setup...")
	status, _ := GetStatus(c.ServerType)
	fmt.Println()

	if status.ClaudeDesktopConfigured {
		fmt.Println("✓ Claude Desktop is already configured!")
		fmt.Print("Would you like to reconfigure? [y/N]: ")
		response, _ := c.reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			fmt.Println()
			fmt.Println("Setup complete. Your server is ready to use!")
			return nil
		}
	}

	// Step 2: Configure Claude Desktop
	fmt.Println()
	fmt.Println("Step 2: Configure Claude Desktop")
	fmt.Println("---------------------------------")

	// Get binary path
	execPath, _ := os.Executable()
	fmt.Printf("Server binary path [%s]: ", execPath)
	binaryPath, _ := c.reader.ReadString('\n')
	binaryPath = strings.TrimSpace(binaryPath)
	if binaryPath == "" {
		binaryPath = execPath
	}

	// Validate binary exists
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		fmt.Printf("⚠ Warning: Binary not found at %s\n", binaryPath)
		fmt.Print("Continue anyway? [y/N]: ")
		response, _ := c.reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			return fmt.Errorf("setup cancelled")
		}
	}

	// Get data directory
	defaultDataDir := GetDefaultDataDir()
	fmt.Printf("Data directory [%s]: ", defaultDataDir)
	dataDir, _ := c.reader.ReadString('\n')
	dataDir = strings.TrimSpace(dataDir)
	if dataDir == "" {
		dataDir = defaultDataDir
	}

	// Step 3: Apply configuration
	fmt.Println()
	fmt.Println("Step 3: Applying configuration...")

	opts := SetupOptions{
		ServerType: c.ServerType,
		BinaryPath: binaryPath,
		DataDir:    dataDir,
	}

	if err := ConfigureClaudeDesktop(opts); err != nil {
		return fmt.Errorf("failed to configure: %w", err)
	}

	// Create data directory
	if err := EnsureDataDir(dataDir); err != nil {
		fmt.Printf("⚠ Warning: Could not create data directory: %v\n", err)
	}

	// Step 4: Success
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════╗")
	fmt.Println("║                    Setup Complete! ✓                     ║")
	fmt.Println("╚══════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Restart Claude Desktop to load the new configuration")
	fmt.Println("  2. Start a new conversation with Claude")
	fmt.Println("  3. Try asking: \"Classify the variant BRCA1:c.5266dupC\"")
	fmt.Println()
	fmt.Println("For help, run: mcp-server-lite setup --help")
	fmt.Println()

	return nil
}
