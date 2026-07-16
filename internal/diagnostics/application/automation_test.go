package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"switchyard.dev/switchyard/internal/diagnostics/domain"
)

func TestAutomationRequiresSeparateEnableAndHonorsLimits(t *testing.T) {
	now := time.Date(2026, 7, 16, 10, 0, 0, 0, time.UTC)
	diagnosticRepo := newRepositoryFake()
	diagnostics, _ := NewService(&collectorFake{bundle: domain.Bundle{
		ProjectID: "project_fixture", ProjectState: "running", CollectedAt: now,
		Evidence: []domain.Evidence{{ID: "runtime", Kind: "runtime", ObservedAt: now}, {ID: "operations", Kind: "operations", ObservedAt: now}},
		Actions:  []domain.Action{{ID: "health-check", Name: "Health check", Type: "health.check", Risk: "read_only"}},
		Snapshot: domain.ProjectSnapshot{FailedRuns: 3, Runtime: domain.RuntimeSnapshot{Driver: "process", EngineConnected: true}, ConfigSources: map[string]string{}},
	}}, nil, diagnosticRepo)
	recipes := newRecipeRepositoryFake()
	actions := &automationSourceFake{actions: []domain.Action{{ID: "health-check", Name: "Health check", Type: "health.check", Risk: "read_only"}, {ID: "delete", Name: "Delete", Type: "cleanup", Risk: "destructive"}}}
	submitter := &submitterFake{}
	service, err := NewAutomationService(recipes, actions, diagnostics, actions, submitter)
	if err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return now }
	recipe, err := service.Save(t.Context(), "project_fixture", "Check after crashes", "REPEATED_CRASH", "health-check", 3600, 1)
	if err != nil || recipe.Enabled {
		t.Fatalf("Save() recipe=%#v err=%v", recipe, err)
	}
	if operations, err := service.Evaluate(t.Context(), "project_fixture"); err != nil || len(operations) != 0 {
		t.Fatalf("disabled Evaluate() operations=%#v err=%v", operations, err)
	}
	if _, err := service.SetEnabled(t.Context(), recipe.ID, true); err != nil {
		t.Fatal(err)
	}
	operations, err := service.Evaluate(t.Context(), "project_fixture")
	if err != nil || len(operations) != 1 || submitter.calls != 1 {
		t.Fatalf("Evaluate() operations=%#v calls=%d err=%v", operations, submitter.calls, err)
	}
	operations, err = service.Evaluate(t.Context(), "project_fixture")
	if err != nil || len(operations) != 0 || submitter.calls != 1 {
		t.Fatalf("limited Evaluate() operations=%#v calls=%d err=%v", operations, submitter.calls, err)
	}
	if _, err := service.Save(t.Context(), "project_fixture", "Unsafe", "REPEATED_CRASH", "delete", 60, 1); !errors.Is(err, ErrInvalidRecipe) {
		t.Fatalf("unsafe Save() error = %v", err)
	}
	actions.actions = append(actions.actions, domain.Action{ID: "contest", Name: "Contest deploy", Type: "contest.deploy", Risk: "mutating"})
	if _, err := service.Save(t.Context(), "project_fixture", "Substring", "REPEATED_CRASH", "contest", 60, 1); !errors.Is(err, ErrInvalidRecipe) {
		t.Fatalf("substring action Save() error = %v", err)
	}
}

type recipeRepositoryFake struct{ values map[string]domain.Recipe }

func newRecipeRepositoryFake() *recipeRepositoryFake {
	return &recipeRepositoryFake{values: map[string]domain.Recipe{}}
}
func (r *recipeRepositoryFake) SaveRecipe(_ context.Context, value domain.Recipe) error {
	r.values[value.ID] = value
	return nil
}
func (r *recipeRepositoryFake) GetRecipe(_ context.Context, id string) (domain.Recipe, error) {
	value, ok := r.values[id]
	if !ok {
		return domain.Recipe{}, ErrRecipeNotFound
	}
	return value, nil
}
func (r *recipeRepositoryFake) ListRecipes(_ context.Context, projectID string) ([]domain.Recipe, error) {
	result := []domain.Recipe{}
	for _, value := range r.values {
		if projectID == "" || value.ProjectID == projectID {
			result = append(result, value)
		}
	}
	return result, nil
}
func (r *recipeRepositoryFake) UpdateRecipeEnabled(_ context.Context, id string, enabled bool, at time.Time) (domain.Recipe, error) {
	value := r.values[id]
	value.Enabled, value.UpdatedAt = enabled, at
	r.values[id] = value
	return value, nil
}
func (r *recipeRepositoryFake) MarkRecipeRun(_ context.Context, id string, at time.Time) (domain.Recipe, error) {
	value := r.values[id]
	value.LastRunAt = &at
	value.RunsToday++
	value.RunsDay = at.Format(time.DateOnly)
	r.values[id] = value
	return value, nil
}

type automationSourceFake struct{ actions []domain.Action }

func (f *automationSourceFake) ApprovedActions(context.Context, string) ([]domain.Action, error) {
	return f.actions, nil
}
func (f *automationSourceFake) ProjectIDs(context.Context) ([]string, error) {
	return []string{"project_fixture"}, nil
}

type submitterFake struct{ calls int }

func (f *submitterFake) SubmitAction(context.Context, string, string, string, string) (string, error) {
	f.calls++
	return "op_fixture", nil
}
