package testing

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ClinicalValidationSuite manages clinical validation tests using known variant datasets
type ClinicalValidationSuite struct {
	factory    *MockClientFactory
	serverURL  string
	logger     *logrus.Logger
	config     ClinicalValidationConfig
	datasets   map[string]ClinicalDataset
	results    []ClinicalTestResult
}

type ClinicalValidationConfig struct {
	StrictValidation      bool          `json:"strict_validation"`
	ToleranceThreshold    float64       `json:"tolerance_threshold"`
	RequireExplanations   bool          `json:"require_explanations"`
	ValidateEvidence      bool          `json:"validate_evidence"`
	CheckConsistency      bool          `json:"check_consistency"`
	TestTimeout          time.Duration `json:"test_timeout"`
	EnableBenchmarking    bool          `json:"enable_benchmarking"`
}

type ClinicalDataset struct {
	Name        string                  `json:"name"`
	Source      string                  `json:"source"`
	Version     string                  `json:"version"`
	Description string                  `json:"description"`
	Variants    []ClinicalVariant       `json:"variants"`
	Statistics  DatasetStatistics       `json:"statistics"`
	Metadata    map[string]interface{}  `json:"metadata"`
}

type ClinicalVariant struct {
	ID                    string                 `json:"id"`
	HGVS                  string                 `json:"hgvs"`
	Gene                  string                 `json:"gene"`
	Transcript            string                 `json:"transcript"`
	VariantType          string                 `json:"variant_type"`
	Consequence          string                 `json:"consequence"`
	ExpectedClassification string                `json:"expected_classification"`
	ConfidenceLevel      string                 `json:"confidence_level"`
	ClinicalSignificance string                 `json:"clinical_significance"`
	ExpectedEvidence     ACMGEvidence           `json:"expected_evidence"`
	PopulationData       PopulationData         `json:"population_data"`
	FunctionalData       FunctionalData         `json:"functional_data"`
	ComputationalData    ComputationalData      `json:"computational_data"`
	ClinicalData         ClinicalData           `json:"clinical_data"`
	ReviewStatus         string                 `json:"review_status"`
	LastUpdated          time.Time              `json:"last_updated"`
	References           []Reference            `json:"references"`
	Tags                 []string               `json:"tags"`
	TestCategory         string                 `json:"test_category"`
}

type ACMGEvidence struct {
	PathogenicStrong     []string `json:"pathogenic_strong"`
	PathogenicModerate   []string `json:"pathogenic_moderate"`
	PathogenicSupporting []string `json:"pathogenic_supporting"`
	BenignStrong         []string `json:"benign_strong"`
	BenignSupporting     []string `json:"benign_supporting"`
	StandAlone          []string `json:"stand_alone"`
}

type PopulationData struct {
	AlleleFrequency      map[string]float64 `json:"allele_frequency"`
	HomozygoteCount      int                `json:"homozygote_count"`
	AlleleCount          int                `json:"allele_count"`
	AlleleNumber         int                `json:"allele_number"`
	PopulationMaxAF      float64            `json:"population_max_af"`
	FilteringAF          float64            `json:"filtering_af"`
}

type FunctionalData struct {
	InVivoStudies        []StudyResult   `json:"in_vivo_studies"`
	InVitroStudies       []StudyResult   `json:"in_vitro_studies"`
	CellularStudies      []StudyResult   `json:"cellular_studies"`
	AnimalModels         []StudyResult   `json:"animal_models"`
	RescueExperiments    []StudyResult   `json:"rescue_experiments"`
	FunctionalDomain     bool            `json:"functional_domain"`
	CriticalRegion       bool            `json:"critical_region"`
}

type ComputationalData struct {
	ConservationScores   map[string]float64  `json:"conservation_scores"`
	SpliceScores         map[string]float64  `json:"splice_scores"`
	PredictionScores     map[string]float64  `json:"prediction_scores"`
	StructuralPrediction StructuralData      `json:"structural_prediction"`
	PhylogeneticAnalysis PhylogeneticData    `json:"phylogenetic_analysis"`
}

type ClinicalData struct {
	PhenotypeAssociation   []PhenotypeInfo     `json:"phenotype_association"`
	DiseaseAssociation     []DiseaseInfo       `json:"disease_association"`
	CaseStudies            []CaseStudy         `json:"case_studies"`
	FamilialSegregation    []SegregationData   `json:"familial_segregation"`
	DeNovo                 bool                `json:"de_novo"`
}

