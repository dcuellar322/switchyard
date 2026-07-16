// Package application exposes system-level control-plane queries.
package application

import (
	"context"
	"fmt"
	"time"

	"switchyard.dev/switchyard/internal/foundation/buildinfo"
)

// HealthRepository reports durable storage health owned by the SQLite adapter.
type HealthRepository interface {
	SchemaVersion(ctx context.Context) (int64, error)
}

// Info is the transport-neutral system status result.
type Info struct {
	Status                string
	Version               string
	Commit                string
	BuiltAt               *time.Time
	APIVersion            string
	DatabaseSchemaVersion int64
	StartedAt             time.Time
}

// Query reads system health without exposing storage details to transports.
type Query struct {
	health    HealthRepository
	build     buildinfo.Info
	startedAt time.Time
}

// NewQuery creates the system status use case.
func NewQuery(health HealthRepository, build buildinfo.Info, startedAt time.Time) *Query {
	return &Query{health: health, build: build, startedAt: startedAt.UTC()}
}

// Get returns current daemon and database status.
func (q *Query) Get(ctx context.Context) (Info, error) {
	schemaVersion, err := q.health.SchemaVersion(ctx)
	if err != nil {
		return Info{}, fmt.Errorf("read database schema health: %w", err)
	}
	return Info{
		Status:                "ready",
		Version:               q.build.Version,
		Commit:                q.build.Commit,
		BuiltAt:               q.build.BuiltAt,
		APIVersion:            "v1",
		DatabaseSchemaVersion: schemaVersion,
		StartedAt:             q.startedAt,
	}, nil
}
