package external

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/time/rate"
)

// HGNCClient handles interactions with the HUGO Gene Nomenclature Committee (HGNC) API
type HGNCClient struct {
	baseURL    string
	httpClient *http.Client
	rateLimit  *rate.Limiter
}

// HGNCConfig represents configuration for HGNC API client
type HGNCConfig struct {
	BaseURL     string        `json:"base_url"`
	Timeout     time.Duration `json:"timeout"`
	RateLimit   int           `json:"rate_limit"` // requests per second
	MaxRetries  int           `json:"max_retries"`
}

// HGNCResponse represents the JSON response structure from HGNC API
type HGNCResponse struct {
	Response struct {
		NumFound int `json:"numFound"`
		Docs     []struct {
			Symbol              string   `json:"symbol"`
			Name                string   `json:"name"`
			Status              string   `json:"status"`
			LocusType           string   `json:"locus_type"`
			LocusGroup          string   `json:"locus_group"`
			PreviousSymbols     []string `json:"prev_symbol"`
			AliasSymbols        []string `json:"alias_symbol"`
			RefSeqAccession     []string `json:"refseq_accession"`
			EnsemblGeneID       string   `json:"ensembl_gene_id"`
			Location            string   `json:"location"`
			LocationSortable    string   `json:"location_sortable"`
			HGNCID              string   `json:"hgnc_id"`
			DateModified        string   `json:"date_modified"`
		} `json:"docs"`
	} `json:"response"`
}

// TranscriptInfo represents gene-to-transcript mapping information
type TranscriptInfo struct {
	RefSeqID        string                `json:"refseq_id"`
	GeneSymbol      string                `json:"gene_symbol"`
	TranscriptType  TranscriptType        `json:"type"`
	Length          int                   `json:"length"`
	Source          ExternalServiceType   `json:"source"`
	LastUpdated     time.Time             `json:"last_updated"`
	Metadata        TranscriptMetadata    `json:"metadata"`
}

// TranscriptMetadata contains additional transcript information
type TranscriptMetadata struct {
	ChromosomeLocation string   `json:"chromosome_location"`
	GenomicCoordinates string   `json:"genomic_coordinates"`
	ProteinID         string   `json:"protein_id,omitempty"`
	Aliases           []string `json:"aliases,omitempty"`
	HGNCID            string   `json:"hgnc_id,omitempty"`
	Status            string   `json:"status,omitempty"`
}

// GeneValidationResult represents gene symbol validation outcome
type GeneValidationResult struct {
	IsValid          bool                `json:"is_valid"`
	NormalizedSymbol string              `json:"normalized_symbol,omitempty"`
	Suggestions      []string            `json:"suggestions,omitempty"`
	DeprecatedFrom   string              `json:"deprecated_from,omitempty"`
	ValidationErrors []string            `json:"validation_errors,omitempty"`
	Source           ExternalServiceType `json:"source"`
}

// TranscriptType represents the type of transcript
type TranscriptType string

const (
	TranscriptTypeCanonical    TranscriptType = "canonical"
	TranscriptTypeAlternative  TranscriptType = "alternative"
	TranscriptTypeUnknown      TranscriptType = "unknown"
)

// ExternalServiceType represents the source of gene/transcript information
type ExternalServiceType string

const (
	ServiceTypeHGNC     ExternalServiceType = "HGNC"
	ServiceTypeRefSeq   ExternalServiceType = "RefSeq"
	ServiceTypeEnsembl  ExternalServiceType = "Ensembl"
)

// NewHGNCClient creates a new HGNC API client
func NewHGNCClient(config HGNCConfig) *HGNCClient {
	if config.BaseURL == "" {
		config.BaseURL = "https://rest.genenames.org"
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.RateLimit == 0 {
		config.RateLimit = 3 // HGNC recommendation: 3 requests per second
	}

	return &HGNCClient{
		baseURL: config.BaseURL,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		rateLimit: rate.NewLimiter(rate.Limit(config.RateLimit), 1),
	}
}

// GetCanonicalTranscript retrieves the canonical transcript for a gene symbol
func (h *HGNCClient) GetCanonicalTranscript(ctx context.Context, geneSymbol string) (*TranscriptInfo, error) {
	// Validate and normalize gene symbol
	geneSymbol = strings.TrimSpace(strings.ToUpper(geneSymbol))
	if geneSymbol == "" {
		return nil, fmt.Errorf("gene symbol cannot be empty")
	}

	// Rate limiting
	if err := h.rateLimit.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit wait failed: %w", err)
	}

	// Search for gene symbol
	hgncData, err := h.searchGeneSymbol(ctx, geneSymbol)
	if err != nil {
		return nil, fmt.Errorf("failed to search gene symbol %s: %w", geneSymbol, err)
	}

	if len(hgncData.Response.Docs) == 0 {
		return nil, fmt.Errorf("gene symbol %s not found in HGNC database", geneSymbol)
	}

	// Use the first result (most relevant)
	doc := hgncData.Response.Docs[0]
	
	// Get the best RefSeq transcript (prefer NM_ over others)
	var refSeqID string
	for _, accession := range doc.RefSeqAccession {
		if strings.HasPrefix(accession, "NM_") {
			refSeqID = accession
			break
		}
	}
	if refSeqID == "" && len(doc.RefSeqAccession) > 0 {
		refSeqID = doc.RefSeqAccession[0] // Use first available if no NM_ found
	}

	if refSeqID == "" {
		return nil, fmt.Errorf("no RefSeq transcript found for gene symbol %s", geneSymbol)
	}

	// Parse last modified date
	var lastUpdated time.Time
	if doc.DateModified != "" {
		if parsed, err := time.Parse("2006-01-02T15:04:05Z", doc.DateModified); err == nil {
			lastUpdated = parsed
		}
	}

	return &TranscriptInfo{
		RefSeqID:       refSeqID,
		GeneSymbol:     doc.Symbol,
		TranscriptType: TranscriptTypeCanonical,
		Source:         ServiceTypeHGNC,
		LastUpdated:    lastUpdated,
		Metadata: TranscriptMetadata{
			ChromosomeLocation: doc.Location,
			GenomicCoordinates: doc.LocationSortable,
			Aliases:           append(doc.PreviousSymbols, doc.AliasSymbols...),
			HGNCID:            doc.HGNCID,
			Status:            doc.Status,
		},
	}, nil
}

