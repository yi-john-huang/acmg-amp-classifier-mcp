# Implementation Tasks

## Task Overview

Implementation of gene symbol-based variant classification through a structured 4-phase approach: Foundation Infrastructure → Service Integration → Quality Assurance → Production Deployment. Each task maps directly to EARS requirements and design components, ensuring comprehensive traceability and testable deliverables.

## Phase 1: Foundation Infrastructure (Estimated: 2 weeks)

### Task 1: External Gene API Client Development
**Status:** ✅ Completed  
**Estimated:** 5 days  
**Dependencies:** None  
**Requirements Traceability:** 6.1-6.5 (External Database Integration)

#### Subtasks:
- [x] 1.1: Create HGNC API client (`pkg/external/hgnc_client.go`)
  - ✅ Implement REST API client with rate limiting (3 req/sec)
  - ✅ Add JSON response parsing for gene symbol validation
  - ✅ Include canonical transcript lookup functionality
  - ✅ Implement comprehensive error handling with retries
  - ✅ Add context support for cancellation and timeouts
- [x] 1.2: Create RefSeq API client (`pkg/external/refseq_client.go`)
  - ✅ Implement E-utilities API integration with API key support
  - ✅ Add transcript metadata retrieval functionality
  - ✅ Include gene-to-transcript mapping capabilities
  - ✅ Implement rate limiting (10 req/sec with API key)
  - ✅ Add XML response parsing for E-utilities format
- [x] 1.3: Create Ensembl REST API client (`pkg/external/ensembl_client.go`)
  - ✅ Implement Ensembl REST API integration
  - ✅ Add alternative transcript lookup functionality
  - ✅ Include cross-reference mapping between databases
  - ✅ Implement rate limiting (15 req/sec)
  - ✅ Add JSON response parsing and validation
- [x] 1.4: Implement unified external API interface
  - ✅ Define common `ExternalGeneAPI` interface in `pkg/external/interfaces.go`
  - ✅ Implement failover logic between HGNC, RefSeq, Ensembl
  - ✅ Add service health monitoring and circuit breaker integration
  - ✅ Create comprehensive integration tests with real API calls
  - ✅ Add performance benchmarking for each client

#### Acceptance Criteria:
- [x] All external API clients respect rate limits and handle HTTP errors gracefully
- [x] HGNC client validates gene symbols and returns canonical transcripts with >95% success rate
- [x] RefSeq client retrieves transcript metadata with proper XML parsing
- [x] Ensembl client provides alternative transcript information
- [x] Circuit breaker integration prevents cascade failures during service outages
- [x] Integration tests pass with real external APIs (can be disabled for CI)

### Task 2: Transcript Resolution Service Implementation
**Status:** ✅ Completed  
**Estimated:** 4 days  
**Dependencies:** Task 1  
**Requirements Traceability:** 2.1-2.5 (Transcript Resolution and Mapping)

#### Subtasks:
- [x] 2.1: Design transcript resolver interface (`internal/service/transcript_resolver.go`)
  - ✅ Define `TranscriptResolver` interface with context support
  - ✅ Create `TranscriptInfo`, `GeneValidationResult` data structures (in external package)
  - ✅ Design caching strategy integration points
  - ✅ Plan concurrent resolution logic for batch processing
- [x] 2.2: Implement cached transcript resolver
  - ✅ Create `CachedTranscriptResolver` struct with multi-level caching
  - ✅ Integrate with existing Redis cache (`internal/mcp/caching/tool_result_cache.go`)
  - ✅ Add in-memory LRU cache for hot data (1000 entries, 15 min TTL)
  - ✅ Implement cache invalidation and refresh logic
  - ✅ Add cache hit/miss metrics collection
- [x] 2.3: Implement canonical transcript selection logic
  - ✅ Define canonical transcript selection rules (HGNC → RefSeq → Ensembl priority)
  - ✅ Handle multiple isoform scenarios with clear selection criteria
  - ✅ Add user preference override capability via `PreferredIsoform` parameter
  - ✅ Create transcript prioritization algorithm based on usage frequency
  - ✅ Add logging for transcript selection decisions
