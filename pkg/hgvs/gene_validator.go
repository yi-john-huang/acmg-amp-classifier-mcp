package hgvs

import (
	"regexp"
	"strings"

	"github.com/acmg-amp-mcp-server/internal/domain"
)

// Enhanced gene validation patterns
var (
	// Standard gene symbol pattern (HUGO Gene Nomenclature Committee standards)
	standardGenePattern = regexp.MustCompile(`^[A-Z][A-Z0-9-]*[A-Z0-9]$`)

	// Alternative gene symbol patterns for special cases
	singleLetterGenePattern = regexp.MustCompile(`^[A-Z]$`)

	// Complex gene patterns (includes pseudogenes, antisense, etc.)
	complexGenePattern = regexp.MustCompile(`^[A-Z][A-Z0-9-]*[A-Z0-9](P\d+|AS\d+|DT|IT\d+|NB)?$`)

	// Transcript patterns
	refSeqTranscriptPattern  = regexp.MustCompile(`^(NM_|NR_|XM_|XR_)\d+\.\d+$`)
	ensemblTranscriptPattern = regexp.MustCompile(`^ENST\d{11}\.\d+$`)

	// Gene ID patterns
	entrezGeneIDPattern  = regexp.MustCompile(`^\d+$`)
	ensemblGeneIDPattern = regexp.MustCompile(`^ENSG\d{11}\.\d+$`)
	hgncIDPattern        = regexp.MustCompile(`^HGNC:\d+$`)
)

// GeneValidator provides enhanced gene symbol and transcript validation
type GeneValidator struct {
	// Known gene symbols for validation (could be loaded from external source)
	knownGenes map[string]bool
	// Known transcripts for validation
	knownTranscripts map[string]bool
}

// NewGeneValidator creates a new gene validator
func NewGeneValidator() *GeneValidator {
	return &GeneValidator{
		knownGenes:       make(map[string]bool),
		knownTranscripts: make(map[string]bool),
	}
}

// ValidateGeneSymbol validates gene symbols according to HUGO standards
func (gv *GeneValidator) ValidateGeneSymbol(symbol string) error {
	if symbol == "" {
		return nil // Gene symbol is optional
	}

	originalSymbol := symbol
	symbol = strings.TrimSpace(symbol)

	// First check if the symbol is already in uppercase (HUGO standard)
	if symbol != strings.ToUpper(symbol) {
		return domain.NewValidationError("gene_symbol",
			"Gene symbol must be in uppercase letters according to HUGO standards",
			originalSymbol)
	}

	// Check basic format
	if !gv.isValidGeneFormat(symbol) {
		return domain.NewValidationError("gene_symbol",
			"Gene symbol must follow HUGO nomenclature standards (uppercase letters, numbers, and hyphens only)",
			originalSymbol)
	}

	// Additional validation rules
	if err := gv.validateGeneNamingRules(symbol); err != nil {
		return err
	}

	return nil
}

// ValidateTranscript validates transcript IDs from various databases
func (gv *GeneValidator) ValidateTranscript(transcript string) error {
	if transcript == "" {
		return nil // Transcript is optional
	}

	transcript = strings.TrimSpace(transcript)

	// Check RefSeq transcript pattern
	if refSeqTranscriptPattern.MatchString(transcript) {
		return gv.validateRefSeqTranscript(transcript)
	}

	// Check Ensembl transcript pattern
	if ensemblTranscriptPattern.MatchString(transcript) {
		return gv.validateEnsemblTranscript(transcript)
	}

	return domain.NewValidationError("transcript",
		"Transcript ID must be a valid RefSeq (NM_/NR_/XM_/XR_) or Ensembl (ENST) identifier",
		transcript)
}

// ValidateGeneID validates various gene ID formats
func (gv *GeneValidator) ValidateGeneID(geneID string) error {
	if geneID == "" {
		return nil // Gene ID is optional
	}

	geneID = strings.TrimSpace(geneID)

	// Check Entrez Gene ID
	if entrezGeneIDPattern.MatchString(geneID) {
		return nil
	}

	// Check Ensembl Gene ID
	if ensemblGeneIDPattern.MatchString(geneID) {
		return nil
	}

	// Check HGNC ID
	if hgncIDPattern.MatchString(geneID) {
		return nil
	}

	return domain.NewValidationError("gene_id",
		"Gene ID must be a valid Entrez, Ensembl (ENSG), or HGNC identifier",
		geneID)
}

// ValidateGeneTranscriptPair validates that gene symbol and transcript are consistent
func (gv *GeneValidator) ValidateGeneTranscriptPair(geneSymbol, transcript string) error {
	if geneSymbol == "" || transcript == "" {
		return nil // Skip validation if either is missing
	}

	// Validate individual components first
	if err := gv.ValidateGeneSymbol(geneSymbol); err != nil {
		return err
	}

	if err := gv.ValidateTranscript(transcript); err != nil {
		return err
	}

	// Additional consistency checks could be added here
	// For example, checking if the transcript actually belongs to the gene
	// This would require external database integration

	return nil
}

