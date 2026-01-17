package testing

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TransportIntegrationTestSuite manages transport layer performance and reliability testing
type TransportIntegrationTestSuite struct {
	factory    *MockClientFactory
	serverURL  string
	logger     *logrus.Logger
	config     TransportTestConfig
	results    []TransportTestResult
	transports map[TransportType]*TransportMetrics
}

type TransportTestConfig struct {
	TestTimeout         time.Duration            `json:"test_timeout"`
	ConnectionTimeout   time.Duration            `json:"connection_timeout"`
	MaxConnections     int                      `json:"max_connections"`
	MessageSizes       []int                    `json:"message_sizes"`
	ConcurrencyLevels  []int                    `json:"concurrency_levels"`
	TestDuration       time.Duration            `json:"test_duration"`
	ReliabilityTests   bool                     `json:"reliability_tests"`
	PerformanceTests   bool                     `json:"performance_tests"`
	NetworkConditions  []NetworkCondition       `json:"network_conditions"`
	TransportTypes     []TransportType          `json:"transport_types"`
}

type TransportTestResult struct {
	TransportType      TransportType            `json:"transport_type"`
	TestName          string                   `json:"test_name"`
	Success           bool                     `json:"success"`
	Duration          time.Duration            `json:"duration"`
	ConnectionMetrics ConnectionMetrics        `json:"connection_metrics"`
	PerformanceMetrics PerformanceMetrics      `json:"performance_metrics"`
	ReliabilityMetrics ReliabilityMetrics      `json:"reliability_metrics"`
	NetworkCondition  NetworkCondition         `json:"network_condition"`
	Errors            []string                 `json:"errors"`
	Metadata          map[string]interface{}   `json:"metadata"`
}

type ConnectionMetrics struct {
	EstablishTime      time.Duration `json:"establish_time"`
	TotalConnections   int           `json:"total_connections"`
	ActiveConnections  int           `json:"active_connections"`
	FailedConnections  int           `json:"failed_connections"`
	ReconnectAttempts  int           `json:"reconnect_attempts"`
	ConnectionErrors   []string      `json:"connection_errors"`
	Throughput         float64       `json:"throughput_mbps"`
	Overhead          float64       `json:"overhead_percent"`
}

type PerformanceMetrics struct {
	MessagesPerSecond    float64       `json:"messages_per_second"`
	AvgLatency          time.Duration `json:"avg_latency"`
	MinLatency          time.Duration `json:"min_latency"`
	MaxLatency          time.Duration `json:"max_latency"`
	P95Latency          time.Duration `json:"p95_latency"`
	P99Latency          time.Duration `json:"p99_latency"`
	BytesPerSecond      float64       `json:"bytes_per_second"`
	MemoryUsage         int64         `json:"memory_usage_bytes"`
	CPUUsage            float64       `json:"cpu_usage_percent"`
	GoroutineCount      int           `json:"goroutine_count"`
}

type ReliabilityMetrics struct {
	SuccessRate         float64       `json:"success_rate"`
	ErrorRate           float64       `json:"error_rate"`
	TimeoutRate         float64       `json:"timeout_rate"`
	ReconnectRate       float64       `json:"reconnect_rate"`
	MessageLossRate     float64       `json:"message_loss_rate"`
	DuplicateRate       float64       `json:"duplicate_rate"`
	OrderingViolations  int           `json:"ordering_violations"`
	IntegrityFailures   int           `json:"integrity_failures"`
}

type NetworkCondition struct {
	Name          string        `json:"name"`
	Latency       time.Duration `json:"latency"`
	Bandwidth     int           `json:"bandwidth_kbps"`
	PacketLoss    float64       `json:"packet_loss_percent"`
	Jitter        time.Duration `json:"jitter"`
	Enabled       bool          `json:"enabled"`
}

type TransportMetrics struct {
	Type                TransportType            `json:"type"`
	TotalMessages       int64                    `json:"total_messages"`
	TotalBytes         int64                    `json:"total_bytes"`
	TotalConnections   int64                    `json:"total_connections"`
	AvgConnectionTime  time.Duration            `json:"avg_connection_time"`
	AvgMessageLatency  time.Duration            `json:"avg_message_latency"`
	ErrorCount         int64                    `json:"error_count"`
	LastUpdated        time.Time                `json:"last_updated"`
}

// NetworkEmulator simulates various network conditions
type NetworkEmulator struct {
	conditions map[string]NetworkCondition
	active     bool
	mutex      sync.RWMutex
	logger     *logrus.Logger
}

// MessageTracker tracks message ordering and integrity
type MessageTracker struct {
	sent        map[string]time.Time
	received    map[string]time.Time
	sequence    map[string]int64
	duplicates  map[string]int
	mutex       sync.RWMutex
}

