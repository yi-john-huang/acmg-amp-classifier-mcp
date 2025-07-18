package domain

import (
	"time"
)

// Core Enums and Types

// Classification represents the ACMG/AMP variant classification categories
type Classification string

const (
	PATHOGENIC        Classification = "PATHOGENIC"
	LIKELY_PATHOGENIC Classification = "LIKELY_PATHOGENIC"
	VUS               Classification = "VUS"
	LIKELY_BENIGN     Classification = "LIKELY_BENIGN"
	BENIGN            Classification = "BENIGN"
)

// VariantType represents the type of genetic variant
type VariantType string

const (
	GERMLINE VariantType = "GERMLINE"
	SOMATIC  VariantType = "SOMATIC"
)

// RuleStrength represents the strength of ACMG/AMP evidence criteria
type RuleStrength string

const (
	VERY_STRONG RuleStrength = "VERY_STRONG"
	STRONG      RuleStrength = "STRONG"
	MODERATE    RuleStrength = "MODERATE"
	SUPPORTING  RuleStrength = "SUPPORTING"
)

// RuleCategory represents the category of ACMG/AMP rules
type RuleCategory string

const (
	PATHOGENIC_RULE RuleCategory = "PATHOGENIC"
	BENIGN_RULE     RuleCategory = "BENIGN"
)

// ConfidenceLevel represents the confidence in the classification
type ConfidenceLevel string

const (
	HIGH_CONFIDENCE   ConfidenceLevel = "HIGH"
	MEDIUM_CONFIDENCE ConfidenceLevel = "MEDIUM"
	LOW_CONFIDENCE    ConfidenceLevel = "LOW"
)

// Request/Response Models

// VariantRequest represents an incoming variant interpretation request
type VariantRequest struct {
	VariantID     string            `json:"variant_id"`
	HGVSNotation  string            `json:"hgvs_notation"`
	VariantType   VariantType       `json:"variant_type"`
	GeneSymbol    string            `json:"gene_symbol"`
	Transcript    string            `json:"transcript,omitempty"`
	ClientContext map[string]string `json:"client_context,omitempty"`
}

// VariantResponse represents the response from variant interpretation
type VariantResponse struct {
	VariantID      string               `json:"variant_id"`
	Classification ClassificationResult `json:"classification"`
	Evidence       []EvidenceItem       `json:"evidence"`
	Report         InterpretationReport `json:"report"`
	ProcessingTime time.Duration        `json:"processing_time"`
	Timestamp      time.Time            `json:"timestamp"`
}

// Core Data Models

// StandardizedVariant represents a parsed and normalized genetic variant
type StandardizedVariant struct {
	Chromosome   string      `json:"chromosome"`
	Position     int64       `json:"position"`
	Reference    string      `json:"reference"`
	Alternative  string      `json:"alternative"`
	HGVSGenomic  string      `json:"hgvs_genomic"`
	HGVSCoding   string      `json:"hgvs_coding,omitempty"`
	HGVSProtein  string      `json:"hgvs_protein,omitempty"`
	GeneSymbol   string      `json:"gene_symbol"`
	TranscriptID string      `json:"transcript_id,omitempty"`
	VariantType  VariantType `json:"variant_type"`
}

// ClassificationResult represents the result of variant classification
type ClassificationResult struct {
	Classification  Classification  `json:"classification"`
	Confidence      ConfidenceLevel `json:"confidence"`
	AppliedRules    []ACMGAMPRule   `json:"applied_rules"`
	RulesSummary    string          `json:"rules_summary"`
	Recommendations []string        `json:"recommendations,omitempty"`
}

// ACMGAMPRule represents an individual ACMG/AMP evidence criterion
type ACMGAMPRule struct {
	Code      string       `json:"code"`     // e.g., "PVS1", "PS1"
	Category  RuleCategory `json:"category"` // PATHOGENIC, BENIGN
	Strength  RuleStrength `json:"strength"` // VERY_STRONG, STRONG, MODERATE, SUPPORTING
	Met       bool         `json:"met"`
	Evidence  string       `json:"evidence"`
	Rationale string       `json:"rationale"`
}

// RuleApplication represents the result of applying ACMG/AMP rules
type RuleApplication struct {
	PathogenicRules []ACMGAMPRule `json:"pathogenic_rules"`
	BenignRules     []ACMGAMPRule `json:"benign_rules"`
	Summary         string        `json:"summary"`
}

// Evidence Models

