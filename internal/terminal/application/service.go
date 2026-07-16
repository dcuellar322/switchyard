package application

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sync"
	"time"

	"switchyard.dev/switchyard/internal/foundation/identifier"
	"switchyard.dev/switchyard/internal/terminal/domain"
)

var (
	// ErrNotFound identifies an unknown terminal session.
	ErrNotFound = errors.New("terminal session not found")
	// ErrOwnerMismatch prevents another browser or local client from attaching.
	ErrOwnerMismatch = errors.New("terminal session belongs to another owner")
	// ErrNotActive identifies a durable record without a live daemon-owned PTY.
	ErrNotActive = errors.New("terminal session is not active")
	// ErrLaunchInvalid rejects an adapter plan that escapes the typed boundary.
	ErrLaunchInvalid = errors.New("terminal launch plan is invalid")
)

// Service owns live PTYs, bounded scrollback, durable metadata, and security audits.
type Service struct {
	repository Repository
	resolver   LaunchResolver
	spawner    Spawner
	config     Config
	now        func() time.Time
	ctx        context.Context
	cancel     context.CancelFunc

	mu       sync.RWMutex
	sessions map[string]*managedSession
	wg       sync.WaitGroup
}

// NewService recovers stale metadata and starts detached-session expiration.
func NewService(parent context.Context, repository Repository, resolver LaunchResolver, spawner Spawner, config Config) (*Service, error) {
	if config.ScrollbackBytes < 4<<10 || config.IdleTimeout <= 0 || config.SubscriberQueue < 1 {
		return nil, errors.New("terminal service bounds are invalid")
	}
	ctx, cancel := context.WithCancel(parent)
	service := &Service{
		repository: repository, resolver: resolver, spawner: spawner, config: config,
		now: time.Now, ctx: ctx, cancel: cancel, sessions: make(map[string]*managedSession),
	}
	if err := repository.InterruptActive(parent, service.now().UTC()); err != nil {
		cancel()
		return nil, fmt.Errorf("recover terminal sessions: %w", err)
	}
	service.wg.Add(1)
	go service.expireDetached()
	return service, nil
}

// Create persists intent before spawning one typed interactive process.
func (s *Service) Create(ctx context.Context, request domain.CreateRequest, owner domain.Owner) (domain.Session, error) {
	if err := request.Validate(); err != nil {
		return domain.Session{}, err
	}
	if err := owner.Validate(); err != nil {
		return domain.Session{}, err
	}
	plan, err := s.resolver.Resolve(ctx, request)
	if err != nil {
		return domain.Session{}, err
	}
	if err := validateLaunchPlan(request, plan); err != nil {
		return domain.Session{}, err
	}
	id, err := identifier.New("terminal")
	if err != nil {
		return domain.Session{}, fmt.Errorf("create terminal session ID: %w", err)
	}
	now := s.now().UTC()
	session := domain.Session{
		ID: id, ProjectID: plan.ProjectID, EnvironmentID: plan.EnvironmentID, Kind: request.Kind,
		DisplayName: plan.DisplayName, Owner: owner, Provider: plan.Provider, ServiceID: plan.ServiceID, ActionID: plan.ActionID,
		WorkingDirectory: plan.WorkingDirectory, Status: domain.StatusStarting,
		PersistencePolicy: domain.PersistenceDetachUntilIdle, CapturePolicy: domain.CaptureUserVisibleOutput,
		CreatedAt: now, DetachedAt: timePointer(now),
	}
	if err := session.Validate(); err != nil {
		return domain.Session{}, fmt.Errorf("%w: %v", ErrLaunchInvalid, err)
	}
	if err := s.repository.Create(ctx, session); err != nil {
		return domain.Session{}, fmt.Errorf("persist terminal session: %w", err)
	}
	if err := s.audit(ctx, session.ID, "created", owner, map[string]any{"kind": request.Kind}); err != nil {
		return domain.Session{}, err
	}
	process, err := s.spawner.Start(s.ctx, plan, request.Size())
	if err != nil {
		finished := s.now().UTC()
		session.Status, session.FinishedAt, session.ErrorCode = domain.StatusFailed, &finished, "TERMINAL_START_FAILED"
		_ = s.repository.Update(context.WithoutCancel(ctx), session)
		_ = s.audit(context.WithoutCancel(ctx), session.ID, "start_failed", owner, nil)
		return session, fmt.Errorf("start terminal PTY: %w", err)
	}
	managed := newManagedSession(session, process, s.config.ScrollbackBytes)
	managed.session.Status = domain.StatusActive
	s.mu.Lock()
	s.sessions[id] = managed
	s.mu.Unlock()
	if err := s.repository.Update(ctx, managed.snapshot()); err != nil {
		_ = process.Terminate(context.Background())
		return domain.Session{}, fmt.Errorf("activate terminal session: %w", err)
	}
	_ = s.audit(ctx, session.ID, "started", owner, map[string]any{"pid": process.PID()})
	s.wg.Add(2)
	go s.pump(managed)
	go s.wait(managed)
	return managed.snapshot(), nil
}

