package testing

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/sirupsen/logrus"
)

// E2ETestSuite manages end-to-end integration tests
type E2ETestSuite struct {
	factory       *MockClientFactory
	serverURL     string
	testData      *ClinicalTestData
	logger        *logrus.Logger
	config        E2ETestConfig
	results       []TestResult
	mutex         sync.RWMutex
}

type E2ETestConfig struct {
	ServerURL           string        `json:"server_url"`
	MaxTestDuration     time.Duration `json:"max_test_duration"`
	ConcurrentClients   int           `json:"concurrent_clients"`
	EnableCleanup       bool          `json:"enable_cleanup"`
	ValidateResponses   bool          `json:"validate_responses"`
	CollectMetrics      bool          `json:"collect_metrics"`
	RetryFailedTests    bool          `json:"retry_failed_tests"`
	LogLevel            string        `json:"log_level"`
}

type ClinicalTestData struct {
	TestVariants        []TestVariant        `json:"test_variants"`
	ExpectedResults     []ExpectedResult     `json:"expected_results"`
	RuleTestCases       []RuleTestCase       `json:"rule_test_cases"`
	ResourceTestCases   []ResourceTestCase   `json:"resource_test_cases"`
	PromptTestCases     []PromptTestCase     `json:"prompt_test_cases"`
}

type TestVariant struct {
	ID                  string                 `json:"id"`
	HGVS                string                 `json:"hgvs"`
	Gene                string                 `json:"gene"`
	Transcript          string                 `json:"transcript"`
	ExpectedClass       string                 `json:"expected_class"`
	ExpectedEvidence    map[string]string      `json:"expected_evidence"`
	Description         string                 `json:"description"`
	Source              string                 `json:"source"`
	Metadata            map[string]interface{} `json:"metadata"`
}

type RuleTestCase struct {
	RuleName            string                 `json:"rule_name"`
	Variant             string                 `json:"variant"`
	Evidence            map[string]interface{} `json:"evidence"`
	ExpectedStrength    string                 `json:"expected_strength"`
	ExpectedApplicable  bool                   `json:"expected_applicable"`
	Description         string                 `json:"description"`
}

type ResourceTestCase struct {
	URI                 string      `json:"uri"`
	ExpectedExists      bool        `json:"expected_exists"`
	ExpectedContentType string      `json:"expected_content_type"`
	ExpectedFields      []string    `json:"expected_fields"`
	ValidationRules     []string    `json:"validation_rules"`
}

type PromptTestCase struct {
	Name                string                 `json:"name"`
	Arguments           map[string]interface{} `json:"arguments"`
	ExpectedContains    []string               `json:"expected_contains"`
	ExpectedLength      int                    `json:"expected_length"`
	ValidationRules     []string               `json:"validation_rules"`
}

func NewE2ETestSuite(serverURL string, config E2ETestConfig) *E2ETestSuite {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	
	if level, err := logrus.ParseLevel(config.LogLevel); err == nil {
		logger.SetLevel(level)
	}

	return &E2ETestSuite{
		factory:   NewMockClientFactory(),
		serverURL: serverURL,
		logger:    logger,
		config:    config,
		results:   make([]TestResult, 0),
		testData:  loadClinicalTestData(),
	}
}

