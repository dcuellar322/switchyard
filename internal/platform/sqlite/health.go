package sqlite

import (
	"context"
	"fmt"
	"time"

	observability "switchyard.dev/switchyard/internal/observability/domain"
)

// HealthRepository stores bounded health history and returns one latest sample per check.
type HealthRepository struct{ database *Database }

// NewHealthRepository creates the SQLite health adapter.
func NewHealthRepository(database *Database) *HealthRepository {
	return &HealthRepository{database: database}
}

// AppendHealth records one sanitized health result.
func (r *HealthRepository) AppendHealth(ctx context.Context, result observability.HealthResult) error {
	_, err := r.database.connection.ExecContext(ctx, `INSERT INTO health_samples
        (project_id, service_id, check_id, check_type, status, severity, required, latency_ms, message, observed_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, result.ProjectID, result.ServiceID, result.CheckID, result.Type,
		result.Status, result.Severity, result.Required, result.LatencyMS, result.Message, formatTime(result.ObservedAt))
	if err != nil {
		return fmt.Errorf("append health sample: %w", err)
	}
	return nil
}

// LatestHealth returns one newest result for each declared project check.
func (r *HealthRepository) LatestHealth(ctx context.Context, projectID string) ([]observability.HealthResult, error) {
	rows, err := r.database.connection.QueryContext(ctx, `SELECT project_id, service_id, check_id, check_type,
        status, severity, required, latency_ms, message, observed_at FROM (
          SELECT *, ROW_NUMBER() OVER (PARTITION BY service_id, check_id ORDER BY observed_at DESC, id DESC) AS rank
          FROM health_samples WHERE project_id = ?
        ) WHERE rank = 1 ORDER BY service_id, check_id`, projectID)
	if err != nil {
		return nil, fmt.Errorf("query latest health: %w", err)
	}
	defer func() { _ = rows.Close() }()
	results := []observability.HealthResult{}
	for rows.Next() {
		var item observability.HealthResult
		var observed string
		if err := rows.Scan(&item.ProjectID, &item.ServiceID, &item.CheckID, &item.Type, &item.Status,
			&item.Severity, &item.Required, &item.LatencyMS, &item.Message, &observed); err != nil {
			return nil, err
		}
		item.ObservedAt, err = parseTime(observed)
		if err != nil {
			return nil, err
		}
		results = append(results, item)
	}
	return results, rows.Err()
}

// PruneHealth caps health history by age.
func (r *HealthRepository) PruneHealth(ctx context.Context, before time.Time) error {
	_, err := r.database.connection.ExecContext(ctx, `DELETE FROM health_samples WHERE observed_at < ?`, formatTime(before))
	if err != nil {
		return fmt.Errorf("prune health samples: %w", err)
	}
	return nil
}
