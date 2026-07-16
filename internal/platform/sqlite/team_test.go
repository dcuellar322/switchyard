package sqlite

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"switchyard.dev/switchyard/internal/team/domain"
)

func TestTeamRepositoryRoundTripAndAtomicSync(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database, err := Open(ctx, filepath.Join(t.TempDir(), "switchyard.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	repository := NewTeamRepository(database)
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	publisher := domain.Publisher{ID: "publisher-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Name: "Maintainers", PublicKey: "cHVibGlj", TrustedAt: now}
	if err := repository.TrustPublisher(ctx, publisher); err != nil {
		t.Fatal(err)
	}
	bundle := domain.Bundle{
		SchemaVersion: domain.BundleSchemaVersion, Kind: domain.KindPolicyPack,
		Metadata:  domain.BundleMetadata{ID: "policy.base", Name: "Base", Version: "1.0.0", PublisherID: publisher.ID, CreatedAt: now},
		Payload:   json.RawMessage(`{"allowedRemoteCapabilities":[],"allowedRemoteActions":[],"allowedPluginPublishers":[],"telemetryAllowed":false}`),
		Signature: domain.Signature{KeyID: publisher.ID, Algorithm: domain.SignatureAlgorithm, Value: "signature"}, InstalledAt: &now,
	}
	if err := repository.InstallBundle(ctx, bundle); err != nil {
		t.Fatal(err)
	}
	stored, err := repository.GetBundle(ctx, bundle.Metadata.ID)
	if err != nil || stored.Metadata.PublisherID != publisher.ID || stored.Kind != domain.KindPolicyPack {
		t.Fatalf("stored bundle=%#v error=%v", stored, err)
	}
	if err := repository.ApplySync(ctx, domain.SyncDocument{Publishers: []domain.Publisher{publisher}, Bundles: []domain.Bundle{bundle}}); err != nil {
		t.Fatal(err)
	}
	if err := repository.RecordAudit(ctx, domain.AuditEvent{Type: "sync.imported", ActorType: "user", ActorID: "test", SubjectID: "sync", OccurredAt: now}); err != nil {
		t.Fatal(err)
	}
	var auditCount int
	if err := database.connection.QueryRowContext(ctx, `SELECT COUNT(*) FROM team_audit_events`).Scan(&auditCount); err != nil || auditCount != 1 {
		t.Fatalf("audit count=%d error=%v", auditCount, err)
	}
}
