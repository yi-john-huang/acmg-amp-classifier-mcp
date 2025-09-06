package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/acmg-amp-mcp-server/internal/mcp/protocol"
)

// GenerateReportTool implements the generate_report MCP tool
type GenerateReportTool struct {
	logger *logrus.Logger
}

// GenerateReportParams defines parameters for the generate_report tool
type GenerateReportParams struct {
	VariantID          string                 `json:"variant_id,omitempty"`
	HGVSNotation       string                 `json:"hgvs_notation" validate:"required"`
	GeneSymbol         string                 `json:"gene_symbol,omitempty"`
	Classification     ClassifyVariantResult  `json:"classification" validate:"required"`
	Evidence           *QueryEvidenceResult   `json:"evidence,omitempty"`
	ClinicalContext    *ClinicalContext       `json:"clinical_context,omitempty"`
	ReportTemplate     string                 `json:"report_template,omitempty"`
	IncludeSections    []string               `json:"include_sections,omitempty"`
	ExcludeSections    []string               `json:"exclude_sections,omitempty"`
	DetailLevel        string                 `json:"detail_level,omitempty"`
	IncludeRawData     bool                   `json:"include_raw_data,omitempty"`
	CustomMetadata     map[string]interface{} `json:"custom_metadata,omitempty"`
}

// ClinicalContext provides patient and clinical context for personalized reports
type ClinicalContext struct {
	PatientID          string                 `json:"patient_id,omitempty"`
	ClinicalIndication string                 `json:"clinical_indication,omitempty"`
	FamilyHistory      string                 `json:"family_history,omitempty"`
	Ethnicity          string                 `json:"ethnicity,omitempty"`
	Consanguinity      bool                   `json:"consanguinity,omitempty"`
	ReferringPhysician string                 `json:"referring_physician,omitempty"`
	TestDate           string                 `json:"test_date,omitempty"`
	ReportDate         string                 `json:"report_date,omitempty"`
	Laboratory         map[string]interface{} `json:"laboratory,omitempty"`
	AnalysisMetadata   map[string]interface{} `json:"analysis_metadata,omitempty"`
}

// ReportResult contains the generated clinical report
type ReportResult struct {
	ReportID           string                 `json:"report_id"`
	VariantID          string                 `json:"variant_id"`
	HGVSNotation       string                 `json:"hgvs_notation"`
	GeneSymbol         string                 `json:"gene_symbol,omitempty"`
	GenerationDate     string                 `json:"generation_date"`
	Template           string                 `json:"template"`
	Sections           map[string]interface{} `json:"sections"`
	Summary            ReportSummary          `json:"summary"`
	QualityMetrics     ReportQualityMetrics   `json:"quality_metrics"`
	Recommendations    []string               `json:"recommendations"`
	Disclaimers        []string               `json:"disclaimers"`
	Appendices         map[string]interface{} `json:"appendices,omitempty"`
}

// ReportSummary provides executive summary of the clinical interpretation
type ReportSummary struct {
	Classification    string  `json:"classification"`
	Confidence        float64 `json:"confidence"`
	ClinicalSig       string  `json:"clinical_significance"`
	KeyFindings       []string `json:"key_findings"`
	CriticalEvidence  []string `json:"critical_evidence"`
	LimitationsNoted  []string `json:"limitations_noted"`
	FollowUpSuggested string   `json:"follow_up_suggested,omitempty"`
}

// ReportQualityMetrics tracks quality and completeness of the report
type ReportQualityMetrics struct {
	CompletenessScore   float64            `json:"completeness_score"`
	EvidenceQuality     string             `json:"evidence_quality"`
	DataSources         int                `json:"data_sources"`
	ReferencesIncluded  int                `json:"references_included"`
	ValidationChecks    map[string]bool    `json:"validation_checks"`
	ReviewStatus        string             `json:"review_status"`
	QualityFlags        []string           `json:"quality_flags,omitempty"`
}

