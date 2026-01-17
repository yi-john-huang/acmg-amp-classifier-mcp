package domain

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// TranscriptInfo represents transcript information for gene resolution
type TranscriptInfo struct {
	RefSeqID    string
	GeneSymbol  string
	Source      string
	LastUpdated time.Time
}

// GeneTranscriptResolver interface for resolving gene symbols to transcripts
type GeneTranscriptResolver interface {
	ResolveGeneToTranscript(ctx context.Context, geneSymbol string) (*TranscriptInfo, error)
}

// StandardInputParser implements the InputParser interface for HGVS notation and gene symbol parsing
type StandardInputParser struct {
	hgvsPattern           *regexp.Regexp
	geneSymbolPattern     *regexp.Regexp
	geneWithVariantPattern *regexp.Regexp
	geneWithProteinPattern *regexp.Regexp
	transcriptResolver    GeneTranscriptResolver
}

// NewStandardInputParser creates a new standard input parser
func NewStandardInputParser() InputParser {
	// Basic HGVS pattern - can be enhanced
	hgvsPattern := regexp.MustCompile(`^(NC_|NM_|NP_|NG_|NR_|XM_|XR_)(\d+)\.(\d+):([cgmnrp])\.(.+)$`)
	
	// Gene symbol patterns following HUGO standards
	geneSymbolPattern := regexp.MustCompile(`^[A-Z][A-Z0-9-]*[A-Z0-9]$|^[A-Z]$`) // HUGO gene symbol pattern
	geneWithVariantPattern := regexp.MustCompile(`^([A-Z][A-Z0-9-]*[A-Z0-9]):([cgp]\..+)$`) // BRCA1:c.123A>G
	geneWithProteinPattern := regexp.MustCompile(`^([A-Z][A-Z0-9-]*[A-Z0-9])\s+(p\..+)$`) // TP53 p.R273H
	
	return &StandardInputParser{
		hgvsPattern:           hgvsPattern,
		geneSymbolPattern:     geneSymbolPattern,
		geneWithVariantPattern: geneWithVariantPattern,
		geneWithProteinPattern: geneWithProteinPattern,
		transcriptResolver:    nil, // Will be injected
	}
}

