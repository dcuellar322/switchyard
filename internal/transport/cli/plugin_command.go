package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"switchyard.dev/switchyard/internal/foundation/identifier"
)

func newPluginCommand(options *rootOptions) *cobra.Command {
	command := &cobra.Command{Use: "plugin", Aliases: []string{"plugins"}, Short: "Review and operate out-of-process plugins"}
	command.AddCommand(
		newPluginListCommand(options, false), newPluginListCommand(options, true), newPluginTrustCommand(options),
		newPluginEnableCommand(options), newPluginDisableCommand(options), newPluginHealthCommand(options),
		newPluginLogsCommand(options), newPluginInspectCommand(options), newPluginRunCommand(options),
	)
	return command
}

func newPluginListCommand(options *rootOptions, refresh bool) *cobra.Command {
	use, short := "list", "List discovered plugins, trust, health, and grants"
	if refresh {
		use, short = "refresh", "Re-read plugin packages without executing them"
	}
	return &cobra.Command{Use: use, Short: short, Args: cobra.NoArgs, RunE: func(command *cobra.Command, _ []string) error {
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		items, err := client.Plugins(command.Context(), refresh)
		if err != nil {
			return err
		}
		return writeResult(options, "plugin."+use, items, func(writer io.Writer) error {
			if len(items) == 0 {
				_, err := fmt.Fprintln(writer, "No plugins discovered. Install packages under the Switchyard data directory's plugins folder.")
				return err
			}
			rows := make([][]string, 0, len(items))
			for _, item := range items {
				rows = append(rows, []string{item.Id, item.Version, string(item.Trust), fmt.Sprint(item.Enabled), string(item.Health), strings.Join(stringValues(item.GrantedScopes), ",")})
			}
			return humanList(writer, []string{"PLUGIN", "VERSION", "TRUST", "ENABLED", "HEALTH", "GRANTS"}, rows)
		})
	}}
}

func newPluginTrustCommand(options *rootOptions) *cobra.Command {
	fingerprint, yes := "", false
	command := &cobra.Command{Use: "trust <plugin>", Short: "Trust an exact reviewed plugin package identity", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		if !yes {
			return usageError("PLUGIN_CONFIRMATION_REQUIRED", "plugin trust requires --yes after reviewing the fingerprint, capabilities, and scopes")
		}
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		item, err := client.TrustPlugin(command.Context(), args[0], fingerprint)
		if err != nil {
			return err
		}
		return writeResult(options, "plugin.trust", item, func(writer io.Writer) error {
			_, err := fmt.Fprintf(writer, "%s trusted at %s and remains disabled\n", item.Id, item.Fingerprint)
			return err
		})
	}}
	command.Flags().StringVar(&fingerprint, "fingerprint", "", "exact SHA-256 package fingerprint shown by plugin list")
	command.Flags().BoolVar(&yes, "yes", false, "confirm trust after reviewing executable identity and requested access")
	_ = command.MarkFlagRequired("fingerprint")
	return command
}

func newPluginEnableCommand(options *rootOptions) *cobra.Command {
	var scopes []string
	yes := false
	command := &cobra.Command{Use: "enable <plugin>", Short: "Enable a trusted plugin with explicit reviewed scopes", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		if !yes {
			return usageError("PLUGIN_CONFIRMATION_REQUIRED", "plugin enable requires --yes and explicit --scope values")
		}
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		item, err := client.EnablePlugin(command.Context(), args[0], scopes)
		if err != nil {
			return err
		}
		return writeResult(options, "plugin.enable", item, func(writer io.Writer) error {
			_, err := fmt.Fprintf(writer, "%s enabled with %s\n", item.Id, strings.Join(stringValues(item.GrantedScopes), ", "))
			return err
		})
	}}
	command.Flags().StringSliceVar(&scopes, "scope", nil, "requested scope to grant (repeatable)")
	command.Flags().BoolVar(&yes, "yes", false, "confirm reviewed plugin permissions")
	return command
}

