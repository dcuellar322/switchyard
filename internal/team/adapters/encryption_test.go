package adapters

import (
	"testing"
	"time"

	"switchyard.dev/switchyard/internal/team/domain"
)

func TestEncryptedSyncRoundTripAndWrongIdentity(t *testing.T) {
	t.Parallel()
	identity, recipient, err := GenerateSyncIdentity()
	if err != nil {
		t.Fatal(err)
	}
	document := domain.SyncDocument{SchemaVersion: domain.SyncSchemaVersion, ExportedAt: time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)}
	encrypted, err := EncryptSync(document, []string{recipient})
	if err != nil {
		t.Fatal(err)
	}
	decrypted, err := DecryptSync(encrypted, identity)
	if err != nil {
		t.Fatal(err)
	}
	if decrypted.SchemaVersion != domain.SyncSchemaVersion || !decrypted.ExportedAt.Equal(document.ExportedAt) {
		t.Fatalf("decrypted document = %#v", decrypted)
	}
	wrongIdentity, _, err := GenerateSyncIdentity()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecryptSync(encrypted, wrongIdentity); err == nil {
		t.Fatal("DecryptSync() with wrong identity succeeded")
	}
}
