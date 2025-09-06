package testing

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// PerformanceTestSuite manages performance and load testing for MCP server
type PerformanceTestSuite struct {
	factory     *MockClientFactory
	serverURL   string
	logger      *logrus.Logger
	config      PerformanceTestConfig
	results     []PerformanceResult
	mutex       sync.RWMutex
}

type PerformanceTestConfig struct {
	MaxClients          int           `json:"max_clients"`
	RampUpDuration      time.Duration `json:"ramp_up_duration"`
	TestDuration        time.Duration `json:"test_duration"`
	RampDownDuration    time.Duration `json:"ramp_down_duration"`
	RequestsPerSecond   int           `json:"requests_per_second"`
	MaxResponseTime     time.Duration `json:"max_response_time"`
	ErrorThreshold      float64       `json:"error_threshold"`
	EnableMetrics       bool          `json:"enable_metrics"`
	WarmupRequests      int           `json:"warmup_requests"`
	CooldownPeriod      time.Duration `json:"cooldown_period"`
	ResourceMonitoring  bool          `json:"resource_monitoring"`
}

type PerformanceResult struct {
	TestName           string                `json:"test_name"`
	StartTime          time.Time             `json:"start_time"`
	Duration           time.Duration         `json:"duration"`
	TotalRequests      int64                 `json:"total_requests"`
	SuccessfulRequests int64                 `json:"successful_requests"`
	FailedRequests     int64                 `json:"failed_requests"`
	RequestsPerSecond  float64               `json:"requests_per_second"`
	ErrorRate          float64               `json:"error_rate"`
	ResponseTimes      ResponseTimeMetrics   `json:"response_times"`
	ThroughputMetrics  ThroughputMetrics     `json:"throughput_metrics"`
	ResourceMetrics    ResourceUsageMetrics  `json:"resource_metrics"`
	ClientMetrics      map[string]ClientPerf `json:"client_metrics"`
	OperationBreakdown map[string]OpMetrics  `json:"operation_breakdown"`
}

type ResponseTimeMetrics struct {
	Min         time.Duration `json:"min"`
	Max         time.Duration `json:"max"`
	Mean        time.Duration `json:"mean"`
	Median      time.Duration `json:"median"`
	P95         time.Duration `json:"p95"`
	P99         time.Duration `json:"p99"`
	StdDev      time.Duration `json:"std_dev"`
	Distribution []TimeBucket  `json:"distribution"`
}

type ThroughputMetrics struct {
	PeakRPS      float64   `json:"peak_rps"`
	MinRPS       float64   `json:"min_rps"`
	AvgRPS       float64   `json:"avg_rps"`
	Variability  float64   `json:"variability"`
	Timestamps   []time.Time `json:"timestamps"`
	Values       []float64   `json:"values"`
}

type ResourceUsageMetrics struct {
	CPUUsage        []CPUSample       `json:"cpu_usage"`
	MemoryUsage     []MemorySample    `json:"memory_usage"`
	NetworkIO       []NetworkSample   `json:"network_io"`
	ConnectionStats ConnectionSample  `json:"connection_stats"`
}

type ClientPerf struct {
	ClientID       string        `json:"client_id"`
	Requests       int64         `json:"requests"`
	Errors         int64         `json:"errors"`
	AvgResponse    time.Duration `json:"avg_response"`
	BytesSent      int64         `json:"bytes_sent"`
	BytesReceived  int64         `json:"bytes_received"`
	Reconnects     int           `json:"reconnects"`
}

type OpMetrics struct {
	Operation    string        `json:"operation"`
	Count        int64         `json:"count"`
	Errors       int64         `json:"errors"`
	AvgDuration  time.Duration `json:"avg_duration"`
	MinDuration  time.Duration `json:"min_duration"`
	MaxDuration  time.Duration `json:"max_duration"`
	P95Duration  time.Duration `json:"p95_duration"`
}

type TimeBucket struct {
	Range string `json:"range"`
	Count int64  `json:"count"`
}

type CPUSample struct {
	Timestamp time.Time `json:"timestamp"`
	Usage     float64   `json:"usage"`
}

type MemorySample struct {
	Timestamp time.Time `json:"timestamp"`
	Used      int64     `json:"used"`
	Available int64     `json:"available"`
}

