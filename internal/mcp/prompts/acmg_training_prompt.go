package prompts

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// ACMGTrainingPrompt provides educational guideline learning assistance
type ACMGTrainingPrompt struct {
	logger    *logrus.Logger
	renderer  *TemplateRenderer
	validator *ArgumentValidator
}

// NewACMGTrainingPrompt creates a new ACMG training prompt template
func NewACMGTrainingPrompt(logger *logrus.Logger) *ACMGTrainingPrompt {
	return &ACMGTrainingPrompt{
		logger:    logger,
		renderer:  NewTemplateRenderer(logger),
		validator: NewArgumentValidator(logger),
	}
}

// GetPromptInfo returns metadata about this prompt template
func (atp *ACMGTrainingPrompt) GetPromptInfo() PromptInfo {
	return PromptInfo{
		Name:        "acmg_training",
		Description: "Interactive educational guidance for learning ACMG/AMP variant interpretation guidelines with case studies and practice exercises",
		Version:     "1.0.0",
		Arguments: []ArgumentInfo{
			{
				Name:        "training_level",
				Description: "Educational level and target audience for training",
				Type:        "string",
				Required:    true,
				Examples:    []string{"beginner", "intermediate", "advanced", "expert", "refresher"},
				Constraints: []string{"enum:beginner,intermediate,advanced,expert,refresher"},
			},
			{
				Name:        "training_focus",
				Description: "Specific aspects of ACMG/AMP guidelines to emphasize",
				Type:        "array",
				Required:    false,
				DefaultValue: []string{"criteria", "application", "interpretation"},
				Examples:    []string{"[\"criteria\", \"application\"]", "[\"edge_cases\", \"updates\"]"},
				Constraints: []string{"enum:criteria,application,interpretation,edge_cases,updates,quality,conflicts,population_specific"},
			},
			{
				Name:        "learning_style",
				Description: "Preferred learning approach and methodology",
				Type:        "string",
				Required:    false,
				DefaultValue: "interactive",
				Examples:    []string{"interactive", "case_based", "systematic", "problem_solving"},
				Constraints: []string{"enum:interactive,case_based,systematic,problem_solving,self_paced,guided"},
			},
			{
				Name:        "professional_role",
				Description: "Professional role of the learner",
				Type:        "string",
				Required:    false,
				Examples:    []string{"clinical_geneticist", "genetic_counselor", "laboratory_director", "resident", "student"},
				Constraints: []string{"enum:clinical_geneticist,genetic_counselor,laboratory_director,bioinformatician,resident,student,technologist,researcher"},
			},
			{
				Name:        "specific_criteria",
				Description: "Specific ACMG/AMP criteria to focus training on",
				Type:        "array",
				Required:    false,
				Examples:    []string{"[\"PVS1\", \"PS1\", \"PM2\"]", "[\"BA1\", \"BS1\", \"BP4\"]"},
				Constraints: []string{"enum:PVS1,PS1,PS2,PS3,PS4,PM1,PM2,PM3,PM4,PM5,PM6,PP1,PP2,PP3,PP4,PP5,BA1,BS1,BS2,BS3,BS4,BP1,BP2,BP3,BP4,BP5,BP6,BP7"},
			},
			{
				Name:        "case_complexity",
				Description: "Complexity level of training cases to include",
				Type:        "string",
				Required:    false,
				DefaultValue: "mixed",
				Examples:    []string{"simple", "moderate", "complex", "mixed"},
				Constraints: []string{"enum:simple,moderate,complex,mixed,challenging"},
			},
			{
				Name:        "include_exercises",
				Description: "Whether to include practice exercises and assessments",
				Type:        "boolean",
				Required:    false,
				DefaultValue: true,
			},
			{
				Name:        "assessment_style",
				Description: "Style of assessment and feedback",
				Type:        "string",
				Required:    false,
				DefaultValue: "formative",
				Examples:    []string{"formative", "summative", "self_assessment", "peer_review"},
				Constraints: []string{"enum:formative,summative,self_assessment,peer_review,competency_based"},
			},
			{
				Name:        "time_commitment",
				Description: "Expected time commitment for training session",
				Type:        "string",
				Required:    false,
				DefaultValue: "standard",
				Examples:    []string{"brief", "standard", "extended", "comprehensive"},
				Constraints: []string{"enum:brief,standard,extended,comprehensive"},
			},
			{
				Name:        "prerequisite_knowledge",
				Description: "Assumed prerequisite knowledge level",
				Type:        "string",
				Required:    false,
				DefaultValue: "basic_genetics",
				Examples:    []string{"none", "basic_genetics", "clinical_genetics", "molecular_genetics"},
				Constraints: []string{"enum:none,basic_genetics,clinical_genetics,molecular_genetics,bioinformatics"},
			},
			{
				Name:        "learning_objectives",
				Description: "Specific learning objectives to achieve",
				Type:        "array",
				Required:    false,
				Examples:    []string{"[\"understand_criteria\", \"apply_rules\"]", "[\"resolve_conflicts\", \"quality_assessment\"]"},
				Constraints: []string{"enum:understand_criteria,apply_rules,resolve_conflicts,quality_assessment,case_analysis,guideline_updates"},
			},
		},
		Examples: []PromptExample{
			{
				Name:        "Beginner training module",
				Description: "Comprehensive introduction to ACMG/AMP guidelines",
				Arguments: map[string]interface{}{
					"training_level":       "beginner",
					"training_focus":       []string{"criteria", "application"},
					"learning_style":       "systematic",
					"professional_role":    "resident",
					"case_complexity":      "simple",
					"include_exercises":    true,
					"time_commitment":      "extended",
				},
				ExpectedUse: "Foundational training for genetics residents and fellows",
			},
			{
				Name:        "Advanced case-based workshop",
				Description: "Complex case analysis and edge case handling",
				Arguments: map[string]interface{}{
					"training_level":      "advanced",
					"training_focus":      []string{"edge_cases", "conflicts", "quality"},
					"learning_style":      "case_based",
					"professional_role":   "clinical_geneticist",
					"case_complexity":     "complex",
					"assessment_style":    "peer_review",
				},
				ExpectedUse: "Advanced training for experienced clinical geneticists",
			},
		},
		Tags:       []string{"education", "training", "acmg", "amp", "guidelines", "learning", "assessment"},
		Category:   "medical_education",
		Difficulty: "variable",
		UsageNotes: []string{
			"Adapt content complexity to learner's experience level",
			"Use interactive elements to enhance engagement",
			"Provide regular feedback and assessment opportunities",
			"Include real-world case examples and scenarios",
			"Reference current guidelines and recent updates",
		},
		Metadata: map[string]interface{}{
			"educational_frameworks": []string{"Bloom's Taxonomy", "Miller's Pyramid", "Competency-based Learning"},
			"target_users":          []string{"medical_students", "residents", "fellows", "practicing_clinicians", "laboratory_staff"},
			"complexity":            "adaptive",
			"time_estimate":         "30 minutes - 4 hours",
		},
	}
}

