package prompts

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// ReportGenerationPrompt provides clinical report customization guidance
type ReportGenerationPrompt struct {
	logger    *logrus.Logger
	renderer  *TemplateRenderer
	validator *ArgumentValidator
}

// NewReportGenerationPrompt creates a new report generation prompt template
func NewReportGenerationPrompt(logger *logrus.Logger) *ReportGenerationPrompt {
	return &ReportGenerationPrompt{
		logger:    logger,
		renderer:  NewTemplateRenderer(logger),
		validator: NewArgumentValidator(logger),
	}
}

// GetPromptInfo returns metadata about this prompt template
func (rgp *ReportGenerationPrompt) GetPromptInfo() PromptInfo {
	return PromptInfo{
		Name:        "report_generation",
		Description: "Comprehensive guidance for generating clinical genetic testing reports with customizable templates and compliance standards",
		Version:     "1.0.0",
		Arguments: []ArgumentInfo{
			{
				Name:        "report_type",
				Description: "Type of clinical report to generate",
				Type:        "string",
				Required:    true,
				Examples:    []string{"clinical", "research", "summary", "detailed", "consultation"},
				Constraints: []string{"enum:clinical,research,summary,detailed,consultation,screening,pharmacogenomic"},
			},
			{
				Name:        "interpretation_data",
				Description: "Interpretation results and classification data",
				Type:        "object",
				Required:    true,
				Examples:    []string{"{\"classification\": \"pathogenic\", \"confidence\": \"high\"}", "{\"classification\": \"uncertain_significance\", \"confidence\": \"moderate\"}"},
			},
			{
				Name:        "patient_context",
				Description: "Patient-specific clinical context and demographics",
				Type:        "object",
				Required:    false,
				Examples:    []string{"{\"age\": 45, \"sex\": \"female\", \"indication\": \"family_history\"}", "{\"pregnancy\": true, \"gestational_age\": 20}"},
			},
			{
				Name:        "report_format",
				Description: "Output format for the generated report",
				Type:        "string",
				Required:    false,
				DefaultValue: "structured",
				Examples:    []string{"structured", "narrative", "tabular", "hybrid"},
				Constraints: []string{"enum:structured,narrative,tabular,hybrid,template"},
			},
			{
				Name:        "compliance_standards",
				Description: "Regulatory and professional compliance standards to follow",
				Type:        "array",
				Required:    false,
				DefaultValue: []string{"acmg", "amp"},
				Examples:    []string{"[\"acmg\", \"amp\", \"clia\"]", "[\"iso15189\", \"cap\", \"acmg\"]"},
				Constraints: []string{"enum:acmg,amp,clia,cap,iso15189,fda,ema,nccls"},
			},
			{
				Name:        "detail_level",
				Description: "Level of detail to include in the report",
				Type:        "string",
				Required:    false,
				DefaultValue: "comprehensive",
				Examples:    []string{"minimal", "standard", "comprehensive", "verbose"},
				Constraints: []string{"enum:minimal,standard,comprehensive,verbose"},
			},
			{
				Name:        "audience",
				Description: "Target audience for the report",
				Type:        "string",
				Required:    false,
				DefaultValue: "clinician",
				Examples:    []string{"clinician", "patient", "laboratory", "researcher", "genetic_counselor"},
				Constraints: []string{"enum:clinician,patient,laboratory,researcher,genetic_counselor,insurance,legal"},
			},
			{
				Name:        "include_sections",
				Description: "Specific sections to include in the report",
				Type:        "array",
				Required:    false,
				Examples:    []string{"[\"summary\", \"interpretation\", \"recommendations\"]", "[\"methods\", \"results\", \"limitations\"]"},
				Constraints: []string{"enum:summary,interpretation,recommendations,methods,results,limitations,references,appendices,quality_metrics"},
			},
			{
				Name:        "language",
				Description: "Language for report generation",
				Type:        "string",
				Required:    false,
				DefaultValue: "en",
				Examples:    []string{"en", "es", "fr", "de", "it"},
				Constraints: []string{"enum:en,es,fr,de,it,pt,nl,sv,no,da"},
			},
			{
				Name:        "template_customization",
				Description: "Custom template parameters and styling preferences",
				Type:        "object",
				Required:    false,
				Examples:    []string{"{\"logo\": true, \"color_scheme\": \"blue\", \"footer_text\": \"Custom footer\"}", "{\"header_style\": \"minimal\", \"table_style\": \"bordered\"}"},
			},
			{
				Name:        "quality_assurance",
				Description: "Quality assurance and validation requirements",
				Type:        "boolean",
				Required:    false,
				DefaultValue: true,
			},
			{
				Name:        "confidentiality_level",
				Description: "Confidentiality and security requirements",
				Type:        "string",
				Required:    false,
				DefaultValue: "standard",
				Examples:    []string{"public", "standard", "confidential", "restricted"},
				Constraints: []string{"enum:public,standard,confidential,restricted,classified"},
			},
		},
		Examples: []PromptExample{
			{
				Name:        "Standard clinical report",
				Description: "Comprehensive clinical report for diagnostic purposes",
				Arguments: map[string]interface{}{
					"report_type":        "clinical",
					"interpretation_data": map[string]interface{}{"classification": "pathogenic", "confidence": "high"},
					"patient_context":    map[string]interface{}{"age": 45, "sex": "female", "indication": "family_history"},
					"report_format":      "structured",
					"compliance_standards": []string{"acmg", "amp", "clia"},
					"detail_level":       "comprehensive",
					"audience":           "clinician",
				},
				ExpectedUse: "Complete clinical diagnostic report for healthcare provider",
			},
			{
				Name:        "Patient-friendly summary",
				Description: "Simplified report for patient education",
				Arguments: map[string]interface{}{
					"report_type":         "summary",
					"interpretation_data": map[string]interface{}{"classification": "likely_pathogenic", "confidence": "moderate"},
					"report_format":       "narrative",
					"detail_level":        "standard",
					"audience":            "patient",
					"language":            "en",
				},
				ExpectedUse: "Patient education and genetic counseling support",
			},
		},
		Tags:       []string{"report", "clinical", "documentation", "compliance", "customization"},
		Category:   "clinical_reporting",
		Difficulty: "intermediate",
		UsageNotes: []string{
			"Ensure compliance with specified regulatory standards",
			"Customize content and language appropriate for target audience",
			"Include appropriate disclaimers and limitations",
			"Maintain patient confidentiality and privacy standards",
			"Validate report completeness and accuracy before finalization",
		},
		Metadata: map[string]interface{}{
			"compliance_frameworks": []string{"ACMG/AMP", "CLIA", "CAP", "ISO 15189"},
			"target_users":         []string{"laboratory_directors", "clinical_geneticists", "genetic_counselors", "laboratory_technicians"},
			"complexity":           "moderate_to_high",
			"time_estimate":        "20-45 minutes",
		},
	}
}

