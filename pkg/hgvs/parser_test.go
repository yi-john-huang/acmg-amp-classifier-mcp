package hgvs

import (
	"testing"

	"github.com/acmg-amp-mcp-server/internal/domain"
)

func TestParseVariant(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name           string
		input          string
		expectedChrom  string
		expectedPos    int64
		expectedRef    string
		expectedAlt    string
		expectedHGVS   string
		expectedType   domain.VariantType
		wantErr        bool
	}{
		{
			name:          "Valid genomic substitution",
			input:         "NC_000017.11:g.43104261G>T",
			expectedChrom: "17",
			expectedPos:   43104261,
			expectedRef:   "G",
			expectedAlt:   "T",
			expectedHGVS:  "NC_000017.11:g.43104261G>T",
			expectedType:  domain.GERMLINE,
			wantErr:       false,
		},
		{
			name:          "Valid chr notation",
			input:         "chr17:g.43104261G>T",
			expectedChrom: "17",
			expectedPos:   43104261,
			expectedRef:   "G",
			expectedAlt:   "T",
			expectedHGVS:  "chr17:g.43104261G>T",
			expectedType:  domain.GERMLINE,
			wantErr:       false,
		},
		{
			name:          "Valid X chromosome",
			input:         "chrX:g.12345A>C",
			expectedChrom: "X",
			expectedPos:   12345,
			expectedRef:   "A",
			expectedAlt:   "C",
			expectedHGVS:  "chrX:g.12345A>C",
			expectedType:  domain.GERMLINE,
			wantErr:       false,
		},
		{
			name:    "Empty input",
			input:   "",
			wantErr: true,
		},
		{
			name:    "Invalid HGVS format",
			input:   "invalid-hgvs",
			wantErr: true,
		},
		{
			name:    "Malformed position",
			input:   "chr17:g.invalidG>T",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.ParseVariant(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseVariant() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && result != nil {
				if result.Chromosome != tt.expectedChrom {
					t.Errorf("ParseVariant() chromosome = %v, want %v", result.Chromosome, tt.expectedChrom)
				}
				if result.Position != tt.expectedPos {
					t.Errorf("ParseVariant() position = %v, want %v", result.Position, tt.expectedPos)
				}
				if result.Reference != tt.expectedRef {
					t.Errorf("ParseVariant() reference = %v, want %v", result.Reference, tt.expectedRef)
				}
				if result.Alternative != tt.expectedAlt {
					t.Errorf("ParseVariant() alternative = %v, want %v", result.Alternative, tt.expectedAlt)
				}
				if result.HGVSGenomic != tt.expectedHGVS {
					t.Errorf("ParseVariant() HGVS = %v, want %v", result.HGVSGenomic, tt.expectedHGVS)
				}
				if result.VariantType != tt.expectedType {
					t.Errorf("ParseVariant() type = %v, want %v", result.VariantType, tt.expectedType)
				}
			}
		})
	}
}

