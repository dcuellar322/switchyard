package application

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"switchyard.dev/switchyard/internal/observability/domain"
)

const (
	defaultSampleInterval   = 10 * time.Second
	defaultRawRetention     = time.Hour
	defaultMinuteRetention  = 24 * time.Hour
	defaultQuarterRetention = 30 * 24 * time.Hour
	defaultMaxHistoryPoints = 1_000
	defaultLogRetention     = 7 * 24 * time.Hour
	defaultLogRetentionSize = int64(256 << 20)
)

// ErrInvalidResourceQuery identifies an unsupported history window or resolution.
var ErrInvalidResourceQuery = errors.New("invalid resource query")

// ResourceConfig controls one bounded sampler and its persisted history tiers.
type ResourceConfig struct {
	SampleInterval          time.Duration
	ProjectTimeout          time.Duration
	RawRetention            time.Duration
	MinuteRetention         time.Duration
	QuarterHourRetention    time.Duration
	MaximumHistoryPoints    int
	MaximumParallelProjects int
	SustainedSamples        int
	LogRetentionAge         time.Duration
	LogRetentionBytes       int64
}

func (c ResourceConfig) withDefaults() ResourceConfig {
	if c.SampleInterval <= 0 {
		c.SampleInterval = defaultSampleInterval
	}
	if c.ProjectTimeout <= 0 {
		c.ProjectTimeout = 4 * time.Second
	}
	if c.RawRetention <= 0 {
		c.RawRetention = defaultRawRetention
	}
	if c.MinuteRetention <= 0 {
		c.MinuteRetention = defaultMinuteRetention
	}
	if c.QuarterHourRetention <= 0 {
		c.QuarterHourRetention = defaultQuarterRetention
	}
	if c.MaximumHistoryPoints <= 0 {
		c.MaximumHistoryPoints = defaultMaxHistoryPoints
	}
	if c.MaximumParallelProjects <= 0 {
		c.MaximumParallelProjects = 4
	}
	if c.SustainedSamples <= 0 {
		c.SustainedSamples = 3
	}
	if c.LogRetentionAge <= 0 {
		c.LogRetentionAge = defaultLogRetention
	}
	if c.LogRetentionBytes <= 0 {
		c.LogRetentionBytes = defaultLogRetentionSize
	}
	return c
}

func (c ResourceConfig) validate() error {
	if c.SampleInterval < time.Second || c.ProjectTimeout < time.Second || c.MaximumParallelProjects > 16 || c.MaximumHistoryPoints > 1_000 {
		return fmt.Errorf("%w: sampling bounds", ErrInvalidResourceQuery)
	}
	if c.RawRetention < c.SampleInterval || c.MinuteRetention < c.RawRetention || c.QuarterHourRetention < c.MinuteRetention {
		return fmt.Errorf("%w: retention tiers", ErrInvalidResourceQuery)
	}
	return nil
}

// ResourceProjectSource exposes catalog identity and warning policy without manifest internals.
type ResourceProjectSource interface {
	ListResourceProjects(context.Context) ([]domain.ProjectDescriptor, error)
}

// ResourceRuntimeSource observes activity and raw service measurements.
type ResourceRuntimeSource interface {
	ObserveResources(context.Context, domain.ProjectDescriptor) (domain.RuntimeSnapshot, error)
}

// MetricRepository owns history persistence, compaction, and self-footprint facts.
type MetricRepository interface {
	WriteMetricPoints(context.Context, []domain.MetricPoint) error
	LatestMetricPoints(context.Context, []string) (map[string][]domain.MetricPoint, error)
	RecentProjectMetricPoints(context.Context, []string, int) (map[string][]domain.MetricPoint, error)
	MetricHistory(context.Context, string, string, time.Time, time.Time, int, int) ([]domain.MetricPoint, error)
	MaintainMetricHistory(context.Context, time.Time, ResourceConfig) error
	ResourceFootprint(context.Context) (domain.Footprint, error)
}

// StorageInspector provides read-only Engine inventory and no prune/delete method.
type StorageInspector interface {
	InspectStorage(context.Context, []domain.ProjectDescriptor) (domain.StorageInventory, error)
}

// ResourceService coordinates sampling, history, policy, storage, and cleanup previews.
type ResourceService struct {
	projects ResourceProjectSource
	runtime  ResourceRuntimeSource
	metrics  MetricRepository
	storage  StorageInspector
	config   ResourceConfig
	now      func() time.Time

	collectMu       sync.Mutex
	lastMaintenance time.Time
	stateMu         sync.RWMutex
	lastAt          time.Time
	states          map[string]domain.RuntimeSnapshot
	warnings        []string
}

