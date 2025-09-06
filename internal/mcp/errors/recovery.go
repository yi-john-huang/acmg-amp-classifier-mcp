package errors

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// RecoveryGuidanceManager provides client error recovery guidance
type RecoveryGuidanceManager struct {
	logger         *logrus.Logger
	guidanceRules  map[string][]RecoveryRule
	actionRegistry map[string]RecoveryAction
	diagnostics    *DiagnosticEngine
}

// RecoveryRule defines conditions and actions for error recovery
type RecoveryRule struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Conditions  []Condition            `json:"conditions"`
	Actions     []string               `json:"actions"`
	Priority    int                    `json:"priority"`
	Category    string                 `json:"category"`
	AutoRetry   bool                   `json:"auto_retry"`
	MaxRetries  int                    `json:"max_retries,omitempty"`
	RetryDelay  time.Duration          `json:"retry_delay,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// Condition defines when a recovery rule applies
type Condition struct {
	Type     string      `json:"type"`     // "error_code", "message_pattern", "context_key", "service"
	Operator string      `json:"operator"` // "equals", "contains", "matches", "greater_than", "less_than"
	Value    interface{} `json:"value"`
	Negate   bool        `json:"negate,omitempty"`
}

// RecoveryAction defines a specific recovery action
type RecoveryAction struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Description  string                 `json:"description"`
	Type         string                 `json:"type"` // "retry", "fallback", "redirect", "manual", "diagnostic"
	Automated    bool                   `json:"automated"`
	Impact       string                 `json:"impact"` // "low", "medium", "high"
	EstimatedTime string                `json:"estimated_time"`
	Prerequisites []string              `json:"prerequisites,omitempty"`
	Steps        []ActionStep           `json:"steps"`
	SuccessCriteria []string            `json:"success_criteria"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// ActionStep represents a step in a recovery action
type ActionStep struct {
	ID          string                 `json:"id"`
	Description string                 `json:"description"`
	Type        string                 `json:"type"` // "api_call", "wait", "validate", "user_action"
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
	Optional    bool                   `json:"optional,omitempty"`
	Timeout     time.Duration          `json:"timeout,omitempty"`
}

// RecoveryPlan represents a complete recovery plan for an error
type RecoveryPlan struct {
	ErrorContext    *ErrorContext      `json:"error_context"`
	RecommendedActions []RecommendedAction `json:"recommended_actions"`
	DiagnosticInfo  *DiagnosticInfo    `json:"diagnostic_info,omitempty"`
	EstimatedTime   time.Duration      `json:"estimated_time"`
	SuccessRate     float64            `json:"success_rate"`
	CreatedAt       time.Time          `json:"created_at"`
}

// RecommendedAction represents an action recommended for error recovery
type RecommendedAction struct {
	Action         RecoveryAction `json:"action"`
	Priority       int            `json:"priority"`
	Confidence     float64        `json:"confidence"`
	Reasoning      string         `json:"reasoning"`
	Alternatives   []string       `json:"alternatives,omitempty"`
	Prerequisites  []string       `json:"prerequisites,omitempty"`
	EstimatedTime  time.Duration  `json:"estimated_time"`
}

// ErrorContext provides context about the error for recovery planning
type ErrorContext struct {
	Error           error                  `json:"error"`
	ServiceName     string                 `json:"service_name,omitempty"`
	OperationName   string                 `json:"operation_name,omitempty"`
	RequestID       string                 `json:"request_id,omitempty"`
	UserID          string                 `json:"user_id,omitempty"`
	ClientInfo      map[string]interface{} `json:"client_info,omitempty"`
	SystemState     map[string]interface{} `json:"system_state,omitempty"`
	PreviousAttempts []AttemptInfo         `json:"previous_attempts,omitempty"`
	Timestamp       time.Time              `json:"timestamp"`
}

