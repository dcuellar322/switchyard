// Package adapters connects the optional fleet application to existing local
// Switchyard application services without exposing repository locations,
// secrets, logs, or driver-native identifiers.
package adapters

import (
	"context"
	"encoding/json"
	"time"

	catalogDomain "switchyard.dev/switchyard/internal/catalog/domain"
	environmentDomain "switchyard.dev/switchyard/internal/environments/domain"
	"switchyard.dev/switchyard/internal/fleet/domain"
	operationsApplication "switchyard.dev/switchyard/internal/operations/application"
	operationsDomain "switchyard.dev/switchyard/internal/operations/domain"
	runtimeDomain "switchyard.dev/switchyard/internal/runtime/domain"
)

type catalogInventory interface {
	ListProjects(context.Context) ([]catalogDomain.Project, error)
}

type runtimeInventory interface {
	Inspect(context.Context, string) (runtimeDomain.Observation, error)
}

type environmentInventory interface {
	List(context.Context) ([]environmentDomain.Environment, error)
}

// LocalInventory produces the deliberately bounded remote inventory view.
type LocalInventory struct {
	catalog      catalogInventory
	runtime      runtimeInventory
	environments environmentInventory
}

// NewLocalInventory creates an inventory adapter over existing application
// boundaries. It does not read another domain's tables.
func NewLocalInventory(catalog catalogInventory, runtime runtimeInventory, environments environmentInventory) *LocalInventory {
	return &LocalInventory{catalog: catalog, runtime: runtime, environments: environments}
}

// Inventory returns trusted projects and registered environments without local
// filesystem locations or runtime-native identifiers.
func (i *LocalInventory) Inventory(ctx context.Context) ([]domain.Project, []domain.Environment, error) {
	projects, err := i.catalog.ListProjects(ctx)
	if err != nil {
		return nil, nil, err
	}
	result := make([]domain.Project, 0, len(projects))
	for _, project := range projects {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		if project.TrustState != catalogDomain.TrustTrusted {
			continue
		}
		item := domain.Project{
			ID: project.ID, Slug: project.Slug, DisplayName: project.DisplayName,
			Runtime: "unknown", State: string(runtimeDomain.StateUnknown), Health: "unknown",
		}
		if observation, inspectErr := i.runtime.Inspect(ctx, project.ID); inspectErr == nil {
			item.Runtime = string(observation.Driver)
			item.State = string(observation.State)
			item.Health, item.Degraded = observationHealth(observation.State)
		}
		result = append(result, item)
	}

	environments, err := i.environments.List(ctx)
	if err != nil {
		return nil, nil, err
	}
	trusted := make(map[string]struct{}, len(result))
	for _, project := range result {
		trusted[project.ID] = struct{}{}
	}
	environmentResult := make([]domain.Environment, 0, len(environments))
	for _, environment := range environments {
		if _, ok := trusted[environment.ProjectID]; !ok {
			continue
		}
		environmentResult = append(environmentResult, domain.Environment{
			ID: environment.ID, ProjectID: environment.ProjectID, Name: environment.Name,
			Branch: environment.Branch, State: string(environment.State), Availability: string(environment.Availability),
		})
	}
	return result, environmentResult, nil
}

func observationHealth(state runtimeDomain.ProjectState) (string, bool) {
	switch state {
	case runtimeDomain.StateUnknown, runtimeDomain.StateStarting, runtimeDomain.StateStopping:
		return "unknown", false
	case runtimeDomain.StateRunning, runtimeDomain.StateRunningExternal:
		return "healthy", false
	case runtimeDomain.StateDegraded, runtimeDomain.StateFailed, runtimeDomain.StatePartiallyRunning:
		return "degraded", true
	case runtimeDomain.StateStopped, runtimeDomain.StatePaused:
		return "inactive", false
	}
	return "unknown", false
}

type operationSubmitter interface {
	Submit(context.Context, operationsApplication.SubmitRequest) (operationsDomain.Operation, error)
}

// LocalOperator submits remote requests through the same durable coordinator
// and operation executor used by local HTTP and CLI requests.
type LocalOperator struct {
	operations operationSubmitter
	now        func() time.Time
}

// NewLocalOperator creates the typed remote operation adapter.
func NewLocalOperator(operations operationSubmitter) *LocalOperator {
	return &LocalOperator{operations: operations, now: time.Now}
}

// SubmitRemote maps the narrow remote action vocabulary to a durable local
// runtime operation. Environment IDs select their registered worktree runtime.
func (o *LocalOperator) SubmitRemote(ctx context.Context, request domain.OperationRequest, controller string) (domain.OperationReceipt, error) {
	targetID := request.ProjectID
	if request.EnvironmentID != "" {
		targetID = request.EnvironmentID
	}
	input, err := json.Marshal(map[string]any{
		"action": request.Action, "removeVolumes": false, "services": []string{},
	})
	if err != nil {
		return domain.OperationReceipt{}, err
	}
	operation, err := o.operations.Submit(ctx, operationsApplication.SubmitRequest{
		ProjectID: targetID, Kind: "runtime." + string(request.Action), Input: input,
		IdempotencyKey: request.RequestID, ActorType: "remote", ActorID: controller,
	})
	if err != nil {
		return domain.OperationReceipt{}, err
	}
	return domain.OperationReceipt{
		RequestID: request.RequestID, OperationID: operation.ID,
		State: string(operation.State), AcceptedAt: o.now().UTC(),
	}, nil
}