// NewResourceService creates the observability resource intelligence use cases.
func NewResourceService(projects ResourceProjectSource, runtime ResourceRuntimeSource, metrics MetricRepository, storage StorageInspector, config ResourceConfig) (*ResourceService, error) {
	config = config.withDefaults()
	if err := config.validate(); err != nil {
		return nil, err
	}
	return &ResourceService{projects: projects, runtime: runtime, metrics: metrics, storage: storage, config: config, now: time.Now, states: map[string]domain.RuntimeSnapshot{}}, nil
}

// Run samples on one process-wide ticker and exits on cancellation.
func (s *ResourceService) Run(ctx context.Context, report func(error)) {
	s.collectAndReport(ctx, report)
	ticker := time.NewTicker(s.config.SampleInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.collectAndReport(ctx, report)
		}
	}
}

func (s *ResourceService) collectAndReport(ctx context.Context, report func(error)) {
	if err := s.Collect(ctx); err != nil && report != nil && !errors.Is(err, context.Canceled) {
		report(err)
	}
}

// Collect performs one bounded cycle; inactive projects never call runtime metrics.
func (s *ResourceService) Collect(ctx context.Context) error {
	s.collectMu.Lock()
	defer s.collectMu.Unlock()
	projects, err := s.projects.ListResourceProjects(ctx)
	if err != nil {
		return err
	}
	inventory, storageErr := s.storage.InspectStorage(ctx, projects)
	storageByProject := projectStorageMap(inventory)
	states, points, warnings := s.collectProjects(ctx, projects, storageByProject)
	warnings = append(warnings, inventory.Warnings...)
	if storageErr != nil {
		warnings = append(warnings, "Docker storage inspection is unavailable; runtime history remains active.")
	}
	warnings = sortedUnique(warnings)
	now := s.now().UTC()
	if len(points) > 0 {
		if err := s.metrics.WriteMetricPoints(ctx, points); err != nil {
			return err
		}
	}
	if len(points) > 0 || s.lastMaintenance.IsZero() || now.Sub(s.lastMaintenance) >= 15*time.Minute {
		if err := s.metrics.MaintainMetricHistory(ctx, now, s.config); err != nil {
			return err
		}
		s.lastMaintenance = now
	}
	s.stateMu.Lock()
	s.lastAt, s.states, s.warnings = now, states, warnings
	s.stateMu.Unlock()
	return nil
}

type collectionResult struct {
	project domain.ProjectDescriptor
	state   domain.RuntimeSnapshot
	err     error
}

func (s *ResourceService) collectProjects(ctx context.Context, projects []domain.ProjectDescriptor, storage map[string]domain.ProjectStorage) (map[string]domain.RuntimeSnapshot, []domain.MetricPoint, []string) {
	jobs := make(chan domain.ProjectDescriptor)
	results := make(chan collectionResult, len(projects))
	workers := min(s.config.MaximumParallelProjects, max(1, len(projects)))
	var group sync.WaitGroup
	for range workers {
		group.Add(1)
		go func() {
			defer group.Done()
			for project := range jobs {
				projectCtx, cancel := context.WithTimeout(ctx, s.config.ProjectTimeout)
				state, err := s.runtime.ObserveResources(projectCtx, project)
				cancel()
				results <- collectionResult{project: project, state: state, err: err}
			}
		}()
	}
	go func() {
		defer close(results)
		for _, project := range projects {
			select {
			case jobs <- project:
			case <-ctx.Done():
				close(jobs)
				group.Wait()
				return
			}
		}
		close(jobs)
		group.Wait()
	}()
	states := make(map[string]domain.RuntimeSnapshot, len(projects))
	points, warnings := []domain.MetricPoint{}, []string{}
	for result := range results {
		if result.err != nil {
			warnings = append(warnings, fmt.Sprintf("%s resource sampling failed.", result.project.Name))
			result.state = domain.RuntimeSnapshot{ProjectID: result.project.ID, Driver: result.project.Driver, State: "unknown", Partial: true, Warnings: []string{result.err.Error()}}
		}
		states[result.project.ID] = result.state
		for range result.state.Warnings {
			warnings = append(warnings, fmt.Sprintf("%s resource observation is partial.", result.project.Name))
			break
		}
		if result.state.Active || result.state.Partial {
			points = append(points, aggregateProject(result.project, result.state, storage[result.project.ID], s.now().UTC())...)
		}
	}
	sort.Strings(warnings)
	return states, points, warnings
}

