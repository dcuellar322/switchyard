package domain

import (
	"errors"
	"testing"
	"time"
)

func TestOperationStateMachine(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC)
	operation := Operation{State: StateQueued}
	running, err := operation.Transition(StateRunning, now, "", "")
	if err != nil || running.StartedAt == nil {
		t.Fatalf("running transition = %#v, %v", running, err)
	}
	finished, err := running.Transition(StateSucceeded, now.Add(time.Second), "", "")
	if err != nil || !finished.Terminal() || finished.FinishedAt == nil {
		t.Fatalf("finished transition = %#v, %v", finished, err)
	}
	if _, err := finished.Transition(StateRunning, now, "", ""); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("terminal transition error = %v", err)
	}
}
