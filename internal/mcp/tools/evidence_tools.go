package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/acmg-amp-mcp-server/internal/mcp/protocol"
)

// QueryEvidenceTool implements the query_evidence MCP tool for comprehensive evidence gathering
type QueryEvidenceTool struct {
	logger *logrus.Logger
	cache  *EvidenceCache
}

// QueryEvidenceParams defines parameters for the query_evidence tool
type QueryEvidenceParams struct {
	VariantID       string   `json:"variant_id,omitempty"`
	HGVSNotation    string   `json:"hgvs_notation" validate:"required"`
	GeneSymbol      string   `json:"gene_symbol,omitempty"`
	GenomicPosition string   `json:"genomic_position,omitempty"`
	Databases       []string `json:"databases,omitempty"` // specific databases to query
	IncludeRaw      bool     `json:"include_raw,omitempty"`
	MaxAge          string   `json:"max_age,omitempty"` // cache max age (e.g., "24h")
}

// QueryEvidenceResult defines the comprehensive evidence result structure
type QueryEvidenceResult struct {
	VariantID           string                 `json:"variant_id"`
	HGVSNotation        string                 `json:"hgvs_notation"`
	QueryTimestamp      string                 `json:"query_timestamp"`
	DatabaseResults     map[string]interface{} `json:"database_results"`
	AggregatedEvidence  AggregatedEvidence     `json:"aggregated_evidence"`
	QualityScores       EvidenceQualityScores  `json:"quality_scores"`
	RecommendedActions  []string               `json:"recommended_actions"`
	DataFreshness       map[string]string      `json:"data_freshness"`
}

// AggregatedEvidence contains summarized evidence across databases
type AggregatedEvidence struct {
	PopulationFrequency PopulationFrequencyData `json:"population_frequency"`
	ClinicalEvidence    ClinicalEvidenceData    `json:"clinical_evidence"`
	FunctionalEvidence  FunctionalEvidenceData  `json:"functional_evidence"`
	ComputationalData   ComputationalData       `json:"computational_data"`
	LiteratureEvidence  LiteratureEvidenceData  `json:"literature_evidence"`
}

// PopulationFrequencyData contains population frequency information
type PopulationFrequencyData struct {
	MaxFrequency        float64            `json:"max_frequency"`
	PopulationFreqs     map[string]float64 `json:"population_frequencies"`
	AlleleCount         int                `json:"allele_count"`
	AlleleNumber        int                `json:"allele_number"`
	HomozygoteCount     int                `json:"homozygote_count"`
	QualityMetrics      map[string]float64 `json:"quality_metrics"`
	FrequencyAssessment string             `json:"frequency_assessment"`
}

// ClinicalEvidenceData contains clinical significance information
type ClinicalEvidenceData struct {
	ClinVarEntries      []ClinVarEntry `json:"clinvar_entries"`
	OverallSignificance string         `json:"overall_significance"`
	ConflictingInterpretations bool    `json:"conflicting_interpretations"`
	ReviewStatus        string         `json:"review_status"`
	SubmissionSummary   map[string]int `json:"submission_summary"`
}

// ClinVarEntry represents a single ClinVar entry
type ClinVarEntry struct {
	AccessionID         string            `json:"accession_id"`
	ClinicalSignificance string           `json:"clinical_significance"`
	ReviewStatus        string            `json:"review_status"`
	Submitter           string            `json:"submitter"`
	SubmissionDate      string            `json:"submission_date"`
	Conditions          []string          `json:"conditions"`
	Assertions          map[string]string `json:"assertions"`
}

// FunctionalEvidenceData contains functional study information
type FunctionalEvidenceData struct {
	HasFunctionalData   bool                   `json:"has_functional_data"`
	FunctionalStudies   []FunctionalStudy      `json:"functional_studies"`
	FunctionalPrediction string                `json:"functional_prediction"`
	StudySummary        map[string]interface{} `json:"study_summary"`
}

// FunctionalStudy represents a single functional study
type FunctionalStudy struct {
	StudyType    string `json:"study_type"`
	Result       string `json:"result"`
	Description  string `json:"description"`
	Reference    string `json:"reference"`
	Reliability  string `json:"reliability"`
}