type StudyResult struct {
	StudyID     string  `json:"study_id"`
	Result      string  `json:"result"`
	Confidence  float64 `json:"confidence"`
	Description string  `json:"description"`
	Reference   string  `json:"reference"`
}

type StructuralData struct {
	ProteinEffect     string             `json:"protein_effect"`
	DomainDisruption  bool               `json:"domain_disruption"`
	StructuralImpact  map[string]float64 `json:"structural_impact"`
}

type PhylogeneticData struct {
	ConservationLevel string             `json:"conservation_level"`
	SpeciesConserved  []string           `json:"species_conserved"`
	ConservationScore float64            `json:"conservation_score"`
}

type PhenotypeInfo struct {
	HPOTerm     string  `json:"hpo_term"`
	Confidence  float64 `json:"confidence"`
	Association string  `json:"association"`
}

type DiseaseInfo struct {
	DiseaseID   string  `json:"disease_id"`
	DiseaseName string  `json:"disease_name"`
	Association string  `json:"association"`
	Inheritance string  `json:"inheritance"`
}

type CaseStudy struct {
	CaseID      string            `json:"case_id"`
	Phenotype   []string          `json:"phenotype"`
	Inheritance string            `json:"inheritance"`
	Evidence    map[string]string `json:"evidence"`
}

type SegregationData struct {
	FamilyID    string `json:"family_id"`
	Segregates  bool   `json:"segregates"`
	AffectedCount   int    `json:"affected_count"`
	UnaffectedCount int    `json:"unaffected_count"`
}

type Reference struct {
	PMID    string `json:"pmid"`
	DOI     string `json:"doi"`
	Title   string `json:"title"`
	Authors string `json:"authors"`
}

type DatasetStatistics struct {
	TotalVariants      int                    `json:"total_variants"`
	ClassificationDist map[string]int         `json:"classification_distribution"`
	GeneDistribution   map[string]int         `json:"gene_distribution"`
	ConsequenceDist    map[string]int         `json:"consequence_distribution"`
	ConfidenceLevels   map[string]int         `json:"confidence_levels"`
	LastUpdated        time.Time              `json:"last_updated"`
}

type ClinicalTestResult struct {
	VariantID              string                 `json:"variant_id"`
	ExpectedClassification string                 `json:"expected_classification"`
	ActualClassification   string                 `json:"actual_classification"`
	Match                  bool                   `json:"match"`
	ConfidenceScore        float64                `json:"confidence_score"`
	ExpectedEvidence       ACMGEvidence           `json:"expected_evidence"`
	ActualEvidence         ACMGEvidence           `json:"actual_evidence"`
	EvidenceMatch          EvidenceMatchResult    `json:"evidence_match"`
	ProcessingTime         time.Duration          `json:"processing_time"`
	Errors                 []string               `json:"errors"`
	Warnings               []string               `json:"warnings"`
	QualityMetrics         QualityMetrics         `json:"quality_metrics"`
}

type EvidenceMatchResult struct {
	PathogenicMatch    float64 `json:"pathogenic_match"`
	BenignMatch        float64 `json:"benign_match"`
	OverallMatch       float64 `json:"overall_match"`
	MissingEvidence    []string `json:"missing_evidence"`
	ExtraEvidence      []string `json:"extra_evidence"`
	MismatchedStrength []string `json:"mismatched_strength"`
}

type QualityMetrics struct {
	Completeness    float64 `json:"completeness"`
	Accuracy        float64 `json:"accuracy"`
	Consistency     float64 `json:"consistency"`
	Explainability  float64 `json:"explainability"`
}

func NewClinicalValidationSuite(serverURL string, config ClinicalValidationConfig) *ClinicalValidationSuite {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})

	suite := &ClinicalValidationSuite{
		factory:   NewMockClientFactory(),
		serverURL: serverURL,
		logger:    logger,
		config:    config,
		datasets:  make(map[string]ClinicalDataset),
		results:   make([]ClinicalTestResult, 0),
	}

	suite.loadClinicalDatasets()
	return suite
}