func newPluginDisableCommand(options *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "disable <plugin>", Short: "Disable a plugin and revoke all grants", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		item, err := client.DisablePlugin(command.Context(), args[0])
		if err != nil {
			return err
		}
		return writeResult(options, "plugin.disable", item, func(writer io.Writer) error {
			_, err := fmt.Fprintf(writer, "%s disabled; grants revoked\n", item.Id)
			return err
		})
	}}
}

func newPluginHealthCommand(options *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "health <plugin>", Short: "Run one supervised plugin health check", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		item, err := client.CheckPlugin(command.Context(), args[0])
		if err != nil {
			return err
		}
		return writeResult(options, "plugin.health", item, func(writer io.Writer) error {
			_, err := fmt.Fprintf(writer, "%s: %s · %s\n", item.Id, item.Health, pointerValue(item.HealthMessage))
			return err
		})
	}}
}

func newPluginLogsCommand(options *rootOptions) *cobra.Command {
	limit := 100
	command := &cobra.Command{Use: "logs <plugin>", Short: "Read bounded redacted plugin supervision logs", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		items, err := client.PluginLogs(command.Context(), args[0], limit)
		if err != nil {
			return err
		}
		return writeResult(options, "plugin.logs", items, func(writer io.Writer) error {
			for _, item := range items {
				if _, err := fmt.Fprintf(writer, "%s %-7s %s\n", item.CreatedAt.Format("15:04:05"), item.Level, item.Message); err != nil {
					return err
				}
			}
			return nil
		})
	}}
	command.Flags().IntVar(&limit, "limit", limit, "newest log entries to return (1-500)")
	return command
}

func newPluginInspectCommand(options *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "inspect <plugin> <project>", Short: "Inspect a trusted project with granted plugin scopes", Args: cobra.ExactArgs(2), RunE: func(command *cobra.Command, args []string) error {
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		project, err := resolveProject(command.Context(), client, args[1])
		if err != nil {
			return err
		}
		result, err := client.InspectPlugin(command.Context(), args[0], project.Id)
		if err != nil {
			return err
		}
		return writeResult(options, "plugin.inspect", result, func(writer io.Writer) error {
			if _, err := fmt.Fprintln(writer, result.Summary); err != nil {
				return err
			}
			for _, fact := range result.Facts {
				if _, err := fmt.Fprintf(writer, "  %s: %s (%s)\n", fact.Label, fact.Value, fact.Source); err != nil {
					return err
				}
			}
			return nil
		})
	}}
}

func newPluginRunCommand(options *rootOptions) *cobra.Command {
	input, yes := "{}", false
	command := &cobra.Command{Use: "run <plugin> <project> <action>", Short: "Queue one durable typed plugin operation", Args: cobra.ExactArgs(3), RunE: func(command *cobra.Command, args []string) error {
		if !yes {
			return usageError("PLUGIN_CONFIRMATION_REQUIRED", "plugin actions require --yes after reviewing the advertised action and input")
		}
		var values map[string]any
		decoder := json.NewDecoder(strings.NewReader(input))
		decoder.UseNumber()
		if err := decoder.Decode(&values); err != nil || values == nil {
			return usageError("PLUGIN_INPUT_INVALID", "--input must be one JSON object")
		}
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		project, err := resolveProject(command.Context(), client, args[1])
		if err != nil {
			return err
		}
		key, err := identifier.New("cli")
		if err != nil {
			return err
		}
		operation, err := client.CreatePluginOperation(command.Context(), args[0], project.Id, args[2], values, key)
		if err != nil {
			return err
		}
		return writeResult(options, "plugin.run", operation, func(writer io.Writer) error {
			_, err := fmt.Fprintf(writer, "%s queued via %s\noperation: %s\n", args[2], args[0], operation.Id)
			return err
		})
	}}
	command.Flags().StringVar(&input, "input", input, "bounded JSON object for the typed plugin action")
	command.Flags().BoolVar(&yes, "yes", false, "confirm the reviewed plugin action")
	return command
}

func stringValues[T ~string](values []T) []string {
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = string(value)
	}
	return result
}

func pointerValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
