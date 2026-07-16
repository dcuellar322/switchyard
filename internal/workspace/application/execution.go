package application

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"switchyard.dev/switchyard/internal/foundation/identifier"
	"switchyard.dev/switchyard/internal/workspace/domain"
)

type projectOutcome struct {
	projectID string
	started   bool
	err       error
}

// Execute synchronously coordinates one workspace start or stop and persists every transition.
func (s *Service) Execute(
	ctx context.Context,
	workspaceID string,
	request ExecuteRequest,
	reporter ProgressReporter,
) (domain.ExecutionSummary, error) {
	if request.Kind != domain.ExecutionStart && request.Kind != domain.ExecutionStop {
		return domain.ExecutionSummary{}, fmt.Errorf("%w: unsupported execution kind %q", ErrInvalidRequest, request.Kind)
	}
	workspace, err := s.repository.Get(ctx, workspaceID)
	if err != nil {
		return domain.ExecutionSummary{}, err
	}
	if err := workspace.Validate(); err != nil {
		return domain.ExecutionSummary{}, err
	}
	selected, maxParallel, profileID, err := workspace.Selection(request.ProfileID)
	if err != nil {
		return domain.ExecutionSummary{}, err
	}
	layers, err := workspace.TopologicalLayers(selected)
	if err != nil {
		return domain.ExecutionSummary{}, err
	}
	policy := request.Policy
	if policy == "" {
		policy = workspace.DefaultFailurePolicy
	}
	if policy != domain.FailurePolicyRollback && policy != domain.FailurePolicyContinue {
		return domain.ExecutionSummary{}, fmt.Errorf("%w: unsupported failure policy %q", ErrInvalidRequest, policy)
	}
	executionID, err := identifier.New("workspace_run")
	if err != nil {
		return domain.ExecutionSummary{}, err
	}
	summary := s.newExecution(workspace, executionID, request, policy, profileID, layers)
	tracker := newExecutionTracker(s.repository, reporter, summary, s.now)
	if err := tracker.initialize(ctx); err != nil {
		return domain.ExecutionSummary{}, fmt.Errorf("initialize workspace execution: %w", err)
	}
	if request.Kind == domain.ExecutionStart {
		return s.executeStart(ctx, workspace, selected, layers, maxParallel, policy, tracker)
	}
	return s.executeStop(ctx, workspace, selected, layers, maxParallel, request.RemoveData, tracker)
}

func (s *Service) newExecution(
	workspace domain.Workspace,
	id string,
	request ExecuteRequest,
	policy domain.FailurePolicy,
	profileID string,
	layers [][]string,
) domain.ExecutionSummary {
	results := make([]domain.ProjectResult, 0)
	for _, layer := range layers {
		for _, projectID := range layer {
			member, _ := workspace.Member(projectID)
			results = append(results, domain.ProjectResult{
				ProjectID: projectID, Role: member.Role, Status: domain.ProjectQueued, Order: member.Order,
			})
		}
	}
	return domain.ExecutionSummary{
		ID: id, WorkspaceID: workspace.ID, Kind: request.Kind, State: domain.ExecutionRunning,
		Policy: policy, ProfileID: profileID, RemoveData: request.RemoveData,
		Projects: results, StartedAt: s.now().UTC(),
	}
}