// RenderPrompt renders the prompt with given arguments
func (atp *ACMGTrainingPrompt) RenderPrompt(ctx context.Context, args map[string]interface{}) (*RenderedPrompt, error) {
	atp.logger.WithField("args", args).Debug("Rendering ACMG training prompt")

	// Extract arguments with defaults
	trainingLevel := atp.getStringArg(args, "training_level", "")
	trainingFocus := atp.getArrayArg(args, "training_focus", []string{"criteria", "application", "interpretation"})
	learningStyle := atp.getStringArg(args, "learning_style", "interactive")
	professionalRole := atp.getStringArg(args, "professional_role", "")
	specificCriteria := atp.getArrayArg(args, "specific_criteria", []string{})
	caseComplexity := atp.getStringArg(args, "case_complexity", "mixed")
	includeExercises := atp.getBoolArg(args, "include_exercises", true)
	assessmentStyle := atp.getStringArg(args, "assessment_style", "formative")
	timeCommitment := atp.getStringArg(args, "time_commitment", "standard")
	prerequisiteKnowledge := atp.getStringArg(args, "prerequisite_knowledge", "basic_genetics")
	learningObjectives := atp.getArrayArg(args, "learning_objectives", []string{})

	// Build the prompt content
	content := atp.buildPromptContent(trainingLevel, trainingFocus, learningStyle, professionalRole,
		specificCriteria, caseComplexity, includeExercises, assessmentStyle, timeCommitment,
		prerequisiteKnowledge, learningObjectives)

	// Generate system and user prompts
	systemPrompt := atp.buildSystemPrompt(trainingLevel, professionalRole, learningStyle)
	userPrompt := atp.buildUserPrompt(trainingLevel, trainingFocus, learningStyle)

	// Create rendered prompt
	rendered := &RenderedPrompt{
		Name:         "acmg_training",
		Content:      content,
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		Context:      atp.buildContextSection(trainingLevel, professionalRole, timeCommitment),
		Instructions: atp.buildInstructions(learningStyle, includeExercises, assessmentStyle),
		Examples:     atp.buildExamples(trainingLevel, caseComplexity, specificCriteria),
		References:   atp.buildReferences(),
		Arguments:    args,
		GeneratedAt:  time.Now(),
		Metadata: map[string]interface{}{
			"training_level":      trainingLevel,
			"professional_role":   professionalRole,
			"learning_style":      learningStyle,
			"training_focus":      trainingFocus,
			"case_complexity":     caseComplexity,
			"generated_by":        "acmg_training_prompt_v1.0.0",
		},
	}

	atp.logger.WithFields(logrus.Fields{
		"training_level":   trainingLevel,
		"professional_role": professionalRole,
		"learning_style":   learningStyle,
		"content_length":   len(content),
	}).Info("Generated ACMG training prompt")

	return rendered, nil
}