func loadClinicalTestData() *ClinicalTestData {
	return &ClinicalTestData{
		TestVariants: []TestVariant{
			{
				ID: "cftr_delF508", HGVS: "NM_000492.3:c.1521_1523del",
				Gene: "CFTR", Transcript: "NM_000492.3",
				ExpectedClass: "pathogenic",
				ExpectedEvidence: map[string]string{
					"PVS1": "strong", "PM2": "moderate", "PP3": "supporting",
				},
				Description: "Classic cystic fibrosis mutation",
				Source: "ClinVar",
			},
			{
				ID: "brca1_5266dup", HGVS: "NM_007294.3:c.5266dup",
				Gene: "BRCA1", Transcript: "NM_007294.3",
				ExpectedClass: "pathogenic",
				ExpectedEvidence: map[string]string{
					"PVS1": "strong", "PS3": "strong", "PM2": "moderate",
				},
				Description: "Known pathogenic BRCA1 frameshift",
				Source: "ClinVar",
			},
			{
				ID: "tp53_missense", HGVS: "NM_000546.5:c.743G>A",
				Gene: "TP53", Transcript: "NM_000546.5",
				ExpectedClass: "likely_pathogenic",
				ExpectedEvidence: map[string]string{
					"PS3": "strong", "PM2": "moderate", "PP3": "supporting",
				},
				Description: "TP53 missense mutation in DNA binding domain",
				Source: "ClinVar",
			},
			{
				ID: "benign_variant", HGVS: "NM_000492.3:c.1408A>G",
				Gene: "CFTR", Transcript: "NM_000492.3",
				ExpectedClass: "benign",
				ExpectedEvidence: map[string]string{
					"BA1": "stand_alone", "BS1": "strong",
				},
				Description: "Common benign variant",
				Source: "gnomAD",
			},
		},
		RuleTestCases: []RuleTestCase{
			{
				RuleName: "PVS1", Variant: "NM_000492.3:c.1521_1523del",
				Evidence: map[string]interface{}{
					"consequence": "frameshift",
					"gene_mechanism": "loss_of_function",
					"critical_domain": false,
				},
				ExpectedStrength: "very_strong", ExpectedApplicable: true,
				Description: "PVS1 rule for null variant in LOF gene",
			},
			{
				RuleName: "PM2", Variant: "NM_007294.3:c.5266dup",
				Evidence: map[string]interface{}{
					"allele_frequency": 0.0,
					"population_databases": []string{"gnomAD", "ExAC"},
				},
				ExpectedStrength: "moderate", ExpectedApplicable: true,
				Description: "PM2 rule for absent variant in population databases",
			},
		},
		ResourceTestCases: []ResourceTestCase{
			{
				URI: "variant/NM_000492.3:c.1521_1523del",
				ExpectedExists: true,
				ExpectedContentType: "application/json",
				ExpectedFields: []string{"hgvs", "gene", "consequence", "classification"},
			},
			{
				URI: "interpretation/cftr_delF508",
				ExpectedExists: true,
				ExpectedContentType: "application/json",
				ExpectedFields: []string{"variant_id", "classification", "evidence", "confidence"},
			},
			{
				URI: "evidence/NM_000492.3:c.1521_1523del",
				ExpectedExists: true,
				ExpectedContentType: "application/json",
				ExpectedFields: []string{"clinvar", "gnomad", "cosmic", "aggregated"},
			},
			{
				URI: "acmg/rules",
				ExpectedExists: true,
				ExpectedContentType: "application/json",
				ExpectedFields: []string{"pathogenic", "benign", "descriptions"},
			},
		},
		PromptTestCases: []PromptTestCase{
			{
				Name: "clinical_interpretation",
				Arguments: map[string]interface{}{
					"variant": "NM_000492.3:c.1521_1523del",
					"context": "diagnostic",
				},
				ExpectedContains: []string{"ACMG", "classification", "evidence"},
				ExpectedLength: 500,
			},
			{
				Name: "evidence_review",
				Arguments: map[string]interface{}{
					"variant_id": "cftr_delF508",
					"focus": "population_data",
				},
				ExpectedContains: []string{"population", "frequency", "database"},
				ExpectedLength: 300,
			},
		},
	}
}

func (suite *E2ETestSuite) RunAllTests(ctx context.Context, t *testing.T) {
	suite.logger.Info("Starting comprehensive E2E test suite")

	tests := []struct {
		name string
		fn   func(context.Context, *testing.T)
	}{
		{"TestBasicWorkflow", suite.TestBasicWorkflow},
		{"TestConcurrentClients", suite.TestConcurrentClients},
		{"TestClinicalWorkflow", suite.TestClinicalWorkflow},
		{"TestToolChaining", suite.TestToolChaining},
		{"TestResourceAccess", suite.TestResourceAccess},
		{"TestPromptGeneration", suite.TestPromptGeneration},
		{"TestErrorRecovery", suite.TestErrorRecovery},
		{"TestSessionManagement", suite.TestSessionManagement},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			testCtx, cancel := context.WithTimeout(ctx, suite.config.MaxTestDuration)
			defer cancel()
			
			suite.logger.WithField("test", test.name).Info("Running E2E test")
			test.fn(testCtx, t)
		})
	}
}

