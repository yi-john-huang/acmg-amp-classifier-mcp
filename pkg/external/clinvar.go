package external

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/acmg-amp-mcp-server/internal/domain"
)

// ClinVarClient handles interactions with the ClinVar database via NCBI E-utilities
type ClinVarClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	rateLimit  time.Duration
}

// NewClinVarClient creates a new ClinVar API client
func NewClinVarClient(config domain.ClinVarConfig) *ClinVarClient {
	return &ClinVarClient{
		baseURL: config.BaseURL,
		apiKey:  config.APIKey,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		rateLimit: time.Second / time.Duration(config.RateLimit),
	}
}

// ClinVarSearchResponse represents the XML response from ClinVar search
type ClinVarSearchResponse struct {
	XMLName xml.Name `xml:"eSearchResult"`
	IDList  struct {
		IDs []string `xml:"Id"`
	} `xml:"IdList"`
	Count int `xml:"Count"`
}

// ClinVarSummaryResponse represents the XML response from ClinVar summary
type ClinVarSummaryResponse struct {
	XMLName        xml.Name           `xml:"eSummaryResult"`
	DocumentSummary []DocumentSummary `xml:"DocumentSummary"`
}

// DocumentSummary represents a single variant summary from ClinVar
type DocumentSummary struct {
	UID                  string `xml:"uid,attr"`
	Title                string `xml:"title"`
	ClinicalSignificance struct {
		ReviewStatus   string `xml:"ReviewStatus"`
		Description    string `xml:"Description"`
		LastEvaluated  string `xml:"LastEvaluated"`
	} `xml:"clinical_significance"`
	Variation struct {
		Name string `xml:"Name"`
		Type string `xml:"VariationType"`
	} `xml:"variation_set>variation"`
	Conditions []struct {
		Name string `xml:"Name"`
	} `xml:"trait_set>trait"`
	Submitters []struct {
		Name               string `xml:"ClinVarAccession>SubmitterName"`
		Significance       string `xml:"ClinVarAccession>Description"`
		SubmissionDate     string `xml:"ClinVarAccession>DateCreated"`
		ReviewStatus       string `xml:"ClinVarAccession>ReviewStatus"`
	} `xml:"clinical_assertion_list>clinical_assertion"`
}

// QueryVariant queries ClinVar for variant information
func (c *ClinVarClient) QueryVariant(ctx context.Context, variant *domain.StandardizedVariant) (*domain.ClinVarData, error) {
	// Rate limiting
	select {
	case <-time.After(c.rateLimit):
		// Proceed after rate limit delay
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// First, search for the variant to get variant IDs
	variantIDs, err := c.searchVariant(ctx, variant)
	if err != nil {
		return nil, fmt.Errorf("failed to search variant in ClinVar: %w", err)
	}

	if len(variantIDs) == 0 {
		// Return empty result if no variants found
		return &domain.ClinVarData{}, nil
	}

	// Get detailed information for the first matching variant
	clinVarData, err := c.getSummary(ctx, variantIDs[0])
	if err != nil {
		return nil, fmt.Errorf("failed to get ClinVar summary: %w", err)
	}

	return clinVarData, nil
}

// searchVariant searches for variants in ClinVar using E-search
func (c *ClinVarClient) searchVariant(ctx context.Context, variant *domain.StandardizedVariant) ([]string, error) {
	// Build search query based on variant information
	searchTerm := c.buildSearchTerm(variant)
	
	// Construct search URL
	searchURL := fmt.Sprintf("%sesearch.fcgi", c.baseURL)
	params := url.Values{
		"db":       {"clinvar"},
		"term":     {searchTerm},
		"retmode":  {"xml"},
		"retmax":   {"20"}, // Limit to 20 results
	}
	
	if c.apiKey != "" {
		params.Set("api_key", c.apiKey)
	}

	fullURL := fmt.Sprintf("%s?%s", searchURL, params.Encode())
	
	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create search request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute search request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ClinVar search returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read search response: %w", err)
	}

	var searchResponse ClinVarSearchResponse
	if err := xml.Unmarshal(body, &searchResponse); err != nil {
		return nil, fmt.Errorf("failed to parse search response: %w", err)
	}

	return searchResponse.IDList.IDs, nil
}

