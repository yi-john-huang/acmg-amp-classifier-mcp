// Package feedback provides user feedback storage for variant classifications.
// It stores user corrections and agreements to improve future classifications.
package feedback

import (
	"context"
	"io"
	"time"
)

// Classification represents the ACMG/AMP classification categories.
type Classification string

const (
	ClassificationPathogenic       Classification = "Pathogenic"
	ClassificationLikelyPathogenic Classification = "Likely Pathogenic"
	ClassificationVUS              Classification = "VUS"
	ClassificationLikelyBenign     Classification = "Likely Benign"
	ClassificationBenign           Classification = "Benign"
)

// Feedback represents a user's feedback on a variant classification.
type Feedback struct {
	ID                      int64          `json:"id,omitempty"`
	Variant                 string         `json:"variant"`                   // Original input
	NormalizedHGVS          string         `json:"normalized_hgvs"`           // Normalized HGVS notation
	CancerType              string         `json:"cancer_type,omitempty"`     // Clinical context
	SuggestedClassification Classification `json:"suggested_classification"`  // System's suggestion
	UserClassification      Classification `json:"user_classification"`       // User's decision
	UserAgreed              bool           `json:"user_agreed"`               // Did user agree with suggestion?
	EvidenceSummary         string         `json:"evidence_summary,omitempty"` // Evidence used
	Notes                   string         `json:"notes,omitempty"`           // User notes
	CreatedAt               time.Time      `json:"created_at"`
	UpdatedAt               time.Time      `json:"updated_at"`
}

// Store defines the interface for feedback storage operations.
type Store interface {
	// Save stores or updates user feedback for a classification.
	// If feedback for the same variant+cancer_type exists, it will be updated.
	Save(ctx context.Context, feedback *Feedback) error

	// Get retrieves the most recent feedback for a variant.
	// If cancerType is empty, returns the first matching variant.
	Get(ctx context.Context, normalizedHGVS string, cancerType string) (*Feedback, error)

	// List returns all feedback entries with pagination.
	List(ctx context.Context, limit, offset int) ([]*Feedback, error)

	// Count returns the total number of feedback entries.
	Count(ctx context.Context) (int64, error)

	// Delete removes a feedback entry by ID.
	Delete(ctx context.Context, id int64) error

	// ExportJSON exports all feedback to a JSON writer.
	ExportJSON(ctx context.Context, writer io.Writer) error

	// ImportJSON imports feedback from a JSON reader.
	// Returns the number of imported and skipped entries.
	ImportJSON(ctx context.Context, reader io.Reader) (imported int, skipped int, err error)

	// Close closes the store and releases resources.
	Close() error
}

// FeedbackExport represents the JSON export format.
type FeedbackExport struct {
	Version   string      `json:"version"`
	ExportedAt time.Time  `json:"exported_at"`
	Count     int         `json:"count"`
	Feedback  []*Feedback `json:"feedback"`
}
