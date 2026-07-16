package process

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"switchyard.dev/switchyard/internal/runtime/domain"
)

type servicePlan struct {
	service     domain.ServiceDeclaration
	definition  domain.ProcessDefinition
	operationID string
}

type executionPlan struct {
	project  domain.ProjectRuntime
	action   domain.Action
	services []servicePlan
}

func buildPlan(request domain.PlanRequest) (domain.Plan, error) {
	if request.Project.Kind != domain.KindProcess || request.Project.Process == nil {
		return domain.Plan{}, fmt.Errorf("%w: missing process runtime", ErrInvalidProcessPlan)
	}
	if request.RemoveVolumes {
		return domain.Plan{}, errors.New("native process runtimes do not own volumes")
	}
	if request.Action != domain.ActionStart && request.Action != domain.ActionStop && request.Action != domain.ActionRestart {
		return domain.Plan{}, fmt.Errorf("native process runtimes do not support %s", request.Action)
	}
	services, err := orderedServices(request.Project)
	if err != nil {
		return domain.Plan{}, err
	}
	for _, service := range services {
		if len(service.definition.Command) == 0 {
			return domain.Plan{}, fmt.Errorf("process %q has no command", service.definition.ID)
		}
		if !service.definition.Shell && shellSyntax(service.definition.Command) {
			return domain.Plan{}, fmt.Errorf("process %q requires explicit shell opt-in", service.definition.ID)
		}
		if service.definition.Shell && len(service.definition.Command) != 1 {
			return domain.Plan{}, fmt.Errorf("shell process %q must contain one command string", service.definition.ID)
		}
	}
	if request.Action == domain.ActionStop {
		reverseServices(services)
	}
	plan := domain.Plan{
		ProjectID: request.Project.ProjectID, Driver: domain.KindProcess, Action: request.Action,
		Risk: processRisk(request.Action), RemoveVolumes: false,
		Summary:    fmt.Sprintf("%s %d native process service(s) for %s", request.Action, len(services), request.Project.ProjectSlug),
		Effects:    []string{"Process ownership will be verified using durable identity fingerprints."},
		DriverData: executionPlan{project: request.Project, action: request.Action, services: services},
	}
	for _, service := range services {
		command := previewCommand(request.Project.Root, service.definition)
		plan.Commands = append(plan.Commands, command)
		environmentCount := len(request.Project.Process.Environment) + len(service.definition.Environment)
		secretCount := len(request.Project.Process.Secrets) + len(service.definition.Secrets)
		plan.Effects = append(plan.Effects, fmt.Sprintf(
			"Service %s uses %d environment overlay(s) and %d keychain reference(s).",
			service.service.ID, environmentCount, secretCount,
		))
	}
	return plan, nil
}

func orderedServices(project domain.ProjectRuntime) ([]servicePlan, error) {
	definitions := make(map[string]domain.ProcessDefinition, len(project.Process.Processes))
	for _, definition := range project.Process.Processes {
		definitions[definition.ID] = definition
	}
	services := make(map[string]domain.ServiceDeclaration, len(project.Services))
	for _, service := range project.Services {
		services[service.ID] = service
	}
	state := make(map[string]uint8, len(services))
	result := make([]servicePlan, 0, len(services))
	var visit func(string) error
	visit = func(id string) error {
		service, ok := services[id]
		if !ok {
			return fmt.Errorf("service %q is not declared", id)
		}
		if state[id] == 1 {
			return fmt.Errorf("service dependency cycle includes %q", id)
		}
		if state[id] == 2 {
			return nil
		}
		state[id] = 1
		dependencies := append([]string(nil), service.Dependencies...)
		sort.Strings(dependencies)
		for _, dependency := range dependencies {
			if err := visit(dependency); err != nil {
				return err
			}
		}
		definition, ok := definitions[service.RuntimeName]
		if !ok {
			return fmt.Errorf("service %q references missing process %q", id, service.RuntimeName)
		}
		state[id] = 2
		result = append(result, servicePlan{service: service, definition: definition})
		return nil
	}
	ids := make([]string, 0, len(services))
	for id := range services {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if err := visit(id); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func reverseServices(services []servicePlan) {
	for left, right := 0, len(services)-1; left < right; left, right = left+1, right-1 {
		services[left], services[right] = services[right], services[left]
	}
}

func previewCommand(root string, definition domain.ProcessDefinition) domain.Command {
	workingDirectory := definition.WorkingDirectory
	if workingDirectory == "" {
		workingDirectory = "."
	}
	if !filepath.IsAbs(workingDirectory) {
		workingDirectory = filepath.Join(root, workingDirectory)
	}
	if definition.Shell {
		executable, arguments := shellCommand(definition.Command[0])
		return domain.Command{Executable: executable, Arguments: arguments, WorkingDirectory: filepath.Clean(workingDirectory)}
	}
	return domain.Command{
		Executable: definition.Command[0], Arguments: append([]string(nil), definition.Command[1:]...),
		WorkingDirectory: filepath.Clean(workingDirectory),
	}
}

func processRisk(action domain.Action) domain.Risk {
	if action == domain.ActionStart {
		return domain.RiskSafe
	}
	return domain.RiskCaution
}

func shellSyntax(command []string) bool {
	if len(command) == 0 || strings.ContainsAny(command[0], " \t\r\n|&;<>()$`") {
		return true
	}
	base := strings.ToLower(strings.TrimSuffix(filepath.Base(command[0]), ".exe"))
	for _, shell := range []string{"sh", "bash", "zsh", "fish", "cmd", "powershell", "pwsh"} {
		if base == shell {
			return true
		}
	}
	return false
}