// ComputationalData contains in silico prediction results
type ComputationalData struct {
	SIFTScore           float64            `json:"sift_score,omitempty"`
	SIFTPrediction      string             `json:"sift_prediction,omitempty"`
	PolyPhenScore       float64            `json:"polyphen_score,omitempty"`
	PolyPhenPrediction  string             `json:"polyphen_prediction,omitempty"`
	CADDScore           float64            `json:"cadd_score,omitempty"`
	REVEL               float64            `json:"revel_score,omitempty"`
	AlphaMissense       float64            `json:"alphamissense_score,omitempty"`
	ConservationScores  map[string]float64 `json:"conservation_scores"`
	SpliceScores        map[string]float64 `json:"splice_scores"`
	ConsensusScore      float64            `json:"consensus_score"`
	ConsensusPrediction string             `json:"consensus_prediction"`
}

// LiteratureEvidenceData contains literature and publication information
type LiteratureEvidenceData struct {
	PubMedCitations     []PubMedCitation `json:"pubmed_citations"`
	TotalCitations      int              `json:"total_citations"`
	RecentCitations     int              `json:"recent_citations"`
	HighImpactCitations int              `json:"high_impact_citations"`
}

// PubMedCitation represents a literature citation
type PubMedCitation struct {
	PMID         string   `json:"pmid"`
	Title        string   `json:"title"`
	Authors      []string `json:"authors"`
	Journal      string   `json:"journal"`
	Year         int      `json:"year"`
	ImpactFactor float64  `json:"impact_factor,omitempty"`
	Relevance    string   `json:"relevance"`
}

// EvidenceQualityScores contains quality assessment metrics
type EvidenceQualityScores struct {
	OverallQuality      string             `json:"overall_quality"`
	DataCompleteness    float64            `json:"data_completeness"`
	SourceReliability   map[string]float64 `json:"source_reliability"`
	ConflictScore       float64            `json:"conflict_score"`
	FreshnessScore      float64            `json:"freshness_score"`
	EvidenceStrength    map[string]string  `json:"evidence_strength"`
}

// NewQueryEvidenceTool creates a new query_evidence tool
func NewQueryEvidenceTool(logger *logrus.Logger) *QueryEvidenceTool {
	return &QueryEvidenceTool{
		logger: logger,
		cache:  NewEvidenceCache(logger),
	}
}

// HandleTool implements the ToolHandler interface for query_evidence
func (t *QueryEvidenceTool) HandleTool(ctx context.Context, req *protocol.JSONRPC2Request) *protocol.JSONRPC2Response {
	startTime := time.Now()
	t.logger.WithField("tool", "query_evidence").Info("Processing evidence query request")

	// Parse and validate parameters
	var params QueryEvidenceParams
	if err := t.parseAndValidateParams(req.Params, &params); err != nil {
		return &protocol.JSONRPC2Response{
			Error: &protocol.RPCError{
				Code:    protocol.InvalidParams,
				Message: "Invalid parameters",
				Data:    err.Error(),
			},
		}
	}

	// Check cache first
	if cacheResult := t.checkCache(&params); cacheResult != nil {
		t.logger.WithField("hgvs", params.HGVSNotation).Debug("Returning cached evidence result")
		return &protocol.JSONRPC2Response{
			Result: map[string]interface{}{
				"evidence": cacheResult,
			},
		}
	}

	// Gather evidence from multiple databases
	result, err := t.gatherEvidence(ctx, &params)
	if err != nil {
		return &protocol.JSONRPC2Response{
			Error: &protocol.RPCError{
				Code:    protocol.MCPToolError,
				Message: "Evidence gathering failed",
				Data:    err.Error(),
			},
		}
	}

	// Calculate processing time
	result.QueryTimestamp = time.Now().Format(time.RFC3339)

	// Cache the result
	t.cacheResult(&params, result)

	t.logger.WithFields(logrus.Fields{
		"hgvs":            params.HGVSNotation,
		"databases":       len(result.DatabaseResults),
		"processing_time": time.Since(startTime).String(),
	}).Info("Evidence gathering completed")

	return &protocol.JSONRPC2Response{
		Result: map[string]interface{}{
			"evidence": result,
		},
	}
}

