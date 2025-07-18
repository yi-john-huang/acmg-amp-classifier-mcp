package domain

import (
	"testing"
)

func TestClassificationConstants(t *testing.T) {
	tests := []struct {
		name     string
		value    Classification
		expected string
	}{
		{"Pathogenic", PATHOGENIC, "PATHOGENIC"},
		{"Likely Pathogenic", LIKELY_PATHOGENIC, "LIKELY_PATHOGENIC"},
		{"VUS", VUS, "VUS"},
		{"Likely Benign", LIKELY_BENIGN, "LIKELY_BENIGN"},
		{"Benign", BENIGN, "BENIGN"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.value) != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, string(tt.value))
			}
		})
	}
}

func TestVariantTypeConstants(t *testing.T) {
	tests := []struct {
		name     string
		value    VariantType
		expected string
	}{
		{"Germline", GERMLINE, "GERMLINE"},
		{"Somatic", SOMATIC, "SOMATIC"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.value) != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, string(tt.value))
			}
		})
	}
}

func TestRuleStrengthConstants(t *testing.T) {
	tests := []struct {
		name     string
		value    RuleStrength
		expected string
	}{
		{"Very Strong", VERY_STRONG, "VERY_STRONG"},
		{"Strong", STRONG, "STRONG"},
		{"Moderate", MODERATE, "MODERATE"},
		{"Supporting", SUPPORTING, "SUPPORTING"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.value) != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, string(tt.value))
			}
		})
	}
}

func TestConfidenceLevelConstants(t *testing.T) {
	tests := []struct {
		name     string
		value    ConfidenceLevel
		expected string
	}{
		{"High", HIGH, "High"},
		{"Medium", MEDIUM, "Medium"},
		{"Low", LOW, "Low"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.value) != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, string(tt.value))
			}
		})
	}
}
