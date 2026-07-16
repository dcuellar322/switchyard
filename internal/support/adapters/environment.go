package adapters

import (
	"context"
	"os/exec"
	"runtime"

	"switchyard.dev/switchyard/internal/support/domain"
)

// EnvironmentProbe checks adapter executables without running them.
type EnvironmentProbe struct{}

// Probe returns stable capability identifiers and no filesystem paths.
func (EnvironmentProbe) Probe(ctx context.Context) ([]domain.AdapterAvailability, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	result := []domain.AdapterAvailability{{
		ID: "platform:" + runtime.GOOS + "/" + runtime.GOARCH, Available: true, Detail: "native build target",
	}}
	for _, candidate := range []struct{ id, executable string }{
		{id: "docker-compose", executable: "docker"},
		{id: "git", executable: "git"},
		{id: "node", executable: "node"},
		{id: "pnpm", executable: "pnpm"},
		{id: "python", executable: "python3"},
		{id: "rust", executable: "cargo"},
		{id: "ssh", executable: "ssh"},
	} {
		_, err := exec.LookPath(candidate.executable)
		availability := domain.AdapterAvailability{ID: candidate.id, Available: err == nil, Detail: "not found on PATH"}
		if err == nil {
			availability.Detail = "available on PATH"
		}
		result = append(result, availability)
	}
	return result, nil
}
