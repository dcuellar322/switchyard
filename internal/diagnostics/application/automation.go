package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"switchyard.dev/switchyard/internal/diagnostics/domain"
	"switchyard.dev/switchyard/internal/foundation/identifier"
)

var (
	// ErrInvalidRecipe identifies an unsupported trigger, action, or execution limit.
	ErrInvalidRecipe = errors.New("invalid automation recipe")
	// ErrRecipeNotFound identifies an unknown saved recipe.
	ErrRecipeNotFound = errors.New("automation recipe not found")
)

// RecipeRepository stores inspectable automation configuration and dispatch limits.
type RecipeRepository interface {
	SaveRecipe(context.Context, domain.Recipe) error
	GetRecipe(context.Context, string) (domain.Recipe, error)
	ListRecipes(context.Context, string) ([]domain.Recipe, error)
	UpdateRecipeEnabled(context.Context, string, bool, time.Time) (domain.Recipe, error)
	MarkRecipeRun(context.Context, string, time.Time) (domain.Recipe, error)
}

// ActionSource resolves the current accepted action vocabulary.
type ActionSource interface {
	ApprovedActions(context.Context, string) ([]domain.Action, error)
}

// ActionSubmitter routes automation through the durable operation kernel.
type ActionSubmitter interface {
	SubmitAction(context.Context, string, string, string, string) (string, error)
}

// ProjectSource lists local projects for explicit scheduler evaluation.
type ProjectSource interface {
	ProjectIDs(context.Context) ([]string, error)
}

// AutomationService owns saved recipes and never executes commands directly.
type AutomationService struct {
	repo        RecipeRepository
	actions     ActionSource
	diagnostics *Service
	projects    ProjectSource
	submitter   ActionSubmitter
	now         func() time.Time
}

// NewAutomationService constructs safe automation over durable operations.
func NewAutomationService(repo RecipeRepository, actions ActionSource, diagnostics *Service, projects ProjectSource, submitter ActionSubmitter) (*AutomationService, error) {
	if repo == nil || actions == nil || diagnostics == nil || projects == nil || submitter == nil {
		return nil, errors.New("automation dependencies are required")
	}
	return &AutomationService{repo: repo, actions: actions, diagnostics: diagnostics, projects: projects, submitter: submitter, now: time.Now}, nil
}

