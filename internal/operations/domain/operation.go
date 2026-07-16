// Package domain owns durable operation lifecycle invariants.
package domain

import (
	"errors"
	"fmt"
	"time"
)

// State is a durable operation state.
type State string

const (
	// StateQueued waits for the project serialization gate.
	StateQueued State = "queued"
	// StateRunning is actively executing.
	StateRunning State = "running"
	// StateSucceeded completed all work.
	StateSucceeded State = "succeeded"
	// StateFailed stopped because of an error.
	StateFailed State = "failed"
	// StateCancelled stopped after an explicit cancellation.
	StateCancelled State = "cancelled"
	// StatePartiallySucceeded completed only a declared subset of work.
	StatePartiallySucceeded State = "partially_succeeded"
)

// ErrInvalidTransition is returned when lifecycle invariants reject a change.
var ErrInvalidTransition = errors.New("invalid operation transition")

// Operation is one durable mutation coordinated for a project.
type Operation struct {
	ID                    string
	ProjectID             string
	WorkspaceID           string
	Kind                  string
	State                 State
	IdempotencyKey        string
	Input                 []byte
	ErrorCode             string
	ErrorMessage          string
	CancellationRequested bool
	RequestedAt           time.Time
	StartedAt             *time.Time
	FinishedAt            *time.Time
	UpdatedAt             time.Time
}

// Terminal reports whether no further transition is allowed.
func (o Operation) Terminal() bool {
	switch o.State {
	case StateSucceeded, StateFailed, StateCancelled, StatePartiallySucceeded:
		return true
	case StateQueued, StateRunning:
		return false
	default:
		return false
	}
}

// CanTransition reports whether next is valid from the current state.
func (o Operation) CanTransition(next State) bool {
	switch o.State {
	case StateQueued:
		return next == StateRunning || next == StateCancelled || next == StateFailed
	case StateRunning:
		return next == StateSucceeded || next == StateFailed || next == StateCancelled || next == StatePartiallySucceeded
	case StateSucceeded, StateFailed, StateCancelled, StatePartiallySucceeded:
		return false
	default:
		return false
	}
}

// Transition validates and applies a lifecycle change.
func (o Operation) Transition(next State, at time.Time, errorCode, errorMessage string) (Operation, error) {
	if !o.CanTransition(next) {
		return Operation{}, fmt.Errorf("%w: %s to %s", ErrInvalidTransition, o.State, next)
	}
	at = at.UTC()
	o.State = next
	o.UpdatedAt = at
	o.ErrorCode = errorCode
	o.ErrorMessage = errorMessage
	if next == StateRunning {
		o.StartedAt = &at
	}
	if next != StateRunning {
		o.FinishedAt = &at
	}
	return o, nil
}
