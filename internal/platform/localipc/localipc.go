// Package localipc abstracts privileged local HTTP transport.
package localipc

import (
	"errors"
	"net"
	"net/http"
)

// ErrUnsupported reports an IPC transport not yet available on the platform.
var ErrUnsupported = errors.New("local IPC transport is not supported on this platform")

// Listener opens the platform-specific local endpoint.
func Listener(address string) (net.Listener, error) { return listen(address) }

// HTTPClient returns an HTTP client whose connections use the local endpoint.
func HTTPClient(address string) *http.Client { return httpClient(address) }
