# Requirements Document

## Introduction

Enable genetic variant classification using gene symbols (e.g., BRCA1, TP53) as an alternative to RefSeq HGVS notation, improving accessibility for clinical users who prefer human-readable gene names over technical RefSeq identifiers like NM_007294.3. This feature maintains full backward compatibility while extending the MCP server to support multiple input formats for the same underlying classification functionality.

## Requirements

### Requirement 1: Gene Symbol Input Processing
**User Story:** As a clinical geneticist, I want to input gene symbols instead of RefSeq IDs, so that I can classify variants using familiar gene names without needing to lookup technical identifiers.

#### Acceptance Criteria

1. WHEN a user provides a valid HUGO gene symbol (e.g., "BRCA1") THEN the system SHALL accept it as valid input for variant classification
2. WHEN a user provides gene symbol with variant notation (e.g., "BRCA1:c.123A>G") THEN the system SHALL parse both the gene symbol and variant components correctly  
3. WHEN a user provides gene symbol with protein notation (e.g., "TP53 p.R273H") THEN the system SHALL interpret the gene and protein change appropriately
4. IF a gene symbol contains invalid characters or format THEN the system SHALL reject it with a clear validation error message
5. WHERE gene symbols are provided in mixed case THE system SHALL normalize them to uppercase following HUGO standards

### Requirement 2: Transcript Resolution and Mapping
**User Story:** As a research scientist, I want gene symbols to be automatically resolved to appropriate RefSeq transcripts, so that I don't need to manually lookup transcript identifiers for each gene.

#### Acceptance Criteria

1. WHEN a valid gene symbol is provided THEN the system SHALL resolve it to the canonical RefSeq transcript identifier
2. IF multiple transcript isoforms exist for a gene THEN the system SHALL select the canonical transcript by default
3. WHEN transcript resolution fails due to external service unavailability THEN the system SHALL return an informative error message with retry guidance
4. WHILE transcript resolution is in progress THE system SHALL respect external API rate limits and handle timeouts gracefully  
5. WHERE successful transcript mappings occur THE system SHALL cache the results for 24 hours to improve performance

### Requirement 3: MCP Tool Interface Enhancement
**User Story:** As an AI agent developer, I want the MCP classify_variant tool to support gene symbols, so that my agent can accept more natural input from users while maintaining existing functionality.

#### Acceptance Criteria

1. WHEN either hgvs_notation OR gene_symbol_notation parameter is provided THEN the classify_variant tool SHALL process the request successfully
2. IF both hgvs_notation AND gene_symbol_notation are provided THEN the system SHALL prioritize hgvs_notation and log a warning
3. WHEN gene_symbol_notation is provided without hgvs_notation THEN the system SHALL generate equivalent HGVS notation internally for processing
4. IF neither hgvs_notation nor gene_symbol_notation is provided THEN the system SHALL return a validation error requiring one of them
5. WHERE gene symbol format is invalid THE tool SHALL return specific error messages indicating the expected format

### Requirement 4: Classification Accuracy and Consistency
**User Story:** As a laboratory technician, I want gene symbol queries to produce identical results to RefSeq queries, so that I can trust the classification regardless of input format.

#### Acceptance Criteria  

1. WHEN the same variant is classified using gene symbol notation and RefSeq HGVS notation THEN the system SHALL produce identical ACMG/AMP classification results
2. IF external evidence databases return different data for gene symbol vs RefSeq queries THEN the system SHALL reconcile and use the most complete evidence set
3. WHEN gene symbol resolution produces multiple possible transcripts THEN the system SHALL use the same canonical transcript selection logic consistently
4. WHERE evidence gathering fails for gene symbol input THE system SHALL fall back to RefSeq-based evidence gathering when available
5. WHILE maintaining accuracy THE system SHALL complete gene symbol-based classifications within 2 seconds for 95% of queries

### Requirement 5: Error Handling and User Guidance
**User Story:** As a clinical user, I want clear error messages and suggestions when gene symbol input fails, so that I can quickly correct issues and successfully submit my query.

#### Acceptance Criteria

1. WHEN an unrecognized gene symbol is provided THEN the system SHALL return error message with suggestions for similar valid symbols
2. IF a deprecated gene symbol is used THEN the system SHALL suggest the current official symbol and optionally process using the updated name
3. WHEN transcript resolution is ambiguous (multiple canonical options) THEN the system SHALL return error with available transcript options for user selection
4. WHERE external gene mapping services are temporarily unavailable THE system SHALL provide clear guidance for alternative input methods
5. IF rate limits are exceeded during transcript resolution THEN the system SHALL queue the request and provide estimated completion time

### Requirement 6: External Database Integration
**User Story:** As a system administrator, I want reliable integration with gene mapping databases, so that gene symbol resolution works consistently even when external services have issues.

#### Acceptance Criteria

1. WHEN querying HGNC database for gene validation THEN the system SHALL respect the 3 requests/second rate limit
2. IF primary gene mapping service fails THEN the system SHALL attempt resolution using backup services (RefSeq, Ensembl)
3. WHEN external API responses are received THEN the system SHALL validate the response format before processing
4. WHERE multiple external services provide conflicting gene mappings THE system SHALL prioritize HGNC as the authoritative source
5. WHILE external services are responding slowly THE system SHALL enforce 10-second timeout limits and provide fallback options

### Requirement 7: Performance and Caching
**User Story:** As a power user performing batch analysis, I want gene symbol resolution to be fast enough for real-time use, so that I can process multiple variants efficiently.

#### Acceptance Criteria

1. WHEN a gene symbol has been resolved previously THEN the system SHALL return cached transcript mapping in under 100 milliseconds
2. IF cache is empty for a gene symbol THEN the system SHALL complete initial resolution within 2 seconds for 95% of queries
3. WHEN cache entries expire after 24 hours THEN the system SHALL refresh them asynchronously during the next access
4. WHERE memory usage for transcript cache exceeds 100MB THE system SHALL implement LRU eviction of least recently used entries
5. WHILE processing batch requests THE system SHALL limit concurrent external API calls to prevent overwhelming external services

### Requirement 8: Backward Compatibility and Migration
**User Story:** As an existing MCP client developer, I want my current integrations to continue working unchanged, so that I can adopt gene symbol functionality without breaking existing workflows.

#### Acceptance Criteria

1. WHEN existing clients send requests with only hgvs_notation parameter THEN the system SHALL process them identically to current behavior
2. IF clients use the current classify_variant tool schema THEN the system SHALL continue supporting all existing parameters without modification
3. WHEN API responses are returned THEN the system SHALL maintain the existing JSON structure and field names
4. WHERE new gene_symbol_notation parameter is ignored by older clients THE system SHALL function normally using hgvs_notation as before
5. WHILE adding new functionality THE system SHALL not break any existing MCP tool contracts or response formats