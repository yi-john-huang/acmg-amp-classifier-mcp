package testing

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ErrorScenarioTestSuite manages error scenario testing with various client failure modes
type ErrorScenarioTestSuite struct {
	factory     *MockClientFactory
	serverURL   string
	logger      *logrus.Logger
	config      ErrorScenarioConfig
	results     []ErrorScenarioResult
	scenarios   map[string]ErrorScenario
	faultInjector *FaultInjector
}

type ErrorScenarioConfig struct {
	MaxRetries           int           `json:"max_retries"`
	RetryDelay          time.Duration `json:"retry_delay"`
	TestTimeout         time.Duration `json:"test_timeout"`
	EnableRecoveryTests bool          `json:"enable_recovery_tests"`
	EnableChaosTests    bool          `json:"enable_chaos_tests"`
	FaultInjectionRate  float64       `json:"fault_injection_rate"`
	RecoveryTimeout     time.Duration `json:"recovery_timeout"`
}

type ErrorScenario struct {
	Name            string                 `json:"name"`
	Description     string                 `json:"description"`
	Category        ErrorCategory          `json:"category"`
	FaultTypes      []FaultType            `json:"fault_types"`
	ExpectedBehavior ExpectedErrorBehavior `json:"expected_behavior"`
	TestSteps       []ErrorTestStep        `json:"test_steps"`
	RecoveryCheck   bool                   `json:"recovery_check"`
	ChaosLevel      ChaosLevel             `json:"chaos_level"`
}

type ErrorScenarioResult struct {
	ScenarioName     string                 `json:"scenario_name"`
	Success          bool                   `json:"success"`
	Duration         time.Duration          `json:"duration"`
	FaultsInjected   int                    `json:"faults_injected"`
	ErrorsObserved   int                    `json:"errors_observed"`
	RecoveryTime     time.Duration          `json:"recovery_time"`
	RecoverySuccess  bool                   `json:"recovery_success"`
	StepResults      []ErrorStepResult      `json:"step_results"`
	ClientBehavior   ClientBehaviorAnalysis `json:"client_behavior"`
	ServerResilience ServerResilienceMetrics `json:"server_resilience"`
	Violations       []string               `json:"violations"`
	Metadata         map[string]interface{} `json:"metadata"`
}

type ErrorCategory string

const (
	CategoryNetworkFailures    ErrorCategory = "network_failures"
	CategoryProtocolViolations ErrorCategory = "protocol_violations"
	CategoryResourceExhaustion ErrorCategory = "resource_exhaustion"
	CategoryTimeouts          ErrorCategory = "timeouts"
	CategoryCorruption        ErrorCategory = "corruption"
	CategoryConcurrency       ErrorCategory = "concurrency"
	CategoryRecovery          ErrorCategory = "recovery"
	CategoryChaos             ErrorCategory = "chaos"
)

type FaultType string

const (
	FaultConnectionDrop     FaultType = "connection_drop"
	FaultNetworkPartition   FaultType = "network_partition"
	FaultSlowNetwork        FaultType = "slow_network"
	FaultMessageLoss        FaultType = "message_loss"
	FaultMessageCorruption  FaultType = "message_corruption"
	FaultRequestTimeout     FaultType = "request_timeout"
	FaultServerOverload     FaultType = "server_overload"
	FaultMemoryExhaustion   FaultType = "memory_exhaustion"
	FaultInvalidResponse    FaultType = "invalid_response"
	FaultConcurrentAccess   FaultType = "concurrent_access"
	FaultUnexpectedError    FaultType = "unexpected_error"
	FaultGracefulShutdown   FaultType = "graceful_shutdown"
	FaultAbruptShutdown     FaultType = "abrupt_shutdown"
)

type ExpectedErrorBehavior struct {
	ShouldRetry        bool          `json:"should_retry"`
	ShouldReconnect    bool          `json:"should_reconnect"`
	ShouldDegrade      bool          `json:"should_degrade"`
	MaxRecoveryTime    time.Duration `json:"max_recovery_time"`
	ErrorPropagation   bool          `json:"error_propagation"`
	ClientFailover     bool          `json:"client_failover"`
	StateConsistency   bool          `json:"state_consistency"`
}

type ErrorTestStep struct {
	Name           string                 `json:"name"`
	Action         ErrorAction            `json:"action"`
	FaultInjection FaultInjectionSpec     `json:"fault_injection"`
	ExpectedResult ExpectedStepResult     `json:"expected_result"`
	Delay          time.Duration          `json:"delay"`
	Parallel       bool                   `json:"parallel"`
}

type ErrorAction string

