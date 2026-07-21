package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"switchyard.dev/switchyard/internal/foundation/identifier"
	operationsApplication "switchyard.dev/switchyard/internal/operations/application"
	operationsDomain "switchyard.dev/switchyard/internal/operations/domain"
	runtimeDomain "switchyard.dev/switchyard/internal/runtime/domain"
	workspaceApplication "switchyard.dev/switchyard/internal/workspace/application"
	workspaceDomain "switchyard.dev/switchyard/internal/workspace/domain"
)

type runtimeProjectResolver interface {
	ResolveRuntime(context.Context, string) (runtimeDomain.ProjectRuntime, error)
}

type workspaceMemberValidator struct{ runtime runtimeProjectResolver }

func (v workspaceMemberValidator) ValidateWorkspaceMember(ctx context.Context, projectID string) error {
	_, err := v.runtime.ResolveRuntime(ctx, projectID)
	return err
}

type workspaceProjectOperator struct {
	operations *operationsApplication.Coordinator
}

func (o *workspaceProjectOperator) Start(ctx context.Context, projectID string) error {
	return o.execute(ctx, projectID, "start", false)
}

func (o *workspaceProjectOperator) Stop(ctx context.Context, projectID string, options workspaceApplication.StopOptions) error {
	action := "stop"
	if options.RemoveData {
		action = "teardown"
	}
	return o.execute(ctx, projectID, action, options.RemoveData)
}

func (o *workspaceProjectOperator) execute(ctx context.Context, projectID, action string, removeVolumes bool) error {
	if o.operations == nil {
		return errors.New("operation coordinator is unavailable")
	}
	idempotency, err := identifier.New("workspace_child")
	if err != nil {
		return err
	}
	input := []byte(fmt.Sprintf(`{"action":%q,"removeVolumes":%t,"services":[]}`, action, removeVolumes))
	operation, err := o.operations.Submit(ctx, operationsApplication.SubmitRequest{
		ProjectID: projectID, Kind: "runtime." + action, Input: input,
		IdempotencyKey: idempotency, ActorType: "system", ActorID: "workspace-orchestrator",
	})
	if err != nil {
		return err
	}
	operation, err = o.operations.Wait(ctx, operation.ID)
	if err != nil {
		return err
	}
	switch operation.State {
	case operationsDomain.StateSucceeded:
		return nil
	case operationsDomain.StateCancelled:
		return context.Canceled
	case operationsDomain.StateQueued, operationsDomain.StateRunning, operationsDomain.StateFailed, operationsDomain.StatePartiallySucceeded:
		if operation.ErrorMessage != "" {
			return errors.New(operation.ErrorMessage)
		}
		return fmt.Errorf("child operation finished in state %s", operation.State)
	}
	return fmt.Errorf("child operation finished in unknown state %q", operation.State)
}

type workspaceHealthGate struct{ health requiredHealthWaiter }

func (g workspaceHealthGate) WaitHealthy(ctx context.Context, projectID string, _ time.Duration) error {
	return g.health.WaitRequired(ctx, projectID)
}

type workspaceProgress struct {
	progress operationsApplication.Progress
}

func (p workspaceProgress) ProjectProgress(ctx context.Context, result workspaceDomain.ProjectResult) error {
	state, err := workspaceOperationStepState(result.Status)
	if err != nil {
		return err
	}
	return p.progress.Step(ctx, "workspace."+result.ProjectID, state, result.Message)
}

func workspaceOperationStepState(status workspaceDomain.ProjectStatus) (string, error) {
	switch status {
	case workspaceDomain.ProjectQueued,
		workspaceDomain.ProjectStarting,
		workspaceDomain.ProjectCheckingHealth,
		workspaceDomain.ProjectStopping,
		workspaceDomain.ProjectRollingBack:
		return "running", nil
	case workspaceDomain.ProjectRunning,
		workspaceDomain.ProjectStopped,
		workspaceDomain.ProjectRolledBack:
		return "succeeded", nil
	case workspaceDomain.ProjectBlocked,
		workspaceDomain.ProjectStartFailed,
		workspaceDomain.ProjectStopFailed,
		workspaceDomain.ProjectRollbackFailed:
		return "failed", nil
	case workspaceDomain.ProjectCancelled:
		return "cancelled", nil
	default:
		return "", fmt.Errorf("unsupported workspace project status %q", status)
	}
}

type workspaceLauncher interface {
	OpenTerminal(context.Context, string, []string, string) error
	OpenEditor(context.Context, string, string) error
	OpenBrowser(context.Context, string) error
}

type workspaceRecipeRunner struct {
	projects runtimeProjectResolver
	launcher workspaceLauncher
}

func (r workspaceRecipeRunner) RunWorkspaceRecipe(ctx context.Context, recipe workspaceDomain.Recipe) error {
	switch recipe.Kind {
	case workspaceDomain.RecipeOpenURL:
		target, err := url.Parse(recipe.Target)
		if err != nil || target.Host == "" || target.User != nil || target.Scheme != "http" && target.Scheme != "https" {
			return errors.New("recipe URL must be an absolute HTTP URL without credentials")
		}
		return r.launcher.OpenBrowser(ctx, target.String())
	case workspaceDomain.RecipeOpenTerminal, workspaceDomain.RecipeOpenEditor, workspaceDomain.RecipeStartAgent:
		project, err := r.projects.ResolveRuntime(ctx, recipe.ProjectID)
		if err != nil {
			return err
		}
		switch recipe.Kind {
		case workspaceDomain.RecipeOpenTerminal:
			return r.launcher.OpenTerminal(ctx, project.Root, recipe.Arguments, "")
		case workspaceDomain.RecipeOpenEditor:
			provider := strings.TrimSpace(recipe.Target)
			if provider == "" {
				provider = "vscode"
			}
			return r.launcher.OpenEditor(ctx, project.Root, provider)
		case workspaceDomain.RecipeStartAgent:
			provider := strings.TrimSpace(recipe.Target)
			if provider != "codex" && provider != "claude" {
				return fmt.Errorf("unsupported agent provider %q", provider)
			}
			arguments := append([]string{provider}, recipe.Arguments...)
			return r.launcher.OpenTerminal(ctx, project.Root, arguments, "")
		case workspaceDomain.RecipeOpenURL:
			return errors.New("URL recipe does not use a project root")
		}
	}
	return fmt.Errorf("unsupported workspace recipe kind %q", recipe.Kind)
}

type environmentLifecycle interface {
	Started(context.Context, string) error
	Stopped(context.Context, string) error
}
