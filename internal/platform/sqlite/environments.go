package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	environmentApplication "switchyard.dev/switchyard/internal/environments/application"
	"switchyard.dev/switchyard/internal/environments/domain"
)

// EnvironmentRepository persists registered worktrees and exact runtime allocations.
type EnvironmentRepository struct{ database *Database }

// NewEnvironmentRepository creates the durable project-environment registry.
func NewEnvironmentRepository(database *Database) *EnvironmentRepository {
	return &EnvironmentRepository{database: database}
}

// List returns every environment and its exact port leases in stable order.
func (r *EnvironmentRepository) List(ctx context.Context) ([]domain.Environment, error) {
	rows, err := r.database.connection.QueryContext(ctx, `SELECT id, project_id, name, path, head, branch,
        detached, bare, locked, is_primary, availability, unavailable_reason, runtime_state, hostname, target,
        compose_project_name, port_lease_namespace, port_offset, registered_at, last_observed_at, updated_at
        FROM project_environments ORDER BY project_id, is_primary DESC, name, id`)
	if err != nil {
		return nil, fmt.Errorf("list project environments: %w", err)
	}
	defer func() { _ = rows.Close() }()
	result := make([]domain.Environment, 0)
	for rows.Next() {
		environment, scanErr := scanEnvironment(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, environment)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for index := range result {
		leases, err := r.listLeases(ctx, r.database.connection, result[index].ID)
		if err != nil {
			return nil, err
		}
		result[index].Allocation.PortLeases = leases
	}
	return result, nil
}

// Get returns one registered environment with its exact leases.
func (r *EnvironmentRepository) Get(ctx context.Context, id string) (domain.Environment, error) {
	row := r.database.connection.QueryRowContext(ctx, `SELECT id, project_id, name, path, head, branch,
        detached, bare, locked, is_primary, availability, unavailable_reason, runtime_state, hostname, target,
        compose_project_name, port_lease_namespace, port_offset, registered_at, last_observed_at, updated_at
        FROM project_environments WHERE id = ?`, id)
	environment, err := scanEnvironment(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Environment{}, environmentApplication.ErrNotFound
	}
	if err != nil {
		return domain.Environment{}, err
	}
	environment.Allocation.PortLeases, err = r.listLeases(ctx, r.database.connection, id)
	return environment, err
}

// ReplaceProject atomically reconciles one project's observed worktrees.
func (r *EnvironmentRepository) ReplaceProject(ctx context.Context, projectID string, environments []domain.Environment) error {
	tx, err := r.database.connection.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM project_environments WHERE project_id = ?`, projectID); err != nil {
		return fmt.Errorf("clear project environments: %w", err)
	}
	for _, environment := range environments {
		if environment.ProjectID != projectID {
			return errors.New("replacement environment belongs to another project")
		}
		if err := insertEnvironment(ctx, tx, environment); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit project environments: %w", err)
	}
	return nil
}

// Update applies a complete validated runtime configuration atomically.
func (r *EnvironmentRepository) Update(ctx context.Context, environment domain.Environment) error {
	if err := environment.Validate(); err != nil {
		return err
	}
	tx, err := r.database.connection.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, `UPDATE project_environments SET name=?, path=?, head=?, branch=?, detached=?, bare=?, locked=?,
        is_primary=?, availability=?, unavailable_reason=?, runtime_state=?, hostname=?, target=?, compose_project_name=?,
        port_lease_namespace=?, port_offset=?, registered_at=?, last_observed_at=?, updated_at=? WHERE id=? AND project_id=?`,
		environment.Name, environment.Path, environment.Head, environment.Branch, boolInt(environment.Detached), boolInt(environment.Bare),
		boolInt(environment.Locked), boolInt(environment.Primary), environment.Availability, environment.UnavailableReason,
		environment.State, environment.Hostname, environment.Target, environment.Allocation.ComposeProjectName,
		environment.Allocation.PortLeaseNamespace, environment.Allocation.PortOffset, formatTime(environment.RegisteredAt),
		formatTime(environment.LastObservedAt), formatTime(environment.UpdatedAt), environment.ID, environment.ProjectID)
	if err != nil {
		return fmt.Errorf("update project environment: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows != 1 {
		return environmentApplication.ErrNotFound
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM environment_port_leases WHERE environment_id = ?`, environment.ID); err != nil {
		return err
	}
	if err := insertLeases(ctx, tx, environment.ID, environment.Allocation.PortLeases); err != nil {
		return err
	}
	return tx.Commit()
}

