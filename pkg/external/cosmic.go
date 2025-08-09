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

// COSMICClient handles interactions with the COSMIC database API
type COSMICClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	rateLimit  time.Duration
}

// NewCOSMICClient creates a new COSMIC API client
func NewCOSMICClient(config domain.COSMICConfig) *COSMICClient {
	return &COSMICClient{
		baseURL: config.BaseURL,
		apiKey:  config.APIKey,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		rateLimit: time.Second / time.Duration(config.RateLimit),
	}
}

// COSMICVariantResponse represents the JSON response from COSMIC API
type COSMICVariantResponse struct {
	Data []struct {
		CosmicID              string `json:"cosmic_id"`
		GeneName              string `json:"gene_name"`
		Transcript            string `json:"transcript"`
		CDSMutation           string `json:"cds_mutation"`
		AAMutation            string `json:"aa_mutation"`
		PrimaryTissue         string `json:"primary_tissue"`
		PrimaryHistology      string `json:"primary_histology"`
		SampleCount           int    `json:"sample_count"`
		MutationCount         int    `json:"mutation_count"`
		FathmmmScore          string `json:"fathmm_score"`
		FathmmmPrediction     string `json:"fathmm_prediction"`
		MutationSomaticStatus string `json:"mutation_somatic_status"`
		Pubmed                string `json:"pubmed"`
		StudyID               string `json:"study_id"`
		Tier                  string `json:"tier"`
		HallmarkTier          string `json:"hallmark_tier"`
		CNVType               string `json:"cnv_type"`
		GenomeWideScreen      string `json:"genome_wide_screen"`
	} `json:"data"`
	Count int `json:"count"`
	Error string `json:"error,omitempty"`
}

// COSMICCountResponse represents mutation count data from COSMIC
type COSMICCountResponse struct {
	TotalMutations int            `json:"total_mutations"`
	TumorTypes     map[string]int `json:"tumor_types"`
}

// QueryVariant queries COSMIC for somatic variant information
func (c *COSMICClient) QueryVariant(ctx context.Context, variant *domain.StandardizedVariant) (*domain.SomaticData, error) {
	// Rate limiting
	select {
	case <-time.After(c.rateLimit):
		// Proceed after rate limit delay
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Try different query methods based on available variant information
	var response *COSMICVariantResponse
	var err error

	// Method 1: Query by HGVS notation
	if variant.HGVSCoding != "" || variant.HGVSProtein != "" {
		response, err = c.queryByHGVS(ctx, variant)
		if err == nil && len(response.Data) > 0 {
			return c.convertToSomaticData(response), nil
		}
	}

	// Method 2: Query by gene symbol and position
	if variant.GeneSymbol != "" {
		response, err = c.queryByGene(ctx, variant)
		if err == nil && len(response.Data) > 0 {
			return c.convertToSomaticData(response), nil
		}
	}

	// Method 3: Query by genomic coordinates
	if variant.Chromosome != "" && variant.Position > 0 {
		response, err = c.queryByCoordinates(ctx, variant)
		if err == nil && len(response.Data) > 0 {
			return c.convertToSomaticData(response), nil
		}
	}

	// If all methods fail or return no results
	if err != nil {
		return nil, fmt.Errorf("failed to query COSMIC: %w", err)
	}

	// Return empty result if no variants found
	return &domain.SomaticData{}, nil
}

// queryByHGVS queries COSMIC using HGVS notation
func (c *COSMICClient) queryByHGVS(ctx context.Context, variant *domain.StandardizedVariant) (*COSMICVariantResponse, error) {
	queryURL := fmt.Sprintf("%s/api/variants/search", strings.TrimSuffix(c.baseURL, "/"))
	
	params := url.Values{
		"format": {"json"},
		"limit":  {"100"},
	}

	// Use coding HGVS if available, otherwise protein HGVS
	if variant.HGVSCoding != "" {
		params.Set("cds_mutation", variant.HGVSCoding)
	} else if variant.HGVSProtein != "" {
		params.Set("aa_mutation", variant.HGVSProtein)
	}

	return c.executeQuery(ctx, queryURL, params)
}

// queryByGene queries COSMIC using gene symbol
func (c *COSMICClient) queryByGene(ctx context.Context, variant *domain.StandardizedVariant) (*COSMICVariantResponse, error) {
	queryURL := fmt.Sprintf("%s/api/variants/search", strings.TrimSuffix(c.baseURL, "/"))
	
	params := url.Values{
		"gene_name": {variant.GeneSymbol},
		"format":    {"json"},
		"limit":     {"100"},
	}

	// Add additional filters if available
	if variant.HGVSCoding != "" {
		params.Set("cds_mutation", "*"+variant.HGVSCoding+"*")
	}
	if variant.HGVSProtein != "" {
		params.Set("aa_mutation", "*"+variant.HGVSProtein+"*")
	}

	return c.executeQuery(ctx, queryURL, params)
}

// queryByCoordinates queries COSMIC using genomic coordinates
func (c *COSMICClient) queryByCoordinates(ctx context.Context, variant *domain.StandardizedVariant) (*COSMICVariantResponse, error) {
	queryURL := fmt.Sprintf("%s/api/variants/genomic", strings.TrimSuffix(c.baseURL, "/"))
	
	params := url.Values{
		"chromosome": {strings.TrimPrefix(variant.Chromosome, "chr")},
		"position":   {strconv.FormatInt(variant.Position, 10)},
		"format":     {"json"},
		"limit":      {"100"},
	}

	// Add allele information if available
	if variant.Reference != "" {
		params.Set("ref_allele", variant.Reference)
	}
	if variant.Alternative != "" {
		params.Set("alt_allele", variant.Alternative)
	}

	return c.executeQuery(ctx, queryURL, params)
}

// executeQuery executes a query against COSMIC API
func (c *COSMICClient) executeQuery(ctx context.Context, queryURL string, params url.Values) (*COSMICVariantResponse, error) {
	// Add API key to parameters
	if c.apiKey != "" {
		params.Set("api_key", c.apiKey)
	} else {
		return nil, fmt.Errorf("COSMIC API key is required")
	}

	fullURL := fmt.Sprintf("%s?%s", queryURL, params.Encode())
	
	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create COSMIC request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "ACMG-AMP-MCP-Server/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute COSMIC request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("COSMIC API authentication failed: invalid API key")
	}

	if resp.StatusCode == http.StatusNotFound {
		// Return empty result if not found
		return &COSMICVariantResponse{Data: []struct {
			CosmicID              string `json:"cosmic_id"`
			GeneName              string `json:"gene_name"`
			Transcript            string `json:"transcript"`
			CDSMutation           string `json:"cds_mutation"`
			AAMutation            string `json:"aa_mutation"`
			PrimaryTissue         string `json:"primary_tissue"`
			PrimaryHistology      string `json:"primary_histology"`
			SampleCount           int    `json:"sample_count"`
			MutationCount         int    `json:"mutation_count"`
			FathmmmScore          string `json:"fathmm_score"`
			FathmmmPrediction     string `json:"fathmm_prediction"`
			MutationSomaticStatus string `json:"mutation_somatic_status"`
			Pubmed                string `json:"pubmed"`
			StudyID               string `json:"study_id"`
			Tier                  string `json:"tier"`
			HallmarkTier          string `json:"hallmark_tier"`
			CNVType               string `json:"cnv_type"`
			GenomeWideScreen      string `json:"genome_wide_screen"`
		}{}}, nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("COSMIC API returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read COSMIC response: %w", err)
	}

	var response COSMICVariantResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse COSMIC response: %w", err)
	}

	if response.Error != "" {
		return nil, fmt.Errorf("COSMIC API error: %s", response.Error)
	}

	return &response, nil
}