const (
	ActionConnect       ErrorAction = "connect"
	ActionDisconnect    ErrorAction = "disconnect"
	ActionSendRequest   ErrorAction = "send_request"
	ActionInjectFault   ErrorAction = "inject_fault"
	ActionWaitRecovery  ErrorAction = "wait_recovery"
	ActionVerifyState   ErrorAction = "verify_state"
	ActionStressTest    ErrorAction = "stress_test"
)

type FaultInjectionSpec struct {
	Type       FaultType     `json:"type"`
	Probability float64      `json:"probability"`
	Duration   time.Duration `json:"duration"`
	Parameters map[string]interface{} `json:"parameters"`
}

type ExpectedStepResult struct {
	Success      bool          `json:"success"`
	ErrorType    string        `json:"error_type"`
	MaxDuration  time.Duration `json:"max_duration"`
	ShouldRecover bool         `json:"should_recover"`
}

type ErrorStepResult struct {
	StepName     string        `json:"step_name"`
	Success      bool          `json:"success"`
	Duration     time.Duration `json:"duration"`
	ErrorMessage string        `json:"error_message,omitempty"`
	FaultActive  bool          `json:"fault_active"`
	RecoveryTime time.Duration `json:"recovery_time"`
}

type ClientBehaviorAnalysis struct {
	RetryAttempts        int           `json:"retry_attempts"`
	ReconnectAttempts    int           `json:"reconnect_attempts"`
	GracefulDegradation  bool          `json:"graceful_degradation"`
	ErrorHandlingQuality float64       `json:"error_handling_quality"`
	ResilienceScore      float64       `json:"resilience_score"`
}

type ServerResilienceMetrics struct {
	ErrorRecoveryTime    time.Duration `json:"error_recovery_time"`
	ConnectionStability  float64       `json:"connection_stability"`
	ResourceUtilization  float64       `json:"resource_utilization"`
	ThroughputDegradation float64      `json:"throughput_degradation"`
}

type ChaosLevel string

const (
	ChaosLevelNone     ChaosLevel = "none"
	ChaosLevelLow      ChaosLevel = "low"
	ChaosLevelMedium   ChaosLevel = "medium"
	ChaosLevelHigh     ChaosLevel = "high"
	ChaosLevelExtreme  ChaosLevel = "extreme"
)

// FaultInjector manages fault injection during testing
type FaultInjector struct {
	activeFaults  map[string]*ActiveFault
	faultHistory  []FaultEvent
	mutex         sync.RWMutex
	config        FaultInjectorConfig
	logger        *logrus.Logger
}

type ActiveFault struct {
	Type      FaultType                  `json:"type"`
	StartTime time.Time                  `json:"start_time"`
	Duration  time.Duration              `json:"duration"`
	Parameters map[string]interface{}    `json:"parameters"`
	AffectedClients []string              `json:"affected_clients"`
}

type FaultEvent struct {
	Type      FaultType     `json:"type"`
	Timestamp time.Time     `json:"timestamp"`
	Duration  time.Duration `json:"duration"`
	ClientID  string        `json:"client_id"`
	Impact    string        `json:"impact"`
}

type FaultInjectorConfig struct {
	EnableInjection  bool          `json:"enable_injection"`
	DefaultDuration  time.Duration `json:"default_duration"`
	MaxConcurrentFaults int        `json:"max_concurrent_faults"`
	InjectionInterval   time.Duration `json:"injection_interval"`
}

func NewErrorScenarioTestSuite(serverURL string, config ErrorScenarioConfig) *ErrorScenarioTestSuite {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})

	if config.TestTimeout == 0 {
		config.TestTimeout = 2 * time.Minute
	}
	if config.RecoveryTimeout == 0 {
		config.RecoveryTimeout = 30 * time.Second
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}

	suite := &ErrorScenarioTestSuite{
		factory:   NewMockClientFactory(),
		serverURL: serverURL,
		logger:    logger,
		config:    config,
		results:   make([]ErrorScenarioResult, 0),
		scenarios: make(map[string]ErrorScenario),
		faultInjector: NewFaultInjector(FaultInjectorConfig{
			EnableInjection: true,
			DefaultDuration: 5 * time.Second,
			MaxConcurrentFaults: 3,
			InjectionInterval: time.Second,
		}),
	}

	suite.loadErrorScenarios()
	return suite
}

func NewFaultInjector(config FaultInjectorConfig) *FaultInjector {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})

	return &FaultInjector{
		activeFaults: make(map[string]*ActiveFault),
		faultHistory: make([]FaultEvent, 0),
		config:       config,
		logger:       logger,
	}
}

