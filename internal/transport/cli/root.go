// Package cli implements thin human and automation command adapters.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"

	"github.com/spf13/cobra"

	"switchyard.dev/switchyard/internal/bootstrap"
	"switchyard.dev/switchyard/internal/foundation/buildinfo"
	"switchyard.dev/switchyard/internal/foundation/identifier"
	"switchyard.dev/switchyard/internal/platform/localipc"
	"switchyard.dev/switchyard/internal/transport/httpclient"
)

type rootOptions struct {
	address string
	dataDir string
	ipcAddr string
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
	root.PersistentFlags().StringVar(&options.ipcAddr, "ipc-address", "", "privileged local IPC address")
	root.AddCommand(
		newVersionCommand(options),
		newDaemonCommand(options),
		newUICommand(options),
		newDoctorCommand(options),
		newAddCommand(options),
		newManifestCommand(options),
	)
	return root
}

func newAddCommand(options *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "add <repository>",
		Short: "Scan a repository and create a reviewable manifest proposal",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			client, err := ipcClient(options)
			if err != nil {
				return err
			}
			key, err := identifier.New("cli")
			if err != nil {
				return err
			}
			proposal, err := client.CreateManifestProposal(command.Context(), args[0], key)
			if err != nil {
				return err
			}
			return writePrettyJSON(options.stdout, proposal)
		},
	}
}

func newManifestCommand(options *rootOptions) *cobra.Command {
	command := &cobra.Command{Use: "manifest", Short: "Inspect effective project manifests"}
	command.AddCommand(
		newManifestReadCommand(options, "explain <project>", "Print effective fields and provenance", func(ctx context.Context, client *httpclient.Client, id string) (any, error) {
			return client.ExplainManifest(ctx, id)
		}),
		newManifestReadCommand(options, "diff <project>", "Compare accepted and effective manifests", func(ctx context.Context, client *httpclient.Client, id string) (any, error) {
			return client.DiffManifest(ctx, id)
		}),
		newManifestReadCommand(options, "validate <project>", "Validate the effective manifest", func(ctx context.Context, client *httpclient.Client, id string) (any, error) {
			return client.ValidateProjectManifest(ctx, id)
		}),
	)
	return command
}

type manifestRead func(context.Context, *httpclient.Client, string) (any, error)

func newManifestReadCommand(options *rootOptions, use, short string, read manifestRead) *cobra.Command {
	return &cobra.Command{
		Use: use, Short: short, Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			client, err := ipcClient(options)
			if err != nil {
				return err
			}
			projectID, err := resolveProject(command.Context(), client, args[0])
			if err != nil {
				return err
			}
			value, err := read(command.Context(), client, projectID)
			if err != nil {
				return err
			}
			return writePrettyJSON(options.stdout, value)
		},
	}
}

func resolveProject(ctx context.Context, client *httpclient.Client, value string) (string, error) {
	projects, err := client.Projects(ctx)
	if err != nil {
		return "", err
	}
	for _, project := range projects {
		if project.Id == value || project.Slug == value {
			return project.Id, nil
		}
	}
	return "", fmt.Errorf("project %q not found", value)
}

func writePrettyJSON(writer io.Writer, value any) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
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
				DataDir: options.dataDir, HTTPAddr: options.address, IPCAddr: options.ipcAddr, Logger: logger,
			})
		},
	}
}

func newUICommand(options *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "ui",
		Short: "Print the local browser UI address",
		RunE: func(command *cobra.Command, _ []string) error {
			client, err := ipcClient(options)
			if err != nil {
				return err
			}
			bootstrap, err := client.BrowserBootstrap(command.Context())
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(
				options.stdout,
				"http://%s/?bootstrap=%s\n",
				options.address,
				url.QueryEscape(bootstrap.Token),
			)
			return err
		},
	}
}

func newDoctorCommand(options *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check daemon and durable storage health",
		RunE: func(command *cobra.Command, _ []string) error {
			client, err := ipcClient(options)
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

func ipcClient(options *rootOptions) (*httpclient.Client, error) {
	address := options.ipcAddr
	if address == "" {
		address = localipc.DefaultAddress(options.dataDir)
	}
	return httpclient.NewIPC(address)
}

// Main executes the process CLI and returns a semantic process status.
func Main(ctx context.Context) int {
	if err := Execute(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
