# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is the ACMG-AMP MCP (Medical Classification Platform) Server - a Go-based backend service for AI-powered genetic variant interpretation using ACMG/AMP guidelines. The service is designed to integrate with AI agents (ChatGPT, Claude, Gemini) to provide standardized variant classification for clinical geneticists and researchers.

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
- **ALWAYS** consult the coding rules in `./kiro/steering/` before implementing any features
- **ALWAYS** read the design and requirements in `./kiro/specs/acmg-amp-mcp-server/` before starting development
- These directories contain essential project-specific guidelines that must be followed for all code changes

### Medical Data Handling
- This handles sensitive genetic information - follow security best practices
- Never commit secrets, API keys, or `.env` files
- All database operations should include audit trails
- Use TLS/HTTPS in production environments

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
- Interface-driven design for modularity
- JSONB for flexible evidence storage
- Connection pooling for PostgreSQL performance
- Structured logging with configurable levels

### Service Deployment
- Supports both development and production Docker Compose configurations
- Health checks on all services (/health endpoint for main service)
- Automatic migration application on startup
- Uses PostgreSQL 15 and Redis 7 Alpine images