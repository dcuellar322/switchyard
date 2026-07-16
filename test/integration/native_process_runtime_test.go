//go:build integration

package integration_test

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	gprocess "github.com/shirou/gopsutil/v4/process"
	catalog "switchyard.dev/switchyard/internal/catalog/application"
	discoveryAdapters "switchyard.dev/switchyard/internal/discovery/adapters"
	"switchyard.dev/switchyard/internal/platform/sqlite"
	runtimeApplication "switchyard.dev/switchyard/internal/runtime/application"
	"switchyard.dev/switchyard/internal/runtime/domain"
	processRuntime "switchyard.dev/switchyard/internal/runtime/process"
)

func TestUVAndNPMFixturesLifecycleWithoutOrphanedChildren(t *testing.T) {
	for _, fixture := range []struct {
		name     string
		oldPort  int
		logToken string
	}{
		{name: "uv-single-process", oldPort: 19861, logToken: "uv fixture listening"},
		{name: "node-single-process", oldPort: 19862, logToken: "npm fixture listening"},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			port := availablePort(t)
			root := copyProcessFixture(t, fixture.name, fixture.oldPort, port)
			ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
			defer cancel()
			database, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "switchyard.db"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = database.Close() })
			catalogService := catalog.NewService(sqlite.NewCatalogRepository(database), discoveryAdapters.Defaults())
			project, proposal, err := catalogService.Scan(ctx, root)
			if err != nil {
				t.Fatal(err)
			}
			if _, _, err := catalogService.Accept(ctx, proposal.ID); err != nil {
				t.Fatal(err)
			}
			runRepository := sqlite.NewRunRepository(database)
			driver := processRuntime.NewDriver(ctx, runRepository)
			runtimeService := runtimeApplication.NewService(runtimeApplication.NewCatalogSource(catalogService), driver)
			executeNativeApplicationAction(t, ctx, runtimeService, project.ID, domain.ActionStart)
			waitForListeningPort(t, ctx, port)
			observation := waitNativeApplicationState(t, ctx, runtimeService, project.ID, domain.StateRunning)
			if observation.Services[0].Process == nil || observation.Services[0].Process.RunID == "" {
				t.Fatalf("observation = %#v", observation)
			}
			waitForRuntimeLog(t, ctx, runtimeService, project.ID, fixture.logToken)
			metrics, err := runtimeService.Metrics(ctx, project.ID, "web")
			if err != nil || len(metrics) != 1 || metrics[0].MemoryBytes == 0 {
				t.Fatalf("metrics = %#v, %v", metrics, err)
			}
			executeNativeApplicationAction(t, ctx, runtimeService, project.ID, domain.ActionStop)
			waitNativeApplicationState(t, ctx, runtimeService, project.ID, domain.StateStopped)
			runs, err := runRepository.ListProjectRuns(ctx, project.ID)
			if err != nil || len(runs) != 1 || runs[0].EndedAt == nil {
				t.Fatalf("runs = %#v, %v", runs, err)
			}
			for _, identity := range runs[0].Processes {
				waitForPIDExit(t, ctx, identity.PID)
			}
		})
	}
}

func executeNativeApplicationAction(
	t *testing.T,
	ctx context.Context,
	service *runtimeApplication.Service,
	projectID string,
	action domain.Action,
) {
	t.Helper()
	plan, err := service.Plan(ctx, projectID, action, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Execute(ctx, plan, progressSink{}); err != nil {
		t.Fatal(err)
	}
}

func waitNativeApplicationState(
	t *testing.T,
	ctx context.Context,
	service *runtimeApplication.Service,
	projectID string,
	wanted domain.ProjectState,
) domain.Observation {
	t.Helper()
	for {
		observation, err := service.Inspect(ctx, projectID)
		if err != nil {
			t.Fatal(err)
		}
		if observation.State == wanted {
			return observation
		}
		select {
		case <-ctx.Done():
			t.Fatalf("waiting for %s: %v; last observation: %#v", wanted, ctx.Err(), observation)
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func waitForRuntimeLog(
	t *testing.T,
	ctx context.Context,
	service *runtimeApplication.Service,
	projectID, token string,
) {
	t.Helper()
	for {
		entries, err := service.Logs(ctx, projectID, "web", "", 100)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			if strings.Contains(entry.Message, token) && entry.Source == "process" && entry.RunID != "" {
				return
			}
		}
		select {
		case <-ctx.Done():
			t.Fatalf("waiting for log %q: %v", token, ctx.Err())
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func copyProcessFixture(t *testing.T, name string, oldPort, port int) string {
	t.Helper()
	source, err := filepath.Abs(filepath.Join("..", "fixtures", name))
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), name)
	if err := filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(target, relative)
		if entry.IsDir() {
			return os.MkdirAll(destination, 0o755)
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if filepath.Base(path) == "project.yml" {
			contents = []byte(strings.ReplaceAll(string(contents), strconv.Itoa(oldPort), strconv.Itoa(port)))
		}
		return os.WriteFile(destination, contents, 0o600)
	}); err != nil {
		t.Fatal(err)
	}
	return target
}

func availablePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	return listener.Addr().(*net.TCPAddr).Port
}

func waitForListeningPort(t *testing.T, ctx context.Context, port int) {
	t.Helper()
	address := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	for {
		connection, err := net.DialTimeout("tcp", address, 100*time.Millisecond)
		if err == nil {
			_ = connection.Close()
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("waiting for %s: %v", address, ctx.Err())
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func waitForPIDExit(t *testing.T, ctx context.Context, pid int32) {
	t.Helper()
	for {
		exists, err := gprocess.PidExistsWithContext(ctx, pid)
		if err == nil && !exists {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("PID %d remained after stop: %v", pid, ctx.Err())
		case <-time.After(25 * time.Millisecond):
		}
	}
}
