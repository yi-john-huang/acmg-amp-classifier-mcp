package external

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/time/rate"
)

// EnsemblClient handles interactions with the Ensembl REST API
type EnsemblClient struct {
	baseURL    string
	httpClient *http.Client
	rateLimit  *rate.Limiter
}

// EnsemblConfig represents configuration for Ensembl API client
type EnsemblConfig struct {
	BaseURL    string        `json:"base_url"`
	Timeout    time.Duration `json:"timeout"`
	RateLimit  int           `json:"rate_limit"` // requests per second
	MaxRetries int           `json:"max_retries"`
}

// EnsemblGeneResponse represents the JSON response from Ensembl gene lookup
type EnsemblGeneResponse struct {
	ID           string `json:"id"`
	DisplayName  string `json:"display_name"`
	Description  string `json:"description"`
	Biotype      string `json:"biotype"`
	Source       string `json:"source"`
	LogicName    string `json:"logic_name"`
	Species      string `json:"species"`
	Assembly     string `json:"assembly_name"`
	SeqRegionName string `json:"seq_region_name"`
	Start        int    `json:"start"`
	End          int    `json:"end"`
	Strand       int    `json:"strand"`
}

// EnsemblTranscriptResponse represents the JSON response from Ensembl transcript lookup
type EnsemblTranscriptResponse struct {
	ID               string `json:"id"`
	DisplayName      string `json:"display_name"`
	Description      string `json:"description"`
	Biotype          string `json:"biotype"`
	Source           string `json:"source"`
	LogicName        string `json:"logic_name"`
	Species          string `json:"species"`
	Assembly         string `json:"assembly_name"`
	SeqRegionName    string `json:"seq_region_name"`
	Start            int    `json:"start"`
	End              int    `json:"end"`
	Strand           int    `json:"strand"`
	Length           int    `json:"length"`
	IsCanonical      int    `json:"is_canonical"`
	ParentGene       string `json:"Parent"`
}

// EnsemblXRefResponse represents cross-references from Ensembl
type EnsemblXRefResponse []struct {
	PrimaryID   string `json:"primary_id"`
	DisplayID   string `json:"display_id"`
	Version     string `json:"version"`
	Description string `json:"description"`
	InfoType    string `json:"info_type"`
	InfoText    string `json:"info_text"`
	DbName      string `json:"dbname"`
	DbDisplayName string `json:"db_display_name"`
	Synonyms    []string `json:"synonyms"`
}