- [x] 2.4: Add batch processing support
  - ✅ Implement `BatchResolve` method with controlled concurrency (max 5 concurrent)
  - ✅ Add semaphore-based rate limiting to prevent API overwhelming
  - ✅ Create partial success handling for batch requests
  - ✅ Add batch result aggregation with error tracking
  - ✅ Implement timeout handling for long-running batch operations

#### Acceptance Criteria:
- [x] Single gene symbol resolution completes within 2 seconds (95th percentile)
- [x] Cache hit ratio exceeds 90% for common gene symbols after warmup
- [x] Batch processing handles 100+ gene symbols efficiently with partial success
- [x] Circuit breaker prevents external service overload during high traffic
- [x] Comprehensive error handling provides actionable error messages
- [x] Performance metrics are collected for monitoring and alerting

### Task 3: Enhanced Input Parser Development
**Status:** ❌ Not Started  
**Estimated:** 3 days  
**Dependencies:** Task 2  
**Requirements Traceability:** 1.1-1.5 (Gene Symbol Input Processing)

#### Subtasks:
- [ ] 3.1: Extend input parser for gene symbols (`internal/domain/input_parser.go`)
  - Add gene symbol pattern recognition to existing `StandardInputParser`
  - Implement format detection logic for HGVS vs gene symbol notation
  - Create parser dispatch system maintaining backward compatibility
  - Add comprehensive input validation with HUGO standards
- [ ] 3.2: Implement gene symbol parsing methods
  - Create `parseGeneWithVariant()` for "BRCA1:c.123A>G" format
  - Implement `parseStandaloneGene()` for "BRCA1" format
  - Add `parseGeneWithProtein()` for "TP53 p.R273H" format
  - Include case normalization and validation for each format
  - Add regex pattern matching with comprehensive error messages
- [ ] 3.3: Implement HGVS generation from gene symbols
  - Create `generateHGVSFromGeneSymbol()` method using transcript resolution
  - Handle coordinate conversion from gene symbol notation to HGVS
  - Maintain variant notation consistency across different input formats
  - Add validation of generated HGVS against existing parser
  - Include error handling for transcript resolution failures
- [ ] 3.4: Update existing helper functions
  - Replace placeholder `extractGeneSymbol()` implementation with robust logic
  - Add gene symbol extraction from various input formats
  - Handle edge cases, aliases, and deprecated symbols
  - Add normalization logic following HUGO guidelines
  - Include comprehensive unit tests for all parsing scenarios

#### Acceptance Criteria:
- [ ] Parser correctly identifies and processes all supported gene symbol formats
- [ ] HUGO gene nomenclature standards are enforced with clear error messages
- [ ] Generated HGVS notation is identical to manual HGVS input for same variants
- [ ] Backward compatibility maintained - all existing HGVS inputs continue to work
- [ ] Performance impact is minimal (<10ms additional processing time)
- [ ] Comprehensive test coverage (>95%) for all parsing scenarios

## Phase 2: Service Integration (Estimated: 1 week)

### Task 4: MCP Tool Parameter Enhancement
**Status:** ❌ Not Started  
**Estimated:** 2 days  
**Dependencies:** Task 3  
**Requirements Traceability:** 3.1-3.5 (MCP Tool Interface Enhancement)

#### Subtasks:
- [ ] 4.1: Update `ClassifyVariantParams` structure (`internal/mcp/tools/classification_tools.go`)
  - Add `GeneSymbolNotation` field with proper JSON tags and validation
  - Make `HGVSNotation` optional in tool parameters
  - Add `PreferredIsoform` field for transcript selection override
  - Update parameter validation to support either/or HGVS/gene symbol requirement
  - Maintain backward compatibility with existing parameter structure
- [ ] 4.2: Enhance parameter validation logic
  - Implement either/or validation ensuring at least one notation format provided
  - Add gene symbol format validation using existing `GeneValidator`
  - Create helpful validation error messages with format examples
  - Update JSON schema validation for MCP tool specification
  - Add parameter precedence logic (HGVS takes priority when both provided)
