package application_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"switchyard.dev/switchyard/internal/operations/application"
	"switchyard.dev/switchyard/internal/operations/domain"
	"switchyard.dev/switchyard/internal/platform/sqlite"
)

func TestCoordinatorIdempotencyAndProjectSerialization(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	database, repository, journal := operationFixture(ctx, t)
	defer closeDatabase(t, database)

	executor := &concurrencyExecutor{release: make(chan struct{}), current: make(map[string]int), maximum: make(map[string]int)}
	coordinator := application.NewCoordinator(ctx, repository, journal, executor)
	first, err := coordinator.Submit(ctx, request("project-a", "key-a"))
	if err != nil {
		t.Fatalf("Submit(first) error = %v", err)
	}
	duplicate, err := coordinator.Submit(ctx, request("project-a", "key-a"))
	if err != nil || duplicate.ID != first.ID {
		t.Fatalf("duplicate = %#v, %v", duplicate, err)
	}
	second, err := coordinator.Submit(ctx, request("project-a", "key-b"))
	if err != nil {
		t.Fatalf("Submit(second) error = %v", err)
	}
	third, err := coordinator.Submit(ctx, request("project-b", "key-c"))
	if err != nil {
		t.Fatalf("Submit(third) error = %v", err)
	}

	executor.waitForTotal(t, 2)
	close(executor.release)
	for _, id := range []string{first.ID, second.ID, third.ID} {
		operation, err := coordinator.Wait(ctx, id)
		if err != nil || operation.State != domain.StateSucceeded {
			t.Fatalf("Wait(%s) = %#v, %v", id, operation, err)
		}
	}
	executor.mu.Lock()
	defer executor.mu.Unlock()
	if executor.maximum["project-a"] != 1 {
		t.Fatalf("same-project maximum = %d, want 1", executor.maximum["project-a"])
	}
	if executor.maximumTotal < 2 {
		t.Fatalf("cross-project maximum = %d, want at least 2", executor.maximumTotal)
	}
	if executor.executions != 3 {
		t.Fatalf("executions = %d, want 3", executor.executions)
	}
	listed, err := coordinator.List(ctx, "project-a", 100)
	if err != nil || len(listed) != 2 {
		t.Fatalf("List(project-a) = %d, %v", len(listed), err)
	}
}

func TestCoordinatorCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	database, repository, journal := operationFixture(ctx, t)
	defer closeDatabase(t, database)
	started := make(chan struct{})
	coordinator := application.NewCoordinator(ctx, repository, journal, application.ExecutorFunc(
		func(ctx context.Context, _ domain.Operation, _ application.Progress) error {
			close(started)
			<-ctx.Done()
			return ctx.Err()
		},
	))
	operation, err := coordinator.Submit(ctx, request("project-a", "cancel-key"))
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}
	<-started
	if _, err := coordinator.Cancel(ctx, operation.ID, "cli", "test", "cancel-request"); err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}
	finished, err := coordinator.Wait(ctx, operation.ID)
	if err != nil || finished.State != domain.StateCancelled {
		t.Fatalf("Wait() = %#v, %v", finished, err)
	}
}

func TestCoordinatorPersistsPartialSuccess(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	database, repository, journal := operationFixture(ctx, t)
	defer closeDatabase(t, database)
	coordinator := application.NewCoordinator(ctx, repository, journal, application.ExecutorFunc(
		func(context.Context, domain.Operation, application.Progress) error {
			return application.PartialSuccess("one workspace member failed")
		},
	))
	operation, err := coordinator.Submit(ctx, request("workspace:fixture", "partial-key"))
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}
	finished, err := coordinator.Wait(ctx, operation.ID)
	if err != nil {
		t.Fatalf("Wait() error = %v", err)
	}
	if finished.State != domain.StatePartiallySucceeded || finished.ErrorCode != "OPERATION_PARTIAL" {
		t.Fatalf("finished = %#v", finished)
	}
}

