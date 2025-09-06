package resources

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
)

// VariantResourceProvider provides variant/{id} resources
type VariantResourceProvider struct {
	logger    *logrus.Logger
	uriParser *URIParser
}

// VariantData represents comprehensive variant information
type VariantData struct {
	VariantID       string                 `json:"variant_id"`
	HGVSNotation    string                 `json:"hgvs_notation"`
	GeneSymbol      string                 `json:"gene_symbol,omitempty"`
	GenomicCoords   GenomicCoordinates     `json:"genomic_coordinates,omitempty"`
	VariantType     string                 `json:"variant_type"`
	Transcripts     []TranscriptInfo       `json:"transcripts,omitempty"`
	ClinicalData    ClinicalVariantData    `json:"clinical_data,omitempty"`
	PopulationData  PopulationVariantData  `json:"population_data,omitempty"`
	FunctionalData  FunctionalVariantData  `json:"functional_data,omitempty"`
	LiteratureRefs  []LiteratureReference  `json:"literature_references,omitempty"`
	LastUpdated     time.Time              `json:"last_updated"`
}

// GenomicCoordinates represents genomic position
type GenomicCoordinates struct {
	Chromosome string `json:"chromosome"`
	Position   int    `json:"position"`
	RefAllele  string `json:"ref_allele"`
	AltAllele  string `json:"alt_allele"`
	Assembly   string `json:"assembly"` // e.g., "GRCh38"
}

// TranscriptInfo represents transcript-specific information
type TranscriptInfo struct {
	TranscriptID   string `json:"transcript_id"`
	RefSeqVersion  string `json:"refseq_version"`
	HGVSc          string `json:"hgvs_c,omitempty"`
	HGVSp          string `json:"hgvs_p,omitempty"`
	ExonNumber     int    `json:"exon_number,omitempty"`
	IntronNumber   int    `json:"intron_number,omitempty"`
	ConsequenceType string `json:"consequence_type"`
}

// ClinicalVariantData contains clinical significance information
type ClinicalVariantData struct {
	ClinVarID           string                 `json:"clinvar_id,omitempty"`
	ClinicalSignificance string                 `json:"clinical_significance"`
	ReviewStatus        string                 `json:"review_status"`
	SubmissionCount     int                    `json:"submission_count"`
	Conditions          []string               `json:"conditions,omitempty"`
	SubmissionSummary   map[string]int         `json:"submission_summary,omitempty"`
	LastEvaluated       string                 `json:"last_evaluated,omitempty"`
	Guidelines          []string               `json:"guidelines,omitempty"`
	ClinicalTrials      []ClinicalTrialInfo    `json:"clinical_trials,omitempty"`
}

// PopulationVariantData contains population frequency information
type PopulationVariantData struct {
	GnomADFrequency    *AlleleFrequency            `json:"gnomad_frequency,omitempty"`
	PopulationFreqs    map[string]*AlleleFrequency `json:"population_frequencies,omitempty"`
	AlleleCount        int                         `json:"allele_count"`
	AlleleNumber       int                         `json:"allele_number"`
	HomozygoteCount    int                         `json:"homozygote_count"`
	FrequencyFilter    string                      `json:"frequency_filter,omitempty"`
}

// AlleleFrequency represents frequency data for a specific population
type AlleleFrequency struct {
	Frequency       float64 `json:"frequency"`
	AlleleCount     int     `json:"allele_count"`
	AlleleNumber    int     `json:"allele_number"`
	HomozygoteCount int     `json:"homozygote_count"`
	Population      string  `json:"population"`
}

// FunctionalVariantData contains functional prediction information
type FunctionalVariantData struct {
	SIFTScore         *float64               `json:"sift_score,omitempty"`
	SIFTPrediction    string                 `json:"sift_prediction,omitempty"`
	PolyPhenScore     *float64               `json:"polyphen_score,omitempty"`
	PolyPhenPrediction string                `json:"polyphen_prediction,omitempty"`
	CADDScore         *float64               `json:"cadd_score,omitempty"`
	ConservationScore *float64               `json:"conservation_score,omitempty"`
	SplicingPredictions []SplicingPrediction  `json:"splicing_predictions,omitempty"`
	ProteinEffect     ProteinEffectData      `json:"protein_effect,omitempty"`
}