// List returns only records owned by the authenticated caller.
func (s *Service) List(ctx context.Context, projectID string, owner domain.Owner) ([]domain.Session, error) {
	if err := owner.Validate(); err != nil {
		return nil, err
	}
	items, err := s.repository.List(ctx, projectID)
	if err != nil {
		return nil, err
	}
	result := make([]domain.Session, 0, len(items))
	for _, item := range items {
		s.mu.RLock()
		managed := s.sessions[item.ID]
		s.mu.RUnlock()
		if managed != nil {
			item = managed.snapshot()
		}
		if item.Owner == owner {
			result = append(result, item)
		}
	}
	return result, nil
}

// Get returns one owner-authorized durable record.
func (s *Service) Get(ctx context.Context, sessionID string, owner domain.Owner) (domain.Session, error) {
	s.mu.RLock()
	managed := s.sessions[sessionID]
	s.mu.RUnlock()
	if managed != nil {
		session := managed.snapshot()
		if session.Owner != owner {
			return domain.Session{}, ErrOwnerMismatch
		}
		return session, nil
	}
	session, err := s.repository.Get(ctx, sessionID)
	if err != nil {
		return domain.Session{}, err
	}
	if session.Owner != owner {
		return domain.Session{}, ErrOwnerMismatch
	}
	return session, nil
}

// Attach returns bounded current scrollback and a non-blocking live stream.
func (s *Service) Attach(ctx context.Context, sessionID string, owner domain.Owner) (*Attachment, error) {
	managed, err := s.ownedActive(sessionID, owner)
	if err != nil {
		return nil, err
	}
	id, output, snapshot := managed.attach(s.config.SubscriberQueue, s.now().UTC())
	if err := s.repository.Update(ctx, managed.snapshot()); err != nil {
		managed.detach(id, s.now().UTC())
		return nil, err
	}
	_ = s.audit(ctx, sessionID, "attached", owner, map[string]any{"scrollbackBytes": len(snapshot)})
	return &Attachment{service: s, managed: managed, subscriberID: id, owner: owner, Snapshot: snapshot, Output: output}, nil
}

// Terminate explicitly ends an owner-authorized PTY process group.
func (s *Service) Terminate(ctx context.Context, sessionID string, owner domain.Owner) (domain.Session, error) {
	managed, err := s.ownedActive(sessionID, owner)
	if err != nil {
		return domain.Session{}, err
	}
	managed.setEndStatus(domain.StatusTerminated)
	if err := s.audit(ctx, sessionID, "termination_requested", owner, nil); err != nil {
		return domain.Session{}, err
	}
	if err := managed.process.Terminate(ctx); err != nil {
		return domain.Session{}, fmt.Errorf("terminate terminal session: %w", err)
	}
	return managed.snapshot(), nil
}

