package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"time"

	actionsDomain "switchyard.dev/switchyard/internal/actions/domain"
	agentsApplication "switchyard.dev/switchyard/internal/agents/application"
	catalogApplication "switchyard.dev/switchyard/internal/catalog/application"
	catalogDomain "switchyard.dev/switchyard/internal/catalog/domain"
	diagnosticsDomain "switchyard.dev/switchyard/internal/diagnostics/domain"
	discoveryDomain "switchyard.dev/switchyard/internal/discovery/domain"
	environmentApplication "switchyard.dev/switchyard/internal/environments/application"
	environmentDomain "switchyard.dev/switchyard/internal/environments/domain"
	fleetApplication "switchyard.dev/switchyard/internal/fleet/application"
	fleetDomain "switchyard.dev/switchyard/internal/fleet/domain"
	manifestApplication "switchyard.dev/switchyard/internal/manifest/application"
	observabilityDomain "switchyard.dev/switchyard/internal/observability/domain"
	operationsApplication "switchyard.dev/switchyard/internal/operations/application"
	operationsDomain "switchyard.dev/switchyard/internal/operations/domain"
	pluginsDomain "switchyard.dev/switchyard/internal/plugins/domain"
	portsDomain "switchyard.dev/switchyard/internal/ports/domain"
	routingDomain "switchyard.dev/switchyard/internal/routing/domain"
	runtimeDomain "switchyard.dev/switchyard/internal/runtime/domain"
	session "switchyard.dev/switchyard/internal/session/application"
	settingsApplication "switchyard.dev/switchyard/internal/settings/application"
	settingsDomain "switchyard.dev/switchyard/internal/settings/domain"
	sourcecontrolDomain "switchyard.dev/switchyard/internal/sourcecontrol/domain"
	"switchyard.dev/switchyard/internal/system/application"
	teamApplication "switchyard.dev/switchyard/internal/team/application"
	teamDomain "switchyard.dev/switchyard/internal/team/domain"
	telemetryApplication "switchyard.dev/switchyard/internal/telemetry/application"
	telemetryDomain "switchyard.dev/switchyard/internal/telemetry/domain"
	terminalApplication "switchyard.dev/switchyard/internal/terminal/application"
	terminalDomain "switchyard.dev/switchyard/internal/terminal/domain"
	"switchyard.dev/switchyard/internal/transport/contract/generated"
	workspaceApplication "switchyard.dev/switchyard/internal/workspace/application"
	workspaceDomain "switchyard.dev/switchyard/internal/workspace/domain"
	pluginsdk "switchyard.dev/switchyard/sdk/plugin"
)

type systemQuery interface {
	Get(ctx context.Context) (application.Info, error)
}

type hostQuery interface {
	Get(context.Context) application.HostObservation
}

type handler struct {
	system                  systemQuery
	host                    hostQuery
	operations              operationService
	sessions                sessionService
	catalog                 catalogService
	runtime                 runtimeService
	health                  healthService
	logs                    logService
	ports                   portService
	git                     gitService
	actions                 actionService
	ai                      aiOnboardingService
	resources               resourceService
	plugins                 pluginService
	diagnostics             diagnosticService
	automations             automationService
	workspaces              workspaceService
	environments            environmentService
	environmentRegistration environmentRegistrationService
	routes                  routeService
	terminals               terminalService
	fleet                   fleetService
	team                    teamService
	telemetry               telemetryService
	settings                settingsService
}

func (h *handler) GetHost(w http.ResponseWriter, r *http.Request) {
	observation := h.host.Get(r.Context())
	warnings := observation.Warnings
	if warnings == nil {
		warnings = []string{}
	}
	response := generated.HostObservation{
		CpuPercent: observation.CPUPercent, MemoryUsedBytes: boundedAPIInt64(observation.MemoryUsedBytes),
		MemoryTotalBytes: boundedAPIInt64(observation.MemoryTotalBytes), ObservedAt: observation.ObservedAt, Warnings: warnings,
		Docker: generated.DockerHostObservation{
			Connected: observation.Docker.Connected, StorageBytes: observation.Docker.StorageBytes,
			ReclaimableBytes: observation.Docker.ReclaimableBytes,
			Attribution:      generated.DockerHostObservationAttribution(observation.Docker.Attribution),
		},
	}
	writeJSON(w, http.StatusOK, response)
}

func boundedAPIInt64(value uint64) int64 {
	if value > math.MaxInt64 {
		return math.MaxInt64
	}
	return int64(value)
}

