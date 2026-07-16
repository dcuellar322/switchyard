package application

import (
	"context"
	"fmt"
	"sort"
	"time"

	"switchyard.dev/switchyard/internal/observability/domain"
)

// Overview returns one aggregate response and never fans out from the browser.
func (s *ResourceService) Overview(ctx context.Context) (domain.ResourceOverview, error) {
	if s.collectionStale() {
		if err := s.Collect(ctx); err != nil {
			return domain.ResourceOverview{}, err
		}
	}
	projects, err := s.projects.ListResourceProjects(ctx)
	if err != nil {
		return domain.ResourceOverview{}, err
	}
	ids := projectIDs(projects)
	latest, err := s.metrics.LatestMetricPoints(ctx, ids)
	if err != nil {
		return domain.ResourceOverview{}, err
	}
	recent, err := s.metrics.RecentProjectMetricPoints(ctx, ids, s.config.SustainedSamples)
	if err != nil {
		return domain.ResourceOverview{}, err
	}
	inventory, storageErr := s.storage.InspectStorage(ctx, projects)
	footprint, footprintErr := s.metrics.ResourceFootprint(ctx)
	s.stateMu.RLock()
	observedAt, states, warnings := s.lastAt, cloneStates(s.states), append([]string(nil), s.warnings...)
	s.stateMu.RUnlock()
	if storageErr != nil {
		warnings = append(warnings, "Docker storage is unavailable.")
	}
	if footprintErr != nil {
		warnings = append(warnings, "Switchyard footprint is unavailable.")
	}
	warnings = sortedUnique(warnings)
	return domain.ResourceOverview{
		ObservedAt: observedAt, Projects: buildSnapshots(projects, states, latest, recent, projectStorageMap(inventory), observedAt, s.config),
		Storage: inventory.Summary, Footprint: footprint, Retention: s.retentionPolicy(), Warnings: warnings,
	}, nil
}

// History returns no more than the configured point cap from one tier.
func (s *ResourceService) History(ctx context.Context, projectID, service, resolution string, from, to time.Time, maxPoints int) (domain.MetricHistory, error) {
	span := to.Sub(from)
	if projectID == "" || from.IsZero() || to.IsZero() || !from.Before(to) || span > s.config.QuarterHourRetention {
		return domain.MetricHistory{}, ErrInvalidResourceQuery
	}
	if maxPoints <= 0 {
		maxPoints = s.config.MaximumHistoryPoints
	}
	if maxPoints > s.config.MaximumHistoryPoints {
		return domain.MetricHistory{}, ErrInvalidResourceQuery
	}
	tier, err := s.historyResolution(resolution, span, maxPoints)
	if err != nil {
		return domain.MetricHistory{}, err
	}
	points, err := s.metrics.MetricHistory(ctx, projectID, service, from.UTC(), to.UTC(), tier, maxPoints)
	if err != nil {
		return domain.MetricHistory{}, err
	}
	if points == nil {
		points = []domain.MetricPoint{}
	}
	return domain.MetricHistory{ProjectID: projectID, ServiceID: service, ResolutionSeconds: tier, From: from.UTC(), To: to.UTC(), Points: points}, nil
}

// Storage returns the current typed inventory; a disconnected Engine is a partial result.
func (s *ResourceService) Storage(ctx context.Context) (domain.StorageInventory, error) {
	projects, err := s.projects.ListResourceProjects(ctx)
	if err != nil {
		return domain.StorageInventory{}, err
	}
	return s.storage.InspectStorage(ctx, projects)
}

// CleanupPreview lists exact currently reclaimable resources and cannot execute them.
func (s *ResourceService) CleanupPreview(ctx context.Context, projectID string) (domain.CleanupPreview, error) {
	inventory, err := s.Storage(ctx)
	if err != nil && !inventory.Connected {
		return domain.CleanupPreview{}, err
	}
	preview := domain.CleanupPreview{ProjectID: projectID, Risk: "destructive", Executable: false, Resources: []domain.StorageResource{}, Warnings: []string{"Preview only. Switchyard does not expose automatic cleanup in this phase."}, ObservedAt: inventory.ObservedAt}
	for _, resource := range inventory.Resources {
		if !resource.Reclaimable || (projectID != "" && !contains(resource.ProjectIDs, projectID)) {
			continue
		}
		preview.Resources = append(preview.Resources, resource)
		if resource.Bytes == nil {
			preview.UnknownSizes++
		} else {
			preview.EstimatedBytes += *resource.Bytes
		}
	}
	sort.Slice(preview.Resources, func(i, j int) bool {
		if preview.Resources[i].Kind != preview.Resources[j].Kind {
			return preview.Resources[i].Kind < preview.Resources[j].Kind
		}
		return preview.Resources[i].ID < preview.Resources[j].ID
	})
	return preview, nil
}

func (s *ResourceService) collectionStale() bool {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	return s.lastAt.IsZero() || s.now().UTC().Sub(s.lastAt) >= s.config.SampleInterval
}

func (s *ResourceService) retentionPolicy() domain.RetentionPolicy {
	return domain.RetentionPolicy{
		SampleIntervalSeconds: int(s.config.SampleInterval.Seconds()), RawSeconds: int64(s.config.RawRetention.Seconds()),
		MinuteSeconds: int64(s.config.MinuteRetention.Seconds()), QuarterHourSeconds: int64(s.config.QuarterHourRetention.Seconds()),
		MaximumHistoryPoints: s.config.MaximumHistoryPoints, LogSeconds: int64(s.config.LogRetentionAge.Seconds()), LogBytes: s.config.LogRetentionBytes,
	}
}

