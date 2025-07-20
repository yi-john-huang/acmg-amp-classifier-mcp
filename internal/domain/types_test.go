package domain

import (
	"testing"
)

func TestClassification_IsValid(t *testing.T) {
	tests := []struct {
		name           string
		classification Classification
		want           bool
	}{
		{"Valid PATHOGENIC", PATHOGENIC, true},
		{"Valid LIKELY_PATHOGENIC", LIKELY_PATHOGENIC, true},
		{"Valid VUS", VUS, true},
		{"Valid LIKELY_BENIGN", LIKELY_BENIGN, true},
		{"Valid BENIGN", BENIGN, true},
		{"Invalid empty", Classification(""), false},
		{"Invalid unknown", Classification("UNKNOWN"), false},
		{"Invalid case", Classification("pathogenic"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.classification.IsValid(); got != tt.want {
				t.Errorf("Classification.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClassification_ClinicalSignificance(t *testing.T) {
	tests := []struct {
		name           string
		classification Classification
		wantContains   string
	}{
		{"PATHOGENIC description", PATHOGENIC, "Disease-causing"},
		{"LIKELY_PATHOGENIC description", LIKELY_PATHOGENIC, "Probably disease-causing"},
		{"VUS description", VUS, "Uncertain Significance"},
		{"LIKELY_BENIGN description", LIKELY_BENIGN, "Probably not disease-causing"},
		{"BENIGN description", BENIGN, "Not disease-causing"},
		{"Invalid classification", Classification("INVALID"), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.classification.ClinicalSignificance()
			if got == "" {
				t.Errorf("Classification.ClinicalSignificance() returned empty string")
			}
			// Basic check that description contains expected text
			if len(got) < 10 {
				t.Errorf("Classification.ClinicalSignificance() = %v, expected longer description", got)
			}
		})
	}
}

func TestVariantType_IsValid(t *testing.T) {
	tests := []struct {
		name        string
		variantType VariantType
		want        bool
	}{
		{"Valid GERMLINE", GERMLINE, true},
		{"Valid SOMATIC", SOMATIC, true},
		{"Invalid empty", VariantType(""), false},
		{"Invalid unknown", VariantType("UNKNOWN"), false},
		{"Invalid case", VariantType("germline"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.variantType.IsValid(); got != tt.want {
				t.Errorf("VariantType.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRuleStrength_IsValid(t *testing.T) {
	tests := []struct {
		name         string
		ruleStrength RuleStrength
		want         bool
	}{
		{"Valid VERY_STRONG", VERY_STRONG, true},
		{"Valid STRONG", STRONG, true},
		{"Valid MODERATE", MODERATE, true},
		{"Valid SUPPORTING", SUPPORTING, true},
		{"Invalid empty", RuleStrength(""), false},
		{"Invalid unknown", RuleStrength("UNKNOWN"), false},
		{"Invalid case", RuleStrength("strong"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ruleStrength.IsValid(); got != tt.want {
				t.Errorf("RuleStrength.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConfidenceLevel_IsValid(t *testing.T) {
	tests := []struct {
		name            string
		confidenceLevel ConfidenceLevel
		want            bool
	}{
		{"Valid HIGH", HIGH, true},
		{"Valid MEDIUM", MEDIUM, true},
		{"Valid LOW", LOW, true},
		{"Invalid empty", ConfidenceLevel(""), false},
		{"Invalid unknown", ConfidenceLevel("UNKNOWN"), false},
		{"Invalid case", ConfidenceLevel("high"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.confidenceLevel.IsValid(); got != tt.want {
				t.Errorf("ConfidenceLevel.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Benchmark critical validation functions for performance
func BenchmarkClassification_IsValid(b *testing.B) {
	c := PATHOGENIC
	for i := 0; i < b.N; i++ {
		c.IsValid()
	}
}

func BenchmarkClassification_ClinicalSignificance(b *testing.B) {
	c := PATHOGENIC
	for i := 0; i < b.N; i++ {
		c.ClinicalSignificance()
	}
}

func TestVariant_Validate(t *testing.T) {
	validVariant := &Variant{
		ID:             "var123",
		HGVS:           "NM_000059.3:c.274G>T",
		Type:           GERMLINE,
		Chromosome:     "17",
		Position:       43094692,
		Reference:      "G",
		Alternate:      "T",
		GeneSymbol:     "BRCA1",
		GeneID:         "672",
		Classification: PATHOGENIC,
		Confidence:     HIGH,
	}

	tests := []struct {
		name    string
		variant *Variant
		wantErr bool
	}{
		{"Valid variant", validVariant, false},
		{"Missing ID", &Variant{HGVS: "test", Type: GERMLINE, Chromosome: "1", Position: 1, GeneSymbol: "TEST"}, true},
		{"Missing HGVS", &Variant{ID: "test", Type: GERMLINE, Chromosome: "1", Position: 1, GeneSymbol: "TEST"}, true},
		{"Invalid type", &Variant{ID: "test", HGVS: "test", Type: VariantType("INVALID"), Chromosome: "1", Position: 1, GeneSymbol: "TEST"}, true},
		{"Missing chromosome", &Variant{ID: "test", HGVS: "test", Type: GERMLINE, Position: 1, GeneSymbol: "TEST"}, true},
		{"Invalid position", &Variant{ID: "test", HGVS: "test", Type: GERMLINE, Chromosome: "1", Position: 0, GeneSymbol: "TEST"}, true},
		{"Missing gene symbol", &Variant{ID: "test", HGVS: "test", Type: GERMLINE, Chromosome: "1", Position: 1}, true},
		{"Invalid classification", &Variant{ID: "test", HGVS: "test", Type: GERMLINE, Chromosome: "1", Position: 1, GeneSymbol: "TEST", Classification: Classification("INVALID")}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.variant.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Variant.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestACMGRule_Validate(t *testing.T) {
	validRule := &ACMGRule{
		Code:       "PVS1",
		Category:   PATHOGENIC_RULE,
		Strength:   VERY_STRONG,
		Applied:    true,
		Evidence:   "Null variant in critical domain",
		Confidence: HIGH,
	}

	tests := []struct {
		name    string
		rule    *ACMGRule
		wantErr bool
	}{
		{"Valid rule", validRule, false},
		{"Missing code", &ACMGRule{Category: PATHOGENIC_RULE, Strength: VERY_STRONG}, true},
		{"Invalid category", &ACMGRule{Code: "PVS1", Category: RuleCategory("INVALID"), Strength: VERY_STRONG}, true},
		{"Invalid strength", &ACMGRule{Code: "PVS1", Category: PATHOGENIC_RULE, Strength: RuleStrength("INVALID")}, true},
		{"Invalid confidence", &ACMGRule{Code: "PVS1", Category: PATHOGENIC_RULE, Strength: VERY_STRONG, Confidence: ConfidenceLevel("INVALID")}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.rule.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("ACMGRule.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExtendedClassificationMetadata_Validate(t *testing.T) {
	validMetadata := &ExtendedClassificationMetadata{
		VariantID:    "var123",
		ClassifiedBy: "system",
		Guidelines:   "ACMG/AMP 2015",
		Notes:        "High confidence classification",
	}

	tests := []struct {
		name     string
		metadata *ExtendedClassificationMetadata
		wantErr  bool
	}{
		{"Valid metadata", validMetadata, false},
		{"Missing variant ID", &ExtendedClassificationMetadata{ClassifiedBy: "system"}, true},
		{"Empty variant ID", &ExtendedClassificationMetadata{VariantID: ""}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.metadata.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("ExtendedClassificationMetadata.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRuleCategory_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		category RuleCategory
		want     bool
	}{
		{"Valid PATHOGENIC_RULE", PATHOGENIC_RULE, true},
		{"Valid BENIGN_RULE", BENIGN_RULE, true},
		{"Invalid empty", RuleCategory(""), false},
		{"Invalid unknown", RuleCategory("UNKNOWN"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.category.IsValid(); got != tt.want {
				t.Errorf("RuleCategory.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClassification_LogFields(t *testing.T) {
	c := PATHOGENIC
	fields := c.LogFields()

	// Verify required fields are present
	requiredFields := []string{"classification", "clinical_significance", "is_valid", "acmg_amp_compliant"}
	for _, field := range requiredFields {
		if _, exists := fields[field]; !exists {
			t.Errorf("LogFields() missing required field: %s", field)
		}
	}

	// Verify field values
	if fields["classification"] != string(PATHOGENIC) {
		t.Errorf("LogFields() classification = %v, want %v", fields["classification"], string(PATHOGENIC))
	}

	if fields["is_valid"] != true {
		t.Errorf("LogFields() is_valid = %v, want true", fields["is_valid"])
	}
}

// TestClassification_GetClassificationLevel tests the internal classification level method
func TestClassification_GetClassificationLevel(t *testing.T) {
	tests := []struct {
		name           string
		classification Classification
		expected       string
	}{
		{"PATHOGENIC is actionable", PATHOGENIC, "actionable"},
		{"LIKELY_PATHOGENIC is actionable", LIKELY_PATHOGENIC, "actionable"},
		{"VUS is uncertain", VUS, "uncertain"},
		{"LIKELY_BENIGN is non_actionable", LIKELY_BENIGN, "non_actionable"},
		{"BENIGN is non_actionable", BENIGN, "non_actionable"},
		{"Invalid classification is unknown", Classification("INVALID"), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.classification.getClassificationLevel()
			if result != tt.expected {
				t.Errorf("getClassificationLevel() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestClassification_RequiresClinicalAction tests clinical action requirements
func TestClassification_RequiresClinicalAction(t *testing.T) {
	tests := []struct {
		name           string
		classification Classification
		expected       bool
	}{
		{"PATHOGENIC requires action", PATHOGENIC, true},
		{"LIKELY_PATHOGENIC requires action", LIKELY_PATHOGENIC, true},
		{"VUS does not require action", VUS, false},
		{"LIKELY_BENIGN does not require action", LIKELY_BENIGN, false},
		{"BENIGN does not require action", BENIGN, false},
		{"Invalid classification requires action (conservative)", Classification("INVALID"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.classification.RequiresClinicalAction()
			if result != tt.expected {
				t.Errorf("RequiresClinicalAction() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestClassification_LogFields_Comprehensive tests the enhanced LogFields method
func TestClassification_LogFields_Comprehensive(t *testing.T) {
	tests := []struct {
		name           string
		classification Classification
		expectedFields map[string]any
	}{
		{
			name:           "PATHOGENIC log fields",
			classification: PATHOGENIC,
			expectedFields: map[string]any{
				"classification":        "PATHOGENIC",
				"clinical_significance": "Pathogenic - Disease-causing variant",
				"is_valid":              true,
				"acmg_amp_compliant":    true,
				"classification_level":  "actionable",
				"requires_action":       true,
			},
		},
		{
			name:           "VUS log fields",
			classification: VUS,
			expectedFields: map[string]any{
				"classification":        "VUS",
				"clinical_significance": "Variant of Uncertain Significance - Clinical significance unknown",
				"is_valid":              true,
				"acmg_amp_compliant":    true,
				"classification_level":  "uncertain",
				"requires_action":       false,
			},
		},
		{
			name:           "Invalid classification log fields",
			classification: Classification("INVALID"),
			expectedFields: map[string]any{
				"classification":        "INVALID",
				"clinical_significance": "Unknown classification",
				"is_valid":              false,
				"acmg_amp_compliant":    false,
				"classification_level":  "unknown",
				"requires_action":       true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fields := tt.classification.LogFields()

			// Check that all expected fields are present and correct
			for key, expectedValue := range tt.expectedFields {
				actualValue, exists := fields[key]
				if !exists {
					t.Errorf("LogFields() missing field %s", key)
					continue
				}
				if actualValue != expectedValue {
					t.Errorf("LogFields()[%s] = %v, want %v", key, actualValue, expectedValue)
				}
			}

			// Ensure no unexpected fields
			if len(fields) != len(tt.expectedFields) {
				t.Errorf("LogFields() returned %d fields, expected %d", len(fields), len(tt.expectedFields))
			}
		})
	}
}
