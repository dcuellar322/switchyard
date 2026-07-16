package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDebugLogsReadsOnlyRedactedInternalEvents(t *testing.T) {
	t.Parallel()

	dataDir := t.TempDir()
	line := `{"time":"2026-07-16T12:00:00Z","level":"ERROR","msg":"token=[REDACTED]","component":"runtime"}` + "\n"
	if err := os.WriteFile(filepath.Join(dataDir, "internal.ndjson"), []byte(line), 0o600); err != nil {
		t.Fatalf("write log: %v", err)
	}
	var output bytes.Buffer
	if err := Execute(context.Background(), []string{"--data-dir", dataDir, "debug", "logs", "--level", "error", "--limit", "1"}, &output, &output); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(output.String(), "[REDACTED]") || !strings.Contains(output.String(), "runtime") {
		t.Fatalf("output = %q", output.String())
	}
}

func TestDoctorBundleFlagsRequireBundle(t *testing.T) {
	t.Parallel()

	command := newRootCommand(&rootOptions{stdout: &bytes.Buffer{}, stderr: &bytes.Buffer{}})
	command.SetArgs([]string{"doctor", "--preview"})
	if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "--bundle") {
		t.Fatalf("Execute() error = %v", err)
	}
}

func TestDefaultBundlePathIsPortable(t *testing.T) {
	t.Parallel()

	path := defaultBundlePath(time.Date(2026, 7, 16, 12, 34, 56, 0, time.FixedZone("test", -5*60*60)))
	if filepath.Base(path) != "switchyard-support-20260716T173456Z.zip" {
		t.Fatalf("defaultBundlePath() = %q", path)
	}
}
