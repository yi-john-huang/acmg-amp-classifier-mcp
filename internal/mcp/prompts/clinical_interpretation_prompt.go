package prompts

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// ClinicalInterpretationPrompt provides systematic workflow guidance for clinical interpretation
type ClinicalInterpretationPrompt struct {
	logger    *logrus.Logger
	renderer  *TemplateRenderer
	validator *ArgumentValidator
}

// NewClinicalInterpretationPrompt creates a new clinical interpretation prompt template
func NewClinicalInterpretationPrompt(logger *logrus.Logger) *ClinicalInterpretationPrompt {
	return &ClinicalInterpretationPrompt{
		logger:    logger,
		renderer:  NewTemplateRenderer(logger),
		validator: NewArgumentValidator(logger),
	}
}

// GetPromptInfo returns metadata about this prompt template
func (cip *ClinicalInterpretationPrompt) GetPromptInfo() PromptInfo {
	return PromptInfo{
		Name:        "clinical_interpretation",
		Description: "Systematic workflow guidance for clinical genetic variant interpretation following ACMG/AMP guidelines",
		Version:     "1.0.0",
		Arguments: []ArgumentInfo{
			{
				Name:        "variant_notation",
				Description: "HGVS notation of the variant to interpret (e.g., NM_000001.3:c.123A>G)",
				Type:        "string",
				Required:    true,
				Examples:    []string{"NM_000001.3:c.123A>G", "NP_000001.2:p.Arg41Gln", "NC_000017.11:g.43094692C>T"},
				Constraints: []string{"min_length:5", "pattern:^(NM_|NP_|NC_)"},
			},
			{
				Name:        "gene_symbol",
				Description: "Gene symbol associated with the variant",
				Type:        "string",
				Required:    false,
				Examples:    []string{"BRCA1", "TP53", "CFTR"},
				Constraints: []string{"max_length:20"},
			},
			{
				Name:        "patient_phenotype",
				Description: "Clinical phenotype or indication for testing",
				Type:        "string",
				Required:    false,
				Examples:    []string{"Hereditary breast cancer", "Cystic fibrosis", "Hypertrophic cardiomyopathy"},
				Constraints: []string{"max_length:500"},
			},
			{
				Name:        "family_history",
				Description: "Relevant family history information",
				Type:        "string",
				Required:    false,
				Examples:    []string{"Mother affected with breast cancer at age 45", "No known family history", "Multiple affected relatives"},
				Constraints: []string{"max_length:1000"},
			},
			{
				Name:        "testing_indication",
				Description: "Clinical indication for genetic testing",
				Type:        "string",
				Required:    false,
				Examples:    []string{"diagnostic", "predictive", "carrier_screening", "pharmacogenomic"},
				Constraints: []string{"enum:diagnostic,predictive,carrier_screening,pharmacogenomic,prenatal,population_screening"},
			},
			{
				Name:        "interpretation_level",
				Description: "Level of interpretation detail required",
				Type:        "string",
				Required:    false,
				DefaultValue: "comprehensive",
				Examples:    []string{"basic", "standard", "comprehensive", "expert"},
				Constraints: []string{"enum:basic,standard,comprehensive,expert"},
			},
			{
				Name:        "evidence_focus",
				Description: "Specific evidence types to emphasize in interpretation",
				Type:        "array",
				Required:    false,
				Examples:    []string{"[\"population\", \"clinical\"]", "[\"functional\", \"computational\"]"},
				Constraints: []string{"enum:population,clinical,functional,computational,literature,segregation"},
			},
			{
				Name:        "clinical_context",
				Description: "Additional clinical context information",
				Type:        "object",
				Required:    false,
				Examples:    []string{"{\"age\": 45, \"sex\": \"female\", \"ethnicity\": \"european\"}", "{\"pregnancy\": true, \"gestational_age\": 20}"},
			},
		},
		Examples: []PromptExample{
			{
				Name:        "Diagnostic interpretation",
				Description: "Comprehensive interpretation for diagnostic testing",
				Arguments: map[string]interface{}{
					"variant_notation":     "NM_000001.3:c.123A>G",
					"gene_symbol":          "GENE1",
					"patient_phenotype":    "Suspected genetic condition",
					"testing_indication":   "diagnostic",
					"interpretation_level": "comprehensive",
				},
				ExpectedUse: "Systematic evaluation of variant significance for diagnostic purposes",
			},
			{
				Name:        "Predictive testing",
				Description: "Interpretation for predictive genetic testing",
				Arguments: map[string]interface{}{
					"variant_notation":     "NM_000001.3:c.456G>A",
					"gene_symbol":          "BRCA1",
					"family_history":       "Mother with breast cancer at age 40",
					"testing_indication":   "predictive",
					"interpretation_level": "standard",
				},
				ExpectedUse: "Assessment of variant significance for predictive testing in asymptomatic individual",
			},
		},
		Tags:       []string{"clinical", "interpretation", "acmg", "amp", "systematic", "workflow"},
		Category:   "clinical_genetics",
		Difficulty: "intermediate",
		UsageNotes: []string{
			"Follow ACMG/AMP 2015 guidelines systematically",
			"Consider all available evidence types comprehensively",
			"Document reasoning and evidence quality clearly",
			"Include appropriate clinical context and limitations",
			"Provide actionable recommendations when possible",
		},
		Metadata: map[string]interface{}{
			"guidelines":     "ACMG/AMP 2015",
			"target_users":   []string{"clinical_geneticists", "genetic_counselors", "laboratory_directors"},
			"complexity":     "moderate_to_high",
			"time_estimate":  "15-30 minutes",
		},
	}
}