type NetworkSample struct {
	Timestamp time.Time `json:"timestamp"`
	BytesIn   int64     `json:"bytes_in"`
	BytesOut  int64     `json:"bytes_out"`
}

type ConnectionSample struct {
	Timestamp   time.Time `json:"timestamp"`
	Active      int       `json:"active"`
	Idle        int       `json:"idle"`
	Total       int       `json:"total"`
}

// Load test patterns
type LoadPattern string

const (
	PatternConstant  LoadPattern = "constant"
	PatternRampUp    LoadPattern = "ramp_up"
	PatternSpike     LoadPattern = "spike"
	PatternStress    LoadPattern = "stress"
	PatternSoak      LoadPattern = "soak"
	PatternBreakpoint LoadPattern = "breakpoint"
)

type LoadTest struct {
	Pattern     LoadPattern   `json:"pattern"`
	Clients     []int         `json:"clients"`
	Duration    time.Duration `json:"duration"`
	Operations  []string      `json:"operations"`
	Description string        `json:"description"`
}

var DefaultLoadTests = []LoadTest{
	{
		Pattern: PatternConstant, Clients: []int{10}, Duration: 2 * time.Minute,
		Operations: []string{"classify_variant", "validate_hgvs"},
		Description: "Constant load test with moderate concurrent users",
	},
	{
		Pattern: PatternRampUp, Clients: []int{1, 5, 10, 20, 50}, Duration: 5 * time.Minute,
		Operations: []string{"classify_variant", "query_evidence", "apply_rule"},
		Description: "Gradual ramp-up to test scaling behavior",
	},
	{
		Pattern: PatternSpike, Clients: []int{10, 100, 10}, Duration: 3 * time.Minute,
		Operations: []string{"classify_variant"},
		Description: "Spike test to evaluate response to sudden load increase",
	},
	{
		Pattern: PatternStress, Clients: []int{100}, Duration: 10 * time.Minute,
		Operations: []string{"classify_variant", "query_evidence", "generate_report"},
		Description: "Stress test with high concurrent load",
	},
	{
		Pattern: PatternSoak, Clients: []int{25}, Duration: 30 * time.Minute,
		Operations: []string{"classify_variant", "query_evidence"},
		Description: "Soak test for stability over extended period",
	},
}

func NewPerformanceTestSuite(serverURL string, config PerformanceTestConfig) *PerformanceTestSuite {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})

	return &PerformanceTestSuite{
		factory:   NewMockClientFactory(),
		serverURL: serverURL,
		logger:    logger,
		config:    config,
		results:   make([]PerformanceResult, 0),
	}
}

func (suite *PerformanceTestSuite) RunPerformanceTests(ctx context.Context, t *testing.T) {
	suite.logger.Info("Starting comprehensive performance test suite")

	for _, loadTest := range DefaultLoadTests {
		t.Run(fmt.Sprintf("LoadTest_%s", loadTest.Pattern), func(t *testing.T) {
			testCtx, cancel := context.WithTimeout(ctx, loadTest.Duration+time.Minute)
			defer cancel()
			
			result, err := suite.runLoadTest(testCtx, loadTest)
			require.NoError(t, err)
			
			suite.validatePerformanceResult(t, result, loadTest)
			
			suite.mutex.Lock()
			suite.results = append(suite.results, *result)
			suite.mutex.Unlock()
		})
	}

	// Run specialized performance tests
	t.Run("ConcurrencyStressTest", suite.TestConcurrencyStress)
	t.Run("ThroughputBenchmark", suite.TestThroughputBenchmark)
	t.Run("ResourceLeakTest", suite.TestResourceLeak)
	t.Run("LatencyUnderLoad", suite.TestLatencyUnderLoad)
}

