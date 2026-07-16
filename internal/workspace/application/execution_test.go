package application

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	"switchyard.dev/switchyard/internal/workspace/domain"
)

func TestExecuteStartOrdersDependenciesAndRunsIndependentProjectsInParallel(t *testing.T) {
	t.Parallel()

	workspace := orchestrationWorkspace()
	repository := newFakeRepository(workspace)
	entered := make(chan string, 2)
	release := make(chan struct{})
	operator := &fakeProjectOperator{}
	operator.start = func(ctx context.Context, projectID string) error {
		if projectID == "database" || projectID == "cache" {
			entered <- projectID
			select {
			case <-release:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return nil
	}
	service := NewService(repository, operator, noHealthGate{}, allowMembers{})

	resultChannel := make(chan domain.ExecutionSummary, 1)
	errorChannel := make(chan error, 1)
	go func() {
		result, err := service.Execute(context.Background(), workspace.ID, ExecuteRequest{Kind: domain.ExecutionStart}, nil)
		resultChannel <- result
		errorChannel <- err
	}()
	seen := map[string]bool{}
	for range 2 {
		select {
		case projectID := <-entered:
			seen[projectID] = true
		case <-time.After(time.Second):
			t.Fatal("independent root projects did not start concurrently")
		}
	}
	close(release)
	result := <-resultChannel
	if err := <-errorChannel; err != nil {
		t.Fatalf("Execute(start) error = %v", err)
	}
	if !seen["database"] || !seen["cache"] || result.State != domain.ExecutionSucceeded {
		t.Fatalf("parallel roots = %#v, execution state = %s", seen, result.State)
	}
	events := operator.eventsCopy()
	assertBefore(t, events, "start:database", "start:api")
	assertBefore(t, events, "start:cache", "start:worker")
	assertBefore(t, events, "start:api", "start:web")
}

func TestExecuteStartDoesNotCreateBarrierBetweenIndependentBranches(t *testing.T) {
	t.Parallel()

	workspace := domain.Workspace{
		ID: "workspace-branches", Name: "Branches", DefaultFailurePolicy: domain.FailurePolicyContinue, Revision: 1,
		Members: []domain.Member{
			{ProjectID: "root-fast", Role: domain.MemberRoleDependency, Order: 0},
			{ProjectID: "root-slow", Role: domain.MemberRoleApplication, Order: 1},
			{ProjectID: "child-fast", Role: domain.MemberRoleApplication, Order: 2},
		},
		Dependencies: []domain.Dependency{{ProjectID: "child-fast", DependsOnProjectID: "root-fast"}},
	}
	repository := newFakeRepository(workspace)
	slowEntered := make(chan struct{})
	releaseSlow := make(chan struct{})
	childStarted := make(chan struct{})
	operator := &fakeProjectOperator{start: func(ctx context.Context, projectID string) error {
		switch projectID {
		case "root-slow":
			close(slowEntered)
			select {
			case <-releaseSlow:
			case <-ctx.Done():
				return ctx.Err()
			}
		case "child-fast":
			close(childStarted)
		}
		return nil
	}}
	service := NewService(repository, operator, noHealthGate{}, allowMembers{})
	done := make(chan error, 1)
	go func() {
		_, err := service.Execute(context.Background(), workspace.ID, ExecuteRequest{Kind: domain.ExecutionStart}, nil)
		done <- err
	}()
	<-slowEntered
	select {
	case <-childStarted:
	case <-time.After(time.Second):
		t.Fatal("fast branch child waited for unrelated slow root")
	}
	close(releaseSlow)
	if err := <-done; err != nil {
		t.Fatalf("Execute(start) error = %v", err)
	}
}

func TestExecuteStartWaitsForHealthGateBeforeDependent(t *testing.T) {
	t.Parallel()

	workspace := linearWorkspace()
	workspace.Members[1].HealthGate = true
	workspace.Members[1].HealthTimeout = 17 * time.Second
	repository := newFakeRepository(workspace)
	events := &eventLog{}
	operator := &fakeProjectOperator{start: func(_ context.Context, projectID string) error {
		events.add("start:" + projectID)
		return nil
	}}
	health := healthGateFunc(func(_ context.Context, projectID string, timeout time.Duration) error {
		events.add(fmt.Sprintf("health:%s:%s", projectID, timeout))
		return nil
	})
	service := NewService(repository, operator, health, allowMembers{})

	result, err := service.Execute(context.Background(), workspace.ID, ExecuteRequest{Kind: domain.ExecutionStart}, nil)
	if err != nil || result.State != domain.ExecutionSucceeded {
		t.Fatalf("Execute(start) = state %s, error %v", result.State, err)
	}
	got := events.copy()
	assertBefore(t, got, "start:api", "health:api:17s")
	assertBefore(t, got, "health:api:17s", "start:web")
}

func TestExecuteStartRollbackStopsStartedProjectsInReverseDependencyOrder(t *testing.T) {
	t.Parallel()

	workspace := linearWorkspace()
	workspace.Members[2].HealthGate = true
	workspace.Members[2].HealthTimeout = time.Second
	repository := newFakeRepository(workspace)
	operator := &fakeProjectOperator{}
	health := healthGateFunc(func(_ context.Context, projectID string, _ time.Duration) error {
		if projectID == "web" {
			return errors.New("web never became healthy")
		}
		return nil
	})
	service := NewService(repository, operator, health, allowMembers{})

	result, err := service.Execute(context.Background(), workspace.ID, ExecuteRequest{
		Kind: domain.ExecutionStart, Policy: domain.FailurePolicyRollback,
	}, nil)
	var executionErr *ExecutionError
	if !errors.As(err, &executionErr) || executionErr.Partial() {
		t.Fatalf("Execute(start) error = %#v, want non-partial ExecutionError", err)
	}
	if result.State != domain.ExecutionFailed {
		t.Fatalf("execution state = %s, want failed", result.State)
	}
	stops := operator.stopEvents()
	want := []string{"stop:web:false", "stop:api:false", "stop:database:false"}
	if !slices.Equal(stops, want) {
		t.Fatalf("rollback stops = %#v, want %#v", stops, want)
	}
	for _, project := range result.Projects {
		if project.Status != domain.ProjectRolledBack {
			t.Fatalf("project %s status = %s, want rolled_back", project.ProjectID, project.Status)
		}
	}
}

func TestExecuteContinueStartsIndependentBranchAndBlocksDependents(t *testing.T) {
	t.Parallel()

	workspace := orchestrationWorkspace()
	repository := newFakeRepository(workspace)
	operator := &fakeProjectOperator{start: func(_ context.Context, projectID string) error {
		if projectID == "api" {
			return errors.New("api launch failed")
		}
		return nil
	}}
	service := NewService(repository, operator, noHealthGate{}, allowMembers{})

	result, err := service.Execute(context.Background(), workspace.ID, ExecuteRequest{
		Kind: domain.ExecutionStart, Policy: domain.FailurePolicyContinue,
	}, nil)
	var executionErr *ExecutionError
	if !errors.As(err, &executionErr) || !executionErr.Partial() {
		t.Fatalf("Execute(start) error = %#v, want partial ExecutionError", err)
	}
	if result.State != domain.ExecutionPartial {
		t.Fatalf("state = %s, want partially_succeeded", result.State)
	}
	statuses := resultStatuses(result)
	if statuses["api"] != domain.ProjectStartFailed || statuses["web"] != domain.ProjectBlocked ||
		statuses["worker"] != domain.ProjectRunning {
		t.Fatalf("project statuses = %#v", statuses)
	}
}

func TestExecuteStopUsesReverseOrderAndPreservesDataByDefault(t *testing.T) {
	t.Parallel()

	workspace := linearWorkspace()
	repository := newFakeRepository(workspace)
	operator := &fakeProjectOperator{}
	service := NewService(repository, operator, noHealthGate{}, allowMembers{})

	result, err := service.Execute(context.Background(), workspace.ID, ExecuteRequest{Kind: domain.ExecutionStop}, nil)
	if err != nil || result.State != domain.ExecutionSucceeded || result.RemoveData {
		t.Fatalf("Execute(stop) = state %s removeData %t error %v", result.State, result.RemoveData, err)
	}
	want := []string{"stop:web:false", "stop:api:false", "stop:database:false"}
	if got := operator.stopEvents(); !slices.Equal(got, want) {
		t.Fatalf("stop events = %#v, want %#v", got, want)
	}
}

func TestExecuteStopBlocksDependencyWhenDependentFails(t *testing.T) {
	t.Parallel()

	workspace := linearWorkspace()
	repository := newFakeRepository(workspace)
	operator := &fakeProjectOperator{stop: func(_ context.Context, projectID string, _ StopOptions) error {
		if projectID == "web" {
			return errors.New("web refused to stop")
		}
		return nil
	}}
	service := NewService(repository, operator, noHealthGate{}, allowMembers{})

	result, err := service.Execute(context.Background(), workspace.ID, ExecuteRequest{Kind: domain.ExecutionStop}, nil)
	var executionErr *ExecutionError
	if !errors.As(err, &executionErr) || executionErr.Partial() {
		t.Fatalf("Execute(stop) error = %#v, want non-partial ExecutionError", err)
	}
	statuses := resultStatuses(result)
	if statuses["web"] != domain.ProjectStopFailed || statuses["api"] != domain.ProjectBlocked ||
		statuses["database"] != domain.ProjectBlocked {
		t.Fatalf("safe stop statuses = %#v", statuses)
	}
	if got := operator.stopEvents(); !slices.Equal(got, []string{"stop:web:false"}) {
		t.Fatalf("stop events = %#v", got)
	}
}

func TestExecuteLowMemoryProfileCapsParallelism(t *testing.T) {
	t.Parallel()

	workspace := orchestrationWorkspace()
	workspace.DefaultProfileID = "low-memory"
	workspace.Profiles = []domain.Profile{{
		ID: "low-memory", Name: "Low memory", ProjectIDs: []string{"api", "worker", "web"}, MaxParallel: 1, LowMemory: true,
	}}
	repository := newFakeRepository(workspace)
	operator := &fakeProjectOperator{start: func(context.Context, string) error {
		time.Sleep(2 * time.Millisecond)
		return nil
	}}
	service := NewService(repository, operator, noHealthGate{}, allowMembers{})

	result, err := service.Execute(context.Background(), workspace.ID, ExecuteRequest{Kind: domain.ExecutionStart}, nil)
	if err != nil || result.ProfileID != "low-memory" {
		t.Fatalf("Execute(profile) = profile %q error %v", result.ProfileID, err)
	}
	if got := operator.maxActiveCount(); got != 1 {
		t.Fatalf("maximum concurrent starts = %d, want 1", got)
	}
}

func TestExecuteCancellationPersistsCancelledSummary(t *testing.T) {
	t.Parallel()

	workspace := domain.Workspace{
		ID: "workspace-cancel", Name: "Cancel", DefaultFailurePolicy: domain.FailurePolicyContinue, Revision: 1,
		Members: []domain.Member{{ProjectID: "api", Role: domain.MemberRoleApplication}},
	}
	repository := newFakeRepository(workspace)
	started := make(chan struct{})
	operator := &fakeProjectOperator{start: func(ctx context.Context, _ string) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	}}
	service := NewService(repository, operator, noHealthGate{}, allowMembers{})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := service.Execute(ctx, workspace.ID, ExecuteRequest{Kind: domain.ExecutionStart}, nil)
		done <- err
	}()
	<-started
	cancel()
	err := <-done
	var executionErr *ExecutionError
	if !errors.As(err, &executionErr) || !errors.Is(err, context.Canceled) {
		t.Fatalf("Execute(cancelled) error = %#v", err)
	}
	latest := repository.latestExecution()
	if latest.State != domain.ExecutionCancelled || latest.Projects[0].Status != domain.ProjectCancelled {
		t.Fatalf("persisted cancellation = %#v", latest)
	}
}

