package prompts

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// EvidenceReviewPrompt provides structured evidence evaluation guidance
type EvidenceReviewPrompt struct {
	logger    *logrus.Logger
	renderer  *TemplateRenderer
	validator *ArgumentValidator
}

// NewEvidenceReviewPrompt creates a new evidence review prompt template
func NewEvidenceReviewPrompt(logger *logrus.Logger) *EvidenceReviewPrompt {
	return &EvidenceReviewPrompt{
		logger:    logger,
		renderer:  NewTemplateRenderer(logger),
		validator: NewArgumentValidator(logger),
	}
}

// GetPromptInfo returns metadata about this prompt template
func (erp *EvidenceReviewPrompt) GetPromptInfo() PromptInfo {
	return PromptInfo{
		Name:        "evidence_review",
		Description: "Structured guidance for comprehensive evaluation of genetic variant evidence across multiple data sources and evidence types",
		Version:     "1.0.0",
		Arguments: []ArgumentInfo{
			{
				Name:        "variant_id",
				Description: "Unique identifier for the variant to review evidence for",
				Type:        "string",
				Required:    true,
				Examples:    []string{"VAR_123456789", "rs123456", "COSV12345"},
				Constraints: []string{"min_length:3", "max_length:50"},
			},
			{
				Name:        "evidence_types",
				Description: "Types of evidence to focus the review on",
				Type:        "array",
				Required:    false,
				DefaultValue: []string{"population", "clinical", "functional", "computational", "literature"},
				Examples:    []string{"[\"population\", \"clinical\"]", "[\"functional\", \"literature\"]"},
				Constraints: []string{"enum:population,clinical,functional,computational,literature,segregation,structural"},
			},
			{
				Name:        "review_depth",
				Description: "Depth of evidence review to perform",
				Type:        "string",
				Required:    false,
				DefaultValue: "thorough",
				Examples:    []string{"summary", "standard", "thorough", "exhaustive"},
				Constraints: []string{"enum:summary,standard,thorough,exhaustive"},
			},
			{
				Name:        "quality_focus",
				Description: "Whether to emphasize evidence quality assessment",
				Type:        "boolean",
				Required:    false,
				DefaultValue: true,
				Examples:    []string{"true", "false"},
			},
			{
				Name:        "population_focus",
				Description: "Specific population groups to focus on for frequency analysis",
				Type:        "array",
				Required:    false,
				Examples:    []string{"[\"european\", \"african\"]", "[\"ashkenazi\", \"finnish\"]"},
				Constraints: []string{"enum:african,european,east_asian,south_asian,latino,ashkenazi,finnish,other"},
			},
			{
				Name:        "database_priority",
				Description: "Priority order for database consultation",
				Type:        "array",
				Required:    false,
				DefaultValue: []string{"clinvar", "gnomad", "hgmd", "cosmic"},
				Examples:    []string{"[\"gnomad\", \"clinvar\", \"exac\"]", "[\"hgmd\", \"lovd\", \"clinvar\"]"},
				Constraints: []string{"enum:clinvar,gnomad,hgmd,cosmic,exac,esp,lovd,clingen"},
			},
			{
				Name:        "functional_evidence_types",
				Description: "Types of functional evidence to emphasize",
				Type:        "array",
				Required:    false,
				Examples:    []string{"[\"in_vitro\", \"cell_based\"]", "[\"animal_model\", \"protein_studies\"]"},
				Constraints: []string{"enum:in_vitro,cell_based,animal_model,protein_studies,splicing,structural_modeling"},
			},
			{
				Name:        "literature_scope",
				Description: "Scope of literature review to perform",
				Type:        "string",
				Required:    false,
				DefaultValue: "comprehensive",
				Examples:    []string{"focused", "standard", "comprehensive", "systematic"},
				Constraints: []string{"enum:focused,standard,comprehensive,systematic"},
			},
			{
				Name:        "bias_assessment",
				Description: "Whether to include detailed bias assessment",
				Type:        "boolean",
				Required:    false,
				DefaultValue: false,
			},
			{
				Name:        "conflict_resolution",
				Description: "Approach to resolving conflicting evidence",
				Type:        "string",
				Required:    false,
				DefaultValue: "systematic",
				Examples:    []string{"simple", "systematic", "expert_consensus"},
				Constraints: []string{"enum:simple,systematic,expert_consensus,meta_analysis"},
			},
		},
		Examples: []PromptExample{
			{
				Name:        "Comprehensive clinical review",
				Description: "Thorough review focusing on clinical and population evidence",
				Arguments: map[string]interface{}{
					"variant_id":      "VAR_123456789",
					"evidence_types":  []string{"clinical", "population", "literature"},
					"review_depth":    "thorough",
					"quality_focus":   true,
					"database_priority": []string{"clinvar", "gnomad", "hgmd"},
				},
				ExpectedUse: "Comprehensive clinical evidence evaluation for variant classification",
			},
			{
				Name:        "Functional evidence focus",
				Description: "Detailed review emphasizing functional and computational evidence",
				Arguments: map[string]interface{}{
					"variant_id":                "rs123456",
					"evidence_types":            []string{"functional", "computational"},
					"review_depth":              "exhaustive",
					"functional_evidence_types": []string{"in_vitro", "protein_studies", "structural_modeling"},
					"quality_focus":             true,
				},
				ExpectedUse: "In-depth functional characterization for research purposes",
			},
		},
		Tags:       []string{"evidence", "review", "evaluation", "systematic", "quality", "bias"},
		Category:   "evidence_assessment",
		Difficulty: "intermediate",
		UsageNotes: []string{
			"Use MCP resources to gather evidence systematically",
			"Apply evidence quality frameworks consistently",
			"Document source reliability and potential biases",
			"Consider population-specific evidence relevance",
			"Integrate multiple evidence types appropriately",
		},
		Metadata: map[string]interface{}{
			"evidence_frameworks": []string{"ACMG/AMP", "GRADE", "CIViC", "AMP/ASCO/CAP"},
			"target_users":        []string{"clinical_geneticists", "researchers", "biocurators", "genetic_counselors"},
			"complexity":          "moderate",
			"time_estimate":       "30-60 minutes",
		},
	}
}

