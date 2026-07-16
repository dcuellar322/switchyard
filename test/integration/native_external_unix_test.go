//go:build integration && (darwin || linux)

package integration_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"

	"switchyard.dev/switchyard/internal/platform/sqlite"
	"switchyard.dev/switchyard/internal/runtime/domain"
	processRuntime "switchyard.dev/switchyard/internal/runtime/process"
)

func TestNativeRuntimeRecognizesExternalUVListenerWithoutOwnership(t *testing.T) {
	port := availablePort(t)
	root, err := filepath.Abs(filepath.Join("..", "fixtures", "uv-single-process"))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	command := exec.Command("uv", "run", "--offline", "--no-project", "python", "server.py")
	command.Dir = root
	command.Env = append(os.Environ(), "PORT="+strconv.Itoa(port))
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		_, _ = command.Process.Wait()
	})
	waitForListeningPort(t, ctx, port)
	database, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "switchyard.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	driver := processRuntime.NewDriver(ctx, sqlite.NewRunRepository(database))
	project := domain.ProjectRuntime{
		ProjectID: "external-uv", ProjectSlug: "external-uv", Root: root, Kind: domain.KindProcess,
		Process: &domain.ProcessRuntime{Processes: []domain.ProcessDefinition{{
			ID: "web", Command: []string{"uv", "run", "--offline", "--no-project", "python", "server.py"},
			WorkingDirectory: ".",
		}}},
		Services: []domain.ServiceDeclaration{{ID: "web", RuntimeName: "web", HostPorts: []int{port}}},
	}
	observation, err := driver.Inspect(ctx, project)
	if err != nil {
		t.Fatal(err)
	}
	if observation.State != domain.StateRunningExternal || observation.Origin != domain.OriginExternal ||
		observation.Services[0].Process == nil || observation.Services[0].Process.RunID != "" {
		t.Fatalf("external observation = %#v", observation)
	}
}
