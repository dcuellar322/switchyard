package compose

import (
	"errors"
	"fmt"
	"slices"

	"switchyard.dev/switchyard/internal/runtime/domain"
)

type executionPlan struct {
	project    domain.ProjectRuntime
	config     normalizedConfig
	invocation domain.Command
}

type commandBuilder struct{}

func (commandBuilder) Build(request domain.PlanRequest, config normalizedConfig) (domain.Plan, error) {
	arguments, err := composeBaseArguments(request.Project, config.Connection, config.ProjectName)
	if err != nil {
		return domain.Plan{}, err
	}
	risk, summary, effects, actionArguments, err := lifecycleArguments(request.Action, request.RemoveVolumes)
	if err != nil {
		return domain.Plan{}, err
	}
	arguments = append(arguments, actionArguments...)
	serviceIDs, runtimeNames, err := selectedComposeServices(request, config)
	if err != nil {
		return domain.Plan{}, err
	}
	arguments = append(arguments, runtimeNames...)
	if len(runtimeNames) > 0 {
		summary = fmt.Sprintf("%s (%d selected)", summary, len(runtimeNames))
		effects = append(effects, "limit lifecycle action to selected services")
	}
	command := domain.Command{Executable: "docker", Arguments: arguments, WorkingDirectory: request.Project.Root}
	return domain.Plan{
		ProjectID: request.Project.ProjectID, Driver: domain.KindCompose, Action: request.Action,
		Risk: risk, Summary: summary, Effects: effects, Services: serviceIDs, Commands: []domain.Command{command},
		RemoveVolumes: request.RemoveVolumes,
		DriverData:    executionPlan{project: request.Project, config: config, invocation: command},
	}, nil
}

func selectedComposeServices(request domain.PlanRequest, config normalizedConfig) ([]string, []string, error) {
	allIDs := make([]string, 0, len(request.Project.Services))
	declared := make(map[string]string, len(request.Project.Services))
	for _, service := range request.Project.Services {
		allIDs = append(allIDs, service.ID)
		declared[service.ID] = service.RuntimeName
	}
	if len(request.Services) == 0 {
		return allIDs, nil, nil
	}
	if request.Action == domain.ActionTeardown {
		return nil, nil, errors.New("compose teardown cannot target individual services")
	}
	selectedIDs := make([]string, 0, len(request.Services))
	runtimeNames := make([]string, 0, len(request.Services))
	seen := make(map[string]struct{}, len(request.Services))
	for _, id := range request.Services {
		if _, duplicate := seen[id]; duplicate {
			return nil, nil, fmt.Errorf("duplicate selected service %q", id)
		}
		seen[id] = struct{}{}
		runtimeName, ok := declared[id]
		if !ok || runtimeName == "" || !slices.Contains(config.Services, runtimeName) {
			return nil, nil, fmt.Errorf("selected service %q is not declared by this Compose project", id)
		}
		selectedIDs = append(selectedIDs, id)
		runtimeNames = append(runtimeNames, runtimeName)
	}
	return selectedIDs, runtimeNames, nil
}

func lifecycleArguments(action domain.Action, removeVolumes bool) (domain.Risk, string, []string, []string, error) {
	switch action {
	case domain.ActionStart:
		return domain.RiskSafe, "Start Compose services", []string{"create or start declared containers"}, []string{"up", "--detach"}, nil
	case domain.ActionStop:
		return domain.RiskCaution, "Stop Compose services", []string{"stop containers", "preserve containers", "preserve volumes"}, []string{"stop"}, nil
	case domain.ActionRestart:
		return domain.RiskCaution, "Restart Compose services", []string{"restart running containers", "preserve volumes"}, []string{"restart"}, nil
	case domain.ActionPause:
		return domain.RiskCaution, "Pause Compose services", []string{"suspend container processes"}, []string{"pause"}, nil
	case domain.ActionUnpause:
		return domain.RiskSafe, "Unpause Compose services", []string{"resume paused container processes"}, []string{"unpause"}, nil
	case domain.ActionRebuild:
		return domain.RiskCaution, "Rebuild and recreate Compose services", []string{"build images", "recreate containers", "preserve named volumes"}, []string{"up", "--detach", "--build", "--force-recreate"}, nil
	case domain.ActionTeardown:
		arguments := []string{"down"}
		effects := []string{"remove Compose containers and networks", "preserve named volumes"}
		if removeVolumes {
			arguments = append(arguments, "--volumes")
			effects[1] = "remove named and anonymous Compose volumes"
		}
		return domain.RiskDestructive, "Tear down Compose resources", effects, arguments, nil
	default:
		return "", "", nil, nil, fmt.Errorf("unsupported Compose action %q", action)
	}
}

func unpackExecutionPlan(plan domain.Plan) (executionPlan, error) {
	value, ok := plan.DriverData.(executionPlan)
	if !ok || len(plan.Commands) != 1 {
		return executionPlan{}, errors.New("compose plan is missing validated driver data")
	}
	return value, nil
}