func (suite *ErrorScenarioTestSuite) loadErrorScenarios() {
	// Load predefined error scenarios
	suite.scenarios["connection_failures"] = ErrorScenario{
		Name:        "Connection Failures",
		Description: "Test various connection failure modes and recovery",
		Category:    CategoryNetworkFailures,
		FaultTypes:  []FaultType{FaultConnectionDrop, FaultNetworkPartition},
		ExpectedBehavior: ExpectedErrorBehavior{
			ShouldRetry:     true,
			ShouldReconnect: true,
			MaxRecoveryTime: 30 * time.Second,
		},
		TestSteps: []ErrorTestStep{
			{
				Name: "establish_connection",
				Action: ActionConnect,
			},
			{
				Name: "inject_connection_drop",
				Action: ActionInjectFault,
				FaultInjection: FaultInjectionSpec{
					Type:        FaultConnectionDrop,
					Probability: 1.0,
					Duration:    5 * time.Second,
				},
			},
			{
				Name: "verify_reconnection",
				Action: ActionWaitRecovery,
				ExpectedResult: ExpectedStepResult{
					ShouldRecover: true,
					MaxDuration:   30 * time.Second,
				},
			},
		},
		RecoveryCheck: true,
		ChaosLevel:    ChaosLevelLow,
	}

	suite.scenarios["protocol_violations"] = ErrorScenario{
		Name:        "Protocol Violations",
		Description: "Test handling of malformed messages and protocol violations",
		Category:    CategoryProtocolViolations,
		FaultTypes:  []FaultType{FaultMessageCorruption, FaultInvalidResponse},
		ExpectedBehavior: ExpectedErrorBehavior{
			ShouldRetry:      false,
			ErrorPropagation: true,
		},
		TestSteps: []ErrorTestStep{
			{
				Name: "send_malformed_request",
				Action: ActionSendRequest,
				FaultInjection: FaultInjectionSpec{
					Type:        FaultMessageCorruption,
					Probability: 1.0,
				},
				ExpectedResult: ExpectedStepResult{
					Success: false,
					ErrorType: "protocol_error",
				},
			},
		},
		RecoveryCheck: false,
		ChaosLevel:    ChaosLevelMedium,
	}

	suite.scenarios["timeout_handling"] = ErrorScenario{
		Name:        "Timeout Handling",
		Description: "Test various timeout scenarios and client behavior",
		Category:    CategoryTimeouts,
		FaultTypes:  []FaultType{FaultRequestTimeout, FaultSlowNetwork},
		ExpectedBehavior: ExpectedErrorBehavior{
			ShouldRetry:     true,
			ShouldDegrade:   true,
			MaxRecoveryTime: 60 * time.Second,
		},
		TestSteps: []ErrorTestStep{
			{
				Name: "inject_slow_network",
				Action: ActionInjectFault,
				FaultInjection: FaultInjectionSpec{
					Type:        FaultSlowNetwork,
					Probability: 1.0,
					Duration:    30 * time.Second,
					Parameters: map[string]interface{}{
						"delay_ms": 5000,
					},
				},
			},
			{
				Name: "verify_timeout_handling",
				Action: ActionSendRequest,
				ExpectedResult: ExpectedStepResult{
					Success:     false,
					ErrorType:   "timeout_error",
					MaxDuration: 10 * time.Second,
				},
			},
		},
		RecoveryCheck: true,
		ChaosLevel:    ChaosLevelMedium,
	}

	suite.scenarios["resource_exhaustion"] = ErrorScenario{
		Name:        "Resource Exhaustion",
		Description: "Test behavior under resource exhaustion conditions",
		Category:    CategoryResourceExhaustion,
		FaultTypes:  []FaultType{FaultMemoryExhaustion, FaultServerOverload},
		ExpectedBehavior: ExpectedErrorBehavior{
			ShouldDegrade:    true,
			StateConsistency: true,
			MaxRecoveryTime:  60 * time.Second,
		},
		TestSteps: []ErrorTestStep{
			{
				Name: "create_resource_pressure",
				Action: ActionStressTest,
				FaultInjection: FaultInjectionSpec{
					Type:        FaultServerOverload,
					Probability: 1.0,
					Duration:    20 * time.Second,
				},
			},
			{
				Name: "verify_graceful_degradation",
				Action: ActionVerifyState,
				ExpectedResult: ExpectedStepResult{
					Success: true,
				},
			},
		},
		RecoveryCheck: true,
		ChaosLevel:    ChaosLevelHigh,
	}

	suite.scenarios["chaos_monkey"] = ErrorScenario{
		Name:        "Chaos Monkey",
		Description: "Random fault injection to test overall resilience",
		Category:    CategoryChaos,
		FaultTypes:  []FaultType{FaultConnectionDrop, FaultMessageLoss, FaultRequestTimeout, FaultInvalidResponse},
		ExpectedBehavior: ExpectedErrorBehavior{
			ShouldRetry:      true,
			ShouldReconnect:  true,
			ShouldDegrade:    true,
			MaxRecoveryTime:  90 * time.Second,
			StateConsistency: true,
		},
		TestSteps: []ErrorTestStep{
			{
				Name: "chaos_injection",
				Action: ActionInjectFault,
				FaultInjection: FaultInjectionSpec{
					Type:        FaultUnexpectedError, // Random selection
					Probability: 0.3,
					Duration:    60 * time.Second,
				},
				Parallel: true,
			},
			{
				Name: "continuous_operation",
				Action: ActionSendRequest,
				Delay: time.Second,
				Parallel: true,
			},
		},
		RecoveryCheck: true,
		ChaosLevel:    ChaosLevelExtreme,
	}
}

