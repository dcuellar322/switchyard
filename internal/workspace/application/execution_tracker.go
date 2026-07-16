package application

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"switchyard.dev/switchyard/internal/workspace/domain"
)

type executionTracker struct {
	mu         sync.Mutex
	repository Repository
	reporter   ProgressReporter
	summary    domain.ExecutionSummary
	now        func() time.Time
	index      map[string]int
}

func newExecutionTracker(
	repository Repository,
	reporter ProgressReporter,
	summary domain.ExecutionSummary,
	now func() time.Time,
) *executionTracker {
	index := make(map[string]int, len(summary.Projects))
	for position, result := range summary.Projects {
		index[result.ProjectID] = position
	}
	return &executionTracker{repository: repository, reporter: reporter, summary: summary, now: now, index: index}
}

func (t *executionTracker) initialize(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.saveLocked(ctx)
}

func (t *executionTracker) transition(ctx context.Context, projectID string, status domain.ProjectStatus, message string) error {
	t.mu.Lock()
	position, ok := t.index[projectID]
	if !ok {
		t.mu.Unlock()
		return fmt.Errorf("track unknown workspace project %s", projectID)
	}
	now := t.now().UTC()
	result := &t.summary.Projects[position]
	result.Status = status
	result.Message = message
	if result.StartedAt == nil && beginsProjectWork(status) {
		result.StartedAt = &now
	}
	if finishesProjectWork(status) {
		result.FinishedAt = &now
	}
	snapshot := cloneSummary(t.summary)
	err := t.repository.SaveExecution(ctx, snapshot)
	t.mu.Unlock()
	if err != nil {
		return fmt.Errorf("save workspace project progress: %w", err)
	}
	if t.reporter != nil {
		if err := t.reporter.ProjectProgress(ctx, snapshot.Projects[position]); err != nil {
			return fmt.Errorf("report workspace project progress: %w", err)
		}
	}
	return nil
}

func (t *executionTracker) status(projectID string) domain.ProjectStatus {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.summary.Projects[t.index[projectID]].Status
}

func (t *executionTracker) finish(ctx context.Context, state domain.ExecutionState, message string) (domain.ExecutionSummary, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	finishedAt := t.now().UTC()
	t.summary.State = state
	t.summary.ErrorMessage = message
	t.summary.FinishedAt = &finishedAt
	if err := t.saveLocked(ctx); err != nil {
		return domain.ExecutionSummary{}, err
	}
	return cloneSummary(t.summary), nil
}

func (t *executionTracker) snapshot() domain.ExecutionSummary {
	t.mu.Lock()
	defer t.mu.Unlock()
	return cloneSummary(t.summary)
}

func (t *executionTracker) saveLocked(ctx context.Context) error {
	return t.repository.SaveExecution(ctx, cloneSummary(t.summary))
}

func cloneSummary(source domain.ExecutionSummary) domain.ExecutionSummary {
	result := source
	result.Projects = slices.Clone(source.Projects)
	for index := range result.Projects {
		if source.Projects[index].StartedAt != nil {
			value := *source.Projects[index].StartedAt
			result.Projects[index].StartedAt = &value
		}
		if source.Projects[index].FinishedAt != nil {
			value := *source.Projects[index].FinishedAt
			result.Projects[index].FinishedAt = &value
		}
	}
	if source.FinishedAt != nil {
		value := *source.FinishedAt
		result.FinishedAt = &value
	}
	return result
}

func beginsProjectWork(status domain.ProjectStatus) bool {
	switch status {
	case domain.ProjectStarting, domain.ProjectStopping, domain.ProjectRollingBack:
		return true
	case domain.ProjectQueued, domain.ProjectBlocked, domain.ProjectCheckingHealth, domain.ProjectRunning,
		domain.ProjectStartFailed, domain.ProjectStopped, domain.ProjectStopFailed, domain.ProjectRolledBack,
		domain.ProjectRollbackFailed, domain.ProjectCancelled:
		return false
	default:
		return false
	}
}

func finishesProjectWork(status domain.ProjectStatus) bool {
	switch status {
	case domain.ProjectBlocked, domain.ProjectRunning, domain.ProjectStartFailed, domain.ProjectStopped,
		domain.ProjectStopFailed, domain.ProjectRolledBack, domain.ProjectRollbackFailed, domain.ProjectCancelled:
		return true
	case domain.ProjectQueued, domain.ProjectStarting, domain.ProjectCheckingHealth, domain.ProjectStopping,
		domain.ProjectRollingBack:
		return false
	default:
		return false
	}
}
