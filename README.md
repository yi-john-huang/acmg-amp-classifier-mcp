# ACMG-AMP MCP Server: AI-Powered Variant Classification

**(Project Status: Production Ready - MCP Integration Complete)**

🔬 **Research & Educational Use Only** | ⚖️ **Non-Commercial License** | 🏥 **Not for Clinical Use**

## Overview

The ACMG-AMP MCP Server is a **Model Context Protocol (MCP)** compliant service that provides AI agents like Claude with direct access to professional-grade genetic variant classification tools. It implements the complete **ACMG/AMP 2015 guidelines** with all 28 evidence criteria, enabling AI assistants to perform standardized variant interpretation through natural language interactions.

**🚀 Key Features:**
- **Native MCP Integration**: Direct tool access for Claude, ChatGPT, and other MCP-compatible AI agents
- **Complete ACMG/AMP Implementation**: All 28 rules (PVS1-BP7) with evidence combination logic
- **Gene Symbol Query Support**: Input variants using gene symbols (e.g., "BRCA1:c.123A>G") with automatic transcript resolution
- **6 External Database Sources**: ClinVar, gnomAD, COSMIC, PubMed, LOVD, HGMD integration
- **9 Gene Database APIs**: HGNC, RefSeq, Ensembl for gene symbol validation and transcript mapping
- **Real-time Classification**: Full workflow from HGVS or gene symbol input to clinical recommendations
- **Production-grade Architecture**: PostgreSQL database, Redis caching, comprehensive logging

## Purpose

The ACMG-AMP MCP Server enables:

1. **AI-Native Genetic Analysis**: Claude and other AI agents can directly access professional genetic tools
2. **Standardized ACMG/AMP Classification**: All 28 evidence criteria implemented with proper combination logic
3. **Evidence-Based Interpretation**: Automated evidence gathering from 6 major databases (ClinVar, gnomAD, etc.)
4. **Natural Language Interface**: Ask Claude about variants in plain English and get structured clinical reports
5. **Reproducible Results**: Consistent application of ACMG/AMP guidelines across all analyses

## 🎯 Implementation Status: **PRODUCTION READY**

### ✅ **MCP Core (100% Complete)**
- **MCP Protocol Integration**: Full JSON-RPC 2.0 compliance with Go SDK
- **Transport Layer**: Stdio and HTTP-SSE transport for AI agent connectivity
- **Tool Registry**: All ACMG/AMP tools registered and functional
- **Session Management**: Client tracking, rate limiting, and graceful shutdown

### ✅ **ACMG/AMP Classification Engine (100% Complete)**
- **All 28 ACMG/AMP Rules**: PVS1, PS1-PS4, PM1-PM6, PP1-PP5, BA1, BS1-BS4, BP1-BP7
- **Evidence Combination Logic**: Complete 2015 ACMG/AMP guidelines implementation
- **Classification Service**: Full workflow orchestration with confidence assessment
- **HGVS Parser**: Medical-grade variant notation validation and normalization

### ✅ **External Database Integration (100% Complete)**
- **6 Major Databases**: ClinVar, gnomAD, COSMIC, PubMed, LOVD, HGMD clients
- **Resilient Architecture**: Circuit breakers, retry logic, fallback mechanisms
- **Evidence Aggregation**: Automated gathering and quality scoring
- **Caching Layer**: Optimized response times with intelligent cache invalidation

### ✅ **Production Infrastructure (100% Complete)**
- **PostgreSQL Database**: Advanced schema with JSONB support and audit trails
- **Comprehensive Logging**: Structured logging with correlation IDs
- **Health Monitoring**: Service and dependency health checks
- **Container Support**: Docker and Kubernetes deployment ready

## 🛠️ Available MCP Tools

The server provides these tools that AI agents can access directly:

