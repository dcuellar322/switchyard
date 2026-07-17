// Package domain owns runtime plans and observations without depending on a concrete driver.
package domain

import (
	"context"
	"errors"
	"time"
)

// Kind identifies a runtime driver.
type Kind string

const (
	// KindCompose selects Docker Compose.
	KindCompose Kind = "compose"
	// KindProcess selects native host processes.
	KindProcess Kind = "process"
)

// Action is a standard runtime lifecycle mutation.
type Action string

const (
	// ActionStart creates or starts services.
	ActionStart Action = "start"
	// ActionStop stops services while preserving containers and volumes.
	ActionStop Action = "stop"
	// ActionRestart restarts existing services.
	ActionRestart Action = "restart"
	// ActionPause suspends container processes.
	ActionPause Action = "pause"
	// ActionUnpause resumes paused container processes.
	ActionUnpause Action = "unpause"
	// ActionRebuild rebuilds images and recreates services.
	ActionRebuild Action = "rebuild"
	// ActionTeardown removes Compose containers and networks.
	ActionTeardown Action = "teardown"
)

// Risk classifies the effects a user must review before execution.
type Risk string

const (
	// RiskSafe has no expected destructive effect.
	RiskSafe Risk = "safe"
	// RiskCaution interrupts or recreates local services.
	RiskCaution Risk = "caution"
	// RiskDestructive removes runtime resources.
	RiskDestructive Risk = "destructive"
)

// ProjectRuntime is the trusted, resolved input supplied to a driver.
type ProjectRuntime struct {
	ProjectID    string
	ProjectSlug  string
	Root         string
	Kind         Kind
	Compose      *ComposeRuntime
	Process      *ProcessRuntime
	Services     []ServiceDeclaration
	Ports        map[string]PortDeclaration
	ManifestHash string
}

// ProcessRuntime contains resolved native process declarations.
type ProcessRuntime struct {
	Environment map[string]string
	Secrets     map[string]SecretReference
	Processes   []ProcessDefinition
}

// ProcessDefinition is one executable process template referenced by a service.
type ProcessDefinition struct {
	ID                 string
	Command            []string
	WorkingDirectory   string
	Shell              bool
	Environment        map[string]string
	Secrets            map[string]SecretReference
	Restart            RestartPolicy
	StopTimeoutSeconds int
}

// SecretReference identifies a credential-store value without carrying the secret.
type SecretReference struct {
	Provider string
	Key      string
	Account  string
}

// RestartPolicy is an explicit bounded crash-restart policy.
type RestartPolicy struct {
	Mode           string
	MaxRetries     int
	BackoffSeconds int
}

// ComposeRuntime identifies Compose inputs and Docker context.
type ComposeRuntime struct {
	Files         []string
	ProjectName   string
	Context       string
	Profiles      []string
	PortOverrides map[string]int
}

// ServiceDeclaration maps a product service to a driver-native service.
type ServiceDeclaration struct {
	ID           string
	RuntimeName  string
	Dependencies []string
	HostPorts    []int
	HealthChecks []HealthCheckDefinition
}

// PortDeclaration is a trusted manifest port available to diagnostics.
type PortDeclaration struct {
	ID       string
	Service  string
	Host     int
	Target   int
	Protocol string
}

// HealthCheckDefinition is the driver-neutral readiness input resolved from a trusted manifest.
type HealthCheckDefinition struct {
	ID                  string
	ServiceID           string
	Type                string
	URL                 string
	Address             string
	ExpectedStatus      int
	JSONPath            string
	ExpectedValue       string
	Command             []string
	Members             []string
	Mode                string
	InitialDelaySeconds int
	IntervalSeconds     int
	TimeoutSeconds      int
	Retries             int
	Severity            string
	Required            bool
}

// PlanRequest asks a driver to preview an action.
type PlanRequest struct {
	Project       ProjectRuntime
	Action        Action
	RemoveVolumes bool
	Services      []string
	Profiles      []string
}

// Plan is an immutable, reviewable description of a lifecycle mutation.
type Plan struct {
	OperationID   string    `json:"-"`
	ProjectID     string    `json:"projectId"`
	Driver        Kind      `json:"driver"`
	Action        Action    `json:"action"`
	Risk          Risk      `json:"risk"`
	Summary       string    `json:"summary"`
	Commands      []Command `json:"commands"`
	Effects       []string  `json:"effects"`
	Services      []string  `json:"services"`
	Profiles      []string  `json:"profiles,omitempty"`
	RemoveVolumes bool      `json:"removeVolumes"`
	DriverData    any       `json:"-"`
}

