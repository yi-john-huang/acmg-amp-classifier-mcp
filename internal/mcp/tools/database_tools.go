package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/acmg-amp-mcp-server/internal/mcp/protocol"
)

// QueryClinVarTool implements database-specific ClinVar queries
type QueryClinVarTool struct {
	logger *logrus.Logger
}

// QueryClinVarParams defines parameters for ClinVar-specific queries
type QueryClinVarParams struct {
	HGVSNotation    string `json:"hgvs_notation,omitempty"`
	VariationID     string `json:"variation_id,omitempty"`
	GeneSymbol      string `json:"gene_symbol,omitempty"`
	ReviewStatus    string `json:"review_status,omitempty"`
	Significance    string `json:"significance,omitempty"`
	IncludeHistory  bool   `json:"include_history,omitempty"`
}

// NewQueryClinVarTool creates a new ClinVar-specific query tool
func NewQueryClinVarTool(logger *logrus.Logger) *QueryClinVarTool {
	return &QueryClinVarTool{logger: logger}
}

// HandleTool implements the ToolHandler interface for query_clinvar
func (t *QueryClinVarTool) HandleTool(ctx context.Context, req *protocol.JSONRPC2Request) *protocol.JSONRPC2Response {
	t.logger.WithField("tool", "query_clinvar").Info("Processing ClinVar query")

	var params QueryClinVarParams
	if err := t.parseAndValidateParams(req.Params, &params); err != nil {
		return &protocol.JSONRPC2Response{
			Error: &protocol.RPCError{
				Code:    protocol.InvalidParams,
				Message: "Invalid parameters",
				Data:    err.Error(),
			},
		}
	}

	result, err := t.queryClinVar(ctx, &params)
	if err != nil {
		return &protocol.JSONRPC2Response{
			Error: &protocol.RPCError{
				Code:    protocol.MCPResourceError,
				Message: "ClinVar query failed",
				Data:    err.Error(),
			},
		}
	}

	return &protocol.JSONRPC2Response{
		Result: map[string]interface{}{
			"clinvar_data": result,
		},
	}
}

// GetToolInfo returns ClinVar tool metadata
func (t *QueryClinVarTool) GetToolInfo() protocol.ToolInfo {
	return protocol.ToolInfo{
		Name:        "query_clinvar",
		Description: "Query ClinVar database for clinical significance and review status information",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"hgvs_notation": map[string]interface{}{
					"type":        "string",
					"description": "HGVS notation to search",
				},
				"variation_id": map[string]interface{}{
					"type":        "string",
					"description": "ClinVar variation ID",
				},
				"gene_symbol": map[string]interface{}{
					"type":        "string",
					"description": "Gene symbol to search",
				},
				"review_status": map[string]interface{}{
					"type": "string",
					"enum": []string{"practice guideline", "reviewed by expert panel", "criteria provided, multiple submitters, no conflicts", "criteria provided, single submitter"},
				},
				"significance": map[string]interface{}{
					"type": "string",
					"enum": []string{"Pathogenic", "Likely pathogenic", "Uncertain significance", "Likely benign", "Benign"},
				},
				"include_history": map[string]interface{}{
					"type":        "boolean",
					"description": "Include historical submissions",
					"default":     false,
				},
			},
		},
	}
}

// ValidateParams validates ClinVar query parameters
func (t *QueryClinVarTool) ValidateParams(params interface{}) error {
	var clinvarParams QueryClinVarParams
	return t.parseAndValidateParams(params, &clinvarParams)
}

// parseAndValidateParams parses and validates ClinVar parameters
func (t *QueryClinVarTool) parseAndValidateParams(params interface{}, target *QueryClinVarParams) error {
	if params == nil {
		return fmt.Errorf("missing required parameters")
	}

	paramsBytes, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("failed to marshal parameters: %w", err)
	}

	if err := json.Unmarshal(paramsBytes, target); err != nil {
		return fmt.Errorf("failed to parse parameters: %w", err)
	}

	if target.HGVSNotation == "" && target.VariationID == "" && target.GeneSymbol == "" {
		return fmt.Errorf("at least one of hgvs_notation, variation_id, or gene_symbol is required")
	}

	return nil
}

