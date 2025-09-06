package benchmarking

import (
	"context"
	"fmt"
	"runtime"
	"sort"
	"sync"
	"time"
)

// BenchmarkConfig defines configuration for performance benchmarking
type BenchmarkConfig struct {
	// Number of concurrent workers
	Concurrency int
	// Duration to run benchmarks
	Duration time.Duration
	// Number of iterations (if not using duration)
	Iterations int
	// Warm-up period before measurements
	WarmupTime time.Duration
	// Cool-down period between tests
	CooldownTime time.Duration
	// Enable memory profiling
	EnableMemoryProfiling bool
	// Enable CPU profiling
	EnableCPUProfiling bool
	// Sample interval for metrics
	SampleInterval time.Duration
	// Output detailed results
	DetailedResults bool
}

// BenchmarkResult represents the results of a single benchmark run
type BenchmarkResult struct {
	TestName        string        `json:"test_name"`
	TotalOperations int64         `json:"total_operations"`
	TotalDuration   time.Duration `json:"total_duration"`
	AverageLatency  time.Duration `json:"average_latency"`
	MinLatency      time.Duration `json:"min_latency"`
	MaxLatency      time.Duration `json:"max_latency"`
	P50Latency      time.Duration `json:"p50_latency"`
	P90Latency      time.Duration `json:"p90_latency"`
	P95Latency      time.Duration `json:"p95_latency"`
	P99Latency      time.Duration `json:"p99_latency"`
	Throughput      float64       `json:"throughput_ops_per_sec"`
	ErrorCount      int64         `json:"error_count"`
	ErrorRate       float64       `json:"error_rate"`
	MemoryUsage     MemoryStats   `json:"memory_usage"`
	CPUUsage        CPUStats      `json:"cpu_usage"`
	Latencies       []time.Duration `json:"-"` // Raw latency data
}

// MemoryStats tracks memory usage during benchmarks
type MemoryStats struct {
	AllocBytes      uint64 `json:"alloc_bytes"`
	TotalAllocBytes uint64 `json:"total_alloc_bytes"`
	SysBytes        uint64 `json:"sys_bytes"`
	NumGC           uint32 `json:"num_gc"`
	HeapObjects     uint64 `json:"heap_objects"`
	StackInUse      uint64 `json:"stack_in_use"`
}

// CPUStats tracks CPU usage during benchmarks
type CPUStats struct {
	UserTime   time.Duration `json:"user_time"`
	SystemTime time.Duration `json:"system_time"`
	IdleTime   time.Duration `json:"idle_time"`
	NumCPU     int           `json:"num_cpu"`
}

// ToolBenchmarkFunc represents a function to benchmark tool execution
type ToolBenchmarkFunc func(ctx context.Context) error

// ResourceBenchmarkFunc represents a function to benchmark resource access
type ResourceBenchmarkFunc func(ctx context.Context) error

// PerformanceSuite manages performance benchmarking for MCP tools and resources
type PerformanceSuite struct {
	config      BenchmarkConfig
	results     map[string]*BenchmarkResult
	resultMutex sync.RWMutex
	toolTests   map[string]ToolBenchmarkFunc
	resourceTests map[string]ResourceBenchmarkFunc
}

// BenchmarkSummary provides summary statistics across all benchmarks
type BenchmarkSummary struct {
	TotalTests      int                            `json:"total_tests"`
	TotalOperations int64                          `json:"total_operations"`
	TotalDuration   time.Duration                  `json:"total_duration"`
	AverageThroughput float64                      `json:"average_throughput"`
	Results         map[string]*BenchmarkResult    `json:"results"`
	SystemInfo      SystemInfo                     `json:"system_info"`
	Timestamp       time.Time                      `json:"timestamp"`
}

// SystemInfo contains information about the system running benchmarks
type SystemInfo struct {
	OS           string `json:"os"`
	Architecture string `json:"architecture"`
	NumCPU       int    `json:"num_cpu"`
	GoVersion    string `json:"go_version"`
	MemoryMB     uint64 `json:"memory_mb"`
}

