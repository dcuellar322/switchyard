package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	observabilityApplication "switchyard.dev/switchyard/internal/observability/application"
	observability "switchyard.dev/switchyard/internal/observability/domain"
)

// MaintainMetricHistory upserts complete rollups before pruning aligned source buckets.
func (d *Database) MaintainMetricHistory(ctx context.Context, now time.Time, config observabilityApplication.ResourceConfig) error {
	if err := d.rollupMetrics(ctx, 0, 60); err != nil {
		return err
	}
	if err := d.rollupMetrics(ctx, 60, 900); err != nil {
		return err
	}
	tx, err := d.connection.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	cutoffs := []struct {
		resolution int
		cutoff     time.Time
	}{
		{0, now.Add(-config.RawRetention).Truncate(time.Minute)},
		{60, now.Add(-config.MinuteRetention).Truncate(15 * time.Minute)},
		{900, now.Add(-config.QuarterHourRetention).Truncate(15 * time.Minute)},
	}
	for _, item := range cutoffs {
		if _, err := tx.ExecContext(ctx, `DELETE FROM resource_metric_samples WHERE resolution_seconds = ? AND sampled_at < ?`, item.resolution, formatTime(item.cutoff)); err != nil {
			return fmt.Errorf("prune metric tier %d: %w", item.resolution, err)
		}
	}
	return tx.Commit()
}

type metricBucketKey struct {
	projectID, serviceID string
	start                time.Time
}

type metricBucket struct {
	point           observability.MetricPoint
	weightedCPU     float64
	weightedMem     float64
	weightedLatency float64
	cpuCount        int
	memoryCount     int
	healthCount     int
	latestNetwork   time.Time
	latestDisk      time.Time
	latestStorage   time.Time
}

