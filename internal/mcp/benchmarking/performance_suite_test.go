package benchmarking

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPerformanceSuite(t *testing.T) {
	config := BenchmarkConfig{}
	suite := NewPerformanceSuite(config)

	assert.NotNil(t, suite)
	assert.Greater(t, suite.config.Concurrency, 0)
	assert.Greater(t, suite.config.Duration, time.Duration(0))
	assert.Greater(t, suite.config.WarmupTime, time.Duration(0))
}

func TestRegisterBenchmarks(t *testing.T) {
	suite := NewPerformanceSuite(BenchmarkConfig{})

	toolFunc := func(ctx context.Context) error { return nil }
	resourceFunc := func(ctx context.Context) error { return nil }

	suite.RegisterToolBenchmark("test_tool", toolFunc)
	suite.RegisterResourceBenchmark("test_resource", resourceFunc)

	assert.Contains(t, suite.toolTests, "test_tool")
	assert.Contains(t, suite.resourceTests, "test_resource")
}

func TestRunSingleBenchmark(t *testing.T) {
	suite := NewPerformanceSuite(BenchmarkConfig{
		Concurrency: 2,
		Duration:    100 * time.Millisecond,
		WarmupTime:  10 * time.Millisecond,
	})

	executionCount := 0
	testFunc := func(ctx context.Context) error {
		executionCount++
		time.Sleep(1 * time.Millisecond) // Simulate work
		return nil
	}

	suite.RegisterToolBenchmark("test_tool", testFunc)

	result, err := suite.RunSingleBenchmark(context.Background(), "test_tool")
	require.NoError(t, err)
	assert.NotNil(t, result)

	assert.Equal(t, "test_tool", result.TestName)
	assert.Greater(t, result.TotalOperations, int64(0))
	assert.Greater(t, result.AverageLatency, time.Duration(0))
	assert.Greater(t, result.Throughput, 0.0)
	assert.Greater(t, executionCount, 0)
}

func TestRunSingleBenchmarkNotFound(t *testing.T) {
	suite := NewPerformanceSuite(BenchmarkConfig{})

	result, err := suite.RunSingleBenchmark(context.Background(), "non_existent")
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "benchmark not found")
}

func TestRunIterationBasedBenchmark(t *testing.T) {
	suite := NewPerformanceSuite(BenchmarkConfig{
		Concurrency: 2,
		Iterations:  10,
		WarmupTime:  10 * time.Millisecond,
	})

	executionCount := 0
	testFunc := func(ctx context.Context) error {
		executionCount++
		return nil
	}

	suite.RegisterToolBenchmark("iteration_test", testFunc)

	result, err := suite.RunSingleBenchmark(context.Background(), "iteration_test")
	require.NoError(t, err)

	assert.Equal(t, int64(10), result.TotalOperations)
	assert.Equal(t, 10, executionCount)
}

func TestBenchmarkWithErrors(t *testing.T) {
	suite := NewPerformanceSuite(BenchmarkConfig{
		Concurrency: 1,
		Iterations:  5,
		WarmupTime:  0, // Skip warmup for this test
	})

	errorFunc := func(ctx context.Context) error {
		return errors.New("test error")
	}

	suite.RegisterToolBenchmark("error_test", errorFunc)

	result, err := suite.RunSingleBenchmark(context.Background(), "error_test")
	require.NoError(t, err)

	assert.Equal(t, int64(5), result.ErrorCount)
	assert.Equal(t, 1.0, result.ErrorRate)
}

