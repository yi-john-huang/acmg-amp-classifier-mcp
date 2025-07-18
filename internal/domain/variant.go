package domain

import (
	"time"
)

// StandardizedVariant represents a genetic variant in standardized format
type StandardizedVariant struct {
	ID           string      `json:"id" db:"id"`
	Chromosome   string      `json:"chromosome" db:"chromosome"`
	Position     int64       `json:"position" db:"position"`
	Reference    string      `json:"reference" db:"reference"`
	Alternative  string      `json:"alternative" db:"alternative"`
	HGVSGenomic  string      `json:"hgvs_genomic" db:"hgvs_notation"`
	HGVSCoding   string      `json:"hgvs_coding,omitempty" db:"hgvs_coding"`
	HGVSProtein  string      `json:"hgvs_protein,omitempty" db:"hgvs_protein"`
	GeneSymbol   string      `json:"gene_symbol" db:"gene_symbol"`
	GeneID       string      `json:"gene_id,omitempty" db:"gene_id"`
	TranscriptID string      `json:"transcript_id,omitempty" db:"transcript_id"`
	VariantType  VariantType `json:"variant_type" db:"variant_type"`
	CreatedAt    time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time   `json:"updated_at" db:"updated_at"`
}

// VariantRequest represents an incoming variant interpretation request
type VariantRequest struct {
	HGVS        string            `json:"hgvs" binding:"required"`
	GeneSymbol  string            `json:"gene_symbol,omitempty"`
	Transcript  string            `json:"transcript,omitempty"`
	ClientID    string            `json:"client_id" binding:"required"`
	RequestID   string            `json:"request_id" binding:"required"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	RequestedAt time.Time         `json:"requested_at,omitempty"`
}

// VariantResponse represents the response for a variant interpretation request
type VariantResponse struct {
	RequestID      string                `json:"request_id"`
	Variant        *StandardizedVariant  `json:"variant"`
	Classification Classification        `json:"classification"`
	Confidence     ConfidenceLevel       `json:"confidence"`
	Report         *InterpretationReport `json:"report"`
	ProcessingTime string                `json:"processing_time"`
	ProcessedAt    time.Time             `json:"processed_at"`
	Errors         []string              `json:"errors,omitempty"`
}
