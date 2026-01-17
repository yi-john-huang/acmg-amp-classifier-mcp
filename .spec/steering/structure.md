# Project Structure and Conventions - ACMG-AMP MCP Server

## Directory Layout

```
/
├── cmd/                          # Main applications
│   └── mcp-server/              # MCP server entry point (main.go)
│
├── internal/                    # Private application code
│   ├── config/                 # Configuration management (Viper)
│   ├── database/               # Database connection and migrations
│   ├── domain/                 # Business entities and interfaces
│   ├── middleware/             # HTTP middleware (auth, logging, etc.)
│   ├── repository/             # Data access layer (PostgreSQL)
│   ├── service/                # Business logic layer
│   └── mcp/                    # MCP protocol implementation
│       ├── protocol/          # JSON-RPC 2.0 protocol core
│       ├── transport/         # Transport layer (stdio/HTTP-SSE)
│       ├── tools/             # ACMG/AMP tool implementations
│       ├── resources/         # MCP resource providers
│       ├── prompts/           # MCP prompt templates
│       ├── connection/        # Connection management
│       ├── caching/           # Response caching
│       ├── compression/       # Payload compression
│       ├── errors/            # Error handling
│       ├── health/            # Health checks
│       ├── logging/           # Structured logging
│       ├── monitoring/        # Metrics and monitoring
│       ├── alerting/          # Alert management
│       ├── optimization/      # Performance optimization
│       ├── benchmarking/      # Performance benchmarks
│       ├── loadtesting/       # Load testing utilities
│       └── testing/           # Test utilities
│
├── pkg/                        # Public library code
│   ├── hgvs/                  # HGVS notation parsing
│   ├── acmg/                  # ACMG/AMP rule definitions
│   └── external/              # External API clients
│                              # (ClinVar, gnomAD, COSMIC, etc.)
│
├── api/                        # API definitions and schemas
├── migrations/                 # Database migration files
├── deployments/               # Deployment configurations
│   └── kubernetes/           # Kubernetes manifests
├── docs/                      # Documentation
├── examples/                  # Usage examples
│   └── ai-agents/            # AI agent integration examples
│       ├── workflows/        # Common workflow examples
│       ├── custom-clients/   # Custom client implementations
│       └── demonstrations/   # Demo scripts
├── scripts/                   # Utility scripts
│   └── deployment/           # Deployment scripts
├── tests/                     # Integration and e2e tests
│   └── deployment/           # Deployment tests
├── validation/                # Validation utilities
├── bin/                       # Compiled binaries (gitignored)
└── diagram/                   # Architecture diagrams
```

## Naming Conventions

### Go Standard Conventions

#### Files
- Use `snake_case` for file names: `variant_repository.go`, `clinvar_client.go`
- Test files: `*_test.go` suffix: `variant_repository_test.go`
- One primary type per file when practical

#### Packages
- Single lowercase word: `domain`, `service`, `repository`
- Avoid underscores and mixed caps
- Package name should describe its purpose

#### Types and Functions
- **Exported (public)**: PascalCase - `ClassificationResult`, `ParseHGVS`
- **Unexported (private)**: camelCase - `parseVariant`, `validateInput`

#### Constants
- **Exported**: PascalCase - `MaxRetries`, `DefaultTimeout`
- **Unexported**: camelCase - `maxRetries`, `defaultTimeout`
- Group related constants with `const ()` block

#### Interfaces
- Use `-er` suffix for single-method interfaces: `Parser`, `Validator`
- Descriptive names for multi-method interfaces: `VariantRepository`, `ClassificationService`

## Code Patterns

### Repository Pattern
All database access goes through repository interfaces.

```go
// internal/domain/repository.go
type VariantRepository interface {
    Create(ctx context.Context, variant *Variant) error
    GetByID(ctx context.Context, id uuid.UUID) (*Variant, error)
    GetByHGVS(ctx context.Context, hgvs string) (*Variant, error)
    Update(ctx context.Context, variant *Variant) error
    Delete(ctx context.Context, id uuid.UUID) error
}

// internal/repository/variant_repository.go
type variantRepository struct {
    db *pgxpool.Pool
}

func NewVariantRepository(db *pgxpool.Pool) VariantRepository {
    return &variantRepository{db: db}
}
```

### Interface-Based Dependency Injection
Services depend on interfaces, not concrete implementations.

