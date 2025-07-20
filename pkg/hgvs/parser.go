package hgvs

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/acmg-amp-mcp-server/internal/domain"
)

// Enhanced HGVS patterns supporting more variant types
var (
	// Genomic patterns
	genomicSubstitutionPattern = regexp.MustCompile(`^(NC_\d+\.\d+|chr\d+|chr[XYM]):g\.(\d+)([ATCG]+)>([ATCG]+)$`)
	genomicDeletionPattern     = regexp.MustCompile(`^(NC_\d+\.\d+|chr\d+|chr[XYM]):g\.(\d+)(_(\d+))?del([ATCG]*)$`)
	genomicInsertionPattern    = regexp.MustCompile(`^(NC_\d+\.\d+|chr\d+|chr[XYM]):g\.(\d+)(_(\d+))?ins([ATCG]+)$`)
	genomicDuplicationPattern  = regexp.MustCompile(`^(NC_\d+\.\d+|chr\d+|chr[XYM]):g\.(\d+)(_(\d+))?dup([ATCG]*)$`)
	genomicInversionPattern    = regexp.MustCompile(`^(NC_\d+\.\d+|chr\d+|chr[XYM]):g\.(\d+)_(\d+)inv$`)

	// Coding patterns
	codingSubstitutionPattern = regexp.MustCompile(`^(NM_\d+\.\d+):c\.([*\-]?\d+)([ATCG]+)>([ATCG]+)$`)
	codingDeletionPattern     = regexp.MustCompile(`^(NM_\d+\.\d+):c\.([*\-]?\d+)(_([*\-]?\d+))?del([ATCG]*)$`)
	codingInsertionPattern    = regexp.MustCompile(`^(NM_\d+\.\d+):c\.([*\-]?\d+)(_([*\-]?\d+))?ins([ATCG]+)$`)
	codingDuplicationPattern  = regexp.MustCompile(`^(NM_\d+\.\d+):c\.([*\-]?\d+)(_([*\-]?\d+))?dup([ATCG]*)$`)
	codingFrameshiftPattern   = regexp.MustCompile(`^(NM_\d+\.\d+):c\.([*\-]?\d+)(_([*\-]?\d+))?(del.*fs|.*fs)$`)

	// Protein patterns
	proteinSubstitutionPattern = regexp.MustCompile(`^(NP_\d+\.\d+):p\.([A-Z][a-z]{2})(\d+)([A-Z][a-z]{2})$`)
	proteinDeletionPattern     = regexp.MustCompile(`^(NP_\d+\.\d+):p\.([A-Z][a-z]{2})(\d+)(_([A-Z][a-z]{2})(\d+))?del$`)
	proteinInsertionPattern    = regexp.MustCompile(`^(NP_\d+\.\d+):p\.([A-Z][a-z]{2})(\d+)_([A-Z][a-z]{2})(\d+)ins([A-Z][a-z]{2})+$`)
	proteinFrameshiftPattern   = regexp.MustCompile(`^(NP_\d+\.\d+):p\.([A-Z][a-z]{2})(\d+)([A-Z][a-z]{2})?fs(\*\d+)?$`)
	proteinStopPattern         = regexp.MustCompile(`^(NP_\d+\.\d+):p\.([A-Z][a-z]{2})(\d+)\*$`)

	// Chromosome patterns for normalization
	chromosomePatterns = map[string]string{
		"chr1": "1", "chr2": "2", "chr3": "3", "chr4": "4", "chr5": "5",
		"chr6": "6", "chr7": "7", "chr8": "8", "chr9": "9", "chr10": "10",
		"chr11": "11", "chr12": "12", "chr13": "13", "chr14": "14", "chr15": "15",
		"chr16": "16", "chr17": "17", "chr18": "18", "chr19": "19", "chr20": "20",
		"chr21": "21", "chr22": "22", "chrX": "X", "chrY": "Y", "chrM": "M",
	}

	// Three-letter to one-letter amino acid codes
	aminoAcidCodes = map[string]string{
		"Ala": "A", "Arg": "R", "Asn": "N", "Asp": "D", "Cys": "C",
		"Gln": "Q", "Glu": "E", "Gly": "G", "His": "H", "Ile": "I",
		"Leu": "L", "Lys": "K", "Met": "M", "Phe": "F", "Pro": "P",
		"Ser": "S", "Thr": "T", "Trp": "W", "Tyr": "Y", "Val": "V",
		"Ter": "*", "Stop": "*",
	}
)