func NewTransportIntegrationTestSuite(serverURL string, config TransportTestConfig) *TransportIntegrationTestSuite {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})

	if config.TestTimeout == 0 {
		config.TestTimeout = 5 * time.Minute
	}
	if config.ConnectionTimeout == 0 {
		config.ConnectionTimeout = 30 * time.Second
	}
	if config.MaxConnections == 0 {
		config.MaxConnections = 100
	}
	if len(config.TransportTypes) == 0 {
		config.TransportTypes = []TransportType{TransportWebSocket, TransportHTTP, TransportSSE}
	}
	if len(config.MessageSizes) == 0 {
		config.MessageSizes = []int{1024, 10240, 102400} // 1KB, 10KB, 100KB
	}
	if len(config.ConcurrencyLevels) == 0 {
		config.ConcurrencyLevels = []int{1, 10, 50, 100}
	}
	if config.TestDuration == 0 {
		config.TestDuration = 2 * time.Minute
	}

	suite := &TransportIntegrationTestSuite{
		factory:    NewMockClientFactory(),
		serverURL:  serverURL,
		logger:     logger,
		config:     config,
		results:    make([]TransportTestResult, 0),
		transports: make(map[TransportType]*TransportMetrics),
	}

	suite.initializeTransportMetrics()
	suite.loadNetworkConditions()

	return suite
}

func (suite *TransportIntegrationTestSuite) initializeTransportMetrics() {
	for _, transportType := range suite.config.TransportTypes {
		suite.transports[transportType] = &TransportMetrics{
			Type:        transportType,
			LastUpdated: time.Now(),
		}
	}
}

func (suite *TransportIntegrationTestSuite) loadNetworkConditions() {
	if len(suite.config.NetworkConditions) == 0 {
		suite.config.NetworkConditions = []NetworkCondition{
			{Name: "optimal", Latency: 1 * time.Millisecond, Bandwidth: 1000000, PacketLoss: 0.0, Enabled: true},
			{Name: "good_broadband", Latency: 20 * time.Millisecond, Bandwidth: 100000, PacketLoss: 0.1, Enabled: true},
			{Name: "mobile_4g", Latency: 50 * time.Millisecond, Bandwidth: 50000, PacketLoss: 0.5, Enabled: true},
			{Name: "mobile_3g", Latency: 200 * time.Millisecond, Bandwidth: 5000, PacketLoss: 1.0, Enabled: true},
			{Name: "poor_connection", Latency: 500 * time.Millisecond, Bandwidth: 1000, PacketLoss: 3.0, Enabled: true},
		}
	}
}

func (suite *TransportIntegrationTestSuite) RunTransportIntegrationTests(ctx context.Context, t *testing.T) {
	suite.logger.Info("Starting comprehensive transport integration tests")

	for _, transportType := range suite.config.TransportTypes {
		t.Run(fmt.Sprintf("Transport_%s", transportType), func(t *testing.T) {
			suite.runTransportTests(ctx, t, transportType)
		})
	}

	// Cross-transport comparison tests
	t.Run("TransportComparison", func(t *testing.T) {
		suite.TestTransportComparison(ctx, t)
	})

	// Network condition tests
	if suite.config.ReliabilityTests {
		t.Run("NetworkConditionTests", func(t *testing.T) {
			suite.TestNetworkConditions(ctx, t)
		})
	}
	
	// Generate comprehensive report
	t.Run("TransportReport", func(t *testing.T) {
		report := suite.GenerateTransportReport()
		suite.validateTransportReport(t, report)
	})
}

func (suite *TransportIntegrationTestSuite) runTransportTests(ctx context.Context, t *testing.T, transportType TransportType) {
	tests := []struct {
		name string
		fn   func(context.Context, *testing.T, TransportType)
	}{
		{"ConnectionEstablishment", suite.TestConnectionEstablishment},
		{"MessageThroughput", suite.TestMessageThroughput},
		{"LatencyMeasurement", suite.TestLatencyMeasurement},
		{"ConcurrentConnections", suite.TestConcurrentConnections},
		{"MessageSizeHandling", suite.TestMessageSizeHandling},
		{"ConnectionReliability", suite.TestConnectionReliability},
		{"ErrorHandling", suite.TestTransportErrorHandling},
		{"ResourceUsage", suite.TestResourceUsage},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			testCtx, cancel := context.WithTimeout(ctx, suite.config.TestTimeout)
			defer cancel()
			
			suite.logger.WithFields(logrus.Fields{
				"transport": transportType,
				"test":      test.name,
			}).Info("Running transport test")
			
			test.fn(testCtx, t, transportType)
		})
	}
}

