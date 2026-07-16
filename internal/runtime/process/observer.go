package process

import (
	"context"
	"sort"
	"time"

	"switchyard.dev/switchyard/internal/runtime/domain"
)

func (d *Driver) inspect(ctx context.Context, project domain.ProjectRuntime) (domain.Observation, error) {
	observation := domain.Observation{
		ProjectID: project.ProjectID, Driver: domain.KindProcess, ProjectIdentity: project.ProjectSlug,
		Origin: domain.OriginExternal, ObservedAt: d.now().UTC(), Services: []domain.ServiceObservation{},
	}
	if project.Process == nil {
		observation.State = domain.StateUnknown
		return observation, ErrInvalidProcessPlan
	}
	runs, err := d.store.ListProjectRuns(ctx, project.ProjectID)
	if err != nil {
		return domain.Observation{}, err
	}
	latest := make(map[string]domain.RunRecord)
	active := make(map[string]domain.RunRecord)
	for _, run := range runs {
		if _, exists := latest[run.ServiceID]; !exists {
			latest[run.ServiceID] = run
		}
		if run.EndedAt == nil {
			active[run.ServiceID] = run
		}
	}
	plans, err := orderedServices(project)
	if err != nil {
		return domain.Observation{}, err
	}
	managedRunning, externalRunning, starting, failed, stale := 0, 0, 0, 0, 0
	for _, service := range plans {
		serviceObservation := domain.ServiceObservation{
			ID: service.service.ID, RuntimeName: service.service.RuntimeName,
			State: "stopped", Health: "none", Ports: declaredProcessPorts(service.service), ObservedAt: observation.ObservedAt,
		}
		if run, ok := active[service.service.ID]; ok {
			verified, verifyErr := verifiedRunMembers(ctx, d.inspector, run)
			if verifyErr != nil {
				return domain.Observation{}, verifyErr
			}
			switch {
			case len(verified) > 0:
				identity, _ := primaryIdentity(verified)
				serviceObservation.State = "running"
				serviceObservation.Process = processMetadata(run, identity)
				managedRunning++
				d.recordRecoveredMembers(ctx, run, verified)
			case observation.ObservedAt.Sub(run.StartedAt) < identityHandoffGrace:
				serviceObservation.State = "starting"
				serviceObservation.Process = historicalProcessMetadata(run)
				starting++
			default:
				serviceObservation.State = "stale"
				serviceObservation.Process = historicalProcessMetadata(run)
				stale++
				_ = d.store.FinishRun(context.WithoutCancel(ctx), run.ID, observation.ObservedAt, nil, "identity_lost")
			}
		} else if identity, found, externalErr := d.externalService(ctx, project, service); externalErr != nil {
			serviceObservation.State = "unknown"
			stale++
		} else if found {
			serviceObservation.State = "running"
			serviceObservation.Process = &domain.ProcessMetadata{
				PID: identity.PID, ProcessGroup: identity.ProcessGroup, Executable: identity.Executable,
				WorkingDirectory: identity.WorkingDirectory, StartedAt: timePointer(identity.StartedAt), Fingerprint: identity.Fingerprint,
			}
			externalRunning++
		} else if run, ok := latest[service.service.ID]; ok && run.EndedAt != nil {
			serviceObservation.Process = historicalProcessMetadata(run)
			if run.ExitCode != nil && *run.ExitCode != 0 {
				serviceObservation.State = "failed"
				failed++
			}
		}
		observation.Services = append(observation.Services, serviceObservation)
	}
	observation.State, observation.Origin = processProjectState(
		len(plans), managedRunning, externalRunning, starting, failed, stale,
	)
	return observation, nil
}

func processProjectState(declared, managed, external, starting, failed, stale int) (domain.ProjectState, domain.Origin) {
	running := managed + external
	if running == 0 {
		if starting > 0 {
			return domain.StateStarting, domain.OriginSwitchyard
		}
		if failed > 0 {
			return domain.StateFailed, domain.OriginSwitchyard
		}
		if stale > 0 {
			return domain.StateUnknown, domain.OriginExternal
		}
		return domain.StateStopped, domain.OriginExternal
	}
	if running < declared || failed > 0 || stale > 0 || (managed > 0 && external > 0) {
		return domain.StatePartiallyRunning, domain.OriginExternal
	}
	if external == declared {
		return domain.StateRunningExternal, domain.OriginExternal
	}
	return domain.StateRunning, domain.OriginSwitchyard
}

func (d *Driver) externalService(
	ctx context.Context,
	_ domain.ProjectRuntime,
	service servicePlan,
) (domain.ProcessIdentity, bool, error) {
	if service.definition.Shell || len(service.definition.Command) == 0 {
		return domain.ProcessIdentity{}, false, nil
	}
	for _, port := range service.service.HostPorts {
		listeners, err := d.inspector.Listeners(ctx, port)
		if err != nil {
			return domain.ProcessIdentity{}, false, err
		}
		for _, identity := range listeners {
			if executableMatches(service.definition.Command[0], identity) ||
				d.inspector.MatchesCommand(ctx, identity.PID, service.definition.Command[0]) {
				return identity, true, nil
			}
		}
	}
	return domain.ProcessIdentity{}, false, nil
}

func (d *Driver) recordRecoveredMembers(ctx context.Context, run domain.RunRecord, verified []domain.ProcessIdentity) {
	groups := uniqueGroups(verified)
	for _, group := range groups {
		members, err := d.inspector.GroupMembers(ctx, group)
		if err != nil {
			continue
		}
		for _, member := range members {
			member.RunID = run.ID
			_ = d.store.RecordProcess(context.WithoutCancel(ctx), member)
		}
	}
}

func processMetadata(run domain.RunRecord, identity domain.ProcessIdentity) *domain.ProcessMetadata {
	return &domain.ProcessMetadata{
		RunID: run.ID, PID: identity.PID, ProcessGroup: identity.ProcessGroup, Executable: identity.Executable,
		WorkingDirectory: identity.WorkingDirectory, StartedAt: timePointer(identity.StartedAt),
		RestartCount: run.RestartCount, Fingerprint: identity.Fingerprint,
	}
}

func historicalProcessMetadata(run domain.RunRecord) *domain.ProcessMetadata {
	if len(run.Processes) == 0 {
		return &domain.ProcessMetadata{RunID: run.ID, ExitCode: run.ExitCode, FinishedAt: run.EndedAt, RestartCount: run.RestartCount}
	}
	identities := append([]domain.ProcessIdentity(nil), run.Processes...)
	sort.Slice(identities, func(i, j int) bool { return identities[i].ObservedAt.After(identities[j].ObservedAt) })
	metadata := processMetadata(run, identities[0])
	metadata.FinishedAt = run.EndedAt
	metadata.ExitCode = run.ExitCode
	return metadata
}

func timePointer(value time.Time) *time.Time {
	value = value.UTC()
	return &value
}

func declaredProcessPorts(service domain.ServiceDeclaration) []domain.PublishedPort {
	result := make([]domain.PublishedPort, 0, len(service.HostPorts))
	for _, port := range service.HostPorts {
		result = append(result, domain.PublishedPort{
			HostIP: "127.0.0.1", HostPort: port, ContainerPort: port, Protocol: "tcp",
		})
	}
	return result
}
