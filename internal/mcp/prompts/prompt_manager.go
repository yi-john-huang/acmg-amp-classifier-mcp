package prompts

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// PromptManager manages MCP prompts and their templates
type PromptManager struct {
	logger    *logrus.Logger
	templates map[string]PromptTemplate
	mutex     sync.RWMutex
}

// PromptTemplate defines the interface for prompt templates
type PromptTemplate interface {
	// GetPromptInfo returns metadata about this prompt template
	GetPromptInfo() PromptInfo
	
	// RenderPrompt renders the prompt with given arguments
	RenderPrompt(ctx context.Context, args map[string]interface{}) (*RenderedPrompt, error)
	
	// ValidateArguments validates the provided arguments
	ValidateArguments(args map[string]interface{}) error
	
	// GetArgumentSchema returns the JSON schema for prompt arguments
	GetArgumentSchema() map[string]interface{}
	
	// SupportsPrompt checks if this template can handle the given prompt name
	SupportsPrompt(name string) bool
}

// PromptInfo contains metadata about a prompt template
type PromptInfo struct {
	Name         string                 `json:"name"`
	Description  string                 `json:"description"`
	Version      string                 `json:"version"`
	Arguments    []ArgumentInfo         `json:"arguments"`
	Examples     []PromptExample        `json:"examples"`
	Tags         []string               `json:"tags"`
	Category     string                 `json:"category"`
	Difficulty   string                 `json:"difficulty"`
	UsageNotes   []string               `json:"usage_notes,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// ArgumentInfo describes a prompt argument
type ArgumentInfo struct {
	Name         string      `json:"name"`
	Description  string      `json:"description"`
	Type         string      `json:"type"`
	Required     bool        `json:"required"`
	DefaultValue interface{} `json:"default_value,omitempty"`
	Examples     []string    `json:"examples,omitempty"`
	Constraints  []string    `json:"constraints,omitempty"`
}

// PromptExample provides usage examples for prompts
type PromptExample struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Arguments   map[string]interface{} `json:"arguments"`
	ExpectedUse string                 `json:"expected_use"`
}

// RenderedPrompt represents a fully rendered prompt ready for AI agent
type RenderedPrompt struct {
	Name         string                 `json:"name"`
	Content      string                 `json:"content"`
	SystemPrompt string                 `json:"system_prompt,omitempty"`
	UserPrompt   string                 `json:"user_prompt,omitempty"`
	Context      string                 `json:"context,omitempty"`
	Instructions []string               `json:"instructions,omitempty"`
	Examples     []string               `json:"examples,omitempty"`
	References   []string               `json:"references,omitempty"`
	Arguments    map[string]interface{} `json:"arguments"`
	GeneratedAt  time.Time              `json:"generated_at"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// PromptList represents a list of available prompts
type PromptList struct {
	Prompts []PromptInfo `json:"prompts"`
	Total   int          `json:"total"`
}

// NewPromptManager creates a new prompt manager
func NewPromptManager(logger *logrus.Logger) *PromptManager {
	return &PromptManager{
		logger:    logger,
		templates: make(map[string]PromptTemplate),
	}
}

// RegisterTemplate registers a new prompt template
func (pm *PromptManager) RegisterTemplate(name string, template PromptTemplate) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()
	
	pm.templates[name] = template
	pm.logger.WithFields(logrus.Fields{
		"template": name,
		"info":     template.GetPromptInfo().Description,
	}).Info("Registered prompt template")
}

// GetPrompt retrieves and renders a prompt by name with given arguments
func (pm *PromptManager) GetPrompt(ctx context.Context, name string, args map[string]interface{}) (*RenderedPrompt, error) {
	pm.logger.WithField("name", name).Debug("Getting prompt")
	
	// Find appropriate template
	template := pm.findTemplate(name)
	if template == nil {
		return nil, fmt.Errorf("no template found for prompt: %s", name)
	}
	
	// Validate arguments
	if err := template.ValidateArguments(args); err != nil {
		return nil, fmt.Errorf("argument validation failed for prompt %s: %w", name, err)
	}
	
	// Render prompt
	rendered, err := template.RenderPrompt(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("failed to render prompt %s: %w", name, err)
	}
	
	pm.logger.WithFields(logrus.Fields{
		"name":         name,
		"template":     template.GetPromptInfo().Name,
		"content_size": len(rendered.Content),
	}).Info("Rendered prompt successfully")
	
	return rendered, nil
}

