package mcpserver

import (
	"context"

	"switchyard.dev/switchyard/internal/transport/contract/generated"
)

// backend is the application-use-case façade consumed by MCP handlers.
// The production implementation is the typed, user-permissioned local IPC client.
type backend interface {
	System(context.Context) (generated.SystemInfo, error)
	Projects(context.Context) ([]generated.Project, error)
	Project(context.Context, string) (generated.Project, error)
	Runtime(context.Context, string) (generated.RuntimeObservation, error)
	RuntimeLogs(context.Context, string, string, string, string, string, int) ([]generated.RuntimeLogEntry, error)
	Health(context.Context, string) (generated.ProjectHealth, error)
	GitState(context.Context, string) (generated.GitState, error)
	PortRegistry(context.Context) (generated.PortRegistry, error)
	SuggestPort(context.Context, int, int, string, string, []int, string) (generated.PortSuggestion, error)
	ProjectActions(context.Context, string) (generated.ProjectActions, error)
	Operation(context.Context, string) (generated.Operation, error)
	ExplainManifest(context.Context, string) (generated.EffectiveManifest, error)
	CreateRuntimeOperationForServices(context.Context, string, generated.RuntimeAction, bool, []string, string) (generated.Operation, error)
	CreateActionOperation(context.Context, string, string, bool, bool, string) (generated.Operation, error)
	CancelOperation(context.Context, string, string) (generated.Operation, error)
	CreateManifestProposal(context.Context, string, string) (generated.ManifestProposal, error)
	ManifestProposal(context.Context, string) (generated.ManifestProposal, error)
	AcceptManifestProposal(context.Context, string, string) (generated.AcceptedManifestProposal, error)
}