// ProteinEffectData contains protein-level functional information
type ProteinEffectData struct {
	ProteinDomain    string   `json:"protein_domain,omitempty"`
	FunctionalDomain string   `json:"functional_domain,omitempty"`
	StructuralInfo   string   `json:"structural_info,omitempty"`
	KnownEffects     []string `json:"known_effects,omitempty"`
}

// LiteratureReference represents a literature citation
type LiteratureReference struct {
	PMID      string   `json:"pmid"`
	Title     string   `json:"title"`
	Authors   []string `json:"authors"`
	Journal   string   `json:"journal"`
	Year      int      `json:"year"`
	DOI       string   `json:"doi,omitempty"`
	Relevance string   `json:"relevance"` // "high", "medium", "low"
	Summary   string   `json:"summary,omitempty"`
}

// ClinicalTrialInfo represents clinical trial information
type ClinicalTrialInfo struct {
	TrialID     string   `json:"trial_id"`
	Title       string   `json:"title"`
	Status      string   `json:"status"`
	Phase       string   `json:"phase,omitempty"`
	Conditions  []string `json:"conditions"`
	Sponsor     string   `json:"sponsor"`
	StartDate   string   `json:"start_date"`
	CompletionDate string `json:"completion_date,omitempty"`
}

// NewVariantResourceProvider creates a new variant resource provider
func NewVariantResourceProvider(logger *logrus.Logger) *VariantResourceProvider {
	provider := &VariantResourceProvider{
		logger:    logger,
		uriParser: NewURIParser(),
	}
	
	// Add URI patterns for variant resources
	provider.uriParser.AddPattern("variant_by_id", `^/variant/(?P<id>[^/]+)$`)
	provider.uriParser.AddPattern("variant_by_hgvs", `^/variant/hgvs/(?P<hgvs>.+)$`)
	provider.uriParser.AddPattern("variant_transcripts", `^/variant/(?P<id>[^/]+)/transcripts$`)
	provider.uriParser.AddPattern("variant_clinical", `^/variant/(?P<id>[^/]+)/clinical$`)
	provider.uriParser.AddPattern("variant_population", `^/variant/(?P<id>[^/]+)/population$`)
	provider.uriParser.AddPattern("variant_functional", `^/variant/(?P<id>[^/]+)/functional$`)
	provider.uriParser.AddPattern("variant_literature", `^/variant/(?P<id>[^/]+)/literature$`)
	
	return provider
}

// GetResource retrieves a variant resource
func (vp *VariantResourceProvider) GetResource(ctx context.Context, uri string) (*ResourceContent, error) {
	vp.logger.WithField("uri", uri).Debug("Getting variant resource")
	
	patternName, params, err := vp.uriParser.ParseURI(uri)
	if err != nil {
		return nil, fmt.Errorf("failed to parse variant URI: %w", err)
	}
	
	switch patternName {
	case "variant_by_id":
		return vp.getVariantByID(ctx, params["id"])
	case "variant_by_hgvs":
		return vp.getVariantByHGVS(ctx, params["hgvs"])
	case "variant_transcripts":
		return vp.getVariantTranscripts(ctx, params["id"])
	case "variant_clinical":
		return vp.getVariantClinical(ctx, params["id"])
	case "variant_population":
		return vp.getVariantPopulation(ctx, params["id"])
	case "variant_functional":
		return vp.getVariantFunctional(ctx, params["id"])
	case "variant_literature":
		return vp.getVariantLiterature(ctx, params["id"])
	default:
		return nil, fmt.Errorf("unsupported variant resource pattern: %s", patternName)
	}
}

