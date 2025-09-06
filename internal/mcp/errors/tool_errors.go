package errors

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// ToolErrorHandler handles tool-specific errors with detailed validation
type ToolErrorHandler struct {
	logger        *logrus.Logger
	validators    map[string]ToolValidator
	errorPatterns map[string]ErrorPattern
}

// ToolValidator defines validation rules for MCP tools
type ToolValidator struct {
	Name            string                         `json:"name"`
	RequiredParams  []string                       `json:"required_params"`
	ParamTypes      map[string]string              `json:"param_types"`
	ParamValidators map[string]ValidationRule      `json:"param_validators"`
	CustomValidator func(params map[string]interface{}) error
}

// ValidationRule defines parameter validation constraints
type ValidationRule struct {
	Type        string      `json:"type"`
	Required    bool        `json:"required"`
	MinLength   *int        `json:"min_length,omitempty"`
	MaxLength   *int        `json:"max_length,omitempty"`
	MinValue    *float64    `json:"min_value,omitempty"`
	MaxValue    *float64    `json:"max_value,omitempty"`
	Pattern     string      `json:"pattern,omitempty"`
	Enum        []string    `json:"enum,omitempty"`
	Default     interface{} `json:"default,omitempty"`
	Description string      `json:"description,omitempty"`
}

// ErrorPattern defines patterns for common tool errors
type ErrorPattern struct {
	Code        int      `json:"code"`
	Message     string   `json:"message"`
	Category    string   `json:"category"`
	Severity    string   `json:"severity"`
	Recoverable bool     `json:"recoverable"`
	Suggestions []string `json:"suggestions"`
}

// ToolError represents a tool-specific error with validation details
type ToolError struct {
	*MCPError
	ToolName       string                 `json:"tool_name"`
	ValidationErrors []ValidationError    `json:"validation_errors,omitempty"`
	Context        map[string]interface{} `json:"context,omitempty"`
	Timestamp      time.Time              `json:"timestamp"`
}

// ValidationError represents a specific parameter validation failure
type ValidationError struct {
	Parameter   string      `json:"parameter"`
	Value       interface{} `json:"value"`
	Expected    string      `json:"expected"`
	Actual      string      `json:"actual"`
	Rule        string      `json:"rule"`
	Message     string      `json:"message"`
	Suggestions []string    `json:"suggestions,omitempty"`
}

// Error implements the error interface
func (ve ValidationError) Error() string {
	return ve.Message
}

// NewToolErrorHandler creates a new tool error handler
func NewToolErrorHandler(logger *logrus.Logger) *ToolErrorHandler {
	handler := &ToolErrorHandler{
		logger:        logger,
		validators:    make(map[string]ToolValidator),
		errorPatterns: make(map[string]ErrorPattern),
	}
	
	handler.initializeDefaultValidators()
	handler.initializeErrorPatterns()
	
	return handler
}

// RegisterToolValidator registers a validator for a specific tool
func (teh *ToolErrorHandler) RegisterToolValidator(validator ToolValidator) {
	teh.validators[validator.Name] = validator
	teh.logger.WithField("tool", validator.Name).Info("Registered tool validator")
}

// ValidateToolCall validates parameters for a tool call
func (teh *ToolErrorHandler) ValidateToolCall(toolName string, params map[string]interface{}) error {
	validator, exists := teh.validators[toolName]
	if !exists {
		return teh.createToolError(toolName, ErrorToolNotFound, "Tool validator not found", nil, nil)
	}

	var validationErrors []ValidationError

	// Check required parameters
	for _, required := range validator.RequiredParams {
		if _, exists := params[required]; !exists {
			validationErrors = append(validationErrors, ValidationError{
				Parameter: required,
				Expected:  "present",
				Actual:    "missing",
				Rule:      "required",
				Message:   fmt.Sprintf("Required parameter '%s' is missing", required),
				Suggestions: []string{
					fmt.Sprintf("Add parameter '%s' to your request", required),
					"Check the tool documentation for required parameters",
				},
			})
		}
	}

	// Validate parameter types and constraints
	for param, value := range params {
		if rule, exists := validator.ParamValidators[param]; exists {
			if valErr := teh.validateParameter(param, value, rule); valErr != nil {
				validationErrors = append(validationErrors, *valErr)
			}
		}
	}

	// Run custom validation if available
	if validator.CustomValidator != nil {
		if err := validator.CustomValidator(params); err != nil {
			validationErrors = append(validationErrors, ValidationError{
				Parameter: "custom",
				Message:   err.Error(),
				Rule:      "custom_validation",
				Suggestions: []string{
					"Review the tool-specific requirements",
					"Check parameter combinations and dependencies",
				},
			})
		}
	}

	if len(validationErrors) > 0 {
		return teh.createToolError(toolName, ErrorInvalidParams, "Parameter validation failed", validationErrors, params)
	}

	return nil
}

