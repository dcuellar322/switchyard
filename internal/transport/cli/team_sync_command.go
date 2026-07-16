package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	teamAdapters "switchyard.dev/switchyard/internal/team/adapters"
	teamDomain "switchyard.dev/switchyard/internal/team/domain"
	"switchyard.dev/switchyard/internal/transport/contract/generated"
)

func newTeamSyncCommand(options *rootOptions) *cobra.Command {
	command := &cobra.Command{
		Use:   "sync",
		Short: "Move signed configuration through encrypted age files",
	}
	command.AddCommand(
		newTeamSyncKeyGenerateCommand(options),
		newTeamSyncExportCommand(options),
		newTeamSyncPreviewCommand(options),
		newTeamSyncImportCommand(options),
	)
	return command
}

func newTeamSyncKeyGenerateCommand(options *rootOptions) *cobra.Command {
	output := ""
	command := &cobra.Command{
		Use:   "key-generate",
		Short: "Generate an owner-only age identity for encrypted sync",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			identity, recipient, err := teamAdapters.GenerateSyncIdentity()
			if err != nil {
				return err
			}
			if err := writeExclusiveFile(output, []byte(identity+"\n"), 0o600); err != nil {
				return err
			}
			result := map[string]string{"identityPath": output, "recipient": recipient}
			return writeResult(options, "team.sync.key.generate", result, func(writer io.Writer) error {
				_, err := fmt.Fprintf(writer, "Identity: %s\nRecipient: %s\n", output, recipient)
				return err
			})
		},
	}
	command.Flags().StringVar(&output, "output", "", "new owner-only age identity file")
	_ = command.MarkFlagRequired("output")
	return command
}

func newTeamSyncExportCommand(options *rootOptions) *cobra.Command {
	var recipients []string
	output := ""
	command := &cobra.Command{
		Use:   "export",
		Short: "Export signed configuration to an encrypted age file",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			client, err := daemonClient(command.Context(), options)
			if err != nil {
				return err
			}
			generatedDocument, err := client.ExportTeamSync(command.Context())
			if err != nil {
				return err
			}
			var document teamDomain.SyncDocument
			if err := convertTeamJSON(generatedDocument, &document); err != nil {
				return err
			}
			encrypted, err := teamAdapters.EncryptSync(document, recipients)
			if err != nil {
				return err
			}
			if err := writeExclusiveFile(output, encrypted, 0o600); err != nil {
				return err
			}
			result := map[string]any{
				"path": output, "publisherCount": len(document.Publishers), "bundleCount": len(document.Bundles),
			}
			return writeResult(options, "team.sync.export", result, func(writer io.Writer) error {
				_, err := fmt.Fprintf(writer, "%s\npublishers: %d\nbundles: %d\n", output, len(document.Publishers), len(document.Bundles))
				return err
			})
		},
	}
	command.Flags().StringArrayVar(&recipients, "recipient", nil, "age X25519 recipient (repeatable)")
	command.Flags().StringVar(&output, "output", "", "new encrypted sync file")
	_ = command.MarkFlagRequired("recipient")
	_ = command.MarkFlagRequired("output")
	return command
}

func newTeamSyncPreviewCommand(options *rootOptions) *cobra.Command {
	identity := ""
	command := &cobra.Command{
		Use:   "preview <encrypted-file>",
		Short: "Decrypt locally and verify every signature without changing state",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			document, err := decryptTeamSync(args[0], identity)
			if err != nil {
				return err
			}
			client, err := daemonClient(command.Context(), options)
			if err != nil {
				return err
			}
			preview, err := client.PreviewTeamSync(command.Context(), document)
			if err != nil {
				return err
			}
			return writeTeamSyncPreview(options, "team.sync.preview", preview)
		},
	}
	command.Flags().StringVar(&identity, "identity", "", "owner-only age identity file")
	_ = command.MarkFlagRequired("identity")
	return command
}

func newTeamSyncImportCommand(options *rootOptions) *cobra.Command {
	identity := ""
	yes := false
	command := &cobra.Command{
		Use:   "import <encrypted-file>",
		Short: "Import a previewed encrypted configuration document",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if !yes {
				return usageError("TEAM_CONFIRMATION_REQUIRED", "sync import requires --yes after previewing publisher trust and bundle IDs")
			}
			document, err := decryptTeamSync(args[0], identity)
			if err != nil {
				return err
			}
			client, err := daemonClient(command.Context(), options)
			if err != nil {
				return err
			}
			preview, err := client.ImportTeamSync(command.Context(), document, true)
			if err != nil {
				return err
			}
			return writeTeamSyncPreview(options, "team.sync.import", preview)
		},
	}
	command.Flags().StringVar(&identity, "identity", "", "owner-only age identity file")
	command.Flags().BoolVar(&yes, "yes", false, "confirm publisher trust and signed bundle installation")
	_ = command.MarkFlagRequired("identity")
	return command
}

func decryptTeamSync(path, identityPath string) (generated.TeamSyncDocument, error) {
	encrypted, err := readBoundedFile(path, 20<<20)
	if err != nil {
		return generated.TeamSyncDocument{}, err
	}
	identity, err := readBoundedFile(identityPath, 8<<10)
	if err != nil {
		return generated.TeamSyncDocument{}, err
	}
	document, err := teamAdapters.DecryptSync(encrypted, string(identity))
	if err != nil {
		return generated.TeamSyncDocument{}, err
	}
	var result generated.TeamSyncDocument
	if err := convertTeamJSON(document, &result); err != nil {
		return generated.TeamSyncDocument{}, err
	}
	return result, nil
}

func writeTeamSyncPreview(options *rootOptions, kind string, preview generated.TeamSyncPreview) error {
	return writeResult(options, kind, preview, func(writer io.Writer) error {
		if _, err := fmt.Fprintf(writer, "publishers: %d\nbundles: %d\n", preview.PublisherCount, preview.BundleCount); err != nil {
			return err
		}
		for _, bundleID := range preview.BundleIds {
			if _, err := fmt.Fprintf(writer, "bundle: %s\n", bundleID); err != nil {
				return err
			}
		}
		for _, warning := range preview.Warnings {
			if _, err := fmt.Fprintf(writer, "warning: %s\n", warning); err != nil {
				return err
			}
		}
		return nil
	})
}

func convertTeamJSON(source, target any) error {
	encoded, err := json.Marshal(source)
	if err != nil {
		return err
	}
	return json.Unmarshal(encoded, target)
}
