package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/acmg-amp-mcp-server/internal/domain"
	"github.com/acmg-amp-mcp-server/pkg/hgvs"
)

// InputParserService implements the domain.InputParser interface
type InputParserService struct {
	parser             *hgvs.Parser
	validator          *hgvs.Validator
	geneValidator      *hgvs.GeneValidator
	domainParser       *domain.StandardInputParser
	transcriptResolver *CachedTranscriptResolver
}

// NewInputParserService creates a new input parser service
func NewInputParserService() *InputParserService {
	domainParser := domain.NewStandardInputParser().(*domain.StandardInputParser)
	
	return &InputParserService{
		parser:             hgvs.NewParser(),
		validator:          hgvs.NewValidator(),
		geneValidator:      hgvs.NewGeneValidator(),
		domainParser:       domainParser,
		transcriptResolver: nil, // Will be injected
	}
}

// NewInputParserServiceWithTranscriptResolver creates a new input parser service with transcript resolver
func NewInputParserServiceWithTranscriptResolver(transcriptResolver *CachedTranscriptResolver) *InputParserService {
	domainParser := domain.NewStandardInputParser().(*domain.StandardInputParser)
	adapter := NewTranscriptResolverAdapter(transcriptResolver)
	domainParser.SetTranscriptResolver(adapter)
	
	return &InputParserService{
		parser:             hgvs.NewParser(),
		validator:          hgvs.NewValidator(),
		geneValidator:      hgvs.NewGeneValidator(),
		domainParser:       domainParser,
		transcriptResolver: transcriptResolver,
	}
}

// ParseVariant parses and validates HGVS notation, returning a StandardizedVariant
func (ips *InputParserService) ParseVariant(input string) (*domain.StandardizedVariant, error) {
	if input == "" {
		return nil, fmt.Errorf("parsing variant: %w", 
			domain.NewValidationError("hgvs", "HGVS notation cannot be empty", input))
	}

	// Use the parser to parse the variant
	variant, err := ips.parser.ParseVariant(input)
	if err != nil {
		return nil, fmt.Errorf("parsing variant: %w", err)
	}

	// Ensure the variant is normalized
	if err := ips.parser.NormalizeVariant(variant); err != nil {
		return nil, fmt.Errorf("normalizing variant: %w", err)
	}

	return variant, nil
}

// ValidateHGVS validates HGVS notation format
func (ips *InputParserService) ValidateHGVS(hgvs string) error {
	return ips.validator.ValidateHGVS(hgvs)
}

// NormalizeVariant normalizes a variant to a consistent representation
func (ips *InputParserService) NormalizeVariant(variant *domain.StandardizedVariant) error {
	if variant == nil {
		return fmt.Errorf("normalizing variant: variant cannot be nil")
	}

	return ips.parser.NormalizeVariant(variant)
}

// ValidateVariantRequest validates a complete variant request with all components
func (ips *InputParserService) ValidateVariantRequest(req *domain.VariantRequest) []error {
	var allErrors []error

	// Validate required fields and HGVS using the standard validator
	if req.HGVS == "" {
		allErrors = append(allErrors, domain.NewValidationError("hgvs", "HGVS notation is required", req.HGVS))
	} else {
		if err := ips.validator.ValidateHGVS(req.HGVS); err != nil {
			allErrors = append(allErrors, err)
		}
	}

	if req.ClientID == "" {
		allErrors = append(allErrors, domain.NewValidationError("client_id", "Client ID is required", req.ClientID))
	}

	if req.RequestID == "" {
		allErrors = append(allErrors, domain.NewValidationError("request_id", "Request ID is required", req.RequestID))
	}

	// Use the enhanced gene validator for medical-grade gene/transcript validation
	geneErrors := ips.geneValidator.ValidateVariantGeneData(req)
	allErrors = append(allErrors, geneErrors...)

	return allErrors
}

// ParseAndValidateVariantRequest is a comprehensive method that parses and validates a variant request
func (ips *InputParserService) ParseAndValidateVariantRequest(ctx context.Context, req *domain.VariantRequest) (*domain.StandardizedVariant, []error) {
	// First validate the request
	errors := ips.ValidateVariantRequest(req)
	if len(errors) > 0 {
		return nil, errors
	}

	// Parse the variant
	variant, err := ips.ParseVariant(req.HGVS)
	if err != nil {
		errors = append(errors, err)
		return nil, errors
	}

	// Set additional fields from the request
	variant.GeneSymbol = req.GeneSymbol
	variant.TranscriptID = req.Transcript

	// Final normalization
	if err := ips.NormalizeVariant(variant); err != nil {
		errors = append(errors, fmt.Errorf("final normalization: %w", err))
		return nil, errors
	}

	return variant, nil
}

