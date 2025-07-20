# MCP Service: AI-Powered Variant Interpretation Assistant

**(Project Status: Active Development - Database Layer Complete)**

🔬 **Research & Educational Use Only** | ⚖️ **Non-Commercial License** | 🏥 **Not for Clinical Use**

## Overview

The Medical Classification Platform (MCP) Service is a backend system designed to assist physicians, clinical geneticists, and researchers in interpreting somatic and germline genetic variants. It leverages the standardized **ACMG/AMP guidelines** for variant classification to provide consistent and evidence-based interpretations.

A key feature of this service is its designed integration capability with popular large language model (LLM) based AI agents such as **ChatGPT, Claude, and Gemini**. This allows clinicians to interact with the service using a natural language interface, simply providing the variant details and receiving the interpretation report back through the AI agent.

## Purpose

The goal of the MCP Service is to:

1.  Standardize the application of ACMG/AMP guidelines for variant interpretation.
2.  Reduce the manual effort and time required for evidence gathering and classification.
3.  Provide a seamless user experience for clinicians through integration with familiar AI tools.
4.  Facilitate accurate and reproducible variant classification in clinical and research settings.

## Implementation Status

### ✅ Completed Components
- **Database Layer**: Complete PostgreSQL implementation with pgx v5 driver, advanced connection pooling, and health monitoring
- **Repository Pattern**: Full CRUD operations for variants and interpretations with comprehensive JSONB handling
- **Domain Models**: Complete type definitions with medical validation for variants, classifications, and ACMG/AMP rules
- **Database Schema**: Production-ready schema with optimized indexing, constraints, and audit triggers
- **Integration Testing**: Comprehensive test suite using testcontainers for isolated database testing
- **Migration System**: Automated database migrations with up/down support and version tracking

### ✅ Recently Completed
- **Input Parser**: HGVS notation validation and variant normalization with medical-grade gene validation

### 🚧 In Development
- **External API Integration**: ClinVar, gnomAD, and COSMIC client implementations
- **ACMG/AMP Rule Engine**: 28 evidence criteria implementation
- **HTTP API Layer**: REST endpoints with Gin framework

### 📋 Planned Features
- **Report Generation**: Structured clinical reports with recommendations
- **Caching Layer**: Redis integration for external API response caching
- **Authentication**: API key validation and rate limiting
- **Monitoring**: Comprehensive logging and metrics collection

## Features

* **ACMG/AMP Guideline Engine:** Implements the core logic for classifying variants based on evidence criteria.
* **Somatic & Germline Support:** Designed to handle both types of mutations (Note: Specific guideline sets like AMP/ASCO/CAP for somatic variants might require distinct implementation paths).
* **AI Agent Ready API:** A defined API layer allows straightforward integration with platforms like ChatGPT, Claude, Gemini, etc.
* **Evidence Aggregation:** Connects to essential public databases (e.g., ClinVar, gnomAD, COSMIC) and potentially internal institutional databases.
* **Structured Reporting:** Outputs clear classification results (Pathogenic, Likely Pathogenic, VUS, Likely Benign, Benign) along with the specific ACMG/AMP evidence codes met.
* **Clinical Database**: PostgreSQL-based storage with full audit trails and JSONB support for flexible evidence storage.

## Architecture

The service follows a modular microservice-oriented architecture:

* **API Gateway:** Entry point for requests from AI agents.
* **MCP Service Backend:** Orchestrates the interpretation workflow.
* **Input Parser:** Validates and standardizes variant nomenclature.
* **Variant Interpretation Engine:** Applies guideline logic using aggregated evidence.
* **Knowledge Base Access:** Interfaces with various data sources.
* **Reporting Module:** Formats the final interpretation output.

```mermaid
graph TD
    subgraph "User Interaction Layer"
        User[Physician] -- Interacts via --> AI_Agent(AI Agent e.g., ChatGPT, Claude, Gemini)
    end

    subgraph "MCP Service"
        API_Gateway(API Gateway) -- Routes requests --> MCP_Backend(MCP Service Backend)
        MCP_Backend -- Uses --> Input_Parser(Input Parser)
        MCP_Backend -- Uses --> Variant_Engine(Variant Interpretation Engine)
        MCP_Backend -- Uses --> Reporting(Reporting Module)
        Variant_Engine -- Implements --> ACMG_AMP_Logic(ACMG/AMP Guideline Logic)
        Variant_Engine -- Queries --> KB_Access(Knowledge Base Access Layer)
    end

    subgraph "Data Layer"
        KB_Access -- Accesses --> Ext_DBs(External Databases e.g., ClinVar, gnomAD, COSMIC)
        KB_Access -- Accesses --> Int_DB(Internal Database e.g., Local Annotations, History)
    end

    AI_Agent -- Sends mutation data / Receives interpretation --> API_Gateway
    Input_Parser -- Parses & Validates --> MCP_Backend
    Reporting -- Formats output --> MCP_Backend
    MCP_Backend -- Sends result back --> API_Gateway
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
├── cmd/                    # Main applications
│   └── server/            # HTTP server entry point
├── internal/              # Private application code
│   ├── api/              # HTTP handlers and routing
│   ├── config/           # Configuration management
│   ├── domain/           # Business logic and entities
│   ├── repository/       # Data access layer
│   └── service/          # Application services
├── pkg/                  # Public library code
│   ├── acmg/            # ACMG/AMP rule engine
│   ├── hgvs/            # HGVS parsing utilities
│   └── external/        # External API clients
├── api/                 # OpenAPI/Swagger specs
├── migrations/          # Database migrations
├── docker/             # Docker configurations
├── docs/               # Documentation
└── config.example.yaml # Example configuration
```

