package websocket

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"switchyard.dev/switchyard/internal/foundation/events"
)

func TestEventsSendsConnectionEnvelope(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(NewEvents(&journalStub{}))
	t.Cleanup(server.Close)
	url := "ws" + strings.TrimPrefix(server.URL, "http")

	connection, response, err := websocket.Dial(context.Background(), url, nil)
	if response != nil && response.Body != nil {
		t.Cleanup(func() { _ = response.Body.Close() })
	}
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	t.Cleanup(func() { _ = connection.CloseNow() })

	var event events.Envelope
	if err := wsjson.Read(context.Background(), connection, &event); err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if event.Type != "system.connected" || event.Sequence != 0 {
		t.Fatalf("event = %#v", event)
	}
}

type journalStub struct {
	replay []events.Envelope
}

func (j *journalStub) Publish(_ context.Context, event events.Envelope) (events.Envelope, error) {
	return event, nil
}

func (j *journalStub) Replay(context.Context, int64, int) ([]events.Envelope, bool, error) {
	return j.replay, false, nil
}

func (*journalStub) Subscribe(int) (<-chan events.Envelope, func()) {
	stream := make(chan events.Envelope)
	return stream, func() { close(stream) }
}

func TestEventsReplaysAfterSequence(t *testing.T) {
	t.Parallel()

	journal := &journalStub{replay: []events.Envelope{{
		ID: "evt_replayed", Type: "operation.running", Sequence: 3,
		OccurredAt: time.Now().UTC(), Payload: []byte(`{}`),
	}}}
	server := httptest.NewServer(NewEvents(journal))
	t.Cleanup(server.Close)
	url := "ws" + strings.TrimPrefix(server.URL, "http") + "?after=2"
	connection, response, err := websocket.Dial(context.Background(), url, nil)
	if response != nil && response.Body != nil {
		t.Cleanup(func() { _ = response.Body.Close() })
	}
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	t.Cleanup(func() { _ = connection.CloseNow() })
	var connected events.Envelope
	if err := wsjson.Read(context.Background(), connection, &connected); err != nil {
		t.Fatalf("Read(connected) error = %v", err)
	}
	var replayed events.Envelope
	if err := wsjson.Read(context.Background(), connection, &replayed); err != nil {
		t.Fatalf("Read(replayed) error = %v", err)
	}
	if replayed.ID != "evt_replayed" || replayed.Sequence != 3 {
		t.Fatalf("replayed = %#v", replayed)
	}
}