// ListPrompts lists all available prompts
func (pm *PromptManager) ListPrompts(ctx context.Context) (*PromptList, error) {
	pm.logger.Debug("Listing prompts")
	
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()
	
	prompts := make([]PromptInfo, 0, len(pm.templates))
	for _, template := range pm.templates {
		prompts = append(prompts, template.GetPromptInfo())
	}
	
	result := &PromptList{
		Prompts: prompts,
		Total:   len(prompts),
	}
	
	pm.logger.WithField("count", len(prompts)).Info("Listed prompts")
	return result, nil
}

// GetPromptInfo returns metadata about a specific prompt
func (pm *PromptManager) GetPromptInfo(ctx context.Context, name string) (*PromptInfo, error) {
	template := pm.findTemplate(name)
	if template == nil {
		return nil, fmt.Errorf("no template found for prompt: %s", name)
	}
	
	info := template.GetPromptInfo()
	return &info, nil
}

// GetPromptSchema returns the JSON schema for a prompt's arguments
func (pm *PromptManager) GetPromptSchema(ctx context.Context, name string) (map[string]interface{}, error) {
	template := pm.findTemplate(name)
	if template == nil {
		return nil, fmt.Errorf("no template found for prompt: %s", name)
	}
	
	return template.GetArgumentSchema(), nil
}

// findTemplate finds the appropriate template for a prompt name
func (pm *PromptManager) findTemplate(name string) PromptTemplate {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()
	
	for _, template := range pm.templates {
		if template.SupportsPrompt(name) {
			return template
		}
	}
	
	return nil
}

// GetTemplateInfo returns information about all registered templates
func (pm *PromptManager) GetTemplateInfo() []PromptInfo {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()
	
	info := make([]PromptInfo, 0, len(pm.templates))
	for _, template := range pm.templates {
		info = append(info, template.GetPromptInfo())
	}
	
	return info
}

// TemplateRenderer provides utilities for rendering prompt templates
type TemplateRenderer struct {
	logger *logrus.Logger
}

// NewTemplateRenderer creates a new template renderer
func NewTemplateRenderer(logger *logrus.Logger) *TemplateRenderer {
	return &TemplateRenderer{
		logger: logger,
	}
}

// RenderTemplate renders a template string with given parameters
func (tr *TemplateRenderer) RenderTemplate(template string, params map[string]interface{}) string {
	result := template
	
	for key, value := range params {
		placeholder := fmt.Sprintf("{{%s}}", key)
		replacement := fmt.Sprintf("%v", value)
		result = strings.ReplaceAll(result, placeholder, replacement)
	}
	
	return result
}

// RenderMarkdown formats content as markdown with proper structure
func (tr *TemplateRenderer) RenderMarkdown(sections map[string]string) string {
	var builder strings.Builder
	
	// Define section order for consistent output
	sectionOrder := []string{
		"title", "overview", "objective", "context", "instructions", 
		"steps", "examples", "guidelines", "references", "notes",
	}
	
	for _, section := range sectionOrder {
		if content, exists := sections[section]; exists && content != "" {
			switch section {
			case "title":
				builder.WriteString(fmt.Sprintf("# %s\n\n", content))
			case "overview", "objective", "context":
				builder.WriteString(fmt.Sprintf("## %s\n\n%s\n\n", 
					strings.Title(section), content))
			default:
				builder.WriteString(fmt.Sprintf("## %s\n\n%s\n\n", 
					strings.Title(section), content))
			}
		}
	}
	
	return builder.String()
}

// FormatList formats a list of items with proper markdown formatting
func (tr *TemplateRenderer) FormatList(items []string, ordered bool) string {
	var builder strings.Builder
	
	for i, item := range items {
		if ordered {
			builder.WriteString(fmt.Sprintf("%d. %s\n", i+1, item))
		} else {
			builder.WriteString(fmt.Sprintf("- %s\n", item))
		}
	}
	
	return builder.String()
}

// FormatTable formats data as a markdown table
func (tr *TemplateRenderer) FormatTable(headers []string, rows [][]string) string {
	var builder strings.Builder
	
	// Header row
	builder.WriteString("| ")
	builder.WriteString(strings.Join(headers, " | "))
	builder.WriteString(" |\n")
	
	// Separator row
	builder.WriteString("|")
	for range headers {
		builder.WriteString("---|")
	}
	builder.WriteString("\n")
	
	// Data rows
	for _, row := range rows {
		builder.WriteString("| ")
		builder.WriteString(strings.Join(row, " | "))
		builder.WriteString(" |\n")
	}
	
	return builder.String()
}