// RenderPrompt renders the prompt with given arguments
func (rgp *ReportGenerationPrompt) RenderPrompt(ctx context.Context, args map[string]interface{}) (*RenderedPrompt, error) {
	rgp.logger.WithField("args", args).Debug("Rendering report generation prompt")

	// Extract arguments with defaults
	reportType := rgp.getStringArg(args, "report_type", "")
	interpretationData := rgp.getObjectArg(args, "interpretation_data", map[string]interface{}{})
	patientContext := rgp.getObjectArg(args, "patient_context", map[string]interface{}{})
	reportFormat := rgp.getStringArg(args, "report_format", "structured")
	complianceStandards := rgp.getArrayArg(args, "compliance_standards", []string{"acmg", "amp"})
	detailLevel := rgp.getStringArg(args, "detail_level", "comprehensive")
	audience := rgp.getStringArg(args, "audience", "clinician")
	includeSections := rgp.getArrayArg(args, "include_sections", []string{})
	language := rgp.getStringArg(args, "language", "en")
	templateCustomization := rgp.getObjectArg(args, "template_customization", map[string]interface{}{})
	qualityAssurance := rgp.getBoolArg(args, "quality_assurance", true)
	confidentialityLevel := rgp.getStringArg(args, "confidentiality_level", "standard")

	// Build the prompt content
	content := rgp.buildPromptContent(reportType, interpretationData, patientContext, reportFormat,
		complianceStandards, detailLevel, audience, includeSections, language, templateCustomization,
		qualityAssurance, confidentialityLevel)

	// Generate system and user prompts
	systemPrompt := rgp.buildSystemPrompt(reportType, audience, complianceStandards, detailLevel)
	userPrompt := rgp.buildUserPrompt(reportType, interpretationData, audience)

	// Create rendered prompt
	rendered := &RenderedPrompt{
		Name:         "report_generation",
		Content:      content,
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		Context:      rgp.buildContextSection(reportType, audience, complianceStandards),
		Instructions: rgp.buildInstructions(reportFormat, detailLevel, qualityAssurance),
		Examples:     rgp.buildExamples(reportType, audience, detailLevel),
		References:   rgp.buildReferences(),
		Arguments:    args,
		GeneratedAt:  time.Now(),
		Metadata: map[string]interface{}{
			"report_type":          reportType,
			"audience":             audience,
			"detail_level":         detailLevel,
			"compliance_standards": complianceStandards,
			"language":             language,
			"generated_by":         "report_generation_prompt_v1.0.0",
		},
	}

	rgp.logger.WithFields(logrus.Fields{
		"report_type":      reportType,
		"audience":         audience,
		"detail_level":     detailLevel,
		"content_length":   len(content),
	}).Info("Generated report generation prompt")

	return rendered, nil
}