// GetSupportedHGVSFormats returns a list of supported HGVS formats
func (ips *InputParserService) GetSupportedHGVSFormats() []string {
	return []string{
		"Genomic notation: NC_000017.11:g.43104261G>T",
		"Genomic notation (chr): chr17:g.43104261G>T",
		"Genomic deletions: NC_000017.11:g.43104261_43104263del",
		"Genomic insertions: NC_000017.11:g.43104261_43104262insATG",
		"Genomic duplications: NC_000017.11:g.43104261_43104263dup",
		"Genomic inversions: NC_000017.11:g.43104261_43104263inv",
		"Coding notation: NM_000059.3:c.274G>T",
		"Coding deletions: NM_000059.3:c.274_276del",
		"Coding insertions: NM_000059.3:c.274_275insATG",
		"Coding frameshifts: NM_000059.3:c.274delfs",
		"Protein notation: NP_000050.2:p.Gly92Cys",
		"Protein nonsense: NP_000050.2:p.Gly92*",
		"Protein frameshifts: NP_000050.2:p.Gly92Alafs*15",
		"Protein deletions: NP_000050.2:p.Gly92_Ala94del",
	}
}

// ValidateGeneSymbolLegacy validates gene symbols using legacy hgvs.GeneValidator
func (ips *InputParserService) ValidateGeneSymbolLegacy(symbol string) error {
	return ips.geneValidator.ValidateGeneSymbol(symbol)
}

// ValidateTranscript validates transcript IDs
func (ips *InputParserService) ValidateTranscript(transcript string) error {
	return ips.geneValidator.ValidateTranscript(transcript)
}

// ValidateGeneID validates gene IDs
func (ips *InputParserService) ValidateGeneID(geneID string) error {
	return ips.geneValidator.ValidateGeneID(geneID)
}

// AddKnownGenes allows adding known genes for validation
func (ips *InputParserService) AddKnownGenes(genes []string) {
	for _, gene := range genes {
		ips.geneValidator.AddKnownGene(gene)
	}
}

// AddKnownTranscripts allows adding known transcripts for validation
func (ips *InputParserService) AddKnownTranscripts(transcripts []string) {
	for _, transcript := range transcripts {
		ips.geneValidator.AddKnownTranscript(transcript)
	}
}

// ParseGeneSymbol parses gene symbol notation into a StandardizedVariant
func (ips *InputParserService) ParseGeneSymbol(input string) (*domain.StandardizedVariant, error) {
	return ips.domainParser.ParseGeneSymbol(input)
}

// ValidateGeneSymbol validates gene symbols according to HUGO standards
func (ips *InputParserService) ValidateGeneSymbol(symbol string) error {
	return ips.domainParser.ValidateGeneSymbol(symbol)
}

// GenerateHGVSFromGeneSymbol generates HGVS notation from gene symbol and variant
func (ips *InputParserService) GenerateHGVSFromGeneSymbol(geneSymbol, variant string) (string, error) {
	return ips.domainParser.GenerateHGVSFromGeneSymbol(geneSymbol, variant)
}

// SetTranscriptResolver allows injection of transcript resolver after creation
func (ips *InputParserService) SetTranscriptResolver(resolver *CachedTranscriptResolver) {
	ips.transcriptResolver = resolver
	if ips.domainParser != nil {
		adapter := NewTranscriptResolverAdapter(resolver)
		ips.domainParser.SetTranscriptResolver(adapter)
	}
}

// ParseVariantWithGeneSymbolSupport parses input that could be HGVS or gene symbol format
func (ips *InputParserService) ParseVariantWithGeneSymbolSupport(input string) (*domain.StandardizedVariant, error) {
	input = strings.TrimSpace(input)
	
	if input == "" {
		return nil, fmt.Errorf("parsing variant: %w", 
			domain.NewValidationError("input", "Input cannot be empty", input))
	}
	
	// Try parsing as gene symbol first (broader format support)
	if variant, err := ips.ParseGeneSymbol(input); err == nil {
		return variant, nil
	}
	
	// Fallback to standard HGVS parsing
	return ips.ParseVariant(input)
}