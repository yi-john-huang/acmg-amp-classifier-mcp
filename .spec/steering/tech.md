# Technology Stack - ACMG-AMP MCP Server

## Language and Runtime

| Component | Version | Notes |
|-----------|---------|-------|
| Go | 1.24.5 | Primary development language |

## Core Dependencies

### MCP and Protocol
| Package | Version | Purpose |
|---------|---------|---------|
| `github.com/modelcontextprotocol/go-sdk` | v0.3.1 | MCP protocol implementation |
| `github.com/gorilla/websocket` | v1.5.3 | WebSocket support for transport |

### Web Framework
| Package | Version | Purpose |
|---------|---------|---------|
| `github.com/gin-gonic/gin` | v1.10.1 | HTTP server and routing |

### Database
| Package | Version | Purpose |
|---------|---------|---------|
| `github.com/jackc/pgx/v5` | v5.7.5 | PostgreSQL driver with connection pooling |
| `github.com/golang-migrate/migrate/v4` | v4.18.3 | Database migrations |
| `github.com/google/uuid` | v1.6.0 | UUID generation |

### Caching
| Package | Version | Purpose |
|---------|---------|---------|
| `github.com/redis/go-redis/v9` | v9.12.0 | Redis client (primary) |
| `github.com/go-redis/redis/v8` | v8.11.5 | Redis client (legacy support) |
| `github.com/hashicorp/golang-lru` | v1.0.2 | In-memory LRU cache |

### Resilience
| Package | Version | Purpose |
|---------|---------|---------|
| `github.com/sony/gobreaker` | v1.0.0 | Circuit breaker pattern |
| `golang.org/x/time` | v0.8.0 | Rate limiting |

### Configuration and Logging
| Package | Version | Purpose |
|---------|---------|---------|
| `github.com/spf13/viper` | v1.20.1 | Configuration management |
| `github.com/sirupsen/logrus` | v1.9.3 | Structured logging |

### Testing
| Package | Version | Purpose |
|---------|---------|---------|
| `github.com/stretchr/testify` | v1.10.0 | Test assertions and mocking |
| `github.com/DATA-DOG/go-sqlmock` | v1.5.2 | SQL mocking |
| `github.com/testcontainers/testcontainers-go` | v0.38.0 | Integration testing with containers |

## Architecture

### Layered Architecture
```
┌─────────────────────────────────────────────────┐
│              MCP Transport Layer                │
│         (stdio / HTTP-SSE / WebSocket)          │
├─────────────────────────────────────────────────┤
│              MCP Protocol Layer                 │
│         (JSON-RPC 2.0, Tool Registry)           │
├─────────────────────────────────────────────────┤
│              Service Layer                      │
│    (Classification, Evidence, Reporting)        │
├─────────────────────────────────────────────────┤
│              Domain Layer                       │
│     (Entities, ACMG/AMP Rules, HGVS)            │
├─────────────────────────────────────────────────┤
│            Repository Layer                     │
│      (Variant, Interpretation Repos)            │
├─────────────────────────────────────────────────┤
│           Infrastructure Layer                  │
│   (PostgreSQL, Redis, External APIs)            │
└─────────────────────────────────────────────────┘
```

### Key Architectural Patterns

1. **Repository Pattern**: Clean separation of data access logic
2. **Interface-Based Dependency Injection**: Testable, loosely-coupled components
3. **Circuit Breaker**: Resilient external API integration (gobreaker)
4. **Structured Logging**: Correlation IDs for request tracing (logrus)
5. **Connection Pooling**: Efficient database connections (pgx)

## Infrastructure Requirements

### Database
| Component | Version | Configuration |
|-----------|---------|---------------|
| PostgreSQL | 15+ | JSONB support, UUID generation |

### Cache
| Component | Version | Configuration |
|-----------|---------|---------------|
| Redis | 7+ | Persistence enabled, LRU eviction |

### Container Runtime
| Component | Purpose |
|-----------|---------|
| Docker | Container builds and local development |
| Kubernetes | Production deployment orchestration |

## Development Commands

### Build and Run
```bash
# Run the MCP server
go run cmd/mcp-server/main.go --stdio

# Build binary
go build -o bin/mcp-server cmd/mcp-server/main.go

# Run with HTTP transport
go run cmd/mcp-server/main.go --http --port 8080
```

### Testing
```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run specific package tests
go test ./internal/service/...

# Run integration tests
go test -tags=integration ./tests/...
```

### Database
```bash
# Run migrations (via golang-migrate)
migrate -path migrations -database "postgres://..." up

# Start local infrastructure
docker-compose up -d postgres redis

# Connect to database
docker-compose exec postgres psql -U mcpuser -d acmg_amp_mcp
```

### Docker
```bash
# Build container
docker build -t acmg-amp-mcp-server .

# Start all services
docker-compose up -d

# View logs
docker-compose logs -f mcp-server

# Health check
curl http://localhost:8080/health
```

## Configuration

### Environment Variables
Configuration uses Viper with `ACMG_AMP_` prefix for environment variables.

| Variable | Description |
|----------|-------------|
| `DATABASE_URL` | PostgreSQL connection string |
| `REDIS_URL` | Redis connection string |
| `MCP_TRANSPORT` | Transport type (stdio/http) |
| `MCP_HTTP_PORT` | HTTP server port |
| `MCP_LOG_LEVEL` | Log level (debug/info/warn/error) |
| `CLINVAR_API_KEY` | NCBI E-utilities API key |
| `GNOMAD_API_KEY` | gnomAD API key |

### Config File (config.yaml)
```yaml
server:
  port: 8080
  timeout: 30s
database:
  host: localhost
  port: 5432
  name: acmg_amp_mcp
redis:
  host: localhost
  port: 6379
logging:
  level: info
  format: json
```
