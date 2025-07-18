package hgvs

import (
	"testing"

	"github.com/acmg-amp-mcp-server/internal/domain"
)

func TestValidateHGVS(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name    string
		hgvs    string
		wantErr bool
	}{
		// Valid genomic HGVS
		{"Valid genomic NC", "NC_000017.11:g.43104261G>T", false},
		{"Valid genomic chr", "chr17:g.43104261G>T", false},
		{"Valid genomic chrX", "chrX:g.12345A>C", false},

		// Valid coding HGVS
		{"Valid coding", "NM_000059.3:c.274G>T", false},
		{"Valid coding with larger position", "NM_000059.3:c.1234A>G", false},

		// Valid protein HGVS
		{"Valid protein", "NP_000050.2:p.Gly92Cys", false},
		{"Valid protein Ala", "NP_000050.2:p.Ala123Val", false},

		// Invalid cases
		{"Empty string", "", true},
		{"Invalid genomic format", "NC_000017.11:g.43104261", true},
		{"Invalid coding format", "NM_000059.3:c.274", true},
		{"Invalid protein format", "NP_000050.2:p.Gly92", true},
		{"Unknown notation", "XYZ_123456:x.123A>T", true},
		{"Invalid nucleotides", "NC_000017.11:g.43104261X>Y", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateHGVS(tt.hgvs)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateHGVS() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateGeneSymbol(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name    string
		symbol  string
		wantErr bool
	}{
		{"Valid gene symbol", "BRCA1", false},
		{"Valid gene symbol with number", "TP53", false},
		{"Valid gene symbol with dash", "HLA-A", false},
		{"Empty symbol (optional)", "", false},
		{"Invalid lowercase", "brca1", true},
		{"Invalid starting with number", "1BRCA", true},
		{"Invalid special characters", "BRCA@1", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateGeneSymbol(tt.symbol)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateGeneSymbol() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateTranscript(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name       string
		transcript string
		wantErr    bool
	}{
		{"Valid NM transcript", "NM_000059.3", false},
		{"Valid NR transcript", "NR_123456.1", false},
		{"Valid XM transcript", "XM_123456.2", false},
		{"Valid XR transcript", "XR_123456.1", false},
		{"Empty transcript (optional)", "", false},
		{"Invalid format", "NM_000059", true},
		{"Invalid prefix", "AB_000059.3", true},
		{"Invalid characters", "NM_00005a.3", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateTranscript(tt.transcript)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTranscript() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateVariantRequest(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name     string
		request  *domain.VariantRequest
		wantErrs int
	}{
		{
			name: "Valid request",
			request: &domain.VariantRequest{
				HGVS:       "NM_000059.3:c.274G>T",
				GeneSymbol: "BRCA1",
				Transcript: "NM_000059.3",
				ClientID:   "test-client",
				RequestID:  "req-123",
			},
			wantErrs: 0,
		},
		{
			name: "Missing required fields",
			request: &domain.VariantRequest{
				GeneSymbol: "BRCA1",
			},
			wantErrs: 3, // Missing HGVS, ClientID, RequestID
		},
		{
			name: "Invalid HGVS and gene symbol",
			request: &domain.VariantRequest{
				HGVS:       "invalid-hgvs",
				GeneSymbol: "invalid-gene",
				ClientID:   "test-client",
				RequestID:  "req-123",
			},
			wantErrs: 2, // Invalid HGVS and gene symbol
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validator.ValidateVariantRequest(tt.request)
			if len(errors) != tt.wantErrs {
				t.Errorf("ValidateVariantRequest() got %d errors, want %d", len(errors), tt.wantErrs)
			}
		})
	}
}

func TestParseHGVSComponents(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name     string
		hgvs     string
		expected *HGVSComponents
		wantErr  bool
	}{
		{
			name: "Parse genomic HGVS",
			hgvs: "NC_000017.11:g.43104261G>T",
			expected: &HGVSComponents{
				Original:  "NC_000017.11:g.43104261G>T",
				Type:      "genomic",
				Reference: "NC_000017.11",
				Position:  "43104261",
				RefAllele: "G",
				AltAllele: "T",
			},
			wantErr: false,
		},
		{
			name: "Parse coding HGVS",
			hgvs: "NM_000059.3:c.274G>T",
			expected: &HGVSComponents{
				Original:  "NM_000059.3:c.274G>T",
				Type:      "coding",
				Reference: "NM_000059.3",
				Position:  "274",
				RefAllele: "G",
				AltAllele: "T",
			},
			wantErr: false,
		},
		{
			name:     "Invalid HGVS",
			hgvs:     "invalid-hgvs",
			expected: nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := validator.ParseHGVSComponents(tt.hgvs)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseHGVSComponents() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && result != nil {
				if result.Type != tt.expected.Type ||
					result.Reference != tt.expected.Reference ||
					result.Position != tt.expected.Position ||
					result.RefAllele != tt.expected.RefAllele ||
					result.AltAllele != tt.expected.AltAllele {
					t.Errorf("ParseHGVSComponents() = %+v, want %+v", result, tt.expected)
				}
			}
		})
	}
}