// GetToolInfo returns tool metadata
func (t *QueryEvidenceTool) GetToolInfo() protocol.ToolInfo {
	return protocol.ToolInfo{
		Name:        "query_evidence",
		Description: "Query multiple genetic databases to gather comprehensive evidence for variant interpretation",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"hgvs_notation": map[string]interface{}{
					"type":        "string",
					"description": "HGVS notation of the variant",
					"pattern":     "^(NC_|NM_|NP_|NG_|NR_|XM_|XR_).*",
				},
				"gene_symbol": map[string]interface{}{
					"type":        "string",
					"description": "HGNC gene symbol",
				},
				"genomic_position": map[string]interface{}{
					"type":        "string",
					"description": "Genomic position (chr:pos format)",
					"pattern":     "^(chr)?[0-9XYxy]+:[0-9]+$",
				},
				"databases": map[string]interface{}{
					"type":        "array",
					"description": "Specific databases to query",
					"items": map[string]interface{}{
						"type": "string",
						"enum": []string{"clinvar", "gnomad", "cosmic", "lovd", "hgmd", "pubmed"},
					},
				},
				"include_raw": map[string]interface{}{
					"type":        "boolean",
					"description": "Include raw database responses",
					"default":     false,
				},
				"max_age": map[string]interface{}{
					"type":        "string",
					"description": "Maximum age for cached results (e.g., '24h', '7d')",
					"default":     "24h",
				},
			},
			"required": []string{"hgvs_notation"},
		},
	}
}

// ValidateParams validates tool parameters
func (t *QueryEvidenceTool) ValidateParams(params interface{}) error {
	var queryParams QueryEvidenceParams
	return t.parseAndValidateParams(params, &queryParams)
}

// parseAndValidateParams parses and validates input parameters
func (t *QueryEvidenceTool) parseAndValidateParams(params interface{}, target *QueryEvidenceParams) error {
	if params == nil {
		return fmt.Errorf("missing required parameters")
	}

	paramsBytes, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("failed to marshal parameters: %w", err)
	}

	if err := json.Unmarshal(paramsBytes, target); err != nil {
		return fmt.Errorf("failed to parse parameters: %w", err)
	}

	if target.HGVSNotation == "" {
		return fmt.Errorf("hgvs_notation is required")
	}

	// Set default databases if none specified
	if len(target.Databases) == 0 {
		target.Databases = []string{"clinvar", "gnomad", "cosmic"}
	}

	// Set default max age
	if target.MaxAge == "" {
		target.MaxAge = "24h"
	}

	return nil
}

// checkCache checks for cached results
func (t *QueryEvidenceTool) checkCache(params *QueryEvidenceParams) *QueryEvidenceResult {
	if t.cache != nil {
		return t.cache.Get(params.HGVSNotation, params.MaxAge)
	}
	return nil
}

// cacheResult caches the evidence result
func (t *QueryEvidenceTool) cacheResult(params *QueryEvidenceParams, result *QueryEvidenceResult) {
	if t.cache != nil {
		t.cache.Set(params.HGVSNotation, result)
	}
}

// gatherEvidence orchestrates evidence gathering from multiple sources
func (t *QueryEvidenceTool) gatherEvidence(ctx context.Context, params *QueryEvidenceParams) (*QueryEvidenceResult, error) {
	result := &QueryEvidenceResult{
		VariantID:       t.generateVariantID(params.HGVSNotation),
		HGVSNotation:    params.HGVSNotation,
		DatabaseResults: make(map[string]interface{}),
		DataFreshness:   make(map[string]string),
	}

	// Query each requested database
	for _, database := range params.Databases {
		dbResult, err := t.queryDatabase(ctx, database, params)
		if err != nil {
			t.logger.WithFields(logrus.Fields{
				"database": database,
				"error":    err,
			}).Warn("Database query failed")
			// Continue with other databases even if one fails
			continue
		}

		result.DatabaseResults[database] = dbResult
		result.DataFreshness[database] = time.Now().Format(time.RFC3339)
	}

	// Aggregate evidence across databases
	result.AggregatedEvidence = t.aggregateEvidence(result.DatabaseResults)

	// Calculate quality scores
	result.QualityScores = t.calculateQualityScores(result.DatabaseResults, result.AggregatedEvidence)

	// Generate recommendations
	result.RecommendedActions = t.generateRecommendations(result.AggregatedEvidence, result.QualityScores)

	return result, nil
}