func (suite *E2ETestSuite) TestBasicWorkflow(ctx context.Context, t *testing.T) {
	// Create a single client for basic workflow testing
	clientConfig := ClientConfig{
		ID: "basic_client", Name: "BasicTestClient", Version: "1.0.0",
		Transport: TransportWebSocket,
		Capabilities: []string{"tools", "resources", "prompts"},
	}

	client, err := suite.factory.CreateClient(clientConfig)
	require.NoError(t, err)
	defer suite.factory.RemoveClient(clientConfig.ID)

	// Connect to server
	err = client.Connect(ctx, suite.serverURL)
	require.NoError(t, err)
	assert.True(t, client.IsConnected())

	// Test capability discovery
	tools, err := client.ListTools(ctx)
	require.NoError(t, err)
	assert.Contains(t, tools, "classify_variant")
	assert.Contains(t, tools, "validate_hgvs")

	resources, err := client.ListResources(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, resources)

	// Test basic tool invocation
	testVariant := suite.testData.TestVariants[0]
	result, err := client.CallTool(ctx, "validate_hgvs", map[string]interface{}{
		"notation": testVariant.HGVS,
	})
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)

	// Test resource access
	resourceResult, err := client.GetResource(ctx, "acmg/rules")
	require.NoError(t, err)
	assert.NotNil(t, resourceResult)
	assert.NotEmpty(t, resourceResult.Contents)

	// Clean disconnect
	err = client.Disconnect()
	assert.NoError(t, err)
	assert.False(t, client.IsConnected())
}

func (suite *E2ETestSuite) TestConcurrentClients(ctx context.Context, t *testing.T) {
	clientCount := suite.config.ConcurrentClients
	if clientCount == 0 {
		clientCount = 5
	}

	var clients []*MockMCPClient
	var wg sync.WaitGroup
	errCh := make(chan error, clientCount)

	// Create and connect multiple clients
	for i := 0; i < clientCount; i++ {
		clientConfig := ClientConfig{
			ID: fmt.Sprintf("concurrent_client_%d", i),
			Name: fmt.Sprintf("ConcurrentClient_%d", i),
			Version: "1.0.0",
			Transport: TransportWebSocket,
		}

		client, err := suite.factory.CreateClient(clientConfig)
		require.NoError(t, err)
		clients = append(clients, client)

		// Connect concurrently
		wg.Add(1)
		go func(c *MockMCPClient, id string) {
			defer wg.Done()
			if err := c.Connect(ctx, suite.serverURL); err != nil {
				errCh <- fmt.Errorf("client %s failed to connect: %w", id, err)
			}
		}(client, clientConfig.ID)
	}

	wg.Wait()
	close(errCh)

	// Check for connection errors
	for err := range errCh {
		require.NoError(t, err)
	}

	// Verify all clients are connected
	for _, client := range clients {
		assert.True(t, client.IsConnected())
	}

	// Execute concurrent tool calls
	testVariant := suite.testData.TestVariants[0]
	wg = sync.WaitGroup{}
	resultCh := make(chan *ToolCallResult, clientCount)

	for _, client := range clients {
		wg.Add(1)
		go func(c *MockMCPClient) {
			defer wg.Done()
			result, err := c.CallTool(ctx, "classify_variant", map[string]interface{}{
				"variant": testVariant.HGVS,
			})
			if err != nil {
				suite.logger.WithError(err).Error("Concurrent tool call failed")
				return
			}
			resultCh <- result
		}(client)
	}

	wg.Wait()
	close(resultCh)

	// Verify results
	results := make([]*ToolCallResult, 0)
	for result := range resultCh {
		results = append(results, result)
	}

	assert.Equal(t, clientCount, len(results))
	for _, result := range results {
		assert.False(t, result.IsError)
	}

	// Cleanup
	for _, client := range clients {
		client.Disconnect()
		suite.factory.RemoveClient(client.ID)
	}
}

