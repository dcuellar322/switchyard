// Package bootstrap composes process-level Switchyard adapters and use cases.
package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	actionsAdapters "switchyard.dev/switchyard/internal/actions/adapters"
	actionsApplication "switchyard.dev/switchyard/internal/actions/application"
	catalog "switchyard.dev/switchyard/internal/catalog/application"
	discoveryAdapters "switchyard.dev/switchyard/internal/discovery/adapters"
	"switchyard.dev/switchyard/internal/foundation/buildinfo"
	observabilityAdapters "switchyard.dev/switchyard/internal/observability/adapters"
	observabilityApplication "switchyard.dev/switchyard/internal/observability/application"
	operations "switchyard.dev/switchyard/internal/operations/application"
	"switchyard.dev/switchyard/internal/operations/domain"
	hostPlatform "switchyard.dev/switchyard/internal/platform/host"
	"switchyard.dev/switchyard/internal/platform/sqlite"
	portsAdapters "switchyard.dev/switchyard/internal/ports/adapters"
	portsApplication "switchyard.dev/switchyard/internal/ports/application"
	runtimeApplication "switchyard.dev/switchyard/internal/runtime/application"
	"switchyard.dev/switchyard/internal/runtime/compose"
	runtimeDomain "switchyard.dev/switchyard/internal/runtime/domain"
	processRuntime "switchyard.dev/switchyard/internal/runtime/process"
	session "switchyard.dev/switchyard/internal/session/application"
	sourcecontrolAdapters "switchyard.dev/switchyard/internal/sourcecontrol/adapters"
	sourcecontrolApplication "switchyard.dev/switchyard/internal/sourcecontrol/application"
	"switchyard.dev/switchyard/internal/system/application"
	"switchyard.dev/switchyard/internal/transport/httpapi"
	eventtransport "switchyard.dev/switchyard/internal/transport/websocket"
	"switchyard.dev/switchyard/web"
)

// RunDaemon starts the migrated local control plane and blocks until shutdown.
func RunDaemon(ctx context.Context, config Config) error {
	if config.Logger == nil {
		config.Logger = slog.Default()
	}
	if err := validateLoopbackAddress(config.HTTPAddr); err != nil {
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
	redactor, err := observabilityAdapters.NewRedactor(config.RedactionPatterns)
	if err != nil {
		return fmt.Errorf("compile log redaction patterns: %w", err)
	}
	runtimeSource := runtimeApplication.NewCatalogSource(catalogService)
	runtimeService := runtimeApplication.NewService(
		runtimeSource,
		compose.NewDriver(),
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
	portService := portsApplication.NewService(
		portsApplication.NewCatalogSource(catalogService),
		portsAdapters.NewRuntimeBindings(catalogService, runtimeService),
		portsAdapters.NewOSListeners(),
		sqlite.NewPortReservationRepository(database),
	)
	gitService := sourcecontrolApplication.NewService(sourcecontrolApplication.NewCatalogSource(catalogService), sourcecontrolAdapters.NewGit())
	actionAudits := sqlite.NewActionAuditRepository(database)
	if err := actionAudits.Recover(ctx, time.Now().UTC()); err != nil {
		return err
	}
	actionService := actionsApplication.NewService(
		actionsApplication.NewCatalogSource(catalogService),
		actionsAdapters.NewRunner(actionsAdapters.NewLauncher()),
		actionAudits,
	)
	operationRepository := sqlite.NewOperationRepository(database)
	coordinator := operations.NewCoordinator(ctx, operationRepository, journal, operations.ExecutorFunc(
		func(operationCtx context.Context, operation domain.Operation, progress operations.Progress) error {
			return executeOperation(operationCtx, runtimeService, healthService, actionService, operation, progress)
		},
	))
	if err := coordinator.Recover(ctx); err != nil {
		return err
	}
	reconcileSink := runtimeReconciliationSink{runtime: runtimeService, journal: journal}
	var background sync.WaitGroup
	background.Add(3)
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
	sessions := session.NewManager()
	system := application.NewQuery(database, buildinfo.Current(), time.Now())
	host := application.NewHostQuery(hostPlatform.NewObserver())
	dependencies := httpapi.Dependencies{
		System: system, Host: host, Operations: coordinator, Sessions: sessions, Catalog: catalogService, Runtime: runtimeService,
		Health: healthService, LogService: logService, Events: eventtransport.NewEvents(journal), Logs: eventtransport.NewLogs(logService),
		Ports: portService, Git: gitService, Actions: actionService,
		Web: web.Handler(), Logger: config.Logger,
	}
	servers, err := newLocalServers(config, dependencies)
	if err != nil {
		return err
	}
	config.Logger.Info(
		"switchyard daemon ready",
		"component", "bootstrap",
		"address", servers.browserAddress(),
		"ipc_address", servers.ipcAddress,
		"data_dir", config.DataDir,
	)
	runErr := servers.run(ctx, coordinator.Shutdown)
	background.Wait()
	return runErr
}

func executeOperation(
	ctx context.Context,
	runtimeService *runtimeApplication.Service,
	health requiredHealthWaiter,
	actions *actionsApplication.Service,
	operation domain.Operation,
	progress operations.Progress,
) error {
	switch operation.Kind {
	case "action.run":
		var input struct {
			ActionID         string `json:"actionId"`
			ConfirmRisk      bool   `json:"confirmRisk"`
			AllowOutsideRoot bool   `json:"allowOutsideRoot"`
			ActorType        string `json:"actorType"`
			ActorID          string `json:"actorId"`
		}
		if err := json.Unmarshal(operation.Input, &input); err != nil || input.ActionID == "" {
			return fmt.Errorf("decode action operation")
		}
		_ = progress.Step(ctx, "action.execute", "running", "action authorized")
		err := actions.Execute(ctx, operation.ID, operation.ProjectID, input.ActionID, input.ActorType, input.ActorID, input.ConfirmRisk, input.AllowOutsideRoot)
		if err == nil {
			_ = progress.Step(ctx, "action.execute", "succeeded", "action completed")
		}
		return err
	default:
		return executeRuntimeOperation(ctx, runtimeService, health, operation, progress)
	}
}

func executeRuntimeOperation(
	ctx context.Context,
	service *runtimeApplication.Service,
	health requiredHealthWaiter,
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
	if action == runtimeDomain.ActionStart || action == runtimeDomain.ActionRestart || action == runtimeDomain.ActionRebuild {
		return health.WaitRequired(ctx, operation.ProjectID)
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
