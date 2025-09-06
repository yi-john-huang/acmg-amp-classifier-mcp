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

// HGMDClient handles interactions with Human Gene Mutation Database
type HGMDClient struct {
	baseURL       string
	apiKey        string
	license       string // Professional license key
	isProfessional bool   // Whether using professional API
	httpClient    *http.Client
	rateLimit     time.Duration
}

// HGMDConfig contains configuration for HGMD client
type HGMDConfig struct {
	BaseURL        string        // Base URL for HGMD API
	APIKey         string        // API key for authentication
	License        string        // Professional license key
	IsProfessional bool          // Use professional API endpoint
	Timeout        time.Duration
	RateLimit      int
}

// NewHGMDClient creates a new HGMD API client
func NewHGMDClient(config HGMDConfig) *HGMDClient {
	if config.BaseURL == "" {
		if config.IsProfessional {
			config.BaseURL = "https://my.qiagendigitalinsights.com/bbp/view/hgmd/pro"
		} else {
			config.BaseURL = "https://my.qiagendigitalinsights.com/bbp/view/hgmd/public"
		}
	}
	if config.RateLimit == 0 {
		config.RateLimit = 5 // 5 requests per second (conservative)
	}
	
	return &HGMDClient{
		baseURL:       config.BaseURL,
		apiKey:        config.APIKey,
		license:       config.License,
		isProfessional: config.IsProfessional && config.License != "",
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		rateLimit: time.Second / time.Duration(config.RateLimit),
	}
}

// HGMDSearchResponse represents the JSON response from HGMD search
type HGMDSearchResponse struct {
	Success bool          `json:"success"`
	Data    []HGMDMutation `json:"data"`
	Meta    struct {
		Total      int    `json:"total"`
		Page       int    `json:"page"`
		PerPage    int    `json:"per_page"`
		Query      string `json:"query"`
		SearchTime string `json:"search_time"`
	} `json:"meta"`
	Message string `json:"message,omitempty"`
}

// HGMDMutation represents a single mutation entry from HGMD
type HGMDMutation struct {
	MutationID      string `json:"mutation_id"`
	AccessionNumber string `json:"accession_number"`
	GeneSymbol      string `json:"gene_symbol"`
	DiseaseName     string `json:"disease_name"`
	MutationType    string `json:"mutation_type"`    // DM, DM?, DP, FP, FTV
	Classification  string `json:"classification"`   // Disease-causing, Likely pathogenic, etc.
	PhenotypeMIM    string `json:"phenotype_mim"`
	GeneMIM         string `json:"gene_mim"`
	Chromosome      string `json:"chromosome"`
	GenomicLocation string `json:"genomic_location"`
	HGVSGenomic     string `json:"hgvs_genomic"`
	HGVSCoding      string `json:"hgvs_coding"`
	HGVSProtein     string `json:"hgvs_protein"`
	Reference       struct {
		Title     string `json:"title"`
		Authors   string `json:"authors"`
		Journal   string `json:"journal"`
		Year      int    `json:"year"`
		PubMedID  string `json:"pubmed_id"`
		DOI       string `json:"doi"`
	} `json:"reference"`
	SubmissionDate  string `json:"submission_date"`
	LastUpdated     string `json:"last_updated"`
	Inheritance     string `json:"inheritance"`      // AD, AR, XL, YL, Mt
	Tag             string `json:"tag"`              // Additional classification tags
	Codon           int    `json:"codon,omitempty"`
	AAChange        string `json:"aa_change,omitempty"`
	Exon            string `json:"exon,omitempty"`
	Comments        string `json:"comments,omitempty"`
	Population      struct {
		Ethnicity string  `json:"ethnicity,omitempty"`
		Frequency float64 `json:"frequency,omitempty"`
	} `json:"population,omitempty"`
}

// HGMDGeneResponse represents gene-specific information from HGMD
type HGMDGeneResponse struct {
	Success bool     `json:"success"`
	Gene    HGMDGene `json:"gene"`
	Message string   `json:"message,omitempty"`
}

// HGMDGene represents gene information from HGMD
type HGMDGene struct {
	Symbol          string `json:"symbol"`
	Name            string `json:"name"`
	MIMNumber       string `json:"mim_number"`
	Chromosome      string `json:"chromosome"`
	TotalMutations  int    `json:"total_mutations"`
	DMutations      int    `json:"d_mutations"`      // Disease-causing mutations
	DQMutations     int    `json:"dq_mutations"`     // Likely disease-causing
	DPMutations     int    `json:"dp_mutations"`     // Disease-associated polymorphisms
	FPMutations     int    `json:"fp_mutations"`     // Functional polymorphisms
	FTVMutations    int    `json:"ftv_mutations"`    // Functional variants
	LastUpdated     string `json:"last_updated"`
}

