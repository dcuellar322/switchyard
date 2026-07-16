//go:build !unix && !windows

package localipc

import (
	"net"
	"net/http"
)

// DefaultAddress returns an empty unsupported endpoint.
func DefaultAddress(string) string { return "" }

func listen(string) (net.Listener, error) { return nil, ErrUnsupported }

func httpClient(string) *http.Client { return &http.Client{} }
