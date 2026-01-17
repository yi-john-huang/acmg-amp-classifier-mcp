package domain

import (
	"context"
)

// APIGateway handles HTTP requests and coordinates the variant interpretation workflow
type APIGateway interface {
	HandleVariantInterpretation(ctx context.Context, req *VariantRequest) (*VariantResponse, error)
	ValidateAPIKey(apiKey string) (*Client, error)
	ApplyRateLimit(clientID string) error
}

// InputParser validates and standardizes variant nomenclature
type InputParser interface {
	ParseVariant(input string) (*StandardizedVariant, error)
	ValidateHGVS(hgvs string) error
	NormalizeVariant(variant *StandardizedVariant) error
	// Gene symbol parsing methods
	ParseGeneSymbol(input string) (*StandardizedVariant, error)
	ValidateGeneSymbol(symbol string) error
	GenerateHGVSFromGeneSymbol(geneSymbol, variant string) (string, error)
}

// InterpretationEngine applies ACMG/AMP guidelines for variant classification
type InterpretationEngine interface {
	ClassifyVariant(ctx context.Context, variant *StandardizedVariant, evidence *AggregatedEvidence) (*ClassificationResult, error)
	ApplyACMGAMPRules(evidence *AggregatedEvidence) (*RuleApplication, error)
	DetermineClassification(rules *RuleApplication) Classification
}

// KnowledgeBaseAccess manages connections to external databases and evidence aggregation
type KnowledgeBaseAccess interface {
	GatherEvidence(ctx context.Context, variant *StandardizedVariant) (*AggregatedEvidence, error)
	QueryClinVar(variant *StandardizedVariant) (*ClinVarData, error)
	QueryGnomAD(variant *StandardizedVariant) (*PopulationData, error)
	QueryCOSMIC(variant *StandardizedVariant) (*SomaticData, error)
}

// ReportGenerator formats classification results into structured reports
type ReportGenerator interface {
	GenerateReport(result *ClassificationResult, variant *StandardizedVariant) (*InterpretationReport, error)
	FormatForAIAgent(report *InterpretationReport) (string, error)
	GeneratePDFReport(report *InterpretationReport) ([]byte, error)
}

// VariantRepository defines the interface for variant data persistence
type VariantRepository interface {
	SaveVariant(ctx context.Context, variant *StandardizedVariant) error
	GetVariant(ctx context.Context, id string) (*StandardizedVariant, error)
	SaveInterpretation(ctx context.Context, interpretation *InterpretationRecord) error
	GetInterpretation(ctx context.Context, variantID string) (*InterpretationRecord, error)
}

// ConfigManager defines the interface for configuration management
type ConfigManager interface {
	GetConfig() *Config
	GetDatabaseConfig() *DatabaseConfig
	GetExternalAPIConfig() *ExternalAPIConfig
	GetServerConfig() *ServerConfig
	Reload() error
	Validate() error
	GetDatabaseConnectionString() string
	GetRedisConnectionString() string
	IsProduction() bool
	IsDevelopment() bool
}