// ValidateArguments validates the provided arguments
func (rgp *ReportGenerationPrompt) ValidateArguments(args map[string]interface{}) error {
	return rgp.validator.ValidateArguments(args, rgp.GetPromptInfo().Arguments)
}

// GetArgumentSchema returns the JSON schema for prompt arguments
func (rgp *ReportGenerationPrompt) GetArgumentSchema() map[string]interface{} {
	return map[string]interface{}{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type":    "object",
		"properties": map[string]interface{}{
			"report_type": map[string]interface{}{
				"type":        "string",
				"description": "Type of clinical report to generate",
				"enum":        []string{"clinical", "research", "summary", "detailed", "consultation", "screening", "pharmacogenomic"},
			},
			"interpretation_data": map[string]interface{}{
				"type":        "object",
				"description": "Interpretation results and classification data",
				"properties": map[string]interface{}{
					"classification": map[string]interface{}{
						"type": "string",
						"enum": []string{"pathogenic", "likely_pathogenic", "uncertain_significance", "likely_benign", "benign"},
					},
					"confidence": map[string]interface{}{
						"type": "string",
						"enum": []string{"high", "moderate", "low"},
					},
				},
			},
			"report_format": map[string]interface{}{
				"type":        "string",
				"description": "Output format for the generated report",
				"enum":        []string{"structured", "narrative", "tabular", "hybrid", "template"},
				"default":     "structured",
			},
			"compliance_standards": map[string]interface{}{
				"type":        "array",
				"description": "Regulatory and professional compliance standards",
				"items": map[string]interface{}{
					"type": "string",
					"enum": []string{"acmg", "amp", "clia", "cap", "iso15189", "fda", "ema", "nccls"},
				},
				"default": []string{"acmg", "amp"},
			},
			"detail_level": map[string]interface{}{
				"type":        "string",
				"description": "Level of detail to include",
				"enum":        []string{"minimal", "standard", "comprehensive", "verbose"},
				"default":     "comprehensive",
			},
			"audience": map[string]interface{}{
				"type":        "string",
				"description": "Target audience for the report",
				"enum":        []string{"clinician", "patient", "laboratory", "researcher", "genetic_counselor", "insurance", "legal"},
				"default":     "clinician",
			},
		},
		"required": []string{"report_type", "interpretation_data"},
	}
}

// SupportsPrompt checks if this template can handle the given prompt name
func (rgp *ReportGenerationPrompt) SupportsPrompt(name string) bool {
	supportedNames := []string{
		"report_generation",
		"report-generation",
		"clinical_report",
		"clinical-report",
		"generate_report",
		"generate-report",
		"report_writing",
		"report-writing",
	}
	
	for _, supported := range supportedNames {
		if name == supported {
			return true
		}
	}
	
	return false
}

// Helper methods for argument extraction
func (rgp *ReportGenerationPrompt) getStringArg(args map[string]interface{}, key, defaultValue string) string {
	if value, exists := args[key]; exists {
		if str, ok := value.(string); ok {
			return str
		}
	}
	return defaultValue
}

