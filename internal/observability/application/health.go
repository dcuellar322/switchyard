// Package application coordinates health checks, log retention, and diagnostics.
package application

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	observability "switchyard.dev/switchyard/internal/observability/domain"
	runtime "switchyard.dev/switchyard/internal/runtime/domain"
)

const (
	defaultHealthInterval = 10 * time.Second
	defaultHealthTimeout  = 3 * time.Second
	staleHealthAfter      = 30 * time.Second
)

// ErrRequiredHealthChecks is returned after a runtime starts but never becomes ready.
var ErrRequiredHealthChecks = errors.New("required health checks did not pass")

// RuntimeSource resolves trusted declarations and enumerates eligible projects.
type RuntimeSource interface {
	ResolveRuntime(context.Context, string) (runtime.ProjectRuntime, error)
	ListRuntimeProjectIDs(context.Context) ([]string, error)
}

// RuntimeObserver supplies current process/container evidence.
type RuntimeObserver interface {
	Inspect(context.Context, string) (runtime.Observation, error)
}

// HealthRepository persists bounded samples without becoming runtime authority.
type HealthRepository interface {
	AppendHealth(context.Context, observability.HealthResult) error
	LatestHealth(context.Context, string) ([]observability.HealthResult, error)
	PruneHealth(context.Context, time.Time) error
}

// HealthEvaluator runs one non-composite check with a strict caller deadline.
type HealthEvaluator interface {
	Evaluate(context.Context, runtime.ProjectRuntime, runtime.Observation, runtime.HealthCheckDefinition) observability.HealthResult
}

// HealthService evaluates, schedules, persists, and aggregates readiness checks.
type HealthService struct {
	source     RuntimeSource
	observer   RuntimeObserver
	repository HealthRepository
	evaluator  HealthEvaluator
	now        func() time.Time

	mu      sync.Mutex
	nextDue map[string]scheduledHealth
}

type scheduledHealth struct {
	due          time.Time
	manifestHash string
}

// NewHealthService constructs the health application service.
func NewHealthService(source RuntimeSource, observer RuntimeObserver, repository HealthRepository, evaluator HealthEvaluator) *HealthService {
	return &HealthService{source: source, observer: observer, repository: repository, evaluator: evaluator, now: time.Now, nextDue: map[string]scheduledHealth{}}
}

// Get returns persisted health with explicit stale and disconnected states.
func (s *HealthService) Get(ctx context.Context, projectID string) (observability.ProjectHealth, error) {
	project, err := s.source.ResolveRuntime(ctx, projectID)
	if err != nil {
		return observability.ProjectHealth{}, err
	}
	observation, err := s.observer.Inspect(ctx, projectID)
	if err != nil {
		return observability.ProjectHealth{}, err
	}
	results, err := s.repository.LatestHealth(ctx, projectID)
	if err != nil {
		return observability.ProjectHealth{}, err
	}
	declared := make(map[string]struct{})
	for _, service := range project.Services {
		for _, check := range service.HealthChecks {
			declared[service.ID+"\x00"+check.ID] = struct{}{}
		}
	}
	filtered := results[:0]
	observed := make(map[string]struct{}, len(results))
	for _, result := range results {
		key := result.ServiceID + "\x00" + result.CheckID
		if _, exists := declared[key]; exists {
			filtered = append(filtered, result)
			observed[key] = struct{}{}
		}
	}
	results = filtered
	for _, service := range project.Services {
		for _, check := range service.HealthChecks {
			key := service.ID + "\x00" + check.ID
			if _, exists := observed[key]; exists {
				continue
			}
			results = append(results, observability.HealthResult{
				ProjectID: projectID, ServiceID: service.ID, CheckID: check.ID, Type: check.Type,
				Status: observability.StatusUnknown, Severity: severity(check.Severity), Required: check.Required,
				Message: "awaiting first health observation", ObservedAt: s.now().UTC(),
			})
		}
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].ServiceID == results[j].ServiceID {
			return results[i].CheckID < results[j].CheckID
		}
		return results[i].ServiceID < results[j].ServiceID
	})
	return aggregateHealth(projectID, observation, results, s.now().UTC()), nil
}

