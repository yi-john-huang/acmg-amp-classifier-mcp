package external

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/acmg-amp-mcp-server/internal/domain"
)

// LOVDClient handles interactions with LOVD (Leiden Open Variation Database)
type LOVDClient struct {
	baseURL    string
	apiKey     string // Optional for some LOVD instances
	httpClient *http.Client
	rateLimit  time.Duration
}

// LOVDConfig contains configuration for LOVD client
type LOVDConfig struct {
	BaseURL   string        // Base URL for LOVD instance
	APIKey    string        // Optional API key
	Timeout   time.Duration
	RateLimit int
}

// NewLOVDClient creates a new LOVD API client
func NewLOVDClient(config LOVDConfig) *LOVDClient {
	if config.BaseURL == "" {
		config.BaseURL = "https://databases.lovd.nl" // Default to main LOVD instance
	}
	if config.RateLimit == 0 {
		config.RateLimit = 10 // 10 requests per second
	}
	
	return &LOVDClient{
		baseURL: config.BaseURL,
		apiKey:  config.APIKey,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		rateLimit: time.Second / time.Duration(config.RateLimit),
	}
}

// LOVDVariantsResponse represents the JSON response from LOVD variants API
type LOVDVariantsResponse struct {
	Data []LOVDVariant `json:"data"`
	Meta struct {
		Total   int `json:"total"`
		Page    int `json:"page"`
		PerPage int `json:"per_page"`
	} `json:"meta"`
}

// LOVDVariant represents a single variant from LOVD
type LOVDVariant struct {
	VariantID           string                 `json:"variantid"`
	GeneSymbol          string                 `json:"gene_symbol"`
	Transcript          string                 `json:"transcript"`
	DNAChange           string                 `json:"dna_change"`
	RNAChange           string                 `json:"rna_change,omitempty"`
	ProteinChange       string                 `json:"protein_change,omitempty"`
	Classification      string                 `json:"classification"`
	ClinicalDescription string                 `json:"clinical_description"`
	Phenotype           string                 `json:"phenotype"`
	Pathogenicity       string                 `json:"pathogenicity"`
	FunctionalAnalysis  []LOVDFunctionalResult `json:"functional_analysis"`
	References          []string               `json:"references"`
	DatabaseURL         string                 `json:"database_url"`
	SubmissionDate      string                 `json:"submission_date"`
	LastUpdated         string                 `json:"last_updated"`
	Curator             string                 `json:"curator,omitempty"`
	PublicAccess        string                 `json:"public_access"`
}

// LOVDFunctionalResult represents functional analysis data from LOVD
type LOVDFunctionalResult struct {
	StudyType   string `json:"study_type"`
	Method      string `json:"method"`
	Result      string `json:"result"`
	Conclusion  string `json:"conclusion"`
	Reference   string `json:"reference"`
	Reliability string `json:"reliability"`
	Comments    string `json:"comments,omitempty"`
}

// LOVDDatabasesResponse represents available gene-specific databases
type LOVDDatabasesResponse struct {
	Databases []LOVDDatabase `json:"databases"`
}

// LOVDDatabase represents a gene-specific LOVD database
type LOVDDatabase struct {
	GeneSymbol  string `json:"gene_symbol"`
	DatabaseURL string `json:"database_url"`
	Curator     string `json:"curator"`
	LastUpdate  string `json:"last_update"`
	VariantCount int   `json:"variant_count"`
	IsActive    bool   `json:"is_active"`
}

