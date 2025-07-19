# Database Architecture and Schema

## Overview

The ACMG/AMP MCP Server uses PostgreSQL as its primary database for storing genetic variants and their clinical interpretations. The database is designed to support high-performance queries for genomic data while maintaining clinical audit requirements.

## Database Requirements

- **PostgreSQL 15+** with UUID and JSONB support
- **Connection Pooling** using pgx driver with configurable pool settings
- **Migrations** managed through golang-migrate
- **Indexing Strategy** optimized for genomic coordinate and gene-based queries

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
- `idx_variants_chromosome_position` - Genomic coordinate queries
- `idx_variants_gene_symbol` - Gene-based searches
- `idx_variants_variant_type` - Filtering by variant type
- `idx_variants_created_at` - Temporal queries

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
- `idx_interpretations_variant_id` - Variant-based queries
- `idx_interpretations_classification` - Classification filtering
- `idx_interpretations_confidence_level` - Confidence-based queries
- `idx_interpretations_applied_rules_gin` - JSONB rule queries
- `idx_interpretations_evidence_summary_gin` - JSONB evidence queries

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
    Host        string
    Port        int
    Database    string
    Username    string
    Password    string
    MaxConns    int32          // Maximum connections
    MinConns    int32          // Minimum connections
    MaxConnLife time.Duration  // Connection lifetime
    MaxConnIdle time.Duration  // Idle connection timeout
    SSLMode     string         // SSL configuration
}
```

### Health Monitoring

The database connection includes health check capabilities:
- Connection pool statistics monitoring
- Ping-based health verification
- Graceful connection handling and cleanup

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

- Test containers for isolated database testing
- Migration testing with real schema changes
- Repository pattern testing with actual database
- Performance testing with realistic data volumes

### Test Data Management

- Synthetic variant data for testing
- Known classification examples from ClinVar
- Edge case scenarios for validation
- Performance benchmarks with large datasets