// RenderPrompt renders the prompt with given arguments
func (erp *EvidenceReviewPrompt) RenderPrompt(ctx context.Context, args map[string]interface{}) (*RenderedPrompt, error) {
	erp.logger.WithField("args", args).Debug("Rendering evidence review prompt")

	// Extract arguments with defaults
	variantID := erp.getStringArg(args, "variant_id", "")
	evidenceTypes := erp.getArrayArg(args, "evidence_types", []string{"population", "clinical", "functional", "computational", "literature"})
	reviewDepth := erp.getStringArg(args, "review_depth", "thorough")
	qualityFocus := erp.getBoolArg(args, "quality_focus", true)
	populationFocus := erp.getArrayArg(args, "population_focus", []string{})
	databasePriority := erp.getArrayArg(args, "database_priority", []string{"clinvar", "gnomad", "hgmd", "cosmic"})
	functionalTypes := erp.getArrayArg(args, "functional_evidence_types", []string{})
	literatureScope := erp.getStringArg(args, "literature_scope", "comprehensive")
	biasAssessment := erp.getBoolArg(args, "bias_assessment", false)
	conflictResolution := erp.getStringArg(args, "conflict_resolution", "systematic")

	// Build the prompt content
	content := erp.buildPromptContent(variantID, evidenceTypes, reviewDepth, qualityFocus, 
		populationFocus, databasePriority, functionalTypes, literatureScope, biasAssessment, conflictResolution)

	// Generate system and user prompts
	systemPrompt := erp.buildSystemPrompt(reviewDepth, qualityFocus, biasAssessment)
	userPrompt := erp.buildUserPrompt(variantID, evidenceTypes, reviewDepth)

	// Create rendered prompt
	rendered := &RenderedPrompt{
		Name:         "evidence_review",
		Content:      content,
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		Context:      erp.buildContextSection(variantID, evidenceTypes, databasePriority),
		Instructions: erp.buildInstructions(reviewDepth, qualityFocus, biasAssessment),
		Examples:     erp.buildExamples(reviewDepth, evidenceTypes),
		References:   erp.buildReferences(),
		Arguments:    args,
		GeneratedAt:  time.Now(),
		Metadata: map[string]interface{}{
			"variant_id":          variantID,
			"evidence_types":      evidenceTypes,
			"review_depth":        reviewDepth,
			"quality_focus":       qualityFocus,
			"database_priority":   databasePriority,
			"generated_by":        "evidence_review_prompt_v1.0.0",
		},
	}

	erp.logger.WithFields(logrus.Fields{
		"variant_id":      variantID,
		"evidence_types":  evidenceTypes,
		"review_depth":    reviewDepth,
		"content_length":  len(content),
	}).Info("Generated evidence review prompt")

	return rendered, nil
}

