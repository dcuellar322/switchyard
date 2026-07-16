package application

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"switchyard.dev/switchyard/internal/terminal/domain"
)

func TestServiceOwnsProcessBeyondCreateRequestAndStreamsBoundedOutput(t *testing.T) {
	parent, cancelParent := context.WithCancel(context.Background())
	repository := newMemoryRepository()
	spawner := &fakeSpawner{}
	service, err := NewService(parent, repository, staticResolver{}, spawner, Config{ScrollbackBytes: 4 << 10, IdleTimeout: time.Hour, SubscriberQueue: 4})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cancelParent(); closeService(t, service) })

	requestCtx, cancelRequest := context.WithCancel(context.Background())
	session, err := service.Create(requestCtx, shellRequest(), testOwner())
	if err != nil {
		t.Fatal(err)
	}
	cancelRequest()
	process := spawner.last()
	if process.context().Err() != nil {
		t.Fatal("PTY inherited the completed request context")
	}

	payload := bytes.Repeat([]byte("x"), 6<<10)
	go process.emit(payload)
	waitFor(t, func() bool {
		item, getErr := service.Get(context.Background(), session.ID, testOwner())
		return getErr == nil && item.OutputBytes == int64(len(payload))
	})
	attachment, err := service.Attach(context.Background(), session.ID, testOwner())
	if err != nil {
		t.Fatal(err)
	}
	defer attachment.Close()
	if len(attachment.Snapshot) != 4<<10 {
		t.Fatalf("snapshot bytes = %d, want %d", len(attachment.Snapshot), 4<<10)
	}
	item, err := service.Get(context.Background(), session.ID, testOwner())
	if err != nil || !item.OutputTruncated {
		t.Fatalf("truncation metadata = %+v, %v", item, err)
	}
	if err := attachment.Write([]byte("echo hello\n")); err != nil {
		t.Fatal(err)
	}
	if got := process.inputString(); got != "echo hello\n" {
		t.Fatalf("process input = %q", got)
	}
	if err := attachment.Resize(domain.Size{Columns: 132, Rows: 43}); err != nil {
		t.Fatal(err)
	}
	if got := process.size(); got != (domain.Size{Columns: 132, Rows: 43}) {
		t.Fatalf("process size = %+v", got)
	}
}

func TestServiceEnforcesOwnershipAndDetachPersistence(t *testing.T) {
	service, spawner, cleanup := newTestService(t, time.Hour)
	defer cleanup()
	session, err := service.Create(context.Background(), shellRequest(), testOwner())
	if err != nil {
		t.Fatal(err)
	}
	other := domain.Owner{Type: "browser", ID: "other_session"}
	if _, err := service.Attach(context.Background(), session.ID, other); !errors.Is(err, ErrOwnerMismatch) {
		t.Fatalf("Attach() error = %v", err)
	}
	attachment, err := service.Attach(context.Background(), session.ID, testOwner())
	if err != nil {
		t.Fatal(err)
	}
	attachment.Close()
	if spawner.last().terminated() {
		t.Fatal("detaching terminated the process")
	}
	current, err := service.Get(context.Background(), session.ID, testOwner())
	if err != nil || current.DetachedAt == nil || current.Status != domain.StatusActive {
		t.Fatalf("detached session = %+v, %v", current, err)
	}
}

func TestServiceExpiresDetachedAndTerminatesExplicitly(t *testing.T) {
	service, spawner, cleanup := newTestService(t, time.Minute)
	defer cleanup()
	base := time.Date(2026, 7, 16, 10, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return base }
	first, err := service.Create(context.Background(), shellRequest(), testOwner())
	if err != nil {
		t.Fatal(err)
	}
	service.expireAt(base.Add(time.Minute + time.Second))
	waitFor(t, spawner.processes[0].terminated)
	waitFor(t, func() bool {
		item, getErr := service.Get(context.Background(), first.ID, testOwner())
		return getErr == nil && item.Status == domain.StatusExpired
	})

	second, err := service.Create(context.Background(), shellRequest(), testOwner())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Terminate(context.Background(), second.ID, testOwner()); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool {
		item, getErr := service.Get(context.Background(), second.ID, testOwner())
		return getErr == nil && item.Status == domain.StatusTerminated
	})
}

