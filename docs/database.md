# Database Architecture and Schema

## Overview

The ACMG/AMP MCP Server uses PostgreSQL as its primary database for storing genetic variants and their clinical interpretations. The database is designed to support high-performance queries for genomic data while maintaining clinical audit requirements.

## Database Requirements

- **PostgreSQL 15+** with UUID generation and advanced JSONB support
- **pgx v5 Driver** with connection pooling, health monitoring, and statistics
- **Migration System** with automated version tracking and rollback support
- **Performance Optimization** with comprehensive indexing strategy for genomic queries
- **Audit Compliance** with automatic timestamp triggers and processing time tracking

## Schema Design

### Core Tables

#### variants
Stores standardized genetic variant information following HGVS nomenclature.

```sql
CREATE TABLE variants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    hgvs_notation VARCHAR(255) NOT NULL UNIQUE,
    chromosome VARCHAR(10) NOT NULL,
    position BIGINT NOT NULL CHECK (position > 0),
    reference VARCHAR(1000) NOT NULL,
    alternative VARCHAR(1000) NOT NULL,
    gene_symbol VARCHAR(50),
    transcript_id VARCHAR(50),
    variant_type VARCHAR(20) NOT NULL CHECK (variant_type IN ('GERMLINE', 'SOMATIC')),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
```

**Key Features:**
- UUID primary keys for distributed system compatibility
- HGVS notation uniqueness constraint for deduplication
- Genomic coordinate validation (position > 0)
- Support for both germline and somatic variants
- Automatic timestamp management

**Indexes:**
- `idx_variants_chromosome_position` - Composite index for genomic coordinate queries
- `idx_variants_gene_symbol` - B-tree index for gene-based searches
- `idx_variants_variant_type` - Enumeration filtering (GERMLINE/SOMATIC)
- `idx_variants_created_at` - Temporal queries and audit trails

**Constraints:**
- `variants_hgvs_unique` - Ensures HGVS notation uniqueness for deduplication
- `variants_position_check` - Validates genomic positions are positive
- `variants_ref_alt_check` - Ensures reference and alternative alleles are not empty

#### interpretations
Stores ACMG/AMP classification results with full audit trail.

```sql
CREATE TABLE interpretations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    variant_id UUID NOT NULL REFERENCES variants(id) ON DELETE CASCADE,
    classification VARCHAR(20) NOT NULL CHECK (classification IN ('PATHOGENIC', 'LIKELY_PATHOGENIC', 'VUS', 'LIKELY_BENIGN', 'BENIGN')),
    confidence_level VARCHAR(20) NOT NULL CHECK (confidence_level IN ('HIGH', 'MEDIUM', 'LOW')),
    applied_rules JSONB NOT NULL DEFAULT '[]'::jsonb,
    evidence_summary JSONB NOT NULL DEFAULT '{}'::jsonb,
    report_data JSONB NOT NULL DEFAULT '{}'::jsonb,
    processing_time_ms INTEGER NOT NULL CHECK (processing_time_ms >= 0),
    client_id VARCHAR(100),
    request_id VARCHAR(100),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
```

**Key Features:**
- Foreign key relationship to variants with cascade delete
- ACMG/AMP classification enumeration
- JSONB storage for flexible rule and evidence data
- Processing time tracking for performance monitoring
- Client and request tracking for audit purposes

**Indexes:**
- `idx_interpretations_variant_id` - Foreign key relationship queries
- `idx_interpretations_classification` - ACMG/AMP classification filtering
- `idx_interpretations_confidence_level` - Confidence-based result filtering
- `idx_interpretations_client_id` - Client audit and tracking queries
- `idx_interpretations_processing_time` - Performance monitoring queries
- `idx_interpretations_applied_rules_gin` - GIN index for JSONB rule queries
- `idx_interpretations_evidence_summary_gin` - GIN index for JSONB evidence queries

**Constraints:**
- `interpretations_processing_time_reasonable` - Validates processing time under 5 minutes
- Foreign key cascade delete ensures data integrity when variants are removed

## Repository Pattern Implementation

### VariantRepository

Handles all variant data persistence operations:

```go
type VariantRepository struct {
    db  *pgxpool.Pool
    log *logrus.Logger
}
```

**Methods:**
- `Create(ctx, variant)` - Insert new variant
- `GetByID(ctx, id)` - Retrieve by UUID
- `GetByHGVS(ctx, hgvs)` - Retrieve by HGVS notation
- `GetByGene(ctx, gene, limit, offset)` - Gene-based pagination
- `Update(ctx, variant)` - Update existing variant
- `Delete(ctx, id)` - Remove variant

### InterpretationRepository

Manages interpretation records with JSONB handling:

```go
type InterpretationRepository struct {
    db  *pgxpool.Pool
    log *logrus.Logger
}
```

**Methods:**
- `Create(ctx, interpretation)` - Store new interpretation
- `GetByID(ctx, id)` - Retrieve by UUID
- `GetByVariantID(ctx, variantID, limit, offset)` - Variant interpretations
- `GetByClassification(ctx, classification, limit, offset)` - Classification filtering
- `Update(ctx, interpretation)` - Update interpretation
- `Delete(ctx, id)` - Remove interpretation

## Connection Management

### Connection Pool Configuration

```go
type Config struct {
    Host        string        // Database host
    Port        int           // Database port
    Database    string        // Database name
    Username    string        // Database username
    Password    string        // Database password
    MaxConns    int32         // Maximum connections in pool
    MinConns    int32         // Minimum connections maintained
    MaxConnLife time.Duration // Maximum connection lifetime
    MaxConnIdle time.Duration // Idle connection timeout
    SSLMode     string        // SSL configuration (disable/require/verify-full)
}
```

### Advanced Connection Management

The database layer provides comprehensive connection management:

```go
type DB struct {
    Pool *pgxpool.Pool  // Connection pool
    log  *logrus.Logger // Structured logging
}
```

**Features:**
- **Health Monitoring**: Real-time connection pool statistics and ping-based health verification
- **Graceful Shutdown**: Clean connection pool closure with resource cleanup
- **Connection Statistics**: Pool utilization metrics (total, idle, acquired connections)
- **Error Handling**: Comprehensive error wrapping with context for debugging
- **Structured Logging**: Detailed connection events with correlation fields

## Migration Strategy

### Migration Files

Located in `/migrations/` directory:
- `000001_create_variants_table.up.sql` - Initial variant schema
- `000002_create_interpretations_table.up.sql` - Interpretation schema
- Corresponding `.down.sql` files for rollbacks

### Migration Management

- Automatic migration on application startup
- Version tracking in `schema_migrations` table
- Support for both up and down migrations
- Transaction-wrapped migrations for consistency

## Performance Considerations

### Indexing Strategy

1. **Genomic Coordinates**: Composite index on (chromosome, position)
2. **Gene Symbols**: B-tree index for exact matches
3. **JSONB Data**: GIN indexes for rule and evidence queries
4. **Temporal Data**: Indexes on created_at for time-based queries

### Query Optimization

- Connection pooling to reduce connection overhead
- Prepared statements for frequently executed queries
- JSONB queries optimized with proper indexing
- Pagination support for large result sets

### Monitoring

- Query execution time logging
- Connection pool utilization metrics
- Database health check endpoints
- Structured logging for database operations

## Security Considerations

### Data Protection

- No patient-identifiable information stored
- Parameterized queries to prevent SQL injection
- Connection encryption with TLS
- Database user with minimal required privileges

### Audit Requirements

- Complete audit trail for all interpretations
- Request and client ID tracking
- Processing time recording for performance analysis
- Immutable interpretation history

## Clinical Compliance

### ACMG/AMP Guidelines

- Structured storage of all 28 ACMG/AMP evidence criteria
- Rule strength and category tracking
- Evidence rationale preservation
- Classification traceability

### Data Retention

- Configurable retention policies
- Soft delete options for regulatory compliance
- Export capabilities for data migration
- Backup and recovery procedures

## Testing Strategy

### Integration Testing

The database layer includes comprehensive integration testing:

```go
func TestDatabaseConnection(t *testing.T) {
    // PostgreSQL test container setup
    pgContainer, err := postgres.Run(ctx,
        "postgres:15-alpine",
        postgres.WithDatabase("testdb"),
        postgres.WithUsername("testuser"),
        postgres.WithPassword("testpass"),
        testcontainers.WithWaitStrategy(
            wait.ForLog("database system is ready to accept connections").
                WithOccurrence(2).
                WithStartupTimeout(30*time.Second)),
    )
    // ... test implementation
}
```

**Testing Features:**
- **Isolated Testing**: PostgreSQL test containers for complete isolation
- **Real Database Testing**: Full integration tests with actual PostgreSQL instances
- **Migration Validation**: Automated testing of schema changes and rollbacks
- **Connection Pool Testing**: Verification of pool behavior and statistics
- **Performance Benchmarking**: Load testing with realistic data volumes

### Test Data Management

- **Synthetic Variants**: Generated test data following HGVS standards
- **Clinical Examples**: Known variant classifications from ClinVar for validation
- **Edge Cases**: Boundary condition testing for genomic coordinates and JSONB data
- **Performance Datasets**: Large-scale test data for benchmarking queries
- **Audit Trail Testing**: Verification of timestamp triggers and audit functionality