// queryClinVar performs the actual ClinVar query
func (t *QueryClinVarTool) queryClinVar(ctx context.Context, params *QueryClinVarParams) (interface{}, error) {
	// Mock ClinVar query - in production this would call the ClinVar E-utilities API
	return map[string]interface{}{
		"query_info": map[string]interface{}{
			"hgvs":        params.HGVSNotation,
			"variation_id": params.VariationID,
			"gene":        params.GeneSymbol,
			"query_date":  "2025-01-01T00:00:00Z",
		},
		"total_results": 3,
		"variations": []map[string]interface{}{
			{
				"variation_id":          "12345",
				"name":                  "NM_000492.3:c.1521_1523delCTT",
				"clinical_significance": "Pathogenic",
				"review_status":         "criteria provided, multiple submitters, no conflicts",
				"last_evaluated":        "2024-12-01",
				"submissions": []map[string]interface{}{
					{
						"submitter": "ClinGen Cystic Fibrosis Variant Curation Expert Panel",
						"method":    "clinical testing",
						"significance": "Pathogenic",
						"date":      "2024-12-01",
					},
				},
				"conditions": []map[string]interface{}{
					{
						"name":    "Cystic fibrosis",
						"medgen":  "C0010674",
					},
				},
				"molecular_consequences": []string{"frameshift_variant"},
			},
		},
	}, nil
}

// QueryGnomADTool implements database-specific gnomAD queries
type QueryGnomADTool struct {
	logger *logrus.Logger
}

// QueryGnomADParams defines parameters for gnomAD-specific queries
type QueryGnomADParams struct {
	HGVSNotation      string   `json:"hgvs_notation,omitempty"`
	GenomicPosition   string   `json:"genomic_position,omitempty"`
	GeneSymbol        string   `json:"gene_symbol,omitempty"`
	Populations       []string `json:"populations,omitempty"`
	IncludeSubsets    bool     `json:"include_subsets,omitempty"`
	QualityThreshold  float64  `json:"quality_threshold,omitempty"`
}

// NewQueryGnomADTool creates a new gnomAD-specific query tool
func NewQueryGnomADTool(logger *logrus.Logger) *QueryGnomADTool {
	return &QueryGnomADTool{logger: logger}
}

// HandleTool implements the ToolHandler interface for query_gnomad
func (t *QueryGnomADTool) HandleTool(ctx context.Context, req *protocol.JSONRPC2Request) *protocol.JSONRPC2Response {
	t.logger.WithField("tool", "query_gnomad").Info("Processing gnomAD query")

	var params QueryGnomADParams
	if err := t.parseAndValidateParams(req.Params, &params); err != nil {
		return &protocol.JSONRPC2Response{
			Error: &protocol.RPCError{
				Code:    protocol.InvalidParams,
				Message: "Invalid parameters",
				Data:    err.Error(),
			},
		}
	}

	result, err := t.queryGnomAD(ctx, &params)
	if err != nil {
		return &protocol.JSONRPC2Response{
			Error: &protocol.RPCError{
				Code:    protocol.MCPResourceError,
				Message: "gnomAD query failed",
				Data:    err.Error(),
			},
		}
	}

	return &protocol.JSONRPC2Response{
		Result: map[string]interface{}{
			"gnomad_data": result,
		},
	}
}

// GetToolInfo returns gnomAD tool metadata
func (t *QueryGnomADTool) GetToolInfo() protocol.ToolInfo {
	return protocol.ToolInfo{
		Name:        "query_gnomad",
		Description: "Query gnomAD database for population frequency and quality metrics",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"hgvs_notation": map[string]interface{}{
					"type":        "string",
					"description": "HGVS notation to search",
				},
				"genomic_position": map[string]interface{}{
					"type":        "string",
					"description": "Genomic position (chr:pos format)",
					"pattern":     "^(chr)?[0-9XYxy]+:[0-9]+$",
				},
				"gene_symbol": map[string]interface{}{
					"type":        "string",
					"description": "Gene symbol to search",
				},
				"populations": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "string",
						"enum": []string{"AFR", "AMR", "ASJ", "EAS", "FIN", "NFE", "OTH", "SAS"},
					},
					"description": "Specific populations to include",
				},
				"include_subsets": map[string]interface{}{
					"type":        "boolean",
					"description": "Include subset datasets (e.g., controls)",
					"default":     false,
				},
				"quality_threshold": map[string]interface{}{
					"type":        "number",
					"description": "Minimum quality score threshold",
					"minimum":     0,
					"maximum":     100,
				},
			},
		},
	}
}

// ValidateParams validates gnomAD query parameters
func (t *QueryGnomADTool) ValidateParams(params interface{}) error {
	var gnomadParams QueryGnomADParams
	return t.parseAndValidateParams(params, &gnomadParams)
}