func (rgp *ReportGenerationPrompt) getArrayArg(args map[string]interface{}, key string, defaultValue []string) []string {
	if value, exists := args[key]; exists {
		if arr, ok := value.([]interface{}); ok {
			result := make([]string, len(arr))
			for i, item := range arr {
				if str, ok := item.(string); ok {
					result[i] = str
				}
			}
			return result
		}
	}
	return defaultValue
}

func (rgp *ReportGenerationPrompt) getObjectArg(args map[string]interface{}, key string, defaultValue map[string]interface{}) map[string]interface{} {
	if value, exists := args[key]; exists {
		if obj, ok := value.(map[string]interface{}); ok {
			return obj
		}
	}
	return defaultValue
}

func (rgp *ReportGenerationPrompt) getBoolArg(args map[string]interface{}, key string, defaultValue bool) bool {
	if value, exists := args[key]; exists {
		if b, ok := value.(bool); ok {
			return b
		}
	}
	return defaultValue
}

// buildPromptContent builds the main prompt content
func (rgp *ReportGenerationPrompt) buildPromptContent(reportType string, interpretationData, patientContext map[string]interface{},
	reportFormat string, complianceStandards []string, detailLevel, audience string, includeSections []string,
	language string, templateCustomization map[string]interface{}, qualityAssurance bool, confidentialityLevel string) string {

	sections := map[string]string{
		"title":        "Clinical Genetic Testing Report Generation",
		"overview":     rgp.buildOverviewSection(reportType, audience, detailLevel),
		"objective":    rgp.buildObjectiveSection(reportType, interpretationData),
		"context":      rgp.buildReportContextSection(patientContext, complianceStandards, confidentialityLevel),
		"instructions": strings.Join(rgp.buildInstructions(reportFormat, detailLevel, qualityAssurance), "\n"),
		"steps":        rgp.buildStepsSection(reportType, reportFormat, includeSections, audience),
		"guidelines":   rgp.buildGuidelinesSection(complianceStandards, audience),
		"examples":     strings.Join(rgp.buildExamples(reportType, audience, detailLevel), "\n\n"),
		"references":   strings.Join(rgp.buildReferences(), "\n"),
		"notes":        rgp.buildNotesSection(confidentialityLevel, qualityAssurance),
	}

	return rgp.renderer.RenderMarkdown(sections)
}

// buildOverviewSection builds the overview section
func (rgp *ReportGenerationPrompt) buildOverviewSection(reportType, audience, detailLevel string) string {
	var overview strings.Builder
	
	overview.WriteString(fmt.Sprintf("This prompt guides the generation of %s genetic testing reports ", reportType))
	overview.WriteString(fmt.Sprintf("tailored for %s with %s level of detail. ", audience, detailLevel))
	
	switch reportType {
	case "clinical":
		overview.WriteString("Clinical reports provide diagnostic interpretation and clinical recommendations for patient care.")
	case "research":
		overview.WriteString("Research reports emphasize scientific methodology, statistical analysis, and research implications.")
	case "summary":
		overview.WriteString("Summary reports provide concise interpretation focused on key findings and actionable recommendations.")
	case "detailed":
		overview.WriteString("Detailed reports include comprehensive analysis, extensive evidence review, and thorough documentation.")
	case "consultation":
		overview.WriteString("Consultation reports provide expert opinion and specialized interpretation for complex cases.")
	case "screening":
		overview.WriteString("Screening reports focus on population-level findings and public health implications.")
	case "pharmacogenomic":
		overview.WriteString("Pharmacogenomic reports provide drug response predictions and dosing recommendations.")
	}
	
	overview.WriteString(fmt.Sprintf("\n\nThe report is customized for the %s audience with appropriate technical depth and language.", audience))
	
	return overview.String()
}