// QueryVariant queries LOVD for variant information
func (l *LOVDClient) QueryVariant(ctx context.Context, variant *domain.StandardizedVariant) (*domain.LOVDData, error) {
	// Rate limiting
	select {
	case <-time.After(l.rateLimit):
		// Proceed after rate limit delay
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// First, find the appropriate gene-specific database
	databases, err := l.getGeneDatabases(ctx, variant.GeneSymbol)
	if err != nil {
		return nil, fmt.Errorf("failed to find LOVD databases for gene %s: %w", variant.GeneSymbol, err)
	}

	if len(databases) == 0 {
		// No gene-specific database found, try global search
		return l.searchGlobalDatabase(ctx, variant)
	}

	// Search in the most relevant gene-specific database
	primaryDB := l.selectPrimaryDatabase(databases)
	return l.searchGeneSpecificDatabase(ctx, variant, primaryDB)
}

// getGeneDatabases retrieves available gene-specific databases
func (l *LOVDClient) getGeneDatabases(ctx context.Context, geneSymbol string) ([]LOVDDatabase, error) {
	if geneSymbol == "" {
		return nil, fmt.Errorf("gene symbol is required for LOVD database search")
	}

	// Rate limiting
	select {
	case <-time.After(l.rateLimit):
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Construct API URL for gene databases
	apiURL := fmt.Sprintf("%s/api/v1/databases", l.baseURL)
	params := url.Values{
		"gene": {geneSymbol},
		"format": {"json"},
	}

	if l.apiKey != "" {
		params.Set("api_key", l.apiKey)
	}

	fullURL := fmt.Sprintf("%s?%s", apiURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create database request: %w", err)
	}

	// Set appropriate headers
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "ACMG-AMP-MCP-Server/1.0")

	resp, err := l.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute database request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("LOVD databases API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read database response: %w", err)
	}

	var dbResponse LOVDDatabasesResponse
	if err := json.Unmarshal(body, &dbResponse); err != nil {
		return nil, fmt.Errorf("failed to parse database response: %w", err)
	}

	return dbResponse.Databases, nil
}

// searchGeneSpecificDatabase searches within a specific gene database
func (l *LOVDClient) searchGeneSpecificDatabase(ctx context.Context, variant *domain.StandardizedVariant, database LOVDDatabase) (*domain.LOVDData, error) {
	// Rate limiting
	select {
	case <-time.After(l.rateLimit):
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Construct search query based on variant information
	searchQuery := l.buildVariantQuery(variant)
	
	// Use the gene-specific database URL
	apiURL := fmt.Sprintf("%s/api/v1/variants", database.DatabaseURL)
	params := url.Values{
		"search": {searchQuery},
		"format": {"json"},
		"limit":  {"20"},
	}

	if l.apiKey != "" {
		params.Set("api_key", l.apiKey)
	}

	fullURL := fmt.Sprintf("%s?%s", apiURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create search request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "ACMG-AMP-MCP-Server/1.0")

	resp, err := l.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute search request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("LOVD search returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read search response: %w", err)
	}

	var searchResponse LOVDVariantsResponse
	if err := json.Unmarshal(body, &searchResponse); err != nil {
		return nil, fmt.Errorf("failed to parse search response: %w", err)
	}

	// Convert to domain object
	return l.convertToLOVDData(searchResponse.Data, database.GeneSymbol)
}

// searchGlobalDatabase searches the global LOVD database when no gene-specific database exists
func (l *LOVDClient) searchGlobalDatabase(ctx context.Context, variant *domain.StandardizedVariant) (*domain.LOVDData, error) {
	// Rate limiting
	select {
	case <-time.After(l.rateLimit):
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Construct search query
	searchQuery := l.buildVariantQuery(variant)
	
	// Use global LOVD API
	apiURL := fmt.Sprintf("%s/api/v1/variants", l.baseURL)
	params := url.Values{
		"search": {searchQuery},
		"format": {"json"},
		"limit":  {"20"},
	}

	if l.apiKey != "" {
		params.Set("api_key", l.apiKey)
	}

	fullURL := fmt.Sprintf("%s?%s", apiURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create global search request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "ACMG-AMP-MCP-Server/1.0")

	resp, err := l.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute global search request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("LOVD global search returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read global search response: %w", err)
	}

	var searchResponse LOVDVariantsResponse
	if err := json.Unmarshal(body, &searchResponse); err != nil {
		return nil, fmt.Errorf("failed to parse global search response: %w", err)
	}

	// Convert to domain object
	return l.convertToLOVDData(searchResponse.Data, variant.GeneSymbol)
}

