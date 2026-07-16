package bootstrap

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type lockFile struct {
	path string
	file *os.File
}

func acquireLock(path string) (*lockFile, error) {
	for attempt := 0; attempt < 2; attempt++ {
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			return initializeLock(path, file)
		}
		if !os.IsExist(err) {
			return nil, fmt.Errorf("create daemon lock: %w", err)
		}
		stale, staleErr := staleLock(path)
		if staleErr != nil {
			return nil, staleErr
		}
		if !stale || attempt > 0 {
			return nil, fmt.Errorf("daemon lock already exists at %s", path)
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("remove stale daemon lock: %w", err)
		}
	}
	return nil, fmt.Errorf("daemon lock already exists at %s", path)
}

func initializeLock(path string, file *os.File) (*lockFile, error) {
	lock := &lockFile{path: path, file: file}
	if _, err := file.WriteString(strconv.Itoa(os.Getpid()) + "\n"); err != nil {
		_ = lock.release()
		return nil, fmt.Errorf("write daemon lock: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = lock.release()
		return nil, fmt.Errorf("sync daemon lock: %w", err)
	}
	return lock, nil
}

func staleLock(path string) (bool, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return true, nil
		}
		return false, fmt.Errorf("read daemon lock: %w", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(contents)))
	if err != nil || pid <= 0 {
		return false, fmt.Errorf("daemon lock at %s contains an invalid process ID", path)
	}
	return !processRunning(pid), nil
}

func (l *lockFile) release() error {
	var closeErr error
	if l.file != nil {
		closeErr = l.file.Close()
	}
	removeErr := os.Remove(l.path)
	if removeErr != nil && !os.IsNotExist(removeErr) {
		return fmt.Errorf("remove daemon lock: %w", removeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close daemon lock: %w", closeErr)
	}
	return nil
}
