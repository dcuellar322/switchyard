// Package bootstrap composes process-level Switchyard adapters and use cases.
package bootstrap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	actionsAdapters "switchyard.dev/switchyard/internal/actions/adapters"
	actionsApplication "switchyard.dev/switchyard/internal/actions/application"
	agentsAdapters "switchyard.dev/switchyard/internal/agents/adapters"
	agentsApplication "switchyard.dev/switchyard/internal/agents/application"
	claudeProvider "switchyard.dev/switchyard/internal/agents/providers/claude"
	codexProvider "switchyard.dev/switchyard/internal/agents/providers/codex"
	openAIProvider "switchyard.dev/switchyard/internal/agents/providers/openai"
	catalog "switchyard.dev/switchyard/internal/catalog/application"
	discoveryAdapters "switchyard.dev/switchyard/internal/discovery/adapters"
	environmentsAdapters "switchyard.dev/switchyard/internal/environments/adapters"
	environmentsApplication "switchyard.dev/switchyard/internal/environments/application"
	"switchyard.dev/switchyard/internal/foundation/buildinfo"
	observabilityAdapters "switchyard.dev/switchyard/internal/observability/adapters"
	observabilityApplication "switchyard.dev/switchyard/internal/observability/application"
	operations "switchyard.dev/switchyard/internal/operations/application"
	"switchyard.dev/switchyard/internal/operations/domain"
	hostPlatform "switchyard.dev/switchyard/internal/platform/host"
	"switchyard.dev/switchyard/internal/platform/sqlite"
	pluginsApplication "switchyard.dev/switchyard/internal/plugins/application"
	portsAdapters "switchyard.dev/switchyard/internal/ports/adapters"
	portsApplication "switchyard.dev/switchyard/internal/ports/application"
	routingAdapters "switchyard.dev/switchyard/internal/routing/adapters"
	routingApplication "switchyard.dev/switchyard/internal/routing/application"
	runtimeApplication "switchyard.dev/switchyard/internal/runtime/application"
	"switchyard.dev/switchyard/internal/runtime/compose"
	runtimeDomain "switchyard.dev/switchyard/internal/runtime/domain"
	processRuntime "switchyard.dev/switchyard/internal/runtime/process"
	session "switchyard.dev/switchyard/internal/session/application"
	sourcecontrolAdapters "switchyard.dev/switchyard/internal/sourcecontrol/adapters"
	sourcecontrolApplication "switchyard.dev/switchyard/internal/sourcecontrol/application"
	"switchyard.dev/switchyard/internal/system/application"
	terminalAdapters "switchyard.dev/switchyard/internal/terminal/adapters"
	terminalApplication "switchyard.dev/switchyard/internal/terminal/application"
	terminalDomain "switchyard.dev/switchyard/internal/terminal/domain"
	"switchyard.dev/switchyard/internal/transport/httpapi"
	eventtransport "switchyard.dev/switchyard/internal/transport/websocket"
	workspaceApplication "switchyard.dev/switchyard/internal/workspace/application"
	workspaceDomain "switchyard.dev/switchyard/internal/workspace/domain"
	"switchyard.dev/switchyard/web"
)

