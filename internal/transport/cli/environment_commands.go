package cli

import (
	"fmt"
	"io"
	"strconv"

	"github.com/spf13/cobra"

	"switchyard.dev/switchyard/internal/foundation/identifier"
)

func newEnvironmentCommand(options *rootOptions) *cobra.Command {
	command := &cobra.Command{Use: "environment", Aliases: []string{"environments", "env"}, Short: "Register and inspect isolated Git worktree environments"}
	command.AddCommand(newEnvironmentListCommand(options), newEnvironmentRegisterCommand(options), newEnvironmentGetCommand(options), newEnvironmentRenameCommand(options), newRoutesCommand(options))
	return command
}

func newEnvironmentListCommand(options *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "list <project>", Short: "List registered worktree environments", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		project, err := resolveProject(command.Context(), client, args[0])
		if err != nil {
			return err
		}
		environments, err := client.ProjectEnvironments(command.Context(), project.Id)
		if err != nil {
			return err
		}
		return writeResult(options, "environment.list", environments, func(w io.Writer) error {
			rows := make([][]string, 0, len(environments))
			for _, environment := range environments {
				rows = append(rows, []string{environment.Id, environment.Name, string(environment.State), environment.Hostname, strconv.Itoa(len(environment.Allocation.PortLeases)), environment.Path})
			}
			return humanList(w, []string{"ENVIRONMENT", "NAME", "STATE", "HOSTNAME", "PORTS", "PATH"}, rows)
		})
	}}
}

func newEnvironmentRegisterCommand(options *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "register <project>", Short: "Reconcile Git worktrees and exact runtime leases", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		project, err := resolveProject(command.Context(), client, args[0])
		if err != nil {
			return err
		}
		key, err := identifier.New("cli")
		if err != nil {
			return err
		}
		registration, err := client.RegisterProjectEnvironments(command.Context(), project.Id, key)
		if err != nil {
			return err
		}
		return writeResult(options, "environment.register", registration, func(w io.Writer) error {
			_, err := fmt.Fprintf(w, "registered %d environments; removed %d stale registrations\n", len(registration.Environments), len(registration.RemovedIds))
			return err
		})
	}}
}

func newEnvironmentGetCommand(options *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "get <environment-id>", Short: "Read one worktree environment", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		environment, err := client.Environment(command.Context(), args[0])
		if err != nil {
			return err
		}
		return writeResult(options, "environment.get", environment, func(w io.Writer) error {
			_, err := fmt.Fprintf(w, "%s\nid: %s\nstate: %s\nhostname: %s\ncompose: %s\npath: %s\n", environment.Name, environment.Id, environment.State, environment.Hostname, environment.Allocation.ComposeProjectName, environment.Path)
			return err
		})
	}}
}

func newEnvironmentRenameCommand(options *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "hostname <environment-id> <name.localhost>", Short: "Change an environment's friendly local hostname", Args: cobra.ExactArgs(2), RunE: func(command *cobra.Command, args []string) error {
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		key, err := identifier.New("cli")
		if err != nil {
			return err
		}
		environment, err := client.UpdateEnvironmentHostname(command.Context(), args[0], args[1], key)
		if err != nil {
			return err
		}
		return writeResult(options, "environment.hostname", environment, func(w io.Writer) error {
			_, err := fmt.Fprintln(w, environment.Hostname)
			return err
		})
	}}
}

func newRoutesCommand(options *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "routes", Short: "List friendly localhost route states", Args: cobra.NoArgs, RunE: func(command *cobra.Command, _ []string) error {
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		routes, err := client.LocalRoutes(command.Context())
		if err != nil {
			return err
		}
		return writeResult(options, "environment.routes", routes, func(w io.Writer) error {
			rows := make([][]string, 0, len(routes))
			for _, route := range routes {
				rows = append(rows, []string{route.Hostname, string(route.Status), valueOr(route.EnvironmentId, "-"), valueOr(route.Target, "-"), valueOr(route.Reason, "-")})
			}
			return humanList(w, []string{"HOSTNAME", "STATE", "ENVIRONMENT", "TARGET", "REASON"}, rows)
		})
	}}
}