// ListResources lists available variant resources
func (vp *VariantResourceProvider) ListResources(ctx context.Context, cursor string) (*ResourceList, error) {
	// In a real implementation, this would query a database
	// For now, return a mock list of available variant resource patterns
	resources := []ResourceInfo{
		{
			URI:         "/variant/{id}",
			Name:        "Variant Information",
			Description: "Complete variant information including clinical and functional data",
			MimeType:    "application/json",
			LastModified: time.Now().Add(-1 * time.Hour),
			Tags:        []string{"variant", "comprehensive"},
		},
		{
			URI:         "/variant/hgvs/{hgvs}",
			Name:        "Variant by HGVS",
			Description: "Variant lookup by HGVS notation",
			MimeType:    "application/json",
			LastModified: time.Now().Add(-1 * time.Hour),
			Tags:        []string{"variant", "hgvs"},
		},
		{
			URI:         "/variant/{id}/transcripts",
			Name:        "Variant Transcripts",
			Description: "Transcript-specific information for a variant",
			MimeType:    "application/json",
			LastModified: time.Now().Add(-30 * time.Minute),
			Tags:        []string{"variant", "transcripts"},
		},
		{
			URI:         "/variant/{id}/clinical",
			Name:        "Clinical Data",
			Description: "Clinical significance and ClinVar information",
			MimeType:    "application/json",
			LastModified: time.Now().Add(-2 * time.Hour),
			Tags:        []string{"variant", "clinical", "clinvar"},
		},
		{
			URI:         "/variant/{id}/population",
			Name:        "Population Data",
			Description: "Population frequency data from gnomAD and other databases",
			MimeType:    "application/json",
			LastModified: time.Now().Add(-1 * time.Hour),
			Tags:        []string{"variant", "population", "frequency"},
		},
		{
			URI:         "/variant/{id}/functional",
			Name:        "Functional Predictions",
			Description: "Functional impact predictions and conservation scores",
			MimeType:    "application/json",
			LastModified: time.Now().Add(-3 * time.Hour),
			Tags:        []string{"variant", "functional", "predictions"},
		},
		{
			URI:         "/variant/{id}/literature",
			Name:        "Literature References",
			Description: "Relevant literature and publications",
			MimeType:    "application/json",
			LastModified: time.Now().Add(-6 * time.Hour),
			Tags:        []string{"variant", "literature", "pubmed"},
		},
	}
	
	return &ResourceList{
		Resources: resources,
		Total:     len(resources),
	}, nil
}

// GetResourceInfo returns metadata about a variant resource
func (vp *VariantResourceProvider) GetResourceInfo(ctx context.Context, uri string) (*ResourceInfo, error) {
	patternName, params, err := vp.uriParser.ParseURI(uri)
	if err != nil {
		return nil, fmt.Errorf("failed to parse variant URI: %w", err)
	}
	
	var name, description string
	tags := []string{"variant"}
	
	switch patternName {
	case "variant_by_id":
		name = fmt.Sprintf("Variant %s", params["id"])
		description = "Complete variant information with clinical and functional data"
		tags = append(tags, "comprehensive")
	case "variant_by_hgvs":
		name = fmt.Sprintf("Variant %s", params["hgvs"])
		description = "Variant information looked up by HGVS notation"
		tags = append(tags, "hgvs")
	case "variant_transcripts":
		name = fmt.Sprintf("Transcripts for %s", params["id"])
		description = "Transcript-specific annotations and consequences"
		tags = append(tags, "transcripts")
	case "variant_clinical":
		name = fmt.Sprintf("Clinical data for %s", params["id"])
		description = "Clinical significance and ClinVar submissions"
		tags = append(tags, "clinical", "clinvar")
	case "variant_population":
		name = fmt.Sprintf("Population data for %s", params["id"])
		description = "Population frequencies and allele counts"
		tags = append(tags, "population", "frequency")
	case "variant_functional":
		name = fmt.Sprintf("Functional predictions for %s", params["id"])
		description = "Functional impact and conservation predictions"
		tags = append(tags, "functional", "predictions")
	case "variant_literature":
		name = fmt.Sprintf("Literature for %s", params["id"])
		description = "Relevant publications and references"
		tags = append(tags, "literature", "pubmed")
	default:
		return nil, fmt.Errorf("unsupported variant resource pattern: %s", patternName)
	}
	
	return &ResourceInfo{
		URI:         uri,
		Name:        name,
		Description: description,
		MimeType:    "application/json",
		LastModified: time.Now().Add(-1 * time.Hour), // Mock timestamp
		Tags:        tags,
		Metadata: map[string]interface{}{
			"provider": "variant",
			"pattern":  patternName,
		},
	}, nil
}

