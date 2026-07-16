package httpapi

import (
	"context"
	"encoding/json"
	"net/http"

	catalogDomain "switchyard.dev/switchyard/internal/catalog/domain"
	discoveryDomain "switchyard.dev/switchyard/internal/discovery/domain"
	manifestApplication "switchyard.dev/switchyard/internal/manifest/application"
	operationsDomain "switchyard.dev/switchyard/internal/operations/domain"
	session "switchyard.dev/switchyard/internal/session/application"
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
	Get(ctx context.Context, id string) (operationsDomain.Operation, error)
	List(ctx context.Context, projectID string, limit int64) ([]operationsDomain.Operation, error)
	Cancel(ctx context.Context, id, actorType, actorID, idempotencyKey string) (operationsDomain.Operation, error)
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
