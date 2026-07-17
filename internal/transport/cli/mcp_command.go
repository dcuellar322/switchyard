package cli

import (
	"errors"
	"log/slog"

	"github.com/spf13/cobra"

	agents "switchyard.dev/switchyard/internal/agents/application"
	"switchyard.dev/switchyard/internal/foundation/buildinfo"
	"switchyard.dev/switchyard/internal/transport/httpclient"
	"switchyard.dev/switchyard/internal/transport/mcpserver"
)

func newMCPCommand(options *rootOptions) *cobra.Command {
	command := &cobra.Command{Use: "mcp", Short: "Serve Switchyard to MCP-compatible agents"}
	command.AddCommand(newMCPServeCommand(options))
	return command
}

func newMCPServeCommand(options *rootOptions) *cobra.Command {
	var transport, provider, agentID, profileName string
	var projectIDs []string
	command := &cobra.Command{
		Use:   "serve",
		Short: "Run a permission-scoped MCP server over stdio",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if transport != "stdio" {
				return usageError("MCP_TRANSPORT_UNSUPPORTED", "supported MCP transport: stdio")
			}
			if options.json || options.jsonl {
				return usageError("MCP_OUTPUT_CONFLICT", "MCP stdio owns stdout; omit --json and --jsonl")
			}
			client, err := daemonClient(command.Context(), options)
			if err != nil {
				return err
			}
			if !command.Flags().Changed("profile") {
				settings, settingsErr := client.DaemonSettings(command.Context())
				if settingsErr != nil {
					return settingsErr
				}
				profileName = string(settings.Settings.Permissions.DefaultAgentProfile)
			}
			profile, err := agents.ParseProfile(profileName)
			if err != nil {
				return usageError("MCP_PROFILE_INVALID", err.Error())
			}
			scope, err := agents.NewScope(provider, agentID, profile, projectIDs)
			if err != nil {
				return usageError("MCP_SCOPE_INVALID", err.Error())
			}
			agentClient, err := httpclient.NewIPCForAgent(ipcAddress(options), scope.ActorID())
			if err != nil {
				return err
			}
			logger := slog.New(slog.NewJSONHandler(options.stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
			server := mcpserver.New(agentClient, scope, buildinfo.Current().Version, logger)
			if err := server.Run(command.Context()); err != nil && !errors.Is(err, command.Context().Err()) {
				return err
			}
			return nil
		},
	}
	command.Flags().StringVar(&transport, "transport", "stdio", "MCP transport (stdio)")
	command.Flags().StringVar(&provider, "provider", "generic", "bounded provider identifier recorded in audit events")
	command.Flags().StringVar(&agentID, "agent-id", "switchyard", "bounded agent identity recorded in audit events")
	command.Flags().StringVar(&profileName, "profile", string(agents.ProfileObserve), "permission profile (defaults to daemon setting): observe, develop, maintain, or admin")
	command.Flags().StringSliceVar(&projectIDs, "project", nil, "restrict access to a project ID (repeatable)")
	return command
}
