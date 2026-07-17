package compose

import (
	"context"
	"errors"
	"net/netip"
	"testing"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"switchyard.dev/switchyard/internal/runtime/domain"
)

func TestInspectUsesComposeLabelsAndRecognizesExternalProject(t *testing.T) {
	t.Parallel()
	engine := &fakeEngine{
		containers: []container.Summary{
			{ID: "owned-by-label", Image: "fixture:latest", State: container.ContainerState("running"), Created: 1,
				Labels: map[string]string{labelProject: "fixture", labelService: "web", labelNumber: "1"},
				Ports:  []container.PortSummary{{IP: netip.MustParseAddr("127.0.0.1"), PublicPort: 18080, PrivatePort: 8080, Type: "tcp"}}},
			{ID: "inactive-profile", Image: "fixture:latest", State: container.ContainerState("exited"), Created: 1,
				Labels: map[string]string{labelProject: "fixture", labelService: "marketing", labelNumber: "1"}},
			{ID: "wrong-project", Labels: map[string]string{labelProject: "other", labelService: "web"}},
			{ID: "oneoff", Labels: map[string]string{labelProject: "fixture", labelService: "web", labelOneoff: "True"}},
		},
		inspects: map[string]container.InspectResponse{
			"owned-by-label": {Name: "/fixture-web-1", State: &container.State{Running: true, Status: container.ContainerState("running")}},
		},
	}
	driver := &Driver{engine: fakeConnector{engine: engine, ping: client.PingResult{APIVersion: "1.55"}, version: client.ServerVersionResult{Version: "29.5"}}, managed: newManagedContainers()}
	project := domain.ProjectRuntime{ProjectID: "project-1", Services: []domain.ServiceDeclaration{{ID: "api", RuntimeName: "web"}}}
	observation, err := driver.inspect(context.Background(), project, normalizedConfig{ProjectName: "fixture", Services: []string{"web"}})
	if err != nil {
		t.Fatal(err)
	}
	if observation.State != domain.StateRunningExternal || observation.Origin != domain.OriginExternal {
		t.Fatalf("observation = %#v", observation)
	}
	if len(observation.Services) != 1 || observation.Services[0].ID != "api" || observation.Services[0].Ports[0].HostPort != 18080 {
		t.Fatalf("services = %#v", observation.Services)
	}
}

func TestInspectAttributesContainersCreatedByPendingSwitchyardAction(t *testing.T) {
	t.Parallel()
	engine := &fakeEngine{
		containers: []container.Summary{{
			ID: "started-container", Image: "fixture:latest", State: container.ContainerState("running"),
			Labels: map[string]string{labelProject: "fixture", labelService: "web", labelNumber: "1"},
		}},
		inspects: map[string]container.InspectResponse{
			"started-container": {Name: "/fixture-web-1", State: &container.State{Running: true, Status: container.ContainerState("running")}},
		},
	}
	managed := newManagedContainers()
	managed.RecordAction("fixture", domain.ActionStart, "operation-1")
	stale := managed.OwnershipToken("fixture")
	managed.CompletePending("fixture", "operation-1")
	managed.Reconcile("fixture", []string{"stale-container"}, 1, stale)
	if managed.Owns("fixture", "stale-container") {
		t.Fatal("observation captured during lifecycle execution claimed stale container")
	}
	driver := &Driver{engine: fakeConnector{engine: engine}, managed: managed}

	observation, err := driver.inspect(context.Background(), domain.ProjectRuntime{ProjectID: "project-1"}, normalizedConfig{ProjectName: "fixture", Services: []string{"web"}})
	if err != nil {
		t.Fatal(err)
	}
	if observation.State != domain.StateRunning || observation.Origin != domain.OriginSwitchyard {
		t.Fatalf("observation = %#v", observation)
	}
}

func TestInspectReturnsDisconnectedObservationWithoutDriverError(t *testing.T) {
	t.Parallel()
	driver := &Driver{engine: fakeConnector{err: errors.New("dial failed")}, managed: newManagedContainers()}
	observation, err := driver.inspect(context.Background(), domain.ProjectRuntime{ProjectID: "project-1"}, normalizedConfig{ProjectName: "fixture"})
	if err != nil {
		t.Fatal(err)
	}
	if observation.State != domain.StateUnknown || observation.Engine.Connected || observation.Engine.ErrorCode != "DOCKER_ENGINE_UNAVAILABLE" {
		t.Fatalf("observation = %#v", observation)
	}
}
