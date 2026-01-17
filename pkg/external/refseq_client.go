package external

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/time/rate"
)

// RefSeqClient handles interactions with NCBI RefSeq database via E-utilities
type RefSeqClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	rateLimit  *rate.Limiter
}

// RefSeqConfig represents configuration for RefSeq API client
type RefSeqConfig struct {
	BaseURL    string        `json:"base_url"`
	APIKey     string        `json:"api_key"`
	Timeout    time.Duration `json:"timeout"`
	RateLimit  int           `json:"rate_limit"` // requests per second
	MaxRetries int           `json:"max_retries"`
}

// ESearchResponse represents the XML response from E-utilities esearch
type ESearchResponse struct {
	XMLName xml.Name `xml:"eSearchResult"`
	Count   int      `xml:"Count"`
	RetMax  int      `xml:"RetMax"`
	IDList  struct {
		IDs []string `xml:"Id"`
	} `xml:"IdList"`
}

// ESummaryResponse represents the XML response from E-utilities esummary
type ESummaryResponse struct {
	XMLName        xml.Name           `xml:"eSummaryResult"`
	DocumentSummary []RefSeqDocumentSummary `xml:"DocumentSummary"`
}

// RefSeqDocumentSummary represents a single record from RefSeq
type RefSeqDocumentSummary struct {
	UID                string `xml:"uid,attr"`
	Caption            string `xml:"Caption"`
	Title              string `xml:"Title"`
	Extra              string `xml:"Extra"`
	Gi                 string `xml:"Gi"`
	CreateDate         string `xml:"CreateDate"`
	UpdateDate         string `xml:"UpdateDate"`
	Flags              string `xml:"Flags"`
	TaxId              string `xml:"TaxId"`
	Length             string `xml:"Length"`
	Status             string `xml:"Status"`
	ReplacedBy         string `xml:"ReplacedBy"`
	Comment            string `xml:"Comment"`
	AssemblyAcc        string `xml:"AssemblyAcc"`
	AssemblyGi         string `xml:"AssemblyGi"`
	Chr                string `xml:"Chr"`
	Sub                string `xml:"Sub"`
	Subtype            string `xml:"Subtype"`
}

