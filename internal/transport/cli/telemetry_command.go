package cli

import (
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"switchyard.dev/switchyard/internal/transport/contract/generated"
)

func newTelemetryCommand(options *rootOptions) *cobra.Command {
	command := &cobra.Command{
		Use:   "telemetry",
		Short: "Manage explicit anonymous usage-counter consent",
	}
	command.AddCommand(
		newTelemetryStatusCommand(options),
		newTelemetryEnableCommand(options),
		newTelemetryDisableCommand(options),
		newTelemetrySendCommand(options),
	)
	return command
}

func newTelemetryStatusCommand(options *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show consent, destination, and the complete pending payload",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			client, err := daemonClient(command.Context(), options)
			if err != nil {
				return err
			}
			status, err := client.TelemetryStatus(command.Context())
			if err != nil {
				return err
			}
			return writeTelemetryStatus(options, "telemetry.status", status)
		},
	}
}

func newTelemetryEnableCommand(options *rootOptions) *cobra.Command {
	endpoint := ""
	yes := false
	command := &cobra.Command{
		Use:   "enable",
		Short: "Opt in to bounded anonymous counter delivery",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if !yes {
				return usageError("TELEMETRY_CONFIRMATION_REQUIRED", "telemetry enable requires --yes after reviewing the HTTPS endpoint and status payload")
			}
			client, err := daemonClient(command.Context(), options)
			if err != nil {
				return err
			}
			status, err := client.UpdateTelemetry(command.Context(), generated.TelemetrySettingsRequest{
				Enabled: true, Endpoint: &endpoint, ConfirmRisk: true,
			})
			if err != nil {
				return err
			}
			return writeTelemetryStatus(options, "telemetry.enable", status)
		},
	}
	command.Flags().StringVar(&endpoint, "endpoint", "", "explicit HTTPS collection endpoint")
	command.Flags().BoolVar(&yes, "yes", false, "confirm anonymous counter delivery")
	_ = command.MarkFlagRequired("endpoint")
	return command
}

func newTelemetryDisableCommand(options *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "disable",
		Short: "Opt out and permanently clear pending anonymous counters",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			client, err := daemonClient(command.Context(), options)
			if err != nil {
				return err
			}
			status, err := client.UpdateTelemetry(command.Context(), generated.TelemetrySettingsRequest{Enabled: false})
			if err != nil {
				return err
			}
			return writeTelemetryStatus(options, "telemetry.disable", status)
		},
	}
}

func newTelemetrySendCommand(options *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "send",
		Short: "Send the currently displayed anonymous counters now",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			client, err := daemonClient(command.Context(), options)
			if err != nil {
				return err
			}
			status, err := client.SendTelemetry(command.Context())
			if err != nil {
				return err
			}
			return writeTelemetryStatus(options, "telemetry.send", status)
		},
	}
}

func writeTelemetryStatus(options *rootOptions, kind string, status generated.TelemetryStatus) error {
	return writeResult(options, kind, status, func(writer io.Writer) error {
		endpoint := "disabled"
		if status.Settings.Endpoint != nil {
			endpoint = *status.Settings.Endpoint
		}
		if _, err := fmt.Fprintf(writer, "enabled: %t\nendpoint: %s\n", status.Settings.Enabled, endpoint); err != nil {
			return err
		}
		if status.Preview != nil {
			if _, err := fmt.Fprintf(writer, "schema: %s\ninstallation: %s\nbuild: %s\nplatform: %s/%s\ngenerated: %s\n",
				status.Preview.SchemaVersion, status.Preview.InstallationId, status.Preview.Version,
				status.Preview.Os, status.Preview.Architecture, status.Preview.GeneratedAt.Format(time.RFC3339)); err != nil {
				return err
			}
		}
		for _, counter := range status.Counters {
			if _, err := fmt.Fprintf(writer, "%s: %d\n", counter.Name, counter.Value); err != nil {
				return err
			}
		}
		return nil
	})
}