// queryDatabase queries a specific database
func (t *QueryEvidenceTool) queryDatabase(ctx context.Context, database string, params *QueryEvidenceParams) (interface{}, error) {
	t.logger.WithField("database", database).Debug("Querying database")

	switch database {
	case "clinvar":
		return t.queryClinVar(ctx, params)
	case "gnomad":
		return t.queryGnomAD(ctx, params)
	case "cosmic":
		return t.queryCOSMIC(ctx, params)
	case "lovd":
		return t.queryLOVD(ctx, params)
	case "hgmd":
		return t.queryHGMD(ctx, params)
	case "pubmed":
		return t.queryPubMed(ctx, params)
	default:
		return nil, fmt.Errorf("unsupported database: %s", database)
	}
}

// queryClinVar queries the ClinVar database (mock implementation)
func (t *QueryEvidenceTool) queryClinVar(ctx context.Context, params *QueryEvidenceParams) (interface{}, error) {
	// Mock ClinVar response - in production, this would call the actual ClinVar API
	return map[string]interface{}{
		"database": "clinvar",
		"entries": []ClinVarEntry{
			{
				AccessionID:          "RCV000123456",
				ClinicalSignificance: "Pathogenic",
				ReviewStatus:         "criteria provided, multiple submitters, no conflicts",
				Submitter:            "ClinGen",
				SubmissionDate:       "2024-01-15",
				Conditions:           []string{"Cystic fibrosis"},
				Assertions: map[string]string{
					"acmg_classification": "Pathogenic",
					"evidence_level":      "Strong",
				},
			},
		},
		"summary": map[string]interface{}{
			"total_entries":           1,
			"pathogenic_entries":      1,
			"conflicting_entries":     0,
			"review_status_summary":   map[string]int{"criteria_provided": 1},
		},
	}, nil
}

// queryGnomAD queries the Genome Aggregation Database (mock implementation)
func (t *QueryEvidenceTool) queryGnomAD(ctx context.Context, params *QueryEvidenceParams) (interface{}, error) {
	// Mock gnomAD response
	return map[string]interface{}{
		"database": "gnomad",
		"frequency_data": map[string]interface{}{
			"allele_frequency":  0.000001,
			"allele_count":      2,
			"allele_number":     251456,
			"homozygote_count":  0,
			"population_frequencies": map[string]float64{
				"AFR": 0.0,
				"AMR": 0.0,
				"ASJ": 0.0,
				"EAS": 0.0,
				"FIN": 0.0,
				"NFE": 0.000002,
				"OTH": 0.0,
			},
			"quality_metrics": map[string]float64{
				"depth":           30.5,
				"genotype_quality": 99.0,
				"variant_quality":  1200.0,
			},
		},
	}, nil
}

// queryCOSMIC queries the COSMIC database (mock implementation)
func (t *QueryEvidenceTool) queryCOSMIC(ctx context.Context, params *QueryEvidenceParams) (interface{}, error) {
	// Mock COSMIC response
	return map[string]interface{}{
		"database": "cosmic",
		"somatic_data": map[string]interface{}{
			"cosmic_id":        "COSV12345",
			"mutation_types":   []string{"substitution - missense"},
			"cancer_types":     []string{"lung adenocarcinoma", "breast carcinoma"},
			"tissue_types":     []string{"lung", "breast"},
			"sample_count":     45,
			"study_count":      12,
			"pathogenicity":    "likely pathogenic",
		},
	}, nil
}

// queryLOVD queries the LOVD database (mock implementation)
func (t *QueryEvidenceTool) queryLOVD(ctx context.Context, params *QueryEvidenceParams) (interface{}, error) {
	// Mock LOVD response
	return map[string]interface{}{
		"database": "lovd",
		"entries": []map[string]interface{}{
			{
				"variant_id":     "123456",
				"classification": "pathogenic",
				"submitter":      "Expert Lab",
				"submission_date": "2024-02-01",
			},
		},
	}, nil
}

// queryHGMD queries the HGMD database (mock implementation)
func (t *QueryEvidenceTool) queryHGMD(ctx context.Context, params *QueryEvidenceParams) (interface{}, error) {
	// Mock HGMD response
	return map[string]interface{}{
		"database": "hgmd",
		"entries": []map[string]interface{}{
			{
				"accession":      "CM123456",
				"mutation_type":  "DM",
				"disease":        "Cystic fibrosis",
				"reference":      "PMID:12345678",
			},
		},
	}, nil
}