// QueryVariant queries HGMD for variant information
func (h *HGMDClient) QueryVariant(ctx context.Context, variant *domain.StandardizedVariant) (*domain.HGMDData, error) {
	// Rate limiting
	select {
	case <-time.After(h.rateLimit):
		// Proceed after rate limit delay
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Build search query based on variant information
	searchQuery := h.buildSearchQuery(variant)
	if searchQuery == "" {
		return &domain.HGMDData{}, nil
	}

	// Perform the search
	mutations, err := h.searchMutations(ctx, searchQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to search HGMD: %w", err)
	}

	if len(mutations) == 0 {
		return &domain.HGMDData{
			GeneSymbol: variant.GeneSymbol,
		}, nil
	}

	// Convert the most relevant mutation to domain object
	return h.convertToHGMDData(mutations[0])
}

// QueryGene queries HGMD for gene-specific mutation statistics
func (h *HGMDClient) QueryGene(ctx context.Context, geneSymbol string) (*HGMDGene, error) {
	if geneSymbol == "" {
		return nil, fmt.Errorf("gene symbol is required")
	}

	// Rate limiting
	select {
	case <-time.After(h.rateLimit):
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Construct gene info API URL
	var apiURL string
	if h.isProfessional {
		apiURL = fmt.Sprintf("%s/api/gene/%s", h.baseURL, geneSymbol)
	} else {
		apiURL = fmt.Sprintf("%s/api/public/gene/%s", h.baseURL, geneSymbol)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create gene request: %w", err)
	}

	// Add authentication headers
	h.addAuthHeaders(req)

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute gene request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return &HGMDGene{Symbol: geneSymbol}, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HGMD gene query returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read gene response: %w", err)
	}

	var geneResponse HGMDGeneResponse
	if err := json.Unmarshal(body, &geneResponse); err != nil {
		return nil, fmt.Errorf("failed to parse gene response: %w", err)
	}

	if !geneResponse.Success {
		return nil, fmt.Errorf("HGMD gene query failed: %s", geneResponse.Message)
	}

	return &geneResponse.Gene, nil
}

// searchMutations performs the actual mutation search
func (h *HGMDClient) searchMutations(ctx context.Context, query string) ([]HGMDMutation, error) {
	// Construct search API URL based on professional vs public access
	var apiURL string
	if h.isProfessional {
		apiURL = fmt.Sprintf("%s/api/search/mutations", h.baseURL)
	} else {
		apiURL = fmt.Sprintf("%s/api/public/search", h.baseURL)
	}

	params := url.Values{
		"query":  {query},
		"format": {"json"},
		"limit":  {"20"},
	}

	fullURL := fmt.Sprintf("%s?%s", apiURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create search request: %w", err)
	}

	// Add authentication headers
	h.addAuthHeaders(req)

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute search request: %w", err)
	}
	defer resp.Body.Close()

	// Handle authentication errors
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("HGMD authentication failed - check API key and license")
	}

	if resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("HGMD access forbidden - professional license required")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HGMD search returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read search response: %w", err)
	}

	var searchResponse HGMDSearchResponse
	if err := json.Unmarshal(body, &searchResponse); err != nil {
		// Try fallback parsing for public API format
		return h.parseFallbackResponse(body)
	}

	if !searchResponse.Success {
		return nil, fmt.Errorf("HGMD search failed: %s", searchResponse.Message)
	}

	return searchResponse.Data, nil
}

// parseFallbackResponse handles different response formats
func (h *HGMDClient) parseFallbackResponse(body []byte) ([]HGMDMutation, error) {
	// Try parsing as simple array (public API might use different format)
	var mutations []HGMDMutation
	if err := json.Unmarshal(body, &mutations); err != nil {
		return nil, fmt.Errorf("failed to parse HGMD response: %w", err)
	}
	return mutations, nil
}

