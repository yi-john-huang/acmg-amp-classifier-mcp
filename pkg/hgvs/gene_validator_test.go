package hgvs

import (
	"testing"

	"github.com/acmg-amp-mcp-server/internal/domain"
)

func TestGeneValidatorValidateGeneSymbol(t *testing.T) {
	validator := NewGeneValidator()

	tests := []struct {
		name    string
		symbol  string
		wantErr bool
	}{
		// Valid gene symbols
		{"Valid standard gene", "BRCA1", false},
		{"Valid gene with number", "TP53", false},
		{"Valid gene with hyphen", "HLA-A", false},
		{"Valid complex gene", "TRBV6-5", false},
		{"Valid single letter", "A", false},
		{"Valid pseudogene", "BRCA1P1", false},
		{"Valid antisense", "BRCA1AS1", false},
		{"Empty symbol (optional)", "", false},

		// Invalid gene symbols
		{"Lowercase letters", "brca1", true},
		{"Starting with number", "1BRCA", true},
		{"Special characters", "BRCA@1", true},
		{"Ending with hyphen", "BRCA1-", true},
		{"Consecutive hyphens", "BRCA1--2", true},
		{"Too long", "VERYLONGGENENAMESYMBOL", true},
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

func TestGeneValidatorValidateTranscript(t *testing.T) {
	validator := NewGeneValidator()

	tests := []struct {
		name       string
		transcript string
		wantErr    bool
	}{
		// Valid RefSeq transcripts
		{"Valid NM transcript", "NM_000059.3", false},
		{"Valid NR transcript", "NR_123456.1", false},
		{"Valid XM transcript", "XM_123456.2", false},
		{"Valid XR transcript", "XR_123456.1", false},

		// Valid Ensembl transcripts
		{"Valid Ensembl transcript", "ENST00000123456.1", false},
		{"Valid Ensembl transcript v2", "ENST00000654321.2", false},

		// Optional empty
		{"Empty transcript (optional)", "", false},

		// Invalid formats
		{"Invalid RefSeq format", "NM_000059", true},
		{"Invalid prefix", "AB_000059.3", true},
		{"Invalid characters", "NM_00005a.3", true},
		{"Invalid Ensembl format", "ENST123456.1", true},
		{"Random string", "randomstring", true},
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

func TestValidateGeneID(t *testing.T) {
	validator := NewGeneValidator()

	tests := []struct {
		name    string
		geneID  string
		wantErr bool
	}{
		// Valid gene IDs
		{"Valid Entrez ID", "672", false},
		{"Valid Entrez ID long", "123456789", false},
		{"Valid Ensembl gene ID", "ENSG00000012345.1", false},
		{"Valid HGNC ID", "HGNC:1100", false},
		{"Empty gene ID (optional)", "", false},

		// Invalid gene IDs
		{"Invalid Entrez format", "ABC123", true},
		{"Invalid Ensembl format", "ENSG123456.1", true},
		{"Invalid HGNC format", "HGNC123", true},
		{"Random string", "randomid", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateGeneID(tt.geneID)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateGeneID() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateGeneTranscriptPair(t *testing.T) {
	validator := NewGeneValidator()

	tests := []struct {
		name       string
		geneSymbol string
		transcript string
		wantErr    bool
	}{
		{
			name:       "Valid pair",
			geneSymbol: "BRCA1",
			transcript: "NM_000059.3",
			wantErr:    false,
		},
		{
			name:       "Empty gene symbol",
			geneSymbol: "",
			transcript: "NM_000059.3",
			wantErr:    false,
		},
		{
			name:       "Empty transcript",
			geneSymbol: "BRCA1",
			transcript: "",
			wantErr:    false,
		},
		{
			name:       "Both empty",
			geneSymbol: "",
			transcript: "",
			wantErr:    false,
		},
		{
			name:       "Invalid gene symbol",
			geneSymbol: "invalid-gene",
			transcript: "NM_000059.3",
			wantErr:    true,
		},
		{
			name:       "Invalid transcript",
			geneSymbol: "BRCA1",
			transcript: "invalid-transcript",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateGeneTranscriptPair(tt.geneSymbol, tt.transcript)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateGeneTranscriptPair() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateVariantGeneData(t *testing.T) {
	validator := NewGeneValidator()

	tests := []struct {
		name     string
		request  *domain.VariantRequest
		wantErrs int
	}{
		{
			name: "Valid gene data",
			request: &domain.VariantRequest{
				GeneSymbol: "BRCA1",
				Transcript: "NM_000059.3",
			},
			wantErrs: 0,
		},
		{
			name: "Empty gene data (valid)",
			request: &domain.VariantRequest{
				GeneSymbol: "",
				Transcript: "",
			},
			wantErrs: 0,
		},
		{
			name: "Invalid gene symbol",
			request: &domain.VariantRequest{
				GeneSymbol: "invalid-gene",
				Transcript: "NM_000059.3",
			},
			wantErrs: 1,
		},
		{
			name: "Invalid transcript",
			request: &domain.VariantRequest{
				GeneSymbol: "BRCA1",
				Transcript: "invalid-transcript",
			},
			wantErrs: 1,
		},
		{
			name: "Both invalid",
			request: &domain.VariantRequest{
				GeneSymbol: "invalid-gene",
				Transcript: "invalid-transcript",
			},
			wantErrs: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validator.ValidateVariantGeneData(tt.request)
			if len(errors) != tt.wantErrs {
				t.Errorf("ValidateVariantGeneData() got %d errors, want %d", len(errors), tt.wantErrs)
			}
		})
	}
}

func TestIsValidGeneFormat(t *testing.T) {
	validator := NewGeneValidator()

	tests := []struct {
		name   string
		symbol string
		want   bool
	}{
		{"Single letter", "A", true},
		{"Standard gene", "BRCA1", true},
		{"Gene with hyphen", "HLA-A", true},
		{"Complex gene", "BRCA1P1", true},
		{"Antisense gene", "BRCA1AS1", true},
		{"Invalid lowercase", "brca1", false},
		{"Invalid start number", "1BRCA", false},
		{"Invalid special char", "BRCA@1", false},
		{"Empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validator.isValidGeneFormat(tt.symbol)
			if got != tt.want {
				t.Errorf("isValidGeneFormat() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateGeneNamingRules(t *testing.T) {
	validator := NewGeneValidator()

	tests := []struct {
		name    string
		symbol  string
		wantErr bool
	}{
		{"Valid standard", "BRCA1", false},
		{"Valid with hyphen", "HLA-A", false},
		{"Valid single letter", "A", false},
		{"Invalid start number", "1BRCA", true},
		{"Invalid end hyphen", "BRCA1-", true},
		{"Invalid consecutive hyphens", "BRCA1--2", true},
		{"Invalid too long", "VERYLONGGENENAMESYMBOL", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateGeneNamingRules(tt.symbol)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateGeneNamingRules() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestKnownGenesFunctionality(t *testing.T) {
	validator := NewGeneValidator()

	// Test adding and checking known genes
	validator.AddKnownGene("BRCA1")
	validator.AddKnownGene("TP53")

	if !validator.IsKnownGene("BRCA1") {
		t.Error("Expected BRCA1 to be a known gene")
	}

	if !validator.IsKnownGene("brca1") { // Case insensitive
		t.Error("Expected brca1 to be recognized as BRCA1")
	}

	if validator.IsKnownGene("UNKNOWN") {
		t.Error("Expected UNKNOWN to not be a known gene")
	}

	// Test adding and checking known transcripts
	validator.AddKnownTranscript("NM_000059.3")
	validator.AddKnownTranscript("NM_000546.5")

	if !validator.IsKnownTranscript("NM_000059.3") {
		t.Error("Expected NM_000059.3 to be a known transcript")
	}

	if validator.IsKnownTranscript("NM_999999.9") {
		t.Error("Expected NM_999999.9 to not be a known transcript")
	}
}

func TestRefSeqTranscriptValidation(t *testing.T) {
	validator := NewGeneValidator()

	tests := []struct {
		name       string
		transcript string
		wantErr    bool
	}{
		{"Valid NM", "NM_000059.3", false},
		{"Valid NR", "NR_123456.1", false},
		{"Valid XM", "XM_123456.2", false},
		{"Valid XR", "XR_123456.1", false},
		{"Invalid prefix", "YM_123456.1", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateRefSeqTranscript(tt.transcript)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateRefSeqTranscript() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEnsemblTranscriptValidation(t *testing.T) {
	validator := NewGeneValidator()

	tests := []struct {
		name       string
		transcript string
		wantErr    bool
	}{
		{"Valid Ensembl", "ENST00000123456.1", false},
		{"Valid Ensembl v2", "ENST00000654321.2", false},
		{"Invalid format", "ENST123456.1", true},
		{"Wrong prefix", "ENSG00000123456.1", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateEnsemblTranscript(tt.transcript)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateEnsemblTranscript() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}