// buildVariantQuery constructs a search query for LOVD
func (l *LOVDClient) buildVariantQuery(variant *domain.StandardizedVariant) string {
	var queryParts []string

	// Use gene symbol as primary search term
	if variant.GeneSymbol != "" {
		queryParts = append(queryParts, fmt.Sprintf("gene:%s", variant.GeneSymbol))
	}

	// Add HGVS notation if available
	if variant.HGVSCoding != "" {
		// Extract the change part from HGVS
		parts := strings.Split(variant.HGVSCoding, ":")
		if len(parts) > 1 {
			queryParts = append(queryParts, fmt.Sprintf("dna:%s", parts[1]))
		}
	}

	if variant.HGVSProtein != "" {
		parts := strings.Split(variant.HGVSProtein, ":")
		if len(parts) > 1 {
			queryParts = append(queryParts, fmt.Sprintf("protein:%s", parts[1]))
		}
	}

	// Add genomic position if available
	if variant.Chromosome != "" && variant.Position > 0 {
		queryParts = append(queryParts, fmt.Sprintf("position:%s:%d", variant.Chromosome, variant.Position))
	}

	return strings.Join(queryParts, " AND ")
}

// selectPrimaryDatabase selects the most appropriate database from available options
func (l *LOVDClient) selectPrimaryDatabase(databases []LOVDDatabase) LOVDDatabase {
	if len(databases) == 0 {
		return LOVDDatabase{}
	}

	// Prefer active databases with more variants
	var bestDB LOVDDatabase
	maxVariants := -1

	for _, db := range databases {
		if db.IsActive && db.VariantCount > maxVariants {
			bestDB = db
			maxVariants = db.VariantCount
		}
	}

	// If no active database found, return the first one
	if maxVariants == -1 {
		return databases[0]
	}

	return bestDB
}

// convertToLOVDData converts LOVD API response to domain object
func (l *LOVDClient) convertToLOVDData(variants []LOVDVariant, geneSymbol string) (*domain.LOVDData, error) {
	if len(variants) == 0 {
		return &domain.LOVDData{
			GeneSpecificDB: geneSymbol,
		}, nil
	}

	// Use the first (most relevant) variant
	variant := variants[0]

	// Parse dates
	var submissionDate, lastUpdated time.Time
	if variant.SubmissionDate != "" {
		if parsed, err := time.Parse("2006-01-02", variant.SubmissionDate); err == nil {
			submissionDate = parsed
		}
	}
	if variant.LastUpdated != "" {
		if parsed, err := time.Parse("2006-01-02", variant.LastUpdated); err == nil {
			lastUpdated = parsed
		}
	}

	// Convert functional data
	var functionalData []domain.LOVDFunctionalData
	for _, fa := range variant.FunctionalAnalysis {
		functionalData = append(functionalData, domain.LOVDFunctionalData{
			StudyType:   fa.StudyType,
			Method:      fa.Method,
			Result:      fa.Result,
			Conclusion:  fa.Conclusion,
			Reference:   fa.Reference,
			Reliability: fa.Reliability,
		})
	}

	return &domain.LOVDData{
		VariantID:           variant.VariantID,
		GeneSpecificDB:      variant.GeneSymbol,
		Classification:      variant.Classification,
		ClinicalDescription: variant.ClinicalDescription,
		Phenotype:           variant.Phenotype,
		Pathogenicity:       variant.Pathogenicity,
		FunctionalData:      functionalData,
		References:          variant.References,
		SubmissionDate:      submissionDate,
		LastUpdated:         lastUpdated,
	}, nil
}

// GetDatabaseInfo returns information about available LOVD databases for a gene
func (l *LOVDClient) GetDatabaseInfo(ctx context.Context, geneSymbol string) ([]LOVDDatabase, error) {
	return l.getGeneDatabases(ctx, geneSymbol)
}

// HealthCheck performs a health check on the LOVD service
func (l *LOVDClient) HealthCheck(ctx context.Context) error {
	// Simple health check by requesting the API info
	healthURL := fmt.Sprintf("%s/api/v1/info", l.baseURL)
	
	req, err := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	resp, err := l.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("LOVD health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("LOVD health check returned status %d", resp.StatusCode)
	}

	return nil
}