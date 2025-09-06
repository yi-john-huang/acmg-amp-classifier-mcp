package resources

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
)

// InterpretationResourceProvider provides interpretation/{id} resources
type InterpretationResourceProvider struct {
	logger    *logrus.Logger
	uriParser *URIParser
}

// InterpretationData represents a complete variant interpretation
type InterpretationData struct {
	InterpretationID   string                      `json:"interpretation_id"`
	VariantID          string                      `json:"variant_id"`
	HGVSNotation       string                      `json:"hgvs_notation"`
	GeneSymbol         string                      `json:"gene_symbol,omitempty"`
	Classification     ClassificationData          `json:"classification"`
	EvidenceAssessment EvidenceAssessmentData      `json:"evidence_assessment"`
	ACMGRuleApplication ACMGRuleApplicationData    `json:"acmg_rule_application"`
	QualityMetrics     InterpretationQualityData   `json:"quality_metrics"`
	ClinicalContext    ClinicalContextData         `json:"clinical_context,omitempty"`
	ReviewHistory      []ReviewHistoryEntry        `json:"review_history"`
	Recommendations    []ClinicalRecommendation    `json:"recommendations"`
	CreatedBy          string                      `json:"created_by"`
	CreatedAt          time.Time                   `json:"created_at"`
	LastModified       time.Time                   `json:"last_modified"`
	Status             string                      `json:"status"` // "draft", "final", "reviewed", "archived"
	Version            int                         `json:"version"`
}

// ClassificationData contains the variant classification details
type ClassificationData struct {
	PrimaryClassification   string                 `json:"primary_classification"`
	ConfidenceLevel        string                 `json:"confidence_level"`
	ConfidenceScore        float64                `json:"confidence_score"`
	ClassificationRationale string                `json:"classification_rationale"`
	AlternativeClassifications []AlternativeClass  `json:"alternative_classifications,omitempty"`
	UncertaintyFactors     []string               `json:"uncertainty_factors,omitempty"`
	ClassificationDate     time.Time              `json:"classification_date"`
	Classifier             string                 `json:"classifier"`
	GuidelinesUsed         []string               `json:"guidelines_used"`
}

// AlternativeClass represents alternative classifications considered
type AlternativeClass struct {
	Classification string  `json:"classification"`
	Probability    float64 `json:"probability"`
	Rationale      string  `json:"rationale"`
}

// EvidenceAssessmentData contains detailed evidence evaluation
type EvidenceAssessmentData struct {
	OverallQuality     string                      `json:"overall_quality"`
	EvidenceSources    []EvidenceSource            `json:"evidence_sources"`
	DataCompleteness   float64                     `json:"data_completeness"`
	ConflictingEvidence []ConflictingEvidenceItem  `json:"conflicting_evidence,omitempty"`
	EvidenceGaps       []string                    `json:"evidence_gaps,omitempty"`
	ReliabilityScores  map[string]float64          `json:"reliability_scores"`
	LastEvidenceUpdate time.Time                   `json:"last_evidence_update"`
}

