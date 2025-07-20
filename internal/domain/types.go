// Package domain contains core business entities and types for genetic variant classification
// following ACMG/AMP (American College of Medical Genetics and Genomics/Association for Molecular Pathology) guidelines.
//
// Reference: Richards et al. (2015) Standards and guidelines for the interpretation of sequence variants.
// Genet Med. 17(5):405-24. doi: 10.1038/gim.2015.30
package domain

import (
	"errors"
	"fmt"
	"time"
)

// Classification represents the ACMG/AMP classification result for genetic variants.
// These classifications follow the 2015 ACMG/AMP guidelines for sequence variant interpretation
// and represent the clinical significance of a genetic variant.
//
// Reference: ACMG/AMP 2015 Guidelines, Table 5
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

// Validation errors for medical data integrity
var (
	ErrNotFound              = errors.New("not found")
	ErrInvalidClassification = errors.New("invalid ACMG/AMP classification")
	ErrInvalidVariantType    = errors.New("invalid variant type")
	ErrInvalidRuleStrength   = errors.New("invalid ACMG/AMP rule strength")
	ErrInvalidConfidence     = errors.New("invalid confidence level")
)

// IsValid validates that the Classification follows ACMG/AMP guidelines.
// This is critical for medical software to ensure only valid classifications
// are used in clinical decision-making.
func (c Classification) IsValid() bool {
	switch c {
	case PATHOGENIC, LIKELY_PATHOGENIC, VUS, LIKELY_BENIGN, BENIGN:
		return true
	default:
		return false
	}
}

// String returns the string representation of the classification.
// Required for proper logging and audit trails in medical software.
func (c Classification) String() string {
	return string(c)
}

// LogFields returns structured logging fields for audit trails.
// Critical for medical software compliance and traceability.
// Returns strongly-typed fields to prevent logging errors in medical contexts.
func (c Classification) LogFields() map[string]any {
	return map[string]any{
		"classification":        string(c),
		"clinical_significance": c.ClinicalSignificance(),
		"is_valid":              c.IsValid(),
		"acmg_amp_compliant":    c.IsValid(),
		"classification_level":  c.getClassificationLevel(),
		"requires_action":       c.RequiresClinicalAction(),
	}
}

// ClinicalSignificance returns a human-readable description of the classification
// for clinical reporting and patient communication.
func (c Classification) ClinicalSignificance() string {
	switch c {
	case PATHOGENIC:
		return "Pathogenic - Disease-causing variant"
	case LIKELY_PATHOGENIC:
		return "Likely Pathogenic - Probably disease-causing variant"
	case VUS:
		return "Variant of Uncertain Significance - Clinical significance unknown"
	case LIKELY_BENIGN:
		return "Likely Benign - Probably not disease-causing"
	case BENIGN:
		return "Benign - Not disease-causing"
	default:
		return "Unknown classification"
	}
}

// getClassificationLevel returns the severity level for audit logging.
// Used internally for structured logging and audit trails.
func (c Classification) getClassificationLevel() string {
	switch c {
	case PATHOGENIC, LIKELY_PATHOGENIC:
		return "actionable"
	case VUS:
		return "uncertain"
	case LIKELY_BENIGN, BENIGN:
		return "non_actionable"
	default:
		return "unknown"
	}
}

// RequiresClinicalAction determines if the classification requires clinical follow-up.
// Critical for medical workflow automation and patient safety.
func (c Classification) RequiresClinicalAction() bool {
	switch c {
	case PATHOGENIC, LIKELY_PATHOGENIC:
		return true
	case VUS, LIKELY_BENIGN, BENIGN:
		return false
	default:
		return true // Conservative approach for unknown classifications
	}
}

// IsValid validates the variant type for medical genetics context.
func (vt VariantType) IsValid() bool {
	switch vt {
	case GERMLINE, SOMATIC:
		return true
	default:
		return false
	}
}

// IsValid validates the rule strength according to ACMG/AMP guidelines.
func (rs RuleStrength) IsValid() bool {
	switch rs {
	case VERY_STRONG, STRONG, MODERATE, SUPPORTING:
		return true
	default:
		return false
	}
}

// IsValid validates the confidence level.
func (cl ConfidenceLevel) IsValid() bool {
	switch cl {
	case HIGH, MEDIUM, LOW:
		return true
	default:
		return false
	}
}

