package prompts

import (
	"context"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPromptManager_RegisterTemplate(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	
	manager := NewPromptManager(logger)
	template := NewClinicalInterpretationPrompt(logger)
	
	manager.RegisterTemplate("clinical_interpretation", template)
	
	// Verify template is registered
	templates := manager.GetTemplateInfo()
	require.Len(t, templates, 1)
	assert.Equal(t, "clinical_interpretation", templates[0].Name)
}

func TestPromptManager_GetPrompt(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	
	manager := NewPromptManager(logger)
	
	// Register all templates
	manager.RegisterTemplate("clinical_interpretation", NewClinicalInterpretationPrompt(logger))
	manager.RegisterTemplate("evidence_review", NewEvidenceReviewPrompt(logger))
	manager.RegisterTemplate("report_generation", NewReportGenerationPrompt(logger))
	manager.RegisterTemplate("acmg_training", NewACMGTrainingPrompt(logger))
	
	ctx := context.Background()
	
	tests := []struct {
		name        string
		promptName  string
		args        map[string]interface{}
		expectError bool
	}{
		{
			name:       "Valid clinical interpretation prompt",
			promptName: "clinical_interpretation",
			args: map[string]interface{}{
				"variant_notation": "NM_000001.3:c.123A>G",
				"gene_symbol":      "TEST1",
			},
			expectError: false,
		},
		{
			name:       "Valid evidence review prompt",
			promptName: "evidence_review",
			args: map[string]interface{}{
				"variant_id": "VAR_123456",
			},
			expectError: false,
		},
		{
			name:       "Valid report generation prompt",
			promptName: "report_generation",
			args: map[string]interface{}{
				"report_type": "clinical",
				"interpretation_data": map[string]interface{}{
					"classification": "pathogenic",
					"confidence":     "high",
				},
			},
			expectError: false,
		},
		{
			name:       "Valid ACMG training prompt",
			promptName: "acmg_training",
			args: map[string]interface{}{
				"training_level": "beginner",
			},
			expectError: false,
		},
		{
			name:       "Invalid prompt name",
			promptName: "nonexistent_prompt",
			args:       map[string]interface{}{},
			expectError: true,
		},
		{
			name:       "Missing required argument",
			promptName: "clinical_interpretation",
			args:       map[string]interface{}{},
			expectError: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rendered, err := manager.GetPrompt(ctx, tt.promptName, tt.args)
			
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, rendered)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, rendered)
				assert.Equal(t, tt.promptName, rendered.Name)
				assert.NotEmpty(t, rendered.Content)
				assert.NotEmpty(t, rendered.SystemPrompt)
				assert.NotEmpty(t, rendered.UserPrompt)
				assert.NotZero(t, rendered.GeneratedAt)
				assert.Equal(t, tt.args, rendered.Arguments)
			}
		})
	}
}

func TestPromptManager_ListPrompts(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	
	manager := NewPromptManager(logger)
	
	// Register all templates
	manager.RegisterTemplate("clinical_interpretation", NewClinicalInterpretationPrompt(logger))
	manager.RegisterTemplate("evidence_review", NewEvidenceReviewPrompt(logger))
	manager.RegisterTemplate("report_generation", NewReportGenerationPrompt(logger))
	manager.RegisterTemplate("acmg_training", NewACMGTrainingPrompt(logger))
	
	ctx := context.Background()
	promptList, err := manager.ListPrompts(ctx)
	
	require.NoError(t, err)
	require.NotNil(t, promptList)
	assert.Equal(t, 4, len(promptList.Prompts))
	assert.Equal(t, 4, promptList.Total)
	
	// Check that all expected prompts are present
	promptNames := make(map[string]bool)
	for _, prompt := range promptList.Prompts {
		promptNames[prompt.Name] = true
		assert.NotEmpty(t, prompt.Description)
		assert.NotEmpty(t, prompt.Version)
		assert.NotEmpty(t, prompt.Arguments)
	}
	
	assert.True(t, promptNames["clinical_interpretation"])
	assert.True(t, promptNames["evidence_review"])
	assert.True(t, promptNames["report_generation"])
	assert.True(t, promptNames["acmg_training"])
}

func TestPromptManager_GetPromptInfo(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	
	manager := NewPromptManager(logger)
	manager.RegisterTemplate("clinical_interpretation", NewClinicalInterpretationPrompt(logger))
	
	ctx := context.Background()
	
	// Test valid prompt
	info, err := manager.GetPromptInfo(ctx, "clinical_interpretation")
	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Equal(t, "clinical_interpretation", info.Name)
	assert.NotEmpty(t, info.Description)
	assert.NotEmpty(t, info.Arguments)
	
	// Test invalid prompt
	info, err = manager.GetPromptInfo(ctx, "nonexistent")
	assert.Error(t, err)
	assert.Nil(t, info)
}