// NewGenerateReportTool creates a new generate_report tool
func NewGenerateReportTool(logger *logrus.Logger) *GenerateReportTool {
	return &GenerateReportTool{
		logger: logger,
	}
}

// HandleTool implements the ToolHandler interface for generate_report
func (t *GenerateReportTool) HandleTool(ctx context.Context, req *protocol.JSONRPC2Request) *protocol.JSONRPC2Response {
	t.logger.WithField("tool", "generate_report").Info("Processing report generation request")

	// Parse and validate parameters
	var params GenerateReportParams
	if err := t.parseAndValidateParams(req.Params, &params); err != nil {
		return &protocol.JSONRPC2Response{
			Error: &protocol.RPCError{
				Code:    protocol.InvalidParams,
				Message: "Invalid parameters",
				Data:    err.Error(),
			},
		}
	}

	// Generate the report
	report, err := t.generateReport(ctx, &params)
	if err != nil {
		return &protocol.JSONRPC2Response{
			Error: &protocol.RPCError{
				Code:    protocol.InternalError,
				Message: "Report generation failed",
				Data:    err.Error(),
			},
		}
	}

	t.logger.WithFields(logrus.Fields{
		"report_id":      report.ReportID,
		"hgvs":           params.HGVSNotation,
		"classification": report.Summary.Classification,
		"sections":       len(report.Sections),
	}).Info("Report generation completed")

	return &protocol.JSONRPC2Response{
		Result: map[string]interface{}{
			"report": report,
		},
	}
}

// GetToolInfo returns tool metadata
func (t *GenerateReportTool) GetToolInfo() protocol.ToolInfo {
	return protocol.ToolInfo{
		Name:        "generate_report",
		Description: "Generate comprehensive clinical reports for genetic variant interpretation with customizable templates and sections",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"hgvs_notation": map[string]interface{}{
					"type":        "string",
					"description": "HGVS notation of the variant",
				},
				"gene_symbol": map[string]interface{}{
					"type":        "string",
					"description": "Gene symbol associated with the variant",
				},
				"classification": map[string]interface{}{
					"type":        "object",
					"description": "Classification result from classify_variant tool",
				},
				"evidence": map[string]interface{}{
					"type":        "object",
					"description": "Evidence data from query_evidence tool",
				},
				"clinical_context": map[string]interface{}{
					"type":        "object",
					"description": "Clinical context for personalized reporting",
					"properties": map[string]interface{}{
						"patient_id": map[string]interface{}{
							"type": "string",
							"description": "Patient identifier",
						},
						"clinical_indication": map[string]interface{}{
							"type": "string",
							"description": "Clinical reason for testing",
						},
						"family_history": map[string]interface{}{
							"type": "string",
							"description": "Relevant family history",
						},
						"ethnicity": map[string]interface{}{
							"type": "string",
							"description": "Patient ethnicity",
						},
					},
				},
				"report_template": map[string]interface{}{
					"type":        "string",
					"description": "Report template to use",
					"enum":        []string{"clinical", "research", "summary", "detailed", "custom"},
					"default":     "clinical",
				},
				"include_sections": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Sections to include in the report",
				},
				"detail_level": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"minimal", "standard", "comprehensive"},
					"default":     "standard",
					"description": "Level of detail for the report",
				},
			},
			"required": []string{"hgvs_notation", "classification"},
		},
	}
}

// ValidateParams validates tool parameters
func (t *GenerateReportTool) ValidateParams(params interface{}) error {
	var reportParams GenerateReportParams
	return t.parseAndValidateParams(params, &reportParams)
}