```go
// internal/service/classification_service.go
type ClassificationService struct {
    variantRepo  domain.VariantRepository
    evidenceRepo domain.EvidenceRepository
    ruleEngine   domain.RuleEngine
    logger       *logrus.Logger
}

func NewClassificationService(
    variantRepo domain.VariantRepository,
    evidenceRepo domain.EvidenceRepository,
    ruleEngine domain.RuleEngine,
    logger *logrus.Logger,
) *ClassificationService {
    return &ClassificationService{
        variantRepo:  variantRepo,
        evidenceRepo: evidenceRepo,
        ruleEngine:   ruleEngine,
        logger:       logger,
    }
}
```

### Circuit Breaker for External APIs
All external API calls use circuit breaker pattern.

```go
// pkg/external/clinvar_client.go
type ClinVarClient struct {
    httpClient *http.Client
    breaker    *gobreaker.CircuitBreaker
    baseURL    string
}

func (c *ClinVarClient) Query(ctx context.Context, variant string) (*ClinVarResult, error) {
    result, err := c.breaker.Execute(func() (interface{}, error) {
        return c.doQuery(ctx, variant)
    })
    if err != nil {
        return nil, fmt.Errorf("clinvar query failed: %w", err)
    }
    return result.(*ClinVarResult), nil
}
```

### Structured Logging with Correlation IDs
All log entries include context for tracing.

```go
func (s *ClassificationService) Classify(ctx context.Context, req *ClassifyRequest) (*Result, error) {
    correlationID := ctx.Value("correlation_id").(string)

    s.logger.WithFields(logrus.Fields{
        "correlation_id": correlationID,
        "hgvs":          req.HGVS,
        "operation":     "classify_variant",
    }).Info("Starting variant classification")

    // ... classification logic
}
```

### Error Handling
Use domain-specific error types with wrapping.

```go
// internal/domain/errors.go
var (
    ErrVariantNotFound    = errors.New("variant not found")
    ErrInvalidHGVS        = errors.New("invalid HGVS notation")
    ErrExternalAPIFailure = errors.New("external API failure")
)

// Usage
func (r *variantRepository) GetByID(ctx context.Context, id uuid.UUID) (*Variant, error) {
    var v Variant
    err := r.db.QueryRow(ctx, query, id).Scan(&v.ID, &v.HGVS)
    if errors.Is(err, pgx.ErrNoRows) {
        return nil, ErrVariantNotFound
    }
    if err != nil {
        return nil, fmt.Errorf("query variant: %w", err)
    }
    return &v, nil
}
```

## Import Order

Follow this order, separated by blank lines:

1. Standard library
2. External packages (third-party)
3. Internal packages

```go
import (
    "context"
    "errors"
    "fmt"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/sirupsen/logrus"

    "github.com/acmg-amp-mcp-server/internal/domain"
    "github.com/acmg-amp-mcp-server/pkg/hgvs"
)
```

## Testing Conventions

### Unit Tests
- Located alongside the code: `service.go` → `service_test.go`
- Use table-driven tests for multiple cases
- Mock dependencies using interfaces

### Integration Tests
- Located in `tests/` directory
- Use testcontainers for database tests
- Tag with `//go:build integration`

### Test Naming
```go
func TestClassificationService_Classify_Success(t *testing.T) { }
func TestClassificationService_Classify_InvalidHGVS(t *testing.T) { }
func TestVariantRepository_GetByID_NotFound(t *testing.T) { }
```

## Configuration Patterns

### Viper Configuration Loading
```go
func LoadConfig() (*Config, error) {
    viper.SetConfigName("config")
    viper.SetConfigType("yaml")
    viper.AddConfigPath(".")
    viper.SetEnvPrefix("ACMG_AMP")
    viper.AutomaticEnv()

    if err := viper.ReadInConfig(); err != nil {
        if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
            return nil, err
        }
    }

    var cfg Config
    if err := viper.Unmarshal(&cfg); err != nil {
        return nil, err
    }
    return &cfg, nil
}
```

## Database Conventions

### Migration Files
- Format: `NNNN_description.up.sql` and `NNNN_description.down.sql`
- Example: `0001_create_variants_table.up.sql`

### Table Names
- Plural, snake_case: `variants`, `interpretations`, `audit_logs`

### Column Names
- snake_case: `created_at`, `hgvs_notation`, `classification_result`

### JSON/JSONB Fields
- Used for flexible data: `rules_applied`, `evidence_data`, `report_json`
- Include GIN indexes for query performance
