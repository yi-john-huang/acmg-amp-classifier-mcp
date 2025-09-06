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

	"github.com/acmg-amp-mcp-server/internal/domain"
)

// PubMedClient handles interactions with NCBI PubMed via E-utilities
type PubMedClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	rateLimit  time.Duration
	email      string // Required by NCBI for large-scale queries
}

// PubMedConfig contains configuration for PubMed client
type PubMedConfig struct {
	BaseURL   string
	APIKey    string
	Email     string
	Timeout   time.Duration
	RateLimit int
}

// NewPubMedClient creates a new PubMed API client
func NewPubMedClient(config PubMedConfig) *PubMedClient {
	if config.BaseURL == "" {
		config.BaseURL = "https://eutils.ncbi.nlm.nih.gov/entrez/eutils/"
	}
	if config.RateLimit == 0 {
		config.RateLimit = 3 // 3 requests per second (with API key)
	}
	
	return &PubMedClient{
		baseURL: config.BaseURL,
		apiKey:  config.APIKey,
		email:   config.Email,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		rateLimit: time.Second / time.Duration(config.RateLimit),
	}
}

// PubMedSearchResponse represents the XML response from PubMed search
type PubMedSearchResponse struct {
	XMLName xml.Name `xml:"eSearchResult"`
	Count   int      `xml:"Count"`
	IDList  struct {
		IDs []string `xml:"Id"`
	} `xml:"IdList"`
	WebEnv    string `xml:"WebEnv"`
	QueryKey  string `xml:"QueryKey"`
}

// PubMedSummaryResponse represents the XML response from PubMed summary
type PubMedSummaryResponse struct {
	XMLName        xml.Name           `xml:"eSummaryResult"`
	DocumentSummary []DocumentSummary `xml:"DocSum"`
}

// DocumentSummary represents a single publication summary from PubMed
type DocumentSummary struct {
	UID   string `xml:"Id"`
	Items []Item `xml:"Item"`
}

// Item represents individual fields in the document summary
type Item struct {
	Name  string `xml:"Name,attr"`
	Type  string `xml:"Type,attr"`
	Value string `xml:",innerxml"`
}

// PubMedAbstractResponse represents the XML response for abstract details
type PubMedAbstractResponse struct {
	XMLName      xml.Name      `xml:"PubmedArticleSet"`
	Articles     []PubmedArticle `xml:"PubmedArticle"`
}

// PubmedArticle represents a complete article from PubMed
type PubmedArticle struct {
	MedlineCitation struct {
		PMID    string `xml:"PMID"`
		Article struct {
			ArticleTitle string `xml:"ArticleTitle"`
			Abstract     struct {
				AbstractText []string `xml:"AbstractText"`
			} `xml:"Abstract"`
			AuthorList struct {
				Authors []struct {
					LastName  string `xml:"LastName"`
					ForeName  string `xml:"ForeName"`
				} `xml:"Author"`
			} `xml:"AuthorList"`
			Journal struct {
				Title         string `xml:"Title"`
				ISOAbbreviation string `xml:"ISOAbbreviation"`
				JournalIssue struct {
					PubDate struct {
						Year  string `xml:"Year"`
						Month string `xml:"Month"`
					} `xml:"PubDate"`
				} `xml:"JournalIssue"`
			} `xml:"Journal"`
		} `xml:"Article"`
	} `xml:"MedlineCitation"`
}