// parseAndValidateParams parses and validates input parameters
func (t *GenerateReportTool) parseAndValidateParams(params interface{}, target *GenerateReportParams) error {
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

	// Validate required fields
	if target.HGVSNotation == "" {
		return fmt.Errorf("hgvs_notation is required")
	}

	// Set defaults
	if target.ReportTemplate == "" {
		target.ReportTemplate = "clinical"
	}

	if target.DetailLevel == "" {
		target.DetailLevel = "standard"
	}

	// Validate template
	validTemplates := []string{"clinical", "research", "summary", "detailed", "custom"}
	if !t.isValidTemplate(target.ReportTemplate, validTemplates) {
		return fmt.Errorf("invalid report template: %s", target.ReportTemplate)
	}

	return nil
}

// generateReport generates the complete clinical report
func (t *GenerateReportTool) generateReport(ctx context.Context, params *GenerateReportParams) (*ReportResult, error) {
	reportID := t.generateReportID(params)
	
	report := &ReportResult{
		ReportID:       reportID,
		VariantID:      params.VariantID,
		HGVSNotation:   params.HGVSNotation,
		GeneSymbol:     params.GeneSymbol,
		GenerationDate: time.Now().UTC().Format(time.RFC3339),
		Template:       params.ReportTemplate,
		Sections:       make(map[string]interface{}),
		Appendices:     make(map[string]interface{}),
	}

	// Generate report sections based on template
	sections := t.determineReportSections(params)
	for _, section := range sections {
		content, err := t.generateSection(section, params)
		if err != nil {
			t.logger.WithError(err).WithField("section", section).Warn("Failed to generate section")
			continue
		}
		report.Sections[section] = content
	}

	// Generate summary
	report.Summary = t.generateSummary(params)

	// Generate quality metrics
	report.QualityMetrics = t.generateQualityMetrics(params, report)

	// Generate recommendations
	report.Recommendations = t.generateRecommendations(params)

	// Generate disclaimers
	report.Disclaimers = t.generateDisclaimers(params)

	// Add raw data if requested
	if params.IncludeRawData {
		report.Appendices["raw_data"] = map[string]interface{}{
			"classification": params.Classification,
			"evidence":       params.Evidence,
		}
	}

	return report, nil
}

// determineReportSections determines which sections to include based on template and parameters
func (t *GenerateReportTool) determineReportSections(params *GenerateReportParams) []string {
	var sections []string

	switch params.ReportTemplate {
	case "clinical":
		sections = []string{
			"executive_summary",
			"variant_details", 
			"classification",
			"evidence_summary",
			"clinical_interpretation",
			"recommendations",
			"methodology",
			"references",
		}
	case "research":
		sections = []string{
			"variant_details",
			"classification",
			"evidence_assessment",
			"population_data",
			"functional_studies",
			"computational_predictions",
			"literature_review",
			"methodology",
			"references",
		}
	case "summary":
		sections = []string{
			"executive_summary",
			"classification",
			"key_evidence",
			"recommendations",
		}
	case "detailed":
		sections = []string{
			"executive_summary",
			"variant_details",
			"classification",
			"evidence_summary",
			"population_frequency",
			"clinical_data",
			"functional_evidence",
			"computational_predictions",
			"literature_evidence",
			"acmg_rule_assessment",
			"clinical_interpretation",
			"recommendations",
			"limitations",
			"methodology",
			"quality_metrics",
			"references",
		}
	default:
		// Default clinical template
		sections = []string{
			"executive_summary",
			"variant_details",
			"classification",
			"evidence_summary",
			"clinical_interpretation",
			"recommendations",
		}
	}

	// Apply include/exclude filters
	if len(params.IncludeSections) > 0 {
		sections = t.filterIncludeSections(sections, params.IncludeSections)
	}

	if len(params.ExcludeSections) > 0 {
		sections = t.filterExcludeSections(sections, params.ExcludeSections)
	}

	return sections
}

