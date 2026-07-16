package application

import (
	"context"
	"sync"
	"time"

	"switchyard.dev/switchyard/internal/terminal/domain"
)

type managedSession struct {
	mu             sync.Mutex
	session        domain.Session
	process        Process
	scrollback     *byteRing
	subscribers    map[uint64]chan []byte
	nextSubscriber uint64
	attachments    int
	requestedEnd   domain.Status
}

func newManagedSession(session domain.Session, process Process, scrollbackBytes int) *managedSession {
	return &managedSession{session: session, process: process, scrollback: newByteRing(scrollbackBytes), subscribers: make(map[uint64]chan []byte)}
}

func (s *managedSession) snapshot() domain.Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.session
}

func (s *managedSession) attach(queue int, at time.Time) (uint64, <-chan []byte, []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextSubscriber++
	id := s.nextSubscriber
	stream := make(chan []byte, queue)
	s.subscribers[id] = stream
	s.attachments++
	s.session.LastAttachedAt = timePointer(at)
	s.session.DetachedAt = nil
	snapshot, _ := s.scrollback.Snapshot()
	return id, stream, snapshot
}

func (s *managedSession) detach(id uint64, at time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	stream, exists := s.subscribers[id]
	if !exists {
		return false
	}
	delete(s.subscribers, id)
	close(stream)
	s.attachments--
	if s.attachments == 0 {
		s.session.DetachedAt = timePointer(at)
	}
	return true
}

func (s *managedSession) appendOutput(value []byte, at time.Time) {
	s.scrollback.Write(value)
	_, truncated := s.scrollback.Snapshot()
	s.mu.Lock()
	s.session.OutputBytes += int64(len(value))
	s.session.OutputTruncated = truncated
	s.session.LastOutputAt = timePointer(at)
	for id, stream := range s.subscribers {
		copyValue := append([]byte(nil), value...)
		select {
		case stream <- copyValue:
		default:
			close(stream)
			delete(s.subscribers, id)
			s.attachments--
		}
	}
	if s.attachments == 0 && s.session.DetachedAt == nil {
		s.session.DetachedAt = timePointer(at)
	}
	s.mu.Unlock()
}

func (s *managedSession) setEndStatus(status domain.Status) {
	s.mu.Lock()
	if s.requestedEnd == "" {
		s.requestedEnd = status
	}
	s.mu.Unlock()
}

func (s *managedSession) endStatus() domain.Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.requestedEnd
}

func (s *managedSession) setErrorCode(code string) {
	s.mu.Lock()
	s.session.ErrorCode = code
	s.mu.Unlock()
}

func (s *managedSession) finish(status domain.Status, exitCode int, at time.Time) {
	s.mu.Lock()
	s.session.Status = status
	s.session.ExitCode = &exitCode
	s.session.FinishedAt = timePointer(at)
	for id, stream := range s.subscribers {
		close(stream)
		delete(s.subscribers, id)
	}
	s.attachments = 0
	s.mu.Unlock()
}

func (s *managedSession) detachedFor(now time.Time) time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.session.Active() || s.attachments > 0 || s.session.DetachedAt == nil {
		return 0
	}
	return now.Sub(*s.session.DetachedAt)
}

// Attachment is an owner-authorized bidirectional view of one live PTY.
type Attachment struct {
	service      *Service
	managed      *managedSession
	subscriberID uint64
	owner        domain.Owner
	once         sync.Once
	Snapshot     []byte
	Output       <-chan []byte
}

// SnapshotBytes returns an isolated reconnect scrollback snapshot.
func (a *Attachment) SnapshotBytes() []byte { return append([]byte(nil), a.Snapshot...) }

// OutputBytes returns the bounded live-output subscription.
func (a *Attachment) OutputBytes() <-chan []byte { return a.Output }

// Write forwards bounded terminal input bytes.
func (a *Attachment) Write(value []byte) error {
	if len(value) == 0 {
		return nil
	}
	if len(value) > 64<<10 {
		return ErrLaunchInvalid
	}
	_, err := a.managed.process.Write(value)
	return err
}

// Resize updates the PTY grid and records only dimensions in the audit trail.
func (a *Attachment) Resize(size domain.Size) error {
	if err := size.Validate(); err != nil {
		return err
	}
	if err := a.managed.process.Resize(size); err != nil {
		return err
	}
	return a.service.audit(context.Background(), a.managed.snapshot().ID, "resized", a.owner, map[string]any{"columns": size.Columns, "rows": size.Rows})
}

// Close detaches the client while preserving the PTY until its idle deadline.
func (a *Attachment) Close() {
	a.once.Do(func() {
		at := a.service.now().UTC()
		if a.managed.detach(a.subscriberID, at) {
			_ = a.service.repository.Update(context.Background(), a.managed.snapshot())
			_ = a.service.audit(context.Background(), a.managed.snapshot().ID, "detached", a.owner, nil)
		}
	})
}
