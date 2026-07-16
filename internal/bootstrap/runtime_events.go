package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"

	"switchyard.dev/switchyard/internal/foundation/events"
	"switchyard.dev/switchyard/internal/foundation/identifier"
	runtimeApplication "switchyard.dev/switchyard/internal/runtime/application"
	"switchyard.dev/switchyard/internal/runtime/domain"
)

type runtimeReconciliationSink struct {
	runtime *runtimeApplication.Service
	journal events.Journal
}

func (s runtimeReconciliationSink) WriteRuntimeEvent(ctx context.Context, trigger domain.RuntimeEvent) error {
	observation, err := s.runtime.Inspect(ctx, trigger.ProjectID)
	if err != nil {
		return fmt.Errorf("reconcile project runtime: %w", err)
	}
	payload, err := json.Marshal(map[string]any{"trigger": trigger, "observation": observation})
	if err != nil {
		return fmt.Errorf("encode runtime reconciliation: %w", err)
	}
	eventID, err := identifier.New("evt")
	if err != nil {
		return err
	}
	_, err = s.journal.Publish(ctx, events.Envelope{
		ID: eventID, Type: "runtime.observed", ProjectID: trigger.ProjectID,
		OccurredAt: trigger.OccurredAt, Payload: payload,
	})
	if err != nil {
		return fmt.Errorf("publish runtime reconciliation: %w", err)
	}
	return nil
}
