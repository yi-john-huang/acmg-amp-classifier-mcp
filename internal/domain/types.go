package domain

// Classification represents the ACMG/AMP classification result
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

// RuleStrength represents the strength of ACMG/AMP evidence rules
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
	HIGH   ConfidenceLevel = "High"
	MEDIUM ConfidenceLevel = "Medium"
	LOW    ConfidenceLevel = "Low"
)
