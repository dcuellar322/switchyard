package application

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"switchyard.dev/switchyard/internal/environments/domain"
)

func TestRegisterWorktreesCreatesIsolatedRunnableEnvironments(t *testing.T) {
	t.Parallel()

	repository := &repositoryStub{}
	service := newTestService(repository, []WorktreeObservation{
		{Path: "/repo/main", Head: "abc", Branch: "main"},
		{Path: "/repo/feature", Head: "def", Branch: "feature/one", Locked: true},
		{Path: "/repo/bare", Head: "000", Bare: true},
	})
	registration, err := service.RegisterWorktrees(context.Background(), "project-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(registration.Environments) != 3 || !registration.Environments[0].Primary {
		t.Fatalf("registration = %#v", registration)
	}
	composeNames := make(map[string]struct{})
	namespaces := make(map[string]struct{})
	offsets := make(map[int]struct{})
	for _, environment := range registration.Environments {
		composeNames[environment.Allocation.ComposeProjectName] = struct{}{}
		namespaces[environment.Allocation.PortLeaseNamespace] = struct{}{}
		offsets[environment.Allocation.PortOffset] = struct{}{}
		if err := environment.Validate(); err != nil {
			t.Fatalf("environment %q: %v", environment.ID, err)
		}
	}
	if len(composeNames) != 3 || len(namespaces) != 3 || len(offsets) != 3 {
		t.Fatalf("allocations names=%v namespaces=%v offsets=%v", composeNames, namespaces, offsets)
	}
	bare := findEnvironment(t, registration.Environments, "/repo/bare")
	if bare.Availability != domain.AvailabilityUnavailable || bare.UnavailableReason == "" {
		t.Fatalf("bare environment = %#v", bare)
	}
	feature := findEnvironment(t, registration.Environments, "/repo/feature")
	if !feature.Locked || feature.Availability != domain.AvailabilityAvailable {
		t.Fatalf("locked environment = %#v", feature)
	}
}

func TestRegistrationPreservesAllocationAndReportsRemovedWorktree(t *testing.T) {
	t.Parallel()

	repository := &repositoryStub{}
	worktrees := &worktreeSourceStub{items: []WorktreeObservation{
		{Path: "/repo/main", Branch: "main"},
		{Path: "/repo/feature", Branch: "feature"},
	}}
	service := &Service{
		projects:  projectSourceStub{descriptor: ProjectDescriptor{ID: "project-1", Slug: "project", PrimaryLocation: "/repo/main"}},
		worktrees: worktrees, repository: repository, now: func() time.Time { return time.Unix(10, 0) },
	}
	first, err := service.RegisterWorktrees(context.Background(), "project-1")
	if err != nil {
		t.Fatal(err)
	}
	feature := findEnvironment(t, first.Environments, "/repo/feature")
	worktrees.items = []WorktreeObservation{
		{Path: "/repo/new", Branch: "new"},
		{Path: "/repo/main", Branch: "main"},
	}
	service.now = func() time.Time { return time.Unix(20, 0) }
	second, err := service.RegisterWorktrees(context.Background(), "project-1")
	if err != nil {
		t.Fatal(err)
	}
	mainBefore := findEnvironment(t, first.Environments, "/repo/main")
	mainAfter := findEnvironment(t, second.Environments, "/repo/main")
	if mainBefore.Allocation.PortOffset != mainAfter.Allocation.PortOffset || !mainBefore.RegisteredAt.Equal(mainAfter.RegisteredAt) {
		t.Fatalf("stable allocation before=%#v after=%#v", mainBefore, mainAfter)
	}
	if !slices.Equal(second.RemovedIDs, []string{feature.ID}) {
		t.Fatalf("removed IDs = %v", second.RemovedIDs)
	}
}

func TestRegisterWorktreesProjectsNestedMonorepoLocationIntoEveryCheckout(t *testing.T) {
	t.Parallel()

	repository := &repositoryStub{}
	service := &Service{
		projects: projectSourceStub{descriptor: ProjectDescriptor{
			ID: "project-1", Slug: "api", PrimaryLocation: "/repo/main/services/api",
		}},
		worktrees: &worktreeSourceStub{items: []WorktreeObservation{
			{Path: "/repo/main", Branch: "main"},
			{Path: "/repo/feature", Branch: "feature"},
		}},
		repository: repository,
		now:        func() time.Time { return time.Unix(10, 0) },
	}

	registration, err := service.RegisterWorktrees(context.Background(), "project-1")
	if err != nil {
		t.Fatal(err)
	}
	primary := findEnvironment(t, registration.Environments, "/repo/main/services/api")
	if !primary.Primary {
		t.Fatalf("primary environment = %#v", primary)
	}
	feature := findEnvironment(t, registration.Environments, "/repo/feature/services/api")
	if feature.Primary || feature.Name != "feature" {
		t.Fatalf("feature environment = %#v", feature)
	}
}

func TestRegisterWorktreesRejectsProjectOutsideObservedCheckouts(t *testing.T) {
	t.Parallel()

	repository := &repositoryStub{}
	service := &Service{
		projects: projectSourceStub{descriptor: ProjectDescriptor{
			ID: "project-1", Slug: "project", PrimaryLocation: "/different/project",
		}},
		worktrees:  &worktreeSourceStub{items: []WorktreeObservation{{Path: "/repo/main", Branch: "main"}}},
		repository: repository,
		now:        func() time.Time { return time.Unix(10, 0) },
	}

	if _, err := service.RegisterWorktrees(context.Background(), "project-1"); err == nil {
		t.Fatal("RegisterWorktrees accepted a project outside the observed worktrees")
	}
	if repository.replaces != 0 {
		t.Fatalf("repository replacements = %d", repository.replaces)
	}
}