// RunDaemon starts the migrated local control plane and blocks until shutdown.
func RunDaemon(ctx context.Context, config Config) error {
	config.Logger = daemonLogger(config.Logger)
	if err := validateDaemonConfig(config); err != nil {
		return err
	}
	if err := os.MkdirAll(config.DataDir, 0o700); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}
	lock, err := acquireLock(filepath.Join(config.DataDir, "daemon.lock"))
	if err != nil {
		return err
	}
	defer func() {
		if err := lock.release(); err != nil {
			config.Logger.Error("release daemon lock", "component", "bootstrap", "error", err)
		}
	}()

	database, err := sqlite.Open(ctx, filepath.Join(config.DataDir, "switchyard.db"))
	if err != nil {
		return err
	}
	defer func() {
		if err := database.Close(); err != nil {
			config.Logger.Error("close database", "component", "bootstrap", "error", err)
		}
	}()

	journal := sqlite.NewJournal(database)
	catalogService := catalog.NewService(sqlite.NewCatalogRepository(database), discoveryAdapters.Defaults())
	gitService := sourcecontrolApplication.NewService(sourcecontrolApplication.NewCatalogSource(catalogService), sourcecontrolAdapters.NewGit())
	environmentService := environmentsApplication.NewService(
		environmentsAdapters.NewCatalogSource(catalogService), environmentsAdapters.NewSourceControlSource(gitService),
		sqlite.NewEnvironmentRepository(database),
	)
	redactor, err := observabilityAdapters.NewRedactor(config.RedactionPatterns)
	if err != nil {
		return fmt.Errorf("compile log redaction patterns: %w", err)
	}
	runtimeSource := runtimeApplication.NewCatalogSource(catalogService, environmentsAdapters.NewRuntimeSource(environmentService))
	runtimeService := runtimeApplication.NewService(
		runtimeSource,
		compose.NewDriverWithArtifacts(filepath.Join(config.DataDir, "runtime", "compose")),
		processRuntime.NewDriver(ctx, sqlite.NewRunRepository(database), redactor),
	)
	logStore, err := sqlite.NewLogStore(database, sqlite.LogStoreConfig{
		Directory: filepath.Join(config.DataDir, "logs"), RingCapacity: config.LogRingCapacity,
		SegmentBytes: config.LogSegmentBytes, RetentionAge: config.LogRetentionAge, RetentionBytes: config.LogRetentionBytes,
	}, redactor)
	if err != nil {
		return err
	}
	defer func() {
		if err := logStore.Close(context.Background()); err != nil {
			config.Logger.Error("close log store", "component", "observability", "error", err)
		}
	}()
	logService := observabilityApplication.NewLogService(runtimeService, logStore)
	healthService := observabilityApplication.NewHealthService(runtimeSource, runtimeService, sqlite.NewHealthRepository(database), observabilityAdapters.NewHealthEvaluator())
	resourceService, err := observabilityApplication.NewResourceService(
		observabilityAdapters.NewResourceCatalogSource(catalogService),
		observabilityAdapters.NewResourceRuntimeSource(runtimeService, healthService),
		database,
		observabilityAdapters.NewDockerStorage(),
		observabilityApplication.ResourceConfig{
			SampleInterval: config.MetricSampleInterval, RawRetention: config.MetricRawRetention,
			MinuteRetention: config.MetricMinuteRetention, QuarterHourRetention: config.MetricQuarterHourRetention,
			MaximumHistoryPoints: config.MetricMaximumHistoryPoints,
			LogRetentionAge:      config.LogRetentionAge, LogRetentionBytes: config.LogRetentionBytes,
		},
	)
	if err != nil {
		return err
	}
	portDeclarations := portsApplication.NewCatalogSource(catalogService)
	portService := portsApplication.NewService(
		portDeclarations,
		portsAdapters.NewRuntimeBindings(catalogService, runtimeService),
		portsAdapters.NewOSListeners(),
		sqlite.NewPortReservationRepository(database),
		environmentsAdapters.NewPortLeaseSource(environmentService),
	)
	environmentRegistration := environmentsApplication.NewRegistrationCoordinator(
		environmentService, environmentsAdapters.NewPortDeclarations(portDeclarations), environmentsAdapters.NewPortAllocator(portService),
	)
	routeService := routingApplication.NewService(config.RoutingAddr != "")
	routeRegistry := routingAdapters.NewRegistry(routeService, routingAdapters.NewEnvironmentSource(environmentService))
	if _, err := routeRegistry.Refresh(ctx); err != nil {
		return err
	}
	environmentRuntime := environmentsAdapters.NewLifecycle(environmentService, runtimeService, func(refreshCtx context.Context) error {
		_, refreshErr := routeRegistry.Refresh(refreshCtx)
		return refreshErr
	})
	actionAudits := sqlite.NewActionAuditRepository(database)
	if err := actionAudits.Recover(ctx, time.Now().UTC()); err != nil {
		return err
	}
	actionCatalog := actionsApplication.NewCatalogSource(catalogService)
	actionService := actionsApplication.NewService(
		actionCatalog,
		actionsAdapters.NewRunner(actionsAdapters.NewLauncher()),
		actionAudits,
	)
	providerRegistry, err := agentsApplication.NewRegistry(
		codexProvider.NewProposalProvider(codexProvider.ProposalConfig{Executable: config.AICodexExecutable, Model: config.AICodexModel, Redactor: redactor}),
		claudeProvider.NewProposalProvider(claudeProvider.ProposalConfig{Executable: config.AIClaudeExecutable, Model: config.AIClaudeModel, Redactor: redactor}),
		openAIProvider.NewProposalProvider(openAIProvider.ProposalConfig{Endpoint: config.AIOpenAIEndpoint, Model: config.AIOpenAIModel, APIKey: os.Getenv(config.AIOpenAIAPIKeyEnv), Redactor: redactor}),
	)
	if err != nil {
		return err
	}
	terminalService, err := terminalApplication.NewService(
		ctx,
		sqlite.NewTerminalSessionRepository(database),
		terminalAdapters.NewResolver(catalogService, actionCatalog, environmentService, config.AICodexExecutable, config.AIClaudeExecutable),
		terminalAdapters.NewPTY(),
		terminalApplication.DefaultConfig(),
	)
	if err != nil {
		return err
	}
	defer closeTerminalSessions(config.Logger, terminalService)
	aiService, err := agentsApplication.NewEnhancementService(
		catalogService, sqlite.NewAgentRunRepository(database), agentsAdapters.RepositoryReader{}, redactor, agentsAdapters.ManifestValidator{}, providerRegistry,
	)
	if err != nil {
		return err
	}
	build := buildinfo.Current()
	pluginService, err := newPluginService(ctx, database, catalogService, redactor, config.DataDir, build.Version, config.Logger)
	if err != nil {
		return err
	}
	operationRepository := sqlite.NewOperationRepository(database)
	launcher := actionsAdapters.NewLauncher()
	var workspaceService *workspaceApplication.Service
	coordinator := operations.NewCoordinator(ctx, operationRepository, journal, operations.ExecutorFunc(
		func(operationCtx context.Context, operation domain.Operation, progress operations.Progress) error {
			return executeOperation(
				operationCtx, runtimeService, healthService, actionService, aiService, pluginService, workspaceService,
				workspaceRecipeRunner{projects: runtimeSource, launcher: launcher}, environmentRuntime, operation, progress,
			)
		},
	))
	workspaceService = workspaceApplication.NewService(
		sqlite.NewWorkspaceRepository(database), &workspaceProjectOperator{operations: coordinator},
		workspaceHealthGate{health: healthService}, workspaceMemberValidator{runtime: runtimeSource},
	)
	if err := workspaceService.Recover(ctx); err != nil {
		return err
	}
	if err := coordinator.Recover(ctx); err != nil {
		return err
	}
	reconcileSink := runtimeReconciliationSink{runtime: runtimeService, journal: journal}
	var background sync.WaitGroup
	background.Add(5)
	go func() {
		defer background.Done()
		runtimeService.WatchAll(ctx, reconcileSink, func(projectID string, watchErr error) {
			config.Logger.Warn("runtime event watcher unavailable", "component", "runtime", "project_id", projectID, "error", watchErr)
		})
	}()
	go func() {
		defer background.Done()
		healthService.Run(ctx, func(projectID string, healthErr error) {
			config.Logger.Warn("health observer unavailable", "component", "observability", "project_id", projectID, "error", healthErr)
		})
	}()
	go func() {
		defer background.Done()
		logService.Run(ctx, func(projectID string, logErr error) {
			config.Logger.Warn("log collector unavailable", "component", "observability", "project_id", projectID, "error", logErr)
		})
	}()
	go func() {
		defer background.Done()
		resourceService.Run(ctx, func(resourceErr error) {
			config.Logger.Warn("resource sampler unavailable", "component", "observability", "error", resourceErr)
		})
	}()
	go func() {
		defer background.Done()
		pluginService.RunHealth(ctx, 30*time.Second)
	}()
	sessions := session.NewManager()
	system := application.NewQuery(database, buildinfo.Current(), time.Now())
	host := application.NewHostQuery(hostPlatform.NewObserver())
	dependencies := httpapi.Dependencies{
		System: system, Host: host, Operations: coordinator, Sessions: sessions, Catalog: catalogService, Runtime: runtimeService,
		Health: healthService, LogService: logService, Events: eventtransport.NewEvents(journal), Logs: eventtransport.NewLogs(logService),
		Ports: portService, Git: gitService, Actions: actionService,
		AI: aiService, Resources: resourceService, Plugins: pluginService,
		Workspaces: workspaceService, Environments: environmentService, EnvironmentRegistration: environmentRegistration, Routes: routeRegistry,
		Terminals: terminalService, Terminal: eventtransport.NewTerminal(terminalService, func(actorContext context.Context) terminalDomain.Owner {
			actorType, actorID := httpapi.RequestActor(actorContext)
			return terminalDomain.Owner{Type: actorType, ID: actorID}
		}),
		Web: web.Handler(), Logger: config.Logger,
	}
	servers, err := newLocalServers(config, dependencies, routingAdapters.NewProxy(routeRegistry))
	if err != nil {
		return err
	}
	config.Logger.Info(
		"switchyard daemon ready",
		"component", "bootstrap",
		"address", servers.browserAddress(),
		"ipc_address", servers.ipcAddress,
		"routing_address", config.RoutingAddr,
		"data_dir", config.DataDir,
	)
	runErr := servers.run(ctx, coordinator.Shutdown)
	background.Wait()
	return runErr
}