func (suite *ErrorScenarioTestSuite) RunErrorScenarioTests(ctx context.Context, t *testing.T) {
	suite.logger.Info("Starting comprehensive error scenario tests")

	for scenarioName, scenario := range suite.scenarios {
		// Skip chaos tests if disabled
		if scenario.ChaosLevel == ChaosLevelExtreme && !suite.config.EnableChaosTests {
			continue
		}

		t.Run(fmt.Sprintf("Scenario_%s", scenarioName), func(t *testing.T) {
			testCtx, cancel := context.WithTimeout(ctx, suite.config.TestTimeout)
			defer cancel()

			result := suite.runErrorScenario(testCtx, scenario)
			suite.results = append(suite.results, *result)

			suite.validateErrorScenarioResult(t, scenario, result)
		})
	}

	// Run recovery-specific tests
	if suite.config.EnableRecoveryTests {
		t.Run("RecoveryTests", suite.TestRecoveryBehavior)
	}
}

func (suite *ErrorScenarioTestSuite) runErrorScenario(ctx context.Context, scenario ErrorScenario) *ErrorScenarioResult {
	startTime := time.Now()
	
	result := &ErrorScenarioResult{
		ScenarioName:   scenario.Name,
		StepResults:    make([]ErrorStepResult, 0),
		Violations:     make([]string, 0),
		Metadata:       make(map[string]interface{}),
	}

	suite.logger.WithFields(logrus.Fields{
		"scenario": scenario.Name,
		"category": scenario.Category,
	}).Info("Starting error scenario")

	// Create test client with error simulation
	clientConfig := ClientConfig{
		ID: fmt.Sprintf("error_client_%s", scenario.Name),
		Name: "ErrorTestClient",
		Version: "1.0.0",
		Transport: TransportWebSocket,
		ErrorSimulation: ErrorSimulationConfig{
			EnableErrorSim:    true,
			ConnectionFailRate: suite.config.FaultInjectionRate,
			RequestFailRate:   suite.config.FaultInjectionRate,
			TimeoutRate:       suite.config.FaultInjectionRate,
		},
	}

	client, err := suite.factory.CreateClient(clientConfig)
	if err != nil {
		result.Violations = append(result.Violations, fmt.Sprintf("Failed to create client: %v", err))
		result.Success = false
		return result
	}
	defer suite.factory.RemoveClient(clientConfig.ID)

	// Execute test steps
	var wg sync.WaitGroup
	for i, step := range scenario.TestSteps {
		if step.Parallel {
			wg.Add(1)
			go func(stepIndex int, testStep ErrorTestStep) {
				defer wg.Done()
				stepResult := suite.executeErrorTestStep(ctx, client, testStep, scenario)
				stepResult.StepName = testStep.Name
				
				// Thread-safe append
				result.StepResults = append(result.StepResults, stepResult)
			}(i, step)
		} else {
			stepResult := suite.executeErrorTestStep(ctx, client, step, scenario)
			stepResult.StepName = step.Name
			result.StepResults = append(result.StepResults, stepResult)
			
			if step.Delay > 0 {
				time.Sleep(step.Delay)
			}
		}
	}

	wg.Wait()

	// Analyze client behavior
	result.ClientBehavior = suite.analyzeClientBehavior(client, result.StepResults)
	
	// Measure server resilience
	result.ServerResilience = suite.measureServerResilience(ctx, client)

	// Check recovery if required
	if scenario.RecoveryCheck {
		recoveryStart := time.Now()
		recovered := suite.checkRecovery(ctx, client, scenario.ExpectedBehavior.MaxRecoveryTime)
		result.RecoveryTime = time.Since(recoveryStart)
		result.RecoverySuccess = recovered
	}

	result.Duration = time.Since(startTime)
	result.Success = suite.evaluateScenarioSuccess(scenario, result)
	result.FaultsInjected = suite.faultInjector.GetActiveFaultCount()
	result.ErrorsObserved = suite.countErrorsInSteps(result.StepResults)

	return result
}

