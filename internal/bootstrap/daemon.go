// Package bootstrap composes process-level Switchyard adapters and use cases.
package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"switchyard.dev/switchyard/internal/foundation/buildinfo"
	"switchyard.dev/switchyard/internal/platform/sqlite"
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

	system := application.NewQuery(database, buildinfo.Current(), time.Now())
	handler := httpapi.New(system, eventtransport.NewEvents(), web.Handler(), config.Logger)
	listener, err := net.Listen("tcp", config.HTTPAddr)
	if err != nil {
		return fmt.Errorf("listen on loopback API: %w", err)
	}
	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}
	serveErrors := make(chan error, 1)
	go func() {
		serveErrors <- server.Serve(listener)
	}()
	config.Logger.Info(
		"switchyard daemon ready",
		"component", "bootstrap",
		"address", listener.Addr().String(),
		"data_dir", config.DataDir,
	)

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown HTTP server: %w", err)
		}
		serveErr := <-serveErrors
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			return fmt.Errorf("serve HTTP: %w", serveErr)
		}
		return nil
	case err := <-serveErrors:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve HTTP: %w", err)
		}
		return nil
	}
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