func daemonLogger(logger *slog.Logger) *slog.Logger {
	if logger == nil {
		return slog.Default()
	}
	return logger
}

func closeTerminalSessions(logger *slog.Logger, service *terminalApplication.Service) {
	closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := service.Close(closeCtx); err != nil {
		logger.Error("close terminal sessions", "component", "terminal", "error", err)
	}
}

func executeOperation(
	ctx context.Context,
	runtimeService *runtimeApplication.Service,
	health requiredHealthWaiter,
	actions *actionsApplication.Service,
	ai *agentsApplication.EnhancementService,
	plugins *pluginsApplication.Service,
	workspaces *workspaceApplication.Service,
	recipes workspaceApplication.RecipeRunner,
	environments environmentLifecycle,
	operation domain.Operation,
	progress operations.Progress,
) error {
	switch operation.Kind {
	case "manifest.enhance":
		return executeManifestEnhancement(ctx, ai, operation, progress)
	case "action.run":
		return executeActionOperation(ctx, actions, operation, progress)
	case "plugin.operate":
		return executePluginOperation(ctx, plugins, operation, progress)
	case "workspace.start", "workspace.stop":
		return executeWorkspaceOperation(ctx, workspaces, recipes, operation, progress)
	default:
		return executeRuntimeOperation(ctx, runtimeService, health, environments, operation, progress)
	}
}