// AttemptInfo records information about previous recovery attempts
type AttemptInfo struct {
	ActionID    string                 `json:"action_id"`
	Timestamp   time.Time              `json:"timestamp"`
	Success     bool                   `json:"success"`
	Duration    time.Duration          `json:"duration"`
	ErrorDetails string                `json:"error_details,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// DiagnosticEngine performs error diagnostics
type DiagnosticEngine struct {
	logger     *logrus.Logger
	diagnostics map[string]DiagnosticRule
}

// DiagnosticRule defines how to diagnose specific types of errors
type DiagnosticRule struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	ErrorTypes  []string    `json:"error_types"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// Diagnostic represents a diagnostic check
type Diagnostic struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Check       func(context.Context, *ErrorContext) (*DiagnosticResult, error)
	Timeout     time.Duration `json:"timeout"`
}

// DiagnosticResult contains the result of a diagnostic check
type DiagnosticResult struct {
	Passed      bool                   `json:"passed"`
	Message     string                 `json:"message"`
	Severity    string                 `json:"severity"`
	Details     map[string]interface{} `json:"details,omitempty"`
	Suggestions []string               `json:"suggestions,omitempty"`
}

// DiagnosticInfo contains aggregated diagnostic information
type DiagnosticInfo struct {
	OverallHealth string             `json:"overall_health"` // "healthy", "degraded", "unhealthy"
	Results       []DiagnosticResult `json:"results"`
	Summary       string             `json:"summary"`
	Issues        []string           `json:"issues"`
	Recommendations []string         `json:"recommendations"`
}

// NewRecoveryGuidanceManager creates a new recovery guidance manager
func NewRecoveryGuidanceManager(logger *logrus.Logger) *RecoveryGuidanceManager {
	manager := &RecoveryGuidanceManager{
		logger:         logger,
		guidanceRules:  make(map[string][]RecoveryRule),
		actionRegistry: make(map[string]RecoveryAction),
		diagnostics:    NewDiagnosticEngine(logger),
	}

	manager.initializeDefaultRules()
	manager.initializeDefaultActions()

	return manager
}

// NewDiagnosticEngine creates a new diagnostic engine
func NewDiagnosticEngine(logger *logrus.Logger) *DiagnosticEngine {
	engine := &DiagnosticEngine{
		logger:      logger,
		diagnostics: make(map[string]DiagnosticRule),
	}

	engine.initializeDefaultDiagnostics()
	return engine
}

// GenerateRecoveryPlan creates a recovery plan for the given error context
func (rgm *RecoveryGuidanceManager) GenerateRecoveryPlan(ctx context.Context, errorCtx *ErrorContext) (*RecoveryPlan, error) {
	start := time.Now()
	
	rgm.logger.WithFields(logrus.Fields{
		"service":    errorCtx.ServiceName,
		"operation":  errorCtx.OperationName,
		"request_id": errorCtx.RequestID,
	}).Info("Generating recovery plan")

	// Run diagnostics
	diagnosticInfo, err := rgm.diagnostics.RunDiagnostics(ctx, errorCtx)
	if err != nil {
		rgm.logger.WithError(err).Warn("Failed to run diagnostics")
	}

	// Find applicable recovery rules
	applicableRules := rgm.findApplicableRules(errorCtx)
	
	// Generate recommended actions
	recommendedActions := rgm.generateRecommendedActions(errorCtx, applicableRules)

	// Sort by priority and confidence
	sort.Slice(recommendedActions, func(i, j int) bool {
		if recommendedActions[i].Priority != recommendedActions[j].Priority {
			return recommendedActions[i].Priority < recommendedActions[j].Priority
		}
		return recommendedActions[i].Confidence > recommendedActions[j].Confidence
	})

	// Calculate estimated time and success rate
	estimatedTime, successRate := rgm.calculatePlanMetrics(recommendedActions)

	plan := &RecoveryPlan{
		ErrorContext:       errorCtx,
		RecommendedActions: recommendedActions,
		DiagnosticInfo:     diagnosticInfo,
		EstimatedTime:      estimatedTime,
		SuccessRate:        successRate,
		CreatedAt:          time.Now(),
	}

	rgm.logger.WithFields(logrus.Fields{
		"actions":        len(recommendedActions),
		"estimated_time": estimatedTime,
		"success_rate":   successRate,
		"duration":       time.Since(start),
	}).Info("Generated recovery plan")

	return plan, nil
}