func (suite *E2ETestSuite) TestClinicalWorkflow(ctx context.Context, t *testing.T) {
	clientConfig := ClientConfig{
		ID: "clinical_client", Name: "ClinicalTestClient", Version: "1.0.0",
		Transport: TransportWebSocket,
	}

	client, err := suite.factory.CreateClient(clientConfig)
	require.NoError(t, err)
	defer suite.factory.RemoveClient(clientConfig.ID)

	err = client.Connect(ctx, suite.serverURL)
	require.NoError(t, err)

	// Test each clinical test case
	for _, testCase := range suite.testData.TestVariants {
		t.Run(fmt.Sprintf("Variant_%s", testCase.ID), func(t *testing.T) {
			suite.runClinicalTestCase(ctx, t, client, testCase)
		})
	}

	client.Disconnect()
}

func (suite *E2ETestSuite) runClinicalTestCase(ctx context.Context, t *testing.T, client *MockMCPClient, testCase TestVariant) {
	// Step 1: Validate HGVS notation
	validationResult, err := client.CallTool(ctx, "validate_hgvs", map[string]interface{}{
		"notation": testCase.HGVS,
	})
	require.NoError(t, err)
	assert.False(t, validationResult.IsError)

	// Step 2: Gather evidence
	evidenceResult, err := client.CallTool(ctx, "query_evidence", map[string]interface{}{
		"variant": testCase.HGVS,
		"sources": []string{"clinvar", "gnomad", "cosmic"},
	})
	require.NoError(t, err)
	assert.False(t, evidenceResult.IsError)

	// Step 3: Apply ACMG rules
	for ruleName, expectedStrength := range testCase.ExpectedEvidence {
		ruleResult, err := client.CallTool(ctx, "apply_rule", map[string]interface{}{
			"rule": ruleName,
			"variant": testCase.HGVS,
			"evidence": map[string]interface{}{}, // Would be populated with actual evidence
		})
		require.NoError(t, err)
		
		if suite.config.ValidateResponses {
			suite.validateRuleApplication(t, ruleResult, ruleName, expectedStrength)
		}
	}

	// Step 4: Final classification
	classificationResult, err := client.CallTool(ctx, "classify_variant", map[string]interface{}{
		"variant": testCase.HGVS,
	})
	require.NoError(t, err)
	assert.False(t, classificationResult.IsError)

	if suite.config.ValidateResponses {
		suite.validateClassification(t, classificationResult, testCase.ExpectedClass)
	}

	// Step 5: Generate report
	reportResult, err := client.CallTool(ctx, "generate_report", map[string]interface{}{
		"variant_id": testCase.ID,
		"format": "clinical",
	})
	require.NoError(t, err)
	assert.False(t, reportResult.IsError)
}

func (suite *E2ETestSuite) TestToolChaining(ctx context.Context, t *testing.T) {
	clientConfig := ClientConfig{
		ID: "chaining_client", Name: "ChainingTestClient", Version: "1.0.0",
		Transport: TransportWebSocket,
	}

	client, err := suite.factory.CreateClient(clientConfig)
	require.NoError(t, err)
	defer suite.factory.RemoveClient(clientConfig.ID)

	err = client.Connect(ctx, suite.serverURL)
	require.NoError(t, err)

	testVariant := suite.testData.TestVariants[0]

	// Chain: validate -> query_evidence -> apply multiple rules -> combine -> classify -> report
	var intermediateResults []interface{}

	// Step 1: Validation
	validationResult, err := client.CallTool(ctx, "validate_hgvs", map[string]interface{}{
		"notation": testVariant.HGVS,
	})
	require.NoError(t, err)
	intermediateResults = append(intermediateResults, validationResult)

	// Step 2: Evidence gathering (using result from step 1)
	evidenceResult, err := client.CallTool(ctx, "query_evidence", map[string]interface{}{
		"variant": testVariant.HGVS,
	})
	require.NoError(t, err)
	intermediateResults = append(intermediateResults, evidenceResult)

	// Step 3: Apply multiple rules in sequence
	rules := []string{"PVS1", "PM2", "PP3"}
	var ruleResults []*ToolCallResult
	
	for _, rule := range rules {
		ruleResult, err := client.CallTool(ctx, "apply_rule", map[string]interface{}{
			"rule": rule,
			"variant": testVariant.HGVS,
			"evidence": evidenceResult, // Use evidence from previous step
		})
		require.NoError(t, err)
		ruleResults = append(ruleResults, ruleResult)
	}

	// Step 4: Combine evidence
	combineResult, err := client.CallTool(ctx, "combine_evidence", map[string]interface{}{
		"rule_results": ruleResults,
	})
	require.NoError(t, err)
	intermediateResults = append(intermediateResults, combineResult)

	// Step 5: Final classification using combined evidence
	classificationResult, err := client.CallTool(ctx, "classify_variant", map[string]interface{}{
		"variant": testVariant.HGVS,
		"combined_evidence": combineResult,
	})
	require.NoError(t, err)
	assert.False(t, classificationResult.IsError)

	// Validate that each step built upon the previous
	assert.Len(t, intermediateResults, 3) // validation, evidence, combine
	for i, result := range intermediateResults {
		assert.NotNil(t, result, "Intermediate result %d should not be nil", i)
	}

	client.Disconnect()
}