func TestParseHGVSDetailed(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name         string
		input        string
		expectedType string
		expectedVar  string
		wantErr      bool
	}{
		{
			name:         "Genomic substitution",
			input:        "NC_000017.11:g.43104261G>T",
			expectedType: "genomic",
			expectedVar:  "substitution",
			wantErr:      false,
		},
		{
			name:         "Genomic deletion",
			input:        "NC_000017.11:g.43104261_43104263del",
			expectedType: "genomic",
			expectedVar:  "deletion",
			wantErr:      false,
		},
		{
			name:         "Genomic insertion",
			input:        "NC_000017.11:g.43104261_43104262insATG",
			expectedType: "genomic",
			expectedVar:  "insertion",
			wantErr:      false,
		},
		{
			name:         "Genomic duplication",
			input:        "NC_000017.11:g.43104261_43104263dup",
			expectedType: "genomic",
			expectedVar:  "duplication",
			wantErr:      false,
		},
		{
			name:         "Genomic inversion",
			input:        "NC_000017.11:g.43104261_43104263inv",
			expectedType: "genomic",
			expectedVar:  "inversion",
			wantErr:      false,
		},
		{
			name:         "Coding substitution",
			input:        "NM_000059.3:c.274G>T",
			expectedType: "coding",
			expectedVar:  "substitution",
			wantErr:      false,
		},
		{
			name:         "Coding deletion",
			input:        "NM_000059.3:c.274_276del",
			expectedType: "coding",
			expectedVar:  "deletion",
			wantErr:      false,
		},
		{
			name:         "Protein substitution",
			input:        "NP_000050.2:p.Gly92Cys",
			expectedType: "protein",
			expectedVar:  "substitution",
			wantErr:      false,
		},
		{
			name:         "Protein nonsense",
			input:        "NP_000050.2:p.Gly92*",
			expectedType: "protein",
			expectedVar:  "nonsense",
			wantErr:      false,
		},
		{
			name:    "Invalid notation",
			input:   "invalid:notation",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.parseHGVSDetailed(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseHGVSDetailed() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && result != nil {
				if result.Type != tt.expectedType {
					t.Errorf("parseHGVSDetailed() type = %v, want %v", result.Type, tt.expectedType)
				}
				if result.VariantType != tt.expectedVar {
					t.Errorf("parseHGVSDetailed() variantType = %v, want %v", result.VariantType, tt.expectedVar)
				}
			}
		})
	}
}

func TestNormalizeChromosome(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"NC accession chr1", "NC_000001.11", "1"},
		{"NC accession chr17", "NC_000017.11", "17"},
		{"NC accession chrX", "NC_000023.11", "X"},
		{"NC accession chrY", "NC_000024.11", "Y"},
		{"NC accession chrMT", "NC_012920.11", "MT"},
		{"Chr prefix chr1", "chr1", "1"},
		{"Chr prefix chr17", "chr17", "17"},
		{"Chr prefix chrX", "chrX", "X"},
		{"Chr prefix chrY", "chrY", "Y"},
		{"Chr prefix chrM", "chrM", "M"},
		{"Plain number", "17", "17"},
		{"Plain X", "X", "X"},
		{"Plain Y", "Y", "Y"},
		{"Unknown format", "unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.normalizeChromosome(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeChromosome() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestNormalizeVariant(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name     string
		input    *domain.StandardizedVariant
		expected *domain.StandardizedVariant
		wantErr  bool
	}{
		{
			name: "Normalize chromosome and alleles",
			input: &domain.StandardizedVariant{
				Chromosome:  "chr17",
				Reference:   "g",
				Alternative: "t",
				HGVSGenomic: " NC_000017.11:g.43104261G>T ",
			},
			expected: &domain.StandardizedVariant{
				Chromosome:  "17",
				Reference:   "G",
				Alternative: "T",
				HGVSGenomic: "NC_000017.11:g.43104261G>T",
			},
			wantErr: false,
		},
		{
			name:    "Nil variant",
			input:   nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := parser.NormalizeVariant(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("NormalizeVariant() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.input != nil {
				if tt.input.Chromosome != tt.expected.Chromosome {
					t.Errorf("NormalizeVariant() chromosome = %v, want %v", tt.input.Chromosome, tt.expected.Chromosome)
				}
				if tt.input.Reference != tt.expected.Reference {
					t.Errorf("NormalizeVariant() reference = %v, want %v", tt.input.Reference, tt.expected.Reference)
				}
				if tt.input.Alternative != tt.expected.Alternative {
					t.Errorf("NormalizeVariant() alternative = %v, want %v", tt.input.Alternative, tt.expected.Alternative)
				}
				if tt.input.HGVSGenomic != tt.expected.HGVSGenomic {
					t.Errorf("NormalizeVariant() HGVS = %v, want %v", tt.input.HGVSGenomic, tt.expected.HGVSGenomic)
				}
			}
		})
	}
}