func TestBenchmarkStatistics(t *testing.T) {
	suite := NewPerformanceSuite(BenchmarkConfig{
		Concurrency: 1,
		Iterations:  100,
		WarmupTime:  0,
	})

	// Function with varying execution times
	counter := 0
	testFunc := func(ctx context.Context) error {
		counter++
		// Create predictable latency pattern
		sleepTime := time.Duration(counter%10) * time.Millisecond
		time.Sleep(sleepTime)
		return nil
	}

	suite.RegisterToolBenchmark("stats_test", testFunc)

	result, err := suite.RunSingleBenchmark(context.Background(), "stats_test")
	require.NoError(t, err)

	assert.Equal(t, int64(100), result.TotalOperations)
	assert.Greater(t, result.MinLatency, time.Duration(0))
	assert.Greater(t, result.MaxLatency, result.MinLatency)
	assert.Greater(t, result.AverageLatency, time.Duration(0))
	assert.Greater(t, result.P50Latency, time.Duration(0))
	assert.Greater(t, result.P90Latency, result.P50Latency)
	assert.Greater(t, result.P95Latency, result.P90Latency)
	assert.Greater(t, result.P99Latency, result.P95Latency)
	assert.Greater(t, result.Throughput, 0.0)
}

func TestRunAllBenchmarks(t *testing.T) {
	suite := NewPerformanceSuite(BenchmarkConfig{
		Concurrency:  1,
		Duration:     50 * time.Millisecond,
		WarmupTime:   5 * time.Millisecond,
		CooldownTime: 5 * time.Millisecond,
	})

	suite.RegisterToolBenchmark("tool1", func(ctx context.Context) error {
		time.Sleep(1 * time.Millisecond)
		return nil
	})

	suite.RegisterResourceBenchmark("resource1", func(ctx context.Context) error {
		time.Sleep(1 * time.Millisecond)
		return nil
	})

	summary, err := suite.RunAllBenchmarks(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, summary)

	assert.Equal(t, 2, summary.TotalTests)
	assert.Contains(t, summary.Results, "tool1")
	assert.Contains(t, summary.Results, "resource1")
	assert.Greater(t, summary.TotalOperations, int64(0))
	assert.Greater(t, summary.AverageThroughput, 0.0)

	// Check system info
	assert.NotEmpty(t, summary.SystemInfo.OS)
	assert.NotEmpty(t, summary.SystemInfo.Architecture)
	assert.Greater(t, summary.SystemInfo.NumCPU, 0)
}

func TestBenchmarkToolExecution(t *testing.T) {
	suite := NewPerformanceSuite(BenchmarkConfig{
		Concurrency: 1,
		Iterations:  5,
		WarmupTime:  0,
	})

	toolFunc := func(ctx context.Context) error {
		time.Sleep(2 * time.Millisecond)
		return nil
	}

	result, err := suite.BenchmarkToolExecution(context.Background(), "classify_variant", toolFunc)
	require.NoError(t, err)

	assert.Equal(t, "tool_classify_variant", result.TestName)
	assert.Equal(t, int64(5), result.TotalOperations)
	assert.Greater(t, result.AverageLatency, time.Millisecond)
}

func TestBenchmarkResourceAccess(t *testing.T) {
	suite := NewPerformanceSuite(BenchmarkConfig{
		Concurrency: 1,
		Iterations:  3,
		WarmupTime:  0,
	})

	resourceFunc := func(ctx context.Context) error {
		time.Sleep(1 * time.Millisecond)
		return nil
	}

	result, err := suite.BenchmarkResourceAccess(context.Background(), "variant", resourceFunc)
	require.NoError(t, err)

	assert.Equal(t, "resource_variant", result.TestName)
	assert.Equal(t, int64(3), result.TotalOperations)
}

func TestGetAndClearResults(t *testing.T) {
	suite := NewPerformanceSuite(BenchmarkConfig{
		Concurrency: 1,
		Iterations:  1,
		WarmupTime:  0,
	})

	suite.RegisterToolBenchmark("test", func(ctx context.Context) error {
		return nil
	})

	// Run benchmark to generate results
	_, err := suite.RunSingleBenchmark(context.Background(), "test")
	require.NoError(t, err)

	// Get results
	results := suite.GetResults()
	assert.Contains(t, results, "test")

	// Clear results
	suite.ClearResults()
	
	// Verify results are cleared
	results = suite.GetResults()
	assert.Empty(t, results)
}

