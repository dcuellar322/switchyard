package application

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	observability "switchyard.dev/switchyard/internal/observability/domain"
	runtime "switchyard.dev/switchyard/internal/runtime/domain"
)

// LogRuntime supplies raw driver streams and trusted project enumeration.
type LogRuntime interface {
	ListProjectIDs(context.Context) ([]string, error)
	FollowLogs(context.Context, string, string, string, int, runtime.LogSink) error
}

// LogArchive is the only path from raw logs to memory, disk, export, or live clients.
type LogArchive interface {
	runtime.LogSink
	QueryLogs(context.Context, observability.LogQuery) ([]runtime.LogEntry, error)
	ReplayLogs(context.Context, string, string, int64, int) ([]runtime.LogEntry, bool, error)
	SubscribeLogs(int) (<-chan runtime.LogEntry, func())
	ExportLogs(context.Context, observability.LogQuery, string, io.Writer) error
	ApplyRetention(context.Context) error
}

// LogService supervises driver collectors and exposes unified persisted queries.
type LogService struct {
	runtime LogRuntime
	archive LogArchive
}

// NewLogService creates the redaction-first log use cases.
func NewLogService(runtime LogRuntime, archive LogArchive) *LogService {
	return &LogService{runtime: runtime, archive: archive}
}

// Logs queries a bounded durable snapshot rather than a daemon-lifetime driver buffer.
func (s *LogService) Logs(ctx context.Context, projectID, service, since, runID, operationID string, tail int) ([]runtime.LogEntry, error) {
	if tail < 1 || tail > 10_000 {
		return nil, errors.New("log tail must be between 1 and 10000")
	}
	query := observability.LogQuery{ProjectID: projectID, ServiceID: service, RunID: runID, OperationID: operationID, Limit: tail}
	if since != "" {
		parsed, err := time.Parse(time.RFC3339, since)
		if err != nil {
			duration, durationErr := time.ParseDuration(since)
			if durationErr != nil || duration <= 0 {
				return nil, errors.New("log since must be an RFC 3339 timestamp or positive duration")
			}
			parsed = time.Now().UTC().Add(-duration)
		}
		query.Since = parsed.UTC()
	}
	return s.archive.QueryLogs(ctx, query)
}

// Export writes a bounded redacted stream in plain text or NDJSON.
func (s *LogService) Export(ctx context.Context, projectID, service, runID, operationID, format string, writer io.Writer) error {
	if format != "plain" && format != "ndjson" {
		return errors.New("log export format must be plain or ndjson")
	}
	return s.archive.ExportLogs(ctx, observability.LogQuery{ProjectID: projectID, ServiceID: service, RunID: runID,
		OperationID: operationID, Limit: 10_000}, format, writer)
}

// Replay returns persisted entries after a durable sequence cursor.
func (s *LogService) Replay(ctx context.Context, projectID, service string, after int64, limit int) ([]runtime.LogEntry, bool, error) {
	return s.archive.ReplayLogs(ctx, projectID, service, after, limit)
}

// Subscribe follows the canonical redacted stream.
func (s *LogService) Subscribe(buffer int) (<-chan runtime.LogEntry, func()) {
	return s.archive.SubscribeLogs(buffer)
}

// Run tracks trusted projects and restarts finite/raw driver streams without duplicating persisted entries.
func (s *LogService) Run(ctx context.Context, onError func(string, error)) {
	if onError == nil {
		onError = func(string, error) {}
	}
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	type worker struct{ cancel context.CancelFunc }
	workers := map[string]worker{}
	var wait sync.WaitGroup
	refresh := func() {
		ids, err := s.runtime.ListProjectIDs(ctx)
		if err != nil {
			onError("", err)
			return
		}
		wanted := make(map[string]struct{}, len(ids))
		for _, projectID := range ids {
			wanted[projectID] = struct{}{}
			if _, exists := workers[projectID]; exists {
				continue
			}
			workerCtx, cancel := context.WithCancel(ctx)
			workers[projectID] = worker{cancel: cancel}
			wait.Add(1)
			go func(id string) {
				defer wait.Done()
				s.collect(workerCtx, id, onError)
			}(projectID)
		}
		for projectID, worker := range workers {
			if _, exists := wanted[projectID]; !exists {
				worker.cancel()
				delete(workers, projectID)
			}
		}
	}
	refresh()
	for {
		select {
		case <-ctx.Done():
			for _, worker := range workers {
				worker.cancel()
			}
			wait.Wait()
			return
		case <-ticker.C:
			refresh()
		}
	}
}

func (s *LogService) collect(ctx context.Context, projectID string, onError func(string, error)) {
	for {
		err := s.runtime.FollowLogs(ctx, projectID, "", "", 1_000, s.archive)
		if ctx.Err() != nil {
			return
		}
		if err != nil && !errors.Is(err, context.Canceled) {
			onError(projectID, fmt.Errorf("collect runtime logs: %w", err))
		}
		timer := time.NewTimer(time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}
