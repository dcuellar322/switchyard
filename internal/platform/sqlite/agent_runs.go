package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	agents "switchyard.dev/switchyard/internal/agents/application"
)

// AgentRunRepository persists assisted-onboarding evidence receipts and review results.
type AgentRunRepository struct{ database *Database }

// NewAgentRunRepository creates the SQLite run adapter.
func NewAgentRunRepository(database *Database) *AgentRunRepository {
	return &AgentRunRepository{database: database}
}

// Start creates or restarts a recoverable operation run without losing its stable identity.
func (r *AgentRunRepository) Start(ctx context.Context, run agents.Run) error {
	limits, _ := json.Marshal(run.Limits)
	fields, _ := json.Marshal(run.Fields)
	conflicts, _ := json.Marshal(run.Conflicts)
	warnings, _ := json.Marshal(run.Warnings)
	dryRun, _ := json.Marshal(run.DryRun)
	usage, _ := json.Marshal(run.Usage)
	_, err := r.database.connection.ExecContext(ctx, `INSERT INTO manifest_ai_runs
		(operation_id, project_id, source_proposal_id, provider, model, state, bundle_json, bundle_sha256,
		 limits_json, fields_json, conflicts_json, warnings_json, dry_run_json, usage_json, started_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(operation_id) DO UPDATE SET state = excluded.state, model = excluded.model,
		 bundle_json = excluded.bundle_json, bundle_sha256 = excluded.bundle_sha256, limits_json = excluded.limits_json,
		 fields_json = '[]', conflicts_json = '[]', warnings_json = '[]', dry_run_json = '{}', usage_json = '{}',
		 result_proposal_id = NULL, error_code = NULL, error_message = NULL, started_at = excluded.started_at, finished_at = NULL`,
		run.OperationID, run.ProjectID, run.SourceProposalID, run.Provider, run.Model, run.State, run.Bundle, run.BundleSHA256,
		limits, fields, conflicts, warnings, dryRun, usage, formatTime(run.StartedAt))
	if err != nil {
		return fmt.Errorf("start assisted onboarding run: %w", err)
	}
	return nil
}

// Finish records terminal result data without persisting raw provider output.
func (r *AgentRunRepository) Finish(ctx context.Context, run agents.Run) error {
	fields, _ := json.Marshal(run.Fields)
	conflicts, _ := json.Marshal(run.Conflicts)
	warnings, _ := json.Marshal(run.Warnings)
	dryRun, _ := json.Marshal(run.DryRun)
	usage, _ := json.Marshal(run.Usage)
	var finished any
	if run.FinishedAt != nil {
		finished = formatTime(*run.FinishedAt)
	}
	result, err := r.database.connection.ExecContext(ctx, `UPDATE manifest_ai_runs SET
		result_proposal_id = ?, model = ?, state = ?, fields_json = ?, conflicts_json = ?, warnings_json = ?,
		dry_run_json = ?, usage_json = ?, error_code = ?, error_message = ?, finished_at = ? WHERE operation_id = ?`,
		nullIfEmpty(run.ResultProposalID), run.Model, run.State, fields, conflicts, warnings, dryRun, usage,
		nullIfEmpty(run.ErrorCode), nullIfEmpty(run.ErrorMessage), finished, run.OperationID)
	if err != nil {
		return fmt.Errorf("finish assisted onboarding run: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil || rows != 1 {
		return agents.ErrRunNotFound
	}
	return nil
}

// Get rehydrates one durable run receipt.
func (r *AgentRunRepository) Get(ctx context.Context, operationID string) (agents.Run, error) {
	row := r.database.connection.QueryRowContext(ctx, `SELECT operation_id, project_id, source_proposal_id,
		result_proposal_id, provider, model, state, bundle_json, bundle_sha256, limits_json, fields_json,
		conflicts_json, warnings_json, dry_run_json, usage_json, error_code, error_message, started_at, finished_at
		FROM manifest_ai_runs WHERE operation_id = ?`, operationID)
	var run agents.Run
	var resultID, errorCode, errorMessage, finished sql.NullString
	var bundle, limits, fields, conflicts, warnings, dryRun, usage, started string
	if err := row.Scan(&run.OperationID, &run.ProjectID, &run.SourceProposalID, &resultID, &run.Provider, &run.Model,
		&run.State, &bundle, &run.BundleSHA256, &limits, &fields, &conflicts, &warnings, &dryRun, &usage,
		&errorCode, &errorMessage, &started, &finished); errors.Is(err, sql.ErrNoRows) {
		return agents.Run{}, agents.ErrRunNotFound
	} else if err != nil {
		return agents.Run{}, fmt.Errorf("read assisted onboarding run: %w", err)
	}
	run.ResultProposalID, run.ErrorCode, run.ErrorMessage = resultID.String, errorCode.String, errorMessage.String
	run.Bundle = json.RawMessage(bundle)
	for value, target := range map[string]any{limits: &run.Limits, fields: &run.Fields, conflicts: &run.Conflicts, warnings: &run.Warnings, dryRun: &run.DryRun, usage: &run.Usage} {
		if err := json.Unmarshal([]byte(value), target); err != nil {
			return agents.Run{}, fmt.Errorf("decode assisted onboarding run: %w", err)
		}
	}
	run.StartedAt, _ = parseTime(started)
	if finished.Valid {
		parsed, err := parseTime(finished.String)
		if err == nil {
			run.FinishedAt = &parsed
		}
	}
	if run.Fields == nil {
		run.Fields = []agents.FieldReview{}
	}
	if run.Conflicts == nil {
		run.Conflicts = []agents.Conflict{}
	}
	if run.Warnings == nil {
		run.Warnings = []string{}
	}
	if run.DryRun.Errors == nil {
		run.DryRun.Errors = []string{}
	}
	if run.DryRun.Warnings == nil {
		run.DryRun.Warnings = []string{}
	}
	return run, nil
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}
