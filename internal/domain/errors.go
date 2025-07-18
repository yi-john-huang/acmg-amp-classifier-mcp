package domain

import (
	"fmt"
	"time"
)

// MCPError represents a standardized error response
type MCPError struct {
	Code      string    `json:"code"`
	Message   string    `json:"message"`
	Details   string    `json:"details,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	RequestID string    `json:"request_id"`
}

// Error implements the error interface
func (e *MCPError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Error codes for different failure scenarios
const (
	ErrInvalidInput   = "INVALID_INPUT"
	ErrDatabaseError  = "DATABASE_ERROR"
	ErrExternalAPI    = "EXTERNAL_API_ERROR"
	ErrClassification = "CLASSIFICATION_ERROR"
	ErrRateLimit      = "RATE_LIMIT_EXCEEDED"
	ErrAuthentication = "AUTHENTICATION_ERROR"
	ErrInternalServer = "INTERNAL_SERVER_ERROR"
	ErrValidation     = "VALIDATION_ERROR"
	ErrHGVSParsing    = "HGVS_PARSING_ERROR"
)

// ValidationError represents input validation errors
type ValidationError struct {
	Field   string      `json:"field"`
	Message string      `json:"message"`
	Value   interface{} `json:"value"`
}

// Error implements the error interface
func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error for field '%s': %s", e.Field, e.Message)
}

// NewMCPError creates a new MCPError with timestamp
func NewMCPError(code, message, details, requestID string) *MCPError {
	return &MCPError{
		Code:      code,
		Message:   message,
		Details:   details,
		Timestamp: time.Now().UTC(),
		RequestID: requestID,
	}
}

// NewValidationError creates a new ValidationError
func NewValidationError(field, message string, value interface{}) *ValidationError {
	return &ValidationError{
		Field:   field,
		Message: message,
		Value:   value,
	}
}
