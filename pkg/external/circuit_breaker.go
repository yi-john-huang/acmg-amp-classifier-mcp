package external

import (
	"context"
	"fmt"
	"time"

	"github.com/sony/gobreaker"
	"github.com/acmg-amp-mcp-server/internal/domain"
)

// CircuitBreakerConfig represents circuit breaker configuration
type CircuitBreakerConfig struct {
	MaxRequests      uint32        `json:"max_requests"`
	Interval         time.Duration `json:"interval"`
	Timeout          time.Duration `json:"timeout"`
	ReadyToTrip      func(counts gobreaker.Counts) bool
	OnStateChange    func(name string, from gobreaker.State, to gobreaker.State)
}

// ResilientExternalClient wraps external API clients with circuit breaker pattern
type ResilientExternalClient struct {
	clinVarClient *ClinVarClient
	gnomADClient  *GnomADClient
	cosmicClient  *COSMICClient
	pubMedClient  *PubMedClient
	lovdClient    *LOVDClient
	hgmdClient    *HGMDClient
	cacheClient   *CacheClient
	
	clinVarBreaker *gobreaker.CircuitBreaker
	gnomADBreaker  *gobreaker.CircuitBreaker
	cosmicBreaker  *gobreaker.CircuitBreaker
	pubMedBreaker  *gobreaker.CircuitBreaker
	lovdBreaker    *gobreaker.CircuitBreaker
	hgmdBreaker    *gobreaker.CircuitBreaker
}

// NewResilientExternalClient creates a new resilient external client with circuit breakers
func NewResilientExternalClient(
	clinVarConfig domain.ClinVarConfig,
	gnomADConfig domain.GnomADConfig,
	cosmicConfig domain.COSMICConfig,
	pubMedConfig domain.PubMedConfig,
	lovdConfig domain.LOVDConfig,
	hgmdConfig domain.HGMDConfig,
	cacheConfig domain.CacheConfig,
) (*ResilientExternalClient, error) {
	
	// Create individual clients
	clinVarClient := NewClinVarClient(clinVarConfig)
	gnomADClient := NewGnomADClient(gnomADConfig)
	cosmicClient := NewCOSMICClient(cosmicConfig)
	
	// Create new clients
	pubMedClient := NewPubMedClient(PubMedConfig{
		BaseURL:   pubMedConfig.BaseURL,
		APIKey:    pubMedConfig.APIKey,
		Email:     pubMedConfig.Email,
		Timeout:   pubMedConfig.Timeout,
		RateLimit: pubMedConfig.RateLimit,
	})
	
	lovdClient := NewLOVDClient(LOVDConfig{
		BaseURL:   lovdConfig.BaseURL,
		APIKey:    lovdConfig.APIKey,
		Timeout:   lovdConfig.Timeout,
		RateLimit: lovdConfig.RateLimit,
	})
	
	hgmdClient := NewHGMDClient(HGMDConfig{
		BaseURL:        hgmdConfig.BaseURL,
		APIKey:         hgmdConfig.APIKey,
		License:        hgmdConfig.License,
		IsProfessional: hgmdConfig.IsProfessional,
		Timeout:        hgmdConfig.Timeout,
		RateLimit:      hgmdConfig.RateLimit,
	})
	
	cacheClient, err := NewCacheClient(cacheConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache client: %w", err)
	}
	
	// Create circuit breakers for each service
	clinVarBreaker := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "ClinVar",
		MaxRequests: 5,
		Interval:    30 * time.Second,
		Timeout:     60 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 3 && failureRatio >= 0.6
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			// Log state changes for monitoring
			fmt.Printf("Circuit breaker %s changed from %v to %v\n", name, from, to)
		},
	})
	
	gnomADBreaker := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "gnomAD",
		MaxRequests: 5,
		Interval:    30 * time.Second,
		Timeout:     60 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 3 && failureRatio >= 0.6
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			fmt.Printf("Circuit breaker %s changed from %v to %v\n", name, from, to)
		},
	})
	
	cosmicBreaker := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "COSMIC",
		MaxRequests: 5,
		Interval:    30 * time.Second,
		Timeout:     60 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 3 && failureRatio >= 0.6
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			fmt.Printf("Circuit breaker %s changed from %v to %v\n", name, from, to)
		},
	})
	
	// Create circuit breakers for new services
	pubMedBreaker := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "PubMed",
		MaxRequests: 3, // More conservative for PubMed
		Interval:    30 * time.Second,
		Timeout:     60 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 2 && failureRatio >= 0.5
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			fmt.Printf("Circuit breaker %s changed from %v to %v\n", name, from, to)
		},
	})
	
	lovdBreaker := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "LOVD",
		MaxRequests: 5,
		Interval:    30 * time.Second,
		Timeout:     60 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 3 && failureRatio >= 0.6
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			fmt.Printf("Circuit breaker %s changed from %v to %v\n", name, from, to)
		},
	})
	
	hgmdBreaker := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "HGMD",
		MaxRequests: 3, // Conservative for HGMD (commercial service)
		Interval:    30 * time.Second,
		Timeout:     90 * time.Second, // Longer timeout for HGMD
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 2 && failureRatio >= 0.5
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			fmt.Printf("Circuit breaker %s changed from %v to %v\n", name, from, to)
		},
	})
	
	return &ResilientExternalClient{
		clinVarClient:  clinVarClient,
		gnomADClient:   gnomADClient,
		cosmicClient:   cosmicClient,
		pubMedClient:   pubMedClient,
		lovdClient:     lovdClient,
		hgmdClient:     hgmdClient,
		cacheClient:    cacheClient,
		clinVarBreaker: clinVarBreaker,
		gnomADBreaker:  gnomADBreaker,
		cosmicBreaker:  cosmicBreaker,
		pubMedBreaker:  pubMedBreaker,
		lovdBreaker:    lovdBreaker,
		hgmdBreaker:    hgmdBreaker,
	}, nil
}