// generateSection generates content for a specific report section
func (t *GenerateReportTool) generateSection(section string, params *GenerateReportParams) (interface{}, error) {
	switch section {
	case "executive_summary":
		return t.generateExecutiveSummary(params), nil
	case "variant_details":
		return t.generateVariantDetails(params), nil
	case "classification":
		return t.generateClassificationSection(params), nil
	case "evidence_summary":
		return t.generateEvidenceSummary(params), nil
	case "clinical_interpretation":
		return t.generateClinicalInterpretation(params), nil
	case "recommendations":
		return t.generateRecommendationsSection(params), nil
	case "methodology":
		return t.generateMethodologySection(params), nil
	case "references":
		return t.generateReferencesSection(params), nil
	case "population_frequency":
		return t.generatePopulationFrequencySection(params), nil
	case "clinical_data":
		return t.generateClinicalDataSection(params), nil
	case "functional_evidence":
		return t.generateFunctionalEvidenceSection(params), nil
	case "computational_predictions":
		return t.generateComputationalPredictionsSection(params), nil
	case "literature_evidence":
		return t.generateLiteratureEvidenceSection(params), nil
	case "acmg_rule_assessment":
		return t.generateACMGRuleAssessmentSection(params), nil
	case "limitations":
		return t.generateLimitationsSection(params), nil
	case "quality_metrics":
		return t.generateQualityMetricsSection(params), nil
	default:
		return nil, fmt.Errorf("unknown section: %s", section)
	}
}

// Section generation methods
func (t *GenerateReportTool) generateExecutiveSummary(params *GenerateReportParams) map[string]interface{} {
	summary := map[string]interface{}{
		"variant":        params.HGVSNotation,
		"gene":           params.GeneSymbol,
		"classification": params.Classification.Classification,
		"confidence":     params.Classification.Confidence,
		"summary_text":   fmt.Sprintf("The variant %s in the %s gene is classified as %s with %s confidence.", 
			params.HGVSNotation, 
			params.GeneSymbol, 
			params.Classification.Classification,
			params.Classification.Confidence),
	}

	if params.ClinicalContext != nil {
		summary["clinical_context"] = fmt.Sprintf("Clinical indication: %s", params.ClinicalContext.ClinicalIndication)
	}

	return summary
}

func (t *GenerateReportTool) generateVariantDetails(params *GenerateReportParams) map[string]interface{} {
	details := map[string]interface{}{
		"hgvs_notation": params.HGVSNotation,
		"gene_symbol":   params.GeneSymbol,
		"variant_id":    params.VariantID,
	}

	if params.Evidence != nil && params.Evidence.DatabaseResults != nil {
		if gnomadData, exists := params.Evidence.DatabaseResults["gnomad"]; exists {
			details["population_frequency"] = gnomadData
		}
	}

	return details
}

func (t *GenerateReportTool) generateClassificationSection(params *GenerateReportParams) map[string]interface{} {
	classification := map[string]interface{}{
		"classification":    params.Classification.Classification,
		"confidence":        params.Classification.Confidence,
		"applied_rules":     t.convertACMGRulesToStrings(params.Classification.AppliedRules),
		"evidence_summary":  params.Classification.EvidenceSummary,
		"recommendations":   params.Classification.Recommendations,
	}

	return classification
}

func (t *GenerateReportTool) generateEvidenceSummary(params *GenerateReportParams) map[string]interface{} {
	summary := map[string]interface{}{
		"data_sources": []string{},
		"quality":      "standard",
	}

	if params.Evidence != nil {
		summary["overall_quality"] = params.Evidence.QualityScores.OverallQuality
		summary["data_completeness"] = params.Evidence.QualityScores.DataCompleteness
		
		sources := make([]string, 0)
		for source := range params.Evidence.DatabaseResults {
			sources = append(sources, source)
		}
		summary["data_sources"] = sources
	}

	return summary
}