func (suite *ClinicalValidationSuite) loadClinicalDatasets() {
	// Load ClinVar Expert Panel variants
	suite.datasets["clinvar_expert"] = suite.createClinVarExpertDataset()
	
	// Load well-characterized pathogenic variants
	suite.datasets["pathogenic_benchmark"] = suite.createPathogenicBenchmarkDataset()
	
	// Load well-characterized benign variants
	suite.datasets["benign_benchmark"] = suite.createBenignBenchmarkDataset()
	
	// Load uncertain significance variants for edge case testing
	suite.datasets["vus_edge_cases"] = suite.createVUSEdgeCaseDataset()
	
	// Load population genetics benchmark
	suite.datasets["population_benchmark"] = suite.createPopulationBenchmarkDataset()
}

func (suite *ClinicalValidationSuite) createClinVarExpertDataset() ClinicalDataset {
	return ClinicalDataset{
		Name:        "ClinVar Expert Panel Variants",
		Source:      "ClinVar",
		Version:     "2024.01",
		Description: "High-confidence variants reviewed by expert panels",
		Variants: []ClinicalVariant{
			{
				ID: "clinvar_pathogenic_001",
				HGVS: "NM_000492.3:c.1521_1523del",
				Gene: "CFTR",
				Transcript: "NM_000492.3",
				VariantType: "deletion",
				Consequence: "frameshift",
				ExpectedClassification: "pathogenic",
				ConfidenceLevel: "high",
				ClinicalSignificance: "pathogenic",
				ExpectedEvidence: ACMGEvidence{
					PathogenicStrong: []string{"PVS1"},
					PathogenicModerate: []string{"PM2"},
					PathogenicSupporting: []string{"PP3"},
				},
				PopulationData: PopulationData{
					AlleleFrequency: map[string]float64{
						"gnomAD_total": 0.000024,
						"gnomAD_NFE": 0.000021,
					},
					PopulationMaxAF: 0.000024,
				},
				ReviewStatus: "reviewed_by_expert_panel",
				TestCategory: "pathogenic_frameshift",
				Tags: []string{"cystic_fibrosis", "expert_panel", "high_confidence"},
			},
			{
				ID: "clinvar_benign_001",
				HGVS: "NM_000492.3:c.1408A>G",
				Gene: "CFTR",
				Transcript: "NM_000492.3",
				VariantType: "substitution",
				Consequence: "missense",
				ExpectedClassification: "benign",
				ConfidenceLevel: "high",
				ClinicalSignificance: "benign",
				ExpectedEvidence: ACMGEvidence{
					BenignStrong: []string{"BS1"},
					BenignSupporting: []string{"BP4", "BP6"},
				},
				PopulationData: PopulationData{
					AlleleFrequency: map[string]float64{
						"gnomAD_total": 0.12,
						"gnomAD_NFE": 0.15,
					},
					PopulationMaxAF: 0.15,
				},
				ReviewStatus: "reviewed_by_expert_panel",
				TestCategory: "benign_common",
				Tags: []string{"common_variant", "expert_panel", "high_confidence"},
			},
		},
		Statistics: DatasetStatistics{
			TotalVariants: 2,
			ClassificationDist: map[string]int{
				"pathogenic": 1,
				"benign": 1,
			},
			GeneDistribution: map[string]int{
				"CFTR": 2,
			},
			ConsequenceDist: map[string]int{
				"frameshift": 1,
				"missense": 1,
			},
		},
	}
}

func (suite *ClinicalValidationSuite) createPathogenicBenchmarkDataset() ClinicalDataset {
	return ClinicalDataset{
		Name:        "Pathogenic Variant Benchmark",
		Source:      "Literature + ClinVar",
		Version:     "2024.01",
		Description: "Well-characterized pathogenic variants across multiple genes",
		Variants: []ClinicalVariant{
			{
				ID: "brca1_pathogenic_001",
				HGVS: "NM_007294.3:c.5266dup",
				Gene: "BRCA1",
				Transcript: "NM_007294.3",
				VariantType: "duplication",
				Consequence: "frameshift",
				ExpectedClassification: "pathogenic",
				ConfidenceLevel: "high",
				ClinicalSignificance: "pathogenic",
				ExpectedEvidence: ACMGEvidence{
					PathogenicStrong: []string{"PVS1", "PS3"},
					PathogenicModerate: []string{"PM2"},
				},
				TestCategory: "pathogenic_frameshift_cancer",
				Tags: []string{"breast_cancer", "ovarian_cancer", "frameshift"},
			},
			{
				ID: "tp53_pathogenic_001",
				HGVS: "NM_000546.5:c.743G>A",
				Gene: "TP53",
				Transcript: "NM_000546.5",
				VariantType: "substitution",
				Consequence: "missense",
				ExpectedClassification: "likely_pathogenic",
				ConfidenceLevel: "high",
				ClinicalSignificance: "pathogenic",
				ExpectedEvidence: ACMGEvidence{
					PathogenicStrong: []string{"PS3"},
					PathogenicModerate: []string{"PM2"},
					PathogenicSupporting: []string{"PP3"},
				},
				TestCategory: "pathogenic_missense_tumor_suppressor",
				Tags: []string{"li_fraumeni", "cancer_predisposition", "missense"},
			},
		},
		Statistics: DatasetStatistics{
			TotalVariants: 2,
			ClassificationDist: map[string]int{
				"pathogenic": 1,
				"likely_pathogenic": 1,
			},
		},
	}
}