// buildObjectiveSection builds the objective section
func (rgp *ReportGenerationPrompt) buildObjectiveSection(reportType string, interpretationData map[string]interface{}) string {
	var objective strings.Builder
	
	objective.WriteString(fmt.Sprintf("Generate a comprehensive %s report incorporating:\n\n", reportType))
	
	if classification, exists := interpretationData["classification"]; exists {
		objective.WriteString(fmt.Sprintf("- **Primary Classification:** %s\n", classification))
	}
	
	if confidence, exists := interpretationData["confidence"]; exists {
		objective.WriteString(fmt.Sprintf("- **Confidence Level:** %s\n", confidence))
	}
	
	objective.WriteString("- **Supporting Evidence:** Comprehensive evidence summary and quality assessment\n")
	objective.WriteString("- **Clinical Interpretation:** Actionable insights and clinical recommendations\n")
	objective.WriteString("- **Quality Assurance:** Validation and compliance with professional standards\n")
	
	return objective.String()
}

// buildReportContextSection builds the report context section
func (rgp *ReportGenerationPrompt) buildReportContextSection(patientContext map[string]interface{}, complianceStandards []string, confidentialityLevel string) string {
	var context strings.Builder
	
	context.WriteString("**Report Configuration:**\n\n")
	
	if len(patientContext) > 0 {
		context.WriteString("- **Patient Context:**\n")
		for key, value := range patientContext {
			context.WriteString(fmt.Sprintf("  - %s: %v\n", strings.Title(key), value))
		}
	}
	
	if len(complianceStandards) > 0 {
		context.WriteString(fmt.Sprintf("- **Compliance Standards:** %s\n", strings.Join(complianceStandards, ", ")))
	}
	
	context.WriteString(fmt.Sprintf("- **Confidentiality Level:** %s\n", confidentialityLevel))
	
	return context.String()
}

// buildStepsSection builds the systematic steps section
func (rgp *ReportGenerationPrompt) buildStepsSection(reportType, reportFormat string, includeSections []string, audience string) string {
	var steps strings.Builder
	
	steps.WriteString("Follow these steps for report generation:\n\n")
	
	// Standard steps for all report types
	stepCounter := 1
	
	steps.WriteString(fmt.Sprintf("%d. **Data Integration:** Compile interpretation results, evidence summaries, and quality metrics\n\n", stepCounter))
	stepCounter++
	
	steps.WriteString(fmt.Sprintf("%d. **Content Structuring:** Organize content according to %s format requirements\n\n", stepCounter, reportFormat))
	stepCounter++
	
	// Custom sections based on includeSections
	if len(includeSections) > 0 {
		steps.WriteString(fmt.Sprintf("%d. **Section Development:** Create the following report sections:\n", stepCounter))
		for _, section := range includeSections {
			switch section {
			case "summary":
				steps.WriteString("   - **Executive Summary:** Key findings and clinical significance\n")
			case "interpretation":
				steps.WriteString("   - **Detailed Interpretation:** Classification rationale and evidence analysis\n")
			case "recommendations":
				steps.WriteString("   - **Clinical Recommendations:** Actionable guidance and follow-up suggestions\n")
			case "methods":
				steps.WriteString("   - **Methodology:** Testing procedures, analysis methods, and quality controls\n")
			case "results":
				steps.WriteString("   - **Results:** Detailed findings and measurement data\n")
			case "limitations":
				steps.WriteString("   - **Limitations:** Test limitations, uncertainties, and interpretive caveats\n")
			case "references":
				steps.WriteString("   - **References:** Supporting literature and guideline citations\n")
			case "appendices":
				steps.WriteString("   - **Appendices:** Supporting data, tables, and supplementary information\n")
			case "quality_metrics":
				steps.WriteString("   - **Quality Metrics:** Quality assurance data and validation results\n")
			}
		}
		stepCounter++
		steps.WriteString("\n")
	}
	
	// Audience-specific customization
	steps.WriteString(fmt.Sprintf("%d. **Audience Customization:** Adapt content and language for %s readability\n\n", stepCounter, audience))
	stepCounter++
	
	steps.WriteString(fmt.Sprintf("%d. **Compliance Review:** Ensure adherence to regulatory and professional standards\n\n", stepCounter))
	stepCounter++
	
	steps.WriteString(fmt.Sprintf("%d. **Quality Validation:** Perform final review and accuracy verification\n\n", stepCounter))
	stepCounter++
	
	steps.WriteString(fmt.Sprintf("%d. **Formatting and Finalization:** Apply template styling and prepare final output\n\n", stepCounter))
	
	return steps.String()
}