// NewPerformanceSuite creates a new performance benchmarking suite
func NewPerformanceSuite(config BenchmarkConfig) *PerformanceSuite {
	if config.Concurrency == 0 {
		config.Concurrency = runtime.NumCPU()
	}
	if config.Duration == 0 && config.Iterations == 0 {
		config.Duration = 10 * time.Second
	}
	if config.WarmupTime == 0 {
		config.WarmupTime = 2 * time.Second
	}
	if config.CooldownTime == 0 {
		config.CooldownTime = 1 * time.Second
	}
	if config.SampleInterval == 0 {
		config.SampleInterval = 100 * time.Millisecond
	}

	return &PerformanceSuite{
		config:        config,
		results:       make(map[string]*BenchmarkResult),
		toolTests:     make(map[string]ToolBenchmarkFunc),
		resourceTests: make(map[string]ResourceBenchmarkFunc),
	}
}

// RegisterToolBenchmark registers a tool benchmark function
func (ps *PerformanceSuite) RegisterToolBenchmark(name string, fn ToolBenchmarkFunc) {
	ps.toolTests[name] = fn
}

// RegisterResourceBenchmark registers a resource benchmark function
func (ps *PerformanceSuite) RegisterResourceBenchmark(name string, fn ResourceBenchmarkFunc) {
	ps.resourceTests[name] = fn
}

// RunAllBenchmarks executes all registered benchmarks
func (ps *PerformanceSuite) RunAllBenchmarks(ctx context.Context) (*BenchmarkSummary, error) {
	summary := &BenchmarkSummary{
		Results:    make(map[string]*BenchmarkResult),
		SystemInfo: ps.getSystemInfo(),
		Timestamp:  time.Now(),
	}

	// Run tool benchmarks
	for name, fn := range ps.toolTests {
		result, err := ps.runBenchmark(ctx, name, func(ctx context.Context) error {
			return fn(ctx)
		})
		if err != nil {
			return nil, fmt.Errorf("tool benchmark %s failed: %w", name, err)
		}
		
		ps.resultMutex.Lock()
		ps.results[name] = result
		summary.Results[name] = result
		ps.resultMutex.Unlock()

		summary.TotalTests++
		summary.TotalOperations += result.TotalOperations
		summary.TotalDuration += result.TotalDuration

		// Cool-down between tests
		if ps.config.CooldownTime > 0 {
			time.Sleep(ps.config.CooldownTime)
		}
	}

	// Run resource benchmarks
	for name, fn := range ps.resourceTests {
		result, err := ps.runBenchmark(ctx, name, func(ctx context.Context) error {
			return fn(ctx)
		})
		if err != nil {
			return nil, fmt.Errorf("resource benchmark %s failed: %w", name, err)
		}
		
		ps.resultMutex.Lock()
		ps.results[name] = result
		summary.Results[name] = result
		ps.resultMutex.Unlock()

		summary.TotalTests++
		summary.TotalOperations += result.TotalOperations
		summary.TotalDuration += result.TotalDuration

		// Cool-down between tests
		if ps.config.CooldownTime > 0 {
			time.Sleep(ps.config.CooldownTime)
		}
	}

	// Calculate average throughput
	if summary.TotalDuration > 0 {
		summary.AverageThroughput = float64(summary.TotalOperations) / summary.TotalDuration.Seconds()
	}

	return summary, nil
}

// RunSingleBenchmark executes a specific benchmark by name
func (ps *PerformanceSuite) RunSingleBenchmark(ctx context.Context, name string) (*BenchmarkResult, error) {
	if toolFn, exists := ps.toolTests[name]; exists {
		return ps.runBenchmark(ctx, name, func(ctx context.Context) error {
			return toolFn(ctx)
		})
	}
	
	if resourceFn, exists := ps.resourceTests[name]; exists {
		return ps.runBenchmark(ctx, name, func(ctx context.Context) error {
			return resourceFn(ctx)
		})
	}

	return nil, fmt.Errorf("benchmark not found: %s", name)
}

// BenchmarkToolExecution benchmarks tool execution performance
func (ps *PerformanceSuite) BenchmarkToolExecution(ctx context.Context, toolName string, executeFunc ToolBenchmarkFunc) (*BenchmarkResult, error) {
	return ps.runBenchmark(ctx, fmt.Sprintf("tool_%s", toolName), func(ctx context.Context) error {
		return executeFunc(ctx)
	})
}