func TestParseGenomicHGVS(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name           string
		input          string
		expectedType   string
		expectedRef    string
		expectedStart  string
		expectedEnd    string
		expectedRefSeq string
		expectedAltSeq string
		wantErr        bool
	}{
		{
			name:           "Simple substitution",
			input:          "NC_000017.11:g.43104261G>T",
			expectedType:   "substitution",
			expectedRef:    "NC_000017.11",
			expectedStart:  "43104261",
			expectedRefSeq: "G",
			expectedAltSeq: "T",
			wantErr:        false,
		},
		{
			name:           "Single position deletion",
			input:          "NC_000017.11:g.43104261delG",
			expectedType:   "deletion",
			expectedRef:    "NC_000017.11",
			expectedStart:  "43104261",
			expectedRefSeq: "G",
			wantErr:        false,
		},
		{
			name:           "Range deletion",
			input:          "NC_000017.11:g.43104261_43104263del",
			expectedType:   "deletion",
			expectedRef:    "NC_000017.11",
			expectedStart:  "43104261",
			expectedEnd:    "43104263",
			wantErr:        false,
		},
		{
			name:           "Insertion",
			input:          "NC_000017.11:g.43104261_43104262insATG",
			expectedType:   "insertion",
			expectedRef:    "NC_000017.11",
			expectedStart:  "43104261",
			expectedEnd:    "43104262",
			expectedAltSeq: "ATG",
			wantErr:        false,
		},
		{
			name:    "Invalid format",
			input:   "invalid:g.123",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			components := &DetailedHGVSComponents{}
			result, err := parser.parseGenomicHGVS(tt.input, components)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseGenomicHGVS() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && result != nil {
				if result.VariantType != tt.expectedType {
					t.Errorf("parseGenomicHGVS() type = %v, want %v", result.VariantType, tt.expectedType)
				}
				if result.Reference != tt.expectedRef {
					t.Errorf("parseGenomicHGVS() reference = %v, want %v", result.Reference, tt.expectedRef)
				}
				if result.StartPosition != tt.expectedStart {
					t.Errorf("parseGenomicHGVS() start = %v, want %v", result.StartPosition, tt.expectedStart)
				}
				if tt.expectedEnd != "" && result.EndPosition != tt.expectedEnd {
					t.Errorf("parseGenomicHGVS() end = %v, want %v", result.EndPosition, tt.expectedEnd)
				}
				if tt.expectedRefSeq != "" && result.RefSequence != tt.expectedRefSeq {
					t.Errorf("parseGenomicHGVS() refSeq = %v, want %v", result.RefSequence, tt.expectedRefSeq)
				}
				if tt.expectedAltSeq != "" && result.AltSequence != tt.expectedAltSeq {
					t.Errorf("parseGenomicHGVS() altSeq = %v, want %v", result.AltSequence, tt.expectedAltSeq)
				}
			}
		})
	}
}

func TestParseCodingHGVS(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name            string
		input           string
		expectedType    string
		expectedRef     string
		expectedStart   string
		expectedRefSeq  string
		expectedAltSeq  string
		expectedFrameshift bool
		wantErr         bool
	}{
		{
			name:           "Simple substitution",
			input:          "NM_000059.3:c.274G>T",
			expectedType:   "substitution",
			expectedRef:    "NM_000059.3",
			expectedStart:  "274",
			expectedRefSeq: "G",
			expectedAltSeq: "T",
			wantErr:        false,
		},
		{
			name:           "UTR position",
			input:          "NM_000059.3:c.-15G>T",
			expectedType:   "substitution",
			expectedRef:    "NM_000059.3",
			expectedStart:  "-15",
			expectedRefSeq: "G",
			expectedAltSeq: "T",
			wantErr:        false,
		},
		{
			name:          "Deletion",
			input:         "NM_000059.3:c.274_276del",
			expectedType:  "deletion",
			expectedRef:   "NM_000059.3",
			expectedStart: "274",
			wantErr:       false,
		},
		{
			name:               "Frameshift deletion",
			input:              "NM_000059.3:c.274delfs",
			expectedFrameshift: true,
			wantErr:            false,
		},
		{
			name:    "Invalid format",
			input:   "invalid:c.123",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			components := &DetailedHGVSComponents{}
			result, err := parser.parseCodingHGVS(tt.input, components)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseCodingHGVS() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && result != nil {
				if result.Type != "coding" {
					t.Errorf("parseCodingHGVS() type = %v, want coding", result.Type)
				}
				if tt.expectedType != "" && result.VariantType != tt.expectedType {
					t.Errorf("parseCodingHGVS() variantType = %v, want %v", result.VariantType, tt.expectedType)
				}
				if tt.expectedRef != "" && result.Reference != tt.expectedRef {
					t.Errorf("parseCodingHGVS() reference = %v, want %v", result.Reference, tt.expectedRef)
				}
				if tt.expectedStart != "" && result.StartPosition != tt.expectedStart {
					t.Errorf("parseCodingHGVS() start = %v, want %v", result.StartPosition, tt.expectedStart)
				}
				if result.IsFrameshift != tt.expectedFrameshift {
					t.Errorf("parseCodingHGVS() frameshift = %v, want %v", result.IsFrameshift, tt.expectedFrameshift)
				}
			}
		})
	}
}