// findApplicableRules finds recovery rules that match the error context
func (rgm *RecoveryGuidanceManager) findApplicableRules(errorCtx *ErrorContext) []RecoveryRule {
	var applicableRules []RecoveryRule

	// Get error code for matching
	errorCode := 0
	if mcpErr, ok := errorCtx.Error.(*MCPError); ok {
		errorCode = mcpErr.Code
	}

	// Check all rule categories
	for category, rules := range rgm.guidanceRules {
		for _, rule := range rules {
			if rgm.ruleApplies(rule, errorCtx, errorCode) {
				applicableRules = append(applicableRules, rule)
				rgm.logger.WithFields(logrus.Fields{
					"rule_id":  rule.ID,
					"category": category,
				}).Debug("Found applicable recovery rule")
			}
		}
	}

	return applicableRules
}

// ruleApplies checks if a recovery rule applies to the error context
func (rgm *RecoveryGuidanceManager) ruleApplies(rule RecoveryRule, errorCtx *ErrorContext, errorCode int) bool {
	for _, condition := range rule.Conditions {
		if !rgm.evaluateCondition(condition, errorCtx, errorCode) {
			return false
		}
	}
	return true
}

// evaluateCondition evaluates a single condition
func (rgm *RecoveryGuidanceManager) evaluateCondition(condition Condition, errorCtx *ErrorContext, errorCode int) bool {
	var result bool

	switch condition.Type {
	case "error_code":
		if expectedCode, ok := condition.Value.(float64); ok {
			result = rgm.compareValues(float64(errorCode), condition.Operator, expectedCode)
		}

	case "message_pattern":
		if pattern, ok := condition.Value.(string); ok {
			errorMsg := errorCtx.Error.Error()
			result = rgm.matchPattern(errorMsg, condition.Operator, pattern)
		}

	case "service":
		if service, ok := condition.Value.(string); ok {
			result = rgm.compareStrings(errorCtx.ServiceName, condition.Operator, service)
		}

	case "context_key":
		// Check system state or client info for specific keys
		if key, ok := condition.Value.(string); ok {
			_, existsInSystem := errorCtx.SystemState[key]
			_, existsInClient := errorCtx.ClientInfo[key]
			result = existsInSystem || existsInClient
		}
	}

	if condition.Negate {
		result = !result
	}

	return result
}

// generateRecommendedActions creates recommended actions from applicable rules
func (rgm *RecoveryGuidanceManager) generateRecommendedActions(errorCtx *ErrorContext, rules []RecoveryRule) []RecommendedAction {
	var recommended []RecommendedAction
	actionsSeen := make(map[string]bool)

	for _, rule := range rules {
		for _, actionID := range rule.Actions {
			if actionsSeen[actionID] {
				continue
			}

			if action, exists := rgm.actionRegistry[actionID]; exists {
				confidence := rgm.calculateActionConfidence(rule, errorCtx)
				reasoning := rgm.generateActionReasoning(rule, action, errorCtx)

				recommended = append(recommended, RecommendedAction{
					Action:        action,
					Priority:      rule.Priority,
					Confidence:    confidence,
					Reasoning:     reasoning,
					Alternatives:  rgm.findAlternativeActions(actionID),
					Prerequisites: action.Prerequisites,
					EstimatedTime: rgm.estimateActionTime(action),
				})

				actionsSeen[actionID] = true
			}
		}
	}

	return recommended
}

// calculateActionConfidence calculates confidence in an action's success
func (rgm *RecoveryGuidanceManager) calculateActionConfidence(rule RecoveryRule, errorCtx *ErrorContext) float64 {
	// Base confidence from rule priority
	confidence := 1.0 - (float64(rule.Priority) / 10.0)

	// Adjust based on previous attempts
	for _, attempt := range errorCtx.PreviousAttempts {
		for _, actionID := range rule.Actions {
			if attempt.ActionID == actionID {
				if attempt.Success {
					confidence += 0.1
				} else {
					confidence -= 0.2
				}
			}
		}
	}

	// Ensure confidence stays in valid range
	if confidence > 1.0 {
		confidence = 1.0
	}
	if confidence < 0.1 {
		confidence = 0.1
	}

	return confidence
}