- [ ] 4.3: Update tool information schema
  - Modify `GetToolInfo()` method to include new parameters in schema
  - Update input schema with gene symbol parameter documentation
  - Add examples for all supported gene symbol formats
  - Update tool description to mention gene symbol capability
  - Include parameter validation rules in schema documentation

#### Acceptance Criteria:
- [ ] MCP tool accepts either HGVS notation or gene symbol notation as input
- [ ] Parameter validation provides clear error messages for invalid formats
- [ ] Tool schema accurately reflects all supported parameters and formats
- [ ] Backward compatibility maintained - existing clients continue to work unchanged
- [ ] Tool information includes comprehensive examples and documentation
- [ ] JSON schema validation correctly handles new optional parameters

### Task 5: Classification Service Integration
**Status:** ❌ Not Started  
**Estimated:** 3 days  
**Dependencies:** Task 4  
**Requirements Traceability:** 4.1-4.5 (Classification Accuracy and Consistency)

#### Subtasks:
- [ ] 5.1: Enhance `ClassifyVariant` method (`internal/service/classifier.go`)
  - Add gene symbol input handling to existing classification workflow
  - Integrate transcript resolution service with proper error handling
  - Implement HGVS generation workflow for gene symbol inputs
  - Maintain identical classification logic ensuring 100% accuracy consistency
  - Add comprehensive logging for gene symbol resolution and classification steps
- [ ] 5.2: Add transcript resolution to service dependencies
  - Inject `TranscriptResolver` dependency into `ClassifierService`
  - Update `NewClassifierService` constructor with transcript resolver
  - Add transcript resolution error handling with fallback mechanisms
  - Implement resolution result caching at service level
  - Add resolution metrics and performance monitoring
- [ ] 5.3: Update service configuration and initialization
  - Add transcript resolver to service initialization in `internal/mcp/server.go`
  - Update dependency injection configuration for external API clients
  - Add service initialization validation ensuring all dependencies available
  - Update configuration management to include external API settings
  - Add health check integration for transcript resolution service

#### Acceptance Criteria:
- [ ] Gene symbol classification produces identical results to HGVS-based classification
- [ ] Service gracefully handles transcript resolution failures with informative errors
- [ ] Performance targets met: <2s resolution, <100ms for cached lookups
- [ ] All existing functionality continues to work without degradation
- [ ] Comprehensive logging enables troubleshooting and monitoring
- [ ] Service initialization validates all external API connectivity

## Phase 3: Quality Assurance (Estimated: 1 week)

### Task 6: Comprehensive Testing Implementation
**Status:** ❌ Not Started  
**Estimated:** 4 days  
**Dependencies:** Task 5  
**Requirements Traceability:** All requirements validation

#### Subtasks:
- [ ] 6.1: Unit tests for all components
  - Create test suite for gene symbol validation with HUGO standards
  - Add transcript resolution tests with mocked external APIs
  - Implement HGVS generation accuracy tests with known variants
  - Add comprehensive error handling scenario tests
  - Include performance regression tests for all new functionality
- [ ] 6.2: Integration tests for end-to-end workflows
  - Create classification accuracy validation comparing gene symbol vs HGVS results
  - Add external API integration tests with circuit breaker scenarios
  - Implement cache behavior verification across all cache tiers
  - Add MCP tool integration tests with various client scenarios
  - Include batch processing integration tests with large gene symbol sets
- [ ] 6.3: Create comprehensive test data sets
  - Curate HUGO official gene symbol test cases (100+ symbols)
  - Create known variant test cases with expected classifications
  - Add edge case scenarios (deprecated symbols, aliases, invalid formats)
  - Include performance benchmark data for load testing
  - Create mock data for offline testing without external API dependencies
- [ ] 6.4: Performance and load testing
  - Implement load testing for transcript resolution under high concurrency
  - Add performance benchmarks comparing gene symbol vs HGVS processing
  - Create stress testing for cache behavior under memory pressure
  - Add external API rate limiting validation
  - Include end-to-end performance testing for classification workflows