func TestExecuteReportsProjectVisibleProgress(t *testing.T) {
	t.Parallel()

	workspace := linearWorkspace()
	repository := newFakeRepository(workspace)
	reporter := &recordingReporter{}
	service := NewService(repository, &fakeProjectOperator{}, noHealthGate{}, allowMembers{})
	if _, err := service.Execute(context.Background(), workspace.ID, ExecuteRequest{Kind: domain.ExecutionStart}, reporter); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	seen := reporter.statuses()
	if !slices.Contains(seen, domain.ProjectStarting) || !slices.Contains(seen, domain.ProjectRunning) {
		t.Fatalf("reported statuses = %#v", seen)
	}
}

type fakeRepository struct {
	mu         sync.Mutex
	workspaces map[string]domain.Workspace
	executions []domain.ExecutionSummary
}

func newFakeRepository(workspaces ...domain.Workspace) *fakeRepository {
	repository := &fakeRepository{workspaces: make(map[string]domain.Workspace)}
	for _, workspace := range workspaces {
		repository.workspaces[workspace.ID] = workspace
	}
	return repository
}

func (r *fakeRepository) Create(_ context.Context, workspace domain.Workspace) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.workspaces[workspace.ID] = workspace
	return nil
}
func (r *fakeRepository) Update(_ context.Context, workspace domain.Workspace, _ int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.workspaces[workspace.ID] = workspace
	return nil
}
func (r *fakeRepository) Get(_ context.Context, id string) (domain.Workspace, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	workspace, ok := r.workspaces[id]
	if !ok {
		return domain.Workspace{}, ErrNotFound
	}
	return workspace, nil
}
func (r *fakeRepository) List(context.Context) ([]domain.Workspace, error) { return nil, nil }
func (r *fakeRepository) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.workspaces, id)
	return nil
}
func (r *fakeRepository) SaveExecution(_ context.Context, execution domain.ExecutionSummary) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.executions = append(r.executions, cloneSummary(execution))
	return nil
}
func (r *fakeRepository) latestExecution() domain.ExecutionSummary {
	r.mu.Lock()
	defer r.mu.Unlock()
	return cloneSummary(r.executions[len(r.executions)-1])
}