// QueryClinVar queries ClinVar with circuit breaker and caching
func (r *ResilientExternalClient) QueryClinVar(ctx context.Context, variant *domain.StandardizedVariant) (*domain.ClinVarData, error) {
	// Check cache first
	if cachedData, found, err := r.cacheClient.GetClinVarData(ctx, variant); err == nil && found {
		return cachedData, nil
	}
	
	// Use circuit breaker
	result, err := r.clinVarBreaker.Execute(func() (interface{}, error) {
		return r.clinVarClient.QueryVariant(ctx, variant)
	})
	
	if err != nil {
		// Check if circuit breaker is open and return cached data if available
		if err == gobreaker.ErrOpenState {
			if cachedData, found, cacheErr := r.cacheClient.GetClinVarData(ctx, variant); cacheErr == nil && found {
				return cachedData, nil
			}
			return nil, fmt.Errorf("ClinVar service unavailable (circuit breaker open)")
		}
		return nil, fmt.Errorf("ClinVar query failed: %w", err)
	}
	
	data := result.(*domain.ClinVarData)
	
	// Cache the result
	if cacheErr := r.cacheClient.SetClinVarData(ctx, variant, data, 0); cacheErr != nil {
		// Log cache error but don't fail the request
		fmt.Printf("Failed to cache ClinVar data: %v\n", cacheErr)
	}
	
	return data, nil
}

// QueryGnomAD queries gnomAD with circuit breaker and caching
func (r *ResilientExternalClient) QueryGnomAD(ctx context.Context, variant *domain.StandardizedVariant) (*domain.PopulationData, error) {
	// Check cache first
	if cachedData, found, err := r.cacheClient.GetPopulationData(ctx, variant); err == nil && found {
		return cachedData, nil
	}
	
	// Use circuit breaker
	result, err := r.gnomADBreaker.Execute(func() (interface{}, error) {
		return r.gnomADClient.QueryVariant(ctx, variant)
	})
	
	if err != nil {
		// Check if circuit breaker is open and return cached data if available
		if err == gobreaker.ErrOpenState {
			if cachedData, found, cacheErr := r.cacheClient.GetPopulationData(ctx, variant); cacheErr == nil && found {
				return cachedData, nil
			}
			return nil, fmt.Errorf("gnomAD service unavailable (circuit breaker open)")
		}
		return nil, fmt.Errorf("gnomAD query failed: %w", err)
	}
	
	data := result.(*domain.PopulationData)
	
	// Cache the result
	if cacheErr := r.cacheClient.SetPopulationData(ctx, variant, data, 0); cacheErr != nil {
		// Log cache error but don't fail the request
		fmt.Printf("Failed to cache population data: %v\n", cacheErr)
	}
	
	return data, nil
}