### **Core Classification Tools**
- **`classify_variant`**: Complete ACMG/AMP workflow - input HGVS notation, get full classification report
- **`validate_hgvs`**: Validate and normalize HGVS variant notation
- **`apply_rule`**: Apply specific ACMG/AMP rules (e.g., PVS1, PS1) to a variant
- **`combine_evidence`**: Combine multiple rule results using ACMG/AMP guidelines

### **Evidence Gathering Tools**
- **`query_evidence`**: Gather evidence from all 6 external databases
- **`query_clinvar`**: Search ClinVar for variant clinical significance
- **`query_gnomad`**: Get population frequency data from gnomAD
- **`query_cosmic`**: Search COSMIC for somatic mutation data

### **Report Generation Tools**
- **`generate_report`**: Create structured clinical interpretation reports
- **`format_report`**: Export reports in multiple formats (JSON, text, PDF)
- **`validate_report`**: Quality assurance for generated reports

## 🏗️ MCP Architecture

The server implements the **Model Context Protocol (MCP)** for direct AI agent integration:

```mermaid
graph TD
    subgraph "AI Agent Layer"
        Claude[Claude Desktop/API] 
        ChatGPT[ChatGPT with MCP]
        CustomAI[Custom MCP Client]
    end

    subgraph "MCP Server Layer"
        Transport[MCP Transport<br/>stdio/HTTP-SSE]
        Protocol[JSON-RPC 2.0<br/>Protocol Handler]
        ToolRegistry[MCP Tool Registry]
    end

    subgraph "Classification Engine"
        ClassifierService[ACMG/AMP<br/>Classifier Service]
        RuleEngine[28 ACMG/AMP Rules<br/>PVS1-BP7]
        EvidenceCombiner[Evidence Combination<br/>Logic]
    end

    subgraph "Evidence Layer"
        ClinVar[ClinVar API]
        gnomAD[gnomAD API]
        COSMIC[COSMIC API]
        PubMed[PubMed API]
        LOVD[LOVD API]
        HGMD[HGMD API]
    end

    subgraph "Data Layer"
        PostgreSQL[(PostgreSQL<br/>Variants & Results)]
        Redis[(Redis<br/>API Cache)]
    end

    Claude --> Transport
    ChatGPT --> Transport
    CustomAI --> Transport
    
    Transport --> Protocol
    Protocol --> ToolRegistry
    
    ToolRegistry --> ClassifierService
    ClassifierService --> RuleEngine
    ClassifierService --> EvidenceCombiner
    
    ClassifierService --> ClinVar
    ClassifierService --> gnomAD
    ClassifierService --> COSMIC
    ClassifierService --> PubMed
    ClassifierService --> LOVD
    ClassifierService --> HGMD
    
    ClassifierService --> PostgreSQL
    ClassifierService --> Redis
```

## Target Audience

* Clinical Geneticists
* Molecular Pathologists
* Oncologists
* Genetic Counselors
* Bioinformaticians
* Medical Researchers

## Project Structure

```
/
├── cmd/                          # Main applications
│   └── mcp-server/              # MCP server entry point
├── internal/                    # Private application code
│   ├── config/                 # Configuration management
│   ├── domain/                 # Business logic and entities
│   ├── mcp/                    # MCP protocol implementation
│   │   ├── protocol/          # JSON-RPC 2.0 protocol core
│   │   ├── transport/         # Transport layer (stdio/HTTP-SSE)
│   │   ├── tools/             # ACMG/AMP tool implementations
│   │   ├── resources/         # MCP resource providers
│   │   └── prompts/           # MCP prompt templates
│   └── service/               # Application services
├── pkg/                        # Public library code
│   └── external/              # External API clients (6 databases)
├── deployments/               # Deployment configurations
│   └── kubernetes/           # Kubernetes manifests
├── docs/                      # Documentation
├── examples/                  # Usage examples and integrations
│   └── ai-agents/            # AI agent integration examples
├── scripts/                   # Deployment and utility scripts
├── .env.example              # Environment configuration template
├── docker-compose.yml        # Docker Compose configuration
└── Dockerfile               # Container build configuration
```