// BenchmarkResourceAccess benchmarks resource access performance
func (ps *PerformanceSuite) BenchmarkResourceAccess(ctx context.Context, resourceType string, accessFunc ResourceBenchmarkFunc) (*BenchmarkResult, error) {
	return ps.runBenchmark(ctx, fmt.Sprintf("resource_%s", resourceType), func(ctx context.Context) error {
		return accessFunc(ctx)
	})
}

// GetResults returns all benchmark results
func (ps *PerformanceSuite) GetResults() map[string]*BenchmarkResult {
	ps.resultMutex.RLock()
	defer ps.resultMutex.RUnlock()

	results := make(map[string]*BenchmarkResult)
	for name, result := range ps.results {
		results[name] = result
	}
	return results
}

// ClearResults clears all stored benchmark results
func (ps *PerformanceSuite) ClearResults() {
	ps.resultMutex.Lock()
	defer ps.resultMutex.Unlock()
	ps.results = make(map[string]*BenchmarkResult)
}

// Private helper methods

func (ps *PerformanceSuite) runBenchmark(ctx context.Context, name string, fn func(context.Context) error) (*BenchmarkResult, error) {
	result := &BenchmarkResult{
		TestName:  name,
		Latencies: make([]time.Duration, 0),
	}

	// Warm-up phase
	if ps.config.WarmupTime > 0 {
		ps.runWarmup(ctx, fn)
	}

	// Collect initial memory stats
	initialMem := ps.getMemoryStats()
	startTime := time.Now()

	// Run benchmark based on configuration
	if ps.config.Duration > 0 {
		ps.runDurationBased(ctx, fn, result)
	} else {
		ps.runIterationBased(ctx, fn, result)
	}

	endTime := time.Now()
	finalMem := ps.getMemoryStats()

	result.TotalDuration = endTime.Sub(startTime)
	
	// Calculate statistics
	ps.calculateStatistics(result)
	
	// Calculate memory usage
	result.MemoryUsage = MemoryStats{
		AllocBytes:      finalMem.AllocBytes - initialMem.AllocBytes,
		TotalAllocBytes: finalMem.TotalAllocBytes,
		SysBytes:        finalMem.SysBytes,
		NumGC:           finalMem.NumGC - initialMem.NumGC,
		HeapObjects:     finalMem.HeapObjects,
		StackInUse:      finalMem.StackInUse,
	}

	return result, nil
}

func (ps *PerformanceSuite) runWarmup(ctx context.Context, fn func(context.Context) error) {
	warmupCtx, cancel := context.WithTimeout(ctx, ps.config.WarmupTime)
	defer cancel()

	warmupWorkers := ps.config.Concurrency
	if warmupWorkers > 4 {
		warmupWorkers = 4 // Limit warmup concurrency
	}

	var wg sync.WaitGroup
	for i := 0; i < warmupWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-warmupCtx.Done():
					return
				default:
					fn(warmupCtx)
				}
			}
		}()
	}
	wg.Wait()
}

func (ps *PerformanceSuite) runDurationBased(ctx context.Context, fn func(context.Context) error, result *BenchmarkResult) {
	benchCtx, cancel := context.WithTimeout(ctx, ps.config.Duration)
	defer cancel()

	var wg sync.WaitGroup
	latencyChan := make(chan time.Duration, ps.config.Concurrency*100)
	errorChan := make(chan error, ps.config.Concurrency*10)

	for i := 0; i < ps.config.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-benchCtx.Done():
					return
				default:
					start := time.Now()
					err := fn(benchCtx)
					latency := time.Since(start)
					
					latencyChan <- latency
					if err != nil {
						errorChan <- err
					}
				}
			}
		}()
	}

	wg.Wait()
	close(latencyChan)
	close(errorChan)

	// Collect results
	for latency := range latencyChan {
		result.Latencies = append(result.Latencies, latency)
		result.TotalOperations++
	}

	for range errorChan {
		result.ErrorCount++
	}
}

