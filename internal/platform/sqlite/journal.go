package sqlite

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"switchyard.dev/switchyard/internal/foundation/events"
	"switchyard.dev/switchyard/internal/platform/sqlite/generated"
)

// Journal is a durable SQLite event journal with in-process fan-out.
type Journal struct {
	queries *generated.Queries
	mu      sync.Mutex
	nextID  uint64
	live    map[uint64]chan events.Envelope
}

// NewJournal creates a journal over database.
func NewJournal(database *Database) *Journal {
	return &Journal{queries: database.queries, live: make(map[uint64]chan events.Envelope)}
}

// Publish persists an event before broadcasting it.
func (j *Journal) Publish(ctx context.Context, event events.Envelope) (events.Envelope, error) {
	if len(event.Payload) == 0 {
		event.Payload = json.RawMessage(`{}`)
	}
	sequence, err := j.queries.CreateJournalEvent(ctx, generated.CreateJournalEventParams{
		ID: event.ID, Type: event.Type, OccurredAt: formatTime(event.OccurredAt),
		ProjectID: nullable(event.ProjectID), OperationID: nullable(event.OperationID),
		PayloadJson: string(event.Payload),
	})
	if err != nil {
		return events.Envelope{}, fmt.Errorf("insert journal event: %w", err)
	}
	event.Sequence = sequence
	j.mu.Lock()
	for id, subscriber := range j.live {
		select {
		case subscriber <- event:
		default:
			close(subscriber)
			delete(j.live, id)
		}
	}
	j.mu.Unlock()
	return event, nil
}

// Replay returns at most limit events and reports when the caller must refresh.
func (j *Journal) Replay(ctx context.Context, after int64, limit int) ([]events.Envelope, bool, error) {
	if limit < 1 {
		limit = 1
	}
	records, err := j.queries.ListJournalEventsAfter(ctx, generated.ListJournalEventsAfterParams{
		Sequence: after, Limit: int64(limit + 1),
	})
	if err != nil {
		return nil, false, fmt.Errorf("replay journal events: %w", err)
	}
	truncated := len(records) > limit
	if truncated {
		records = records[:limit]
	}
	result := make([]events.Envelope, 0, len(records))
	for _, record := range records {
		occurredAt, err := parseTime(record.OccurredAt)
		if err != nil {
			return nil, false, err
		}
		result = append(result, events.Envelope{
			ID: record.ID, Type: record.Type, OccurredAt: occurredAt,
			Sequence: record.Sequence, ProjectID: record.ProjectID.String,
			OperationID: record.OperationID.String, Payload: json.RawMessage(record.PayloadJson),
		})
	}
	return result, truncated, nil
}

// Subscribe returns a bounded live event stream and idempotent cancellation.
func (j *Journal) Subscribe(buffer int) (<-chan events.Envelope, func()) {
	if buffer < 1 {
		buffer = 1
	}
	j.mu.Lock()
	id := j.nextID
	j.nextID++
	stream := make(chan events.Envelope, buffer)
	j.live[id] = stream
	j.mu.Unlock()
	return stream, func() {
		j.mu.Lock()
		if existing, ok := j.live[id]; ok {
			close(existing)
			delete(j.live, id)
		}
		j.mu.Unlock()
	}
}