func TestParseProteinHGVS(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name               string
		input              string
		expectedType       string
		expectedRef        string
		expectedStart      string
		expectedAAStart    string
		expectedAAEnd      string
		expectedFrameshift bool
		wantErr            bool
	}{
		{
			name:            "Simple substitution",
			input:           "NP_000050.2:p.Gly92Cys",
			expectedType:    "substitution",
			expectedRef:     "NP_000050.2",
			expectedStart:   "92",
			expectedAAStart: "Gly",
			expectedAAEnd:   "Cys",
			wantErr:         false,
		},
		{
			name:            "Nonsense mutation",
			input:           "NP_000050.2:p.Gly92*",
			expectedType:    "nonsense",
			expectedRef:     "NP_000050.2",
			expectedStart:   "92",
			expectedAAStart: "Gly",
			expectedAAEnd:   "*",
			wantErr:         false,
		},
		{
			name:               "Frameshift",
			input:              "NP_000050.2:p.Gly92Alafs*15",
			expectedType:       "frameshift",
			expectedRef:        "NP_000050.2",
			expectedStart:      "92",
			expectedAAStart:    "Gly",
			expectedFrameshift: true,
			wantErr:            false,
		},
		{
			name:            "Deletion",
			input:           "NP_000050.2:p.Gly92_Ala94del",
			expectedType:    "deletion",
			expectedRef:     "NP_000050.2",
			expectedStart:   "92",
			expectedAAStart: "Gly",
			wantErr:         false,
		},
		{
			name:    "Invalid format",
			input:   "invalid:p.Gly92",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			components := &DetailedHGVSComponents{}
			result, err := parser.parseProteinHGVS(tt.input, components)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseProteinHGVS() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && result != nil {
				if result.Type != "protein" {
					t.Errorf("parseProteinHGVS() type = %v, want protein", result.Type)
				}
				if tt.expectedType != "" && result.VariantType != tt.expectedType {
					t.Errorf("parseProteinHGVS() variantType = %v, want %v", result.VariantType, tt.expectedType)
				}
				if tt.expectedRef != "" && result.Reference != tt.expectedRef {
					t.Errorf("parseProteinHGVS() reference = %v, want %v", result.Reference, tt.expectedRef)
				}
				if tt.expectedStart != "" && result.StartPosition != tt.expectedStart {
					t.Errorf("parseProteinHGVS() start = %v, want %v", result.StartPosition, tt.expectedStart)
				}
				if tt.expectedAAStart != "" && result.AminoAcidStart != tt.expectedAAStart {
					t.Errorf("parseProteinHGVS() aaStart = %v, want %v", result.AminoAcidStart, tt.expectedAAStart)
				}
				if tt.expectedAAEnd != "" && result.AminoAcidEnd != tt.expectedAAEnd {
					t.Errorf("parseProteinHGVS() aaEnd = %v, want %v", result.AminoAcidEnd, tt.expectedAAEnd)
				}
				if result.IsFrameshift != tt.expectedFrameshift {
					t.Errorf("parseProteinHGVS() frameshift = %v, want %v", result.IsFrameshift, tt.expectedFrameshift)
				}
			}
		})
	}
}