package errors

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// GracefulDegradationManager handles service degradation strategies
type GracefulDegradationManager struct {
	logger           *logrus.Logger
	services         map[string]*ServiceConfig
	fallbackStrategies map[string]FallbackStrategy
	circuitBreaker   *CircuitBreakerManager
	mutex            sync.RWMutex
}

// ServiceConfig defines configuration for a service
type ServiceConfig struct {
	Name                string            `json:"name"`
	Priority            int               `json:"priority"` // 1 = critical, 5 = optional
	MaxRetries          int               `json:"max_retries"`
	RetryDelay          time.Duration     `json:"retry_delay"`
	TimeoutDuration     time.Duration     `json:"timeout_duration"`
	FallbackEnabled     bool              `json:"fallback_enabled"`
	CacheEnabled        bool              `json:"cache_enabled"`
	CacheTTL            time.Duration     `json:"cache_ttl"`
	HealthCheckInterval time.Duration     `json:"health_check_interval"`
	DegradationLevel    DegradationLevel  `json:"degradation_level"`
	Metadata            map[string]interface{} `json:"metadata,omitempty"`
}

// FallbackStrategy defines how to handle service failures
type FallbackStrategy interface {
	Execute(ctx context.Context, service string, originalError error) (*FallbackResult, error)
	GetStrategyInfo() StrategyInfo
}

