// Package websocket contains authenticated live-stream transport adapters.
package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"switchyard.dev/switchyard/internal/foundation/events"
	"switchyard.dev/switchyard/internal/foundation/identifier"
)

const replayLimit = 500

// Events replays durable history and streams journal events.
type Events struct {
	journal events.Journal
	now     func() time.Time
}

// NewEvents creates the event stream transport.
func NewEvents(journal events.Journal) *Events {
	return &Events{journal: journal, now: time.Now}
}

// ServeHTTP upgrades a local request, replays after, and follows live events.
func (e *Events) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	after, err := parseAfter(r)
	if err != nil {
		http.Error(w, "after must be a non-negative event sequence", http.StatusBadRequest)
		return
	}
	connection, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"localhost:*", "127.0.0.1:*", "[::1]:*"},
	})
	if err != nil {
		return
	}
	defer func() { _ = connection.CloseNow() }()
	streamCtx, cancel := context.WithCancel(r.Context())
	defer cancel()
	go func() {
		for {
			if _, _, err := connection.Read(streamCtx); err != nil {
				cancel()
				return
			}
		}
	}()
	live, unsubscribe := e.journal.Subscribe(64)
	defer unsubscribe()

	eventID, err := identifier.New("evt")
	if err != nil {
		_ = connection.Close(websocket.StatusInternalError, "event id unavailable")
		return
	}
	connected := events.Envelope{
		ID:         eventID,
		Type:       "system.connected",
		OccurredAt: e.now().UTC(),
		Sequence:   after,
		Payload:    json.RawMessage(`{"apiVersion":"v1"}`),
	}
	if err := wsjson.Write(streamCtx, connection, connected); err != nil {
		return
	}
	replayed, truncated, err := e.journal.Replay(streamCtx, after, replayLimit)
	if err != nil {
		_ = connection.Close(websocket.StatusInternalError, "event replay unavailable")
		return
	}
	lastSequence := after
	for _, event := range replayed {
		if err := wsjson.Write(streamCtx, connection, event); err != nil {
			return
		}
		lastSequence = event.Sequence
	}
	if truncated {
		refreshID, idErr := identifier.New("evt")
		if idErr != nil {
			return
		}
		if err := wsjson.Write(streamCtx, connection, events.Envelope{
			ID: refreshID, Type: "system.refresh_required", OccurredAt: e.now().UTC(),
			Sequence: lastSequence, Payload: json.RawMessage(`{"reason":"replay_window_exceeded"}`),
		}); err != nil {
			return
		}
	}
	for {
		select {
		case <-streamCtx.Done():
			return
		case event, ok := <-live:
			if !ok {
				_ = connection.Close(websocket.StatusTryAgainLater, "event subscriber overflow")
				return
			}
			if event.Sequence <= lastSequence {
				continue
			}
			if err := wsjson.Write(streamCtx, connection, event); err != nil {
				return
			}
			lastSequence = event.Sequence
		}
	}
}

func parseAfter(r *http.Request) (int64, error) {
	value := r.URL.Query().Get("after")
	if value == "" {
		return 0, nil
	}
	after, err := strconv.ParseInt(value, 10, 64)
	if err != nil || after < 0 {
		return 0, fmt.Errorf("invalid event sequence %q", value)
	}
	return after, nil
}
