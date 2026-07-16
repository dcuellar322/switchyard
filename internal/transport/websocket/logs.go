package websocket

import (
	"context"
	"net/http"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	runtime "switchyard.dev/switchyard/internal/runtime/domain"
)

const logReplayLimit = 500

// LogStream is the redacted persisted/live log boundary used by WebSocket clients.
type LogStream interface {
	Replay(context.Context, string, string, int64, int) ([]runtime.LogEntry, bool, error)
	Subscribe(int) (<-chan runtime.LogEntry, func())
}

// Logs replays by durable sequence before following the same canonical stream.
type Logs struct{ stream LogStream }

// NewLogs creates the project-scoped log WebSocket transport.
func NewLogs(stream LogStream) *Logs { return &Logs{stream: stream} }

// ServeHTTP prevents gaps by subscribing before replay and deduplicating at the durable cursor.
func (l *Logs) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("projectId")
	if projectID == "" {
		http.Error(w, "projectId is required", http.StatusBadRequest)
		return
	}
	after, err := parseAfter(r)
	if err != nil {
		http.Error(w, "after must be a non-negative log sequence", http.StatusBadRequest)
		return
	}
	connection, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: []string{"localhost:*", "127.0.0.1:*", "[::1]:*"}})
	if err != nil {
		return
	}
	defer func() { _ = connection.CloseNow() }()
	streamCtx, cancel := context.WithCancel(r.Context())
	defer cancel()
	go readLogConnection(streamCtx, connection, cancel)
	live, unsubscribe := l.stream.Subscribe(256)
	defer unsubscribe()
	if err := wsjson.Write(streamCtx, connection, map[string]any{"type": "logs.connected", "sequence": after}); err != nil {
		return
	}
	service := r.URL.Query().Get("service")
	last, ok := l.replay(streamCtx, connection, projectID, service, after)
	if !ok {
		return
	}
	l.follow(streamCtx, connection, live, projectID, service, last)
}

func readLogConnection(ctx context.Context, connection *websocket.Conn, cancel context.CancelFunc) {
	for {
		if _, _, err := connection.Read(ctx); err != nil {
			cancel()
			return
		}
	}
}

func (l *Logs) replay(ctx context.Context, connection *websocket.Conn, projectID, service string, after int64) (int64, bool) {
	last := after
	for {
		entries, truncated, err := l.stream.Replay(ctx, projectID, service, last, logReplayLimit)
		if err != nil {
			_ = connection.Close(websocket.StatusInternalError, "log replay unavailable")
			return last, false
		}
		for _, entry := range entries {
			if err := wsjson.Write(ctx, connection, entry); err != nil {
				return last, false
			}
			last = entry.Sequence
		}
		if !truncated {
			return last, true
		}
	}
}

func (l *Logs) follow(ctx context.Context, connection *websocket.Conn, live <-chan runtime.LogEntry, projectID, service string, last int64) {
	for {
		select {
		case <-ctx.Done():
			return
		case entry, ok := <-live:
			if !ok {
				_ = connection.Close(websocket.StatusTryAgainLater, "log subscriber overflow")
				return
			}
			if entry.ProjectID != projectID || service != "" && entry.ServiceID != service || entry.Sequence <= last {
				continue
			}
			if err := wsjson.Write(ctx, connection, entry); err != nil {
				return
			}
			last = entry.Sequence
		}
	}
}
