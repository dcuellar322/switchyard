package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"switchyard.dev/switchyard/internal/telemetry/domain"
)

func TestTelemetryRepositoryDefaultsDisabledAndClearsOnOptOut(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database, err := Open(ctx, filepath.Join(t.TempDir(), "switchyard.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	repository := NewTelemetryRepository(database)
	status, err := repository.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if status.Settings.Enabled || status.Settings.Endpoint != "" || len(status.Counters) != 0 {
		t.Fatalf("default status = %#v", status)
	}
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	settings := domain.Settings{Enabled: true, Endpoint: "https://metrics.example.test/v1", InstallationID: "anonymous-test", UpdatedAt: now}
	event := domain.AuditEvent{Type: "telemetry.configured", ActorType: "user", ActorID: "test", OccurredAt: now}
	if err := repository.Configure(ctx, settings, false, event); err != nil {
		t.Fatal(err)
	}
	if err := repository.Increment(ctx, "remote.operation", now); err != nil {
		t.Fatal(err)
	}
	status, err = repository.Status(ctx)
	if err != nil || len(status.Counters) != 1 || status.Counters[0].Value != 1 {
		t.Fatalf("enabled status=%#v error=%v", status, err)
	}
	settings = domain.Settings{UpdatedAt: now.Add(time.Minute)}
	if err := repository.Configure(ctx, settings, true, event); err != nil {
		t.Fatal(err)
	}
	status, err = repository.Status(ctx)
	if err != nil || status.Settings.Enabled || status.Settings.InstallationID != "" || len(status.Counters) != 0 {
		t.Fatalf("disabled status=%#v error=%v", status, err)
	}
}
