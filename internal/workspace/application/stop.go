package application

import (
	"context"
	"errors"
	"fmt"

	"switchyard.dev/switchyard/internal/workspace/domain"
)

func (s *Service) executeStop(
	ctx context.Context,
	workspace domain.Workspace,
	selected map[string]struct{},
	layers [][]string,
	maxParallel int,
	removeData bool,
	tracker *executionTracker,
) (domain.ExecutionSummary, error) {
	pending := cloneProjectSet(selected)
	order := flattenLayers(layers, true)
	outcomes := make(chan projectOutcome, maxParallel)
	active := 0
	failures := make([]error, 0)
	for len(pending) > 0 || active > 0 {
		if ctx.Err() == nil {
			for _, projectID := range order {
				if active >= maxParallel {
					break
				}
				if _, ok := pending[projectID]; !ok {
					continue
				}
				ready, blocked := stopDependentState(workspace, selected, tracker, projectID)
				if blocked {
					delete(pending, projectID)
					message := "not stopped because a dependent project failed to stop"
					if err := tracker.transition(ctx, projectID, domain.ProjectBlocked, message); err != nil {
						failures = append(failures, err)
					}
					continue
				}
				if !ready {
					continue
				}
				delete(pending, projectID)
				active++
				go func() { outcomes <- s.stopProject(ctx, tracker, projectID, removeData) }()
			}
		}
		if active == 0 {
			break
		}
		outcome := <-outcomes
		active--
		if outcome.err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", outcome.projectID, outcome.err))
		}
	}
	if ctx.Err() != nil {
		s.markProjects(context.WithoutCancel(ctx), tracker, pending, domain.ProjectCancelled, "workspace execution cancelled")
		return s.cancelExecution(tracker, errors.Join(append([]error{ctx.Err()}, failures...)...))
	}
	if len(pending) > 0 {
		s.markProjects(ctx, tracker, pending, domain.ProjectBlocked, "not stopped because a dependent project failed to stop")
	}
	if len(failures) == 0 && !hasStatus(tracker, domain.ProjectBlocked) {
		return tracker.finish(ctx, domain.ExecutionSucceeded, "")
	}
	if len(failures) == 0 {
		failures = append(failures, errors.New("one or more dependencies remained running for safety"))
	}
	partial := hasStatus(tracker, domain.ProjectStopped)
	state := domain.ExecutionFailed
	if partial {
		state = domain.ExecutionPartial
	}
	return finishExecution(tracker, state, failures, partial)
}

func (s *Service) stopProject(
	ctx context.Context,
	tracker *executionTracker,
	projectID string,
	removeData bool,
) projectOutcome {
	message := "stopping project; data will be preserved"
	if removeData {
		message = "stopping project and explicitly removing runtime data"
	}
	if err := tracker.transition(ctx, projectID, domain.ProjectStopping, message); err != nil {
		return projectOutcome{projectID: projectID, err: err}
	}
	if s.projects == nil {
		err := errors.New("project lifecycle operator is unavailable")
		_ = tracker.transition(ctx, projectID, domain.ProjectStopFailed, err.Error())
		return projectOutcome{projectID: projectID, err: err}
	}
	if err := s.projects.Stop(ctx, projectID, StopOptions{RemoveData: removeData}); err != nil {
		status := domain.ProjectStopFailed
		if ctx.Err() != nil {
			status = domain.ProjectCancelled
		}
		_ = tracker.transition(context.WithoutCancel(ctx), projectID, status, err.Error())
		return projectOutcome{projectID: projectID, err: err}
	}
	message = "project stopped; data preserved"
	if removeData {
		message = "project stopped; runtime data removed by explicit request"
	}
	err := tracker.transition(ctx, projectID, domain.ProjectStopped, message)
	return projectOutcome{projectID: projectID, err: err}
}

func stopDependentState(
	workspace domain.Workspace,
	selected map[string]struct{},
	tracker *executionTracker,
	projectID string,
) (bool, bool) {
	ready := true
	for _, dependent := range workspace.DependentsOf(projectID, selected) {
		switch tracker.status(dependent) {
		case domain.ProjectStopped:
		case domain.ProjectStopFailed, domain.ProjectBlocked, domain.ProjectCancelled:
			return false, true
		case domain.ProjectQueued, domain.ProjectStarting, domain.ProjectCheckingHealth, domain.ProjectRunning,
			domain.ProjectStartFailed, domain.ProjectStopping, domain.ProjectRollingBack,
			domain.ProjectRolledBack, domain.ProjectRollbackFailed:
			ready = false
		default:
			ready = false
		}
	}
	return ready, false
}

func (s *Service) cancelExecution(
	tracker *executionTracker,
	cause error,
) (domain.ExecutionSummary, error) {
	ctx := context.WithoutCancel(context.Background())
	s.markQueued(ctx, tracker, domain.ProjectCancelled, "workspace execution cancelled")
	summary, persistErr := tracker.finish(ctx, domain.ExecutionCancelled, cause.Error())
	if persistErr != nil {
		cause = errors.Join(cause, persistErr)
	}
	return summary, &ExecutionError{Summary: summary, cause: cause}
}

func (s *Service) markQueued(
	ctx context.Context,
	tracker *executionTracker,
	status domain.ProjectStatus,
	message string,
) {
	for _, result := range tracker.snapshot().Projects {
		if result.Status == domain.ProjectQueued {
			_ = tracker.transition(ctx, result.ProjectID, status, message)
		}
	}
}

func (s *Service) markProjects(
	ctx context.Context,
	tracker *executionTracker,
	projects map[string]struct{},
	status domain.ProjectStatus,
	message string,
) {
	for _, result := range tracker.snapshot().Projects {
		if _, ok := projects[result.ProjectID]; ok && result.Status == domain.ProjectQueued {
			_ = tracker.transition(ctx, result.ProjectID, status, message)
		}
	}
}

func hasStatus(tracker *executionTracker, status domain.ProjectStatus) bool {
	for _, result := range tracker.snapshot().Projects {
		if result.Status == status {
			return true
		}
	}
	return false
}

func finishExecution(
	tracker *executionTracker,
	state domain.ExecutionState,
	failures []error,
	partial bool,
) (domain.ExecutionSummary, error) {
	message := errors.Join(failures...).Error()
	summary, err := tracker.finish(context.WithoutCancel(context.Background()), state, message)
	if err != nil {
		failures = append(failures, err)
	}
	cause := errors.Join(failures...)
	return summary, &ExecutionError{Summary: summary, cause: cause, partial: partial}
}
