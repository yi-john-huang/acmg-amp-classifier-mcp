package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"

	"github.com/acmg-amp-mcp-server/internal/domain"
)

// VariantRepository handles variant data persistence
type VariantRepository struct {
	db  *pgxpool.Pool
	log *logrus.Logger
}

// NewVariantRepository creates a new variant repository
func NewVariantRepository(db *pgxpool.Pool, logger *logrus.Logger) *VariantRepository {
	return &VariantRepository{
		db:  db,
		log: logger,
	}
}

// Create inserts a new variant into the database
func (r *VariantRepository) Create(ctx context.Context, variant *domain.StandardizedVariant) error {
	query := `
		INSERT INTO variants (
			id, hgvs_notation, chromosome, position, reference, alternative, 
			gene_symbol, transcript_id, variant_type
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9
		)`

	_, err := r.db.Exec(ctx, query,
		variant.ID,
		variant.HGVSGenomic,
		variant.Chromosome,
		variant.Position,
		variant.Reference,
		variant.Alternative,
		variant.GeneSymbol,
		variant.TranscriptID,
		variant.VariantType,
	)

	if err != nil {
		r.log.WithFields(logrus.Fields{
			"variant_id": variant.ID,
			"hgvs":       variant.HGVSGenomic,
			"error":      err,
		}).Error("Failed to create variant")
		return fmt.Errorf("creating variant: %w", err)
	}

	r.log.WithFields(logrus.Fields{
		"variant_id": variant.ID,
		"hgvs":       variant.HGVSGenomic,
		"gene":       variant.GeneSymbol,
	}).Info("Variant created successfully")

	return nil
}

// GetByID retrieves a variant by its ID
func (r *VariantRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.StandardizedVariant, error) {
	query := `
		SELECT id, hgvs_notation, chromosome, position, reference, alternative,
			   gene_symbol, transcript_id, variant_type, created_at, updated_at
		FROM variants 
		WHERE id = $1`

	var variant domain.StandardizedVariant
	var createdAt, updatedAt time.Time

	err := r.db.QueryRow(ctx, query, id).Scan(
		&variant.ID,
		&variant.HGVSGenomic,
		&variant.Chromosome,
		&variant.Position,
		&variant.Reference,
		&variant.Alternative,
		&variant.GeneSymbol,
		&variant.TranscriptID,
		&variant.VariantType,
		&createdAt,
		&updatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("variant not found: %w", domain.ErrNotFound)
		}
		r.log.WithFields(logrus.Fields{
			"variant_id": id,
			"error":      err,
		}).Error("Failed to get variant by ID")
		return nil, fmt.Errorf("getting variant by ID: %w", err)
	}

	return &variant, nil
}

// GetByHGVS retrieves a variant by its HGVS notation
func (r *VariantRepository) GetByHGVS(ctx context.Context, hgvs string) (*domain.StandardizedVariant, error) {
	query := `
		SELECT id, hgvs_notation, chromosome, position, reference, alternative,
			   gene_symbol, transcript_id, variant_type, created_at, updated_at
		FROM variants 
		WHERE hgvs_notation = $1`

	var variant domain.StandardizedVariant
	var createdAt, updatedAt time.Time

	err := r.db.QueryRow(ctx, query, hgvs).Scan(
		&variant.ID,
		&variant.HGVSGenomic,
		&variant.Chromosome,
		&variant.Position,
		&variant.Reference,
		&variant.Alternative,
		&variant.GeneSymbol,
		&variant.TranscriptID,
		&variant.VariantType,
		&createdAt,
		&updatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("variant not found: %w", domain.ErrNotFound)
		}
		r.log.WithFields(logrus.Fields{
			"hgvs":  hgvs,
			"error": err,
		}).Error("Failed to get variant by HGVS")
		return nil, fmt.Errorf("getting variant by HGVS: %w", err)
	}

	return &variant, nil
}

// GetByGene retrieves variants by gene symbol with pagination
func (r *VariantRepository) GetByGene(ctx context.Context, geneSymbol string, limit, offset int) ([]*domain.StandardizedVariant, error) {
	query := `
		SELECT id, hgvs_notation, chromosome, position, reference, alternative,
			   gene_symbol, transcript_id, variant_type, created_at, updated_at
		FROM variants 
		WHERE gene_symbol = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.db.Query(ctx, query, geneSymbol, limit, offset)
	if err != nil {
		r.log.WithFields(logrus.Fields{
			"gene_symbol": geneSymbol,
			"error":       err,
		}).Error("Failed to get variants by gene")
		return nil, fmt.Errorf("getting variants by gene: %w", err)
	}
	defer rows.Close()

	var variants []*domain.StandardizedVariant
	for rows.Next() {
		var variant domain.StandardizedVariant
		var createdAt, updatedAt time.Time

		err := rows.Scan(
			&variant.ID,
			&variant.HGVSGenomic,
			&variant.Chromosome,
			&variant.Position,
			&variant.Reference,
			&variant.Alternative,
			&variant.GeneSymbol,
			&variant.TranscriptID,
			&variant.VariantType,
			&createdAt,
			&updatedAt,
		)
		if err != nil {
			r.log.WithFields(logrus.Fields{
				"gene_symbol": geneSymbol,
				"error":       err,
			}).Error("Failed to scan variant row")
			return nil, fmt.Errorf("scanning variant row: %w", err)
		}

		variants = append(variants, &variant)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating variant rows: %w", err)
	}

	return variants, nil
}

// Update updates an existing variant
func (r *VariantRepository) Update(ctx context.Context, variant *domain.StandardizedVariant) error {
	query := `
		UPDATE variants 
		SET hgvs_notation = $2, chromosome = $3, position = $4, reference = $5,
			alternative = $6, gene_symbol = $7, transcript_id = $8, variant_type = $9,
			updated_at = NOW()
		WHERE id = $1`

	result, err := r.db.Exec(ctx, query,
		variant.ID,
		variant.HGVSGenomic,
		variant.Chromosome,
		variant.Position,
		variant.Reference,
		variant.Alternative,
		variant.GeneSymbol,
		variant.TranscriptID,
		variant.VariantType,
	)

	if err != nil {
		r.log.WithFields(logrus.Fields{
			"variant_id": variant.ID,
			"error":      err,
		}).Error("Failed to update variant")
		return fmt.Errorf("updating variant: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("variant not found: %w", domain.ErrNotFound)
	}

	r.log.WithFields(logrus.Fields{
		"variant_id": variant.ID,
		"hgvs":       variant.HGVSGenomic,
	}).Info("Variant updated successfully")

	return nil
}

// Delete removes a variant from the database
func (r *VariantRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM variants WHERE id = $1`

	result, err := r.db.Exec(ctx, query, id)
	if err != nil {
		r.log.WithFields(logrus.Fields{
			"variant_id": id,
			"error":      err,
		}).Error("Failed to delete variant")
		return fmt.Errorf("deleting variant: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("variant not found: %w", domain.ErrNotFound)
	}

	r.log.WithFields(logrus.Fields{
		"variant_id": id,
	}).Info("Variant deleted successfully")

	return nil
}
