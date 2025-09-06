package loadtesting

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// LoadTestConfig defines configuration for load testing scenarios
type LoadTestConfig struct {
	// Number of virtual AI agents
	VirtualAgents int
	// Test duration
	Duration time.Duration
	// Ramp-up time to reach full load
	RampUpTime time.Duration
	// Ramp-down time to reduce load
	RampDownTime time.Duration
	// Think time between requests (per agent)
	ThinkTime time.Duration
	// Request timeout
	RequestTimeout time.Duration
	// Enable real-time metrics collection
	EnableMetrics bool
	// Metrics collection interval
	MetricsInterval time.Duration
	// Target throughput (requests per second)
	TargetThroughput float64
}

// Scenario represents a load testing scenario
type Scenario struct {
	Name        string                                        `json:"name"`
	Description string                                        `json:"description"`
	Weight      int                                           `json:"weight"`
	Execute     func(ctx context.Context, agent *AIAgent) error `json:"-"`
}

// AIAgent represents a virtual AI agent in the load test
type AIAgent struct {
	ID            string                 `json:"id"`
	StartTime     time.Time              `json:"start_time"`
	RequestCount  int64                  `json:"request_count"`
	ErrorCount    int64                  `json:"error_count"`
	LastActivity  time.Time              `json:"last_activity"`
	SessionData   map[string]interface{} `json:"session_data"`
	ActiveScenario string                `json:"active_scenario"`
}

// LoadTestResult contains the results of a load test execution
type LoadTestResult struct {
	TestName         string                   `json:"test_name"`
	TotalDuration    time.Duration            `json:"total_duration"`
	TotalRequests    int64                    `json:"total_requests"`
	TotalErrors      int64                    `json:"total_errors"`
	AverageThroughput float64                 `json:"average_throughput"`
	PeakThroughput   float64                  `json:"peak_throughput"`
	ErrorRate        float64                  `json:"error_rate"`
	ResponseTimes    ResponseTimeStats        `json:"response_times"`
	AgentStats       map[string]*AIAgent      `json:"agent_stats"`
	ScenarioStats    map[string]*ScenarioStats `json:"scenario_stats"`
	Timeline         []TimelinePoint          `json:"timeline"`
	SystemMetrics    SystemMetrics            `json:"system_metrics"`
}

// ResponseTimeStats contains response time statistics
type ResponseTimeStats struct {
	Min     time.Duration `json:"min"`
	Max     time.Duration `json:"max"`
	Mean    time.Duration `json:"mean"`
	P50     time.Duration `json:"p50"`
	P90     time.Duration `json:"p90"`
	P95     time.Duration `json:"p95"`
	P99     time.Duration `json:"p99"`
	StdDev  time.Duration `json:"std_dev"`
}

// ScenarioStats tracks statistics for individual scenarios
type ScenarioStats struct {
	Name          string        `json:"name"`
	Executions    int64         `json:"executions"`
	Errors        int64         `json:"errors"`
	TotalTime     time.Duration `json:"total_time"`
	AverageTime   time.Duration `json:"average_time"`
	ErrorRate     float64       `json:"error_rate"`
}

// TimelinePoint represents a point in time during the load test
type TimelinePoint struct {
	Timestamp    time.Time `json:"timestamp"`
	ActiveAgents int       `json:"active_agents"`
	Throughput   float64   `json:"throughput"`
	ErrorRate    float64   `json:"error_rate"`
	ResponseTime time.Duration `json:"response_time"`
}

// SystemMetrics tracks system-level metrics during load testing
type SystemMetrics struct {
	CPUUsage       float64 `json:"cpu_usage"`
	MemoryUsage    float64 `json:"memory_usage"`
	ActiveGoroutines int   `json:"active_goroutines"`
	HeapSize       uint64  `json:"heap_size"`
	GCPause        time.Duration `json:"gc_pause"`
}

// ScenarioRunner manages load testing scenarios with multiple virtual AI agents
type ScenarioRunner struct {
	config           LoadTestConfig
	scenarios        []Scenario
	agents           map[string]*AIAgent
	agentsMutex      sync.RWMutex
	metrics          *LoadTestMetrics
	stopCh           chan struct{}
	wg               sync.WaitGroup
	startTime        time.Time
	responseTimes    []time.Duration
	responseTimesMutex sync.RWMutex
}