#### Acceptance Criteria:
- [ ] Unit test coverage exceeds 95% for all new components
- [ ] Integration tests validate 100% classification accuracy between input formats
- [ ] Performance tests confirm all SLO targets are met consistently
- [ ] Load testing demonstrates system handles 100+ concurrent requests
- [ ] All edge cases and error scenarios are covered with appropriate tests
- [ ] Test suite runs reliably in CI environment with proper mocking

### Task 7: Error Handling and User Experience Enhancement
**Status:** ❌ Not Started  
**Estimated:** 3 days  
**Dependencies:** Task 6  
**Requirements Traceability:** 5.1-5.5 (Error Handling and User Guidance)

#### Subtasks:
- [ ] 7.1: Implement comprehensive error types
  - Create gene symbol specific error types extending existing `ErrorManager`
  - Add `GeneSymbolValidationError`, `TranscriptResolutionError`, `AmbiguousTranscriptError`
  - Implement error suggestion system for common misspellings and alternatives
  - Add recovery guidance with actionable next steps for each error type
  - Include error correlation for debugging and user support
- [ ] 7.2: Add user guidance and suggestion system
  - Implement gene symbol spell checking with fuzzy matching algorithms
  - Add common alias and deprecated symbol suggestions
  - Create format correction hints for invalid input patterns
  - Add contextual help messages based on error type and user input
  - Include links to HUGO nomenclature guidelines for validation errors
- [ ] 7.3: Implement graceful degradation mechanisms
  - Add fallback to cached data during external service outages
  - Implement degraded mode operation with HGVS-only functionality
  - Add offline mode capabilities with pre-cached common gene symbols
  - Create service health monitoring with automated degradation triggers
  - Include user notification system for degraded service capabilities

#### Acceptance Criteria:
- [ ] Error messages provide specific, actionable guidance for resolution
- [ ] Suggestion system provides relevant alternatives for invalid gene symbols
- [ ] Graceful degradation maintains service availability during external outages
- [ ] User experience remains consistent with clear status communication
- [ ] Error handling integrates seamlessly with existing MCP error patterns
- [ ] Recovery mechanisms are tested and validated under failure scenarios

## Phase 4: Production Deployment (Estimated: 2 days)

### Task 8: Production Readiness and Deployment
**Status:** ❌ Not Started  
**Estimated:** 2 days  
**Dependencies:** Task 7  
**Requirements Traceability:** 8.1-8.5 (Backward Compatibility), 7.1-7.5 (Performance)

#### Subtasks:
- [ ] 8.1: Feature flag implementation and configuration
  - Add `ENABLE_GENE_SYMBOL_QUERIES` feature flag with runtime toggle capability
  - Implement graceful feature toggle without service restart requirement
  - Add configuration validation ensuring external API credentials present
  - Create rollback procedures for immediate feature disable if needed
  - Include feature flag monitoring and alerting for configuration changes
- [ ] 8.2: Monitoring, observability, and alerting
  - Add gene symbol query metrics to existing Prometheus integration
  - Create service health dashboards for transcript resolution performance
  - Add alerting for transcript resolution failures, cache misses, external API errors
  - Implement audit logging for all gene symbol operations (compliance)
  - Include performance monitoring dashboards for SLO tracking
- [ ] 8.3: Database migration and schema deployment
  - Create database migration scripts for gene symbol cache tables
  - Add proper indexing strategy for optimal query performance
  - Implement data retention policies for audit logs (30-day retention)
  - Add database health checks for cache table availability
  - Include rollback procedures for schema changes
- [ ] 8.4: Load testing and capacity planning for production
  - Conduct production-scale load testing with realistic traffic patterns
  - Validate external API rate limits under production load
  - Test Redis cache performance and memory usage under sustained load
  - Verify circuit breaker behavior during simulated external service outages
  - Include capacity planning recommendations for production deployment