// validateParameter validates a single parameter against its rule
func (teh *ToolErrorHandler) validateParameter(param string, value interface{}, rule ValidationRule) *ValidationError {
	// Type validation
	actualType := reflect.TypeOf(value).String()
	if rule.Type != "" && !teh.isCompatibleType(actualType, rule.Type) {
		return &ValidationError{
			Parameter: param,
			Value:     value,
			Expected:  rule.Type,
			Actual:    actualType,
			Rule:      "type",
			Message:   fmt.Sprintf("Parameter '%s' has incorrect type. Expected %s, got %s", param, rule.Type, actualType),
			Suggestions: []string{
				fmt.Sprintf("Convert '%s' to type %s", param, rule.Type),
				"Check the parameter documentation for expected types",
			},
		}
	}

	// String validations
	if str, ok := value.(string); ok {
		if rule.MinLength != nil && len(str) < *rule.MinLength {
			return &ValidationError{
				Parameter: param,
				Value:     value,
				Expected:  fmt.Sprintf("minimum length %d", *rule.MinLength),
				Actual:    fmt.Sprintf("length %d", len(str)),
				Rule:      "min_length",
				Message:   fmt.Sprintf("Parameter '%s' is too short", param),
				Suggestions: []string{
					fmt.Sprintf("Ensure '%s' has at least %d characters", param, *rule.MinLength),
				},
			}
		}

		if rule.MaxLength != nil && len(str) > *rule.MaxLength {
			return &ValidationError{
				Parameter: param,
				Value:     value,
				Expected:  fmt.Sprintf("maximum length %d", *rule.MaxLength),
				Actual:    fmt.Sprintf("length %d", len(str)),
				Rule:      "max_length",
				Message:   fmt.Sprintf("Parameter '%s' is too long", param),
				Suggestions: []string{
					fmt.Sprintf("Ensure '%s' has at most %d characters", param, *rule.MaxLength),
				},
			}
		}

		if len(rule.Enum) > 0 && !teh.contains(rule.Enum, str) {
			return &ValidationError{
				Parameter: param,
				Value:     value,
				Expected:  fmt.Sprintf("one of: %s", strings.Join(rule.Enum, ", ")),
				Actual:    str,
				Rule:      "enum",
				Message:   fmt.Sprintf("Parameter '%s' has invalid value", param),
				Suggestions: []string{
					fmt.Sprintf("Use one of the allowed values: %s", strings.Join(rule.Enum, ", ")),
				},
			}
		}
	}

	// Numeric validations
	if num, ok := teh.toFloat64(value); ok {
		if rule.MinValue != nil && num < *rule.MinValue {
			return &ValidationError{
				Parameter: param,
				Value:     value,
				Expected:  fmt.Sprintf("minimum value %f", *rule.MinValue),
				Actual:    fmt.Sprintf("value %f", num),
				Rule:      "min_value",
				Message:   fmt.Sprintf("Parameter '%s' is too small", param),
				Suggestions: []string{
					fmt.Sprintf("Use a value >= %f for '%s'", *rule.MinValue, param),
				},
			}
		}

		if rule.MaxValue != nil && num > *rule.MaxValue {
			return &ValidationError{
				Parameter: param,
				Value:     value,
				Expected:  fmt.Sprintf("maximum value %f", *rule.MaxValue),
				Actual:    fmt.Sprintf("value %f", num),
				Rule:      "max_value",
				Message:   fmt.Sprintf("Parameter '%s' is too large", param),
				Suggestions: []string{
					fmt.Sprintf("Use a value <= %f for '%s'", *rule.MaxValue, param),
				},
			}
		}
	}

	return nil
}

// createToolError creates a detailed tool error
func (teh *ToolErrorHandler) createToolError(toolName string, code int, message string, validationErrors []ValidationError, context map[string]interface{}) *ToolError {
	mcpError := &MCPError{
		Code:          code,
		Message:       message,
		Data:          make(map[string]interface{}),
		CorrelationID: generateCorrelationID(),
		Severity:      SeverityMedium,
		Category:      CategoryValidation,
		Recoverable:   true,
		Suggestions: []string{
			"Check parameter names and types",
			"Refer to tool documentation",
			"Validate input data before calling tools",
		},
	}

	if context != nil {
		mcpError.Data["context"] = context
	}

	toolError := &ToolError{
		MCPError:         mcpError,
		ToolName:         toolName,
		ValidationErrors: validationErrors,
		Context:          context,
		Timestamp:        time.Now(),
	}

	// Add detailed suggestions based on validation errors
	if len(validationErrors) > 0 {
		suggestions := make([]string, 0)
		for _, valErr := range validationErrors {
			suggestions = append(suggestions, valErr.Suggestions...)
		}
		mcpError.Suggestions = append(mcpError.Suggestions, suggestions...)
	}

	return toolError
}