func aggregateProject(project domain.ProjectDescriptor, state domain.RuntimeSnapshot, storage domain.ProjectStorage, now time.Time) []domain.MetricPoint {
	services := aggregateServices(state.Samples, now, project.ID, project.Driver)
	projectPoint := domain.MetricPoint{Timestamp: now, ProjectID: project.ID, SampleCount: 1, Partial: state.Partial, StorageClassification: domain.StorageUnknown}
	for _, point := range services {
		if point.CPUAvailable {
			projectPoint.CPUPercent += point.CPUPercent
			projectPoint.CPUMaxPercent += point.CPUMaxPercent
			projectPoint.CPUAvailable = true
		}
		if point.MemoryAvailable {
			projectPoint.MemoryBytes += point.MemoryBytes
			projectPoint.MemoryMaxBytes += point.MemoryMaxBytes
			projectPoint.MemoryLimit = aggregateMemoryLimit(projectPoint.MemoryLimit, point.MemoryLimit, project.Driver)
			projectPoint.MemoryAvailable = true
		}
		projectPoint.NetworkRxBytes += point.NetworkRxBytes
		projectPoint.NetworkTxBytes += point.NetworkTxBytes
		projectPoint.NetworkAvailable = projectPoint.NetworkAvailable || point.NetworkAvailable
		projectPoint.DiskReadBytes += point.DiskReadBytes
		projectPoint.DiskWriteBytes += point.DiskWriteBytes
		projectPoint.DiskAvailable = projectPoint.DiskAvailable || point.DiskAvailable
		projectPoint.ProcessCount += point.ProcessCount
		projectPoint.RestartCount += point.RestartCount
		projectPoint.HealthLatencyMS = max(projectPoint.HealthLatencyMS, point.HealthLatencyMS)
		projectPoint.HealthAvailable = projectPoint.HealthAvailable || point.HealthAvailable
		projectPoint.Partial = projectPoint.Partial || point.Partial
	}
	if storage.ProjectID != "" {
		value := storage.Summary.Bytes
		projectPoint.StorageBytes = &value
		projectPoint.StorageClassification = storage.Summary.Classification
	}
	return append([]domain.MetricPoint{projectPoint}, services...)
}

func aggregateServices(samples []domain.MetricPoint, now time.Time, projectID, driver string) []domain.MetricPoint {
	byService := map[string]domain.MetricPoint{}
	for _, sample := range samples {
		current := byService[sample.ServiceID]
		current.Timestamp, current.ProjectID, current.ServiceID = now, projectID, sample.ServiceID
		current.SampleCount = 1
		if sample.CPUAvailable {
			current.CPUPercent += sample.CPUPercent
			current.CPUMaxPercent += max(sample.CPUMaxPercent, sample.CPUPercent)
			current.CPUAvailable = true
		}
		if sample.MemoryAvailable {
			current.MemoryBytes += sample.MemoryBytes
			current.MemoryMaxBytes += max(sample.MemoryMaxBytes, sample.MemoryBytes)
			current.MemoryLimit = aggregateMemoryLimit(current.MemoryLimit, sample.MemoryLimit, driver)
			current.MemoryAvailable = true
		}
		current.NetworkRxBytes += sample.NetworkRxBytes
		current.NetworkTxBytes += sample.NetworkTxBytes
		current.NetworkAvailable = current.NetworkAvailable || sample.NetworkAvailable
		current.DiskReadBytes += sample.DiskReadBytes
		current.DiskWriteBytes += sample.DiskWriteBytes
		current.DiskAvailable = current.DiskAvailable || sample.DiskAvailable
		current.ProcessCount += sample.ProcessCount
		current.RestartCount += sample.RestartCount
		current.HealthLatencyMS = max(current.HealthLatencyMS, sample.HealthLatencyMS)
		current.HealthAvailable = current.HealthAvailable || sample.HealthAvailable
		current.Partial = current.Partial || sample.Partial
		current.StorageClassification = domain.StorageUnknown
		byService[sample.ServiceID] = current
	}
	result := make([]domain.MetricPoint, 0, len(byService))
	for _, point := range byService {
		result = append(result, point)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ServiceID < result[j].ServiceID })
	return result
}

func aggregateMemoryLimit(current, next uint64, driver string) uint64 {
	if driver == "compose" {
		return current + next
	}
	// Native process samples report the same host memory ceiling for every
	// process and service, so summing would invent capacity.
	return max(current, next)
}

func sortedUnique(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