func (d *Database) rollupMetrics(ctx context.Context, sourceResolution, targetResolution int) error {
	rows, err := d.connection.QueryContext(ctx, `SELECT `+metricColumns+` FROM resource_metric_samples AS source
		WHERE source.resolution_seconds = ?
		  AND source.sampled_at >= COALESCE((
			SELECT MAX(target.sampled_at)
			FROM resource_metric_samples AS target
			WHERE target.project_id = source.project_id
			  AND target.service_id = source.service_id
			  AND target.resolution_seconds = ?
		  ), '0001-01-01T00:00:00Z')
		ORDER BY source.sampled_at, source.id`, sourceResolution, targetResolution)
	if err != nil {
		return fmt.Errorf("read metric rollup source: %w", err)
	}
	points, err := scanMetricRows(rows)
	if err != nil {
		return err
	}
	buckets := map[metricBucketKey]metricBucket{}
	for _, point := range points {
		start := point.Timestamp.Truncate(time.Duration(targetResolution) * time.Second)
		key := metricBucketKey{projectID: point.ProjectID, serviceID: point.ServiceID, start: start}
		buckets[key] = addMetricToBucket(buckets[key], point, start, targetResolution)
	}
	tx, err := d.connection.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, bucket := range buckets {
		if err := upsertRollup(ctx, tx, finishMetricBucket(bucket)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func addMetricToBucket(bucket metricBucket, point observability.MetricPoint, start time.Time, resolution int) metricBucket {
	if bucket.point.SampleCount == 0 {
		bucket.point = observability.MetricPoint{
			Timestamp: start, ProjectID: point.ProjectID, ServiceID: point.ServiceID, ResolutionSeconds: resolution,
			StorageClassification: point.StorageClassification,
		}
	}
	count := max(1, point.SampleCount)
	bucket.point.SampleCount += count
	if point.CPUAvailable {
		bucket.weightedCPU += point.CPUPercent * float64(count)
		bucket.cpuCount += count
		bucket.point.CPUAvailable = true
		bucket.point.CPUMaxPercent = max(bucket.point.CPUMaxPercent, max(point.CPUMaxPercent, point.CPUPercent))
	}
	if point.MemoryAvailable {
		bucket.weightedMem += float64(point.MemoryBytes) * float64(count)
		bucket.memoryCount += count
		bucket.point.MemoryAvailable = true
		bucket.point.MemoryMaxBytes = max(bucket.point.MemoryMaxBytes, max(point.MemoryMaxBytes, point.MemoryBytes))
	}
	if point.HealthAvailable {
		bucket.weightedLatency += float64(point.HealthLatencyMS) * float64(count)
		bucket.healthCount += count
	}
	bucket.point.MemoryLimit = max(bucket.point.MemoryLimit, point.MemoryLimit)
	bucket.point.ProcessCount = max(bucket.point.ProcessCount, point.ProcessCount)
	bucket.point.RestartCount = max(bucket.point.RestartCount, point.RestartCount)
	bucket.point.HealthAvailable = bucket.point.HealthAvailable || point.HealthAvailable
	bucket.point.Partial = bucket.point.Partial || point.Partial
	if point.NetworkAvailable && !point.Timestamp.Before(bucket.latestNetwork) {
		bucket.latestNetwork = point.Timestamp
		bucket.point.NetworkAvailable = true
		bucket.point.NetworkRxBytes, bucket.point.NetworkTxBytes = point.NetworkRxBytes, point.NetworkTxBytes
	}
	if point.DiskAvailable && !point.Timestamp.Before(bucket.latestDisk) {
		bucket.latestDisk = point.Timestamp
		bucket.point.DiskAvailable = true
		bucket.point.DiskReadBytes, bucket.point.DiskWriteBytes = point.DiskReadBytes, point.DiskWriteBytes
	}
	if point.StorageBytes != nil && !point.Timestamp.Before(bucket.latestStorage) {
		bucket.latestStorage = point.Timestamp
		bucket.point.StorageBytes, bucket.point.StorageClassification = point.StorageBytes, point.StorageClassification
	}
	return bucket
}

func finishMetricBucket(bucket metricBucket) observability.MetricPoint {
	if bucket.cpuCount > 0 {
		bucket.point.CPUPercent = bucket.weightedCPU / float64(bucket.cpuCount)
	}
	if bucket.memoryCount > 0 {
		bucket.point.MemoryBytes = uint64(bucket.weightedMem / float64(bucket.memoryCount))
	}
	if bucket.healthCount > 0 {
		bucket.point.HealthLatencyMS = int64(bucket.weightedLatency / float64(bucket.healthCount))
	}
	return bucket.point
}

func upsertRollup(ctx context.Context, tx *sql.Tx, point observability.MetricPoint) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO resource_metric_samples
        (project_id, service_id, sampled_at, resolution_seconds, sample_count,
		 cpu_percent, cpu_max_percent, cpu_available, memory_bytes, memory_max_bytes, memory_limit, memory_available,
         network_rx_bytes, network_tx_bytes, network_available, disk_read_bytes, disk_write_bytes, disk_available,
         process_count, restart_count, health_latency_ms, health_available, storage_bytes, storage_classification, partial)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(project_id, service_id, resolution_seconds, sampled_at) DO UPDATE SET
		  sample_count=excluded.sample_count, cpu_percent=excluded.cpu_percent, cpu_max_percent=excluded.cpu_max_percent, cpu_available=excluded.cpu_available,
		  memory_bytes=excluded.memory_bytes, memory_max_bytes=excluded.memory_max_bytes, memory_limit=excluded.memory_limit, memory_available=excluded.memory_available,
          network_rx_bytes=excluded.network_rx_bytes, network_tx_bytes=excluded.network_tx_bytes, network_available=excluded.network_available,
          disk_read_bytes=excluded.disk_read_bytes, disk_write_bytes=excluded.disk_write_bytes, disk_available=excluded.disk_available,
          process_count=excluded.process_count, restart_count=excluded.restart_count,
          health_latency_ms=excluded.health_latency_ms, health_available=excluded.health_available,
          storage_bytes=excluded.storage_bytes, storage_classification=excluded.storage_classification, partial=excluded.partial`,
		point.ProjectID, point.ServiceID, formatTime(point.Timestamp), point.ResolutionSeconds, point.SampleCount,
		point.CPUPercent, point.CPUMaxPercent, point.CPUAvailable,
		safeUint(point.MemoryBytes), safeUint(point.MemoryMaxBytes), safeUint(point.MemoryLimit), point.MemoryAvailable,
		safeUint(point.NetworkRxBytes), safeUint(point.NetworkTxBytes), point.NetworkAvailable, safeUint(point.DiskReadBytes), safeUint(point.DiskWriteBytes), point.DiskAvailable,
		point.ProcessCount, point.RestartCount, point.HealthLatencyMS, point.HealthAvailable, nullableInt(point.StorageBytes), classification(point.StorageClassification), point.Partial)
	if err != nil {
		return fmt.Errorf("upsert metric rollup: %w", err)
	}
	return nil
}