// NewEnsemblClient creates a new Ensembl API client
func NewEnsemblClient(config EnsemblConfig) *EnsemblClient {
	if config.BaseURL == "" {
		config.BaseURL = "https://rest.ensembl.org"
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.RateLimit == 0 {
		config.RateLimit = 15 // Ensembl allows 15 requests per second
	}

	return &EnsemblClient{
		baseURL: config.BaseURL,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		rateLimit: rate.NewLimiter(rate.Limit(config.RateLimit), 1),
	}
}

// GetCanonicalTranscript retrieves the canonical transcript for a gene symbol from Ensembl
func (e *EnsemblClient) GetCanonicalTranscript(ctx context.Context, geneSymbol string) (*TranscriptInfo, error) {
	// Validate and normalize gene symbol
	geneSymbol = strings.TrimSpace(strings.ToUpper(geneSymbol))
	if geneSymbol == "" {
		return nil, fmt.Errorf("gene symbol cannot be empty")
	}

	// Rate limiting
	if err := e.rateLimit.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit wait failed: %w", err)
	}

	// First, get gene information by symbol
	geneData, err := e.lookupGeneBySymbol(ctx, geneSymbol)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup gene %s in Ensembl: %w", geneSymbol, err)
	}

	// Rate limiting for transcript lookup
	if err := e.rateLimit.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit wait failed: %w", err)
	}

	// Get transcripts for this gene
	transcripts, err := e.getTranscriptsForGene(ctx, geneData.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get transcripts for gene %s: %w", geneSymbol, err)
	}

	if len(transcripts) == 0 {
		return nil, fmt.Errorf("no transcripts found for gene symbol %s", geneSymbol)
	}

	// Find canonical transcript or the best alternative
	var canonicalTranscript *EnsemblTranscriptResponse
	for i := range transcripts {
		if transcripts[i].IsCanonical == 1 {
			canonicalTranscript = &transcripts[i]
			break
		}
	}

	// If no canonical transcript, use the first protein-coding transcript
	if canonicalTranscript == nil {
		for i := range transcripts {
			if transcripts[i].Biotype == "protein_coding" {
				canonicalTranscript = &transcripts[i]
				break
			}
		}
	}

	// If still no transcript, use the first one
	if canonicalTranscript == nil {
		canonicalTranscript = &transcripts[0]
	}

	// Rate limiting for cross-reference lookup
	if err := e.rateLimit.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit wait failed: %w", err)
	}

	// Get RefSeq cross-references
	refSeqID, err := e.getRefSeqID(ctx, canonicalTranscript.ID)
	if err != nil {
		// Log warning but don't fail - we can still provide Ensembl transcript info
		refSeqID = ""
	}

	// Determine transcript type
	transcriptType := TranscriptTypeCanonical
	if canonicalTranscript.IsCanonical != 1 {
		transcriptType = TranscriptTypeAlternative
	}

	// Build genomic coordinates
	genomicCoords := fmt.Sprintf("chr%s:%d-%d:%d", 
		canonicalTranscript.SeqRegionName, 
		canonicalTranscript.Start, 
		canonicalTranscript.End, 
		canonicalTranscript.Strand)

	return &TranscriptInfo{
		RefSeqID:       refSeqID, // May be empty if no RefSeq mapping
		GeneSymbol:     geneData.DisplayName,
		TranscriptType: transcriptType,
		Length:         canonicalTranscript.Length,
		Source:         ServiceTypeEnsembl,
		LastUpdated:    time.Now(), // Ensembl doesn't provide last modified date
		Metadata: TranscriptMetadata{
			ChromosomeLocation: canonicalTranscript.SeqRegionName,
			GenomicCoordinates: genomicCoords,
			Aliases:           []string{canonicalTranscript.ID}, // Ensembl gene ID
		},
	}, nil
}

// ValidateGeneSymbol validates a gene symbol against Ensembl database
func (e *EnsemblClient) ValidateGeneSymbol(ctx context.Context, geneSymbol string) (*GeneValidationResult, error) {
	originalSymbol := geneSymbol
	geneSymbol = strings.TrimSpace(strings.ToUpper(geneSymbol))

	if geneSymbol == "" {
		return &GeneValidationResult{
			IsValid:          false,
			ValidationErrors: []string{"Gene symbol cannot be empty"},
			Source:           ServiceTypeEnsembl,
		}, nil
	}

	// Rate limiting
	if err := e.rateLimit.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit wait failed: %w", err)
	}

	// Try to lookup gene by symbol
	geneData, err := e.lookupGeneBySymbol(ctx, geneSymbol)
	if err != nil {
		return &GeneValidationResult{
			IsValid:          false,
			ValidationErrors: []string{fmt.Sprintf("Gene symbol '%s' not found in Ensembl database", originalSymbol)},
			Source:           ServiceTypeEnsembl,
			Suggestions:      e.findSimilarSymbols(ctx, geneSymbol),
		}, nil
	}

	return &GeneValidationResult{
		IsValid:          true,
		NormalizedSymbol: geneData.DisplayName,
		Source:           ServiceTypeEnsembl,
		Suggestions:      []string{fmt.Sprintf("Ensembl Gene ID: %s", geneData.ID)},
	}, nil
}

