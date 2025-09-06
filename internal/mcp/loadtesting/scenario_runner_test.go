package loadtesting

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewScenarioRunner(t *testing.T) {
	config := LoadTestConfig{}
	runner := NewScenarioRunner(config)

	assert.NotNil(t, runner)
	assert.Equal(t, 10, runner.config.VirtualAgents)
	assert.Equal(t, 5*time.Minute, runner.config.Duration)
	assert.Equal(t, 30*time.Second, runner.config.RampUpTime)
	assert.Equal(t, 1*time.Second, runner.config.ThinkTime)
}

func TestAddScenario(t *testing.T) {
	runner := NewScenarioRunner(LoadTestConfig{})

	scenario := Scenario{
		Name:        "test_scenario",
		Description: "Test scenario",
		Weight:      50,
		Execute: func(ctx context.Context, agent *AIAgent) error {
			return nil
		},
	}

	runner.AddScenario(scenario)
	assert.Len(t, runner.scenarios, 1)
	assert.Equal(t, "test_scenario", runner.scenarios[0].Name)
}

func TestRunLoadTest(t *testing.T) {
	config := LoadTestConfig{
		VirtualAgents:   2,
		Duration:        100 * time.Millisecond,
		RampUpTime:      10 * time.Millisecond,
		RampDownTime:    10 * time.Millisecond,
		ThinkTime:       5 * time.Millisecond,
		EnableMetrics:   true,
		MetricsInterval: 20 * time.Millisecond,
	}

	runner := NewScenarioRunner(config)

	// Add a simple test scenario
	executionCount := 0
	scenario := Scenario{
		Name:        "simple_test",
		Description: "Simple test scenario",
		Weight:      100,
		Execute: func(ctx context.Context, agent *AIAgent) error {
			executionCount++
			time.Sleep(1 * time.Millisecond)
			return nil
		},
	}

	runner.AddScenario(scenario)

	result, err := runner.RunLoadTest(context.Background(), "test_load")
	require.NoError(t, err)
	assert.NotNil(t, result)

	assert.Equal(t, "test_load", result.TestName)
	assert.Greater(t, result.TotalRequests, int64(0))
	assert.Equal(t, int64(0), result.TotalErrors)
	assert.Equal(t, 0.0, result.ErrorRate)
	assert.Greater(t, result.AverageThroughput, 0.0)
	assert.Contains(t, result.ScenarioStats, "simple_test")
	assert.Greater(t, executionCount, 0)

	// Check that we have agent stats
	assert.Len(t, result.AgentStats, 2) // 2 virtual agents

	// Check that we have timeline data (metrics enabled)
	assert.Greater(t, len(result.Timeline), 0)
}

func TestRunLoadTestWithErrors(t *testing.T) {
	config := LoadTestConfig{
		VirtualAgents: 1,
		Duration:      50 * time.Millisecond,
		RampUpTime:    5 * time.Millisecond,
		RampDownTime:  5 * time.Millisecond,
		ThinkTime:     0,
		EnableMetrics: false,
	}

	runner := NewScenarioRunner(config)

	// Add an error-prone scenario
	scenario := Scenario{
		Name:        "error_test",
		Description: "Error test scenario",
		Weight:      100,
		Execute: func(ctx context.Context, agent *AIAgent) error {
			return errors.New("test error")
		},
	}

	runner.AddScenario(scenario)

	result, err := runner.RunLoadTest(context.Background(), "error_test")
	require.NoError(t, err)

	assert.Greater(t, result.TotalErrors, int64(0))
	assert.Greater(t, result.ErrorRate, 0.0)
	
	// Check scenario stats
	scenarioStats := result.ScenarioStats["error_test"]
	assert.NotNil(t, scenarioStats)
	assert.Greater(t, scenarioStats.Errors, int64(0))
	assert.Greater(t, scenarioStats.ErrorRate, 0.0)
}

func TestSelectScenario(t *testing.T) {
	runner := NewScenarioRunner(LoadTestConfig{})

	// Add scenarios with different weights
	scenario1 := Scenario{Name: "scenario1", Weight: 30}
	scenario2 := Scenario{Name: "scenario2", Weight: 70}

	runner.AddScenario(scenario1)
	runner.AddScenario(scenario2)

	// Test selection multiple times
	selections := make(map[string]int)
	for i := 0; i < 1000; i++ {
		selected := runner.selectScenario()
		require.NotNil(t, selected)
		selections[selected.Name]++
	}

	// Scenario2 should be selected more often due to higher weight
	assert.Greater(t, selections["scenario2"], selections["scenario1"])
	
	// Both scenarios should be selected at least once
	assert.Greater(t, selections["scenario1"], 0)
	assert.Greater(t, selections["scenario2"], 0)
}

