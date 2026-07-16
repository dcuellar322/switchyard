package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"os"
	"time"

	observability "switchyard.dev/switchyard/internal/observability/domain"
)

// WriteMetricPoints atomically appends idempotent project and service aggregates.
func (d *Database) WriteMetricPoints(ctx context.Context, points []observability.MetricPoint) error {
	if len(points) == 0 {
		return nil
	}
	tx, err := d.connection.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin metric append: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, point := range points {
		if err := writeMetricPoint(ctx, tx, point); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit metric append: %w", err)
	}
	return nil
}

type sqlExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func writeMetricPoint(ctx context.Context, executor sqlExecutor, point observability.MetricPoint) error {
	if point.Timestamp.IsZero() || point.ProjectID == "" {
		return fmt.Errorf("metric point requires project and timestamp")
	}
	if point.SampleCount <= 0 {
		point.SampleCount = 1
	}
	_, err := executor.ExecContext(ctx, `INSERT INTO resource_metric_samples
        (project_id, service_id, sampled_at, resolution_seconds, sample_count,
		 cpu_percent, cpu_max_percent, cpu_available, memory_bytes, memory_max_bytes, memory_limit, memory_available,
         network_rx_bytes, network_tx_bytes, network_available, disk_read_bytes, disk_write_bytes, disk_available,
         process_count, restart_count, health_latency_ms, health_available, storage_bytes, storage_classification, partial)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(project_id, service_id, resolution_seconds, sampled_at) DO NOTHING`,
		point.ProjectID, point.ServiceID, formatTime(point.Timestamp), point.ResolutionSeconds, point.SampleCount,
		point.CPUPercent, max(point.CPUMaxPercent, point.CPUPercent), point.CPUAvailable,
		safeUint(point.MemoryBytes), safeUint(max(point.MemoryMaxBytes, point.MemoryBytes)), safeUint(point.MemoryLimit), point.MemoryAvailable,
		safeUint(point.NetworkRxBytes), safeUint(point.NetworkTxBytes), point.NetworkAvailable, safeUint(point.DiskReadBytes), safeUint(point.DiskWriteBytes), point.DiskAvailable,
		point.ProcessCount, point.RestartCount, point.HealthLatencyMS, point.HealthAvailable, nullableInt(point.StorageBytes), classification(point.StorageClassification), point.Partial)
	if err != nil {
		return fmt.Errorf("append metric point: %w", err)
	}
	return nil
}

// LatestMetricPoints returns every scope captured in the latest raw cycle per project.
func (d *Database) LatestMetricPoints(ctx context.Context, projectIDs []string) (map[string][]observability.MetricPoint, error) {
	result := make(map[string][]observability.MetricPoint, len(projectIDs))
	for _, projectID := range projectIDs {
		rows, err := d.connection.QueryContext(ctx, `SELECT `+metricColumns+` FROM resource_metric_samples
            WHERE project_id = ? AND resolution_seconds = 0 AND sampled_at = (
                SELECT MAX(sampled_at) FROM resource_metric_samples WHERE project_id = ? AND resolution_seconds = 0
            ) ORDER BY service_id`, projectID, projectID)
		if err != nil {
			return nil, fmt.Errorf("query latest metric points: %w", err)
		}
		points, err := scanMetricRows(rows)
		if err != nil {
			return nil, err
		}
		result[projectID] = points
	}
	return result, nil
}

// RecentProjectMetricPoints returns newest raw project aggregates first.
func (d *Database) RecentProjectMetricPoints(ctx context.Context, projectIDs []string, limit int) (map[string][]observability.MetricPoint, error) {
	result := make(map[string][]observability.MetricPoint, len(projectIDs))
	for _, projectID := range projectIDs {
		rows, err := d.connection.QueryContext(ctx, `SELECT `+metricColumns+` FROM resource_metric_samples
            WHERE project_id = ? AND service_id = '' AND resolution_seconds = 0 ORDER BY sampled_at DESC, id DESC LIMIT ?`, projectID, limit)
		if err != nil {
			return nil, fmt.Errorf("query recent metric points: %w", err)
		}
		points, err := scanMetricRows(rows)
		if err != nil {
			return nil, err
		}
		result[projectID] = points
	}
	return result, nil
}

// MetricHistory returns a bounded chronological series from one stored tier.
func (d *Database) MetricHistory(ctx context.Context, projectID, serviceID string, from, to time.Time, resolution, limit int) ([]observability.MetricPoint, error) {
	rows, err := d.connection.QueryContext(ctx, `SELECT `+metricColumns+` FROM resource_metric_samples
        WHERE project_id = ? AND service_id = ? AND resolution_seconds = ? AND sampled_at >= ? AND sampled_at <= ?
        ORDER BY sampled_at, id LIMIT ?`, projectID, serviceID, resolution, formatTime(from), formatTime(to), limit)
	if err != nil {
		return nil, fmt.Errorf("query metric history: %w", err)
	}
	return scanMetricRows(rows)
}

