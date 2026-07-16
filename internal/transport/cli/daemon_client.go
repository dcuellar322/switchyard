package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"switchyard.dev/switchyard/internal/platform/localipc"
	"switchyard.dev/switchyard/internal/transport/httpclient"
)

func ipcClient(options *rootOptions) (*httpclient.Client, error) {
	address := options.ipcAddr
	if address == "" {
		address = localipc.DefaultAddress(options.dataDir)
	}
	return httpclient.NewIPC(address)
}

func daemonClient(ctx context.Context, options *rootOptions) (*httpclient.Client, error) {
	client, err := ipcClient(options)
	if err != nil {
		return nil, err
	}
	probeErr := probeDaemon(ctx, client)
	if probeErr == nil {
		return client, nil
	}
	if err := startDaemon(options); err != nil {
		return nil, &Error{Code: "DAEMON_START_FAILED", Message: err.Error(), ExitCode: exitUnavailable, Cause: err}
	}
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-deadline.C:
			return nil, &Error{Code: "DAEMON_UNAVAILABLE", Message: "Switchyard daemon did not become ready within 5 seconds", ExitCode: exitUnavailable, Cause: probeErr}
		case <-ticker.C:
			if err := probeDaemon(ctx, client); err == nil {
				return client, nil
			}
		}
	}
}

func probeDaemon(ctx context.Context, client *httpclient.Client) error {
	probeCtx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()
	_, err := client.System(probeCtx)
	return err
}

func startDaemon(options *rootOptions) error {
	if err := os.MkdirAll(options.dataDir, 0o700); err != nil {
		return fmt.Errorf("create daemon data directory: %w", err)
	}
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate switchyard executable: %w", err)
	}
	args := []string{"daemon", "--data-dir", options.dataDir, "--address", options.address}
	if options.ipcAddr != "" {
		args = append(args, "--ipc-address", options.ipcAddr)
	}
	command := exec.Command(executable, args...)
	configureDetached(command)
	logPath := filepath.Join(options.dataDir, "daemon.log")
	if info, statErr := os.Lstat(logPath); statErr == nil && !info.Mode().IsRegular() {
		return fmt.Errorf("daemon log path is not a regular file: %s", logPath)
	} else if statErr != nil && !os.IsNotExist(statErr) {
		return fmt.Errorf("inspect daemon log: %w", statErr)
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open daemon log: %w", err)
	}
	if err := logFile.Chmod(0o600); err != nil {
		_ = logFile.Close()
		return fmt.Errorf("restrict daemon log: %w", err)
	}
	command.Stdout, command.Stderr = logFile, logFile
	if err := command.Start(); err != nil {
		_ = logFile.Close()
		return fmt.Errorf("start daemon: %w", err)
	}
	if err := command.Process.Release(); err != nil {
		_ = logFile.Close()
		return fmt.Errorf("release daemon process: %w", err)
	}
	return logFile.Close()
}
