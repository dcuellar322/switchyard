package process

import (
	"context"
	"testing"
	"time"

	"switchyard.dev/switchyard/internal/runtime/domain"
)

func TestInspectRejectsReusedPIDFingerprint(t *testing.T) {
	t.Parallel()
	store := newMemoryRunStore()
	started := time.Now().Add(-time.Minute).UTC()
	stored := domain.ProcessIdentity{
		RunID: "run-stale", PID: 42, ProcessGroup: 42, Executable: "/usr/bin/uv",
		WorkingDirectory: "/repo", StartedAt: started, Fingerprint: "original", ObservedAt: started,
	}
	if err := store.CreateRun(context.Background(), domain.RunRecord{
		ID: "run-stale", ProjectID: "project-process", ServiceID: "api", RuntimeDriver: domain.KindProcess,
		Origin: domain.OriginSwitchyard, StartedAt: started, IdentityFingerprint: stored.Fingerprint,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordProcess(context.Background(), stored); err != nil {
		t.Fatal(err)
	}
	inspector := inspectorFake{snapshots: map[int32]domain.ProcessIdentity{42: {
		PID: 42, ProcessGroup: 42, Executable: "/usr/bin/uv", WorkingDirectory: "/repo",
		StartedAt: started.Add(30 * time.Second), Fingerprint: "reused",
	}}}
	driver := newDriver(context.Background(), store, inspector, &secretResolverFake{})
	project := processProject("/repo", []servicePlan{{
		service:    domain.ServiceDeclaration{ID: "api", RuntimeName: "api"},
		definition: domain.ProcessDefinition{ID: "api", Command: []string{"uv", "run", "api.py"}},
	}})
	observation, err := driver.Inspect(context.Background(), project)
	if err != nil {
		t.Fatal(err)
	}
	if observation.State != domain.StateUnknown || observation.Services[0].State != "stale" {
		t.Fatalf("observation = %#v", observation)
	}
	runs, _ := store.ListProjectRuns(context.Background(), project.ProjectID)
	if runs[0].EndedAt == nil || runs[0].TerminationReason != "identity_lost" {
		t.Fatalf("run = %#v", runs[0])
	}
}

func TestInspectLabelsMatchingExternalListenerHonestly(t *testing.T) {
	t.Parallel()
	identity := domain.ProcessIdentity{
		PID: 99, ProcessGroup: 99, Executable: "/opt/homebrew/bin/uv", WorkingDirectory: "/repo",
		StartedAt: time.Now().UTC(), Fingerprint: "external",
	}
	inspector := inspectorFake{listeners: map[int][]domain.ProcessIdentity{18082: {identity}}}
	driver := newDriver(context.Background(), newMemoryRunStore(), inspector, &secretResolverFake{})
	project := processProject("/repo", []servicePlan{{
		service:    domain.ServiceDeclaration{ID: "api", RuntimeName: "api", HostPorts: []int{18082}},
		definition: domain.ProcessDefinition{ID: "api", Command: []string{"uv", "run", "api.py"}},
	}})
	observation, err := driver.Inspect(context.Background(), project)
	if err != nil {
		t.Fatal(err)
	}
	if observation.State != domain.StateRunningExternal || observation.Origin != domain.OriginExternal || observation.Services[0].Process.RunID != "" {
		t.Fatalf("observation = %#v", observation)
	}
}
