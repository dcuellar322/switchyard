package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"switchyard.dev/switchyard/internal/foundation/identifier"
	"switchyard.dev/switchyard/internal/transport/contract/generated"
)

func newSettingsCommand(options *rootOptions) *cobra.Command {
	command := &cobra.Command{Use: "settings", Short: "Inspect and update durable local preferences"}
	command.AddCommand(newSettingsShowCommand(options), newSettingsExportCommand(options), newSettingsApplyCommand(options))
	return command
}

func newSettingsShowCommand(options *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "show", Short: "Show current revisioned settings", Args: cobra.NoArgs, RunE: func(command *cobra.Command, _ []string) error {
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		status, err := client.DaemonSettings(command.Context())
		if err != nil {
			return err
		}
		return writeResult(options, "settings.show", status, func(writer io.Writer) error { return writeSettingsSummary(writer, status) })
	}}
}

func newSettingsExportCommand(options *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "export <new-json-file>", Short: "Export the editable settings document to a new private file", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		status, err := client.DaemonSettings(command.Context())
		if err != nil {
			return err
		}
		encoded, err := json.MarshalIndent(status.Settings, "", "  ")
		if err != nil {
			return err
		}
		if err := writeExclusiveFile(args[0], append(encoded, '\n'), 0o600); err != nil {
			return err
		}
		result := map[string]any{"path": args[0], "revision": status.Settings.Revision}
		return writeResult(options, "settings.export", result, func(writer io.Writer) error {
			_, err := fmt.Fprintf(writer, "exported settings revision %d to %s\n", status.Settings.Revision, args[0])
			return err
		})
	}}
}

func newSettingsApplyCommand(options *rootOptions) *cobra.Command {
	yes := false
	command := &cobra.Command{Use: "apply <json-file>", Short: "Validate and atomically apply one exported settings revision", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		if !yes {
			return usageError("CONFIRMATION_REQUIRED", "settings apply requires --yes after reviewing the complete document")
		}
		settings, err := readSettingsDocument(args[0])
		if err != nil {
			return err
		}
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		key, err := identifier.New("cli")
		if err != nil {
			return err
		}
		status, err := client.UpdateDaemonSettings(command.Context(), settings, key)
		if err != nil {
			return err
		}
		return writeResult(options, "settings.apply", status, func(writer io.Writer) error { return writeSettingsSummary(writer, status) })
	}}
	command.Flags().BoolVar(&yes, "yes", false, "confirm the reviewed full-document replacement")
	return command
}

func readSettingsDocument(path string) (generated.DaemonSettings, error) {
	file, err := os.Open(path)
	if err != nil {
		return generated.DaemonSettings{}, err
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil {
		return generated.DaemonSettings{}, err
	}
	if info.Size() > 64<<10 {
		return generated.DaemonSettings{}, errors.New("settings document exceeds 64 KiB")
	}
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var settings generated.DaemonSettings
	if err := decoder.Decode(&settings); err != nil {
		return generated.DaemonSettings{}, fmt.Errorf("decode settings document: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return generated.DaemonSettings{}, errors.New("settings document contains multiple JSON values or exceeds 64 KiB")
	}
	return settings, nil
}

func writeSettingsSummary(writer io.Writer, status generated.DaemonSettingsStatus) error {
	restart := "none"
	if len(status.PendingRestart) > 0 {
		values := make([]string, len(status.PendingRestart))
		for index, value := range status.PendingRestart {
			values[index] = string(value)
		}
		restart = strings.Join(values, ",")
	}
	_, err := fmt.Fprintf(writer, "revision: %d\nproject roots: %d\npreferred ports: %d-%d\ndefault agent profile: %s\npending restart: %s\n",
		status.Settings.Revision, len(status.Settings.ProjectRoots), status.Settings.Ports.RangeStart,
		status.Settings.Ports.RangeEnd, status.Settings.Permissions.DefaultAgentProfile, restart)
	return err
}