func (suite *ClinicalValidationSuite) createBenignBenchmarkDataset() ClinicalDataset {
	return ClinicalDataset{
		Name:        "Benign Variant Benchmark",
		Source:      "gnomAD + ClinVar",
		Version:     "2024.01",
		Description: "Well-characterized benign variants with population evidence",
		Variants: []ClinicalVariant{
			{
				ID: "benign_common_001",
				HGVS: "NM_000038.5:c.79G>A",
				Gene: "APC",
				Transcript: "NM_000038.5",
				VariantType: "substitution",
				Consequence: "missense",
				ExpectedClassification: "benign",
				ConfidenceLevel: "high",
				ClinicalSignificance: "benign",
				ExpectedEvidence: ACMGEvidence{
					StandAlone: []string{"BA1"},
				},
				PopulationData: PopulationData{
					AlleleFrequency: map[string]float64{
						"gnomAD_total": 0.25,
						"gnomAD_AFR": 0.18,
						"gnomAD_NFE": 0.28,
					},
					PopulationMaxAF: 0.28,
				},
				TestCategory: "benign_high_frequency",
				Tags: []string{"common_variant", "population_evidence"},
			},
		},
		Statistics: DatasetStatistics{
			TotalVariants: 1,
			ClassificationDist: map[string]int{
				"benign": 1,
			},
		},
	}
}

func (suite *ClinicalValidationSuite) createVUSEdgeCaseDataset() ClinicalDataset {
	return ClinicalDataset{
		Name:        "VUS Edge Cases",
		Source:      "Curated",
		Version:     "2024.01",
		Description: "Variants of uncertain significance representing edge cases",
		Variants: []ClinicalVariant{
			{
				ID: "vus_edge_001",
				HGVS: "NM_000059.3:c.1234C>T",
				Gene: "BRCA2",
				Transcript: "NM_000059.3",
				VariantType: "substitution",
				Consequence: "missense",
				ExpectedClassification: "uncertain_significance",
				ConfidenceLevel: "medium",
				ClinicalSignificance: "uncertain_significance",
				ExpectedEvidence: ACMGEvidence{
					PathogenicModerate: []string{"PM2"},
					PathogenicSupporting: []string{"PP3"},
					BenignSupporting: []string{"BP4"},
				},
				TestCategory: "vus_conflicting_evidence",
				Tags: []string{"uncertain_significance", "conflicting_evidence"},
			},
		},
		Statistics: DatasetStatistics{
			TotalVariants: 1,
			ClassificationDist: map[string]int{
				"uncertain_significance": 1,
			},
		},
	}
}

func (suite *ClinicalValidationSuite) createPopulationBenchmarkDataset() ClinicalDataset {
	return ClinicalDataset{
		Name:        "Population Genetics Benchmark",
		Source:      "gnomAD v3.1",
		Version:     "2024.01",
		Description: "Variants selected to test population frequency-based rules",
		Variants: []ClinicalVariant{
			{
				ID: "population_001",
				HGVS: "NM_000492.3:c.350G>A",
				Gene: "CFTR",
				Transcript: "NM_000492.3",
				VariantType: "substitution",
				Consequence: "missense",
				ExpectedClassification: "benign",
				ConfidenceLevel: "medium",
				ExpectedEvidence: ACMGEvidence{
					BenignStrong: []string{"BS1"},
				},
				PopulationData: PopulationData{
					AlleleFrequency: map[string]float64{
						"gnomAD_total": 0.008,
						"gnomAD_NFE": 0.012,
					},
					PopulationMaxAF: 0.012,
					AlleleCount: 2000,
					AlleleNumber: 250000,
				},
				TestCategory: "benign_population_frequency",
				Tags: []string{"population_evidence", "frequency_based"},
			},
		},
		Statistics: DatasetStatistics{
			TotalVariants: 1,
			ClassificationDist: map[string]int{
				"benign": 1,
			},
		},
	}
}

