package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	agentsApplication "switchyard.dev/switchyard/internal/agents/application"
)

// StructuredProvider adapts the provider registry without exposing agent packages to diagnostics.
type StructuredProvider struct{ registry *agentsApplication.Registry }

// NewStructuredProvider creates an optional diagnosis provider adapter.
func NewStructuredProvider(registry *agentsApplication.Registry) *StructuredProvider {
	return &StructuredProvider{registry: registry}
}

// Diagnose invokes a provider with conservative fixed budgets and no repository access.
func (p *StructuredProvider) Diagnose(ctx context.Context, providerID string, bundle, schema json.RawMessage) (json.RawMessage, string, error) {
	if p == nil || p.registry == nil {
		return nil, "", errors.New("diagnosis provider registry is unavailable")
	}
	limits, err := (agentsApplication.Limits{
		EvidenceBytes: max(4<<10, int64(len(bundle))), OutputBytes: 256 << 10, Timeout: 90 * time.Second,
		MaxTurns: 1, MaxOutputTokens: 4096,
	}).Normalize()
	if err != nil {
		return nil, "", err
	}
	result, err := p.registry.Diagnose(ctx, providerID, agentsApplication.ProviderRequest{Bundle: bundle, OutputSchema: schema, Limits: limits})
	return result.Output, result.Model, err
}
