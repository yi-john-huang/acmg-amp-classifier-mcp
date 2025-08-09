package external

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/acmg-amp-mcp-server/internal/domain"
)

func TestClinVarClient_QueryVariant(t *testing.T) {
	tests := []struct {
		name           string
		variant        *domain.StandardizedVariant
		mockSearchXML  string
		mockSummaryXML string
		expectedData   *domain.ClinVarData
		expectError    bool
	}{
		{
			name: "successful variant query",
			variant: &domain.StandardizedVariant{
				HGVSGenomic: "NC_000017.11:g.43104121G>A",
				GeneSymbol:  "BRCA1",
				Chromosome:  "17",
				Position:    43104121,
				Reference:   "G",
				Alternative: "A",
			},
			mockSearchXML: `<?xml version="1.0"?>
<eSearchResult>
	<Count>1</Count>
	<IdList>
		<Id>12345</Id>
	</IdList>
</eSearchResult>`,
			mockSummaryXML: `<?xml version="1.0"?>
<eSummaryResult>
	<DocumentSummary uid="12345">
		<title>BRCA1 c.181T&gt;G (p.Cys61Gly)</title>
		<clinical_significance>
			<ReviewStatus>criteria provided, single submitter</ReviewStatus>
			<Description>Pathogenic</Description>
			<LastEvaluated>2023/01/15</LastEvaluated>
		</clinical_significance>
		<variation_set>
			<variation>
				<Name>NC_000017.11:g.43104121G&gt;A</Name>
				<VariationType>single nucleotide variant</VariationType>
			</variation>
		</variation_set>
		<trait_set>
			<trait>
				<Name>Hereditary breast and ovarian cancer syndrome</Name>
			</trait>
		</trait_set>
	</DocumentSummary>
</eSummaryResult>`,
			expectedData: &domain.ClinVarData{
				VariationID:          "12345",
				ClinicalSignificance: "Pathogenic",
				ReviewStatus:         "criteria provided, single submitter",
				Conditions:           []string{"Hereditary breast and ovarian cancer syndrome"},
			},
			expectError: false,
		},
		{
			name: "variant not found",
			variant: &domain.StandardizedVariant{
				HGVSGenomic: "NC_000001.11:g.123456C>T",
			},
			mockSearchXML: `<?xml version="1.0"?>
<eSearchResult>
	<Count>0</Count>
	<IdList>
	</IdList>
</eSearchResult>`,
			expectedData: &domain.ClinVarData{},
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server
			requestCount := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requestCount++
				w.Header().Set("Content-Type", "application/xml")
				
				if requestCount == 1 {
					// First request is search
					fmt.Fprint(w, tt.mockSearchXML)
				} else {
					// Second request is summary
					fmt.Fprint(w, tt.mockSummaryXML)
				}
			}))
			defer server.Close()

			// Create client with mock server URL
			config := domain.ClinVarConfig{
				BaseURL:    server.URL + "/",
				Timeout:    5 * time.Second,
				RateLimit:  100,
			}
			client := NewClinVarClient(config)

			// Execute query
			result, err := client.QueryVariant(context.Background(), tt.variant)

			// Assertions
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedData.VariationID, result.VariationID)
				assert.Equal(t, tt.expectedData.ClinicalSignificance, result.ClinicalSignificance)
				assert.Equal(t, tt.expectedData.ReviewStatus, result.ReviewStatus)
			}
		})
	}
}

