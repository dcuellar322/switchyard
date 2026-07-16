package cli

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"switchyard.dev/switchyard/internal/foundation/identifier"
)

func newPortsCommand(options *rootOptions) *cobra.Command {
	command := &cobra.Command{Use: "ports", Short: "Inspect declarations, reservations, listeners, and conflicts"}
	command.AddCommand(newPortsListCommand(options), newPortsNextCommand(options))
	return command
}

func newPortsListCommand(options *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "list", Aliases: []string{"status"}, Short: "Read the current host port registry", Args: cobra.NoArgs, RunE: func(command *cobra.Command, _ []string) error {
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		registry, err := client.PortRegistry(command.Context())
		if err != nil {
			return err
		}
		return writeResult(options, "ports.list", registry, func(w io.Writer) error {
			rows := make([][]string, 0, len(registry.Facts))
			conflicted := make(map[int]bool, len(registry.Conflicts))
			for _, conflict := range registry.Conflicts {
				conflicted[conflict.Port] = true
			}
			for _, fact := range registry.Facts {
				project := valueOr(fact.ProjectName, "unknown")
				state := string(fact.Kind)
				if conflicted[fact.Port] {
					state = "conflict"
				}
				rows = append(rows, []string{strconv.Itoa(fact.Port), string(fact.Protocol), project, valueOr(fact.ServiceId, "-"), string(fact.Source), state})
			}
			if len(rows) == 0 {
				_, err := fmt.Fprintln(w, "No declared, reserved, or bound ports were observed.")
				return err
			}
			return humanList(w, []string{"PORT", "PROTO", "PROJECT", "SERVICE", "SOURCE", "STATE"}, rows)
		})
	}}
}

func newPortsNextCommand(options *rootOptions) *cobra.Command {
	rangeValue, protocol, projectValue := "15000-19999", "tcp", ""
	var excluded []int
	command := &cobra.Command{Use: "next", Short: "Suggest a free port in a preferred range", Args: cobra.NoArgs, RunE: func(command *cobra.Command, _ []string) error {
		start, end, err := parsePortRange(rangeValue)
		if err != nil {
			return err
		}
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		projectID := ""
		if projectValue != "" {
			project, resolveErr := resolveProject(command.Context(), client, projectValue)
			if resolveErr != nil {
				return resolveErr
			}
			projectID = project.Id
		}
		key, err := identifier.New("cli")
		if err != nil {
			return err
		}
		suggestion, err := client.SuggestPort(command.Context(), start, end, protocol, projectID, excluded, key)
		if err != nil {
			return err
		}
		return writeResult(options, "ports.next", suggestion, func(w io.Writer) error {
			_, err := fmt.Fprintf(w, "%d/%s is free in %d-%d\n", suggestion.Port, suggestion.Protocol, suggestion.RangeStart, suggestion.RangeEnd)
			return err
		})
	}}
	command.Flags().StringVar(&rangeValue, "range", rangeValue, "inclusive preferred range, for example 15000-19999")
	command.Flags().StringVar(&protocol, "protocol", protocol, "tcp or udp")
	command.Flags().StringVar(&projectValue, "project", "", "ignore this project's own protected declarations")
	command.Flags().IntSliceVar(&excluded, "exclude", nil, "port to exclude (repeatable or comma-separated)")
	return command
}

func newGitCommand(options *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "git <project>", Short: "Read current Git branch, changes, remotes, and worktrees", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		project, err := resolveProject(command.Context(), client, args[0])
		if err != nil {
			return err
		}
		state, err := client.GitState(command.Context(), project.Id)
		if err != nil {
			return err
		}
		return writeResult(options, "git.status", state, func(w io.Writer) error {
			if !state.Repository {
				_, err := fmt.Fprintln(w, "No Git repository was found at the trusted project root.")
				return err
			}
			branch := valueOr(state.Branch, "detached")
			changes := state.Changes.Staged + state.Changes.Modified + state.Changes.Untracked + state.Changes.Conflicted
			_, err := fmt.Fprintf(w, "%s · %d changes · ahead %d · behind %d · %d stashes\n", branch, changes, state.Ahead, state.Behind, state.Stashes)
			return err
		})
	}}
}

func newActionCommand(options *rootOptions) *cobra.Command {
	command := &cobra.Command{Use: "action", Aliases: []string{"actions"}, Short: "List and run trusted project actions"}
	command.AddCommand(newActionListCommand(options), newActionRunCommand(options))
	return command
}

func newActionListCommand(options *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "list <project>", Short: "List trusted project quick actions", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		project, err := resolveProject(command.Context(), client, args[0])
		if err != nil {
			return err
		}
		actions, err := client.ProjectActions(command.Context(), project.Id)
		if err != nil {
			return err
		}
		return writeResult(options, "action.list", actions.Actions, func(w io.Writer) error {
			rows := make([][]string, 0, len(actions.Actions))
			for _, action := range actions.Actions {
				workingDirectory := action.WorkingDirectory
				if workingDirectory == "" {
					workingDirectory = "."
				}
				rows = append(rows, []string{action.Id, action.Name, action.Type, string(action.Risk), workingDirectory})
			}
			return humanList(w, []string{"ACTION", "NAME", "TYPE", "RISK", "CWD"}, rows)
		})
	}}
}

func newActionRunCommand(options *rootOptions) *cobra.Command {
	yes, allowOutside := false, false
	command := &cobra.Command{Use: "run <project> <action>", Short: "Queue one durable audited project action", Args: cobra.ExactArgs(2), RunE: func(command *cobra.Command, args []string) error {
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
		operation, err := client.CreateActionOperation(command.Context(), project.Id, args[1], yes, allowOutside, key)
		if err != nil {
			return err
		}
		return writeResult(options, "action.run", operation, func(w io.Writer) error {
			_, err := fmt.Fprintf(w, "%s queued for %s\noperation: %s\nstate: %s\n", args[1], project.Slug, operation.Id, operation.State)
			return err
		})
	}}
	command.Flags().BoolVar(&yes, "yes", false, "confirm a destructive action after review")
	command.Flags().BoolVar(&allowOutside, "allow-outside-root", false, "explicitly permit this action's reviewed working directory outside the project root")
	return command
}

func parsePortRange(value string) (int, int, error) {
	startValue, endValue, found := strings.Cut(value, "-")
	if !found {
		return 0, 0, usageError("PORT_RANGE_INVALID", "--range must use start-end")
	}
	start, startErr := strconv.Atoi(startValue)
	end, endErr := strconv.Atoi(endValue)
	if startErr != nil || endErr != nil || start < 1 || end > 65535 || start > end {
		return 0, 0, usageError("PORT_RANGE_INVALID", "--range must be between 1 and 65535 with start no greater than end")
	}
	return start, end, nil
}

func valueOr(value *string, fallback string) string {
	if value == nil || *value == "" {
		return fallback
	}
	return *value
}
