package domain

import (
	"time"
)

// ClassificationResult represents the result of variant classification
type ClassificationResult struct {
	Classification    Classification      `json:"classification"`
	Confidence        ConfidenceLevel     `json:"confidence"`
	Rules             *RuleApplication    `json:"rules"`
	Evidence          *AggregatedEvidence `json:"evidence"`
	Rationale         string              `json:"rationale"`
	ClassifiedAt      time.Time           `json:"classified_at"`
	ClassifierVersion string              `json:"classifier_version"`
}

// RuleApplication represents the application of ACMG/AMP rules
type RuleApplication struct {
	PathogenicRules []ACMGAMPRule `json:"pathogenic_rules"`
	BenignRules     []ACMGAMPRule `json:"benign_rules"`
	Summary         string        `json:"summary"`
	AppliedAt       time.Time     `json:"applied_at"`
}

// ACMGAMPRule represents a single ACMG/AMP evidence rule
type ACMGAMPRule struct {
	Code      string       `json:"code"`      // e.g., "PVS1", "PS1"
	Category  RuleCategory `json:"category"`  // PATHOGENIC, BENIGN
	Strength  RuleStrength `json:"strength"`  // VERY_STRONG, STRONG, MODERATE, SUPPORTING
	Met       bool         `json:"met"`       // Whether the rule criteria was met
	Evidence  string       `json:"evidence"`  // Evidence supporting the rule
	Rationale string       `json:"rationale"` // Rationale for applying the rule
}

// InterpretationReport represents a comprehensive interpretation report
type InterpretationReport struct {
	ID              string                `json:"id"`
	Variant         *StandardizedVariant  `json:"variant"`
	Classification  *ClassificationResult `json:"classification"`
	Summary         string                `json:"summary"`
	ClinicalContext string                `json:"clinical_context"`
	Recommendations []string              `json:"recommendations"`
	References      []Reference           `json:"references"`
	GeneratedAt     time.Time             `json:"generated_at"`
	Version         string                `json:"version"`
}

// Reference represents a literature or database reference
type Reference struct {
	Type    string `json:"type"`
	ID      string `json:"id"`
	Title   string `json:"title"`
	Authors string `json:"authors"`
	Journal string `json:"journal"`
	Year    int    `json:"year"`
	URL     string `json:"url"`
}
