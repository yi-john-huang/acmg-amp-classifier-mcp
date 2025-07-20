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
- `go test -v ./internal/repository/variant_test.go` - Run specific test with verbose output
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
- **Domain Models** (`internal/domain/`): Business entities and interfaces
- **Repository Layer** (`internal/repository/`): PostgreSQL data access with pgx driver
- **Service Layer** (`internal/service/`): Business logic orchestration
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

### External API Integration
- ClinVar: Public NCBI database (optional API key for higher rate limits)
- gnomAD: Population frequency data (optional API key)
- COSMIC: Somatic mutation database (requires API key)
- All external calls should be cached in Redis with appropriate TTLs

### Testing Strategy
- Unit tests for domain logic and utilities
- Integration tests using testcontainers for database operations
- Repository tests verify PostgreSQL interactions
- HGVS validator tests ensure proper variant notation parsing

### Code Patterns
- Repository pattern for data access
- Interface-driven design for modularity (keep interfaces small and focused)
- JSONB for flexible evidence storage
- Connection pooling for PostgreSQL performance
- Structured logging with configurable levels
- Custom error types with context wrapping (use `fmt.Errorf` with `%w` verb)
- Table-driven tests for ACMG/AMP rule validation
- Context.Context as first parameter for cancellable operations

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

### In Progress
- **Input Parser**: HGVS notation parsing and variant normalization (Task 4)
- **External APIs**: Integration with ClinVar, gnomAD, and COSMIC databases (Task 5)

### Pending Implementation
- ACMG/AMP rule engine with all 28 evidence criteria
- Variant interpretation engine with classification workflow
- Report generation system with structured outputs
- API gateway with authentication and rate limiting
- Comprehensive logging and monitoring
- OpenAPI documentation and error handling

See `./.kiro/specs/acmg-amp-mcp-server/tasks.md` for detailed implementation plan.