const metricColumns = `project_id, service_id, sampled_at, resolution_seconds, sample_count,
    cpu_percent, cpu_max_percent, cpu_available, memory_bytes, memory_max_bytes, memory_limit, memory_available,
    network_rx_bytes, network_tx_bytes, network_available, disk_read_bytes, disk_write_bytes, disk_available,
    process_count, restart_count, health_latency_ms, health_available, storage_bytes, storage_classification, partial`

type metricRowScanner interface{ Scan(...any) error }

func scanMetric(scanner metricRowScanner) (observability.MetricPoint, error) {
	var point observability.MetricPoint
	var sampledAt, classificationValue string
	var memory, memoryMax, memoryLimit, rx, tx, read, written int64
	var cpuAvailable, memoryAvailable, networkAvailable, diskAvailable, healthAvailable, partial bool
	var storage sql.NullInt64
	err := scanner.Scan(&point.ProjectID, &point.ServiceID, &sampledAt, &point.ResolutionSeconds, &point.SampleCount,
		&point.CPUPercent, &point.CPUMaxPercent, &cpuAvailable, &memory, &memoryMax, &memoryLimit, &memoryAvailable,
		&rx, &tx, &networkAvailable, &read, &written, &diskAvailable,
		&point.ProcessCount, &point.RestartCount, &point.HealthLatencyMS, &healthAvailable, &storage, &classificationValue, &partial)
	if err != nil {
		return observability.MetricPoint{}, err
	}
	point.Timestamp, err = parseTime(sampledAt)
	point.CPUAvailable = cpuAvailable
	point.MemoryBytes, point.MemoryMaxBytes, point.MemoryLimit = uint64(memory), uint64(memoryMax), uint64(memoryLimit)
	point.MemoryAvailable = memoryAvailable
	point.NetworkRxBytes, point.NetworkTxBytes, point.NetworkAvailable = uint64(rx), uint64(tx), networkAvailable
	point.DiskReadBytes, point.DiskWriteBytes, point.DiskAvailable = uint64(read), uint64(written), diskAvailable
	point.HealthAvailable, point.Partial = healthAvailable, partial
	point.StorageClassification = observability.StorageClassification(classificationValue)
	if storage.Valid {
		value := storage.Int64
		point.StorageBytes = &value
	}
	return point, err
}

type rowsScanner interface {
	Next() bool
	Scan(...any) error
	Err() error
	Close() error
}

func scanMetricRows(rows rowsScanner) ([]observability.MetricPoint, error) {
	defer func() { _ = rows.Close() }()
	result := []observability.MetricPoint{}
	for rows.Next() {
		point, err := scanMetric(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, point)
	}
	return result, rows.Err()
}

// ResourceFootprint reports only Switchyard-owned SQLite/log/metric storage.
func (d *Database) ResourceFootprint(ctx context.Context) (observability.Footprint, error) {
	result := observability.Footprint{Classification: "exclusive"}
	result.DatabaseBytes = fileSize(d.path)
	result.DatabaseWALBytes = fileSize(d.path + "-wal")
	result.DatabaseSHMBytes = fileSize(d.path + "-shm")
	if err := d.connection.QueryRowContext(ctx, `SELECT COALESCE(SUM(size_bytes), 0), COUNT(*) FROM log_segments`).Scan(&result.LogBytes, &result.LogSegments); err != nil {
		return observability.Footprint{}, fmt.Errorf("query log footprint: %w", err)
	}
	var oldest sql.NullString
	if err := d.connection.QueryRowContext(ctx, `SELECT COUNT(*), MIN(sampled_at) FROM resource_metric_samples`).Scan(&result.MetricRows, &oldest); err != nil {
		return observability.Footprint{}, fmt.Errorf("query metric footprint: %w", err)
	}
	if oldest.Valid {
		value, err := parseTime(oldest.String)
		if err != nil {
			return observability.Footprint{}, err
		}
		result.OldestMetricAt = &value
	}
	return result, nil
}

func classification(value observability.StorageClassification) string {
	if value == "" {
		return string(observability.StorageUnknown)
	}
	return string(value)
}

func nullableInt(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

func safeUint(value uint64) int64 {
	if value > math.MaxInt64 {
		return math.MaxInt64
	}
	return int64(value)
}

func fileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return 0
	}
	return info.Size()
}
