package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"

	terminalDomain "switchyard.dev/switchyard/internal/terminal/domain"
)

func TestTerminalWebSocketCarriesReadyScrollbackInputResizeAndOutput(t *testing.T) {
	stream := &fakeTerminalStream{
		snapshot: []byte("prior output\r\n"), output: make(chan []byte, 2),
		writes: make(chan []byte, 1), resizes: make(chan terminalDomain.Size, 1),
	}
	sessions := &fakeTerminalSessions{stream: stream}
	handler := &Terminal{sessions: sessions, owner: func(context.Context) terminalDomain.Owner {
		return terminalDomain.Owner{Type: "browser", ID: "session_one"}
	}}
	router := chi.NewRouter()
	router.Handle("/ws/v1/terminal/{sessionId}", handler)
	server := httptest.NewServer(router)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	options := &websocket.DialOptions{HTTPHeader: map[string][]string{"Origin": {server.URL}}}
	connection, response, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/ws/v1/terminal/terminal_one", options)
	if response != nil && response.Body != nil {
		t.Cleanup(func() {
			if closeErr := response.Body.Close(); closeErr != nil {
				t.Errorf("close handshake response: %v", closeErr)
			}
		})
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = connection.CloseNow()
	})

	messageType, value, err := connection.Read(ctx)
	if err != nil || messageType != websocket.MessageText {
		t.Fatalf("ready read = %v, %v", messageType, err)
	}
	var ready terminalServerControl
	if err := json.Unmarshal(value, &ready); err != nil || ready.Type != "ready" || ready.CapturePolicy != terminalDomain.CaptureUserVisibleOutput {
		t.Fatalf("ready = %#v, %v", ready, err)
	}
	messageType, value, err = connection.Read(ctx)
	if err != nil || messageType != websocket.MessageBinary || string(value) != "prior output\r\n" {
		t.Fatalf("snapshot = %q, %v, %v", value, messageType, err)
	}
	if err := connection.Write(ctx, websocket.MessageBinary, []byte("echo terminal\n")); err != nil {
		t.Fatal(err)
	}
	select {
	case input := <-stream.writes:
		if string(input) != "echo terminal\n" {
			t.Fatalf("input = %q", input)
		}
	case <-ctx.Done():
		t.Fatal("input was not forwarded")
	}
	if err := connection.Write(ctx, websocket.MessageText, []byte(`{"type":"resize","columns":140,"rows":44}`)); err != nil {
		t.Fatal(err)
	}
	select {
	case size := <-stream.resizes:
		if size != (terminalDomain.Size{Columns: 140, Rows: 44}) {
			t.Fatalf("resize = %+v", size)
		}
	case <-ctx.Done():
		t.Fatal("resize was not forwarded")
	}
	stream.output <- []byte("live output")
	messageType, value, err = connection.Read(ctx)
	if err != nil || messageType != websocket.MessageBinary || string(value) != "live output" {
		t.Fatalf("live output = %q, %v, %v", value, messageType, err)
	}
	sessions.finish()
	close(stream.output)
	messageType, value, err = connection.Read(ctx)
	if err != nil || messageType != websocket.MessageText {
		t.Fatalf("exit read = %q, %v, %v", value, messageType, err)
	}
	var exit terminalServerControl
	if err := json.Unmarshal(value, &exit); err != nil || exit.Type != "exit" || exit.Status != terminalDomain.StatusExited {
		t.Fatalf("exit = %#v, %v", exit, err)
	}
}

func TestTerminalWebSocketRejectsUnknownControlMessage(t *testing.T) {
	stream := &fakeTerminalStream{output: make(chan []byte), writes: make(chan []byte, 1), resizes: make(chan terminalDomain.Size, 1)}
	handler := &Terminal{sessions: &fakeTerminalSessions{stream: stream}, owner: func(context.Context) terminalDomain.Owner {
		return terminalDomain.Owner{Type: "browser", ID: "session_one"}
	}}
	router := chi.NewRouter()
	router.Handle("/ws/v1/terminal/{sessionId}", handler)
	server := httptest.NewServer(router)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	connection, response, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/ws/v1/terminal/terminal_one", &websocket.DialOptions{HTTPHeader: map[string][]string{"Origin": {server.URL}}})
	if response != nil && response.Body != nil {
		t.Cleanup(func() {
			if closeErr := response.Body.Close(); closeErr != nil {
				t.Errorf("close handshake response: %v", closeErr)
			}
		})
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = connection.CloseNow()
	})
	if _, _, err := connection.Read(ctx); err != nil {
		t.Fatal(err)
	}
	if err := connection.Write(ctx, websocket.MessageText, []byte(`{"type":"execute","command":"id"}`)); err != nil {
		t.Fatal(err)
	}
	_, _, err = connection.Read(ctx)
	if websocket.CloseStatus(err) != websocket.StatusPolicyViolation {
		t.Fatalf("close error = %v", err)
	}
}

type fakeTerminalStream struct {
	snapshot []byte
	output   chan []byte
	writes   chan []byte
	resizes  chan terminalDomain.Size
}

func (s *fakeTerminalStream) Write(value []byte) error {
	s.writes <- append([]byte(nil), value...)
	return nil
}
func (s *fakeTerminalStream) Resize(size terminalDomain.Size) error { s.resizes <- size; return nil }
func (s *fakeTerminalStream) Close()                                {}
func (s *fakeTerminalStream) SnapshotBytes() []byte                 { return append([]byte(nil), s.snapshot...) }
func (s *fakeTerminalStream) OutputBytes() <-chan []byte            { return s.output }

type fakeTerminalSessions struct {
	stream   terminalStream
	mu       sync.Mutex
	finished bool
}

func (s *fakeTerminalSessions) Attach(_ context.Context, _ string, owner terminalDomain.Owner) (terminalStream, error) {
	if owner != (terminalDomain.Owner{Type: "browser", ID: "session_one"}) {
		return nil, errors.New("owner mismatch")
	}
	return s.stream, nil
}
func (s *fakeTerminalSessions) Get(context.Context, string, terminalDomain.Owner) (terminalDomain.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	status := terminalDomain.StatusActive
	var exitCode *int
	if s.finished {
		status = terminalDomain.StatusExited
		code := 0
		exitCode = &code
	}
	return terminalDomain.Session{
		ID: "terminal_one", ProjectID: "project_one", Kind: terminalDomain.KindShell, DisplayName: "Project shell",
		Owner: terminalDomain.Owner{Type: "browser", ID: "session_one"}, WorkingDirectory: "/tmp/project", Status: status,
		PersistencePolicy: terminalDomain.PersistenceDetachUntilIdle, CapturePolicy: terminalDomain.CaptureUserVisibleOutput,
		ExitCode: exitCode, CreatedAt: time.Now().UTC(),
	}, nil
}
func (s *fakeTerminalSessions) finish() { s.mu.Lock(); s.finished = true; s.mu.Unlock() }
