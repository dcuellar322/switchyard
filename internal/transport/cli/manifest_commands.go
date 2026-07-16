package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/spf13/cobra"

	manifestApplication "switchyard.dev/switchyard/internal/manifest/application"
	"switchyard.dev/switchyard/internal/transport/httpclient"
)

func newManifestCommand(options *rootOptions) *cobra.Command {
	command := &cobra.Command{Use: "manifest", Short: "Inspect effective project manifests"}
	command.AddCommand(
		newManifestMigrateCommand(options),
		newManifestReadCommand(options, "explain <project>", "Print effective fields and provenance", "manifest.explain", func(ctx context.Context, client *httpclient.Client, id string) (any, error) {
			return client.ExplainManifest(ctx, id)
		}),
		newManifestReadCommand(options, "diff <project>", "Compare accepted and effective manifests", "manifest.diff", func(ctx context.Context, client *httpclient.Client, id string) (any, error) {
			return client.DiffManifest(ctx, id)
		}),
		newManifestReadCommand(options, "validate <project>", "Validate the effective manifest", "manifest.validate", func(ctx context.Context, client *httpclient.Client, id string) (any, error) {
			return client.ValidateProjectManifest(ctx, id)
		}),
	)
	return command
}

func newManifestMigrateCommand(options *rootOptions) *cobra.Command {
	apply := false
	command := &cobra.Command{
		Use:   "migrate <path>",
		Short: "Preview or apply the alpha/beta manifest migration to v1",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			result, err := manifestApplication.MigrateFile(args[0], apply)
			if err != nil {
				return err
			}
			return writeResult(options, "manifest.migrate", result, func(writer io.Writer) error {
				switch {
				case !result.Changed:
					_, err = fmt.Fprintln(writer, "Manifest already uses the stable v1 schema.")
				case result.Applied:
					_, err = fmt.Fprintf(writer, "Migrated %s; backup: %s\n", result.Path, result.BackupPath)
				default:
					_, err = fmt.Fprint(writer, result.Preview)
				}
				return err
			})
		},
	}
	command.Flags().BoolVar(&apply, "write", false, "apply migration after creating a non-overwriting backup")
	return command
}

type manifestRead func(context.Context, *httpclient.Client, string) (any, error)

func newManifestReadCommand(options *rootOptions, use, short, outputCommand string, read manifestRead) *cobra.Command {
	return &cobra.Command{Use: use, Short: short, Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		project, err := resolveProject(command.Context(), client, args[0])
		if err != nil {
			return err
		}
		value, err := read(command.Context(), client, project.Id)
		if err != nil {
			return err
		}
		return writeResult(options, outputCommand, value, func(w io.Writer) error {
			if outputCommand == "manifest.explain" {
				return writeManifestExplanation(w, value)
			}
			return writePrettyJSON(w, value)
		})
	}}
}

func writeManifestExplanation(writer io.Writer, value any) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	var document struct {
		Provenance map[string]string `json:"provenance"`
	}
	if err := json.Unmarshal(encoded, &document); err != nil {
		return err
	}
	keys := make([]string, 0, len(document.Provenance))
	for key := range document.Provenance {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	rows := make([][]string, 0, len(keys))
	for _, key := range keys {
		rows = append(rows, []string{key, document.Provenance[key]})
	}
	if len(rows) == 0 {
		_, err := fmt.Fprintln(writer, "No effective manifest provenance is available.")
		return err
	}
	return humanList(writer, []string{"FIELD", "SOURCE"}, rows)
}