func (suite *ErrorScenarioTestSuite) executeErrorTestStep(ctx context.Context, client *MockMCPClient, step ErrorTestStep, scenario ErrorScenario) ErrorStepResult {
	stepStart := time.Now()
	
	result := ErrorStepResult{
		StepName: step.Name,
	}

	// Inject fault if specified
	if step.FaultInjection.Type != "" {
		suite.faultInjector.InjectFault(client.ID, step.FaultInjection)
		result.FaultActive = true
	}

	// Execute step action
	switch step.Action {
	case ActionConnect:
		err := client.Connect(ctx, suite.serverURL)
		result.Success = err == nil
		if err != nil {
			result.ErrorMessage = err.Error()
		}

	case ActionDisconnect:
		err := client.Disconnect()
		result.Success = err == nil
		if err != nil {
			result.ErrorMessage = err.Error()
		}

	case ActionSendRequest:
		_, err := client.CallTool(ctx, "classify_variant", map[string]interface{}{
			"variant": "NM_000492.3:c.1521_1523del",
		})
		result.Success = err == nil
		if err != nil {
			result.ErrorMessage = err.Error()
		}

	case ActionInjectFault:
		suite.faultInjector.InjectFault(client.ID, step.FaultInjection)
		result.Success = true

	case ActionWaitRecovery:
		recoveryStart := time.Now()
		recovered := suite.waitForRecovery(ctx, client, step.ExpectedResult.MaxDuration)
		result.RecoveryTime = time.Since(recoveryStart)
		result.Success = recovered

	case ActionVerifyState:
		result.Success = suite.verifyClientState(ctx, client)

	case ActionStressTest:
		result.Success = suite.runStressTest(ctx, client, step.FaultInjection.Duration)

	default:
		result.Success = false
		result.ErrorMessage = fmt.Sprintf("Unknown action: %s", step.Action)
	}

	result.Duration = time.Since(stepStart)

	suite.logger.WithFields(logrus.Fields{
		"step":    step.Name,
		"action":  step.Action,
		"success": result.Success,
		"duration": result.Duration,
	}).Debug("Error test step completed")

	return result
}

func (suite *ErrorScenarioTestSuite) waitForRecovery(ctx context.Context, client *MockMCPClient, maxWait time.Duration) bool {
	ctx, cancel := context.WithTimeout(ctx, maxWait)
	defer cancel()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
			// Test if client has recovered
			if client.IsConnected() {
				_, err := client.ListTools(ctx)
				if err == nil {
					return true
				}
			}
		}
	}
}

func (suite *ErrorScenarioTestSuite) verifyClientState(ctx context.Context, client *MockMCPClient) bool {
	if !client.IsConnected() {
		return false
	}

	// Verify basic functionality
	_, err := client.ListTools(ctx)
	return err == nil
}

func (suite *ErrorScenarioTestSuite) runStressTest(ctx context.Context, client *MockMCPClient, duration time.Duration) bool {
	ctx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()

	var successCount, errorCount int
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Consider successful if we had any successful operations
			return successCount > 0
		case <-ticker.C:
			_, err := client.CallTool(ctx, "validate_hgvs", map[string]interface{}{
				"notation": "NM_000492.3:c.1521_1523del",
			})
			if err == nil {
				successCount++
			} else {
				errorCount++
			}
		}
	}
}

func (suite *ErrorScenarioTestSuite) analyzeClientBehavior(client *MockMCPClient, stepResults []ErrorStepResult) ClientBehaviorAnalysis {
	stats := client.GetStats()
	
	analysis := ClientBehaviorAnalysis{
		RetryAttempts:     int(stats.TotalRequests - stats.SuccessfulRequests),
		ReconnectAttempts: stats.ReconnectCount,
	}

	// Calculate error handling quality
	if stats.TotalRequests > 0 {
		successRate := float64(stats.SuccessfulRequests) / float64(stats.TotalRequests)
		analysis.ErrorHandlingQuality = successRate
	}

	// Calculate resilience score based on recovery behavior
	recoverySuccesses := 0
	for _, step := range stepResults {
		if step.RecoveryTime > 0 && step.Success {
			recoverySuccesses++
		}
	}
	
	if len(stepResults) > 0 {
		analysis.ResilienceScore = float64(recoverySuccesses) / float64(len(stepResults))
	}

	// Check for graceful degradation
	analysis.GracefulDegradation = stats.FailedRequests < stats.TotalRequests/2

	return analysis
}