// QueryCOSMIC queries COSMIC with circuit breaker and caching
func (r *ResilientExternalClient) QueryCOSMIC(ctx context.Context, variant *domain.StandardizedVariant) (*domain.SomaticData, error) {
	// Check cache first
	if cachedData, found, err := r.cacheClient.GetSomaticData(ctx, variant); err == nil && found {
		return cachedData, nil
	}
	
	// Use circuit breaker
	result, err := r.cosmicBreaker.Execute(func() (interface{}, error) {
		return r.cosmicClient.QueryVariant(ctx, variant)
	})
	
	if err != nil {
		// Check if circuit breaker is open and return cached data if available
		if err == gobreaker.ErrOpenState {
			if cachedData, found, cacheErr := r.cacheClient.GetSomaticData(ctx, variant); cacheErr == nil && found {
				return cachedData, nil
			}
			return nil, fmt.Errorf("COSMIC service unavailable (circuit breaker open)")
		}
		return nil, fmt.Errorf("COSMIC query failed: %w", err)
	}
	
	data := result.(*domain.SomaticData)
	
	// Cache the result
	if cacheErr := r.cacheClient.SetSomaticData(ctx, variant, data, 0); cacheErr != nil {
		// Log cache error but don't fail the request
		fmt.Printf("Failed to cache somatic data: %v\n", cacheErr)
	}
	
	return data, nil
}

// QueryPubMed queries PubMed with circuit breaker and caching
func (r *ResilientExternalClient) QueryPubMed(ctx context.Context, variant *domain.StandardizedVariant) (*domain.LiteratureData, error) {
	// Check cache first (if cache methods exist)
	// TODO: Add cache methods for literature data
	
	// Use circuit breaker
	result, err := r.pubMedBreaker.Execute(func() (interface{}, error) {
		return r.pubMedClient.QueryLiterature(ctx, variant)
	})
	
	if err != nil {
		// Check if circuit breaker is open
		if err == gobreaker.ErrOpenState {
			return nil, fmt.Errorf("PubMed service unavailable (circuit breaker open)")
		}
		return nil, fmt.Errorf("PubMed query failed: %w", err)
	}
	
	data := result.(*domain.LiteratureData)
	
	// TODO: Cache the result when cache methods are available
	
	return data, nil
}

// QueryLOVD queries LOVD with circuit breaker and caching
func (r *ResilientExternalClient) QueryLOVD(ctx context.Context, variant *domain.StandardizedVariant) (*domain.LOVDData, error) {
	// Check cache first (if cache methods exist)
	// TODO: Add cache methods for LOVD data
	
	// Use circuit breaker
	result, err := r.lovdBreaker.Execute(func() (interface{}, error) {
		return r.lovdClient.QueryVariant(ctx, variant)
	})
	
	if err != nil {
		// Check if circuit breaker is open
		if err == gobreaker.ErrOpenState {
			return nil, fmt.Errorf("LOVD service unavailable (circuit breaker open)")
		}
		return nil, fmt.Errorf("LOVD query failed: %w", err)
	}
	
	data := result.(*domain.LOVDData)
	
	// TODO: Cache the result when cache methods are available
	
	return data, nil
}

// QueryHGMD queries HGMD with circuit breaker and caching
func (r *ResilientExternalClient) QueryHGMD(ctx context.Context, variant *domain.StandardizedVariant) (*domain.HGMDData, error) {
	// Check cache first (if cache methods exist)
	// TODO: Add cache methods for HGMD data
	
	// Use circuit breaker
	result, err := r.hgmdBreaker.Execute(func() (interface{}, error) {
		return r.hgmdClient.QueryVariant(ctx, variant)
	})
	
	if err != nil {
		// Check if circuit breaker is open
		if err == gobreaker.ErrOpenState {
			return nil, fmt.Errorf("HGMD service unavailable (circuit breaker open)")
		}
		return nil, fmt.Errorf("HGMD query failed: %w", err)
	}
	
	data := result.(*domain.HGMDData)
	
	// TODO: Cache the result when cache methods are available
	
	return data, nil
}

