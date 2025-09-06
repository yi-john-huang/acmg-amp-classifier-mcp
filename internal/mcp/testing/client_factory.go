package testing

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// MockClientFactory creates and manages multiple mock MCP clients for testing
type MockClientFactory struct {
	clients     map[string]*MockMCPClient
	scenarios   map[string]TestScenario
	mutex       sync.RWMutex
	logger      *logrus.Logger
	defaultConfig MockClientConfig
}

type TestScenario struct {
	Name                string                `json:"name"`
	Description         string                `json:"description"`
	ClientConfigs       []ClientConfig        `json:"client_configs"`
	TestSequence        []TestStep            `json:"test_sequence"`
	ExpectedResults     []ExpectedResult      `json:"expected_results"`
	MaxDuration         time.Duration         `json:"max_duration"`
	CleanupAfter        bool                  `json:"cleanup_after"`
	ConcurrentExecution bool                  `json:"concurrent_execution"`
}

type ClientConfig struct {
	ID               string                `json:"id"`
	Name             string                `json:"name"`
	Version          string                `json:"version"`
	Transport        TransportType         `json:"transport"`
	MockConfig       MockClientConfig      `json:"mock_config"`
	ErrorSimulation  ErrorSimulationConfig `json:"error_simulation"`
	Capabilities     []string              `json:"capabilities"`
	CustomHeaders    map[string]string     `json:"custom_headers"`
	AuthToken        string                `json:"auth_token"`
}

type TestStep struct {
	Type        StepType               `json:"type"`
	ClientID    string                 `json:"client_id"`
	Method      string                 `json:"method"`
	Parameters  map[string]interface{} `json:"parameters"`
	ExpectError bool                   `json:"expect_error"`
	Delay       time.Duration          `json:"delay"`
	Timeout     time.Duration          `json:"timeout"`
	Retry       int                    `json:"retry"`
	Parallel    bool                   `json:"parallel"`
}

type ExpectedResult struct {
	ClientID      string                 `json:"client_id"`
	StepIndex     int                    `json:"step_index"`
	Success       bool                   `json:"success"`
	ResponseTime  time.Duration          `json:"max_response_time"`
	ErrorContains string                 `json:"error_contains"`
	ResultContains map[string]interface{} `json:"result_contains"`
	StatsCheck    StatsExpectation       `json:"stats_check"`
}

type StatsExpectation struct {
	MinRequests     int64         `json:"min_requests"`
	MaxRequests     int64         `json:"max_requests"`
	MaxFailureRate  float64       `json:"max_failure_rate"`
	MaxResponseTime time.Duration `json:"max_response_time"`
}

type StepType string

const (
	StepConnect         StepType = "connect"
	StepDisconnect      StepType = "disconnect"
	StepCallTool        StepType = "call_tool"
	StepGetResource     StepType = "get_resource"
	StepListTools       StepType = "list_tools"
	StepListResources   StepType = "list_resources"
	StepGetPrompt       StepType = "get_prompt"
	StepSubscribe       StepType = "subscribe"
	StepUnsubscribe     StepType = "unsubscribe"
	StepWait            StepType = "wait"
	StepCheckStats      StepType = "check_stats"
	StepSimulateError   StepType = "simulate_error"
)