// RenderPrompt renders the prompt with given arguments
func (cip *ClinicalInterpretationPrompt) RenderPrompt(ctx context.Context, args map[string]interface{}) (*RenderedPrompt, error) {
	cip.logger.WithField("args", args).Debug("Rendering clinical interpretation prompt")

	// Extract arguments with defaults
	variantNotation := cip.getStringArg(args, "variant_notation", "")
	geneSymbol := cip.getStringArg(args, "gene_symbol", "")
	patientPhenotype := cip.getStringArg(args, "patient_phenotype", "")
	familyHistory := cip.getStringArg(args, "family_history", "")
	testingIndication := cip.getStringArg(args, "testing_indication", "diagnostic")
	interpretationLevel := cip.getStringArg(args, "interpretation_level", "comprehensive")
	evidenceFocus := cip.getArrayArg(args, "evidence_focus", []string{"population", "clinical", "functional", "computational"})
	clinicalContext := cip.getObjectArg(args, "clinical_context", map[string]interface{}{})

	// Build the prompt content
	content := cip.buildPromptContent(variantNotation, geneSymbol, patientPhenotype, familyHistory, 
		testingIndication, interpretationLevel, evidenceFocus, clinicalContext)

	// Generate system and user prompts
	systemPrompt := cip.buildSystemPrompt(interpretationLevel, testingIndication)
	userPrompt := cip.buildUserPrompt(variantNotation, geneSymbol, patientPhenotype, familyHistory, clinicalContext)

	// Create rendered prompt
	rendered := &RenderedPrompt{
		Name:         "clinical_interpretation",
		Content:      content,
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		Context:      cip.buildContextSection(testingIndication, evidenceFocus),
		Instructions: cip.buildInstructions(interpretationLevel),
		Examples:     cip.buildExamples(interpretationLevel),
		References:   cip.buildReferences(),
		Arguments:    args,
		GeneratedAt:  time.Now(),
		Metadata: map[string]interface{}{
			"variant_notation":     variantNotation,
			"gene_symbol":          geneSymbol,
			"interpretation_level": interpretationLevel,
			"evidence_focus":       evidenceFocus,
			"generated_by":         "clinical_interpretation_prompt_v1.0.0",
		},
	}

	cip.logger.WithFields(logrus.Fields{
		"variant":        variantNotation,
		"gene":           geneSymbol,
		"level":          interpretationLevel,
		"content_length": len(content),
	}).Info("Generated clinical interpretation prompt")

	return rendered, nil
}

