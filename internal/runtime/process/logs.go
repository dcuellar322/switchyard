package process

import (
	"context"
	"sort"
	"sync"

	"switchyard.dev/switchyard/internal/runtime/domain"
)

type logBuffer struct {
	mu          sync.RWMutex
	capacity    int
	entries     []domain.LogEntry
	subscribers map[chan domain.LogEntry]struct{}
}

func newLogBuffer(capacity int) *logBuffer {
	return &logBuffer{capacity: capacity, subscribers: make(map[chan domain.LogEntry]struct{})}
}

func (b *logBuffer) add(entry domain.LogEntry) {
	b.mu.Lock()
	if len(b.entries) == b.capacity {
		copy(b.entries, b.entries[1:])
		b.entries[len(b.entries)-1] = entry
	} else {
		b.entries = append(b.entries, entry)
	}
	for subscriber := range b.subscribers {
		select {
		case subscriber <- entry:
		default:
		}
	}
	b.mu.Unlock()
}

func (b *logBuffer) snapshot(tail int) []domain.LogEntry {
	b.mu.RLock()
	defer b.mu.RUnlock()
	start := max(0, len(b.entries)-tail)
	return append([]domain.LogEntry(nil), b.entries[start:]...)
}

func (b *logBuffer) subscribe() (<-chan domain.LogEntry, func()) {
	channel := make(chan domain.LogEntry, 128)
	b.mu.Lock()
	b.subscribers[channel] = struct{}{}
	b.mu.Unlock()
	return channel, func() {
		b.mu.Lock()
		delete(b.subscribers, channel)
		close(channel)
		b.mu.Unlock()
	}
}

func (d *Driver) streamLogs(ctx context.Context, request domain.LogRequest, sink domain.LogSink) error {
	selected, err := d.selectLogBuffers(ctx, request)
	if err != nil {
		return err
	}
	if err := writeLogSnapshot(ctx, selected, request.Tail, sink); err != nil {
		return err
	}
	if !request.Follow || len(selected) == 0 {
		return nil
	}
	return followLogBuffers(ctx, selected, sink)
}

func (d *Driver) selectLogBuffers(ctx context.Context, request domain.LogRequest) ([]*logBuffer, error) {
	runs, err := d.store.ListProjectRuns(ctx, request.Project.ProjectID)
	if err != nil {
		return nil, err
	}
	selected := []*logBuffer{}
	d.mu.RLock()
	for index := len(runs) - 1; index >= 0; index-- {
		run := runs[index]
		if request.Service != "" && run.ServiceID != request.Service {
			continue
		}
		if buffer := d.logs[run.ID]; buffer != nil {
			selected = append(selected, buffer)
		}
	}
	d.mu.RUnlock()
	return selected, nil
}

func writeLogSnapshot(ctx context.Context, selected []*logBuffer, tail int, sink domain.LogSink) error {
	entries := []domain.LogEntry{}
	for _, buffer := range selected {
		entries = append(entries, buffer.snapshot(tail)...)
	}
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].Timestamp.Before(entries[j].Timestamp) })
	if len(entries) > tail {
		entries = entries[len(entries)-tail:]
	}
	for _, entry := range entries {
		if err := sink.WriteLog(ctx, entry); err != nil {
			return err
		}
	}
	return nil
}

func followLogBuffers(ctx context.Context, selected []*logBuffer, sink domain.LogSink) error {
	type subscription struct {
		entries <-chan domain.LogEntry
		cancel  func()
	}
	subscriptions := make([]subscription, 0, len(selected))
	merged := make(chan domain.LogEntry, 128)
	for _, buffer := range selected {
		entries, cancel := buffer.subscribe()
		subscriptions = append(subscriptions, subscription{entries: entries, cancel: cancel})
		go func(source <-chan domain.LogEntry) {
			for entry := range source {
				select {
				case merged <- entry:
				case <-ctx.Done():
					return
				}
			}
		}(entries)
	}
	defer func() {
		for _, item := range subscriptions {
			item.cancel()
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case entry := <-merged:
			if err := sink.WriteLog(ctx, entry); err != nil {
				return err
			}
		}
	}
}