func TestSelectScenarioNoScenarios(t *testing.T) {
	runner := NewScenarioRunner(LoadTestConfig{})
	
	selected := runner.selectScenario()
	assert.Nil(t, selected)
}

func TestCreateAgents(t *testing.T) {
	config := LoadTestConfig{VirtualAgents: 5}
	runner := NewScenarioRunner(config)

	runner.createAgents()

	assert.Len(t, runner.agents, 5)
	
	for i := 0; i < 5; i++ {
		agentID := "agent_0000" + string(rune('0'+i))
		agent, exists := runner.agents[agentID]
		assert.True(t, exists)
		assert.Equal(t, agentID, agent.ID)
		assert.NotNil(t, agent.SessionData)
	}
}

func TestResponseTimeCalculation(t *testing.T) {
	runner := NewScenarioRunner(LoadTestConfig{})

	// Add some test response times
	runner.responseTimes = []time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
		30 * time.Millisecond,
		40 * time.Millisecond,
		50 * time.Millisecond,
	}

	result := &LoadTestResult{}
	runner.calculateResponseTimeStats(result)

	assert.Equal(t, 10*time.Millisecond, result.ResponseTimes.Min)
	assert.Equal(t, 50*time.Millisecond, result.ResponseTimes.Max)
	assert.Equal(t, 30*time.Millisecond, result.ResponseTimes.Mean)
	assert.Equal(t, 30*time.Millisecond, result.ResponseTimes.P50)
	assert.Equal(t, 50*time.Millisecond, result.ResponseTimes.P90)
}

func TestPercentileCalculation(t *testing.T) {
	runner := NewScenarioRunner(LoadTestConfig{})

	times := []time.Duration{
		1 * time.Millisecond,
		2 * time.Millisecond,
		3 * time.Millisecond,
		4 * time.Millisecond,
		5 * time.Millisecond,
	}

	p50 := runner.percentile(times, 0.50)
	p90 := runner.percentile(times, 0.90)
	p99 := runner.percentile(times, 0.99)

	assert.Equal(t, 3*time.Millisecond, p50)
	assert.Equal(t, 5*time.Millisecond, p90)
	assert.Equal(t, 5*time.Millisecond, p99)

	// Test empty slice
	emptyP50 := runner.percentile([]time.Duration{}, 0.50)
	assert.Equal(t, time.Duration(0), emptyP50)
}

func TestCreateMCPAgentScenarios(t *testing.T) {
	runner := NewScenarioRunner(LoadTestConfig{})

	runner.CreateMCPAgentScenarios()

	assert.Len(t, runner.scenarios, 5)

	expectedScenarios := []string{
		"variant_classification",
		"evidence_gathering",
		"resource_browsing",
		"batch_processing",
		"error_simulation",
	}

	for _, expected := range expectedScenarios {
		found := false
		for _, scenario := range runner.scenarios {
			if scenario.Name == expected {
				found = true
				assert.Greater(t, scenario.Weight, 0)
				assert.NotNil(t, scenario.Execute)
				break
			}
		}
		assert.True(t, found, "Scenario %s not found", expected)
	}
}

func TestCreateStressTestScenario(t *testing.T) {
	runner := NewScenarioRunner(LoadTestConfig{})

	runner.CreateStressTestScenario()

	assert.Len(t, runner.scenarios, 1)
	assert.Equal(t, "stress_test", runner.scenarios[0].Name)
	assert.Equal(t, 100, runner.scenarios[0].Weight)
	assert.NotNil(t, runner.scenarios[0].Execute)
}

func TestScenarioExecution(t *testing.T) {
	runner := NewScenarioRunner(LoadTestConfig{})

	// Test variant classification scenario
	runner.CreateMCPAgentScenarios()

	agent := &AIAgent{
		ID:          "test_agent",
		SessionData: make(map[string]interface{}),
	}

	// Find and execute variant classification scenario
	var variantScenario *Scenario
	for i := range runner.scenarios {
		if runner.scenarios[i].Name == "variant_classification" {
			variantScenario = &runner.scenarios[i]
			break
		}
	}
	require.NotNil(t, variantScenario)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := variantScenario.Execute(ctx, agent)
	assert.NoError(t, err)

	// Check that session data was updated
	expectedSteps := []string{
		"validate_hgvs",
		"query_evidence",
		"classify_variant",
		"generate_report",
	}

	for _, step := range expectedSteps {
		assert.Contains(t, agent.SessionData, step)
	}
}