## Core Interfaces

The service is built around well-defined interfaces:

- **APIGateway**: HTTP request handling and coordination
- **InputParser**: HGVS validation and variant standardization  
- **InterpretationEngine**: ACMG/AMP rule application and classification
- **KnowledgeBaseAccess**: External database integration
- **ReportGenerator**: Structured report generation

## 🚀 Quick Start Guide

### Prerequisites
- **Go 1.21+** - For building the MCP server
- **PostgreSQL 15+** - For variant and results storage  
- **Docker & Docker Compose** - For easy deployment

### 📦 Method 1: Docker Deployment (Recommended)

1. **Clone and configure**
   ```bash
   git clone https://github.com/your-username/acmg-amp-classifier-mcp.git
   cd acmg-amp-classifier-mcp
   
   # Copy and edit environment configuration
   cp .env.example .env
   ```

2. **Configure environment variables**
   
   Edit `.env` file with your secure credentials:
   ```bash
   # Required: Set secure passwords
   POSTGRES_PASSWORD=your_secure_postgres_password_here
   REDIS_PASSWORD=your_secure_redis_password_here
   
   # Optional: External database API keys (for production use)
   CLINVAR_API_KEY=your_ncbi_api_key_here
   GNOMAD_API_KEY=your_gnomad_api_key_here  
   COSMIC_USERNAME=your_cosmic_username_here
   COSMIC_PASSWORD=your_cosmic_password_here
   
   # Optional: Adjust ports if needed
   POSTGRES_PORT=5432
   REDIS_PORT=6379
   MCP_HTTP_PORT=8080
   ```

3. **Start the services**
   ```bash
   # Start PostgreSQL, Redis, and MCP server
   docker-compose up -d
   
   # Check all services are running
   docker-compose ps
   
   # Check server health
   curl http://localhost:8080/health
   
   # View logs (optional)
   docker-compose logs -f mcp-server
   ```

4. **Verify deployment**
   ```bash
   # Test database connection
   docker-compose exec postgres psql -U mcpuser -d acmg_amp_mcp -c "SELECT version();"
   
   # Test Redis connection  
   docker-compose exec redis redis-cli -a $REDIS_PASSWORD ping
   
   # Test MCP server tools
   curl http://localhost:8080/tools/list
   ```

5. **Configure Claude Desktop**
   
   Add to your Claude Desktop MCP settings (`~/Library/Application Support/Claude/claude_desktop_config.json`):
   ```json
   {
     "mcpServers": {
       "acmg-amp-classifier": {
         "command": "docker",
         "args": ["exec", "acmg-amp-mcp-server", "/app/mcp-server"],
         "env": {}
       }
     }
   }
   ```

6. **Test with Claude**
   
   Ask Claude: *"Can you classify the variant NM_000492.3:c.1521_1523delCTT using ACMG/AMP guidelines?"*

### 🔧 Docker Compose Services

The provided `docker-compose.yml` includes:

**PostgreSQL Database (`postgres`)**
- Image: `postgres:15-alpine`
- Port: `5432` (configurable via `POSTGRES_PORT`)
- Persistent storage with health checks
- Automatic database initialization scripts
- Resource limits for production use

**Redis Cache (`redis`)**  
- Image: `redis:7-alpine`
- Port: `6379` (configurable via `REDIS_PORT`)
- Password-protected with persistence enabled
- Memory limit (512MB) with LRU eviction policy
- Append-only file for data durability

**MCP Server (`mcp-server`)**
- Built from local Dockerfile
- Port: `8080` (configurable via `MCP_HTTP_PORT`)  
- Automatic database migrations on startup
- Health checks and restart policies
- Comprehensive logging and monitoring