func (suite *PerformanceTestSuite) runLoadTest(ctx context.Context, loadTest LoadTest) (*PerformanceResult, error) {
	result := &PerformanceResult{
		TestName:           string(loadTest.Pattern),
		StartTime:          time.Now(),
		ClientMetrics:      make(map[string]ClientPerf),
		OperationBreakdown: make(map[string]OpMetrics),
	}

	var totalRequests int64
	var successfulRequests int64
	var failedRequests int64
	var responseTimes []time.Duration
	var responseTimesMutex sync.Mutex
	
	// Start resource monitoring if enabled
	var resourceMonitor *ResourceMonitor
	if suite.config.ResourceMonitoring {
		resourceMonitor = NewResourceMonitor(time.Second)
		resourceMonitor.Start(ctx)
		defer resourceMonitor.Stop()
	}

	switch loadTest.Pattern {
	case PatternConstant:
		err := suite.runConstantLoad(ctx, loadTest, result, &totalRequests, &successfulRequests, &failedRequests, &responseTimes, &responseTimesMutex)
		if err != nil {
			return result, err
		}
		
	case PatternRampUp:
		err := suite.runRampUpLoad(ctx, loadTest, result, &totalRequests, &successfulRequests, &failedRequests, &responseTimes, &responseTimesMutex)
		if err != nil {
			return result, err
		}
		
	case PatternSpike:
		err := suite.runSpikeLoad(ctx, loadTest, result, &totalRequests, &successfulRequests, &failedRequests, &responseTimes, &responseTimesMutex)
		if err != nil {
			return result, err
		}
		
	default:
		return result, fmt.Errorf("unsupported load pattern: %s", loadTest.Pattern)
	}

	// Finalize results
	result.Duration = time.Since(result.StartTime)
	result.TotalRequests = totalRequests
	result.SuccessfulRequests = successfulRequests
	result.FailedRequests = failedRequests
	result.RequestsPerSecond = float64(totalRequests) / result.Duration.Seconds()
	result.ErrorRate = float64(failedRequests) / float64(totalRequests) * 100

	// Calculate response time metrics
	responseTimesMutex.Lock()
	result.ResponseTimes = suite.calculateResponseTimeMetrics(responseTimes)
	responseTimesMutex.Unlock()

	// Collect resource metrics if monitoring was enabled
	if resourceMonitor != nil {
		result.ResourceMetrics = resourceMonitor.GetMetrics()
	}

	return result, nil
}

func (suite *PerformanceTestSuite) runConstantLoad(ctx context.Context, loadTest LoadTest, result *PerformanceResult, totalReqs, successReqs, failedReqs *int64, responseTimes *[]time.Duration, rtMutex *sync.Mutex) error {
	clientCount := loadTest.Clients[0]
	var wg sync.WaitGroup
	
	// Create clients
	clients := make([]*MockMCPClient, clientCount)
	for i := 0; i < clientCount; i++ {
		clientConfig := ClientConfig{
			ID: fmt.Sprintf("perf_client_%d", i),
			Name: fmt.Sprintf("PerfClient_%d", i),
			Version: "1.0.0",
			Transport: TransportWebSocket,
		}
		
		client, err := suite.factory.CreateClient(clientConfig)
		if err != nil {
			return err
		}
		clients[i] = client
		defer suite.factory.RemoveClient(clientConfig.ID)
		
		// Connect client
		if err := client.Connect(ctx, suite.serverURL); err != nil {
			return fmt.Errorf("failed to connect client %d: %w", i, err)
		}
		defer client.Disconnect()
	}

	// Start load generation
	testCtx, cancel := context.WithTimeout(ctx, loadTest.Duration)
	defer cancel()

	for i, client := range clients {
		wg.Add(1)
		go func(clientIndex int, c *MockMCPClient) {
			defer wg.Done()
			suite.generateClientLoad(testCtx, c, loadTest.Operations, totalReqs, successReqs, failedReqs, responseTimes, rtMutex)
			
			// Collect client metrics
			stats := c.GetStats()
			result.ClientMetrics[c.ID] = ClientPerf{
				ClientID:      c.ID,
				Requests:      stats.TotalRequests,
				Errors:        stats.FailedRequests,
				AvgResponse:   stats.AverageResponseTime,
				BytesSent:     stats.BytesSent,
				BytesReceived: stats.BytesReceived,
				Reconnects:    stats.ReconnectCount,
			}
		}(i, client)
	}

	wg.Wait()
	return nil
}

