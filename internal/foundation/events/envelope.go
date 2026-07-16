// Package events defines the stable in-process and WebSocket event envelope.
package events

import (
	"context"
	"encoding/json"
	"time"
)

// Envelope is the versioned event record shared by application publishers.
type Envelope struct {
	ID          string          `json:"id"`
	Type        string          `json:"type"`
	OccurredAt  time.Time       `json:"occurredAt"`
	Sequence    int64           `json:"sequence"`
	ProjectID   string          `json:"projectId,omitempty"`
	OperationID string          `json:"operationId,omitempty"`
	Payload     json.RawMessage `json:"payload"`
}

// Journal persists events, replays bounded history, and fans out live events.
type Journal interface {
	Publish(ctx context.Context, event Envelope) (Envelope, error)
	Replay(ctx context.Context, after int64, limit int) ([]Envelope, bool, error)
	Subscribe(buffer int) (<-chan Envelope, func())
}