// buildGuidelinesSection builds the guidelines section
func (rgp *ReportGenerationPrompt) buildGuidelinesSection(complianceStandards []string, audience string) string {
	guidelines := []string{
		"**Accuracy and Completeness:** Ensure all information is accurate, complete, and properly verified",
		"**Professional Standards:** Follow established professional and regulatory guidelines",
		"**Clear Communication:** Use appropriate language and terminology for the target audience",
		"**Evidence-Based Content:** Base all conclusions on documented evidence and established criteria",
	}
	
	// Add compliance-specific guidelines
	for _, standard := range complianceStandards {
		switch standard {
		case "acmg", "amp":
			guidelines = append(guidelines, "**ACMG/AMP Compliance:** Follow ACMG/AMP variant interpretation guidelines")
		case "clia":
			guidelines = append(guidelines, "**CLIA Requirements:** Meet Clinical Laboratory Improvement Amendments standards")
		case "cap":
			guidelines = append(guidelines, "**CAP Standards:** Adhere to College of American Pathologists requirements")
		case "iso15189":
			guidelines = append(guidelines, "**ISO 15189:** Comply with medical laboratory quality standards")
		}
	}
	
	// Add audience-specific guidelines
	switch audience {
	case "patient":
		guidelines = append(guidelines, "**Patient-Friendly Language:** Use clear, non-technical language with appropriate explanations")
	case "clinician":
		guidelines = append(guidelines, "**Clinical Relevance:** Emphasize clinical actionability and patient management implications")
	case "laboratory":
		guidelines = append(guidelines, "**Technical Detail:** Include methodology, quality metrics, and analytical considerations")
	case "researcher":
		guidelines = append(guidelines, "**Scientific Rigor:** Provide detailed methodology, statistical analysis, and research context")
	}
	
	guidelines = append(guidelines,
		"**Confidentiality:** Maintain appropriate patient privacy and data security measures",
		"**Documentation:** Maintain comprehensive audit trails and version control")
	
	return strings.Join(guidelines, "\n")
}

// buildSystemPrompt builds the system prompt
func (rgp *ReportGenerationPrompt) buildSystemPrompt(reportType, audience string, complianceStandards []string, detailLevel string) string {
	var prompt strings.Builder
	
	prompt.WriteString("You are an expert clinical genetics report writer with extensive experience in ")
	prompt.WriteString("genetic testing documentation, regulatory compliance, and clinical communication. ")
	
	prompt.WriteString(fmt.Sprintf("Generate a %s %s report with %s level of detail. ", 
		reportType, audience, detailLevel))
	
	if len(complianceStandards) > 0 {
		prompt.WriteString(fmt.Sprintf("Ensure compliance with %s standards. ", 
			strings.Join(complianceStandards, ", ")))
	}
	
	switch audience {
	case "patient":
		prompt.WriteString("Use patient-friendly language while maintaining accuracy and completeness. ")
	case "clinician":
		prompt.WriteString("Focus on clinical actionability and patient management implications. ")
	case "laboratory":
		prompt.WriteString("Include technical methodology and quality assurance details. ")
	case "researcher":
		prompt.WriteString("Emphasize scientific methodology and research implications. ")
	}
	
	prompt.WriteString("Maintain professional standards and ensure accuracy throughout.")
	
	return prompt.String()
}

// buildUserPrompt builds the user prompt
func (rgp *ReportGenerationPrompt) buildUserPrompt(reportType string, interpretationData map[string]interface{}, audience string) string {
	var prompt strings.Builder
	
	prompt.WriteString(fmt.Sprintf("Please generate a comprehensive %s report ", reportType))
	
	if classification, exists := interpretationData["classification"]; exists {
		prompt.WriteString(fmt.Sprintf("for a variant classified as %s ", classification))
	}
	
	prompt.WriteString(fmt.Sprintf("targeted for %s audience. ", audience))
	prompt.WriteString("Use the provided interpretation data and follow the systematic workflow. ")
	prompt.WriteString("Ensure professional quality, accuracy, and compliance with specified standards.")
	
	return prompt.String()
}

