package adapters

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"switchyard.dev/switchyard/internal/observability/domain"
)

const composeProjectLabel = "com.docker.compose.project"

type dockerDiskClient interface {
	DiskUsage(context.Context, client.DiskUsageOptions) (client.DiskUsageResult, error)
	Close() error
}

type dockerDiskClientFactory func() (dockerDiskClient, error)

// DockerStorage inspects Engine disk usage and never exposes a mutation method.
type DockerStorage struct {
	clientFactory dockerDiskClientFactory
	now           func() time.Time
	cacheFor      time.Duration
	timeout       time.Duration

	mu       sync.Mutex
	cached   *client.DiskUsageResult
	cacheTil time.Time
}

// NewDockerStorage creates a cached Docker SDK storage observer.
func NewDockerStorage() *DockerStorage {
	return &DockerStorage{
		clientFactory: func() (dockerDiskClient, error) { return client.New(client.FromEnv) },
		now:           time.Now, cacheFor: 2 * time.Minute, timeout: 5 * time.Second,
	}
}

// InspectStorage returns a partial disconnected result rather than disabling process metrics.
func (s *DockerStorage) InspectStorage(ctx context.Context, projects []domain.ProjectDescriptor) (domain.StorageInventory, error) {
	usage, err := s.diskUsage(ctx)
	if err != nil {
		// A disconnected Engine is an explicitly modeled partial result; native
		// process metrics and persisted history remain available.
		//nolint:nilerr
		return domain.StorageInventory{
			Connected: false, ObservedAt: s.now().UTC(), Resources: []domain.StorageResource{}, Projects: []domain.ProjectStorage{},
			Summary: domain.StorageSummary{Classification: domain.StorageUnknown}, Warnings: []string{"Docker Engine storage is unavailable."},
		}, nil
	}
	return classifyDiskUsage(usage, projects, s.now().UTC()), nil
}

func (s *DockerStorage) diskUsage(ctx context.Context) (client.DiskUsageResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now().UTC()
	if s.cached != nil && now.Before(s.cacheTil) {
		return *s.cached, nil
	}
	engine, err := s.clientFactory()
	if err != nil {
		return client.DiskUsageResult{}, err
	}
	defer func() { _ = engine.Close() }()
	timeout := s.timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	inspectCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	usage, err := engine.DiskUsage(inspectCtx, client.DiskUsageOptions{Containers: true, Images: true, Volumes: true, BuildCache: true, Verbose: true})
	if err != nil {
		return client.DiskUsageResult{}, err
	}
	s.cached, s.cacheTil = &usage, now.Add(s.cacheFor)
	return usage, nil
}

func classifyDiskUsage(usage client.DiskUsageResult, projects []domain.ProjectDescriptor, observedAt time.Time) domain.StorageInventory {
	projectByCompose := map[string]string{}
	for _, project := range projects {
		if project.ComposeProjectName != "" {
			projectByCompose[project.ComposeProjectName] = project.ID
		}
	}
	imageProjects := map[string]map[string]struct{}{}
	volumeProjects := map[string]map[string]struct{}{}
	resources := []domain.StorageResource{}
	for _, item := range usage.Containers.Items {
		projectIDs := projectsForLabels(item.Labels, projectByCompose)
		addProjects(imageProjects, item.ImageID, projectIDs)
		for _, mount := range item.Mounts {
			if mount.Name != "" {
				addProjects(volumeProjects, mount.Name, projectIDs)
			}
		}
		classification, reason := domain.StorageUnknown, "Container has no canonical Compose project label."
		if len(projectIDs) == 1 {
			classification, reason = domain.StorageExclusive, "Writable container layer is uniquely associated through the canonical Compose project label."
		}
		resources = append(resources, domain.StorageResource{
			Kind: "container", ID: item.ID, Name: firstName(item.Names, item.ID), ProjectIDs: projectIDs,
			Bytes: nonNegative(item.SizeRw), Reclaimable: inactiveContainer(item.State), Classification: classification, Reason: reason,
		})
	}
	for _, item := range usage.Images.Items {
		projectIDs := setValues(imageProjects[item.ID])
		classification, reason := classifyImage(item.SharedSize, projectIDs)
		resources = append(resources, domain.StorageResource{
			Kind: "image", ID: item.ID, Name: firstName(item.RepoTags, item.ID), ProjectIDs: projectIDs,
			Bytes: nonNegative(item.Size), Reclaimable: item.Containers == 0, Classification: classification, Reason: reason,
		})
	}
	for _, item := range usage.Volumes.Items {
		projectIDs := unionProjects(projectsForLabels(item.Labels, projectByCompose), setValues(volumeProjects[item.Name]))
		var size *int64
		refCount := int64(-1)
		if item.UsageData != nil {
			size, refCount = nonNegative(item.UsageData.Size), item.UsageData.RefCount
		}
		classification, reason := classifyVolume(item.Driver, size, projectIDs)
		resources = append(resources, domain.StorageResource{
			Kind: "volume", ID: item.Name, Name: item.Name, ProjectIDs: projectIDs, Bytes: size,
			Reclaimable: refCount == 0, Classification: classification, Reason: reason,
		})
	}
	for _, item := range usage.BuildCache.Items {
		classification := domain.StorageUnknown
		reason := "Docker build cache records do not expose reliable project labels."
		if item.Shared {
			classification, reason = domain.StorageShared, "Docker reports this build cache record as shared."
		}
		resources = append(resources, domain.StorageResource{
			Kind: "build_cache", ID: item.ID, Name: item.Type, ProjectIDs: []string{}, Bytes: nonNegative(item.Size),
			Reclaimable: !item.InUse, Classification: classification, Reason: reason,
		})
	}
	sort.Slice(resources, func(i, j int) bool {
		if resources[i].Kind != resources[j].Kind {
			return resources[i].Kind < resources[j].Kind
		}
		return resources[i].ID < resources[j].ID
	})
	return domain.StorageInventory{
		Connected: true, ObservedAt: observedAt,
		Summary: domain.StorageSummary{
			Bytes:            max(0, usage.Containers.TotalSize) + max(0, usage.Images.TotalSize) + max(0, usage.Volumes.TotalSize) + max(0, usage.BuildCache.TotalSize),
			ReclaimableBytes: max(0, usage.Containers.Reclaimable) + max(0, usage.Images.Reclaimable) + max(0, usage.Volumes.Reclaimable) + max(0, usage.BuildCache.Reclaimable),
			Classification:   domain.StorageShared, ResourceCount: len(resources),
		},
		Projects: summarizeProjectStorage(projects, resources), Resources: resources, Warnings: []string{"Image layers and build cache may be shared; project values disclose their attribution class."},
	}
}

