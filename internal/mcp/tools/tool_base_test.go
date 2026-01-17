package tools

import (
	"testing"
)

// TestParseParams_Success tests successful parameter parsing
func TestParseParams_Success(t *testing.T) {
	type TestParams struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	input := map[string]interface{}{
		"name":  "test",
		"value": 42,
	}

	var target TestParams
	err := ParseParams(input, &target)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if target.Name != "test" {
		t.Errorf("Expected name 'test', got: %s", target.Name)
	}
	if target.Value != 42 {
		t.Errorf("Expected value 42, got: %d", target.Value)
	}
}

// TestParseParams_NilParams tests handling of nil parameters
func TestParseParams_NilParams(t *testing.T) {
	type TestParams struct {
		Name string `json:"name"`
	}

	var target TestParams
	err := ParseParams(nil, &target)

	if err == nil {
		t.Error("Expected error for nil params, got nil")
	}
	if err.Error() != "missing required parameters" {
		t.Errorf("Expected 'missing required parameters' error, got: %s", err.Error())
	}
}

// TestParseParams_InvalidJSON tests handling of invalid JSON
func TestParseParams_InvalidJSON(t *testing.T) {
	type TestParams struct {
		Value int `json:"value"`
	}

	// String where int expected - should fail during unmarshal
	input := map[string]interface{}{
		"value": "not a number",
	}

	var target TestParams
	err := ParseParams(input, &target)

	if err == nil {
		t.Error("Expected error for invalid JSON type, got nil")
	}
}

// TestParseParams_NestedStruct tests parsing nested structures
func TestParseParams_NestedStruct(t *testing.T) {
	type Inner struct {
		ID string `json:"id"`
	}
	type Outer struct {
		Inner Inner `json:"inner"`
		Name  string `json:"name"`
	}

	input := map[string]interface{}{
		"name": "outer",
		"inner": map[string]interface{}{
			"id": "inner-id",
		},
	}

	var target Outer
	err := ParseParams(input, &target)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if target.Name != "outer" {
		t.Errorf("Expected name 'outer', got: %s", target.Name)
	}
	if target.Inner.ID != "inner-id" {
		t.Errorf("Expected inner.id 'inner-id', got: %s", target.Inner.ID)
	}
}

// TestParseParams_ArrayField tests parsing arrays
func TestParseParams_ArrayField(t *testing.T) {
	type TestParams struct {
		Items []string `json:"items"`
	}

	input := map[string]interface{}{
		"items": []interface{}{"a", "b", "c"},
	}

	var target TestParams
	err := ParseParams(input, &target)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if len(target.Items) != 3 {
		t.Errorf("Expected 3 items, got: %d", len(target.Items))
	}
	if target.Items[0] != "a" {
		t.Errorf("Expected first item 'a', got: %s", target.Items[0])
	}
}

// TestParseParams_OptionalFields tests handling of optional fields
func TestParseParams_OptionalFields(t *testing.T) {
	type TestParams struct {
		Required string `json:"required"`
		Optional string `json:"optional,omitempty"`
	}

	input := map[string]interface{}{
		"required": "value",
	}

	var target TestParams
	err := ParseParams(input, &target)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if target.Required != "value" {
		t.Errorf("Expected required 'value', got: %s", target.Required)
	}
	if target.Optional != "" {
		t.Errorf("Expected optional to be empty, got: %s", target.Optional)
	}
}

// TestParseParams_BooleanFields tests handling of boolean fields
func TestParseParams_BooleanFields(t *testing.T) {
	type TestParams struct {
		Enabled bool `json:"enabled"`
	}

	testCases := []struct {
		name     string
		input    map[string]interface{}
		expected bool
	}{
		{
			name:     "true value",
			input:    map[string]interface{}{"enabled": true},
			expected: true,
		},
		{
			name:     "false value",
			input:    map[string]interface{}{"enabled": false},
			expected: false,
		},
		{
			name:     "missing value defaults to false",
			input:    map[string]interface{}{},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var target TestParams
			err := ParseParams(tc.input, &target)

			if err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
			if target.Enabled != tc.expected {
				t.Errorf("Expected enabled %v, got: %v", tc.expected, target.Enabled)
			}
		})
	}
}

// TestParseParams_FloatFields tests handling of float fields
func TestParseParams_FloatFields(t *testing.T) {
	type TestParams struct {
		Score float64 `json:"score"`
	}

	input := map[string]interface{}{
		"score": 0.85,
	}

	var target TestParams
	err := ParseParams(input, &target)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if target.Score != 0.85 {
		t.Errorf("Expected score 0.85, got: %f", target.Score)
	}
}