// LoadTestMetrics tracks real-time metrics during load testing
type LoadTestMetrics struct {
	totalRequests   int64
	totalErrors     int64
	activeAgents    int64
	currentThroughput float64
	mutex           sync.RWMutex
}

// NewScenarioRunner creates a new load testing scenario runner
func NewScenarioRunner(config LoadTestConfig) *ScenarioRunner {
	if config.VirtualAgents == 0 {
		config.VirtualAgents = 10
	}
	if config.Duration == 0 {
		config.Duration = 5 * time.Minute
	}
	if config.RampUpTime == 0 {
		config.RampUpTime = 30 * time.Second
	}
	if config.RampDownTime == 0 {
		config.RampDownTime = 30 * time.Second
	}
	if config.ThinkTime == 0 {
		config.ThinkTime = 1 * time.Second
	}
	if config.RequestTimeout == 0 {
		config.RequestTimeout = 30 * time.Second
	}
	if config.MetricsInterval == 0 {
		config.MetricsInterval = 5 * time.Second
	}

	return &ScenarioRunner{
		config:        config,
		scenarios:     make([]Scenario, 0),
		agents:        make(map[string]*AIAgent),
		metrics:       &LoadTestMetrics{},
		stopCh:        make(chan struct{}),
		responseTimes: make([]time.Duration, 0),
	}
}

// AddScenario adds a load testing scenario
func (sr *ScenarioRunner) AddScenario(scenario Scenario) {
	sr.scenarios = append(sr.scenarios, scenario)
}

// RunLoadTest executes the load test with all configured scenarios
func (sr *ScenarioRunner) RunLoadTest(ctx context.Context, testName string) (*LoadTestResult, error) {
	sr.startTime = time.Now()
	result := &LoadTestResult{
		TestName:      testName,
		AgentStats:    make(map[string]*AIAgent),
		ScenarioStats: make(map[string]*ScenarioStats),
		Timeline:      make([]TimelinePoint, 0),
	}

	// Initialize scenario stats
	for _, scenario := range sr.scenarios {
		result.ScenarioStats[scenario.Name] = &ScenarioStats{
			Name: scenario.Name,
		}
	}

	// Start metrics collection if enabled
	if sr.config.EnableMetrics {
		sr.wg.Add(1)
		go sr.collectMetrics(ctx, result)
	}

	// Start load test execution
	sr.wg.Add(1)
	go sr.executeLoadTest(ctx, result)

	// Wait for completion or timeout
	testCtx, cancel := context.WithTimeout(ctx, sr.config.Duration+sr.config.RampUpTime+sr.config.RampDownTime+time.Minute)
	defer cancel()

	select {
	case <-testCtx.Done():
		close(sr.stopCh)
	case <-time.After(sr.config.Duration + sr.config.RampUpTime + sr.config.RampDownTime):
		close(sr.stopCh)
	}

	sr.wg.Wait()

	// Finalize results
	sr.finalizeResults(result)
	return result, nil
}

