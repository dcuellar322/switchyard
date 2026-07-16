package compose

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"switchyard.dev/switchyard/internal/runtime/domain"
)

func (d *Driver) inspect(ctx context.Context, project domain.ProjectRuntime, config normalizedConfig) (domain.Observation, error) {
	observation := domain.Observation{
		ProjectID: project.ProjectID, Driver: domain.KindCompose, ProjectIdentity: config.ProjectName,
		Origin: domain.OriginExternal, ObservedAt: time.Now().UTC(),
		Engine: domain.EngineObservation{Context: config.Connection.ContextName},
	}
	engine, ping, version, err := d.engine.Connect(ctx, config.Connection)
	if err != nil {
		return disconnectedObservation(project, config, err), nil
	}
	defer func() { _ = engine.Close() }()
	observation.Engine.Connected = true
	observation.Engine.APIVersion = ping.APIVersion
	observation.Engine.ServerVersion = version.Version
	containers, err := engine.ContainerList(ctx, client.ContainerListOptions{All: true, Filters: projectFilters(config.ProjectName)})
	if err != nil {
		return disconnectedObservation(project, config, err), nil
	}
	items := composeContainers(containers.Items, config.ProjectName)
	ids := make([]string, 0, len(items))
	allOwned := len(items) > 0
	for _, item := range items {
		ids = append(ids, item.ID)
		allOwned = allOwned && d.managed.Owns(config.ProjectName, item.ID)
		service, inspectErr := serviceObservation(ctx, engine, project, item, observation.ObservedAt)
		if inspectErr != nil {
			return domain.Observation{}, inspectErr
		}
		observation.Services = append(observation.Services, service)
	}
	d.managed.Reconcile(config.ProjectName, ids)
	if allOwned || allContainersOwned(d.managed, config.ProjectName, items) {
		observation.Origin = domain.OriginSwitchyard
	}
	sort.Slice(observation.Services, func(i, j int) bool {
		if observation.Services[i].RuntimeName == observation.Services[j].RuntimeName {
			return observation.Services[i].Container.Name < observation.Services[j].Container.Name
		}
		return observation.Services[i].RuntimeName < observation.Services[j].RuntimeName
	})
	observation.State = deriveProjectState(observation.Services, len(config.Services), observation.Origin)
	return observation, nil
}

func disconnectedObservation(project domain.ProjectRuntime, config normalizedConfig, err error) domain.Observation {
	message := ErrEngineUnavailable.Error()
	if err != nil {
		message = err.Error()
	}
	return domain.Observation{
		ProjectID: project.ProjectID, Driver: domain.KindCompose, ProjectIdentity: config.ProjectName,
		State: domain.StateUnknown, Origin: domain.OriginExternal, ObservedAt: time.Now().UTC(),
		Engine: domain.EngineObservation{
			Connected: false, Context: config.Connection.ContextName,
			ErrorCode: "DOCKER_ENGINE_UNAVAILABLE", ErrorMessage: message,
		},
	}
}

func composeContainers(items []container.Summary, projectName string) []container.Summary {
	result := make([]container.Summary, 0, len(items))
	for _, item := range items {
		if item.Labels[labelProject] != projectName || strings.EqualFold(item.Labels[labelOneoff], "True") || item.Labels[labelService] == "" {
			continue
		}
		result = append(result, item)
	}
	return result
}

func allContainersOwned(managed *managedContainers, project string, items []container.Summary) bool {
	if len(items) == 0 {
		return false
	}
	for _, item := range items {
		if !managed.Owns(project, item.ID) {
			return false
		}
	}
	return true
}

func serviceObservation(ctx context.Context, engine engineClient, project domain.ProjectRuntime, item container.Summary, observedAt time.Time) (domain.ServiceObservation, error) {
	inspect, err := engine.ContainerInspect(ctx, item.ID, client.ContainerInspectOptions{})
	if err != nil {
		return domain.ServiceObservation{}, err
	}
	runtimeName := item.Labels[labelService]
	serviceID := productServiceID(project, runtimeName)
	replica, _ := strconv.Atoi(item.Labels[labelNumber])
	if replica > 1 {
		serviceID += "#" + strconv.Itoa(replica)
	}
	result := domain.ServiceObservation{
		ID: serviceID, RuntimeName: runtimeName, State: string(item.State), Health: healthStatus(item), ObservedAt: observedAt,
		Container: domain.ContainerMetadata{
			ID: item.ID, Name: strings.TrimPrefix(inspect.Container.Name, "/"), Image: item.Image,
			CreatedAt: unixTime(item.Created), RestartCount: inspect.Container.RestartCount,
		},
	}
	if state := inspect.Container.State; state != nil {
		result.Container.StartedAt = parseDockerTime(state.StartedAt)
		if !state.Running {
			result.Container.FinishedAt = parseDockerTime(state.FinishedAt)
			exitCode := state.ExitCode
			result.Container.ExitCode = &exitCode
		}
		if state.Paused {
			result.State = "paused"
		}
	}
	for _, port := range item.Ports {
		hostIP := ""
		if port.IP.IsValid() {
			hostIP = port.IP.String()
		}
		result.Ports = append(result.Ports, domain.PublishedPort{
			HostIP: hostIP, HostPort: int(port.PublicPort), ContainerPort: int(port.PrivatePort), Protocol: port.Type,
		})
	}
	return result, nil
}

func productServiceID(project domain.ProjectRuntime, runtimeName string) string {
	for _, service := range project.Services {
		if service.RuntimeName == runtimeName {
			return service.ID
		}
	}
	return runtimeName
}

func healthStatus(item container.Summary) string {
	if item.Health == nil || item.Health.Status == "" {
		return "none"
	}
	return string(item.Health.Status)
}

func deriveProjectState(services []domain.ServiceObservation, declared int, origin domain.Origin) domain.ProjectState {
	if len(services) == 0 {
		return domain.StateStopped
	}
	counts := summarizeServices(services)
	if counts.paused == len(services) {
		return domain.StatePaused
	}
	if counts.stopping > 0 {
		return domain.StateStopping
	}
	if counts.running >= declared && counts.unhealthy > 0 {
		return domain.StateDegraded
	}
	if counts.starting > 0 {
		return domain.StateStarting
	}
	if counts.running >= declared {
		if origin == domain.OriginExternal {
			return domain.StateRunningExternal
		}
		return domain.StateRunning
	}
	if counts.running > 0 {
		return domain.StatePartiallyRunning
	}
	if counts.failed > 0 {
		return domain.StateFailed
	}
	return domain.StateStopped
}

type serviceCounts struct {
	running   int
	paused    int
	starting  int
	stopping  int
	unhealthy int
	failed    int
}

func summarizeServices(services []domain.ServiceObservation) serviceCounts {
	var counts serviceCounts
	for _, service := range services {
		switch service.State {
		case "running":
			counts.running++
			if service.Health == "starting" {
				counts.starting++
			}
		case "paused":
			counts.paused++
		case "restarting", "created":
			counts.starting++
		case "removing":
			counts.stopping++
		case "exited", "dead":
			if service.Container.ExitCode != nil && *service.Container.ExitCode != 0 {
				counts.failed++
			}
		}
		if service.Health == "unhealthy" {
			counts.unhealthy++
		}
	}
	return counts
}

func unixTime(value int64) time.Time {
	if value == 0 {
		return time.Time{}
	}
	return time.Unix(value, 0).UTC()
}

func parseDockerTime(value string) *time.Time {
	if value == "" || strings.HasPrefix(value, "0001-") {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return nil
	}
	parsed = parsed.UTC()
	return &parsed
}
