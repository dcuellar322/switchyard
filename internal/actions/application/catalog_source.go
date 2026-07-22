package application

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"switchyard.dev/switchyard/internal/actions/domain"
	catalog "switchyard.dev/switchyard/internal/catalog/application"
	catalogDomain "switchyard.dev/switchyard/internal/catalog/domain"
	manifestDomain "switchyard.dev/switchyard/internal/manifest/domain"
)

// ToolPreferenceSource exposes user-selected integration identifiers without
// coupling actions to settings persistence.
type ToolPreferenceSource interface {
	PreferredTools(context.Context) (string, string, error)
}

// CatalogSource exposes only accepted action and endpoint declarations.
type CatalogSource struct {
	catalog *catalog.Service
	tools   ToolPreferenceSource
}

// NewCatalogSource adapts accepted manifest actions and safe defaults.
func NewCatalogSource(service *catalog.Service, preferences ...ToolPreferenceSource) *CatalogSource {
	source := &CatalogSource{catalog: service}
	if len(preferences) > 0 {
		source.tools = preferences[0]
	}
	return source
}

// ResolveActions returns actions only after project trust has been established.
func (s *CatalogSource) ResolveActions(ctx context.Context, projectID string) (domain.ProjectActions, error) {
	project, err := s.catalog.GetProject(ctx, projectID)
	if err != nil {
		return domain.ProjectActions{}, err
	}
	if project.TrustState != catalogDomain.TrustTrusted {
		return domain.ProjectActions{}, ErrProjectUntrusted
	}
	effective, err := s.catalog.EffectiveManifest(ctx, projectID, nil)
	if err != nil {
		return domain.ProjectActions{}, err
	}
	actions := make(map[string]domain.Definition)
	primaryEndpointActionID := ""
	for _, action := range effective.Manifest.Actions {
		actions[action.ID] = definition(action)
	}
	terminal, editor := "system", "vscode"
	if s.tools != nil {
		terminal, editor, err = s.tools.PreferredTools(ctx)
		if err != nil {
			return domain.ProjectActions{}, err
		}
	}
	if action, available := builtInTerminalAction(terminal); available {
		addDefault(actions, action)
	}
	if editor == "vscode" {
		addDefault(actions, domain.Definition{ID: "vscode", Name: "Open VS Code", Type: "editor.open", Provider: "vscode", WorkingDirectory: ".", Risk: domain.RiskInteractive})
	}
	addDefault(actions, domain.Definition{ID: "codex", Name: "Start Codex", Type: "agent.start", Provider: "codex", WorkingDirectory: ".", Risk: domain.RiskInteractive})
	addDefault(actions, domain.Definition{ID: "claude", Name: "Start Claude Code", Type: "agent.start", Provider: "claude", WorkingDirectory: ".", Risk: domain.RiskInteractive})
	addDefault(actions, domain.Definition{ID: "git-pull", Name: "Git pull", Type: "git.pull", WorkingDirectory: ".", Risk: domain.RiskNetworked, TimeoutSeconds: 300})
	for _, endpoint := range effective.Manifest.Endpoints {
		actionID := "open-" + endpoint.ID
		addDefault(actions, domain.Definition{
			ID: actionID, Name: "Open " + endpoint.Name, Type: "browser.open",
			Target: resolveEndpoint(endpoint.URL, effective.Manifest.Ports), Risk: domain.RiskInteractive,
		})
		if endpoint.Primary {
			primaryEndpointActionID = actionID
		}
	}
	result := make([]domain.Definition, 0, len(actions))
	for _, action := range actions {
		if action.Command == nil {
			action.Command = []string{}
		}
		result = append(result, action)
	}
	sortDefinitions(result, primaryEndpointActionID)
	return domain.ProjectActions{ProjectID: project.ID, ProjectName: project.DisplayName, Root: project.PrimaryLocation, Actions: result}, nil
}

func builtInTerminalAction(preference string) (domain.Definition, bool) {
	if preference == "integrated" {
		return domain.Definition{}, false
	}
	provider := ""
	if preference != "system" {
		provider = preference
	}
	return domain.Definition{
		ID: "terminal", Name: "Open terminal", Type: "terminal.open", Provider: provider,
		WorkingDirectory: ".", Risk: domain.RiskInteractive,
	}, true
}

func sortDefinitions(result []domain.Definition, primaryEndpointActionID string) {
	sort.Slice(result, func(i, j int) bool {
		if result[i].ID == primaryEndpointActionID {
			return true
		}
		if result[j].ID == primaryEndpointActionID {
			return false
		}
		return result[i].ID < result[j].ID
	})
}

func definition(action manifestDomain.Action) domain.Definition {
	risk := domain.Risk(action.Risk)
	if risk == "" {
		risk = defaultRisk(action.Type)
	}
	return domain.Definition{
		ID: action.ID, Name: action.Name, Type: action.Type, Command: append([]string(nil), action.Command...),
		WorkingDirectory: action.WorkingDirectory, Shell: action.Shell, CaptureOutput: action.CaptureOutput,
		Provider: action.Provider, Target: action.Target, Risk: risk, TimeoutSeconds: action.TimeoutSeconds,
		Environment: cloneEnvironment(action.Environment),
	}
}

func defaultRisk(actionType string) domain.Risk {
	switch actionType {
	case "git.fetch", "git.pull", "git.push":
		return domain.RiskNetworked
	case "terminal.open", "editor.open", "browser.open", "agent.start":
		return domain.RiskInteractive
	case "command", "command.run", "tests.run", "migration.run":
		return domain.RiskMutating
	default:
		return domain.RiskReadOnly
	}
}

func addDefault(actions map[string]domain.Definition, action domain.Definition) {
	if _, exists := actions[action.ID]; !exists {
		actions[action.ID] = action
	}
}

func resolveEndpoint(url string, ports []manifestDomain.Port) string {
	for _, port := range ports {
		url = strings.ReplaceAll(url, fmt.Sprintf("${ports.%s}", port.ID), fmt.Sprint(port.Host))
	}
	return url
}

func cloneEnvironment(values map[string]string) map[string]string {
	result := make(map[string]string, len(values))
	for key, value := range values {
		result[key] = value
	}
	return result
}