// SupportsURI checks if this provider supports the given URI
func (vp *VariantResourceProvider) SupportsURI(uri string) bool {
	_, _, err := vp.uriParser.ParseURI(uri)
	return err == nil
}

// GetProviderInfo returns information about this provider
func (vp *VariantResourceProvider) GetProviderInfo() ProviderInfo {
	return ProviderInfo{
		Name:        "Variant Resource Provider",
		Description: "Provides comprehensive variant information including clinical, functional, and population data",
		Version:     "1.0.0",
		URIPatterns: []string{
			"/variant/{id}",
			"/variant/hgvs/{hgvs}",
			"/variant/{id}/transcripts",
			"/variant/{id}/clinical",
			"/variant/{id}/population",
			"/variant/{id}/functional",
			"/variant/{id}/literature",
		},
	}
}

// Implementation methods for different resource types

func (vp *VariantResourceProvider) getVariantByID(ctx context.Context, id string) (*ResourceContent, error) {
	// In a real implementation, this would query a database
	// For now, return mock data based on the ID
	
	variant := vp.generateMockVariantData(id)
	
	return &ResourceContent{
		URI:         fmt.Sprintf("/variant/%s", id),
		Name:        fmt.Sprintf("Variant %s", id),
		Description: fmt.Sprintf("Complete information for variant %s", id),
		MimeType:    "application/json",
		Content:     variant,
		LastModified: time.Now().Add(-1 * time.Hour),
		ETag:        fmt.Sprintf("variant-%s-%d", id, time.Now().Unix()),
		Metadata: map[string]interface{}{
			"provider":     "variant",
			"variant_id":   id,
			"content_type": "complete_variant",
		},
	}, nil
}

func (vp *VariantResourceProvider) getVariantByHGVS(ctx context.Context, hgvs string) (*ResourceContent, error) {
	// Convert HGVS to variant ID (mock implementation)
	variantID := vp.hgvsToVariantID(hgvs)
	
	variant := vp.generateMockVariantData(variantID)
	variant.HGVSNotation = hgvs
	
	return &ResourceContent{
		URI:         fmt.Sprintf("/variant/hgvs/%s", hgvs),
		Name:        fmt.Sprintf("Variant %s", hgvs),
		Description: fmt.Sprintf("Variant information for %s", hgvs),
		MimeType:    "application/json",
		Content:     variant,
		LastModified: time.Now().Add(-1 * time.Hour),
		ETag:        fmt.Sprintf("variant-hgvs-%s-%d", hgvs, time.Now().Unix()),
		Metadata: map[string]interface{}{
			"provider":      "variant",
			"hgvs_notation": hgvs,
			"variant_id":    variantID,
			"content_type":  "complete_variant",
		},
	}, nil
}

func (vp *VariantResourceProvider) getVariantTranscripts(ctx context.Context, id string) (*ResourceContent, error) {
	transcripts := vp.generateMockTranscripts(id)
	
	return &ResourceContent{
		URI:         fmt.Sprintf("/variant/%s/transcripts", id),
		Name:        fmt.Sprintf("Transcripts for %s", id),
		Description: "Transcript-specific annotations and consequences",
		MimeType:    "application/json",
		Content:     map[string]interface{}{"transcripts": transcripts},
		LastModified: time.Now().Add(-30 * time.Minute),
		ETag:        fmt.Sprintf("transcripts-%s-%d", id, time.Now().Unix()),
		Metadata: map[string]interface{}{
			"provider":     "variant",
			"variant_id":   id,
			"content_type": "transcripts",
		},
	}, nil
}

func (vp *VariantResourceProvider) getVariantClinical(ctx context.Context, id string) (*ResourceContent, error) {
	clinical := vp.generateMockClinicalData(id)
	
	return &ResourceContent{
		URI:         fmt.Sprintf("/variant/%s/clinical", id),
		Name:        fmt.Sprintf("Clinical data for %s", id),
		Description: "Clinical significance and ClinVar information",
		MimeType:    "application/json",
		Content:     clinical,
		LastModified: time.Now().Add(-2 * time.Hour),
		ETag:        fmt.Sprintf("clinical-%s-%d", id, time.Now().Unix()),
		Metadata: map[string]interface{}{
			"provider":     "variant",
			"variant_id":   id,
			"content_type": "clinical",
		},
	}, nil
}