func (s *Service) executeStart(
	ctx context.Context,
	workspace domain.Workspace,
	selected map[string]struct{},
	layers [][]string,
	maxParallel int,
	policy domain.FailurePolicy,
	tracker *executionTracker,
) (domain.ExecutionSummary, error) {
	started := make(map[string]struct{}, len(selected))
	pending := cloneProjectSet(selected)
	order := flattenLayers(layers, false)
	outcomes := make(chan projectOutcome, maxParallel)
	active := 0
	rollbackActivated := false
	failures := make([]error, 0)
	for len(pending) > 0 || active > 0 {
		if ctx.Err() == nil && !rollbackActivated {
			launched, scheduleErrors := s.scheduleStarts(
				ctx, workspace, selected, order, pending, maxParallel-active, tracker, outcomes,
			)
			active += launched
			failures = append(failures, scheduleErrors...)
		}
		if active == 0 {
			break
		}
		outcome := <-outcomes
		active--
		if outcome.started {
			started[outcome.projectID] = struct{}{}
		}
		if outcome.err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", outcome.projectID, outcome.err))
			rollbackActivated = policy == domain.FailurePolicyRollback
		}
	}
	if ctx.Err() != nil {
		s.markProjects(context.WithoutCancel(ctx), tracker, pending, domain.ProjectCancelled, "workspace execution cancelled")
		if policy == domain.FailurePolicyRollback {
			failures = append(failures, s.rollback(ctx, layers, started, maxParallel, tracker)...)
		}
		return s.cancelExecution(tracker, errors.Join(append([]error{ctx.Err()}, failures...)...))
	}
	if rollbackActivated {
		s.markProjects(ctx, tracker, pending, domain.ProjectBlocked, "not started because rollback policy was activated")
		failures = append(failures, s.rollback(ctx, layers, started, maxParallel, tracker)...)
		return finishExecution(tracker, domain.ExecutionFailed, failures, false)
	}
	if len(pending) > 0 {
		s.markProjects(ctx, tracker, pending, domain.ProjectBlocked, "blocked because a dependency did not start successfully")
	}
	if len(failures) == 0 && !hasStatus(tracker, domain.ProjectBlocked) {
		return tracker.finish(ctx, domain.ExecutionSucceeded, "")
	}
	if len(failures) == 0 {
		failures = append(failures, errors.New("one or more projects were blocked by failed dependencies"))
	}
	partial := hasStatus(tracker, domain.ProjectRunning)
	state := domain.ExecutionFailed
	if partial {
		state = domain.ExecutionPartial
	}
	return finishExecution(tracker, state, failures, partial)
}

func (s *Service) scheduleStarts(
	ctx context.Context,
	workspace domain.Workspace,
	selected map[string]struct{},
	order []string,
	pending map[string]struct{},
	capacity int,
	tracker *executionTracker,
	outcomes chan<- projectOutcome,
) (int, []error) {
	launched := 0
	failures := make([]error, 0)
	for _, projectID := range order {
		if launched >= capacity {
			break
		}
		if _, ok := pending[projectID]; !ok {
			continue
		}
		ready, blocked := startDependencyState(workspace, selected, tracker, projectID)
		if blocked {
			delete(pending, projectID)
			message := "blocked because a dependency did not start successfully"
			if err := tracker.transition(ctx, projectID, domain.ProjectBlocked, message); err != nil {
				failures = append(failures, err)
			}
			continue
		}
		if !ready {
			continue
		}
		delete(pending, projectID)
		launched++
		go func() { outcomes <- s.startProject(ctx, workspace, tracker, projectID) }()
	}
	return launched, failures
}

func (s *Service) startProject(
	ctx context.Context,
	workspace domain.Workspace,
	tracker *executionTracker,
	projectID string,
) projectOutcome {
	if err := tracker.transition(ctx, projectID, domain.ProjectStarting, "starting project"); err != nil {
		return projectOutcome{projectID: projectID, err: err}
	}
	if s.projects == nil {
		err := errors.New("project lifecycle operator is unavailable")
		_ = tracker.transition(ctx, projectID, domain.ProjectStartFailed, err.Error())
		return projectOutcome{projectID: projectID, err: err}
	}
	if err := s.projects.Start(ctx, projectID); err != nil {
		status := domain.ProjectStartFailed
		if ctx.Err() != nil {
			status = domain.ProjectCancelled
		}
		_ = tracker.transition(context.WithoutCancel(ctx), projectID, status, err.Error())
		return projectOutcome{projectID: projectID, err: err}
	}
	member, _ := workspace.Member(projectID)
	if member.HealthGate {
		if err := tracker.transition(ctx, projectID, domain.ProjectCheckingHealth, "waiting for health gate"); err != nil {
			return projectOutcome{projectID: projectID, started: true, err: err}
		}
		if s.health == nil {
			err := errors.New("health gate is unavailable")
			_ = tracker.transition(ctx, projectID, domain.ProjectStartFailed, err.Error())
			return projectOutcome{projectID: projectID, started: true, err: err}
		}
		healthCtx, cancel := context.WithTimeout(ctx, member.HealthTimeout)
		err := s.health.WaitHealthy(healthCtx, projectID, member.HealthTimeout)
		cancel()
		if err != nil {
			status := domain.ProjectStartFailed
			if ctx.Err() != nil {
				status = domain.ProjectCancelled
			}
			_ = tracker.transition(context.WithoutCancel(ctx), projectID, status, err.Error())
			return projectOutcome{projectID: projectID, started: true, err: err}
		}
	}
	if err := tracker.transition(ctx, projectID, domain.ProjectRunning, "project is ready"); err != nil {
		return projectOutcome{projectID: projectID, started: true, err: err}
	}
	return projectOutcome{projectID: projectID, started: true}
}