// QueryLiterature searches PubMed for literature related to a variant
func (p *PubMedClient) QueryLiterature(ctx context.Context, variant *domain.StandardizedVariant) (*domain.LiteratureData, error) {
	// Rate limiting
	select {
	case <-time.After(p.rateLimit):
		// Proceed after rate limit delay
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Build search query for the variant
	searchQuery := p.buildSearchQuery(variant)
	if searchQuery == "" {
		return &domain.LiteratureData{}, nil
	}

	// Search PubMed for articles
	pmids, err := p.searchArticles(ctx, searchQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to search PubMed: %w", err)
	}

	if len(pmids) == 0 {
		return &domain.LiteratureData{
			TotalCitations: 0,
			SearchQuery:    searchQuery,
		}, nil
	}

	// Get summaries for the first 20 articles
	maxResults := 20
	if len(pmids) > maxResults {
		pmids = pmids[:maxResults]
	}

	summaries, err := p.getArticleSummaries(ctx, pmids)
	if err != nil {
		return nil, fmt.Errorf("failed to get article summaries: %w", err)
	}

	// Convert to domain objects
	citations := p.convertToCitations(summaries)

	return &domain.LiteratureData{
		TotalCitations:      len(pmids),
		RetrievedCitations:  len(citations),
		Citations:           citations,
		SearchQuery:         searchQuery,
		LastUpdated:         time.Now(),
		HighImpactCitations: p.countHighImpactCitations(citations),
		RecentCitations:     p.countRecentCitations(citations, 5), // Last 5 years
	}, nil
}

// searchArticles performs the initial search and returns PMIDs
func (p *PubMedClient) searchArticles(ctx context.Context, query string) ([]string, error) {
	searchURL := fmt.Sprintf("%sesearch.fcgi", p.baseURL)
	
	params := url.Values{
		"db":       {"pubmed"},
		"term":     {query},
		"retmode":  {"xml"},
		"retmax":   {"100"}, // Get up to 100 results
		"usehistory": {"y"}, // Use history for large result sets
	}
	
	if p.apiKey != "" {
		params.Set("api_key", p.apiKey)
	}
	if p.email != "" {
		params.Set("email", p.email)
	}

	fullURL := fmt.Sprintf("%s?%s", searchURL, params.Encode())
	
	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create search request: %w", err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute search request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("PubMed search returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read search response: %w", err)
	}

	var searchResponse PubMedSearchResponse
	if err := xml.Unmarshal(body, &searchResponse); err != nil {
		return nil, fmt.Errorf("failed to parse search response: %w", err)
	}

	return searchResponse.IDList.IDs, nil
}

// getArticleSummaries retrieves summaries for given PMIDs
func (p *PubMedClient) getArticleSummaries(ctx context.Context, pmids []string) ([]DocumentSummary, error) {
	// Rate limiting for the second request
	select {
	case <-time.After(p.rateLimit):
		// Proceed after rate limit delay
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	summaryURL := fmt.Sprintf("%sesummary.fcgi", p.baseURL)
	
	params := url.Values{
		"db":      {"pubmed"},
		"id":      {strings.Join(pmids, ",")},
		"retmode": {"xml"},
	}
	
	if p.apiKey != "" {
		params.Set("api_key", p.apiKey)
	}

	fullURL := fmt.Sprintf("%s?%s", summaryURL, params.Encode())
	
	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create summary request: %w", err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute summary request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("PubMed summary returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read summary response: %w", err)
	}

	var summaryResponse PubMedSummaryResponse
	if err := xml.Unmarshal(body, &summaryResponse); err != nil {
		return nil, fmt.Errorf("failed to parse summary response: %w", err)
	}

	return summaryResponse.DocumentSummary, nil
}

// buildSearchQuery constructs a search query for PubMed
func (p *PubMedClient) buildSearchQuery(variant *domain.StandardizedVariant) string {
	var terms []string

	// Primary search terms based on variant information
	if variant.GeneSymbol != "" {
		terms = append(terms, fmt.Sprintf("\"%s\"[gene]", variant.GeneSymbol))
	}

	// Add HGVS notation if available
	if variant.HGVSCoding != "" {
		// Extract the actual change part (e.g., "c.185delA" from full HGVS)
		hgvsParts := strings.Split(variant.HGVSCoding, ":")
		if len(hgvsParts) > 1 {
			terms = append(terms, fmt.Sprintf("\"%s\"", hgvsParts[1]))
		} else {
			terms = append(terms, fmt.Sprintf("\"%s\"", variant.HGVSCoding))
		}
	}

	if variant.HGVSProtein != "" {
		// Extract protein change
		proteinParts := strings.Split(variant.HGVSProtein, ":")
		if len(proteinParts) > 1 {
			terms = append(terms, fmt.Sprintf("\"%s\"", proteinParts[1]))
		}
	}

	// Add specific search terms for genetic variants
	if len(terms) > 0 {
		baseQuery := strings.Join(terms, " AND ")
		// Add filters for relevant studies
		filters := []string{
			"(\"genetic variant\"[tiab] OR \"mutation\"[tiab] OR \"pathogenic\"[tiab] OR \"benign\"[tiab])",
			"(\"functional studies\"[tiab] OR \"clinical significance\"[tiab] OR \"disease association\"[tiab])",
			"NOT \"review\"[pt]", // Exclude reviews to focus on original research
		}
		
		return fmt.Sprintf("(%s) AND (%s)", baseQuery, strings.Join(filters, " AND "))
	}

	return ""
}