// LatestResults returns persisted check latencies without triggering runtime inspection.
func (s *HealthService) LatestResults(ctx context.Context, projectID string) ([]observability.HealthResult, error) {
	return s.repository.LatestHealth(ctx, projectID)
}

// EvaluateProject runs every declared check and records only sanitized result messages.
func (s *HealthService) EvaluateProject(ctx context.Context, projectID string) (observability.ProjectHealth, error) {
	project, err := s.source.ResolveRuntime(ctx, projectID)
	if err != nil {
		return observability.ProjectHealth{}, err
	}
	observation, err := s.observer.Inspect(ctx, projectID)
	if err != nil {
		return observability.ProjectHealth{}, err
	}
	results := s.evaluateChecks(ctx, project, observation, true)
	for _, result := range results {
		if err := s.repository.AppendHealth(ctx, result); err != nil {
			return observability.ProjectHealth{}, fmt.Errorf("persist health result: %w", err)
		}
	}
	return aggregateHealth(projectID, observation, results, s.now().UTC()), nil
}

// WaitRequired waits initial delays and bounded retries without stopping a healthy runtime on failure.
func (s *HealthService) WaitRequired(ctx context.Context, projectID string) error {
	project, err := s.source.ResolveRuntime(ctx, projectID)
	if err != nil {
		return err
	}
	if !hasRequiredChecks(project) {
		return nil
	}
	maxDelay := time.Duration(0)
	for _, service := range project.Services {
		for _, check := range service.HealthChecks {
			if check.Required {
				maxDelay = max(maxDelay, time.Duration(check.InitialDelaySeconds)*time.Second)
			}
		}
	}
	if err := sleepContext(ctx, maxDelay); err != nil {
		return err
	}
	observation, err := s.observer.Inspect(ctx, projectID)
	if err != nil {
		return err
	}
	results := s.evaluateChecks(ctx, project, observation, true)
	for _, result := range results {
		if persistErr := s.repository.AppendHealth(ctx, result); persistErr != nil {
			return persistErr
		}
		if result.Required && result.Status != observability.StatusHealthy {
			return fmt.Errorf("%w: %s/%s: %s", ErrRequiredHealthChecks, result.ServiceID, result.CheckID, result.Message)
		}
	}
	return nil
}

func (s *HealthService) evaluateChecks(ctx context.Context, project runtime.ProjectRuntime, observation runtime.Observation, retry bool) []observability.HealthResult {
	results := make([]observability.HealthResult, 0)
	byID := map[string]observability.HealthResult{}
	var composites []runtime.HealthCheckDefinition
	for _, service := range project.Services {
		for _, check := range service.HealthChecks {
			if check.Type == "composite" {
				composites = append(composites, check)
				continue
			}
			result := s.runCheck(ctx, project, observation, check, retry)
			results = append(results, result)
			byID[service.ID+"\x00"+check.ID] = result
		}
	}
	pending := append([]runtime.HealthCheckDefinition(nil), composites...)
	for len(pending) > 0 {
		progress := false
		next := make([]runtime.HealthCheckDefinition, 0, len(pending))
		for _, check := range pending {
			if !compositeMembersAvailable(check, byID) {
				next = append(next, check)
				continue
			}
			result := compositeResult(project.ProjectID, check, byID, s.now().UTC())
			results = append(results, result)
			byID[check.ServiceID+"\x00"+check.ID] = result
			progress = true
		}
		pending = next
		if !progress {
			for _, check := range pending {
				result := compositeResult(project.ProjectID, check, byID, s.now().UTC())
				results = append(results, result)
			}
			break
		}
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].ServiceID == results[j].ServiceID {
			return results[i].CheckID < results[j].CheckID
		}
		return results[i].ServiceID < results[j].ServiceID
	})
	return results
}