// Parser implements the InputParser interface
type Parser struct {
	validator *Validator
}

// NewParser creates a new HGVS parser
func NewParser() *Parser {
	return &Parser{
		validator: NewValidator(),
	}
}

// ParseVariant parses and validates HGVS notation, returning a StandardizedVariant
func (p *Parser) ParseVariant(input string) (*domain.StandardizedVariant, error) {
	if input == "" {
		return nil, fmt.Errorf("parsing variant: %w", domain.NewValidationError("hgvs", "HGVS notation cannot be empty", input))
	}

	// Clean and validate input
	hgvs := strings.TrimSpace(input)
	if err := p.validator.ValidateHGVS(hgvs); err != nil {
		return nil, fmt.Errorf("parsing variant %q: %w", input, err)
	}

	// Parse HGVS components
	components, err := p.parseHGVSDetailed(hgvs)
	if err != nil {
		return nil, fmt.Errorf("parsing variant components: %w", err)
	}

	// Convert to StandardizedVariant
	variant, err := p.componentsToVariant(components)
	if err != nil {
		return nil, fmt.Errorf("converting to standardized variant: %w", err)
	}

	return variant, nil
}

// ValidateHGVS validates HGVS notation format
func (p *Parser) ValidateHGVS(hgvs string) error {
	return p.validator.ValidateHGVS(hgvs)
}

// NormalizeVariant normalizes a variant to a consistent representation
func (p *Parser) NormalizeVariant(variant *domain.StandardizedVariant) error {
	if variant == nil {
		return fmt.Errorf("normalizing variant: variant cannot be nil")
	}

	// Normalize chromosome representation
	if variant.Chromosome != "" {
		variant.Chromosome = p.normalizeChromosome(variant.Chromosome)
	}

	// Normalize alleles to uppercase
	variant.Reference = strings.ToUpper(variant.Reference)
	variant.Alternative = strings.ToUpper(variant.Alternative)

	// Normalize HGVS notations
	if variant.HGVSGenomic != "" {
		normalized, err := p.normalizeHGVS(variant.HGVSGenomic)
		if err != nil {
			return fmt.Errorf("normalizing genomic HGVS: %w", err)
		}
		variant.HGVSGenomic = normalized
	}

	if variant.HGVSCoding != "" {
		normalized, err := p.normalizeHGVS(variant.HGVSCoding)
		if err != nil {
			return fmt.Errorf("normalizing coding HGVS: %w", err)
		}
		variant.HGVSCoding = normalized
	}

	if variant.HGVSProtein != "" {
		normalized, err := p.normalizeHGVS(variant.HGVSProtein)
		if err != nil {
			return fmt.Errorf("normalizing protein HGVS: %w", err)
		}
		variant.HGVSProtein = normalized
	}

	return nil
}

// Enhanced HGVS components structure
type DetailedHGVSComponents struct {
	Original       string
	Type           string // genomic, coding, protein
	Reference      string
	VariantType    string // substitution, deletion, insertion, duplication, etc.
	StartPosition  string
	EndPosition    string
	RefSequence    string
	AltSequence    string
	AminoAcidStart string
	AminoAcidEnd   string
	IsFrameshift   bool
}

// parseHGVSDetailed provides comprehensive HGVS parsing
func (p *Parser) parseHGVSDetailed(hgvs string) (*DetailedHGVSComponents, error) {
	components := &DetailedHGVSComponents{
		Original: hgvs,
	}

	// Determine type and parse accordingly
	if strings.Contains(hgvs, ":g.") {
		return p.parseGenomicHGVS(hgvs, components)
	} else if strings.Contains(hgvs, ":c.") {
		return p.parseCodingHGVS(hgvs, components)
	} else if strings.Contains(hgvs, ":p.") {
		return p.parseProteinHGVS(hgvs, components)
	}

	return nil, fmt.Errorf("unrecognized HGVS notation format: %s", hgvs)
}