func executeManifestEnhancement(ctx context.Context, ai *agentsApplication.EnhancementService, operation domain.Operation, progress operations.Progress) error {
	var input struct {
		ProposalID string                   `json:"proposalId"`
		Provider   string                   `json:"provider"`
		Limits     agentsApplication.Limits `json:"limits"`
	}
	if err := json.Unmarshal(operation.Input, &input); err != nil || input.ProposalID == "" || input.Provider == "" {
		return errors.New("decode manifest enhancement operation")
	}
	_ = progress.Step(ctx, "manifest.evidence", "succeeded", "redacted evidence bundle prepared")
	_ = progress.Step(ctx, "manifest.provider", "running", "provider proposal requested")
	err := ai.Execute(ctx, operation.ID, input.ProposalID, input.Provider, input.Limits)
	if err == nil {
		_ = progress.Step(ctx, "manifest.provider", "succeeded", "provider proposal validated and merged")
	}
	return err
}

func executeActionOperation(ctx context.Context, actions *actionsApplication.Service, operation domain.Operation, progress operations.Progress) error {
	var input struct {
		ActionID         string `json:"actionId"`
		ConfirmRisk      bool   `json:"confirmRisk"`
		AllowOutsideRoot bool   `json:"allowOutsideRoot"`
		ActorType        string `json:"actorType"`
		ActorID          string `json:"actorId"`
	}
	if err := json.Unmarshal(operation.Input, &input); err != nil || input.ActionID == "" {
		return errors.New("decode action operation")
	}
	_ = progress.Step(ctx, "action.execute", "running", "action authorized")
	err := actions.Execute(ctx, operation.ID, operation.ProjectID, input.ActionID, input.ActorType, input.ActorID, input.ConfirmRisk, input.AllowOutsideRoot)
	if err == nil {
		_ = progress.Step(ctx, "action.execute", "succeeded", "action completed")
	}
	return err
}

func executePluginOperation(ctx context.Context, plugins *pluginsApplication.Service, operation domain.Operation, progress operations.Progress) error {
	if plugins == nil {
		return errors.New("plugin service is unavailable")
	}
	var input struct {
		PluginID string          `json:"pluginId"`
		Action   string          `json:"action"`
		Input    json.RawMessage `json:"input"`
	}
	if err := json.Unmarshal(operation.Input, &input); err != nil || input.PluginID == "" || input.Action == "" {
		return errors.New("decode plugin operation")
	}
	_ = progress.Step(ctx, "plugin.execute", "running", "reviewed plugin action started")
	result, err := plugins.Operate(ctx, input.PluginID, operation.ProjectID, input.Action, input.Input)
	if err != nil {
		return err
	}
	_ = progress.Step(ctx, "plugin.execute", "succeeded", result.Summary)
	if result.Status == "partially_succeeded" {
		return operations.PartialSuccess(result.Summary)
	}
	if result.Status == "failed" {
		return errors.New("plugin reported action failure")
	}
	return nil
}

