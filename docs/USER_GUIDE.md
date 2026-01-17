# ACMG-AMP MCP Server - User Guide

This guide provides step-by-step instructions for installing, configuring, and using the ACMG-AMP MCP Server with Claude Desktop.

## Table of Contents

1. [Overview](#overview)
2. [System Requirements](#system-requirements)
3. [Installation](#installation)
   - [Quick Install (Recommended)](#quick-install-recommended)
   - [Manual Installation](#manual-installation)
   - [Build from Source](#build-from-source)
   - [Docker Installation](#docker-installation)
4. [Configuration](#configuration)
   - [Setup Wizard](#setup-wizard)
   - [Manual Configuration](#manual-configuration)
   - [Environment Variables](#environment-variables)
5. [Using with Claude Desktop](#using-with-claude-desktop)
   - [Verifying the Connection](#verifying-the-connection)
   - [Example Conversations](#example-conversations)
6. [Available Tools](#available-tools)
7. [Available Skills](#available-skills)
8. [Feedback System](#feedback-system)
9. [Troubleshooting](#troubleshooting)
10. [Upgrading](#upgrading)
11. [Uninstallation](#uninstallation)

---

## Overview

The ACMG-AMP MCP Server enables Claude to perform professional-grade genetic variant classification using the ACMG/AMP 2015 guidelines. Once installed, you can ask Claude to:

- Classify genetic variants using standardized ACMG/AMP criteria
- Query evidence from 6 major databases (ClinVar, gnomAD, COSMIC, etc.)
- Generate clinical interpretation reports
- Apply specific ACMG/AMP rules to variants

**Important**: This software is for research and educational purposes only. It is NOT approved for clinical use.

---

## System Requirements

### Lite Server (Recommended)

| Requirement | Specification |
|-------------|---------------|
| Operating System | macOS 10.15+, Linux (Ubuntu 18.04+, Debian 10+), Windows 10+ |
| Architecture | Intel x64 (amd64) or Apple Silicon/ARM64 |
| Disk Space | ~50 MB for binary, ~100 MB for data |
| Memory | 256 MB minimum |
| Claude Desktop | Latest version |

### Full Server (Production)

| Requirement | Specification |
|-------------|---------------|
| All Lite requirements | Plus: |
| PostgreSQL | 15+ |
| Redis | 7+ |
| Docker | 20.10+ (optional) |

---

## Installation

### Quick Install (Recommended)

The fastest way to get started is using the one-liner installation script:

#### macOS / Linux

Open Terminal and run:

```bash
curl -fsSL https://raw.githubusercontent.com/yi-john-huang/acmg-amp-classifier-mcp/main/scripts/install.sh | bash
```

This script will:
1. Detect your platform (macOS/Linux, Intel/ARM)
2. Download the appropriate binary
3. Install to `~/.local/bin/`
4. Add the directory to your PATH
5. Launch the interactive setup wizard

#### What to Expect

```
╔══════════════════════════════════════════════════════════╗
║        ACMG-AMP MCP Server - Installation Script         ║
╚══════════════════════════════════════════════════════════╝

Detected platform: darwin/arm64
Fetching latest version...
Latest version: v1.0.0
Downloading from: https://github.com/...
Binary installed to: /Users/you/.local/bin/mcp-server-lite

Installation complete!

Would you like to run the setup wizard now? [Y/n]:
```

Press Enter (or type `Y`) to continue with the setup wizard.

---

### Manual Installation

If you prefer to install manually:

#### Step 1: Download the Binary

**macOS (Apple Silicon / M1/M2/M3):**
```bash
curl -L -o mcp-server-lite \
  https://github.com/yi-john-huang/acmg-amp-classifier-mcp/releases/latest/download/mcp-server-lite-darwin-arm64
chmod +x mcp-server-lite
```

**macOS (Intel):**
```bash
curl -L -o mcp-server-lite \
  https://github.com/yi-john-huang/acmg-amp-classifier-mcp/releases/latest/download/mcp-server-lite-darwin-amd64
chmod +x mcp-server-lite
```

**Linux (x64):**
```bash
curl -L -o mcp-server-lite \
  https://github.com/yi-john-huang/acmg-amp-classifier-mcp/releases/latest/download/mcp-server-lite-linux-amd64
chmod +x mcp-server-lite
```

**Linux (ARM64):**
```bash
curl -L -o mcp-server-lite \
  https://github.com/yi-john-huang/acmg-amp-classifier-mcp/releases/latest/download/mcp-server-lite-linux-arm64
chmod +x mcp-server-lite
```

#### Step 2: Move to a Directory in PATH

```bash
# Create local bin directory if it doesn't exist
mkdir -p ~/.local/bin

# Move the binary
mv mcp-server-lite ~/.local/bin/

# Add to PATH (add this to your ~/.zshrc or ~/.bashrc)
export PATH="$PATH:$HOME/.local/bin"

# Reload your shell configuration
source ~/.zshrc  # or source ~/.bashrc
```

#### Step 3: Run Setup

```bash
mcp-server-lite setup wizard
```

---

### Build from Source

If you want to build from source:

#### Prerequisites

- Go 1.24 or later
- Git

#### Steps

```bash
# Clone the repository
git clone https://github.com/yi-john-huang/acmg-amp-classifier-mcp.git
cd acmg-amp-classifier-mcp

# Build the lite server
go build -o mcp-server-lite ./cmd/mcp-server-lite/

# Move to PATH
mv mcp-server-lite ~/.local/bin/

# Run setup
mcp-server-lite setup wizard
```

---

### Docker Installation

For containerized deployment:

```bash
# Clone the repository
git clone https://github.com/yi-john-huang/acmg-amp-classifier-mcp.git
cd acmg-amp-classifier-mcp

# Build the Docker image
docker build -f Dockerfile.lite -t acmg-amp-mcp-lite .

# Run interactively (for testing)
docker run -it --rm acmg-amp-mcp-lite
```

For Claude Desktop integration with Docker, see the [Docker Configuration](#docker-configuration) section.

---

## Configuration

### Setup Wizard

The setup wizard guides you through configuration step by step:

```bash
mcp-server-lite setup wizard
```

#### Wizard Steps

```
╔══════════════════════════════════════════════════════════╗
║     ACMG-AMP MCP Server - Interactive Setup Wizard       ║
╚══════════════════════════════════════════════════════════╝

Step 1: Checking current setup...

Step 2: Configure Claude Desktop
---------------------------------
Server binary path [/Users/you/.local/bin/mcp-server-lite]:
Data directory [/Users/you/.acmg-amp-mcp]:

Step 3: Applying configuration...

╔══════════════════════════════════════════════════════════╗
║                    Setup Complete! ✓                     ║
╚══════════════════════════════════════════════════════════╝

Next steps:
  1. Restart Claude Desktop to load the new configuration
  2. Start a new conversation with Claude
  3. Try asking: "Classify the variant BRCA1:c.5266dupC"
```

### Other Setup Commands

| Command | Description |
|---------|-------------|
| `mcp-server-lite setup wizard` | Interactive setup (recommended) |
| `mcp-server-lite setup claude-desktop` | Configure Claude Desktop only |
| `mcp-server-lite setup status` | Show current configuration |
| `mcp-server-lite setup validate` | Validate configuration works |

#### Setup Status Example

```bash
$ mcp-server-lite setup status

ACMG-AMP MCP Server Status
==========================

Claude Desktop:
  Config path: /Users/you/Library/Application Support/Claude/claude_desktop_config.json
  Status: ✓ Configured

Server:
  Binary: /Users/you/.local/bin/mcp-server-lite
  Status: ✓ Found

Data Directory:
  Path: /Users/you/.acmg-amp-mcp
  Status: ✓ Exists
  Feedback DB: ✓ Present
```

---

### Manual Configuration

If you prefer to configure Claude Desktop manually:

#### Step 1: Locate the Configuration File

| Platform | Path |
|----------|------|
| macOS | `~/Library/Application Support/Claude/claude_desktop_config.json` |
| Linux | `~/.config/Claude/claude_desktop_config.json` |
| Windows | `%APPDATA%\Claude\claude_desktop_config.json` |

#### Step 2: Edit the Configuration

Open the file and add the ACMG-AMP server configuration:

```json
{
  "mcpServers": {
    "acmg-amp-classifier": {
      "command": "/Users/you/.local/bin/mcp-server-lite",
      "args": [],
      "env": {
        "ACMG_DATA_DIR": "/Users/you/.acmg-amp-mcp"
      }
    }
  }
}
```

**Important**: Replace `/Users/you` with your actual home directory path.

#### Step 3: Restart Claude Desktop

Quit Claude Desktop completely and reopen it.

---

### Environment Variables

The server supports the following environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `ACMG_DATA_DIR` | `~/.acmg-amp-mcp` | Directory for data storage |
| `ACMG_TRANSPORT` | `stdio` | Transport type: `stdio` or `http` |
| `ACMG_HTTP_PORT` | `8080` | HTTP port (if transport is http) |
| `ACMG_LOG_LEVEL` | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `ACMG_CACHE_MAX_ITEMS` | `1000` | Maximum items in memory cache |
| `ACMG_CACHE_TTL` | `24h` | Cache time-to-live |
| `CLINVAR_API_KEY` | *(none)* | NCBI API key for higher rate limits |
| `COSMIC_API_KEY` | *(none)* | COSMIC API key |

To set environment variables in Claude Desktop config:

```json
{
  "mcpServers": {
    "acmg-amp-classifier": {
      "command": "/path/to/mcp-server-lite",
      "args": [],
      "env": {
        "ACMG_DATA_DIR": "/custom/data/path",
        "ACMG_LOG_LEVEL": "debug",
        "CLINVAR_API_KEY": "your-ncbi-api-key"
      }
    }
  }
}
```

---

## Using with Claude Desktop

### Verifying the Connection

After restarting Claude Desktop, verify the server is connected:

1. **Start a new conversation** with Claude
2. **Ask**: "What MCP tools do you have available?"

Claude should respond listing the ACMG-AMP tools, including:
- `classify_variant`
- `validate_hgvs`
- `query_evidence`
- `generate_report`
- And more...

If Claude doesn't list these tools, see [Troubleshooting](#troubleshooting).

---

### Example Conversations

#### Basic Variant Classification

**You**: Classify the variant BRCA1:c.5266dupC using ACMG/AMP guidelines.

**Claude**: I'll classify this variant using the ACMG/AMP guidelines...

[Claude will use the `classify_variant` tool and provide a detailed classification]

---

#### HGVS Notation Classification

**You**: What is the ACMG classification for NM_000492.3:c.1521_1523delCTT?

**Claude**: Let me analyze this CFTR variant...

[Claude will classify the ΔF508 mutation]

---

#### Evidence Query

**You**: What evidence is available for TP53:p.R273H from ClinVar and gnomAD?

**Claude**: I'll gather evidence for this TP53 variant...

[Claude will query databases and summarize findings]

---

#### Specific Rule Application

**You**: Does the variant BRCA1:c.5266dupC meet the PVS1 criteria?

**Claude**: I'll apply the PVS1 rule to this variant...

[Claude will explain whether the null variant criterion is met]

---

#### Generate Clinical Report

**You**: Generate a clinical interpretation report for CFTR:c.1521_1523del

**Claude**: I'll create a comprehensive clinical report...

[Claude will generate a formatted report suitable for clinical documentation]

---

#### Batch Classification

**You**: Classify these variants and compare them:
1. BRCA1:c.5266dupC
2. TP53:p.R273H
3. CFTR:c.1521_1523del

**Claude**: I'll classify each variant and provide a comparison...

[Claude will classify all three and summarize the results]

---

## Available Tools

The server provides 17 MCP tools organized by category:

### Core Classification Tools

| Tool | Description |
|------|-------------|
| `classify_variant` | Complete ACMG/AMP classification workflow |
| `validate_hgvs` | Validate and normalize HGVS notation |
| `apply_rule` | Apply specific ACMG/AMP rule (e.g., PVS1, PS1) |
| `combine_evidence` | Combine rule results into final classification |

### Evidence Gathering Tools

| Tool | Description |
|------|-------------|
| `query_evidence` | Query all 6 databases for variant evidence |
| `batch_query_evidence` | Batch query with caching for multiple variants |
| `query_clinvar` | Search ClinVar for clinical significance |
| `query_gnomad` | Get population frequency from gnomAD |
| `query_cosmic` | Search COSMIC for somatic mutations |

### Report Generation Tools

| Tool | Description |
|------|-------------|
| `generate_report` | Create clinical interpretation report |
| `format_report` | Export report in different formats |
| `validate_report` | Quality assurance for reports |

### Feedback Tools

| Tool | Description |
|------|-------------|
| `submit_feedback` | Save classification correction/agreement |
| `query_feedback` | Check previous feedback for a variant |
| `list_feedback` | List all stored feedback |
| `export_feedback` | Export feedback to JSON file |
| `import_feedback` | Import feedback from JSON file |

---

## Available Skills

Skills are slash commands that orchestrate multi-step workflows:

### /classify

Full ACMG/AMP classification workflow.

```
/classify NM_000492.3:c.1521_1523delCTT
/classify BRCA1:c.5266dupC
/classify TP53:p.R273H --report
```

### /batch

Process multiple variants with progress tracking.

```
/batch CFTR:c.1521_1523del, BRCA1:c.5266dupC, TP53:p.R273H
/batch --detailed
```

---

## Feedback System

The feedback system allows you to record your agreement or corrections to classifications:

### Submitting Feedback

**You**: I agree with the Pathogenic classification for BRCA1:c.5266dupC

**Claude**: [Uses `submit_feedback` to record your agreement]

---

**You**: I disagree with the VUS classification for TP53:p.R175H - it should be Likely Pathogenic based on functional studies

**Claude**: [Uses `submit_feedback` to record your correction with notes]

---

### Querying Previous Feedback

**You**: Do we have any previous feedback for BRCA1:c.5266dupC?

**Claude**: [Uses `query_feedback` to check and report any previous feedback]

---

### Exporting Feedback

**You**: Export all our classification feedback to a backup file

**Claude**: [Uses `export_feedback` to create a JSON backup]

---

## Troubleshooting

### Claude doesn't see the MCP tools

**Symptoms**: Claude says it doesn't have access to ACMG tools

**Solutions**:

1. **Check configuration file**:
   ```bash
   mcp-server-lite setup status
   ```

2. **Verify the binary exists and is executable**:
   ```bash
   ls -la ~/.local/bin/mcp-server-lite
   ```

3. **Check Claude Desktop config syntax**:
   ```bash
   cat ~/Library/Application\ Support/Claude/claude_desktop_config.json | python3 -m json.tool
   ```

4. **Restart Claude Desktop completely** (Cmd+Q on macOS, not just close window)

---

### "Command not found" error

**Symptoms**: Running `mcp-server-lite` gives "command not found"

**Solutions**:

1. **Check if binary is in PATH**:
   ```bash
   which mcp-server-lite
   ```

2. **Add to PATH** (add to your `~/.zshrc` or `~/.bashrc`):
   ```bash
   export PATH="$PATH:$HOME/.local/bin"
   ```

3. **Reload shell**:
   ```bash
   source ~/.zshrc  # or ~/.bashrc
   ```

---

### Classification seems slow

**Symptoms**: Claude takes a long time to classify variants

**Solutions**:

1. **Check network connection**: The server queries external databases

2. **Add API keys** for higher rate limits:
   ```json
   {
     "env": {
       "CLINVAR_API_KEY": "your-ncbi-api-key"
     }
   }
   ```

3. **Enable debug logging** to see what's happening:
   ```json
   {
     "env": {
       "ACMG_LOG_LEVEL": "debug"
     }
   }
   ```

---

### Data directory permission errors

**Symptoms**: Errors about unable to write to data directory

**Solutions**:

1. **Check directory exists and is writable**:
   ```bash
   mkdir -p ~/.acmg-amp-mcp
   chmod 755 ~/.acmg-amp-mcp
   ```

2. **Specify a different data directory**:
   ```json
   {
     "env": {
       "ACMG_DATA_DIR": "/path/to/writable/directory"
     }
   }
   ```

---

### Getting Help

If you're still having issues:

1. **Check the logs**: Enable debug logging and check Claude Desktop's developer console
2. **Validate configuration**: Run `mcp-server-lite setup validate`
3. **Report an issue**: https://github.com/yi-john-huang/acmg-amp-classifier-mcp/issues

---

## Upgrading

### Using Install Script

Re-run the installation script to upgrade:

```bash
curl -fsSL https://raw.githubusercontent.com/yi-john-huang/acmg-amp-classifier-mcp/main/scripts/install.sh | bash
```

### Manual Upgrade

1. Download the new binary (same as installation)
2. Replace the old binary:
   ```bash
   mv mcp-server-lite ~/.local/bin/mcp-server-lite
   ```
3. Restart Claude Desktop

### Checking Version

```bash
mcp-server-lite --version
```

---

## Uninstallation

### Remove the Binary

```bash
rm ~/.local/bin/mcp-server-lite
```

### Remove Data Directory (optional)

```bash
rm -rf ~/.acmg-amp-mcp
```

### Remove from Claude Desktop Config

Edit `~/Library/Application Support/Claude/claude_desktop_config.json` and remove the `acmg-amp-classifier` entry from `mcpServers`.

### Remove from PATH (if added manually)

Edit your `~/.zshrc` or `~/.bashrc` and remove the PATH export line.

---

## Support

- **Documentation**: https://github.com/yi-john-huang/acmg-amp-classifier-mcp
- **Issues**: https://github.com/yi-john-huang/acmg-amp-classifier-mcp/issues
- **License**: Non-Commercial License (see LICENSE file)

---

## Disclaimer

**This software is for research and educational purposes only.**

- NOT approved for clinical use or patient care
- NOT a medical device or diagnostic tool
- Requires additional validation for clinical settings
- Should not be used as sole basis for medical decisions
- Requires regulatory approval for clinical use

Any clinical application requires appropriate medical oversight, validation studies, and regulatory compliance.
