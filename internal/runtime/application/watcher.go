package application

import (
	"context"
	"errors"
	"time"

	"switchyard.dev/switchyard/internal/runtime/domain"
)

const watcherRefreshInterval = 2 * time.Second

type activeWatch struct {
	manifestHash string
	cancel       context.CancelFunc
}

type watchResult struct {
	projectID    string
	manifestHash string
	err          error
}

// WatchAll maintains targeted Engine-event subscriptions as trusted projects change.
func (s *Service) WatchAll(ctx context.Context, sink domain.EventSink, onError func(string, error)) {
	if onError == nil {
		onError = func(string, error) {}
	}
	ticker := time.NewTicker(watcherRefreshInterval)
	defer ticker.Stop()
	results := make(chan watchResult)
	active := make(map[string]activeWatch)
	refresh := func() { s.refreshWatches(ctx, sink, active, results, onError) }
	refresh()
	for {
		select {
		case <-ctx.Done():
			for _, watch := range active {
				watch.cancel()
			}
			return
		case result := <-results:
			watch, ok := active[result.projectID]
			if ok && watch.manifestHash == result.manifestHash {
				delete(active, result.projectID)
			}
			if result.err != nil && !errors.Is(result.err, context.Canceled) {
				onError(result.projectID, result.err)
			}
		case <-ticker.C:
			refresh()
		}
	}
}

func (s *Service) refreshWatches(
	ctx context.Context,
	sink domain.EventSink,
	active map[string]activeWatch,
	results chan<- watchResult,
	onError func(string, error),
) {
	ids, err := s.projects.ListRuntimeProjectIDs(ctx)
	if err != nil {
		onError("", err)
		return
	}
	wanted := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		wanted[id] = struct{}{}
		project, driver, resolveErr := s.resolve(ctx, id)
		if resolveErr != nil {
			onError(id, resolveErr)
			continue
		}
		if existing, ok := active[id]; ok && existing.manifestHash == project.ManifestHash {
			continue
		} else if ok {
			existing.cancel()
		}
		watchCtx, cancel := context.WithCancel(ctx)
		active[id] = activeWatch{manifestHash: project.ManifestHash, cancel: cancel}
		go func(project domain.ProjectRuntime, driver Driver) {
			err := driver.WatchEvents(watchCtx, project, projectEventSink{projectID: project.ProjectID, sink: sink})
			select {
			case results <- watchResult{projectID: project.ProjectID, manifestHash: project.ManifestHash, err: err}:
			case <-ctx.Done():
			}
		}(project, driver)
	}
	for id, watch := range active {
		if _, ok := wanted[id]; !ok {
			watch.cancel()
			delete(active, id)
		}
	}
}

type projectEventSink struct {
	projectID string
	sink      domain.EventSink
}

func (s projectEventSink) WriteRuntimeEvent(ctx context.Context, event domain.RuntimeEvent) error {
	event.ProjectID = s.projectID
	return s.sink.WriteRuntimeEvent(ctx, event)
}