// parseGenomicHGVS parses genomic HGVS notation
func (p *Parser) parseGenomicHGVS(hgvs string, components *DetailedHGVSComponents) (*DetailedHGVSComponents, error) {
	components.Type = "genomic"

	// Try different genomic patterns
	if matches := genomicSubstitutionPattern.FindStringSubmatch(hgvs); matches != nil {
		components.Reference = matches[1]
		components.VariantType = "substitution"
		components.StartPosition = matches[2]
		components.RefSequence = matches[3]
		components.AltSequence = matches[4]
		return components, nil
	}

	if matches := genomicDeletionPattern.FindStringSubmatch(hgvs); matches != nil {
		components.Reference = matches[1]
		components.VariantType = "deletion"
		components.StartPosition = matches[2]
		if matches[4] != "" {
			components.EndPosition = matches[4]
		}
		components.RefSequence = matches[5]
		return components, nil
	}

	if matches := genomicInsertionPattern.FindStringSubmatch(hgvs); matches != nil {
		components.Reference = matches[1]
		components.VariantType = "insertion"
		components.StartPosition = matches[2]
		if matches[4] != "" {
			components.EndPosition = matches[4]
		}
		components.AltSequence = matches[5]
		return components, nil
	}

	if matches := genomicDuplicationPattern.FindStringSubmatch(hgvs); matches != nil {
		components.Reference = matches[1]
		components.VariantType = "duplication"
		components.StartPosition = matches[2]
		if matches[4] != "" {
			components.EndPosition = matches[4]
		}
		components.RefSequence = matches[5]
		return components, nil
	}

	if matches := genomicInversionPattern.FindStringSubmatch(hgvs); matches != nil {
		components.Reference = matches[1]
		components.VariantType = "inversion"
		components.StartPosition = matches[2]
		components.EndPosition = matches[3]
		return components, nil
	}

	return nil, fmt.Errorf("unable to parse genomic HGVS notation: %s", hgvs)
}

// parseCodingHGVS parses coding HGVS notation
func (p *Parser) parseCodingHGVS(hgvs string, components *DetailedHGVSComponents) (*DetailedHGVSComponents, error) {
	components.Type = "coding"

	// Check for frameshift patterns first (they may contain "del")
	if matches := codingFrameshiftPattern.FindStringSubmatch(hgvs); matches != nil {
		components.Reference = matches[1]
		components.VariantType = "frameshift"
		components.StartPosition = matches[2]
		if matches[4] != "" {
			components.EndPosition = matches[4]
		}
		components.IsFrameshift = true
		return components, nil
	}

	// Try different coding patterns
	if matches := codingSubstitutionPattern.FindStringSubmatch(hgvs); matches != nil {
		components.Reference = matches[1]
		components.VariantType = "substitution"
		components.StartPosition = matches[2]
		components.RefSequence = matches[3]
		components.AltSequence = matches[4]
		return components, nil
	}

	if matches := codingDeletionPattern.FindStringSubmatch(hgvs); matches != nil {
		components.Reference = matches[1]
		components.VariantType = "deletion"
		components.StartPosition = matches[2]
		if matches[4] != "" {
			components.EndPosition = matches[4]
		}
		components.RefSequence = matches[5]
		return components, nil
	}

	if matches := codingInsertionPattern.FindStringSubmatch(hgvs); matches != nil {
		components.Reference = matches[1]
		components.VariantType = "insertion"
		components.StartPosition = matches[2]
		if matches[4] != "" {
			components.EndPosition = matches[4]
		}
		components.AltSequence = matches[5]
		return components, nil
	}

	if matches := codingDuplicationPattern.FindStringSubmatch(hgvs); matches != nil {
		components.Reference = matches[1]
		components.VariantType = "duplication"
		components.StartPosition = matches[2]
		if matches[4] != "" {
			components.EndPosition = matches[4]
		}
		components.RefSequence = matches[5]
		return components, nil
	}

	return nil, fmt.Errorf("unable to parse coding HGVS notation: %s", hgvs)
}