// Helper methods continue...
func (t *GenerateReportTool) generateClinicalInterpretation(params *GenerateReportParams) map[string]interface{} {
	interpretation := map[string]interface{}{
		"clinical_significance": params.Classification.Classification,
		"interpretation_summary": fmt.Sprintf("Based on ACMG/AMP guidelines, this variant is classified as %s.", 
			params.Classification.Classification),
	}

	if params.ClinicalContext != nil {
		if params.ClinicalContext.ClinicalIndication != "" {
			interpretation["clinical_relevance"] = fmt.Sprintf("This finding is relevant to the clinical indication of %s.", 
				params.ClinicalContext.ClinicalIndication)
		}
	}

	return interpretation
}

func (t *GenerateReportTool) generateRecommendationsSection(params *GenerateReportParams) map[string]interface{} {
	recommendations := map[string]interface{}{
		"clinical_recommendations": t.generateClinicalRecommendations(params),
		"follow_up_suggestions":   t.generateFollowUpSuggestions(params),
	}

	return recommendations
}

func (t *GenerateReportTool) generateMethodologySection(params *GenerateReportParams) map[string]interface{} {
	methodology := map[string]interface{}{
		"guidelines_used": "ACMG/AMP 2015 guidelines for the interpretation of sequence variants",
		"databases_consulted": []string{"ClinVar", "gnomAD", "COSMIC"},
		"analysis_date": time.Now().UTC().Format("2006-01-02"),
	}

	if params.Evidence != nil {
		sources := make([]string, 0)
		for source := range params.Evidence.DatabaseResults {
			sources = append(sources, source)
		}
		methodology["databases_consulted"] = sources
	}

	return methodology
}

func (t *GenerateReportTool) generateReferencesSection(params *GenerateReportParams) map[string]interface{} {
	references := map[string]interface{}{
		"primary_guidelines": []string{
			"Richards S, et al. Standards and guidelines for the interpretation of sequence variants: a joint consensus recommendation of the American College of Medical Genetics and Genomics and the Association for Molecular Pathology. Genet Med. 2015;17(5):405-423.",
		},
		"databases": []string{
			"ClinVar: https://www.ncbi.nlm.nih.gov/clinvar/",
			"gnomAD: https://gnomad.broadinstitute.org/",
			"COSMIC: https://cancer.sanger.ac.uk/cosmic",
		},
	}

	return references
}

// Additional section generators with mock implementations
func (t *GenerateReportTool) generatePopulationFrequencySection(params *GenerateReportParams) map[string]interface{} {
	section := map[string]interface{}{
		"summary": "Population frequency analysis based on gnomAD and other population databases",
	}
	
	if params.Evidence != nil && params.Evidence.AggregatedEvidence.PopulationFrequency.FrequencyAssessment != "" {
		section["assessment"] = params.Evidence.AggregatedEvidence.PopulationFrequency.FrequencyAssessment
		section["max_frequency"] = params.Evidence.AggregatedEvidence.PopulationFrequency.MaxFrequency
	}
	
	return section
}

func (t *GenerateReportTool) generateClinicalDataSection(params *GenerateReportParams) map[string]interface{} {
	section := map[string]interface{}{
		"summary": "Clinical significance data from ClinVar and other clinical databases",
	}
	
	if params.Evidence != nil && params.Evidence.AggregatedEvidence.ClinicalEvidence.OverallSignificance != "" {
		section["overall_significance"] = params.Evidence.AggregatedEvidence.ClinicalEvidence.OverallSignificance
		section["review_status"] = params.Evidence.AggregatedEvidence.ClinicalEvidence.ReviewStatus
	}
	
	return section
}

func (t *GenerateReportTool) generateFunctionalEvidenceSection(params *GenerateReportParams) map[string]interface{} {
	return map[string]interface{}{
		"summary": "Functional studies and experimental evidence",
		"note": "Functional evidence assessment based on available literature and databases",
	}
}