func TestGnomADClient_QueryVariant(t *testing.T) {
	tests := []struct {
		name         string
		variant      *domain.StandardizedVariant
		mockResponse GnomADVariantResponse
		expectedData *domain.PopulationData
		expectError  bool
	}{
		{
			name: "successful variant query",
			variant: &domain.StandardizedVariant{
				Chromosome:  "17",
				Position:    43104121,
				Reference:   "G",
				Alternative: "A",
			},
			mockResponse: GnomADVariantResponse{
				Data: struct {
					Variant struct {
						VariantID string `json:"variantId"`
						Genome struct {
							AC  int     `json:"ac"`
							AN  int     `json:"an"`
							AF  float64 `json:"af"`
							Hom int     `json:"hom"`
							Populations []struct {
								ID string  `json:"id"`
								AC int     `json:"ac"`
								AN int     `json:"an"`
								AF float64 `json:"af"`
							} `json:"populations"`
							QualityMetrics struct {
								MeanDP  float64 `json:"mean_dp"`
								MeanGQ  float64 `json:"mean_gq"`
								Pass    bool    `json:"pass"`
							} `json:"qualityMetrics"`
						} `json:"genome"`
						Exome struct {
							AC  int     `json:"ac"`
							AN  int     `json:"an"`
							AF  float64 `json:"af"`
							Hom int     `json:"hom"`
							Populations []struct {
								ID string  `json:"id"`
								AC int     `json:"ac"`
								AN int     `json:"an"`
								AF float64 `json:"af"`
							} `json:"populations"`
							QualityMetrics struct {
								MeanDP  float64 `json:"mean_dp"`
								MeanGQ  float64 `json:"mean_gq"`
								Pass    bool    `json:"pass"`
							} `json:"qualityMetrics"`
						} `json:"exome"`
					} `json:"variant"`
				}{
					Variant: struct {
						VariantID string `json:"variantId"`
						Genome struct {
							AC  int     `json:"ac"`
							AN  int     `json:"an"`
							AF  float64 `json:"af"`
							Hom int     `json:"hom"`
							Populations []struct {
								ID string  `json:"id"`
								AC int     `json:"ac"`
								AN int     `json:"an"`
								AF float64 `json:"af"`
							} `json:"populations"`
							QualityMetrics struct {
								MeanDP  float64 `json:"mean_dp"`
								MeanGQ  float64 `json:"mean_gq"`
								Pass    bool    `json:"pass"`
							} `json:"qualityMetrics"`
						} `json:"genome"`
						Exome struct {
							AC  int     `json:"ac"`
							AN  int     `json:"an"`
							AF  float64 `json:"af"`
							Hom int     `json:"hom"`
							Populations []struct {
								ID string  `json:"id"`
								AC int     `json:"ac"`
								AN int     `json:"an"`
								AF float64 `json:"af"`
							} `json:"populations"`
							QualityMetrics struct {
								MeanDP  float64 `json:"mean_dp"`
								MeanGQ  float64 `json:"mean_gq"`
								Pass    bool    `json:"pass"`
							} `json:"qualityMetrics"`
						} `json:"exome"`
					}{
						VariantID: "17-43104121-G-A",
						Genome: struct {
							AC  int     `json:"ac"`
							AN  int     `json:"an"`
							AF  float64 `json:"af"`
							Hom int     `json:"hom"`
							Populations []struct {
								ID string  `json:"id"`
								AC int     `json:"ac"`
								AN int     `json:"an"`
								AF float64 `json:"af"`
							} `json:"populations"`
							QualityMetrics struct {
								MeanDP  float64 `json:"mean_dp"`
								MeanGQ  float64 `json:"mean_gq"`
								Pass    bool    `json:"pass"`
							} `json:"qualityMetrics"`
						}{
							AC:  10,
							AN:  100000,
							AF:  0.0001,
							Hom: 0,
							Populations: []struct {
								ID string  `json:"id"`
								AC int     `json:"ac"`
								AN int     `json:"an"`
								AF float64 `json:"af"`
							}{
								{ID: "afr", AC: 2, AN: 20000, AF: 0.0001},
								{ID: "eur", AC: 8, AN: 80000, AF: 0.0001},
							},
							QualityMetrics: struct {
								MeanDP  float64 `json:"mean_dp"`
								MeanGQ  float64 `json:"mean_gq"`
								Pass    bool    `json:"pass"`
							}{
								MeanDP: 30.5,
								MeanGQ: 99.0,
								Pass:   true,
							},
						},
					},
				},
			},
			expectedData: &domain.PopulationData{
				AlleleFrequency: 0.0001,
				AlleleCount:     10,
				AlleleNumber:    100000,
				HomozygoteCount: 0,
				PopulationFrequencies: map[string]float64{
					"afr": 0.0001,
					"eur": 0.0001,
				},
				QualityMetrics: &domain.QualityMetrics{
					Coverage:   30,
					Quality:    99.0,
					FilterPass: true,
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(tt.mockResponse)
			}))
			defer server.Close()

			// Create client with mock server URL
			config := domain.GnomADConfig{
				BaseURL:   server.URL,
				Timeout:   5 * time.Second,
				RateLimit: 100,
			}
			client := NewGnomADClient(config)

			// Execute query
			result, err := client.QueryVariant(context.Background(), tt.variant)

			// Assertions
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedData.AlleleFrequency, result.AlleleFrequency)
				assert.Equal(t, tt.expectedData.AlleleCount, result.AlleleCount)
				assert.Equal(t, tt.expectedData.AlleleNumber, result.AlleleNumber)
			}
		})
	}
}