**Key Features:**
- **Production-ready**: Resource limits, health checks, restart policies
- **Secure**: Password-protected services, non-root users
- **Persistent**: Data volumes for PostgreSQL and Redis
- **Scalable**: Network isolation and configurable resources
- **Monitored**: Health checks and logging integration

### 🛠️ Environment Variables Reference

The `.env` file supports comprehensive configuration:

```bash
# =============================================================================
# Database Configuration
# =============================================================================
POSTGRES_DB=acmg_amp_mcp              # Database name
POSTGRES_USER=mcpuser                 # Database username  
POSTGRES_PASSWORD=secure_password     # Database password (REQUIRED)
POSTGRES_PORT=5432                    # Database port

# =============================================================================
# Redis Configuration  
# =============================================================================
REDIS_PASSWORD=secure_redis_password  # Redis password (REQUIRED)
REDIS_PORT=6379                       # Redis port

# =============================================================================
# MCP Server Configuration
# =============================================================================
MCP_TRANSPORT=http                    # Transport type (http/stdio)
MCP_HTTP_PORT=8080                    # HTTP server port
MCP_LOG_LEVEL=info                    # Log level (debug/info/warn/error)
MCP_MAX_CONNECTIONS=1000              # Max concurrent connections
MCP_CACHE_ENABLED=true                # Enable caching

# =============================================================================
# External Database APIs (Optional - for enhanced functionality)
# =============================================================================
CLINVAR_API_KEY=your_ncbi_key         # NCBI E-utilities API key
GNOMAD_API_KEY=your_gnomad_key        # gnomAD API key  
COSMIC_USERNAME=your_cosmic_user      # COSMIC database username
COSMIC_PASSWORD=your_cosmic_pass      # COSMIC database password
LOVD_API_KEY=your_lovd_key           # LOVD API key
HGMD_API_KEY=your_hgmd_key           # HGMD API key

# =============================================================================
# Security Configuration (Production)
# =============================================================================
MCP_TLS_ENABLED=false                 # Enable HTTPS/TLS
MCP_TLS_CERT_PATH=/app/certs/server.crt
MCP_TLS_KEY_PATH=/app/certs/server.key
```

### 🔧 Method 2: Local Development

1. **Setup dependencies**
   ```bash
   # Install Go dependencies
   go mod download
   
   # Start local PostgreSQL and Redis
   brew install postgresql redis
   brew services start postgresql redis
   
   # Create database
   createdb acmg_amp_dev
   ```

2. **Configure and run**
   ```bash
   # Copy and edit environment variables
   cp .env.example .env
   # Edit .env with your local database settings
   
   # Set environment variables for local development
   export DATABASE_URL="postgres://mcpuser:password@localhost:5432/acmg_amp_dev"
   export REDIS_URL="redis://localhost:6379/0"
   
   # Run database migrations (if migration command exists)
   # go run cmd/migrate/main.go up
   
   # Start the MCP server
   go run cmd/mcp-server/main.go --stdio
   ```

3. **Connect to Claude Desktop**
   
   Configure Claude Desktop to use stdio transport:
   ```json
   {
     "mcpServers": {
       "acmg-amp-classifier": {
         "command": "/path/to/your/built/mcp-server",
         "args": ["--stdio"],
         "env": {
           "DATABASE_URL": "postgres://mcpuser:password@localhost:5432/acmg_amp_dev",
           "REDIS_URL": "redis://localhost:6379/0"
         }
       }
     }
   }
   ```

   Or use `go run` directly:
   ```json
   {
     "mcpServers": {
       "acmg-amp-classifier": {
         "command": "go",
         "args": ["run", "/path/to/acmg-amp-classifier-mcp/cmd/mcp-server/main.go", "--stdio"],
         "cwd": "/path/to/acmg-amp-classifier-mcp",
         "env": {
           "DATABASE_URL": "postgres://mcpuser:password@localhost:5432/acmg_amp_dev",
           "REDIS_URL": "redis://localhost:6379/0"
         }
       }
     }
   }
   ```