func (ps *PerformanceSuite) runIterationBased(ctx context.Context, fn func(context.Context) error, result *BenchmarkResult) {
	var wg sync.WaitGroup
	latencyChan := make(chan time.Duration, ps.config.Iterations)
	errorChan := make(chan error, ps.config.Iterations)

	iterationsPerWorker := ps.config.Iterations / ps.config.Concurrency
	remainder := ps.config.Iterations % ps.config.Concurrency

	for i := 0; i < ps.config.Concurrency; i++ {
		iterations := iterationsPerWorker
		if i < remainder {
			iterations++
		}

		wg.Add(1)
		go func(iters int) {
			defer wg.Done()
			for j := 0; j < iters; j++ {
				start := time.Now()
				err := fn(ctx)
				latency := time.Since(start)
				
				latencyChan <- latency
				if err != nil {
					errorChan <- err
				}
			}
		}(iterations)
	}

	wg.Wait()
	close(latencyChan)
	close(errorChan)

	// Collect results
	for latency := range latencyChan {
		result.Latencies = append(result.Latencies, latency)
		result.TotalOperations++
	}

	for range errorChan {
		result.ErrorCount++
	}
}

func (ps *PerformanceSuite) calculateStatistics(result *BenchmarkResult) {
	if len(result.Latencies) == 0 {
		return
	}

	// Sort latencies for percentile calculations
	sort.Slice(result.Latencies, func(i, j int) bool {
		return result.Latencies[i] < result.Latencies[j]
	})

	// Basic statistics
	result.MinLatency = result.Latencies[0]
	result.MaxLatency = result.Latencies[len(result.Latencies)-1]

	// Calculate average
	var total time.Duration
	for _, latency := range result.Latencies {
		total += latency
	}
	result.AverageLatency = total / time.Duration(len(result.Latencies))

	// Calculate percentiles
	result.P50Latency = ps.calculatePercentile(result.Latencies, 0.50)
	result.P90Latency = ps.calculatePercentile(result.Latencies, 0.90)
	result.P95Latency = ps.calculatePercentile(result.Latencies, 0.95)
	result.P99Latency = ps.calculatePercentile(result.Latencies, 0.99)

	// Calculate throughput
	if result.TotalDuration > 0 {
		result.Throughput = float64(result.TotalOperations) / result.TotalDuration.Seconds()
	}

	// Calculate error rate
	if result.TotalOperations > 0 {
		result.ErrorRate = float64(result.ErrorCount) / float64(result.TotalOperations)
	}
}

func (ps *PerformanceSuite) calculatePercentile(latencies []time.Duration, percentile float64) time.Duration {
	if len(latencies) == 0 {
		return 0
	}
	
	index := int(float64(len(latencies)-1) * percentile)
	return latencies[index]
}

func (ps *PerformanceSuite) getMemoryStats() MemoryStats {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return MemoryStats{
		AllocBytes:      m.Alloc,
		TotalAllocBytes: m.TotalAlloc,
		SysBytes:        m.Sys,
		NumGC:           m.NumGC,
		HeapObjects:     m.HeapObjects,
		StackInUse:      m.StackInuse,
	}
}

func (ps *PerformanceSuite) getSystemInfo() SystemInfo {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return SystemInfo{
		OS:           runtime.GOOS,
		Architecture: runtime.GOARCH,
		NumCPU:       runtime.NumCPU(),
		GoVersion:    runtime.Version(),
		MemoryMB:     m.Sys / 1024 / 1024,
	}
}

// CompareResults compares two benchmark results and returns improvement metrics
func CompareResults(baseline, current *BenchmarkResult) map[string]float64 {
	comparison := make(map[string]float64)

	if baseline.AverageLatency > 0 {
		improvement := float64(baseline.AverageLatency-current.AverageLatency) / float64(baseline.AverageLatency)
		comparison["average_latency_improvement"] = improvement * 100
	}

	if baseline.Throughput > 0 {
		improvement := (current.Throughput - baseline.Throughput) / baseline.Throughput
		comparison["throughput_improvement"] = improvement * 100
	}

	if baseline.P95Latency > 0 {
		improvement := float64(baseline.P95Latency-current.P95Latency) / float64(baseline.P95Latency)
		comparison["p95_latency_improvement"] = improvement * 100
	}

	comparison["error_rate_change"] = current.ErrorRate - baseline.ErrorRate

	return comparison
}