package daemonlog

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileHandlerRedactsAndRotatesPrivateLogs(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "internal.ndjson")
	redact := func(value string) (string, bool) {
		result := strings.ReplaceAll(value, "secret-value", "[REDACTED]")
		return result, result != value
	}
	handler, err := Open(path, 64<<10, redact)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	logger := slog.New(handler)
	for range 500 {
		logger.Warn("provider secret-value failed", "token", "secret-value", "padding", strings.Repeat("x", 160))
	}
	if err := handler.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	for _, candidate := range []string{path, path + ".1"} {
		contents, readErr := os.ReadFile(candidate)
		if readErr != nil {
			t.Fatalf("read %s: %v", candidate, readErr)
		}
		if bytes.Contains(contents, []byte("secret-value")) || !bytes.Contains(contents, []byte("[REDACTED]")) {
			t.Fatalf("unsafe contents in %s", candidate)
		}
		info, statErr := os.Stat(candidate)
		if statErr != nil {
			t.Fatalf("stat %s: %v", candidate, statErr)
		}
		if info.Mode().Perm() != fileMode {
			t.Fatalf("mode for %s = %v", candidate, info.Mode().Perm())
		}
	}
}

func TestTeePreservesBothHandlers(t *testing.T) {
	t.Parallel()

	var first, second bytes.Buffer
	logger := slog.New(Tee(slog.NewTextHandler(&first, nil), slog.NewTextHandler(&second, nil)))
	logger.Log(context.Background(), slog.LevelInfo, "ready")
	if !strings.Contains(first.String(), "ready") || !strings.Contains(second.String(), "ready") {
		t.Fatalf("tee outputs = %q / %q", first.String(), second.String())
	}
}