type fakeProjectOperator struct {
	mu        sync.Mutex
	events    []string
	active    int
	maxActive int
	start     func(context.Context, string) error
	stop      func(context.Context, string, StopOptions) error
}

func (o *fakeProjectOperator) Start(ctx context.Context, projectID string) error {
	o.mu.Lock()
	o.events = append(o.events, "start:"+projectID)
	o.active++
	if o.active > o.maxActive {
		o.maxActive = o.active
	}
	o.mu.Unlock()
	defer func() {
		o.mu.Lock()
		o.active--
		o.mu.Unlock()
	}()
	if o.start != nil {
		return o.start(ctx, projectID)
	}
	return nil
}
func (o *fakeProjectOperator) Stop(ctx context.Context, projectID string, options StopOptions) error {
	o.mu.Lock()
	o.events = append(o.events, fmt.Sprintf("stop:%s:%t", projectID, options.RemoveData))
	o.mu.Unlock()
	if o.stop != nil {
		return o.stop(ctx, projectID, options)
	}
	return nil
}
func (o *fakeProjectOperator) eventsCopy() []string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return slices.Clone(o.events)
}
func (o *fakeProjectOperator) stopEvents() []string {
	result := []string{}
	for _, event := range o.eventsCopy() {
		if len(event) >= 5 && event[:5] == "stop:" {
			result = append(result, event)
		}
	}
	return result
}
func (o *fakeProjectOperator) maxActiveCount() int {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.maxActive
}