func TestCoordinatorRecoveryFailsInterruptedAndResumesQueued(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	database, repository, journal := operationFixture(ctx, t)
	defer closeDatabase(t, database)
	now := time.Now().UTC()
	interrupted := domain.Operation{
		ID: "op_interrupted", ProjectID: "project-a", Kind: "test.fixture",
		State: domain.StateQueued, IdempotencyKey: "interrupted-key",
		Input: []byte(`{}`), RequestedAt: now, UpdatedAt: now,
	}
	if _, _, err := repository.CreateOrGet(ctx, interrupted); err != nil {
		t.Fatalf("CreateOrGet(interrupted) error = %v", err)
	}
	running, err := interrupted.Transition(domain.StateRunning, now.Add(time.Second), "", "")
	if err != nil || repository.Transition(ctx, interrupted, running) != nil {
		t.Fatalf("seed running operation: %v", err)
	}
	queued := domain.Operation{
		ID: "op_queued", ProjectID: "project-b", Kind: "test.fixture",
		State: domain.StateQueued, IdempotencyKey: "queued-key",
		Input: []byte(`{}`), RequestedAt: now, UpdatedAt: now,
	}
	if _, _, err := repository.CreateOrGet(ctx, queued); err != nil {
		t.Fatalf("CreateOrGet(queued) error = %v", err)
	}
	coordinator := application.NewCoordinator(ctx, repository, journal, application.ExecutorFunc(
		func(context.Context, domain.Operation, application.Progress) error { return nil },
	))
	if err := coordinator.Recover(ctx); err != nil {
		t.Fatalf("Recover() error = %v", err)
	}
	resumed, err := coordinator.Wait(ctx, queued.ID)
	if err != nil || resumed.State != domain.StateSucceeded {
		t.Fatalf("resumed = %#v, %v", resumed, err)
	}
	failed, err := coordinator.Get(ctx, interrupted.ID)
	if err != nil || failed.State != domain.StateFailed || failed.ErrorCode != "DAEMON_RESTARTED" {
		t.Fatalf("interrupted = %#v, %v", failed, err)
	}
}

func request(projectID, key string) application.SubmitRequest {
	return application.SubmitRequest{
		ProjectID: projectID, Kind: "test.fixture", IdempotencyKey: key,
		Input: []byte(`{}`), ActorType: "test", ActorID: "coordinator-test",
	}
}

func operationFixture(ctx context.Context, t *testing.T) (*sqlite.Database, *sqlite.OperationRepository, *sqlite.Journal) {
	t.Helper()
	database, err := sqlite.Open(ctx, t.TempDir()+"/switchyard.db")
	if err != nil {
		t.Fatalf("sqlite.Open() error = %v", err)
	}
	return database, sqlite.NewOperationRepository(database), sqlite.NewJournal(database)
}

func closeDatabase(t *testing.T, database *sqlite.Database) {
	t.Helper()
	if err := database.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

type concurrencyExecutor struct {
	mu           sync.Mutex
	release      chan struct{}
	current      map[string]int
	maximum      map[string]int
	currentTotal int
	maximumTotal int
	executions   int
}

func (e *concurrencyExecutor) Execute(_ context.Context, operation domain.Operation, progress application.Progress) error {
	e.mu.Lock()
	e.executions++
	e.current[operation.ProjectID]++
	e.currentTotal++
	e.maximum[operation.ProjectID] = max(e.maximum[operation.ProjectID], e.current[operation.ProjectID])
	e.maximumTotal = max(e.maximumTotal, e.currentTotal)
	e.mu.Unlock()
	if err := progress.Step(context.Background(), "fixture", "running", "test fixture running"); err != nil {
		return err
	}
	<-e.release
	e.mu.Lock()
	e.current[operation.ProjectID]--
	e.currentTotal--
	e.mu.Unlock()
	return nil
}

func (e *concurrencyExecutor) waitForTotal(t *testing.T, want int) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		e.mu.Lock()
		current := e.currentTotal
		e.mu.Unlock()
		if current >= want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("executor did not reach %d concurrent operations", want)
}