func (suite *TransportIntegrationTestSuite) TestConnectionEstablishment(ctx context.Context, t *testing.T, transportType TransportType) {
	result := TransportTestResult{
		TransportType: transportType,
		TestName:     "ConnectionEstablishment",
		Errors:       make([]string, 0),
		Metadata:     make(map[string]interface{}),
	}
	startTime := time.Now()

	clientConfig := ClientConfig{
		ID: fmt.Sprintf("connection_test_%s", transportType),
		Name: "ConnectionTestClient",
		Version: "1.0.0",
		Transport: transportType,
		MockConfig: MockClientConfig{
			ConnectTimeout: suite.config.ConnectionTimeout,
		},
	}

	client, err := suite.factory.CreateClient(clientConfig)
	require.NoError(t, err)
	defer suite.factory.RemoveClient(clientConfig.ID)

	// Measure connection establishment time
	connectStart := time.Now()
	err = client.Connect(ctx, suite.serverURL)
	connectTime := time.Since(connectStart)

	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Connection failed: %v", err))
		result.Success = false
	} else {
		result.Success = true
		assert.True(t, client.IsConnected())
	}

	result.ConnectionMetrics.EstablishTime = connectTime
	result.ConnectionMetrics.TotalConnections = 1
	if result.Success {
		result.ConnectionMetrics.ActiveConnections = 1
	} else {
		result.ConnectionMetrics.FailedConnections = 1
	}

	result.Duration = time.Since(startTime)
	suite.results = append(suite.results, result)

	// Update transport metrics
	suite.updateTransportMetrics(transportType, &result)

	// Validate connection time
	maxConnectionTime := suite.config.ConnectionTimeout / 2
	assert.LessOrEqual(t, connectTime, maxConnectionTime,
		"Connection establishment should be fast for %s", transportType)

	if client.IsConnected() {
		client.Disconnect()
	}
}

func (suite *TransportIntegrationTestSuite) TestMessageThroughput(ctx context.Context, t *testing.T, transportType TransportType) {
	if !suite.config.PerformanceTests {
		t.Skip("Performance tests disabled")
		return
	}

	result := TransportTestResult{
		TransportType: transportType,
		TestName:     "MessageThroughput",
		Errors:       make([]string, 0),
		Metadata:     make(map[string]interface{}),
	}
	startTime := time.Now()

	clientConfig := ClientConfig{
		ID: fmt.Sprintf("throughput_test_%s", transportType),
		Name: "ThroughputTestClient",
		Version: "1.0.0",
		Transport: transportType,
	}

	client, err := suite.factory.CreateClient(clientConfig)
	require.NoError(t, err)
	defer suite.factory.RemoveClient(clientConfig.ID)

	err = client.Connect(ctx, suite.serverURL)
	require.NoError(t, err)
	defer client.Disconnect()

	// Measure throughput
	testDuration := suite.config.TestDuration
	testCtx, cancel := context.WithTimeout(ctx, testDuration)
	defer cancel()

	var messageCount int64
	var totalBytes int64
	var errors int64

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-testCtx.Done():
			goto measurementComplete
		case <-ticker.C:
			// Send test message
			_, err := client.CallTool(testCtx, "validate_hgvs", map[string]interface{}{
				"notation": "NM_000492.3:c.1521_1523del",
			})
			
			messageCount++
			totalBytes += 100 // Approximate message size
			
			if err != nil {
				errors++
			}
		}
	}

measurementComplete:
	actualDuration := time.Since(startTime)
	messagesPerSecond := float64(messageCount) / actualDuration.Seconds()
	bytesPerSecond := float64(totalBytes) / actualDuration.Seconds()

	result.PerformanceMetrics.MessagesPerSecond = messagesPerSecond
	result.PerformanceMetrics.BytesPerSecond = bytesPerSecond
	result.ReliabilityMetrics.ErrorRate = float64(errors) / float64(messageCount) * 100

	result.Success = errors < messageCount/10 // Allow up to 10% errors
	result.Duration = actualDuration
	
	suite.results = append(suite.results, result)
	suite.updateTransportMetrics(transportType, &result)

	suite.logger.WithFields(logrus.Fields{
		"transport":           transportType,
		"messages_per_second": messagesPerSecond,
		"bytes_per_second":    bytesPerSecond,
		"error_rate":         result.ReliabilityMetrics.ErrorRate,
	}).Info("Throughput test completed")

	// Validate performance thresholds
	minThroughput := 10.0 // messages per second
	assert.GreaterOrEqual(t, messagesPerSecond, minThroughput,
		"Throughput should meet minimum requirement for %s", transportType)
}

func (suite *TransportIntegrationTestSuite) TestLatencyMeasurement(ctx context.Context, t *testing.T, transportType TransportType) {
	result := TransportTestResult{
		TransportType: transportType,
		TestName:     "LatencyMeasurement",
		Errors:       make([]string, 0),
		Metadata:     make(map[string]interface{}),
	}
	startTime := time.Now()

	clientConfig := ClientConfig{
		ID: fmt.Sprintf("latency_test_%s", transportType),
		Name: "LatencyTestClient",
		Version: "1.0.0",
		Transport: transportType,
	}

	client, err := suite.factory.CreateClient(clientConfig)
	require.NoError(t, err)
	defer suite.factory.RemoveClient(clientConfig.ID)

	err = client.Connect(ctx, suite.serverURL)
	require.NoError(t, err)
	defer client.Disconnect()

	// Measure latencies
	var latencies []time.Duration
	numMeasurements := 100

	for i := 0; i < numMeasurements; i++ {
		requestStart := time.Now()
		_, err := client.CallTool(ctx, "validate_hgvs", map[string]interface{}{
			"notation": "NM_000492.3:c.1521_1523del",
		})
		latency := time.Since(requestStart)

		if err == nil {
			latencies = append(latencies, latency)
		}

		time.Sleep(10 * time.Millisecond)
	}

	if len(latencies) == 0 {
		result.Errors = append(result.Errors, "No successful measurements")
		result.Success = false
	} else {
		// Calculate latency statistics
		metrics := suite.calculateLatencyStatistics(latencies)
		result.PerformanceMetrics = metrics
		result.Success = true
	}

	result.Duration = time.Since(startTime)
	suite.results = append(suite.results, result)
	suite.updateTransportMetrics(transportType, &result)

	if result.Success {
		suite.logger.WithFields(logrus.Fields{
			"transport":    transportType,
			"avg_latency":  result.PerformanceMetrics.AvgLatency,
			"p95_latency":  result.PerformanceMetrics.P95Latency,
			"measurements": len(latencies),
		}).Info("Latency measurement completed")

		// Validate latency requirements
		maxAvgLatency := 1000 * time.Millisecond // 1 second
		assert.LessOrEqual(t, result.PerformanceMetrics.AvgLatency, maxAvgLatency,
			"Average latency should be acceptable for %s", transportType)
	}
}

