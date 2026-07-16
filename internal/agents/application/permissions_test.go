package application

import (
	"errors"
	"testing"
)

func TestObserveCannotMutateAndAdminIsExplicit(t *testing.T) {
	t.Parallel()
	observe, err := NewScope("codex", "session-1", ProfileObserve, nil)
	if err != nil {
		t.Fatal(err)
	}
	if observe.Allows(CapabilityLifecycle) || observe.Allows(CapabilityDestructive) {
		t.Fatal("observe profile received mutation capability")
	}
	admin, err := NewScope("claude", "operator", ProfileAdmin, []string{"project-1"})
	if err != nil {
		t.Fatal(err)
	}
	if err := admin.Authorize(CapabilityDestructive, "project-1"); err != nil {
		t.Fatalf("admin authorization failed: %v", err)
	}
	if err := admin.Authorize(CapabilityLifecycle, "project-2"); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("out-of-scope error = %v", err)
	}
}

func TestScopeValidatesAuditIdentity(t *testing.T) {
	t.Parallel()
	if _, err := NewScope("bad provider", "id", ProfileDevelop, nil); err == nil {
		t.Fatal("invalid provider accepted")
	}
	scope, err := NewScope("generic", "", ProfileDevelop, nil)
	if err != nil {
		t.Fatal(err)
	}
	if scope.ActorID() != "generic/generic" {
		t.Fatalf("actor ID = %q", scope.ActorID())
	}
}

func TestProjectScopeAppliesToReadAndMutation(t *testing.T) {
	t.Parallel()
	scope, err := NewScope("codex", "worker", ProfileDevelop, []string{"project-a"})
	if err != nil {
		t.Fatal(err)
	}
	if err := scope.AuthorizeRead("project-a"); err != nil {
		t.Fatalf("AuthorizeRead(project-a) error = %v", err)
	}
	if err := scope.AuthorizeRead("project-b"); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("AuthorizeRead(project-b) error = %v", err)
	}
	if err := scope.Authorize(CapabilityLifecycle, "project-b"); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("Authorize(project-b) error = %v", err)
	}
}