// Pre-defined test scenarios
var DefaultTestScenarios = map[string]TestScenario{
	"basic_functionality": {
		Name: "Basic MCP Functionality Test",
		Description: "Tests basic MCP client operations with a single client",
		ClientConfigs: []ClientConfig{
			{
				ID: "claude_client", Name: "Claude", Version: "1.0.0",
				Transport: TransportWebSocket,
				Capabilities: []string{"tools", "resources", "prompts"},
			},
		},
		TestSequence: []TestStep{
			{Type: StepConnect, ClientID: "claude_client"},
			{Type: StepListTools, ClientID: "claude_client"},
			{Type: StepCallTool, ClientID: "claude_client", Method: "classify_variant", 
				Parameters: map[string]interface{}{
					"variant": "NM_000492.3:c.1521_1523del",
				}},
			{Type: StepGetResource, ClientID: "claude_client", 
				Parameters: map[string]interface{}{"uri": "variant/NM_000492.3:c.1521_1523del"}},
			{Type: StepDisconnect, ClientID: "claude_client"},
		},
		MaxDuration: 60 * time.Second,
		CleanupAfter: true,
	},
	"concurrent_clients": {
		Name: "Concurrent Multiple Clients Test",
		Description: "Tests multiple clients accessing MCP server concurrently",
		ClientConfigs: []ClientConfig{
			{ID: "claude_1", Name: "Claude", Version: "1.0.0", Transport: TransportWebSocket},
			{ID: "claude_2", Name: "Claude", Version: "1.0.0", Transport: TransportWebSocket},
			{ID: "chatgpt_1", Name: "ChatGPT", Version: "1.0.0", Transport: TransportHTTP},
		},
		TestSequence: []TestStep{
			{Type: StepConnect, ClientID: "claude_1", Parallel: true},
			{Type: StepConnect, ClientID: "claude_2", Parallel: true},
			{Type: StepConnect, ClientID: "chatgpt_1", Parallel: true},
			{Type: StepCallTool, ClientID: "claude_1", Method: "classify_variant", Parallel: true,
				Parameters: map[string]interface{}{"variant": "NM_000492.3:c.1521_1523del"}},
			{Type: StepCallTool, ClientID: "claude_2", Method: "validate_hgvs", Parallel: true,
				Parameters: map[string]interface{}{"notation": "NM_000492.3:c.1521_1523del"}},
			{Type: StepCallTool, ClientID: "chatgpt_1", Method: "apply_rule", Parallel: true,
				Parameters: map[string]interface{}{"rule": "PVS1", "variant": "test"}},
		},
		ConcurrentExecution: true,
		MaxDuration: 30 * time.Second,
		CleanupAfter: true,
	},
	"error_handling": {
		Name: "Error Handling and Recovery Test",
		Description: "Tests client behavior under various error conditions",
		ClientConfigs: []ClientConfig{
			{
				ID: "error_client", Name: "ErrorClient", Version: "1.0.0",
				Transport: TransportWebSocket,
				ErrorSimulation: ErrorSimulationConfig{
					EnableErrorSim: true,
					RequestFailRate: 0.3,
					TimeoutRate: 0.1,
				},
			},
		},
		TestSequence: []TestStep{
			{Type: StepConnect, ClientID: "error_client"},
			{Type: StepCallTool, ClientID: "error_client", Method: "classify_variant", 
				ExpectError: true, Retry: 3},
			{Type: StepCallTool, ClientID: "error_client", Method: "invalid_tool", 
				ExpectError: true},
			{Type: StepGetResource, ClientID: "error_client", 
				Parameters: map[string]interface{}{"uri": "nonexistent/resource"}, 
				ExpectError: true},
		},
		MaxDuration: 90 * time.Second,
		CleanupAfter: true,
	},
	"performance_stress": {
		Name: "Performance and Stress Test",
		Description: "Tests server performance under high load",
		ClientConfigs: generateStressTestClients(10),
		ConcurrentExecution: true,
		MaxDuration: 5 * time.Minute,
		CleanupAfter: true,
	},
}

func NewMockClientFactory() *MockClientFactory {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})

	return &MockClientFactory{
		clients:   make(map[string]*MockMCPClient),
		scenarios: DefaultTestScenarios,
		logger:    logger,
		defaultConfig: MockClientConfig{
			ConnectTimeout: 10 * time.Second,
			RequestTimeout: 30 * time.Second,
			MaxRetries:     3,
			RetryDelay:     time.Second,
			EnableLogging:  true,
		},
	}
}

