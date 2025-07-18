---
inclusion: always
---

# Golang Medical Software Development Standards

## Project Context
This is a medical genetics software project implementing ACMG/AMP guidelines for genetic variant classification. The system must meet high standards for reliability, accuracy, and clinical safety.

## Code Quality Standards

### Medical Software Requirements
- **Clinical Accuracy**: All ACMG/AMP rule implementations must be validated against published guidelines
- **Traceability**: Every classification decision must be fully traceable with evidence citations
- **Error Handling**: Medical software requires comprehensive error handling with detailed logging
- **Validation**: Input validation is critical - genetic data must be thoroughly validated before processing
- **Audit Trail**: All operations must be logged for clinical audit requirements

### Go Project Structure
Follow standard Go project layout:
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
└── docs/               # Documentation
```

### Naming Conventions
- **Interfaces**: Use descriptive names ending with behavior (e.g., `VariantClassifier`, `EvidenceGatherer`)
- **Structs**: Use clear, domain-specific names (e.g., `StandardizedVariant`, `ACMGRule`)
- **Methods**: Use verb-noun patterns (e.g., `ClassifyVariant`, `ParseHGVS`)
- **Constants**: Use ALL_CAPS with descriptive prefixes (e.g., `CLASSIFICATION_PATHOGENIC`)

### Error Handling Patterns
```go
// Use custom error types for different categories
type ValidationError struct {
    Field   string
    Message string
    Value   interface{}
}

// Always wrap errors with context
func (p *Parser) ParseHGVS(input string) (*Variant, error) {
    if input == "" {
        return nil, fmt.Errorf("parsing HGVS: %w", ErrEmptyInput)
    }
    // ... parsing logic
    if err != nil {
        return nil, fmt.Errorf("parsing HGVS notation %q: %w", input, err)
    }
}
```

### Interface Design
- Keep interfaces small and focused (Interface Segregation Principle)
- Define interfaces in the package that uses them, not where they're implemented
- Use context.Context as the first parameter for operations that may be cancelled

### Testing Standards
- **Unit Tests**: Minimum 90% coverage for medical logic
- **Table-Driven Tests**: Use for ACMG/AMP rule validation
- **Integration Tests**: Test external API integrations with real data
- **Clinical Validation**: Test against known variant classifications

### Documentation Requirements
- **GoDoc**: All public functions must have comprehensive documentation
- **Medical Context**: Include clinical rationale in comments for ACMG/AMP rules
- **Examples**: Provide usage examples for complex medical logic
- **References**: Cite medical literature and guidelines in code comments

### Performance Considerations
- Use connection pooling for database and external API connections
- Implement caching for expensive operations (external database queries)
- Use context for request timeouts and cancellation
- Profile memory usage for large variant datasets

### Security Requirements
- Validate all inputs to prevent injection attacks
- Use parameterized queries for database operations
- Implement rate limiting to prevent abuse
- Log security events for audit purposes
- Never log patient-identifiable information

### Configuration Management
- Use environment variables for deployment-specific settings
- Provide sensible defaults for development
- Validate configuration on startup
- Support configuration hot-reloading where appropriate

### Logging Standards
```go
// Use structured logging with consistent fields
log.WithFields(logrus.Fields{
    "variant_id": variantID,
    "operation": "classify_variant",
    "duration_ms": duration.Milliseconds(),
    "classification": result.Classification,
}).Info("Variant classification completed")
```

### Database Patterns
- Use repository pattern for data access
- Implement proper transaction handling
- Use database migrations for schema changes
- Index frequently queried fields (chromosome, position, gene_symbol)

### External API Integration
- Implement circuit breaker pattern for resilience
- Use exponential backoff for retries
- Cache responses to reduce external API load
- Handle rate limiting gracefully
- Log all external API interactions

### Medical Data Handling
- Never store patient-identifiable information
- Validate genetic nomenclature (HGVS) strictly
- Implement data retention policies
- Ensure HIPAA compliance considerations
- Use secure communication (TLS) for all external connections

## Code Review Checklist
- [ ] Medical logic validated against ACMG/AMP guidelines
- [ ] Comprehensive error handling implemented
- [ ] Input validation covers all edge cases
- [ ] Logging includes necessary audit information
- [ ] Tests cover both positive and negative cases
- [ ] Documentation includes clinical context
- [ ] Security considerations addressed
- [ ] Performance implications considered