// EvidenceSource represents an evidence source used in interpretation
type EvidenceSource struct {
	Database      string                 `json:"database"`
	DataType      string                 `json:"data_type"`
	Quality       string                 `json:"quality"`
	Reliability   float64                `json:"reliability"`
	LastAccessed  time.Time              `json:"last_accessed"`
	RecordCount   int                    `json:"record_count,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// ConflictingEvidenceItem represents conflicting evidence that needs resolution
type ConflictingEvidenceItem struct {
	EvidenceType   string    `json:"evidence_type"`
	Source1        string    `json:"source1"`
	Source1Value   string    `json:"source1_value"`
	Source2        string    `json:"source2"`
	Source2Value   string    `json:"source2_value"`
	Resolution     string    `json:"resolution,omitempty"`
	ResolutionDate time.Time `json:"resolution_date,omitempty"`
	Impact         string    `json:"impact"` // "high", "medium", "low"
}

// ACMGRuleApplicationData contains detailed ACMG/AMP rule application
type ACMGRuleApplicationData struct {
	AppliedRules        []AppliedACMGRule      `json:"applied_rules"`
	RejectedRules       []RejectedACMGRule     `json:"rejected_rules"`
	UncertainRules      []UncertainACMGRule    `json:"uncertain_rules"`
	CombinationLogic    CombinationLogicData   `json:"combination_logic"`
	ManualOverrides     []ManualOverride       `json:"manual_overrides,omitempty"`
	RuleApplicationDate time.Time              `json:"rule_application_date"`
	ApplicationMethod   string                 `json:"application_method"` // "automated", "manual", "hybrid"
}

// AppliedACMGRule represents an ACMG/AMP rule that was applied
type AppliedACMGRule struct {
	RuleCode      string                 `json:"rule_code"`
	RuleName      string                 `json:"rule_name"`
	Category      string                 `json:"category"` // "pathogenic", "benign"
	Strength      string                 `json:"strength"` // "very_strong", "strong", "moderate", "supporting"
	Evidence      string                 `json:"evidence"`
	Rationale     string                 `json:"rationale"`
	Confidence    float64                `json:"confidence"`
	AutoApplied   bool                   `json:"auto_applied"`
	ReviewStatus  string                 `json:"review_status"` // "accepted", "modified", "pending"
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// RejectedACMGRule represents a rule that was considered but rejected
type RejectedACMGRule struct {
	RuleCode      string  `json:"rule_code"`
	RuleName      string  `json:"rule_name"`
	ReasonRejected string `json:"reason_rejected"`
	Evidence      string  `json:"evidence,omitempty"`
	ReviewedBy    string  `json:"reviewed_by,omitempty"`
}

// UncertainACMGRule represents a rule with uncertain application
type UncertainACMGRule struct {
	RuleCode       string   `json:"rule_code"`
	RuleName       string   `json:"rule_name"`
	UncertaintyReason string `json:"uncertainty_reason"`
	PossibleStrengths []string `json:"possible_strengths"`
	RequiredEvidence  []string `json:"required_evidence"`
	RecommendedAction string   `json:"recommended_action"`
}

// CombinationLogicData explains how rules were combined
type CombinationLogicData struct {
	PathogenicRules    []string               `json:"pathogenic_rules"`
	BenignRules        []string               `json:"benign_rules"`
	CombinationMethod  string                 `json:"combination_method"`
	FinalClassification string                `json:"final_classification"`
	AlternativePaths   []AlternativePath      `json:"alternative_paths,omitempty"`
	DecisionTree       map[string]interface{} `json:"decision_tree,omitempty"`
}

// AlternativePath represents alternative rule combinations
type AlternativePath struct {
	Rules          []string `json:"rules"`
	Classification string   `json:"classification"`
	Probability    float64  `json:"probability"`
	Notes          string   `json:"notes,omitempty"`
}

// ManualOverride represents manual modifications to automated rule application
type ManualOverride struct {
	RuleCode       string    `json:"rule_code"`
	OriginalApplied bool     `json:"original_applied"`
	OverrideApplied bool     `json:"override_applied"`
	OriginalStrength string  `json:"original_strength,omitempty"`
	OverrideStrength string  `json:"override_strength,omitempty"`
	Justification   string   `json:"justification"`
	OverriddenBy    string   `json:"overridden_by"`
	OverrideDate    time.Time `json:"override_date"`
}

// InterpretationQualityData contains quality metrics for the interpretation
type InterpretationQualityData struct {
	OverallQualityScore  float64                `json:"overall_quality_score"`
	CompletenessScore    float64                `json:"completeness_score"`
	ConsistencyScore     float64                `json:"consistency_score"`
	ReproducibilityScore float64                `json:"reproducibility_score"`
	QualityFlags         []QualityFlag          `json:"quality_flags"`
	ValidationChecks     map[string]bool        `json:"validation_checks"`
	PeerReviewStatus     string                 `json:"peer_review_status,omitempty"`
	QualityAssessmentDate time.Time             `json:"quality_assessment_date"`
}

// QualityFlag represents quality issues or notes
type QualityFlag struct {
	Type        string    `json:"type"`        // "warning", "info", "critical"
	Code        string    `json:"code"`
	Message     string    `json:"message"`
	Severity    string    `json:"severity"`
	Resolution  string    `json:"resolution,omitempty"`
	FlaggedDate time.Time `json:"flagged_date"`
}

// ClinicalContextData contains clinical context for interpretation
type ClinicalContextData struct {
	PatientID          string            `json:"patient_id,omitempty"`
	ClinicalIndication string            `json:"clinical_indication,omitempty"`
	Phenotype          []string          `json:"phenotype,omitempty"`
	FamilyHistory      FamilyHistoryData `json:"family_history"`
	Ethnicity          string            `json:"ethnicity,omitempty"`
	ConsanguinityKnown bool              `json:"consanguinity_known"`
	ClinicalRelevance  string            `json:"clinical_relevance"`
	TestingLaboratory  string            `json:"testing_laboratory,omitempty"`
	ReferringPhysician string            `json:"referring_physician,omitempty"`
}

// FamilyHistoryData contains family history information
type FamilyHistoryData struct {
	AffectedRelatives   []AffectedRelative `json:"affected_relatives,omitempty"`
	FamilyHistoryNotes  string             `json:"family_history_notes,omitempty"`
	Consanguinity       bool               `json:"consanguinity"`
	AdoptionStatus      string             `json:"adoption_status,omitempty"`
	FamilyStudyAvailable bool              `json:"family_study_available"`
}

// AffectedRelative represents an affected family member
type AffectedRelative struct {
	Relationship string `json:"relationship"`
	Condition    string `json:"condition"`
	AgeOfOnset   int    `json:"age_of_onset,omitempty"`
	Severity     string `json:"severity,omitempty"`
	TestedStatus string `json:"tested_status"` // "tested", "not_tested", "unknown"
}

// ReviewHistoryEntry represents a review or modification of the interpretation
type ReviewHistoryEntry struct {
	Action          string                 `json:"action"` // "created", "modified", "reviewed", "approved", "rejected"
	Reviewer        string                 `json:"reviewer"`
	ReviewDate      time.Time              `json:"review_date"`
	Changes         []ChangeDescription    `json:"changes,omitempty"`
	Comments        string                 `json:"comments,omitempty"`
	PreviousVersion int                    `json:"previous_version,omitempty"`
	ReviewType      string                 `json:"review_type"` // "peer_review", "quality_check", "routine_update"
}

// ChangeDescription describes what was changed in a review
type ChangeDescription struct {
	Field       string      `json:"field"`
	OldValue    interface{} `json:"old_value"`
	NewValue    interface{} `json:"new_value"`
	ChangeType  string      `json:"change_type"` // "added", "modified", "removed"
	Justification string    `json:"justification,omitempty"`
}

// ClinicalRecommendation contains clinical recommendations based on interpretation
type ClinicalRecommendation struct {
	RecommendationType string    `json:"recommendation_type"`
	Priority           string    `json:"priority"` // "high", "medium", "low"
	Recommendation     string    `json:"recommendation"`
	Rationale          string    `json:"rationale"`
	ActionRequired     bool      `json:"action_required"`
	Timeline           string    `json:"timeline,omitempty"`
	ResponsibleParty   string    `json:"responsible_party,omitempty"`
	FollowUpRequired   bool      `json:"follow_up_required"`
	CreatedDate        time.Time `json:"created_date"`
}

// NewInterpretationResourceProvider creates a new interpretation resource provider
func NewInterpretationResourceProvider(logger *logrus.Logger) *InterpretationResourceProvider {
	provider := &InterpretationResourceProvider{
		logger:    logger,
		uriParser: NewURIParser(),
	}
	
	// Add URI patterns for interpretation resources
	provider.uriParser.AddPattern("interpretation_by_id", `^/interpretation/(?P<id>[^/]+)$`)
	provider.uriParser.AddPattern("interpretation_classification", `^/interpretation/(?P<id>[^/]+)/classification$`)
	provider.uriParser.AddPattern("interpretation_evidence", `^/interpretation/(?P<id>[^/]+)/evidence$`)
	provider.uriParser.AddPattern("interpretation_rules", `^/interpretation/(?P<id>[^/]+)/rules$`)
	provider.uriParser.AddPattern("interpretation_quality", `^/interpretation/(?P<id>[^/]+)/quality$`)
	provider.uriParser.AddPattern("interpretation_history", `^/interpretation/(?P<id>[^/]+)/history$`)
	provider.uriParser.AddPattern("interpretation_recommendations", `^/interpretation/(?P<id>[^/]+)/recommendations$`)
	
	return provider
}

// GetResource retrieves an interpretation resource
func (ip *InterpretationResourceProvider) GetResource(ctx context.Context, uri string) (*ResourceContent, error) {
	ip.logger.WithField("uri", uri).Debug("Getting interpretation resource")
	
	patternName, params, err := ip.uriParser.ParseURI(uri)
	if err != nil {
		return nil, fmt.Errorf("failed to parse interpretation URI: %w", err)
	}
	
	switch patternName {
	case "interpretation_by_id":
		return ip.getInterpretationByID(ctx, params["id"])
	case "interpretation_classification":
		return ip.getInterpretationClassification(ctx, params["id"])
	case "interpretation_evidence":
		return ip.getInterpretationEvidence(ctx, params["id"])
	case "interpretation_rules":
		return ip.getInterpretationRules(ctx, params["id"])
	case "interpretation_quality":
		return ip.getInterpretationQuality(ctx, params["id"])
	case "interpretation_history":
		return ip.getInterpretationHistory(ctx, params["id"])
	case "interpretation_recommendations":
		return ip.getInterpretationRecommendations(ctx, params["id"])
	default:
		return nil, fmt.Errorf("unsupported interpretation resource pattern: %s", patternName)
	}
}

// ListResources lists available interpretation resources
func (ip *InterpretationResourceProvider) ListResources(ctx context.Context, cursor string) (*ResourceList, error) {
	resources := []ResourceInfo{
		{
			URI:         "/interpretation/{id}",
			Name:        "Variant Interpretation",
			Description: "Complete variant interpretation with classification and evidence",
			MimeType:    "application/json",
			LastModified: time.Now().Add(-2 * time.Hour),
			Tags:        []string{"interpretation", "comprehensive"},
		},
		{
			URI:         "/interpretation/{id}/classification",
			Name:        "Classification Details",
			Description: "Detailed classification information and rationale",
			MimeType:    "application/json",
			LastModified: time.Now().Add(-1 * time.Hour),
			Tags:        []string{"interpretation", "classification"},
		},
		{
			URI:         "/interpretation/{id}/evidence",
			Name:        "Evidence Assessment",
			Description: "Detailed evidence evaluation and assessment",
			MimeType:    "application/json",
			LastModified: time.Now().Add(-3 * time.Hour),
			Tags:        []string{"interpretation", "evidence"},
		},
		{
			URI:         "/interpretation/{id}/rules",
			Name:        "ACMG Rule Application",
			Description: "ACMG/AMP rule application details",
			MimeType:    "application/json",
			LastModified: time.Now().Add(-1 * time.Hour),
			Tags:        []string{"interpretation", "acmg", "rules"},
		},
		{
			URI:         "/interpretation/{id}/quality",
			Name:        "Quality Metrics",
			Description: "Quality assessment and validation results",
			MimeType:    "application/json",
			LastModified: time.Now().Add(-30 * time.Minute),
			Tags:        []string{"interpretation", "quality"},
		},
		{
			URI:         "/interpretation/{id}/history",
			Name:        "Review History",
			Description: "Interpretation review and modification history",
			MimeType:    "application/json",
			LastModified: time.Now().Add(-4 * time.Hour),
			Tags:        []string{"interpretation", "history", "audit"},
		},
		{
			URI:         "/interpretation/{id}/recommendations",
			Name:        "Clinical Recommendations",
			Description: "Clinical recommendations based on interpretation",
			MimeType:    "application/json",
			LastModified: time.Now().Add(-2 * time.Hour),
			Tags:        []string{"interpretation", "recommendations", "clinical"},
		},
	}
	
	return &ResourceList{
		Resources: resources,
		Total:     len(resources),
	}, nil
}

// GetResourceInfo returns metadata about an interpretation resource
func (ip *InterpretationResourceProvider) GetResourceInfo(ctx context.Context, uri string) (*ResourceInfo, error) {
	patternName, params, err := ip.uriParser.ParseURI(uri)
	if err != nil {
		return nil, fmt.Errorf("failed to parse interpretation URI: %w", err)
	}
	
	var name, description string
	tags := []string{"interpretation"}
	
	switch patternName {
	case "interpretation_by_id":
		name = fmt.Sprintf("Interpretation %s", params["id"])
		description = "Complete variant interpretation with classification and evidence"
		tags = append(tags, "comprehensive")
	case "interpretation_classification":
		name = fmt.Sprintf("Classification for %s", params["id"])
		description = "Detailed classification information and rationale"
		tags = append(tags, "classification")
	case "interpretation_evidence":
		name = fmt.Sprintf("Evidence for %s", params["id"])
		description = "Detailed evidence evaluation and assessment"
		tags = append(tags, "evidence")
	case "interpretation_rules":
		name = fmt.Sprintf("ACMG rules for %s", params["id"])
		description = "ACMG/AMP rule application details"
		tags = append(tags, "acmg", "rules")
	case "interpretation_quality":
		name = fmt.Sprintf("Quality metrics for %s", params["id"])
		description = "Quality assessment and validation results"
		tags = append(tags, "quality")
	case "interpretation_history":
		name = fmt.Sprintf("History for %s", params["id"])
		description = "Interpretation review and modification history"
		tags = append(tags, "history", "audit")
	case "interpretation_recommendations":
		name = fmt.Sprintf("Recommendations for %s", params["id"])
		description = "Clinical recommendations based on interpretation"
		tags = append(tags, "recommendations", "clinical")
	default:
		return nil, fmt.Errorf("unsupported interpretation resource pattern: %s", patternName)
	}
	
	return &ResourceInfo{
		URI:         uri,
		Name:        name,
		Description: description,
		MimeType:    "application/json",
		LastModified: time.Now().Add(-1 * time.Hour),
		Tags:        tags,
		Metadata: map[string]interface{}{
			"provider":          "interpretation",
			"interpretation_id": params["id"],
			"pattern":           patternName,
		},
	}, nil
}

// SupportsURI checks if this provider supports the given URI
func (ip *InterpretationResourceProvider) SupportsURI(uri string) bool {
	_, _, err := ip.uriParser.ParseURI(uri)
	return err == nil
}

// GetProviderInfo returns information about this provider
func (ip *InterpretationResourceProvider) GetProviderInfo() ProviderInfo {
	return ProviderInfo{
		Name:        "Interpretation Resource Provider",
		Description: "Provides comprehensive variant interpretation data including classification, evidence, and quality metrics",
		Version:     "1.0.0",
		URIPatterns: []string{
			"/interpretation/{id}",
			"/interpretation/{id}/classification",
			"/interpretation/{id}/evidence",
			"/interpretation/{id}/rules",
			"/interpretation/{id}/quality",
			"/interpretation/{id}/history",
			"/interpretation/{id}/recommendations",
		},
	}
}

// Implementation methods for different resource types

func (ip *InterpretationResourceProvider) getInterpretationByID(ctx context.Context, id string) (*ResourceContent, error) {
	interpretation := ip.generateMockInterpretationData(id)
	
	return &ResourceContent{
		URI:         fmt.Sprintf("/interpretation/%s", id),
		Name:        fmt.Sprintf("Interpretation %s", id),
		Description: fmt.Sprintf("Complete interpretation for %s", id),
		MimeType:    "application/json",
		Content:     interpretation,
		LastModified: time.Now().Add(-2 * time.Hour),
		ETag:        fmt.Sprintf("interpretation-%s-%d", id, time.Now().Unix()),
		Metadata: map[string]interface{}{
			"provider":          "interpretation",
			"interpretation_id": id,
			"content_type":      "complete_interpretation",
		},
	}, nil
}

func (ip *InterpretationResourceProvider) getInterpretationClassification(ctx context.Context, id string) (*ResourceContent, error) {
	classification := ip.generateMockClassificationData(id)
	
	return &ResourceContent{
		URI:         fmt.Sprintf("/interpretation/%s/classification", id),
		Name:        fmt.Sprintf("Classification for %s", id),
		Description: "Detailed classification information and rationale",
		MimeType:    "application/json",
		Content:     classification,
		LastModified: time.Now().Add(-1 * time.Hour),
		ETag:        fmt.Sprintf("classification-%s-%d", id, time.Now().Unix()),
		Metadata: map[string]interface{}{
			"provider":          "interpretation",
			"interpretation_id": id,
			"content_type":      "classification",
		},
	}, nil
}

func (ip *InterpretationResourceProvider) getInterpretationEvidence(ctx context.Context, id string) (*ResourceContent, error) {
	evidence := ip.generateMockEvidenceAssessment(id)
	
	return &ResourceContent{
		URI:         fmt.Sprintf("/interpretation/%s/evidence", id),
		Name:        fmt.Sprintf("Evidence for %s", id),
		Description: "Detailed evidence evaluation and assessment",
		MimeType:    "application/json",
		Content:     evidence,
		LastModified: time.Now().Add(-3 * time.Hour),
		ETag:        fmt.Sprintf("evidence-%s-%d", id, time.Now().Unix()),
		Metadata: map[string]interface{}{
			"provider":          "interpretation",
			"interpretation_id": id,
			"content_type":      "evidence",
		},
	}, nil
}

func (ip *InterpretationResourceProvider) getInterpretationRules(ctx context.Context, id string) (*ResourceContent, error) {
	rules := ip.generateMockACMGRules(id)
	
	return &ResourceContent{
		URI:         fmt.Sprintf("/interpretation/%s/rules", id),
		Name:        fmt.Sprintf("ACMG rules for %s", id),
		Description: "ACMG/AMP rule application details",
		MimeType:    "application/json",
		Content:     rules,
		LastModified: time.Now().Add(-1 * time.Hour),
		ETag:        fmt.Sprintf("rules-%s-%d", id, time.Now().Unix()),
		Metadata: map[string]interface{}{
			"provider":          "interpretation",
			"interpretation_id": id,
			"content_type":      "rules",
		},
	}, nil
}

func (ip *InterpretationResourceProvider) getInterpretationQuality(ctx context.Context, id string) (*ResourceContent, error) {
	quality := ip.generateMockQualityMetrics(id)
	
	return &ResourceContent{
		URI:         fmt.Sprintf("/interpretation/%s/quality", id),
		Name:        fmt.Sprintf("Quality metrics for %s", id),
		Description: "Quality assessment and validation results",
		MimeType:    "application/json",
		Content:     quality,
		LastModified: time.Now().Add(-30 * time.Minute),
		ETag:        fmt.Sprintf("quality-%s-%d", id, time.Now().Unix()),
		Metadata: map[string]interface{}{
			"provider":          "interpretation",
			"interpretation_id": id,
			"content_type":      "quality",
		},
	}, nil
}

func (ip *InterpretationResourceProvider) getInterpretationHistory(ctx context.Context, id string) (*ResourceContent, error) {
	history := ip.generateMockReviewHistory(id)
	
	return &ResourceContent{
		URI:         fmt.Sprintf("/interpretation/%s/history", id),
		Name:        fmt.Sprintf("History for %s", id),
		Description: "Interpretation review and modification history",
		MimeType:    "application/json",
		Content:     map[string]interface{}{"history": history},
		LastModified: time.Now().Add(-4 * time.Hour),
		ETag:        fmt.Sprintf("history-%s-%d", id, time.Now().Unix()),
		Metadata: map[string]interface{}{
			"provider":          "interpretation",
			"interpretation_id": id,
			"content_type":      "history",
		},
	}, nil
}

func (ip *InterpretationResourceProvider) getInterpretationRecommendations(ctx context.Context, id string) (*ResourceContent, error) {
	recommendations := ip.generateMockRecommendations(id)
	
	return &ResourceContent{
		URI:         fmt.Sprintf("/interpretation/%s/recommendations", id),
		Name:        fmt.Sprintf("Recommendations for %s", id),
		Description: "Clinical recommendations based on interpretation",
		MimeType:    "application/json",
		Content:     map[string]interface{}{"recommendations": recommendations},
		LastModified: time.Now().Add(-2 * time.Hour),
		ETag:        fmt.Sprintf("recommendations-%s-%d", id, time.Now().Unix()),
		Metadata: map[string]interface{}{
			"provider":          "interpretation",
			"interpretation_id": id,
			"content_type":      "recommendations",
		},
	}, nil
}

// Mock data generation methods (in production, these would query real databases)

func (ip *InterpretationResourceProvider) generateMockInterpretationData(id string) *InterpretationData {
	hash := ip.hashString(id)
	now := time.Now()
	
	return &InterpretationData{
		InterpretationID:    id,
		VariantID:           fmt.Sprintf("VAR_%09d", hash%1000000000),
		HGVSNotation:        fmt.Sprintf("NM_%06d.3:c.%dA>G", hash%1000000, hash%3000),
		GeneSymbol:          ip.generateGeneSymbol(hash),
		Classification:      ip.generateMockClassificationData(id),
		EvidenceAssessment:  ip.generateMockEvidenceAssessment(id),
		ACMGRuleApplication: ip.generateMockACMGRules(id),
		QualityMetrics:      ip.generateMockQualityMetrics(id),
		ReviewHistory:       ip.generateMockReviewHistory(id),
		Recommendations:     ip.generateMockRecommendations(id),
		CreatedBy:           "geneticist@example.com",
		CreatedAt:           now.Add(-48 * time.Hour),
		LastModified:        now.Add(-2 * time.Hour),
		Status:              []string{"draft", "final", "reviewed"}[hash%3],
		Version:             1 + (hash % 5),
	}
}

func (ip *InterpretationResourceProvider) generateMockClassificationData(id string) ClassificationData {
	hash := ip.hashString(id)
	classifications := []string{"Pathogenic", "Likely pathogenic", "VUS", "Likely benign", "Benign"}
	
	return ClassificationData{
		PrimaryClassification:   classifications[hash%len(classifications)],
		ConfidenceLevel:        []string{"high", "moderate", "low"}[hash%3],
		ConfidenceScore:        0.3 + (float64(hash%700) / 1000.0),
		ClassificationRationale: "Based on ACMG/AMP guidelines and available evidence",
		GuidelinesUsed:         []string{"ACMG/AMP 2015"},
		ClassificationDate:     time.Now().Add(-24 * time.Hour),
		Classifier:            "automated_system_v1.0",
	}
}

func (ip *InterpretationResourceProvider) generateMockEvidenceAssessment(id string) EvidenceAssessmentData {
	hash := ip.hashString(id)
	
	return EvidenceAssessmentData{
		OverallQuality:     []string{"high", "moderate", "low"}[hash%3],
		DataCompleteness:   0.5 + (float64(hash%500) / 1000.0),
		ReliabilityScores:  map[string]float64{"clinvar": 0.9, "gnomad": 0.95, "cosmic": 0.8},
		LastEvidenceUpdate: time.Now().Add(-12 * time.Hour),
		EvidenceSources: []EvidenceSource{
			{
				Database:     "ClinVar",
				DataType:     "clinical_significance",
				Quality:      "high",
				Reliability:  0.9,
				LastAccessed: time.Now().Add(-1 * time.Hour),
				RecordCount:  5,
			},
			{
				Database:     "gnomAD",
				DataType:     "population_frequency",
				Quality:      "high",
				Reliability:  0.95,
				LastAccessed: time.Now().Add(-2 * time.Hour),
				RecordCount:  1,
			},
		},
	}
}

func (ip *InterpretationResourceProvider) generateMockACMGRules(id string) ACMGRuleApplicationData {
	return ACMGRuleApplicationData{
		AppliedRules: []AppliedACMGRule{
			{
				RuleCode:     "PVS1",
				RuleName:     "Null variant",
				Category:     "pathogenic",
				Strength:     "very_strong",
				Evidence:     "Nonsense variant in gene where LOF is pathogenic mechanism",
				Rationale:    "Variant introduces premature stop codon",
				Confidence:   0.95,
				AutoApplied:  true,
				ReviewStatus: "accepted",
			},
		},
		RejectedRules: []RejectedACMGRule{
			{
				RuleCode:       "PM2",
				RuleName:       "Absent from controls",
				ReasonRejected: "Variant found in population databases",
			},
		},
		CombinationLogic: CombinationLogicData{
			PathogenicRules:     []string{"PVS1"},
			BenignRules:         []string{},
			CombinationMethod:   "ACMG 2015 guidelines",
			FinalClassification: "Pathogenic",
		},
		RuleApplicationDate: time.Now().Add(-24 * time.Hour),
		ApplicationMethod:   "automated",
	}
}

func (ip *InterpretationResourceProvider) generateMockQualityMetrics(id string) InterpretationQualityData {
	hash := ip.hashString(id)
	
	return InterpretationQualityData{
		OverallQualityScore:   0.6 + (float64(hash%400) / 1000.0),
		CompletenessScore:     0.7 + (float64(hash%300) / 1000.0),
		ConsistencyScore:      0.8 + (float64(hash%200) / 1000.0),
		ReproducibilityScore:  0.75 + (float64(hash%250) / 1000.0),
		ValidationChecks:      map[string]bool{"hgvs_valid": true, "gene_symbol_valid": true, "rules_consistent": true},
		PeerReviewStatus:      []string{"pending", "reviewed", "approved"}[hash%3],
		QualityAssessmentDate: time.Now().Add(-1 * time.Hour),
		QualityFlags:          []QualityFlag{},
	}
}

func (ip *InterpretationResourceProvider) generateMockReviewHistory(id string) []ReviewHistoryEntry {
	return []ReviewHistoryEntry{
		{
			Action:      "created",
			Reviewer:    "geneticist@example.com",
			ReviewDate:  time.Now().Add(-48 * time.Hour),
			Comments:    "Initial interpretation created",
			ReviewType:  "routine_update",
		},
		{
			Action:      "reviewed",
			Reviewer:    "senior_geneticist@example.com",
			ReviewDate:  time.Now().Add(-24 * time.Hour),
			Comments:    "Reviewed and approved classification",
			ReviewType:  "peer_review",
		},
	}
}

func (ip *InterpretationResourceProvider) generateMockRecommendations(id string) []ClinicalRecommendation {
	hash := ip.hashString(id)
	
	recommendations := []ClinicalRecommendation{
		{
			RecommendationType: "genetic_counseling",
			Priority:           "high",
			Recommendation:     "Genetic counseling recommended",
			Rationale:          "Pathogenic variant with clinical implications",
			ActionRequired:     true,
			Timeline:           "within 2 weeks",
			FollowUpRequired:   true,
			CreatedDate:        time.Now().Add(-24 * time.Hour),
		},
	}
	
	if hash%3 == 0 {
		recommendations = append(recommendations, ClinicalRecommendation{
			RecommendationType: "family_testing",
			Priority:           "medium",
			Recommendation:     "Consider family member testing",
			Rationale:          "Hereditary condition with autosomal dominant inheritance",
			ActionRequired:     false,
			Timeline:           "within 3 months",
			FollowUpRequired:   false,
			CreatedDate:        time.Now().Add(-24 * time.Hour),
		})
	}
	
	return recommendations
}

// Utility methods

func (ip *InterpretationResourceProvider) hashString(s string) int {
	hash := 0
	for _, c := range s {
		hash = hash*31 + int(c)
	}
	if hash < 0 {
		hash = -hash
	}
	return hash
}

func (ip *InterpretationResourceProvider) generateGeneSymbol(hash int) string {
	genes := []string{"BRCA1", "BRCA2", "TP53", "CFTR", "APOE", "LDLR", "MYH7", "SCN5A", "RYR2", "PKD1"}
	return genes[hash%len(genes)]
}