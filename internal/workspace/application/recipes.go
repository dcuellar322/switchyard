package application

import (
	"context"
	"fmt"
	"slices"
	"sort"

	"switchyard.dev/switchyard/internal/workspace/domain"
)

// RecipeRunner launches one validated, bounded workspace recipe.
type RecipeRunner interface {
	RunWorkspaceRecipe(context.Context, domain.Recipe) error
}

// ExecuteRecipes runs the durable workspace recipe list in declared order.
// Recipes are intentionally opt-in and never run as part of ordinary start.
func (s *Service) ExecuteRecipes(ctx context.Context, workspaceID string, runner RecipeRunner) error {
	workspace, err := s.repository.Get(ctx, workspaceID)
	if err != nil {
		return err
	}
	if runner == nil {
		return fmt.Errorf("%w: workspace recipe runner is unavailable", ErrInvalidRequest)
	}
	recipes := slices.Clone(workspace.Recipes)
	sort.SliceStable(recipes, func(left, right int) bool {
		if recipes[left].Order != recipes[right].Order {
			return recipes[left].Order < recipes[right].Order
		}
		return recipes[left].ID < recipes[right].ID
	})
	for _, recipe := range recipes {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := runner.RunWorkspaceRecipe(ctx, recipe); err != nil {
			return fmt.Errorf("run workspace recipe %s: %w", recipe.ID, err)
		}
	}
	return nil
}