// CreateMCPAgentScenarios creates realistic MCP agent interaction scenarios
func (sr *ScenarioRunner) CreateMCPAgentScenarios() {
	// Scenario 1: Variant Classification Workflow
	sr.AddScenario(Scenario{
		Name:        "variant_classification",
		Description: "Complete variant classification workflow",
		Weight:      30,
		Execute: func(ctx context.Context, agent *AIAgent) error {
			// Simulate variant classification workflow
			steps := []string{
				"validate_hgvs",
				"query_evidence",
				"classify_variant",
				"generate_report",
			}

			for _, step := range steps {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
					// Simulate request processing time
					delay := time.Duration(rand.Intn(500)+100) * time.Millisecond
					time.Sleep(delay)
					agent.SessionData[step] = time.Now()
				}
			}
			return nil
		},
	})

	// Scenario 2: Evidence Gathering
	sr.AddScenario(Scenario{
		Name:        "evidence_gathering",
		Description: "Extensive evidence gathering from multiple sources",
		Weight:      25,
		Execute: func(ctx context.Context, agent *AIAgent) error {
			databases := []string{"clinvar", "gnomad", "cosmic", "hgmd"}
			
			for _, db := range databases {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
					// Simulate database query time
					delay := time.Duration(rand.Intn(800)+200) * time.Millisecond
					time.Sleep(delay)
					agent.SessionData[fmt.Sprintf("query_%s", db)] = time.Now()
				}
			}
			return nil
		},
	})

	// Scenario 3: Resource Browsing
	sr.AddScenario(Scenario{
		Name:        "resource_browsing",
		Description: "Browse various MCP resources",
		Weight:      20,
		Execute: func(ctx context.Context, agent *AIAgent) error {
			resources := []string{
				"variant/NM_000492.3:c.1521_1523delCTT",
				"interpretation/int_12345",
				"evidence/evidence_67890",
				"acmg/rules",
			}

			for _, resource := range resources {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
					// Simulate resource access time
					delay := time.Duration(rand.Intn(300)+50) * time.Millisecond
					time.Sleep(delay)
					agent.SessionData[fmt.Sprintf("access_%s", resource)] = time.Now()
				}
			}
			return nil
		},
	})

	// Scenario 4: Batch Processing
	sr.AddScenario(Scenario{
		Name:        "batch_processing",
		Description: "Process multiple variants in batch",
		Weight:      15,
		Execute: func(ctx context.Context, agent *AIAgent) error {
			batchSize := rand.Intn(10) + 5 // 5-15 variants
			
			for i := 0; i < batchSize; i++ {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
					// Simulate batch item processing
					delay := time.Duration(rand.Intn(400)+100) * time.Millisecond
					time.Sleep(delay)
				}
			}
			
			agent.SessionData["batch_size"] = batchSize
			return nil
		},
	})

	// Scenario 5: Error Simulation
	sr.AddScenario(Scenario{
		Name:        "error_simulation",
		Description: "Simulate various error conditions",
		Weight:      10,
		Execute: func(ctx context.Context, agent *AIAgent) error {
			// Randomly simulate different types of errors
			errorType := rand.Intn(4)
			
			switch errorType {
			case 0:
				time.Sleep(50 * time.Millisecond)
				return fmt.Errorf("invalid_hgvs_notation")
			case 1:
				time.Sleep(100 * time.Millisecond)
				return fmt.Errorf("database_timeout")
			case 2:
				time.Sleep(30 * time.Millisecond)
				return fmt.Errorf("insufficient_evidence")
			default:
				time.Sleep(200 * time.Millisecond)
				return fmt.Errorf("system_overload")
			}
		},
	})
}

// CreateStressTestScenario creates a high-intensity stress testing scenario
func (sr *ScenarioRunner) CreateStressTestScenario() {
	sr.AddScenario(Scenario{
		Name:        "stress_test",
		Description: "High-intensity stress test with rapid requests",
		Weight:      100,
		Execute: func(ctx context.Context, agent *AIAgent) error {
			// Rapid-fire requests with minimal think time
			for i := 0; i < rand.Intn(20)+10; i++ {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
					// Very short processing time
					time.Sleep(time.Duration(rand.Intn(50)+10) * time.Millisecond)
				}
			}
			return nil
		},
	})
}

// Private helper methods

func (sr *ScenarioRunner) executeLoadTest(ctx context.Context, result *LoadTestResult) {
	defer sr.wg.Done()

	// Create and start virtual agents
	sr.createAgents()
	
	// Ramp-up phase
	sr.rampUpAgents(ctx, result)
	
	// Steady state phase
	sr.steadyStateExecution(ctx, result)
	
	// Ramp-down phase
	sr.rampDownAgents(ctx, result)
}

func (sr *ScenarioRunner) createAgents() {
	sr.agentsMutex.Lock()
	defer sr.agentsMutex.Unlock()

	for i := 0; i < sr.config.VirtualAgents; i++ {
		agentID := fmt.Sprintf("agent_%04d", i)
		agent := &AIAgent{
			ID:          agentID,
			StartTime:   sr.startTime,
			SessionData: make(map[string]interface{}),
		}
		sr.agents[agentID] = agent
	}
}

func (sr *ScenarioRunner) rampUpAgents(ctx context.Context, result *LoadTestResult) {
	rampUpInterval := sr.config.RampUpTime / time.Duration(sr.config.VirtualAgents)
	agentIndex := 0

	sr.agentsMutex.RLock()
	agents := make([]*AIAgent, 0, len(sr.agents))
	for _, agent := range sr.agents {
		agents = append(agents, agent)
	}
	sr.agentsMutex.RUnlock()

	for _, agent := range agents {
		select {
		case <-ctx.Done():
			return
		case <-sr.stopCh:
			return
		default:
			sr.wg.Add(1)
			go sr.runAgent(ctx, agent, result)
			atomic.AddInt64(&sr.metrics.activeAgents, 1)
			agentIndex++
			
			if agentIndex < sr.config.VirtualAgents {
				time.Sleep(rampUpInterval)
			}
		}
	}
}

