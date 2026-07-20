// Package adapters implements trusted project launch and platform PTY boundaries.
package adapters

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	actionsApplication "switchyard.dev/switchyard/internal/actions/application"
	actionsDomain "switchyard.dev/switchyard/internal/actions/domain"
	catalogApplication "switchyard.dev/switchyard/internal/catalog/application"
	catalogDomain "switchyard.dev/switchyard/internal/catalog/domain"
	environmentsDomain "switchyard.dev/switchyard/internal/environments/domain"
	manifestDomain "switchyard.dev/switchyard/internal/manifest/domain"
	"switchyard.dev/switchyard/internal/terminal/application"
	"switchyard.dev/switchyard/internal/terminal/domain"
)

var (
	// ErrUnsupportedTarget identifies an interactive mode unavailable for a runtime.
	ErrUnsupportedTarget = errors.New("interactive target is unsupported by this project runtime")
	// ErrInteractiveActionRequired rejects ordinary manifest commands at the PTY boundary.
	ErrInteractiveActionRequired = errors.New("custom terminal actions must be classified as interactive")
	escalationPattern            = regexp.MustCompile(`(^|[;&|[:space:]])(sudo|doas)([[:space:]]|$)`)
)

// ActionSource exposes accepted actions through an explicit application boundary.
type ActionSource interface {
	ResolveActions(context.Context, string) (actionsDomain.ProjectActions, error)
}

// EnvironmentSource resolves a durable worktree environment.
type EnvironmentSource interface {
	Get(context.Context, string) (environmentsDomain.Environment, error)
}

// Resolver maps typed terminal requests to trusted argument-array commands.
type Resolver struct {
	catalog          *catalogApplication.Service
	actions          ActionSource
	environments     EnvironmentSource
	codexExecutable  string
	claudeExecutable string
}

// NewResolver creates the terminal-facing catalog and environment adapter.
func NewResolver(catalog *catalogApplication.Service, actions ActionSource, environments EnvironmentSource, codexExecutable, claudeExecutable string) *Resolver {
	return &Resolver{catalog: catalog, actions: actions, environments: environments, codexExecutable: codexExecutable, claudeExecutable: claudeExecutable}
}

// Resolve creates no process and reads only accepted project configuration.
func (r *Resolver) Resolve(ctx context.Context, request domain.CreateRequest) (application.LaunchPlan, error) {
	project, err := r.catalog.GetProject(ctx, request.ProjectID)
	if err != nil {
		return application.LaunchPlan{}, err
	}
	if project.TrustState != catalogDomain.TrustTrusted {
		return application.LaunchPlan{}, actionsApplication.ErrProjectUntrusted
	}
	root := project.PrimaryLocation
	composeProjectName := ""
	if request.EnvironmentID != "" {
		environment, environmentErr := r.environments.Get(ctx, request.EnvironmentID)
		if environmentErr != nil {
			return application.LaunchPlan{}, environmentErr
		}
		if environment.ProjectID != request.ProjectID || environment.Availability != environmentsDomain.AvailabilityAvailable {
			return application.LaunchPlan{}, errors.New("terminal environment is not an available checkout of this project")
		}
		root = environment.Path
		composeProjectName = environment.Allocation.ComposeProjectName
	}
	effective, err := r.catalog.EffectiveManifest(ctx, request.ProjectID, nil)
	if err != nil {
		return application.LaunchPlan{}, err
	}
	plan := application.LaunchPlan{
		ProjectID: request.ProjectID, EnvironmentID: request.EnvironmentID,
		DisplayName: project.DisplayName + " shell", WorkingDirectory: root,
	}
	switch request.Kind {
	case domain.KindShell:
		plan.Executable, plan.Arguments = hostShell(request.Shell)
	case domain.KindService:
		return r.composeExec(plan, effective.Manifest, composeProjectName, request.ServiceID, shellName(request.Shell), "service shell")
	case domain.KindDatabase:
		return r.composeExec(plan, effective.Manifest, composeProjectName, request.ServiceID, request.DatabaseClient, request.DatabaseClient)
	case domain.KindAgent:
		plan.Provider = request.Provider
		plan.DisplayName = project.DisplayName + " · " + providerLabel(request.Provider)
		if request.Provider == "codex" {
			plan.Executable = r.codexExecutable
		} else {
			plan.Executable = r.claudeExecutable
		}
	case domain.KindAction:
		return r.interactiveAction(ctx, plan, request.ActionID)
	default:
		return application.LaunchPlan{}, ErrUnsupportedTarget
	}
	return plan, nil
}

