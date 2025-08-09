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

	"github.com/acmg-amp-mcp-server/internal/domain"
)

// GnomADClient handles interactions with the gnomAD API
type GnomADClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	rateLimit  time.Duration
}

// NewGnomADClient creates a new gnomAD API client
func NewGnomADClient(config domain.GnomADConfig) *GnomADClient {
	return &GnomADClient{
		baseURL: config.BaseURL,
		apiKey:  config.APIKey,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		rateLimit: time.Second / time.Duration(config.RateLimit),
	}
}

// GnomADVariantResponse represents the JSON response from gnomAD API
type GnomADVariantResponse struct {
	Data struct {
		Variant struct {
			VariantID string `json:"variantId"`
			Genome struct {
				AC  int     `json:"ac"`
				AN  int     `json:"an"`
				AF  float64 `json:"af"`
				Hom int     `json:"hom"`
				Populations []struct {
					ID string  `json:"id"`
					AC int     `json:"ac"`
					AN int     `json:"an"`
					AF float64 `json:"af"`
				} `json:"populations"`
				QualityMetrics struct {
					MeanDP  float64 `json:"mean_dp"`
					MeanGQ  float64 `json:"mean_gq"`
					Pass    bool    `json:"pass"`
				} `json:"qualityMetrics"`
			} `json:"genome"`
			Exome struct {
				AC  int     `json:"ac"`
				AN  int     `json:"an"`
				AF  float64 `json:"af"`
				Hom int     `json:"hom"`
				Populations []struct {
					ID string  `json:"id"`
					AC int     `json:"ac"`
					AN int     `json:"an"`
					AF float64 `json:"af"`
				} `json:"populations"`
				QualityMetrics struct {
					MeanDP  float64 `json:"mean_dp"`
					MeanGQ  float64 `json:"mean_gq"`
					Pass    bool    `json:"pass"`
				} `json:"qualityMetrics"`
			} `json:"exome"`
		} `json:"variant"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// QueryVariant queries gnomAD for population frequency data
func (g *GnomADClient) QueryVariant(ctx context.Context, variant *domain.StandardizedVariant) (*domain.PopulationData, error) {
	// Rate limiting
	select {
	case <-time.After(g.rateLimit):
		// Proceed after rate limit delay
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Build variant identifier for gnomAD
	variantID, err := g.buildVariantID(variant)
	if err != nil {
		return nil, fmt.Errorf("failed to build variant ID for gnomAD: %w", err)
	}

	// Query gnomAD GraphQL API
	response, err := g.queryGraphQL(ctx, variantID)
	if err != nil {
		return nil, fmt.Errorf("failed to query gnomAD: %w", err)
	}

	// Check for API errors
	if len(response.Errors) > 0 {
		return nil, fmt.Errorf("gnomAD API error: %s", response.Errors[0].Message)
	}

	// Convert response to domain PopulationData
	return g.convertToPopulationData(response), nil
}

// buildVariantID constructs a gnomAD variant identifier
func (g *GnomADClient) buildVariantID(variant *domain.StandardizedVariant) (string, error) {
	// gnomAD uses format: chrom-pos-ref-alt
	if variant.Chromosome == "" || variant.Position == 0 || variant.Reference == "" || variant.Alternative == "" {
		return "", fmt.Errorf("insufficient variant information for gnomAD query: need chrom, pos, ref, alt")
	}

	// Normalize chromosome format (remove 'chr' prefix if present)
	chrom := strings.TrimPrefix(variant.Chromosome, "chr")
	
	return fmt.Sprintf("%s-%d-%s-%s", chrom, variant.Position, variant.Reference, variant.Alternative), nil
}

// queryGraphQL executes a GraphQL query against gnomAD API
func (g *GnomADClient) queryGraphQL(ctx context.Context, variantID string) (*GnomADVariantResponse, error) {
	// GraphQL query for variant frequency data
	query := `
	query VariantQuery($variantId: String!) {
		variant(variantId: $variantId, dataset: gnomad_r4) {
			variantId
			genome {
				ac
				an
				af
				hom
				populations {
					id
					ac
					an
					af
				}
				qualityMetrics {
					mean_dp
					mean_gq
					pass
				}
			}
			exome {
				ac
				an
				af
				hom
				populations {
					id
					ac
					an
					af
				}
				qualityMetrics {
					mean_dp
					mean_gq
					pass
				}
			}
		}
	}`

	requestBody := map[string]interface{}{
		"query": query,
		"variables": map[string]interface{}{
			"variantId": variantID,
		},
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal GraphQL request: %w", err)
	}

	// Construct request URL
	queryURL := fmt.Sprintf("%s/graphql", strings.TrimSuffix(g.baseURL, "/"))
	
	req, err := http.NewRequestWithContext(ctx, "POST", queryURL, strings.NewReader(string(jsonBody)))
	if err != nil {
		return nil, fmt.Errorf("failed to create GraphQL request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if g.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+g.apiKey)
	}

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute GraphQL request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gnomAD API returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read GraphQL response: %w", err)
	}

	var response GnomADVariantResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse GraphQL response: %w", err)
	}

	return &response, nil
}

// convertToPopulationData converts gnomAD response to domain PopulationData
func (g *GnomADClient) convertToPopulationData(response *GnomADVariantResponse) *domain.PopulationData {
	variant := response.Data.Variant
	
	// Combine genome and exome data, preferring genome data when available
	var ac, an, hom int
	var af float64
	var qualityMetrics *domain.QualityMetrics
	populationFreqs := make(map[string]float64)
	
	// Use genome data as primary source
	if variant.Genome.AN > 0 {
		ac = variant.Genome.AC
		an = variant.Genome.AN
		af = variant.Genome.AF
		hom = variant.Genome.Hom
		
		// Quality metrics from genome data
		qualityMetrics = &domain.QualityMetrics{
			Coverage:   int(variant.Genome.QualityMetrics.MeanDP),
			Quality:    variant.Genome.QualityMetrics.MeanGQ,
			FilterPass: variant.Genome.QualityMetrics.Pass,
		}
		
		// Population frequencies from genome data
		for _, pop := range variant.Genome.Populations {
			if pop.AF > 0 {
				populationFreqs[pop.ID] = pop.AF
			}
		}
	} else if variant.Exome.AN > 0 {
		// Fall back to exome data if no genome data available
		ac = variant.Exome.AC
		an = variant.Exome.AN
		af = variant.Exome.AF
		hom = variant.Exome.Hom
		
		// Quality metrics from exome data
		qualityMetrics = &domain.QualityMetrics{
			Coverage:   int(variant.Exome.QualityMetrics.MeanDP),
			Quality:    variant.Exome.QualityMetrics.MeanGQ,
			FilterPass: variant.Exome.QualityMetrics.Pass,
		}
		
		// Population frequencies from exome data
		for _, pop := range variant.Exome.Populations {
			if pop.AF > 0 {
				populationFreqs[pop.ID] = pop.AF
			}
		}
	}

	return &domain.PopulationData{
		AlleleFrequency:       af,
		AlleleCount:           ac,
		AlleleNumber:          an,
		PopulationFrequencies: populationFreqs,
		HomozygoteCount:       hom,
		QualityMetrics:        qualityMetrics,
	}
}

// QueryByCoordinates queries gnomAD using genomic coordinates as an alternative method
func (g *GnomADClient) QueryByCoordinates(ctx context.Context, chrom string, pos int, ref, alt string) (*domain.PopulationData, error) {
	// Rate limiting
	select {
	case <-time.After(g.rateLimit):
		// Proceed after rate limit delay
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Normalize chromosome format
	chrom = strings.TrimPrefix(chrom, "chr")
	
	// Construct REST API endpoint for coordinate-based lookup
	queryURL := fmt.Sprintf("%s/variant/%s-%d-%s-%s", 
		strings.TrimSuffix(g.baseURL, "/"), chrom, pos, ref, alt)
	
	params := url.Values{}
	if g.apiKey != "" {
		params.Set("api_key", g.apiKey)
	}
	
	if len(params) > 0 {
		queryURL += "?" + params.Encode()
	}
	
	req, err := http.NewRequestWithContext(ctx, "GET", queryURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create coordinate request: %w", err)
	}

	req.Header.Set("Accept", "application/json")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute coordinate request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// Return empty result if variant not found
		return &domain.PopulationData{}, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gnomAD coordinate API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read coordinate response: %w", err)
	}

	// Parse the JSON response (structure may vary for REST endpoint)
	var jsonResponse map[string]interface{}
	if err := json.Unmarshal(body, &jsonResponse); err != nil {
		return nil, fmt.Errorf("failed to parse coordinate response: %w", err)
	}

	return g.parseCoordinateResponse(jsonResponse), nil
}

// parseCoordinateResponse parses REST API response to PopulationData
func (g *GnomADClient) parseCoordinateResponse(response map[string]interface{}) *domain.PopulationData {
	populationData := &domain.PopulationData{}
	
	// Extract allele frequency data
	if af, ok := response["af"].(float64); ok {
		populationData.AlleleFrequency = af
	}
	
	if ac, ok := response["ac"].(float64); ok {
		populationData.AlleleCount = int(ac)
	}
	
	if an, ok := response["an"].(float64); ok {
		populationData.AlleleNumber = int(an)
	}
	
	if hom, ok := response["hom"].(float64); ok {
		populationData.HomozygoteCount = int(hom)
	}
	
	// Extract population-specific frequencies
	if populations, ok := response["populations"].(map[string]interface{}); ok {
		populationFreqs := make(map[string]float64)
		for pop, data := range populations {
			if popData, ok := data.(map[string]interface{}); ok {
				if popAF, ok := popData["af"].(float64); ok {
					populationFreqs[pop] = popAF
				}
			}
		}
		populationData.PopulationFrequencies = populationFreqs
	}
	
	return populationData
}