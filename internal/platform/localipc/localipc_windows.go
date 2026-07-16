//go:build windows

package localipc

import (
	"net"
	"net/http"
	"path/filepath"
)

// DefaultAddress returns the stable named-pipe identity reserved for Phase 18.
func DefaultAddress(dataDir string) string {
	return `\\.\pipe\switchyard-` + filepath.Base(dataDir)
}

func listen(string) (net.Listener, error) { return nil, ErrUnsupported }

func httpClient(string) *http.Client { return &http.Client{} }
