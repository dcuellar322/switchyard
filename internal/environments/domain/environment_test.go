package domain

import (
	"strings"
	"testing"
)

func TestStableIdentityAndAllocationNames(t *testing.T) {
	t.Parallel()

	first, err := StableID("project-1", "/repo/worktrees/feature")
	if err != nil {
		t.Fatal(err)
	}
	again, err := StableID("project-1", "/repo/worktrees/feature/.")
	if err != nil {
		t.Fatal(err)
	}
	other, err := StableID("project-1", "/repo/worktrees/other")
	if err != nil {
		t.Fatal(err)
	}
	if first != again || first == other {
		t.Fatalf("IDs first=%q again=%q other=%q", first, again, other)
	}
	name, err := ComposeProjectName(strings.Repeat("Project Name ", 20), "Feature/One", first)
	if err != nil {
		t.Fatal(err)
	}
	if len(name) > MaximumComposeProjectName || !composeNamePattern.MatchString(name) {
		t.Fatalf("Compose name = %q", name)
	}
	namespace, err := PortLeaseNamespace(first)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(namespace, "worktree:") || PortOffsetSeed(first) < 1 {
		t.Fatalf("port allocation = %q/%d", namespace, PortOffsetSeed(first))
	}
	hostname, err := LocalhostName("Project Name", first)
	if err != nil || !strings.HasSuffix(hostname, ".localhost") {
		t.Fatalf("hostname=%q error=%v", hostname, err)
	}
}

func TestDifferentProjectsCannotShareRuntimeIdentity(t *testing.T) {
	t.Parallel()

	first, _ := StableID("project-1", "/repo/feature")
	second, _ := StableID("project-2", "/repo/feature")
	firstName, _ := ComposeProjectName("project", "feature", first)
	secondName, _ := ComposeProjectName("project", "feature", second)
	firstNamespace, _ := PortLeaseNamespace(first)
	secondNamespace, _ := PortLeaseNamespace(second)
	firstHost, _ := LocalhostName("project", first)
	secondHost, _ := LocalhostName("project", second)
	if first == second || firstName == secondName || firstNamespace == secondNamespace || firstHost == secondHost {
		t.Fatalf("runtime identities collided: %q %q / %q %q", firstName, secondName, firstNamespace, secondNamespace)
	}
}