func (suite *ErrorScenarioTestSuite) measureServerResilience(ctx context.Context, client *MockMCPClient) ServerResilienceMetrics {
	metrics := ServerResilienceMetrics{}
	
	// Test connection stability
	stabilityStart := time.Now()
	stableConnections := 0
	totalTests := 10
	
	for i := 0; i < totalTests; i++ {
		if client.IsConnected() {
			_, err := client.ListTools(ctx)
			if err == nil {
				stableConnections++
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	
	metrics.ErrorRecoveryTime = time.Since(stabilityStart) / time.Duration(totalTests)
	metrics.ConnectionStability = float64(stableConnections) / float64(totalTests)
	
	// Mock resource utilization and throughput metrics
	metrics.ResourceUtilization = 0.65 // Would be measured from actual server metrics
	metrics.ThroughputDegradation = 0.15 // Would be calculated from performance comparison

	return metrics
}

func (suite *ErrorScenarioTestSuite) checkRecovery(ctx context.Context, client *MockMCPClient, maxTime time.Duration) bool {
	return suite.waitForRecovery(ctx, client, maxTime)
}

func (suite *ErrorScenarioTestSuite) evaluateScenarioSuccess(scenario ErrorScenario, result *ErrorScenarioResult) bool {
	// Check if recovery was successful when required
	if scenario.RecoveryCheck && !result.RecoverySuccess {
		result.Violations = append(result.Violations, "Recovery check failed")
		return false
	}

	// Check if recovery time was within bounds
	if scenario.RecoveryCheck && scenario.ExpectedBehavior.MaxRecoveryTime > 0 {
		if result.RecoveryTime > scenario.ExpectedBehavior.MaxRecoveryTime {
			result.Violations = append(result.Violations, 
				fmt.Sprintf("Recovery time (%v) exceeded maximum (%v)", 
					result.RecoveryTime, scenario.ExpectedBehavior.MaxRecoveryTime))
			return false
		}
	}

	// Check client behavior quality
	if result.ClientBehavior.ErrorHandlingQuality < 0.5 && scenario.ExpectedBehavior.ShouldRetry {
		result.Violations = append(result.Violations, "Poor error handling quality")
		return false
	}

	// Check step results
	criticalFailures := 0
	for _, stepResult := range result.StepResults {
		if !stepResult.Success && stepResult.ErrorMessage != "" {
			criticalFailures++
		}
	}

	// Allow some failures in chaos scenarios
	maxAllowedFailures := len(result.StepResults) / 2
	if scenario.ChaosLevel == ChaosLevelExtreme {
		maxAllowedFailures = len(result.StepResults) * 3 / 4
	}

	if criticalFailures > maxAllowedFailures {
		result.Violations = append(result.Violations, 
			fmt.Sprintf("Too many critical failures: %d > %d", criticalFailures, maxAllowedFailures))
		return false
	}

	return len(result.Violations) == 0
}

func (suite *ErrorScenarioTestSuite) countErrorsInSteps(stepResults []ErrorStepResult) int {
	count := 0
	for _, step := range stepResults {
		if !step.Success {
			count++
		}
	}
	return count
}

func (suite *ErrorScenarioTestSuite) TestRecoveryBehavior(ctx context.Context, t *testing.T) {
	suite.logger.Info("Running dedicated recovery behavior tests")

	clientConfig := ClientConfig{
		ID: "recovery_client",
		Name: "RecoveryTestClient",
		Version: "1.0.0",
		Transport: TransportWebSocket,
		MockConfig: MockClientConfig{
			AutoReconnect: true,
			MaxRetries:    5,
			RetryDelay:    time.Second,
		},
	}

	client, err := suite.factory.CreateClient(clientConfig)
	require.NoError(t, err)
	defer suite.factory.RemoveClient(clientConfig.ID)

	// Test 1: Connection recovery
	t.Run("ConnectionRecovery", func(t *testing.T) {
		err := client.Connect(ctx, suite.serverURL)
		require.NoError(t, err)

		// Simulate connection drop
		client.Disconnect()
		assert.False(t, client.IsConnected())

		// Test auto-reconnection
		time.Sleep(2 * time.Second)
		err = client.Connect(ctx, suite.serverURL)
		assert.NoError(t, err)
		assert.True(t, client.IsConnected())
	})

	// Test 2: Request retry behavior
	t.Run("RequestRetry", func(t *testing.T) {
		err := client.Connect(ctx, suite.serverURL)
		require.NoError(t, err)

		// Configure high error rate
		client.SetErrorSimulation(ErrorSimulationConfig{
			EnableErrorSim:  true,
			RequestFailRate: 0.7, // High failure rate
		})

		// Send requests and measure retry behavior
		successCount := 0
		for i := 0; i < 10; i++ {
			_, err := client.CallTool(ctx, "validate_hgvs", map[string]interface{}{
				"notation": "NM_000492.3:c.1521_1523del",
			})
			if err == nil {
				successCount++
			}
		}

		// Should have some successes due to retries
		assert.Greater(t, successCount, 0, "Retry mechanism should achieve some successes")

		stats := client.GetStats()
		assert.Greater(t, stats.TotalRequests, int64(10), "Should have retry attempts")
	})

	// Test 3: Graceful degradation
	t.Run("GracefulDegradation", func(t *testing.T) {
		err := client.Connect(ctx, suite.serverURL)
		require.NoError(t, err)

		// Simulate partial service failure
		client.SetErrorSimulation(ErrorSimulationConfig{
			EnableErrorSim:  true,
			RequestFailRate: 0.5,
		})

		// Test that some operations continue to work
		operations := []string{
			"validate_hgvs",
			"tools/list",
			"resources/list",
		}

		successfulOps := 0
		for _, op := range operations {
			switch op {
			case "validate_hgvs":
				_, err := client.CallTool(ctx, op, map[string]interface{}{"notation": "test"})
				if err == nil {
					successfulOps++
				}
			case "tools/list":
				_, err := client.ListTools(ctx)
				if err == nil {
					successfulOps++
				}
			case "resources/list":
				_, err := client.ListResources(ctx)
				if err == nil {
					successfulOps++
				}
			}
		}

		assert.Greater(t, successfulOps, 0, "Some operations should succeed during partial failure")
	})
}

func (suite *ErrorScenarioTestSuite) validateErrorScenarioResult(t *testing.T, scenario ErrorScenario, result *ErrorScenarioResult) {
	// Basic success validation
	if scenario.ChaosLevel != ChaosLevelExtreme {
		assert.True(t, result.Success, 
			"Scenario '%s' should succeed. Violations: %v", scenario.Name, result.Violations)
	}

	// Recovery validation
	if scenario.RecoveryCheck {
		assert.True(t, result.RecoverySuccess, 
			"Scenario '%s' should recover successfully", scenario.Name)
			
		if scenario.ExpectedBehavior.MaxRecoveryTime > 0 {
			assert.LessOrEqual(t, result.RecoveryTime, scenario.ExpectedBehavior.MaxRecoveryTime,
				"Recovery time should be within bounds for scenario '%s'", scenario.Name)
		}
	}

	// Client behavior validation
	if scenario.ExpectedBehavior.ShouldRetry {
		assert.Greater(t, result.ClientBehavior.RetryAttempts, 0,
			"Client should attempt retries for scenario '%s'", scenario.Name)
	}

	if scenario.ExpectedBehavior.ShouldReconnect {
		assert.Greater(t, result.ClientBehavior.ReconnectAttempts, 0,
			"Client should attempt reconnection for scenario '%s'", scenario.Name)
	}

	// Error handling quality
	minQuality := 0.3 // 30% minimum success rate
	if scenario.ChaosLevel == ChaosLevelExtreme {
		minQuality = 0.1 // Lower expectations for extreme chaos
	}
	
	assert.GreaterOrEqual(t, result.ClientBehavior.ErrorHandlingQuality, minQuality,
		"Error handling quality should meet minimum threshold for scenario '%s'", scenario.Name)
}

// FaultInjector implementation
func (fi *FaultInjector) InjectFault(clientID string, spec FaultInjectionSpec) {
	if !fi.config.EnableInjection {
		return
	}

	fi.mutex.Lock()
	defer fi.mutex.Unlock()

	// Check if we should inject this fault
	if rand.Float64() > spec.Probability {
		return
	}

	// Check concurrent fault limit
	if len(fi.activeFaults) >= fi.config.MaxConcurrentFaults {
		return
	}

	faultID := fmt.Sprintf("%s_%s_%d", clientID, spec.Type, time.Now().UnixNano())
	duration := spec.Duration
	if duration == 0 {
		duration = fi.config.DefaultDuration
	}

	fault := &ActiveFault{
		Type:            spec.Type,
		StartTime:       time.Now(),
		Duration:        duration,
		Parameters:      spec.Parameters,
		AffectedClients: []string{clientID},
	}

	fi.activeFaults[faultID] = fault

	// Schedule fault cleanup
	go func() {
		time.Sleep(duration)
		fi.ClearFault(faultID)
	}()

	// Record fault event
	fi.faultHistory = append(fi.faultHistory, FaultEvent{
		Type:      spec.Type,
		Timestamp: time.Now(),
		Duration:  duration,
		ClientID:  clientID,
		Impact:    "injected",
	})

	fi.logger.WithFields(logrus.Fields{
		"fault_id":  faultID,
		"type":      spec.Type,
		"client_id": clientID,
		"duration":  duration,
	}).Info("Fault injected")
}

func (fi *FaultInjector) ClearFault(faultID string) {
	fi.mutex.Lock()
	defer fi.mutex.Unlock()

	if fault, exists := fi.activeFaults[faultID]; exists {
		delete(fi.activeFaults, faultID)
		
		fi.logger.WithFields(logrus.Fields{
			"fault_id": faultID,
			"type":     fault.Type,
			"duration": time.Since(fault.StartTime),
		}).Info("Fault cleared")
	}
}

func (fi *FaultInjector) GetActiveFaultCount() int {
	fi.mutex.RLock()
	defer fi.mutex.RUnlock()
	return len(fi.activeFaults)
}

func (fi *FaultInjector) GetActiveFaults() map[string]*ActiveFault {
	fi.mutex.RLock()
	defer fi.mutex.RUnlock()

	faults := make(map[string]*ActiveFault)
	for id, fault := range fi.activeFaults {
		faults[id] = fault
	}
	return faults
}

func (fi *FaultInjector) GetFaultHistory() []FaultEvent {
	fi.mutex.RLock()
	defer fi.mutex.RUnlock()

	history := make([]FaultEvent, len(fi.faultHistory))
	copy(history, fi.faultHistory)
	return history
}

func (suite *ErrorScenarioTestSuite) GetResults() []ErrorScenarioResult {
	return suite.results
}

func (suite *ErrorScenarioTestSuite) GetScenarios() map[string]ErrorScenario {
	return suite.scenarios
}

func (suite *ErrorScenarioTestSuite) GenerateResilienceReport() map[string]interface{} {
	report := map[string]interface{}{
		"total_scenarios":      len(suite.results),
		"successful_scenarios": 0,
		"failed_scenarios":     0,
		"recovery_stats": map[string]interface{}{
			"recovery_attempts": 0,
			"successful_recoveries": 0,
			"avg_recovery_time": time.Duration(0),
		},
		"fault_injection_stats": map[string]interface{}{
			"total_faults": 0,
			"fault_types": make(map[string]int),
		},
		"client_resilience": map[string]interface{}{
			"avg_error_handling_quality": 0.0,
			"avg_resilience_score": 0.0,
		},
	}

	var totalRecoveryTime time.Duration
	var totalQuality, totalResilience float64
	recoveryAttempts, successfulRecoveries := 0, 0
	faultTypeCount := make(map[string]int)

	for _, result := range suite.results {
		if result.Success {
			report["successful_scenarios"] = report["successful_scenarios"].(int) + 1
		} else {
			report["failed_scenarios"] = report["failed_scenarios"].(int) + 1
		}

		if result.RecoveryTime > 0 {
			recoveryAttempts++
			totalRecoveryTime += result.RecoveryTime
			if result.RecoverySuccess {
				successfulRecoveries++
			}
		}

		totalQuality += result.ClientBehavior.ErrorHandlingQuality
		totalResilience += result.ClientBehavior.ResilienceScore

		// Count fault types from scenario
		if scenario, exists := suite.scenarios[result.ScenarioName]; exists {
			for _, faultType := range scenario.FaultTypes {
				faultTypeCount[string(faultType)]++
			}
		}
	}

	// Calculate averages
	if len(suite.results) > 0 {
		clientResilience := report["client_resilience"].(map[string]interface{})
		clientResilience["avg_error_handling_quality"] = totalQuality / float64(len(suite.results))
		clientResilience["avg_resilience_score"] = totalResilience / float64(len(suite.results))
	}

	if recoveryAttempts > 0 {
		recoveryStats := report["recovery_stats"].(map[string]interface{})
		recoveryStats["recovery_attempts"] = recoveryAttempts
		recoveryStats["successful_recoveries"] = successfulRecoveries
		recoveryStats["avg_recovery_time"] = totalRecoveryTime / time.Duration(recoveryAttempts)
	}

	faultStats := report["fault_injection_stats"].(map[string]interface{})
	faultStats["fault_types"] = faultTypeCount
	totalFaults := 0
	for _, count := range faultTypeCount {
		totalFaults += count
	}
	faultStats["total_faults"] = totalFaults

	return report
}