type environmentRowScanner interface{ Scan(...any) error }

func scanEnvironment(row environmentRowScanner) (domain.Environment, error) {
	var environment domain.Environment
	var detached, bare, locked, primary int
	var registered, observed, updated string
	err := row.Scan(
		&environment.ID, &environment.ProjectID, &environment.Name, &environment.Path, &environment.Head, &environment.Branch,
		&detached, &bare, &locked, &primary, &environment.Availability, &environment.UnavailableReason,
		&environment.State, &environment.Hostname, &environment.Target, &environment.Allocation.ComposeProjectName,
		&environment.Allocation.PortLeaseNamespace, &environment.Allocation.PortOffset, &registered, &observed, &updated,
	)
	if err != nil {
		return domain.Environment{}, err
	}
	environment.Detached, environment.Bare, environment.Locked, environment.Primary = detached == 1, bare == 1, locked == 1, primary == 1
	if environment.RegisteredAt, err = parseTime(registered); err != nil {
		return domain.Environment{}, err
	}
	if environment.LastObservedAt, err = parseTime(observed); err != nil {
		return domain.Environment{}, err
	}
	if environment.UpdatedAt, err = parseTime(updated); err != nil {
		return domain.Environment{}, err
	}
	return environment, nil
}

func insertEnvironment(ctx context.Context, tx *sql.Tx, environment domain.Environment) error {
	if err := environment.Validate(); err != nil {
		return fmt.Errorf("validate project environment: %w", err)
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO project_environments
        (id, project_id, name, path, head, branch, detached, bare, locked, is_primary, availability, unavailable_reason,
         runtime_state, hostname, target, compose_project_name, port_lease_namespace, port_offset, registered_at, last_observed_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		environment.ID, environment.ProjectID, environment.Name, environment.Path, environment.Head, environment.Branch,
		boolInt(environment.Detached), boolInt(environment.Bare), boolInt(environment.Locked), boolInt(environment.Primary),
		environment.Availability, environment.UnavailableReason, environment.State, environment.Hostname, environment.Target,
		environment.Allocation.ComposeProjectName, environment.Allocation.PortLeaseNamespace, environment.Allocation.PortOffset,
		formatTime(environment.RegisteredAt), formatTime(environment.LastObservedAt), formatTime(environment.UpdatedAt))
	if err != nil {
		return fmt.Errorf("insert project environment: %w", err)
	}
	return insertLeases(ctx, tx, environment.ID, environment.Allocation.PortLeases)
}

func insertLeases(ctx context.Context, tx *sql.Tx, environmentID string, leases []domain.PortLease) error {
	for _, lease := range leases {
		_, err := tx.ExecContext(ctx, `INSERT INTO environment_port_leases
            (environment_id, port_id, protocol, target_port, host_port) VALUES (?, ?, ?, ?, ?)`,
			environmentID, lease.PortID, lease.Protocol, lease.TargetPort, lease.HostPort)
		if err != nil {
			return fmt.Errorf("insert environment port lease: %w", err)
		}
	}
	return nil
}

type leaseQuery interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func (r *EnvironmentRepository) listLeases(ctx context.Context, query leaseQuery, environmentID string) ([]domain.PortLease, error) {
	rows, err := query.QueryContext(ctx, `SELECT port_id, protocol, target_port, host_port
        FROM environment_port_leases WHERE environment_id = ? ORDER BY port_id`, environmentID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	leases := make([]domain.PortLease, 0)
	for rows.Next() {
		var lease domain.PortLease
		if err := rows.Scan(&lease.PortID, &lease.Protocol, &lease.TargetPort, &lease.HostPort); err != nil {
			return nil, err
		}
		leases = append(leases, lease)
	}
	return leases, rows.Err()
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
