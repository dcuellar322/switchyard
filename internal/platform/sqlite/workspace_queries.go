package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	workspace "switchyard.dev/switchyard/internal/workspace/application"
	"switchyard.dev/switchyard/internal/workspace/domain"
)

func (r *WorkspaceRepository) loadWorkspaceChildren(ctx context.Context, item *domain.Workspace) error {
	if err := r.loadWorkspaceMembers(ctx, item); err != nil {
		return err
	}
	if err := r.loadWorkspaceDependencies(ctx, item); err != nil {
		return err
	}
	if err := r.loadWorkspaceProfiles(ctx, item); err != nil {
		return err
	}
	return r.loadWorkspaceRecipes(ctx, item)
}

func (r *WorkspaceRepository) loadWorkspaceMembers(ctx context.Context, item *domain.Workspace) error {
	rows, err := r.database.connection.QueryContext(ctx, `SELECT project_id, role, sort_order, health_gate,
        health_timeout_seconds FROM workspace_projects WHERE workspace_id = ? ORDER BY sort_order, project_id`, item.ID)
	if err != nil {
		return fmt.Errorf("list workspace members: %w", err)
	}
	defer func() { _ = rows.Close() }()
	item.Members = []domain.Member{}
	for rows.Next() {
		var member domain.Member
		var healthGate int
		var healthTimeout int64
		if err := rows.Scan(&member.ProjectID, &member.Role, &member.Order, &healthGate, &healthTimeout); err != nil {
			return err
		}
		member.HealthGate = healthGate == 1
		member.HealthTimeout = time.Duration(healthTimeout) * time.Second
		item.Members = append(item.Members, member)
	}
	return rows.Err()
}

func (r *WorkspaceRepository) loadWorkspaceDependencies(ctx context.Context, item *domain.Workspace) error {
	rows, err := r.database.connection.QueryContext(ctx, `SELECT project_id, depends_on_project_id
        FROM workspace_dependencies WHERE workspace_id = ? ORDER BY project_id, depends_on_project_id`, item.ID)
	if err != nil {
		return fmt.Errorf("list workspace dependencies: %w", err)
	}
	defer func() { _ = rows.Close() }()
	item.Dependencies = []domain.Dependency{}
	for rows.Next() {
		var edge domain.Dependency
		if err := rows.Scan(&edge.ProjectID, &edge.DependsOnProjectID); err != nil {
			return err
		}
		item.Dependencies = append(item.Dependencies, edge)
	}
	return rows.Err()
}