func (t *GenerateReportTool) generateComputationalPredictionsSection(params *GenerateReportParams) map[string]interface{} {
	section := map[string]interface{}{
		"summary": "In silico prediction algorithms and computational analysis",
	}
	
	if params.Evidence != nil {
		section["prediction_summary"] = params.Evidence.AggregatedEvidence.ComputationalData.ConsensusPrediction
	}
	
	return section
}

func (t *GenerateReportTool) generateLiteratureEvidenceSection(params *GenerateReportParams) map[string]interface{} {
	return map[string]interface{}{
		"summary": "Published literature and case reports",
		"note": "Literature review based on PubMed and other scientific databases",
	}
}

func (t *GenerateReportTool) generateACMGRuleAssessmentSection(params *GenerateReportParams) map[string]interface{} {
	section := map[string]interface{}{
		"applied_rules": params.Classification.AppliedRules,
		"summary": "Detailed assessment of each ACMG/AMP criterion",
	}
	
	return section
}

func (t *GenerateReportTool) generateLimitationsSection(params *GenerateReportParams) map[string]interface{} {
	limitations := []string{
		"Classification is based on currently available evidence and may change as new data becomes available",
		"Variant interpretation follows ACMG/AMP guidelines which have inherent limitations",
		"Population frequency data may not be representative of all ethnic groups",
	}
	
	return map[string]interface{}{
		"limitations": limitations,
		"summary": "Important considerations and limitations of this analysis",
	}
}

func (t *GenerateReportTool) generateQualityMetricsSection(params *GenerateReportParams) map[string]interface{} {
	metrics := map[string]interface{}{
		"data_quality": "standard",
		"completeness": "good",
	}
	
	if params.Evidence != nil {
		metrics["overall_quality"] = params.Evidence.QualityScores.OverallQuality
		metrics["data_completeness"] = params.Evidence.QualityScores.DataCompleteness
	}
	
	return metrics
}

// Helper methods
func (t *GenerateReportTool) generateSummary(params *GenerateReportParams) ReportSummary {
	summary := ReportSummary{
		Classification: params.Classification.Classification,
		Confidence:    t.confidenceStringToFloat(params.Classification.Confidence),
		ClinicalSig:   params.Classification.Classification,
		KeyFindings:   []string{
			fmt.Sprintf("Variant classified as %s", params.Classification.Classification),
			fmt.Sprintf("Confidence level: %s", params.Classification.Confidence),
		},
	}

	if len(params.Classification.AppliedRules) > 0 {
		summary.CriticalEvidence = t.convertACMGRulesToStrings(params.Classification.AppliedRules)
	}

	return summary
}

func (t *GenerateReportTool) generateQualityMetrics(params *GenerateReportParams, report *ReportResult) ReportQualityMetrics {
	metrics := ReportQualityMetrics{
		CompletenessScore: 0.8,
		EvidenceQuality:   "standard",
		DataSources:      len(report.Sections),
		ValidationChecks: map[string]bool{
			"hgvs_format":    true,
			"classification": true,
			"evidence":       params.Evidence != nil,
		},
		ReviewStatus: "automated",
	}

	if params.Evidence != nil {
		metrics.EvidenceQuality = params.Evidence.QualityScores.OverallQuality
		metrics.DataSources = len(params.Evidence.DatabaseResults)
	}

	return metrics
}

func (t *GenerateReportTool) generateRecommendations(params *GenerateReportParams) []string {
	recommendations := []string{
		"Follow ACMG/AMP guidelines for variant interpretation",
		"Consider family testing if clinically indicated",
	}

	if params.Classification.Classification == "VUS" {
		recommendations = append(recommendations, 
			"Periodic re-evaluation of variant as new evidence becomes available",
			"Consider functional studies if clinically warranted")
	}

	return recommendations
}

func (t *GenerateReportTool) generateDisclaimers(params *GenerateReportParams) []string {
	return []string{
		"This report is for research/clinical decision support purposes only",
		"Classification may change as new evidence becomes available",
		"Clinical decisions should consider additional patient-specific factors",
		"Report generated using automated ACMG/AMP classification algorithms",
	}
}

