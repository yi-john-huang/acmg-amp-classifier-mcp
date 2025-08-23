# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is the ACMG-AMP MCP (Medical Classification Platform) Server - a Go-based backend service for AI-powered genetic variant interpretation using ACMG/AMP guidelines. The service is designed to integrate with AI agents (ChatGPT, Claude, Gemini) to provide standardized variant classification for clinical geneticists and researchers.

The system implements all 28 ACMG/AMP evidence criteria for variant classification and follows a multi-stage pipeline: input validation and parsing, evidence aggregation from multiple databases, ACMG/AMP rule application, and structured report generation.

## Development Commands

### Building and Running
- `go run cmd/server/main.go` - Start the development server
- `go build -o server cmd/server/main.go` - Build the server binary
- `docker-compose up -d` - Run with Docker (requires .env file)
- `docker-compose -f docker-compose.prod.yml up -d` - Production deployment

### Testing
- `go test ./...` - Run all tests
- `go test ./internal/service ./pkg/hgvs ./internal/domain` - Run tests for completed components
- `go test -v ./internal/repository/variant_test.go` - Run specific test with verbose output
- `go test -v ./pkg/hgvs/...` - Run HGVS validation and parsing tests
- `go test -v ./internal/service/...` - Run service layer tests
- Tests use testcontainers for database integration testing

### Database Operations
- `go run cmd/migrate/main.go up` - Apply database migrations (auto-applied on startup)
- Migrations are located in `migrations/` directory
- Uses PostgreSQL 15+ with JSONB support for evidence storage
- Database layer is fully implemented with repository pattern and test containers

### Development Setup
1. Copy `config.example.yaml` to `config.yaml`
2. Set up `.env` file (never commit this!)
3. Start PostgreSQL and Redis (via Docker Compose)
4. Run migrations with `go run cmd/migrate/main.go up`

## Architecture

### Core Components
- **API Gateway** (`internal/api/`): HTTP request handling with Gin framework
- **Domain Models** (`internal/domain/`): Business entities with medical validation and audit support
- **Repository Layer** (`internal/repository/`): PostgreSQL data access with pgx driver
- **Service Layer** (`internal/service/`): Business logic orchestration and input parsing
- **External Packages** (`pkg/`): Reusable components (ACMG rules, HGVS validation, external APIs)

### Key Interfaces (internal/domain/interfaces.go)
- `APIGateway`: HTTP request coordination and authentication
- `InputParser`: HGVS notation validation and variant normalization
- `InterpretationEngine`: ACMG/AMP guideline application
- `KnowledgeBaseAccess`: External database integration (ClinVar, gnomAD, COSMIC)
- `ReportGenerator`: Structured clinical report generation
- `VariantRepository`: Variant and interpretation persistence

### Database Schema
- **variants**: Genetic variant information with HGVS notation and genomic coordinates
- **interpretations**: Classification results with ACMG/AMP rule applications stored as JSONB
- Uses UUID primary keys and optimized indexing for genomic queries

### Enhanced Domain Types
- **Classification types** with medical validation (`IsValid()`, `RequiresClinicalAction()`)
- **Audit logging support** with structured fields for medical compliance
- **Clinical significance** descriptions for patient communication
- **Extended metadata** for traceability and regulatory requirements

### Configuration
- Uses Viper for configuration management
- Supports YAML files, environment variables (prefixed `ACMG_AMP_`), and defaults
- Environment variables automatically substitute in config.yaml using `${VAR_NAME}` syntax
- Key sections: server, database, external_api, cache (Redis), logging, security

## Development Guidelines

### Coding Standards and Requirements
- **ALWAYS** consult the coding rules in `./.kiro/steering/` before implementing any features
- **ALWAYS** read the design and requirements in `./.kiro/specs/acmg-amp-mcp-server/` before starting development
- These directories contain essential project-specific guidelines that must be followed for all code changes

### Medical Data Handling
- This handles sensitive genetic information - follow security best practices
- **Clinical Accuracy**: All ACMG/AMP rule implementations must be validated against published guidelines
- **Traceability**: Every classification decision must be fully traceable with evidence citations
- **Audit Trail**: All operations must be logged for clinical audit requirements
- Never commit secrets, API keys, or `.env` files
- Never store patient-identifiable information
- Use TLS/HTTPS in production environments
- Validate genetic nomenclature (HGVS) strictly

### AI Tool Security and Credential Protection

**CRITICAL**: This project uses sensitive medical data and credentials that must never be exposed to AI tools or version control.

#### File Protection
Claude Code and other AI tools are configured to ignore sensitive files through:
- `.claudecode-ignore`: Claude Code specific patterns
- `.kiro-ignore`: Kiro AI tool patterns  
- `.dockerignore`: Docker build protection
- `.gitignore`: Version control protection