#### Acceptance Criteria:
- [ ] Feature flag enables/disables gene symbol functionality without service impact
- [ ] Monitoring provides comprehensive visibility into system performance and health
- [ ] Database migrations execute successfully with zero downtime
- [ ] Load testing demonstrates production readiness with target performance metrics
- [ ] Rollback procedures are documented and tested for rapid issue resolution
- [ ] Production deployment checklist completed with all stakeholder approvals

## Success Criteria

### Functional Success Metrics
- [ ] **Classification Accuracy**: 100% identical results between gene symbol and HGVS inputs for identical variants
- [ ] **Input Format Support**: All specified gene symbol formats (standalone, with variant, with protein) parse correctly
- [ ] **Backward Compatibility**: All existing HGVS-based functionality continues to work unchanged
- [ ] **Error Handling**: Comprehensive error messages with actionable guidance for all failure scenarios

### Performance Success Metrics  
- [ ] **Gene Symbol Resolution**: <2000ms (95th percentile) for transcript resolution
- [ ] **Cache Performance**: <100ms (99th percentile) for cached transcript lookups
- [ ] **Cache Hit Ratio**: >90% for common gene symbols after 24-hour warmup period
- [ ] **Concurrent Processing**: >100 requests/second sustained throughput
- [ ] **External API Reliability**: >99.5% uptime through circuit breaker protection

### Quality Success Metrics
- [ ] **Test Coverage**: >95% unit test coverage for all new components
- [ ] **Integration Validation**: End-to-end workflow tests pass for all supported input formats
- [ ] **Performance Regression**: <10% impact on existing HGVS-based classification performance
- [ ] **Security Validation**: All external API integrations use secure protocols and proper authentication
- [ ] **Documentation**: Complete API documentation and user guides for gene symbol functionality

## Risk Mitigation

### High Priority Risks
- **External API Dependency Risk**: Multiple API providers (HGNC, RefSeq, Ensembl) with circuit breaker failover
- **Performance Impact Risk**: Multi-level caching strategy with comprehensive performance monitoring
- **Data Accuracy Risk**: Extensive validation against known variant datasets with 100% accuracy requirement
- **Service Reliability Risk**: Health monitoring, graceful degradation, and automated rollback procedures

### Medium Priority Risks  
- **Gene Symbol Ambiguity Risk**: Clear error messages with suggested alternatives and manual override capability
- **Cache Consistency Risk**: TTL-based invalidation with manual refresh endpoints for cache management
- **Rate Limiting Risk**: Intelligent client-side rate limiting with queue management and backoff strategies
- **Integration Complexity Risk**: Comprehensive integration testing with existing MCP architecture components

### Risk Monitoring and Alerting
- **Performance Degradation**: Automated alerts for response time SLO violations
- **External Service Issues**: Circuit breaker state monitoring with automatic failover
- **Cache Performance**: Cache hit ratio monitoring with alerts for unusual patterns
- **Error Rate Monitoring**: Gene symbol resolution error rate tracking with threshold alerts

## Resource Requirements

### Development Time Estimate: 4 weeks total
- **Foundation Infrastructure**: 2 weeks (External APIs, Transcript Resolution, Input Parser)
- **Service Integration**: 1 week (MCP Tools, Classification Service)  
- **Quality Assurance**: 1 week (Testing, Error Handling, User Experience)
- **Production Deployment**: 2 days (Feature Flags, Monitoring, Database Migration)

### Technical Skill Requirements
- **Go Development**: Expert level for medical software with MCP protocol integration
- **External API Integration**: Experience with REST APIs, rate limiting, circuit breakers
- **Caching Strategy**: Redis expertise with multi-level caching implementation
- **Testing Strategy**: Comprehensive testing including integration, performance, and load testing
- **DevOps/SRE**: Production deployment, monitoring, and reliability engineering

### External Dependencies and Coordination
- **HGNC API Access**: Validate API access and rate limits for production usage
- **RefSeq API Key**: Obtain production API key with appropriate rate limits
- **Ensembl API**: Confirm production usage terms and reliability requirements
- **Infrastructure**: Redis cluster capacity for caching requirements
- **Monitoring Tools**: Prometheus/Grafana setup for metrics and alerting