## Core Interfaces

The service is built around well-defined interfaces:

- **APIGateway**: HTTP request handling and coordination
- **InputParser**: HGVS validation and variant standardization  
- **InterpretationEngine**: ACMG/AMP rule application and classification
- **KnowledgeBaseAccess**: External database integration
- **ReportGenerator**: Structured report generation

## Getting Started

### Prerequisites
- Go 1.21+
- PostgreSQL 15+
- Redis 7+ (for caching)

### Quick Start with Docker

1. **Clone the repository**
   ```bash
   git clone <repository-url>
   cd acmg-amp-mcp-server
   ```

2. **Set up environment variables (IMPORTANT)**
   ```bash
   cp .env.example .env
   # Edit .env with your actual values - NEVER commit this file!
   ```

3. **Copy configuration**
   ```bash
   cp config.example.yaml config.yaml
   # Configuration will automatically use environment variables
   ```

4. **Run with Docker Compose**
   ```bash
   # Development (uses .env file)
   docker-compose up -d
   
   # Production (uses Docker secrets - recommended)
   ./scripts/setup-secrets.sh
   docker-compose -f docker-compose.prod.yml up -d
   ```

5. **Run database migrations**
   ```bash
   # Migrations are automatically applied on startup
   # Or run manually: go run cmd/migrate/main.go up
   ```

6. **Check health**
   ```bash
   curl http://localhost:8080/health
   ```

### Security & Compliance Notice

⚠️ **This is medical software handling genetic data. Security and compliance are critical:**

**Security Requirements:**
- Never commit `.env` files or secrets to version control
- Use strong, unique passwords for all services
- Enable TLS/HTTPS in production environments
- Regularly rotate API keys and database passwords
- Monitor audit logs for suspicious activity
- See [SECURITY.md](SECURITY.md) for complete security guidelines

**License Compliance:**
- ✅ Ensure your use case complies with the Non-Commercial License
- ❌ Commercial use requires separate licensing agreement
- 🏥 Clinical use is prohibited without regulatory approval
- 📚 Keep this README and LICENSE files with any distribution

### Local Development

```bash
# Install dependencies
go mod download

# Set up local PostgreSQL database
createdb acmg_amp_dev

# Copy and configure environment
cp config.example.yaml config.yaml
# Edit database connection settings

# Run database migrations
go run cmd/migrate/main.go up

# Run the server
go run cmd/server/main.go

# Run tests (requires test database)
go test ./...
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

## API Endpoints

### Health and Status
- `GET /health` - Service health check with database connectivity
- `GET /metrics` - Prometheus metrics endpoint
- `GET /version` - Service version information

### Variant Management
- `POST /api/v1/variants` - Create or retrieve variant by HGVS
- `GET /api/v1/variants/:id` - Get variant details by UUID
- `GET /api/v1/variants/hgvs/:notation` - Get variant by HGVS notation
- `GET /api/v1/variants/gene/:symbol` - List variants by gene symbol

### Interpretation Services
- `POST /api/v1/interpret` - Perform variant interpretation with ACMG/AMP rules
- `GET /api/v1/interpretations/:id` - Get interpretation results by UUID
- `GET /api/v1/interpretations/variant/:variant_id` - Get interpretations for a variant
- `GET /api/v1/interpretations/classification/:type` - Filter by classification type

### Request/Response Format

#### Variant Interpretation Request
```json
{
  "hgvs": "NC_000017.11:g.43094692G>A",
  "gene_symbol": "BRCA1",
  "transcript": "NM_007294.4",
  "client_id": "clinical_lab_001",
  "request_id": "req_12345",
  "metadata": {
    "patient_age": "45",
    "indication": "breast_cancer_risk"
  }
}
```

#### Interpretation Response
```json
{
  "request_id": "req_12345",
  "variant": {
    "id": "uuid-here",
    "hgvs_genomic": "NC_000017.11:g.43094692G>A",
    "chromosome": "17",
    "position": 43094692,
    "gene_symbol": "BRCA1",
    "variant_type": "GERMLINE"
  },
  "classification": "PATHOGENIC",
  "confidence": "HIGH",
  "report": {
    "applied_rules": [
      {
        "code": "PVS1",
        "category": "PATHOGENIC",
        "strength": "VERY_STRONG",
        "met": true,
        "evidence": "Null variant in critical domain",
        "rationale": "Frameshift variant in BRCA1 exon 11"
      }
    ],
    "summary": "This variant is classified as Pathogenic based on strong evidence...",
    "recommendations": ["Genetic counseling recommended", "Consider cascade testing"]
  },
  "processing_time": "1.2s",
  "processed_at": "2025-01-19T10:30:00Z"
}
```

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
*This README was generated on: 2025-04-12*