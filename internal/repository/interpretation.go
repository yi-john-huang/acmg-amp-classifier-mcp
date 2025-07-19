package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"

	"github.com/acmg-amp-mcp-server/internal/domain"
)

// InterpretationRepository handles interpretation data persistence
type InterpretationRepository struct {
	db  *pgxpool.Pool
	log *logrus.Logger
}

// NewInterpretationRepository creates a new interpretation repository
func NewInterpretationRepository(db *pgxpool.Pool, logger *logrus.Logger) *InterpretationRepository {
	return &InterpretationRepository{
		db:  db,
		log: logger,
	}
}

// Create inserts a new interpretation into the database
func (r *InterpretationRepository) Create(ctx context.Context, interpretation *domain.InterpretationRecord) error {
	// Marshal JSONB fields
	appliedRulesJSON, err := json.Marshal(interpretation.AppliedRules)
	if err != nil {
		return fmt.Errorf("marshaling applied rules: %w", err)
	}

	evidenceSummaryJSON, err := json.Marshal(interpretation.EvidenceSummary)
	if err != nil {
		return fmt.Errorf("marshaling evidence summary: %w", err)
	}

	reportDataJSON, err := json.Marshal(interpretation.ReportData)
	if err != nil {
		return fmt.Errorf("marshaling report data: %w", err)
	}

	query := `
		INSERT INTO interpretations (
			id, variant_id, classification, confidence_level, applied_rules,
			evidence_summary, report_data, processing_time_ms, client_id, request_id
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10
		)`

	_, err = r.db.Exec(ctx, query,
		interpretation.ID,
		interpretation.VariantID,
		interpretation.Classification,
		interpretation.ConfidenceLevel,
		appliedRulesJSON,
		evidenceSummaryJSON,
		reportDataJSON,
		interpretation.ProcessingTimeMS,
		interpretation.ClientID,
		interpretation.RequestID,
	)

	if err != nil {
		r.log.WithFields(logrus.Fields{
			"interpretation_id": interpretation.ID,
			"variant_id":        interpretation.VariantID,
			"classification":    interpretation.Classification,
			"error":             err,
		}).Error("Failed to create interpretation")
		return fmt.Errorf("creating interpretation: %w", err)
	}

	r.log.WithFields(logrus.Fields{
		"interpretation_id": interpretation.ID,
		"variant_id":        interpretation.VariantID,
		"classification":    interpretation.Classification,
		"confidence":        interpretation.ConfidenceLevel,
		"processing_time":   interpretation.ProcessingTimeMs,
	}).Info("Interpretation created successfully")

	return nil
}

// GetByID retrieves an interpretation by its ID
func (r *InterpretationRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.InterpretationRecord, error) {
	query := `
		SELECT id, variant_id, classification, confidence_level, applied_rules,
			   evidence_summary, report_data, processing_time_ms, client_id, 
			   request_id, created_at, updated_at
		FROM interpretations 
		WHERE id = $1`

	var interpretation domain.InterpretationRecord
	var appliedRulesJSON, evidenceSummaryJSON, reportDataJSON []byte
	var createdAt, updatedAt time.Time

	err := r.db.QueryRow(ctx, query, id).Scan(
		&interpretation.ID,
		&interpretation.VariantID,
		&interpretation.Classification,
		&interpretation.ConfidenceLevel,
		&appliedRulesJSON,
		&evidenceSummaryJSON,
		&reportDataJSON,
		&interpretation.ProcessingTimeMS,
		&interpretation.ClientID,
		&interpretation.RequestID,
		&createdAt,
		&updatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("interpretation not found: %w", domain.ErrNotFound)
		}
		r.log.WithFields(logrus.Fields{
			"interpretation_id": id,
			"error":             err,
		}).Error("Failed to get interpretation by ID")
		return nil, fmt.Errorf("getting interpretation by ID: %w", err)
	}

	// Unmarshal JSONB fields
	if err := json.Unmarshal(appliedRulesJSON, &interpretation.AppliedRules); err != nil {
		return nil, fmt.Errorf("unmarshaling applied rules: %w", err)
	}

	if err := json.Unmarshal(evidenceSummaryJSON, &interpretation.EvidenceSummary); err != nil {
		return nil, fmt.Errorf("unmarshaling evidence summary: %w", err)
	}

	if err := json.Unmarshal(reportDataJSON, &interpretation.ReportData); err != nil {
		return nil, fmt.Errorf("unmarshaling report data: %w", err)
	}

	interpretation.CreatedAt = createdAt
	interpretation.UpdatedAt = updatedAt

	return &interpretation, nil
}