// ValidateArguments validates the provided arguments
func (cip *ClinicalInterpretationPrompt) ValidateArguments(args map[string]interface{}) error {
	return cip.validator.ValidateArguments(args, cip.GetPromptInfo().Arguments)
}

// GetArgumentSchema returns the JSON schema for prompt arguments
func (cip *ClinicalInterpretationPrompt) GetArgumentSchema() map[string]interface{} {
	return map[string]interface{}{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type":    "object",
		"properties": map[string]interface{}{
			"variant_notation": map[string]interface{}{
				"type":        "string",
				"description": "HGVS notation of the variant to interpret",
				"minLength":   5,
				"pattern":     "^(NM_|NP_|NC_)",
				"examples":    []string{"NM_000001.3:c.123A>G", "NP_000001.2:p.Arg41Gln"},
			},
			"gene_symbol": map[string]interface{}{
				"type":        "string",
				"description": "Gene symbol associated with the variant",
				"maxLength":   20,
				"examples":    []string{"BRCA1", "TP53", "CFTR"},
			},
			"patient_phenotype": map[string]interface{}{
				"type":        "string",
				"description": "Clinical phenotype or indication for testing",
				"maxLength":   500,
			},
			"family_history": map[string]interface{}{
				"type":        "string",
				"description": "Relevant family history information",
				"maxLength":   1000,
			},
			"testing_indication": map[string]interface{}{
				"type":        "string",
				"description": "Clinical indication for genetic testing",
				"enum":        []string{"diagnostic", "predictive", "carrier_screening", "pharmacogenomic", "prenatal", "population_screening"},
				"default":     "diagnostic",
			},
			"interpretation_level": map[string]interface{}{
				"type":        "string",
				"description": "Level of interpretation detail required",
				"enum":        []string{"basic", "standard", "comprehensive", "expert"},
				"default":     "comprehensive",
			},
			"evidence_focus": map[string]interface{}{
				"type":        "array",
				"description": "Specific evidence types to emphasize in interpretation",
				"items": map[string]interface{}{
					"type": "string",
					"enum": []string{"population", "clinical", "functional", "computational", "literature", "segregation"},
				},
				"uniqueItems": true,
			},
			"clinical_context": map[string]interface{}{
				"type":        "object",
				"description": "Additional clinical context information",
				"properties": map[string]interface{}{
					"age":        map[string]interface{}{"type": "number"},
					"sex":        map[string]interface{}{"type": "string", "enum": []string{"male", "female", "other"}},
					"ethnicity":  map[string]interface{}{"type": "string"},
					"pregnancy":  map[string]interface{}{"type": "boolean"},
				},
			},
		},
		"required": []string{"variant_notation"},
	}
}

// SupportsPrompt checks if this template can handle the given prompt name
func (cip *ClinicalInterpretationPrompt) SupportsPrompt(name string) bool {
	supportedNames := []string{
		"clinical_interpretation",
		"clinical-interpretation",
		"variant_interpretation",
		"variant-interpretation",
		"acmg_interpretation",
		"acmg-interpretation",
	}
	
	for _, supported := range supportedNames {
		if name == supported {
			return true
		}
	}
	
	return false
}

// Helper methods for argument extraction
func (cip *ClinicalInterpretationPrompt) getStringArg(args map[string]interface{}, key, defaultValue string) string {
	if value, exists := args[key]; exists {
		if str, ok := value.(string); ok {
			return str
		}
	}
	return defaultValue
}

