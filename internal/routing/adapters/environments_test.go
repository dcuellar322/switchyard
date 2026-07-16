package adapters

import (
	"context"
	"testing"

	environmentDomain "switchyard.dev/switchyard/internal/environments/domain"
)

func TestEnvironmentSourcePreservesUnavailableAndActiveStates(t *testing.T) {
	t.Parallel()

	source := NewEnvironmentSource(environmentReaderStub{items: []environmentDomain.Environment{
		{ID: "active", ProjectID: "project", Hostname: "active.localhost", Target: "http://127.0.0.1:8080", State: environmentDomain.StateActive, Availability: environmentDomain.AvailabilityAvailable},
		{ID: "bare", ProjectID: "project", Hostname: "bare.localhost", State: environmentDomain.StateUnavailable, Availability: environmentDomain.AvailabilityUnavailable, UnavailableReason: "bare"},
	}})
	candidates, err := source.Candidates(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 2 || !candidates[0].Active || candidates[1].Available || candidates[1].UnavailableReason != "bare" {
		t.Fatalf("candidates = %#v", candidates)
	}
}

type environmentReaderStub struct {
	items []environmentDomain.Environment
}

func (s environmentReaderStub) List(context.Context) ([]environmentDomain.Environment, error) {
	return s.items, nil
}