// initializeDefaultValidators sets up validators for common MCP tools
func (teh *ToolErrorHandler) initializeDefaultValidators() {
	// Variant Classification Tool
	teh.RegisterToolValidator(ToolValidator{
		Name:           "classify_variant",
		RequiredParams: []string{"variant", "gene"},
		ParamTypes: map[string]string{
			"variant":          "string",
			"gene":            "string",
			"transcript":      "string",
			"population_data": "object",
		},
		ParamValidators: map[string]ValidationRule{
			"variant": {
				Type:        "string",
				Required:    true,
				MinLength:   &[]int{3}[0],
				MaxLength:   &[]int{100}[0],
				Description: "Variant in HGVS format",
			},
			"gene": {
				Type:        "string",
				Required:    true,
				MinLength:   &[]int{1}[0],
				MaxLength:   &[]int{20}[0],
				Description: "Gene symbol",
			},
			"classification_level": {
				Type: "string",
				Enum: []string{"pathogenic", "likely_pathogenic", "uncertain", "likely_benign", "benign"},
				Description: "ACMG/AMP classification level",
			},
		},
	})

	// Evidence Gathering Tool
	teh.RegisterToolValidator(ToolValidator{
		Name:           "gather_evidence",
		RequiredParams: []string{"variant_id"},
		ParamValidators: map[string]ValidationRule{
			"variant_id": {
				Type:        "string",
				Required:    true,
				MinLength:   &[]int{1}[0],
				Description: "Unique variant identifier",
			},
			"evidence_types": {
				Type: "array",
				Description: "Types of evidence to gather",
			},
			"databases": {
				Type: "array",
				Description: "External databases to query",
			},
		},
	})

	// Report Generation Tool
	teh.RegisterToolValidator(ToolValidator{
		Name:           "generate_report",
		RequiredParams: []string{"interpretation_id"},
		ParamValidators: map[string]ValidationRule{
			"interpretation_id": {
				Type:        "string",
				Required:    true,
				Description: "Interpretation identifier",
			},
			"format": {
				Type: "string",
				Enum: []string{"json", "html", "pdf", "text"},
				Default: "json",
				Description: "Report output format",
			},
			"include_evidence": {
				Type:    "boolean",
				Default: true,
				Description: "Include evidence details in report",
			},
		},
	})
}

// initializeErrorPatterns sets up common error patterns
func (teh *ToolErrorHandler) initializeErrorPatterns() {
	teh.errorPatterns["invalid_variant_format"] = ErrorPattern{
		Code:        ErrorInvalidParams,
		Message:     "Invalid variant format",
		Category:    CategoryValidation,
		Severity:    SeverityMedium,
		Recoverable: true,
		Suggestions: []string{
			"Use HGVS nomenclature (e.g., NM_000314.6:c.1A>G)",
			"Verify chromosome coordinates",
			"Check reference genome version",
		},
	}

	teh.errorPatterns["gene_not_found"] = ErrorPattern{
		Code:        ErrorResourceNotFound,
		Message:     "Gene not found in database",
		Category:    CategoryResource,
		Severity:    SeverityMedium,
		Recoverable: true,
		Suggestions: []string{
			"Check gene symbol spelling",
			"Try using gene aliases or previous symbols",
			"Verify gene exists in current genome build",
		},
	}

	teh.errorPatterns["database_timeout"] = ErrorPattern{
		Code:        ErrorServiceUnavailable,
		Message:     "Database query timed out",
		Category:    CategoryExternal,
		Severity:    SeverityHigh,
		Recoverable: true,
		Suggestions: []string{
			"Retry the request after a brief delay",
			"Consider using cached data if available",
			"Check database service status",
		},
	}
}

// Helper methods

func (teh *ToolErrorHandler) isCompatibleType(actual, expected string) bool {
	typeMap := map[string][]string{
		"string":  {"string"},
		"number":  {"int", "int64", "float64", "float32"},
		"boolean": {"bool"},
		"array":   {"[]interface{}", "[]string", "[]int", "[]float64"},
		"object":  {"map[string]interface{}", "struct"},
	}

	if compatibles, exists := typeMap[expected]; exists {
		for _, compatible := range compatibles {
			if strings.Contains(actual, compatible) {
				return true
			}
		}
	}

	return actual == expected
}

func (teh *ToolErrorHandler) toFloat64(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case float32:
		return float64(v), true
	case float64:
		return v, true
	default:
		return 0, false
	}
}

func (teh *ToolErrorHandler) contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// GetValidatorInfo returns information about registered validators
func (teh *ToolErrorHandler) GetValidatorInfo() map[string]ToolValidator {
	result := make(map[string]ToolValidator)
	for name, validator := range teh.validators {
		result[name] = validator
	}
	return result
}

// GetErrorPatterns returns available error patterns
func (teh *ToolErrorHandler) GetErrorPatterns() map[string]ErrorPattern {
	result := make(map[string]ErrorPattern)
	for name, pattern := range teh.errorPatterns {
		result[name] = pattern
	}
	return result
}