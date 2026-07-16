//go:build windows

package localipc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"
)

// DefaultAddress returns a collision-resistant named-pipe identity for the data directory.
func DefaultAddress(dataDir string) string {
	absolute, err := filepath.Abs(dataDir)
	if err == nil {
		dataDir = absolute
	}
	digest := sha256.Sum256([]byte(strings.ToLower(filepath.Clean(dataDir))))
	return `\\.\pipe\switchyard-` + hex.EncodeToString(digest[:12])
}

func listen(address string) (net.Listener, error) {
	probe, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	connection, err := winio.DialPipeContext(probe, address)
	if err == nil {
		_ = connection.Close()
		return nil, fmt.Errorf("local IPC endpoint is already active at %s", address)
	}
	descriptor, err := currentUserPipeDescriptor()
	if err != nil {
		return nil, fmt.Errorf("build named-pipe security descriptor: %w", err)
	}
	listener, err := winio.ListenPipe(address, &winio.PipeConfig{
		SecurityDescriptor: descriptor,
		InputBufferSize:    64 << 10,
		OutputBufferSize:   64 << 10,
	})
	if err != nil {
		return nil, fmt.Errorf("listen on Windows named pipe: %w", err)
	}
	return listener, nil
}

func httpClient(address string) *http.Client {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return winio.DialPipeContext(ctx, address)
		},
	}
	return &http.Client{Transport: transport}
}

func currentUserPipeDescriptor() (string, error) {
	token, err := windows.OpenCurrentProcessToken()
	if err != nil {
		return "", err
	}
	defer func() { _ = token.Close() }()
	user, err := token.GetTokenUser()
	if err != nil {
		return "", err
	}
	// Protected DACL: LocalSystem and only the daemon owner's SID receive full access.
	return "D:P(A;;GA;;;SY)(A;;GA;;;" + user.User.Sid.String() + ")", nil
}