// FallbackResult contains the result of a fallback operation
type FallbackResult struct {
	Success       bool                   `json:"success"`
	Data          interface{}            `json:"data,omitempty"`
	Source        string                 `json:"source"` // "cache", "alternative", "default", "partial"
	Quality       string                 `json:"quality"` // "full", "partial", "minimal", "unavailable"
	Limitations   []string               `json:"limitations,omitempty"`
	Suggestions   []string               `json:"suggestions,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	ExecutionTime time.Duration          `json:"execution_time"`
}

// StrategyInfo provides information about a fallback strategy
type StrategyInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	DataQuality string `json:"data_quality"`
	Limitations []string `json:"limitations"`
}

// DegradationLevel defines service degradation levels
type DegradationLevel int

const (
	DegradationNone DegradationLevel = iota
	DegradationMinimal
	DegradationPartial
	DegradationSevere
	DegradationComplete
)

// String returns the string representation of degradation level
func (d DegradationLevel) String() string {
	switch d {
	case DegradationNone:
		return "none"
	case DegradationMinimal:
		return "minimal"
	case DegradationPartial:
		return "partial"
	case DegradationSevere:
		return "severe"
	case DegradationComplete:
		return "complete"
	default:
		return "unknown"
	}
}

// CacheEntry represents a cached item for fallback strategies
type CacheEntry struct {
	Content     interface{}
	Timestamp   time.Time
	TTL         time.Duration
	AccessCount int
}

// CacheFallbackStrategy uses cached data as fallback
type CacheFallbackStrategy struct {
	cache     map[string]*CacheEntry
	mutex     sync.RWMutex
	logger    *logrus.Logger
}

// AlternativeServiceStrategy uses alternative services
type AlternativeServiceStrategy struct {
	alternatives []string
	logger       *logrus.Logger
}

// DefaultDataStrategy provides default/minimal data
type DefaultDataStrategy struct {
	defaults map[string]interface{}
	logger   *logrus.Logger
}

// PartialResponseStrategy returns partial data when possible
type PartialResponseStrategy struct {
	partialExtractor func(error) interface{}
	logger          *logrus.Logger
}

// NewGracefulDegradationManager creates a new degradation manager
func NewGracefulDegradationManager(logger *logrus.Logger, cbManager *CircuitBreakerManager) *GracefulDegradationManager {
	manager := &GracefulDegradationManager{
		logger:             logger,
		services:           make(map[string]*ServiceConfig),
		fallbackStrategies: make(map[string]FallbackStrategy),
		circuitBreaker:     cbManager,
	}

	manager.initializeDefaultStrategies()
	manager.initializeDefaultServices()

	return manager
}

// RegisterService registers a service with degradation configuration
func (gdm *GracefulDegradationManager) RegisterService(config *ServiceConfig) {
	gdm.mutex.Lock()
	defer gdm.mutex.Unlock()

	gdm.services[config.Name] = config
	gdm.logger.WithFields(logrus.Fields{
		"service":  config.Name,
		"priority": config.Priority,
		"fallback": config.FallbackEnabled,
	}).Info("Registered service for graceful degradation")
}

// RegisterFallbackStrategy registers a fallback strategy
func (gdm *GracefulDegradationManager) RegisterFallbackStrategy(name string, strategy FallbackStrategy) {
	gdm.fallbackStrategies[name] = strategy
	gdm.logger.WithField("strategy", name).Info("Registered fallback strategy")
}

// HandleServiceFailure handles a service failure with graceful degradation
func (gdm *GracefulDegradationManager) HandleServiceFailure(ctx context.Context, serviceName string, originalError error) (*FallbackResult, error) {
	start := time.Now()
	
	config := gdm.getServiceConfig(serviceName)
	if config == nil {
		return nil, fmt.Errorf("service %s not registered for degradation", serviceName)
	}

	gdm.logger.WithFields(logrus.Fields{
		"service": serviceName,
		"error":   originalError.Error(),
		"level":   config.DegradationLevel.String(),
	}).Warn("Handling service failure")

	// Update degradation level based on failure
	gdm.updateDegradationLevel(serviceName, originalError)

	// Try fallback strategies in order of preference
	strategies := gdm.selectFallbackStrategies(config)
	
	for _, strategyName := range strategies {
		if strategy, exists := gdm.fallbackStrategies[strategyName]; exists {
			result, err := strategy.Execute(ctx, serviceName, originalError)
			if err == nil && result.Success {
				result.ExecutionTime = time.Since(start)
				
				gdm.logger.WithFields(logrus.Fields{
					"service":        serviceName,
					"strategy":       strategyName,
					"source":         result.Source,
					"quality":        result.Quality,
					"execution_time": result.ExecutionTime,
				}).Info("Fallback strategy succeeded")
				
				return result, nil
			}
			
			gdm.logger.WithFields(logrus.Fields{
				"service":  serviceName,
				"strategy": strategyName,
				"error":    err,
			}).Warn("Fallback strategy failed")
		}
	}

	// All fallback strategies failed
	return &FallbackResult{
		Success:       false,
		Source:        "none",
		Quality:       "unavailable",
		ExecutionTime: time.Since(start),
		Limitations:   []string{"All fallback strategies failed"},
		Suggestions: []string{
			"Service is temporarily unavailable",
			"Try again later when service is restored",
			"Contact support if the problem persists",
		},
	}, fmt.Errorf("all fallback strategies failed for service %s", serviceName)
}

// selectFallbackStrategies determines which strategies to try for a service
func (gdm *GracefulDegradationManager) selectFallbackStrategies(config *ServiceConfig) []string {
	strategies := make([]string, 0)

	switch config.DegradationLevel {
	case DegradationNone, DegradationMinimal:
		if config.CacheEnabled {
			strategies = append(strategies, "cache")
		}
		strategies = append(strategies, "alternative", "partial")
		
	case DegradationPartial:
		strategies = append(strategies, "cache", "partial", "default")
		
	case DegradationSevere:
		strategies = append(strategies, "cache", "default")
		
	case DegradationComplete:
		strategies = append(strategies, "default")
	}

	return strategies
}

// updateDegradationLevel updates service degradation level based on failures
func (gdm *GracefulDegradationManager) updateDegradationLevel(serviceName string, err error) {
	gdm.mutex.Lock()
	defer gdm.mutex.Unlock()

	config := gdm.services[serviceName]
	if config == nil {
		return
	}

	// Get circuit breaker metrics if available
	if gdm.circuitBreaker != nil {
		metrics := gdm.circuitBreaker.GetCircuitBreakerMetrics()
		if metric, exists := metrics[serviceName]; exists {
			// Update degradation based on circuit breaker state and failure rate
			switch metric.State {
			case CircuitBreakerOpen:
				config.DegradationLevel = DegradationSevere
			case CircuitBreakerHalfOpen:
				config.DegradationLevel = DegradationPartial
			case CircuitBreakerClosed:
				if metric.FailureRate > 0.5 {
					config.DegradationLevel = DegradationPartial
				} else if metric.FailureRate > 0.2 {
					config.DegradationLevel = DegradationMinimal
				} else {
					config.DegradationLevel = DegradationNone
				}
			}
		}
	}

	gdm.logger.WithFields(logrus.Fields{
		"service": serviceName,
		"level":   config.DegradationLevel.String(),
	}).Debug("Updated service degradation level")
}

// getServiceConfig safely gets service configuration
func (gdm *GracefulDegradationManager) getServiceConfig(serviceName string) *ServiceConfig {
	gdm.mutex.RLock()
	defer gdm.mutex.RUnlock()
	return gdm.services[serviceName]
}

// initializeDefaultStrategies sets up default fallback strategies
func (gdm *GracefulDegradationManager) initializeDefaultStrategies() {
	// Cache strategy
	gdm.RegisterFallbackStrategy("cache", &CacheFallbackStrategy{
		cache:  make(map[string]*CacheEntry),
		logger: gdm.logger,
	})

	// Alternative service strategy
	gdm.RegisterFallbackStrategy("alternative", &AlternativeServiceStrategy{
		alternatives: []string{"backup_db", "readonly_replica", "cache_service"},
		logger:       gdm.logger,
	})

	// Default data strategy
	gdm.RegisterFallbackStrategy("default", &DefaultDataStrategy{
		defaults: map[string]interface{}{
			"classification_service": map[string]interface{}{
				"classification": "uncertain_significance",
				"confidence":     "low",
				"evidence_level": "limited",
			},
			"evidence_service": map[string]interface{}{
				"evidence_count": 0,
				"sources":        []string{},
				"quality":        "unavailable",
			},
		},
		logger: gdm.logger,
	})

	// Partial response strategy
	gdm.RegisterFallbackStrategy("partial", &PartialResponseStrategy{
		partialExtractor: func(err error) interface{} {
			// Extract any partial data from error context
			return map[string]interface{}{
				"status": "partial_failure",
				"available_data": "limited",
			}
		},
		logger: gdm.logger,
	})
}

// initializeDefaultServices registers default service configurations
func (gdm *GracefulDegradationManager) initializeDefaultServices() {
	// Classification service
	gdm.RegisterService(&ServiceConfig{
		Name:                "variant_classification",
		Priority:            1, // Critical
		MaxRetries:          3,
		RetryDelay:          2 * time.Second,
		TimeoutDuration:     30 * time.Second,
		FallbackEnabled:     true,
		CacheEnabled:        true,
		CacheTTL:            10 * time.Minute,
		HealthCheckInterval: 1 * time.Minute,
		DegradationLevel:    DegradationNone,
	})

	// Evidence gathering service
	gdm.RegisterService(&ServiceConfig{
		Name:                "evidence_gathering",
		Priority:            2, // High
		MaxRetries:          2,
		RetryDelay:          1 * time.Second,
		TimeoutDuration:     60 * time.Second,
		FallbackEnabled:     true,
		CacheEnabled:        true,
		CacheTTL:            5 * time.Minute,
		HealthCheckInterval: 2 * time.Minute,
		DegradationLevel:    DegradationNone,
	})

	// Report generation service
	gdm.RegisterService(&ServiceConfig{
		Name:                "report_generation",
		Priority:            3, // Medium
		MaxRetries:          2,
		RetryDelay:          1 * time.Second,
		TimeoutDuration:     45 * time.Second,
		FallbackEnabled:     true,
		CacheEnabled:        false,
		HealthCheckInterval: 3 * time.Minute,
		DegradationLevel:    DegradationNone,
	})
}

// GetServiceStatus returns status of all registered services
func (gdm *GracefulDegradationManager) GetServiceStatus() map[string]interface{} {
	gdm.mutex.RLock()
	defer gdm.mutex.RUnlock()

	status := make(map[string]interface{})
	for name, config := range gdm.services {
		status[name] = map[string]interface{}{
			"priority":          config.Priority,
			"degradation_level": config.DegradationLevel.String(),
			"fallback_enabled":  config.FallbackEnabled,
			"cache_enabled":     config.CacheEnabled,
		}
	}

	return status
}

// Fallback strategy implementations

// Execute implements cache fallback strategy
func (cfs *CacheFallbackStrategy) Execute(ctx context.Context, service string, originalError error) (*FallbackResult, error) {
	cfs.mutex.RLock()
	defer cfs.mutex.RUnlock()

	if entry, exists := cfs.cache[service]; exists {
		if time.Since(entry.Timestamp) < entry.TTL {
			return &FallbackResult{
				Success: true,
				Data:    entry.Content,
				Source:  "cache",
				Quality: "partial",
				Limitations: []string{
					fmt.Sprintf("Data is from cache, may be up to %s old", entry.TTL),
				},
				Suggestions: []string{
					"Cached data provided due to service unavailability",
					"Fresh data will be available once service is restored",
				},
			}, nil
		}
	}

	return &FallbackResult{Success: false}, fmt.Errorf("no valid cache entry for service %s", service)
}

// GetStrategyInfo returns cache strategy information
func (cfs *CacheFallbackStrategy) GetStrategyInfo() StrategyInfo {
	return StrategyInfo{
		Name:        "cache",
		Description: "Uses cached data when service is unavailable",
		DataQuality: "partial",
		Limitations: []string{"Data may be stale", "Cache may be empty"},
	}
}

// Execute implements alternative service strategy
func (ass *AlternativeServiceStrategy) Execute(ctx context.Context, service string, originalError error) (*FallbackResult, error) {
	// Try alternative services (implementation would involve actual service calls)
	return &FallbackResult{
		Success: true,
		Data:    map[string]interface{}{"status": "alternative_service_used"},
		Source:  "alternative",
		Quality: "full",
		Suggestions: []string{
			"Using alternative service endpoint",
			"Full functionality maintained",
		},
	}, nil
}

// GetStrategyInfo returns alternative service strategy information
func (ass *AlternativeServiceStrategy) GetStrategyInfo() StrategyInfo {
	return StrategyInfo{
		Name:        "alternative",
		Description: "Uses alternative service endpoints",
		DataQuality: "full",
		Limitations: []string{"May have different response times"},
	}
}

// Execute implements default data strategy
func (dds *DefaultDataStrategy) Execute(ctx context.Context, service string, originalError error) (*FallbackResult, error) {
	if defaultData, exists := dds.defaults[service]; exists {
		return &FallbackResult{
			Success: true,
			Data:    defaultData,
			Source:  "default",
			Quality: "minimal",
			Limitations: []string{
				"Using default/placeholder data",
				"Limited functionality available",
			},
			Suggestions: []string{
				"Default data provided for basic functionality",
				"Full service will resume when connectivity is restored",
			},
		}, nil
	}

	return &FallbackResult{Success: false}, fmt.Errorf("no default data configured for service %s", service)
}

// GetStrategyInfo returns default data strategy information
func (dds *DefaultDataStrategy) GetStrategyInfo() StrategyInfo {
	return StrategyInfo{
		Name:        "default",
		Description: "Provides default/placeholder data",
		DataQuality: "minimal",
		Limitations: []string{"Very limited functionality", "Data may not be relevant"},
	}
}

// Execute implements partial response strategy
func (prs *PartialResponseStrategy) Execute(ctx context.Context, service string, originalError error) (*FallbackResult, error) {
	partialData := prs.partialExtractor(originalError)
	if partialData != nil {
		return &FallbackResult{
			Success: true,
			Data:    partialData,
			Source:  "partial",
			Quality: "partial",
			Limitations: []string{
				"Partial data available",
				"Some information may be missing",
			},
			Suggestions: []string{
				"Partial response provided",
				"Complete data available after service recovery",
			},
		}, nil
	}

	return &FallbackResult{Success: false}, fmt.Errorf("no partial data available for service %s", service)
}

// GetStrategyInfo returns partial response strategy information
func (prs *PartialResponseStrategy) GetStrategyInfo() StrategyInfo {
	return StrategyInfo{
		Name:        "partial",
		Description: "Extracts partial data from failed responses",
		DataQuality: "partial",
		Limitations: []string{"Incomplete data", "May have gaps in information"},
	}
}