package application

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"switchyard.dev/switchyard/internal/actions/domain"
)

type projectSourceStub struct{ project domain.ProjectActions }

func (s projectSourceStub) ResolveActions(context.Context, string) (domain.ProjectActions, error) {
	return s.project, nil
}

type runnerStub struct{ execution domain.Execution }

func (r *runnerStub) Run(_ context.Context, execution domain.Execution) error {
	r.execution = execution
	return nil
}

type auditStub struct{ begun, finished bool }

func (a *auditStub) Begin(context.Context, domain.Audit) error { a.begun = true; return nil }
func (a *auditStub) Finish(context.Context, string, string, string, time.Time) error {
	a.finished = true
	return nil
}

func TestResolveWorkingDirectoryRejectsParentAndSymlinkEscape(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	outside := t.TempDir()
	if _, err := ResolveWorkingDirectory(root, outside, false); !errors.Is(err, ErrWorkingDirEscape) {
		t.Fatalf("absolute escape error = %v", err)
	}
	link := filepath.Join(root, "outside")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveWorkingDirectory(root, "outside", false); !errors.Is(err, ErrWorkingDirEscape) {
		t.Fatalf("symlink escape error = %v", err)
	}
	resolvedOutside, err := filepath.EvalSymlinks(outside)
	if err != nil {
		t.Fatal(err)
	}
	if resolved, err := ResolveWorkingDirectory(root, "outside", true); err != nil || resolved != resolvedOutside {
		t.Fatalf("explicit escape = %q, %v", resolved, err)
	}
}

func TestExecuteUsesTrustedWorkingDirectoryAndAudit(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	runner, audits := &runnerStub{}, &auditStub{}
	service := NewService(projectSourceStub{project: domain.ProjectActions{ProjectID: "project", Root: root, Actions: []domain.Definition{
		{ID: "terminal", Type: "terminal.open", WorkingDirectory: ".", Risk: domain.RiskInteractive},
	}}}, runner, audits)
	if err := service.Execute(context.Background(), "operation", "project", "terminal", "cli", "user", false, false); err != nil {
		t.Fatal(err)
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if runner.execution.WorkingDirectory != resolvedRoot || !audits.begun || !audits.finished {
		t.Fatalf("execution=%#v audit=%#v", runner.execution, audits)
	}
}

func TestExecuteRequiresDestructiveConfirmationBeforeAudit(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	runner, audits := &runnerStub{}, &auditStub{}
	service := NewService(projectSourceStub{project: domain.ProjectActions{ProjectID: "project", Root: root, Actions: []domain.Definition{
		{ID: "destroy", Type: "command", Command: []string{"false"}, Risk: domain.RiskDestructive},
	}}}, runner, audits)
	err := service.Execute(context.Background(), "operation", "project", "destroy", "cli", "user", false, false)
	if !errors.Is(err, ErrConfirmationRequired) || audits.begun {
		t.Fatalf("error=%v audit=%#v", err, audits)
	}
}