// getSummary gets detailed variant information using E-summary
func (c *ClinVarClient) getSummary(ctx context.Context, variantID string) (*domain.ClinVarData, error) {
	// Rate limiting for the second request
	select {
	case <-time.After(c.rateLimit):
		// Proceed after rate limit delay
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	summaryURL := fmt.Sprintf("%sesummary.fcgi", c.baseURL)
	params := url.Values{
		"db":       {"clinvar"},
		"id":       {variantID},
		"retmode":  {"xml"},
	}
	
	if c.apiKey != "" {
		params.Set("api_key", c.apiKey)
	}

	fullURL := fmt.Sprintf("%s?%s", summaryURL, params.Encode())
	
	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create summary request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute summary request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ClinVar summary returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read summary response: %w", err)
	}

	var summaryResponse ClinVarSummaryResponse
	if err := xml.Unmarshal(body, &summaryResponse); err != nil {
		return nil, fmt.Errorf("failed to parse summary response: %w", err)
	}

	if len(summaryResponse.DocumentSummary) == 0 {
		return &domain.ClinVarData{}, nil
	}

	// Convert the first document summary to domain ClinVarData
	return c.convertTodomainClinVarData(summaryResponse.DocumentSummary[0])
}

// buildSearchTerm constructs a search term for ClinVar based on variant information
func (c *ClinVarClient) buildSearchTerm(variant *domain.StandardizedVariant) string {
	// Try to use HGVS notation if available
	if variant.HGVSGenomic != "" {
		return fmt.Sprintf("%s[variant name]", variant.HGVSGenomic)
	}
	
	if variant.HGVSCoding != "" {
		return fmt.Sprintf("%s[variant name]", variant.HGVSCoding)
	}
	
	if variant.HGVSProtein != "" {
		return fmt.Sprintf("%s[variant name]", variant.HGVSProtein)
	}
	
	// Fall back to genomic coordinates if no HGVS available
	if variant.Chromosome != "" && variant.Position > 0 {
		return fmt.Sprintf("%s[chr] AND %d[chrpos]", variant.Chromosome, variant.Position)
	}
	
	// Last resort: use gene symbol
	if variant.GeneSymbol != "" {
		return fmt.Sprintf("%s[gene]", variant.GeneSymbol)
	}
	
	return ""
}

// convertTodomainClinVarData converts ClinVar XML response to domain ClinVarData
func (c *ClinVarClient) convertTodomainClinVarData(doc DocumentSummary) (*domain.ClinVarData, error) {
	// Parse last evaluated date
	var lastEvaluated time.Time
	if doc.ClinicalSignificance.LastEvaluated != "" {
		if parsed, err := time.Parse("2006/01/02", doc.ClinicalSignificance.LastEvaluated); err == nil {
			lastEvaluated = parsed
		}
	}
	
	// Extract conditions
	var conditions []string
	for _, condition := range doc.Conditions {
		if condition.Name != "" {
			conditions = append(conditions, condition.Name)
		}
	}
	
	// Extract submissions
	var submissions []domain.ClinVarSubmission
	for _, submitter := range doc.Submitters {
		var submissionDate time.Time
		if submitter.SubmissionDate != "" {
			if parsed, err := time.Parse("2006-01-02", submitter.SubmissionDate); err == nil {
				submissionDate = parsed
			}
		}
		
		if submitter.Name != "" {
			submissions = append(submissions, domain.ClinVarSubmission{
				Submitter:            submitter.Name,
				ClinicalSignificance: submitter.Significance,
				ReviewStatus:         submitter.ReviewStatus,
				SubmissionDate:       submissionDate,
				Condition:            "", // Individual condition not available in this format
			})
		}
	}
	
	return &domain.ClinVarData{
		VariationID:          doc.UID,
		ClinicalSignificance: doc.ClinicalSignificance.Description,
		ReviewStatus:         doc.ClinicalSignificance.ReviewStatus,
		Submissions:          submissions,
		LastEvaluated:        lastEvaluated,
		Conditions:           conditions,
	}, nil
}