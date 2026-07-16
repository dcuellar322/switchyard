// Package process implements the native host-process runtime driver.
package process

import (
	"context"
	"errors"
	"sync"
	"time"

	"switchyard.dev/switchyard/internal/runtime/domain"
)

const (
	defaultStopTimeout   = 10 * time.Second
	defaultLogCapacity   = 10_000
	identityHandoffGrace = 2 * time.Second
)

var (
	// ErrInvalidProcessPlan identifies driver data that was not produced by this driver.
	ErrInvalidProcessPlan = errors.New("invalid native process plan")
	// ErrExternalProcess prevents silently launching over a matching external service.
	ErrExternalProcess = errors.New("service appears to be running externally")
)

// RunStore persists PID-reuse-resistant run ownership evidence.
type RunStore interface {
	CreateRun(context.Context, domain.RunRecord) error
	RecordProcess(context.Context, domain.ProcessIdentity) error
	FinishRun(context.Context, string, time.Time, *int, string) error
	SetRestartCount(context.Context, string, int) error
	ListProjectRuns(context.Context, string) ([]domain.RunRecord, error)
}

// SecretResolver obtains explicit credential-store references at launch time.
type SecretResolver interface {
	Resolve(context.Context, domain.SecretReference) (string, error)
}

// SecretObserver receives resolved values only in memory so downstream redaction can recognize them.
type SecretObserver interface {
	AddSecret(string)
}

// Driver supervises native process groups and reconciles them against durable fingerprints.
type Driver struct {
	ctx            context.Context
	store          RunStore
	inspector      processInspector
	secrets        SecretResolver
	secretObserver SecretObserver
	now            func() time.Time

	mu          sync.RWMutex
	managed     map[string]*managedRun
	logs        map[string]*logBuffer
	subscribers map[string]map[chan domain.RuntimeEvent]struct{}
}

// NewDriver creates a production native-process driver scoped to the daemon lifecycle.
func NewDriver(ctx context.Context, store RunStore, observers ...SecretObserver) *Driver {
	driver := newDriver(ctx, store, gopsutilInspector{}, keychainResolver{})
	if len(observers) > 0 {
		driver.secretObserver = observers[0]
	}
	return driver
}

func newDriver(ctx context.Context, store RunStore, inspector processInspector, secrets SecretResolver) *Driver {
	return &Driver{
		ctx: ctx, store: store, inspector: inspector, secrets: secrets, now: time.Now,
		managed: make(map[string]*managedRun), logs: make(map[string]*logBuffer),
		subscribers: make(map[string]map[chan domain.RuntimeEvent]struct{}),
	}
}

// Kind identifies the driver contract.
func (*Driver) Kind() domain.Kind { return domain.KindProcess }

// Plan produces a side-effect-free native process lifecycle preview.
func (d *Driver) Plan(_ context.Context, request domain.PlanRequest) (domain.Plan, error) {
	return buildPlan(request)
}

// Execute applies a validated native process lifecycle plan.
func (d *Driver) Execute(ctx context.Context, plan domain.Plan, sink domain.ProgressSink) error {
	data, ok := plan.DriverData.(executionPlan)
	if !ok || plan.Driver != domain.KindProcess || data.project.ProjectID != plan.ProjectID || data.action != plan.Action {
		return ErrInvalidProcessPlan
	}
	for index := range data.services {
		data.services[index].operationID = plan.OperationID
	}
	if err := sink.Step(ctx, "process.preview", "succeeded", plan.Summary); err != nil {
		return err
	}
	switch plan.Action {
	case domain.ActionStart:
		return d.startAll(ctx, data, sink)
	case domain.ActionStop:
		return d.stopAll(ctx, data, sink, "stopped")
	case domain.ActionRestart:
		stopPlan := data
		stopPlan.services = append([]servicePlan(nil), data.services...)
		reverseServices(stopPlan.services)
		if err := d.stopAll(ctx, stopPlan, sink, "restarted"); err != nil {
			return err
		}
		return d.startAll(ctx, data, sink)
	case domain.ActionPause, domain.ActionUnpause, domain.ActionRebuild, domain.ActionTeardown:
		return ErrInvalidProcessPlan
	}
	return ErrInvalidProcessPlan
}

// Inspect reconciles durable run fingerprints and honestly classified external listeners.
func (d *Driver) Inspect(ctx context.Context, project domain.ProjectRuntime) (domain.Observation, error) {
	return d.inspect(ctx, project)
}

// StreamLogs reads bounded captured stdout/stderr without exposing environment values.
func (d *Driver) StreamLogs(ctx context.Context, request domain.LogRequest, sink domain.LogSink) error {
	return d.streamLogs(ctx, request, sink)
}

// StreamMetrics samples verified managed process identities.
func (d *Driver) StreamMetrics(ctx context.Context, request domain.MetricRequest, sink domain.MetricSink) error {
	return d.streamMetrics(ctx, request, sink)
}

// WatchEvents publishes targeted native lifecycle and periodic reconciliation events.
func (d *Driver) WatchEvents(ctx context.Context, project domain.ProjectRuntime, sink domain.EventSink) error {
	return d.watchEvents(ctx, project, sink)
}

func serviceKey(projectID, serviceID string) string { return projectID + "\x00" + serviceID }
