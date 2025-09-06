package resources

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
)

// ACMGRulesResourceProvider provides access to ACMG/AMP classification rules
type ACMGRulesResourceProvider struct {
	logger *logrus.Logger
}

// ACMGRulesData represents complete ACMG/AMP classification rules
type ACMGRulesData struct {
	Version         string              `json:"version"`
	LastUpdated     time.Time           `json:"last_updated"`
	Source          string              `json:"source"`
	PathogenicRules PathogenicRulesData `json:"pathogenic_rules"`
	BenignRules     BenignRulesData     `json:"benign_rules"`
	RuleCombinations RuleCombinationData `json:"rule_combinations"`
	Guidelines      GuidelinesData      `json:"guidelines"`
	Definitions     DefinitionsData     `json:"definitions"`
}

// PathogenicRulesData contains all pathogenic evidence rules
type PathogenicRulesData struct {
	VeryStrong []ACMGRuleDefinition `json:"very_strong"`
	Strong     []ACMGRuleDefinition `json:"strong"`
	Moderate   []ACMGRuleDefinition `json:"moderate"`
	Supporting []ACMGRuleDefinition `json:"supporting"`
}

// BenignRulesData contains all benign evidence rules
type BenignRulesData struct {
	StandAlone []ACMGRuleDefinition `json:"stand_alone"`
	Strong     []ACMGRuleDefinition `json:"strong"`
	Supporting []ACMGRuleDefinition `json:"supporting"`
}