// buildSearchQuery constructs a search query for HGMD
func (h *HGMDClient) buildSearchQuery(variant *domain.StandardizedVariant) string {
	var queryParts []string

	// Primary search by gene symbol
	if variant.GeneSymbol != "" {
		queryParts = append(queryParts, fmt.Sprintf("gene:%s", variant.GeneSymbol))
	}

	// Add HGVS notation if available
	if variant.HGVSCoding != "" {
		// Extract the change part
		parts := strings.Split(variant.HGVSCoding, ":")
		if len(parts) > 1 {
			queryParts = append(queryParts, fmt.Sprintf("cdna:%s", parts[1]))
		}
	}

	if variant.HGVSProtein != "" {
		parts := strings.Split(variant.HGVSProtein, ":")
		if len(parts) > 1 {
			queryParts = append(queryParts, fmt.Sprintf("protein:%s", parts[1]))
		}
	}

	// Add chromosomal location if available
	if variant.Chromosome != "" && variant.Position > 0 {
		queryParts = append(queryParts, fmt.Sprintf("chr:%s", variant.Chromosome))
		queryParts = append(queryParts, fmt.Sprintf("pos:%d", variant.Position))
	}

	return strings.Join(queryParts, " AND ")
}

// addAuthHeaders adds authentication headers to the request
func (h *HGMDClient) addAuthHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "ACMG-AMP-MCP-Server/1.0")

	if h.apiKey != "" {
		req.Header.Set("X-API-Key", h.apiKey)
	}

	if h.isProfessional && h.license != "" {
		req.Header.Set("X-HGMD-License", h.license)
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", h.license))
	}
}

// convertToHGMDData converts HGMD API response to domain object
func (h *HGMDClient) convertToHGMDData(mutation HGMDMutation) (*domain.HGMDData, error) {
	// Parse submission date
	var submissionDate time.Time
	if mutation.SubmissionDate != "" {
		if parsed, err := time.Parse("2006-01-02", mutation.SubmissionDate); err == nil {
			submissionDate = parsed
		} else if parsed, err := time.Parse("2006/01/02", mutation.SubmissionDate); err == nil {
			submissionDate = parsed
		}
	}

	// Determine classification based on mutation type
	classification := h.mapMutationTypeToClassification(mutation.MutationType)

	// Format reference
	reference := h.formatReference(mutation.Reference)

	return &domain.HGMDData{
		MutationID:      mutation.AccessionNumber, // Use accession as ID
		DiseaseName:     mutation.DiseaseName,
		MutationType:    mutation.MutationType,
		Classification:  classification,
		PhenotypeMIM:    mutation.PhenotypeMIM,
		GeneSymbol:      mutation.GeneSymbol,
		Chromosome:      mutation.Chromosome,
		GenomicLocation: mutation.GenomicLocation,
		Reference:       reference,
		PubMedID:        mutation.Reference.PubMedID,
		SubmissionDate:  submissionDate,
		Inheritance:     mutation.Inheritance,
		Tag:             mutation.Tag,
	}, nil
}

// mapMutationTypeToClassification maps HGMD mutation types to classifications
func (h *HGMDClient) mapMutationTypeToClassification(mutationType string) string {
	switch strings.ToUpper(mutationType) {
	case "DM":
		return "Disease-causing"
	case "DM?":
		return "Likely pathogenic"
	case "DP":
		return "Disease-associated polymorphism"
	case "FP":
		return "Functional polymorphism"
	case "FTV":
		return "Functional variant"
	default:
		return "Unknown"
	}
}

// formatReference formats reference information
func (h *HGMDClient) formatReference(ref struct {
	Title    string `json:"title"`
	Authors  string `json:"authors"`
	Journal  string `json:"journal"`
	Year     int    `json:"year"`
	PubMedID string `json:"pubmed_id"`
	DOI      string `json:"doi"`
}) string {
	var parts []string

	if ref.Authors != "" {
		parts = append(parts, ref.Authors)
	}

	if ref.Title != "" {
		parts = append(parts, ref.Title)
	}

	if ref.Journal != "" && ref.Year > 0 {
		parts = append(parts, fmt.Sprintf("%s (%d)", ref.Journal, ref.Year))
	} else if ref.Journal != "" {
		parts = append(parts, ref.Journal)
	} else if ref.Year > 0 {
		parts = append(parts, strconv.Itoa(ref.Year))
	}

	return strings.Join(parts, ". ")
}

// HealthCheck performs a health check on the HGMD service
func (h *HGMDClient) HealthCheck(ctx context.Context) error {
	// Simple health check by requesting API status
	var healthURL string
	if h.isProfessional {
		healthURL = fmt.Sprintf("%s/api/status", h.baseURL)
	} else {
		healthURL = fmt.Sprintf("%s/api/public/status", h.baseURL)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	h.addAuthHeaders(req)

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("HGMD health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("HGMD authentication failed - check credentials")
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HGMD health check returned status %d", resp.StatusCode)
	}

	return nil
}

// IsProfessional returns whether the client is using professional API access
func (h *HGMDClient) IsProfessional() bool {
	return h.isProfessional
}