func (f *MockClientFactory) CreateClient(config ClientConfig) (*MockMCPClient, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if _, exists := f.clients[config.ID]; exists {
		return nil, fmt.Errorf("client with ID %s already exists", config.ID)
	}

	client := NewMockMCPClient(config.ID, config.Name, config.Version, config.Transport)
	
	// Apply configuration
	if config.MockConfig != (MockClientConfig{}) {
		client.SetConfig(config.MockConfig)
	} else {
		client.SetConfig(f.defaultConfig)
	}
	
	if config.ErrorSimulation != (ErrorSimulationConfig{}) {
		client.SetErrorSimulation(config.ErrorSimulation)
	}

	client.Capabilities = config.Capabilities

	f.clients[config.ID] = client

	f.logger.WithFields(logrus.Fields{
		"client_id": config.ID,
		"name":      config.Name,
		"transport": config.Transport,
	}).Info("Mock client created")

	return client, nil
}

func (f *MockClientFactory) GetClient(clientID string) (*MockMCPClient, error) {
	f.mutex.RLock()
	defer f.mutex.RUnlock()

	client, exists := f.clients[clientID]
	if !exists {
		return nil, fmt.Errorf("client with ID %s not found", clientID)
	}

	return client, nil
}

func (f *MockClientFactory) RemoveClient(clientID string) error {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	client, exists := f.clients[clientID]
	if !exists {
		return fmt.Errorf("client with ID %s not found", clientID)
	}

	if client.IsConnected() {
		client.Disconnect()
	}

	delete(f.clients, clientID)

	f.logger.WithField("client_id", clientID).Info("Mock client removed")
	return nil
}

func (f *MockClientFactory) ListClients() []string {
	f.mutex.RLock()
	defer f.mutex.RUnlock()

	clientIDs := make([]string, 0, len(f.clients))
	for id := range f.clients {
		clientIDs = append(clientIDs, id)
	}

	return clientIDs
}

func (f *MockClientFactory) CleanupAll() error {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	for id, client := range f.clients {
		if client.IsConnected() {
			client.Disconnect()
		}
		delete(f.clients, id)
	}

	f.logger.Info("All mock clients cleaned up")
	return nil
}

func (f *MockClientFactory) AddScenario(scenario TestScenario) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.scenarios[scenario.Name] = scenario
}

func (f *MockClientFactory) RunScenario(ctx context.Context, scenarioName, serverURL string) (*TestResult, error) {
	scenario, exists := f.scenarios[scenarioName]
	if !exists {
		return nil, fmt.Errorf("scenario %s not found", scenarioName)
	}

	f.logger.WithField("scenario", scenarioName).Info("Starting test scenario")

	// Set timeout for entire scenario
	if scenario.MaxDuration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, scenario.MaxDuration)
		defer cancel()
	}

	result := &TestResult{
		ScenarioName: scenarioName,
		StartTime:    time.Now(),
		Steps:        make([]StepResult, 0),
		ClientStats:  make(map[string]ConnectionStats),
	}

	// Create clients
	for _, config := range scenario.ClientConfigs {
		_, err := f.CreateClient(config)
		if err != nil {
			result.Error = err.Error()
			result.Success = false
			return result, err
		}
		defer f.RemoveClient(config.ID)
	}

	// Execute test steps
	if scenario.ConcurrentExecution {
		err := f.executeStepsConcurrently(ctx, scenario.TestSequence, serverURL, result)
		if err != nil {
			result.Error = err.Error()
			result.Success = false
		}
	} else {
		err := f.executeStepsSequentially(ctx, scenario.TestSequence, serverURL, result)
		if err != nil {
			result.Error = err.Error()
			result.Success = false
		}
	}

	// Collect final statistics
	for _, clientID := range f.ListClients() {
		if client, err := f.GetClient(clientID); err == nil {
			result.ClientStats[clientID] = client.GetStats()
		}
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	// Validate results against expectations
	if len(scenario.ExpectedResults) > 0 {
		result.Success = f.validateResults(result, scenario.ExpectedResults)
	} else {
		result.Success = result.Error == ""
	}

	if scenario.CleanupAfter {
		f.CleanupAll()
	}

	f.logger.WithFields(logrus.Fields{
		"scenario": scenarioName,
		"success":  result.Success,
		"duration": result.Duration,
	}).Info("Test scenario completed")

	return result, nil
}

