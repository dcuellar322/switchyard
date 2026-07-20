package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	workspace "switchyard.dev/switchyard/internal/workspace/application"
	"switchyard.dev/switchyard/internal/workspace/domain"
)

// WorkspaceRepository persists workspace graphs and coordinated execution snapshots.
type WorkspaceRepository struct{ database *Database }

// NewWorkspaceRepository creates a durable workspace adapter.
func NewWorkspaceRepository(database *Database) *WorkspaceRepository {
	return &WorkspaceRepository{database: database}
}

// Create atomically stores a validated workspace aggregate.
func (r *WorkspaceRepository) Create(ctx context.Context, item domain.Workspace) error {
	tx, err := r.database.connection.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin workspace create: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, `INSERT INTO workspaces
        (id, name, description, default_failure_policy, default_profile_id, revision, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, item.ID, item.Name, item.Description, item.DefaultFailurePolicy,
		item.DefaultProfileID, item.Revision, formatTime(item.CreatedAt), formatTime(item.UpdatedAt))
	if err != nil {
		return fmt.Errorf("insert workspace: %w", err)
	}
	if err := insertWorkspaceChildren(ctx, tx, item); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit workspace create: %w", err)
	}
	return nil
}

// Update atomically replaces editable graph records after a revision check.
func (r *WorkspaceRepository) Update(ctx context.Context, item domain.Workspace, expectedRevision int64) error {
	tx, err := r.database.connection.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin workspace update: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, `UPDATE workspaces SET name=?, description=?, default_failure_policy=?,
        default_profile_id=?, revision=?, updated_at=? WHERE id=? AND revision=?`, item.Name, item.Description,
		item.DefaultFailurePolicy, item.DefaultProfileID, item.Revision, formatTime(item.UpdatedAt), item.ID, expectedRevision)
	if err != nil {
		return fmt.Errorf("update workspace: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read workspace update count: %w", err)
	}
	if rows != 1 {
		return workspace.ErrRevisionConflict
	}
	deleteStatements := []struct{ name, query string }{
		{name: "workspace dependencies", query: `DELETE FROM workspace_dependencies WHERE workspace_id = ?`},
		{name: "workspace profile projects", query: `DELETE FROM workspace_profile_projects WHERE workspace_id = ?`},
		{name: "workspace recipes", query: `DELETE FROM workspace_recipes WHERE workspace_id = ?`},
		{name: "workspace profiles", query: `DELETE FROM workspace_profiles WHERE workspace_id = ?`},
		{name: "workspace projects", query: `DELETE FROM workspace_projects WHERE workspace_id = ?`},
	}
	for _, statement := range deleteStatements {
		if _, err := tx.ExecContext(ctx, statement.query, item.ID); err != nil {
			return fmt.Errorf("clear %s: %w", statement.name, err)
		}
	}
	if err := insertWorkspaceChildren(ctx, tx, item); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit workspace update: %w", err)
	}
	return nil
}

// Get rehydrates one graph and overlays the latest project-visible execution state.
func (r *WorkspaceRepository) Get(ctx context.Context, id string) (domain.Workspace, error) {
	var item domain.Workspace
	var createdAt, updatedAt string
	err := r.database.connection.QueryRowContext(ctx, `SELECT id, name, description, default_failure_policy,
        default_profile_id, revision, created_at, updated_at FROM workspaces WHERE id = ?`, id).Scan(
		&item.ID, &item.Name, &item.Description, &item.DefaultFailurePolicy, &item.DefaultProfileID,
		&item.Revision, &createdAt, &updatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Workspace{}, workspace.ErrNotFound
	}
	if err != nil {
		return domain.Workspace{}, fmt.Errorf("get workspace: %w", err)
	}
	item.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return domain.Workspace{}, fmt.Errorf("parse workspace creation time: %w", err)
	}
	item.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return domain.Workspace{}, fmt.Errorf("parse workspace update time: %w", err)
	}
	if err := r.loadWorkspaceChildren(ctx, &item); err != nil {
		return domain.Workspace{}, err
	}
	lastRun, err := r.latestExecution(ctx, id)
	if err != nil && !errors.Is(err, workspace.ErrNotFound) {
		return domain.Workspace{}, err
	}
	if err == nil {
		item.LastRun = &lastRun
		overlayMemberStatus(&item, lastRun)
	}
	return item, nil
}

// List returns all workspaces in stable name order.
func (r *WorkspaceRepository) List(ctx context.Context) ([]domain.Workspace, error) {
	rows, err := r.database.connection.QueryContext(ctx, `SELECT id FROM workspaces ORDER BY name COLLATE NOCASE, id`)
	if err != nil {
		return nil, fmt.Errorf("list workspace ids: %w", err)
	}
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	items := make([]domain.Workspace, 0, len(ids))
	for _, id := range ids {
		item, err := r.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

// Delete removes coordination metadata only; project and runtime data remain untouched.
func (r *WorkspaceRepository) Delete(ctx context.Context, id string) error {
	result, err := r.database.connection.ExecContext(ctx, `DELETE FROM workspaces WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete workspace: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read workspace delete count: %w", err)
	}
	if rows != 1 {
		return workspace.ErrNotFound
	}
	return nil
}