### Security & Compliance Notice

⚠️ **This is medical software handling genetic data. Security and compliance are critical:**

**Security Requirements:**
- **Never commit `.env` files** or secrets to version control (added to `.gitignore`)
- **Use strong, unique passwords** for all services (minimum 16 characters)
- **Enable TLS/HTTPS** in production environments (`MCP_TLS_ENABLED=true`)
- **Regularly rotate API keys** and database passwords
- **Monitor audit logs** for suspicious activity 
- **Use environment variables** for all sensitive configuration
- See [SECURITY.md](SECURITY.md) for complete security guidelines

**Production Deployment Checklist:**
- [ ] Set secure `POSTGRES_PASSWORD` and `REDIS_PASSWORD` in `.env`
- [ ] Configure external database API keys for enhanced functionality  
- [ ] Enable TLS/HTTPS for production (`MCP_TLS_ENABLED=true`)
- [ ] Set appropriate resource limits in `docker-compose.yml`
- [ ] Configure monitoring and log aggregation
- [ ] Set up regular database backups
- [ ] Review and apply security patches regularly

**License Compliance:**
- ✅ Ensure your use case complies with the Non-Commercial License
- ❌ Commercial use requires separate licensing agreement
- 🏥 Clinical use is prohibited without regulatory approval
- 📚 Keep this README and LICENSE files with any distribution

## 💬 Example Usage with Claude

Once configured, you can ask Claude to perform genetic variant analysis:

### **Basic Variant Classification (HGVS)**
```
"Can you classify the variant NM_000492.3:c.1521_1523delCTT and explain the ACMG/AMP rules that apply?"
```

### **Gene Symbol Classification (NEW)**
```
"Classify the BRCA1:c.5266dupC variant and explain the pathogenicity."
```

```
"What is the ACMG/AMP classification for TP53 p.R273H?"
```

### **Evidence Gathering**
```
"What evidence is available for the BRCA1 variant chr17:g.43094692G>A from ClinVar and gnomAD?"
```

### **Rule-Specific Analysis**
```
"Apply the PVS1 rule to the variant NM_000492.3:c.1521_1523delCTT and explain whether it meets the criteria."
```

### **Batch Analysis**
```
"Can you classify these variants and compare their pathogenicity:
1. NM_000492.3:c.1521_1523delCTT
2. BRCA1:c.5266dupC
3. TP53 p.R273H"
```

### **Report Generation**
```
"Generate a clinical interpretation report for CFTR:c.1521_1523delCTT including recommendations for genetic counseling."
```

## Configuration

The service uses Viper for configuration management with support for:
- YAML configuration files
- Environment variables (prefixed with `ACMG_AMP_`)
- Sensible defaults for development

Key configuration sections:
- **Server**: HTTP server settings (port, timeouts, CORS)
- **Database**: PostgreSQL connection settings with connection pooling
- **Redis**: Cache configuration for external API responses
- **External**: API keys and settings for ClinVar, gnomAD, COSMIC
- **Logging**: Structured logging with configurable levels

### Database Configuration

The service uses PostgreSQL 15+ with advanced features:
- **Connection Pooling**: pgx v5 driver with configurable pool settings (min/max connections, lifetime management)
- **UUID Generation**: Native `gen_random_uuid()` for distributed system compatibility
- **JSONB Support**: Advanced JSONB storage and indexing for evidence and rule data
- **Health Monitoring**: Built-in connection health checks and pool statistics
- **Audit Triggers**: Automatic timestamp updates with PL/pgSQL functions

### Database Schema

Production-ready schema with two core tables:

**variants table:**
- UUID primary keys with HGVS notation uniqueness constraints
- Genomic coordinate validation and indexing
- Support for both germline and somatic variants
- Automatic audit trail with created_at/updated_at timestamps