func (f *MockClientFactory) executeStepsSequentially(ctx context.Context, steps []TestStep, serverURL string, result *TestResult) error {
	for i, step := range steps {
		stepResult := f.executeStep(ctx, step, serverURL, i)
		result.Steps = append(result.Steps, stepResult)

		if !stepResult.Success && !step.ExpectError {
			return fmt.Errorf("step %d failed: %s", i, stepResult.Error)
		}

		if step.Delay > 0 {
			time.Sleep(step.Delay)
		}
	}
	return nil
}

func (f *MockClientFactory) executeStepsConcurrently(ctx context.Context, steps []TestStep, serverURL string, result *TestResult) error {
	var wg sync.WaitGroup
	stepResults := make([]StepResult, len(steps))
	errorCh := make(chan error, len(steps))

	for i, step := range steps {
		if step.Parallel {
			wg.Add(1)
			go func(index int, s TestStep) {
				defer wg.Done()
				stepResults[index] = f.executeStep(ctx, s, serverURL, index)
				if !stepResults[index].Success && !s.ExpectError {
					errorCh <- fmt.Errorf("parallel step %d failed: %s", index, stepResults[index].Error)
				}
			}(i, step)
		} else {
			stepResults[i] = f.executeStep(ctx, step, serverURL, i)
			if !stepResults[i].Success && !step.ExpectError {
				return fmt.Errorf("sequential step %d failed: %s", i, stepResults[i].Error)
			}
		}
	}

	wg.Wait()
	close(errorCh)

	// Check for any parallel errors
	for err := range errorCh {
		if err != nil {
			return err
		}
	}

	result.Steps = stepResults
	return nil
}

func (f *MockClientFactory) executeStep(ctx context.Context, step TestStep, serverURL string, stepIndex int) StepResult {
	startTime := time.Now()
	stepResult := StepResult{
		StepIndex: stepIndex,
		Type:      step.Type,
		ClientID:  step.ClientID,
		StartTime: startTime,
	}

	client, err := f.GetClient(step.ClientID)
	if err != nil {
		stepResult.Error = err.Error()
		stepResult.Success = false
		stepResult.Duration = time.Since(startTime)
		return stepResult
	}

	// Apply step timeout if specified
	if step.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, step.Timeout)
		defer cancel()
	}

	// Execute step with retry logic
	for attempt := 0; attempt <= step.Retry; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Second * time.Duration(attempt))
		}

		err = f.performStepAction(ctx, client, step, serverURL, &stepResult)
		if err == nil {
			stepResult.Success = true
			break
		}

		if attempt == step.Retry {
			stepResult.Error = err.Error()
			stepResult.Success = false
		}
	}

	stepResult.Duration = time.Since(startTime)
	return stepResult
}

func (f *MockClientFactory) performStepAction(ctx context.Context, client *MockMCPClient, step TestStep, serverURL string, result *StepResult) error {
	switch step.Type {
	case StepConnect:
		return client.Connect(ctx, serverURL)
		
	case StepDisconnect:
		return client.Disconnect()
		
	case StepCallTool:
		toolName := step.Method
		args := step.Parameters
		if args == nil {
			args = make(map[string]interface{})
		}
		
		toolResult, err := client.CallTool(ctx, toolName, args)
		if err != nil {
			return err
		}
		result.Response = toolResult
		return nil
		
	case StepGetResource:
		uri, ok := step.Parameters["uri"].(string)
		if !ok {
			return fmt.Errorf("resource URI not specified")
		}
		
		resource, err := client.GetResource(ctx, uri)
		if err != nil {
			return err
		}
		result.Response = resource
		return nil
		
	case StepListTools:
		tools, err := client.ListTools(ctx)
		if err != nil {
			return err
		}
		result.Response = tools
		return nil
		
	case StepListResources:
		resources, err := client.ListResources(ctx)
		if err != nil {
			return err
		}
		result.Response = resources
		return nil
		
	case StepGetPrompt:
		name, ok := step.Parameters["name"].(string)
		if !ok {
			return fmt.Errorf("prompt name not specified")
		}
		
		args, _ := step.Parameters["arguments"].(map[string]interface{})
		prompt, err := client.GetPrompt(ctx, name, args)
		if err != nil {
			return err
		}
		result.Response = prompt
		return nil
		
	case StepWait:
		if delay, ok := step.Parameters["duration"].(time.Duration); ok {
			time.Sleep(delay)
		}
		return nil
		
	case StepCheckStats:
		stats := client.GetStats()
		result.Response = stats
		return nil
		
	default:
		return fmt.Errorf("unsupported step type: %s", step.Type)
	}
}