func TestPromptManager_GetPromptSchema(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	
	manager := NewPromptManager(logger)
	manager.RegisterTemplate("clinical_interpretation", NewClinicalInterpretationPrompt(logger))
	
	ctx := context.Background()
	
	schema, err := manager.GetPromptSchema(ctx, "clinical_interpretation")
	require.NoError(t, err)
	require.NotNil(t, schema)
	
	// Verify schema structure
	assert.Equal(t, "http://json-schema.org/draft-07/schema#", schema["$schema"])
	assert.Equal(t, "object", schema["type"])
	
	properties, ok := schema["properties"].(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, properties, "variant_notation")
	
	required, ok := schema["required"].([]string)
	require.True(t, ok)
	assert.Contains(t, required, "variant_notation")
}

func TestTemplateRenderer_RenderTemplate(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	
	renderer := NewTemplateRenderer(logger)
	
	template := "Hello {{name}}, you have {{count}} messages."
	params := map[string]interface{}{
		"name":  "Alice",
		"count": 5,
	}
	
	result := renderer.RenderTemplate(template, params)
	expected := "Hello Alice, you have 5 messages."
	
	assert.Equal(t, expected, result)
}

func TestTemplateRenderer_RenderMarkdown(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	
	renderer := NewTemplateRenderer(logger)
	
	sections := map[string]string{
		"title":        "Test Document",
		"overview":     "This is an overview section.",
		"instructions": "These are the instructions.",
	}
	
	result := renderer.RenderMarkdown(sections)
	
	assert.Contains(t, result, "# Test Document")
	assert.Contains(t, result, "## Overview")
	assert.Contains(t, result, "This is an overview section.")
	assert.Contains(t, result, "## Instructions")
	assert.Contains(t, result, "These are the instructions.")
}

func TestTemplateRenderer_FormatList(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	
	renderer := NewTemplateRenderer(logger)
	
	items := []string{"First item", "Second item", "Third item"}
	
	// Test unordered list
	unorderedResult := renderer.FormatList(items, false)
	assert.Contains(t, unorderedResult, "- First item")
	assert.Contains(t, unorderedResult, "- Second item")
	assert.Contains(t, unorderedResult, "- Third item")
	
	// Test ordered list
	orderedResult := renderer.FormatList(items, true)
	assert.Contains(t, orderedResult, "1. First item")
	assert.Contains(t, orderedResult, "2. Second item")
	assert.Contains(t, orderedResult, "3. Third item")
}

func TestTemplateRenderer_FormatTable(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	
	renderer := NewTemplateRenderer(logger)
	
	headers := []string{"Name", "Age", "City"}
	rows := [][]string{
		{"Alice", "30", "New York"},
		{"Bob", "25", "Boston"},
		{"Carol", "35", "Chicago"},
	}
	
	result := renderer.FormatTable(headers, rows)
	
	// Check header row
	assert.Contains(t, result, "| Name | Age | City |")
	
	// Check separator row
	assert.Contains(t, result, "|---|---|---|")
	
	// Check data rows
	assert.Contains(t, result, "| Alice | 30 | New York |")
	assert.Contains(t, result, "| Bob | 25 | Boston |")
	assert.Contains(t, result, "| Carol | 35 | Chicago |")
}