func (sr *ScenarioRunner) steadyStateExecution(ctx context.Context, result *LoadTestResult) {
	steadyDuration := sr.config.Duration - sr.config.RampUpTime - sr.config.RampDownTime
	if steadyDuration > 0 {
		time.Sleep(steadyDuration)
	}
}

func (sr *ScenarioRunner) rampDownAgents(ctx context.Context, result *LoadTestResult) {
	// Gradual shutdown is handled by context cancellation
	// Individual agents will stop naturally
}

func (sr *ScenarioRunner) runAgent(ctx context.Context, agent *AIAgent, result *LoadTestResult) {
	defer sr.wg.Done()
	defer atomic.AddInt64(&sr.metrics.activeAgents, -1)

	for {
		select {
		case <-ctx.Done():
			return
		case <-sr.stopCh:
			return
		default:
			// Select a scenario based on weight
			scenario := sr.selectScenario()
			if scenario == nil {
				continue
			}

			// Execute scenario
			startTime := time.Now()
			agent.ActiveScenario = scenario.Name
			agent.LastActivity = startTime
			
			err := scenario.Execute(ctx, agent)
			
			executionTime := time.Since(startTime)
			atomic.AddInt64(&agent.RequestCount, 1)
			atomic.AddInt64(&sr.metrics.totalRequests, 1)

			// Record response time
			sr.responseTimesMutex.Lock()
			sr.responseTimes = append(sr.responseTimes, executionTime)
			sr.responseTimesMutex.Unlock()

			// Update scenario stats
			if stats, exists := result.ScenarioStats[scenario.Name]; exists {
				atomic.AddInt64(&stats.Executions, 1)
				// Note: This is a simplified update; in production, use atomic operations or mutex
				stats.TotalTime += executionTime
				stats.AverageTime = stats.TotalTime / time.Duration(stats.Executions)
			}

			if err != nil {
				atomic.AddInt64(&agent.ErrorCount, 1)
				atomic.AddInt64(&sr.metrics.totalErrors, 1)
				
				if stats, exists := result.ScenarioStats[scenario.Name]; exists {
					atomic.AddInt64(&stats.Errors, 1)
					stats.ErrorRate = float64(stats.Errors) / float64(stats.Executions)
				}
			}

			// Think time
			if sr.config.ThinkTime > 0 {
				thinkTime := sr.config.ThinkTime + time.Duration(rand.Intn(int(sr.config.ThinkTime.Milliseconds())))*time.Millisecond
				time.Sleep(thinkTime)
			}
		}
	}
}

func (sr *ScenarioRunner) selectScenario() *Scenario {
	if len(sr.scenarios) == 0 {
		return nil
	}

	// Calculate total weight
	totalWeight := 0
	for _, scenario := range sr.scenarios {
		totalWeight += scenario.Weight
	}

	// Random selection based on weight
	random := rand.Intn(totalWeight)
	currentWeight := 0

	for i := range sr.scenarios {
		currentWeight += sr.scenarios[i].Weight
		if random < currentWeight {
			return &sr.scenarios[i]
		}
	}

	return &sr.scenarios[0] // Fallback
}