**interpretations table:**
- Foreign key relationships with cascade delete
- ACMG/AMP classification enumeration with validation
- Advanced JSONB storage for rules, evidence, and report data
- Processing time tracking and client audit fields
- GIN indexes for efficient JSONB queries

**Migration Features:**
- Automated migration on startup with version tracking
- Transaction-wrapped migrations for consistency
- Up/down migration support for rollbacks
- Comprehensive indexing strategy for performance

## 🔧 MCP Tool Reference

### **classify_variant**
Complete ACMG/AMP variant classification workflow with support for both HGVS and gene symbol input.

**Parameters:**
- `hgvs_notation` (optional*): HGVS variant notation (e.g., "NM_000492.3:c.1521_1523delCTT")
- `gene_symbol_notation` (optional*): Gene symbol with variant (e.g., "BRCA1:c.123A>G", "TP53 p.R273H")
- `preferred_isoform` (optional): Preferred transcript isoform when multiple exist
- `gene_symbol` (optional): HGNC gene symbol for additional context
- `variant_type` (optional): "SNV", "indel", "CNV", "SV"
- `clinical_context` (optional): Clinical context information

*At least one of `hgvs_notation` or `gene_symbol_notation` is required.

**Supported Gene Symbol Formats:**
- `BRCA1:c.123A>G` - Gene symbol with coding variant
- `TP53 p.R273H` - Gene symbol with protein change
- `CFTR` - Standalone gene symbol (for gene-level queries)

**Example Claude Requests:**
- *"Use classify_variant to analyze NM_000492.3:c.1521_1523delCTT"*
- *"Classify the BRCA1:c.5266dupC variant using ACMG/AMP guidelines"*
- *"What is the classification for TP53 p.R273H?"*

---

### **apply_rule**
Apply a specific ACMG/AMP rule to a variant.

**Parameters:**
- `rule_code` (required): ACMG/AMP rule (e.g., "PVS1", "PS1", "PM2", "PP3")
- `variant_data` (required): Variant information object
- `evidence_data` (optional): Additional evidence

**Example Claude Request:**
*"Apply the PVS1 rule to variant NM_000492.3:c.1521_1523delCTT"*

---

### **validate_hgvs**
Validate and normalize HGVS notation.

**Parameters:**
- `hgvs_notation` (required): HGVS string to validate
- `strict_mode` (optional): Enable strict validation

**Example Claude Request:**
*"Validate this HGVS notation: NM_000492.3:c.1521_1523delCTT"*

---

### **combine_evidence**
Combine multiple ACMG/AMP rules according to guidelines.

**Parameters:**
- `applied_rules` (required): Array of rule evaluation results
- `guidelines` (optional): Guidelines version ("ACMG2015")

**Example Claude Request:**
*"Combine these ACMG/AMP rule results into a final classification"*

## License

This software is released under a **Non-Commercial License**. 

### ✅ **Permitted Uses (Non-Commercial)**
- Academic research and education
- Personal experimentation and learning
- Non-profit organization internal research
- Open source contributions and improvements
- Clinical research (non-patient care)

### ❌ **Prohibited Uses (Commercial)**
- Clinical practice and patient care
- Integration into commercial products or services
- Paid consulting or analysis services
- Revenue-generating operations
- Commercial distribution or resale

### 📋 **Medical Software Disclaimer**

⚠️ **IMPORTANT: This software is for research and educational purposes only.**

- **NOT approved for clinical use or patient care**
- **NOT a medical device or diagnostic tool**
- **Requires additional validation for clinical settings**
- **Should not be used as sole basis for medical decisions**
- **Requires regulatory approval for clinical use**

Any clinical application requires appropriate medical oversight, validation studies, and regulatory compliance.

## Contact for Commercial Licensing

If you wish to use the MCP Service for commercial purposes, please contact:

**[Yi John Huang]**
**[yi.john.huang@me.com]**

---
*This README was last updated on: 2026-01-17*