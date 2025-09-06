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
	LiteratureData    *LiteratureData    `json:"literature_data,omitempty"`
	LOVDData          *LOVDData          `json:"lovd_data,omitempty"`
	HGMDData          *HGMDData          `json:"hgmd_data,omitempty"`
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

// LiteratureData represents literature evidence from PubMed and other sources
type LiteratureData struct {
	TotalCitations      int        `json:"total_citations"`
	RetrievedCitations  int        `json:"retrieved_citations"`
	Citations           []Citation `json:"citations"`
	SearchQuery         string     `json:"search_query"`
	LastUpdated         time.Time  `json:"last_updated"`
	HighImpactCitations int        `json:"high_impact_citations"`
	RecentCitations     int        `json:"recent_citations"`
}

// Citation represents a single literature citation
type Citation struct {
	PMID         string   `json:"pmid"`
	DOI          string   `json:"doi,omitempty"`
	Title        string   `json:"title"`
	Authors      []string `json:"authors"`
	Journal      string   `json:"journal"`
	Year         int      `json:"year"`
	Volume       string   `json:"volume,omitempty"`
	Pages        string   `json:"pages,omitempty"`
	ISSN         string   `json:"issn,omitempty"`
	Abstract     string   `json:"abstract,omitempty"`
	StudyType    string   `json:"study_type"`    // functional_study, clinical_study, case_report, etc.
	Relevance    string   `json:"relevance"`     // high, moderate, low
	Database     string   `json:"database"`      // PubMed, EMBASE, etc.
	ImpactFactor float64  `json:"impact_factor,omitempty"`
	KeyFindings  []string `json:"key_findings,omitempty"`
}

// LOVDData represents data from LOVD (Leiden Open Variation Database)
type LOVDData struct {
	VariantID           string              `json:"variant_id"`
	GeneSpecificDB      string              `json:"gene_specific_db"`
	Classification      string              `json:"classification"`
	ClinicalDescription string              `json:"clinical_description"`
	Phenotype           string              `json:"phenotype"`
	Pathogenicity       string              `json:"pathogenicity"`
	FunctionalData      []LOVDFunctionalData `json:"functional_data"`
	References          []string            `json:"references"`
	SubmissionDate      time.Time           `json:"submission_date"`
	LastUpdated         time.Time           `json:"last_updated"`
}

// LOVDFunctionalData represents functional study data from LOVD
type LOVDFunctionalData struct {
	StudyType   string `json:"study_type"`
	Method      string `json:"method"`
	Result      string `json:"result"`
	Conclusion  string `json:"conclusion"`
	Reference   string `json:"reference"`
	Reliability string `json:"reliability"`
}

// HGMDData represents data from Human Gene Mutation Database
type HGMDData struct {
	MutationID      string    `json:"mutation_id"`
	DiseaseName     string    `json:"disease_name"`
	MutationType    string    `json:"mutation_type"`    // DM, DM?, DP, FP
	Classification  string    `json:"classification"`   // Disease-causing, Likely pathogenic, etc.
	PhenotypeMIM    string    `json:"phenotype_mim"`
	GeneSymbol      string    `json:"gene_symbol"`
	Chromosome      string    `json:"chromosome"`
	GenomicLocation string    `json:"genomic_location"`
	Reference       string    `json:"reference"`
	PubMedID        string    `json:"pubmed_id"`
	SubmissionDate  time.Time `json:"submission_date"`
	Inheritance     string    `json:"inheritance"`      // AD, AR, XL, etc.
	Tag             string    `json:"tag"`              // Additional classification tags
}