func (s *Service) rollback(
	ctx context.Context,
	layers [][]string,
	started map[string]struct{},
	maxParallel int,
	tracker *executionTracker,
) []error {
	rollbackCtx := context.WithoutCancel(ctx)
	failures := make([]error, 0)
	for index := len(layers) - 1; index >= 0; index-- {
		projects := filterProjects(layers[index], started)
		outcomes := runParallel(rollbackCtx, projects, maxParallel, func(projectID string) projectOutcome {
			if err := tracker.transition(rollbackCtx, projectID, domain.ProjectRollingBack, "rolling back started project"); err != nil {
				return projectOutcome{projectID: projectID, err: err}
			}
			err := s.projects.Stop(rollbackCtx, projectID, StopOptions{RemoveData: false})
			if err != nil {
				_ = tracker.transition(rollbackCtx, projectID, domain.ProjectRollbackFailed, err.Error())
				return projectOutcome{projectID: projectID, err: err}
			}
			err = tracker.transition(rollbackCtx, projectID, domain.ProjectRolledBack, "project rolled back; data preserved")
			return projectOutcome{projectID: projectID, err: err}
		})
		for _, outcome := range outcomes {
			if outcome.err != nil {
				failures = append(failures, fmt.Errorf("rollback %s: %w", outcome.projectID, outcome.err))
			}
		}
	}
	return failures
}

func startDependencyState(
	workspace domain.Workspace,
	selected map[string]struct{},
	tracker *executionTracker,
	projectID string,
) (bool, bool) {
	ready := true
	for _, dependency := range workspace.DependenciesOf(projectID, selected) {
		switch tracker.status(dependency) {
		case domain.ProjectRunning:
		case domain.ProjectStartFailed, domain.ProjectBlocked, domain.ProjectCancelled,
			domain.ProjectRolledBack, domain.ProjectRollbackFailed:
			return false, true
		case domain.ProjectQueued, domain.ProjectStarting, domain.ProjectCheckingHealth,
			domain.ProjectStopping, domain.ProjectStopped, domain.ProjectStopFailed, domain.ProjectRollingBack:
			ready = false
		default:
			ready = false
		}
	}
	return ready, false
}

func runParallel(
	ctx context.Context,
	projects []string,
	limit int,
	run func(string) projectOutcome,
) []projectOutcome {
	if len(projects) == 0 {
		return nil
	}
	if limit < 1 {
		limit = 1
	}
	semaphore := make(chan struct{}, limit)
	outcomes := make(chan projectOutcome, len(projects))
	var group sync.WaitGroup
	for _, projectID := range projects {
		projectID := projectID
		group.Go(func() {
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
				outcomes <- run(projectID)
			case <-ctx.Done():
				outcomes <- projectOutcome{projectID: projectID, err: ctx.Err()}
			}
		})
	}
	group.Wait()
	close(outcomes)
	result := make([]projectOutcome, 0, len(projects))
	for outcome := range outcomes {
		result = append(result, outcome)
	}
	slices.SortFunc(result, func(left, right projectOutcome) int {
		return strings.Compare(left.projectID, right.projectID)
	})
	return result
}

func filterProjects(projects []string, included map[string]struct{}) []string {
	result := make([]string, 0, len(projects))
	for _, projectID := range projects {
		if _, ok := included[projectID]; ok {
			result = append(result, projectID)
		}
	}
	return result
}

func cloneProjectSet(source map[string]struct{}) map[string]struct{} {
	result := make(map[string]struct{}, len(source))
	for projectID := range source {
		result[projectID] = struct{}{}
	}
	return result
}

func flattenLayers(layers [][]string, reverse bool) []string {
	result := make([]string, 0)
	if reverse {
		for index := len(layers) - 1; index >= 0; index-- {
			result = append(result, layers[index]...)
		}
		return result
	}
	for _, layer := range layers {
		result = append(result, layer...)
	}
	return result
}
