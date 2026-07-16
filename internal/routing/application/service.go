// Package application owns the optional local HTTP routing registry.
package application

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"switchyard.dev/switchyard/internal/routing/domain"
)

// CandidateSource builds current route candidates from another application
// boundary, such as the project environment registry.
type CandidateSource interface {
	Candidates(context.Context) ([]domain.Candidate, error)
}

// Service is a concurrency-safe, constructor-owned routing registry.
type Service struct {
	mu         sync.RWMutex
	enabled    bool
	candidates map[string][]domain.Candidate
	routes     map[string]domain.Route
	now        func() time.Time
}

// NewService creates an empty registry. Routing remains opt-in through enabled.
func NewService(enabled bool) *Service {
	return &Service{
		enabled: enabled, candidates: make(map[string][]domain.Candidate),
		routes: make(map[string]domain.Route), now: time.Now,
	}
}

// Enabled reports whether proxy resolution is allowed.
func (s *Service) Enabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.enabled
}

// SetEnabled changes only proxy availability; it starts no listener and
// performs no certificate or operating-system configuration.
func (s *Service) SetEnabled(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.enabled = enabled
	s.rebuildLocked(s.now().UTC())
}

// Refresh reads candidates through an explicit application port.
func (s *Service) Refresh(ctx context.Context, source CandidateSource) ([]domain.Route, error) {
	candidates, err := source.Candidates(ctx)
	if err != nil {
		return nil, fmt.Errorf("read local route candidates: %w", err)
	}
	return s.Reconcile(ctx, candidates)
}

// Reconcile atomically replaces route candidates and derives explicit active,
// unavailable, conflict, or disabled states.
func (s *Service) Reconcile(ctx context.Context, candidates []domain.Candidate) ([]domain.Route, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	grouped := make(map[string][]domain.Candidate)
	for _, candidate := range candidates {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if candidate.ProjectID == "" || candidate.EnvironmentID == "" {
			return nil, errors.New("route candidate requires project and environment IDs")
		}
		hostname, err := domain.NormalizeHostname(candidate.Hostname)
		if err != nil {
			return nil, fmt.Errorf("environment %s: %w", candidate.EnvironmentID, err)
		}
		candidate.Hostname = hostname
		grouped[hostname] = append(grouped[hostname], candidate)
	}
	s.mu.Lock()
	s.candidates = grouped
	s.rebuildLocked(s.now().UTC())
	routes := snapshotRoutes(s.routes)
	s.mu.Unlock()
	return routes, nil
}

// Resolve returns an explicit status even when no route is registered.
func (s *Service) Resolve(ctx context.Context, requestHost string) (domain.Route, error) {
	if err := ctx.Err(); err != nil {
		return domain.Route{}, err
	}
	hostname, err := domain.HostnameFromRequest(requestHost)
	if err != nil {
		return domain.Route{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if route, exists := s.routes[hostname]; exists {
		return cloneRoute(route), nil
	}
	status := domain.StatusUnavailable
	reason := "no local route is registered for this hostname"
	if !s.enabled {
		status = domain.StatusDisabled
		reason = "local HTTP routing is disabled"
	}
	return domain.Route{Hostname: hostname, Status: status, Reason: reason, CandidateEnvironmentIDs: []string{}, UpdatedAt: s.now().UTC()}, nil
}

// Snapshot returns all configured routes in hostname order.
func (s *Service) Snapshot() []domain.Route {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return snapshotRoutes(s.routes)
}

func (s *Service) rebuildLocked(at time.Time) {
	routes := make(map[string]domain.Route, len(s.candidates))
	for hostname, candidates := range s.candidates {
		routes[hostname] = resolveCandidates(s.enabled, hostname, candidates, at)
	}
	s.routes = routes
}

func resolveCandidates(enabled bool, hostname string, candidates []domain.Candidate, at time.Time) domain.Route {
	route := domain.Route{
		Hostname: hostname, CandidateEnvironmentIDs: domain.CandidateIDs(candidates), UpdatedAt: at,
	}
	if !enabled {
		route.Status = domain.StatusDisabled
		route.Reason = "local HTTP routing is disabled"
		return route
	}
	active, invalidReasons := activeCandidates(candidates)
	switch len(active) {
	case 0:
		route.Status = domain.StatusUnavailable
		route.Reason = unavailableReason(candidates, invalidReasons)
	case 1:
		route.Status = domain.StatusActive
		route.ProjectID = active[0].ProjectID
		route.EnvironmentID = active[0].EnvironmentID
		route.Target = active[0].Target
	default:
		route.Status = domain.StatusConflict
		route.Reason = "multiple active environments claim this hostname"
	}
	return route
}

func activeCandidates(candidates []domain.Candidate) ([]domain.Candidate, []string) {
	activeByBinding := make(map[string]domain.Candidate)
	var invalidReasons []string
	for _, candidate := range candidates {
		if !candidate.Active || !candidate.Available {
			continue
		}
		if _, err := domain.ValidateTarget(candidate.Target); err != nil {
			invalidReasons = append(invalidReasons, err.Error())
			continue
		}
		activeByBinding[candidate.EnvironmentID+"\x00"+candidate.Target] = candidate
	}
	result := make([]domain.Candidate, 0, len(activeByBinding))
	for _, candidate := range activeByBinding {
		result = append(result, candidate)
	}
	sort.Slice(result, func(left, right int) bool { return result[left].EnvironmentID < result[right].EnvironmentID })
	return result, invalidReasons
}

func unavailableReason(candidates []domain.Candidate, invalidReasons []string) string {
	if len(invalidReasons) > 0 {
		sort.Strings(invalidReasons)
		return "active environment target is unavailable: " + invalidReasons[0]
	}
	for _, candidate := range candidates {
		if candidate.Active && !candidate.Available && candidate.UnavailableReason != "" {
			return candidate.UnavailableReason
		}
	}
	return "no registered environment for this hostname is active"
}

func snapshotRoutes(routes map[string]domain.Route) []domain.Route {
	result := make([]domain.Route, 0, len(routes))
	for _, route := range routes {
		result = append(result, cloneRoute(route))
	}
	sort.Slice(result, func(left, right int) bool { return result[left].Hostname < result[right].Hostname })
	return result
}

func cloneRoute(route domain.Route) domain.Route {
	route.CandidateEnvironmentIDs = append([]string(nil), route.CandidateEnvironmentIDs...)
	route.Reason = strings.TrimSpace(route.Reason)
	return route
}