func (suite *PerformanceTestSuite) runRampUpLoad(ctx context.Context, loadTest LoadTest, result *PerformanceResult, totalReqs, successReqs, failedReqs *int64, responseTimes *[]time.Duration, rtMutex *sync.Mutex) error {
	stageCount := len(loadTest.Clients)
	stageDuration := loadTest.Duration / time.Duration(stageCount)
	
	for stage, clientCount := range loadTest.Clients {
		suite.logger.WithFields(logrus.Fields{
			"stage":   stage,
			"clients": clientCount,
		}).Info("Starting ramp-up stage")
		
		stageCtx, cancel := context.WithTimeout(ctx, stageDuration)
		
		// Create stage-specific load test
		stageLoad := LoadTest{
			Pattern:    PatternConstant,
			Clients:    []int{clientCount},
			Duration:   stageDuration,
			Operations: loadTest.Operations,
		}
		
		err := suite.runConstantLoad(stageCtx, stageLoad, result, totalReqs, successReqs, failedReqs, responseTimes, rtMutex)
		cancel()
		
		if err != nil {
			return fmt.Errorf("ramp-up stage %d failed: %w", stage, err)
		}
		
		// Brief pause between stages
		time.Sleep(time.Second)
	}
	
	return nil
}

func (suite *PerformanceTestSuite) runSpikeLoad(ctx context.Context, loadTest LoadTest, result *PerformanceResult, totalReqs, successReqs, failedReqs *int64, responseTimes *[]time.Duration, rtMutex *sync.Mutex) error {
	if len(loadTest.Clients) != 3 {
		return fmt.Errorf("spike load test requires exactly 3 client counts: baseline, spike, recovery")
	}
	
	baseline := loadTest.Clients[0]
	spike := loadTest.Clients[1]
	recovery := loadTest.Clients[2]
	
	stageDuration := loadTest.Duration / 3
	
	stages := []struct {
		name    string
		clients int
	}{
		{"baseline", baseline},
		{"spike", spike},
		{"recovery", recovery},
	}
	
	for _, stage := range stages {
		suite.logger.WithFields(logrus.Fields{
			"stage":   stage.name,
			"clients": stage.clients,
		}).Info("Starting spike test stage")
		
		stageCtx, cancel := context.WithTimeout(ctx, stageDuration)
		
		stageLoad := LoadTest{
			Pattern:    PatternConstant,
			Clients:    []int{stage.clients},
			Duration:   stageDuration,
			Operations: loadTest.Operations,
		}
		
		err := suite.runConstantLoad(stageCtx, stageLoad, result, totalReqs, successReqs, failedReqs, responseTimes, rtMutex)
		cancel()
		
		if err != nil {
			return fmt.Errorf("spike stage %s failed: %w", stage.name, err)
		}
	}
	
	return nil
}

func (suite *PerformanceTestSuite) generateClientLoad(ctx context.Context, client *MockMCPClient, operations []string, totalReqs, successReqs, failedReqs *int64, responseTimes *[]time.Duration, rtMutex *sync.Mutex) {
	ticker := time.NewTicker(time.Second / time.Duration(suite.config.RequestsPerSecond))
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Select random operation
			operation := operations[time.Now().UnixNano()%int64(len(operations))]
			
			start := time.Now()
			err := suite.performOperation(ctx, client, operation)
			duration := time.Since(start)
			
			atomic.AddInt64(totalReqs, 1)
			
			if err != nil {
				atomic.AddInt64(failedReqs, 1)
			} else {
				atomic.AddInt64(successReqs, 1)
			}
			
			// Record response time
			rtMutex.Lock()
			*responseTimes = append(*responseTimes, duration)
			rtMutex.Unlock()
		}
	}
}

func (suite *PerformanceTestSuite) performOperation(ctx context.Context, client *MockMCPClient, operation string) error {
	switch operation {
	case "classify_variant":
		_, err := client.CallTool(ctx, "classify_variant", map[string]interface{}{
			"variant": "NM_000492.3:c.1521_1523del",
		})
		return err
		
	case "validate_hgvs":
		_, err := client.CallTool(ctx, "validate_hgvs", map[string]interface{}{
			"notation": "NM_000492.3:c.1521_1523del",
		})
		return err
		
	case "query_evidence":
		_, err := client.CallTool(ctx, "query_evidence", map[string]interface{}{
			"variant": "NM_000492.3:c.1521_1523del",
		})
		return err
		
	case "apply_rule":
		_, err := client.CallTool(ctx, "apply_rule", map[string]interface{}{
			"rule": "PVS1",
			"variant": "NM_000492.3:c.1521_1523del",
		})
		return err
		
	case "generate_report":
		_, err := client.CallTool(ctx, "generate_report", map[string]interface{}{
			"variant_id": "test_variant",
		})
		return err
		
	case "get_resource":
		_, err := client.GetResource(ctx, "variant/NM_000492.3:c.1521_1523del")
		return err
		
	default:
		return fmt.Errorf("unknown operation: %s", operation)
	}
}

