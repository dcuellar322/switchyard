package websocket

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	runtime "switchyard.dev/switchyard/internal/runtime/domain"
)

type logStreamStub struct {
	replay []runtime.LogEntry
	live   chan runtime.LogEntry
}

func (s *logStreamStub) Replay(_ context.Context, _, _ string, after int64, _ int) ([]runtime.LogEntry, bool, error) {
	var result []runtime.LogEntry
	for _, entry := range s.replay {
		if entry.Sequence > after {
			result = append(result, entry)
		}
	}
	return result, false, nil
}
func (s *logStreamStub) Subscribe(int) (<-chan runtime.LogEntry, func()) { return s.live, func() {} }

func TestLogsReconnectReplaysThenDeduplicatesLiveOverlap(t *testing.T) {
	t.Parallel()
	stream := &logStreamStub{replay: []runtime.LogEntry{{Sequence: 2, ProjectID: "project-1"}, {Sequence: 3, ProjectID: "project-1"}}, live: make(chan runtime.LogEntry, 2)}
	server := httptest.NewServer(NewLogs(stream))
	t.Cleanup(server.Close)
	connection, response, err := websocket.Dial(context.Background(), "ws"+strings.TrimPrefix(server.URL, "http")+"?projectId=project-1&after=1", nil)
	if response != nil && response.Body != nil {
		t.Cleanup(func() { _ = response.Body.Close() })
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.CloseNow() })
	var connected map[string]any
	if err := wsjson.Read(context.Background(), connection, &connected); err != nil {
		t.Fatal(err)
	}
	stream.live <- runtime.LogEntry{Sequence: 3, ProjectID: "project-1"}
	stream.live <- runtime.LogEntry{Sequence: 4, ProjectID: "project-1", Timestamp: time.Now()}
	for _, want := range []int64{2, 3, 4} {
		var entry runtime.LogEntry
		if err := wsjson.Read(context.Background(), connection, &entry); err != nil {
			t.Fatal(err)
		}
		if entry.Sequence != want {
			t.Fatalf("sequence = %d, want %d", entry.Sequence, want)
		}
	}
}