// Close interrupts every live session; PTYs are not resumable across daemon restarts.
func (s *Service) Close(ctx context.Context) error {
	s.cancel()
	s.mu.RLock()
	active := make([]*managedSession, 0, len(s.sessions))
	for _, managed := range s.sessions {
		active = append(active, managed)
	}
	s.mu.RUnlock()
	for _, managed := range active {
		managed.setEndStatus(domain.StatusInterrupted)
		_ = managed.process.Terminate(ctx)
	}
	done := make(chan struct{})
	go func() { s.wg.Wait(); close(done) }()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func validateLaunchPlan(request domain.CreateRequest, plan LaunchPlan) error {
	if plan.ProjectID != request.ProjectID || plan.Executable == "" || !filepath.IsAbs(plan.WorkingDirectory) || plan.DisplayName == "" {
		return ErrLaunchInvalid
	}
	if request.EnvironmentID != plan.EnvironmentID {
		return ErrLaunchInvalid
	}
	return nil
}

func (s *Service) ownedActive(sessionID string, owner domain.Owner) (*managedSession, error) {
	s.mu.RLock()
	managed := s.sessions[sessionID]
	s.mu.RUnlock()
	if managed == nil {
		session, err := s.repository.Get(context.Background(), sessionID)
		if err != nil {
			return nil, err
		}
		if session.Owner != owner {
			return nil, ErrOwnerMismatch
		}
		return nil, ErrNotActive
	}
	if managed.snapshot().Owner != owner {
		return nil, ErrOwnerMismatch
	}
	if !managed.snapshot().Active() {
		return nil, ErrNotActive
	}
	return managed, nil
}

func (s *Service) pump(managed *managedSession) {
	defer s.wg.Done()
	buffer := make([]byte, 32<<10)
	for {
		count, err := managed.process.Read(buffer)
		if count > 0 {
			managed.appendOutput(buffer[:count], s.now().UTC())
		}
		if err != nil {
			if !errors.Is(err, io.EOF) && managed.snapshot().Active() {
				managed.setErrorCode("TERMINAL_STREAM_FAILED")
				terminateCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				_ = managed.process.Terminate(terminateCtx)
				cancel()
			}
			return
		}
	}
}

func (s *Service) wait(managed *managedSession) {
	defer s.wg.Done()
	err := managed.process.Wait()
	_ = managed.process.Close()
	finished := s.now().UTC()
	status := managed.endStatus()
	if status == "" {
		status = domain.StatusExited
		if s.ctx.Err() != nil {
			status = domain.StatusInterrupted
		}
	}
	exitCode := 0
	var exitErr interface{ ExitCode() int }
	if errors.As(err, &exitErr) {
		exitCode = exitErr.ExitCode()
	} else if err != nil {
		status = domain.StatusFailed
		managed.setErrorCode("TERMINAL_WAIT_FAILED")
	}
	managed.finish(status, exitCode, finished)
	_ = s.repository.Update(context.Background(), managed.snapshot())
	_ = s.audit(context.Background(), managed.snapshot().ID, string(status), managed.snapshot().Owner, map[string]any{"exitCode": exitCode})
	s.mu.Lock()
	delete(s.sessions, managed.snapshot().ID)
	s.mu.Unlock()
}

func (s *Service) expireDetached() {
	defer s.wg.Done()
	interval := min(s.config.IdleTimeout/4, time.Minute)
	if interval < time.Second {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.expireAt(s.now().UTC())
		}
	}
}

func (s *Service) expireAt(now time.Time) {
	s.mu.RLock()
	items := make([]*managedSession, 0, len(s.sessions))
	for _, item := range s.sessions {
		items = append(items, item)
	}
	s.mu.RUnlock()
	for _, item := range items {
		if item.detachedFor(now) >= s.config.IdleTimeout {
			item.setEndStatus(domain.StatusExpired)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			_ = item.process.Terminate(ctx)
			cancel()
		}
	}
}

func (s *Service) audit(ctx context.Context, sessionID, event string, actor domain.Owner, detail map[string]any) error {
	id, err := identifier.New("terminalaudit")
	if err != nil {
		return err
	}
	if detail == nil {
		detail = map[string]any{}
	}
	return s.repository.AppendAudit(ctx, domain.Audit{ID: id, SessionID: sessionID, Event: event, Actor: actor, Detail: detail, OccurredAt: s.now().UTC()})
}

func timePointer(value time.Time) *time.Time { return &value }