// NewRefSeqClient creates a new RefSeq API client
func NewRefSeqClient(config RefSeqConfig) *RefSeqClient {
	if config.BaseURL == "" {
		config.BaseURL = "https://eutils.ncbi.nlm.nih.gov/entrez/eutils"
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.RateLimit == 0 {
		if config.APIKey != "" {
			config.RateLimit = 10 // With API key: 10 requests per second
		} else {
			config.RateLimit = 3 // Without API key: 3 requests per second
		}
	}

	return &RefSeqClient{
		baseURL:    config.BaseURL,
		apiKey:     config.APIKey,
		httpClient: &http.Client{Timeout: config.Timeout},
		rateLimit:  rate.NewLimiter(rate.Limit(config.RateLimit), 1),
	}
}

// GetCanonicalTranscript retrieves transcript information for a gene symbol from RefSeq
func (r *RefSeqClient) GetCanonicalTranscript(ctx context.Context, geneSymbol string) (*TranscriptInfo, error) {
	// Validate and normalize gene symbol
	geneSymbol = strings.TrimSpace(strings.ToUpper(geneSymbol))
	if geneSymbol == "" {
		return nil, fmt.Errorf("gene symbol cannot be empty")
	}

	// Rate limiting
	if err := r.rateLimit.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit wait failed: %w", err)
	}

	// Search for gene symbol in RefSeq
	searchTerm := fmt.Sprintf("%s[Gene Name] AND \"Homo sapiens\"[Organism] AND \"mRNA\"[Filter]", geneSymbol)
	searchResponse, err := r.esearch(ctx, "nucleotide", searchTerm, 20)
	if err != nil {
		return nil, fmt.Errorf("failed to search RefSeq for gene %s: %w", geneSymbol, err)
	}

	if len(searchResponse.IDList.IDs) == 0 {
		return nil, fmt.Errorf("no RefSeq entries found for gene symbol %s", geneSymbol)
	}

	// Get detailed information for the transcripts
	if err := r.rateLimit.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit wait failed: %w", err)
	}

	summaryResponse, err := r.esummary(ctx, "nucleotide", searchResponse.IDList.IDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get RefSeq summary for gene %s: %w", geneSymbol, err)
	}

	if len(summaryResponse.DocumentSummary) == 0 {
		return nil, fmt.Errorf("no RefSeq summary found for gene symbol %s", geneSymbol)
	}

	// Find the best transcript (prefer NM_ accessions, then longest)
	var bestTranscript *RefSeqDocumentSummary
	var bestScore int

	for i := range summaryResponse.DocumentSummary {
		doc := &summaryResponse.DocumentSummary[i]
		score := r.scoreTranscript(doc)
		if bestTranscript == nil || score > bestScore {
			bestTranscript = doc
			bestScore = score
		}
	}

	if bestTranscript == nil {
		return nil, fmt.Errorf("no suitable transcript found for gene symbol %s", geneSymbol)
	}

	// Parse length
	var length int
	if bestTranscript.Length != "" {
		if parsed, err := strconv.Atoi(bestTranscript.Length); err == nil {
			length = parsed
		}
	}

	// Parse dates
	var lastUpdated time.Time
	if bestTranscript.UpdateDate != "" {
		if parsed, err := time.Parse("2006/01/02", bestTranscript.UpdateDate); err == nil {
			lastUpdated = parsed
		}
	}

	// Determine transcript type
	transcriptType := TranscriptTypeCanonical
	if strings.HasPrefix(bestTranscript.Caption, "XM_") || strings.HasPrefix(bestTranscript.Caption, "XR_") {
		transcriptType = TranscriptTypeAlternative
	}

	return &TranscriptInfo{
		RefSeqID:       bestTranscript.Caption,
		GeneSymbol:     geneSymbol,
		TranscriptType: transcriptType,
		Length:         length,
		Source:         ServiceTypeRefSeq,
		LastUpdated:    lastUpdated,
		Metadata: TranscriptMetadata{
			ChromosomeLocation: bestTranscript.Chr,
			GenomicCoordinates: fmt.Sprintf("GI:%s", bestTranscript.Gi),
			Status:            bestTranscript.Status,
		},
	}, nil
}

// ValidateGeneSymbol validates a gene symbol against RefSeq database
func (r *RefSeqClient) ValidateGeneSymbol(ctx context.Context, geneSymbol string) (*GeneValidationResult, error) {
	originalSymbol := geneSymbol
	geneSymbol = strings.TrimSpace(strings.ToUpper(geneSymbol))

	if geneSymbol == "" {
		return &GeneValidationResult{
			IsValid:          false,
			ValidationErrors: []string{"Gene symbol cannot be empty"},
			Source:           ServiceTypeRefSeq,
		}, nil
	}

	// Rate limiting
	if err := r.rateLimit.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit wait failed: %w", err)
	}

	// Search for gene symbol
	searchTerm := fmt.Sprintf("%s[Gene Name] AND \"Homo sapiens\"[Organism]", geneSymbol)
	searchResponse, err := r.esearch(ctx, "nucleotide", searchTerm, 5)
	if err != nil {
		return nil, fmt.Errorf("failed to validate gene symbol %s: %w", geneSymbol, err)
	}

	result := &GeneValidationResult{
		Source: ServiceTypeRefSeq,
	}

	if len(searchResponse.IDList.IDs) > 0 {
		result.IsValid = true
		result.NormalizedSymbol = geneSymbol
		result.Suggestions = []string{fmt.Sprintf("Found %d RefSeq entries", len(searchResponse.IDList.IDs))}
	} else {
		result.IsValid = false
		result.ValidationErrors = []string{fmt.Sprintf("Gene symbol '%s' not found in RefSeq database", originalSymbol)}
		
		// Try to find similar symbols
		suggestions := r.findSimilarSymbols(ctx, geneSymbol)
		result.Suggestions = suggestions
	}

	return result, nil
}

