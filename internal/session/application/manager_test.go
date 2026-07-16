package application

import (
	"errors"
	"testing"
	"time"
)

func TestBootstrapIsOneTimeAndMutationRequiresCSRF(t *testing.T) {
	t.Parallel()

	manager := NewManager()
	bootstrap, err := manager.IssueBootstrap()
	if err != nil {
		t.Fatalf("IssueBootstrap() error = %v", err)
	}
	session, err := manager.Exchange(bootstrap.Token)
	if err != nil {
		t.Fatalf("Exchange() error = %v", err)
	}
	if _, err := manager.Exchange(bootstrap.Token); !errors.Is(err, ErrInvalidBootstrap) {
		t.Fatalf("second Exchange() error = %v", err)
	}
	if _, err := manager.ValidateMutation(session.ID, "wrong"); !errors.Is(err, ErrInvalidCSRF) {
		t.Fatalf("ValidateMutation(wrong) error = %v", err)
	}
	if _, err := manager.ValidateMutation(session.ID, session.CSRFToken); err != nil {
		t.Fatalf("ValidateMutation() error = %v", err)
	}
}

func TestExpiredCredentialsAreRejected(t *testing.T) {
	t.Parallel()

	manager := NewManager()
	now := time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC)
	manager.now = func() time.Time { return now }
	bootstrap, err := manager.IssueBootstrap()
	if err != nil {
		t.Fatalf("IssueBootstrap() error = %v", err)
	}
	now = now.Add(bootstrapLifetime + time.Second)
	if _, err := manager.Exchange(bootstrap.Token); !errors.Is(err, ErrInvalidBootstrap) {
		t.Fatalf("Exchange(expired) error = %v", err)
	}
}