// SearchGeneVariants searches for variants associated with a gene symbol
func (e *EnsemblClient) SearchGeneVariants(ctx context.Context, geneSymbol string) ([]*VariantInfo, error) {
	// Ensembl has variant information, but the REST API for variants is complex
	// This would require additional implementation for variant lookup
	return nil, fmt.Errorf("Ensembl variant lookup not implemented - use specialized variant databases")
}

// lookupGeneBySymbol performs gene lookup by symbol
func (e *EnsemblClient) lookupGeneBySymbol(ctx context.Context, geneSymbol string) (*EnsemblGeneResponse, error) {
	lookupURL := fmt.Sprintf("%s/lookup/symbol/homo_sapiens/%s", e.baseURL, geneSymbol)

	req, err := http.NewRequestWithContext(ctx, "GET", lookupURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "ACMG-AMP-MCP-Server/1.0")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("gene symbol %s not found", geneSymbol)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Ensembl API returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var geneResponse EnsemblGeneResponse
	if err := json.Unmarshal(body, &geneResponse); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	return &geneResponse, nil
}

// getTranscriptsForGene retrieves all transcripts for a given gene ID
func (e *EnsemblClient) getTranscriptsForGene(ctx context.Context, geneID string) ([]EnsemblTranscriptResponse, error) {
	transcriptsURL := fmt.Sprintf("%s/lookup/id/%s", e.baseURL, geneID)

	req, err := http.NewRequestWithContext(ctx, "GET", transcriptsURL+"?expand=1", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "ACMG-AMP-MCP-Server/1.0")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Ensembl API returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse the expanded gene response which includes transcripts
	var expandedGene struct {
		Transcripts []EnsemblTranscriptResponse `json:"Transcript"`
	}
	if err := json.Unmarshal(body, &expandedGene); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	return expandedGene.Transcripts, nil
}

// getRefSeqID attempts to get the RefSeq ID for an Ensembl transcript
func (e *EnsemblClient) getRefSeqID(ctx context.Context, transcriptID string) (string, error) {
	xrefURL := fmt.Sprintf("%s/xrefs/id/%s", e.baseURL, transcriptID)

	req, err := http.NewRequestWithContext(ctx, "GET", xrefURL+"?external_db=RefSeq_mRNA", nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "ACMG-AMP-MCP-Server/1.0")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Ensembl xrefs API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	var xrefs EnsemblXRefResponse
	if err := json.Unmarshal(body, &xrefs); err != nil {
		return "", fmt.Errorf("failed to parse JSON response: %w", err)
	}

	// Look for RefSeq mRNA accession
	for _, xref := range xrefs {
		if xref.DbName == "RefSeq_mRNA" && strings.HasPrefix(xref.PrimaryID, "NM_") {
			return xref.PrimaryID, nil
		}
	}

	return "", fmt.Errorf("no RefSeq mRNA mapping found")
}

// findSimilarSymbols attempts to find similar gene symbols for suggestions
func (e *EnsemblClient) findSimilarSymbols(ctx context.Context, geneSymbol string) []string {
	// Simple fuzzy matching - try common variations
	variations := []string{
		geneSymbol + "1",   // Try with number suffix
		geneSymbol + "2",   // Try with different number suffix
	}

	if len(geneSymbol) > 1 {
		variations = append(variations, geneSymbol[:len(geneSymbol)-1]) // Try without last character
	}

	var suggestions []string
	for _, variation := range variations {
		if len(variation) > 0 && variation != geneSymbol {
			if _, err := e.lookupGeneBySymbol(ctx, variation); err == nil {
				suggestions = append(suggestions, variation)
			}
		}
	}

	// Remove duplicates and limit to 3 suggestions
	uniqueSuggestions := make([]string, 0)
	seen := make(map[string]bool)
	for _, suggestion := range suggestions {
		if !seen[suggestion] && len(uniqueSuggestions) < 3 {
			uniqueSuggestions = append(uniqueSuggestions, suggestion)
			seen[suggestion] = true
		}
	}

	return uniqueSuggestions
}