// parseAndValidateParams parses and validates gnomAD parameters
func (t *QueryGnomADTool) parseAndValidateParams(params interface{}, target *QueryGnomADParams) error {
	if params == nil {
		return fmt.Errorf("missing required parameters")
	}

	paramsBytes, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("failed to marshal parameters: %w", err)
	}

	if err := json.Unmarshal(paramsBytes, target); err != nil {
		return fmt.Errorf("failed to parse parameters: %w", err)
	}

	if target.HGVSNotation == "" && target.GenomicPosition == "" && target.GeneSymbol == "" {
		return fmt.Errorf("at least one of hgvs_notation, genomic_position, or gene_symbol is required")
	}

	return nil
}

// queryGnomAD performs the actual gnomAD query
func (t *QueryGnomADTool) queryGnomAD(ctx context.Context, params *QueryGnomADParams) (interface{}, error) {
	// Mock gnomAD query - in production this would call the gnomAD GraphQL API
	return map[string]interface{}{
		"query_info": map[string]interface{}{
			"hgvs":     params.HGVSNotation,
			"position": params.GenomicPosition,
			"gene":     params.GeneSymbol,
			"version":  "v4.0.0",
			"query_date": "2025-01-01T00:00:00Z",
		},
		"variant_data": map[string]interface{}{
			"variant_id":       "7-117199644-CTT-C",
			"consequence":      "frameshift_variant",
			"allele_count":     2,
			"allele_number":    251456,
			"allele_frequency": 0.000007952,
			"homozygote_count": 0,
			"population_frequencies": map[string]interface{}{
				"AFR": map[string]interface{}{"ac": 0, "an": 24034, "af": 0.0},
				"AMR": map[string]interface{}{"ac": 0, "an": 34246, "af": 0.0},
				"ASJ": map[string]interface{}{"ac": 0, "an": 10418, "af": 0.0},
				"EAS": map[string]interface{}{"ac": 0, "an": 18862, "af": 0.0},
				"FIN": map[string]interface{}{"ac": 0, "an": 13540, "af": 0.0},
				"NFE": map[string]interface{}{"ac": 2, "an": 130372, "af": 0.000015338},
				"OTH": map[string]interface{}{"ac": 0, "an": 6782, "af": 0.0},
			},
			"quality_metrics": map[string]interface{}{
				"site_quality":      998.77,
				"inbreeding_coeff": -0.0123,
				"read_depth":        30.45,
				"allele_balance":    []float64{0.515, 0.485},
				"vqslod":           15.23,
			},
			"flags": []string{"LC_LoF", "LoF"},
		},
		"gene_constraint": map[string]interface{}{
			"gene":           "CFTR",
			"oe_lof":         0.089,
			"oe_lof_upper":   0.13,
			"oe_lof_lower":   0.061,
			"lof_z_score":    4.48,
			"constraint_flag": "lof_constrained",
		},
	}, nil
}

// QueryCOSMICTool implements database-specific COSMIC queries
type QueryCOSMICTool struct {
	logger *logrus.Logger
}

// QueryCOSMICParams defines parameters for COSMIC-specific queries
type QueryCOSMICParams struct {
	HGVSNotation   string   `json:"hgvs_notation,omitempty"`
	GeneSymbol     string   `json:"gene_symbol,omitempty"`
	CosmicID       string   `json:"cosmic_id,omitempty"`
	CancerTypes    []string `json:"cancer_types,omitempty"`
	TissueTypes    []string `json:"tissue_types,omitempty"`
	MutationTypes  []string `json:"mutation_types,omitempty"`
	IncludeSamples bool     `json:"include_samples,omitempty"`
}

// NewQueryCOSMICTool creates a new COSMIC-specific query tool
func NewQueryCOSMICTool(logger *logrus.Logger) *QueryCOSMICTool {
	return &QueryCOSMICTool{logger: logger}
}