func (suite *ClinicalValidationSuite) RunClinicalValidationTests(ctx context.Context, t *testing.T) {
	suite.logger.Info("Starting comprehensive clinical validation tests")

	for datasetName, dataset := range suite.datasets {
		t.Run(fmt.Sprintf("Dataset_%s", datasetName), func(t *testing.T) {
			suite.runDatasetValidation(ctx, t, dataset)
		})
	}

	// Run aggregate analysis
	t.Run("AggregateAnalysis", func(t *testing.T) {
		suite.runAggregateAnalysis(t)
	})
	
	// Run consistency checks
	t.Run("ConsistencyChecks", func(t *testing.T) {
		suite.runConsistencyChecks(ctx, t)
	})
}

func (suite *ClinicalValidationSuite) runDatasetValidation(ctx context.Context, t *testing.T, dataset ClinicalDataset) {
	suite.logger.WithFields(logrus.Fields{
		"dataset": dataset.Name,
		"variants": len(dataset.Variants),
	}).Info("Running dataset validation")

	clientConfig := ClientConfig{
		ID: "clinical_validator", Name: "ClinicalValidator", Version: "1.0.0",
		Transport: TransportWebSocket,
	}

	client, err := suite.factory.CreateClient(clientConfig)
	require.NoError(t, err)
	defer suite.factory.RemoveClient(clientConfig.ID)

	err = client.Connect(ctx, suite.serverURL)
	require.NoError(t, err)
	defer client.Disconnect()

	var correctPredictions int
	var totalVariants int

	for _, variant := range dataset.Variants {
		t.Run(fmt.Sprintf("Variant_%s", variant.ID), func(t *testing.T) {
			testCtx, cancel := context.WithTimeout(ctx, suite.config.TestTimeout)
			defer cancel()

			result := suite.validateVariant(testCtx, t, client, variant)
			suite.results = append(suite.results, result)

			if result.Match {
				correctPredictions++
			}
			totalVariants++

			// Assert individual variant results
			if suite.config.StrictValidation {
				assert.True(t, result.Match, 
					"Variant %s classification should match expected result", variant.ID)
				
				if suite.config.ValidateEvidence {
					assert.GreaterOrEqual(t, result.EvidenceMatch.OverallMatch, suite.config.ToleranceThreshold,
						"Evidence match should meet threshold for variant %s", variant.ID)
				}
			}
		})
	}

	// Assert dataset-level accuracy
	accuracy := float64(correctPredictions) / float64(totalVariants)
	suite.logger.WithFields(logrus.Fields{
		"dataset": dataset.Name,
		"accuracy": accuracy,
		"correct": correctPredictions,
		"total": totalVariants,
	}).Info("Dataset validation completed")

	minAccuracy := 0.8 // 80% minimum accuracy
	assert.GreaterOrEqual(t, accuracy, minAccuracy,
		"Dataset %s accuracy should be at least %.1f%%", dataset.Name, minAccuracy*100)
}