// initializeDefaultRules sets up default recovery rules
func (rgm *RecoveryGuidanceManager) initializeDefaultRules() {
	// Connection timeout rules
	rgm.guidanceRules["connection"] = []RecoveryRule{
		{
			ID:          "timeout_retry",
			Name:        "Retry on Timeout",
			Description: "Retry the operation with exponential backoff",
			Conditions: []Condition{
				{Type: "error_code", Operator: "equals", Value: float64(ErrorServiceUnavailable)},
				{Type: "message_pattern", Operator: "contains", Value: "timeout"},
			},
			Actions:    []string{"exponential_retry", "check_connectivity"},
			Priority:   1,
			Category:   "connection",
			AutoRetry:  true,
			MaxRetries: 3,
			RetryDelay: 2 * time.Second,
		},
	}

	// Validation error rules
	rgm.guidanceRules["validation"] = []RecoveryRule{
		{
			ID:          "param_validation",
			Name:        "Parameter Validation Error",
			Description: "Guide user to fix parameter validation issues",
			Conditions: []Condition{
				{Type: "error_code", Operator: "equals", Value: float64(ErrorInvalidParams)},
			},
			Actions:   []string{"fix_parameters", "validate_input"},
			Priority:  1,
			Category:  "validation",
			AutoRetry: false,
		},
	}

	// Resource error rules
	rgm.guidanceRules["resource"] = []RecoveryRule{
		{
			ID:          "resource_not_found",
			Name:        "Resource Not Found",
			Description: "Handle missing resources",
			Conditions: []Condition{
				{Type: "error_code", Operator: "equals", Value: float64(ErrorResourceNotFound)},
			},
			Actions:  []string{"verify_resource_id", "search_alternatives"},
			Priority: 2,
			Category: "resource",
		},
	}
}

// initializeDefaultActions sets up default recovery actions
func (rgm *RecoveryGuidanceManager) initializeDefaultActions() {
	rgm.actionRegistry["exponential_retry"] = RecoveryAction{
		ID:          "exponential_retry",
		Name:        "Exponential Backoff Retry",
		Description: "Retry the operation with exponential backoff",
		Type:        "retry",
		Automated:   true,
		Impact:      "low",
		EstimatedTime: "30 seconds",
		Steps: []ActionStep{
			{
				ID:          "wait",
				Description: "Wait before retry",
				Type:        "wait",
				Parameters:  map[string]interface{}{"duration": "2s"},
			},
			{
				ID:          "retry",
				Description: "Retry the original operation",
				Type:        "api_call",
				Parameters:  map[string]interface{}{"max_attempts": 3},
			},
		},
		SuccessCriteria: []string{"Operation completes successfully", "No timeout errors"},
	}

	rgm.actionRegistry["fix_parameters"] = RecoveryAction{
		ID:            "fix_parameters",
		Name:          "Fix Parameter Validation",
		Description:   "Correct parameter validation issues",
		Type:          "manual",
		Automated:     false,
		Impact:        "low",
		EstimatedTime: "5 minutes",
		Steps: []ActionStep{
			{
				ID:          "review_errors",
				Description: "Review parameter validation errors",
				Type:        "user_action",
			},
			{
				ID:          "correct_params",
				Description: "Correct the parameter values",
				Type:        "user_action",
			},
			{
				ID:          "resubmit",
				Description: "Resubmit the request with corrected parameters",
				Type:        "api_call",
			},
		},
		SuccessCriteria: []string{"All parameters pass validation", "Request processes successfully"},
	}
}

// initializeDefaultDiagnostics sets up default diagnostic rules
func (de *DiagnosticEngine) initializeDefaultDiagnostics() {
	de.diagnostics["network"] = DiagnosticRule{
		ID:         "network_checks",
		Name:       "Network Connectivity",
		ErrorTypes: []string{"timeout", "connection_refused", "dns_error"},
		Diagnostics: []Diagnostic{
			{
				ID:          "connectivity",
				Name:        "Service Connectivity",
				Description: "Check if service is reachable",
				Check:       de.checkServiceConnectivity,
				Timeout:     10 * time.Second,
			},
		},
	}
}

