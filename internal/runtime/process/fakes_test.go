package process

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"switchyard.dev/switchyard/internal/runtime/domain"
)

type memoryRunStore struct {
	mu   sync.Mutex
	runs map[string]domain.RunRecord
}

func newMemoryRunStore() *memoryRunStore {
	return &memoryRunStore{runs: make(map[string]domain.RunRecord)}
}

func (s *memoryRunStore) CreateRun(_ context.Context, run domain.RunRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.runs[run.ID]; exists {
		return errors.New("duplicate run")
	}
	s.runs[run.ID] = cloneRun(run)
	return nil
}

func (s *memoryRunStore) RecordProcess(_ context.Context, identity domain.ProcessIdentity) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	run, exists := s.runs[identity.RunID]
	if !exists {
		return errors.New("missing run")
	}
	for index, existing := range run.Processes {
		if existing.PID == identity.PID && existing.StartedAt.Equal(identity.StartedAt) {
			run.Processes[index] = identity
			s.runs[run.ID] = run
			return nil
		}
	}
	run.Processes = append(run.Processes, identity)
	s.runs[run.ID] = run
	return nil
}

func (s *memoryRunStore) FinishRun(_ context.Context, id string, endedAt time.Time, exitCode *int, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	run, exists := s.runs[id]
	if !exists {
		return errors.New("missing run")
	}
	if run.EndedAt == nil {
		run.EndedAt = timePointer(endedAt)
		run.ExitCode = cloneInt(exitCode)
		run.TerminationReason = reason
		s.runs[id] = run
	}
	return nil
}

func (s *memoryRunStore) SetRestartCount(_ context.Context, id string, count int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	run := s.runs[id]
	run.RestartCount = count
	s.runs[id] = run
	return nil
}

func (s *memoryRunStore) ListProjectRuns(_ context.Context, projectID string) ([]domain.RunRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := []domain.RunRecord{}
	for _, run := range s.runs {
		if run.ProjectID == projectID {
			result = append(result, cloneRun(run))
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].StartedAt.After(result[j].StartedAt) })
	return result, nil
}

func cloneRun(run domain.RunRecord) domain.RunRecord {
	run.Processes = append([]domain.ProcessIdentity(nil), run.Processes...)
	run.EndedAt = cloneTime(run.EndedAt)
	run.ExitCode = cloneInt(run.ExitCode)
	return run
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	valueCopy := *value
	return &valueCopy
}

func cloneInt(value *int) *int {
	if value == nil {
		return nil
	}
	valueCopy := *value
	return &valueCopy
}

type inspectorFake struct {
	snapshots map[int32]domain.ProcessIdentity
	groups    map[int32][]domain.ProcessIdentity
	listeners map[int][]domain.ProcessIdentity
	matches   map[int32]bool
	usage     processUsage
}

func (f inspectorFake) Snapshot(_ context.Context, pid int32) (domain.ProcessIdentity, error) {
	value, ok := f.snapshots[pid]
	if !ok {
		return domain.ProcessIdentity{}, errors.New("process not found")
	}
	return value, nil
}
func (f inspectorFake) GroupMembers(_ context.Context, group int32) ([]domain.ProcessIdentity, error) {
	return append([]domain.ProcessIdentity(nil), f.groups[group]...), nil
}
func (f inspectorFake) Listeners(_ context.Context, port int) ([]domain.ProcessIdentity, error) {
	return append([]domain.ProcessIdentity(nil), f.listeners[port]...), nil
}
func (f inspectorFake) MatchesCommand(_ context.Context, pid int32, _ string) bool {
	return f.matches[pid]
}
func (f inspectorFake) Usage(context.Context, int32) (processUsage, error) { return f.usage, nil }

type secretResolverFake struct {
	values map[string]string
	keys   []string
}

type secretObserverFake struct{ values []string }

func (f *secretObserverFake) AddSecret(value string) { f.values = append(f.values, value) }

func (f *secretResolverFake) Resolve(_ context.Context, reference domain.SecretReference) (string, error) {
	f.keys = append(f.keys, reference.Key)
	value, ok := f.values[reference.Key]
	if !ok {
		return "", errors.New("missing secret")
	}
	return value, nil
}

type progressSinkFake struct{}

func (progressSinkFake) Step(context.Context, string, string, string) error { return nil }

type logSinkFake struct{ entries []domain.LogEntry }

func (s *logSinkFake) WriteLog(_ context.Context, entry domain.LogEntry) error {
	s.entries = append(s.entries, entry)
	return nil
}

type metricSinkFake struct{ samples []domain.MetricSample }

func (s *metricSinkFake) WriteMetric(_ context.Context, sample domain.MetricSample) error {
	s.samples = append(s.samples, sample)
	return nil
}
