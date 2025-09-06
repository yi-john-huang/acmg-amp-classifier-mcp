package external

import (
	"context"

	"github.com/acmg-amp-mcp-server/internal/domain"
)

// KnowledgeBaseService implements the domain.KnowledgeBaseAccess interface
// It provides a clean interface to external databases with resilience patterns
type KnowledgeBaseService struct {
	resilientClient *ResilientExternalClient
}

// NewKnowledgeBaseService creates a new knowledge base service
func NewKnowledgeBaseService(
	clinVarConfig domain.ClinVarConfig,
	gnomADConfig domain.GnomADConfig,
	cosmicConfig domain.COSMICConfig,
	pubMedConfig domain.PubMedConfig,
	lovdConfig domain.LOVDConfig,
	hgmdConfig domain.HGMDConfig,
	cacheConfig domain.CacheConfig,
) (*KnowledgeBaseService, error) {
	resilientClient, err := NewResilientExternalClient(
		clinVarConfig,
		gnomADConfig,
		cosmicConfig,
		pubMedConfig,
		lovdConfig,
		hgmdConfig,
		cacheConfig,
	)
	if err != nil {
		return nil, err
	}
	
	return &KnowledgeBaseService{
		resilientClient: resilientClient,
	}, nil
}

// GatherEvidence gathers evidence from all external databases
func (k *KnowledgeBaseService) GatherEvidence(ctx context.Context, variant *domain.StandardizedVariant) (*domain.AggregatedEvidence, error) {
	return k.resilientClient.GatherEvidence(ctx, variant)
}

// QueryClinVar queries ClinVar database
func (k *KnowledgeBaseService) QueryClinVar(variant *domain.StandardizedVariant) (*domain.ClinVarData, error) {
	return k.resilientClient.QueryClinVar(context.Background(), variant)
}

// QueryGnomAD queries gnomAD database
func (k *KnowledgeBaseService) QueryGnomAD(variant *domain.StandardizedVariant) (*domain.PopulationData, error) {
	return k.resilientClient.QueryGnomAD(context.Background(), variant)
}

// QueryCOSMIC queries COSMIC database
func (k *KnowledgeBaseService) QueryCOSMIC(variant *domain.StandardizedVariant) (*domain.SomaticData, error) {
	return k.resilientClient.QueryCOSMIC(context.Background(), variant)
}

// QueryPubMed queries PubMed database
func (k *KnowledgeBaseService) QueryPubMed(variant *domain.StandardizedVariant) (*domain.LiteratureData, error) {
	return k.resilientClient.QueryPubMed(context.Background(), variant)
}

// QueryLOVD queries LOVD database
func (k *KnowledgeBaseService) QueryLOVD(variant *domain.StandardizedVariant) (*domain.LOVDData, error) {
	return k.resilientClient.QueryLOVD(context.Background(), variant)
}

// QueryHGMD queries HGMD database
func (k *KnowledgeBaseService) QueryHGMD(variant *domain.StandardizedVariant) (*domain.HGMDData, error) {
	return k.resilientClient.QueryHGMD(context.Background(), variant)
}

// GetStats returns comprehensive statistics about the knowledge base service
func (k *KnowledgeBaseService) GetStats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})
	
	// Circuit breaker stats
	stats["circuit_breaker_stats"] = k.resilientClient.GetCircuitBreakerStats()
	stats["circuit_breaker_states"] = k.resilientClient.GetCircuitBreakerStates()
	
	// Cache stats
	cacheStats, err := k.resilientClient.cacheClient.GetStats(ctx)
	if err == nil {
		stats["cache_stats"] = cacheStats
	}
	
	return stats, nil
}

// InvalidateCache removes cached data for a variant
func (k *KnowledgeBaseService) InvalidateCache(ctx context.Context, variant *domain.StandardizedVariant) error {
	return k.resilientClient.InvalidateCache(ctx, variant)
}

// Close closes the service and all underlying connections
func (k *KnowledgeBaseService) Close() error {
	return k.resilientClient.Close()
}

// HealthCheck performs health checks on all external services
func (k *KnowledgeBaseService) HealthCheck(ctx context.Context) map[string]bool {
	health := make(map[string]bool)
	
	// Check circuit breaker states
	states := k.resilientClient.GetCircuitBreakerStates()
	for service, state := range states {
		// Service is healthy if circuit breaker is closed
		health[service] = (state == 0) // gobreaker.StateClosed = 0
	}
	
	// Check cache connectivity
	if err := k.resilientClient.cacheClient.Ping(ctx); err == nil {
		health["cache"] = true
	} else {
		health["cache"] = false
	}
	
	return health
}