// convertToSomaticData converts COSMIC response to domain SomaticData
func (c *COSMICClient) convertToSomaticData(response *COSMICVariantResponse) *domain.SomaticData {
	if len(response.Data) == 0 {
		return &domain.SomaticData{}
	}

	// Use the first result as primary data
	primary := response.Data[0]
	
	// Aggregate tumor types from all results
	tumorTypeMap := make(map[string]bool)
	totalSamples := 0
	totalMutations := 0
	var cosmicIDs []string

	for _, entry := range response.Data {
		// Collect unique tumor types
		if entry.PrimaryTissue != "" {
			tumorTypeMap[entry.PrimaryTissue] = true
		}
		if entry.PrimaryHistology != "" && entry.PrimaryHistology != entry.PrimaryTissue {
			tumorTypeMap[entry.PrimaryHistology] = true
		}
		
		// Sum sample and mutation counts
		totalSamples += entry.SampleCount
		totalMutations += entry.MutationCount
		
		// Collect COSMIC IDs
		if entry.CosmicID != "" {
			cosmicIDs = append(cosmicIDs, entry.CosmicID)
		}
	}

	// Convert tumor type map to slice
	var tumorTypes []string
	for tumorType := range tumorTypeMap {
		tumorTypes = append(tumorTypes, tumorType)
	}

	// Determine pathogenicity based on FATHMM prediction
	pathogenicity := "Unknown"
	if primary.FathmmmPrediction != "" {
		switch strings.ToLower(primary.FathmmmPrediction) {
		case "pathogenic", "damaging":
			pathogenicity = "Pathogenic"
		case "tolerated", "neutral":
			pathogenicity = "Benign"
		case "possibly_damaging", "possibly damaging":
			pathogenicity = "Possibly Pathogenic"
		}
	}

	// Use primary COSMIC ID or concatenate multiple if available
	cosmicID := primary.CosmicID
	if len(cosmicIDs) > 1 {
		cosmicID = strings.Join(cosmicIDs[:min(3, len(cosmicIDs))], ",") // Limit to first 3
	}

	return &domain.SomaticData{
		CosmicID:      cosmicID,
		TumorTypes:    tumorTypes,
		SampleCount:   totalSamples,
		MutationCount: totalMutations,
		Pathogenicity: pathogenicity,
	}
}

// GetMutationCounts queries COSMIC for mutation count statistics
func (c *COSMICClient) GetMutationCounts(ctx context.Context, geneSymbol string) (*COSMICCountResponse, error) {
	// Rate limiting
	select {
	case <-time.After(c.rateLimit):
		// Proceed after rate limit delay
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	queryURL := fmt.Sprintf("%s/api/genes/%s/mutations/count", strings.TrimSuffix(c.baseURL, "/"), geneSymbol)
	
	params := url.Values{
		"format": {"json"},
	}

	if c.apiKey != "" {
		params.Set("api_key", c.apiKey)
	} else {
		return nil, fmt.Errorf("COSMIC API key is required")
	}

	fullURL := fmt.Sprintf("%s?%s", queryURL, params.Encode())
	
	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create count request: %w", err)
	}

	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute count request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("COSMIC count API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read count response: %w", err)
	}

	var countResponse COSMICCountResponse
	if err := json.Unmarshal(body, &countResponse); err != nil {
		return nil, fmt.Errorf("failed to parse count response: %w", err)
	}

	return &countResponse, nil
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}