func (vp *VariantResourceProvider) getVariantPopulation(ctx context.Context, id string) (*ResourceContent, error) {
	population := vp.generateMockPopulationData(id)
	
	return &ResourceContent{
		URI:         fmt.Sprintf("/variant/%s/population", id),
		Name:        fmt.Sprintf("Population data for %s", id),
		Description: "Population frequencies and allele counts",
		MimeType:    "application/json",
		Content:     population,
		LastModified: time.Now().Add(-1 * time.Hour),
		ETag:        fmt.Sprintf("population-%s-%d", id, time.Now().Unix()),
		Metadata: map[string]interface{}{
			"provider":     "variant",
			"variant_id":   id,
			"content_type": "population",
		},
	}, nil
}

func (vp *VariantResourceProvider) getVariantFunctional(ctx context.Context, id string) (*ResourceContent, error) {
	functional := vp.generateMockFunctionalData(id)
	
	return &ResourceContent{
		URI:         fmt.Sprintf("/variant/%s/functional", id),
		Name:        fmt.Sprintf("Functional predictions for %s", id),
		Description: "Functional impact and conservation predictions",
		MimeType:    "application/json",
		Content:     functional,
		LastModified: time.Now().Add(-3 * time.Hour),
		ETag:        fmt.Sprintf("functional-%s-%d", id, time.Now().Unix()),
		Metadata: map[string]interface{}{
			"provider":     "variant",
			"variant_id":   id,
			"content_type": "functional",
		},
	}, nil
}

func (vp *VariantResourceProvider) getVariantLiterature(ctx context.Context, id string) (*ResourceContent, error) {
	literature := vp.generateMockLiterature(id)
	
	return &ResourceContent{
		URI:         fmt.Sprintf("/variant/%s/literature", id),
		Name:        fmt.Sprintf("Literature for %s", id),
		Description: "Relevant publications and references",
		MimeType:    "application/json",
		Content:     map[string]interface{}{"literature": literature},
		LastModified: time.Now().Add(-6 * time.Hour),
		ETag:        fmt.Sprintf("literature-%s-%d", id, time.Now().Unix()),
		Metadata: map[string]interface{}{
			"provider":     "variant",
			"variant_id":   id,
			"content_type": "literature",
		},
	}, nil
}

// Mock data generation methods (in production, these would query real databases)

func (vp *VariantResourceProvider) generateMockVariantData(id string) *VariantData {
	// Generate deterministic mock data based on ID
	hash := vp.hashString(id)
	
	return &VariantData{
		VariantID:    id,
		HGVSNotation: fmt.Sprintf("NM_%06d.3:c.%dA>G", hash%1000000, hash%3000),
		GeneSymbol:   vp.generateGeneSymbol(hash),
		VariantType:  "SNV",
		GenomicCoords: GenomicCoordinates{
			Chromosome: fmt.Sprintf("%d", (hash%22)+1),
			Position:   hash%100000000,
			RefAllele:  "A",
			AltAllele:  "G",
			Assembly:   "GRCh38",
		},
		Transcripts:    vp.generateMockTranscripts(id),
		ClinicalData:   vp.generateMockClinicalData(id),
		PopulationData: vp.generateMockPopulationData(id),
		FunctionalData: vp.generateMockFunctionalData(id),
		LiteratureRefs: vp.generateMockLiterature(id),
		LastUpdated:    time.Now().Add(-24 * time.Hour),
	}
}

func (vp *VariantResourceProvider) generateMockTranscripts(id string) []TranscriptInfo {
	hash := vp.hashString(id)
	
	return []TranscriptInfo{
		{
			TranscriptID:    fmt.Sprintf("NM_%06d.3", hash%1000000),
			RefSeqVersion:   "3",
			HGVSc:           fmt.Sprintf("c.%dA>G", hash%3000),
			HGVSp:           fmt.Sprintf("p.Lys%dGlu", (hash%500)+1),
			ExonNumber:      (hash % 20) + 1,
			ConsequenceType: "missense_variant",
		},
	}
}

