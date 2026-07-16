package process

import (
	"context"
	"strconv"
	"strings"
	"time"

	"switchyard.dev/switchyard/internal/runtime/domain"
)

func (d *Driver) watchEvents(ctx context.Context, project domain.ProjectRuntime, sink domain.EventSink) error {
	channel := make(chan domain.RuntimeEvent, 32)
	d.mu.Lock()
	if d.subscribers[project.ProjectID] == nil {
		d.subscribers[project.ProjectID] = make(map[chan domain.RuntimeEvent]struct{})
	}
	d.subscribers[project.ProjectID][channel] = struct{}{}
	d.mu.Unlock()
	defer func() {
		d.mu.Lock()
		delete(d.subscribers[project.ProjectID], channel)
		if len(d.subscribers[project.ProjectID]) == 0 {
			delete(d.subscribers, project.ProjectID)
		}
		d.mu.Unlock()
	}()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	lastSignature := ""
	if observation, err := d.inspect(ctx, project); err == nil {
		lastSignature = observationSignature(observation)
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event := <-channel:
			if err := sink.WriteRuntimeEvent(ctx, event); err != nil {
				return err
			}
			if observation, err := d.inspect(ctx, project); err == nil {
				lastSignature = observationSignature(observation)
			}
		case occurredAt := <-ticker.C:
			observation, err := d.inspect(ctx, project)
			if err != nil {
				return err
			}
			signature := observationSignature(observation)
			if signature == lastSignature {
				continue
			}
			lastSignature = signature
			if err := sink.WriteRuntimeEvent(ctx, domain.RuntimeEvent{
				Driver: domain.KindProcess, ProjectIdentity: project.ProjectSlug,
				Action: "reconcile", OccurredAt: occurredAt.UTC(),
			}); err != nil {
				return err
			}
		}
	}
}

func observationSignature(observation domain.Observation) string {
	var builder strings.Builder
	builder.WriteString(string(observation.State))
	builder.WriteByte('|')
	builder.WriteString(string(observation.Origin))
	for _, service := range observation.Services {
		builder.WriteByte('|')
		builder.WriteString(service.ID)
		builder.WriteByte(':')
		builder.WriteString(service.State)
		if service.Process != nil {
			builder.WriteByte(':')
			builder.WriteString(strconv.FormatInt(int64(service.Process.PID), 10))
			builder.WriteByte(':')
			builder.WriteString(service.Process.Fingerprint)
		}
	}
	return builder.String()
}

func (d *Driver) emit(projectID string, event domain.RuntimeEvent) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	for subscriber := range d.subscribers[projectID] {
		select {
		case subscriber <- event:
		default:
		}
	}
}