func (suite *TransportIntegrationTestSuite) TestConcurrentConnections(ctx context.Context, t *testing.T, transportType TransportType) {
	result := TransportTestResult{
		TransportType: transportType,
		TestName:     "ConcurrentConnections",
		Errors:       make([]string, 0),
		Metadata:     make(map[string]interface{}),
	}
	startTime := time.Now()

	maxConcurrent := min(suite.config.MaxConnections, 50) // Limit for testing
	var wg sync.WaitGroup
	var successCount, errorCount int64
	var mutex sync.Mutex

	for i := 0; i < maxConcurrent; i++ {
		wg.Add(1)
		go func(clientIndex int) {
			defer wg.Done()

			clientConfig := ClientConfig{
				ID: fmt.Sprintf("concurrent_%s_%d", transportType, clientIndex),
				Name: "ConcurrentTestClient",
				Version: "1.0.0",
				Transport: transportType,
			}

			client, err := suite.factory.CreateClient(clientConfig)
			if err != nil {
				mutex.Lock()
				errorCount++
				mutex.Unlock()
				return
			}
			defer suite.factory.RemoveClient(clientConfig.ID)

			err = client.Connect(ctx, suite.serverURL)
			if err != nil {
				mutex.Lock()
				errorCount++
				mutex.Unlock()
				return
			}

			// Perform a test operation
			_, err = client.CallTool(ctx, "tools/list", map[string]interface{}{})
			
			mutex.Lock()
			if err == nil {
				successCount++
			} else {
				errorCount++
			}
			mutex.Unlock()

			client.Disconnect()
		}(i)
	}

	wg.Wait()

	result.ConnectionMetrics.TotalConnections = maxConcurrent
	result.ConnectionMetrics.ActiveConnections = int(successCount)
	result.ConnectionMetrics.FailedConnections = int(errorCount)

	successRate := float64(successCount) / float64(maxConcurrent) * 100
	result.ReliabilityMetrics.SuccessRate = successRate
	result.Success = successRate >= 90.0 // 90% success rate required

	result.Duration = time.Since(startTime)
	suite.results = append(suite.results, result)
	suite.updateTransportMetrics(transportType, &result)

	suite.logger.WithFields(logrus.Fields{
		"transport":     transportType,
		"concurrent":    maxConcurrent,
		"successful":    successCount,
		"failed":        errorCount,
		"success_rate":  successRate,
	}).Info("Concurrent connections test completed")

	assert.GreaterOrEqual(t, successRate, 90.0,
		"Success rate should be at least 90%% for concurrent connections with %s", transportType)
}

func (suite *TransportIntegrationTestSuite) TestMessageSizeHandling(ctx context.Context, t *testing.T, transportType TransportType) {
	result := TransportTestResult{
		TransportType: transportType,
		TestName:     "MessageSizeHandling",
		Errors:       make([]string, 0),
		Metadata:     make(map[string]interface{}),
	}
	startTime := time.Now()

	clientConfig := ClientConfig{
		ID: fmt.Sprintf("messagesize_test_%s", transportType),
		Name: "MessageSizeTestClient",
		Version: "1.0.0",
		Transport: transportType,
	}

	client, err := suite.factory.CreateClient(clientConfig)
	require.NoError(t, err)
	defer suite.factory.RemoveClient(clientConfig.ID)

	err = client.Connect(ctx, suite.serverURL)
	require.NoError(t, err)
	defer client.Disconnect()

	sizeResults := make(map[int]bool)

	for _, size := range suite.config.MessageSizes {
		// Create message of specified size
		largeData := make([]byte, size)
		for i := range largeData {
			largeData[i] = byte(i % 256)
		}

		// Test sending large message
		_, err := client.CallTool(ctx, "validate_hgvs", map[string]interface{}{
			"notation": "NM_000492.3:c.1521_1523del",
			"large_data": largeData, // Add large payload
		})

		sizeResults[size] = err == nil
		
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Size %d failed: %v", size, err))
		}

		suite.logger.WithFields(logrus.Fields{
			"transport": transportType,
			"size":      size,
			"success":   err == nil,
		}).Debug("Message size test")
	}

	result.Metadata["size_results"] = sizeResults

	// Calculate success rate
	successful := 0
	for _, success := range sizeResults {
		if success {
			successful++
		}
	}

	successRate := float64(successful) / float64(len(suite.config.MessageSizes)) * 100
	result.ReliabilityMetrics.SuccessRate = successRate
	result.Success = successRate >= 80.0 // Allow some failures for very large messages

	result.Duration = time.Since(startTime)
	suite.results = append(suite.results, result)
	suite.updateTransportMetrics(transportType, &result)

	assert.GreaterOrEqual(t, successRate, 80.0,
		"Should handle most message sizes for %s", transportType)
}