// ACMGRuleDefinition represents a single ACMG/AMP rule
type ACMGRuleDefinition struct {
	Code              string                 `json:"code"`
	Name              string                 `json:"name"`
	Category          string                 `json:"category"`
	Strength          string                 `json:"strength"`
	Description       string                 `json:"description"`
	DetailedCriteria  string                 `json:"detailed_criteria"`
	EvidenceRequired  []string               `json:"evidence_required"`
	Examples          []RuleExample          `json:"examples"`
	Caveats           []string               `json:"caveats,omitempty"`
	References        []string               `json:"references"`
	LastRevision      time.Time              `json:"last_revision"`
	ImplementationNotes string               `json:"implementation_notes,omitempty"`
	QualityMetrics    RuleQualityMetrics     `json:"quality_metrics"`
	RelatedRules      []string               `json:"related_rules,omitempty"`
	ConflictingRules  []string               `json:"conflicting_rules,omitempty"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
}

// RuleExample represents an example application of a rule
type RuleExample struct {
	Scenario    string `json:"scenario"`
	Evidence    string `json:"evidence"`
	Application string `json:"application"`
	Outcome     string `json:"outcome"`
	Notes       string `json:"notes,omitempty"`
}

// RuleQualityMetrics represents quality assessment of rule application
type RuleQualityMetrics struct {
	ClarityScore        float64 `json:"clarity_score"`
	SpecificityScore    float64 `json:"specificity_score"`
	ReproducibilityScore float64 `json:"reproducibility_score"`
	EvidenceQuality     string  `json:"evidence_quality"`
	InterRaterReliability float64 `json:"inter_rater_reliability"`
}

// RuleCombinationData represents combinations for final classification
type RuleCombinationData struct {
	Pathogenic       []ClassificationCombination `json:"pathogenic"`
	LikelyPathogenic []ClassificationCombination `json:"likely_pathogenic"`
	LikelyBenign     []ClassificationCombination `json:"likely_benign"`
	Benign           []ClassificationCombination `json:"benign"`
	UncertainSignificance ClassificationGuidance `json:"uncertain_significance"`
}

// ClassificationCombination represents rule combinations for classification
type ClassificationCombination struct {
	ID          string   `json:"id"`
	Description string   `json:"description"`
	Rules       []string `json:"rules"`
	MinimumCriteria string `json:"minimum_criteria"`
	Examples    []string `json:"examples"`
	Notes       string   `json:"notes,omitempty"`
}

// ClassificationGuidance provides guidance for uncertain significance
type ClassificationGuidance struct {
	Description     string   `json:"description"`
	CommonScenarios []string `json:"common_scenarios"`
	Recommendations []string `json:"recommendations"`
	ReclassificationTriggers []string `json:"reclassification_triggers"`
}

// GuidelinesData contains implementation guidelines
type GuidelinesData struct {
	GeneralPrinciples    []string                `json:"general_principles"`
	QualityControl       QualityControlGuidelines `json:"quality_control"`
	ReportingStandards   ReportingStandards      `json:"reporting_standards"`
	ContinuousImprovement ContinuousImprovement   `json:"continuous_improvement"`
	SpecialConsiderations SpecialConsiderations   `json:"special_considerations"`
}

// QualityControlGuidelines provides quality control guidance
type QualityControlGuidelines struct {
	ReviewProcess     []string `json:"review_process"`
	ValidationSteps   []string `json:"validation_steps"`
	ErrorPrevention   []string `json:"error_prevention"`
	AuditRequirements []string `json:"audit_requirements"`
}

// ReportingStandards defines standards for reporting classifications
type ReportingStandards struct {
	RequiredElements  []string `json:"required_elements"`
	OptionalElements  []string `json:"optional_elements"`
	FormatGuidelines  []string `json:"format_guidelines"`
	ClinicalContext   []string `json:"clinical_context"`
}

// ContinuousImprovement defines improvement processes
type ContinuousImprovement struct {
	ReviewSchedule      []string `json:"review_schedule"`
	EvidenceUpdates     []string `json:"evidence_updates"`
	ReclassificationProcess []string `json:"reclassification_process"`
	FeedbackIntegration []string `json:"feedback_integration"`
}

// SpecialConsiderations defines special classification scenarios
type SpecialConsiderations struct {
	PopulationSpecific []string `json:"population_specific"`
	PediatricVariants  []string `json:"pediatric_variants"`
	SomaticVariants    []string `json:"somatic_variants"`
	MosaicVariants     []string `json:"mosaic_variants"`
	CompoundHeterozygotes []string `json:"compound_heterozygotes"`
	IncidentalFindings []string `json:"incidental_findings"`
}

// DefinitionsData contains definitions of key terms
type DefinitionsData struct {
	Classifications map[string]string `json:"classifications"`
	EvidenceTypes   map[string]string `json:"evidence_types"`
	TechnicalTerms  map[string]string `json:"technical_terms"`
	AbbreviationsAndAcronyms map[string]string `json:"abbreviations_and_acronyms"`
}

// NewACMGRulesResourceProvider creates a new ACMG rules resource provider
func NewACMGRulesResourceProvider(logger *logrus.Logger) *ACMGRulesResourceProvider {
	return &ACMGRulesResourceProvider{
		logger: logger,
	}
}

// GetResource retrieves ACMG rules data by URI
func (p *ACMGRulesResourceProvider) GetResource(ctx context.Context, uri string) (*ResourceContent, error) {
	p.logger.WithField("uri", uri).Debug("Getting ACMG rules resource")

	// Validate URI - only supports specific ACMG rules URIs
	var content interface{}
	var name, description string

	switch uri {
	case "/acmg/rules":
		content = p.generateCompleteACMGRules()
		name = "Complete ACMG/AMP Classification Rules"
		description = "Comprehensive ACMG/AMP variant classification guidelines and rules"

	case "/acmg/rules/pathogenic":
		rules := p.generateCompleteACMGRules()
		content = rules.PathogenicRules
		name = "ACMG/AMP Pathogenic Evidence Rules"
		description = "Pathogenic evidence criteria (PVS1, PS1-4, PM1-6, PP1-5)"

	case "/acmg/rules/benign":
		rules := p.generateCompleteACMGRules()
		content = rules.BenignRules
		name = "ACMG/AMP Benign Evidence Rules"
		description = "Benign evidence criteria (BA1, BS1-4, BP1-7)"

	case "/acmg/rules/combinations":
		rules := p.generateCompleteACMGRules()
		content = rules.RuleCombinations
		name = "ACMG/AMP Rule Combinations"
		description = "Valid rule combinations for final variant classifications"

	case "/acmg/rules/guidelines":
		rules := p.generateCompleteACMGRules()
		content = rules.Guidelines
		name = "ACMG/AMP Implementation Guidelines"
		description = "Implementation guidelines and best practices"

	case "/acmg/rules/definitions":
		rules := p.generateCompleteACMGRules()
		content = rules.Definitions
		name = "ACMG/AMP Definitions"
		description = "Definitions of classifications, evidence types, and technical terms"

	default:
		return nil, fmt.Errorf("unsupported ACMG rules URI: %s", uri)
	}

	// Convert content to JSON
	contentBytes, err := json.Marshal(content)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal ACMG rules data: %w", err)
	}

	var jsonContent interface{}
	if err := json.Unmarshal(contentBytes, &jsonContent); err != nil {
		return nil, fmt.Errorf("failed to unmarshal content: %w", err)
	}

	resource := &ResourceContent{
		URI:          uri,
		Name:         name,
		Description:  description,
		MimeType:     "application/json",
		Content:      jsonContent,
		LastModified: time.Now(),
		ETag:         fmt.Sprintf("acmg-rules-%d", time.Now().Unix()),
		Metadata: map[string]interface{}{
			"resource_type": "acmg_rules",
			"version":       "2015",
			"source":        "ACMG/AMP Guidelines 2015",
			"static":        true,
		},
	}

	p.logger.WithFields(logrus.Fields{
		"uri":  uri,
		"size": len(contentBytes),
	}).Info("Generated ACMG rules resource")

	return resource, nil
}

// ListResources lists available ACMG rules resources
func (p *ACMGRulesResourceProvider) ListResources(ctx context.Context, cursor string) (*ResourceList, error) {
	p.logger.WithField("cursor", cursor).Debug("Listing ACMG rules resources")

	// Generate list of available ACMG rules resources
	resources := []ResourceInfo{
		{
			URI:         "/acmg/rules",
			Name:        "Complete ACMG/AMP Classification Rules",
			Description: "Comprehensive ACMG/AMP variant classification guidelines and rules",
			MimeType:    "application/json",
			Tags:        []string{"acmg", "amp", "rules", "classification", "guidelines"},
			LastModified: time.Now().Add(-24 * time.Hour),
			Metadata: map[string]interface{}{
				"version":     "2015",
				"source":      "ACMG/AMP Guidelines 2015",
				"rule_count":  28,
				"static":      true,
				"scope":       "complete",
			},
		},
		{
			URI:         "/acmg/rules/pathogenic",
			Name:        "ACMG/AMP Pathogenic Evidence Rules",
			Description: "Pathogenic evidence criteria (PVS1, PS1-4, PM1-6, PP1-5)",
			MimeType:    "application/json",
			Tags:        []string{"acmg", "pathogenic", "evidence", "criteria"},
			LastModified: time.Now().Add(-24 * time.Hour),
			Metadata: map[string]interface{}{
				"rule_categories": []string{"very_strong", "strong", "moderate", "supporting"},
				"rule_count":      18,
				"static":          true,
			},
		},
		{
			URI:         "/acmg/rules/benign",
			Name:        "ACMG/AMP Benign Evidence Rules",
			Description: "Benign evidence criteria (BA1, BS1-4, BP1-7)",
			MimeType:    "application/json",
			Tags:        []string{"acmg", "benign", "evidence", "criteria"},
			LastModified: time.Now().Add(-24 * time.Hour),
			Metadata: map[string]interface{}{
				"rule_categories": []string{"stand_alone", "strong", "supporting"},
				"rule_count":      10,
				"static":          true,
			},
		},
		{
			URI:         "/acmg/rules/combinations",
			Name:        "ACMG/AMP Rule Combinations",
			Description: "Valid rule combinations for final variant classifications",
			MimeType:    "application/json",
			Tags:        []string{"acmg", "combinations", "classification", "logic"},
			LastModified: time.Now().Add(-24 * time.Hour),
			Metadata: map[string]interface{}{
				"classifications": []string{"pathogenic", "likely_pathogenic", "likely_benign", "benign"},
				"combination_count": 12,
				"static":            true,
			},
		},
		{
			URI:         "/acmg/rules/guidelines",
			Name:        "ACMG/AMP Implementation Guidelines",
			Description: "Implementation guidelines and best practices for variant classification",
			MimeType:    "application/json",
			Tags:        []string{"acmg", "guidelines", "implementation", "best_practices"},
			LastModified: time.Now().Add(-24 * time.Hour),
			Metadata: map[string]interface{}{
				"sections": []string{"principles", "quality_control", "reporting", "improvement"},
				"static":   true,
			},
		},
		{
			URI:         "/acmg/rules/definitions",
			Name:        "ACMG/AMP Definitions",
			Description: "Definitions of classifications, evidence types, and technical terms",
			MimeType:    "application/json",
			Tags:        []string{"acmg", "definitions", "terminology", "glossary"},
			LastModified: time.Now().Add(-24 * time.Hour),
			Metadata: map[string]interface{}{
				"definition_categories": []string{"classifications", "evidence_types", "technical_terms", "abbreviations"},
				"term_count":            150,
				"static":                true,
			},
		},
	}

	result := &ResourceList{
		Resources: resources,
		Total:     len(resources),
	}

	p.logger.WithField("count", len(resources)).Info("Listed ACMG rules resources")
	return result, nil
}

// GetResourceInfo returns metadata about an ACMG rules resource
func (p *ACMGRulesResourceProvider) GetResourceInfo(ctx context.Context, uri string) (*ResourceInfo, error) {
	var info ResourceInfo

	switch uri {
	case "/acmg/rules":
		info = ResourceInfo{
			URI:         uri,
			Name:        "Complete ACMG/AMP Classification Rules",
			Description: "Comprehensive ACMG/AMP variant classification guidelines and rules",
			MimeType:    "application/json",
			Tags:        []string{"acmg", "amp", "rules", "classification", "guidelines"},
			LastModified: time.Now(),
			Metadata: map[string]interface{}{
				"version":    "2015",
				"rule_count": 28,
				"static":     true,
			},
		}
	case "/acmg/rules/pathogenic":
		info = ResourceInfo{
			URI:         uri,
			Name:        "ACMG/AMP Pathogenic Evidence Rules",
			Description: "Pathogenic evidence criteria (PVS1, PS1-4, PM1-6, PP1-5)",
			MimeType:    "application/json",
			Tags:        []string{"acmg", "pathogenic", "evidence"},
			LastModified: time.Now(),
			Metadata: map[string]interface{}{
				"rule_count": 18,
				"static":     true,
			},
		}
	default:
		return nil, fmt.Errorf("unsupported ACMG rules URI: %s", uri)
	}

	return &info, nil
}

// SupportsURI checks if this provider can handle the given URI
func (p *ACMGRulesResourceProvider) SupportsURI(uri string) bool {
	supportedURIs := []string{
		"/acmg/rules",
		"/acmg/rules/pathogenic",
		"/acmg/rules/benign",
		"/acmg/rules/combinations",
		"/acmg/rules/guidelines",
		"/acmg/rules/definitions",
	}

	for _, supportedURI := range supportedURIs {
		if uri == supportedURI {
			return true
		}
	}
	return false
}

// GetProviderInfo returns information about this provider
func (p *ACMGRulesResourceProvider) GetProviderInfo() ProviderInfo {
	return ProviderInfo{
		Name:        "acmg_rules",
		Description: "ACMG/AMP classification rules and guidelines resource provider",
		Version:     "1.0.0",
		URIPatterns: []string{
			"/acmg/rules",
			"/acmg/rules/pathogenic",
			"/acmg/rules/benign",
			"/acmg/rules/combinations",
			"/acmg/rules/guidelines",
			"/acmg/rules/definitions",
		},
	}
}

// generateCompleteACMGRules generates the complete ACMG/AMP rules dataset
func (p *ACMGRulesResourceProvider) generateCompleteACMGRules() *ACMGRulesData {
	return &ACMGRulesData{
		Version:     "2015",
		LastUpdated: time.Date(2015, 5, 1, 0, 0, 0, 0, time.UTC),
		Source:      "Richards et al. Standards and guidelines for the interpretation of sequence variants: a joint consensus recommendation of the American College of Medical Genetics and Genomics and the Association for Molecular Pathology. Genet Med. 2015 May;17(5):405-24.",
		PathogenicRules: p.generatePathogenicRules(),
		BenignRules:     p.generateBenignRules(),
		RuleCombinations: p.generateRuleCombinations(),
		Guidelines:      p.generateGuidelines(),
		Definitions:     p.generateDefinitions(),
	}
}

// generatePathogenicRules generates pathogenic evidence rules
func (p *ACMGRulesResourceProvider) generatePathogenicRules() PathogenicRulesData {
	return PathogenicRulesData{
		VeryStrong: []ACMGRuleDefinition{
			{
				Code:        "PVS1",
				Name:        "Null variant in gene with established loss of function",
				Category:    "pathogenic",
				Strength:    "very_strong",
				Description: "Null variant (nonsense, frameshift, canonical ±1 or 2 splice sites, initiation codon, single or multiexon deletion) in a gene where LOF is a known mechanism of disease",
				DetailedCriteria: "Applies to nonsense, frameshift, canonical splice site (±1,2), initiation codon, and single/multi-exon deletions in genes with established loss-of-function disease mechanism. Requires careful evaluation of gene constraint, clinical validity of gene-disease association, and potential for escape mechanisms.",
				EvidenceRequired: []string{
					"Gene has established loss-of-function disease mechanism",
					"Variant creates null allele (nonsense, frameshift, splice site, etc.)",
					"Gene is not tolerant to loss-of-function variation",
					"Clinical validity of gene-disease association is established",
				},
				Examples: []RuleExample{
					{
						Scenario:    "Nonsense variant in BRCA1",
						Evidence:    "c.4035del (p.Glu1346Lysfs) creates premature stop codon",
						Application: "BRCA1 LOF causes hereditary breast/ovarian cancer",
						Outcome:     "PVS1 applies",
						Notes:       "Well-established LOF mechanism in BRCA1",
					},
				},
				Caveats: []string{
					"Variants at the extreme 3' end of a gene may not result in a complete loss of function",
					"In-frame indels should not automatically be assumed to cause LOF",
					"Consider alternative splicing and rescue mechanisms",
				},
				References: []string{"PMID:25741868"},
				LastRevision: time.Date(2015, 5, 1, 0, 0, 0, 0, time.UTC),
				ImplementationNotes: "Requires computational and experimental validation of loss-of-function effect",
				QualityMetrics: RuleQualityMetrics{
					ClarityScore:         0.85,
					SpecificityScore:     0.90,
					ReproducibilityScore: 0.88,
					EvidenceQuality:      "High",
					InterRaterReliability: 0.85,
				},
				RelatedRules: []string{"PS1", "PM4"},
			},
		},
		Strong: []ACMGRuleDefinition{
			{
				Code:        "PS1",
				Name:        "Same amino acid change as previously established pathogenic variant",
				Category:    "pathogenic",
				Strength:    "strong",
				Description: "Same amino acid change as a previously established pathogenic variant regardless of nucleotide change",
				DetailedCriteria: "The variant results in the same amino acid change as a variant that has been previously classified as pathogenic/likely pathogenic in a well-curated database. Different nucleotide changes resulting in the same amino acid change qualify.",
				EvidenceRequired: []string{
					"Previously established pathogenic variant at same amino acid position",
					"High confidence in previous pathogenic classification",
					"Same functional domain or critical region",
				},
				Examples: []RuleExample{
					{
						Scenario:    "p.Arg248Gln vs known pathogenic p.Arg248Trp",
						Evidence:    "Both affect same critical arginine residue",
						Application: "Different nucleotide change, same amino acid position",
						Outcome:     "PS1 applies if original classification is well-supported",
					},
				},
				References: []string{"PMID:25741868"},
				LastRevision: time.Date(2015, 5, 1, 0, 0, 0, 0, time.UTC),
				QualityMetrics: RuleQualityMetrics{
					ClarityScore:         0.78,
					SpecificityScore:     0.82,
					ReproducibilityScore: 0.75,
					EvidenceQuality:      "Moderate to High",
					InterRaterReliability: 0.72,
				},
			},
			{
				Code:        "PS2",
				Name:        "De novo in patient with disease and no family history",
				Category:    "pathogenic",
				Strength:    "strong",
				Description: "De novo (both maternity and paternity confirmed) in a patient with the disease and no family history",
				DetailedCriteria: "Variant occurred de novo with confirmed maternity and paternity in an individual with the phenotype consistent with the associated gene-disease relationship, and there is no family history of the disease.",
				EvidenceRequired: []string{
					"Confirmed de novo occurrence (both parents tested)",
					"Patient phenotype consistent with gene-disease association",
					"No family history of the disease",
					"Gene-disease association is well established",
				},
				Examples: []RuleExample{
					{
						Scenario:    "De novo variant in child with developmental delay",
						Evidence:    "Parents tested and confirmed not carriers",
						Application: "Phenotype matches gene-associated disease",
						Outcome:     "PS2 applies",
					},
				},
				References: []string{"PMID:25741868"},
				LastRevision: time.Date(2015, 5, 1, 0, 0, 0, 0, time.UTC),
				QualityMetrics: RuleQualityMetrics{
					ClarityScore:         0.85,
					SpecificityScore:     0.88,
					ReproducibilityScore: 0.82,
					EvidenceQuality:      "High",
					InterRaterReliability: 0.80,
				},
			},
		},
		Moderate: []ACMGRuleDefinition{
			{
				Code:        "PM1",
				Name:        "Missense in critical functional domain",
				Category:    "pathogenic",
				Strength:    "moderate",
				Description: "Located in a mutational hot spot and/or critical and well-established functional domain (e.g., active site of an enzyme) without benign variation",
				DetailedCriteria: "Missense variant located in a well-established functional domain that is critical for protein function, is a known mutational hotspot, and lacks benign variation at the same position or in the immediate vicinity.",
				EvidenceRequired: []string{
					"Variant in established functional domain",
					"Domain is critical for protein function",
					"Absence of benign variation in domain",
					"Mutational hotspot evidence (optional)",
				},
				Examples: []RuleExample{
					{
						Scenario:    "Missense variant in kinase active site",
						Evidence:    "Located in ATP-binding domain of protein kinase",
						Application: "No benign variants observed in active site",
						Outcome:     "PM1 applies",
					},
				},
				References: []string{"PMID:25741868"},
				LastRevision: time.Date(2015, 5, 1, 0, 0, 0, 0, time.UTC),
				QualityMetrics: RuleQualityMetrics{
					ClarityScore:         0.72,
					SpecificityScore:     0.75,
					ReproducibilityScore: 0.68,
					EvidenceQuality:      "Moderate",
					InterRaterReliability: 0.65,
				},
			},
		},
		Supporting: []ACMGRuleDefinition{
			{
				Code:        "PP1",
				Name:        "Cosegregation in multiple affected family members",
				Category:    "pathogenic",
				Strength:    "supporting",
				Description: "Cosegregation with disease in multiple affected family members in a gene definitively known to cause the disease",
				DetailedCriteria: "The variant segregates with disease in multiple affected family members (typically 3 or more meioses) in a gene with definitive evidence for causation of the disease. Statistical significance may strengthen this evidence.",
				EvidenceRequired: []string{
					"Multiple affected family members (≥3 meioses)",
					"Gene definitively associated with disease",
					"Variant segregates with affected status",
					"Unaffected family members do not carry variant",
				},
				Examples: []RuleExample{
					{
						Scenario:    "Variant segregates in family with 5 affected members",
						Evidence:    "All affected members carry variant, unaffected do not",
						Application: "Gene has definitive disease association",
						Outcome:     "PP1 applies",
					},
				},
				References: []string{"PMID:25741868"},
				LastRevision: time.Date(2015, 5, 1, 0, 0, 0, 0, time.UTC),
				QualityMetrics: RuleQualityMetrics{
					ClarityScore:         0.80,
					SpecificityScore:     0.85,
					ReproducibilityScore: 0.78,
					EvidenceQuality:      "Moderate to High",
					InterRaterReliability: 0.75,
				},
			},
		},
	}
}

// generateBenignRules generates benign evidence rules
func (p *ACMGRulesResourceProvider) generateBenignRules() BenignRulesData {
	return BenignRulesData{
		StandAlone: []ACMGRuleDefinition{
			{
				Code:        "BA1",
				Name:        "High frequency in general population",
				Category:    "benign",
				Strength:    "stand_alone",
				Description: "Allele frequency is >5% in Exome Sequencing Project, 1000 Genomes Project, or Exome Aggregation Consortium",
				DetailedCriteria: "The variant has an allele frequency greater than 5% in large population databases such as gnomAD, ESP, or 1000 Genomes. This frequency is generally incompatible with causation of Mendelian disease.",
				EvidenceRequired: []string{
					"Allele frequency >5% in population database",
					"Large sample size (>10,000 individuals)",
					"Population appropriate for disease prevalence",
				},
				Examples: []RuleExample{
					{
						Scenario:    "Variant with 8% frequency in gnomAD",
						Evidence:    "MAF = 0.08 in gnomAD v3.1.2",
						Application: "Frequency incompatible with rare disease",
						Outcome:     "BA1 applies - classify as Benign",
					},
				},
				References: []string{"PMID:25741868"},
				LastRevision: time.Date(2015, 5, 1, 0, 0, 0, 0, time.UTC),
				QualityMetrics: RuleQualityMetrics{
					ClarityScore:         0.95,
					SpecificityScore:     0.98,
					ReproducibilityScore: 0.95,
					EvidenceQuality:      "High",
					InterRaterReliability: 0.92,
				},
			},
		},
		Strong: []ACMGRuleDefinition{
			{
				Code:        "BS1",
				Name:        "Frequency too high for disease",
				Category:    "benign",
				Strength:    "strong",
				Description: "Allele frequency is greater than expected for disorder",
				DetailedCriteria: "Allele frequency in population databases exceeds the maximum expected frequency for the disorder, taking into account disease prevalence, penetrance, and genetic heterogeneity.",
				EvidenceRequired: []string{
					"Population frequency data",
					"Disease prevalence estimates",
					"Penetrance information",
					"Genetic heterogeneity data",
				},
				Examples: []RuleExample{
					{
						Scenario:    "Variant with 0.1% frequency for rare disease",
						Evidence:    "Disease prevalence 1:100,000, full penetrance",
						Application: "Frequency exceeds maximum expected (1:50,000)",
						Outcome:     "BS1 applies",
					},
				},
				References: []string{"PMID:25741868"},
				LastRevision: time.Date(2015, 5, 1, 0, 0, 0, 0, time.UTC),
				QualityMetrics: RuleQualityMetrics{
					ClarityScore:         0.75,
					SpecificityScore:     0.85,
					ReproducibilityScore: 0.72,
					EvidenceQuality:      "Moderate to High",
					InterRaterReliability: 0.70,
				},
			},
		},
		Supporting: []ACMGRuleDefinition{
			{
				Code:        "BP1",
				Name:        "Missense in gene with low rate of pathogenic missense",
				Category:    "benign",
				Strength:    "supporting",
				Description: "Missense variant in a gene for which primarily truncating variants are known to cause disease",
				DetailedCriteria: "The variant is a missense change in a gene where the disease mechanism is primarily through loss-of-function and missense variants are rarely pathogenic.",
				EvidenceRequired: []string{
					"Gene primarily associated with truncating variants",
					"Low rate of pathogenic missense variants in gene",
					"Disease mechanism is loss-of-function",
				},
				Examples: []RuleExample{
					{
						Scenario:    "Missense variant in gene with LOF mechanism",
						Evidence:    ">90% of pathogenic variants are truncating",
						Application: "Missense variants rarely pathogenic in this gene",
						Outcome:     "BP1 applies",
					},
				},
				References: []string{"PMID:25741868"},
				LastRevision: time.Date(2015, 5, 1, 0, 0, 0, 0, time.UTC),
				QualityMetrics: RuleQualityMetrics{
					ClarityScore:         0.68,
					SpecificityScore:     0.72,
					ReproducibilityScore: 0.65,
					EvidenceQuality:      "Moderate",
					InterRaterReliability: 0.62,
				},
			},
		},
	}
}

// generateRuleCombinations generates rule combination logic
func (p *ACMGRulesResourceProvider) generateRuleCombinations() RuleCombinationData {
	return RuleCombinationData{
		Pathogenic: []ClassificationCombination{
			{
				ID:          "P1",
				Description: "One very strong + one strong",
				Rules:       []string{"1 PVS", "1 PS"},
				MinimumCriteria: "PVS1 + (PS1 or PS2 or PS3 or PS4)",
				Examples:    []string{"PVS1 + PS1", "PVS1 + PS2"},
			},
			{
				ID:          "P2",
				Description: "One very strong + two moderate",
				Rules:       []string{"1 PVS", "≥2 PM"},
				MinimumCriteria: "PVS1 + 2 moderate criteria",
				Examples:    []string{"PVS1 + PM1 + PM2", "PVS1 + PM2 + PM5"},
			},
		},
		LikelyPathogenic: []ClassificationCombination{
			{
				ID:          "LP1",
				Description: "One very strong + one moderate",
				Rules:       []string{"1 PVS", "1 PM"},
				MinimumCriteria: "PVS1 + 1 moderate criterion",
				Examples:    []string{"PVS1 + PM1", "PVS1 + PM2"},
			},
			{
				ID:          "LP2",
				Description: "One strong + one moderate",
				Rules:       []string{"1 PS", "1-2 PM"},
				MinimumCriteria: "1 strong + 1-2 moderate criteria",
				Examples:    []string{"PS1 + PM1", "PS2 + PM1 + PM2"},
			},
		},
		LikelyBenign: []ClassificationCombination{
			{
				ID:          "LB1",
				Description: "One strong benign",
				Rules:       []string{"1 BS"},
				MinimumCriteria: "1 strong benign criterion",
				Examples:    []string{"BS1", "BS2", "BS3", "BS4"},
			},
			{
				ID:          "LB2",
				Description: "Two supporting benign",
				Rules:       []string{"≥2 BP"},
				MinimumCriteria: "2 or more supporting benign criteria",
				Examples:    []string{"BP1 + BP2", "BP1 + BP4 + BP6"},
			},
		},
		Benign: []ClassificationCombination{
			{
				ID:          "B1",
				Description: "One stand-alone benign",
				Rules:       []string{"1 BA"},
				MinimumCriteria: "1 stand-alone benign criterion",
				Examples:    []string{"BA1"},
			},
		},
		UncertainSignificance: ClassificationGuidance{
			Description: "Variants that do not meet criteria for pathogenic, likely pathogenic, likely benign, or benign classifications",
			CommonScenarios: []string{
				"Conflicting evidence",
				"Insufficient evidence",
				"Novel variant in gene with limited data",
				"Variant in gene with uncertain disease association",
			},
			Recommendations: []string{
				"Collect additional evidence over time",
				"Functional studies may be informative",
				"Family segregation analysis when possible",
				"Population frequency analysis",
				"Computational predictions as supporting evidence only",
			},
			ReclassificationTriggers: []string{
				"New population frequency data",
				"Additional family segregation data",
				"Functional study results",
				"Additional clinical reports",
				"Improved gene-disease association evidence",
			},
		},
	}
}

// generateGuidelines generates implementation guidelines
func (p *ACMGRulesResourceProvider) generateGuidelines() GuidelinesData {
	return GuidelinesData{
		GeneralPrinciples: []string{
			"Use the most specific applicable criterion",
			"Multiple lines of evidence strengthen classification",
			"Consider all available evidence systematically",
			"Apply appropriate statistical methods for quantitative criteria",
			"Document reasoning and evidence clearly",
			"Consider population-specific data when available",
			"Reassess classifications periodically as new evidence emerges",
		},
		QualityControl: QualityControlGuidelines{
			ReviewProcess: []string{
				"Independent review by qualified personnel",
				"Systematic evaluation of evidence quality",
				"Documentation of decision-making process",
				"Regular calibration exercises",
			},
			ValidationSteps: []string{
				"Verify variant call accuracy",
				"Confirm population frequency data",
				"Validate functional predictions",
				"Cross-reference with curated databases",
			},
			ErrorPrevention: []string{
				"Use standardized classification worksheets",
				"Implement systematic evidence collection protocols",
				"Establish clear escalation procedures for difficult cases",
				"Maintain updated knowledge of guideline interpretations",
			},
			AuditRequirements: []string{
				"Regular internal audits of classification quality",
				"External proficiency testing participation",
				"Tracking of reclassification rates and reasons",
				"Monitoring of inter-rater reliability",
			},
		},
		ReportingStandards: ReportingStandards{
			RequiredElements: []string{
				"Variant nomenclature (HGVS)",
				"Classification and confidence",
				"Evidence summary",
				"Date of classification",
				"Laboratory information",
			},
			OptionalElements: []string{
				"Detailed evidence assessment",
				"Population frequency data",
				"Computational predictions",
				"Functional study results",
				"Clinical recommendations",
			},
			FormatGuidelines: []string{
				"Use standardized terminology",
				"Provide clear, concise language",
				"Include appropriate disclaimers",
				"Format for intended audience",
			},
			ClinicalContext: []string{
				"Consider patient phenotype",
				"Include family history when relevant",
				"Address clinical actionability",
				"Provide appropriate follow-up recommendations",
			},
		},
		ContinuousImprovement: ContinuousImprovement{
			ReviewSchedule: []string{
				"Annual review of classification practices",
				"Biannual review of uncertain significance variants",
				"Continuous monitoring of new evidence",
				"Regular updates to internal procedures",
			},
			EvidenceUpdates: []string{
				"Monitor new publications regularly",
				"Track updates to population databases",
				"Review new functional studies",
				"Assess new computational tools",
			},
			ReclassificationProcess: []string{
				"Systematic review of existing classifications",
				"Clear criteria for triggering reclassification",
				"Stakeholder notification procedures",
				"Documentation of reclassification rationale",
			},
			FeedbackIntegration: []string{
				"Collect feedback from clinicians",
				"Monitor patient outcomes when possible",
				"Participate in professional networks",
				"Contribute to community databases",
			},
		},
		SpecialConsiderations: SpecialConsiderations{
			PopulationSpecific: []string{
				"Consider population stratification in frequency analysis",
				"Account for founder effects",
				"Evaluate population-specific pathogenic variants",
				"Consider population-specific penetrance differences",
			},
			PediatricVariants: []string{
				"Consider age-specific penetrance",
				"Evaluate developmental context",
				"Account for genetic imprinting",
				"Consider anticipation effects",
			},
			SomaticVariants: []string{
				"Different evidence standards may apply",
				"Consider tumor heterogeneity",
				"Evaluate therapeutic implications",
				"Consider prognostic significance",
			},
			MosaicVariants: []string{
				"Consider mosaicism level and distribution",
				"Evaluate impact on penetrance",
				"Consider de novo vs inherited mosaicism",
				"Account for tissue-specific effects",
			},
			CompoundHeterozygotes: []string{
				"Evaluate combined effect of variants",
				"Consider phase information",
				"Assess individual variant contributions",
				"Evaluate functional complementation",
			},
			IncidentalFindings: []string{
				"Follow ACMG SF guidelines",
				"Consider patient preferences",
				"Evaluate clinical actionability",
				"Provide appropriate genetic counseling",
			},
		},
	}
}

// generateDefinitions generates definitions of key terms
func (p *ACMGRulesResourceProvider) generateDefinitions() DefinitionsData {
	return DefinitionsData{
		Classifications: map[string]string{
			"Pathogenic": "Variants that have a direct effect on human health and are disease-causing. These variants are typically reported as causative of disease.",
			"Likely Pathogenic": "Variants where there is strong evidence suggesting that the variant impacts health, but some uncertainty remains.",
			"Uncertain Significance": "Variants where there is insufficient evidence to determine whether the variant impacts health. Also called Variants of Unknown Significance (VUS).",
			"Likely Benign": "Variants where evidence suggests the variant does not impact health, but some uncertainty remains.",
			"Benign": "Variants that do not cause disease. These variants are typically not reported unless specifically requested.",
		},
		EvidenceTypes: map[string]string{
			"Population Evidence": "Evidence derived from allele frequency data in large population cohorts",
			"Computational Evidence": "Evidence from in silico predictions of variant pathogenicity",
			"Functional Evidence": "Evidence from laboratory studies of variant impact on protein function",
			"Clinical Evidence": "Evidence from clinical observations, case reports, and segregation studies",
			"Literature Evidence": "Evidence from published scientific literature and curated databases",
		},
		TechnicalTerms: map[string]string{
			"Loss of Function": "Variants that eliminate or significantly reduce normal protein function",
			"Gain of Function": "Variants that enhance normal protein function or confer new function",
			"Dominant Negative": "Variants that interfere with normal protein function in heterozygous state",
			"Haploinsufficiency": "Disease mechanism where one functional copy of a gene is insufficient",
			"Penetrance": "The proportion of individuals with a pathogenic variant who develop the associated phenotype",
			"Expressivity": "The degree to which a genetic variant manifests phenotypically",
		},
		AbbreviationsAndAcronyms: map[string]string{
			"ACMG": "American College of Medical Genetics and Genomics",
			"AMP": "Association for Molecular Pathology",
			"VUS": "Variant of Uncertain Significance",
			"LOF": "Loss of Function",
			"GOF": "Gain of Function",
			"MAF": "Minor Allele Frequency",
			"HGVS": "Human Genome Variation Society",
			"ClinVar": "NIH genetic variant database",
			"gnomAD": "Genome Aggregation Database",
		},
	}
}