// queryPubMed queries PubMed for literature (mock implementation)
func (t *QueryEvidenceTool) queryPubMed(ctx context.Context, params *QueryEvidenceParams) (interface{}, error) {
	// Mock PubMed response
	return map[string]interface{}{
		"database": "pubmed",
		"citations": []PubMedCitation{
			{
				PMID:      "12345678",
				Title:     "Functional analysis of CFTR variants in cystic fibrosis",
				Authors:   []string{"Smith J", "Johnson A", "Brown K"},
				Journal:   "Human Molecular Genetics",
				Year:      2024,
				Relevance: "high",
			},
		},
		"search_summary": map[string]interface{}{
			"total_results":    15,
			"relevant_results": 8,
			"recent_results":   3,
		},
	}, nil
}

// generateVariantID creates a unique identifier for the variant
func (t *QueryEvidenceTool) generateVariantID(hgvs string) string {
	// Simple hash-based ID - in production, use proper variant normalization
	return fmt.Sprintf("VAR_EVIDENCE_%d", time.Now().Unix())
}

// aggregateEvidence aggregates evidence across all database sources
func (t *QueryEvidenceTool) aggregateEvidence(dbResults map[string]interface{}) AggregatedEvidence {
	aggregated := AggregatedEvidence{}

	// Aggregate population frequency data
	aggregated.PopulationFrequency = t.aggregatePopulationFrequency(dbResults)

	// Aggregate clinical evidence
	aggregated.ClinicalEvidence = t.aggregateClinicalEvidence(dbResults)

	// Aggregate functional evidence
	aggregated.FunctionalEvidence = t.aggregateFunctionalEvidence(dbResults)

	// Aggregate computational data
	aggregated.ComputationalData = t.aggregateComputationalData(dbResults)

	// Aggregate literature evidence
	aggregated.LiteratureEvidence = t.aggregateLiteratureEvidence(dbResults)

	return aggregated
}

// aggregatePopulationFrequency aggregates frequency data from population databases
func (t *QueryEvidenceTool) aggregatePopulationFrequency(dbResults map[string]interface{}) PopulationFrequencyData {
	frequency := PopulationFrequencyData{
		PopulationFreqs: make(map[string]float64),
		QualityMetrics:  make(map[string]float64),
	}

	// Extract frequency data from gnomAD
	if gnomadData, exists := dbResults["gnomad"]; exists {
		if gnomadMap, ok := gnomadData.(map[string]interface{}); ok {
			if freqData, exists := gnomadMap["frequency_data"]; exists {
				if freqMap, ok := freqData.(map[string]interface{}); ok {
					if af, exists := freqMap["allele_frequency"]; exists {
						if afFloat, ok := af.(float64); ok {
							frequency.MaxFrequency = afFloat
						}
					}
				}
			}
		}
	}

	// Assess frequency for pathogenicity
	if frequency.MaxFrequency > 0.05 {
		frequency.FrequencyAssessment = "common - likely benign"
	} else if frequency.MaxFrequency > 0.01 {
		frequency.FrequencyAssessment = "intermediate frequency - uncertain"
	} else if frequency.MaxFrequency > 0.0001 {
		frequency.FrequencyAssessment = "rare - compatible with pathogenicity"
	} else {
		frequency.FrequencyAssessment = "absent/very rare - supports pathogenicity"
	}

	return frequency
}

// aggregateClinicalEvidence aggregates clinical significance data
func (t *QueryEvidenceTool) aggregateClinicalEvidence(dbResults map[string]interface{}) ClinicalEvidenceData {
	clinical := ClinicalEvidenceData{
		SubmissionSummary: make(map[string]int),
	}

	// Aggregate ClinVar data
	if _, exists := dbResults["clinvar"]; exists {
		// Extract ClinVar entries and determine overall significance
		clinical.OverallSignificance = "Pathogenic" // Mock result
		clinical.ReviewStatus = "criteria provided, multiple submitters, no conflicts"
	}

	return clinical
}

// aggregateFunctionalEvidence aggregates functional study data
func (t *QueryEvidenceTool) aggregateFunctionalEvidence(dbResults map[string]interface{}) FunctionalEvidenceData {
	functional := FunctionalEvidenceData{
		StudySummary: make(map[string]interface{}),
	}

	// Check for functional data across databases
	functional.HasFunctionalData = false
	functional.FunctionalPrediction = "no functional data available"

	return functional
}

