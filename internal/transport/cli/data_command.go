package cli

import (
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"switchyard.dev/switchyard/internal/bootstrap"
)

func newDataCommand(options *rootOptions) *cobra.Command {
	command := &cobra.Command{Use: "data", Short: "Inspect, back up, and migrate local Switchyard data"}
	command.AddCommand(newDataInspectCommand(options), newDataBackupCommand(options), newDataMigrateCommand(options))
	return command
}

func newDataInspectCommand(options *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use: "inspect", Short: "Preview database compatibility without mutation", Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			status, err := bootstrap.InspectDataMigration(command.Context(), options.dataDir)
			if err != nil {
				return err
			}
			return writeResult(options, "data.inspect", status, func(writer io.Writer) error {
				_, err := fmt.Fprintf(writer, "database=%s schema=%d target=%d migration=%t\n", status.Path, status.CurrentVersion, status.TargetVersion, status.MigrationRequired)
				return err
			})
		},
	}
}

func newDataBackupCommand(options *rootOptions) *cobra.Command {
	output := ""
	command := &cobra.Command{
		Use: "backup", Short: "Create a verified SQLite-consistent backup", Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if output == "" {
				output = filepath.Join(options.dataDir, "switchyard.db.manual-"+time.Now().UTC().Format("20060102T150405Z")+".bak")
			}
			if err := bootstrap.BackupData(command.Context(), options.dataDir, output); err != nil {
				return err
			}
			absolute, _ := filepath.Abs(output)
			value := map[string]any{"path": absolute, "verified": true}
			return writeResult(options, "data.backup", value, func(writer io.Writer) error {
				_, err := fmt.Fprintf(writer, "Verified backup: %s\n", absolute)
				return err
			})
		},
	}
	command.Flags().StringVar(&output, "output", "", "non-existing backup destination")
	return command
}

func newDataMigrateCommand(options *rootOptions) *cobra.Command {
	apply := false
	command := &cobra.Command{
		Use: "migrate", Short: "Preview or apply embedded alpha/beta database migrations", Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			status, err := bootstrap.InspectDataMigration(command.Context(), options.dataDir)
			if err != nil {
				return err
			}
			if apply && status.MigrationRequired {
				status, err = bootstrap.MigrateData(command.Context(), options.dataDir)
				if err != nil {
					return err
				}
			}
			return writeResult(options, "data.migrate", status, func(writer io.Writer) error {
				if status.MigrationRequired {
					_, err = fmt.Fprintf(writer, "Migration required: schema %d -> %d; backup will be %s\n", status.CurrentVersion, status.TargetVersion, status.PreMigrationBackup)
				} else {
					_, err = fmt.Fprintf(writer, "Database is current at schema %d.\n", status.CurrentVersion)
				}
				return err
			})
		},
	}
	command.Flags().BoolVar(&apply, "write", false, "apply migrations after creating and verifying a pre-migration backup")
	return command
}
