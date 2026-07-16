package application

import (
	"context"
	"errors"
	"testing"

	"switchyard.dev/switchyard/internal/sourcecontrol/domain"
)

func TestListWorktreesResolvesTrustedRootAndUsesExplicitInventory(t *testing.T) {
	t.Parallel()

	observer := &worktreeObserverStub{items: []domain.Worktree{{Path: "/repo/feature", Branch: "feature"}}}
	service := NewService(projectSourceStub{root: "/repo"}, observer)
	worktrees, err := service.ListWorktrees(context.Background(), "project-1")
	if err != nil {
		t.Fatal(err)
	}
	if observer.root != "/repo" || len(worktrees) != 1 || worktrees[0].Branch != "feature" {
		t.Fatalf("root=%q worktrees=%#v", observer.root, worktrees)
	}
}

func TestListWorktreesPreservesTrustAndCapabilityErrors(t *testing.T) {
	t.Parallel()

	untrusted := errors.New("untrusted")
	service := NewService(projectSourceStub{err: untrusted}, &worktreeObserverStub{})
	if _, err := service.ListWorktrees(context.Background(), "project-1"); !errors.Is(err, untrusted) {
		t.Fatalf("trust error = %v", err)
	}
	service = NewService(projectSourceStub{root: "/repo"}, observeOnlyStub{})
	if _, err := service.ListWorktrees(context.Background(), "project-1"); !errors.Is(err, ErrWorktreeObservationUnsupported) {
		t.Fatalf("capability error = %v", err)
	}
}

type projectSourceStub struct {
	root string
	err  error
}

func (s projectSourceStub) Root(context.Context, string) (string, error) { return s.root, s.err }

type observeOnlyStub struct{}

func (observeOnlyStub) Observe(context.Context, string, string) (domain.State, error) {
	return domain.State{}, nil
}

type worktreeObserverStub struct {
	items []domain.Worktree
	root  string
}

func (s *worktreeObserverStub) Observe(context.Context, string, string) (domain.State, error) {
	return domain.State{}, nil
}

func (s *worktreeObserverStub) ObserveWorktrees(_ context.Context, root string) ([]domain.Worktree, error) {
	s.root = root
	return s.items, nil
}