// AggregatedEvidence represents all evidence gathered for a variant
type AggregatedEvidence struct {
	ClinicalSignificance *ClinVarData     `json:"clinical_significance,omitempty"`
	PopulationFrequency  *PopulationData  `json:"population_frequency,omitempty"`
	SomaticEvidence      *SomaticData     `json:"somatic_evidence,omitempty"`
	ComputationalData    *PredictionData  `json:"computational_data,omitempty"`
	FunctionalData       *FunctionalData  `json:"functional_data,omitempty"`
	SegregationData      *SegregationData `json:"segregation_data,omitempty"`
}

// ClinVarData represents data from ClinVar database
type ClinVarData struct {
	VariationID          string    `json:"variation_id"`
	ClinicalSignificance string    `json:"clinical_significance"`
	ReviewStatus         string    `json:"review_status"`
	Submitters           []string  `json:"submitters"`
	LastUpdated          time.Time `json:"last_updated"`
}

// PopulationData represents population frequency data from gnomAD
type PopulationData struct {
	AlleleFrequency float64            `json:"allele_frequency"`
	AlleleCount     int                `json:"allele_count"`
	AlleleNumber    int                `json:"allele_number"`
	PopulationFreqs map[string]float64 `json:"population_frequencies"`
	HomozygoteCount int                `json:"homozygote_count"`
}

// SomaticData represents somatic variant data from COSMIC
type SomaticData struct {
	CosmicID           string   `json:"cosmic_id"`
	TumorTypes         []string `json:"tumor_types"`
	MutationFrequency  float64  `json:"mutation_frequency"`
	DrugResistance     []string `json:"drug_resistance,omitempty"`
	TherapeuticTargets []string `json:"therapeutic_targets,omitempty"`
}

// PredictionData represents computational prediction data
type PredictionData struct {
	SIFTScore     float64                `json:"sift_score,omitempty"`
	PolyPhenScore float64                `json:"polyphen_score,omitempty"`
	CADDScore     float64                `json:"cadd_score,omitempty"`
	Predictions   map[string]interface{} `json:"predictions,omitempty"`
}

// FunctionalData represents functional study data
type FunctionalData struct {
	FunctionalStudies []string `json:"functional_studies,omitempty"`
	ProteinEffect     string   `json:"protein_effect,omitempty"`
	RNAEffect         string   `json:"rna_effect,omitempty"`
}

// SegregationData represents family segregation data
type SegregationData struct {
	FamilyStudies      []string `json:"family_studies,omitempty"`
	SegregationScore   float64  `json:"segregation_score,omitempty"`
	AffectedCarriers   int      `json:"affected_carriers,omitempty"`
	UnaffectedCarriers int      `json:"unaffected_carriers,omitempty"`
}

// EvidenceItem represents a single piece of evidence
type EvidenceItem struct {
	Source      string      `json:"source"`
	Type        string      `json:"type"`
	Description string      `json:"description"`
	Value       interface{} `json:"value"`
	Confidence  float64     `json:"confidence"`
}

// Report Models

// InterpretationReport represents a complete interpretation report
type InterpretationReport struct {
	Summary         string            `json:"summary"`
	Classification  Classification    `json:"classification"`
	EvidenceSummary []EvidenceSummary `json:"evidence_summary"`
	Recommendations []string          `json:"recommendations"`
	Limitations     []string          `json:"limitations"`
	References      []Reference       `json:"references"`
	GeneratedAt     time.Time         `json:"generated_at"`
}

// EvidenceSummary represents a summary of evidence for the report
type EvidenceSummary struct {
	Category    string   `json:"category"`
	Description string   `json:"description"`
	Strength    string   `json:"strength"`
	Sources     []string `json:"sources"`
}

// Reference represents a literature or database reference
type Reference struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Authors string `json:"authors,omitempty"`
	Journal string `json:"journal,omitempty"`
	Year    int    `json:"year,omitempty"`
	URL     string `json:"url,omitempty"`
	DOI     string `json:"doi,omitempty"`
}

// Client Models