type catalogService interface {
	Scan(context.Context, string) (catalogDomain.Project, discoveryDomain.Proposal, error)
	ScanAs(context.Context, string, catalogApplication.MutationActor) (catalogDomain.Project, discoveryDomain.Proposal, error)
	ScanWithRootOverrideAs(context.Context, string, bool, catalogApplication.MutationActor) (catalogDomain.Project, discoveryDomain.Proposal, error)
	GetProposal(context.Context, string) (discoveryDomain.Proposal, error)
	Validate(context.Context, string) (discoveryDomain.Proposal, error)
	Accept(context.Context, string) (catalogDomain.Project, discoveryDomain.Proposal, error)
	AcceptAs(context.Context, string, catalogApplication.MutationActor) (catalogDomain.Project, discoveryDomain.Proposal, error)
	ListProjects(context.Context) ([]catalogDomain.Project, error)
	GetProject(context.Context, string) (catalogDomain.Project, error)
	TrustProject(context.Context, string) (catalogDomain.Project, discoveryDomain.Proposal, error)
	TrustProjectAs(context.Context, string, catalogApplication.MutationActor) (catalogDomain.Project, discoveryDomain.Proposal, error)
	RemoveProject(context.Context, string) error
	RemoveProjectAs(context.Context, string, catalogApplication.MutationActor) error
	EffectiveManifest(context.Context, string, []byte) (manifestApplication.EffectiveManifest, error)
	Diff(context.Context, string) (map[string]json.RawMessage, error)
	ValidateProject(context.Context, string) (manifestApplication.ValidationResult, error)
}

type operationService interface {
	Submit(context.Context, operationsApplication.SubmitRequest) (operationsDomain.Operation, error)
	Get(ctx context.Context, id string) (operationsDomain.Operation, error)
	List(ctx context.Context, projectID string, limit int64) ([]operationsDomain.Operation, error)
	Cancel(ctx context.Context, id, actorType, actorID, idempotencyKey string) (operationsDomain.Operation, error)
}

type fleetService interface {
	Register(context.Context, fleetApplication.RegisterRequest, fleetApplication.Actor) (fleetDomain.Machine, error)
	List(context.Context) ([]fleetDomain.Machine, error)
	Get(context.Context, string) (fleetDomain.Machine, error)
	ConfigureAccess(context.Context, string, bool, []fleetDomain.Capability, bool, fleetApplication.Actor) (fleetDomain.Machine, error)
	Probe(context.Context, string, fleetApplication.Actor) (fleetDomain.Machine, error)
	Snapshot(context.Context, string) (fleetDomain.Snapshot, error)
	Operate(context.Context, string, fleetDomain.OperationRequest, fleetApplication.Actor) (fleetDomain.OperationReceipt, error)
	Remove(context.Context, string, bool, fleetApplication.Actor) error
}

type teamService interface {
	TrustPublisher(context.Context, string, string, bool, teamApplication.Actor) (teamDomain.Publisher, error)
	Publishers(context.Context) ([]teamDomain.Publisher, error)
	Install(context.Context, teamDomain.Bundle, bool, teamApplication.Actor) (teamDomain.Bundle, error)
	Bundles(context.Context, teamDomain.BundleKind) ([]teamDomain.Bundle, error)
	RenderTemplate(context.Context, string, map[string]string) (json.RawMessage, error)
	Registry(context.Context) ([]teamDomain.RegistryEntry, error)
	EffectivePolicy(context.Context) (teamDomain.EffectivePolicy, error)
	ExportSync(context.Context) (teamDomain.SyncDocument, error)
	PreviewSync(context.Context, teamDomain.SyncDocument) (teamDomain.SyncPreview, error)
	ImportSync(context.Context, teamDomain.SyncDocument, bool, teamApplication.Actor) (teamDomain.SyncPreview, error)
}

type telemetryService interface {
	Status(context.Context) (telemetryDomain.Status, error)
	Configure(context.Context, bool, string, bool, telemetryApplication.Actor) (telemetryDomain.Status, error)
	Send(context.Context) (telemetryDomain.Status, error)
}

type settingsService interface {
	Status(context.Context) (settingsApplication.Status, error)
	Update(context.Context, int64, settingsDomain.Settings, settingsApplication.Actor) (settingsApplication.Status, error)
}

type runtimeService interface {
	Inspect(context.Context, string) (runtimeDomain.Observation, error)
	Plan(context.Context, string, runtimeDomain.Action, bool) (runtimeDomain.Plan, error)
	PlanServices(context.Context, string, runtimeDomain.Action, bool, []string) (runtimeDomain.Plan, error)
	PlanSelection(context.Context, string, runtimeDomain.Action, bool, []string, []string) (runtimeDomain.Plan, error)
	Metrics(context.Context, string, string) ([]runtimeDomain.MetricSample, error)
}

type healthService interface {
	Get(context.Context, string) (observabilityDomain.ProjectHealth, error)
}

type logService interface {
	Logs(context.Context, string, string, string, string, string, int) ([]runtimeDomain.LogEntry, error)
	Export(context.Context, string, string, string, string, string, io.Writer) error
}

type portService interface {
	Registry(context.Context) (portsDomain.Registry, error)
	Suggest(context.Context, int, int, string, string, []int) (portsDomain.Suggestion, error)
}

type gitService interface {
	Get(context.Context, string) (sourcecontrolDomain.State, error)
}