func TestArgumentValidator_ValidateArguments(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	
	validator := NewArgumentValidator(logger)
	
	schema := []ArgumentInfo{
		{
			Name:        "required_string",
			Type:        "string",
			Required:    true,
			Constraints: []string{"min_length:3"},
		},
		{
			Name:     "optional_number",
			Type:     "number",
			Required: false,
		},
		{
			Name:        "enum_field",
			Type:        "string",
			Required:    false,
			Constraints: []string{"enum:option1,option2,option3"},
		},
	}
	
	tests := []struct {
		name        string
		args        map[string]interface{}
		expectError bool
		errorMsg    string
	}{
		{
			name: "Valid arguments",
			args: map[string]interface{}{
				"required_string": "valid_string",
				"optional_number": 42,
				"enum_field":      "option1",
			},
			expectError: false,
		},
		{
			name:        "Missing required argument",
			args:        map[string]interface{}{},
			expectError: true,
			errorMsg:    "required argument 'required_string' is missing",
		},
		{
			name: "Wrong type",
			args: map[string]interface{}{
				"required_string": "valid",
				"optional_number": "not_a_number",
			},
			expectError: true,
			errorMsg:    "must be a number",
		},
		{
			name: "String too short",
			args: map[string]interface{}{
				"required_string": "ab", // Less than 3 characters
			},
			expectError: true,
			errorMsg:    "must be at least 3 characters long",
		},
		{
			name: "Invalid enum value",
			args: map[string]interface{}{
				"required_string": "valid",
				"enum_field":      "invalid_option",
			},
			expectError: true,
			errorMsg:    "must be one of: option1,option2,option3",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateArguments(tt.args, schema)
			
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestClinicalInterpretationPrompt_Integration(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	
	prompt := NewClinicalInterpretationPrompt(logger)
	ctx := context.Background()
	
	// Test prompt info
	info := prompt.GetPromptInfo()
	assert.Equal(t, "clinical_interpretation", info.Name)
	assert.NotEmpty(t, info.Description)
	assert.NotEmpty(t, info.Arguments)
	assert.Contains(t, info.Tags, "clinical")
	assert.Contains(t, info.Tags, "acmg")
	
	// Test argument schema
	schema := prompt.GetArgumentSchema()
	assert.NotEmpty(t, schema)
	
	// Test supports prompt
	assert.True(t, prompt.SupportsPrompt("clinical_interpretation"))
	assert.True(t, prompt.SupportsPrompt("variant_interpretation"))
	assert.False(t, prompt.SupportsPrompt("unknown_prompt"))
	
	// Test argument validation
	validArgs := map[string]interface{}{
		"variant_notation": "NM_000001.3:c.123A>G",
		"gene_symbol":      "TEST1",
		"patient_phenotype": "Test phenotype",
	}
	
	err := prompt.ValidateArguments(validArgs)
	assert.NoError(t, err)
	
	// Test invalid arguments
	invalidArgs := map[string]interface{}{
		"variant_notation": "NM_", // Too short, less than 5 chars but matches pattern
	}
	
	err = prompt.ValidateArguments(invalidArgs)
	assert.Error(t, err)
	
	// Test prompt rendering
	rendered, err := prompt.RenderPrompt(ctx, validArgs)
	require.NoError(t, err)
	require.NotNil(t, rendered)
	
	assert.Equal(t, "clinical_interpretation", rendered.Name)
	assert.NotEmpty(t, rendered.Content)
	assert.NotEmpty(t, rendered.SystemPrompt)
	assert.NotEmpty(t, rendered.UserPrompt)
	assert.Contains(t, rendered.Content, "NM_000001.3:c.123A>G")
	assert.Contains(t, rendered.Content, "TEST1")
	assert.NotZero(t, rendered.GeneratedAt)
	assert.Equal(t, validArgs, rendered.Arguments)
}

func TestEvidenceReviewPrompt_Integration(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	
	prompt := NewEvidenceReviewPrompt(logger)
	ctx := context.Background()
	
	// Test basic functionality
	info := prompt.GetPromptInfo()
	assert.Equal(t, "evidence_review", info.Name)
	assert.Contains(t, info.Tags, "evidence")
	
	// Test argument validation and rendering
	validArgs := map[string]interface{}{
		"variant_id":     "VAR_123456",
		"evidence_types": []interface{}{"population", "clinical"},
		"review_depth":   "thorough",
	}
	
	err := prompt.ValidateArguments(validArgs)
	assert.NoError(t, err)
	
	rendered, err := prompt.RenderPrompt(ctx, validArgs)
	require.NoError(t, err)
	require.NotNil(t, rendered)
	
	assert.Equal(t, "evidence_review", rendered.Name)
	assert.Contains(t, rendered.Content, "VAR_123456")
	assert.Contains(t, rendered.Content, "thorough")
}

func TestReportGenerationPrompt_Integration(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	
	prompt := NewReportGenerationPrompt(logger)
	ctx := context.Background()
	
	// Test basic functionality
	info := prompt.GetPromptInfo()
	assert.Equal(t, "report_generation", info.Name)
	assert.Contains(t, info.Tags, "report")
	
	// Test argument validation and rendering
	validArgs := map[string]interface{}{
		"report_type": "clinical",
		"interpretation_data": map[string]interface{}{
			"classification": "pathogenic",
			"confidence":     "high",
		},
		"audience": "clinician",
	}
	
	err := prompt.ValidateArguments(validArgs)
	assert.NoError(t, err)
	
	rendered, err := prompt.RenderPrompt(ctx, validArgs)
	require.NoError(t, err)
	require.NotNil(t, rendered)
	
	assert.Equal(t, "report_generation", rendered.Name)
	assert.Contains(t, rendered.Content, "clinical")
	assert.Contains(t, rendered.Content, "pathogenic")
}

func TestACMGTrainingPrompt_Integration(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	
	prompt := NewACMGTrainingPrompt(logger)
	ctx := context.Background()
	
	// Test basic functionality
	info := prompt.GetPromptInfo()
	assert.Equal(t, "acmg_training", info.Name)
	assert.Contains(t, info.Tags, "training")
	assert.Contains(t, info.Tags, "acmg")
	
	// Test argument validation and rendering
	validArgs := map[string]interface{}{
		"training_level":      "beginner",
		"training_focus":      []interface{}{"criteria", "application"},
		"learning_style":      "interactive",
		"professional_role":   "resident",
	}
	
	err := prompt.ValidateArguments(validArgs)
	assert.NoError(t, err)
	
	rendered, err := prompt.RenderPrompt(ctx, validArgs)
	require.NoError(t, err)
	require.NotNil(t, rendered)
	
	assert.Equal(t, "acmg_training", rendered.Name)
	assert.Contains(t, rendered.Content, "beginner")
	assert.Contains(t, rendered.Content, "interactive")
}

func TestPromptTemplate_ArgumentTypes(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	
	prompt := NewClinicalInterpretationPrompt(logger)
	ctx := context.Background()
	
	// Test different argument types
	args := map[string]interface{}{
		"variant_notation":     "NM_000001.3:c.123A>G",
		"gene_symbol":          "TEST1",
		"testing_indication":   "diagnostic",
		"interpretation_level": "comprehensive",
		"evidence_focus":       []interface{}{"population", "clinical"},
		"clinical_context": map[string]interface{}{
			"age": 45,
			"sex": "female",
		},
	}
	
	rendered, err := prompt.RenderPrompt(ctx, args)
	require.NoError(t, err)
	require.NotNil(t, rendered)
	
	// Verify that different argument types are handled correctly
	assert.Contains(t, rendered.Content, "NM_000001.3:c.123A>G")
	assert.Contains(t, rendered.Content, "TEST1")
	assert.Contains(t, rendered.Content, "diagnostic")
	assert.Contains(t, rendered.Content, "comprehensive")
}

func TestPromptManager_ConcurrentAccess(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	
	manager := NewPromptManager(logger)
	manager.RegisterTemplate("clinical_interpretation", NewClinicalInterpretationPrompt(logger))
	
	ctx := context.Background()
	args := map[string]interface{}{
		"variant_notation": "NM_000001.3:c.123A>G",
	}
	
	// Test concurrent access
	const numGoroutines = 10
	done := make(chan bool, numGoroutines)
	
	for i := 0; i < numGoroutines; i++ {
		go func(routineID int) {
			defer func() { done <- true }()
			
			rendered, err := manager.GetPrompt(ctx, "clinical_interpretation", args)
			assert.NoError(t, err)
			assert.NotNil(t, rendered)
			assert.Equal(t, "clinical_interpretation", rendered.Name)
		}(i)
	}
	
	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}
}

func TestPromptTemplate_EdgeCases(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	
	ctx := context.Background()
	
	tests := []struct {
		name         string
		template     PromptTemplate
		args         map[string]interface{}
		expectError  bool
	}{
		{
			name:     "Empty arguments with defaults",
			template: NewEvidenceReviewPrompt(logger),
			args: map[string]interface{}{
				"variant_id": "VAR_123",
			},
			expectError: false,
		},
		{
			name:     "Empty arrays",
			template: NewACMGTrainingPrompt(logger),
			args: map[string]interface{}{
				"training_level":   "beginner",
				"training_focus":   []interface{}{},
				"specific_criteria": []interface{}{},
			},
			expectError: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.template.ValidateArguments(tt.args)
			if !tt.expectError {
				assert.NoError(t, err)
				
				rendered, err := tt.template.RenderPrompt(ctx, tt.args)
				assert.NoError(t, err)
				assert.NotNil(t, rendered)
				assert.NotEmpty(t, rendered.Content)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestPromptManager_ErrorHandling(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	
	manager := NewPromptManager(logger)
	ctx := context.Background()
	
	// Test with no templates registered
	rendered, err := manager.GetPrompt(ctx, "nonexistent", map[string]interface{}{})
	assert.Error(t, err)
	assert.Nil(t, rendered)
	assert.Contains(t, err.Error(), "no template found")
	
	// Test GetPromptInfo with no templates
	info, err := manager.GetPromptInfo(ctx, "nonexistent")
	assert.Error(t, err)
	assert.Nil(t, info)
	
	// Test GetPromptSchema with no templates
	schema, err := manager.GetPromptSchema(ctx, "nonexistent")
	assert.Error(t, err)
	assert.Nil(t, schema)
}