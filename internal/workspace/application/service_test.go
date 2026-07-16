package application

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"switchyard.dev/switchyard/internal/workspace/domain"
)

func TestServiceCreateValidatesMembersAndDefaultsFailurePolicy(t *testing.T) {
	t.Parallel()

	repository := newFakeRepository()
	validator := &recordingMemberValidator{}
	service := NewService(repository, &fakeProjectOperator{}, noHealthGate{}, validator)
	service.now = func() time.Time { return time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC) }

	created, err := service.Create(context.Background(), SaveRequest{
		Name: "  Product stack  ",
		Members: []domain.Member{
			{ProjectID: "api", Role: domain.MemberRoleApplication},
			{ProjectID: "database", Role: domain.MemberRoleDependency, Order: 1},
		},
		Dependencies: []domain.Dependency{{ProjectID: "api", DependsOnProjectID: "database"}},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.ID == "" || created.Name != "Product stack" || created.Revision != 1 ||
		created.DefaultFailurePolicy != domain.FailurePolicyRollback {
		t.Fatalf("created workspace = %#v", created)
	}
	if got := validator.projectsCopy(); !slices.Equal(got, []string{"api", "database"}) {
		t.Fatalf("validated members = %#v", got)
	}
}

func TestServiceCreateRejectsIneligibleMember(t *testing.T) {
	t.Parallel()

	repository := newFakeRepository()
	validator := &recordingMemberValidator{failure: errors.New("project is not trusted")}
	service := NewService(repository, &fakeProjectOperator{}, noHealthGate{}, validator)
	_, err := service.Create(context.Background(), SaveRequest{
		Name: "Product", Members: []domain.Member{{ProjectID: "pending", Role: domain.MemberRoleApplication}},
	})
	if err == nil || len(repository.workspaces) != 0 {
		t.Fatalf("Create() error = %v, repository = %#v", err, repository.workspaces)
	}
}

func TestServiceUpdatePreservesLatestRunAndIncrementsRevision(t *testing.T) {
	t.Parallel()

	workspace := linearWorkspace()
	workspace.LastRun = &domain.ExecutionSummary{ID: "run-1", WorkspaceID: workspace.ID, State: domain.ExecutionSucceeded}
	repository := newFakeRepository(workspace)
	service := NewService(repository, &fakeProjectOperator{}, noHealthGate{}, allowMembers{})
	updated, err := service.Update(context.Background(), workspace.ID, SaveRequest{
		Name: "Renamed", DefaultFailurePolicy: domain.FailurePolicyContinue, Revision: workspace.Revision,
		Members: workspace.Members, Dependencies: workspace.Dependencies,
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.Revision != workspace.Revision+1 || updated.LastRun == nil || updated.LastRun.ID != "run-1" {
		t.Fatalf("updated workspace = %#v", updated)
	}
}

type recordingMemberValidator struct {
	projects []string
	failure  error
}

func (v *recordingMemberValidator) ValidateWorkspaceMember(_ context.Context, projectID string) error {
	v.projects = append(v.projects, projectID)
	return v.failure
}

func (v *recordingMemberValidator) projectsCopy() []string {
	return slices.Clone(v.projects)
}
