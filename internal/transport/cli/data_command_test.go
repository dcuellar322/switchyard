package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"switchyard.dev/switchyard/internal/bootstrap"
)

func TestDataInspectAndBackupCommandsDoNotStartDaemon(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()
	if _, err := bootstrap.MigrateData(context.Background(), dataDir); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := Execute(context.Background(), []string{"--data-dir", dataDir, "data", "inspect"}, &output, &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "schema=13 target=13 migration=false") {
		t.Fatalf("inspect output = %q", output.String())
	}
	output.Reset()
	backup := filepath.Join(dataDir, "manual.bak")
	if err := Execute(context.Background(), []string{"--data-dir", dataDir, "data", "backup", "--output", backup}, &output, &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "Verified backup") {
		t.Fatalf("backup output = %q", output.String())
	}
	if info, err := os.Stat(backup); err != nil || info.Size() == 0 {
		t.Fatalf("backup info=%v error=%v", info, err)
	}
}