type actionService interface {
	List(context.Context, string) (actionsDomain.ProjectActions, error)
}

type aiOnboardingService interface {
	Providers(context.Context) []agentsApplication.ProviderDescriptor
	Preview(context.Context, string, agentsApplication.Limits) (agentsApplication.BundlePreview, error)
	GetRun(context.Context, string) (agentsApplication.Run, error)
}

type resourceService interface {
	Overview(context.Context) (observabilityDomain.ResourceOverview, error)
	History(context.Context, string, string, string, time.Time, time.Time, int) (observabilityDomain.MetricHistory, error)
	Storage(context.Context) (observabilityDomain.StorageInventory, error)
	CleanupPreview(context.Context, string) (observabilityDomain.CleanupPreview, error)
}

type pluginService interface {
	Refresh(context.Context) ([]pluginsDomain.Plugin, error)
	List(context.Context) ([]pluginsDomain.Plugin, error)
	Trust(context.Context, string, string) (pluginsDomain.Plugin, error)
	Enable(context.Context, string, []string) (pluginsDomain.Plugin, error)
	Disable(context.Context, string) (pluginsDomain.Plugin, error)
	Health(context.Context, string) (pluginsDomain.Plugin, error)
	Logs(context.Context, string, int) ([]pluginsDomain.LogEntry, error)
	Inspect(context.Context, string, string) (pluginsdk.InspectResult, error)
	ValidateOperation(context.Context, string, string) error
}

type diagnosticService interface {
	Diagnose(context.Context, string, string) (diagnosticsDomain.Diagnosis, error)
	Get(context.Context, string) (diagnosticsDomain.Diagnosis, error)
	Latest(context.Context, string) (diagnosticsDomain.Diagnosis, error)
	RecordFeedback(context.Context, string, string, string, string) (diagnosticsDomain.Feedback, error)
	AuthorizeAction(context.Context, string, string) (diagnosticsDomain.Diagnosis, error)
	Notifications(context.Context, string, bool, int) ([]diagnosticsDomain.Notification, error)
	Acknowledge(context.Context, string) (diagnosticsDomain.Notification, error)
}

type automationService interface {
	Save(context.Context, string, string, string, string, int, int) (diagnosticsDomain.Recipe, error)
	List(context.Context, string) ([]diagnosticsDomain.Recipe, error)
	SetEnabled(context.Context, string, bool) (diagnosticsDomain.Recipe, error)
	Evaluate(context.Context, string) ([]string, error)
}

type workspaceService interface {
	Create(context.Context, workspaceApplication.SaveRequest) (workspaceDomain.Workspace, error)
	Update(context.Context, string, workspaceApplication.SaveRequest) (workspaceDomain.Workspace, error)
	Get(context.Context, string) (workspaceDomain.Workspace, error)
	List(context.Context) ([]workspaceDomain.Workspace, error)
	Delete(context.Context, string) error
}

type environmentService interface {
	Get(context.Context, string) (environmentDomain.Environment, error)
	ListProject(context.Context, string) ([]environmentDomain.Environment, error)
	ConfigureRuntime(context.Context, string, environmentApplication.RuntimeConfiguration) (environmentDomain.Environment, error)
}

type environmentRegistrationService interface {
	RegisterWorktrees(context.Context, string) (environmentApplication.Registration, error)
}

type routeService interface {
	Refresh(context.Context) ([]routingDomain.Route, error)
	Snapshot() []routingDomain.Route
}

type terminalService interface {
	Create(context.Context, terminalDomain.CreateRequest, terminalDomain.Owner) (terminalDomain.Session, error)
	List(context.Context, string, terminalDomain.Owner) ([]terminalDomain.Session, error)
	Get(context.Context, string, terminalDomain.Owner) (terminalDomain.Session, error)
	Terminate(context.Context, string, terminalDomain.Owner) (terminalDomain.Session, error)
	Attach(context.Context, string, terminalDomain.Owner) (*terminalApplication.Attachment, error)
}

type sessionService interface {
	IssueBootstrap() (session.Bootstrap, error)
	Exchange(token string) (session.Session, error)
	ValidateSession(id string) (session.Session, error)
	ValidateMutation(id, csrfToken string) (session.Session, error)
}

func (h *handler) GetSystem(w http.ResponseWriter, r *http.Request) {
	info, err := h.system.Get(r.Context())
	if err != nil {
		writeProblem(w, r, http.StatusInternalServerError, "SYSTEM_STATUS_UNAVAILABLE", "System status unavailable", "The daemon could not read its durable status.")
		return
	}
	response := generated.SystemInfo{
		Status:                generated.Ready,
		Version:               info.Version,
		Commit:                info.Commit,
		BuiltAt:               info.BuiltAt,
		ApiVersion:            info.APIVersion,
		DatabaseSchemaVersion: info.DatabaseSchemaVersion,
		StartedAt:             info.StartedAt,
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}