#### Credential Handling Rules
1. **NEVER** discuss, display, or work with actual API keys, passwords, or secrets
2. **ALWAYS** use example/template files (.env.example, config.example.yaml)
3. **NEVER** generate or suggest real credentials - use placeholders only
4. **ALWAYS** refer developers to proper credential management procedures

#### Medical Data Protection
1. **NEVER** work with real patient data, genetic variants, or clinical information
2. **ALWAYS** use synthetic/example data for development assistance
3. **NEVER** generate or modify code that processes actual medical records
4. **ALWAYS** remind developers of HIPAA and clinical safety requirements

#### Safe Development Practices
```bash
# Verify AI tool protection is in place
ls -la .kiro-ignore .claudecode-ignore

# Check for accidental credential exposure
grep -r "password\|secret\|api.*key" --exclude-dir=.git .

# Use example files for reference
cp config.example.yaml config.yaml
cp .env.example .env
```

#### Emergency Credential Exposure Response
If credentials are accidentally exposed in AI conversations:
1. Immediately rotate all potentially exposed credentials
2. Review and update ignore file patterns
3. Notify security team per SECURITY.md guidelines
4. Audit AI tool access logs and conversation history

#### Medical Code Validation Requirements
- All AI-generated ACMG/AMP rule implementations MUST undergo clinical review
- Never commit AI-generated medical decision logic without expert validation
- Maintain clear audit trails of AI assistance in medical software development
- Follow medical software development lifecycle (IEC 62304) principles

### HGVS Validation and Parsing
- **Input Parser Service** (`internal/service/input_parser.go`): Orchestrates HGVS parsing and validation
- **HGVS Parser** (`pkg/hgvs/parser.go`): Handles genomic, coding, and protein HGVS notation
- **Gene Validator** (`pkg/hgvs/gene_validator.go`): Medical-grade gene symbol and transcript validation following HUGO standards
- **Basic Validator** (`pkg/hgvs/validator.go`): Core HGVS format validation and component parsing
- Supports RefSeq, Ensembl, Entrez, and HGNC identifier validation
- Comprehensive normalization for consistent variant representation

### External API Integration
- ClinVar: Public NCBI database (optional API key for higher rate limits)
- gnomAD: Population frequency data (optional API key)
- COSMIC: Somatic mutation database (requires API key)
- All external calls should be cached in Redis with appropriate TTLs

### Testing Strategy
- Unit tests for domain logic and utilities (90%+ coverage for medical components)
- Integration tests using testcontainers for database operations
- Repository tests verify PostgreSQL interactions
- HGVS validator tests ensure proper variant notation parsing
- Table-driven tests for medical validation scenarios
- Clinical validation against known variant classifications

### Code Patterns
- Repository pattern for data access
- Interface-driven design for modularity (keep interfaces small and focused)
- JSONB for flexible evidence storage
- Connection pooling for PostgreSQL performance
- Structured logging with configurable levels
- Custom error types with context wrapping (use `fmt.Errorf` with `%w` verb)
- Table-driven tests for ACMG/AMP rule validation
- Context.Context as first parameter for cancellable operations

### Medical Validation Guidelines
- **Always use GeneValidator** for medical-grade gene/transcript validation in production code
- **Use Basic Validator** only for simple HGVS format validation or component parsing
- **Input Parser Service** should be the primary entry point for variant validation workflows
- **Validation Errors** must include medical context and be traceable for audit requirements
- **Gene Symbols** must follow HUGO standards (uppercase, proper formatting)
- **Transcript IDs** must be validated against RefSeq/Ensembl patterns before processing

### Service Deployment
- Supports both development and production Docker Compose configurations
- Health checks on all services (/health endpoint for main service)
- Automatic migration application on startup
- Uses PostgreSQL 15 and Redis 7 Alpine images

## Implementation Status

### Completed Components
- **Project Structure**: Go module with standard directory layout (✅ Task 1)
- **Data Models**: Core structs and validation for genetic variants (✅ Task 2)
- **Database Layer**: PostgreSQL integration with migrations and repository pattern (✅ Task 3)
- **Input Parser**: HGVS notation parsing and variant normalization (✅ Task 4)

### In Progress
- **External APIs**: Integration with ClinVar, gnomAD, and COSMIC databases (Task 5)

### Pending Implementation
- ACMG/AMP rule engine with all 28 evidence criteria (Task 6)
- Variant interpretation engine with classification workflow (Task 7)
- Report generation system with structured outputs (Task 8)
- API gateway with authentication and rate limiting (Task 9)
- Comprehensive logging and monitoring (Task 10)
- OpenAPI documentation and error handling (Tasks 11-12)

See `./.kiro/specs/acmg-amp-mcp-server/tasks.md` for detailed implementation plan.