func (suite *E2ETestSuite) TestResourceAccess(ctx context.Context, t *testing.T) {
	clientConfig := ClientConfig{
		ID: "resource_client", Name: "ResourceTestClient", Version: "1.0.0",
		Transport: TransportWebSocket,
	}

	client, err := suite.factory.CreateClient(clientConfig)
	require.NoError(t, err)
	defer suite.factory.RemoveClient(clientConfig.ID)

	err = client.Connect(ctx, suite.serverURL)
	require.NoError(t, err)

	// Test each resource case
	for _, testCase := range suite.testData.ResourceTestCases {
		t.Run(fmt.Sprintf("Resource_%s", testCase.URI), func(t *testing.T) {
			resource, err := client.GetResource(ctx, testCase.URI)
			
			if testCase.ExpectedExists {
				require.NoError(t, err)
				assert.NotNil(t, resource)
				assert.NotEmpty(t, resource.Contents)
				
				// Validate content type if specified
				if testCase.ExpectedContentType != "" {
					assert.Equal(t, testCase.ExpectedContentType, resource.Contents[0].MimeType)
				}
				
				// Validate expected fields
				if suite.config.ValidateResponses && len(testCase.ExpectedFields) > 0 {
					suite.validateResourceFields(t, resource, testCase.ExpectedFields)
				}
			} else {
				assert.Error(t, err)
			}
		})
	}

	client.Disconnect()
}

func (suite *E2ETestSuite) TestPromptGeneration(ctx context.Context, t *testing.T) {
	clientConfig := ClientConfig{
		ID: "prompt_client", Name: "PromptTestClient", Version: "1.0.0",
		Transport: TransportWebSocket,
	}

	client, err := suite.factory.CreateClient(clientConfig)
	require.NoError(t, err)
	defer suite.factory.RemoveClient(clientConfig.ID)

	err = client.Connect(ctx, suite.serverURL)
	require.NoError(t, err)

	// Test each prompt case
	for _, testCase := range suite.testData.PromptTestCases {
		t.Run(fmt.Sprintf("Prompt_%s", testCase.Name), func(t *testing.T) {
			prompt, err := client.GetPrompt(ctx, testCase.Name, testCase.Arguments)
			require.NoError(t, err)
			assert.NotEmpty(t, prompt)
			
			if suite.config.ValidateResponses {
				// Validate expected content
				for _, expectedContent := range testCase.ExpectedContains {
					assert.Contains(t, prompt, expectedContent)
				}
				
				// Validate length if specified
				if testCase.ExpectedLength > 0 {
					assert.GreaterOrEqual(t, len(prompt), testCase.ExpectedLength)
				}
			}
		})
	}

	client.Disconnect()
}

