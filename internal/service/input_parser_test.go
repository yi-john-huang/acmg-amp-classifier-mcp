package service

import (
	"context"
	"testing"
	"time"

	"github.com/acmg-amp-mcp-server/internal/domain"
)

func TestInputParserService_ParseVariant(t *testing.T) {
	service := NewInputParserService()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "Valid genomic HGVS",
			input:   "NC_000017.11:g.43104261G>T",
			wantErr: false,
		},
		{
			name:    "Valid coding HGVS",
			input:   "NM_000059.3:c.274G>T",
			wantErr: false,
		},
		{
			name:    "Valid protein HGVS",
			input:   "NP_000050.2:p.Gly92Cys",
			wantErr: false,
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.ParseVariant(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseVariant() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && result == nil {
				t.Error("ParseVariant() returned nil result for valid input")
			}
		})
	}
}

func TestInputParserService_ValidateHGVS(t *testing.T) {
	service := NewInputParserService()

	tests := []struct {
		name    string
		hgvs    string
		wantErr bool
	}{
		{"Valid genomic", "NC_000017.11:g.43104261G>T", false},
		{"Valid coding", "NM_000059.3:c.274G>T", false},
		{"Valid protein", "NP_000050.2:p.Gly92Cys", false},
		{"Empty string", "", true},
		{"Invalid format", "invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.ValidateHGVS(tt.hgvs)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateHGVS() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestInputParserService_NormalizeVariant(t *testing.T) {
	service := NewInputParserService()

	tests := []struct {
		name    string
		variant *domain.StandardizedVariant
		wantErr bool
	}{
		{
			name: "Valid variant",
			variant: &domain.StandardizedVariant{
				Chromosome:  "chr17",
				Reference:   "g",
				Alternative: "t",
			},
			wantErr: false,
		},
		{
			name:    "Nil variant",
			variant: nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.NormalizeVariant(tt.variant)
			if (err != nil) != tt.wantErr {
				t.Errorf("NormalizeVariant() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr && tt.variant != nil {
				// Check normalization results
				if tt.variant.Chromosome != "17" {
					t.Errorf("Expected chromosome to be normalized to '17', got '%s'", tt.variant.Chromosome)
				}
				if tt.variant.Reference != "G" {
					t.Errorf("Expected reference to be uppercase 'G', got '%s'", tt.variant.Reference)
				}
				if tt.variant.Alternative != "T" {
					t.Errorf("Expected alternative to be uppercase 'T', got '%s'", tt.variant.Alternative)
				}
			}
		})
	}
}

func TestInputParserService_ValidateVariantRequest(t *testing.T) {
	service := NewInputParserService()

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
			name: "Invalid HGVS and gene data",
			request: &domain.VariantRequest{
				HGVS:       "invalid-hgvs",
				GeneSymbol: "invalid-gene",
				ClientID:   "test-client",
				RequestID:  "req-123",
			},
			wantErrs: 2, // Invalid HGVS + gene symbol error
		},
		{
			name: "Invalid transcript",
			request: &domain.VariantRequest{
				HGVS:       "NM_000059.3:c.274G>T",
				GeneSymbol: "BRCA1",
				Transcript: "invalid-transcript",
				ClientID:   "test-client",
				RequestID:  "req-123",
			},
			wantErrs: 1, // One transcript validation error from gene validator
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := service.ValidateVariantRequest(tt.request)
			if len(errors) != tt.wantErrs {
				t.Errorf("ValidateVariantRequest() got %d errors, want %d", len(errors), tt.wantErrs)
				for i, err := range errors {
					t.Logf("Error %d: %v", i+1, err)
				}
			}
		})
	}
}

