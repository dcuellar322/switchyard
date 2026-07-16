package application

import (
	"bytes"
	"testing"

	"switchyard.dev/switchyard/internal/manifest/domain"
)

func TestParseYAMLNormalizesLegacySchemaVersion(t *testing.T) {
	t.Parallel()
	contents := []byte("schemaVersion: switchyard.dev/v1alpha1\nkind: Project\nmetadata:\n  id: fixture\n  name: Fixture\nrepository:\n  root: .\n")
	manifest, err := ParseYAML(contents)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.SchemaVersion != domain.SchemaVersion {
		t.Fatalf("schema version = %q", manifest.SchemaVersion)
	}
}

func TestMigrateYAMLPreservesCommentsAndIsIdempotent(t *testing.T) {
	t.Parallel()
	contents := []byte("# project identity\nschemaVersion: switchyard.dev/v1alpha1\nkind: Project\nmetadata:\n  id: fixture\n  name: Fixture\nrepository:\n  root: .\n")
	migrated, changed, err := MigrateYAML(contents)
	if err != nil {
		t.Fatal(err)
	}
	if !changed || !bytes.Contains(migrated, []byte("# project identity")) || !bytes.Contains(migrated, []byte("switchyard.dev/v1\n")) {
		t.Fatalf("migrated = %s", migrated)
	}
	again, changed, err := MigrateYAML(migrated)
	if err != nil || changed || !bytes.Equal(again, migrated) {
		t.Fatalf("second migration changed=%v error=%v\n%s", changed, err, again)
	}
}