func (f *MockClientFactory) validateResults(result *TestResult, expectations []ExpectedResult) bool {
	for _, expectation := range expectations {
		if !f.validateExpectation(result, expectation) {
			return false
		}
	}
	return true
}

func (f *MockClientFactory) validateExpectation(result *TestResult, expectation ExpectedResult) bool {
	// Validate step-specific expectations
	if expectation.StepIndex < len(result.Steps) {
		step := result.Steps[expectation.StepIndex]
		
		if step.Success != expectation.Success {
			return false
		}
		
		if expectation.ResponseTime > 0 && step.Duration > expectation.ResponseTime {
			return false
		}
		
		if expectation.ErrorContains != "" && !contains(step.Error, expectation.ErrorContains) {
			return false
		}
	}
	
	// Validate client statistics
	if clientStats, exists := result.ClientStats[expectation.ClientID]; exists {
		if !f.validateStatsExpectation(clientStats, expectation.StatsCheck) {
			return false
		}
	}
	
	return true
}

func (f *MockClientFactory) validateStatsExpectation(stats ConnectionStats, expectation StatsExpectation) bool {
	if expectation.MinRequests > 0 && stats.TotalRequests < expectation.MinRequests {
		return false
	}
	
	if expectation.MaxRequests > 0 && stats.TotalRequests > expectation.MaxRequests {
		return false
	}
	
	if expectation.MaxFailureRate > 0 {
		failureRate := float64(stats.FailedRequests) / float64(stats.TotalRequests)
		if failureRate > expectation.MaxFailureRate {
			return false
		}
	}
	
	if expectation.MaxResponseTime > 0 && stats.AverageResponseTime > expectation.MaxResponseTime {
		return false
	}
	
	return true
}

func generateStressTestClients(count int) []ClientConfig {
	configs := make([]ClientConfig, count)
	transports := []TransportType{TransportWebSocket, TransportHTTP, TransportSSE}
	
	for i := 0; i < count; i++ {
		configs[i] = ClientConfig{
			ID:        fmt.Sprintf("stress_client_%d", i),
			Name:      fmt.Sprintf("StressClient_%d", i),
			Version:   "1.0.0",
			Transport: transports[i%len(transports)],
			Capabilities: []string{"tools", "resources"},
		}
	}
	
	return configs
}

func contains(s, substr string) bool {
	return len(substr) == 0 || (len(s) > 0 && len(substr) > 0 && 
		fmt.Sprintf("%s", s) != fmt.Sprintf("%s", s[:len(s)-len(substr)]) && 
		fmt.Sprintf("%s", s[len(substr):]) != s)
}

// TestResult represents the outcome of running a test scenario
type TestResult struct {
	ScenarioName string                      `json:"scenario_name"`
	Success      bool                        `json:"success"`
	Error        string                      `json:"error,omitempty"`
	StartTime    time.Time                   `json:"start_time"`
	EndTime      time.Time                   `json:"end_time"`
	Duration     time.Duration               `json:"duration"`
	Steps        []StepResult                `json:"steps"`
	ClientStats  map[string]ConnectionStats  `json:"client_stats"`
}

// StepResult represents the outcome of executing a single test step
type StepResult struct {
	StepIndex int           `json:"step_index"`
	Type      StepType      `json:"type"`
	ClientID  string        `json:"client_id"`
	Success   bool          `json:"success"`
	Error     string        `json:"error,omitempty"`
	StartTime time.Time     `json:"start_time"`
	Duration  time.Duration `json:"duration"`
	Response  interface{}   `json:"response,omitempty"`
}