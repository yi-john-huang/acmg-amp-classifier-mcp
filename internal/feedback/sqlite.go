package feedback

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// SQLiteStore implements the Store interface using SQLite.
type SQLiteStore struct {
	db     *sql.DB
	dbPath string
}

// NewSQLiteStore creates a new SQLite feedback store.
// It creates the database file and schema if they don't exist.
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	// Open database
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable WAL mode for better concurrency
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to set WAL mode: %w", err)
	}

	// Create schema
	if err := createSchema(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create schema: %w", err)
	}

	return &SQLiteStore{
		db:     db,
		dbPath: dbPath,
	}, nil
}

// scanner is an interface for sql.Row and sql.Rows
type scanner interface {
	Scan(dest ...interface{}) error
}

// scanFeedback scans a row into a Feedback struct.
func scanFeedback(s scanner) (*Feedback, error) {
	fb := &Feedback{}
	var suggestedClass, userClass string

	err := s.Scan(
		&fb.ID, &fb.Variant, &fb.NormalizedHGVS, &fb.CancerType,
		&suggestedClass, &userClass, &fb.UserAgreed,
		&fb.EvidenceSummary, &fb.Notes, &fb.CreatedAt, &fb.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	fb.SuggestedClassification = Classification(suggestedClass)
	fb.UserClassification = Classification(userClass)
	return fb, nil
}

// createSchema creates the database tables and indexes.
func createSchema(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS feedback (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		variant TEXT NOT NULL,
		normalized_hgvs TEXT NOT NULL,
		cancer_type TEXT DEFAULT '',
		suggested_classification TEXT NOT NULL,
		user_classification TEXT NOT NULL,
		user_agreed INTEGER NOT NULL DEFAULT 0,
		evidence_summary TEXT DEFAULT '',
		notes TEXT DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(normalized_hgvs, cancer_type)
	);

	CREATE INDEX IF NOT EXISTS idx_normalized_hgvs ON feedback(normalized_hgvs);
	CREATE INDEX IF NOT EXISTS idx_cancer_type ON feedback(cancer_type);
	CREATE INDEX IF NOT EXISTS idx_created_at ON feedback(created_at);
	`

	_, err := db.Exec(schema)
	return err
}

// Save stores or updates user feedback for a classification.
func (s *SQLiteStore) Save(ctx context.Context, feedback *Feedback) error {
	now := time.Now()

	// Check if exists
	var existingID int64
	err := s.db.QueryRowContext(ctx,
		"SELECT id FROM feedback WHERE normalized_hgvs = ? AND cancer_type = ?",
		feedback.NormalizedHGVS, feedback.CancerType,
	).Scan(&existingID)

	if err == nil {
		// Update existing
		feedback.ID = existingID
		feedback.UpdatedAt = now

		_, err = s.db.ExecContext(ctx, `
			UPDATE feedback SET
				variant = ?,
				suggested_classification = ?,
				user_classification = ?,
				user_agreed = ?,
				evidence_summary = ?,
				notes = ?,
				updated_at = ?
			WHERE id = ?
		`,
			feedback.Variant,
			string(feedback.SuggestedClassification),
			string(feedback.UserClassification),
			feedback.UserAgreed,
			feedback.EvidenceSummary,
			feedback.Notes,
			now,
			existingID,
		)
		return err
	}

	if err != sql.ErrNoRows {
		return fmt.Errorf("failed to check existing: %w", err)
	}

	// Insert new
	feedback.CreatedAt = now
	feedback.UpdatedAt = now

	result, err := s.db.ExecContext(ctx, `
		INSERT INTO feedback (
			variant, normalized_hgvs, cancer_type,
			suggested_classification, user_classification, user_agreed,
			evidence_summary, notes, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
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
	)
	if err != nil {
		return fmt.Errorf("failed to insert: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get insert ID: %w", err)
	}
	feedback.ID = id

	return nil
}

// Get retrieves the most recent feedback for a variant.
func (s *SQLiteStore) Get(ctx context.Context, normalizedHGVS string, cancerType string) (*Feedback, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, variant, normalized_hgvs, cancer_type,
			suggested_classification, user_classification, user_agreed,
			evidence_summary, notes, created_at, updated_at
		FROM feedback
		WHERE normalized_hgvs = ? AND cancer_type = ?
		LIMIT 1
	`, normalizedHGVS, cancerType)

	fb, err := scanFeedback(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to scan: %w", err)
	}
	return fb, nil
}

// List returns all feedback entries with pagination.
func (s *SQLiteStore) List(ctx context.Context, limit, offset int) ([]*Feedback, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, variant, normalized_hgvs, cancer_type,
			suggested_classification, user_classification, user_agreed,
			evidence_summary, notes, created_at, updated_at
		FROM feedback
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query: %w", err)
	}
	defer rows.Close()

	var result []*Feedback
	for rows.Next() {
		fb, err := scanFeedback(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		result = append(result, fb)
	}
	return result, rows.Err()
}

// Count returns the total number of feedback entries.
func (s *SQLiteStore) Count(ctx context.Context) (int64, error) {
	var count int64
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM feedback").Scan(&count)
	return count, err
}

// Delete removes a feedback entry by ID.
func (s *SQLiteStore) Delete(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM feedback WHERE id = ?", id)
	return err
}

// maxExportLimit is the maximum number of entries to export at once.
const maxExportLimit = 1000000

// ExportJSON exports all feedback to a JSON writer.
func (s *SQLiteStore) ExportJSON(ctx context.Context, writer io.Writer) error {
	all, err := s.List(ctx, maxExportLimit, 0)
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
func (s *SQLiteStore) ImportJSON(ctx context.Context, reader io.Reader) (imported int, skipped int, err error) {
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
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}
