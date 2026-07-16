package application_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"switchyard.dev/switchyard/internal/platform/sqlite"
	plugins "switchyard.dev/switchyard/internal/plugins/application"
	"switchyard.dev/switchyard/internal/plugins/domain"
	pluginsdk "switchyard.dev/switchyard/sdk/plugin"
)

type discoveryStub struct{ values []domain.Plugin }

func (d *discoveryStub) Discover(context.Context) ([]domain.Plugin, error) { return d.values, nil }

type projectSourceStub struct{ trusted bool }

func (p projectSourceStub) Project(context.Context, string) (plugins.Project, error) {
	return plugins.Project{ID: "project-1", DisplayName: "Fixture", Root: "/trusted/fixture", Trusted: p.trusted}, nil
}

type runnerStub struct {
	err  error
	last plugins.Invocation
}

func (r *runnerStub) Call(_ context.Context, invocation plugins.Invocation) ([]domain.LogEntry, error) {
	r.last = invocation
	if r.err != nil {
		return []domain.LogEntry{{Level: "warning", Message: "child stopped"}}, r.err
	}
	switch result := invocation.Result.(type) {
	case *pluginsdk.HealthResult:
		*result = pluginsdk.HealthResult{Status: "healthy", Message: "ready", Checked: time.Now().UTC()}
	case *pluginsdk.InspectResult:
		*result = pluginsdk.InspectResult{Summary: "inspected", Facts: []pluginsdk.Fact{}, Actions: []pluginsdk.Action{}, Warnings: []string{}, Observed: time.Now().UTC()}
	case *pluginsdk.OperateResult:
		*result = pluginsdk.OperateResult{Status: "succeeded", Summary: "done"}
	}
	return nil, nil
}

func TestServiceRequiresExactTrustAndEnforcesScopes(t *testing.T) {
	service, discovery, runner := newService(t, true)
	items, err := service.Refresh(context.Background())
	if err != nil || len(items) != 1 || items[0].Trust != domain.TrustUntrusted {
		t.Fatalf("Refresh() = %#v, %v", items, err)
	}
	if _, err := service.Enable(context.Background(), "fixture", []string{string(pluginsdk.ScopeProjectMetadataRead)}); !errors.Is(err, plugins.ErrTrustRequired) {
		t.Fatalf("Enable() error = %v, want trust required", err)
	}
	if _, err := service.Trust(context.Background(), "fixture", "wrong"); !errors.Is(err, plugins.ErrFingerprint) {
		t.Fatalf("Trust(wrong) error = %v", err)
	}
	if _, err := service.Trust(context.Background(), "fixture", "fingerprint-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Enable(context.Background(), "fixture", []string{"undeclared"}); !errors.Is(err, plugins.ErrPermissionDenied) {
		t.Fatalf("Enable(undeclared) error = %v", err)
	}
	if _, err := service.Enable(context.Background(), "fixture", []string{string(pluginsdk.ScopeProjectMetadataRead)}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Inspect(context.Background(), "fixture", "project-1"); err != nil {
		t.Fatal(err)
	}
	request := runner.last.Params.(pluginsdk.InspectRequest)
	if request.Project.Root != "" {
		t.Fatalf("project root leaked without project.files.read: %q", request.Project.Root)
	}
	if _, err := service.Operate(context.Background(), "fixture", "project-1", "fixture.echo", nil); !errors.Is(err, plugins.ErrPermissionDenied) {
		t.Fatalf("Operate() error = %v, want permission denial", err)
	}

	discovery.values[0].Fingerprint = "fingerprint-2"
	changed, err := service.Refresh(context.Background())
	if err != nil || changed[0].Enabled || changed[0].Trust != domain.TrustChanged {
		t.Fatalf("changed plugin = %#v, %v", changed[0], err)
	}
}

func TestServiceContainsPluginCrashAndRecordsHealth(t *testing.T) {
	service, _, runner := newService(t, true)
	_, _ = service.Refresh(context.Background())
	_, _ = service.Trust(context.Background(), "fixture", "fingerprint-1")
	_, _ = service.Enable(context.Background(), "fixture", []string{string(pluginsdk.ScopeProjectMetadataRead)})
	runner.err = errors.New("process exited 17")
	item, err := service.Health(context.Background(), "fixture")
	if !errors.Is(err, plugins.ErrInvocation) || item.Health != domain.HealthUnhealthy {
		t.Fatalf("Health() = %#v, %v", item, err)
	}
	logs, err := service.Logs(context.Background(), "fixture", 100)
	if err != nil || len(logs) < 2 {
		t.Fatalf("Logs() = %#v, %v", logs, err)
	}
}

func TestServiceRejectsUntrustedProject(t *testing.T) {
	service, _, _ := newService(t, false)
	_, _ = service.Refresh(context.Background())
	_, _ = service.Trust(context.Background(), "fixture", "fingerprint-1")
	_, _ = service.Enable(context.Background(), "fixture", []string{string(pluginsdk.ScopeProjectMetadataRead)})
	if _, err := service.Inspect(context.Background(), "fixture", "project-1"); err == nil {
		t.Fatal("expected untrusted project rejection")
	}
}

func newService(t *testing.T, projectTrusted bool) (*plugins.Service, *discoveryStub, *runnerStub) {
	t.Helper()
	database, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "switchyard.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	now := time.Now().UTC()
	discovery := &discoveryStub{values: []domain.Plugin{{
		ID: "fixture", Name: "Fixture", Version: "1.0.0", ProtocolVersion: pluginsdk.ProtocolVersion,
		ManifestPath: "/plugins/fixture/plugin.json", Executable: "/plugins/fixture/bin", Fingerprint: "fingerprint-1",
		Capabilities:    []string{string(pluginsdk.CapabilityProjectInspect), string(pluginsdk.CapabilityProjectOperate)},
		RequestedScopes: []string{string(pluginsdk.ScopeProjectMetadataRead), string(pluginsdk.ScopeProjectFilesRead), string(pluginsdk.ScopeProjectOperate)},
		Available:       true, Trust: domain.TrustUntrusted, Health: domain.HealthUnknown, DiscoveredAt: now, UpdatedAt: now,
	}}}
	runner := &runnerStub{}
	service, err := plugins.NewService(sqlite.NewPluginRepository(database), discovery, runner, projectSourceStub{trusted: projectTrusted})
	if err != nil {
		t.Fatal(err)
	}
	return service, discovery, runner
}