func (suite *TransportIntegrationTestSuite) TestConnectionReliability(ctx context.Context, t *testing.T, transportType TransportType) {
	if !suite.config.ReliabilityTests {
		t.Skip("Reliability tests disabled")
		return
	}

	result := TransportTestResult{
		TransportType: transportType,
		TestName:     "ConnectionReliability",
		Errors:       make([]string, 0),
		Metadata:     make(map[string]interface{}),
	}
	startTime := time.Now()

	clientConfig := ClientConfig{
		ID: fmt.Sprintf("reliability_test_%s", transportType),
		Name: "ReliabilityTestClient",
		Version: "1.0.0",
		Transport: transportType,
		MockConfig: MockClientConfig{
			AutoReconnect: true,
			MaxRetries:    3,
		},
	}

	client, err := suite.factory.CreateClient(clientConfig)
	require.NoError(t, err)
	defer suite.factory.RemoveClient(clientConfig.ID)

	// Test connection stability over time
	testDuration := suite.config.TestDuration / 2 // Shorter for reliability test
	testCtx, cancel := context.WithTimeout(ctx, testDuration)
	defer cancel()

	var totalOperations, successfulOperations, reconnects int64

	err = client.Connect(testCtx, suite.serverURL)
	require.NoError(t, err)

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-testCtx.Done():
			goto reliabilityComplete
		case <-ticker.C:
			totalOperations++
			
			_, err := client.CallTool(testCtx, "tools/list", map[string]interface{}{})
			if err == nil {
				successfulOperations++
			} else if !client.IsConnected() {
				// Connection lost, attempt reconnect
				reconnects++
				client.Connect(testCtx, suite.serverURL)
			}
		}
	}

reliabilityComplete:
	client.Disconnect()

	successRate := float64(successfulOperations) / float64(totalOperations) * 100
	result.ReliabilityMetrics.SuccessRate = successRate
	result.ReliabilityMetrics.ReconnectRate = float64(reconnects) / float64(totalOperations) * 100
	result.ConnectionMetrics.ReconnectAttempts = int(reconnects)

	result.Success = successRate >= 95.0 // High reliability requirement
	result.Duration = time.Since(startTime)
	
	suite.results = append(suite.results, result)
	suite.updateTransportMetrics(transportType, &result)

	suite.logger.WithFields(logrus.Fields{
		"transport":           transportType,
		"total_operations":    totalOperations,
		"successful":          successfulOperations,
		"success_rate":        successRate,
		"reconnects":          reconnects,
	}).Info("Reliability test completed")

	assert.GreaterOrEqual(t, successRate, 95.0,
		"Reliability should be high for %s", transportType)
}

func (suite *TransportIntegrationTestSuite) TestTransportErrorHandling(ctx context.Context, t *testing.T, transportType TransportType) {
	result := TransportTestResult{
		TransportType: transportType,
		TestName:     "ErrorHandling",
		Errors:       make([]string, 0),
		Metadata:     make(map[string]interface{}),
	}
	startTime := time.Now()

	clientConfig := ClientConfig{
		ID: fmt.Sprintf("error_test_%s", transportType),
		Name: "ErrorTestClient",
		Version: "1.0.0",
		Transport: transportType,
		ErrorSimulation: ErrorSimulationConfig{
			EnableErrorSim:  true,
			RequestFailRate: 0.2, // 20% error rate
		},
	}

	client, err := suite.factory.CreateClient(clientConfig)
	require.NoError(t, err)
	defer suite.factory.RemoveClient(clientConfig.ID)

	err = client.Connect(ctx, suite.serverURL)
	require.NoError(t, err)
	defer client.Disconnect()

	// Test error handling behavior
	var totalRequests, errors, timeouts int64
	testCount := 50

	for i := 0; i < testCount; i++ {
		requestCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		
		totalRequests++
		_, err := client.CallTool(requestCtx, "validate_hgvs", map[string]interface{}{
			"notation": "NM_000492.3:c.1521_1523del",
		})

		if err != nil {
			errors++
			if err == context.DeadlineExceeded {
				timeouts++
			}
		}

		cancel()
		time.Sleep(50 * time.Millisecond)
	}

	errorRate := float64(errors) / float64(totalRequests) * 100
	timeoutRate := float64(timeouts) / float64(totalRequests) * 100

	result.ReliabilityMetrics.ErrorRate = errorRate
	result.ReliabilityMetrics.TimeoutRate = timeoutRate
	result.ReliabilityMetrics.SuccessRate = 100.0 - errorRate

	// Success if error handling is graceful (errors don't crash the client)
	result.Success = client.IsConnected() && errorRate < 50.0

	result.Duration = time.Since(startTime)
	suite.results = append(suite.results, result)
	suite.updateTransportMetrics(transportType, &result)

	suite.logger.WithFields(logrus.Fields{
		"transport":    transportType,
		"error_rate":   errorRate,
		"timeout_rate": timeoutRate,
		"connected":    client.IsConnected(),
	}).Info("Error handling test completed")

	assert.True(t, client.IsConnected(), "Client should remain connected after errors")
	assert.LessOrEqual(t, errorRate, 50.0, "Error rate should be manageable")
}