// Client represents an API client
type Client struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	APIKey    string    `json:"api_key"`
	RateLimit int       `json:"rate_limit"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	LastUsed  time.Time `json:"last_used,omitempty"`
}

// Database Models

// InterpretationRecord represents a stored interpretation record
type InterpretationRecord struct {
	ID               string               `json:"id"`
	VariantID        string               `json:"variant_id"`
	Classification   Classification       `json:"classification"`
	ConfidenceLevel  ConfidenceLevel      `json:"confidence_level"`
	AppliedRules     []ACMGAMPRule        `json:"applied_rules"`
	EvidenceSummary  AggregatedEvidence   `json:"evidence_summary"`
	ReportData       InterpretationReport `json:"report_data"`
	ProcessingTimeMS int                  `json:"processing_time_ms"`
	CreatedAt        time.Time            `json:"created_at"`
}

// Configuration Models

// Config represents the main application configuration
type Config struct {
	Server      ServerConfig      `mapstructure:"server"`
	Database    DatabaseConfig    `mapstructure:"database"`
	ExternalAPI ExternalAPIConfig `mapstructure:"external_api"`
	Cache       CacheConfig       `mapstructure:"cache"`
	Logging     LoggingConfig     `mapstructure:"logging"`
}

// ServerConfig represents HTTP server configuration
type ServerConfig struct {
	Host         string        `mapstructure:"host"`
	Port         int           `mapstructure:"port"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	IdleTimeout  time.Duration `mapstructure:"idle_timeout"`
	TLSEnabled   bool          `mapstructure:"tls_enabled"`
	CertFile     string        `mapstructure:"cert_file"`
	KeyFile      string        `mapstructure:"key_file"`
}

// DatabaseConfig represents database connection configuration
type DatabaseConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	Database        string        `mapstructure:"database"`
	Username        string        `mapstructure:"username"`
	Password        string        `mapstructure:"password"`
	SSLMode         string        `mapstructure:"ssl_mode"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
}

// ExternalAPIConfig represents external API configuration
type ExternalAPIConfig struct {
	ClinVar ClinVarConfig `mapstructure:"clinvar"`
	GnomAD  GnomADConfig  `mapstructure:"gnomad"`
	COSMIC  COSMICConfig  `mapstructure:"cosmic"`
}

// ClinVarConfig represents ClinVar API configuration
type ClinVarConfig struct {
	BaseURL    string        `mapstructure:"base_url"`
	APIKey     string        `mapstructure:"api_key"`
	Timeout    time.Duration `mapstructure:"timeout"`
	RateLimit  int           `mapstructure:"rate_limit"`
	RetryCount int           `mapstructure:"retry_count"`
}

// GnomADConfig represents gnomAD API configuration
type GnomADConfig struct {
	BaseURL    string        `mapstructure:"base_url"`
	APIKey     string        `mapstructure:"api_key"`
	Timeout    time.Duration `mapstructure:"timeout"`
	RateLimit  int           `mapstructure:"rate_limit"`
	RetryCount int           `mapstructure:"retry_count"`
}

// COSMICConfig represents COSMIC API configuration
type COSMICConfig struct {
	BaseURL    string        `mapstructure:"base_url"`
	APIKey     string        `mapstructure:"api_key"`
	Timeout    time.Duration `mapstructure:"timeout"`
	RateLimit  int           `mapstructure:"rate_limit"`
	RetryCount int           `mapstructure:"retry_count"`
}

// CacheConfig represents cache configuration
type CacheConfig struct {
	RedisURL    string        `mapstructure:"redis_url"`
	DefaultTTL  time.Duration `mapstructure:"default_ttl"`
	MaxRetries  int           `mapstructure:"max_retries"`
	PoolSize    int           `mapstructure:"pool_size"`
	PoolTimeout time.Duration `mapstructure:"pool_timeout"`
}

// LoggingConfig represents logging configuration
type LoggingConfig struct {
	Level      string `mapstructure:"level"`
	Format     string `mapstructure:"format"`
	Output     string `mapstructure:"output"`
	Filename   string `mapstructure:"filename"`
	MaxSize    int    `mapstructure:"max_size"`
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAge     int    `mapstructure:"max_age"`
	Compress   bool   `mapstructure:"compress"`
}

// Error Models

// MCPError represents a structured error response
type MCPError struct {
	Code      string    `json:"code"`
	Message   string    `json:"message"`
	Details   string    `json:"details,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	RequestID string    `json:"request_id"`
}

// Error implements the error interface
func (e *MCPError) Error() string {
	return e.Message
}

// Error Categories
const (
	ErrInvalidInput   = "INVALID_INPUT"
	ErrDatabaseError  = "DATABASE_ERROR"
	ErrExternalAPI    = "EXTERNAL_API_ERROR"
	ErrClassification = "CLASSIFICATION_ERROR"
	ErrRateLimit      = "RATE_LIMIT_EXCEEDED"
	ErrAuthentication = "AUTHENTICATION_ERROR"
	ErrInternalServer = "INTERNAL_SERVER_ERROR"
)