// parseProteinHGVS parses protein HGVS notation
func (p *Parser) parseProteinHGVS(hgvs string, components *DetailedHGVSComponents) (*DetailedHGVSComponents, error) {
	components.Type = "protein"

	// Check for frameshift
	if proteinFrameshiftPattern.MatchString(hgvs) {
		components.IsFrameshift = true
	}

	// Try different protein patterns
	if matches := proteinSubstitutionPattern.FindStringSubmatch(hgvs); matches != nil {
		components.Reference = matches[1]
		components.VariantType = "substitution"
		components.AminoAcidStart = matches[2]
		components.StartPosition = matches[3]
		components.AminoAcidEnd = matches[4]
		return components, nil
	}

	if matches := proteinStopPattern.FindStringSubmatch(hgvs); matches != nil {
		components.Reference = matches[1]
		components.VariantType = "nonsense"
		components.AminoAcidStart = matches[2]
		components.StartPosition = matches[3]
		components.AminoAcidEnd = "*"
		return components, nil
	}

	if matches := proteinFrameshiftPattern.FindStringSubmatch(hgvs); matches != nil {
		components.Reference = matches[1]
		components.VariantType = "frameshift"
		components.AminoAcidStart = matches[2]
		components.StartPosition = matches[3]
		components.IsFrameshift = true
		return components, nil
	}

	if matches := proteinDeletionPattern.FindStringSubmatch(hgvs); matches != nil {
		components.Reference = matches[1]
		components.VariantType = "deletion"
		components.AminoAcidStart = matches[2]
		components.StartPosition = matches[3]
		if matches[5] != "" && matches[6] != "" {
			components.AminoAcidEnd = matches[5]
			components.EndPosition = matches[6]
		}
		return components, nil
	}

	return nil, fmt.Errorf("unable to parse protein HGVS notation: %s", hgvs)
}

// componentsToVariant converts parsed components to StandardizedVariant
func (p *Parser) componentsToVariant(components *DetailedHGVSComponents) (*domain.StandardizedVariant, error) {
	variant := &domain.StandardizedVariant{}

	// Set basic fields
	variant.Reference = components.RefSequence
	variant.Alternative = components.AltSequence

	// Extract chromosome and position for genomic variants
	if components.Type == "genomic" {
		variant.Chromosome = p.normalizeChromosome(components.Reference)

		// Parse position
		pos, err := strconv.ParseInt(components.StartPosition, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parsing position %s: %w", components.StartPosition, err)
		}
		variant.Position = pos
		variant.HGVSGenomic = components.Original
	}

	// Set other HGVS notations based on type
	switch components.Type {
	case "coding":
		variant.HGVSCoding = components.Original
	case "protein":
		variant.HGVSProtein = components.Original
	}

	// Default to germline unless specified otherwise
	variant.VariantType = domain.GERMLINE

	return variant, nil
}

// normalizeChromosome normalizes chromosome representation
func (p *Parser) normalizeChromosome(chr string) string {
	// Handle RefSeq accessions (NC_000001.11 -> 1)
	if strings.HasPrefix(chr, "NC_") {
		parts := strings.Split(chr, ".")
		if len(parts) >= 1 {
			accession := parts[0]
			switch accession {
			case "NC_000001":
				return "1"
			case "NC_000002":
				return "2"
			case "NC_000003":
				return "3"
			case "NC_000004":
				return "4"
			case "NC_000005":
				return "5"
			case "NC_000006":
				return "6"
			case "NC_000007":
				return "7"
			case "NC_000008":
				return "8"
			case "NC_000009":
				return "9"
			case "NC_000010":
				return "10"
			case "NC_000011":
				return "11"
			case "NC_000012":
				return "12"
			case "NC_000013":
				return "13"
			case "NC_000014":
				return "14"
			case "NC_000015":
				return "15"
			case "NC_000016":
				return "16"
			case "NC_000017":
				return "17"
			case "NC_000018":
				return "18"
			case "NC_000019":
				return "19"
			case "NC_000020":
				return "20"
			case "NC_000021":
				return "21"
			case "NC_000022":
				return "22"
			case "NC_000023":
				return "X"
			case "NC_000024":
				return "Y"
			case "NC_012920":
				return "MT"
			}
		}
	}

	// Handle chr prefixed chromosomes
	if normalized, exists := chromosomePatterns[chr]; exists {
		return normalized
	}

	// Remove chr prefix if present
	if strings.HasPrefix(chr, "chr") {
		return chr[3:]
	}

	return chr
}

// normalizeHGVS normalizes HGVS notation
func (p *Parser) normalizeHGVS(hgvs string) (string, error) {
	// For now, just trim whitespace and return
	// More sophisticated normalization can be added later
	return strings.TrimSpace(hgvs), nil
}