// ArgumentValidator provides validation for prompt arguments
type ArgumentValidator struct {
	logger *logrus.Logger
}

// NewArgumentValidator creates a new argument validator
func NewArgumentValidator(logger *logrus.Logger) *ArgumentValidator {
	return &ArgumentValidator{
		logger: logger,
	}
}

// ValidateArguments validates arguments against expected schema
func (av *ArgumentValidator) ValidateArguments(args map[string]interface{}, schema []ArgumentInfo) error {
	// Check required arguments
	for _, arg := range schema {
		if arg.Required {
			if _, exists := args[arg.Name]; !exists {
				return fmt.Errorf("required argument '%s' is missing", arg.Name)
			}
		}
	}
	
	// Validate argument types and constraints
	for name, value := range args {
		argInfo := av.findArgumentInfo(name, schema)
		if argInfo == nil {
			av.logger.WithField("argument", name).Warn("Unknown argument provided")
			continue
		}
		
		if err := av.validateArgumentType(name, value, argInfo.Type); err != nil {
			return err
		}
		
		if err := av.validateConstraints(name, value, argInfo.Constraints); err != nil {
			return err
		}
	}
	
	return nil
}

// findArgumentInfo finds argument info by name
func (av *ArgumentValidator) findArgumentInfo(name string, schema []ArgumentInfo) *ArgumentInfo {
	for _, arg := range schema {
		if arg.Name == name {
			return &arg
		}
	}
	return nil
}

// validateArgumentType validates the type of an argument
func (av *ArgumentValidator) validateArgumentType(name string, value interface{}, expectedType string) error {
	switch expectedType {
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("argument '%s' must be a string", name)
		}
	case "number", "integer":
		switch value.(type) {
		case int, int32, int64, float32, float64:
			// Valid numeric types
		default:
			return fmt.Errorf("argument '%s' must be a number", name)
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("argument '%s' must be a boolean", name)
		}
	case "array":
		if _, ok := value.([]interface{}); !ok {
			return fmt.Errorf("argument '%s' must be an array", name)
		}
	case "object":
		if _, ok := value.(map[string]interface{}); !ok {
			return fmt.Errorf("argument '%s' must be an object", name)
		}
	}
	
	return nil
}

// validateConstraints validates argument constraints
func (av *ArgumentValidator) validateConstraints(name string, value interface{}, constraints []string) error {
	for _, constraint := range constraints {
		if err := av.validateSingleConstraint(name, value, constraint); err != nil {
			return err
		}
	}
	return nil
}

// validateSingleConstraint validates a single constraint
func (av *ArgumentValidator) validateSingleConstraint(name string, value interface{}, constraint string) error {
	// Parse constraint format: "type:condition"
	parts := strings.SplitN(constraint, ":", 2)
	if len(parts) != 2 {
		return nil // Invalid constraint format, skip
	}
	
	constraintType := parts[0]
	condition := parts[1]
	
	switch constraintType {
	case "min_length":
		if str, ok := value.(string); ok {
			var minLength int
			if _, err := fmt.Sscanf(condition, "%d", &minLength); err == nil {
				if len(str) < minLength {
					return fmt.Errorf("argument '%s' must be at least %d characters long", name, minLength)
				}
			}
		}
	case "max_length":
		if str, ok := value.(string); ok {
			var maxLength int
			if _, err := fmt.Sscanf(condition, "%d", &maxLength); err == nil {
				if len(str) > maxLength {
					return fmt.Errorf("argument '%s' must be at most %d characters long", name, maxLength)
				}
			}
		}
	case "pattern":
		if _, ok := value.(string); ok {
			// This would require regex validation - simplified for now
			av.logger.WithFields(logrus.Fields{
				"argument": name,
				"pattern":  condition,
			}).Debug("Pattern validation not implemented")
		}
	case "enum":
		allowedValues := strings.Split(condition, ",")
		valueStr := fmt.Sprintf("%v", value)
		found := false
		for _, allowed := range allowedValues {
			if strings.TrimSpace(allowed) == valueStr {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("argument '%s' must be one of: %s", name, condition)
		}
	}
	
	return nil
}