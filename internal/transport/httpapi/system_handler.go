package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	actionsDomain "switchyard.dev/switchyard/internal/actions/domain"
	agentsApplication "switchyard.dev/switchyard/internal/agents/application"
	catalogApplication "switchyard.dev/switchyard/internal/catalog/application"
	catalogDomain "switchyard.dev/switchyard/internal/catalog/domain"
	discoveryDomain "switchyard.dev/switchyard/internal/discovery/domain"
	environmentApplication "switchyard.dev/switchyard/internal/environments/application"
	environmentDomain "switchyard.dev/switchyard/internal/environments/domain"
	manifestApplication "switchyard.dev/switchyard/internal/manifest/application"
	observabilityDomain "switchyard.dev/switchyard/internal/observability/domain"
	operationsApplication "switchyard.dev/switchyard/internal/operations/application"
	operationsDomain "switchyard.dev/switchyard/internal/operations/domain"
	portsDomain "switchyard.dev/switchyard/internal/ports/domain"
	routingDomain "switchyard.dev/switchyard/internal/routing/domain"
	runtimeDomain "switchyard.dev/switchyard/internal/runtime/domain"
	session "switchyard.dev/switchyard/internal/session/application"
	sourcecontrolDomain "switchyard.dev/switchyard/internal/sourcecontrol/domain"
	"switchyard.dev/switchyard/internal/system/application"
	terminalApplication "switchyard.dev/switchyard/internal/terminal/application"
	terminalDomain "switchyard.dev/switchyard/internal/terminal/domain"
	"switchyard.dev/switchyard/internal/transport/contract/generated"
	workspaceApplication "switchyard.dev/switchyard/internal/workspace/application"
	workspaceDomain "switchyard.dev/switchyard/internal/workspace/domain"
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
	workspaces              workspaceService
	environments            environmentService
	environmentRegistration environmentRegistrationService
	routes                  routeService
	terminals               terminalService
}

func (h *handler) GetHost(w http.ResponseWriter, r *http.Request) {
	observation := h.host.Get(r.Context())
	warnings := observation.Warnings
	if warnings == nil {
		warnings = []string{}
	}
	response := generated.HostObservation{
		CpuPercent: observation.CPUPercent, MemoryUsedBytes: int64(observation.MemoryUsedBytes),
		MemoryTotalBytes: int64(observation.MemoryTotalBytes), ObservedAt: observation.ObservedAt, Warnings: warnings,
		Docker: generated.DockerHostObservation{
			Connected: observation.Docker.Connected, StorageBytes: observation.Docker.StorageBytes,
			ReclaimableBytes: observation.Docker.ReclaimableBytes,
			Attribution:      generated.DockerHostObservationAttribution(observation.Docker.Attribution),
		},
	}
	writeJSON(w, http.StatusOK, response)
}

type catalogService interface {
	Scan(context.Context, string) (catalogDomain.Project, discoveryDomain.Proposal, error)
	ScanAs(context.Context, string, catalogApplication.MutationActor) (catalogDomain.Project, discoveryDomain.Proposal, error)
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

type runtimeService interface {
	Inspect(context.Context, string) (runtimeDomain.Observation, error)
	Plan(context.Context, string, runtimeDomain.Action, bool) (runtimeDomain.Plan, error)
	PlanServices(context.Context, string, runtimeDomain.Action, bool, []string) (runtimeDomain.Plan, error)
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