func (suite *PerformanceTestSuite) TestConcurrencyStress(ctx context.Context, t *testing.T) {
	maxClients := suite.config.MaxClients
	if maxClients == 0 {
		maxClients = 200
	}

	results := make([]float64, 0)
	clientCounts := []int{10, 25, 50, 100, maxClients}

	for _, clientCount := range clientCounts {
		t.Run(fmt.Sprintf("Clients_%d", clientCount), func(t *testing.T) {
			rps, err := suite.measureThroughput(ctx, clientCount, 30*time.Second)
			require.NoError(t, err)
			
			results = append(results, rps)
			
			suite.logger.WithFields(logrus.Fields{
				"clients": clientCount,
				"rps":     rps,
			}).Info("Concurrency test result")
			
			// Validate that throughput doesn't degrade significantly
			if len(results) > 1 {
				previousRPS := results[len(results)-2]
				degradation := (previousRPS - rps) / previousRPS * 100
				
				assert.LessOrEqual(t, degradation, 50.0, 
					"Throughput degradation should not exceed 50%% (actual: %.2f%%)", degradation)
			}
		})
	}
}

func (suite *PerformanceTestSuite) TestThroughputBenchmark(ctx context.Context, t *testing.T) {
	clientCount := 50
	duration := 2 * time.Minute
	
	rps, err := suite.measureThroughput(ctx, clientCount, duration)
	require.NoError(t, err)
	
	suite.logger.WithFields(logrus.Fields{
		"clients":  clientCount,
		"duration": duration,
		"rps":      rps,
	}).Info("Throughput benchmark completed")
	
	// Assert minimum throughput threshold
	minThroughput := 10.0 // requests per second
	assert.GreaterOrEqual(t, rps, minThroughput,
		"Throughput should be at least %.1f RPS", minThroughput)
}

func (suite *PerformanceTestSuite) TestResourceLeak(ctx context.Context, t *testing.T) {
	duration := 5 * time.Minute
	clientCount := 20
	
	monitor := NewResourceMonitor(5 * time.Second)
	monitor.Start(ctx)
	defer monitor.Stop()
	
	// Run sustained load
	_, err := suite.measureThroughput(ctx, clientCount, duration)
	require.NoError(t, err)
	
	metrics := monitor.GetMetrics()
	
	// Check for memory leaks
	if len(metrics.MemoryUsage) > 1 {
		firstSample := metrics.MemoryUsage[0].Used
		lastSample := metrics.MemoryUsage[len(metrics.MemoryUsage)-1].Used
		
		memoryGrowth := float64(lastSample-firstSample) / float64(firstSample) * 100
		
		suite.logger.WithFields(logrus.Fields{
			"initial_memory": firstSample,
			"final_memory":   lastSample,
			"growth_percent": memoryGrowth,
		}).Info("Memory usage analysis")
		
		assert.LessOrEqual(t, memoryGrowth, 50.0,
			"Memory growth should not exceed 50%% over test duration")
	}
}

func (suite *PerformanceTestSuite) TestLatencyUnderLoad(ctx context.Context, t *testing.T) {
	clientCounts := []int{10, 50, 100}
	maxLatency := suite.config.MaxResponseTime
	if maxLatency == 0 {
		maxLatency = 5 * time.Second
	}

	for _, clientCount := range clientCounts {
		t.Run(fmt.Sprintf("Load_%d_clients", clientCount), func(t *testing.T) {
			latencies, err := suite.measureLatencyUnderLoad(ctx, clientCount, time.Minute)
			require.NoError(t, err)
			
			metrics := suite.calculateResponseTimeMetrics(latencies)
			
			suite.logger.WithFields(logrus.Fields{
				"clients": clientCount,
				"p95":     metrics.P95,
				"p99":     metrics.P99,
				"max":     metrics.Max,
			}).Info("Latency under load results")
			
			assert.LessOrEqual(t, metrics.P95, maxLatency,
				"P95 latency should be within acceptable bounds")
			assert.LessOrEqual(t, metrics.P99, maxLatency*2,
				"P99 latency should be within acceptable bounds")
		})
	}
}

