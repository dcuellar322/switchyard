package bootstrap

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestLockFilePreventsConcurrentDaemonAndCleansUp(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "daemon.lock")
	lock, err := acquireLock(path)
	if err != nil {
		t.Fatalf("acquireLock() error = %v", err)
	}
	if _, err := acquireLock(path); err == nil {
		t.Fatal("second acquireLock() error = nil")
	}
	if err := lock.release(); err != nil {
		t.Fatalf("release() error = %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("lock still exists, stat error = %v", err)
	}
	lock, err = acquireLock(path)
	if err != nil {
		t.Fatalf("acquireLock() after release error = %v", err)
	}
	if err := lock.release(); err != nil {
		t.Fatalf("second release() error = %v", err)
	}
}

func TestLockFileRecoversAfterUncleanShutdown(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "daemon.lock")
	if err := os.WriteFile(path, []byte(strconv.Itoa(99_999_999)+"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	lock, err := acquireLock(path)
	if err != nil {
		t.Fatalf("acquireLock() error = %v", err)
	}
	if err := lock.release(); err != nil {
		t.Fatalf("release() error = %v", err)
	}
}