func (sr *ScenarioRunner) collectMetrics(ctx context.Context, result *LoadTestResult) {
	defer sr.wg.Done()

	ticker := time.NewTicker(sr.config.MetricsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-sr.stopCh:
			return
		case <-ticker.C:
			// Collect current metrics
			point := TimelinePoint{
				Timestamp:    time.Now(),
				ActiveAgents: int(atomic.LoadInt64(&sr.metrics.activeAgents)),
			}

			// Calculate current throughput
			totalRequests := atomic.LoadInt64(&sr.metrics.totalRequests)
			totalErrors := atomic.LoadInt64(&sr.metrics.totalErrors)
			
			if len(result.Timeline) > 0 {
				prevPoint := result.Timeline[len(result.Timeline)-1]
				timeDiff := point.Timestamp.Sub(prevPoint.Timestamp).Seconds()
				if timeDiff > 0 {
					requestDiff := float64(totalRequests - int64(prevPoint.Throughput*timeDiff))
					point.Throughput = requestDiff / timeDiff
				}
			}

			if totalRequests > 0 {
				point.ErrorRate = float64(totalErrors) / float64(totalRequests)
			}

			// Calculate current average response time
			sr.responseTimesMutex.RLock()
			if len(sr.responseTimes) > 0 {
				var sum time.Duration
				recentResponses := sr.responseTimes
				if len(recentResponses) > 100 {
					recentResponses = recentResponses[len(recentResponses)-100:]
				}
				for _, rt := range recentResponses {
					sum += rt
				}
				point.ResponseTime = sum / time.Duration(len(recentResponses))
			}
			sr.responseTimesMutex.RUnlock()

			result.Timeline = append(result.Timeline, point)
		}
	}
}

func (sr *ScenarioRunner) finalizeResults(result *LoadTestResult) {
	result.TotalDuration = time.Since(sr.startTime)
	result.TotalRequests = atomic.LoadInt64(&sr.metrics.totalRequests)
	result.TotalErrors = atomic.LoadInt64(&sr.metrics.totalErrors)

	if result.TotalRequests > 0 {
		result.ErrorRate = float64(result.TotalErrors) / float64(result.TotalRequests)
		result.AverageThroughput = float64(result.TotalRequests) / result.TotalDuration.Seconds()
	}

	// Calculate peak throughput from timeline
	for _, point := range result.Timeline {
		if point.Throughput > result.PeakThroughput {
			result.PeakThroughput = point.Throughput
		}
	}

	// Copy agent stats
	sr.agentsMutex.RLock()
	for id, agent := range sr.agents {
		result.AgentStats[id] = agent
	}
	sr.agentsMutex.RUnlock()

	// Calculate response time statistics
	sr.calculateResponseTimeStats(result)

	// Collect final system metrics
	result.SystemMetrics = sr.collectSystemMetrics()
}

func (sr *ScenarioRunner) calculateResponseTimeStats(result *LoadTestResult) {
	sr.responseTimesMutex.RLock()
	defer sr.responseTimesMutex.RUnlock()

	if len(sr.responseTimes) == 0 {
		return
	}

	// Sort response times for percentile calculation
	sortedTimes := make([]time.Duration, len(sr.responseTimes))
	copy(sortedTimes, sr.responseTimes)
	
	// Simple bubble sort for small datasets
	for i := 0; i < len(sortedTimes); i++ {
		for j := i + 1; j < len(sortedTimes); j++ {
			if sortedTimes[i] > sortedTimes[j] {
				sortedTimes[i], sortedTimes[j] = sortedTimes[j], sortedTimes[i]
			}
		}
	}

	result.ResponseTimes.Min = sortedTimes[0]
	result.ResponseTimes.Max = sortedTimes[len(sortedTimes)-1]
	
	// Calculate mean
	var sum time.Duration
	for _, rt := range sortedTimes {
		sum += rt
	}
	result.ResponseTimes.Mean = sum / time.Duration(len(sortedTimes))

	// Calculate percentiles
	result.ResponseTimes.P50 = sr.percentile(sortedTimes, 0.50)
	result.ResponseTimes.P90 = sr.percentile(sortedTimes, 0.90)
	result.ResponseTimes.P95 = sr.percentile(sortedTimes, 0.95)
	result.ResponseTimes.P99 = sr.percentile(sortedTimes, 0.99)
}

func (sr *ScenarioRunner) percentile(sortedTimes []time.Duration, p float64) time.Duration {
	if len(sortedTimes) == 0 {
		return 0
	}
	index := int(float64(len(sortedTimes)-1) * p)
	return sortedTimes[index]
}

func (sr *ScenarioRunner) collectSystemMetrics() SystemMetrics {
	// Simplified system metrics collection
	// In production, use proper system monitoring libraries
	return SystemMetrics{
		CPUUsage:       0.0, // Would collect actual CPU usage
		MemoryUsage:    0.0, // Would collect actual memory usage
		ActiveGoroutines: int(atomic.LoadInt64(&sr.metrics.activeAgents)),
		HeapSize:       0,   // Would collect actual heap size
		GCPause:        0,   // Would collect actual GC pause time
	}
}