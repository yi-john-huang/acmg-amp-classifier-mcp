package feedback

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"time"

	_ "github.com/lib/pq"
)

// PostgresStore implements the Store interface using PostgreSQL.
type PostgresStore struct {
	db *sql.DB
}

// NewPostgresStore creates a new PostgreSQL feedback store.
// It expects the database and schema to already exist (created via migrations).
func NewPostgresStore(db *sql.DB) (*PostgresStore, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection is required")
	}

	// Verify connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &PostgresStore{db: db}, nil
}

// NewPostgresStoreFromURL creates a new PostgreSQL feedback store from a connection URL.
func NewPostgresStoreFromURL(databaseURL string) (*PostgresStore, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	store, err := NewPostgresStore(db)
	if err != nil {
		db.Close()
		return nil, err
	}

	return store, nil
}

// Save stores or updates user feedback for a classification.
func (s *PostgresStore) Save(ctx context.Context, feedback *Feedback) error {
	now := time.Now()

	// Use upsert (INSERT ... ON CONFLICT)
	query := `
		INSERT INTO feedback (
			variant, normalized_hgvs, cancer_type,
			suggested_classification, user_classification, user_agreed,
			evidence_summary, notes, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (normalized_hgvs, cancer_type) DO UPDATE SET
			variant = EXCLUDED.variant,
			suggested_classification = EXCLUDED.suggested_classification,
			user_classification = EXCLUDED.user_classification,
			user_agreed = EXCLUDED.user_agreed,
			evidence_summary = EXCLUDED.evidence_summary,
			notes = EXCLUDED.notes,
			updated_at = EXCLUDED.updated_at
		RETURNING id, created_at
	`

	err := s.db.QueryRowContext(ctx, query,
		feedback.Variant,
		feedback.NormalizedHGVS,
		feedback.CancerType,
		string(feedback.SuggestedClassification),
		string(feedback.UserClassification),
		feedback.UserAgreed,
		feedback.EvidenceSummary,
		feedback.Notes,
		now,
		now,
	).Scan(&feedback.ID, &feedback.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to save feedback: %w", err)
	}

	feedback.UpdatedAt = now
	return nil
}

// Get retrieves the most recent feedback for a variant.
func (s *PostgresStore) Get(ctx context.Context, normalizedHGVS string, cancerType string) (*Feedback, error) {
	query := `
		SELECT id, variant, normalized_hgvs, cancer_type,
			suggested_classification, user_classification, user_agreed,
			evidence_summary, notes, created_at, updated_at
		FROM feedback
		WHERE normalized_hgvs = $1 AND cancer_type = $2
		LIMIT 1
	`

	fb := &Feedback{}
	var suggestedClass, userClass string

	err := s.db.QueryRowContext(ctx, query, normalizedHGVS, cancerType).Scan(
		&fb.ID, &fb.Variant, &fb.NormalizedHGVS, &fb.CancerType,
		&suggestedClass, &userClass, &fb.UserAgreed,
		&fb.EvidenceSummary, &fb.Notes, &fb.CreatedAt, &fb.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get feedback: %w", err)
	}

	fb.SuggestedClassification = Classification(suggestedClass)
	fb.UserClassification = Classification(userClass)
	return fb, nil
}

// List returns all feedback entries with pagination.
func (s *PostgresStore) List(ctx context.Context, limit, offset int) ([]*Feedback, error) {
	query := `
		SELECT id, variant, normalized_hgvs, cancer_type,
			suggested_classification, user_classification, user_agreed,
			evidence_summary, notes, created_at, updated_at
		FROM feedback
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := s.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list feedback: %w", err)
	}
	defer rows.Close()

	var result []*Feedback
	for rows.Next() {
		fb := &Feedback{}
		var suggestedClass, userClass string

		err := rows.Scan(
			&fb.ID, &fb.Variant, &fb.NormalizedHGVS, &fb.CancerType,
			&suggestedClass, &userClass, &fb.UserAgreed,
			&fb.EvidenceSummary, &fb.Notes, &fb.CreatedAt, &fb.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		fb.SuggestedClassification = Classification(suggestedClass)
		fb.UserClassification = Classification(userClass)
		result = append(result, fb)
	}

	return result, rows.Err()
}

// Count returns the total number of feedback entries.
func (s *PostgresStore) Count(ctx context.Context) (int64, error) {
	var count int64
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM feedback").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count feedback: %w", err)
	}
	return count, nil
}

// Delete removes a feedback entry by ID.
func (s *PostgresStore) Delete(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM feedback WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("failed to delete feedback: %w", err)
	}
	return nil
}

// maxExportLimit is the maximum number of entries to export at once.
const pgMaxExportLimit = 1000000

// ExportJSON exports all feedback to a JSON writer.
func (s *PostgresStore) ExportJSON(ctx context.Context, writer io.Writer) error {
	all, err := s.List(ctx, pgMaxExportLimit, 0)
	if err != nil {
		return fmt.Errorf("failed to list feedback: %w", err)
	}

	export := &FeedbackExport{
		Version:    "1.0",
		ExportedAt: time.Now(),
		Count:      len(all),
		Feedback:   all,
	}

	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(export)
}

// ImportJSON imports feedback from a JSON reader.
func (s *PostgresStore) ImportJSON(ctx context.Context, reader io.Reader) (imported int, skipped int, err error) {
	var export FeedbackExport
	if err := json.NewDecoder(reader).Decode(&export); err != nil {
		return 0, 0, fmt.Errorf("failed to decode JSON: %w", err)
	}

	for _, fb := range export.Feedback {
		// Check if exists
		existing, err := s.Get(ctx, fb.NormalizedHGVS, fb.CancerType)
		if err != nil {
			return imported, skipped, fmt.Errorf("failed to check existing: %w", err)
		}

		if existing != nil {
			skipped++
			continue
		}

		// Import
		if err := s.Save(ctx, fb); err != nil {
			return imported, skipped, fmt.Errorf("failed to save: %w", err)
		}
		imported++
	}

	return imported, skipped, nil
}

// Close closes the store and releases resources.
func (s *PostgresStore) Close() error {
	return s.db.Close()
}
