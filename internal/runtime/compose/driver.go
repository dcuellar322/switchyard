// Package compose implements the Docker Compose runtime driver.
package compose

import (
	"context"

	"switchyard.dev/switchyard/internal/runtime/domain"
)

// Driver uses the Compose CLI for lifecycle and the Engine SDK for observation.
type Driver struct {
	config   configReader
	builder  commandBuilder
	executor executor
	engine   engineConnector
	managed  *managedContainers
}

// NewDriver creates a production Compose driver using the installed Docker CLI.
func NewDriver() *Driver {
	runner := osCommandRunner{}
	managed := newManagedContainers()
	return &Driver{
		config:  configReader{runner: runner, contexts: contextResolver{runner: runner}},
		builder: commandBuilder{}, executor: executor{runner: runner, managed: managed},
		engine: engineFactory{}, managed: managed,
	}
}

// Kind identifies the driver contract.
func (*Driver) Kind() domain.Kind { return domain.KindCompose }

// Plan normalizes Compose config and produces an argument-array preview.
func (d *Driver) Plan(ctx context.Context, request domain.PlanRequest) (domain.Plan, error) {
	config, err := d.config.Normalize(ctx, request.Project)
	if err != nil {
		return domain.Plan{}, err
	}
	return d.builder.Build(request, config)
}

// Execute runs a validated Compose CLI plan.
func (d *Driver) Execute(ctx context.Context, plan domain.Plan, sink domain.ProgressSink) error {
	return d.executor.Execute(ctx, plan, sink)
}

// Inspect derives project state from canonical Compose labels and Engine metadata.
func (d *Driver) Inspect(ctx context.Context, project domain.ProjectRuntime) (domain.Observation, error) {
	config, err := d.config.Normalize(ctx, project)
	if err != nil {
		return disconnectedObservation(project, normalizedConfig{}, err), nil
	}
	return d.inspect(ctx, project, config)
}

// StreamLogs reads container output through the Engine SDK.
func (d *Driver) StreamLogs(ctx context.Context, request domain.LogRequest, sink domain.LogSink) error {
	config, err := d.config.Normalize(ctx, request.Project)
	if err != nil {
		return err
	}
	return d.streamLogs(ctx, request, config, operationLogSink{operationID: d.managed.Operation(config.ProjectName), sink: sink})
}

type operationLogSink struct {
	operationID string
	sink        domain.LogSink
}

func (s operationLogSink) WriteLog(ctx context.Context, entry domain.LogEntry) error {
	if entry.OperationID == "" {
		entry.OperationID = s.operationID
	}
	return s.sink.WriteLog(ctx, entry)
}

// StreamMetrics reads current container stats through the Engine SDK.
func (d *Driver) StreamMetrics(ctx context.Context, request domain.MetricRequest, sink domain.MetricSink) error {
	config, err := d.config.Normalize(ctx, request.Project)
	if err != nil {
		return err
	}
	return d.streamMetrics(ctx, request, config, sink)
}

// WatchEvents subscribes to Engine events filtered by canonical project labels.
func (d *Driver) WatchEvents(ctx context.Context, project domain.ProjectRuntime, sink domain.EventSink) error {
	config, err := d.config.Normalize(ctx, project)
	if err != nil {
		return err
	}
	return d.watchEvents(ctx, config, sink)
}