// convertToCitations converts PubMed summaries to domain citations
func (p *PubMedClient) convertToCitations(summaries []DocumentSummary) []domain.Citation {
	var citations []domain.Citation

	for _, summary := range summaries {
		citation := domain.Citation{
			PMID:        summary.UID,
			Database:    "PubMed",
		}

		// Extract fields from items
		for _, item := range summary.Items {
			switch item.Name {
			case "Title":
				citation.Title = p.cleanXMLValue(item.Value)
			case "AuthorList":
				citation.Authors = p.parseAuthors(item.Value)
			case "Source":
				citation.Journal = p.cleanXMLValue(item.Value)
			case "PubDate":
				if year, err := p.extractYear(item.Value); err == nil {
					citation.Year = year
				}
			case "ISSN":
				citation.ISSN = p.cleanXMLValue(item.Value)
			}
		}

		// Assess relevance based on title and context
		citation.Relevance = p.assessRelevance(citation.Title)
		citation.StudyType = p.determineStudyType(citation.Title)

		citations = append(citations, citation)
	}

	return citations
}

// parseAuthors extracts author names from XML
func (p *PubMedClient) parseAuthors(authorXML string) []string {
	// Simple extraction - in practice, this would need more sophisticated XML parsing
	authors := strings.Split(authorXML, ",")
	var cleanAuthors []string
	for _, author := range authors {
		if trimmed := strings.TrimSpace(author); trimmed != "" {
			cleanAuthors = append(cleanAuthors, trimmed)
		}
	}
	return cleanAuthors
}

// extractYear extracts publication year from date string
func (p *PubMedClient) extractYear(dateStr string) (int, error) {
	// Try different date formats
	dateStr = p.cleanXMLValue(dateStr)
	
	// Look for 4-digit year pattern
	for _, part := range strings.Fields(dateStr) {
		if len(part) == 4 {
			if year, err := strconv.Atoi(part); err == nil && year > 1900 && year <= time.Now().Year() {
				return year, nil
			}
		}
	}
	
	return 0, fmt.Errorf("could not extract year from: %s", dateStr)
}

// cleanXMLValue removes XML tags and cleans up text
func (p *PubMedClient) cleanXMLValue(value string) string {
	// Remove common XML tags
	cleaners := []string{
		"<b>", "</b>",
		"<i>", "</i>",
		"<em>", "</em>",
		"<strong>", "</strong>",
	}
	
	result := value
	for _, cleaner := range cleaners {
		result = strings.ReplaceAll(result, cleaner, "")
	}
	
	return strings.TrimSpace(result)
}

// assessRelevance determines the relevance score of a citation
func (p *PubMedClient) assessRelevance(title string) string {
	title = strings.ToLower(title)
	
	highRelevanceTerms := []string{
		"functional", "pathogenic", "benign", "clinical significance",
		"disease-causing", "deleterious", "damaging", "variant classification",
	}
	
	moderateRelevanceTerms := []string{
		"mutation", "variant", "genetic", "association", "phenotype",
	}

	for _, term := range highRelevanceTerms {
		if strings.Contains(title, term) {
			return "high"
		}
	}
	
	for _, term := range moderateRelevanceTerms {
		if strings.Contains(title, term) {
			return "moderate"
		}
	}
	
	return "low"
}

// determineStudyType categorizes the type of study
func (p *PubMedClient) determineStudyType(title string) string {
	title = strings.ToLower(title)
	
	if strings.Contains(title, "functional") || strings.Contains(title, "in vitro") || 
	   strings.Contains(title, "cell culture") || strings.Contains(title, "assay") {
		return "functional_study"
	}
	
	if strings.Contains(title, "clinical") || strings.Contains(title, "patient") || 
	   strings.Contains(title, "cohort") {
		return "clinical_study"
	}
	
	if strings.Contains(title, "population") || strings.Contains(title, "frequency") || 
	   strings.Contains(title, "epidemiology") {
		return "population_study"
	}
	
	if strings.Contains(title, "case report") || strings.Contains(title, "case series") {
		return "case_report"
	}
	
	return "other"
}

// countHighImpactCitations counts citations from high-impact journals
func (p *PubMedClient) countHighImpactCitations(citations []domain.Citation) int {
	highImpactJournals := []string{
		"nature", "science", "cell", "new england journal of medicine", "lancet",
		"nature genetics", "american journal of human genetics", "genome research",
		"nature medicine", "science translational medicine",
	}
	
	count := 0
	for _, citation := range citations {
		journalLower := strings.ToLower(citation.Journal)
		for _, journal := range highImpactJournals {
			if strings.Contains(journalLower, journal) {
				count++
				break
			}
		}
	}
	
	return count
}

// countRecentCitations counts citations from recent years
func (p *PubMedClient) countRecentCitations(citations []domain.Citation, yearsBack int) int {
	cutoffYear := time.Now().Year() - yearsBack
	count := 0
	
	for _, citation := range citations {
		if citation.Year >= cutoffYear {
			count++
		}
	}
	
	return count
}