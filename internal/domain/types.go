package domain

import (
	"errors"
	"time"
)

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

// InterpretationRecord represents a stored variant interpretation
type InterpretationRecord struct {
	ID               string                 `json:"id"`
	VariantID        string                 `json:"variant_id"`
	Classification   Classification         `json:"classification"`
	ConfidenceLevel  ConfidenceLevel        `json:"confidence_level"`
	AppliedRules     []ACMGAMPRule          `json:"applied_rules"`
	EvidenceSummary  map[string]interface{} `json:"evidence_summary"`
	ReportData       map[string]interface{} `json:"report_data"`
	ProcessingTimeMS int                    `json:"processing_time_ms"`
	ClientID         *string                `json:"client_id,omitempty"`
	RequestID        *string                `json:"request_id,omitempty"`
	CreatedAt        time.Time              `json:"created_at"`
	UpdatedAt        time.Time              `json:"updated_at"`
}

// Common errors
var (
	ErrNotFound = errors.New("not found")
)
