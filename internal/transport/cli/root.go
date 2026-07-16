// Package cli implements thin human and automation command adapters.
package cli

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"switchyard.dev/switchyard/internal/bootstrap"
	"switchyard.dev/switchyard/internal/foundation/buildinfo"
	"switchyard.dev/switchyard/internal/transport/httpclient"
)

type rootOptions struct {
	address string
	dataDir string
	stdout  io.Writer
	stderr  io.Writer
}

// Execute runs the CLI with explicit process dependencies.
func Execute(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	config, err := bootstrap.DefaultConfig()
	if err != nil {
		return err
	}
	options := &rootOptions{
		address: config.HTTPAddr,
		dataDir: config.DataDir,
		stdout:  stdout,
		stderr:  stderr,
	}
	command := newRootCommand(options)
	command.SetArgs(args)
	command.SetOut(stdout)
	command.SetErr(stderr)
	return command.ExecuteContext(ctx)
}

func newRootCommand(options *rootOptions) *cobra.Command {
	root := &cobra.Command{
		Use:           "switchyard",
		Short:         "Local project-oriented development command center",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().StringVar(&options.address, "address", options.address, "loopback daemon address")
	root.PersistentFlags().StringVar(&options.dataDir, "data-dir", options.dataDir, "local Switchyard data directory")
	root.AddCommand(
		newVersionCommand(options),
		newDaemonCommand(options),
		newUICommand(options),
		newDoctorCommand(options),
	)
	return root
}

func newVersionCommand(options *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print Switchyard build identity",
		RunE: func(*cobra.Command, []string) error {
			info := buildinfo.Current()
			_, err := fmt.Fprintf(options.stdout, "Switchyard %s (%s)\n", info.Version, info.Commit)
			return err
		},
	}
}

func newDaemonCommand(options *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "daemon",
		Short: "Run the local Switchyard control plane",
		RunE: func(command *cobra.Command, _ []string) error {
			logger := slog.New(slog.NewJSONHandler(options.stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
			return bootstrap.RunDaemon(command.Context(), bootstrap.Config{
				DataDir: options.dataDir, HTTPAddr: options.address, Logger: logger,
			})
		},
	}
}

func newUICommand(options *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "ui",
		Short: "Print the local browser UI address",
		RunE: func(*cobra.Command, []string) error {
			_, err := fmt.Fprintf(options.stdout, "http://%s/\n", options.address)
			return err
		},
	}
}

func newDoctorCommand(options *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check daemon and durable storage health",
		RunE: func(command *cobra.Command, _ []string) error {
			client, err := httpclient.New(options.address)
			if err != nil {
				return err
			}
			info, err := client.System(command.Context())
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(
				options.stdout,
				"daemon=%s version=%s api=%s schema=%d\n",
				info.Status,
				info.Version,
				info.ApiVersion,
				info.DatabaseSchemaVersion,
			)
			return err
		},
	}
}

// Main executes the process CLI and returns a semantic process status.
func Main(ctx context.Context) int {
	if err := Execute(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
