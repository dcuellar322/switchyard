// Package application manages short-lived browser bootstrap and session credentials.
package application

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"switchyard.dev/switchyard/internal/foundation/identifier"
)

var (
	// ErrInvalidBootstrap identifies an unknown, reused, or expired bootstrap token.
	ErrInvalidBootstrap = errors.New("invalid browser bootstrap token")
	// ErrInvalidSession identifies an unknown or expired browser session.
	ErrInvalidSession = errors.New("invalid browser session")
	// ErrInvalidCSRF identifies a missing or mismatched mutation token.
	ErrInvalidCSRF = errors.New("invalid csrf token")
)

const (
	bootstrapLifetime = time.Minute
	sessionLifetime   = 8 * time.Hour
)

// Bootstrap is a one-time credential delivered through privileged local IPC.
type Bootstrap struct {
	Token     string
	ExpiresAt time.Time
}

// Session is a same-origin browser session and its CSRF credential.
type Session struct {
	ID        string
	CSRFToken string
	ExpiresAt time.Time
}

// Manager owns in-memory browser credentials; daemon restart invalidates them.
type Manager struct {
	mu         sync.Mutex
	now        func() time.Time
	bootstraps map[string]time.Time
	sessions   map[string]Session
}

// NewManager creates an empty credential manager.
func NewManager() *Manager {
	return &Manager{
		now: time.Now, bootstraps: make(map[string]time.Time), sessions: make(map[string]Session),
	}
}

// IssueBootstrap creates a one-time token for a local IPC client.
func (m *Manager) IssueBootstrap() (Bootstrap, error) {
	token, err := identifier.New("boot")
	if err != nil {
		return Bootstrap{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reapLocked()
	expiresAt := m.now().UTC().Add(bootstrapLifetime)
	m.bootstraps[token] = expiresAt
	return Bootstrap{Token: token, ExpiresAt: expiresAt}, nil
}

// Exchange consumes a one-time bootstrap token and creates a browser session.
func (m *Manager) Exchange(token string) (Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reapLocked()
	expiresAt, ok := m.bootstraps[token]
	if !ok || !expiresAt.After(m.now()) {
		return Session{}, ErrInvalidBootstrap
	}
	delete(m.bootstraps, token)
	sessionID, err := identifier.New("session")
	if err != nil {
		return Session{}, fmt.Errorf("create browser session: %w", err)
	}
	csrfToken, err := identifier.New("csrf")
	if err != nil {
		return Session{}, fmt.Errorf("create csrf token: %w", err)
	}
	session := Session{ID: sessionID, CSRFToken: csrfToken, ExpiresAt: m.now().UTC().Add(sessionLifetime)}
	m.sessions[session.ID] = session
	return session, nil
}

// ValidateSession checks an opaque session credential.
func (m *Manager) ValidateSession(id string) (Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reapLocked()
	session, ok := m.sessions[id]
	if !ok {
		return Session{}, ErrInvalidSession
	}
	return session, nil
}

// ValidateMutation checks both session and CSRF credentials.
func (m *Manager) ValidateMutation(id, csrfToken string) (Session, error) {
	session, err := m.ValidateSession(id)
	if err != nil {
		return Session{}, err
	}
	if csrfToken == "" || csrfToken != session.CSRFToken {
		return Session{}, ErrInvalidCSRF
	}
	return session, nil
}

func (m *Manager) reapLocked() {
	now := m.now()
	for token, expiresAt := range m.bootstraps {
		if !expiresAt.After(now) {
			delete(m.bootstraps, token)
		}
	}
	for id, session := range m.sessions {
		if !session.ExpiresAt.After(now) {
			delete(m.sessions, id)
		}
	}
}
