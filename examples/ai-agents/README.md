# ACMG/AMP MCP Server - AI Agent Integration Examples

This directory contains examples and documentation for integrating the ACMG/AMP MCP Server with various AI agents and systems.

## Quick Start

### Claude Desktop Integration

The ACMG/AMP MCP Server can be integrated with Claude Desktop using any of the three provided configuration examples:

1. **Node.js/JavaScript execution** (`claude-desktop-config.json`)
2. **Binary execution** (`claude-desktop-binary-config.json`)  
3. **Docker container execution** (`claude-desktop-docker-config.json`)

#### Setup Steps

1. **Choose your deployment method** and copy the appropriate config file
2. **Update paths** in the configuration to match your system
3. **Set environment variables** or create a `.env` file with your API keys
4. **Add the configuration** to your Claude Desktop settings file

#### Claude Desktop Configuration File Locations

- **macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`
- **Windows**: `%APPDATA%\Claude\claude_desktop_config.json`
- **Linux**: `~/.config/claude/claude_desktop_config.json`

#### Environment Variables

Required environment variables for all configurations:

```bash
# Database Configuration
DATABASE_URL=postgresql://mcpuser:mcppass@localhost:5432/acmg_amp_mcp

# Redis Configuration (optional, for caching)
REDIS_URL=redis://localhost:6379

# External API Keys (obtain from respective providers)
CLINVAR_API_KEY=your-clinvar-api-key
GNOMAD_API_KEY=your-gnomad-api-key
COSMIC_API_KEY=your-cosmic-api-key

# MCP Transport Configuration
MCP_TRANSPORT=stdio
LOG_LEVEL=info
```

## Available MCP Tools

Once configured, Claude Desktop will have access to the following ACMG/AMP classification tools:

### Core Classification Tools
- `classify_variant` - Complete ACMG/AMP variant classification workflow
- `validate_hgvs` - Validate and normalize HGVS notation
- `apply_rule` - Apply individual ACMG/AMP classification criteria
- `combine_evidence` - Combine evidence according to ACMG/AMP guidelines

### Evidence Gathering Tools
- `query_evidence` - Gather evidence from multiple databases
- `query_clinvar` - Query ClinVar database for variant information
- `query_gnomad` - Query gnomAD population frequency data
- `query_cosmic` - Query COSMIC cancer mutation database

### Report Generation Tools
- `generate_report` - Create comprehensive classification reports
- `format_report` - Format reports in various output formats (JSON, text, PDF)

## Available MCP Resources

The server provides structured access to clinical data through MCP resources:

- `variant/{id}` - Detailed variant information and annotations
- `interpretation/{id}` - Classification results and evidence summaries
- `evidence/{variant_id}` - Aggregated evidence from all databases
- `acmg/rules` - Complete ACMG/AMP classification criteria definitions

## Available MCP Prompts

Pre-configured prompts for clinical workflows:

- `clinical_interpretation` - Systematic variant interpretation workflow
- `evidence_review` - Structured evidence evaluation guidance
- `report_generation` - Clinical report writing assistance
- `acmg_training` - Educational prompts for learning ACMG/AMP guidelines

## Example Usage in Claude Desktop

After configuration, you can interact with the ACMG/AMP server through natural language:

```
"Please classify the variant NM_000492.3:c.1521_1523delCTT using ACMG/AMP guidelines"

"Query ClinVar for information about the BRCA1 variant c.185delA"

"Generate a clinical report for the variant chr17:g.43094692G>A including population frequency and pathogenicity predictions"

"Help me understand the evidence for classifying this variant as likely pathogenic"
```

## Troubleshooting

### Common Issues

1. **Server not starting**: Check that all required dependencies (PostgreSQL, Redis) are running
2. **API key errors**: Ensure all external API keys are properly configured
3. **Permission issues**: Verify that the MCP server binary has execute permissions
4. **Network connectivity**: For Docker configurations, ensure proper network access

### Debug Mode

Enable debug logging by setting `LOG_LEVEL=debug` in your environment configuration.

### Health Check

Test server connectivity:
```bash
# For binary deployment
./bin/mcp-server --health-check

# For Docker deployment  
docker run --rm mcp-acmg-amp-server:latest /app/bin/mcp-server --health-check
```

## Next Steps

- See `workflows/` directory for example clinical workflows
- Check `troubleshooting.md` for detailed problem-solving guides
- Review `chatgpt-integration.md` for ChatGPT MCP integration
- Explore `custom-clients/` for building your own MCP clients