// GatherEvidence implements the KnowledgeBaseAccess interface with resilience
func (r *ResilientExternalClient) GatherEvidence(ctx context.Context, variant *domain.StandardizedVariant) (*domain.AggregatedEvidence, error) {
	evidence := &domain.AggregatedEvidence{
		GatheredAt: time.Now(),
	}
	
	// Query all databases concurrently with timeout
	type result struct {
		clinVarData    *domain.ClinVarData
		populationData *domain.PopulationData
		somaticData    *domain.SomaticData
		literatureData *domain.LiteratureData
		lovdData       *domain.LOVDData
		hgmdData       *domain.HGMDData
		clinVarErr     error
		populationErr  error
		somaticErr     error
		literatureErr  error
		lovdErr        error
		hgmdErr        error
	}
	
	results := make(chan result, 1)
	
	go func() {
		res := result{}
		
		// Query all databases concurrently
		// Channel to coordinate concurrent queries
		done := make(chan struct{})
		
		// Query ClinVar
		go func() {
			res.clinVarData, res.clinVarErr = r.QueryClinVar(ctx, variant)
			done <- struct{}{}
		}()
		
		// Query gnomAD
		go func() {
			res.populationData, res.populationErr = r.QueryGnomAD(ctx, variant)
			done <- struct{}{}
		}()
		
		// Query COSMIC
		go func() {
			res.somaticData, res.somaticErr = r.QueryCOSMIC(ctx, variant)
			done <- struct{}{}
		}()
		
		// Query PubMed
		go func() {
			res.literatureData, res.literatureErr = r.QueryPubMed(ctx, variant)
			done <- struct{}{}
		}()
		
		// Query LOVD
		go func() {
			res.lovdData, res.lovdErr = r.QueryLOVD(ctx, variant)
			done <- struct{}{}
		}()
		
		// Query HGMD
		go func() {
			res.hgmdData, res.hgmdErr = r.QueryHGMD(ctx, variant)
			done <- struct{}{}
		}()
		
		// Wait for all queries to complete
		for i := 0; i < 6; i++ {
			<-done
		}
		
		results <- res
	}()
	
	select {
	case res := <-results:
		// Set data even if some queries failed
		if res.clinVarErr == nil {
			evidence.ClinVarData = res.clinVarData
		}
		if res.populationErr == nil {
			evidence.PopulationData = res.populationData
		}
		if res.somaticErr == nil {
			evidence.SomaticData = res.somaticData
		}
		if res.literatureErr == nil {
			evidence.LiteratureData = res.literatureData
		}
		if res.lovdErr == nil {
			evidence.LOVDData = res.lovdData
		}
		if res.hgmdErr == nil {
			evidence.HGMDData = res.hgmdData
		}
		
		// Return error only if all queries failed
		allFailed := res.clinVarErr != nil && res.populationErr != nil && res.somaticErr != nil &&
			res.literatureErr != nil && res.lovdErr != nil && res.hgmdErr != nil
		
		if allFailed {
			return nil, fmt.Errorf("all external database queries failed: ClinVar=%v, gnomAD=%v, COSMIC=%v, PubMed=%v, LOVD=%v, HGMD=%v", 
				res.clinVarErr, res.populationErr, res.somaticErr, res.literatureErr, res.lovdErr, res.hgmdErr)
		}
		
		return evidence, nil
		
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// GetCircuitBreakerStats returns statistics for all circuit breakers
func (r *ResilientExternalClient) GetCircuitBreakerStats() map[string]gobreaker.Counts {
	return map[string]gobreaker.Counts{
		"ClinVar": r.clinVarBreaker.Counts(),
		"gnomAD":  r.gnomADBreaker.Counts(),
		"COSMIC":  r.cosmicBreaker.Counts(),
		"PubMed":  r.pubMedBreaker.Counts(),
		"LOVD":    r.lovdBreaker.Counts(),
		"HGMD":    r.hgmdBreaker.Counts(),
	}
}

// GetCircuitBreakerStates returns the current state of all circuit breakers
func (r *ResilientExternalClient) GetCircuitBreakerStates() map[string]gobreaker.State {
	return map[string]gobreaker.State{
		"ClinVar": r.clinVarBreaker.State(),
		"gnomAD":  r.gnomADBreaker.State(),
		"COSMIC":  r.cosmicBreaker.State(),
		"PubMed":  r.pubMedBreaker.State(),
		"LOVD":    r.lovdBreaker.State(),
		"HGMD":    r.hgmdBreaker.State(),
	}
}

// InvalidateCache removes cached data for a variant
func (r *ResilientExternalClient) InvalidateCache(ctx context.Context, variant *domain.StandardizedVariant) error {
	return r.cacheClient.InvalidateVariant(ctx, variant)
}

// Close closes all connections and resources
func (r *ResilientExternalClient) Close() error {
	return r.cacheClient.Close()
}