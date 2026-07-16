package adapters

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moby/moby/api/types/build"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/api/types/volume"
	"github.com/moby/moby/client"
	"switchyard.dev/switchyard/internal/observability/domain"
)

func TestDockerStorageClassifiesOnlyCanonicalAttributionEvidence(t *testing.T) {
	t.Parallel()
	size := int64(400)
	usage := client.DiskUsageResult{
		Containers: client.ContainersDiskUsage{TotalSize: 30, Reclaimable: 20, Items: []container.Summary{
			{ID: "container-one", Names: []string{"/one-api-1"}, ImageID: "image-shared", SizeRw: 10, State: container.StateExited,
				Labels: map[string]string{composeProjectLabel: "compose-one"}, Mounts: []container.MountPoint{{Name: "shared-data"}}},
			{ID: "container-two", Names: []string{"/two-api-1"}, ImageID: "image-shared", SizeRw: 20, State: container.StateRunning,
				Labels: map[string]string{composeProjectLabel: "compose-two"}, Mounts: []container.MountPoint{{Name: "shared-data"}}},
			{ID: "container-unowned", ImageID: "image-unowned", SizeRw: -1, State: container.StateExited,
				Labels: map[string]string{"project": "compose-one"}},
		}},
		Images: client.ImagesDiskUsage{TotalSize: 800, Reclaimable: 100, Items: []image.Summary{
			{ID: "image-shared", RepoTags: []string{"example/api:latest"}, Size: 600, SharedSize: 200, Containers: 2},
			{ID: "image-unowned", Size: 200, SharedSize: -1, Containers: 0},
		}},
		Volumes: client.VolumesDiskUsage{TotalSize: 400, Reclaimable: 400, Items: []volume.Volume{
			{Name: "shared-data", Driver: "local", Labels: map[string]string{}, UsageData: &volume.UsageData{Size: size, RefCount: 0}},
			{Name: "remote-data", Driver: "nfs", Labels: map[string]string{composeProjectLabel: "compose-one"}, UsageData: &volume.UsageData{Size: -1, RefCount: -1}},
		}},
		BuildCache: client.BuildCacheDiskUsage{TotalSize: 50, Reclaimable: 50, Items: []build.CacheRecord{
			{ID: "cache-shared", Type: "regular", Size: 50, Shared: true},
		}},
	}
	projects := []domain.ProjectDescriptor{
		{ID: "one", ComposeProjectName: "compose-one"},
		{ID: "two", ComposeProjectName: "compose-two"},
	}
	inventory := classifyDiskUsage(usage, projects, time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC))
	if !inventory.Connected || inventory.Summary.Bytes != 1280 || inventory.Summary.ReclaimableBytes != 570 || inventory.Summary.Classification != domain.StorageShared {
		t.Fatalf("summary = %#v", inventory.Summary)
	}
	if len(inventory.Resources) != 8 {
		t.Fatalf("resources = %#v", inventory.Resources)
	}
	assertStorageResource(t, inventory.Resources, "container", "container-one", domain.StorageExclusive, []string{"one"}, true, int64Pointer(10))
	assertStorageResource(t, inventory.Resources, "container", "container-unowned", domain.StorageUnknown, []string{}, true, nil)
	assertStorageResource(t, inventory.Resources, "image", "image-shared", domain.StorageShared, []string{"one", "two"}, false, int64Pointer(600))
	assertStorageResource(t, inventory.Resources, "image", "image-unowned", domain.StorageUnknown, []string{}, true, int64Pointer(200))
	assertStorageResource(t, inventory.Resources, "volume", "shared-data", domain.StorageShared, []string{"one", "two"}, true, int64Pointer(400))
	assertStorageResource(t, inventory.Resources, "volume", "remote-data", domain.StorageUnknown, []string{"one"}, false, nil)
	assertStorageResource(t, inventory.Resources, "build_cache", "cache-shared", domain.StorageShared, []string{}, true, int64Pointer(50))
	one := findProjectStorage(t, inventory.Projects, "one")
	if one.Summary.Classification != domain.StorageUnknown || one.SharedResources != 2 || one.UnknownSizes != 1 {
		t.Fatalf("project one storage = %#v", one)
	}
}

