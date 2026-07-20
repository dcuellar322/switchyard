package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	observabilityAdapters "switchyard.dev/switchyard/internal/observability/adapters"
	observability "switchyard.dev/switchyard/internal/observability/domain"
	runtime "switchyard.dev/switchyard/internal/runtime/domain"
)

func TestLogStoreRedactsLivePersistedExportAndDiagnosticQueries(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	_, store, root := newTestLogStore(t, LogStoreConfig{RingCapacity: 2, SegmentBytes: 1 << 20, RetentionBytes: 1 << 20})
	live, cancel := store.SubscribeLogs(2)
	defer cancel()
	entry := runtime.LogEntry{Timestamp: time.Now().UTC(), ProjectID: "project-1", ServiceID: "api", RunID: "run-1",
		Source: "process", Stream: "stdout", Level: "info", Message: "token=fixture-secret", Attributes: map[string]string{"authorization": "Bearer abc.def"}}
	if err := store.WriteLog(ctx, entry); err != nil {
		t.Fatal(err)
	}
	liveEntry := <-live
	queried, err := store.QueryLogs(ctx, observability.LogQuery{ProjectID: "project-1", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	var exported strings.Builder
	if err := store.ExportLogs(ctx, observability.LogQuery{ProjectID: "project-1", Limit: 10}, "ndjson", &exported); err != nil {
		t.Fatal(err)
	}
	disk, err := readAllLogFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]string{"live": liveEntry.Message + liveEntry.Attributes["authorization"], "query": queried[0].Message, "export": exported.String(), "disk": disk} {
		if strings.Contains(value, "fixture-secret") || strings.Contains(value, "abc.def") || !strings.Contains(value, "[REDACTED]") {
			t.Fatalf("%s output is not redacted: %s", name, value)
		}
	}
	if !liveEntry.Redacted || liveEntry.Sequence < 1 {
		t.Fatalf("live entry = %#v", liveEntry)
	}
	if err := filepath.Walk(root, func(_ string, info os.FileInfo, walkErr error) error {
		if walkErr == nil && !info.IsDir() && info.Mode().Perm() != 0o600 {
			t.Errorf("log mode = %o, want 600", info.Mode().Perm())
		}
		return walkErr
	}); err != nil {
		t.Fatal(err)
	}
}

func TestLogStoreDeduplicatesReplayedDriverSnapshots(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	_, store, _ := newTestLogStore(t, LogStoreConfig{})
	entry := runtime.LogEntry{Timestamp: time.Now().UTC(), ProjectID: "project-1", ServiceID: "api", RunID: "run-1", Source: "process", Stream: "stdout", Message: "same", Attributes: map[string]string{}}
	if err := store.WriteLog(ctx, entry); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteLog(ctx, entry); err != nil {
		t.Fatal(err)
	}
	entries, err := store.QueryLogs(ctx, observability.LogQuery{ProjectID: "project-1", Limit: 10})
	if err != nil || len(entries) != 1 {
		t.Fatalf("entries = %#v, error = %v", entries, err)
	}
}

func TestLogStoreRotationAndRetentionStayWithinDiskCap(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database, store, root := newTestLogStore(t, LogStoreConfig{SegmentBytes: 220, RetentionBytes: 500, RetentionAge: time.Hour})
	for index := 0; index < 30; index++ {
		entry := runtime.LogEntry{Timestamp: time.Now().UTC().Add(time.Duration(index) * time.Millisecond), ProjectID: "project-1", ServiceID: "api", RunID: "run-1",
			Source: "process", Stream: "stdout", Message: strings.Repeat("x", 80) + string(rune('a'+index%26)), Attributes: map[string]string{}}
		if err := store.WriteLog(ctx, entry); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.ApplyRetention(ctx); err != nil {
		t.Fatal(err)
	}
	var bytes int64
	err := filepath.Walk(root, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			bytes += info.Size()
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if bytes > 500 {
		t.Fatalf("retained bytes = %d, want <= 500", bytes)
	}
	var missingChecksums int
	if err := database.connection.QueryRow(`SELECT COUNT(*) FROM log_segments WHERE closed_at IS NOT NULL AND (sha256 IS NULL OR length(sha256) != 64)`).Scan(&missingChecksums); err != nil {
		t.Fatal(err)
	}
	if missingChecksums != 0 {
		t.Fatalf("closed segments without checksums = %d", missingChecksums)
	}
}

func TestLogStoreRetentionRemovesClosedSegmentsPastAge(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database, store, _ := newTestLogStore(t, LogStoreConfig{SegmentBytes: 220, RetentionBytes: 1 << 20, RetentionAge: time.Minute})
	base := time.Now().UTC()
	store.now = func() time.Time { return base }
	for index := 0; index < 4; index++ {
		if err := store.WriteLog(ctx, runtime.LogEntry{Timestamp: base.Add(time.Duration(index) * time.Millisecond), ProjectID: "project-1", ServiceID: "api",
			RunID: "run-1", Source: "process", Stream: "stdout", Message: strings.Repeat("age", 40) + string(rune('a'+index)), Attributes: map[string]string{}}); err != nil {
			t.Fatal(err)
		}
	}
	store.now = func() time.Time { return base.Add(2 * time.Minute) }
	if err := store.ApplyRetention(ctx); err != nil {
		t.Fatal(err)
	}
	var closed int
	if err := database.connection.QueryRow(`SELECT COUNT(*) FROM log_segments WHERE closed_at IS NOT NULL`).Scan(&closed); err != nil {
		t.Fatal(err)
	}
	if closed != 0 {
		t.Fatalf("expired closed segments = %d", closed)
	}
}

func TestLogStoreRecoveryCannotDeleteThroughStagedSymlink(t *testing.T) {
	t.Parallel()
	_, store, root := newTestLogStore(t, LogStoreConfig{})
	target := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(target, []byte("preserve"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "orphan.deleting")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink fixture is unavailable: %v", err)
	}
	if err := store.recoverStagedDeletions(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("outside target was affected: %v", err)
	}
	if _, err := os.Lstat(link); !os.IsNotExist(err) {
		t.Fatalf("staged symlink still exists: %v", err)
	}
}

func newTestLogStore(t *testing.T, config LogStoreConfig) (*Database, *LogStore, string) {
	t.Helper()
	root := t.TempDir()
	database, err := Open(context.Background(), filepath.Join(root, "switchyard.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	_, err = database.connection.Exec(`INSERT INTO projects
        (id, slug, display_name, trust_state, primary_location, created_at, updated_at)
        VALUES ('project-1', 'project-1', 'Project One', 'trusted', '/tmp/project-1', ?, ?)`, formatTime(time.Now()), formatTime(time.Now()))
	if err != nil {
		t.Fatal(err)
	}
	redactor, err := observabilityAdapters.NewRedactor(nil)
	if err != nil {
		t.Fatal(err)
	}
	config.Directory = filepath.Join(root, "logs")
	store, err := NewLogStore(database, config, redactor)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close(context.Background()) })
	return database, store, config.Directory
}

func readAllLogFiles(root string) (string, error) {
	var result strings.Builder
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		value, err := os.ReadFile(path)
		if err == nil {
			result.Write(value)
		}
		return err
	})
	return result.String(), err
}
