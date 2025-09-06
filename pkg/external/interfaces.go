package external

import (
	"context"
	"fmt"
	"time"

	"github.com/sony/gobreaker"
	"github.com/sirupsen/logrus"
)

// ExternalGeneAPI defines the common interface for gene mapping services
type ExternalGeneAPI interface {
	GetCanonicalTranscript(ctx context.Context, geneSymbol string) (*TranscriptInfo, error)
	ValidateGeneSymbol(ctx context.Context, geneSymbol string) (*GeneValidationResult, error)
	SearchGeneVariants(ctx context.Context, geneSymbol string) ([]*VariantInfo, error)
}

// UnifiedGeneAPIClient provides a unified interface with failover logic
type UnifiedGeneAPIClient struct {
	hgncClient     *HGNCClient
	refSeqClient   *RefSeqClient
	ensemblClient  *EnsemblClient
	logger         *logrus.Logger
	circuitBreaker *gobreaker.CircuitBreaker
}

// UnifiedGeneAPIConfig represents configuration for the unified client
type UnifiedGeneAPIConfig struct {
	HGNCConfig    HGNCConfig    `json:"hgnc"`
	RefSeqConfig  RefSeqConfig  `json:"refseq"`
	EnsemblConfig EnsemblConfig `json:"ensembl"`
	CircuitBreaker CircuitBreakerConfig `json:"circuit_breaker"`
}

// CircuitBreakerConfig represents circuit breaker configuration
type CircuitBreakerConfig struct {
	MaxRequests      uint32        `json:"max_requests"`
	Interval         time.Duration `json:"interval"`
	Timeout          time.Duration `json:"timeout"`
	FailureThreshold uint32        `json:"failure_threshold"`
}

// ServiceHealth represents the health status of external services
type ServiceHealth struct {
	Service   ExternalServiceType `json:"service"`
	Healthy   bool                `json:"healthy"`
	LastCheck time.Time           `json:"last_check"`
	Error     string              `json:"error,omitempty"`
}

// NewUnifiedGeneAPIClient creates a new unified gene API client with failover support
func NewUnifiedGeneAPIClient(config UnifiedGeneAPIConfig, logger *logrus.Logger) *UnifiedGeneAPIClient {
	// Set default circuit breaker configuration
	if config.CircuitBreaker.MaxRequests == 0 {
		config.CircuitBreaker.MaxRequests = 3
	}
	if config.CircuitBreaker.Interval == 0 {
		config.CircuitBreaker.Interval = 10 * time.Second
	}
	if config.CircuitBreaker.Timeout == 0 {
		config.CircuitBreaker.Timeout = 5 * time.Second
	}
	if config.CircuitBreaker.FailureThreshold == 0 {
		config.CircuitBreaker.FailureThreshold = 5
	}

	// Create circuit breaker
	cbSettings := gobreaker.Settings{
		Name:        "UnifiedGeneAPI",
		MaxRequests: config.CircuitBreaker.MaxRequests,
		Interval:    config.CircuitBreaker.Interval,
		Timeout:     config.CircuitBreaker.Timeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= config.CircuitBreaker.FailureThreshold
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			logger.WithFields(logrus.Fields{
				"circuit_breaker": name,
				"from_state":      from,
				"to_state":        to,
			}).Warn("Circuit breaker state changed")
		},
	}

	return &UnifiedGeneAPIClient{
		hgncClient:     NewHGNCClient(config.HGNCConfig),
		refSeqClient:   NewRefSeqClient(config.RefSeqConfig),
		ensemblClient:  NewEnsemblClient(config.EnsemblConfig),
		logger:         logger,
		circuitBreaker: gobreaker.NewCircuitBreaker(cbSettings),
	}
}

// GetCanonicalTranscript retrieves canonical transcript with failover logic
// Priority: HGNC (authoritative) -> RefSeq -> Ensembl
func (u *UnifiedGeneAPIClient) GetCanonicalTranscript(ctx context.Context, geneSymbol string) (*TranscriptInfo, error) {
	operation := func() (interface{}, error) {
		return u.getCanonicalTranscriptWithFailover(ctx, geneSymbol)
	}

	result, err := u.circuitBreaker.Execute(operation)
	if err != nil {
		return nil, fmt.Errorf("circuit breaker execution failed: %w", err)
	}

	return result.(*TranscriptInfo), nil
}

// ValidateGeneSymbol validates gene symbol with failover logic
// Priority: HGNC (authoritative) -> RefSeq -> Ensembl
func (u *UnifiedGeneAPIClient) ValidateGeneSymbol(ctx context.Context, geneSymbol string) (*GeneValidationResult, error) {
	operation := func() (interface{}, error) {
		return u.validateGeneSymbolWithFailover(ctx, geneSymbol)
	}

	result, err := u.circuitBreaker.Execute(operation)
	if err != nil {
		return nil, fmt.Errorf("circuit breaker execution failed: %w", err)
	}

	return result.(*GeneValidationResult), nil
}

// SearchGeneVariants searches for gene variants (delegates to variant-specific databases)
func (u *UnifiedGeneAPIClient) SearchGeneVariants(ctx context.Context, geneSymbol string) ([]*VariantInfo, error) {
	// Gene databases don't typically contain variant information
	// This would typically delegate to ClinVar, COSMIC, etc.
	return nil, fmt.Errorf("variant search not supported by gene mapping services - use variant-specific databases like ClinVar")
}

