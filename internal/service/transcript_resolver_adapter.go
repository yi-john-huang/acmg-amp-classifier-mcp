package service

import (
	"context"

	"github.com/acmg-amp-mcp-server/internal/domain"
)

// TranscriptResolverAdapter adapts CachedTranscriptResolver to domain.GeneTranscriptResolver
type TranscriptResolverAdapter struct {
	resolver *CachedTranscriptResolver
}

// NewTranscriptResolverAdapter creates a new adapter
func NewTranscriptResolverAdapter(resolver *CachedTranscriptResolver) domain.GeneTranscriptResolver {
	return &TranscriptResolverAdapter{
		resolver: resolver,
	}
}

// ResolveGeneToTranscript resolves gene symbols to transcript information
func (a *TranscriptResolverAdapter) ResolveGeneToTranscript(ctx context.Context, geneSymbol string) (*domain.TranscriptInfo, error) {
	// Use the service layer transcript resolver
	transcript, err := a.resolver.ResolveGeneToTranscript(ctx, geneSymbol)
	if err != nil {
		return nil, err
	}
	
	// Convert from external.TranscriptInfo to domain.TranscriptInfo
	return &domain.TranscriptInfo{
		RefSeqID:    transcript.RefSeqID,
		GeneSymbol:  transcript.GeneSymbol,
		Source:      string(transcript.Source), // Convert ExternalServiceType to string
		LastUpdated: transcript.LastUpdated,
	}, nil
}