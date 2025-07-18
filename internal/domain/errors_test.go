package domain

import (
	"testing"
	"time"
)

func TestMCPError(t *testing.T) {
	tests := []struct {
		name      string
		code      string
		message   string
		details   string
		requestID string
	}{
		{
			name:      "Basic error",
			code:      ErrInvalidInput,
			message:   "Invalid HGVS notation",
			details:   "The provided HGVS notation does not match expected format",
			requestID: "req-123",
		},
		{
			name:      "Database error",
			code:      ErrDatabaseError,
			message:   "Database connection failed",
			details:   "Unable to connect to PostgreSQL",
			requestID: "req-456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewMCPError(tt.code, tt.message, tt.details, tt.requestID)

			if err.Code != tt.code {
				t.Errorf("Expected code %s, got %s", tt.code, err.Code)
			}

			if err.Message != tt.message {
				t.Errorf("Expected message %s, got %s", tt.message, err.Message)
			}

			if err.Details != tt.details {
				t.Errorf("Expected details %s, got %s", tt.details, err.Details)
			}

			if err.RequestID != tt.requestID {
				t.Errorf("Expected requestID %s, got %s", tt.requestID, err.RequestID)
			}

			// Check that timestamp is recent (within last minute)
			if time.Since(err.Timestamp) > time.Minute {
				t.Errorf("Timestamp should be recent, got %v", err.Timestamp)
			}

			// Test Error() method
			expectedError := tt.code + ": " + tt.message
			if err.Error() != expectedError {
				t.Errorf("Expected error string %s, got %s", expectedError, err.Error())
			}
		})
	}
}

func TestValidationError(t *testing.T) {
	tests := []struct {
		name    string
		field   string
		message string
		value   interface{}
	}{
		{
			name:    "String validation error",
			field:   "hgvs",
			message: "Invalid format",
			value:   "invalid-hgvs",
		},
		{
			name:    "Integer validation error",
			field:   "position",
			message: "Must be positive",
			value:   -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewValidationError(tt.field, tt.message, tt.value)

			if err.Field != tt.field {
				t.Errorf("Expected field %s, got %s", tt.field, err.Field)
			}

			if err.Message != tt.message {
				t.Errorf("Expected message %s, got %s", tt.message, err.Message)
			}

			if err.Value != tt.value {
				t.Errorf("Expected value %v, got %v", tt.value, err.Value)
			}

			// Test Error() method
			expectedError := "validation error for field '" + tt.field + "': " + tt.message
			if err.Error() != expectedError {
				t.Errorf("Expected error string %s, got %s", expectedError, err.Error())
			}
		})
	}
}

func TestErrorConstants(t *testing.T) {
	constants := map[string]string{
		"ErrInvalidInput":   ErrInvalidInput,
		"ErrDatabaseError":  ErrDatabaseError,
		"ErrExternalAPI":    ErrExternalAPI,
		"ErrClassification": ErrClassification,
		"ErrRateLimit":      ErrRateLimit,
		"ErrAuthentication": ErrAuthentication,
		"ErrInternalServer": ErrInternalServer,
		"ErrValidation":     ErrValidation,
		"ErrHGVSParsing":    ErrHGVSParsing,
	}

	expectedValues := map[string]string{
		"ErrInvalidInput":   "INVALID_INPUT",
		"ErrDatabaseError":  "DATABASE_ERROR",
		"ErrExternalAPI":    "EXTERNAL_API_ERROR",
		"ErrClassification": "CLASSIFICATION_ERROR",
		"ErrRateLimit":      "RATE_LIMIT_EXCEEDED",
		"ErrAuthentication": "AUTHENTICATION_ERROR",
		"ErrInternalServer": "INTERNAL_SERVER_ERROR",
		"ErrValidation":     "VALIDATION_ERROR",
		"ErrHGVSParsing":    "HGVS_PARSING_ERROR",
	}

	for name, actual := range constants {
		expected := expectedValues[name]
		if actual != expected {
			t.Errorf("Expected %s to be %s, got %s", name, expected, actual)
		}
	}
}