func TestCOSMICClient_QueryVariant(t *testing.T) {
	tests := []struct {
		name         string
		variant      *domain.StandardizedVariant
		mockResponse COSMICVariantResponse
		expectedData *domain.SomaticData
		expectError  bool
	}{
		{
			name: "successful variant query",
			variant: &domain.StandardizedVariant{
				GeneSymbol: "TP53",
				HGVSCoding: "c.817C>T",
			},
			mockResponse: COSMICVariantResponse{
				Data: []struct {
					CosmicID              string `json:"cosmic_id"`
					GeneName              string `json:"gene_name"`
					Transcript            string `json:"transcript"`
					CDSMutation           string `json:"cds_mutation"`
					AAMutation            string `json:"aa_mutation"`
					PrimaryTissue         string `json:"primary_tissue"`
					PrimaryHistology      string `json:"primary_histology"`
					SampleCount           int    `json:"sample_count"`
					MutationCount         int    `json:"mutation_count"`
					FathmmmScore          string `json:"fathmm_score"`
					FathmmmPrediction     string `json:"fathmm_prediction"`
					MutationSomaticStatus string `json:"mutation_somatic_status"`
					Pubmed                string `json:"pubmed"`
					StudyID               string `json:"study_id"`
					Tier                  string `json:"tier"`
					HallmarkTier          string `json:"hallmark_tier"`
					CNVType               string `json:"cnv_type"`
					GenomeWideScreen      string `json:"genome_wide_screen"`
				}{
					{
						CosmicID:          "COSV12345",
						GeneName:          "TP53",
						CDSMutation:       "c.817C>T",
						AAMutation:        "p.Arg273Cys",
						PrimaryTissue:     "lung",
						PrimaryHistology:  "carcinoma",
						SampleCount:       150,
						MutationCount:     300,
						FathmmmPrediction: "pathogenic",
					},
				},
				Count: 1,
			},
			expectedData: &domain.SomaticData{
				CosmicID:      "COSV12345",
				TumorTypes:    []string{"lung", "carcinoma"},
				SampleCount:   150,
				MutationCount: 300,
				Pathogenicity: "Pathogenic",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Require API key
				if r.URL.Query().Get("api_key") == "" {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(tt.mockResponse)
			}))
			defer server.Close()

			// Create client with mock server URL
			config := domain.COSMICConfig{
				BaseURL:   server.URL,
				APIKey:    "test-api-key",
				Timeout:   5 * time.Second,
				RateLimit: 100,
			}
			client := NewCOSMICClient(config)

			// Execute query
			result, err := client.QueryVariant(context.Background(), tt.variant)

			// Assertions
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedData.CosmicID, result.CosmicID)
				assert.Equal(t, tt.expectedData.SampleCount, result.SampleCount)
				assert.Equal(t, tt.expectedData.MutationCount, result.MutationCount)
				assert.Contains(t, result.TumorTypes, "lung")
			}
		})
	}
}

