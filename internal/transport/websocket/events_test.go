package websocket

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

func TestEventsSendsConnectionEnvelope(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(NewEvents())
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

	var event Event
	if err := wsjson.Read(context.Background(), connection, &event); err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if event.Type != "system.connected" || event.Sequence != 1 {
		t.Fatalf("event = %#v", event)
	}
}