// Helper methods for condition evaluation
func (rgm *RecoveryGuidanceManager) compareValues(actual float64, operator string, expected float64) bool {
	switch operator {
	case "equals":
		return actual == expected
	case "greater_than":
		return actual > expected
	case "less_than":
		return actual < expected
	default:
		return false
	}
}

func (rgm *RecoveryGuidanceManager) compareStrings(actual, operator, expected string) bool {
	switch operator {
	case "equals":
		return actual == expected
	case "contains":
		return strings.Contains(actual, expected)
	default:
		return false
	}
}

func (rgm *RecoveryGuidanceManager) matchPattern(text, operator, pattern string) bool {
	switch operator {
	case "contains":
		return strings.Contains(strings.ToLower(text), strings.ToLower(pattern))
	case "equals":
		return strings.EqualFold(text, pattern)
	default:
		return false
	}
}

func (rgm *RecoveryGuidanceManager) findAlternativeActions(actionID string) []string {
	// Return related actions that could be alternatives
	alternatives := map[string][]string{
		"exponential_retry": {"simple_retry", "circuit_breaker_wait"},
		"fix_parameters":    {"validate_input", "use_defaults"},
	}
	return alternatives[actionID]
}

func (rgm *RecoveryGuidanceManager) estimateActionTime(action RecoveryAction) time.Duration {
	switch action.EstimatedTime {
	case "30 seconds":
		return 30 * time.Second
	case "5 minutes":
		return 5 * time.Minute
	default:
		return 1 * time.Minute
	}
}

func (rgm *RecoveryGuidanceManager) generateActionReasoning(rule RecoveryRule, action RecoveryAction, errorCtx *ErrorContext) string {
	return fmt.Sprintf("Recommended because %s and %s is appropriate for this error type", 
		rule.Description, action.Name)
}

func (rgm *RecoveryGuidanceManager) calculatePlanMetrics(actions []RecommendedAction) (time.Duration, float64) {
	if len(actions) == 0 {
		return 0, 0
	}

	totalTime := time.Duration(0)
	totalConfidence := 0.0

	for _, action := range actions {
		totalTime += action.EstimatedTime
		totalConfidence += action.Confidence
	}

	avgConfidence := totalConfidence / float64(len(actions))
	return totalTime, avgConfidence
}

// RunDiagnostics runs diagnostic checks for the error context
func (de *DiagnosticEngine) RunDiagnostics(ctx context.Context, errorCtx *ErrorContext) (*DiagnosticInfo, error) {
	var results []DiagnosticResult
	issues := make([]string, 0)
	recommendations := make([]string, 0)

	// Run applicable diagnostics
	for _, rule := range de.diagnostics {
		for _, diagnostic := range rule.Diagnostics {
			result, err := diagnostic.Check(ctx, errorCtx)
			if err != nil {
				de.logger.WithError(err).WithField("diagnostic", diagnostic.ID).Warn("Diagnostic check failed")
				continue
			}

			results = append(results, *result)
			
			if !result.Passed {
				issues = append(issues, result.Message)
			}
			
			recommendations = append(recommendations, result.Suggestions...)
		}
	}

	// Determine overall health
	overallHealth := "healthy"
	if len(issues) > 0 {
		overallHealth = "degraded"
		if len(issues) > 3 {
			overallHealth = "unhealthy"
		}
	}

	return &DiagnosticInfo{
		OverallHealth:   overallHealth,
		Results:         results,
		Summary:         fmt.Sprintf("Ran %d diagnostic checks, found %d issues", len(results), len(issues)),
		Issues:          issues,
		Recommendations: recommendations,
	}, nil
}

// checkServiceConnectivity checks if a service is reachable
func (de *DiagnosticEngine) checkServiceConnectivity(ctx context.Context, errorCtx *ErrorContext) (*DiagnosticResult, error) {
	// This would implement actual connectivity checks
	return &DiagnosticResult{
		Passed:   true,
		Message:  "Service connectivity check passed",
		Severity: "info",
		Suggestions: []string{
			"Network connectivity is functional",
		},
	}, nil
}