// Command is a shell-free command preview.
type Command struct {
	Executable       string   `json:"executable"`
	Arguments        []string `json:"arguments"`
	WorkingDirectory string   `json:"workingDirectory"`
}

// ProjectState is derived from current observations.
type ProjectState string

const (
	// StateUnknown means the runtime cannot currently be observed.
	StateUnknown ProjectState = "unknown"
	// StateStopped means no declared services are running.
	StateStopped ProjectState = "stopped"
	// StateStarting means services are running but not yet ready.
	StateStarting ProjectState = "starting"
	// StateRunning means all declared services are managed and running.
	StateRunning ProjectState = "running"
	// StateRunningExternal means all services run without proven Switchyard ownership.
	StateRunningExternal ProjectState = "running_external"
	// StatePartiallyRunning means only part of the declared topology is running.
	StatePartiallyRunning ProjectState = "partially_running"
	// StateDegraded means a running service is unhealthy.
	StateDegraded ProjectState = "degraded"
	// StatePaused means all observed services are paused.
	StatePaused ProjectState = "paused"
	// StateStopping means runtime resources are being removed.
	StateStopping ProjectState = "stopping"
	// StateFailed means stopped services include a nonzero exit.
	StateFailed ProjectState = "failed"
)

// Origin explains whether Switchyard can prove ownership of the observed runtime.
type Origin string

const (
	// OriginSwitchyard means this daemon session initiated the current containers.
	OriginSwitchyard Origin = "switchyard"
	// OriginExternal means Switchyard cannot prove ownership of the current containers.
	OriginExternal Origin = "external"
)

// Observation is a point-in-time project runtime snapshot.
type Observation struct {
	ProjectID         string               `json:"projectId"`
	Driver            Kind                 `json:"driver"`
	ProjectIdentity   string               `json:"projectIdentity"`
	State             ProjectState         `json:"state"`
	Origin            Origin               `json:"origin"`
	Engine            *EngineObservation   `json:"engine,omitempty"`
	Services          []ServiceObservation `json:"services"`
	AvailableProfiles []string             `json:"availableProfiles,omitempty"`
	ObservedAt        time.Time            `json:"observedAt"`
}

// EngineObservation reports a bounded Docker connection summary.
type EngineObservation struct {
	Connected     bool   `json:"connected"`
	Context       string `json:"context,omitempty"`
	ServerVersion string `json:"serverVersion,omitempty"`
	APIVersion    string `json:"apiVersion,omitempty"`
	ErrorCode     string `json:"errorCode,omitempty"`
	ErrorMessage  string `json:"errorMessage,omitempty"`
}

// ServiceObservation is a product-level view of one Compose service/container.
type ServiceObservation struct {
	ID          string             `json:"id"`
	RuntimeName string             `json:"runtimeName"`
	State       string             `json:"state"`
	Health      string             `json:"health"`
	Container   *ContainerMetadata `json:"container,omitempty"`
	Process     *ProcessMetadata   `json:"process,omitempty"`
	Ports       []PublishedPort    `json:"ports"`
	ObservedAt  time.Time          `json:"observedAt"`
}

// ProcessMetadata exposes verified process identity without environment values.
type ProcessMetadata struct {
	RunID            string     `json:"runId,omitempty"`
	PID              int32      `json:"pid"`
	ProcessGroup     int32      `json:"processGroup,omitempty"`
	Executable       string     `json:"executable"`
	WorkingDirectory string     `json:"workingDirectory,omitempty"`
	StartedAt        *time.Time `json:"startedAt,omitempty"`
	FinishedAt       *time.Time `json:"finishedAt,omitempty"`
	ExitCode         *int       `json:"exitCode,omitempty"`
	RestartCount     int        `json:"restartCount"`
	Fingerprint      string     `json:"fingerprint,omitempty"`
}

// ContainerMetadata exposes useful, non-secret container facts.
type ContainerMetadata struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	Image        string     `json:"image"`
	CreatedAt    time.Time  `json:"createdAt,omitempty"`
	StartedAt    *time.Time `json:"startedAt,omitempty"`
	FinishedAt   *time.Time `json:"finishedAt,omitempty"`
	ExitCode     *int       `json:"exitCode,omitempty"`
	OOMKilled    bool       `json:"oomKilled,omitempty"`
	RestartCount int        `json:"restartCount"`
}

// PublishedPort is an observed Engine binding.
type PublishedPort struct {
	HostIP        string `json:"hostIp,omitempty"`
	HostPort      int    `json:"hostPort,omitempty"`
	ContainerPort int    `json:"containerPort"`
	Protocol      string `json:"protocol"`
}

