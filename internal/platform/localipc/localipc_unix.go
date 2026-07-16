//go:build unix

package localipc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// DefaultAddress returns a private Unix-domain-socket path.
func DefaultAddress(dataDir string) string {
	candidate := filepath.Join(dataDir, "switchyard.sock")
	if len(candidate) < 96 {
		return candidate
	}
	digest := sha256.Sum256([]byte(candidate))
	return filepath.Join(os.TempDir(), "switchyard-"+hex.EncodeToString(digest[:8])+".sock")
}

func listen(address string) (net.Listener, error) {
	if _, err := os.Lstat(address); err == nil {
		connection, dialErr := net.DialTimeout("unix", address, 100*time.Millisecond)
		if dialErr == nil {
			_ = connection.Close()
			return nil, fmt.Errorf("local IPC endpoint is already active at %s", address)
		}
		if err := os.Remove(address); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("remove stale IPC socket: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("inspect IPC socket: %w", err)
	}
	listener, err := net.Listen("unix", address)
	if err != nil {
		return nil, fmt.Errorf("listen on Unix IPC socket: %w", err)
	}
	if err := os.Chmod(address, 0o600); err != nil {
		_ = listener.Close()
		return nil, fmt.Errorf("restrict Unix IPC socket: %w", err)
	}
	return listener, nil
}

func httpClient(address string) *http.Client {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var dialer net.Dialer
			return dialer.DialContext(ctx, "unix", address)
		},
	}
	return &http.Client{Transport: transport}
}