// Variant represents a genetic variant with all necessary information
// for ACMG/AMP classification. This struct ensures all required data
// is captured for clinical decision-making.
type Variant struct {
	// Core identification
	ID   string      `json:"id" validate:"required"`
	HGVS string      `json:"hgvs" validate:"required,hgvs"`
	Type VariantType `json:"type" validate:"required"`

	// Genomic coordinates
	Chromosome string `json:"chromosome" validate:"required"`
	Position   int64  `json:"position" validate:"min=1"`
	Reference  string `json:"reference" validate:"required"`
	Alternate  string `json:"alternate" validate:"required"`

	// Gene information
	GeneSymbol string `json:"gene_symbol" validate:"required"`
	GeneID     string `json:"gene_id"`

	// Classification results
	Classification Classification  `json:"classification"`
	Confidence     ConfidenceLevel `json:"confidence"`

	// Audit trail
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Validate ensures the variant data meets medical software requirements.
// This is critical for preventing invalid data from entering the classification pipeline.
func (v *Variant) Validate() error {
	if v.ID == "" {
		return fmt.Errorf("variant validation: %w", errors.New("ID is required"))
	}

	if v.HGVS == "" {
		return fmt.Errorf("variant validation: %w", errors.New("HGVS notation is required"))
	}

	if !v.Type.IsValid() {
		return fmt.Errorf("variant validation: %w", ErrInvalidVariantType)
	}

	if v.Chromosome == "" {
		return fmt.Errorf("variant validation: %w", errors.New("chromosome is required"))
	}

	if v.Position <= 0 {
		return fmt.Errorf("variant validation: %w", errors.New("position must be positive"))
	}

	if v.GeneSymbol == "" {
		return fmt.Errorf("variant validation: %w", errors.New("gene symbol is required"))
	}

	// Validate classification if set
	if v.Classification != "" && !v.Classification.IsValid() {
		return fmt.Errorf("variant validation: %w", ErrInvalidClassification)
	}

	// Validate confidence if set
	if v.Confidence != "" && !v.Confidence.IsValid() {
		return fmt.Errorf("variant validation: %w", ErrInvalidConfidence)
	}

	return nil
}

// ACMGRule represents an individual ACMG/AMP rule with its assessment
type ACMGRule struct {
	Code       string          `json:"code" validate:"required"`     // e.g., "PVS1", "PS1", "PM1"
	Category   RuleCategory    `json:"category" validate:"required"` // PATHOGENIC or BENIGN
	Strength   RuleStrength    `json:"strength" validate:"required"` // VERY_STRONG, STRONG, etc.
	Applied    bool            `json:"applied"`                      // Whether this rule was applied
	Evidence   string          `json:"evidence,omitempty"`           // Supporting evidence
	Confidence ConfidenceLevel `json:"confidence,omitempty"`         // Confidence in rule application
	Reference  string          `json:"reference,omitempty"`          // Literature reference
}

// Validate ensures the ACMG rule data is valid for medical use.
func (r *ACMGRule) Validate() error {
	if r.Code == "" {
		return fmt.Errorf("ACMG rule validation: %w", errors.New("rule code is required"))
	}

	if !r.Category.IsValid() {
		return fmt.Errorf("ACMG rule validation: invalid category %s", r.Category)
	}

	if !r.Strength.IsValid() {
		return fmt.Errorf("ACMG rule validation: %w", ErrInvalidRuleStrength)
	}

	if r.Confidence != "" && !r.Confidence.IsValid() {
		return fmt.Errorf("ACMG rule validation: %w", ErrInvalidConfidence)
	}

	return nil
}

// IsValid validates the rule category
func (rc RuleCategory) IsValid() bool {
	switch rc {
	case PATHOGENIC_RULE, BENIGN_RULE:
		return true
	default:
		return false
	}
}

// ExtendedClassificationMetadata provides additional audit and traceability features
// that can be used alongside the existing ClassificationResult.
type ExtendedClassificationMetadata struct {
	VariantID    string            `json:"variant_id" validate:"required"`
	ClassifiedBy string            `json:"classified_by,omitempty"`
	Guidelines   string            `json:"guidelines,omitempty"` // ACMG/AMP version used
	Notes        string            `json:"notes,omitempty"`
	References   []string          `json:"references,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// Validate ensures the extended metadata meets medical software standards.
func (ecm *ExtendedClassificationMetadata) Validate() error {
	if ecm.VariantID == "" {
		return fmt.Errorf("extended classification metadata validation: %w", errors.New("variant ID is required"))
	}

	return nil
}