func TestRegistrationResolvesLogicalPortOffsetCollision(t *testing.T) {
	t.Parallel()

	id, err := domain.StableID("project-1", "/repo/main")
	if err != nil {
		t.Fatal(err)
	}
	repository := &repositoryStub{items: []domain.Environment{{
		ID: "env-other", ProjectID: "project-2",
		Allocation: domain.RuntimeAllocation{PortOffset: domain.PortOffsetSeed(id)},
	}}}
	service := newTestService(repository, []WorktreeObservation{{Path: "/repo/main", Branch: "main"}})
	registration, err := service.RegisterWorktrees(context.Background(), "project-1")
	if err != nil {
		t.Fatal(err)
	}
	if registration.Environments[0].Allocation.PortOffset == domain.PortOffsetSeed(id) {
		t.Fatal("registration retained an occupied logical port offset")
	}
}

func TestRegistrationFailureAndCancellationDoNotReplaceState(t *testing.T) {
	t.Parallel()

	repository := &repositoryStub{}
	service := newTestService(repository, nil)
	if _, err := service.RegisterWorktrees(context.Background(), "project-1"); !errors.Is(err, ErrNoWorktrees) {
		t.Fatalf("empty inventory error = %v", err)
	}
	if repository.replaces != 0 {
		t.Fatalf("replaces = %d", repository.replaces)
	}

	service = newTestService(repository, []WorktreeObservation{{Path: "/repo/main", Branch: "main"}})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.RegisterWorktrees(ctx, "project-1"); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled registration error = %v", err)
	}
	if repository.replaces != 0 {
		t.Fatalf("replaces after cancellation = %d", repository.replaces)
	}
}

func TestConfigureRuntimeValidatesRouteAndExactLeases(t *testing.T) {
	t.Parallel()

	repository := &repositoryStub{}
	service := newTestService(repository, []WorktreeObservation{
		{Path: "/repo/main", Branch: "main"},
		{Path: "/repo/feature", Branch: "feature"},
	})
	registration, err := service.RegisterWorktrees(context.Background(), "project-1")
	if err != nil {
		t.Fatal(err)
	}
	main := findEnvironment(t, registration.Environments, "/repo/main")
	configured, err := service.ConfigureRuntime(context.Background(), main.ID, RuntimeConfiguration{
		State: domain.StateActive, Target: "http://127.0.0.1:18080",
		PortLeases: []domain.PortLease{{PortID: "web", Protocol: "tcp", TargetPort: 8080, HostPort: 18080}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if configured.State != domain.StateActive || configured.Target == "" || len(configured.Allocation.PortLeases) != 1 {
		t.Fatalf("configured = %#v", configured)
	}
	if _, err := service.ConfigureRuntime(context.Background(), main.ID, RuntimeConfiguration{
		State: domain.StateActive, Target: "https://127.0.0.1:18080",
	}); err == nil {
		t.Fatal("ConfigureRuntime accepted HTTPS")
	}
	feature := findEnvironment(t, registration.Environments, "/repo/feature")
	if _, err := service.ConfigureRuntime(context.Background(), feature.ID, RuntimeConfiguration{
		State: domain.StateActive, Target: "http://127.0.0.1:18081",
		PortLeases: []domain.PortLease{{PortID: "web", Protocol: "tcp", TargetPort: 8080, HostPort: 18080}},
	}); !errors.Is(err, ErrRuntimeConflict) {
		t.Fatalf("port conflict error = %v", err)
	}
}

func newTestService(repository EnvironmentRepository, worktrees []WorktreeObservation) *Service {
	return &Service{
		projects:  projectSourceStub{descriptor: ProjectDescriptor{ID: "project-1", Slug: "project", PrimaryLocation: "/repo/main"}},
		worktrees: &worktreeSourceStub{items: worktrees}, repository: repository,
		now: func() time.Time { return time.Unix(10, 0) },
	}
}

type projectSourceStub struct {
	descriptor ProjectDescriptor
	err        error
}

func (s projectSourceStub) Project(context.Context, string) (ProjectDescriptor, error) {
	return s.descriptor, s.err
}

type worktreeSourceStub struct {
	items []WorktreeObservation
	err   error
}

func (s *worktreeSourceStub) ListWorktrees(ctx context.Context, _ string) ([]WorktreeObservation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return slices.Clone(s.items), s.err
}

type repositoryStub struct {
	items    []domain.Environment
	replaces int
}

func (r *repositoryStub) Get(ctx context.Context, environmentID string) (domain.Environment, error) {
	if err := ctx.Err(); err != nil {
		return domain.Environment{}, err
	}
	for _, environment := range r.items {
		if environment.ID == environmentID {
			return environment, nil
		}
	}
	return domain.Environment{}, errors.New("environment not found")
}

func (r *repositoryStub) List(ctx context.Context) ([]domain.Environment, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return slices.Clone(r.items), nil
}

func (r *repositoryStub) ReplaceProject(ctx context.Context, projectID string, environments []domain.Environment) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.replaces++
	retained := make([]domain.Environment, 0, len(r.items)+len(environments))
	for _, environment := range r.items {
		if environment.ProjectID != projectID {
			retained = append(retained, environment)
		}
	}
	retained = append(retained, environments...)
	r.items = retained
	return nil
}

func (r *repositoryStub) Update(ctx context.Context, updated domain.Environment) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	for index := range r.items {
		if r.items[index].ID == updated.ID {
			r.items[index] = updated
			return nil
		}
	}
	return errors.New("environment not found")
}

func findEnvironment(t *testing.T, environments []domain.Environment, path string) domain.Environment {
	t.Helper()
	for _, environment := range environments {
		if environment.Path == path {
			return environment
		}
	}
	t.Fatalf("environment path %q not found in %#v", path, environments)
	return domain.Environment{}
}