// ValidateArguments validates the provided arguments
func (atp *ACMGTrainingPrompt) ValidateArguments(args map[string]interface{}) error {
	return atp.validator.ValidateArguments(args, atp.GetPromptInfo().Arguments)
}

// GetArgumentSchema returns the JSON schema for prompt arguments
func (atp *ACMGTrainingPrompt) GetArgumentSchema() map[string]interface{} {
	return map[string]interface{}{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type":    "object",
		"properties": map[string]interface{}{
			"training_level": map[string]interface{}{
				"type":        "string",
				"description": "Educational level and target audience",
				"enum":        []string{"beginner", "intermediate", "advanced", "expert", "refresher"},
			},
			"training_focus": map[string]interface{}{
				"type":        "array",
				"description": "Specific aspects of guidelines to emphasize",
				"items": map[string]interface{}{
					"type": "string",
					"enum": []string{"criteria", "application", "interpretation", "edge_cases", "updates", "quality", "conflicts", "population_specific"},
				},
				"default": []string{"criteria", "application", "interpretation"},
			},
			"learning_style": map[string]interface{}{
				"type":        "string",
				"description": "Preferred learning approach",
				"enum":        []string{"interactive", "case_based", "systematic", "problem_solving", "self_paced", "guided"},
				"default":     "interactive",
			},
			"professional_role": map[string]interface{}{
				"type":        "string",
				"description": "Professional role of the learner",
				"enum":        []string{"clinical_geneticist", "genetic_counselor", "laboratory_director", "bioinformatician", "resident", "student", "technologist", "researcher"},
			},
			"case_complexity": map[string]interface{}{
				"type":        "string",
				"description": "Complexity level of training cases",
				"enum":        []string{"simple", "moderate", "complex", "mixed", "challenging"},
				"default":     "mixed",
			},
		},
		"required": []string{"training_level"},
	}
}

// SupportsPrompt checks if this template can handle the given prompt name
func (atp *ACMGTrainingPrompt) SupportsPrompt(name string) bool {
	supportedNames := []string{
		"acmg_training",
		"acmg-training",
		"acmg_education",
		"acmg-education",
		"variant_interpretation_training",
		"variant-interpretation-training",
		"genetics_training",
		"genetics-training",
	}
	
	for _, supported := range supportedNames {
		if name == supported {
			return true
		}
	}
	
	return false
}