func (r *WorkspaceRepository) loadWorkspaceProfiles(ctx context.Context, item *domain.Workspace) error {
	rows, err := r.database.connection.QueryContext(ctx, `SELECT id, name, description, max_parallel,
        low_memory, memory_budget_bytes FROM workspace_profiles WHERE workspace_id = ? ORDER BY name COLLATE NOCASE, id`, item.ID)
	if err != nil {
		return fmt.Errorf("list workspace profiles: %w", err)
	}
	item.Profiles = []domain.Profile{}
	for rows.Next() {
		var profile domain.Profile
		var lowMemory int
		if err := rows.Scan(&profile.ID, &profile.Name, &profile.Description, &profile.MaxParallel,
			&lowMemory, &profile.MemoryBudgetBytes); err != nil {
			_ = rows.Close()
			return err
		}
		profile.LowMemory = lowMemory == 1
		item.Profiles = append(item.Profiles, profile)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for index := range item.Profiles {
		projectRows, err := r.database.connection.QueryContext(ctx, `SELECT project_id
            FROM workspace_profile_projects WHERE workspace_id = ? AND profile_id = ? ORDER BY sort_order, project_id`,
			item.ID, item.Profiles[index].ID)
		if err != nil {
			return fmt.Errorf("list workspace profile projects: %w", err)
		}
		for projectRows.Next() {
			var projectID string
			if err := projectRows.Scan(&projectID); err != nil {
				_ = projectRows.Close()
				return err
			}
			item.Profiles[index].ProjectIDs = append(item.Profiles[index].ProjectIDs, projectID)
		}
		if err := projectRows.Close(); err != nil {
			return err
		}
		if err := projectRows.Err(); err != nil {
			return err
		}
	}
	return nil
}

func (r *WorkspaceRepository) loadWorkspaceRecipes(ctx context.Context, item *domain.Workspace) error {
	rows, err := r.database.connection.QueryContext(ctx, `SELECT id, name, kind, project_id, target,
        arguments_json, sort_order FROM workspace_recipes WHERE workspace_id = ? ORDER BY sort_order, id`, item.ID)
	if err != nil {
		return fmt.Errorf("list workspace recipes: %w", err)
	}
	defer func() { _ = rows.Close() }()
	item.Recipes = []domain.Recipe{}
	for rows.Next() {
		var recipe domain.Recipe
		var projectID sql.NullString
		var arguments string
		if err := rows.Scan(&recipe.ID, &recipe.Name, &recipe.Kind, &projectID, &recipe.Target, &arguments, &recipe.Order); err != nil {
			return err
		}
		recipe.ProjectID = projectID.String
		if err := json.Unmarshal([]byte(arguments), &recipe.Arguments); err != nil {
			return fmt.Errorf("decode workspace recipe arguments: %w", err)
		}
		item.Recipes = append(item.Recipes, recipe)
	}
	return rows.Err()
}

func overlayMemberStatus(item *domain.Workspace, execution domain.ExecutionSummary) {
	statuses := make(map[string]domain.ProjectResult, len(execution.Projects))
	for _, result := range execution.Projects {
		statuses[result.ProjectID] = result
	}
	for index := range item.Members {
		if result, ok := statuses[item.Members[index].ProjectID]; ok {
			item.Members[index].Status = result.Status
			item.Members[index].Message = result.Message
		}
	}
}

func (r *WorkspaceRepository) latestExecution(ctx context.Context, workspaceID string) (domain.ExecutionSummary, error) {
	var id string
	err := r.database.connection.QueryRowContext(ctx, `SELECT id FROM workspace_runs
        WHERE workspace_id = ? ORDER BY started_at DESC, id DESC LIMIT 1`, workspaceID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ExecutionSummary{}, workspace.ErrNotFound
	}
	if err != nil {
		return domain.ExecutionSummary{}, fmt.Errorf("get latest workspace execution id: %w", err)
	}
	return r.execution(ctx, id)
}

func (r *WorkspaceRepository) execution(ctx context.Context, id string) (domain.ExecutionSummary, error) {
	var result domain.ExecutionSummary
	var removeData int
	var startedAt string
	var finishedAt sql.NullString
	err := r.database.connection.QueryRowContext(ctx, `SELECT id, workspace_id, kind, state, failure_policy,
        profile_id, remove_data, error_message, started_at, finished_at FROM workspace_runs WHERE id = ?`, id).Scan(
		&result.ID, &result.WorkspaceID, &result.Kind, &result.State, &result.Policy, &result.ProfileID,
		&removeData, &result.ErrorMessage, &startedAt, &finishedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ExecutionSummary{}, workspace.ErrNotFound
	}
	if err != nil {
		return domain.ExecutionSummary{}, fmt.Errorf("get workspace execution: %w", err)
	}
	result.RemoveData = removeData == 1
	result.StartedAt, err = parseTime(startedAt)
	if err != nil {
		return domain.ExecutionSummary{}, err
	}
	if finishedAt.Valid {
		finished, err := parseTime(finishedAt.String)
		if err != nil {
			return domain.ExecutionSummary{}, err
		}
		result.FinishedAt = &finished
	}
	result.Projects, err = r.executionProjects(ctx, id)
	return result, err
}

func (r *WorkspaceRepository) executionProjects(ctx context.Context, runID string) ([]domain.ProjectResult, error) {
	rows, err := r.database.connection.QueryContext(ctx, `SELECT project_id, role, state, message, sort_order,
        started_at, finished_at FROM workspace_run_projects WHERE run_id = ? ORDER BY sort_order, project_id`, runID)
	if err != nil {
		return nil, fmt.Errorf("list workspace execution projects: %w", err)
	}
	defer func() { _ = rows.Close() }()
	result := []domain.ProjectResult{}
	for rows.Next() {
		var item domain.ProjectResult
		var startedAt, finishedAt sql.NullString
		if err := rows.Scan(&item.ProjectID, &item.Role, &item.Status, &item.Message, &item.Order, &startedAt, &finishedAt); err != nil {
			return nil, err
		}
		if item.StartedAt, err = parseNullableTime(startedAt); err != nil {
			return nil, err
		}
		if item.FinishedAt, err = parseNullableTime(finishedAt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