func (suite *E2ETestSuite) TestErrorRecovery(ctx context.Context, t *testing.T) {
	clientConfig := ClientConfig{
		ID: "error_client", Name: "ErrorTestClient", Version: "1.0.0",
		Transport: TransportWebSocket,
		ErrorSimulation: ErrorSimulationConfig{
			EnableErrorSim: true,
			RequestFailRate: 0.3,
		},
	}

	client, err := suite.factory.CreateClient(clientConfig)
	require.NoError(t, err)
	defer suite.factory.RemoveClient(clientConfig.ID)

	err = client.Connect(ctx, suite.serverURL)
	require.NoError(t, err)

	// Test error scenarios
	testCases := []struct {
		name     string
		toolName string
		params   map[string]interface{}
		expectError bool
	}{
		{
			name: "invalid_tool", toolName: "nonexistent_tool",
			params: map[string]interface{}{}, expectError: true,
		},
		{
			name: "invalid_params", toolName: "classify_variant",
			params: map[string]interface{}{"invalid": "params"}, expectError: true,
		},
		{
			name: "malformed_hgvs", toolName: "validate_hgvs",
			params: map[string]interface{}{"notation": "invalid_hgvs"}, expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := client.CallTool(ctx, tc.toolName, tc.params)
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}

	client.Disconnect()
}

func (suite *E2ETestSuite) TestSessionManagement(ctx context.Context, t *testing.T) {
	// Test multiple connection/disconnection cycles
	clientConfig := ClientConfig{
		ID: "session_client", Name: "SessionTestClient", Version: "1.0.0",
		Transport: TransportWebSocket,
	}

	client, err := suite.factory.CreateClient(clientConfig)
	require.NoError(t, err)
	defer suite.factory.RemoveClient(clientConfig.ID)

	cycles := 3
	for i := 0; i < cycles; i++ {
		t.Run(fmt.Sprintf("Cycle_%d", i), func(t *testing.T) {
			// Connect
			err := client.Connect(ctx, suite.serverURL)
			require.NoError(t, err)
			assert.True(t, client.IsConnected())

			// Perform some operations
			_, err = client.ListTools(ctx)
			assert.NoError(t, err)

			_, err = client.CallTool(ctx, "validate_hgvs", map[string]interface{}{
				"notation": "NM_000492.3:c.1521_1523del",
			})
			assert.NoError(t, err)

			// Disconnect
			err = client.Disconnect()
			assert.NoError(t, err)
			assert.False(t, client.IsConnected())
		})
	}

	// Verify connection statistics
	stats := client.GetStats()
	assert.Equal(t, int(cycles), stats.ReconnectCount + 1) // +1 for initial connection
	assert.Greater(t, stats.TotalRequests, int64(0))
}

// Helper methods for validation
func (suite *E2ETestSuite) validateRuleApplication(t *testing.T, result *ToolCallResult, ruleName, expectedStrength string) {
	assert.False(t, result.IsError)
	
	// Parse result and validate strength
	if len(result.Content) > 0 {
		var ruleResult map[string]interface{}
		err := json.Unmarshal([]byte(result.Content[0].Text), &ruleResult)
		if err == nil {
			if strength, ok := ruleResult["strength"].(string); ok {
				assert.Equal(t, expectedStrength, strength, 
					"Rule %s should have strength %s", ruleName, expectedStrength)
			}
		}
	}
}

func (suite *E2ETestSuite) validateClassification(t *testing.T, result *ToolCallResult, expectedClass string) {
	assert.False(t, result.IsError)
	
	// Parse result and validate classification
	if len(result.Content) > 0 {
		var classResult map[string]interface{}
		err := json.Unmarshal([]byte(result.Content[0].Text), &classResult)
		if err == nil {
			if classification, ok := classResult["classification"].(string); ok {
				assert.Equal(t, expectedClass, classification,
					"Classification should be %s", expectedClass)
			}
		}
	}
}

func (suite *E2ETestSuite) validateResourceFields(t *testing.T, resource *ResourceResponse, expectedFields []string) {
	if len(resource.Contents) == 0 {
		t.Fatal("Resource has no content")
	}
	
	var resourceData map[string]interface{}
	err := json.Unmarshal([]byte(resource.Contents[0].Text), &resourceData)
	if err != nil {
		t.Fatalf("Failed to parse resource content: %v", err)
	}
	
	for _, field := range expectedFields {
		assert.Contains(t, resourceData, field, "Resource should contain field %s", field)
	}
}

func (suite *E2ETestSuite) GetResults() []TestResult {
	suite.mutex.RLock()
	defer suite.mutex.RUnlock()
	
	results := make([]TestResult, len(suite.results))
	copy(results, suite.results)
	return results
}

func (suite *E2ETestSuite) Cleanup() error {
	return suite.factory.CleanupAll()
}