// Package cli implements thin human and automation command adapters.
package cli

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"switchyard.dev/switchyard/internal/bootstrap"
	"switchyard.dev/switchyard/internal/foundation/buildinfo"
)

type rootOptions struct {
	address           string
	dataDir           string
	ipcAddr           string
	json              bool
	jsonl             bool
	nonInteractive    bool
	noColor           bool
	stdout            io.Writer
	stderr            io.Writer
	logRingCapacity   int
	logSegmentBytes   int64
	logRetentionAge   time.Duration
	logRetentionBytes int64
	redactionPatterns []string
}

// Execute runs the CLI with explicit process dependencies.
func Execute(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	config, err := bootstrap.DefaultConfig()
	if err != nil {
		return err
	}
	options := &rootOptions{address: config.HTTPAddr, dataDir: config.DataDir, stdout: stdout, stderr: stderr,
		logRingCapacity: config.LogRingCapacity, logSegmentBytes: config.LogSegmentBytes,
		logRetentionAge: config.LogRetentionAge, logRetentionBytes: config.LogRetentionBytes,
	}
	command := newRootCommand(options)
	command.SetArgs(args)
	command.SetOut(stdout)
	command.SetErr(stderr)
	return command.ExecuteContext(ctx)
}

func newRootCommand(options *rootOptions) *cobra.Command {
	root := &cobra.Command{
		Use: "switchyard", Short: "Local project-oriented development command center",
		SilenceUsage: true, SilenceErrors: true,
		PersistentPreRunE: func(*cobra.Command, []string) error {
			if options.json && options.jsonl {
				return usageError("OUTPUT_MODE_CONFLICT", "--json and --jsonl cannot be used together")
			}
			return nil
		},
	}
	root.PersistentFlags().StringVar(&options.address, "address", options.address, "loopback daemon address")
	root.PersistentFlags().StringVar(&options.dataDir, "data-dir", options.dataDir, "local Switchyard data directory")
	root.PersistentFlags().StringVar(&options.ipcAddr, "ipc-address", "", "privileged local IPC address")
	root.PersistentFlags().BoolVar(&options.json, "json", false, "emit a stable JSON envelope")
	root.PersistentFlags().BoolVar(&options.jsonl, "jsonl", false, "emit one stable JSON envelope per item")
	root.PersistentFlags().BoolVar(&options.nonInteractive, "non-interactive", false, "disable interactive prompts")
	root.PersistentFlags().BoolVar(&options.noColor, "no-color", false, "disable ANSI color output")
	root.AddCommand(
		newVersionCommand(options), newDaemonCommand(options), newUICommand(options), newDoctorCommand(options),
		newAddCommand(options), newListAliasCommand(options), newProjectCommand(options), newOperationCommand(options),
		newManifestCommand(options), newOpenCommand(options), newCompletionCommand(root), newSchemaCommand(options),
		newStatusCommand(options), newPlanCommand(options), newLogsCommand(options), newMetricsCommand(options),
		newLifecycleCommand(options, "start"), newLifecycleCommand(options, "stop"), newLifecycleCommand(options, "restart"),
		newLifecycleCommand(options, "pause"), newLifecycleCommand(options, "unpause"), newLifecycleCommand(options, "rebuild"),
		newLifecycleCommand(options, "teardown"),
	)
	return root
}

func newVersionCommand(options *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "version", Short: "Print Switchyard build identity", Args: cobra.NoArgs, RunE: func(*cobra.Command, []string) error {
		info := buildinfo.Current()
		return writeResult(options, "version", info, func(w io.Writer) error {
			_, err := fmt.Fprintf(w, "Switchyard %s (%s)\n", info.Version, info.Commit)
			return err
		})
	}}
}

func newDaemonCommand(options *rootOptions) *cobra.Command {
	command := &cobra.Command{Use: "daemon", Short: "Run the local Switchyard control plane", Args: cobra.NoArgs, RunE: func(command *cobra.Command, _ []string) error {
		logger := slog.New(slog.NewJSONHandler(options.stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
		return bootstrap.RunDaemon(command.Context(), bootstrap.Config{
			DataDir: options.dataDir, HTTPAddr: options.address, IPCAddr: options.ipcAddr, Logger: logger,
			LogRingCapacity: options.logRingCapacity, LogSegmentBytes: options.logSegmentBytes,
			LogRetentionAge: options.logRetentionAge, LogRetentionBytes: options.logRetentionBytes,
			RedactionPatterns: options.redactionPatterns,
		})
	}}
	command.Flags().IntVar(&options.logRingCapacity, "log-ring-entries", options.logRingCapacity, "redacted in-memory log entries per service")
	command.Flags().Int64Var(&options.logSegmentBytes, "log-segment-bytes", options.logSegmentBytes, "maximum bytes per NDJSON log segment")
	command.Flags().DurationVar(&options.logRetentionAge, "log-retention-age", options.logRetentionAge, "maximum retained log age")
	command.Flags().Int64Var(&options.logRetentionBytes, "log-retention-bytes", options.logRetentionBytes, "maximum retained log bytes")
	command.Flags().StringSliceVar(&options.redactionPatterns, "redact-pattern", nil, "additional regular expression to redact (repeatable)")
	return command
}

func newUICommand(options *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "ui", Short: "Print the local browser UI address", Args: cobra.NoArgs, RunE: func(command *cobra.Command, _ []string) error {
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		credential, err := client.BrowserBootstrap(command.Context())
		if err != nil {
			return err
		}
		address := "http://" + options.address + "/?bootstrap=" + url.QueryEscape(credential.Token)
		return writeResult(options, "ui", map[string]any{"url": address, "expiresAt": credential.ExpiresAt}, func(w io.Writer) error { _, err := fmt.Fprintln(w, address); return err })
	}}
}

func newDoctorCommand(options *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "doctor", Short: "Check daemon and durable storage health", Args: cobra.NoArgs, RunE: func(command *cobra.Command, _ []string) error {
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		info, err := client.System(command.Context())
		if err != nil {
			return err
		}
		return writeResult(options, "doctor", info, func(w io.Writer) error {
			_, err := fmt.Fprintf(w, "daemon=%s version=%s api=%s schema=%d\n", info.Status, info.Version, info.ApiVersion, info.DatabaseSchemaVersion)
			return err
		})
	}}
}

func newCompletionCommand(root *cobra.Command) *cobra.Command {
	command := &cobra.Command{Use: "completion [bash|zsh|fish|powershell]", Short: "Generate a shell completion script", Args: cobra.ExactArgs(1), DisableFlagsInUseLine: true}
	command.ValidArgs = []string{"bash", "zsh", "fish", "powershell"}
	command.RunE = func(_ *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return root.GenBashCompletion(root.OutOrStdout())
		case "zsh":
			return root.GenZshCompletion(root.OutOrStdout())
		case "fish":
			return root.GenFishCompletion(root.OutOrStdout(), true)
		case "powershell":
			return root.GenPowerShellCompletion(root.OutOrStdout())
		default:
			return usageError("SHELL_UNSUPPORTED", "supported shells: "+strings.Join(command.ValidArgs, ", "))
		}
	}
	return command
}

// Main executes the process CLI and returns a semantic process status.
func Main(ctx context.Context) int {
	err := Execute(ctx, os.Args[1:], os.Stdout, os.Stderr)
	if err == nil {
		return 0
	}
	cliErr := classifyError(err)
	if machineRequested(os.Args[1:]) {
		_ = writeMachineError(os.Stderr, cliErr)
	} else {
		_, _ = fmt.Fprintln(os.Stderr, cliErr.Message)
	}
	return cliErr.ExitCode
}