// ValidateArguments validates the provided arguments
func (erp *EvidenceReviewPrompt) ValidateArguments(args map[string]interface{}) error {
	return erp.validator.ValidateArguments(args, erp.GetPromptInfo().Arguments)
}

// GetArgumentSchema returns the JSON schema for prompt arguments
func (erp *EvidenceReviewPrompt) GetArgumentSchema() map[string]interface{} {
	return map[string]interface{}{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type":    "object",
		"properties": map[string]interface{}{
			"variant_id": map[string]interface{}{
				"type":        "string",
				"description": "Unique identifier for the variant to review evidence for",
				"minLength":   3,
				"maxLength":   50,
			},
			"evidence_types": map[string]interface{}{
				"type":        "array",
				"description": "Types of evidence to focus the review on",
				"items": map[string]interface{}{
					"type": "string",
					"enum": []string{"population", "clinical", "functional", "computational", "literature", "segregation", "structural"},
				},
				"default": []string{"population", "clinical", "functional", "computational", "literature"},
			},
			"review_depth": map[string]interface{}{
				"type":        "string",
				"description": "Depth of evidence review to perform",
				"enum":        []string{"summary", "standard", "thorough", "exhaustive"},
				"default":     "thorough",
			},
			"quality_focus": map[string]interface{}{
				"type":        "boolean",
				"description": "Whether to emphasize evidence quality assessment",
				"default":     true,
			},
			"population_focus": map[string]interface{}{
				"type":        "array",
				"description": "Specific population groups to focus on",
				"items": map[string]interface{}{
					"type": "string",
					"enum": []string{"african", "european", "east_asian", "south_asian", "latino", "ashkenazi", "finnish", "other"},
				},
			},
			"database_priority": map[string]interface{}{
				"type":        "array",
				"description": "Priority order for database consultation",
				"items": map[string]interface{}{
					"type": "string",
					"enum": []string{"clinvar", "gnomad", "hgmd", "cosmic", "exac", "esp", "lovd", "clingen"},
				},
				"default": []string{"clinvar", "gnomad", "hgmd", "cosmic"},
			},
		},
		"required": []string{"variant_id"},
	}
}

// SupportsPrompt checks if this template can handle the given prompt name
func (erp *EvidenceReviewPrompt) SupportsPrompt(name string) bool {
	supportedNames := []string{
		"evidence_review",
		"evidence-review",
		"evidence_evaluation",
		"evidence-evaluation",
		"variant_evidence_review",
		"variant-evidence-review",
	}
	
	for _, supported := range supportedNames {
		if name == supported {
			return true
		}
	}
	
	return false
}

// Helper methods for argument extraction
func (erp *EvidenceReviewPrompt) getStringArg(args map[string]interface{}, key, defaultValue string) string {
	if value, exists := args[key]; exists {
		if str, ok := value.(string); ok {
			return str
		}
	}
	return defaultValue
}

