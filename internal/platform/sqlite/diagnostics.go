package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	diagnosticsApplication "switchyard.dev/switchyard/internal/diagnostics/application"
	"switchyard.dev/switchyard/internal/diagnostics/domain"
)

// DiagnosticRepository persists diagnostic review and automation state.
type DiagnosticRepository struct{ database *Database }

// NewDiagnosticRepository creates the diagnostics SQLite adapter.
func NewDiagnosticRepository(database *Database) *DiagnosticRepository {
	return &DiagnosticRepository{database: database}
}

// SaveDiagnosis persists the complete bounded diagnostic receipt.
func (r *DiagnosticRepository) SaveDiagnosis(ctx context.Context, diagnosis domain.Diagnosis) error {
	encoded, err := json.Marshal(diagnosis)
	if err != nil {
		return err
	}
	_, err = r.database.connection.ExecContext(ctx, `INSERT INTO diagnoses
        (id, project_id, provider, bundle_sha256, diagnosis_json, generated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		diagnosis.ID, diagnosis.ProjectID, diagnosis.Provider, diagnosis.Bundle.SHA256, encoded, formatTime(diagnosis.GeneratedAt))
	if err != nil {
		return fmt.Errorf("save diagnosis: %w", err)
	}
	// Periodic diagnosis is intentionally bounded. Feedback keeps its referenced
	// diagnosis alive through the foreign-key constraint; the newest unreviewed
	// receipts remain available for local inspection.
	_, err = r.database.connection.ExecContext(ctx, `DELETE FROM diagnoses
        WHERE project_id = ? AND id NOT IN (
          SELECT id FROM diagnoses WHERE project_id = ? ORDER BY generated_at DESC, id DESC LIMIT 100
        ) AND id NOT IN (SELECT diagnosis_id FROM diagnostic_feedback)`, diagnosis.ProjectID, diagnosis.ProjectID)
	if err != nil {
		return fmt.Errorf("apply diagnosis retention: %w", err)
	}
	return nil
}

// GetDiagnosis returns one complete diagnosis.
func (r *DiagnosticRepository) GetDiagnosis(ctx context.Context, id string) (domain.Diagnosis, error) {
	return decodeDiagnosis(r.database.connection.QueryRowContext(ctx, `SELECT diagnosis_json FROM diagnoses WHERE id = ?`, id))
}

// LatestDiagnosis returns the newest diagnosis for one project.
func (r *DiagnosticRepository) LatestDiagnosis(ctx context.Context, projectID string) (domain.Diagnosis, error) {
	return decodeDiagnosis(r.database.connection.QueryRowContext(ctx, `SELECT diagnosis_json FROM diagnoses
        WHERE project_id = ? ORDER BY generated_at DESC, id DESC LIMIT 1`, projectID))
}

func decodeDiagnosis(row *sql.Row) (domain.Diagnosis, error) {
	var encoded string
	if err := row.Scan(&encoded); errors.Is(err, sql.ErrNoRows) {
		return domain.Diagnosis{}, diagnosticsApplication.ErrDiagnosisNotFound
	} else if err != nil {
		return domain.Diagnosis{}, err
	}
	var diagnosis domain.Diagnosis
	if err := json.Unmarshal([]byte(encoded), &diagnosis); err != nil {
		return domain.Diagnosis{}, fmt.Errorf("decode diagnosis: %w", err)
	}
	return diagnosis, nil
}

// SaveFeedback records a local review without telemetry.
func (r *DiagnosticRepository) SaveFeedback(ctx context.Context, feedback domain.Feedback) error {
	_, err := r.database.connection.ExecContext(ctx, `INSERT INTO diagnostic_feedback
        (id, diagnosis_id, hypothesis_id, verdict, note, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		feedback.ID, feedback.DiagnosisID, feedback.HypothesisID, feedback.Verdict, feedback.Note, formatTime(feedback.CreatedAt))
	return err
}

// UpsertNotification increments one project/code warning and reopens it.
func (r *DiagnosticRepository) UpsertNotification(ctx context.Context, notification domain.Notification) (domain.Notification, error) {
	_, err := r.database.connection.ExecContext(ctx, `INSERT INTO diagnostic_notifications
        (id, project_id, code, title, detail, occurrences, first_seen_at, last_seen_at)
        VALUES (?, ?, ?, ?, ?, 1, ?, ?)
        ON CONFLICT(project_id, code) DO UPDATE SET title = excluded.title, detail = excluded.detail,
          occurrences = diagnostic_notifications.occurrences + 1, last_seen_at = excluded.last_seen_at,
          acknowledged_at = NULL`, notification.ID, notification.ProjectID, notification.Code, notification.Title,
		notification.Detail, formatTime(notification.FirstSeenAt), formatTime(notification.LastSeenAt))
	if err != nil {
		return domain.Notification{}, fmt.Errorf("upsert diagnostic notification: %w", err)
	}
	return r.getNotification(ctx, notification.ID, notification.ProjectID, notification.Code)
}

// ListNotifications returns newest warnings first.
func (r *DiagnosticRepository) ListNotifications(ctx context.Context, projectID string, includeAcknowledged bool, limit int) ([]domain.Notification, error) {
	rows, err := r.database.connection.QueryContext(ctx, `SELECT id, project_id, code, title, detail, occurrences,
        first_seen_at, last_seen_at, acknowledged_at FROM diagnostic_notifications
        WHERE (? = '' OR project_id = ?) AND (? = 1 OR acknowledged_at IS NULL)
        ORDER BY last_seen_at DESC, id DESC LIMIT ?`, projectID, projectID, boolInteger(includeAcknowledged), limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	result := []domain.Notification{}
	for rows.Next() {
		notification, err := scanNotification(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, notification)
	}
	return result, rows.Err()
}

// AcknowledgeNotification marks one warning reviewed.
func (r *DiagnosticRepository) AcknowledgeNotification(ctx context.Context, id string, at time.Time) (domain.Notification, error) {
	result, err := r.database.connection.ExecContext(ctx, `UPDATE diagnostic_notifications SET acknowledged_at = ? WHERE id = ?`, formatTime(at), id)
	if err != nil {
		return domain.Notification{}, err
	}
	count, _ := result.RowsAffected()
	if count != 1 {
		return domain.Notification{}, diagnosticsApplication.ErrDiagnosisNotFound
	}
	return r.getNotification(ctx, id, "", "")
}

func (r *DiagnosticRepository) getNotification(ctx context.Context, id, projectID, code string) (domain.Notification, error) {
	row := r.database.connection.QueryRowContext(ctx, `SELECT id, project_id, code, title, detail, occurrences,
        first_seen_at, last_seen_at, acknowledged_at FROM diagnostic_notifications
        WHERE id = ? OR (? <> '' AND project_id = ? AND code = ?) LIMIT 1`, id, projectID, projectID, code)
	return scanNotification(row)
}

func scanNotification(row rowScanner) (domain.Notification, error) {
	var result domain.Notification
	var first, last string
	var acknowledged sql.NullString
	if err := row.Scan(&result.ID, &result.ProjectID, &result.Code, &result.Title, &result.Detail, &result.Occurrences, &first, &last, &acknowledged); err != nil {
		return domain.Notification{}, err
	}
	result.FirstSeenAt, _ = parseTime(first)
	result.LastSeenAt, _ = parseTime(last)
	result.AcknowledgedAt, _ = parseNullableTime(acknowledged)
	return result, nil
}

// SaveRecipe persists a separately reviewed disabled recipe.
func (r *DiagnosticRepository) SaveRecipe(ctx context.Context, recipe domain.Recipe) error {
	_, err := r.database.connection.ExecContext(ctx, `INSERT INTO automation_recipes
        (id, project_id, name, trigger_code, action_id, enabled, cooldown_seconds, max_runs_per_day,
         runs_today, runs_day, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0, '', ?, ?)`,
		recipe.ID, recipe.ProjectID, recipe.Name, recipe.TriggerCode, recipe.ActionID, boolInteger(recipe.Enabled),
		recipe.CooldownSeconds, recipe.MaxRunsPerDay, formatTime(recipe.CreatedAt), formatTime(recipe.UpdatedAt))
	return err
}

// GetRecipe returns one saved recipe.
func (r *DiagnosticRepository) GetRecipe(ctx context.Context, id string) (domain.Recipe, error) {
	row := r.database.connection.QueryRowContext(ctx, recipeSelect+` WHERE id = ?`, id)
	return scanRecipe(row)
}

// ListRecipes includes disabled recipes in stable order.
func (r *DiagnosticRepository) ListRecipes(ctx context.Context, projectID string) ([]domain.Recipe, error) {
	rows, err := r.database.connection.QueryContext(ctx, recipeSelect+` WHERE (? = '' OR project_id = ?)
        ORDER BY enabled DESC, name COLLATE NOCASE, id`, projectID, projectID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	result := []domain.Recipe{}
	for rows.Next() {
		recipe, err := scanRecipe(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, recipe)
	}
	return result, rows.Err()
}

// UpdateRecipeEnabled changes only the reviewed enabled state.
func (r *DiagnosticRepository) UpdateRecipeEnabled(ctx context.Context, id string, enabled bool, at time.Time) (domain.Recipe, error) {
	result, err := r.database.connection.ExecContext(ctx, `UPDATE automation_recipes SET enabled = ?, updated_at = ? WHERE id = ?`, boolInteger(enabled), formatTime(at), id)
	if err != nil {
		return domain.Recipe{}, err
	}
	count, _ := result.RowsAffected()
	if count != 1 {
		return domain.Recipe{}, diagnosticsApplication.ErrRecipeNotFound
	}
	return r.GetRecipe(ctx, id)
}

// MarkRecipeRun advances cooldown and per-day counters after durable dispatch.
func (r *DiagnosticRepository) MarkRecipeRun(ctx context.Context, id string, at time.Time) (domain.Recipe, error) {
	day := at.UTC().Format(time.DateOnly)
	result, err := r.database.connection.ExecContext(ctx, `UPDATE automation_recipes SET last_run_at = ?,
        runs_today = CASE WHEN runs_day = ? THEN runs_today + 1 ELSE 1 END, runs_day = ?, updated_at = ? WHERE id = ?`,
		formatTime(at), day, day, formatTime(at), id)
	if err != nil {
		return domain.Recipe{}, err
	}
	count, _ := result.RowsAffected()
	if count != 1 {
		return domain.Recipe{}, diagnosticsApplication.ErrRecipeNotFound
	}
	return r.GetRecipe(ctx, id)
}

const recipeSelect = `SELECT id, project_id, name, trigger_code, action_id, enabled, cooldown_seconds,
    max_runs_per_day, last_run_at, runs_today, runs_day, created_at, updated_at FROM automation_recipes`

func scanRecipe(row rowScanner) (domain.Recipe, error) {
	var recipe domain.Recipe
	var enabled int
	var last sql.NullString
	var created, updated string
	if err := row.Scan(&recipe.ID, &recipe.ProjectID, &recipe.Name, &recipe.TriggerCode, &recipe.ActionID, &enabled,
		&recipe.CooldownSeconds, &recipe.MaxRunsPerDay, &last, &recipe.RunsToday, &recipe.RunsDay, &created, &updated); errors.Is(err, sql.ErrNoRows) {
		return domain.Recipe{}, diagnosticsApplication.ErrRecipeNotFound
	} else if err != nil {
		return domain.Recipe{}, err
	}
	recipe.Enabled = enabled == 1
	recipe.LastRunAt, _ = parseNullableTime(last)
	recipe.CreatedAt, _ = parseTime(created)
	recipe.UpdatedAt, _ = parseTime(updated)
	return recipe, nil
}
