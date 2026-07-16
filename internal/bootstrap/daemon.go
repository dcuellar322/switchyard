// Package bootstrap composes process-level Switchyard adapters and use cases.
package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"time"

	catalog "switchyard.dev/switchyard/internal/catalog/application"
	discoveryAdapters "switchyard.dev/switchyard/internal/discovery/adapters"
	"switchyard.dev/switchyard/internal/foundation/buildinfo"
	operations "switchyard.dev/switchyard/internal/operations/application"
	"switchyard.dev/switchyard/internal/operations/domain"
	"switchyard.dev/switchyard/internal/platform/sqlite"
	session "switchyard.dev/switchyard/internal/session/application"
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
	operationRepository := sqlite.NewOperationRepository(database)
	coordinator := operations.NewCoordinator(ctx, operationRepository, journal, operations.ExecutorFunc(
		func(_ context.Context, operation domain.Operation, _ operations.Progress) error {
			return fmt.Errorf("no executor registered for operation kind %q", operation.Kind)
		},
	))
	if err := coordinator.Recover(ctx); err != nil {
		return err
	}
	sessions := session.NewManager()
	system := application.NewQuery(database, buildinfo.Current(), time.Now())
	dependencies := httpapi.Dependencies{
		System: system, Operations: coordinator, Sessions: sessions, Catalog: catalogService,
		Events: eventtransport.NewEvents(journal), Web: web.Handler(), Logger: config.Logger,
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
	return servers.run(ctx, coordinator.Shutdown)
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
