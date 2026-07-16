// Package application reconciles port evidence and derives conflicts.
package application

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"switchyard.dev/switchyard/internal/ports/domain"
)

// ErrNoPortAvailable indicates that every port in the preferred range is protected or bound.
var ErrNoPortAvailable = errors.New("no free port is available in the requested range")

// FactSource observes one bounded class of port evidence.
type FactSource interface {
	Facts(context.Context) ([]domain.Fact, error)
}

// ReservationRepository reconciles manifest-backed leases and returns current reservations.
type ReservationRepository interface {
	Reconcile(context.Context, []domain.Fact, time.Time) ([]domain.Fact, error)
}

// Service builds the honest current registry from independent evidence sources.
type Service struct {
	declarations FactSource
	bindings     FactSource
	listeners    FactSource
	additional   []FactSource
	reservations ReservationRepository
	now          func() time.Time
}

// NewService creates the port registry from its independent evidence sources.
func NewService(declarations, bindings, listeners FactSource, reservations ReservationRepository, additional ...FactSource) *Service {
	return &Service{
		declarations: declarations, bindings: bindings, listeners: listeners,
		reservations: reservations, additional: additional, now: time.Now,
	}
}

// Registry refreshes every source; unavailable optional sources become explicit warnings.
func (s *Service) Registry(ctx context.Context) (domain.Registry, error) {
	now := s.now().UTC()
	declarations, err := s.declarations.Facts(ctx)
	if err != nil {
		return domain.Registry{}, fmt.Errorf("read port declarations: %w", err)
	}
	reservations, err := s.reservations.Reconcile(ctx, declarations, now)
	if err != nil {
		return domain.Registry{}, fmt.Errorf("reconcile port reservations: %w", err)
	}
	facts := append(append([]domain.Fact{}, declarations...), reservations...)
	var warnings []string
	for _, source := range s.additional {
		additional, additionalErr := source.Facts(ctx)
		if additionalErr != nil {
			warnings = append(warnings, "additional port evidence unavailable: "+additionalErr.Error())
			continue
		}
		facts = append(facts, additional...)
	}
	runtimeBindings, bindingErr := s.bindings.Facts(ctx)
	if bindingErr != nil {
		warnings = append(warnings, "runtime bindings unavailable: "+bindingErr.Error())
	} else {
		facts = append(facts, runtimeBindings...)
	}
	listeners, listenerErr := s.listeners.Facts(ctx)
	if listenerErr != nil {
		warnings = append(warnings, "OS listeners unavailable: "+listenerErr.Error())
	} else {
		facts = append(facts, removeKnownListeners(listeners, runtimeBindings)...)
	}
	for index := range facts {
		if facts[index].ObservedAt.IsZero() {
			facts[index].ObservedAt = now
		}
	}
	sortFacts(facts)
	return domain.Registry{Facts: facts, Conflicts: Classify(facts), ObservedAt: now, Warnings: warnings}, nil
}

// Suggest returns the first free port after considering every current fact and exclusion.
func (s *Service) Suggest(ctx context.Context, start, end int, protocol, projectID string, excluded []int) (domain.Suggestion, error) {
	if start < 1 || end > 65535 || start > end || protocol != "tcp" && protocol != "udp" {
		return domain.Suggestion{}, errors.New("invalid port suggestion range or protocol")
	}
	registry, err := s.Registry(ctx)
	if err != nil {
		return domain.Suggestion{}, err
	}
	used := make(map[int]struct{}, len(registry.Facts)+len(excluded))
	for _, fact := range registry.Facts {
		if fact.ProjectID != projectID || fact.Kind == domain.KindBinding {
			used[fact.Port] = struct{}{}
		}
	}
	for _, port := range excluded {
		used[port] = struct{}{}
	}
	for port := start; port <= end; port++ {
		if _, exists := used[port]; !exists {
			return domain.Suggestion{Port: port, RangeStart: start, RangeEnd: end, Protocol: protocol, ObservedAt: registry.ObservedAt}, nil
		}
	}
	return domain.Suggestion{}, ErrNoPortAvailable
}

func removeKnownListeners(listeners, known []domain.Fact) []domain.Fact {
	result := make([]domain.Fact, 0, len(listeners))
	for _, listener := range listeners {
		matched := false
		for _, binding := range known {
			if listener.Port == binding.Port && listener.Protocol == binding.Protocol && hostsOverlap(listener.Host, binding.Host) {
				matched = true
				break
			}
		}
		if !matched {
			result = append(result, listener)
		}
	}
	return result
}

func sortFacts(facts []domain.Fact) {
	sort.Slice(facts, func(left, right int) bool {
		if facts[left].Port != facts[right].Port {
			return facts[left].Port < facts[right].Port
		}
		if facts[left].ProjectName != facts[right].ProjectName {
			return facts[left].ProjectName < facts[right].ProjectName
		}
		return facts[left].Kind < facts[right].Kind
	})
}