// ValidateGeneSymbol validates a gene symbol against HGNC standards
func (h *HGNCClient) ValidateGeneSymbol(ctx context.Context, geneSymbol string) (*GeneValidationResult, error) {
	originalSymbol := geneSymbol
	geneSymbol = strings.TrimSpace(strings.ToUpper(geneSymbol))
	
	if geneSymbol == "" {
		return &GeneValidationResult{
			IsValid:          false,
			ValidationErrors: []string{"Gene symbol cannot be empty"},
			Source:           ServiceTypeHGNC,
		}, nil
	}

	// Rate limiting
	if err := h.rateLimit.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit wait failed: %w", err)
	}

	// Search for exact match
	hgncData, err := h.searchGeneSymbol(ctx, geneSymbol)
	if err != nil {
		return nil, fmt.Errorf("failed to validate gene symbol %s: %w", geneSymbol, err)
	}

	result := &GeneValidationResult{
		Source: ServiceTypeHGNC,
	}

	if len(hgncData.Response.Docs) > 0 {
		doc := hgncData.Response.Docs[0]
		
		// Check for exact match
		if doc.Symbol == geneSymbol {
			result.IsValid = true
			result.NormalizedSymbol = doc.Symbol
			return result, nil
		}

		// Check if it's a previous symbol or alias
		for _, prevSymbol := range doc.PreviousSymbols {
			if strings.ToUpper(prevSymbol) == geneSymbol {
				result.IsValid = true
				result.NormalizedSymbol = doc.Symbol
				result.DeprecatedFrom = prevSymbol
				result.Suggestions = []string{fmt.Sprintf("Current symbol: %s (was: %s)", doc.Symbol, prevSymbol)}
				return result, nil
			}
		}

		for _, aliasSymbol := range doc.AliasSymbols {
			if strings.ToUpper(aliasSymbol) == geneSymbol {
				result.IsValid = true
				result.NormalizedSymbol = doc.Symbol
				result.Suggestions = []string{fmt.Sprintf("Official symbol: %s (alias: %s)", doc.Symbol, aliasSymbol)}
				return result, nil
			}
		}
	}

	// Gene not found - try to find similar symbols
	suggestions := h.findSimilarSymbols(ctx, geneSymbol)
	
	result.IsValid = false
	result.ValidationErrors = []string{fmt.Sprintf("Gene symbol '%s' not found in HGNC database", originalSymbol)}
	result.Suggestions = suggestions
	
	return result, nil
}

// SearchGeneVariants searches for variants associated with a gene symbol
func (h *HGNCClient) SearchGeneVariants(ctx context.Context, geneSymbol string) ([]*VariantInfo, error) {
	// HGNC doesn't provide variant information directly
	// This is a placeholder for interface compliance
	// Real variant information would come from other databases like ClinVar
	return nil, fmt.Errorf("HGNC does not provide variant information - use ClinVar or other variant databases")
}

// searchGeneSymbol performs the actual API call to search for a gene symbol
func (h *HGNCClient) searchGeneSymbol(ctx context.Context, geneSymbol string) (*HGNCResponse, error) {
	// Build search URL
	params := url.Values{
		"q":      {fmt.Sprintf("symbol:%s OR prev_symbol:%s OR alias_symbol:%s", geneSymbol, geneSymbol, geneSymbol)},
		"rows":   {"10"},
		"format": {"json"},
	}
	
	searchURL := fmt.Sprintf("%s/search?%s", h.baseURL, params.Encode())
	
	// Create request with context
	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "ACMG-AMP-MCP-Server/1.0")

	// Execute request
	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HGNC API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var hgncResponse HGNCResponse
	if err := json.Unmarshal(body, &hgncResponse); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	return &hgncResponse, nil
}

// findSimilarSymbols attempts to find similar gene symbols for suggestions
func (h *HGNCClient) findSimilarSymbols(ctx context.Context, geneSymbol string) []string {
	// Simple fuzzy matching - try common variations
	variations := []string{
		geneSymbol + "1",   // Try with number suffix
		geneSymbol[:len(geneSymbol)-1], // Try without last character if length > 1
	}
	
	var suggestions []string
	for _, variation := range variations {
		if len(variation) > 0 {
			if hgncData, err := h.searchGeneSymbol(ctx, variation); err == nil {
				if len(hgncData.Response.Docs) > 0 {
					suggestions = append(suggestions, hgncData.Response.Docs[0].Symbol)
				}
			}
		}
	}
	
	// Remove duplicates and limit to 5 suggestions
	uniqueSuggestions := make([]string, 0)
	seen := make(map[string]bool)
	for _, suggestion := range suggestions {
		if !seen[suggestion] && len(uniqueSuggestions) < 5 {
			uniqueSuggestions = append(uniqueSuggestions, suggestion)
			seen[suggestion] = true
		}
	}
	
	return uniqueSuggestions
}

// VariantInfo placeholder for interface compliance
type VariantInfo struct {
	ID          string `json:"id"`
	GeneSymbol  string `json:"gene_symbol"`
	Description string `json:"description"`
}