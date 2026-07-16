package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"switchyard.dev/switchyard/internal/foundation/identifier"
	"switchyard.dev/switchyard/internal/transport/contract/generated"
)

func newWorkspaceCommand(options *rootOptions) *cobra.Command {
	command := &cobra.Command{Use: "workspace", Aliases: []string{"workspaces"}, Short: "Coordinate durable multi-project workspaces"}
	command.AddCommand(
		newWorkspaceListCommand(options), newWorkspaceGetCommand(options), newWorkspaceCreateCommand(options),
		newWorkspaceUpdateCommand(options), newWorkspaceDeleteCommand(options),
		newWorkspaceStartCommand(options), newWorkspaceStopCommand(options),
	)
	return command
}

func newWorkspaceListCommand(options *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "list", Short: "List durable workspace graphs", Args: cobra.NoArgs, RunE: func(command *cobra.Command, _ []string) error {
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		workspaces, err := client.Workspaces(command.Context())
		if err != nil {
			return err
		}
		return writeResult(options, "workspace.list", workspaces, func(w io.Writer) error {
			rows := make([][]string, 0, len(workspaces))
			for _, workspace := range workspaces {
				lastState := "idle"
				if workspace.LastRun != nil {
					lastState = string(workspace.LastRun.State)
				}
				rows = append(rows, []string{workspace.Id, workspace.Name, strconv.Itoa(len(workspace.Members)), string(workspace.Policy), lastState})
			}
			return humanList(w, []string{"WORKSPACE", "NAME", "MEMBERS", "POLICY", "STATE"}, rows)
		})
	}}
}

func newWorkspaceGetCommand(options *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "get <workspace-id>", Short: "Read a workspace graph and latest execution", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		workspace, err := client.Workspace(command.Context(), args[0])
		if err != nil {
			return err
		}
		return writeResult(options, "workspace.get", workspace, func(w io.Writer) error {
			_, err := fmt.Fprintf(w, "%s\nid: %s\nmembers: %d\npolicy: %s\nrevision: %d\n", workspace.Name, workspace.Id, len(workspace.Members), workspace.Policy, workspace.Revision)
			return err
		})
	}}
}

func newWorkspaceCreateCommand(options *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "create <definition.json>", Short: "Create a workspace from a reviewed JSON definition", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		var definition generated.WorkspaceDefinition
		if err := readJSONFile(args[0], &definition); err != nil {
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
		workspace, err := client.CreateWorkspace(command.Context(), definition, key)
		if err != nil {
			return err
		}
		return writeResult(options, "workspace.create", workspace, func(w io.Writer) error {
			_, err := fmt.Fprintf(w, "created %s\nworkspace: %s\n", workspace.Name, workspace.Id)
			return err
		})
	}}
}

func newWorkspaceUpdateCommand(options *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "update <workspace-id> <update.json>", Short: "Revision-check and replace a workspace graph", Args: cobra.ExactArgs(2), RunE: func(command *cobra.Command, args []string) error {
		var update generated.WorkspaceUpdate
		if err := readJSONFile(args[1], &update); err != nil {
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
		workspace, err := client.UpdateWorkspace(command.Context(), args[0], update, key)
		if err != nil {
			return err
		}
		return writeResult(options, "workspace.update", workspace, func(w io.Writer) error {
			_, err := fmt.Fprintf(w, "updated %s to revision %d\n", workspace.Id, workspace.Revision)
			return err
		})
	}}
}

func newWorkspaceDeleteCommand(options *rootOptions) *cobra.Command {
	yes := false
	command := &cobra.Command{Use: "delete <workspace-id>", Short: "Delete workspace metadata without touching projects or data", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		if !yes {
			return usageError("CONFIRMATION_REQUIRED", "workspace delete requires --yes; project runtimes and data are not changed")
		}
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		key, err := identifier.New("cli")
		if err != nil {
			return err
		}
		if err := client.DeleteWorkspace(command.Context(), args[0], key); err != nil {
			return err
		}
		return writeResult(options, "workspace.delete", map[string]any{"id": args[0], "deleted": true, "runtimeDataChanged": false}, func(w io.Writer) error {
			_, err := fmt.Fprintln(w, "workspace metadata deleted; project runtimes and data were not changed")
			return err
		})
	}}
	command.Flags().BoolVar(&yes, "yes", false, "confirm metadata deletion")
	return command
}

func newWorkspaceStartCommand(options *rootOptions) *cobra.Command {
	policy, profile := "", ""
	runRecipes := false
	command := &cobra.Command{Use: "start <workspace-id>", Short: "Queue dependency-ordered workspace start", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		request := generated.WorkspaceOperationRequest{Action: generated.WorkspaceStart}
		if policy != "" {
			value := generated.WorkspaceFailurePolicy(policy)
			request.Policy = &value
		}
		if profile != "" {
			request.ProfileId = &profile
		}
		request.RunRecipes = &runRecipes
		return queueWorkspaceOperation(command, options, args[0], request, "workspace.start")
	}}
	command.Flags().StringVar(&policy, "policy", "", "override failure policy: rollback or continue")
	command.Flags().StringVar(&profile, "profile", "", "workspace profile ID")
	command.Flags().BoolVar(&runRecipes, "run-recipes", false, "run reviewed launch recipes after a successful start")
	return command
}

func newWorkspaceStopCommand(options *rootOptions) *cobra.Command {
	profile := ""
	removeData, yes := false, false
	command := &cobra.Command{Use: "stop <workspace-id>", Short: "Queue dependency-safe bulk stop that preserves data by default", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		if removeData && !yes {
			return usageError("CONFIRMATION_REQUIRED", "--remove-data requires --yes after reviewing the destructive scope")
		}
		request := generated.WorkspaceOperationRequest{Action: generated.WorkspaceStop, RemoveData: &removeData, ConfirmDataRemoval: &yes}
		if profile != "" {
			request.ProfileId = &profile
		}
		return queueWorkspaceOperation(command, options, args[0], request, "workspace.stop")
	}}
	command.Flags().StringVar(&profile, "profile", "", "workspace profile ID")
	command.Flags().BoolVar(&removeData, "remove-data", false, "tear down runtime volumes instead of preserving data")
	command.Flags().BoolVar(&yes, "yes", false, "confirm destructive data removal")
	return command
}

func queueWorkspaceOperation(command *cobra.Command, options *rootOptions, workspaceID string, request generated.WorkspaceOperationRequest, kind string) error {
	client, err := daemonClient(command.Context(), options)
	if err != nil {
		return err
	}
	key, err := identifier.New("cli")
	if err != nil {
		return err
	}
	operation, err := client.CreateWorkspaceOperation(command.Context(), workspaceID, request, key)
	if err != nil {
		return err
	}
	return writeResult(options, kind, operation, func(w io.Writer) error {
		_, err := fmt.Fprintf(w, "%s queued\noperation: %s\nstate: %s\n", kind, operation.Id, operation.State)
		return err
	})
}

func readJSONFile(path string, target any) error {
	document, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read JSON document: %w", err)
	}
	if err := json.Unmarshal(document, target); err != nil {
		return usageError("DOCUMENT_INVALID", "JSON document does not match the generated API contract")
	}
	return nil
}