func (s *HealthService) runCheck(ctx context.Context, project runtime.ProjectRuntime, observation runtime.Observation, check runtime.HealthCheckDefinition, retry bool) observability.HealthResult {
	attempts := 1
	if retry {
		attempts += check.Retries
	}
	var result observability.HealthResult
	for attempt := 0; attempt < attempts; attempt++ {
		timeout := time.Duration(check.TimeoutSeconds) * time.Second
		if timeout <= 0 {
			timeout = defaultHealthTimeout
		}
		checkCtx, cancel := context.WithTimeout(ctx, timeout)
		result = s.evaluator.Evaluate(checkCtx, project, observation, check)
		cancel()
		result.ProjectID = project.ProjectID
		result.ServiceID = check.ServiceID
		result.CheckID = check.ID
		result.Type = check.Type
		result.Severity = severity(check.Severity)
		result.Required = check.Required
		if result.ObservedAt.IsZero() {
			result.ObservedAt = s.now().UTC()
		}
		if result.Status == observability.StatusHealthy || attempt+1 == attempts {
			break
		}
		if err := sleepContext(ctx, 200*time.Millisecond); err != nil {
			result.Status, result.Message = observability.StatusUnknown, "health evaluation cancelled"
			break
		}
		if refreshed, err := s.observer.Inspect(ctx, project.ProjectID); err == nil {
			observation = refreshed
		}
	}
	return result
}

func aggregateHealth(projectID string, observation runtime.Observation, results []observability.HealthResult, now time.Time) observability.ProjectHealth {
	status := observability.StatusHealthy
	observer := observability.ObserverConnected
	if observation.State == runtime.StateUnknown || observation.Engine != nil && !observation.Engine.Connected {
		status, observer = observability.StatusUnknown, observability.ObserverDisconnected
	}
	latest := observation.ObservedAt
	if len(results) > 0 {
		latest = time.Time{}
	}
	for _, result := range results {
		if result.ObservedAt.After(latest) {
			latest = result.ObservedAt
		}
		if observer == observability.ObserverConnected && (result.Required || result.Severity != "info") && result.Status == observability.StatusUnhealthy {
			status = observability.StatusUnhealthy
		} else if status == observability.StatusHealthy && result.Status == observability.StatusUnknown {
			status = observability.StatusUnknown
		}
	}
	if observer == observability.ObserverConnected && !latest.IsZero() && now.Sub(latest) > staleHealthAfter {
		observer, status = observability.ObserverStale, observability.StatusUnknown
	}
	return observability.ProjectHealth{ProjectID: projectID, Status: status, ObserverState: observer, Results: results, ObservedAt: latest}
}

func compositeResult(projectID string, check runtime.HealthCheckDefinition, results map[string]observability.HealthResult, now time.Time) observability.HealthResult {
	mode := check.Mode
	if mode == "" {
		mode = "all"
	}
	healthy, found := 0, 0
	for _, member := range check.Members {
		result, ok := results[check.ServiceID+"\x00"+member]
		if !ok {
			continue
		}
		found++
		if result.Status == observability.StatusHealthy {
			healthy++
		}
	}
	status, message := observability.StatusUnhealthy, "composite members did not pass"
	if found == 0 {
		status, message = observability.StatusUnknown, "composite members have no observations"
	} else if mode == "all" && found == len(check.Members) && healthy == found || mode == "any" && healthy > 0 {
		status, message = observability.StatusHealthy, "composite members passed"
	}
	return observability.HealthResult{ProjectID: projectID, ServiceID: check.ServiceID, CheckID: check.ID, Type: check.Type,
		Status: status, Severity: severity(check.Severity), Required: check.Required, Message: message, ObservedAt: now}
}

func compositeMembersAvailable(check runtime.HealthCheckDefinition, results map[string]observability.HealthResult) bool {
	for _, member := range check.Members {
		if _, exists := results[check.ServiceID+"\x00"+member]; !exists {
			return false
		}
	}
	return true
}

func hasRequiredChecks(project runtime.ProjectRuntime) bool {
	for _, service := range project.Services {
		for _, check := range service.HealthChecks {
			if check.Required {
				return true
			}
		}
	}
	return false
}

func severity(value string) string {
	if value == "" {
		return "warning"
	}
	return value
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
