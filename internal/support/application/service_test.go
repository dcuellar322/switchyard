package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"switchyard.dev/switchyard/internal/support/domain"
)

type probeStub struct{ values []domain.AdapterAvailability }

func (p probeStub) Probe(context.Context) ([]domain.AdapterAvailability, error) { return p.values, nil }

type logsStub struct{ values []domain.InternalLogEntry }

func (l logsStub) List(context.Context, LogQuery) ([]domain.InternalLogEntry, error) {
	return l.values, nil
}

func TestPreviewIsExplicitlyRedactedAndBounded(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	service, err := NewService(probeStub{values: []domain.AdapterAvailability{{ID: "git", Available: true}}}, logsStub{}, func() time.Time { return now })
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	preview, err := service.Preview(context.Background(), PreviewInput{System: domain.SystemIdentity{Version: "1.0.0", DatabaseSchema: 16}})
	if err != nil {
		t.Fatalf("Preview() error = %v", err)
	}
	if preview.SchemaVersion != domain.PreviewSchema || len(preview.Excluded) < 4 || preview.GeneratedAt != now {
		t.Fatalf("preview = %#v", preview)
	}
}

func TestLogsRejectsUnboundedOrUnknownQueries(t *testing.T) {
	t.Parallel()

	service, _ := NewService(probeStub{}, logsStub{}, nil)
	for _, query := range []LogQuery{{Limit: 0}, {Limit: 2_001}, {Limit: 10, MinimumLevel: "TRACE"}} {
		if _, err := service.Logs(context.Background(), query); !errors.Is(err, ErrInvalidQuery) {
			t.Errorf("Logs(%#v) error = %v", query, err)
		}
	}
}