// LogRequest selects a bounded or followed runtime stream.
type LogRequest struct {
	Project ProjectRuntime
	Service string
	Since   string
	Tail    int
	Follow  bool
}

// LogEntry is an immutable line retaining driver, stream, service, and run identity.
type LogEntry struct {
	Sequence    int64             `json:"sequence,omitempty"`
	Timestamp   time.Time         `json:"timestamp"`
	ProjectID   string            `json:"projectId"`
	ServiceID   string            `json:"serviceId"`
	RunID       string            `json:"runId"`
	Source      string            `json:"source"`
	Stream      string            `json:"stream"`
	Level       string            `json:"level"`
	Message     string            `json:"message"`
	OperationID string            `json:"operationId,omitempty"`
	Redacted    bool              `json:"redacted,omitempty"`
	Attributes  map[string]string `json:"attributes"`
}

// MetricRequest selects live container measurements.
type MetricRequest struct {
	Project ProjectRuntime
	Service string
}

// MetricSample is one current resource measurement.
type MetricSample struct {
	Timestamp        time.Time `json:"timestamp"`
	ProjectID        string    `json:"projectId"`
	ServiceID        string    `json:"serviceId"`
	InstanceID       string    `json:"instanceId,omitempty"`
	CPUPercent       float64   `json:"cpuPercent"`
	CPUAvailable     bool      `json:"cpuAvailable"`
	MemoryBytes      uint64    `json:"memoryBytes"`
	MemoryLimit      uint64    `json:"memoryLimit"`
	MemoryAvailable  bool      `json:"memoryAvailable"`
	NetworkRxBytes   uint64    `json:"networkRxBytes"`
	NetworkTxBytes   uint64    `json:"networkTxBytes"`
	NetworkAvailable bool      `json:"networkAvailable"`
	DiskReadBytes    uint64    `json:"diskReadBytes"`
	DiskWriteBytes   uint64    `json:"diskWriteBytes"`
	DiskAvailable    bool      `json:"diskAvailable"`
	ProcessCount     int       `json:"processCount"`
	RestartCount     int       `json:"restartCount"`
	Partial          bool      `json:"partial"`
}

// RuntimeEvent identifies one Compose-labelled Engine event.
type RuntimeEvent struct {
	ProjectID       string    `json:"projectId,omitempty"`
	Driver          Kind      `json:"driver"`
	ProjectIdentity string    `json:"projectIdentity"`
	ServiceIdentity string    `json:"serviceIdentity"`
	ContainerID     string    `json:"containerId"`
	RunID           string    `json:"runId,omitempty"`
	Action          string    `json:"action"`
	OccurredAt      time.Time `json:"occurredAt"`
}

// RunRecord is the durable ownership record for one managed native service run.
type RunRecord struct {
	ID                  string
	ProjectID           string
	ServiceID           string
	OperationID         string
	RuntimeDriver       Kind
	Origin              Origin
	StartedAt           time.Time
	EndedAt             *time.Time
	ExitCode            *int
	TerminationReason   string
	IdentityFingerprint string
	RestartCount        int
	Processes           []ProcessIdentity
}

// ProcessIdentity is the PID-reuse-resistant evidence for one process-group member.
type ProcessIdentity struct {
	RunID            string
	PID              int32
	ProcessGroup     int32
	Executable       string
	StartedAt        time.Time
	WorkingDirectory string
	Fingerprint      string
	ObservedAt       time.Time
}

// ProgressSink receives lifecycle execution progress.
type ProgressSink interface {
	Step(context.Context, string, string, string) error
}

// LogSink receives runtime log lines.
type LogSink interface {
	WriteLog(context.Context, LogEntry) error
}

// MetricSink receives runtime metric samples.
type MetricSink interface {
	WriteMetric(context.Context, MetricSample) error
}

// EventSink receives driver events that require targeted reconciliation.
type EventSink interface {
	WriteRuntimeEvent(context.Context, RuntimeEvent) error
}

// ErrUnsupportedDriver identifies a trusted manifest without an available runtime driver.
var ErrUnsupportedDriver = errors.New("runtime driver is unsupported")

// ErrRuntimeUnavailable identifies a disconnected or inaccessible runtime backend.
var ErrRuntimeUnavailable = errors.New("runtime backend is unavailable")

// ParseAction validates a public lifecycle name.
func ParseAction(value string) (Action, error) {
	action := Action(value)
	switch action {
	case ActionStart, ActionStop, ActionRestart, ActionPause, ActionUnpause, ActionRebuild, ActionTeardown:
		return action, nil
	default:
		return "", errors.New("unsupported runtime action")
	}
}