// Helper methods for argument extraction
func (atp *ACMGTrainingPrompt) getStringArg(args map[string]interface{}, key, defaultValue string) string {
	if value, exists := args[key]; exists {
		if str, ok := value.(string); ok {
			return str
		}
	}
	return defaultValue
}

func (atp *ACMGTrainingPrompt) getArrayArg(args map[string]interface{}, key string, defaultValue []string) []string {
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

func (atp *ACMGTrainingPrompt) getBoolArg(args map[string]interface{}, key string, defaultValue bool) bool {
	if value, exists := args[key]; exists {
		if b, ok := value.(bool); ok {
			return b
		}
	}
	return defaultValue
}

// buildPromptContent builds the main prompt content
func (atp *ACMGTrainingPrompt) buildPromptContent(trainingLevel string, trainingFocus []string, learningStyle, professionalRole string,
	specificCriteria []string, caseComplexity string, includeExercises bool, assessmentStyle, timeCommitment,
	prerequisiteKnowledge string, learningObjectives []string) string {

	sections := map[string]string{
		"title":        "ACMG/AMP Variant Interpretation Training",
		"overview":     atp.buildOverviewSection(trainingLevel, professionalRole, timeCommitment),
		"objective":    atp.buildObjectiveSection(trainingFocus, learningObjectives),
		"context":      atp.buildTrainingContextSection(prerequisiteKnowledge, professionalRole),
		"instructions": strings.Join(atp.buildInstructions(learningStyle, includeExercises, assessmentStyle), "\n"),
		"steps":        atp.buildStepsSection(trainingLevel, trainingFocus, learningStyle, specificCriteria),
		"guidelines":   atp.buildGuidelinesSection(trainingLevel, caseComplexity),
		"examples":     strings.Join(atp.buildExamples(trainingLevel, caseComplexity, specificCriteria), "\n\n"),
		"references":   strings.Join(atp.buildReferences(), "\n"),
		"notes":        atp.buildNotesSection(assessmentStyle, includeExercises),
	}

	return atp.renderer.RenderMarkdown(sections)
}

// buildOverviewSection builds the overview section
func (atp *ACMGTrainingPrompt) buildOverviewSection(trainingLevel, professionalRole, timeCommitment string) string {
	var overview strings.Builder
	
	overview.WriteString(fmt.Sprintf("This interactive training module provides %s-level education on ACMG/AMP variant interpretation guidelines. ", trainingLevel))
	
	if professionalRole != "" {
		overview.WriteString(fmt.Sprintf("The content is tailored for %s with appropriate depth and clinical context. ", 
			strings.Replace(professionalRole, "_", " ", -1)))
	}
	
	switch trainingLevel {
	case "beginner":
		overview.WriteString("This foundational module introduces core concepts, basic criteria, and fundamental interpretation principles.")
	case "intermediate":
		overview.WriteString("This module builds on basic knowledge with practical application, case studies, and nuanced interpretation skills.")
	case "advanced":
		overview.WriteString("This advanced module covers complex scenarios, edge cases, and sophisticated interpretation challenges.")
	case "expert":
		overview.WriteString("This expert-level module focuses on cutting-edge developments, research applications, and guideline evolution.")
	case "refresher":
		overview.WriteString("This refresher module updates knowledge with recent guideline changes and reinforces key concepts.")
	}
	
	switch timeCommitment {
	case "brief":
		overview.WriteString("\n\nDesigned for a focused 30-45 minute learning session.")
	case "standard":
		overview.WriteString("\n\nDesigned for a comprehensive 1-2 hour learning session.")
	case "extended":
		overview.WriteString("\n\nDesigned for an intensive 2-4 hour learning session.")
	case "comprehensive":
		overview.WriteString("\n\nDesigned for a thorough multi-session learning experience.")
	}
	
	return overview.String()
}

// buildObjectiveSection builds the objective section
func (atp *ACMGTrainingPrompt) buildObjectiveSection(trainingFocus, learningObjectives []string) string {
	var objective strings.Builder
	
	objective.WriteString("Upon completion of this training, learners will be able to:\n\n")
	
	// Standard objectives based on training focus
	for _, focus := range trainingFocus {
		switch focus {
		case "criteria":
			objective.WriteString("- **Understand ACMG/AMP Criteria:** Comprehend all 28 criteria with their evidence requirements and applications\n")
		case "application":
			objective.WriteString("- **Apply Classification Rules:** Systematically apply criteria to real variants and determine classifications\n")
		case "interpretation":
			objective.WriteString("- **Interpret Clinical Significance:** Assess clinical implications and provide actionable recommendations\n")
		case "edge_cases":
			objective.WriteString("- **Handle Edge Cases:** Navigate complex scenarios and ambiguous evidence situations\n")
		case "updates":
			objective.WriteString("- **Incorporate Updates:** Apply recent guideline modifications and best practices\n")
		case "quality":
			objective.WriteString("- **Assess Quality:** Evaluate evidence quality and apply appropriate confidence levels\n")
		case "conflicts":
			objective.WriteString("- **Resolve Conflicts:** Address contradictory evidence and achieve consensus classifications\n")
		case "population_specific":
			objective.WriteString("- **Consider Population Factors:** Apply population-specific considerations in interpretation\n")
		}
	}
	
	// Custom learning objectives
	if len(learningObjectives) > 0 {
		objective.WriteString("\n**Specific Learning Objectives:**\n")
		for _, obj := range learningObjectives {
			objective.WriteString(fmt.Sprintf("- %s\n", strings.Replace(obj, "_", " ", -1)))
		}
	}
	
	return objective.String()
}

// buildTrainingContextSection builds the training context section
func (atp *ACMGTrainingPrompt) buildTrainingContextSection(prerequisiteKnowledge, professionalRole string) string {
	var context strings.Builder
	
	context.WriteString("**Training Configuration:**\n\n")
	
	switch prerequisiteKnowledge {
	case "none":
		context.WriteString("- **Prerequisites:** No prior genetics knowledge assumed - basic concepts will be introduced\n")
	case "basic_genetics":
		context.WriteString("- **Prerequisites:** Basic understanding of genetics, inheritance patterns, and molecular biology\n")
	case "clinical_genetics":
		context.WriteString("- **Prerequisites:** Clinical genetics background with patient care experience\n")
	case "molecular_genetics":
		context.WriteString("- **Prerequisites:** Advanced molecular genetics and genomics knowledge\n")
	case "bioinformatics":
		context.WriteString("- **Prerequisites:** Bioinformatics and computational genomics experience\n")
	}
	
	if professionalRole != "" {
		context.WriteString(fmt.Sprintf("- **Target Role:** %s\n", strings.Replace(professionalRole, "_", " ", -1)))
	}
	
	context.WriteString("- **Resources Available:** MCP resources for accessing ACMG rules, examples, and reference materials\n")
	
	return context.String()
}

// buildStepsSection builds the learning steps section
func (atp *ACMGTrainingPrompt) buildStepsSection(trainingLevel string, trainingFocus []string, learningStyle string, specificCriteria []string) string {
	var steps strings.Builder
	
	steps.WriteString("Follow this structured learning pathway:\n\n")
	
	stepCounter := 1
	
	// Introduction and foundation
	steps.WriteString(fmt.Sprintf("%d. **Foundation and Context:**\n", stepCounter))
	steps.WriteString("   - Review ACMG/AMP guideline background and rationale\n")
	steps.WriteString("   - Understand classification framework and terminology\n")
	if trainingLevel == "beginner" {
		steps.WriteString("   - Learn basic genetics concepts and variant types\n")
	}
	stepCounter++
	steps.WriteString("\n")
	
	// Criteria learning
	if contains(trainingFocus, "criteria") || len(specificCriteria) > 0 {
		steps.WriteString(fmt.Sprintf("%d. **ACMG/AMP Criteria Mastery:**\n", stepCounter))
		if len(specificCriteria) > 0 {
			steps.WriteString(fmt.Sprintf("   - Focus on specified criteria: %s\n", strings.Join(specificCriteria, ", ")))
		} else {
			steps.WriteString("   - Study pathogenic evidence criteria (PVS1, PS1-4, PM1-6, PP1-5)\n")
			steps.WriteString("   - Learn benign evidence criteria (BA1, BS1-4, BP1-7)\n")
		}
		steps.WriteString("   - Understand evidence strength and application rules\n")
		stepCounter++
		steps.WriteString("\n")
	}
	
	// Application and practice
	if contains(trainingFocus, "application") {
		steps.WriteString(fmt.Sprintf("%d. **Practical Application:**\n", stepCounter))
		switch learningStyle {
		case "case_based":
			steps.WriteString("   - Work through curated case studies with guided analysis\n")
			steps.WriteString("   - Practice systematic criteria application\n")
		case "interactive":
			steps.WriteString("   - Engage in interactive exercises and decision trees\n")
			steps.WriteString("   - Participate in real-time classification challenges\n")
		case "systematic":
			steps.WriteString("   - Follow structured workflow for variant classification\n")
			steps.WriteString("   - Apply systematic decision-making frameworks\n")
		case "problem_solving":
			steps.WriteString("   - Tackle complex interpretation problems\n")
			steps.WriteString("   - Develop independent reasoning skills\n")
		}
		stepCounter++
		steps.WriteString("\n")
	}
	
	// Advanced topics based on focus
	if contains(trainingFocus, "edge_cases") || contains(trainingFocus, "conflicts") {
		steps.WriteString(fmt.Sprintf("%d. **Advanced Scenarios:**\n", stepCounter))
		steps.WriteString("   - Handle conflicting evidence and ambiguous cases\n")
		steps.WriteString("   - Navigate edge cases and guideline limitations\n")
		steps.WriteString("   - Apply expert judgment and consensus approaches\n")
		stepCounter++
		steps.WriteString("\n")
	}
	
	// Assessment and validation
	steps.WriteString(fmt.Sprintf("%d. **Assessment and Validation:**\n", stepCounter))
	steps.WriteString("   - Complete practice exercises and self-assessments\n")
	steps.WriteString("   - Validate understanding through case-based testing\n")
	steps.WriteString("   - Receive feedback and identify areas for improvement\n")
	stepCounter++
	steps.WriteString("\n")
	
	// Continuing education
	steps.WriteString(fmt.Sprintf("%d. **Ongoing Development:**\n", stepCounter))
	steps.WriteString("   - Establish plan for continuing education and skill maintenance\n")
	steps.WriteString("   - Identify resources for staying current with guideline updates\n")
	steps.WriteString("   - Connect with professional communities and expert networks\n")
	
	return steps.String()
}

// buildGuidelinesSection builds the training guidelines section
func (atp *ACMGTrainingPrompt) buildGuidelinesSection(trainingLevel, caseComplexity string) string {
	guidelines := []string{
		"**Active Learning:** Engage actively with exercises and case studies rather than passive reading",
		"**Systematic Approach:** Follow the structured methodology consistently",
		"**Evidence-Based Practice:** Base all interpretations on documented evidence and established criteria",
		"**Critical Thinking:** Question assumptions and evaluate evidence quality",
	}
	
	switch trainingLevel {
	case "beginner":
		guidelines = append(guidelines,
			"**Foundation First:** Master basic concepts before advancing to complex scenarios",
			"**Ask Questions:** Seek clarification on unclear concepts or terminology")
	case "advanced", "expert":
		guidelines = append(guidelines,
			"**Challenge Assumptions:** Question edge cases and explore guideline limitations",
			"**Contribute Knowledge:** Share experiences and contribute to community learning")
	}
	
	switch caseComplexity {
	case "complex", "challenging":
		guidelines = append(guidelines,
			"**Embrace Complexity:** View challenging cases as learning opportunities",
			"**Collaborative Approach:** Discuss difficult cases with colleagues and experts")
	case "simple":
		guidelines = append(guidelines,
			"**Build Confidence:** Use simple cases to build foundational skills",
			"**Gradual Progression:** Move to more complex scenarios as competence develops")
	}
	
	guidelines = append(guidelines,
		"**Continuous Improvement:** Regularly assess and update your interpretation skills",
		"**Professional Standards:** Maintain highest standards of accuracy and professional integrity")
	
	return strings.Join(guidelines, "\n")
}

// buildSystemPrompt builds the system prompt
func (atp *ACMGTrainingPrompt) buildSystemPrompt(trainingLevel, professionalRole, learningStyle string) string {
	var prompt strings.Builder
	
	prompt.WriteString("You are an expert genetics educator with extensive experience in ACMG/AMP guidelines, ")
	prompt.WriteString("medical education, and clinical genetics training. ")
	
	prompt.WriteString(fmt.Sprintf("Provide %s-level education ", trainingLevel))
	
	if professionalRole != "" {
		prompt.WriteString(fmt.Sprintf("tailored for %s ", strings.Replace(professionalRole, "_", " ", -1)))
	}
	
	prompt.WriteString(fmt.Sprintf("using %s learning approach. ", strings.Replace(learningStyle, "_", " ", -1)))
	
	switch learningStyle {
	case "interactive":
		prompt.WriteString("Encourage active participation and provide immediate feedback. ")
	case "case_based":
		prompt.WriteString("Use real clinical cases and guide through systematic analysis. ")
	case "systematic":
		prompt.WriteString("Present information in logical sequence with clear structure. ")
	case "problem_solving":
		prompt.WriteString("Challenge learners with problems and guide them to solutions. ")
	}
	
	prompt.WriteString("Maintain accuracy, provide constructive feedback, and adapt to the learner's progress.")
	
	return prompt.String()
}

// buildUserPrompt builds the user prompt
func (atp *ACMGTrainingPrompt) buildUserPrompt(trainingLevel string, trainingFocus []string, learningStyle string) string {
	var prompt strings.Builder
	
	prompt.WriteString(fmt.Sprintf("Please provide %s-level ACMG/AMP training ", trainingLevel))
	prompt.WriteString(fmt.Sprintf("focusing on %s ", strings.Join(trainingFocus, ", ")))
	prompt.WriteString(fmt.Sprintf("using %s learning approach. ", strings.Replace(learningStyle, "_", " ", -1)))
	prompt.WriteString("Use MCP resources to access ACMG rules and provide relevant examples. ")
	prompt.WriteString("Make the learning experience engaging, comprehensive, and practically applicable.")
	
	return prompt.String()
}

// buildContextSection builds the context section
func (atp *ACMGTrainingPrompt) buildContextSection(trainingLevel, professionalRole, timeCommitment string) string {
	var context strings.Builder
	
	context.WriteString(fmt.Sprintf("**Training Level:** %s\n", trainingLevel))
	
	if professionalRole != "" {
		context.WriteString(fmt.Sprintf("**Professional Role:** %s\n", strings.Replace(professionalRole, "_", " ", -1)))
	}
	
	context.WriteString(fmt.Sprintf("**Time Commitment:** %s\n", timeCommitment))
	
	return context.String()
}

// buildInstructions builds the instructions list
func (atp *ACMGTrainingPrompt) buildInstructions(learningStyle string, includeExercises bool, assessmentStyle string) []string {
	instructions := []string{
		"Use MCP resources to access ACMG/AMP rules and examples",
		"Follow the structured learning pathway systematically",
		"Engage actively with the content and exercises",
		"Apply learning to practical scenarios and case studies",
	}
	
	switch learningStyle {
	case "interactive":
		instructions = append(instructions,
			"Participate actively in interactive elements",
			"Seek clarification and ask questions throughout")
	case "case_based":
		instructions = append(instructions,
			"Work through each case systematically",
			"Document your reasoning and decision process")
	case "problem_solving":
		instructions = append(instructions,
			"Approach problems systematically",
			"Try to solve problems independently before reviewing solutions")
	}
	
	if includeExercises {
		instructions = append(instructions, "Complete all practice exercises and assessments")
	}
	
	switch assessmentStyle {
	case "formative":
		instructions = append(instructions, "Use feedback to guide your learning progress")
	case "self_assessment":
		instructions = append(instructions, "Regularly assess your own understanding and progress")
	case "peer_review":
		instructions = append(instructions, "Engage with peers for collaborative learning")
	}
	
	return instructions
}

// buildExamples builds learning examples
func (atp *ACMGTrainingPrompt) buildExamples(trainingLevel, caseComplexity string, specificCriteria []string) []string {
	examples := []string{
		"**Criteria Application:** Practice applying PVS1 to loss-of-function variants with detailed reasoning",
		"**Evidence Evaluation:** Assess population frequency data for BS1/BA1 criteria application",
	}
	
	if len(specificCriteria) > 0 {
		examples = append(examples, fmt.Sprintf("**Focused Practice:** Work through examples specifically targeting %s criteria", strings.Join(specificCriteria, ", ")))
	}
	
	switch caseComplexity {
	case "simple":
		examples = append(examples, "**Basic Cases:** Clear-cut pathogenic and benign variants with straightforward evidence")
	case "complex":
		examples = append(examples, "**Complex Cases:** Multi-evidence variants requiring careful weighing of contradictory data")
	case "challenging":
		examples = append(examples, "**Edge Cases:** Unusual variants that test guideline boundaries and require expert judgment")
	}
	
	if trainingLevel == "advanced" || trainingLevel == "expert" {
		examples = append(examples, "**Publication Cases:** Real cases from literature with detailed analysis and discussion")
	}
	
	return examples
}

// buildReferences builds the references list
func (atp *ACMGTrainingPrompt) buildReferences() []string {
	return []string{
		"Richards, S. et al. Standards and guidelines for the interpretation of sequence variants: a joint consensus recommendation of the American College of Medical Genetics and Genomics and the Association for Molecular Pathology. Genet Med. 2015 May;17(5):405-24.",
		"Tavtigian, S.V. et al. Modeling the ACMG/AMP variant classification guidelines as a Bayesian classification framework. Genet Med. 2018;20(9):1054-1060.",
		"Biesecker, L.G. & Harrison, S.M. The ACMG/AMP reputable source criteria for the interpretation of sequence variants. Genet Med. 2018;20(12):1687-1688.",
		"ClinGen Sequence Variant Interpretation Working Group. https://clinicalgenome.org/working-groups/sequence-variant-interpretation/",
		"ACMG Laboratory Quality Assurance Committee. Standards and guidelines for clinical genetics laboratories. Genet Med. 2019;21(2):256-271.",
		"Harrison, S.M. et al. Clinical laboratories collaborate to resolve differences in variant interpretations submitted to ClinVar. Genet Med. 2017;19(10):1096-1104.",
	}
}

// buildNotesSection builds the notes section
func (atp *ACMGTrainingPrompt) buildNotesSection(assessmentStyle string, includeExercises bool) string {
	notes := []string{
		"Learning is most effective when combined with practical application",
		"Regular practice and review help maintain proficiency",
		"Guidelines evolve - stay current with updates and modifications",
		"Collaboration with colleagues enhances learning and quality",
	}
	
	if assessmentStyle == "competency_based" {
		notes = append(notes, "Competency demonstration is required before advancing to next level")
	}
	
	if includeExercises {
		notes = append(notes, "Practice exercises are essential for skill development")
	}
	
	notes = append(notes,
		"Consider joining professional organizations and continuing education programs",
		"Maintain a learning log to track progress and identify improvement areas")
	
	return strings.Join(notes, "\n")
}

// Helper function to check if array contains string
func contains(arr []string, str string) bool {
	for _, item := range arr {
		if item == str {
			return true
		}
	}
	return false
}