// GetServiceHealth returns health status of all external services
func (u *UnifiedGeneAPIClient) GetServiceHealth(ctx context.Context) []ServiceHealth {
	services := []struct {
		name   ExternalServiceType
		client ExternalGeneAPI
	}{
		{ServiceTypeHGNC, u.hgncClient},
		{ServiceTypeRefSeq, u.refSeqClient},
		{ServiceTypeEnsembl, u.ensemblClient},
	}

	health := make([]ServiceHealth, len(services))
	for i, service := range services {
		health[i] = ServiceHealth{
			Service:   service.name,
			LastCheck: time.Now(),
		}

		// Simple health check by trying to validate a known gene
		if _, err := service.client.ValidateGeneSymbol(ctx, "BRCA1"); err != nil {
			health[i].Healthy = false
			health[i].Error = err.Error()
			u.logger.WithFields(logrus.Fields{
				"service": service.name,
				"error":   err.Error(),
			}).Warn("Service health check failed")
		} else {
			health[i].Healthy = true
		}
	}

	return health
}

// getCanonicalTranscriptWithFailover implements failover logic for transcript resolution
func (u *UnifiedGeneAPIClient) getCanonicalTranscriptWithFailover(ctx context.Context, geneSymbol string) (*TranscriptInfo, error) {
	// Try HGNC first (authoritative source)
	if transcript, err := u.hgncClient.GetCanonicalTranscript(ctx, geneSymbol); err == nil {
		u.logger.WithFields(logrus.Fields{
			"gene_symbol": geneSymbol,
			"service":     "HGNC",
			"transcript":  transcript.RefSeqID,
		}).Debug("Successfully resolved transcript via HGNC")
		return transcript, nil
	} else {
		u.logger.WithFields(logrus.Fields{
			"gene_symbol": geneSymbol,
			"service":     "HGNC",
			"error":       err.Error(),
		}).Debug("HGNC transcript resolution failed, trying RefSeq")
	}

	// Try RefSeq as backup
	if transcript, err := u.refSeqClient.GetCanonicalTranscript(ctx, geneSymbol); err == nil {
		u.logger.WithFields(logrus.Fields{
			"gene_symbol": geneSymbol,
			"service":     "RefSeq",
			"transcript":  transcript.RefSeqID,
		}).Debug("Successfully resolved transcript via RefSeq")
		return transcript, nil
	} else {
		u.logger.WithFields(logrus.Fields{
			"gene_symbol": geneSymbol,
			"service":     "RefSeq",
			"error":       err.Error(),
		}).Debug("RefSeq transcript resolution failed, trying Ensembl")
	}

	// Try Ensembl as last resort
	if transcript, err := u.ensemblClient.GetCanonicalTranscript(ctx, geneSymbol); err == nil {
		u.logger.WithFields(logrus.Fields{
			"gene_symbol": geneSymbol,
			"service":     "Ensembl",
			"transcript":  transcript.RefSeqID,
		}).Debug("Successfully resolved transcript via Ensembl")
		return transcript, nil
	} else {
		u.logger.WithFields(logrus.Fields{
			"gene_symbol": geneSymbol,
			"service":     "Ensembl",
			"error":       err.Error(),
		}).Error("All transcript resolution services failed")
	}

	return nil, fmt.Errorf("failed to resolve transcript for gene symbol %s using all available services", geneSymbol)
}

// validateGeneSymbolWithFailover implements failover logic for gene validation
func (u *UnifiedGeneAPIClient) validateGeneSymbolWithFailover(ctx context.Context, geneSymbol string) (*GeneValidationResult, error) {
	var allErrors []string

	// Try HGNC first (authoritative source)
	if result, err := u.hgncClient.ValidateGeneSymbol(ctx, geneSymbol); err == nil {
		u.logger.WithFields(logrus.Fields{
			"gene_symbol": geneSymbol,
			"service":     "HGNC",
			"valid":       result.IsValid,
		}).Debug("Successfully validated gene symbol via HGNC")
		return result, nil
	} else {
		allErrors = append(allErrors, fmt.Sprintf("HGNC: %s", err.Error()))
		u.logger.WithFields(logrus.Fields{
			"gene_symbol": geneSymbol,
			"service":     "HGNC",
			"error":       err.Error(),
		}).Debug("HGNC gene validation failed, trying RefSeq")
	}

	// Try RefSeq as backup
	if result, err := u.refSeqClient.ValidateGeneSymbol(ctx, geneSymbol); err == nil {
		u.logger.WithFields(logrus.Fields{
			"gene_symbol": geneSymbol,
			"service":     "RefSeq",
			"valid":       result.IsValid,
		}).Debug("Successfully validated gene symbol via RefSeq")
		return result, nil
	} else {
		allErrors = append(allErrors, fmt.Sprintf("RefSeq: %s", err.Error()))
		u.logger.WithFields(logrus.Fields{
			"gene_symbol": geneSymbol,
			"service":     "RefSeq",
			"error":       err.Error(),
		}).Debug("RefSeq gene validation failed, trying Ensembl")
	}

	// Try Ensembl as last resort
	if result, err := u.ensemblClient.ValidateGeneSymbol(ctx, geneSymbol); err == nil {
		u.logger.WithFields(logrus.Fields{
			"gene_symbol": geneSymbol,
			"service":     "Ensembl",
			"valid":       result.IsValid,
		}).Debug("Successfully validated gene symbol via Ensembl")
		return result, nil
	} else {
		allErrors = append(allErrors, fmt.Sprintf("Ensembl: %s", err.Error()))
		u.logger.WithFields(logrus.Fields{
			"gene_symbol": geneSymbol,
			"service":     "Ensembl",
			"error":       err.Error(),
		}).Error("All gene validation services failed")
	}

	// Return aggregated error result
	return &GeneValidationResult{
		IsValid:          false,
		ValidationErrors: allErrors,
		Source:           ServiceTypeHGNC, // Default to HGNC as primary source
		Suggestions:      []string{"Check gene symbol spelling", "Verify gene symbol is approved by HGNC"},
	}, nil
}