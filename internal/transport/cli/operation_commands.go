package cli

import (
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"switchyard.dev/switchyard/internal/foundation/identifier"
)

func newOperationCommand(options *rootOptions) *cobra.Command {
	command := &cobra.Command{Use: "operation", Aliases: []string{"operations", "op"}, Short: "Inspect and cancel durable operations"}
	command.AddCommand(newOperationListCommand(options), newOperationGetCommand(options), newOperationCancelCommand(options))
	return command
}

func newOperationListCommand(options *rootOptions) *cobra.Command {
	projectSelection := ""
	limit := int64(100)
	command := &cobra.Command{Use: "list", Short: "List recent durable operations", Args: cobra.NoArgs, RunE: func(command *cobra.Command, _ []string) error {
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		projectID := ""
		if projectSelection != "" {
			project, err := resolveProject(command.Context(), client, projectSelection)
			if err != nil {
				return err
			}
			projectID = project.Id
		}
		operations, err := client.Operations(command.Context(), projectID, limit)
		if err != nil {
			return err
		}
		return writeResult(options, "operation.list", operations, func(w io.Writer) error {
			rows := make([][]string, 0, len(operations))
			for _, operation := range operations {
				rows = append(rows, []string{operation.Id, operation.ProjectId, operation.Kind, string(operation.State), operation.RequestedAt.Local().Format(time.RFC3339)})
			}
			return humanList(w, []string{"OPERATION", "PROJECT", "KIND", "STATE", "REQUESTED"}, rows)
		})
	}}
	command.Flags().StringVar(&projectSelection, "project", "", "filter by project ID, slug, or path")
	command.Flags().Int64Var(&limit, "limit", 100, "maximum operations to return (1-500)")
	return command
}

func newOperationGetCommand(options *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "get <operation>", Short: "Read one durable operation", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		operation, err := client.Operation(command.Context(), args[0])
		if err != nil {
			return err
		}
		return writeResult(options, "operation.get", operation, func(w io.Writer) error {
			_, err := fmt.Fprintf(w, "%s\nproject: %s\nkind: %s\nstate: %s\nrequested: %s\n", operation.Id, operation.ProjectId, operation.Kind, operation.State, operation.RequestedAt.Local().Format(time.RFC3339))
			return err
		})
	}}
}

func newOperationCancelCommand(options *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "cancel <operation>", Short: "Request idempotent operation cancellation", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		key, err := identifier.New("cli")
		if err != nil {
			return err
		}
		operation, err := client.CancelOperation(command.Context(), args[0], key)
		if err != nil {
			return err
		}
		return writeResult(options, "operation.cancel", operation, func(w io.Writer) error {
			_, err := fmt.Fprintf(w, "cancellation requested for %s; state=%s\n", operation.Id, operation.State)
			return err
		})
	}}
}