// ParseVariant parses HGVS notation into a StandardizedVariant
func (p *StandardInputParser) ParseVariant(input string) (*StandardizedVariant, error) {
	input = strings.TrimSpace(input)
	
	// Validate HGVS format
	if err := p.ValidateHGVS(input); err != nil {
		return nil, fmt.Errorf("invalid HGVS notation: %w", err)
	}
	
	// Parse HGVS components
	matches := p.hgvsPattern.FindStringSubmatch(input)
	if len(matches) < 6 {
		return nil, fmt.Errorf("failed to parse HGVS notation: %s", input)
	}
	
	prefix := matches[1]
	accession := matches[2]
	version := matches[3]
	sequenceType := matches[4]
	variation := matches[5]
	
	// Create variant
	variant := &StandardizedVariant{
		ID:           generateVariantID(input),
		HGVSGenomic:  input,
		VariantType:  determineVariantType(variation),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	
	// Set sequence-specific fields based on type
	switch sequenceType {
	case "c":
		variant.HGVSCoding = input
		variant.TranscriptID = fmt.Sprintf("%s%s.%s", prefix, accession, version)
	case "p":
		variant.HGVSProtein = input
	case "g":
		variant.HGVSGenomic = input
		// Extract position and chromosome if possible
		if pos, chr := extractGenomicPosition(variation); pos > 0 {
			variant.Position = pos
			variant.Chromosome = chr
		}
	}
	
	// Extract gene symbol if available (simplified approach)
	variant.GeneSymbol = extractGeneSymbol(input)
	
	return variant, nil
}

// ValidateHGVS validates HGVS notation format
func (p *StandardInputParser) ValidateHGVS(hgvs string) error {
	if hgvs == "" {
		return fmt.Errorf("HGVS notation cannot be empty")
	}
	
	if !p.hgvsPattern.MatchString(hgvs) {
		return fmt.Errorf("invalid HGVS format: %s", hgvs)
	}
	
	return nil
}

// NormalizeVariant normalizes a variant to standard format
func (p *StandardInputParser) NormalizeVariant(variant *StandardizedVariant) error {
	if variant == nil {
		return fmt.Errorf("variant cannot be nil")
	}
	
	// Normalize HGVS notation (basic implementation)
	if variant.HGVSGenomic != "" {
		variant.HGVSGenomic = strings.TrimSpace(variant.HGVSGenomic)
	}
	
	if variant.HGVSCoding != "" {
		variant.HGVSCoding = strings.TrimSpace(variant.HGVSCoding)
	}
	
	if variant.HGVSProtein != "" {
		variant.HGVSProtein = strings.TrimSpace(variant.HGVSProtein)
	}
	
	// Normalize gene symbol to uppercase
	if variant.GeneSymbol != "" {
		variant.GeneSymbol = strings.ToUpper(strings.TrimSpace(variant.GeneSymbol))
	}
	
	variant.UpdatedAt = time.Now()
	
	return nil
}

// Helper functions

func generateVariantID(hgvs string) string {
	// Simple ID generation - in production, use proper normalization and hashing
	return fmt.Sprintf("VAR_%d", time.Now().UnixNano()%1000000)
}

func determineVariantType(variation string) VariantType {
	// Simplified variant type detection
	variation = strings.ToLower(variation)
	
	if strings.Contains(variation, "del") || strings.Contains(variation, "ins") || strings.Contains(variation, "dup") {
		return GERMLINE // Default for structural variants
	}
	
	return GERMLINE // Default to germline
}

func extractGenomicPosition(variation string) (int64, string) {
	// Extract position from genomic notation (simplified)
	// Example: g.123456A>C -> position 123456
	
	re := regexp.MustCompile(`(\d+)`)
	matches := re.FindStringSubmatch(variation)
	
	if len(matches) > 0 {
		if pos, err := strconv.ParseInt(matches[0], 10, 64); err == nil {
			return pos, "1" // Default to chromosome 1 - would need better parsing
		}
	}
	
	return 0, ""
}

func extractGeneSymbol(hgvs string) string {
	// Enhanced gene symbol extraction from HGVS notation
	// This will be replaced by more sophisticated transcript mapping
	if strings.Contains(hgvs, "NM_") {
		// Extract from common transcript patterns
		commonTranscripts := map[string]string{
			"NM_007294": "BRCA1",
			"NM_000546": "TP53",
			"NM_000059": "BRCA2",
			"NM_000492": "CFTR",
		}
		
		for transcript, gene := range commonTranscripts {
			if strings.Contains(hgvs, transcript) {
				return gene
			}
		}
	}
	return ""
}

// ParseGeneSymbol parses gene symbol notation into a StandardizedVariant
func (p *StandardInputParser) ParseGeneSymbol(input string) (*StandardizedVariant, error) {
	input = strings.TrimSpace(input)
	
	// Detect input format and dispatch to appropriate parser
	if p.isHGVSFormat(input) {
		return p.ParseVariant(input)
	}
	
	// Try different gene symbol formats
	if matches := p.geneWithVariantPattern.FindStringSubmatch(input); len(matches) == 3 {
		return p.parseGeneWithVariant(matches[1], matches[2])
	}
	
	if matches := p.geneWithProteinPattern.FindStringSubmatch(input); len(matches) == 3 {
		return p.parseGeneWithProtein(matches[1], matches[2])
	}
	
	if p.geneSymbolPattern.MatchString(input) {
		return p.parseStandaloneGene(input)
	}
	
	return nil, fmt.Errorf("unrecognized input format: %s", input)
}

// ValidateGeneSymbol validates gene symbols according to HUGO standards
func (p *StandardInputParser) ValidateGeneSymbol(symbol string) error {
	symbol = strings.TrimSpace(strings.ToUpper(symbol))
	
	if symbol == "" {
		return fmt.Errorf("gene symbol cannot be empty")
	}
	
	if !p.geneSymbolPattern.MatchString(symbol) {
		return fmt.Errorf("invalid gene symbol format: %s. Must follow HUGO standards (e.g., BRCA1, TP53)", symbol)
	}
	
	// Additional HUGO validation rules
	if len(symbol) > 15 {
		return fmt.Errorf("gene symbol too long: %s. HUGO symbols are typically 1-15 characters", symbol)
	}
	
	// Check for common invalid patterns
	if strings.HasPrefix(symbol, "-") || strings.HasSuffix(symbol, "-") {
		return fmt.Errorf("gene symbol cannot start or end with hyphen: %s", symbol)
	}
	
	return nil
}

// GenerateHGVSFromGeneSymbol generates HGVS notation from gene symbol and variant
func (p *StandardInputParser) GenerateHGVSFromGeneSymbol(geneSymbol, variant string) (string, error) {
	if p.transcriptResolver == nil {
		return "", fmt.Errorf("transcript resolver not available for HGVS generation")
	}
	
	geneSymbol = strings.TrimSpace(strings.ToUpper(geneSymbol))
	variant = strings.TrimSpace(variant)
	
	if err := p.ValidateGeneSymbol(geneSymbol); err != nil {
		return "", fmt.Errorf("invalid gene symbol: %w", err)
	}
	
	// Get canonical transcript for the gene
	ctx := context.Background()
	transcript, err := p.transcriptResolver.ResolveGeneToTranscript(ctx, geneSymbol)
	if err != nil {
		return "", fmt.Errorf("failed to resolve transcript for gene %s: %w", geneSymbol, err)
	}
	
	// Generate HGVS notation
	var hgvs string
	if strings.HasPrefix(variant, "c.") {
		hgvs = fmt.Sprintf("%s:%s", transcript.RefSeqID, variant)
	} else if strings.HasPrefix(variant, "p.") {
		// For protein variants, we need the protein RefSeq ID
		// For now, assume we can derive it from mRNA RefSeq
		proteinID := strings.Replace(transcript.RefSeqID, "NM_", "NP_", 1)
		hgvs = fmt.Sprintf("%s:%s", proteinID, variant)
	} else {
		// Try to infer the sequence type
		if containsCodonOrAminoAcid(variant) {
			hgvs = fmt.Sprintf("%s:p.%s", transcript.RefSeqID, variant)
		} else {
			hgvs = fmt.Sprintf("%s:c.%s", transcript.RefSeqID, variant)
		}
	}
	
	return hgvs, nil
}

// Helper methods for gene symbol parsing

func (p *StandardInputParser) isHGVSFormat(input string) bool {
	return p.hgvsPattern.MatchString(input)
}

func (p *StandardInputParser) parseGeneWithVariant(geneSymbol, variant string) (*StandardizedVariant, error) {
	// Parse format like "BRCA1:c.123A>G"
	if err := p.ValidateGeneSymbol(geneSymbol); err != nil {
		return nil, err
	}
	
	// Generate HGVS and parse it
	hgvs, err := p.GenerateHGVSFromGeneSymbol(geneSymbol, variant)
	if err != nil {
		return nil, fmt.Errorf("failed to generate HGVS: %w", err)
	}
	
	// Parse the generated HGVS
	standardVariant, err := p.ParseVariant(hgvs)
	if err != nil {
		return nil, fmt.Errorf("failed to parse generated HGVS: %w", err)
	}
	
	// Ensure gene symbol is set
	standardVariant.GeneSymbol = geneSymbol
	
	return standardVariant, nil
}

func (p *StandardInputParser) parseGeneWithProtein(geneSymbol, proteinVariant string) (*StandardizedVariant, error) {
	// Parse format like "TP53 p.R273H"
	if err := p.ValidateGeneSymbol(geneSymbol); err != nil {
		return nil, err
	}
	
	// Generate HGVS and parse it
	hgvs, err := p.GenerateHGVSFromGeneSymbol(geneSymbol, proteinVariant)
	if err != nil {
		return nil, fmt.Errorf("failed to generate HGVS: %w", err)
	}
	
	// Parse the generated HGVS
	standardVariant, err := p.ParseVariant(hgvs)
	if err != nil {
		return nil, fmt.Errorf("failed to parse generated HGVS: %w", err)
	}
	
	// Ensure gene symbol and protein notation are set
	standardVariant.GeneSymbol = geneSymbol
	standardVariant.HGVSProtein = hgvs
	
	return standardVariant, nil
}

func (p *StandardInputParser) parseStandaloneGene(geneSymbol string) (*StandardizedVariant, error) {
	// Parse format like "BRCA1" - returns variant with just gene info
	if err := p.ValidateGeneSymbol(geneSymbol); err != nil {
		return nil, err
	}
	
	variant := &StandardizedVariant{
		ID:         generateVariantID(fmt.Sprintf("GENE_%s", geneSymbol)),
		GeneSymbol: geneSymbol,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	
	// Try to resolve transcript information if resolver is available
	if p.transcriptResolver != nil {
		ctx := context.Background()
		if transcript, err := p.transcriptResolver.ResolveGeneToTranscript(ctx, geneSymbol); err == nil {
			variant.TranscriptID = transcript.RefSeqID
		}
	}
	
	return variant, nil
}

func containsCodonOrAminoAcid(variant string) bool {
	// Simple heuristic to detect protein-level variants
	aminoAcids := []string{"Ala", "Arg", "Asn", "Asp", "Cys", "Gln", "Glu", "Gly", "His", "Ile", 
		"Leu", "Lys", "Met", "Phe", "Pro", "Ser", "Thr", "Trp", "Tyr", "Val", "*"}
	
	variant = strings.ToUpper(variant)
	for _, aa := range aminoAcids {
		if strings.Contains(variant, strings.ToUpper(aa)) {
			return true
		}
	}
	return false
}

// SetTranscriptResolver allows injection of transcript resolver
func (p *StandardInputParser) SetTranscriptResolver(resolver GeneTranscriptResolver) {
	p.transcriptResolver = resolver
}