package cli

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"switchyard.dev/switchyard/internal/bootstrap"
	supportApplication "switchyard.dev/switchyard/internal/support/application"
	"switchyard.dev/switchyard/internal/support/domain"
)

func newDoctorCommand(options *rootOptions) *cobra.Command {
	var bundle, previewOnly bool
	var output string
	command := &cobra.Command{
		Use: "doctor", Short: "Check daemon health or create a redacted support bundle", Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if (previewOnly || output != "") && !bundle {
				return usageError("SUPPORT_BUNDLE_FLAG_REQUIRED", "--preview and --output require --bundle")
			}
			client, err := daemonClient(command.Context(), options)
			if err != nil {
				return err
			}
			info, err := client.System(command.Context())
			if err != nil {
				return err
			}
			if !bundle {
				return writeResult(options, "doctor", info, func(w io.Writer) error {
					_, err := fmt.Fprintf(w, "daemon=%s version=%s api=%s schema=%d\n", info.Status, info.Version, info.ApiVersion, info.DatabaseSchemaVersion)
					return err
				})
			}
			service, err := newSupportService(options)
			if err != nil {
				return err
			}
			configuration, err := bootstrap.ReadSupportConfiguration(options.dataDir)
			if err != nil {
				return err
			}
			host, hostErr := client.Host(command.Context())
			docker := domain.AdapterAvailability{ID: "docker-engine", Detail: "host observation unavailable"}
			if hostErr == nil {
				docker.Available = host.Docker.Connected
				docker.Detail = "Docker Engine unavailable"
				if docker.Available {
					docker.Detail = "Docker Engine connected"
				}
			}
			bundlePreview, err := service.Preview(command.Context(), supportApplication.PreviewInput{
				System: domain.SystemIdentity{
					Status: string(info.Status), Version: info.Version, Commit: info.Commit,
					APIVersion: info.ApiVersion, DatabaseSchema: int(info.DatabaseSchemaVersion),
				},
				Configuration: configuration, AdditionalAdapters: []domain.AdapterAvailability{docker},
			})
			if err != nil {
				return err
			}
			if previewOnly {
				return writeResult(options, "doctor.bundle.preview", bundlePreview, func(w io.Writer) error {
					return writeBundlePreview(w, bundlePreview)
				})
			}
			if strings.TrimSpace(output) == "" {
				output = defaultBundlePath(bundlePreview.GeneratedAt)
			}
			receipt, err := bootstrap.WriteSupportBundle(output, bundlePreview)
			if err != nil {
				return err
			}
			return writeResult(options, "doctor.bundle", receipt, func(w io.Writer) error {
				_, err := fmt.Fprintf(w, "Support bundle written to %s\nsha256=%s size=%d bytes\n", receipt.Path, receipt.SHA256, receipt.SizeBytes)
				return err
			})
		},
	}
	command.Flags().BoolVar(&bundle, "bundle", false, "preview or write a private redacted support ZIP")
	command.Flags().BoolVar(&previewOnly, "preview", false, "show exact included and excluded support evidence without writing")
	command.Flags().StringVar(&output, "output", "", "support ZIP output path (must not already exist)")
	return command
}

func newDebugCommand(options *rootOptions) *cobra.Command {
	command := &cobra.Command{Use: "debug", Short: "Inspect redacted Switchyard control-plane diagnostics"}
	command.AddCommand(newDebugLogsCommand(options))
	return command
}

func newDebugLogsCommand(options *rootOptions) *cobra.Command {
	limit := 200
	level := "DEBUG"
	command := &cobra.Command{
		Use: "logs", Short: "Print bounded redacted internal daemon logs", Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			service, err := newSupportService(options)
			if err != nil {
				return err
			}
			entries, err := service.Logs(command.Context(), supportApplication.LogQuery{Limit: limit, MinimumLevel: level})
			if err != nil {
				return err
			}
			return writeResult(options, "debug.logs", entries, func(w io.Writer) error {
				if len(entries) == 0 {
					_, err := fmt.Fprintln(w, "No redacted internal daemon logs are available yet.")
					return err
				}
				rows := make([][]string, 0, len(entries))
				for _, entry := range entries {
					detail := entry.Message
					if entry.Error != "" {
						detail += ": " + entry.Error
					}
					rows = append(rows, []string{entry.Timestamp.Local().Format(time.RFC3339), entry.Level, entry.Component, detail})
				}
				return humanList(w, []string{"TIME", "LEVEL", "COMPONENT", "MESSAGE"}, rows)
			})
		},
	}
	command.Flags().IntVar(&limit, "limit", limit, "maximum entries (1-2000)")
	command.Flags().StringVar(&level, "level", level, "minimum level: debug, info, warn, or error")
	return command
}

func newSupportService(options *rootOptions) (*supportApplication.Service, error) {
	return bootstrap.NewSupportService(options.dataDir, options.redactionPatterns)
}

func writeBundlePreview(writer io.Writer, preview domain.Preview) error {
	if _, err := fmt.Fprintf(writer, "Support bundle preview (%s)\n\nIncluded:\n", preview.SchemaVersion); err != nil {
		return err
	}
	for _, item := range preview.Included {
		if _, err := fmt.Fprintf(writer, "  + %s\n", item); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(writer, "\nExcluded:"); err != nil {
		return err
	}
	for _, item := range preview.Excluded {
		if _, err := fmt.Fprintf(writer, "  - %s\n", item); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(writer, "\nRecent internal warnings/errors: %d\nNo file was written. Use --bundle without --preview after review.\n", len(preview.InternalErrors))
	return err
}

func defaultBundlePath(at time.Time) string {
	return filepath.Join(".", fmt.Sprintf("switchyard-support-%s.zip", at.UTC().Format("20060102T150405Z")))
}
