package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"switchyard.dev/switchyard/internal/telemetry/domain"
)

func TestTelemetryRequiresConsentAndEmitsOnlyFixedCounters(t *testing.T) {
	t.Parallel()
	repository := &telemetryRepositoryStub{}
	sender := &telemetrySenderStub{}
	service, err := NewService(repository, sender, Build{Version: "1.0.0"}, telemetryPolicyStub{allowed: true})
	if err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC) }
	service.Record(context.Background(), "daemon.started")
	if len(repository.status.Counters) != 0 {
		t.Fatal("disabled telemetry recorded a counter")
	}
	if _, err := service.Configure(context.Background(), true, "https://metrics.example.test/v1", false, Actor{}); !errors.Is(err, ErrConfirmation) {
		t.Fatalf("Configure() error = %v", err)
	}
	status, err := service.Configure(context.Background(), true, "https://metrics.example.test/v1", true, Actor{Type: "user", ID: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if !status.Settings.Enabled || status.Settings.InstallationID == "" || status.Preview == nil {
		t.Fatalf("enabled status = %#v", status)
	}
	service.ObserveOperation(context.Background(), "remote.project.start")
	service.ObserveOperation(context.Background(), "contains.project-secret-id")
	service.Record(context.Background(), "arbitrary.project.id")
	status, err = service.Send(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if sender.calls != 1 || len(sender.payloads[0].Counters) != 1 || sender.payloads[0].Counters[0].Name != "remote.operation" {
		t.Fatalf("sent payload = %#v", sender.payloads)
	}
	if status.LastSentAt == nil {
		t.Fatal("successful delivery was not recorded")
	}
	status, err = service.Configure(context.Background(), false, "", false, Actor{})
	if err != nil {
		t.Fatal(err)
	}
	if status.Settings.Enabled || status.Settings.InstallationID != "" || len(status.Counters) != 0 {
		t.Fatalf("disabled status = %#v", status)
	}
}

func TestTelemetryPolicyCanDenyOptIn(t *testing.T) {
	t.Parallel()
	service, err := NewService(&telemetryRepositoryStub{}, &telemetrySenderStub{}, Build{Version: "1.0.0"}, telemetryPolicyStub{allowed: false})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Configure(context.Background(), true, "https://metrics.example.test", true, Actor{}); !errors.Is(err, ErrPolicyDenied) {
		t.Fatalf("Configure() error = %v", err)
	}
}

type telemetryRepositoryStub struct{ status domain.Status }

func (r *telemetryRepositoryStub) Status(context.Context) (domain.Status, error) {
	return r.status, nil
}
func (r *telemetryRepositoryStub) Configure(_ context.Context, settings domain.Settings, clearCounters bool, _ domain.AuditEvent) error {
	r.status.Settings = settings
	if clearCounters {
		r.status.Counters = nil
		r.status.LastSentAt = nil
		r.status.LastError = ""
	}
	return nil
}
func (r *telemetryRepositoryStub) Increment(_ context.Context, name string, _ time.Time) error {
	for index := range r.status.Counters {
		if r.status.Counters[index].Name == name {
			r.status.Counters[index].Value++
			return nil
		}
	}
	r.status.Counters = append(r.status.Counters, domain.Counter{Name: name, Value: 1})
	return nil
}
func (r *telemetryRepositoryStub) RecordDelivery(_ context.Context, success bool, message string, now time.Time) error {
	if success {
		r.status.LastSentAt = &now
	}
	r.status.LastError = message
	return nil
}

type telemetrySenderStub struct {
	calls    int
	payloads []domain.Payload
}

func (s *telemetrySenderStub) Send(_ context.Context, _ string, payload domain.Payload) error {
	s.calls++
	s.payloads = append(s.payloads, payload)
	return nil
}

type telemetryPolicyStub struct{ allowed bool }

func (p telemetryPolicyStub) EffectiveTelemetryAllowed(context.Context) (bool, error) {
	return p.allowed, nil
}