func (cip *ClinicalInterpretationPrompt) getArrayArg(args map[string]interface{}, key string, defaultValue []string) []string {
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

func (cip *ClinicalInterpretationPrompt) getObjectArg(args map[string]interface{}, key string, defaultValue map[string]interface{}) map[string]interface{} {
	if value, exists := args[key]; exists {
		if obj, ok := value.(map[string]interface{}); ok {
			return obj
		}
	}
	return defaultValue
}

// buildPromptContent builds the main prompt content
func (cip *ClinicalInterpretationPrompt) buildPromptContent(variantNotation, geneSymbol, patientPhenotype, familyHistory, 
	testingIndication, interpretationLevel string, evidenceFocus []string, clinicalContext map[string]interface{}) string {

	sections := map[string]string{
		"title": "Clinical Genetic Variant Interpretation",
		"overview": cip.buildOverviewSection(interpretationLevel, testingIndication),
		"objective": cip.buildObjectiveSection(variantNotation, geneSymbol, testingIndication),
		"context": cip.buildClinicalContextSection(patientPhenotype, familyHistory, clinicalContext),
		"instructions": strings.Join(cip.buildInstructions(interpretationLevel), "\n"),
		"steps": cip.buildStepsSection(interpretationLevel, evidenceFocus),
		"guidelines": cip.buildGuidelinesSection(),
		"examples": strings.Join(cip.buildExamples(interpretationLevel), "\n\n"),
		"references": strings.Join(cip.buildReferences(), "\n"),
		"notes": cip.buildNotesSection(interpretationLevel),
	}

	return cip.renderer.RenderMarkdown(sections)
}

// buildOverviewSection builds the overview section
func (cip *ClinicalInterpretationPrompt) buildOverviewSection(interpretationLevel, testingIndication string) string {
	var overview strings.Builder
	
	overview.WriteString("This prompt guides systematic clinical interpretation of genetic variants following the ACMG/AMP 2015 guidelines. ")
	
	switch interpretationLevel {
	case "basic":
		overview.WriteString("This basic interpretation focuses on fundamental classification criteria and provides a streamlined approach suitable for straightforward variants.")
	case "standard":
		overview.WriteString("This standard interpretation covers all essential ACMG/AMP criteria with moderate detail, suitable for most clinical variants.")
	case "comprehensive":
		overview.WriteString("This comprehensive interpretation provides detailed evaluation of all evidence types and criteria, suitable for complex variants requiring thorough analysis.")
	case "expert":
		overview.WriteString("This expert-level interpretation includes advanced considerations, edge cases, and detailed documentation suitable for complex cases and research contexts.")
	}
	
	overview.WriteString(fmt.Sprintf("\n\nThe interpretation is tailored for **%s** testing and follows evidence-based assessment protocols.", testingIndication))
	
	return overview.String()
}

// buildObjectiveSection builds the objective section
func (cip *ClinicalInterpretationPrompt) buildObjectiveSection(variantNotation, geneSymbol, testingIndication string) string {
	var objective strings.Builder
	
	objective.WriteString(fmt.Sprintf("Systematically evaluate the clinical significance of variant **%s**", variantNotation))
	
	if geneSymbol != "" {
		objective.WriteString(fmt.Sprintf(" in gene **%s**", geneSymbol))
	}
	
	objective.WriteString(" according to ACMG/AMP guidelines to determine its pathogenicity and clinical actionability.")
	
	switch testingIndication {
	case "diagnostic":
		objective.WriteString(" The interpretation will support diagnostic decision-making and patient management.")
	case "predictive":
		objective.WriteString(" The interpretation will assess risk prediction and inform preventive care strategies.")
	case "carrier_screening":
		objective.WriteString(" The interpretation will determine carrier status and reproductive risk assessment.")
	case "pharmacogenomic":
		objective.WriteString(" The interpretation will evaluate drug response implications and dosing recommendations.")
	}
	
	return objective.String()
}

// buildClinicalContextSection builds the clinical context section
func (cip *ClinicalInterpretationPrompt) buildClinicalContextSection(patientPhenotype, familyHistory string, clinicalContext map[string]interface{}) string {
	var context strings.Builder
	
	context.WriteString("**Clinical Information:**\n\n")
	
	if patientPhenotype != "" {
		context.WriteString(fmt.Sprintf("- **Phenotype:** %s\n", patientPhenotype))
	}
	
	if familyHistory != "" {
		context.WriteString(fmt.Sprintf("- **Family History:** %s\n", familyHistory))
	}
	
	if len(clinicalContext) > 0 {
		context.WriteString("- **Additional Context:**\n")
		for key, value := range clinicalContext {
			context.WriteString(fmt.Sprintf("  - %s: %v\n", strings.Title(key), value))
		}
	}
	
	if patientPhenotype == "" && familyHistory == "" && len(clinicalContext) == 0 {
		context.WriteString("- No specific clinical context provided. Consider gathering additional phenotypic and family history information to enhance interpretation accuracy.\n")
	}
	
	return context.String()
}

// buildStepsSection builds the systematic steps section
func (cip *ClinicalInterpretationPrompt) buildStepsSection(interpretationLevel string, evidenceFocus []string) string {
	var steps strings.Builder
	
	steps.WriteString("Follow these systematic steps for variant interpretation:\n\n")
	
	// Core steps for all levels
	coreSteps := []string{
		"**Variant Verification:** Confirm variant nomenclature and transcript information",
		"**Gene-Disease Association:** Verify gene-disease relationship and inheritance pattern",
		"**Population Frequency:** Evaluate allele frequencies in relevant populations",
		"**Computational Predictions:** Assess in silico pathogenicity predictions",
		"**Clinical Evidence:** Review clinical reports and databases (ClinVar, HGMD)",
		"**Functional Evidence:** Evaluate experimental functional studies",
		"**ACMG Criteria Application:** Systematically apply applicable ACMG/AMP criteria",
		"**Classification Assignment:** Determine final pathogenicity classification",
		"**Clinical Interpretation:** Assess clinical significance and actionability",
		"**Documentation:** Document evidence and reasoning comprehensively",
	}
	
	// Filter steps based on evidence focus
	focusedSteps := make([]string, 0)
	for _, step := range coreSteps {
		include := true
		
		if len(evidenceFocus) > 0 {
			include = false
			for _, focus := range evidenceFocus {
				if cip.stepMatchesFocus(step, focus) {
					include = true
					break
				}
			}
			// Always include verification, criteria application, and documentation
			if strings.Contains(step, "Verification") || strings.Contains(step, "ACMG Criteria") || strings.Contains(step, "Documentation") {
				include = true
			}
		}
		
		if include {
			focusedSteps = append(focusedSteps, step)
		}
	}
	
	// Add level-specific details
	for i, step := range focusedSteps {
		steps.WriteString(fmt.Sprintf("%d. %s", i+1, step))
		
		if interpretationLevel == "comprehensive" || interpretationLevel == "expert" {
			steps.WriteString(cip.getStepDetails(step, interpretationLevel))
		}
		
		steps.WriteString("\n\n")
	}
	
	return steps.String()
}

// stepMatchesFocus checks if a step matches evidence focus
func (cip *ClinicalInterpretationPrompt) stepMatchesFocus(step, focus string) bool {
	stepFocusMap := map[string][]string{
		"population":     {"Population Frequency"},
		"clinical":       {"Clinical Evidence", "Gene-Disease Association"},
		"functional":     {"Functional Evidence"},
		"computational":  {"Computational Predictions"},
		"literature":     {"Clinical Evidence", "Functional Evidence"},
		"segregation":    {"Clinical Evidence"},
	}
	
	focusSteps, exists := stepFocusMap[focus]
	if !exists {
		return true
	}
	
	for _, focusStep := range focusSteps {
		if strings.Contains(step, focusStep) {
			return true
		}
	}
	
	return false
}

// getStepDetails provides detailed guidance for each step
func (cip *ClinicalInterpretationPrompt) getStepDetails(step, level string) string {
	if level != "comprehensive" && level != "expert" {
		return ""
	}
	
	detailsMap := map[string]string{
		"Variant Verification": "\n   - Validate HGVS nomenclature against reference sequences\n   - Confirm variant coordinates and transcript selection\n   - Check for alternative annotations or aliases",
		"Population Frequency": "\n   - Review gnomAD, ExAC, ESP, and 1000 Genomes data\n   - Consider population stratification and founder effects\n   - Assess allele frequency thresholds for disease prevalence",
		"Clinical Evidence": "\n   - Search ClinVar, HGMD, and literature databases\n   - Evaluate clinical report quality and evidence level\n   - Assess segregation data and family studies",
		"ACMG Criteria Application": "\n   - Systematically evaluate all 28 ACMG/AMP criteria\n   - Document evidence strength and confidence levels\n   - Consider rule modifications and gene-specific adjustments",
	}
	
	for key, details := range detailsMap {
		if strings.Contains(step, key) {
			return details
		}
	}
	
	return ""
}

// buildGuidelinesSection builds the guidelines section
func (cip *ClinicalInterpretationPrompt) buildGuidelinesSection() string {
	guidelines := []string{
		"**ACMG/AMP Framework:** Apply the 2015 ACMG/AMP guidelines systematically",
		"**Evidence Hierarchy:** Prioritize high-quality evidence over quantity",
		"**Clinical Context:** Consider patient phenotype and family history",
		"**Population Relevance:** Use appropriate population reference data",
		"**Functional Validation:** Evaluate experimental evidence quality",
		"**Literature Review:** Assess publication quality and study design",
		"**Uncertainty Management:** Acknowledge limitations and uncertainties",
		"**Regular Review:** Consider periodic reclassification as new evidence emerges",
	}
	
	return strings.Join(guidelines, "\n")
}

// buildSystemPrompt builds the system prompt for AI agents
func (cip *ClinicalInterpretationPrompt) buildSystemPrompt(interpretationLevel, testingIndication string) string {
	var prompt strings.Builder
	
	prompt.WriteString("You are a clinical genetics expert specializing in genetic variant interpretation. ")
	prompt.WriteString("Follow the ACMG/AMP 2015 guidelines systematically and provide evidence-based assessments. ")
	
	switch interpretationLevel {
	case "basic":
		prompt.WriteString("Provide a streamlined interpretation focusing on key classification criteria. ")
	case "standard":
		prompt.WriteString("Provide a thorough interpretation covering all relevant evidence types. ")
	case "comprehensive":
		prompt.WriteString("Provide a detailed interpretation with comprehensive evidence evaluation. ")
	case "expert":
		prompt.WriteString("Provide an expert-level interpretation with advanced considerations and detailed documentation. ")
	}
	
	prompt.WriteString(fmt.Sprintf("This interpretation is for %s purposes. ", testingIndication))
	prompt.WriteString("Be systematic, evidence-based, and clinically relevant in your assessment.")
	
	return prompt.String()
}

// buildUserPrompt builds the user prompt
func (cip *ClinicalInterpretationPrompt) buildUserPrompt(variantNotation, geneSymbol, patientPhenotype, familyHistory string, clinicalContext map[string]interface{}) string {
	var prompt strings.Builder
	
	prompt.WriteString(fmt.Sprintf("Please interpret the clinical significance of variant %s", variantNotation))
	
	if geneSymbol != "" {
		prompt.WriteString(fmt.Sprintf(" in gene %s", geneSymbol))
	}
	
	prompt.WriteString(" following the systematic workflow provided. ")
	
	if patientPhenotype != "" || familyHistory != "" || len(clinicalContext) > 0 {
		prompt.WriteString("Consider the provided clinical context in your interpretation. ")
	}
	
	prompt.WriteString("Use available MCP resources to gather evidence and apply ACMG/AMP criteria systematically.")
	
	return prompt.String()
}

// buildContextSection builds the context section
func (cip *ClinicalInterpretationPrompt) buildContextSection(testingIndication string, evidenceFocus []string) string {
	var context strings.Builder
	
	context.WriteString(fmt.Sprintf("**Testing Context:** %s\n", strings.Title(testingIndication)))
	
	if len(evidenceFocus) > 0 {
		context.WriteString(fmt.Sprintf("**Evidence Focus:** %s\n", strings.Join(evidenceFocus, ", ")))
	}
	
	context.WriteString("**Available Resources:** Use MCP tools and resources for evidence gathering and classification")
	
	return context.String()
}

// buildInstructions builds the instructions list
func (cip *ClinicalInterpretationPrompt) buildInstructions(interpretationLevel string) []string {
	instructions := []string{
		"Follow the systematic workflow step by step",
		"Use MCP resources to gather comprehensive evidence",
		"Apply ACMG/AMP criteria objectively and systematically",
		"Document your reasoning and evidence quality",
		"Consider clinical context and patient-specific factors",
		"Provide clear classification and confidence assessment",
		"Include appropriate limitations and disclaimers",
	}
	
	if interpretationLevel == "comprehensive" || interpretationLevel == "expert" {
		instructions = append(instructions,
			"Evaluate evidence conflicts and resolution strategies",
			"Consider gene-specific guideline modifications",
			"Assess population-specific considerations",
			"Provide recommendations for additional studies if needed")
	}
	
	return instructions
}

// buildExamples builds usage examples
func (cip *ClinicalInterpretationPrompt) buildExamples(interpretationLevel string) []string {
	examples := []string{
		"**Population Frequency Assessment:** Use gnomAD data to evaluate BA1 and BS1 criteria",
		"**Clinical Evidence Review:** Assess ClinVar submissions and literature reports for PS4/BP6",
		"**Functional Evidence:** Evaluate in vitro studies for PS3/BS3 criteria application",
	}
	
	if interpretationLevel == "comprehensive" || interpretationLevel == "expert" {
		examples = append(examples,
			"**Segregation Analysis:** Calculate LOD scores and assess PP1/BS4 criteria",
			"**Computational Prediction Integration:** Combine multiple prediction tools for PP3/BP4",
			"**De Novo Assessment:** Evaluate PS2/PM6 criteria with parental testing confirmation")
	}
	
	return examples
}

// buildReferences builds the references list
func (cip *ClinicalInterpretationPrompt) buildReferences() []string {
	return []string{
		"Richards, S. et al. Standards and guidelines for the interpretation of sequence variants: a joint consensus recommendation of the American College of Medical Genetics and Genomics and the Association for Molecular Pathology. Genet Med. 2015;17(5):405-24.",
		"Plon, S.E. et al. Sequence variant classification and reporting: recommendations for improving the interpretation of cancer susceptibility genetic test results. Hum Mutat. 2008;29(11):1282-91.",
		"MacArthur, D.G. et al. Guidelines for investigating causality of sequence variants in human disease. Nature. 2014;508(7497):469-76.",
		"Harrison, S.M. et al. Clinical laboratories collaborate to resolve differences in variant interpretations submitted to ClinVar. Genet Med. 2017;19(10):1096-1104.",
	}
}

// buildNotesSection builds the notes section
func (cip *ClinicalInterpretationPrompt) buildNotesSection(interpretationLevel string) string {
	notes := []string{
		"This interpretation is based on currently available evidence and guidelines",
		"Classifications may change as new evidence becomes available",
		"Consider laboratory-specific policies and gene-specific modifications",
		"Ensure appropriate clinical correlation and genetic counseling",
	}
	
	if interpretationLevel == "expert" {
		notes = append(notes,
			"Consider research and publication opportunities for novel findings",
			"Evaluate contribution to variant databases and knowledge sharing",
			"Assess need for functional validation studies")
	}
	
	return strings.Join(notes, "\n")
}