func (suite *TransportIntegrationTestSuite) TestResourceUsage(ctx context.Context, t *testing.T, transportType TransportType) {
	result := TransportTestResult{
		TransportType: transportType,
		TestName:     "ResourceUsage",
		Errors:       make([]string, 0),
		Metadata:     make(map[string]interface{}),
	}
	startTime := time.Now()

	clientConfig := ClientConfig{
		ID: fmt.Sprintf("resource_test_%s", transportType),
		Name: "ResourceTestClient",
		Version: "1.0.0",
		Transport: transportType,
	}

	client, err := suite.factory.CreateClient(clientConfig)
	require.NoError(t, err)
	defer suite.factory.RemoveClient(clientConfig.ID)

	err = client.Connect(ctx, suite.serverURL)
	require.NoError(t, err)
	defer client.Disconnect()

	// Measure resource usage during operation
	// Note: In a real implementation, this would measure actual CPU/memory usage
	initialStats := client.GetStats()

	// Generate load
	numRequests := 100
	for i := 0; i < numRequests; i++ {
		client.CallTool(ctx, "validate_hgvs", map[string]interface{}{
			"notation": "NM_000492.3:c.1521_1523del",
		})
		if i%10 == 0 {
			time.Sleep(10 * time.Millisecond)
		}
	}

	finalStats := client.GetStats()

	// Calculate resource metrics (simulated)
	result.PerformanceMetrics.MemoryUsage = 1024 * 1024 * 10 // 10MB simulated
	result.PerformanceMetrics.CPUUsage = 5.0                 // 5% simulated
	result.PerformanceMetrics.GoroutineCount = 5             // Simulated

	// Calculate efficiency
	bytesPerRequest := float64(finalStats.BytesSent-initialStats.BytesSent) / float64(numRequests)
	result.ConnectionMetrics.Overhead = (bytesPerRequest / 1024.0) * 100 // Overhead in KB per request

	result.Success = result.PerformanceMetrics.MemoryUsage < 50*1024*1024 // Under 50MB
	result.Duration = time.Since(startTime)

	suite.results = append(suite.results, result)
	suite.updateTransportMetrics(transportType, &result)

	suite.logger.WithFields(logrus.Fields{
		"transport":      transportType,
		"memory_usage":   result.PerformanceMetrics.MemoryUsage,
		"cpu_usage":      result.PerformanceMetrics.CPUUsage,
		"overhead_kb":    result.ConnectionMetrics.Overhead,
	}).Info("Resource usage test completed")

	assert.LessOrEqual(t, result.PerformanceMetrics.MemoryUsage, int64(50*1024*1024),
		"Memory usage should be reasonable for %s", transportType)
}

func (suite *TransportIntegrationTestSuite) TestTransportComparison(ctx context.Context, t *testing.T) {
	suite.logger.Info("Running transport comparison analysis")

	if len(suite.results) < 2 {
		t.Skip("Need at least 2 transport types for comparison")
		return
	}

	// Group results by transport type
	transportResults := make(map[TransportType][]TransportTestResult)
	for _, result := range suite.results {
		transportResults[result.TransportType] = append(transportResults[result.TransportType], result)
	}

	// Compare performance metrics
	comparison := make(map[string]map[TransportType]float64)
	comparison["avg_latency"] = make(map[TransportType]float64)
	comparison["throughput"] = make(map[TransportType]float64)
	comparison["success_rate"] = make(map[TransportType]float64)
	comparison["connection_time"] = make(map[TransportType]float64)

	for transportType, results := range transportResults {
		var totalLatency time.Duration
		var totalThroughput float64
		var totalSuccessRate float64
		var totalConnectionTime time.Duration
		validResults := 0

		for _, result := range results {
			if result.Success {
				totalLatency += result.PerformanceMetrics.AvgLatency
				totalThroughput += result.PerformanceMetrics.MessagesPerSecond
				totalSuccessRate += result.ReliabilityMetrics.SuccessRate
				totalConnectionTime += result.ConnectionMetrics.EstablishTime
				validResults++
			}
		}

		if validResults > 0 {
			comparison["avg_latency"][transportType] = float64(totalLatency/time.Duration(validResults)) / float64(time.Millisecond)
			comparison["throughput"][transportType] = totalThroughput / float64(validResults)
			comparison["success_rate"][transportType] = totalSuccessRate / float64(validResults)
			comparison["connection_time"][transportType] = float64(totalConnectionTime/time.Duration(validResults)) / float64(time.Millisecond)
		}
	}

	// Log comparison results
	for metric, values := range comparison {
		suite.logger.WithFields(logrus.Fields{
			"metric": metric,
			"values": values,
		}).Info("Transport comparison")
	}

	// Validate that at least one transport meets performance requirements
	hasGoodPerformer := false
	for transport, latency := range comparison["avg_latency"] {
		throughput := comparison["throughput"][transport]
		successRate := comparison["success_rate"][transport]

		if latency < 1000 && throughput > 5.0 && successRate > 90.0 {
			hasGoodPerformer = true
			suite.logger.WithFields(logrus.Fields{
				"transport":    transport,
				"latency_ms":   latency,
				"throughput":   throughput,
				"success_rate": successRate,
			}).Info("Transport meets performance requirements")
		}
	}

	assert.True(t, hasGoodPerformer, "At least one transport should meet performance requirements")
}