func (suite *ClinicalValidationSuite) validateVariant(ctx context.Context, t *testing.T, client *MockMCPClient, variant ClinicalVariant) ClinicalTestResult {
	startTime := time.Now()
	
	result := ClinicalTestResult{
		VariantID: variant.ID,
		ExpectedClassification: variant.ExpectedClassification,
		ExpectedEvidence: variant.ExpectedEvidence,
		Errors: make([]string, 0),
		Warnings: make([]string, 0),
	}

	// Step 1: Validate HGVS
	_, err := client.CallTool(ctx, "validate_hgvs", map[string]interface{}{
		"notation": variant.HGVS,
	})
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("HGVS validation failed: %v", err))
	}

	// Step 2: Query evidence
	evidenceResult, err := client.CallTool(ctx, "query_evidence", map[string]interface{}{
		"variant": variant.HGVS,
		"sources": []string{"clinvar", "gnomad", "cosmic"},
	})
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Evidence query failed: %v", err))
	}

	// Step 3: Apply ACMG rules
	var ruleResults []*ToolCallResult
	allExpectedRules := append(
		append(
			append(variant.ExpectedEvidence.PathogenicStrong, variant.ExpectedEvidence.PathogenicModerate...),
			variant.ExpectedEvidence.PathogenicSupporting...),
		append(
			append(variant.ExpectedEvidence.BenignStrong, variant.ExpectedEvidence.BenignSupporting...),
			variant.ExpectedEvidence.StandAlone...)...,
	)

	for _, rule := range allExpectedRules {
		ruleResult, err := client.CallTool(ctx, "apply_rule", map[string]interface{}{
			"rule": rule,
			"variant": variant.HGVS,
			"evidence": evidenceResult,
		})
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Rule %s application failed: %v", rule, err))
		} else {
			ruleResults = append(ruleResults, ruleResult)
		}
	}

	// Step 4: Combine evidence
	combineResult, err := client.CallTool(ctx, "combine_evidence", map[string]interface{}{
		"rule_results": ruleResults,
	})
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Evidence combination failed: %v", err))
	}

	// Step 5: Final classification
	classificationResult, err := client.CallTool(ctx, "classify_variant", map[string]interface{}{
		"variant": variant.HGVS,
		"combined_evidence": combineResult,
	})
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Classification failed: %v", err))
		result.ActualClassification = "error"
	} else {
		result.ActualClassification = suite.extractClassification(classificationResult)
	}

	// Extract evidence from results
	result.ActualEvidence = suite.extractEvidence(ruleResults)
	
	// Calculate metrics
	result.ProcessingTime = time.Since(startTime)
	result.Match = suite.classificationMatches(result.ExpectedClassification, result.ActualClassification)
	result.EvidenceMatch = suite.calculateEvidenceMatch(result.ExpectedEvidence, result.ActualEvidence)
	result.QualityMetrics = suite.calculateQualityMetrics(result, variant)
	result.ConfidenceScore = suite.calculateConfidenceScore(result)

	return result
}

func (suite *ClinicalValidationSuite) extractClassification(result *ToolCallResult) string {
	if result == nil || result.IsError || len(result.Content) == 0 {
		return "unknown"
	}

	var classResult map[string]interface{}
	if err := json.Unmarshal([]byte(result.Content[0].Text), &classResult); err != nil {
		return "unknown"
	}

	if classification, ok := classResult["classification"].(string); ok {
		return classification
	}

	return "unknown"
}

func (suite *ClinicalValidationSuite) extractEvidence(ruleResults []*ToolCallResult) ACMGEvidence {
	evidence := ACMGEvidence{
		PathogenicStrong:     make([]string, 0),
		PathogenicModerate:   make([]string, 0),
		PathogenicSupporting: make([]string, 0),
		BenignStrong:         make([]string, 0),
		BenignSupporting:     make([]string, 0),
		StandAlone:          make([]string, 0),
	}

	for _, result := range ruleResults {
		if result == nil || result.IsError || len(result.Content) == 0 {
			continue
		}

		var ruleResult map[string]interface{}
		if err := json.Unmarshal([]byte(result.Content[0].Text), &ruleResult); err != nil {
			continue
		}

		ruleName, _ := ruleResult["rule"].(string)
		strength, _ := ruleResult["strength"].(string)
		applicable, _ := ruleResult["applicable"].(bool)

		if !applicable {
			continue
		}

		// Categorize based on rule type and strength
		if strings.HasPrefix(ruleName, "PVS") || 
		   (strings.HasPrefix(ruleName, "PS") && strength == "very_strong") {
			evidence.PathogenicStrong = append(evidence.PathogenicStrong, ruleName)
		} else if strings.HasPrefix(ruleName, "PS") || 
				  (strings.HasPrefix(ruleName, "PM") && strength == "strong") {
			evidence.PathogenicStrong = append(evidence.PathogenicStrong, ruleName)
		} else if strings.HasPrefix(ruleName, "PM") {
			evidence.PathogenicModerate = append(evidence.PathogenicModerate, ruleName)
		} else if strings.HasPrefix(ruleName, "PP") {
			evidence.PathogenicSupporting = append(evidence.PathogenicSupporting, ruleName)
		} else if strings.HasPrefix(ruleName, "BA") {
			evidence.StandAlone = append(evidence.StandAlone, ruleName)
		} else if strings.HasPrefix(ruleName, "BS") {
			evidence.BenignStrong = append(evidence.BenignStrong, ruleName)
		} else if strings.HasPrefix(ruleName, "BP") {
			evidence.BenignSupporting = append(evidence.BenignSupporting, ruleName)
		}
	}

	return evidence
}