// buildContextSection builds the context section
func (rgp *ReportGenerationPrompt) buildContextSection(reportType, audience string, complianceStandards []string) string {
	var context strings.Builder
	
	context.WriteString(fmt.Sprintf("**Report Type:** %s\n", reportType))
	context.WriteString(fmt.Sprintf("**Target Audience:** %s\n", audience))
	
	if len(complianceStandards) > 0 {
		context.WriteString(fmt.Sprintf("**Compliance Standards:** %s\n", strings.Join(complianceStandards, ", ")))
	}
	
	return context.String()
}

// buildInstructions builds the instructions list
func (rgp *ReportGenerationPrompt) buildInstructions(reportFormat, detailLevel string, qualityAssurance bool) []string {
	instructions := []string{
		"Use MCP tools to access interpretation data and evidence summaries",
		"Follow the systematic workflow for report generation step by step",
		"Ensure accuracy and completeness of all information included",
		"Apply appropriate formatting and structure for the specified format",
		"Include all required sections and elements for the report type",
	}
	
	switch reportFormat {
	case "structured":
		instructions = append(instructions, "Organize content using clear headings and structured format")
	case "narrative":
		instructions = append(instructions, "Present information in flowing narrative format with logical progression")
	case "tabular":
		instructions = append(instructions, "Use tables and structured data presentation where appropriate")
	case "hybrid":
		instructions = append(instructions, "Combine narrative and structured elements for optimal readability")
	}
	
	if detailLevel == "comprehensive" || detailLevel == "verbose" {
		instructions = append(instructions, "Include comprehensive details and supporting documentation")
	}
	
	if qualityAssurance {
		instructions = append(instructions,
			"Perform quality validation checks before finalizing",
			"Verify compliance with specified standards and requirements")
	}
	
	return instructions
}

// buildExamples builds usage examples
func (rgp *ReportGenerationPrompt) buildExamples(reportType, audience, detailLevel string) []string {
	examples := []string{
		"**Executive Summary:** Provide clear, concise summary of key findings and clinical significance",
		"**Evidence Integration:** Synthesize population, clinical, and functional evidence appropriately",
	}
	
	if audience == "patient" {
		examples = append(examples, "**Patient Education:** Explain genetic concepts in accessible language with appropriate context")
	} else if audience == "clinician" {
		examples = append(examples, "**Clinical Actionability:** Provide specific recommendations for patient management and follow-up")
	}
	
	if detailLevel == "comprehensive" || detailLevel == "verbose" {
		examples = append(examples, "**Detailed Analysis:** Include comprehensive methodology description and quality metrics")
	}
	
	return examples
}

// buildReferences builds the references list
func (rgp *ReportGenerationPrompt) buildReferences() []string {
	return []string{
		"Richards, S. et al. Standards and guidelines for the interpretation of sequence variants. Genet Med. 2015;17(5):405-24.",
		"Rehm, H.L. et al. ACMG clinical laboratory standards for next-generation sequencing. Genet Med. 2013;15(9):733-47.",
		"Matthijs, G. et al. Guidelines for diagnostic next-generation sequencing. Eur J Hum Genet. 2016;24(1):2-5.",
		"ACMG Board of Directors. Laboratory and clinical genomic data sharing is crucial to improving genetic health care. Genet Med. 2017;19(7):721-722.",
		"Bunnik, E.M. et al. A tiered-layered-staged model for informed consent in personal genome testing. Eur J Hum Genet. 2013;21(6):596-601.",
	}
}

// buildNotesSection builds the notes section
func (rgp *ReportGenerationPrompt) buildNotesSection(confidentialityLevel string, qualityAssurance bool) string {
	notes := []string{
		"Reports should be reviewed by qualified personnel before release",
		"All patient information must be handled according to privacy regulations",
		"Consider periodic review and updates as new evidence becomes available",
	}
	
	switch confidentialityLevel {
	case "confidential", "restricted":
		notes = append(notes, "Enhanced security measures apply due to confidentiality requirements")
	case "public":
		notes = append(notes, "Information may be suitable for broader sharing with appropriate consent")
	}
	
	if qualityAssurance {
		notes = append(notes, "Quality assurance protocols must be followed throughout the process")
	}
	
	notes = append(notes, "Maintain documentation of report generation process for audit purposes")
	
	return strings.Join(notes, "\n")
}