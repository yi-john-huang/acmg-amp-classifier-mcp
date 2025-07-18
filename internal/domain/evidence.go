package domain

import (
	"time"
)

// AggregatedEvidence represents all evidence gathered for variant interpretation
type AggregatedEvidence struct {
	ClinVarData       *ClinVarData       `json:"clinvar_data,omitempty"`
	PopulationData    *PopulationData    `json:"population_data,omitempty"`
	SomaticData       *SomaticData       `json:"somatic_data,omitempty"`
	ComputationalData *ComputationalData `json:"computational_data,omitempty"`
	GatheredAt        time.Time          `json:"gathered_at"`
}

// ClinVarData represents data from ClinVar database
type ClinVarData struct {
	VariationID          string              `json:"variation_id"`
	ClinicalSignificance string              `json:"clinical_significance"`
	ReviewStatus         string              `json:"review_status"`
	Submissions          []ClinVarSubmission `json:"submissions"`
	LastEvaluated        time.Time           `json:"last_evaluated"`
	Conditions           []string            `json:"conditions"`
}

// ClinVarSubmission represents a single ClinVar submission
type ClinVarSubmission struct {
	Submitter            string    `json:"submitter"`
	ClinicalSignificance string    `json:"clinical_significance"`
	ReviewStatus         string    `json:"review_status"`
	SubmissionDate       time.Time `json:"submission_date"`
	Condition            string    `json:"condition"`
}

// PopulationData represents population frequency data from gnomAD
type PopulationData struct {
	AlleleFrequency       float64            `json:"allele_frequency"`
	AlleleCount           int                `json:"allele_count"`
	AlleleNumber          int                `json:"allele_number"`
	PopulationFrequencies map[string]float64 `json:"population_frequencies"`
	HomozygoteCount       int                `json:"homozygote_count"`
	QualityMetrics        *QualityMetrics    `json:"quality_metrics"`
}

// QualityMetrics represents quality metrics for population data
type QualityMetrics struct {
	Coverage   int     `json:"coverage"`
	Quality    float64 `json:"quality"`
	FilterPass bool    `json:"filter_pass"`
}

// SomaticData represents somatic variant data from COSMIC
type SomaticData struct {
	CosmicID      string   `json:"cosmic_id"`
	TumorTypes    []string `json:"tumor_types"`
	SampleCount   int      `json:"sample_count"`
	MutationCount int      `json:"mutation_count"`
	Pathogenicity string   `json:"pathogenicity"`
}

// ComputationalData represents computational prediction scores
type ComputationalData struct {
	SIFTScore     float64 `json:"sift_score"`
	PolyPhenScore float64 `json:"polyphen_score"`
	CADDScore     float64 `json:"cadd_score"`
	GERPScore     float64 `json:"gerp_score"`
	PhyloPScore   float64 `json:"phylop_score"`
}
