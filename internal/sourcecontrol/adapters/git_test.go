package adapters

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"switchyard.dev/switchyard/internal/sourcecontrol/domain"
)

func TestParseStatusPorcelainV2(t *testing.T) {
	t.Parallel()
	input := []byte("# branch.oid abc123\n# branch.head main\n# branch.upstream origin/main\n# branch.ab +2 -1\n# stash 3\n1 M. N... 100644 100644 100644 a b staged.txt\n1 .M N... 100644 100644 100644 a b dirty.txt\n? new.txt\nu UU N... 100644 100644 100644 100644 a b c conflict.txt\n")
	var state domain.State
	parseStatus(input, &state)
	if state.Branch != "main" || state.Ahead != 2 || state.Behind != 1 || state.Stashes != 3 {
		t.Fatalf("branch metadata = %#v", state)
	}
	if state.Changes != (domain.ChangeCounts{Staged: 1, Modified: 1, Untracked: 1, Conflicted: 1}) {
		t.Fatalf("changes = %#v", state.Changes)
	}
}

func TestGitObservesRealRepositoryChanges(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	runGit(t, root, "init", "-b", "main")
	runGit(t, root, "config", "user.name", "Switchyard Test")
	runGit(t, root, "config", "user.email", "switchyard@example.invalid")
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("first\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "tracked.txt")
	runGit(t, root, "commit", "-m", "initial")
	observer := NewGit()
	clean, err := observer.Observe(context.Background(), "project-1", root)
	if err != nil {
		t.Fatal(err)
	}
	if !clean.Repository || clean.Branch != "main" || clean.LastCommit == nil || clean.Changes != (domain.ChangeCounts{}) {
		t.Fatalf("clean = %#v", clean)
	}
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("changed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "untracked.txt"), []byte("new\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	dirty, err := observer.Observe(context.Background(), "project-1", root)
	if err != nil {
		t.Fatal(err)
	}
	if dirty.Changes.Modified != 1 || dirty.Changes.Untracked != 1 {
		t.Fatalf("dirty = %#v", dirty.Changes)
	}
}

func runGit(t *testing.T, root string, arguments ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, arguments...)...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", arguments, err, output)
	}
}
