// Package websocket contains authenticated live-stream transport adapters.
package websocket

import (
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"switchyard.dev/switchyard/internal/foundation/correlation"
)

// Event is the versioned WebSocket event envelope.
type Event struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	OccurredAt time.Time      `json:"occurredAt"`
	Sequence   int64          `json:"sequence"`
	Payload    map[string]any `json:"payload"`
}

// Events sends the walking-skeleton connection event.
type Events struct {
	now func() time.Time
}

// NewEvents creates the event stream transport.
func NewEvents() *Events {
	return &Events{now: time.Now}
}

// ServeHTTP upgrades a local request and emits a typed connection event.
func (e *Events) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	connection, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"localhost:*", "127.0.0.1:*", "[::1]:*"},
	})
	if err != nil {
		return
	}
	defer func() { _ = connection.CloseNow() }()

	eventID, err := correlation.NewID()
	if err != nil {
		_ = connection.Close(websocket.StatusInternalError, "event id unavailable")
		return
	}
	event := Event{
		ID:         eventID,
		Type:       "system.connected",
		OccurredAt: e.now().UTC(),
		Sequence:   1,
		Payload:    map[string]any{"apiVersion": "v1"},
	}
	if err := wsjson.Write(r.Context(), connection, event); err != nil {
		return
	}
	for {
		if _, _, err := connection.Read(r.Context()); err != nil {
			return
		}
	}
}