func TestKnowledgeBaseService_GatherEvidence(t *testing.T) {
	// This test requires a Redis instance for caching
	// Skip if integration test environment not available
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create mock servers for each external service
	clinVarServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		if r.URL.Path == "/esearch.fcgi" {
			fmt.Fprint(w, `<?xml version="1.0"?><eSearchResult><Count>1</Count><IdList><Id>12345</Id></IdList></eSearchResult>`)
		} else {
			fmt.Fprint(w, `<?xml version="1.0"?><eSummaryResult><DocumentSummary uid="12345"><clinical_significance><Description>Pathogenic</Description></clinical_significance></DocumentSummary></eSummaryResult>`)
		}
	}))
	defer clinVarServer.Close()

	gnomADServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := GnomADVariantResponse{
			Data: struct {
				Variant struct {
					VariantID string `json:"variantId"`
					Genome struct {
						AC  int     `json:"ac"`
						AN  int     `json:"an"`
						AF  float64 `json:"af"`
						Hom int     `json:"hom"`
						Populations []struct {
							ID string  `json:"id"`
							AC int     `json:"ac"`
							AN int     `json:"an"`
							AF float64 `json:"af"`
						} `json:"populations"`
						QualityMetrics struct {
							MeanDP  float64 `json:"mean_dp"`
							MeanGQ  float64 `json:"mean_gq"`
							Pass    bool    `json:"pass"`
						} `json:"qualityMetrics"`
					} `json:"genome"`
					Exome struct {
						AC  int     `json:"ac"`
						AN  int     `json:"an"`
						AF  float64 `json:"af"`
						Hom int     `json:"hom"`
						Populations []struct {
							ID string  `json:"id"`
							AC int     `json:"ac"`
							AN int     `json:"an"`
							AF float64 `json:"af"`
						} `json:"populations"`
						QualityMetrics struct {
							MeanDP  float64 `json:"mean_dp"`
							MeanGQ  float64 `json:"mean_gq"`
							Pass    bool    `json:"pass"`
						} `json:"qualityMetrics"`
					} `json:"exome"`
				} `json:"variant"`
			}{},
		}
		response.Data.Variant.Genome.AF = 0.001
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer gnomADServer.Close()

	cosmicServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("api_key") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		
		response := COSMICVariantResponse{
			Data: []struct {
				CosmicID              string `json:"cosmic_id"`
				GeneName              string `json:"gene_name"`
				Transcript            string `json:"transcript"`
				CDSMutation           string `json:"cds_mutation"`
				AAMutation            string `json:"aa_mutation"`
				PrimaryTissue         string `json:"primary_tissue"`
				PrimaryHistology      string `json:"primary_histology"`
				SampleCount           int    `json:"sample_count"`
				MutationCount         int    `json:"mutation_count"`
				FathmmmScore          string `json:"fathmm_score"`
				FathmmmPrediction     string `json:"fathmm_prediction"`
				MutationSomaticStatus string `json:"mutation_somatic_status"`
				Pubmed                string `json:"pubmed"`
				StudyID               string `json:"study_id"`
				Tier                  string `json:"tier"`
				HallmarkTier          string `json:"hallmark_tier"`
				CNVType               string `json:"cnv_type"`
				GenomeWideScreen      string `json:"genome_wide_screen"`
			}{{CosmicID: "COSV123", SampleCount: 50}},
			Count: 1,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer cosmicServer.Close()

	// Create service configurations
	clinVarConfig := domain.ClinVarConfig{
		BaseURL:   clinVarServer.URL + "/",
		Timeout:   5 * time.Second,
		RateLimit: 100,
	}
	
	gnomADConfig := domain.GnomADConfig{
		BaseURL:   gnomADServer.URL,
		Timeout:   5 * time.Second,
		RateLimit: 100,
	}
	
	cosmicConfig := domain.COSMICConfig{
		BaseURL:   cosmicServer.URL,
		APIKey:    "test-key",
		Timeout:   5 * time.Second,
		RateLimit: 100,
	}
	
	cacheConfig := domain.CacheConfig{
		RedisURL:    "redis://localhost:6379",
		DefaultTTL:  time.Hour,
		MaxRetries:  3,
		PoolSize:    10,
		PoolTimeout: 4 * time.Second,
	}

	// Create knowledge base service
	service, err := NewKnowledgeBaseService(clinVarConfig, gnomADConfig, cosmicConfig, cacheConfig)
	if err != nil {
		t.Skipf("Failed to create service (Redis may not be available): %v", err)
	}
	defer service.Close()

	// Test variant
	variant := &domain.StandardizedVariant{
		HGVSGenomic: "NC_000017.11:g.43104121G>A",
		GeneSymbol:  "BRCA1",
		Chromosome:  "17",
		Position:    43104121,
		Reference:   "G",
		Alternative: "A",
	}

	// Gather evidence
	evidence, err := service.GatherEvidence(context.Background(), variant)
	
	// Assertions
	require.NoError(t, err)
	assert.NotNil(t, evidence)
	assert.NotNil(t, evidence.ClinVarData)
	assert.NotNil(t, evidence.PopulationData)
	assert.NotNil(t, evidence.SomaticData)
	assert.Equal(t, "Pathogenic", evidence.ClinVarData.ClinicalSignificance)
}