func (vp *VariantResourceProvider) generateMockClinicalData(id string) ClinicalVariantData {
	hash := vp.hashString(id)
	
	significance := []string{"Pathogenic", "Likely pathogenic", "VUS", "Likely benign", "Benign"}
	
	return ClinicalVariantData{
		ClinVarID:           fmt.Sprintf("RCV%09d", hash%1000000000),
		ClinicalSignificance: significance[hash%len(significance)],
		ReviewStatus:        "criteria provided, single submitter",
		SubmissionCount:     (hash % 10) + 1,
		Conditions:          []string{fmt.Sprintf("Disease_%d", hash%1000)},
		LastEvaluated:       "2023-01-01",
		Guidelines:          []string{"ACMG/AMP 2015"},
	}
}

func (vp *VariantResourceProvider) generateMockPopulationData(id string) PopulationVariantData {
	hash := vp.hashString(id)
	frequency := float64(hash%10000) / 1000000.0 // 0 to 0.01
	
	return PopulationVariantData{
		GnomADFrequency: &AlleleFrequency{
			Frequency:       frequency,
			AlleleCount:     hash % 1000,
			AlleleNumber:    250000 + (hash % 50000),
			HomozygoteCount: hash % 10,
			Population:      "gnomAD",
		},
		AlleleCount:     hash % 1000,
		AlleleNumber:    250000 + (hash % 50000),
		HomozygoteCount: hash % 10,
	}
}

func (vp *VariantResourceProvider) generateMockFunctionalData(id string) FunctionalVariantData {
	hash := vp.hashString(id)
	
	siftScore := float64(hash%1000) / 1000.0
	polyphenScore := float64(hash%1000) / 1000.0
	caddScore := float64(hash%40) + 1.0
	
	return FunctionalVariantData{
		SIFTScore:         &siftScore,
		SIFTPrediction:    vp.getSIFTPrediction(siftScore),
		PolyPhenScore:     &polyphenScore,
		PolyPhenPrediction: vp.getPolyPhenPrediction(polyphenScore),
		CADDScore:         &caddScore,
		ConservationScore: &siftScore,
		ProteinEffect: ProteinEffectData{
			ProteinDomain:    "Protein kinase domain",
			FunctionalDomain: "ATP binding site",
			KnownEffects:     []string{"Affects protein stability"},
		},
	}
}

func (vp *VariantResourceProvider) generateMockLiterature(id string) []LiteratureReference {
	hash := vp.hashString(id)
	
	return []LiteratureReference{
		{
			PMID:      fmt.Sprintf("%d", 20000000+(hash%10000000)),
			Title:     fmt.Sprintf("Functional analysis of variant %s", id),
			Authors:   []string{"Smith J", "Doe A", "Johnson M"},
			Journal:   "Nature Genetics",
			Year:      2020 + (hash % 4),
			DOI:       fmt.Sprintf("10.1038/ng.%d", hash%10000),
			Relevance: []string{"high", "medium", "low"}[hash%3],
			Summary:   "This study demonstrates the functional impact of the variant.",
		},
	}
}

// Utility methods

func (vp *VariantResourceProvider) hgvsToVariantID(hgvs string) string {
	// Simple conversion for demo - in reality this would be a database lookup
	hash := vp.hashString(hgvs)
	return fmt.Sprintf("VAR_%09d", hash%1000000000)
}

func (vp *VariantResourceProvider) hashString(s string) int {
	hash := 0
	for _, c := range s {
		hash = hash*31 + int(c)
	}
	if hash < 0 {
		hash = -hash
	}
	return hash
}

func (vp *VariantResourceProvider) generateGeneSymbol(hash int) string {
	genes := []string{"BRCA1", "BRCA2", "TP53", "CFTR", "APOE", "LDLR", "MYH7", "SCN5A", "RYR2", "PKD1"}
	return genes[hash%len(genes)]
}

func (vp *VariantResourceProvider) getSIFTPrediction(score float64) string {
	if score < 0.05 {
		return "deleterious"
	}
	return "tolerated"
}

func (vp *VariantResourceProvider) getPolyPhenPrediction(score float64) string {
	if score > 0.9 {
		return "probably_damaging"
	} else if score > 0.5 {
		return "possibly_damaging"
	}
	return "benign"
}