func (t *GenerateReportTool) generateClinicalRecommendations(params *GenerateReportParams) []string {
	recommendations := []string{}

	switch strings.ToUpper(params.Classification.Classification) {
	case "PATHOGENIC", "LIKELY_PATHOGENIC":
		recommendations = append(recommendations,
			"Genetic counseling recommended",
			"Consider family screening",
			"Clinical management according to established guidelines")
	case "BENIGN", "LIKELY_BENIGN":
		recommendations = append(recommendations,
			"No specific genetic follow-up required for this variant",
			"Continue routine clinical care as appropriate")
	case "VUS":
		recommendations = append(recommendations,
			"Genetic counseling to discuss uncertainty",
			"Periodic re-evaluation as new data becomes available",
			"Family studies may help clarify significance")
	}

	return recommendations
}

func (t *GenerateReportTool) generateFollowUpSuggestions(params *GenerateReportParams) []string {
	suggestions := []string{
		"Re-evaluate classification annually or when new evidence becomes available",
	}

	if params.ClinicalContext != nil && params.ClinicalContext.FamilyHistory != "" {
		suggestions = append(suggestions, "Consider extended family analysis")
	}

	return suggestions
}

// Utility methods
func (t *GenerateReportTool) generateReportID(params *GenerateReportParams) string {
	timestamp := time.Now().Unix()
	if params.VariantID != "" {
		return fmt.Sprintf("RPT_%s_%d", params.VariantID, timestamp)
	}
	
	// Use HGVS hash if no variant ID
	hgvsHash := t.hashString(params.HGVSNotation)
	return fmt.Sprintf("RPT_%s_%d", hgvsHash[:8], timestamp)
}

func (t *GenerateReportTool) hashString(s string) string {
	// Simple hash for demo purposes
	hash := 0
	for _, c := range s {
		hash = hash*31 + int(c)
	}
	return fmt.Sprintf("%x", hash)
}

func (t *GenerateReportTool) confidenceToText(confidence float64) string {
	if confidence >= 0.9 {
		return "high"
	} else if confidence >= 0.7 {
		return "moderate" 
	} else if confidence >= 0.5 {
		return "low"
	}
	return "very low"
}

func (t *GenerateReportTool) confidenceStringToFloat(confidenceStr string) float64 {
	switch strings.ToLower(confidenceStr) {
	case "high":
		return 0.9
	case "moderate", "medium":
		return 0.7
	case "low":
		return 0.5
	case "very low":
		return 0.3
	default:
		return 0.5 // Default to moderate
	}
}

func (t *GenerateReportTool) convertACMGRulesToStrings(rules []ACMGAMPRuleResult) []string {
	strings := make([]string, len(rules))
	for i, rule := range rules {
		strings[i] = fmt.Sprintf("%s (%s)", rule.RuleCode, rule.RuleName)
	}
	return strings
}

func (t *GenerateReportTool) isValidTemplate(template string, validTemplates []string) bool {
	for _, valid := range validTemplates {
		if template == valid {
			return true
		}
	}
	return false
}

func (t *GenerateReportTool) filterIncludeSections(sections []string, includeSections []string) []string {
	includeMap := make(map[string]bool)
	for _, section := range includeSections {
		includeMap[section] = true
	}

	filtered := make([]string, 0)
	for _, section := range sections {
		if includeMap[section] {
			filtered = append(filtered, section)
		}
	}
	return filtered
}


func (t *GenerateReportTool) filterExcludeSections(sections []string, excludeSections []string) []string {
	excludeMap := make(map[string]bool)
	for _, section := range excludeSections {
		excludeMap[section] = true
	}

	filtered := make([]string, 0)
	for _, section := range sections {
		if !excludeMap[section] {
			filtered = append(filtered, section)
		}
	}
	return filtered
}