// GetByVariantID retrieves interpretations for a specific variant
func (r *InterpretationRepository) GetByVariantID(ctx context.Context, variantID uuid.UUID, limit, offset int) ([]*domain.InterpretationRecord, error) {
	query := `
		SELECT id, variant_id, classification, confidence_level, applied_rules,
			   evidence_summary, report_data, processing_time_ms, client_id, 
			   request_id, created_at, updated_at
		FROM interpretations 
		WHERE variant_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.db.Query(ctx, query, variantID, limit, offset)
	if err != nil {
		r.log.WithFields(logrus.Fields{
			"variant_id": variantID,
			"error":      err,
		}).Error("Failed to get interpretations by variant ID")
		return nil, fmt.Errorf("getting interpretations by variant ID: %w", err)
	}
	defer rows.Close()

	var interpretations []*domain.InterpretationRecord
	for rows.Next() {
		var interpretation domain.InterpretationRecord
		var appliedRulesJSON, evidenceSummaryJSON, reportDataJSON []byte
		var createdAt, updatedAt time.Time

		err := rows.Scan(
			&interpretation.ID,
			&interpretation.VariantID,
			&interpretation.Classification,
			&interpretation.ConfidenceLevel,
			&appliedRulesJSON,
			&evidenceSummaryJSON,
			&reportDataJSON,
			&interpretation.ProcessingTimeMS,
			&interpretation.ClientID,
			&interpretation.RequestID,
			&createdAt,
			&updatedAt,
		)
		if err != nil {
			r.log.WithFields(logrus.Fields{
				"variant_id": variantID,
				"error":      err,
			}).Error("Failed to scan interpretation row")
			return nil, fmt.Errorf("scanning interpretation row: %w", err)
		}

		// Unmarshal JSONB fields
		if err := json.Unmarshal(appliedRulesJSON, &interpretation.AppliedRules); err != nil {
			return nil, fmt.Errorf("unmarshaling applied rules: %w", err)
		}

		if err := json.Unmarshal(evidenceSummaryJSON, &interpretation.EvidenceSummary); err != nil {
			return nil, fmt.Errorf("unmarshaling evidence summary: %w", err)
		}

		if err := json.Unmarshal(reportDataJSON, &interpretation.ReportData); err != nil {
			return nil, fmt.Errorf("unmarshaling report data: %w", err)
		}

		interpretation.CreatedAt = createdAt
		interpretation.UpdatedAt = updatedAt

		interpretations = append(interpretations, &interpretation)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating interpretation rows: %w", err)
	}

	return interpretations, nil
}

// GetByClassification retrieves interpretations by classification with pagination
func (r *InterpretationRepository) GetByClassification(ctx context.Context, classification domain.Classification, limit, offset int) ([]*domain.InterpretationRecord, error) {
	query := `
		SELECT id, variant_id, classification, confidence_level, applied_rules,
			   evidence_summary, report_data, processing_time_ms, client_id, 
			   request_id, created_at, updated_at
		FROM interpretations 
		WHERE classification = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.db.Query(ctx, query, classification, limit, offset)
	if err != nil {
		r.log.WithFields(logrus.Fields{
			"classification": classification,
			"error":          err,
		}).Error("Failed to get interpretations by classification")
		return nil, fmt.Errorf("getting interpretations by classification: %w", err)
	}
	defer rows.Close()

	var interpretations []*domain.InterpretationRecord
	for rows.Next() {
		var interpretation domain.InterpretationRecord
		var appliedRulesJSON, evidenceSummaryJSON, reportDataJSON []byte
		var createdAt, updatedAt time.Time

		err := rows.Scan(
			&interpretation.ID,
			&interpretation.VariantID,
			&interpretation.Classification,
			&interpretation.ConfidenceLevel,
			&appliedRulesJSON,
			&evidenceSummaryJSON,
			&reportDataJSON,
			&interpretation.ProcessingTimeMS,
			&interpretation.ClientID,
			&interpretation.RequestID,
			&createdAt,
			&updatedAt,
		)
		if err != nil {
			r.log.WithFields(logrus.Fields{
				"classification": classification,
				"error":          err,
			}).Error("Failed to scan interpretation row")
			return nil, fmt.Errorf("scanning interpretation row: %w", err)
		}

		// Unmarshal JSONB fields
		if err := json.Unmarshal(appliedRulesJSON, &interpretation.AppliedRules); err != nil {
			return nil, fmt.Errorf("unmarshaling applied rules: %w", err)
		}

		if err := json.Unmarshal(evidenceSummaryJSON, &interpretation.EvidenceSummary); err != nil {
			return nil, fmt.Errorf("unmarshaling evidence summary: %w", err)
		}

		if err := json.Unmarshal(reportDataJSON, &interpretation.ReportData); err != nil {
			return nil, fmt.Errorf("unmarshaling report data: %w", err)
		}

		interpretation.CreatedAt = createdAt
		interpretation.UpdatedAt = updatedAt

		interpretations = append(interpretations, &interpretation)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating interpretation rows: %w", err)
	}

	return interpretations, nil
}