func TestErrorSimulationScenario(t *testing.T) {
	runner := NewScenarioRunner(LoadTestConfig{})
	runner.CreateMCPAgentScenarios()

	agent := &AIAgent{
		ID:          "test_agent",
		SessionData: make(map[string]interface{}),
	}

	// Find error simulation scenario
	var errorScenario *Scenario
	for i := range runner.scenarios {
		if runner.scenarios[i].Name == "error_simulation" {
			errorScenario = &runner.scenarios[i]
			break
		}
	}
	require.NotNil(t, errorScenario)

	ctx := context.Background()

	// Execute multiple times to ensure we get an error
	errorOccurred := false
	for i := 0; i < 10; i++ {
		err := errorScenario.Execute(ctx, agent)
		if err != nil {
			errorOccurred = true
			assert.Contains(t, []string{
				"invalid_hgvs_notation",
				"database_timeout",
				"insufficient_evidence",
				"system_overload",
			}, err.Error())
			break
		}
	}
	assert.True(t, errorOccurred, "Error simulation should have produced at least one error")
}

func TestLoadTestWithContextCancellation(t *testing.T) {
	config := LoadTestConfig{
		VirtualAgents: 2,
		Duration:      1 * time.Second, // Long duration
		RampUpTime:    10 * time.Millisecond,
		ThinkTime:     10 * time.Millisecond,
	}

	runner := NewScenarioRunner(config)

	scenario := Scenario{
		Name:   "cancellation_test",
		Weight: 100,
		Execute: func(ctx context.Context, agent *AIAgent) error {
			select {
			case <-time.After(50 * time.Millisecond):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	}

	runner.AddScenario(scenario)

	// Cancel context early
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	result, err := runner.RunLoadTest(ctx, "cancellation_test")
	require.NoError(t, err)

	// Test should complete quickly due to context cancellation
	assert.Less(t, result.TotalDuration, time.Second)
	assert.Greater(t, result.TotalRequests, int64(0))
}

func TestSystemMetricsCollection(t *testing.T) {
	runner := NewScenarioRunner(LoadTestConfig{})

	metrics := runner.collectSystemMetrics()

	// Basic validation - actual values would depend on system state
	assert.GreaterOrEqual(t, metrics.CPUUsage, 0.0)
	assert.GreaterOrEqual(t, metrics.MemoryUsage, 0.0)
	assert.GreaterOrEqual(t, metrics.ActiveGoroutines, 0)
	assert.GreaterOrEqual(t, metrics.HeapSize, uint64(0))
	assert.GreaterOrEqual(t, metrics.GCPause, time.Duration(0))
}

func TestMetricsCollectionDisabled(t *testing.T) {
	config := LoadTestConfig{
		VirtualAgents: 1,
		Duration:      50 * time.Millisecond,
		EnableMetrics: false, // Disabled
	}

	runner := NewScenarioRunner(config)

	scenario := Scenario{
		Name:   "no_metrics_test",
		Weight: 100,
		Execute: func(ctx context.Context, agent *AIAgent) error {
			return nil
		},
	}

	runner.AddScenario(scenario)

	result, err := runner.RunLoadTest(context.Background(), "no_metrics_test")
	require.NoError(t, err)

	// Timeline should be empty when metrics collection is disabled
	assert.Empty(t, result.Timeline)
}

func TestAgentSessionData(t *testing.T) {
	runner := NewScenarioRunner(LoadTestConfig{})
	runner.CreateMCPAgentScenarios()

	agent := &AIAgent{
		ID:          "session_test_agent",
		SessionData: make(map[string]interface{}),
	}

	// Execute batch processing scenario
	var batchScenario *Scenario
	for i := range runner.scenarios {
		if runner.scenarios[i].Name == "batch_processing" {
			batchScenario = &runner.scenarios[i]
			break
		}
	}
	require.NotNil(t, batchScenario)

	ctx := context.Background()
	err := batchScenario.Execute(ctx, agent)
	assert.NoError(t, err)

	// Check that batch size was recorded in session data
	batchSize, exists := agent.SessionData["batch_size"]
	assert.True(t, exists)
	assert.IsType(t, 0, batchSize)
	assert.GreaterOrEqual(t, batchSize.(int), 5)
	assert.LessOrEqual(t, batchSize.(int), 15)
}