func TestServiceRecoversInterruptedMetadataAndRecordsStartFailure(t *testing.T) {
	repository := newMemoryRepository()
	repository.sessions["terminal_old"] = domain.Session{
		ID: "terminal_old", ProjectID: "project_one", Kind: domain.KindShell, DisplayName: "old", Owner: testOwner(),
		WorkingDirectory: "/tmp/project", Status: domain.StatusActive, PersistencePolicy: domain.PersistenceDetachUntilIdle,
		CapturePolicy: domain.CaptureUserVisibleOutput, CreatedAt: time.Now().UTC(),
	}
	service, err := NewService(context.Background(), repository, staticResolver{}, &fakeSpawner{startErr: errors.New("pty unavailable")}, DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { closeService(t, service) })
	if repository.sessions["terminal_old"].Status != domain.StatusInterrupted {
		t.Fatalf("stale status = %s", repository.sessions["terminal_old"].Status)
	}
	failed, err := service.Create(context.Background(), shellRequest(), testOwner())
	if err == nil {
		t.Fatal("Create() expected start failure")
	}
	if failed.Status != domain.StatusFailed || failed.ErrorCode != "TERMINAL_START_FAILED" {
		t.Fatalf("failed session = %+v", failed)
	}
}

func TestTerminalOutputFanoutRemainsBoundedWithSlowSubscribers(t *testing.T) {
	service, spawner, cleanup := newTestService(t, time.Hour)
	defer cleanup()
	session, err := service.Create(context.Background(), shellRequest(), testOwner())
	if err != nil {
		t.Fatal(err)
	}
	attachments := make([]*Attachment, 0, 128)
	for range 128 {
		attachment, attachErr := service.Attach(context.Background(), session.ID, testOwner())
		if attachErr != nil {
			t.Fatal(attachErr)
		}
		attachments = append(attachments, attachment)
	}
	t.Cleanup(func() {
		for _, attachment := range attachments {
			attachment.Close()
		}
	})
	payload := bytes.Repeat([]byte("load"), 64<<10)
	done := make(chan struct{})
	go func() { spawner.last().emit(payload); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("slow subscribers blocked PTY output")
	}
	waitFor(t, func() bool {
		item, getErr := service.Get(context.Background(), session.ID, testOwner())
		return getErr == nil && item.OutputBytes == int64(len(payload))
	})
	reconnected, err := service.Attach(context.Background(), session.ID, testOwner())
	if err != nil {
		t.Fatal(err)
	}
	defer reconnected.Close()
	if len(reconnected.Snapshot) > 4<<10 {
		t.Fatalf("scrollback grew to %d bytes", len(reconnected.Snapshot))
	}
}

type staticResolver struct{}

func (staticResolver) Resolve(_ context.Context, request domain.CreateRequest) (LaunchPlan, error) {
	return LaunchPlan{ProjectID: request.ProjectID, EnvironmentID: request.EnvironmentID, DisplayName: "Project shell", WorkingDirectory: "/tmp/project", Executable: "/bin/sh"}, nil
}

type fakeSpawner struct {
	mu        sync.Mutex
	processes []*fakeProcess
	startErr  error
}

func (s *fakeSpawner) Start(ctx context.Context, _ LaunchPlan, size domain.Size) (Process, error) {
	if s.startErr != nil {
		return nil, s.startErr
	}
	reader, writer := io.Pipe()
	process := &fakeProcess{ctx: ctx, reader: reader, writer: writer, done: make(chan struct{}), dimensions: size}
	s.mu.Lock()
	s.processes = append(s.processes, process)
	s.mu.Unlock()
	return process, nil
}

func (s *fakeSpawner) last() *fakeProcess {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.processes[len(s.processes)-1]
}

