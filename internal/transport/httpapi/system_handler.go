package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	actionsDomain "switchyard.dev/switchyard/internal/actions/domain"
	catalogDomain "switchyard.dev/switchyard/internal/catalog/domain"
	discoveryDomain "switchyard.dev/switchyard/internal/discovery/domain"
	manifestApplication "switchyard.dev/switchyard/internal/manifest/application"
	observabilityDomain "switchyard.dev/switchyard/internal/observability/domain"
	operationsApplication "switchyard.dev/switchyard/internal/operations/application"
	operationsDomain "switchyard.dev/switchyard/internal/operations/domain"
	portsDomain "switchyard.dev/switchyard/internal/ports/domain"
	runtimeDomain "switchyard.dev/switchyard/internal/runtime/domain"
	session "switchyard.dev/switchyard/internal/session/application"
	sourcecontrolDomain "switchyard.dev/switchyard/internal/sourcecontrol/domain"
	"switchyard.dev/switchyard/internal/system/application"
	"switchyard.dev/switchyard/internal/transport/contract/generated"
)

type systemQuery interface {
	Get(ctx context.Context) (application.Info, error)
}

type handler struct {
	system     systemQuery
	operations operationService
	sessions   sessionService
	catalog    catalogService
	runtime    runtimeService
	health     healthService
	logs       logService
	ports      portService
	git        gitService
	actions    actionService
}

type catalogService interface {
	Scan(context.Context, string) (catalogDomain.Project, discoveryDomain.Proposal, error)
	GetProposal(context.Context, string) (discoveryDomain.Proposal, error)
	Validate(context.Context, string) (discoveryDomain.Proposal, error)
	Accept(context.Context, string) (catalogDomain.Project, discoveryDomain.Proposal, error)
	ListProjects(context.Context) ([]catalogDomain.Project, error)
	GetProject(context.Context, string) (catalogDomain.Project, error)
	TrustProject(context.Context, string) (catalogDomain.Project, discoveryDomain.Proposal, error)
	RemoveProject(context.Context, string) error
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