// HandleTool implements the ToolHandler interface for query_cosmic
func (t *QueryCOSMICTool) HandleTool(ctx context.Context, req *protocol.JSONRPC2Request) *protocol.JSONRPC2Response {
	t.logger.WithField("tool", "query_cosmic").Info("Processing COSMIC query")

	var params QueryCOSMICParams
	if err := t.parseAndValidateParams(req.Params, &params); err != nil {
		return &protocol.JSONRPC2Response{
			Error: &protocol.RPCError{
				Code:    protocol.InvalidParams,
				Message: "Invalid parameters",
				Data:    err.Error(),
			},
		}
	}

	result, err := t.queryCOSMIC(ctx, &params)
	if err != nil {
		return &protocol.JSONRPC2Response{
			Error: &protocol.RPCError{
				Code:    protocol.MCPResourceError,
				Message: "COSMIC query failed",
				Data:    err.Error(),
			},
		}
	}

	return &protocol.JSONRPC2Response{
		Result: map[string]interface{}{
			"cosmic_data": result,
		},
	}
}

// GetToolInfo returns COSMIC tool metadata
func (t *QueryCOSMICTool) GetToolInfo() protocol.ToolInfo {
	return protocol.ToolInfo{
		Name:        "query_cosmic",
		Description: "Query COSMIC database for somatic mutation and cancer association data",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"hgvs_notation": map[string]interface{}{
					"type":        "string",
					"description": "HGVS notation to search",
				},
				"gene_symbol": map[string]interface{}{
					"type":        "string",
					"description": "Gene symbol to search",
				},
				"cosmic_id": map[string]interface{}{
					"type":        "string",
					"description": "COSMIC mutation ID",
				},
				"cancer_types": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "string",
					},
					"description": "Specific cancer types to filter",
				},
				"tissue_types": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "string",
					},
					"description": "Specific tissue types to filter",
				},
				"include_samples": map[string]interface{}{
					"type":        "boolean",
					"description": "Include detailed sample information",
					"default":     false,
				},
			},
		},
	}
}

// ValidateParams validates COSMIC query parameters
func (t *QueryCOSMICTool) ValidateParams(params interface{}) error {
	var cosmicParams QueryCOSMICParams
	return t.parseAndValidateParams(params, &cosmicParams)
}

// parseAndValidateParams parses and validates COSMIC parameters
func (t *QueryCOSMICTool) parseAndValidateParams(params interface{}, target *QueryCOSMICParams) error {
	if params == nil {
		return fmt.Errorf("missing required parameters")
	}

	paramsBytes, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("failed to marshal parameters: %w", err)
	}

	if err := json.Unmarshal(paramsBytes, target); err != nil {
		return fmt.Errorf("failed to parse parameters: %w", err)
	}

	if target.HGVSNotation == "" && target.GeneSymbol == "" && target.CosmicID == "" {
		return fmt.Errorf("at least one of hgvs_notation, gene_symbol, or cosmic_id is required")
	}

	return nil
}

// queryCOSMIC performs the actual COSMIC query
func (t *QueryCOSMICTool) queryCOSMIC(ctx context.Context, params *QueryCOSMICParams) (interface{}, error) {
	// Mock COSMIC query - in production this would call the COSMIC REST API
	return map[string]interface{}{
		"query_info": map[string]interface{}{
			"hgvs":        params.HGVSNotation,
			"gene":        params.GeneSymbol,
			"cosmic_id":   params.CosmicID,
			"version":     "v97",
			"query_date":  "2025-01-01T00:00:00Z",
		},
		"mutation_data": []map[string]interface{}{
			{
				"cosmic_id":        "COSM12345",
				"legacy_id":        "COSV12345",
				"mutation_cds":     "c.1521_1523delCTT",
				"mutation_aa":      "p.Phe508del",
				"mutation_type":    "Deletion - In frame",
				"fathmm_score":     0.65,
				"fathmm_prediction": "PATHOGENIC",
				"mutation_zygosity": "het",
				"sample_data": map[string]interface{}{
					"total_samples":     156,
					"mutated_samples":   12,
					"mutation_frequency": 0.077,
				},
				"tissue_distribution": map[string]interface{}{
					"lung":                45,
					"large_intestine":     23,
					"breast":              18,
					"pancreas":            12,
					"other":               58,
				},
				"cancer_type_distribution": map[string]interface{}{
					"adenocarcinoma":      78,
					"carcinoma":           45,
					"squamous_carcinoma":  33,
				},
			},
		},
		"gene_statistics": map[string]interface{}{
			"gene_symbol":          "CFTR",
			"total_mutations":      1234,
			"coding_mutations":     987,
			"samples_with_mutation": 567,
			"mutation_significance": map[string]interface{}{
				"pathogenic":        234,
				"likely_pathogenic": 123,
				"uncertain":         345,
				"likely_benign":     234,
				"benign":            45,
			},
		},
	}, nil
}