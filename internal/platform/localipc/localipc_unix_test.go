//go:build unix

package localipc

import (
	"fmt"
	"net/http"
	"os"
	"testing"
)

func TestUnixListenerAndHTTPClient(t *testing.T) {
	t.Parallel()

	address := DefaultAddress(t.TempDir())
	listener, err := Listener(address)
	if err != nil {
		t.Fatalf("Listener() error = %v", err)
	}
	if second, err := Listener(address); err == nil {
		_ = second.Close()
		t.Fatal("second Listener() error = nil")
	}
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, "ready")
	})}
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	t.Cleanup(func() {
		_ = server.Close()
		<-done
	})
	response, err := HTTPClient(address).Get("http://switchyard.local/status")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", response.StatusCode)
	}
	info, err := os.Stat(address)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("socket permissions = %o, want 600", got)
	}
}
