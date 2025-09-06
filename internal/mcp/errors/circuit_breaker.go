package errors

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// CircuitBreakerManager manages circuit breakers for external dependencies
type CircuitBreakerManager struct {
	breakers map[string]*CircuitBreaker
	mutex    sync.RWMutex
	config   CircuitBreakerConfig
}

// CircuitBreakerConfig configures circuit breaker behavior
type CircuitBreakerConfig struct {
	DefaultThreshold      int           `json:"default_threshold"`
	DefaultTimeout        time.Duration `json:"default_timeout"`
	DefaultHalfOpenLimit  int           `json:"default_half_open_limit"`
	MonitoringInterval    time.Duration `json:"monitoring_interval"`
	MetricsRetentionPeriod time.Duration `json:"metrics_retention_period"`
}

// CircuitBreakerResult represents the result of a circuit breaker operation
type CircuitBreakerResult struct {
	Allowed       bool                   `json:"allowed"`
	State         string                 `json:"state"`
	FailureCount  int                    `json:"failure_count"`
	SuccessCount  int                    `json:"success_count"`
	Reason        string                 `json:"reason,omitempty"`
	NextRetry     *time.Time             `json:"next_retry,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// CircuitBreakerMetrics holds metrics for circuit breaker analysis
type CircuitBreakerMetrics struct {
	Name                string        `json:"name"`
	State               string        `json:"state"`
	TotalRequests       int64         `json:"total_requests"`
	SuccessfulRequests  int64         `json:"successful_requests"`
	FailedRequests      int64         `json:"failed_requests"`
	RejectedRequests    int64         `json:"rejected_requests"`
	AverageResponseTime time.Duration `json:"average_response_time"`
	LastStateChange     time.Time     `json:"last_state_change"`
	SuccessRate         float64       `json:"success_rate"`
	FailureRate         float64       `json:"failure_rate"`
}

// NewCircuitBreakerManager creates a new circuit breaker manager
func NewCircuitBreakerManager(config CircuitBreakerConfig) *CircuitBreakerManager {
	if config.DefaultThreshold == 0 {
		config.DefaultThreshold = 5
	}
	if config.DefaultTimeout == 0 {
		config.DefaultTimeout = 60 * time.Second
	}
	if config.DefaultHalfOpenLimit == 0 {
		config.DefaultHalfOpenLimit = 3
	}
	if config.MonitoringInterval == 0 {
		config.MonitoringInterval = 30 * time.Second
	}
	if config.MetricsRetentionPeriod == 0 {
		config.MetricsRetentionPeriod = 24 * time.Hour
	}

	return &CircuitBreakerManager{
		breakers: make(map[string]*CircuitBreaker),
		config:   config,
	}
}

// GetOrCreateCircuitBreaker gets or creates a circuit breaker for a service
func (cbm *CircuitBreakerManager) GetOrCreateCircuitBreaker(serviceName string, options ...CircuitBreakerOption) *CircuitBreaker {
	cbm.mutex.Lock()
	defer cbm.mutex.Unlock()

	if breaker, exists := cbm.breakers[serviceName]; exists {
		return breaker
	}

	// Create new circuit breaker with default settings
	breaker := &CircuitBreaker{
		Name:          serviceName,
		State:         CircuitBreakerClosed,
		Threshold:     cbm.config.DefaultThreshold,
		Timeout:       cbm.config.DefaultTimeout,
		HalfOpenLimit: cbm.config.DefaultHalfOpenLimit,
		metrics:       NewCircuitBreakerMetrics(serviceName),
	}

	// Apply options
	for _, option := range options {
		option(breaker)
	}

	cbm.breakers[serviceName] = breaker
	return breaker
}

// CircuitBreakerOption defines functional options for circuit breaker creation
type CircuitBreakerOption func(*CircuitBreaker)

// WithThreshold sets the failure threshold
func WithThreshold(threshold int) CircuitBreakerOption {
	return func(cb *CircuitBreaker) {
		cb.Threshold = threshold
	}
}

// WithTimeout sets the timeout duration
func WithTimeout(timeout time.Duration) CircuitBreakerOption {
	return func(cb *CircuitBreaker) {
		cb.Timeout = timeout
	}
}

// WithHalfOpenLimit sets the half-open state limit
func WithHalfOpenLimit(limit int) CircuitBreakerOption {
	return func(cb *CircuitBreaker) {
		cb.HalfOpenLimit = limit
	}
}

// Enhanced CircuitBreaker with metrics
type CircuitBreaker struct {
	Name           string                 `json:"name"`
	FailureCount   int                    `json:"failure_count"`
	SuccessCount   int                    `json:"success_count"`
	LastFailure    time.Time              `json:"last_failure"`
	LastSuccess    time.Time              `json:"last_success"`
	State          string                 `json:"state"`
	Threshold      int                    `json:"threshold"`
	Timeout        time.Duration          `json:"timeout"`
	HalfOpenLimit  int                    `json:"half_open_limit"`
	StateChangedAt time.Time              `json:"state_changed_at"`
	mutex          sync.RWMutex
	metrics        *CircuitBreakerMetrics
}

// NewCircuitBreakerMetrics creates new metrics for a circuit breaker
func NewCircuitBreakerMetrics(name string) *CircuitBreakerMetrics {
	return &CircuitBreakerMetrics{
		Name:            name,
		State:           CircuitBreakerClosed,
		LastStateChange: time.Now(),
	}
}

// Call executes a function with circuit breaker protection
func (cb *CircuitBreaker) Call(ctx context.Context, operation func(context.Context) error) error {
	result := cb.CanExecute()
	if !result.Allowed {
		cb.metrics.RejectedRequests++
		return &MCPError{
			Code:    ErrorServiceUnavailable,
			Message: fmt.Sprintf("Service %s unavailable: %s", cb.Name, result.Reason),
			Data: map[string]interface{}{
				"service":       cb.Name,
				"state":         result.State,
				"failure_count": result.FailureCount,
				"next_retry":    result.NextRetry,
			},
			Severity:    SeverityHigh,
			Category:    CategoryExternal,
			Recoverable: true,
			RetryAfter:  &cb.Timeout,
			Suggestions: []string{
				"Retry after the circuit breaker timeout period",
				"Check service health and connectivity",
				"Consider using alternative services or fallback mechanisms",
			},
		}
	}

	// Record request start
	start := time.Now()
	cb.metrics.TotalRequests++

	// Execute operation
	err := operation(ctx)
	
	// Record operation result
	duration := time.Since(start)
	cb.recordResult(err == nil, duration)

	return err
}

// CanExecute determines if a request can be executed based on circuit breaker state
func (cb *CircuitBreaker) CanExecute() CircuitBreakerResult {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()

	result := CircuitBreakerResult{
		State:        cb.State,
		FailureCount: cb.FailureCount,
		SuccessCount: cb.SuccessCount,
		Metadata:     make(map[string]interface{}),
	}

	switch cb.State {
	case CircuitBreakerClosed:
		result.Allowed = true
		result.Reason = "Circuit breaker is closed, allowing requests"

	case CircuitBreakerOpen:
		if time.Since(cb.StateChangedAt) >= cb.Timeout {
			// Transition to half-open
			cb.transitionToHalfOpen()
			result.Allowed = true
			result.State = CircuitBreakerHalfOpen
			result.Reason = "Circuit breaker transitioned to half-open, allowing limited requests"
		} else {
			result.Allowed = false
			result.Reason = "Circuit breaker is open, rejecting requests"
			nextRetry := cb.StateChangedAt.Add(cb.Timeout)
			result.NextRetry = &nextRetry
		}

	case CircuitBreakerHalfOpen:
		if cb.SuccessCount < cb.HalfOpenLimit {
			result.Allowed = true
			result.Reason = fmt.Sprintf("Circuit breaker is half-open, allowing request (%d/%d)", cb.SuccessCount+1, cb.HalfOpenLimit)
		} else {
			result.Allowed = false
			result.Reason = "Circuit breaker half-open limit reached"
		}

	default:
		result.Allowed = false
		result.Reason = "Unknown circuit breaker state"
	}

	return result
}

// recordResult records the result of an operation
func (cb *CircuitBreaker) recordResult(success bool, duration time.Duration) {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	if success {
		cb.SuccessCount++
		cb.LastSuccess = time.Now()
		cb.metrics.SuccessfulRequests++
		cb.handleSuccess()
	} else {
		cb.FailureCount++
		cb.LastFailure = time.Now()
		cb.metrics.FailedRequests++
		cb.handleFailure()
	}

	// Update average response time
	cb.updateAverageResponseTime(duration)
	cb.updateSuccessRate()
}

// handleSuccess handles a successful operation
func (cb *CircuitBreaker) handleSuccess() {
	switch cb.State {
	case CircuitBreakerHalfOpen:
		if cb.SuccessCount >= cb.HalfOpenLimit {
			cb.transitionToClosed()
		}
	case CircuitBreakerClosed:
		// Reset failure count on success in closed state
		cb.FailureCount = 0
	}
}

// handleFailure handles a failed operation
func (cb *CircuitBreaker) handleFailure() {
	switch cb.State {
	case CircuitBreakerClosed:
		if cb.FailureCount >= cb.Threshold {
			cb.transitionToOpen()
		}
	case CircuitBreakerHalfOpen:
		// Any failure in half-open state transitions back to open
		cb.transitionToOpen()
	}
}

// transitionToOpen transitions circuit breaker to open state
func (cb *CircuitBreaker) transitionToOpen() {
	cb.State = CircuitBreakerOpen
	cb.StateChangedAt = time.Now()
	cb.SuccessCount = 0
	cb.metrics.State = CircuitBreakerOpen
	cb.metrics.LastStateChange = cb.StateChangedAt
}

// transitionToHalfOpen transitions circuit breaker to half-open state
func (cb *CircuitBreaker) transitionToHalfOpen() {
	cb.State = CircuitBreakerHalfOpen
	cb.StateChangedAt = time.Now()
	cb.SuccessCount = 0
	cb.metrics.State = CircuitBreakerHalfOpen
	cb.metrics.LastStateChange = cb.StateChangedAt
}

// transitionToClosed transitions circuit breaker to closed state
func (cb *CircuitBreaker) transitionToClosed() {
	cb.State = CircuitBreakerClosed
	cb.StateChangedAt = time.Now()
	cb.FailureCount = 0
	cb.SuccessCount = 0
	cb.metrics.State = CircuitBreakerClosed
	cb.metrics.LastStateChange = cb.StateChangedAt
}

// updateAverageResponseTime updates the average response time metric
func (cb *CircuitBreaker) updateAverageResponseTime(duration time.Duration) {
	// Simple moving average implementation
	totalRequests := cb.metrics.TotalRequests
	if totalRequests == 1 {
		cb.metrics.AverageResponseTime = duration
	} else {
		// Weighted average with more weight on recent requests
		weight := 0.1
		cb.metrics.AverageResponseTime = time.Duration(
			float64(cb.metrics.AverageResponseTime)*(1-weight) + float64(duration)*weight,
		)
	}
}

// updateSuccessRate updates success and failure rates
func (cb *CircuitBreaker) updateSuccessRate() {
	total := cb.metrics.TotalRequests
	if total == 0 {
		cb.metrics.SuccessRate = 0
		cb.metrics.FailureRate = 0
		return
	}

	cb.metrics.SuccessRate = float64(cb.metrics.SuccessfulRequests) / float64(total)
	cb.metrics.FailureRate = float64(cb.metrics.FailedRequests) / float64(total)
}

// GetMetrics returns current circuit breaker metrics
func (cb *CircuitBreaker) GetMetrics() CircuitBreakerMetrics {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()

	// Return copy to avoid mutation
	metrics := *cb.metrics
	return metrics
}

// Reset resets the circuit breaker to initial state
func (cb *CircuitBreaker) Reset() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	cb.FailureCount = 0
	cb.SuccessCount = 0
	cb.State = CircuitBreakerClosed
	cb.StateChangedAt = time.Now()
	
	// Reset metrics
	cb.metrics.State = CircuitBreakerClosed
	cb.metrics.LastStateChange = cb.StateChangedAt
	cb.metrics.TotalRequests = 0
	cb.metrics.SuccessfulRequests = 0
	cb.metrics.FailedRequests = 0
	cb.metrics.RejectedRequests = 0
	cb.metrics.SuccessRate = 0
	cb.metrics.FailureRate = 0
}

// GetAllCircuitBreakers returns all registered circuit breakers
func (cbm *CircuitBreakerManager) GetAllCircuitBreakers() map[string]*CircuitBreaker {
	cbm.mutex.RLock()
	defer cbm.mutex.RUnlock()

	// Return copy to avoid external mutation
	result := make(map[string]*CircuitBreaker)
	for name, breaker := range cbm.breakers {
		result[name] = breaker
	}

	return result
}

// GetCircuitBreakerMetrics returns metrics for all circuit breakers
func (cbm *CircuitBreakerManager) GetCircuitBreakerMetrics() map[string]CircuitBreakerMetrics {
	cbm.mutex.RLock()
	defer cbm.mutex.RUnlock()

	metrics := make(map[string]CircuitBreakerMetrics)
	for name, breaker := range cbm.breakers {
		metrics[name] = breaker.GetMetrics()
	}

	return metrics
}

// MonitorCircuitBreakers starts monitoring circuit breaker health
func (cbm *CircuitBreakerManager) MonitorCircuitBreakers(ctx context.Context, callback func(string, CircuitBreakerMetrics)) {
	ticker := time.NewTicker(cbm.config.MonitoringInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			metrics := cbm.GetCircuitBreakerMetrics()
			for name, metric := range metrics {
				if callback != nil {
					callback(name, metric)
				}
			}
		}
	}
}

// ResetCircuitBreaker manually resets a specific circuit breaker
func (cbm *CircuitBreakerManager) ResetCircuitBreaker(serviceName string) error {
	cbm.mutex.RLock()
	defer cbm.mutex.RUnlock()

	breaker, exists := cbm.breakers[serviceName]
	if !exists {
		return fmt.Errorf("circuit breaker for service %s not found", serviceName)
	}

	breaker.Reset()
	return nil
}

// ResetAllCircuitBreakers resets all circuit breakers
func (cbm *CircuitBreakerManager) ResetAllCircuitBreakers() {
	cbm.mutex.RLock()
	defer cbm.mutex.RUnlock()

	for _, breaker := range cbm.breakers {
		breaker.Reset()
	}
}

// GetCircuitBreakerStatus returns a summary status of all circuit breakers
func (cbm *CircuitBreakerManager) GetCircuitBreakerStatus() map[string]interface{} {
	metrics := cbm.GetCircuitBreakerMetrics()
	
	status := map[string]interface{}{
		"total_breakers": len(metrics),
		"by_state":       make(map[string]int),
		"unhealthy":      []string{},
		"summary":        make(map[string]interface{}),
	}

	stateCounts := make(map[string]int)
	var totalRequests int64
	var totalSuccessful int64
	var unhealthy []string

	for name, metric := range metrics {
		stateCounts[metric.State]++
		totalRequests += metric.TotalRequests
		totalSuccessful += metric.SuccessfulRequests

		// Consider unhealthy if open or high failure rate
		if metric.State == CircuitBreakerOpen || metric.FailureRate > 0.5 {
			unhealthy = append(unhealthy, name)
		}
	}

	status["by_state"] = stateCounts
	status["unhealthy"] = unhealthy

	if totalRequests > 0 {
		status["summary"] = map[string]interface{}{
			"overall_success_rate": float64(totalSuccessful) / float64(totalRequests),
			"total_requests":       totalRequests,
		}
	}

	return status
}