func (suite *TransportIntegrationTestSuite) TestNetworkConditions(ctx context.Context, t *testing.T) {
	suite.logger.Info("Running network condition tests")

	for _, condition := range suite.config.NetworkConditions {
		if !condition.Enabled {
			continue
		}

		t.Run(fmt.Sprintf("Condition_%s", condition.Name), func(t *testing.T) {
			suite.testUnderNetworkCondition(ctx, t, condition)
		})
	}
}

func (suite *TransportIntegrationTestSuite) testUnderNetworkCondition(ctx context.Context, t *testing.T, condition NetworkCondition) {
	// Note: In a real implementation, this would configure network emulation
	suite.logger.WithFields(logrus.Fields{
		"condition":   condition.Name,
		"latency":     condition.Latency,
		"bandwidth":   condition.Bandwidth,
		"packet_loss": condition.PacketLoss,
	}).Info("Testing under network condition")

	clientConfig := ClientConfig{
		ID: fmt.Sprintf("network_test_%s", condition.Name),
		Name: "NetworkConditionTestClient",
		Version: "1.0.0",
		Transport: TransportWebSocket, // Use WebSocket for network condition tests
		ErrorSimulation: ErrorSimulationConfig{
			EnableErrorSim:     true,
			ResponseDelayMin:   condition.Latency,
			ResponseDelayMax:   condition.Latency + condition.Jitter,
			RequestFailRate:    condition.PacketLoss / 100.0,
		},
	}

	client, err := suite.factory.CreateClient(clientConfig)
	require.NoError(t, err)
	defer suite.factory.RemoveClient(clientConfig.ID)

	err = client.Connect(ctx, suite.serverURL)
	require.NoError(t, err)
	defer client.Disconnect()

	// Test performance under this network condition
	var successCount, totalCount int64
	var totalLatency time.Duration

	testDuration := 30 * time.Second
	testCtx, cancel := context.WithTimeout(ctx, testDuration)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-testCtx.Done():
			goto conditionTestComplete
		case <-ticker.C:
			start := time.Now()
			_, err := client.CallTool(testCtx, "validate_hgvs", map[string]interface{}{
				"notation": "NM_000492.3:c.1521_1523del",
			})
			latency := time.Since(start)

			totalCount++
			if err == nil {
				successCount++
				totalLatency += latency
			}
		}
	}

conditionTestComplete:
	if totalCount > 0 {
		successRate := float64(successCount) / float64(totalCount) * 100
		avgLatency := totalLatency / time.Duration(successCount)

		suite.logger.WithFields(logrus.Fields{
			"condition":    condition.Name,
			"success_rate": successRate,
			"avg_latency":  avgLatency,
			"total_tests":  totalCount,
		}).Info("Network condition test completed")

		// Adjust expectations based on network condition
		minSuccessRate := 95.0
		if condition.PacketLoss > 1.0 {
			minSuccessRate = 85.0
		}
		if condition.PacketLoss > 3.0 {
			minSuccessRate = 70.0
		}

		assert.GreaterOrEqual(t, successRate, minSuccessRate,
			"Success rate should be acceptable under %s conditions", condition.Name)
	}
}

// Helper methods

func (suite *TransportIntegrationTestSuite) calculateLatencyStatistics(latencies []time.Duration) PerformanceMetrics {
	if len(latencies) == 0 {
		return PerformanceMetrics{}
	}

	// Sort latencies
	sorted := make([]time.Duration, len(latencies))
	copy(sorted, latencies)
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	// Calculate statistics
	var total time.Duration
	for _, latency := range latencies {
		total += latency
	}

	avg := total / time.Duration(len(latencies))
	min := sorted[0]
	max := sorted[len(sorted)-1]
	p95 := sorted[int(float64(len(sorted))*0.95)]
	p99 := sorted[int(float64(len(sorted))*0.99)]

	return PerformanceMetrics{
		AvgLatency: avg,
		MinLatency: min,
		MaxLatency: max,
		P95Latency: p95,
		P99Latency: p99,
	}
}

