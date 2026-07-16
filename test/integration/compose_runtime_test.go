//go:build integration

package integration_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"switchyard.dev/switchyard/internal/runtime/compose"
	"switchyard.dev/switchyard/internal/runtime/domain"
)

func TestComposeRuntimeLifecycleObservationLogsMetricsAndExternalRecognition(t *testing.T) {
	if os.Getenv("SWITCHYARD_DOCKER_INTEGRATION") != "1" {
		t.Skip("set SWITCHYARD_DOCKER_INTEGRATION=1 to run Docker integration tests")
	}
	root, err := filepath.Abs(filepath.Join("..", "fixtures", "compose-runtime"))
	if err != nil {
		t.Fatal(err)
	}
	prepareFixtureBinary(t, root)
	project := domain.ProjectRuntime{
		ProjectID: "integration-project", ProjectSlug: "compose-runtime", Root: root, Kind: domain.KindCompose,
		Compose:  &domain.ComposeRuntime{Files: []string{"compose.yaml"}, ProjectName: "switchyard-phase5-fixture"},
		Services: []domain.ServiceDeclaration{{ID: "web", RuntimeName: "web"}},
	}
	driver := compose.NewDriver()
	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if plan, planErr := driver.Plan(ctx, domain.PlanRequest{Project: project, Action: domain.ActionTeardown, RemoveVolumes: true}); planErr == nil {
			_ = driver.Execute(ctx, plan, progressSink{})
		}
		_ = exec.CommandContext(ctx, "docker", "image", "rm", "--force", "switchyard/compose-runtime-fixture:phase5").Run()
	}
	cleanup()
	t.Cleanup(cleanup)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	executeAction(t, ctx, driver, project, domain.ActionStart, false)
	waitForState(t, ctx, driver, project, domain.StateRunning)
	assertLogsAndMetrics(t, ctx, driver, project)

	executeAction(t, ctx, driver, project, domain.ActionPause, false)
	waitForState(t, ctx, driver, project, domain.StatePaused)
	executeAction(t, ctx, driver, project, domain.ActionUnpause, false)
	waitForState(t, ctx, driver, project, domain.StateRunning)
	executeAction(t, ctx, driver, project, domain.ActionRestart, false)
	waitForState(t, ctx, driver, project, domain.StateRunning)
	executeAction(t, ctx, driver, project, domain.ActionRebuild, false)
	waitForState(t, ctx, driver, project, domain.StateRunning)

	executeAction(t, ctx, driver, project, domain.ActionStop, false)
	waitForState(t, ctx, driver, project, domain.StateStopped)
	assertFixtureVolumeExists(t, ctx)
	executeAction(t, ctx, driver, project, domain.ActionTeardown, false)
	waitForState(t, ctx, driver, project, domain.StateStopped)
	assertFixtureVolumeExists(t, ctx)

	externalComposeUp(t, ctx, root)
	externalDriver := compose.NewDriver()
	observation := waitForState(t, ctx, externalDriver, project, domain.StateRunningExternal)
	if observation.State != domain.StateRunningExternal || observation.Origin != domain.OriginExternal {
		t.Fatalf("external observation = %#v", observation)
	}
	executeAction(t, ctx, driver, project, domain.ActionTeardown, true)
	if exec.CommandContext(ctx, "docker", "volume", "inspect", "switchyard-phase5-fixture_fixture-state").Run() == nil {
		t.Fatal("teardown --volumes preserved the fixture volume")
	}
}

func executeAction(t *testing.T, ctx context.Context, driver *compose.Driver, project domain.ProjectRuntime, action domain.Action, removeVolumes bool) {
	t.Helper()
	plan, err := driver.Plan(ctx, domain.PlanRequest{Project: project, Action: action, RemoveVolumes: removeVolumes})
	if err != nil {
		t.Fatal(err)
	}
	if err := driver.Execute(ctx, plan, progressSink{}); err != nil {
		t.Fatal(err)
	}
}

func waitForState(t *testing.T, ctx context.Context, driver *compose.Driver, project domain.ProjectRuntime, wanted domain.ProjectState) domain.Observation {
	t.Helper()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		observation, err := driver.Inspect(ctx, project)
		if err != nil {
			t.Fatal(err)
		}
		if observation.State == wanted {
			return observation
		}
		select {
		case <-ctx.Done():
			t.Fatalf("waiting for %s: %v; last observation: %#v", wanted, ctx.Err(), observation)
		case <-ticker.C:
		}
	}
}

func assertLogsAndMetrics(t *testing.T, ctx context.Context, driver *compose.Driver, project domain.ProjectRuntime) {
	t.Helper()
	logs := &logSink{}
	if err := driver.StreamLogs(ctx, domain.LogRequest{Project: project, Service: "web", Tail: 50}, logs); err != nil {
		t.Fatal(err)
	}
	if len(logs.entries) == 0 || logs.entries[0].ProjectID != project.ProjectID || logs.entries[0].ServiceID != "web" {
		t.Fatalf("logs = %#v", logs.entries)
	}
	metrics := &metricSink{}
	if err := driver.StreamMetrics(ctx, domain.MetricRequest{Project: project, Service: "web"}, metrics); err != nil {
		t.Fatal(err)
	}
	if len(metrics.samples) != 1 || metrics.samples[0].MemoryBytes == 0 {
		t.Fatalf("metrics = %#v", metrics.samples)
	}
}

func prepareFixtureBinary(t *testing.T, root string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	architectureOutput, err := exec.CommandContext(ctx, "docker", "version", "--format", "{{.Server.Arch}}").Output()
	if err != nil {
		t.Fatalf("read Docker Engine architecture: %v", err)
	}
	architecture := strings.TrimSpace(string(architectureOutput))
	switch architecture {
	case "x86_64":
		architecture = "amd64"
	case "aarch64":
		architecture = "arm64"
	}
	binary := filepath.Join(root, ".switchyard-fixture-server")
	command := exec.CommandContext(ctx, "go", "build", "-trimpath", "-o", binary, "server.go")
	command.Dir = root
	command.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS=linux", "GOARCH="+architecture)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build fixture server: %v: %s", err, output)
	}
	t.Cleanup(func() { _ = os.Remove(binary) })
}

func assertFixtureVolumeExists(t *testing.T, ctx context.Context) {
	t.Helper()
	if err := exec.CommandContext(ctx, "docker", "volume", "inspect", "switchyard-phase5-fixture_fixture-state").Run(); err != nil {
		t.Fatalf("fixture volume was not preserved: %v", err)
	}
}

func externalComposeUp(t *testing.T, ctx context.Context, root string) {
	t.Helper()
	command := exec.CommandContext(ctx, "docker", "compose", "--project-directory", root, "--file", filepath.Join(root, "compose.yaml"), "--project-name", "switchyard-phase5-fixture", "up", "--detach")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("external Compose up: %v: %s", err, output)
	}
}

type progressSink struct{}

func (progressSink) Step(context.Context, string, string, string) error { return nil }

type logSink struct{ entries []domain.LogEntry }

func (s *logSink) WriteLog(_ context.Context, entry domain.LogEntry) error {
	s.entries = append(s.entries, entry)
	return nil
}

type metricSink struct{ samples []domain.MetricSample }

func (s *metricSink) WriteMetric(_ context.Context, sample domain.MetricSample) error {
	s.samples = append(s.samples, sample)
	return nil
}
