package hgvs

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/acmg-amp-mcp-server/internal/domain"
)

// HGVS notation patterns for validation
var (
	// Genomic HGVS pattern: NC_000017.11:g.43104261G>T
	genomicPattern = regexp.MustCompile(`^(NC_\d+\.\d+|chr\d+|chr[XY]):g\.(\d+)([ATCG]+)>([ATCG]+)$`)

	// Coding HGVS pattern: NM_000059.3:c.274G>T
	codingPattern = regexp.MustCompile(`^(NM_\d+\.\d+):c\.(\d+)([ATCG]+)>([ATCG]+)$`)

	// Protein HGVS pattern: NP_000050.2:p.Gly92Cys
	proteinPattern = regexp.MustCompile(`^(NP_\d+\.\d+):p\.([A-Z][a-z]{2})(\d+)([A-Z][a-z]{2})$`)

	// Gene symbol pattern
	geneSymbolPattern = regexp.MustCompile(`^[A-Z][A-Z0-9-]*$`)

	// Transcript ID pattern
	transcriptPattern = regexp.MustCompile(`^(NM_|NR_|XM_|XR_)\d+\.\d+$`)
)

// Validator provides HGVS validation functionality
type Validator struct{}

// NewValidator creates a new HGVS validator
func NewValidator() *Validator {
	return &Validator{}
}

// ValidateHGVS validates HGVS notation format
func (v *Validator) ValidateHGVS(hgvs string) error {
	if hgvs == "" {
		return domain.NewValidationError("hgvs", "HGVS notation cannot be empty", hgvs)
	}

	hgvs = strings.TrimSpace(hgvs)

	// Check for genomic notation
	if strings.Contains(hgvs, ":g.") {
		if !genomicPattern.MatchString(hgvs) {
			return domain.NewValidationError("hgvs", "Invalid genomic HGVS notation format", hgvs)
		}
		return nil
	}

	// Check for coding notation
	if strings.Contains(hgvs, ":c.") {
		if !codingPattern.MatchString(hgvs) {
			return domain.NewValidationError("hgvs", "Invalid coding HGVS notation format", hgvs)
		}
		return nil
	}

	// Check for protein notation
	if strings.Contains(hgvs, ":p.") {
		if !proteinPattern.MatchString(hgvs) {
			return domain.NewValidationError("hgvs", "Invalid protein HGVS notation format", hgvs)
		}
		return nil
	}

	return domain.NewValidationError("hgvs", "Unrecognized HGVS notation format", hgvs)
}

// ValidateGeneSymbol validates gene symbol format
func (v *Validator) ValidateGeneSymbol(symbol string) error {
	if symbol == "" {
		return nil // Gene symbol is optional
	}

	if !geneSymbolPattern.MatchString(symbol) {
		return domain.NewValidationError("gene_symbol", "Invalid gene symbol format", symbol)
	}

	return nil
}

// ValidateTranscript validates transcript ID format
func (v *Validator) ValidateTranscript(transcript string) error {
	if transcript == "" {
		return nil // Transcript is optional
	}

	if !transcriptPattern.MatchString(transcript) {
		return domain.NewValidationError("transcript", "Invalid transcript ID format", transcript)
	}

	return nil
}

// ValidateVariantRequest validates a complete variant request
func (v *Validator) ValidateVariantRequest(req *domain.VariantRequest) []error {
	var errors []error

	// Validate required fields
	if req.HGVS == "" {
		errors = append(errors, domain.NewValidationError("hgvs", "HGVS notation is required", req.HGVS))
	} else {
		if err := v.ValidateHGVS(req.HGVS); err != nil {
			errors = append(errors, err)
		}
	}

	if req.ClientID == "" {
		errors = append(errors, domain.NewValidationError("client_id", "Client ID is required", req.ClientID))
	}

	if req.RequestID == "" {
		errors = append(errors, domain.NewValidationError("request_id", "Request ID is required", req.RequestID))
	}

	// Validate optional fields
	if err := v.ValidateGeneSymbol(req.GeneSymbol); err != nil {
		errors = append(errors, err)
	}

	if err := v.ValidateTranscript(req.Transcript); err != nil {
		errors = append(errors, err)
	}

	return errors
}

// ParseHGVSComponents extracts components from HGVS notation
func (v *Validator) ParseHGVSComponents(hgvs string) (*HGVSComponents, error) {
	if err := v.ValidateHGVS(hgvs); err != nil {
		return nil, err
	}

	components := &HGVSComponents{
		Original: hgvs,
	}

	// Parse genomic notation
	if matches := genomicPattern.FindStringSubmatch(hgvs); matches != nil {
		components.Type = "genomic"
		components.Reference = matches[1]
		components.Position = matches[2]
		components.RefAllele = matches[3]
		components.AltAllele = matches[4]
		return components, nil
	}

	// Parse coding notation
	if matches := codingPattern.FindStringSubmatch(hgvs); matches != nil {
		components.Type = "coding"
		components.Reference = matches[1]
		components.Position = matches[2]
		components.RefAllele = matches[3]
		components.AltAllele = matches[4]
		return components, nil
	}

	// Parse protein notation
	if matches := proteinPattern.FindStringSubmatch(hgvs); matches != nil {
		components.Type = "protein"
		components.Reference = matches[1]
		components.Position = matches[3]
		components.RefAllele = matches[2]
		components.AltAllele = matches[4]
		return components, nil
	}

	return nil, fmt.Errorf("unable to parse HGVS notation: %s", hgvs)
}

// HGVSComponents represents parsed HGVS notation components
type HGVSComponents struct {
	Original  string `json:"original"`
	Type      string `json:"type"`      // genomic, coding, protein
	Reference string `json:"reference"` // NC_000017.11, NM_000059.3, etc.
	Position  string `json:"position"`  // position in the sequence
	RefAllele string `json:"ref_allele"`
	AltAllele string `json:"alt_allele"`
}