// SearchGeneVariants searches for variants associated with a gene symbol
func (r *RefSeqClient) SearchGeneVariants(ctx context.Context, geneSymbol string) ([]*VariantInfo, error) {
	// RefSeq doesn't directly provide variant information
	// This would typically query ClinVar or other variant databases
	return nil, fmt.Errorf("RefSeq does not provide variant information directly - use ClinVar or other variant databases")
}

// esearch performs an E-utilities esearch query
func (r *RefSeqClient) esearch(ctx context.Context, database, term string, retmax int) (*ESearchResponse, error) {
	params := url.Values{
		"db":      {database},
		"term":    {term},
		"retmode": {"xml"},
		"retmax":  {strconv.Itoa(retmax)},
		"sort":    {"relevance"},
	}

	if r.apiKey != "" {
		params.Set("api_key", r.apiKey)
	}

	searchURL := fmt.Sprintf("%s/esearch.fcgi?%s", r.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "ACMG-AMP-MCP-Server/1.0")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("RefSeq esearch returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var searchResponse ESearchResponse
	if err := xml.Unmarshal(body, &searchResponse); err != nil {
		return nil, fmt.Errorf("failed to parse XML response: %w", err)
	}

	return &searchResponse, nil
}

// esummary performs an E-utilities esummary query
func (r *RefSeqClient) esummary(ctx context.Context, database string, ids []string) (*ESummaryResponse, error) {
	if len(ids) == 0 {
		return &ESummaryResponse{}, nil
	}

	params := url.Values{
		"db":      {database},
		"id":      {strings.Join(ids, ",")},
		"retmode": {"xml"},
	}

	if r.apiKey != "" {
		params.Set("api_key", r.apiKey)
	}

	summaryURL := fmt.Sprintf("%s/esummary.fcgi?%s", r.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", summaryURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "ACMG-AMP-MCP-Server/1.0")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("RefSeq esummary returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var summaryResponse ESummaryResponse
	if err := xml.Unmarshal(body, &summaryResponse); err != nil {
		return nil, fmt.Errorf("failed to parse XML response: %w", err)
	}

	return &summaryResponse, nil
}

// scoreTranscript assigns a score to prioritize transcripts
func (r *RefSeqClient) scoreTranscript(doc *RefSeqDocumentSummary) int {
	score := 0

	// Prefer NM_ (curated mRNA) over XM_ (predicted mRNA)
	if strings.HasPrefix(doc.Caption, "NM_") {
		score += 100
	} else if strings.HasPrefix(doc.Caption, "XM_") {
		score += 50
	}

	// Prefer longer transcripts
	if length, err := strconv.Atoi(doc.Length); err == nil {
		score += length / 1000 // Add points based on length in kb
	}

	// Prefer more recent updates
	if updateDate, err := time.Parse("2006/01/02", doc.UpdateDate); err == nil {
		// More recent = higher score (days since epoch / 1000 to keep reasonable scale)
		score += int(updateDate.Unix()) / 86400000
	}

	return score
}

// findSimilarSymbols attempts to find similar gene symbols for suggestions
func (r *RefSeqClient) findSimilarSymbols(ctx context.Context, geneSymbol string) []string {
	// Simple fuzzy matching - try common variations
	variations := []string{
		geneSymbol + "1",                    // Try with number suffix
		strings.TrimSuffix(geneSymbol, "1"), // Try without number suffix
	}

	if len(geneSymbol) > 1 {
		variations = append(variations, geneSymbol[:len(geneSymbol)-1]) // Try without last character
	}

	var suggestions []string
	for _, variation := range variations {
		if len(variation) > 0 && variation != geneSymbol {
			searchTerm := fmt.Sprintf("%s[Gene Name] AND \"Homo sapiens\"[Organism]", variation)
			if searchResponse, err := r.esearch(ctx, "nucleotide", searchTerm, 1); err == nil {
				if len(searchResponse.IDList.IDs) > 0 {
					suggestions = append(suggestions, variation)
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