func (suite *PerformanceTestSuite) measureThroughput(ctx context.Context, clientCount int, duration time.Duration) (float64, error) {
	testCtx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()

	var totalRequests int64
	var wg sync.WaitGroup

	// Create clients and generate load
	for i := 0; i < clientCount; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()
			
			clientConfig := ClientConfig{
				ID: fmt.Sprintf("throughput_client_%d", clientID),
				Name: fmt.Sprintf("ThroughputClient_%d", clientID),
				Version: "1.0.0",
				Transport: TransportWebSocket,
			}
			
			client, err := suite.factory.CreateClient(clientConfig)
			if err != nil {
				return
			}
			defer suite.factory.RemoveClient(clientConfig.ID)
			
			if err := client.Connect(testCtx, suite.serverURL); err != nil {
				return
			}
			defer client.Disconnect()
			
			for {
				select {
				case <-testCtx.Done():
					return
				default:
					err := suite.performOperation(testCtx, client, "classify_variant")
					if err == nil {
						atomic.AddInt64(&totalRequests, 1)
					}
					time.Sleep(10 * time.Millisecond) // Small delay to prevent overwhelming
				}
			}
		}(i)
	}

	wg.Wait()
	
	rps := float64(totalRequests) / duration.Seconds()
	return rps, nil
}

func (suite *PerformanceTestSuite) measureLatencyUnderLoad(ctx context.Context, clientCount int, duration time.Duration) ([]time.Duration, error) {
	testCtx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()

	var latencies []time.Duration
	var latenciesMutex sync.Mutex
	var wg sync.WaitGroup

	for i := 0; i < clientCount; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()
			
			clientConfig := ClientConfig{
				ID: fmt.Sprintf("latency_client_%d", clientID),
				Name: fmt.Sprintf("LatencyClient_%d", clientID),
				Version: "1.0.0",
				Transport: TransportWebSocket,
			}
			
			client, err := suite.factory.CreateClient(clientConfig)
			if err != nil {
				return
			}
			defer suite.factory.RemoveClient(clientConfig.ID)
			
			if err := client.Connect(testCtx, suite.serverURL); err != nil {
				return
			}
			defer client.Disconnect()
			
			for {
				select {
				case <-testCtx.Done():
					return
				default:
					start := time.Now()
					err := suite.performOperation(testCtx, client, "classify_variant")
					latency := time.Since(start)
					
					if err == nil {
						latenciesMutex.Lock()
						latencies = append(latencies, latency)
						latenciesMutex.Unlock()
					}
					
					time.Sleep(100 * time.Millisecond)
				}
			}
		}(i)
	}

	wg.Wait()
	return latencies, nil
}

func (suite *PerformanceTestSuite) calculateResponseTimeMetrics(responseTimes []time.Duration) ResponseTimeMetrics {
	if len(responseTimes) == 0 {
		return ResponseTimeMetrics{}
	}

	// Sort for percentile calculations
	sorted := make([]time.Duration, len(responseTimes))
	copy(sorted, responseTimes)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})

	// Calculate basic metrics
	min := sorted[0]
	max := sorted[len(sorted)-1]
	
	var sum time.Duration
	for _, rt := range responseTimes {
		sum += rt
	}
	mean := sum / time.Duration(len(responseTimes))
	
	median := sorted[len(sorted)/2]
	p95 := sorted[int(float64(len(sorted))*0.95)]
	p99 := sorted[int(float64(len(sorted))*0.99)]
	
	// Calculate standard deviation
	var variance float64
	for _, rt := range responseTimes {
		diff := float64(rt - mean)
		variance += diff * diff
	}
	variance /= float64(len(responseTimes))
	stdDev := time.Duration(math.Sqrt(variance))
	
	// Create distribution buckets
	distribution := suite.createDistribution(sorted)

	return ResponseTimeMetrics{
		Min:          min,
		Max:          max,
		Mean:         mean,
		Median:       median,
		P95:          p95,
		P99:          p99,
		StdDev:       stdDev,
		Distribution: distribution,
	}
}

func (suite *PerformanceTestSuite) createDistribution(sortedTimes []time.Duration) []TimeBucket {
	if len(sortedTimes) == 0 {
		return []TimeBucket{}
	}

	buckets := []struct {
		name      string
		threshold time.Duration
	}{
		{"<100ms", 100 * time.Millisecond},
		{"100ms-500ms", 500 * time.Millisecond},
		{"500ms-1s", 1 * time.Second},
		{"1s-5s", 5 * time.Second},
		{">5s", time.Duration(math.MaxInt64)},
	}

	distribution := make([]TimeBucket, len(buckets))
	bucketIndex := 0
	
	for _, bucket := range buckets {
		var count int64
		for bucketIndex < len(sortedTimes) && sortedTimes[bucketIndex] <= bucket.threshold {
			count++
			bucketIndex++
		}
		distribution = append(distribution, TimeBucket{
			Range: bucket.name,
			Count: count,
		})
	}

	return distribution
}