func (erp *EvidenceReviewPrompt) getArrayArg(args map[string]interface{}, key string, defaultValue []string) []string {
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

func (erp *EvidenceReviewPrompt) getBoolArg(args map[string]interface{}, key string, defaultValue bool) bool {
	if value, exists := args[key]; exists {
		if b, ok := value.(bool); ok {
			return b
		}
	}
	return defaultValue
}

// buildPromptContent builds the main prompt content
func (erp *EvidenceReviewPrompt) buildPromptContent(variantID string, evidenceTypes []string, reviewDepth string,
	qualityFocus bool, populationFocus, databasePriority, functionalTypes []string, 
	literatureScope string, biasAssessment bool, conflictResolution string) string {

	sections := map[string]string{
		"title":        "Systematic Genetic Variant Evidence Review",
		"overview":     erp.buildOverviewSection(reviewDepth, qualityFocus),
		"objective":    erp.buildObjectiveSection(variantID, evidenceTypes),
		"context":      erp.buildReviewContextSection(databasePriority, populationFocus),
		"instructions": strings.Join(erp.buildInstructions(reviewDepth, qualityFocus, biasAssessment), "\n"),
		"steps":        erp.buildStepsSection(evidenceTypes, reviewDepth, qualityFocus, functionalTypes),
		"guidelines":   erp.buildGuidelinesSection(qualityFocus, biasAssessment),
		"examples":     strings.Join(erp.buildExamples(reviewDepth, evidenceTypes), "\n\n"),
		"references":   strings.Join(erp.buildReferences(), "\n"),
		"notes":        erp.buildNotesSection(conflictResolution, biasAssessment),
	}

	return erp.renderer.RenderMarkdown(sections)
}

// buildOverviewSection builds the overview section
func (erp *EvidenceReviewPrompt) buildOverviewSection(reviewDepth string, qualityFocus bool) string {
	var overview strings.Builder
	
	overview.WriteString("This prompt provides structured guidance for comprehensive evaluation of genetic variant evidence across multiple data sources and evidence types. ")
	
	switch reviewDepth {
	case "summary":
		overview.WriteString("This summary review provides a high-level assessment of key evidence categories with emphasis on major findings.")
	case "standard":
		overview.WriteString("This standard review covers all essential evidence types with moderate depth, suitable for most variant assessment needs.")
	case "thorough":
		overview.WriteString("This thorough review provides detailed evaluation of evidence quality, source reliability, and comprehensive coverage of available data.")
	case "exhaustive":
		overview.WriteString("This exhaustive review includes systematic evaluation of all available evidence with detailed quality assessment, bias analysis, and conflict resolution.")
	}
	
	if qualityFocus {
		overview.WriteString("\n\nSpecial emphasis is placed on evidence quality assessment, including evaluation of study design, sample sizes, methodology reliability, and potential sources of bias.")
	}
	
	return overview.String()
}

// buildObjectiveSection builds the objective section
func (erp *EvidenceReviewPrompt) buildObjectiveSection(variantID string, evidenceTypes []string) string {
	var objective strings.Builder
	
	objective.WriteString(fmt.Sprintf("Conduct a systematic review of evidence for variant **%s** across the following evidence categories:\n\n", variantID))
	
	for _, evidenceType := range evidenceTypes {
		switch evidenceType {
		case "population":
			objective.WriteString("- **Population Evidence:** Allele frequencies, population genetics, and demographic distribution\n")
		case "clinical":
			objective.WriteString("- **Clinical Evidence:** Clinical reports, phenotype associations, and database submissions\n")
		case "functional":
			objective.WriteString("- **Functional Evidence:** Experimental studies, in vitro assays, and functional characterization\n")
		case "computational":
			objective.WriteString("- **Computational Evidence:** In silico predictions, conservation scores, and structural modeling\n")
		case "literature":
			objective.WriteString("- **Literature Evidence:** Published studies, case reports, and systematic reviews\n")
		case "segregation":
			objective.WriteString("- **Segregation Evidence:** Family studies, co-segregation analysis, and inheritance patterns\n")
		case "structural":
			objective.WriteString("- **Structural Evidence:** Protein structure analysis, domain impacts, and structural predictions\n")
		}
	}
	
	return objective.String()
}

// buildReviewContextSection builds the review context section
func (erp *EvidenceReviewPrompt) buildReviewContextSection(databasePriority, populationFocus []string) string {
	var context strings.Builder
	
	context.WriteString("**Review Configuration:**\n\n")
	
	if len(databasePriority) > 0 {
		context.WriteString("- **Database Priority:** ")
		context.WriteString(strings.Join(databasePriority, " → "))
		context.WriteString("\n")
	}
	
	if len(populationFocus) > 0 {
		context.WriteString("- **Population Focus:** ")
		context.WriteString(strings.Join(populationFocus, ", "))
		context.WriteString("\n")
	}
	
	context.WriteString("- **Available Resources:** Use MCP resources to access variant, evidence, and literature data systematically\n")
	
	return context.String()
}

// buildStepsSection builds the systematic steps section
func (erp *EvidenceReviewPrompt) buildStepsSection(evidenceTypes []string, reviewDepth string, qualityFocus bool, functionalTypes []string) string {
	var steps strings.Builder
	
	steps.WriteString("Follow these systematic steps for evidence review:\n\n")
	
	stepCounter := 1
	
	// Always start with data gathering
	steps.WriteString(fmt.Sprintf("%d. **Data Collection:** Gather evidence from MCP resources using variant ID\n", stepCounter))
	stepCounter++
	
	// Evidence-specific steps
	for _, evidenceType := range evidenceTypes {
		switch evidenceType {
		case "population":
			steps.WriteString(fmt.Sprintf("%d. **Population Analysis:**\n", stepCounter))
			steps.WriteString("   - Extract frequency data from gnomAD, ExAC, ESP, 1000 Genomes\n")
			steps.WriteString("   - Analyze population stratification and ethnic distribution\n")
			steps.WriteString("   - Assess frequency thresholds and disease prevalence compatibility\n")
			if qualityFocus {
				steps.WriteString("   - Evaluate data quality, coverage depth, and call confidence\n")
			}
			
		case "clinical":
			steps.WriteString(fmt.Sprintf("%d. **Clinical Evidence Review:**\n", stepCounter))
			steps.WriteString("   - Review ClinVar submissions and review status\n")
			steps.WriteString("   - Analyze HGMD entries and classification rationale\n")
			steps.WriteString("   - Assess clinical reports and phenotype associations\n")
			if qualityFocus {
				steps.WriteString("   - Evaluate submission quality, evidence strength, and consensus level\n")
			}
			
		case "functional":
			steps.WriteString(fmt.Sprintf("%d. **Functional Evidence Assessment:**\n", stepCounter))
			if len(functionalTypes) > 0 {
				steps.WriteString("   - Focus on specified functional evidence types:\n")
				for _, funcType := range functionalTypes {
					steps.WriteString(fmt.Sprintf("     • %s studies\n", strings.Replace(funcType, "_", " ", -1)))
				}
			} else {
				steps.WriteString("   - Review in vitro experimental studies\n")
				steps.WriteString("   - Analyze cell-based assays and model system data\n")
				steps.WriteString("   - Evaluate protein functional studies\n")
			}
			if qualityFocus {
				steps.WriteString("   - Assess experimental design, controls, and reproducibility\n")
			}
			
		case "computational":
			steps.WriteString(fmt.Sprintf("%d. **Computational Prediction Analysis:**\n", stepCounter))
			steps.WriteString("   - Compile pathogenicity prediction scores (SIFT, PolyPhen, CADD, REVEL)\n")
			steps.WriteString("   - Assess conservation scores across species\n")
			steps.WriteString("   - Evaluate structural impact predictions\n")
			if qualityFocus {
				steps.WriteString("   - Assess prediction agreement, confidence intervals, and tool limitations\n")
			}
			
		case "literature":
			steps.WriteString(fmt.Sprintf("%d. **Literature Review:**\n", stepCounter))
			steps.WriteString("   - Search PubMed and literature databases\n")
			steps.WriteString("   - Evaluate case reports and clinical studies\n")
			steps.WriteString("   - Assess review articles and meta-analyses\n")
			if qualityFocus {
				steps.WriteString("   - Evaluate publication quality, study design, and potential bias\n")
			}
		}
		
		stepCounter++
		steps.WriteString("\n")
	}
	
	// Quality assessment and synthesis
	if qualityFocus {
		steps.WriteString(fmt.Sprintf("%d. **Quality Assessment:** Evaluate evidence quality, reliability, and potential biases\n\n", stepCounter))
		stepCounter++
	}
	
	steps.WriteString(fmt.Sprintf("%d. **Evidence Integration:** Synthesize findings across evidence types and resolve conflicts\n\n", stepCounter))
	stepCounter++
	
	steps.WriteString(fmt.Sprintf("%d. **Documentation:** Prepare comprehensive evidence summary with quality metrics\n\n", stepCounter))
	
	return steps.String()
}

// buildGuidelinesSection builds the guidelines section
func (erp *EvidenceReviewPrompt) buildGuidelinesSection(qualityFocus, biasAssessment bool) string {
	guidelines := []string{
		"**Systematic Approach:** Follow the structured workflow consistently",
		"**Source Verification:** Validate data sources and cross-reference findings",
		"**Evidence Hierarchy:** Prioritize high-quality evidence appropriately",
		"**Comprehensive Coverage:** Include all relevant evidence types specified",
	}
	
	if qualityFocus {
		guidelines = append(guidelines,
			"**Quality Metrics:** Apply established quality assessment frameworks",
			"**Reliability Assessment:** Evaluate data source reliability and methodology")
	}
	
	if biasAssessment {
		guidelines = append(guidelines,
			"**Bias Recognition:** Identify and document potential sources of bias",
			"**Mitigation Strategies:** Apply appropriate bias mitigation approaches")
	}
	
	guidelines = append(guidelines,
		"**Conflict Resolution:** Address contradictory evidence systematically",
		"**Documentation Standards:** Maintain comprehensive evidence trails")
	
	return strings.Join(guidelines, "\n")
}

// buildSystemPrompt builds the system prompt
func (erp *EvidenceReviewPrompt) buildSystemPrompt(reviewDepth string, qualityFocus, biasAssessment bool) string {
	var prompt strings.Builder
	
	prompt.WriteString("You are an expert in genetic variant evidence evaluation with deep knowledge of multiple databases, ")
	prompt.WriteString("evidence quality assessment, and systematic review methodologies. ")
	
	switch reviewDepth {
	case "summary":
		prompt.WriteString("Provide a concise but comprehensive summary of key evidence. ")
	case "standard":
		prompt.WriteString("Provide thorough coverage of evidence with appropriate detail. ")
	case "thorough":
		prompt.WriteString("Provide detailed evaluation with emphasis on evidence quality and reliability. ")
	case "exhaustive":
		prompt.WriteString("Provide comprehensive evaluation with systematic quality assessment and detailed documentation. ")
	}
	
	if qualityFocus {
		prompt.WriteString("Pay special attention to evidence quality, methodology, and reliability assessment. ")
	}
	
	if biasAssessment {
		prompt.WriteString("Include detailed bias assessment and mitigation strategies. ")
	}
	
	prompt.WriteString("Use MCP resources systematically and document your evidence gathering process.")
	
	return prompt.String()
}

// buildUserPrompt builds the user prompt
func (erp *EvidenceReviewPrompt) buildUserPrompt(variantID string, evidenceTypes []string, reviewDepth string) string {
	var prompt strings.Builder
	
	prompt.WriteString(fmt.Sprintf("Please conduct a %s evidence review for variant %s ", reviewDepth, variantID))
	prompt.WriteString(fmt.Sprintf("focusing on %s evidence. ", strings.Join(evidenceTypes, ", ")))
	prompt.WriteString("Follow the systematic workflow and use available MCP resources to gather comprehensive evidence. ")
	prompt.WriteString("Provide detailed documentation of your findings and quality assessment.")
	
	return prompt.String()
}

// buildContextSection builds the context section
func (erp *EvidenceReviewPrompt) buildContextSection(variantID string, evidenceTypes, databasePriority []string) string {
	var context strings.Builder
	
	context.WriteString(fmt.Sprintf("**Variant:** %s\n", variantID))
	context.WriteString(fmt.Sprintf("**Evidence Types:** %s\n", strings.Join(evidenceTypes, ", ")))
	context.WriteString(fmt.Sprintf("**Database Priority:** %s\n", strings.Join(databasePriority, " → ")))
	
	return context.String()
}

// buildInstructions builds the instructions list
func (erp *EvidenceReviewPrompt) buildInstructions(reviewDepth string, qualityFocus, biasAssessment bool) []string {
	instructions := []string{
		"Use MCP resources systematically to gather evidence",
		"Follow the specified evidence type priorities and database order",
		"Document source reliability and data quality for each evidence type",
		"Cross-reference findings across multiple sources when possible",
		"Identify and address conflicting evidence appropriately",
		"Provide clear summary of findings for each evidence category",
	}
	
	if qualityFocus {
		instructions = append(instructions,
			"Apply established quality assessment criteria consistently",
			"Document methodology limitations and confidence levels")
	}
	
	if biasAssessment {
		instructions = append(instructions,
			"Identify potential sources of bias in evidence",
			"Apply appropriate bias mitigation strategies")
	}
	
	if reviewDepth == "exhaustive" {
		instructions = append(instructions,
			"Include comprehensive statistical analysis where applicable",
			"Provide detailed methodology assessment for all studies")
	}
	
	return instructions
}

// buildExamples builds usage examples
func (erp *EvidenceReviewPrompt) buildExamples(reviewDepth string, evidenceTypes []string) []string {
	examples := []string{
		"**Population Evidence:** Compare allele frequencies across gnomAD populations and assess disease prevalence compatibility",
		"**Clinical Evidence:** Evaluate ClinVar star ratings and analyze submission consensus patterns",
	}
	
	if contains(evidenceTypes, "functional") {
		examples = append(examples, "**Functional Evidence:** Assess experimental design quality and result reproducibility across studies")
	}
	
	if contains(evidenceTypes, "computational") {
		examples = append(examples, "**Computational Evidence:** Integrate multiple prediction scores and assess consensus reliability")
	}
	
	if reviewDepth == "thorough" || reviewDepth == "exhaustive" {
		examples = append(examples, "**Quality Assessment:** Apply GRADE criteria or similar frameworks to evaluate evidence quality")
	}
	
	return examples
}

// buildReferences builds the references list
func (erp *EvidenceReviewPrompt) buildReferences() []string {
	return []string{
		"Guyatt, G.H. et al. GRADE: an emerging consensus on rating quality of evidence and strength of recommendations. BMJ. 2008;336(7650):924-6.",
		"Whiffin, N. et al. Using high-resolution variant frequencies to empower clinical genome interpretation. Genet Med. 2017;19(10):1151-1158.",
		"MacArthur, D.G. et al. A systematic survey of loss-of-function variants in human protein-coding genes. Science. 2012;335(6070):823-8.",
		"Landrum, M.J. et al. ClinVar: improving access to variant interpretations and supporting evidence. Nucleic Acids Res. 2018;46(D1):D1062-D1067.",
		"Ioannidis, N.M. et al. REVEL: An Ensemble Method for Predicting the Pathogenicity of Rare Missense Variants. Am J Hum Genet. 2016;99(4):877-885.",
	}
}

// buildNotesSection builds the notes section
func (erp *EvidenceReviewPrompt) buildNotesSection(conflictResolution string, biasAssessment bool) string {
	notes := []string{
		"Evidence quality and reliability may vary significantly across sources",
		"Population-specific considerations are important for frequency analysis",
		"Functional studies require careful methodology assessment",
		"Computational predictions have inherent limitations and should be interpreted cautiously",
	}
	
	if biasAssessment {
		notes = append(notes,
			"Publication bias may affect literature evidence availability",
			"Selection bias in clinical databases should be considered")
	}
	
	notes = append(notes, fmt.Sprintf("Conflict resolution approach: %s", conflictResolution))
	
	return strings.Join(notes, "\n")
}