func (suite *ClinicalValidationSuite) classificationMatches(expected, actual string) bool {
	// Direct match
	if expected == actual {
		return true
	}

	// Allow some flexibility in classification matching
	pathogenicClasses := []string{"pathogenic", "likely_pathogenic"}
	benignClasses := []string{"benign", "likely_benign"}

	expectedPathogenic := sliceContains(pathogenicClasses, expected)
	actualPathogenic := sliceContains(pathogenicClasses, actual)
	expectedBenign := sliceContains(benignClasses, expected)
	actualBenign := sliceContains(benignClasses, actual)

	// Allow pathogenic/likely_pathogenic interchange if tolerance threshold is met
	if expectedPathogenic && actualPathogenic && suite.config.ToleranceThreshold > 0 {
		return true
	}

	// Allow benign/likely_benign interchange if tolerance threshold is met
	if expectedBenign && actualBenign && suite.config.ToleranceThreshold > 0 {
		return true
	}

	return false
}

func (suite *ClinicalValidationSuite) calculateEvidenceMatch(expected, actual ACMGEvidence) EvidenceMatchResult {
	result := EvidenceMatchResult{
		MissingEvidence:    make([]string, 0),
		ExtraEvidence:      make([]string, 0),
		MismatchedStrength: make([]string, 0),
	}

	// Calculate pathogenic evidence match
	pathogenicExpected := append(append(expected.PathogenicStrong, expected.PathogenicModerate...), expected.PathogenicSupporting...)
	pathogenicActual := append(append(actual.PathogenicStrong, actual.PathogenicModerate...), actual.PathogenicSupporting...)
	result.PathogenicMatch = suite.calculateListMatch(pathogenicExpected, pathogenicActual)

	// Calculate benign evidence match
	benignExpected := append(append(expected.BenignStrong, expected.BenignSupporting...), expected.StandAlone...)
	benignActual := append(append(actual.BenignStrong, actual.BenignSupporting...), actual.StandAlone...)
	result.BenignMatch = suite.calculateListMatch(benignExpected, benignActual)

	// Overall match
	result.OverallMatch = (result.PathogenicMatch + result.BenignMatch) / 2

	// Find missing and extra evidence
	allExpected := append(pathogenicExpected, benignExpected...)
	allActual := append(pathogenicActual, benignActual...)

	for _, rule := range allExpected {
		if !sliceContains(allActual, rule) {
			result.MissingEvidence = append(result.MissingEvidence, rule)
		}
	}

	for _, rule := range allActual {
		if !sliceContains(allExpected, rule) {
			result.ExtraEvidence = append(result.ExtraEvidence, rule)
		}
	}

	return result
}

func (suite *ClinicalValidationSuite) calculateListMatch(expected, actual []string) float64 {
	if len(expected) == 0 && len(actual) == 0 {
		return 1.0
	}
	if len(expected) == 0 || len(actual) == 0 {
		return 0.0
	}

	matches := 0
	for _, item := range expected {
		if sliceContains(actual, item) {
			matches++
		}
	}

	return float64(matches) / float64(len(expected))
}

func (suite *ClinicalValidationSuite) calculateQualityMetrics(result ClinicalTestResult, variant ClinicalVariant) QualityMetrics {
	// Completeness: How much of the expected workflow was completed
	completeness := 1.0
	if len(result.Errors) > 0 {
		completeness = 1.0 - (float64(len(result.Errors)) / 5.0) // Assume 5 main steps
	}

	// Accuracy: Classification correctness
	accuracy := 0.0
	if result.Match {
		accuracy = 1.0
	}

	// Consistency: Evidence consistency with classification
	consistency := result.EvidenceMatch.OverallMatch

	// Explainability: Whether results include sufficient explanation
	explainability := 0.8 // Default score, would be calculated from actual explanations

	return QualityMetrics{
		Completeness:   completeness,
		Accuracy:       accuracy,
		Consistency:    consistency,
		Explainability: explainability,
	}
}