// isValidGeneFormat checks basic gene symbol format
func (gv *GeneValidator) isValidGeneFormat(symbol string) bool {
	// Check for single letter genes (A, B, C, etc.)
	if singleLetterGenePattern.MatchString(symbol) {
		return true
	}

	// Check for standard gene pattern
	if standardGenePattern.MatchString(symbol) {
		return true
	}

	// Check for complex gene pattern (pseudogenes, antisense, etc.)
	if complexGenePattern.MatchString(symbol) {
		return true
	}

	return false
}

// validateGeneNamingRules enforces additional HUGO naming rules
func (gv *GeneValidator) validateGeneNamingRules(symbol string) error {
	// Gene symbols should not start with numbers
	if len(symbol) > 0 && symbol[0] >= '0' && symbol[0] <= '9' {
		return domain.NewValidationError("gene_symbol",
			"Gene symbol cannot start with a number",
			symbol)
	}

	// Gene symbols should not end with hyphen
	if strings.HasSuffix(symbol, "-") {
		return domain.NewValidationError("gene_symbol",
			"Gene symbol cannot end with a hyphen",
			symbol)
	}

	// Gene symbols should not have consecutive hyphens
	if strings.Contains(symbol, "--") {
		return domain.NewValidationError("gene_symbol",
			"Gene symbol cannot contain consecutive hyphens",
			symbol)
	}

	// Validate length (HUGO recommends 1-15 characters)
	if len(symbol) > 15 {
		return domain.NewValidationError("gene_symbol",
			"Gene symbol should not exceed 15 characters",
			symbol)
	}

	return nil
}

// validateRefSeqTranscript validates RefSeq transcript IDs
func (gv *GeneValidator) validateRefSeqTranscript(transcript string) error {
	// Extract prefix and validate
	if strings.HasPrefix(transcript, "NM_") {
		// Coding transcript
		return nil
	} else if strings.HasPrefix(transcript, "NR_") {
		// Non-coding transcript
		return nil
	} else if strings.HasPrefix(transcript, "XM_") {
		// Predicted coding transcript
		return nil
	} else if strings.HasPrefix(transcript, "XR_") {
		// Predicted non-coding transcript
		return nil
	}

	return domain.NewValidationError("transcript",
		"Invalid RefSeq transcript prefix",
		transcript)
}

// validateEnsemblTranscript validates Ensembl transcript IDs
func (gv *GeneValidator) validateEnsemblTranscript(transcript string) error {
	// Ensembl transcript IDs follow pattern ENST00000000000.version
	if !ensemblTranscriptPattern.MatchString(transcript) {
		return domain.NewValidationError("transcript",
			"Invalid Ensembl transcript format",
			transcript)
	}

	return nil
}

// AddKnownGene adds a gene symbol to the known genes list
func (gv *GeneValidator) AddKnownGene(symbol string) {
	if gv.knownGenes == nil {
		gv.knownGenes = make(map[string]bool)
	}
	gv.knownGenes[strings.ToUpper(symbol)] = true
}

// AddKnownTranscript adds a transcript to the known transcripts list
func (gv *GeneValidator) AddKnownTranscript(transcript string) {
	if gv.knownTranscripts == nil {
		gv.knownTranscripts = make(map[string]bool)
	}
	gv.knownTranscripts[transcript] = true
}

// IsKnownGene checks if a gene symbol is in the known genes list
func (gv *GeneValidator) IsKnownGene(symbol string) bool {
	if gv.knownGenes == nil {
		return false
	}
	return gv.knownGenes[strings.ToUpper(symbol)]
}

// IsKnownTranscript checks if a transcript is in the known transcripts list
func (gv *GeneValidator) IsKnownTranscript(transcript string) bool {
	if gv.knownTranscripts == nil {
		return false
	}
	return gv.knownTranscripts[transcript]
}

// ValidateVariantGeneData validates all gene-related fields in a variant request
func (gv *GeneValidator) ValidateVariantGeneData(req *domain.VariantRequest) []error {
	var errors []error

	// Validate gene symbol
	if err := gv.ValidateGeneSymbol(req.GeneSymbol); err != nil {
		errors = append(errors, err)
	}

	// Validate transcript
	if err := gv.ValidateTranscript(req.Transcript); err != nil {
		errors = append(errors, err)
	}

	// Only validate consistency if both individual validations passed
	if req.GeneSymbol != "" && req.Transcript != "" {
		// Additional consistency checks could be added here
		// For example, checking if the transcript actually belongs to the gene
		// This would require external database integration
	}

	return errors
}