type healthGateFunc func(context.Context, string, time.Duration) error

func (f healthGateFunc) WaitHealthy(ctx context.Context, projectID string, timeout time.Duration) error {
	return f(ctx, projectID, timeout)
}

type noHealthGate struct{}

func (noHealthGate) WaitHealthy(context.Context, string, time.Duration) error { return nil }

type allowMembers struct{}

func (allowMembers) ValidateWorkspaceMember(context.Context, string) error { return nil }

type eventLog struct {
	mu     sync.Mutex
	events []string
}

func (l *eventLog) add(event string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, event)
}
func (l *eventLog) copy() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return slices.Clone(l.events)
}

type recordingReporter struct {
	mu       sync.Mutex
	progress []domain.ProjectResult
}

func (r *recordingReporter) ProjectProgress(_ context.Context, progress domain.ProjectResult) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.progress = append(r.progress, progress)
	return nil
}
func (r *recordingReporter) statuses() []domain.ProjectStatus {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]domain.ProjectStatus, 0, len(r.progress))
	for _, progress := range r.progress {
		result = append(result, progress.Status)
	}
	return result
}

func orchestrationWorkspace() domain.Workspace {
	return domain.Workspace{
		ID: "workspace-product", Name: "Product", DefaultFailurePolicy: domain.FailurePolicyRollback, Revision: 1,
		Members: []domain.Member{
			{ProjectID: "database", Role: domain.MemberRoleDependency, Order: 0},
			{ProjectID: "cache", Role: domain.MemberRoleDependency, Order: 1},
			{ProjectID: "api", Role: domain.MemberRoleApplication, Order: 2},
			{ProjectID: "worker", Role: domain.MemberRoleApplication, Order: 3},
			{ProjectID: "web", Role: domain.MemberRoleApplication, Order: 4},
		},
		Dependencies: []domain.Dependency{
			{ProjectID: "api", DependsOnProjectID: "database"},
			{ProjectID: "worker", DependsOnProjectID: "cache"},
			{ProjectID: "web", DependsOnProjectID: "api"},
		},
	}
}

func linearWorkspace() domain.Workspace {
	return domain.Workspace{
		ID: "workspace-linear", Name: "Linear", DefaultFailurePolicy: domain.FailurePolicyRollback, Revision: 1,
		Members: []domain.Member{
			{ProjectID: "database", Role: domain.MemberRoleDependency, Order: 0},
			{ProjectID: "api", Role: domain.MemberRoleApplication, Order: 1},
			{ProjectID: "web", Role: domain.MemberRoleApplication, Order: 2},
		},
		Dependencies: []domain.Dependency{
			{ProjectID: "api", DependsOnProjectID: "database"},
			{ProjectID: "web", DependsOnProjectID: "api"},
		},
	}
}

func resultStatuses(result domain.ExecutionSummary) map[string]domain.ProjectStatus {
	statuses := make(map[string]domain.ProjectStatus, len(result.Projects))
	for _, project := range result.Projects {
		statuses[project.ProjectID] = project.Status
	}
	return statuses
}

func assertBefore(t *testing.T, events []string, before, after string) {
	t.Helper()
	left := slices.Index(events, before)
	right := slices.Index(events, after)
	if left < 0 || right < 0 || left >= right {
		t.Fatalf("events %#v do not place %q before %q", events, before, after)
	}
}
