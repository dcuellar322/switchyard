package sqlite

import (
	"context"
	"errors"
	"math"
	"path/filepath"
	"testing"
	"time"

	observabilityApplication "switchyard.dev/switchyard/internal/observability/application"
	observability "switchyard.dev/switchyard/internal/observability/domain"
)

func TestMetricPersistenceIsAtomicIdempotentAndBounded(t *testing.T) {
	t.Parallel()
	database := newResourceMetricDatabase(t)
	ctx := context.Background()
	base := time.Date(2026, 7, 16, 11, 59, 0, 0, time.UTC)
	storage := int64(2048)
	points := []observability.MetricPoint{
		{Timestamp: base, ProjectID: "project-1", CPUPercent: 10, MemoryBytes: math.MaxUint64, StorageBytes: &storage, StorageClassification: observability.StorageExclusive},
		{Timestamp: base, ProjectID: "project-1", ServiceID: "api", CPUPercent: 5, MemoryBytes: 512, NetworkAvailable: true, NetworkRxBytes: 11, Partial: true},
	}
	if err := database.WriteMetricPoints(ctx, points); err != nil {
		t.Fatal(err)
	}
	if err := database.WriteMetricPoints(ctx, points); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := database.connection.QueryRowContext(ctx, `SELECT COUNT(*) FROM resource_metric_samples`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("idempotent row count = %d, want 2", count)
	}
	latest, err := database.LatestMetricPoints(ctx, []string{"project-1", "missing"})
	if err != nil {
		t.Fatal(err)
	}
	if len(latest["project-1"]) != 2 || len(latest["missing"]) != 0 {
		t.Fatalf("latest = %#v", latest)
	}
	project := latest["project-1"][0]
	if project.ServiceID != "" || project.MemoryBytes != math.MaxInt64 || project.CPUMaxPercent != 10 || project.SampleCount != 1 || project.StorageClassification != observability.StorageExclusive {
		t.Fatalf("project point = %#v", project)
	}
	history, err := database.MetricHistory(ctx, "project-1", "api", base.Add(-time.Minute), base.Add(time.Minute), 0, 1)
	if err != nil || len(history) != 1 || !history[0].Partial || history[0].NetworkRxBytes != 11 {
		t.Fatalf("history = %#v, error = %v", history, err)
	}

	atomicBatch := []observability.MetricPoint{
		{Timestamp: base.Add(time.Second), ProjectID: "project-1", CPUPercent: 1},
		{Timestamp: base.Add(time.Second), CPUPercent: 1},
	}
	if err := database.WriteMetricPoints(ctx, atomicBatch); err == nil {
		t.Fatal("WriteMetricPoints() accepted point without a project")
	}
	if err := database.connection.QueryRowContext(ctx, `SELECT COUNT(*) FROM resource_metric_samples`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("failed atomic batch left %d rows, want 2", count)
	}
}

