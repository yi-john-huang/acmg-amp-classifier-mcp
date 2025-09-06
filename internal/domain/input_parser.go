package domain

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// StandardInputParser implements the InputParser interface for HGVS notation parsing
type StandardInputParser struct {
	hgvsPattern *regexp.Regexp
}

// NewStandardInputParser creates a new standard input parser
func NewStandardInputParser() InputParser {
	// Basic HGVS pattern - can be enhanced
	hgvsPattern := regexp.MustCompile(`^(NC_|NM_|NP_|NG_|NR_|XM_|XR_)(\d+)\.(\d+):([cgmnrp])\.(.+)$`)
	
	return &StandardInputParser{
		hgvsPattern: hgvsPattern,
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
	// Simplified gene symbol extraction - in production, would use transcript/gene mapping
	// For now, just return empty string as this requires database lookup
	return ""
}