func TestDockerStorageReturnsDisconnectedPartialInventory(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	storage := &DockerStorage{
		clientFactory: func() (dockerDiskClient, error) { return nil, errors.New("socket unavailable") },
		now:           func() time.Time { return now }, cacheFor: 2 * time.Minute,
	}
	inventory, err := storage.InspectStorage(context.Background(), []domain.ProjectDescriptor{{ID: "process-only"}})
	if err != nil {
		t.Fatalf("InspectStorage() error = %v", err)
	}
	if inventory.Connected || inventory.Summary.Classification != domain.StorageUnknown || !inventory.ObservedAt.Equal(now) || len(inventory.Warnings) != 1 {
		t.Fatalf("disconnected inventory = %#v", inventory)
	}
}

func TestDockerStorageCachesReadOnlyDiskUsageWithBoundedOptions(t *testing.T) {
	t.Parallel()
	clock := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	clients := []*dockerDiskClientFake{}
	storage := &DockerStorage{
		clientFactory: func() (dockerDiskClient, error) {
			engine := &dockerDiskClientFake{result: client.DiskUsageResult{Containers: client.ContainersDiskUsage{TotalSize: int64(len(clients) + 1)}}}
			clients = append(clients, engine)
			return engine, nil
		},
		now: func() time.Time { return clock }, cacheFor: 2 * time.Minute,
	}
	first, err := storage.InspectStorage(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	clock = clock.Add(time.Minute)
	second, err := storage.InspectStorage(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if first.Summary.Bytes != second.Summary.Bytes || len(clients) != 1 || clients[0].calls != 1 || clients[0].closes != 1 {
		t.Fatalf("cache clients=%d first=%#v second=%#v fake=%#v", len(clients), first.Summary, second.Summary, clients[0])
	}
	if options := clients[0].options; !options.Containers || !options.Images || !options.Volumes || !options.BuildCache || !options.Verbose {
		t.Fatalf("DiskUsage options = %#v", options)
	}
	clock = clock.Add(2 * time.Minute)
	third, err := storage.InspectStorage(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(clients) != 2 || third.Summary.Bytes == first.Summary.Bytes {
		t.Fatalf("expired cache clients=%d first=%d third=%d", len(clients), first.Summary.Bytes, third.Summary.Bytes)
	}
}

type dockerDiskClientFake struct {
	result  client.DiskUsageResult
	err     error
	options client.DiskUsageOptions
	calls   int
	closes  int
}

func (f *dockerDiskClientFake) DiskUsage(_ context.Context, options client.DiskUsageOptions) (client.DiskUsageResult, error) {
	f.calls++
	f.options = options
	return f.result, f.err
}

func (f *dockerDiskClientFake) Close() error {
	f.closes++
	return nil
}

func assertStorageResource(t testing.TB, resources []domain.StorageResource, kind, id string, classification domain.StorageClassification, projects []string, reclaimable bool, bytes *int64) {
	t.Helper()
	for _, resource := range resources {
		if resource.Kind != kind || resource.ID != id {
			continue
		}
		if resource.Classification != classification || resource.Reclaimable != reclaimable || !sameStrings(resource.ProjectIDs, projects) || !sameInt64Pointer(resource.Bytes, bytes) {
			t.Fatalf("resource %s/%s = %#v", kind, id, resource)
		}
		if resource.Reason == "" {
			t.Fatalf("resource %s/%s has no attribution reason", kind, id)
		}
		return
	}
	t.Fatalf("resource %s/%s not found", kind, id)
}

func findProjectStorage(t testing.TB, projects []domain.ProjectStorage, projectID string) domain.ProjectStorage {
	t.Helper()
	for _, project := range projects {
		if project.ProjectID == projectID {
			return project
		}
	}
	t.Fatalf("project storage %q not found", projectID)
	return domain.ProjectStorage{}
}

func sameStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func int64Pointer(value int64) *int64 { return &value }

func sameInt64Pointer(left, right *int64) bool {
	return left == nil && right == nil || left != nil && right != nil && *left == *right
}