func (r *Resolver) composeExec(plan application.LaunchPlan, manifest manifestDomain.Manifest, projectName, serviceID, program, label string) (application.LaunchPlan, error) {
	if manifest.Runtime.Driver != "compose" || manifest.Runtime.Compose == nil {
		return application.LaunchPlan{}, ErrUnsupportedTarget
	}
	runtimeService := ""
	for _, service := range manifest.Services {
		if service.ID == serviceID {
			runtimeService = service.Source.ComposeService
			break
		}
	}
	if runtimeService == "" {
		return application.LaunchPlan{}, fmt.Errorf("declared service %q has no Compose target", serviceID)
	}
	arguments := make([]string, 0, 12)
	if manifest.Runtime.Compose.Context != "" {
		arguments = append(arguments, "--context", manifest.Runtime.Compose.Context)
	}
	resolvedRoot, composeFiles, err := resolveComposeFiles(plan.WorkingDirectory, manifest.Runtime.Compose.Files)
	if err != nil {
		return application.LaunchPlan{}, err
	}
	plan.WorkingDirectory = resolvedRoot
	arguments = append(arguments, "compose", "--project-directory", resolvedRoot)
	for _, file := range composeFiles {
		arguments = append(arguments, "--file", file)
	}
	if projectName == "" {
		projectName = manifest.Runtime.Compose.ProjectName
	}
	if projectName != "" {
		arguments = append(arguments, "--project-name", projectName)
	}
	arguments = append(arguments, "exec", runtimeService, program)
	plan.Executable = "docker"
	plan.Arguments = arguments
	plan.ServiceID = serviceID
	plan.DisplayName = serviceID + " · " + label
	return plan, nil
}

func resolveComposeFiles(root string, files []string) (string, []string, error) {
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", nil, fmt.Errorf("resolve trusted terminal root: %w", err)
	}
	resolvedRoot, err = filepath.Abs(resolvedRoot)
	if err != nil {
		return "", nil, fmt.Errorf("resolve trusted terminal root: %w", err)
	}
	resolvedFiles := make([]string, 0, len(files))
	for _, file := range files {
		candidate := file
		if !filepath.IsAbs(candidate) {
			candidate = filepath.Join(resolvedRoot, candidate)
		}
		resolved, err := filepath.EvalSymlinks(filepath.Clean(candidate))
		if err != nil {
			return "", nil, fmt.Errorf("resolve Compose file: %w", err)
		}
		resolved, err = filepath.Abs(resolved)
		if err != nil {
			return "", nil, fmt.Errorf("resolve Compose file: %w", err)
		}
		relative, err := filepath.Rel(resolvedRoot, resolved)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
			return "", nil, errors.New("compose file leaves the trusted terminal root")
		}
		info, err := os.Stat(resolved)
		if err != nil || !info.Mode().IsRegular() {
			return "", nil, errors.New("compose file must be a regular file")
		}
		resolvedFiles = append(resolvedFiles, resolved)
	}
	return resolvedRoot, resolvedFiles, nil
}

func (r *Resolver) interactiveAction(ctx context.Context, plan application.LaunchPlan, actionID string) (application.LaunchPlan, error) {
	project, err := r.actions.ResolveActions(ctx, plan.ProjectID)
	if err != nil {
		return application.LaunchPlan{}, err
	}
	var action actionsDomain.Definition
	found := false
	for _, candidate := range project.Actions {
		if candidate.ID == actionID {
			action, found = candidate, true
			break
		}
	}
	if !found {
		return application.LaunchPlan{}, actionsApplication.ErrActionNotFound
	}
	if action.Risk != actionsDomain.RiskInteractive {
		return application.LaunchPlan{}, ErrInteractiveActionRequired
	}
	workingDirectory, err := actionsApplication.ResolveWorkingDirectory(plan.WorkingDirectory, action.WorkingDirectory, false)
	if err != nil {
		return application.LaunchPlan{}, err
	}
	plan.WorkingDirectory = workingDirectory
	plan.DisplayName = action.Name
	plan.ActionID = action.ID
	plan.Environment = action.Environment
	switch action.Type {
	case "terminal.open":
		plan.Executable, plan.Arguments = hostShell("")
	case "agent.start":
		if action.Provider != "codex" && action.Provider != "claude" {
			return application.LaunchPlan{}, ErrUnsupportedTarget
		}
		plan.Provider = action.Provider
		if action.Provider == "codex" {
			plan.Executable = r.codexExecutable
		} else {
			plan.Executable = r.claudeExecutable
		}
	default:
		if len(action.Command) == 0 {
			return application.LaunchPlan{}, errors.New("interactive action command is empty")
		}
		if action.Shell {
			if len(action.Command) != 1 || escalationPattern.MatchString(action.Command[0]) {
				return application.LaunchPlan{}, errors.New("interactive shell action is invalid or requests privilege escalation")
			}
			plan.Executable, plan.Arguments = "/bin/sh", []string{"-lc", action.Command[0]}
		} else {
			if action.Command[0] == "sudo" || action.Command[0] == "doas" {
				return application.LaunchPlan{}, errors.New("interactive action requests privilege escalation")
			}
			plan.Executable, plan.Arguments = action.Command[0], append([]string(nil), action.Command[1:]...)
		}
	}
	return plan, nil
}

func hostShell(requested string) (string, []string) {
	if requested == "" {
		candidate := filepath.Base(os.Getenv("SHELL"))
		if candidate == "sh" || candidate == "bash" || candidate == "zsh" {
			requested = candidate
		} else {
			requested = "sh"
		}
	}
	return "/bin/" + requested, []string{"-l"}
}

func shellName(requested string) string {
	if requested == "" {
		return "sh"
	}
	return requested
}

func providerLabel(provider string) string {
	if provider == "codex" {
		return "Codex"
	}
	return "Claude Code"
}