type fakeProcess struct {
	ctx        context.Context
	reader     *io.PipeReader
	writer     *io.PipeWriter
	done       chan struct{}
	once       sync.Once
	mu         sync.Mutex
	input      bytes.Buffer
	dimensions domain.Size
	wasKilled  bool
}

func (p *fakeProcess) Read(value []byte) (int, error) { return p.reader.Read(value) }
func (p *fakeProcess) Write(value []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.input.Write(value)
}
func (p *fakeProcess) Resize(size domain.Size) error {
	p.mu.Lock()
	p.dimensions = size
	p.mu.Unlock()
	return nil
}
func (p *fakeProcess) Terminate(context.Context) error { p.finish(true); return nil }
func (p *fakeProcess) Wait() error                     { <-p.done; return nil }
func (p *fakeProcess) PID() int                        { return 42 }
func (p *fakeProcess) Close() error                    { return p.reader.Close() }
func (p *fakeProcess) emit(value []byte)               { _, _ = p.writer.Write(value) }
func (p *fakeProcess) context() context.Context        { return p.ctx }
func (p *fakeProcess) inputString() string             { p.mu.Lock(); defer p.mu.Unlock(); return p.input.String() }
func (p *fakeProcess) size() domain.Size               { p.mu.Lock(); defer p.mu.Unlock(); return p.dimensions }
func (p *fakeProcess) terminated() bool                { p.mu.Lock(); defer p.mu.Unlock(); return p.wasKilled }
func (p *fakeProcess) finish(killed bool) {
	p.once.Do(func() {
		p.mu.Lock()
		p.wasKilled = killed
		p.mu.Unlock()
		_ = p.writer.Close()
		close(p.done)
	})
}

type memoryRepository struct {
	mu       sync.Mutex
	sessions map[string]domain.Session
	audits   []domain.Audit
}

func newMemoryRepository() *memoryRepository {
	return &memoryRepository{sessions: make(map[string]domain.Session)}
}
func (r *memoryRepository) Create(_ context.Context, session domain.Session) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions[session.ID] = session
	return nil
}
func (r *memoryRepository) Update(_ context.Context, session domain.Session) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions[session.ID] = session
	return nil
}
func (r *memoryRepository) Get(_ context.Context, id string) (domain.Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.sessions[id]
	if !ok {
		return domain.Session{}, ErrNotFound
	}
	return item, nil
}
func (r *memoryRepository) List(_ context.Context, projectID string) ([]domain.Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := []domain.Session{}
	for _, item := range r.sessions {
		if projectID == "" || item.ProjectID == projectID {
			result = append(result, item)
		}
	}
	return result, nil
}
func (r *memoryRepository) InterruptActive(_ context.Context, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, item := range r.sessions {
		if item.Active() {
			item.Status = domain.StatusInterrupted
			item.FinishedAt = &at
			item.ErrorCode = "DAEMON_RESTARTED"
			r.sessions[id] = item
		}
	}
	return nil
}
func (r *memoryRepository) AppendAudit(_ context.Context, audit domain.Audit) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.audits = append(r.audits, audit)
	return nil
}

func shellRequest() domain.CreateRequest {
	return domain.CreateRequest{ProjectID: "project_one", Kind: domain.KindShell, Columns: 80, Rows: 24}
}
func testOwner() domain.Owner { return domain.Owner{Type: "browser", ID: "session_one"} }

func newTestService(t *testing.T, idle time.Duration) (*Service, *fakeSpawner, func()) {
	t.Helper()
	spawner := &fakeSpawner{}
	service, err := NewService(context.Background(), newMemoryRepository(), staticResolver{}, spawner, Config{ScrollbackBytes: 4 << 10, IdleTimeout: idle, SubscriberQueue: 4})
	if err != nil {
		t.Fatal(err)
	}
	return service, spawner, func() { closeService(t, service) }
}

func closeService(t *testing.T, service *Service) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := service.Close(ctx); err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal("condition was not satisfied")
		}
		time.Sleep(time.Millisecond)
	}
}