// Update updates an existing interpretation
func (r *InterpretationRepository) Update(ctx context.Context, interpretation *domain.InterpretationRecord) error {
	// Marshal JSONB fields
	appliedRulesJSON, err := json.Marshal(interpretation.AppliedRules)
	if err != nil {
		return fmt.Errorf("marshaling applied rules: %w", err)
	}

	evidenceSummaryJSON, err := json.Marshal(interpretation.EvidenceSummary)
	if err != nil {
		return fmt.Errorf("marshaling evidence summary: %w", err)
	}

	reportDataJSON, err := json.Marshal(interpretation.ReportData)
	if err != nil {
		return fmt.Errorf("marshaling report data: %w", err)
	}

	query := `
		UPDATE interpretations 
		SET classification = $2, confidence_level = $3, applied_rules = $4,
			evidence_summary = $5, report_data = $6, processing_time_ms = $7,
			client_id = $8, request_id = $9, updated_at = NOW()
		WHERE id = $1`

	result, err := r.db.Exec(ctx, query,
		interpretation.ID,
		interpretation.Classification,
		interpretation.ConfidenceLevel,
		appliedRulesJSON,
		evidenceSummaryJSON,
		reportDataJSON,
		interpretation.ProcessingTimeMS,
		interpretation.ClientID,
		interpretation.RequestID,
	)

	if err != nil {
		r.log.WithFields(logrus.Fields{
			"interpretation_id": interpretation.ID,
			"error":             err,
		}).Error("Failed to update interpretation")
		return fmt.Errorf("updating interpretation: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("interpretation not found: %w", domain.ErrNotFound)
	}

	r.log.WithFields(logrus.Fields{
		"interpretation_id": interpretation.ID,
		"classification":    interpretation.Classification,
	}).Info("Interpretation updated successfully")

	return nil
}

// Delete removes an interpretation from the database
func (r *InterpretationRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM interpretations WHERE id = $1`

	result, err := r.db.Exec(ctx, query, id)
	if err != nil {
		r.log.WithFields(logrus.Fields{
			"interpretation_id": id,
			"error":             err,
		}).Error("Failed to delete interpretation")
		return fmt.Errorf("deleting interpretation: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("interpretation not found: %w", domain.ErrNotFound)
	}

	r.log.WithFields(logrus.Fields{
		"interpretation_id": id,
	}).Info("Interpretation deleted successfully")

	return nil
}