func insertWorkspaceChildren(ctx context.Context, tx *sql.Tx, item domain.Workspace) error {
	for _, member := range item.Members {
		timeoutSeconds := int64((member.HealthTimeout + time.Second - 1) / time.Second)
		_, err := tx.ExecContext(ctx, `INSERT INTO workspace_projects
            (workspace_id, project_id, role, sort_order, health_gate, health_timeout_seconds)
            VALUES (?, ?, ?, ?, ?, ?)`, item.ID, member.ProjectID, member.Role, member.Order,
			boolInteger(member.HealthGate), timeoutSeconds)
		if err != nil {
			return fmt.Errorf("insert workspace member %s: %w", member.ProjectID, err)
		}
	}
	for _, edge := range item.Dependencies {
		_, err := tx.ExecContext(ctx, `INSERT INTO workspace_dependencies
            (workspace_id, project_id, depends_on_project_id) VALUES (?, ?, ?)`,
			item.ID, edge.ProjectID, edge.DependsOnProjectID)
		if err != nil {
			return fmt.Errorf("insert workspace dependency: %w", err)
		}
	}
	if err := insertWorkspaceProfiles(ctx, tx, item); err != nil {
		return err
	}
	return insertWorkspaceRecipes(ctx, tx, item)
}

func insertWorkspaceProfiles(ctx context.Context, tx *sql.Tx, item domain.Workspace) error {
	for _, profile := range item.Profiles {
		if profile.MemoryBudgetBytes > math.MaxInt64 {
			return fmt.Errorf("workspace profile %s memory budget exceeds sqlite integer range", profile.ID)
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO workspace_profiles
            (workspace_id, id, name, description, max_parallel, low_memory, memory_budget_bytes)
            VALUES (?, ?, ?, ?, ?, ?, ?)`, item.ID, profile.ID, profile.Name, profile.Description,
			profile.MaxParallel, boolInteger(profile.LowMemory), int64(profile.MemoryBudgetBytes))
		if err != nil {
			return fmt.Errorf("insert workspace profile %s: %w", profile.ID, err)
		}
		for order, projectID := range profile.ProjectIDs {
			_, err = tx.ExecContext(ctx, `INSERT INTO workspace_profile_projects
                (workspace_id, profile_id, project_id, sort_order) VALUES (?, ?, ?, ?)`,
				item.ID, profile.ID, projectID, order)
			if err != nil {
				return fmt.Errorf("insert workspace profile project: %w", err)
			}
		}
	}
	return nil
}

func insertWorkspaceRecipes(ctx context.Context, tx *sql.Tx, item domain.Workspace) error {
	for _, recipe := range item.Recipes {
		arguments, err := json.Marshal(recipe.Arguments)
		if err != nil {
			return fmt.Errorf("encode workspace recipe arguments: %w", err)
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO workspace_recipes
            (workspace_id, id, name, kind, project_id, target, arguments_json, sort_order)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, item.ID, recipe.ID, recipe.Name, recipe.Kind,
			nullString(recipe.ProjectID), recipe.Target, arguments, recipe.Order)
		if err != nil {
			return fmt.Errorf("insert workspace recipe %s: %w", recipe.ID, err)
		}
	}
	return nil
}

func boolInteger(value bool) int {
	if value {
		return 1
	}
	return 0
}