func TestCalculatePercentile(t *testing.T) {
	suite := NewPerformanceSuite(BenchmarkConfig{})

	latencies := []time.Duration{
		1 * time.Millisecond,
		2 * time.Millisecond,
		3 * time.Millisecond,
		4 * time.Millisecond,
		5 * time.Millisecond,
	}

	p50 := suite.calculatePercentile(latencies, 0.50)
	p90 := suite.calculatePercentile(latencies, 0.90)

	assert.Equal(t, 3*time.Millisecond, p50)
	assert.Equal(t, 5*time.Millisecond, p90)

	// Test empty slice
	emptyP50 := suite.calculatePercentile([]time.Duration{}, 0.50)
	assert.Equal(t, time.Duration(0), emptyP50)
}

func TestMemoryStats(t *testing.T) {
	suite := NewPerformanceSuite(BenchmarkConfig{})

	stats1 := suite.getMemoryStats()
	assert.Greater(t, stats1.AllocBytes, uint64(0))
	assert.Greater(t, stats1.TotalAllocBytes, uint64(0))
	assert.Greater(t, stats1.SysBytes, uint64(0))

	// Allocate some memory
	data := make([]byte, 1024*1024) // 1MB
	_ = data

	stats2 := suite.getMemoryStats()
	assert.GreaterOrEqual(t, stats2.AllocBytes, stats1.AllocBytes)
}

func TestSystemInfo(t *testing.T) {
	suite := NewPerformanceSuite(BenchmarkConfig{})

	info := suite.getSystemInfo()
	assert.NotEmpty(t, info.OS)
	assert.NotEmpty(t, info.Architecture)
	assert.Greater(t, info.NumCPU, 0)
	assert.NotEmpty(t, info.GoVersion)
	assert.Greater(t, info.MemoryMB, uint64(0))
}

func TestCompareResults(t *testing.T) {
	baseline := &BenchmarkResult{
		AverageLatency: 100 * time.Millisecond,
		P95Latency:     200 * time.Millisecond,
		Throughput:     1000.0,
		ErrorRate:      0.05,
	}

	current := &BenchmarkResult{
		AverageLatency: 80 * time.Millisecond,  // 20% improvement
		P95Latency:     180 * time.Millisecond, // 10% improvement
		Throughput:     1200.0,                 // 20% improvement
		ErrorRate:      0.03,                   // 2% reduction
	}

	comparison := CompareResults(baseline, current)

	assert.Equal(t, 20.0, comparison["average_latency_improvement"])
	assert.Equal(t, 10.0, comparison["p95_latency_improvement"])
	assert.Equal(t, 20.0, comparison["throughput_improvement"])
	assert.Equal(t, -0.02, comparison["error_rate_change"])
}

func TestBenchmarkTimeout(t *testing.T) {
	suite := NewPerformanceSuite(BenchmarkConfig{
		Concurrency: 1,
		Duration:    50 * time.Millisecond,
		WarmupTime:  0,
	})

	// Function that sometimes takes longer than the benchmark duration
	testFunc := func(ctx context.Context) error {
		select {
		case <-time.After(10 * time.Millisecond):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	suite.RegisterToolBenchmark("timeout_test", testFunc)

	result, err := suite.RunSingleBenchmark(context.Background(), "timeout_test")
	require.NoError(t, err)

	// Should complete within reasonable bounds
	assert.Greater(t, result.TotalOperations, int64(0))
	assert.LessOrEqual(t, result.TotalDuration, 100*time.Millisecond) // Allow some overhead
}

func TestConcurrentBenchmark(t *testing.T) {
	suite := NewPerformanceSuite(BenchmarkConfig{
		Concurrency: 4,
		Iterations:  20,
		WarmupTime:  0,
	})

	counter := 0
	testFunc := func(ctx context.Context) error {
		counter++ // Note: This is not thread-safe, but that's intentional for testing
		time.Sleep(1 * time.Millisecond)
		return nil
	}

	suite.RegisterToolBenchmark("concurrent_test", testFunc)

	result, err := suite.RunSingleBenchmark(context.Background(), "concurrent_test")
	require.NoError(t, err)

	assert.Equal(t, int64(20), result.TotalOperations)
	// Due to concurrency, counter might be higher due to race conditions,
	// but it should be at least 20
	assert.GreaterOrEqual(t, counter, 20)
}