package application

import (
	"context"
	"errors"
	"slices"
	"testing"

	"switchyard.dev/switchyard/internal/workspace/domain"
)

func TestExecuteRecipesRunsExplicitRecipesInStableOrder(t *testing.T) {
	t.Parallel()
	workspace := orchestrationWorkspace()
	workspace.Recipes = []domain.Recipe{
		{ID: "editor", Name: "Editor", Kind: domain.RecipeOpenEditor, ProjectID: "web", Arguments: []string{}, Order: 2},
		{ID: "url", Name: "URL", Kind: domain.RecipeOpenURL, Target: "http://web.localhost", Arguments: []string{}, Order: 1},
	}
	service := NewService(newFakeRepository(workspace), &fakeProjectOperator{}, noHealthGate{}, allowMembers{})
	runner := &recipeRunnerStub{}
	if err := service.ExecuteRecipes(context.Background(), workspace.ID, runner); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(runner.ids, []string{"url", "editor"}) {
		t.Fatalf("recipe order = %v", runner.ids)
	}
}

func TestExecuteRecipesStopsOnFailureAndHonorsCancellation(t *testing.T) {
	t.Parallel()
	workspace := orchestrationWorkspace()
	workspace.Recipes = []domain.Recipe{
		{ID: "first", Name: "First", Kind: domain.RecipeOpenURL, Target: "http://one.localhost", Arguments: []string{}},
		{ID: "second", Name: "Second", Kind: domain.RecipeOpenURL, Target: "http://two.localhost", Arguments: []string{}, Order: 1},
	}
	service := NewService(newFakeRepository(workspace), &fakeProjectOperator{}, noHealthGate{}, allowMembers{})
	runner := &recipeRunnerStub{err: errors.New("launcher failed")}
	if err := service.ExecuteRecipes(context.Background(), workspace.ID, runner); err == nil || len(runner.ids) != 1 {
		t.Fatalf("error=%v recipes=%v", err, runner.ids)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	runner = &recipeRunnerStub{}
	if err := service.ExecuteRecipes(ctx, workspace.ID, runner); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation error = %v", err)
	}
}

type recipeRunnerStub struct {
	ids []string
	err error
}

func (r *recipeRunnerStub) RunWorkspaceRecipe(_ context.Context, recipe domain.Recipe) error {
	r.ids = append(r.ids, recipe.ID)
	return r.err
}
