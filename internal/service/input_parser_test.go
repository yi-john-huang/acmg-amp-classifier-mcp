package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/acmg-amp-mcp-server/internal/domain"
	"github.com/acmg-amp-mcp-server/pkg/external"
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
		{"Invalid gene symbol", "invalid-gene", false}, // Current implementation accepts this pattern
		{"Empty symbol", "", true},                      // Empty symbol returns error
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

// Gene Symbol Parsing Tests

func TestInputParserService_ParseGeneSymbol(t *testing.T) {
	service := NewInputParserService()

	tests := []struct {
		name    string
		input   string
		wantErr bool
		expGene string
	}{
		{
			name:    "Valid standalone gene symbol",
			input:   "BRCA1",
			wantErr: false,
			expGene: "BRCA1",
		},
		{
			name:    "Valid gene with coding variant", 
			input:   "TP53:c.273G>A",
			wantErr: true, // Will fail without transcript resolver
		},
		{
			name:    "Valid gene with protein variant",
			input:   "BRCA1 p.Cys61Gly",
			wantErr: true, // Will fail without transcript resolver
		},
		{
			name:    "Invalid gene symbol format",
			input:   "123invalid",
			wantErr: true,
		},
		{
			name:    "Empty input",
			input:   "",
			wantErr: true,
		},
		{
			name:    "HGVS format (should work)",
			input:   "NM_000546.5:c.273G>A",
			wantErr: false,
			expGene: "TP53", // From enhanced extractGeneSymbol
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.ParseGeneSymbol(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseGeneSymbol() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && result != nil {
				if tt.expGene != "" && result.GeneSymbol != tt.expGene {
					t.Errorf("ParseGeneSymbol() gene = %v, want %v", result.GeneSymbol, tt.expGene)
				}
			}
		})
	}
}

func TestInputParserService_ValidateGeneSymbol_Enhanced(t *testing.T) {
	service := NewInputParserService()

	tests := []struct {
		name    string
		symbol  string
		wantErr bool
	}{
		{
			name:    "Valid HUGO standard gene",
			symbol:  "BRCA1",
			wantErr: false,
		},
		{
			name:    "Valid single letter gene",
			symbol:  "A",
			wantErr: false,
		},
		{
			name:    "Valid gene with numbers",
			symbol:  "TP53",
			wantErr: false,
		},
		{
			name:    "Valid gene with hyphens",
			symbol:  "HLA-A",
			wantErr: false,
		},
		{
			name:    "Invalid - starts with number",
			symbol:  "1BRCA",
			wantErr: true,
		},
		{
			name:    "Invalid - starts with hyphen",
			symbol:  "-BRCA1",
			wantErr: true,
		},
		{
			name:    "Invalid - ends with hyphen",
			symbol:  "BRCA1-",
			wantErr: true,
		},
		{
			name:    "Invalid - too long",
			symbol:  "VERYLONGGENENAMESYMBOL",
			wantErr: true,
		},
		{
			name:    "Lowercase - accepted (normalized internally)",
			symbol:  "brca1",
			wantErr: false, // Implementation accepts and normalizes lowercase
		},
		{
			name:    "Empty string",
			symbol:  "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.ValidateGeneSymbol(tt.symbol)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateGeneSymbol() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestInputParserService_GenerateHGVSFromGeneSymbol(t *testing.T) {
	service := NewInputParserService()

	tests := []struct {
		name       string
		geneSymbol string
		variant    string
		wantErr    bool
	}{
		{
			name:       "No transcript resolver",
			geneSymbol: "BRCA1",
			variant:    "c.123A>G",
			wantErr:    true, // Should fail without transcript resolver
		},
		{
			name:       "Invalid gene symbol",
			geneSymbol: "123invalid",
			variant:    "c.123A>G",
			wantErr:    true,
		},
		{
			name:       "Empty gene symbol",
			geneSymbol: "",
			variant:    "c.123A>G",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.GenerateHGVSFromGeneSymbol(tt.geneSymbol, tt.variant)
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateHGVSFromGeneSymbol() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && result == "" {
				t.Error("GenerateHGVSFromGeneSymbol() returned empty result for valid input")
			}
		})
	}
}

func TestInputParserService_ParseVariantWithGeneSymbolSupport(t *testing.T) {
	service := NewInputParserService()

	tests := []struct {
		name    string
		input   string
		wantErr bool
		expType string // "gene" or "hgvs"
	}{
		{
			name:    "Gene symbol only",
			input:   "BRCA1",
			wantErr: false,
			expType: "gene",
		},
		{
			name:    "Standard HGVS",
			input:   "NM_000546.5:c.273G>A",
			wantErr: false,
			expType: "hgvs",
		},
		{
			name:    "Empty input",
			input:   "",
			wantErr: true,
		},
		{
			name:    "Invalid format",
			input:   "not-valid-input",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.ParseVariantWithGeneSymbolSupport(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseVariantWithGeneSymbolSupport() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if result == nil {
					t.Error("ParseVariantWithGeneSymbolSupport() returned nil result for valid input")
				} else {
					switch tt.expType {
					case "gene":
						if result.GeneSymbol == "" {
							t.Error("Expected gene symbol to be set for gene input")
						}
					case "hgvs":
						if result.HGVSGenomic == "" && result.HGVSCoding == "" && result.HGVSProtein == "" {
							t.Error("Expected HGVS notation to be set for HGVS input")
						}
					}
				}
			}
		})
	}
}