func (s *ResourceService) historyResolution(requested string, span time.Duration, maxPoints int) (int, error) {
	if requested == "" || requested == "auto" {
		for _, tier := range []struct {
			seconds   int
			retention time.Duration
		}{{0, s.config.RawRetention}, {60, s.config.MinuteRetention}, {900, s.config.QuarterHourRetention}} {
			interval := s.config.SampleInterval
			if tier.seconds > 0 {
				interval = time.Duration(tier.seconds) * time.Second
			}
			if span <= tier.retention && int((span+interval-1)/interval) <= maxPoints {
				return tier.seconds, nil
			}
		}
		return 900, nil
	}
	var tier int
	var retention time.Duration
	switch requested {
	case "raw":
		tier, retention = 0, s.config.RawRetention
	case "1m":
		tier, retention = 60, s.config.MinuteRetention
	case "15m":
		tier, retention = 900, s.config.QuarterHourRetention
	default:
		return 0, ErrInvalidResourceQuery
	}
	interval := s.config.SampleInterval
	if tier > 0 {
		interval = time.Duration(tier) * time.Second
	}
	if span > retention || int((span+interval-1)/interval) > maxPoints {
		return 0, ErrInvalidResourceQuery
	}
	return tier, nil
}

func projectStorageMap(inventory domain.StorageInventory) map[string]domain.ProjectStorage {
	result := make(map[string]domain.ProjectStorage, len(inventory.Projects))
	for _, value := range inventory.Projects {
		result[value.ProjectID] = value
	}
	return result
}

func projectIDs(projects []domain.ProjectDescriptor) []string {
	result := make([]string, 0, len(projects))
	for _, project := range projects {
		result = append(result, project.ID)
	}
	return result
}

func cloneStates(source map[string]domain.RuntimeSnapshot) map[string]domain.RuntimeSnapshot {
	result := make(map[string]domain.RuntimeSnapshot, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func buildSnapshots(projects []domain.ProjectDescriptor, states map[string]domain.RuntimeSnapshot, latest, recent map[string][]domain.MetricPoint, storage map[string]domain.ProjectStorage, observedAt time.Time, config ResourceConfig) []domain.ProjectSnapshot {
	result := make([]domain.ProjectSnapshot, 0, len(projects))
	for _, project := range projects {
		state := states[project.ID]
		snapshot := domain.ProjectSnapshot{
			ProjectID: project.ID, Name: project.Name, Driver: project.Driver, State: state.State, Active: state.Active,
			Metric: domain.MetricPoint{Timestamp: observedAt, ProjectID: project.ID, SampleCount: 1, StorageClassification: domain.StorageUnknown, Partial: state.Partial},
			Budget: project.Budget, Services: []domain.ServiceSnapshot{}, Warnings: []domain.BudgetWarning{},
		}
		if state.Active {
			snapshot.Warnings = evaluateBudget(project.Budget, recent[project.ID], config)
			for _, point := range latest[project.ID] {
				if point.ServiceID == "" {
					snapshot.Metric = point
				} else {
					snapshot.Services = append(snapshot.Services, domain.ServiceSnapshot{ServiceID: point.ServiceID, Metric: point})
				}
			}
		}
		if projectStorage, ok := storage[project.ID]; ok {
			value := projectStorage.Summary.Bytes
			snapshot.Metric.StorageBytes = &value
			snapshot.Metric.StorageClassification = projectStorage.Summary.Classification
		}
		result = append(result, snapshot)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Metric.MemoryBytes > result[j].Metric.MemoryBytes })
	return result
}

func evaluateBudget(budget domain.ResourceBudget, points []domain.MetricPoint, config ResourceConfig) []domain.BudgetWarning {
	if len(points) < config.SustainedSamples {
		return []domain.BudgetWarning{}
	}
	points = points[:config.SustainedSamples]
	if points[0].Timestamp.Sub(points[len(points)-1].Timestamp) > time.Duration(config.SustainedSamples)*config.SampleInterval*2 {
		return []domain.BudgetWarning{}
	}
	warnings := []domain.BudgetWarning{}
	if budget.CPUPercent > 0 && allPoints(points, func(point domain.MetricPoint) bool { return point.CPUAvailable && point.CPUPercent > budget.CPUPercent }) {
		warnings = append(warnings, budgetWarning("RESOURCE_CPU_SUSTAINED", "CPU", budget.CPUPercent, points[0].CPUPercent, "% of one core", points))
	}
	if budget.MemoryBytes > 0 && allPoints(points, func(point domain.MetricPoint) bool {
		return point.MemoryAvailable && point.MemoryBytes > budget.MemoryBytes
	}) {
		warnings = append(warnings, budgetWarning("RESOURCE_MEMORY_SUSTAINED", "memory", float64(budget.MemoryBytes), float64(points[0].MemoryBytes), "bytes", points))
	}
	if budget.StorageBytes > 0 && allPoints(points, func(point domain.MetricPoint) bool {
		return point.StorageBytes != nil && *point.StorageBytes > budget.StorageBytes
	}) {
		warnings = append(warnings, budgetWarning("RESOURCE_STORAGE_SUSTAINED", "storage", float64(budget.StorageBytes), float64(*points[0].StorageBytes), "estimated bytes", points))
	}
	return warnings
}

func allPoints(points []domain.MetricPoint, predicate func(domain.MetricPoint) bool) bool {
	for _, point := range points {
		if !predicate(point) {
			return false
		}
	}
	return true
}

func budgetWarning(code, resource string, limit, observed float64, unit string, points []domain.MetricPoint) domain.BudgetWarning {
	return domain.BudgetWarning{Code: code, Resource: resource, Limit: limit, Observed: observed, Unit: unit, Samples: len(points), SustainedFrom: points[len(points)-1].Timestamp, Message: fmt.Sprintf("%s exceeded its configured threshold for %d consecutive samples.", resource, len(points))}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
