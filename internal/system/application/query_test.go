package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"switchyard.dev/switchyard/internal/foundation/buildinfo"
)

type healthStub struct {
	version int64
	err     error
}

func (s healthStub) SchemaVersion(context.Context) (int64, error) {
	return s.version, s.err
}

func TestQueryGet(t *testing.T) {
	t.Parallel()

	startedAt := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	query := NewQuery(healthStub{version: 1}, buildinfo.Info{Version: "0.1.0", Commit: "abc"}, startedAt)

	got, err := query.Get(context.Background())
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Status != "ready" || got.DatabaseSchemaVersion != 1 || got.StartedAt != startedAt {
		t.Fatalf("Get() = %#v", got)
	}
}

func TestQueryGetWrapsStorageFailure(t *testing.T) {
	t.Parallel()

	query := NewQuery(healthStub{err: errors.New("unavailable")}, buildinfo.Info{}, time.Now())
	if _, err := query.Get(context.Background()); err == nil {
		t.Fatal("Get() error = nil, want failure")
	}
}
