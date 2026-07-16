package application

import (
	"context"
	"errors"
	"hash/fnv"
	"sync"
	"time"

	runtime "switchyard.dev/switchyard/internal/runtime/domain"
)

// Run schedules checks with stable jitter and exits promptly on cancellation.
func (s *HealthService) Run(ctx context.Context, onError func(string, error)) {
	if onError == nil {
		onError = func(string, error) {}
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	s.runDue(ctx, onError)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runDue(ctx, onError)
		}
	}
}

func (s *HealthService) runDue(ctx context.Context, onError func(string, error)) {
	ids, err := s.source.ListRuntimeProjectIDs(ctx)
	if err != nil {
		onError("", err)
		return
	}
	now := s.now().UTC()
	semaphore := make(chan struct{}, 4)
	var wait sync.WaitGroup
	wanted := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		wanted[id] = struct{}{}
		project, resolveErr := s.source.ResolveRuntime(ctx, id)
		if resolveErr != nil {
			onError(id, resolveErr)
			continue
		}
		s.mu.Lock()
		schedule, scheduled := s.nextDue[id]
		if !scheduled || schedule.manifestHash != project.ManifestHash {
			schedule = scheduledHealth{due: now.Add(healthInitialDelay(project)), manifestHash: project.ManifestHash}
			s.nextDue[id] = schedule
		}
		if schedule.due.After(now) {
			s.mu.Unlock()
			continue
		}
		s.nextDue[id] = scheduledHealth{due: now.Add(jitteredInterval(id, healthInterval(project))), manifestHash: project.ManifestHash}
		s.mu.Unlock()
		wait.Add(1)
		go s.evaluateScheduled(ctx, id, semaphore, &wait, onError)
	}
	wait.Wait()
	s.removeUnscheduled(wanted)
	_ = s.repository.PruneHealth(ctx, now.Add(-24*time.Hour))
}

func (s *HealthService) evaluateScheduled(ctx context.Context, projectID string, semaphore chan struct{}, wait *sync.WaitGroup, onError func(string, error)) {
	defer wait.Done()
	select {
	case semaphore <- struct{}{}:
		defer func() { <-semaphore }()
	case <-ctx.Done():
		return
	}
	if _, err := s.EvaluateProject(ctx, projectID); err != nil && !errors.Is(err, context.Canceled) {
		onError(projectID, err)
	}
}

func (s *HealthService) removeUnscheduled(wanted map[string]struct{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id := range s.nextDue {
		if _, exists := wanted[id]; !exists {
			delete(s.nextDue, id)
		}
	}
}

func jitteredInterval(key string, interval time.Duration) time.Duration {
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(key))
	percent := int(hash.Sum32()%21) - 10
	return interval + time.Duration(percent)*interval/100
}

func healthInterval(project runtime.ProjectRuntime) time.Duration {
	interval := defaultHealthInterval
	for _, service := range project.Services {
		for _, check := range service.HealthChecks {
			configured := time.Duration(check.IntervalSeconds) * time.Second
			if configured > 0 && configured < interval {
				interval = configured
			}
		}
	}
	return interval
}

func healthInitialDelay(project runtime.ProjectRuntime) time.Duration {
	var delay time.Duration
	for _, service := range project.Services {
		for _, check := range service.HealthChecks {
			delay = max(delay, time.Duration(check.InitialDelaySeconds)*time.Second)
		}
	}
	return delay
}