// Save creates a disabled recipe so enabling is always a separate review step.
func (s *AutomationService) Save(ctx context.Context, projectID, name, triggerCode, actionID string, cooldownSeconds, maxRunsPerDay int) (domain.Recipe, error) {
	if strings.TrimSpace(name) == "" || len(name) > 120 || !supportedTrigger(triggerCode) || cooldownSeconds < 60 || cooldownSeconds > 86_400 || maxRunsPerDay < 1 || maxRunsPerDay > 20 {
		return domain.Recipe{}, ErrInvalidRecipe
	}
	actions, err := s.actions.ApprovedActions(ctx, projectID)
	if err != nil {
		return domain.Recipe{}, err
	}
	action, ok := findApprovedAction(actions, actionID)
	if !ok || !safeForAutomation(action) {
		return domain.Recipe{}, fmt.Errorf("%w: action must be read-only or a declared test/check action", ErrInvalidRecipe)
	}
	id, err := identifier.New("recipe")
	if err != nil {
		return domain.Recipe{}, err
	}
	now := s.now().UTC()
	recipe := domain.Recipe{
		ID: id, ProjectID: projectID, Name: strings.TrimSpace(name), TriggerCode: triggerCode, ActionID: actionID,
		Enabled: false, CooldownSeconds: cooldownSeconds, MaxRunsPerDay: maxRunsPerDay, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.repo.SaveRecipe(ctx, recipe); err != nil {
		return domain.Recipe{}, err
	}
	return recipe, nil
}

// List returns every recipe, including disabled ones.
func (s *AutomationService) List(ctx context.Context, projectID string) ([]domain.Recipe, error) {
	return s.repo.ListRecipes(ctx, projectID)
}

// SetEnabled explicitly enables or disables one recipe after revalidating its action.
func (s *AutomationService) SetEnabled(ctx context.Context, id string, enabled bool) (domain.Recipe, error) {
	recipe, err := s.repo.GetRecipe(ctx, id)
	if err != nil {
		return domain.Recipe{}, err
	}
	if enabled {
		actions, actionsErr := s.actions.ApprovedActions(ctx, recipe.ProjectID)
		action, ok := findApprovedAction(actions, recipe.ActionID)
		if actionsErr != nil || !ok || !safeForAutomation(action) {
			return domain.Recipe{}, fmt.Errorf("%w: approved action changed", ErrInvalidRecipe)
		}
	}
	return s.repo.UpdateRecipeEnabled(ctx, id, enabled, s.now().UTC())
}

// Evaluate runs deterministic diagnosis and dispatches only due, still-safe recipes.
func (s *AutomationService) Evaluate(ctx context.Context, projectID string) ([]string, error) {
	diagnosis, err := s.diagnostics.Diagnose(ctx, projectID, "")
	if err != nil {
		return nil, err
	}
	recipes, err := s.repo.ListRecipes(ctx, projectID)
	if err != nil {
		return nil, err
	}
	actions, err := s.actions.ApprovedActions(ctx, projectID)
	if err != nil {
		return nil, err
	}
	codes := diagnosisCodes(diagnosis)
	operationIDs := []string{}
	for _, recipe := range recipes {
		if !recipe.Enabled || !codes[recipe.TriggerCode] || !due(recipe, s.now().UTC()) {
			continue
		}
		action, ok := findApprovedAction(actions, recipe.ActionID)
		if !ok || !safeForAutomation(action) {
			_, _ = s.repo.UpdateRecipeEnabled(ctx, recipe.ID, false, s.now().UTC())
			continue
		}
		operationID, submitErr := s.submitter.SubmitAction(ctx, projectID, recipe.ActionID, recipe.ID, diagnosis.ID)
		if submitErr != nil {
			return operationIDs, submitErr
		}
		if _, markErr := s.repo.MarkRecipeRun(ctx, recipe.ID, s.now().UTC()); markErr != nil {
			return operationIDs, markErr
		}
		operationIDs = append(operationIDs, operationID)
	}
	return operationIDs, nil
}

// Run evaluates enabled recipes periodically until cancellation.
func (s *AutomationService) Run(ctx context.Context, interval time.Duration, onError func(string, error)) {
	if interval < time.Minute {
		interval = time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			projects, err := s.projects.ProjectIDs(ctx)
			if err != nil {
				if onError != nil {
					onError("", err)
				}
				continue
			}
			for _, projectID := range projects {
				if _, err := s.Evaluate(ctx, projectID); err != nil {
					if onError != nil {
						onError(projectID, err)
					}
				}
			}
		}
	}
}

func supportedTrigger(value string) bool {
	switch value {
	case "REPEATED_CRASH", "PORT_CONFLICT", "PORT_BIND_FAILED", "RESOURCE_PRESSURE", "RESOURCE_EXHAUSTED", "UNHEALTHY_DEPENDENCY", "DEPENDENCY_UNREACHABLE":
		return true
	default:
		return false
	}
}

func safeForAutomation(action domain.Action) bool {
	if action.Risk == "read_only" {
		return true
	}
	kind := strings.ToLower(action.Type)
	return action.Risk != "destructive" && action.Risk != "networked" && action.Risk != "interactive" &&
		hasSafeAutomationVerb(kind)
}

func hasSafeAutomationVerb(kind string) bool {
	for _, token := range strings.FieldsFunc(kind, func(value rune) bool {
		return value < 'a' || value > 'z'
	}) {
		switch token {
		case "test", "tests", "check", "checks", "inspect", "inspection":
			return true
		}
	}
	return false
}

func findApprovedAction(actions []domain.Action, id string) (domain.Action, bool) {
	for _, action := range actions {
		if action.ID == id {
			return action, true
		}
	}
	return domain.Action{}, false
}

func diagnosisCodes(diagnosis domain.Diagnosis) map[string]bool {
	result := map[string]bool{}
	for _, hypothesis := range diagnosis.Hypotheses {
		if hypothesis.Source == "deterministic" {
			result[hypothesis.Code] = true
		}
	}
	return result
}

func due(recipe domain.Recipe, now time.Time) bool {
	if recipe.LastRunAt != nil && now.Sub(*recipe.LastRunAt) < time.Duration(recipe.CooldownSeconds)*time.Second {
		return false
	}
	day := now.UTC().Format(time.DateOnly)
	return recipe.RunsDay != day || recipe.RunsToday < recipe.MaxRunsPerDay
}

// ActionOperationInput is the only payload emitted by automation.
func ActionOperationInput(actionID, recipeID string) []byte {
	encoded, _ := json.Marshal(map[string]any{
		"actionId": actionID, "confirmRisk": false, "allowOutsideRoot": false,
		"actorType": "automation", "actorId": recipeID,
	})
	return encoded
}