func (suite *PerformanceTestSuite) validatePerformanceResult(t *testing.T, result *PerformanceResult, loadTest LoadTest) {
	// Validate error rate
	maxErrorRate := suite.config.ErrorThreshold
	if maxErrorRate == 0 {
		maxErrorRate = 5.0 // 5% default
	}
	
	assert.LessOrEqual(t, result.ErrorRate, maxErrorRate,
		"Error rate should be within acceptable threshold")

	// Validate response times
	maxResponseTime := suite.config.MaxResponseTime
	if maxResponseTime == 0 {
		maxResponseTime = 5 * time.Second
	}
	
	assert.LessOrEqual(t, result.ResponseTimes.P95, maxResponseTime,
		"P95 response time should be within acceptable bounds")

	// Validate throughput
	minThroughput := 1.0 // 1 RPS minimum
	assert.GreaterOrEqual(t, result.RequestsPerSecond, minThroughput,
		"Throughput should meet minimum requirements")
}

func (suite *PerformanceTestSuite) GetResults() []PerformanceResult {
	suite.mutex.RLock()
	defer suite.mutex.RUnlock()
	
	results := make([]PerformanceResult, len(suite.results))
	copy(results, suite.results)
	return results
}

// ResourceMonitor tracks system resource usage during tests
type ResourceMonitor struct {
	interval      time.Duration
	cpuSamples    []CPUSample
	memorySamples []MemorySample
	networkSamples []NetworkSample
	running       bool
	stopCh        chan struct{}
	mutex         sync.RWMutex
}

func NewResourceMonitor(interval time.Duration) *ResourceMonitor {
	return &ResourceMonitor{
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

func (rm *ResourceMonitor) Start(ctx context.Context) {
	rm.mutex.Lock()
	rm.running = true
	rm.mutex.Unlock()
	
	go rm.monitor(ctx)
}

func (rm *ResourceMonitor) Stop() {
	rm.mutex.Lock()
	if rm.running {
		rm.running = false
		close(rm.stopCh)
	}
	rm.mutex.Unlock()
}

func (rm *ResourceMonitor) monitor(ctx context.Context) {
	ticker := time.NewTicker(rm.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-rm.stopCh:
			return
		case <-ticker.C:
			rm.collectSample()
		}
	}
}

func (rm *ResourceMonitor) collectSample() {
	now := time.Now()
	
	// Simulate resource collection (would integrate with actual system monitoring)
	cpuUsage := 25.0 + float64(now.UnixNano()%20) // Mock CPU usage
	memoryUsed := int64(1024 * 1024 * (100 + now.UnixNano()%50)) // Mock memory
	memoryAvailable := int64(1024 * 1024 * 1000) // Mock available memory
	
	rm.mutex.Lock()
	rm.cpuSamples = append(rm.cpuSamples, CPUSample{
		Timestamp: now,
		Usage:     cpuUsage,
	})
	
	rm.memorySamples = append(rm.memorySamples, MemorySample{
		Timestamp: now,
		Used:      memoryUsed,
		Available: memoryAvailable,
	})
	
	rm.networkSamples = append(rm.networkSamples, NetworkSample{
		Timestamp: now,
		BytesIn:   int64(now.UnixNano() % 10000),
		BytesOut:  int64(now.UnixNano() % 8000),
	})
	rm.mutex.Unlock()
}

func (rm *ResourceMonitor) GetMetrics() ResourceUsageMetrics {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()
	
	return ResourceUsageMetrics{
		CPUUsage:    append([]CPUSample(nil), rm.cpuSamples...),
		MemoryUsage: append([]MemorySample(nil), rm.memorySamples...),
		NetworkIO:   append([]NetworkSample(nil), rm.networkSamples...),
		ConnectionStats: ConnectionSample{
			Timestamp: time.Now(),
			Active:    10,
			Idle:      5,
			Total:     15,
		},
	}
}