//go:build windows

package localipc

import (
	"fmt"
	"net/http"
	"testing"
)

func TestWindowsNamedPipeListenerAndHTTPClient(t *testing.T) {
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
	server := &http.Server{Handler: http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(writer, "ready")
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
}

func TestWindowsNamedPipeAddressUsesFullDataDirectory(t *testing.T) {
	t.Parallel()
	if DefaultAddress(`C:\\Users\\one\\Switchyard`) == DefaultAddress(`D:\\Users\\one\\Switchyard`) {
		t.Fatal("different data directories produced the same pipe address")
	}
}