func executeWorkspaceOperation(ctx context.Context, workspaces *workspaceApplication.Service, recipes workspaceApplication.RecipeRunner, operation domain.Operation, progress operations.Progress) error {
	if workspaces == nil {
		return errors.New("workspace service is unavailable")
	}
	var input struct {
		Action     string                        `json:"action"`
		Policy     workspaceDomain.FailurePolicy `json:"policy"`
		ProfileID  string                        `json:"profileId"`
		RemoveData bool                          `json:"removeData"`
		RunRecipes bool                          `json:"runRecipes"`
	}
	if err := json.Unmarshal(operation.Input, &input); err != nil || "workspace."+input.Action != operation.Kind {
		return errors.New("decode workspace operation")
	}
	workspaceID := strings.TrimPrefix(operation.ProjectID, "workspace:")
	kind := workspaceDomain.ExecutionKind(input.Action)
	_, err := workspaces.Execute(ctx, workspaceID, workspaceApplication.ExecuteRequest{
		Kind: kind, Policy: input.Policy, ProfileID: input.ProfileID, RemoveData: input.RemoveData,
	}, workspaceProgress{progress: progress})
	var executionErr *workspaceApplication.ExecutionError
	if errors.As(err, &executionErr) && executionErr.Partial() {
		return operations.PartialSuccess(executionErr.Error())
	}
	if err != nil {
		return err
	}
	if kind != workspaceDomain.ExecutionStart || !input.RunRecipes {
		return nil
	}
	_ = progress.Step(ctx, "workspace.recipes", "running", "running opt-in workspace recipes")
	if err := workspaces.ExecuteRecipes(ctx, workspaceID, recipes); err != nil {
		return err
	}
	_ = progress.Step(ctx, "workspace.recipes", "succeeded", "workspace recipes completed")
	return nil
}

func executeRuntimeOperation(
	ctx context.Context,
	service *runtimeApplication.Service,
	health requiredHealthWaiter,
	environments environmentLifecycle,
	operation domain.Operation,
	progress operations.Progress,
) error {
	var input struct {
		Action        string   `json:"action"`
		RemoveVolumes bool     `json:"removeVolumes"`
		Services      []string `json:"services"`
	}
	if err := json.Unmarshal(operation.Input, &input); err != nil {
		return fmt.Errorf("decode runtime operation: %w", err)
	}
	action, err := runtimeDomain.ParseAction(input.Action)
	if err != nil || operation.Kind != "runtime."+input.Action {
		return fmt.Errorf("invalid runtime operation kind %q", operation.Kind)
	}
	plan, err := service.PlanServices(ctx, operation.ProjectID, action, input.RemoveVolumes, input.Services)
	if err != nil {
		return err
	}
	plan.OperationID = operation.ID
	if err := service.Execute(ctx, plan, progress); err != nil {
		return err
	}
	if action == runtimeDomain.ActionStart || action == runtimeDomain.ActionRestart || action == runtimeDomain.ActionRebuild || action == runtimeDomain.ActionUnpause {
		if err := health.WaitRequired(ctx, operation.ProjectID); err != nil {
			return err
		}
		if environments != nil {
			return environments.Started(ctx, operation.ProjectID)
		}
	}
	if action == runtimeDomain.ActionStop || action == runtimeDomain.ActionTeardown || action == runtimeDomain.ActionPause {
		if environments != nil {
			return environments.Stopped(ctx, operation.ProjectID)
		}
	}
	return nil
}

type requiredHealthWaiter interface {
	WaitRequired(context.Context, string) error
}

func validateLoopbackAddress(address string) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("parse daemon address: %w", err)
	}
	if host == "localhost" {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("daemon address must use a loopback host: %s", address)
	}
	return nil
}

func validateDaemonConfig(config Config) error {
	if err := validateLoopbackAddress(config.HTTPAddr); err != nil {
		return err
	}
	if config.RoutingAddr == "" {
		return nil
	}
	if err := validateLoopbackAddress(config.RoutingAddr); err != nil {
		return fmt.Errorf("validate local routing address: %w", err)
	}
	return nil
}