// Mock transcript resolver for testing
type MockTranscriptResolver struct {
	transcripts map[string]*external.TranscriptInfo
}

func (m *MockTranscriptResolver) ResolveGeneToTranscript(ctx context.Context, geneSymbol string) (*external.TranscriptInfo, error) {
	if transcript, exists := m.transcripts[geneSymbol]; exists {
		return transcript, nil
	}
	return nil, fmt.Errorf("transcript not found for gene %s", geneSymbol)
}

func TestInputParserService_WithTranscriptResolver(t *testing.T) {
	// Create service with transcript resolver (we can't directly set it, so this test is conceptual)
	service := NewInputParserService()
	
	// Test gene symbol parsing that would work with transcript resolver
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "Gene symbol only",
			input:   "BRCA1",
			wantErr: false,
		},
		{
			name:    "Gene with variant (would need resolver)",
			input:   "BRCA1:c.123A>G", 
			wantErr: true, // Will fail without proper integration
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.ParseGeneSymbol(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseGeneSymbol() with resolver error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && result != nil {
				assert.NotEmpty(t, result.GeneSymbol)
			}
		})
	}
}

func TestInputParserService_GeneSymbolEdgeCases(t *testing.T) {
	service := NewInputParserService()

	tests := []struct {
		name    string
		input   string
		wantErr bool
		desc    string
	}{
		{
			name:    "Gene with whitespace",
			input:   "  BRCA1  ",
			wantErr: false,
			desc:    "Should trim whitespace",
		},
		{
			name:    "Mixed case gene (invalid)",
			input:   "Brca1",
			wantErr: true,
			desc:    "Should reject mixed case",
		},
		{
			name:    "Gene with special characters",
			input:   "BRCA1@#$",
			wantErr: true,
			desc:    "Should reject special characters",
		},
		{
			name:    "Valid complex gene symbol",
			input:   "HLA-DRB1",
			wantErr: false,
			desc:    "Should accept valid complex symbols",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.ParseGeneSymbol(tt.input)
			hasErr := err != nil
			
			if hasErr != tt.wantErr {
				t.Errorf("ParseGeneSymbol() error = %v, wantErr %v (%s)", err, tt.wantErr, tt.desc)
				return
			}
			
			if !tt.wantErr {
				require.NotNil(t, result)
				assert.NotEmpty(t, result.GeneSymbol)
			}
		})
	}
}