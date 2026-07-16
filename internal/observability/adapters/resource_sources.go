package adapters

import (
	"context"

	catalogApplication "switchyard.dev/switchyard/internal/catalog/application"
	catalogDomain "switchyard.dev/switchyard/internal/catalog/domain"
	observabilityApplication "switchyard.dev/switchyard/internal/observability/application"
	observabilityDomain "switchyard.dev/switchyard/internal/observability/domain"
	runtimeApplication "switchyard.dev/switchyard/internal/runtime/application"
	runtimeDomain "switchyard.dev/switchyard/internal/runtime/domain"
)

// ResourceCatalogSource adapts trusted manifests to resource identity and policy.
type ResourceCatalogSource struct{ catalog *catalogApplication.Service }

// NewResourceCatalogSource creates a catalog-backed resource project source.
func NewResourceCatalogSource(catalog *catalogApplication.Service) *ResourceCatalogSource {
	return &ResourceCatalogSource{catalog: catalog}
}

// ListResourceProjects returns only trusted, non-secret resource inputs.
func (s *ResourceCatalogSource) ListResourceProjects(ctx context.Context) ([]observabilityDomain.ProjectDescriptor, error) {
	projects, err := s.catalog.ListProjects(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]observabilityDomain.ProjectDescriptor, 0, len(projects))
	for _, project := range projects {
		if project.TrustState != catalogDomain.TrustTrusted {
			continue
		}
		effective, effectiveErr := s.catalog.EffectiveManifest(ctx, project.ID, nil)
		if effectiveErr != nil {
			return nil, effectiveErr
		}
		manifest := effective.Manifest
		descriptor := observabilityDomain.ProjectDescriptor{
			ID: project.ID, Name: project.DisplayName, Driver: manifest.Runtime.Driver,
			Budget: observabilityDomain.ResourceBudget{
				CPUPercent:   float64(manifest.ResourcePolicy.Warnings.CPUPercent),
				MemoryBytes:  uint64(manifest.ResourcePolicy.Warnings.MemoryMiB) << 20,
				StorageBytes: int64(manifest.ResourcePolicy.Warnings.StorageGiB) << 30,
			},
		}
		if manifest.Runtime.Compose != nil {
			descriptor.ComposeProjectName = manifest.Runtime.Compose.ProjectName
		}
		result = append(result, descriptor)
	}
	return result, nil
}

type runtimeMetrics interface {
	Inspect(context.Context, string) (runtimeDomain.Observation, error)
	Metrics(context.Context, string, string) ([]runtimeDomain.MetricSample, error)
}

type latestHealth interface {
	LatestResults(context.Context, string) ([]observabilityDomain.HealthResult, error)
}

// ResourceRuntimeSource adapts raw runtime and health observations to resource samples.
type ResourceRuntimeSource struct {
	runtime runtimeMetrics
	health  latestHealth
}

// NewResourceRuntimeSource creates a combined runtime/health sampling adapter.
func NewResourceRuntimeSource(runtime *runtimeApplication.Service, health *observabilityApplication.HealthService) *ResourceRuntimeSource {
	return &ResourceRuntimeSource{runtime: runtime, health: health}
}

// ObserveResources skips driver statistics unless current runtime evidence is active.
func (s *ResourceRuntimeSource) ObserveResources(ctx context.Context, project observabilityDomain.ProjectDescriptor) (observabilityDomain.RuntimeSnapshot, error) {
	observation, err := s.runtime.Inspect(ctx, project.ID)
	if err != nil {
		return observabilityDomain.RuntimeSnapshot{}, err
	}
	result := observabilityDomain.RuntimeSnapshot{ProjectID: project.ID, Driver: string(observation.Driver), State: string(observation.State), Active: activeRuntime(observation.State), Samples: []observabilityDomain.MetricPoint{}, Warnings: []string{}}
	if !result.Active {
		return result, nil
	}
	samples, err := s.runtime.Metrics(ctx, project.ID, "")
	if err != nil {
		return result, err
	}
	latencies := map[string]int64{}
	if s.health != nil {
		health, healthErr := s.health.LatestResults(ctx, project.ID)
		if healthErr != nil {
			result.Partial = true
			result.Warnings = append(result.Warnings, "Health latency is unavailable.")
		} else {
			for _, item := range health {
				latencies[item.ServiceID] = max(latencies[item.ServiceID], item.LatencyMS)
			}
		}
	}
	for _, sample := range samples {
		latency, healthAvailable := latencies[sample.ServiceID]
		result.Samples = append(result.Samples, observabilityDomain.MetricPoint{
			Timestamp: sample.Timestamp, ProjectID: sample.ProjectID, ServiceID: sample.ServiceID,
			CPUPercent: sample.CPUPercent, CPUMaxPercent: sample.CPUPercent, CPUAvailable: sample.CPUAvailable,
			MemoryBytes: sample.MemoryBytes, MemoryMaxBytes: sample.MemoryBytes, MemoryLimit: sample.MemoryLimit, MemoryAvailable: sample.MemoryAvailable,
			NetworkRxBytes: sample.NetworkRxBytes, NetworkTxBytes: sample.NetworkTxBytes, NetworkAvailable: sample.NetworkAvailable,
			DiskReadBytes: sample.DiskReadBytes, DiskWriteBytes: sample.DiskWriteBytes, DiskAvailable: sample.DiskAvailable,
			ProcessCount: sample.ProcessCount, RestartCount: sample.RestartCount,
			HealthLatencyMS: latency, HealthAvailable: healthAvailable,
			StorageClassification: observabilityDomain.StorageUnknown, Partial: sample.Partial,
		})
		result.Partial = result.Partial || sample.Partial
	}
	return result, nil
}

func activeRuntime(state runtimeDomain.ProjectState) bool {
	switch state {
	case runtimeDomain.StateStarting, runtimeDomain.StateRunning, runtimeDomain.StateRunningExternal,
		runtimeDomain.StatePartiallyRunning, runtimeDomain.StateDegraded, runtimeDomain.StatePaused:
		return true
	case runtimeDomain.StateUnknown, runtimeDomain.StateStopped, runtimeDomain.StateStopping, runtimeDomain.StateFailed:
		return false
	}
	return false
}