// aggregateComputationalData aggregates in silico predictions
func (t *QueryEvidenceTool) aggregateComputationalData(dbResults map[string]interface{}) ComputationalData {
	computational := ComputationalData{
		ConservationScores: make(map[string]float64),
		SpliceScores:       make(map[string]float64),
	}

	// Mock computational predictions
	computational.SIFTScore = 0.02
	computational.SIFTPrediction = "deleterious"
	computational.PolyPhenScore = 0.95
	computational.PolyPhenPrediction = "probably damaging"
	computational.CADDScore = 25.3
	computational.ConsensusScore = 0.85
	computational.ConsensusPrediction = "damaging"

	return computational
}

// aggregateLiteratureEvidence aggregates literature and citation data
func (t *QueryEvidenceTool) aggregateLiteratureEvidence(dbResults map[string]interface{}) LiteratureEvidenceData {
	literature := LiteratureEvidenceData{}

	if pubmedData, exists := dbResults["pubmed"]; exists {
		if pubmedMap, ok := pubmedData.(map[string]interface{}); ok {
			if summary, exists := pubmedMap["search_summary"]; exists {
				if summaryMap, ok := summary.(map[string]interface{}); ok {
					if total, exists := summaryMap["total_results"]; exists {
						if totalInt, ok := total.(float64); ok {
							literature.TotalCitations = int(totalInt)
						}
					}
				}
			}
		}
	}

	return literature
}

// calculateQualityScores assesses the quality of evidence
func (t *QueryEvidenceTool) calculateQualityScores(dbResults map[string]interface{}, evidence AggregatedEvidence) EvidenceQualityScores {
	quality := EvidenceQualityScores{
		SourceReliability: make(map[string]float64),
		EvidenceStrength:  make(map[string]string),
	}

	// Calculate overall data completeness
	totalSources := 6.0 // clinvar, gnomad, cosmic, lovd, hgmd, pubmed
	availableSources := float64(len(dbResults))
	quality.DataCompleteness = availableSources / totalSources

	// Assess source reliability
	for source := range dbResults {
		switch source {
		case "clinvar":
			quality.SourceReliability[source] = 0.95
		case "gnomad":
			quality.SourceReliability[source] = 0.98
		case "cosmic":
			quality.SourceReliability[source] = 0.85
		default:
			quality.SourceReliability[source] = 0.80
		}
	}

	// Calculate conflict score
	quality.ConflictScore = 0.1 // Low conflict

	// Assess freshness
	quality.FreshnessScore = 0.9 // Recent data

	// Overall quality assessment
	if quality.DataCompleteness >= 0.8 && quality.ConflictScore <= 0.2 {
		quality.OverallQuality = "High"
	} else if quality.DataCompleteness >= 0.6 {
		quality.OverallQuality = "Moderate"
	} else {
		quality.OverallQuality = "Limited"
	}

	return quality
}

// generateRecommendations creates actionable recommendations based on evidence
func (t *QueryEvidenceTool) generateRecommendations(evidence AggregatedEvidence, quality EvidenceQualityScores) []string {
	recommendations := make([]string, 0)

	// Frequency-based recommendations
	if strings.Contains(evidence.PopulationFrequency.FrequencyAssessment, "common") {
		recommendations = append(recommendations, "High population frequency suggests benign interpretation")
	} else if strings.Contains(evidence.PopulationFrequency.FrequencyAssessment, "absent") {
		recommendations = append(recommendations, "Absent from population databases supports pathogenic interpretation")
	}

	// Clinical evidence recommendations
	if evidence.ClinicalEvidence.OverallSignificance == "Pathogenic" {
		recommendations = append(recommendations, "Strong clinical evidence supports pathogenic classification")
	}

	// Quality-based recommendations
	if quality.OverallQuality == "Limited" {
		recommendations = append(recommendations, "Limited evidence available - consider additional functional studies")
	}

	if quality.DataCompleteness < 0.5 {
		recommendations = append(recommendations, "Query additional databases for more comprehensive evidence")
	}

	if len(recommendations) == 0 {
		recommendations = append(recommendations, "Evidence review complete - proceed with variant classification")
	}

	return recommendations
}