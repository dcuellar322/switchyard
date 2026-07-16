package sqlite

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"switchyard.dev/switchyard/internal/fleet/domain"
)

func TestFleetRepositoryPersistsRedactedIdentityAndRetainsAudit(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database, err := Open(ctx, filepath.Join(t.TempDir(), "switchyard.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	repository := NewFleetRepository(database)
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	machine := domain.Machine{
		ID: "machine-1", Name: "Build box", Endpoint: "https://127.0.0.1:19618", CertificateFingerprint: strings.Repeat("a", 64),
		Credentials: domain.CredentialReferences{CACertificate: "/private/ca.pem", ClientCertificate: "/private/client.pem", ClientKey: "/private/client-key.pem"},
		Enabled:     true, GrantedCapabilities: []domain.Capability{domain.CapabilityInventoryRead}, State: domain.MachinePending, CreatedAt: now, UpdatedAt: now,
	}
	if err := repository.Create(ctx, machine); err != nil {
		t.Fatal(err)
	}
	snapshot := domain.Snapshot{
		Identity: domain.Identity{ProtocolVersion: domain.ProtocolVersion, MachineID: "peer-1", Name: "Build box", Version: "1.0.0", OS: "linux", Architecture: "amd64", Capabilities: []domain.Capability{domain.CapabilityInventoryRead}},
		Projects: []domain.Project{{ID: "project-1", Slug: "example", DisplayName: "Example", Runtime: "compose", State: "running", Health: "healthy"}}, ObservedAt: now,
	}
	if err := repository.RecordObservation(ctx, machine.ID, snapshot, domain.MachineOnline, "", now); err != nil {
		t.Fatal(err)
	}
	stored, err := repository.Get(ctx, machine.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.PeerID != "peer-1" || stored.State != domain.MachineOnline || stored.LastSeenAt == nil || !stored.CredentialConfigured {
		t.Fatalf("stored machine = %#v", stored)
	}
	encoded, err := json.Marshal(stored)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"/private/ca.pem", "/private/client.pem", "/private/client-key.pem"} {
		if strings.Contains(string(encoded), secret) {
			t.Fatalf("credential reference leaked in JSON: %s", encoded)
		}
	}
	if err := repository.RecordAudit(ctx, domain.AuditEvent{MachineID: machine.ID, Type: "machine.removed", ActorType: "user", ActorID: "fixture", OccurredAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := repository.Delete(ctx, machine.ID); err != nil {
		t.Fatal(err)
	}
	var auditCount, snapshotCount int
	if err := database.connection.QueryRowContext(ctx, `SELECT COUNT(*) FROM fleet_audit_events WHERE machine_id = ?`, machine.ID).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if err := database.connection.QueryRowContext(ctx, `SELECT COUNT(*) FROM fleet_snapshots WHERE machine_id = ?`, machine.ID).Scan(&snapshotCount); err != nil {
		t.Fatal(err)
	}
	if auditCount != 1 || snapshotCount != 0 {
		t.Fatalf("audit=%d snapshot=%d", auditCount, snapshotCount)
	}
}