func summarizeProjectStorage(projects []domain.ProjectDescriptor, resources []domain.StorageResource) []domain.ProjectStorage {
	result := make([]domain.ProjectStorage, 0, len(projects))
	for _, project := range projects {
		value := domain.ProjectStorage{ProjectID: project.ID, Summary: domain.StorageSummary{Classification: domain.StorageExclusive}}
		for _, resource := range resources {
			if !containsString(resource.ProjectIDs, project.ID) {
				continue
			}
			value.Summary.ResourceCount++
			if resource.Bytes == nil {
				value.UnknownSizes++
			} else {
				value.Summary.Bytes += *resource.Bytes
				if resource.Reclaimable {
					value.Summary.ReclaimableBytes += *resource.Bytes
				}
			}
			if resource.Classification == domain.StorageShared {
				value.SharedResources++
			}
			value.Summary.Classification = lessCertain(value.Summary.Classification, resource.Classification)
		}
		if value.Summary.ResourceCount == 0 {
			value.Summary.Classification = domain.StorageUnknown
		}
		result = append(result, value)
	}
	return result
}

func classifyImage(sharedSize int64, projectIDs []string) (domain.StorageClassification, string) {
	if len(projectIDs) == 0 {
		return domain.StorageUnknown, "No project container or canonical label references this image."
	}
	if len(projectIDs) > 1 || sharedSize > 0 {
		return domain.StorageShared, "Image bytes include layers shared across images or projects."
	}
	return domain.StorageEstimated, "Image is associated with one project, but layer exclusivity is not proven."
}

func classifyVolume(driver string, size *int64, projectIDs []string) (domain.StorageClassification, string) {
	if driver != "local" || size == nil {
		return domain.StorageUnknown, "Volume size or local-driver ownership is unavailable."
	}
	if len(projectIDs) == 1 {
		return domain.StorageExclusive, "Local volume is referenced only by one canonical Compose project."
	}
	if len(projectIDs) > 1 {
		return domain.StorageShared, "Volume is referenced by more than one project."
	}
	return domain.StorageUnknown, "Volume has no canonical project ownership evidence."
}

func projectsForLabels(labels map[string]string, projects map[string]string) []string {
	if projectID := projects[labels[composeProjectLabel]]; projectID != "" {
		return []string{projectID}
	}
	return []string{}
}

func addProjects(target map[string]map[string]struct{}, key string, values []string) {
	if key == "" {
		return
	}
	if target[key] == nil {
		target[key] = map[string]struct{}{}
	}
	for _, value := range values {
		target[key][value] = struct{}{}
	}
}

func setValues(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func unionProjects(groups ...[]string) []string {
	values := map[string]struct{}{}
	for _, group := range groups {
		for _, value := range group {
			values[value] = struct{}{}
		}
	}
	return setValues(values)
}

func lessCertain(left, right domain.StorageClassification) domain.StorageClassification {
	rank := map[domain.StorageClassification]int{domain.StorageExclusive: 0, domain.StorageEstimated: 1, domain.StorageShared: 2, domain.StorageUnknown: 3}
	if rank[right] > rank[left] {
		return right
	}
	return left
}

func nonNegative(value int64) *int64 {
	if value < 0 {
		return nil
	}
	return &value
}

func inactiveContainer(state container.ContainerState) bool {
	return state != container.StateRunning && state != container.StatePaused && state != container.StateRestarting
}

func firstName(values []string, fallback string) string {
	if len(values) == 0 || values[0] == "" {
		return shortID(fallback)
	}
	return strings.TrimPrefix(values[0], "/")
}

func shortID(value string) string {
	value = strings.TrimPrefix(value, "sha256:")
	if len(value) > 12 {
		return value[:12]
	}
	return value
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
