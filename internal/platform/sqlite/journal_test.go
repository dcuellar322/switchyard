package sqlite

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"switchyard.dev/switchyard/internal/foundation/events"
)

func TestJournalPersistsReplaysAndPublishes(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database, err := Open(ctx, filepath.Join(t.TempDir(), "switchyard.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	journal := NewJournal(database)
	live, unsubscribe := journal.Subscribe(1)
	t.Cleanup(unsubscribe)
	published, err := journal.Publish(ctx, events.Envelope{
		ID: "evt_test", Type: "operation.running", OccurredAt: time.Now().UTC(),
		OperationID: "op_test", Payload: json.RawMessage(`{"step":1}`),
	})
	if err != nil || published.Sequence != 1 {
		t.Fatalf("Publish() = %#v, %v", published, err)
	}
	select {
	case event := <-live:
		if event.ID != published.ID {
			t.Fatalf("live event = %#v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("live event timed out")
	}
	replayed, truncated, err := journal.Replay(ctx, 0, 10)
	if err != nil || truncated || len(replayed) != 1 || replayed[0].ID != published.ID {
		t.Fatalf("Replay() = %#v, %v, %v", replayed, truncated, err)
	}
}