func (suite *ClinicalValidationSuite) calculateConfidenceScore(result ClinicalTestResult) float64 {
	score := 0.0
	
	// Base score on classification match
	if result.Match {
		score += 0.4
	}
	
	// Evidence match contribution
	score += result.EvidenceMatch.OverallMatch * 0.3
	
	// Error penalty
	if len(result.Errors) == 0 {
		score += 0.2
	}
	
	// Processing time bonus (faster is better, up to a point)
	if result.ProcessingTime < 2*time.Second {
		score += 0.1
	}
	
	return score
}

func (suite *ClinicalValidationSuite) runAggregateAnalysis(t *testing.T) {
	if len(suite.results) == 0 {
		t.Skip("No results available for aggregate analysis")
		return
	}

	// Calculate overall metrics
	var totalMatches int
	var totalResults int
	var totalProcessingTime time.Duration
	var evidenceMatchSum float64
	
	classificationStats := make(map[string]int)
	errorStats := make(map[string]int)

	for _, result := range suite.results {
		totalResults++
		if result.Match {
			totalMatches++
		}
		
		totalProcessingTime += result.ProcessingTime
		evidenceMatchSum += result.EvidenceMatch.OverallMatch
		
		classificationStats[result.ActualClassification]++
		
		for _, err := range result.Errors {
			errorType := "general_error"
			if strings.Contains(err, "timeout") {
				errorType = "timeout"
			} else if strings.Contains(err, "validation") {
				errorType = "validation_error"
			}
			errorStats[errorType]++
		}
	}

	overallAccuracy := float64(totalMatches) / float64(totalResults)
	avgProcessingTime := totalProcessingTime / time.Duration(totalResults)
	avgEvidenceMatch := evidenceMatchSum / float64(totalResults)

	suite.logger.WithFields(logrus.Fields{
		"overall_accuracy":       overallAccuracy,
		"avg_processing_time":    avgProcessingTime,
		"avg_evidence_match":     avgEvidenceMatch,
		"classification_stats":   classificationStats,
		"error_stats":           errorStats,
	}).Info("Clinical validation aggregate analysis")

	// Assert aggregate quality thresholds
	assert.GreaterOrEqual(t, overallAccuracy, 0.85, 
		"Overall accuracy should be at least 85%%")
	assert.LessOrEqual(t, avgProcessingTime, 10*time.Second,
		"Average processing time should be reasonable")
	assert.GreaterOrEqual(t, avgEvidenceMatch, 0.7,
		"Average evidence match should be at least 70%%")
}

func (suite *ClinicalValidationSuite) runConsistencyChecks(ctx context.Context, t *testing.T) {
	if !suite.config.CheckConsistency {
		t.Skip("Consistency checks disabled")
		return
	}

	suite.logger.Info("Running consistency checks")

	clientConfig := ClientConfig{
		ID: "consistency_client", Name: "ConsistencyClient", Version: "1.0.0",
		Transport: TransportWebSocket,
	}

	client, err := suite.factory.CreateClient(clientConfig)
	require.NoError(t, err)
	defer suite.factory.RemoveClient(clientConfig.ID)

	err = client.Connect(ctx, suite.serverURL)
	require.NoError(t, err)
	defer client.Disconnect()

	// Test same variant multiple times for consistency
	testVariant := "NM_000492.3:c.1521_1523del"
	results := make([]string, 5)

	for i := 0; i < 5; i++ {
		classificationResult, err := client.CallTool(ctx, "classify_variant", map[string]interface{}{
			"variant": testVariant,
		})
		require.NoError(t, err)
		
		results[i] = suite.extractClassification(classificationResult)
	}

	// Check for consistency
	firstResult := results[0]
	allConsistent := true
	for _, result := range results {
		if result != firstResult {
			allConsistent = false
			break
		}
	}

	assert.True(t, allConsistent, 
		"Multiple classifications of the same variant should be consistent")

	suite.logger.WithFields(logrus.Fields{
		"variant": testVariant,
		"results": results,
		"consistent": allConsistent,
	}).Info("Consistency check completed")
}

func (suite *ClinicalValidationSuite) GetResults() []ClinicalTestResult {
	return suite.results
}

func (suite *ClinicalValidationSuite) GetDatasets() map[string]ClinicalDataset {
	return suite.datasets
}

func sliceContains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}