func TestMetricRollupRetentionAndFootprintRemainIdempotent(t *testing.T) {
	t.Parallel()
	database := newResourceMetricDatabase(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	storage := int64(100)
	points := []observability.MetricPoint{
		{
			Timestamp: now.Add(-50 * time.Second), ProjectID: "project-1", ServiceID: "api", SampleCount: 1,
			CPUPercent: 10, CPUMaxPercent: 12, CPUAvailable: true, MemoryBytes: 100, MemoryMaxBytes: 120, MemoryLimit: 1_000, MemoryAvailable: true,
			NetworkAvailable: true, NetworkRxBytes: 1, NetworkTxBytes: 2,
			HealthLatencyMS: 999, StorageBytes: &storage, StorageClassification: observability.StorageEstimated,
		},
		{
			Timestamp: now.Add(-20 * time.Second), ProjectID: "project-1", ServiceID: "api", SampleCount: 1,
			CPUPercent: 30, CPUMaxPercent: 35, CPUAvailable: true, MemoryBytes: 300, MemoryMaxBytes: 350, MemoryLimit: 1_000, MemoryAvailable: true,
			NetworkRxBytes: 999, DiskAvailable: true, DiskReadBytes: 7, DiskWriteBytes: 8,
			HealthAvailable: true, HealthLatencyMS: 40, ProcessCount: 3, RestartCount: 2, Partial: true,
		},
		{Timestamp: now.Add(-2*time.Hour - 10*time.Minute), ProjectID: "project-1", CPUPercent: 1},
		{Timestamp: now.Add(-25 * time.Hour), ProjectID: "project-1", ResolutionSeconds: 60, CPUPercent: 1},
		{Timestamp: now.Add(-5 * time.Hour), ProjectID: "project-1", ResolutionSeconds: 900, CPUPercent: 1},
	}
	if err := database.WriteMetricPoints(ctx, points); err != nil {
		t.Fatal(err)
	}
	config := observabilityApplication.ResourceConfig{
		SampleInterval: 10 * time.Second, ProjectTimeout: time.Second,
		RawRetention: time.Hour, MinuteRetention: 2 * time.Hour, QuarterHourRetention: 4 * time.Hour,
		MaximumHistoryPoints: 100, MaximumParallelProjects: 4, SustainedSamples: 3,
	}
	if err := database.MaintainMetricHistory(ctx, now, config); err != nil {
		t.Fatal(err)
	}
	minute := oneMetricAtResolution(t, database, "project-1", "api", 60)
	if minute.SampleCount != 2 || minute.CPUPercent != 20 || minute.CPUMaxPercent != 35 || minute.MemoryBytes != 200 || minute.MemoryMaxBytes != 350 {
		t.Fatalf("minute aggregate = %#v", minute)
	}
	if minute.HealthLatencyMS != 40 || !minute.HealthAvailable || minute.NetworkRxBytes != 1 || !minute.NetworkAvailable || minute.DiskWriteBytes != 8 || !minute.DiskAvailable {
		t.Fatalf("availability-aware minute aggregate = %#v", minute)
	}
	if minute.StorageBytes == nil || *minute.StorageBytes != 100 || minute.StorageClassification != observability.StorageEstimated || !minute.Partial || minute.ProcessCount != 3 || minute.RestartCount != 2 {
		t.Fatalf("latest/max minute aggregate = %#v", minute)
	}
	quarter := oneMetricAtResolution(t, database, "project-1", "api", 900)
	if quarter.SampleCount != 2 || quarter.CPUPercent != 20 || quarter.MemoryBytes != 200 {
		t.Fatalf("quarter-hour aggregate = %#v", quarter)
	}

	countsBefore := metricCountsByResolution(t, database)
	if err := database.MaintainMetricHistory(ctx, now, config); err != nil {
		t.Fatal(err)
	}
	countsAfter := metricCountsByResolution(t, database)
	if countsBefore != countsAfter {
		t.Fatalf("second maintenance changed row counts: before=%#v after=%#v", countsBefore, countsAfter)
	}
	minuteAgain := oneMetricAtResolution(t, database, "project-1", "api", 60)
	if minuteAgain.SampleCount != 2 || minuteAgain.CPUPercent != 20 {
		t.Fatalf("second maintenance double-counted rollup = %#v", minuteAgain)
	}
	assertNoMetricsBefore(t, database, 0, now.Add(-config.RawRetention).Truncate(time.Minute))
	assertNoMetricsBefore(t, database, 60, now.Add(-config.MinuteRetention).Truncate(15*time.Minute))
	assertNoMetricsBefore(t, database, 900, now.Add(-config.QuarterHourRetention).Truncate(15*time.Minute))
	if _, err := database.connection.ExecContext(ctx, `INSERT INTO log_segments
		(id, project_id, service_id, run_id, path, created_at, entry_count, size_bytes)
		VALUES ('segment-1', 'project-1', 'api', 'run-1', '/tmp/segment-1.ndjson', ?, 3, 321)`, formatTime(now)); err != nil {
		t.Fatal(err)
	}

	footprint, err := database.ResourceFootprint(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if footprint.DatabaseBytes <= 0 || footprint.LogBytes != 321 || footprint.LogSegments != 1 || footprint.MetricRows != int64(countsAfter.raw+countsAfter.minute+countsAfter.quarter) || footprint.OldestMetricAt == nil || footprint.Classification != "exclusive" {
		t.Fatalf("footprint = %#v, counts = %#v", footprint, countsAfter)
	}
}

func TestMetricHistoryIsChronologicalAndHonorsLimit(t *testing.T) {
	t.Parallel()
	database := newResourceMetricDatabase(t)
	ctx := context.Background()
	base := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	for index := 3; index >= 0; index-- {
		if err := database.WriteMetricPoints(ctx, []observability.MetricPoint{{Timestamp: base.Add(time.Duration(index) * time.Second), ProjectID: "project-1", CPUPercent: float64(index)}}); err != nil {
			t.Fatal(err)
		}
	}
	history, err := database.MetricHistory(ctx, "project-1", "", base, base.Add(10*time.Second), 0, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 3 || !history[0].Timestamp.Equal(base) || !history[1].Timestamp.Equal(base.Add(time.Second)) || !history[2].Timestamp.Equal(base.Add(2*time.Second)) {
		t.Fatalf("bounded chronological history = %#v", history)
	}
	recent, err := database.RecentProjectMetricPoints(ctx, []string{"project-1"}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(recent["project-1"]) != 2 || !recent["project-1"][0].Timestamp.Equal(base.Add(3*time.Second)) {
		t.Fatalf("recent points = %#v", recent)
	}
}

func newResourceMetricDatabase(t testing.TB) *Database {
	t.Helper()
	database, err := Open(context.Background(), filepath.Join(t.TempDir(), "switchyard.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := database.Close(); err != nil && !errors.Is(err, context.Canceled) {
			t.Errorf("Close() error = %v", err)
		}
	})
	now := formatTime(time.Now().UTC())
	if _, err := database.connection.Exec(`INSERT INTO projects
		(id, slug, display_name, trust_state, primary_location, created_at, updated_at)
		VALUES ('project-1', 'project-1', 'Project One', 'trusted', '/tmp/project-1', ?, ?)`, now, now); err != nil {
		t.Fatal(err)
	}
	return database
}

func oneMetricAtResolution(t testing.TB, database *Database, projectID, serviceID string, resolution int) observability.MetricPoint {
	t.Helper()
	rows, err := database.connection.Query(`SELECT `+metricColumns+` FROM resource_metric_samples
		WHERE project_id = ? AND service_id = ? AND resolution_seconds = ? ORDER BY sampled_at DESC`, projectID, serviceID, resolution)
	if err != nil {
		t.Fatal(err)
	}
	points, err := scanMetricRows(rows)
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 1 {
		t.Fatalf("metrics at resolution %d = %#v, want one", resolution, points)
	}
	return points[0]
}

type resolutionCounts struct{ raw, minute, quarter int }

func metricCountsByResolution(t testing.TB, database *Database) resolutionCounts {
	t.Helper()
	var result resolutionCounts
	rows, err := database.connection.Query(`SELECT resolution_seconds, COUNT(*) FROM resource_metric_samples GROUP BY resolution_seconds`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var resolution, count int
		if err := rows.Scan(&resolution, &count); err != nil {
			t.Fatal(err)
		}
		switch resolution {
		case 0:
			result.raw = count
		case 60:
			result.minute = count
		case 900:
			result.quarter = count
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return result
}

func assertNoMetricsBefore(t testing.TB, database *Database, resolution int, cutoff time.Time) {
	t.Helper()
	var count int
	if err := database.connection.QueryRow(`SELECT COUNT(*) FROM resource_metric_samples WHERE resolution_seconds = ? AND sampled_at < ?`, resolution, formatTime(cutoff)).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("resolution %d has %d rows before %s", resolution, count, cutoff)
	}
}
