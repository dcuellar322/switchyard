package bootstrap

import (
	"context"
	"testing"

	workspaceDomain "switchyard.dev/switchyard/internal/workspace/domain"
)

type recordingWorkspaceProgress struct {
	name    string
	state   string
	message string
	calls   int
}

func (p *recordingWorkspaceProgress) Step(_ context.Context, name, state, message string) error {
	p.name = name
	p.state = state
	p.message = message
	p.calls++
	return nil
}

func TestWorkspaceProgressMapsDomainStatusesToOperationStepStates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want string
		in   []workspaceDomain.ProjectStatus
	}{
		{name: "in progress", want: "running", in: []workspaceDomain.ProjectStatus{
			workspaceDomain.ProjectQueued,
			workspaceDomain.ProjectStarting,
			workspaceDomain.ProjectCheckingHealth,
			workspaceDomain.ProjectStopping,
			workspaceDomain.ProjectRollingBack,
		}},
		{name: "completed", want: "succeeded", in: []workspaceDomain.ProjectStatus{
			workspaceDomain.ProjectRunning,
			workspaceDomain.ProjectStopped,
			workspaceDomain.ProjectRolledBack,
		}},
		{name: "failed", want: "failed", in: []workspaceDomain.ProjectStatus{
			workspaceDomain.ProjectBlocked,
			workspaceDomain.ProjectStartFailed,
			workspaceDomain.ProjectStopFailed,
			workspaceDomain.ProjectRollbackFailed,
		}},
		{name: "cancelled", want: "cancelled", in: []workspaceDomain.ProjectStatus{
			workspaceDomain.ProjectCancelled,
		}},
	}

	for _, test := range tests {
		for _, status := range test.in {
			status := status
			t.Run(test.name+"/"+string(status), func(t *testing.T) {
				t.Parallel()
				recorder := &recordingWorkspaceProgress{}
				reporter := workspaceProgress{progress: recorder}
				err := reporter.ProjectProgress(context.Background(), workspaceDomain.ProjectResult{
					ProjectID: "project-1",
					Status:    status,
					Message:   "status changed",
				})
				if err != nil {
					t.Fatalf("ProjectProgress() error = %v", err)
				}
				if recorder.calls != 1 || recorder.name != "workspace.project-1" || recorder.state != test.want || recorder.message != "status changed" {
					t.Fatalf("recorded progress = %#v", recorder)
				}
			})
		}
	}
}

func TestWorkspaceProgressRejectsUnknownDomainStatus(t *testing.T) {
	t.Parallel()

	recorder := &recordingWorkspaceProgress{}
	reporter := workspaceProgress{progress: recorder}
	err := reporter.ProjectProgress(context.Background(), workspaceDomain.ProjectResult{
		ProjectID: "project-1",
		Status:    workspaceDomain.ProjectStatus("unexpected"),
	})
	if err == nil {
		t.Fatal("ProjectProgress() error = nil")
	}
	if recorder.calls != 0 {
		t.Fatalf("progress calls = %d, want 0", recorder.calls)
	}
}