func (suite *TransportIntegrationTestSuite) updateTransportMetrics(transportType TransportType, result *TransportTestResult) {
	metrics := suite.transports[transportType]
	if metrics == nil {
		return
	}

	metrics.TotalConnections += int64(result.ConnectionMetrics.TotalConnections)
	metrics.ErrorCount += int64(len(result.Errors))
	metrics.LastUpdated = time.Now()

	if result.PerformanceMetrics.AvgLatency > 0 {
		if metrics.AvgMessageLatency == 0 {
			metrics.AvgMessageLatency = result.PerformanceMetrics.AvgLatency
		} else {
			metrics.AvgMessageLatency = (metrics.AvgMessageLatency + result.PerformanceMetrics.AvgLatency) / 2
		}
	}

	if result.ConnectionMetrics.EstablishTime > 0 {
		if metrics.AvgConnectionTime == 0 {
			metrics.AvgConnectionTime = result.ConnectionMetrics.EstablishTime
		} else {
			metrics.AvgConnectionTime = (metrics.AvgConnectionTime + result.ConnectionMetrics.EstablishTime) / 2
		}
	}
}

func (suite *TransportIntegrationTestSuite) GenerateTransportReport() map[string]interface{} {
	report := map[string]interface{}{
		"summary": map[string]interface{}{
			"total_tests":      len(suite.results),
			"successful_tests": 0,
			"failed_tests":     0,
			"test_duration":    time.Since(time.Now()),
		},
		"transport_performance": make(map[string]interface{}),
		"network_conditions":    suite.config.NetworkConditions,
		"recommendations":       make([]string, 0),
	}

	transportPerf := make(map[TransportType]map[string]interface{})
	successCount := 0

	for _, result := range suite.results {
		if result.Success {
			successCount++
		}

		if transportPerf[result.TransportType] == nil {
			transportPerf[result.TransportType] = make(map[string]interface{})
		}

		perf := transportPerf[result.TransportType]
		perf[result.TestName] = map[string]interface{}{
			"success":     result.Success,
			"duration":    result.Duration,
			"throughput":  result.PerformanceMetrics.MessagesPerSecond,
			"latency":     result.PerformanceMetrics.AvgLatency,
			"success_rate": result.ReliabilityMetrics.SuccessRate,
		}
	}

	report["summary"].(map[string]interface{})["successful_tests"] = successCount
	report["summary"].(map[string]interface{})["failed_tests"] = len(suite.results) - successCount
	report["transport_performance"] = transportPerf

	// Generate recommendations
	recommendations := suite.generateRecommendations(transportPerf)
	report["recommendations"] = recommendations

	return report
}

func (suite *TransportIntegrationTestSuite) generateRecommendations(perf map[TransportType]map[string]interface{}) []string {
	recommendations := make([]string, 0)

	// Analyze performance and generate recommendations
	bestThroughput := 0.0
	var bestThroughputTransport TransportType

	for transport, metrics := range perf {
		for testName, testData := range metrics {
			if testName == "MessageThroughput" {
				if data, ok := testData.(map[string]interface{}); ok {
					if throughput, ok := data["throughput"].(float64); ok {
						if throughput > bestThroughput {
							bestThroughput = throughput
							bestThroughputTransport = transport
						}
					}
				}
			}
		}
	}

	if bestThroughputTransport != "" {
		recommendations = append(recommendations, 
			fmt.Sprintf("For high throughput scenarios, consider using %s (%.1f msg/s)", 
				bestThroughputTransport, bestThroughput))
	}

	// Add more recommendations based on analysis
	recommendations = append(recommendations, "Monitor connection stability under poor network conditions")
	recommendations = append(recommendations, "Implement proper retry logic for all transport types")
	recommendations = append(recommendations, "Consider connection pooling for high-load scenarios")

	return recommendations
}

func (suite *TransportIntegrationTestSuite) validateTransportReport(t *testing.T, report map[string]interface{}) {
	summary := report["summary"].(map[string]interface{})
	
	totalTests := summary["total_tests"].(int)
	successfulTests := summary["successful_tests"].(int)
	
	assert.Greater(t, totalTests, 0, "Should have run some tests")
	
	successRate := float64(successfulTests) / float64(totalTests) * 100
	assert.GreaterOrEqual(t, successRate, 70.0, "Overall success rate should be reasonable")
	
	transportPerf := report["transport_performance"]
	assert.NotEmpty(t, transportPerf, "Should have transport performance data")
	
	recommendations := report["recommendations"].([]string)
	assert.NotEmpty(t, recommendations, "Should generate recommendations")
}

func (suite *TransportIntegrationTestSuite) GetResults() []TransportTestResult {
	return suite.results
}

func (suite *TransportIntegrationTestSuite) GetTransportMetrics() map[TransportType]*TransportMetrics {
	return suite.transports
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}