func TestInputParserService_ParseAndValidateVariantRequest(t *testing.T) {
	service := NewInputParserService()
	ctx := context.Background()

	tests := []struct {
		name     string
		request  *domain.VariantRequest
		wantErrs int
		checkVar bool
	}{
		{
			name: "Valid complete request",
			request: &domain.VariantRequest{
				HGVS:       "NC_000017.11:g.43104261G>T",
				GeneSymbol: "BRCA1",
				Transcript: "NM_000059.3",
				ClientID:   "test-client",
				RequestID:  "req-123",
				Metadata: map[string]string{
					"indication": "breast_cancer_risk",
				},
				RequestedAt: time.Now(),
			},
			wantErrs: 0,
			checkVar: true,
		},
		{
			name: "Invalid request",
			request: &domain.VariantRequest{
				HGVS: "invalid-hgvs",
			},
			wantErrs: 3, // Invalid HGVS, missing ClientID, missing RequestID
			checkVar: false,
		},
		{
			name: "Valid HGVS but invalid gene",
			request: &domain.VariantRequest{
				HGVS:       "NC_000017.11:g.43104261G>T",
				GeneSymbol: "invalid-gene",
				ClientID:   "test-client",
				RequestID:  "req-123",
			},
			wantErrs: 1, // One gene symbol validation error
			checkVar: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			variant, errors := service.ParseAndValidateVariantRequest(ctx, tt.request)
			
			if len(errors) != tt.wantErrs {
				t.Errorf("ParseAndValidateVariantRequest() got %d errors, want %d", len(errors), tt.wantErrs)
				for i, err := range errors {
					t.Logf("Error %d: %v", i+1, err)
				}
			}

			if tt.checkVar {
				if variant == nil {
					t.Error("ParseAndValidateVariantRequest() returned nil variant for valid request")
				} else {
					if variant.GeneSymbol != tt.request.GeneSymbol {
						t.Errorf("Expected gene symbol %s, got %s", tt.request.GeneSymbol, variant.GeneSymbol)
					}
					if variant.TranscriptID != tt.request.Transcript {
						t.Errorf("Expected transcript %s, got %s", tt.request.Transcript, variant.TranscriptID)
					}
				}
			} else {
				if variant != nil && len(errors) > 0 {
					t.Error("ParseAndValidateVariantRequest() returned variant despite errors")
				}
			}
		})
	}
}

func TestInputParserService_GetSupportedHGVSFormats(t *testing.T) {
	service := NewInputParserService()

	formats := service.GetSupportedHGVSFormats()
	
	if len(formats) == 0 {
		t.Error("GetSupportedHGVSFormats() returned empty list")
	}

	// Check that it includes expected formats
	expectedFormats := []string{
		"Genomic notation",
		"Coding notation", 
		"Protein notation",
	}

	for _, expected := range expectedFormats {
		found := false
		for _, format := range formats {
			if indexOf(format, expected) != -1 {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected to find format containing '%s' in supported formats", expected)
		}
	}
}

func TestInputParserService_ValidateGeneComponents(t *testing.T) {
	service := NewInputParserService()

	tests := []struct {
		name    string
		symbol  string
		wantErr bool
	}{
		{"Valid gene symbol", "BRCA1", false},
		{"Invalid gene symbol", "invalid-gene", true},
		{"Empty symbol", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.ValidateGeneSymbol(tt.symbol)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateGeneSymbol() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}

	transcriptTests := []struct {
		name       string
		transcript string
		wantErr    bool
	}{
		{"Valid RefSeq transcript", "NM_000059.3", false},
		{"Valid Ensembl transcript", "ENST00000123456.1", false},
		{"Invalid transcript", "invalid-transcript", true},
		{"Empty transcript", "", false},
	}

	for _, tt := range transcriptTests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.ValidateTranscript(tt.transcript)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTranscript() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestInputParserService_AddKnownElements(t *testing.T) {
	service := NewInputParserService()

	// Test adding known genes
	testGenes := []string{"BRCA1", "TP53", "EGFR"}
	service.AddKnownGenes(testGenes)

	// Test adding known transcripts
	testTranscripts := []string{"NM_000059.3", "NM_000546.5"}
	service.AddKnownTranscripts(testTranscripts)

	// Verify they were added (indirectly through the gene validator)
	// This test mainly ensures the methods don't panic and complete successfully
	t.Log("Successfully added known genes and transcripts")
}

func TestInputParserService_ComplexScenarios(t *testing.T) {
	service := NewInputParserService()

	tests := []struct {
		name        string
		hgvs        string
		geneSymbol  string
		transcript  string
		expectChrom string
		expectPos   int64
		wantErr     bool
	}{
		{
			name:        "BRCA1 pathogenic variant",
			hgvs:        "NC_000017.11:g.43094692G>A",
			geneSymbol:  "BRCA1",
			transcript:  "NM_007294.4",
			expectChrom: "17",
			expectPos:   43094692,
			wantErr:     false,
		},
		{
			name:        "TP53 variant on chromosome notation",
			hgvs:        "chr17:g.7674220C>T",
			geneSymbol:  "TP53",
			transcript:  "NM_000546.5",
			expectChrom: "17",
			expectPos:   7674220,
			wantErr:     false,
		},
		{
			name:       "X-linked variant",
			hgvs:       "chrX:g.154363738G>A",
			geneSymbol: "GLA",
			transcript: "NM_000169.3",
			expectChrom: "X",
			expectPos:  154363738,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			variant, err := service.ParseVariant(tt.hgvs)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseVariant() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if variant.Chromosome != tt.expectChrom {
					t.Errorf("Expected chromosome %s, got %s", tt.expectChrom, variant.Chromosome)
				}
				if variant.Position != tt.expectPos {
					t.Errorf("Expected position %d, got %d", tt.expectPos, variant.Position)
				}
			}
		})
	}
}

// Simple indexOf implementation
func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}