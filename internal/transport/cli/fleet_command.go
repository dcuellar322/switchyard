package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"switchyard.dev/switchyard/internal/foundation/identifier"
	"switchyard.dev/switchyard/internal/transport/contract/generated"
	"switchyard.dev/switchyard/internal/transport/httpclient"
)

func newMachineCommand(options *rootOptions) *cobra.Command {
	command := &cobra.Command{Use: "machine", Aliases: []string{"machines", "fleet"}, Short: "Manage optional authenticated remote Switchyard machines"}
	command.AddCommand(
		newMachineListCommand(options), newMachineAddCommand(options), newMachineShowCommand(options),
		newMachineProbeCommand(options), newMachineAccessCommand(options), newMachineDisableCommand(options),
		newMachineRemoveCommand(options), newMachineSnapshotCommand(options), newMachineRunCommand(options),
	)
	return command
}

func newMachineListCommand(options *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "list", Short: "List remote machines, identity, state, and grants", Args: cobra.NoArgs, RunE: func(command *cobra.Command, _ []string) error {
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		items, err := client.Machines(command.Context())
		if err != nil {
			return err
		}
		return writeResult(options, "machine.list", items, func(writer io.Writer) error {
			if len(items) == 0 {
				_, err := fmt.Fprintln(writer, "No remote machines configured. Local-only Switchyard remains fully available.")
				return err
			}
			rows := make([][]string, 0, len(items))
			for _, item := range items {
				rows = append(rows, []string{item.Id, item.Name, string(item.State), fmt.Sprint(item.Enabled), strings.Join(stringValues(item.GrantedCapabilities), ","), item.Endpoint})
			}
			return humanList(writer, []string{"MACHINE", "NAME", "STATE", "ENABLED", "GRANTS", "ENDPOINT"}, rows)
		})
	}}
}

func newMachineAddCommand(options *rootOptions) *cobra.Command {
	fingerprint, caPath, certificatePath, keyPath := "", "", "", ""
	grants := []string{"inventory.read"}
	yes := false
	command := &cobra.Command{Use: "add <name> <https-endpoint>", Short: "Register and probe a certificate-pinned remote agent", Args: cobra.ExactArgs(2), RunE: func(command *cobra.Command, args []string) error {
		if !yes {
			return usageError("REMOTE_CONFIRMATION_REQUIRED", "machine registration requires --yes after reviewing the endpoint, certificate pin, and grants")
		}
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		machine, err := client.CreateMachine(command.Context(), generated.MachineRegistrationRequest{
			Name: args[0], Endpoint: args[1], CertificateFingerprint: fingerprint,
			CaCertificatePath: caPath, ClientCertificatePath: certificatePath, ClientKeyPath: keyPath,
			GrantedCapabilities: generatedCapabilities(grants), ConfirmRisk: yes,
		})
		if err != nil {
			return err
		}
		return writeMachineResult(options, "machine.add", machine)
	}}
	command.Flags().StringVar(&fingerprint, "server-fingerprint", "", "reviewed SHA-256 server certificate fingerprint")
	command.Flags().StringVar(&caPath, "ca", "", "absolute peer CA certificate path")
	command.Flags().StringVar(&certificatePath, "client-certificate", "", "absolute client certificate path")
	command.Flags().StringVar(&keyPath, "client-key", "", "absolute client private key path")
	command.Flags().StringSliceVar(&grants, "grant", grants, "capability to grant (repeatable)")
	command.Flags().BoolVar(&yes, "yes", false, "confirm the reviewed peer identity and access")
	for _, flag := range []string{"server-fingerprint", "ca", "client-certificate", "client-key"} {
		_ = command.MarkFlagRequired(flag)
	}
	return command
}

func newMachineShowCommand(options *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "show <machine>", Short: "Show one redacted machine registration", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		machine, err := resolveMachine(command.Context(), client, args[0])
		if err != nil {
			return err
		}
		return writeMachineResult(options, "machine.show", machine)
	}}
}

func newMachineProbeCommand(options *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "probe <machine>", Short: "Refresh authenticated peer identity and state", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		machine, err := resolveMachine(command.Context(), client, args[0])
		if err != nil {
			return err
		}
		machine, err = client.ProbeMachine(command.Context(), machine.Id)
		if err != nil {
			return err
		}
		return writeMachineResult(options, "machine.probe", machine)
	}}
}

func newMachineAccessCommand(options *rootOptions) *cobra.Command {
	var grants []string
	yes := false
	command := &cobra.Command{Use: "access <machine>", Aliases: []string{"grant"}, Short: "Replace the complete enabled capability grant set", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		if !yes || len(grants) == 0 {
			return usageError("REMOTE_CONFIRMATION_REQUIRED", "machine access requires --yes and one or more explicit --grant values")
		}
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		machine, err := resolveMachine(command.Context(), client, args[0])
		if err != nil {
			return err
		}
		machine, err = client.UpdateMachineAccess(command.Context(), machine.Id, generated.MachineAccessRequest{
			Enabled: true, GrantedCapabilities: generatedCapabilities(grants), ConfirmRisk: yes,
		})
		if err != nil {
			return err
		}
		return writeMachineResult(options, "machine.access", machine)
	}}
	command.Flags().StringSliceVar(&grants, "grant", nil, "complete capability grant set (repeatable)")
	command.Flags().BoolVar(&yes, "yes", false, "confirm the reviewed remote access change")
	return command
}

func newMachineDisableCommand(options *rootOptions) *cobra.Command {
	yes := false
	command := &cobra.Command{Use: "disable <machine>", Short: "Disable a remote machine and revoke all local grants", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		if !yes {
			return usageError("REMOTE_CONFIRMATION_REQUIRED", "machine disable requires --yes")
		}
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		machine, err := resolveMachine(command.Context(), client, args[0])
		if err != nil {
			return err
		}
		machine, err = client.UpdateMachineAccess(command.Context(), machine.Id, generated.MachineAccessRequest{Enabled: false, GrantedCapabilities: []generated.FleetCapability{}, ConfirmRisk: yes})
		if err != nil {
			return err
		}
		return writeMachineResult(options, "machine.disable", machine)
	}}
	command.Flags().BoolVar(&yes, "yes", false, "confirm disabling the remote machine")
	return command
}

func newMachineRemoveCommand(options *rootOptions) *cobra.Command {
	yes := false
	command := &cobra.Command{Use: "remove <machine>", Short: "Remove the local registration without changing the peer", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		if !yes {
			return usageError("REMOTE_CONFIRMATION_REQUIRED", "machine removal requires --yes")
		}
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		machine, err := resolveMachine(command.Context(), client, args[0])
		if err != nil {
			return err
		}
		if err := client.DeleteMachine(command.Context(), machine.Id, yes); err != nil {
			return err
		}
		return writeResult(options, "machine.remove", map[string]string{"id": machine.Id}, func(writer io.Writer) error {
			_, err := fmt.Fprintf(writer, "%s removed locally; the remote machine was not changed\n", machine.Id)
			return err
		})
	}}
	command.Flags().BoolVar(&yes, "yes", false, "confirm local registration removal")
	return command
}

func newMachineSnapshotCommand(options *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "snapshot <machine>", Short: "Read a fresh bounded remote inventory", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		machine, err := resolveMachine(command.Context(), client, args[0])
		if err != nil {
			return err
		}
		snapshot, err := client.MachineSnapshot(command.Context(), machine.Id)
		if err != nil {
			return err
		}
		return writeResult(options, "machine.snapshot", snapshot, func(writer io.Writer) error {
			_, err := fmt.Fprintf(writer, "%s · %d projects · %d environments · %s\n", snapshot.Identity.Name, len(snapshot.Projects), len(snapshot.Environments), snapshot.ObservedAt.Format("2006-01-02 15:04:05Z07:00"))
			return err
		})
	}}
}

func newMachineRunCommand(options *rootOptions) *cobra.Command {
	environmentID, requestID := "", ""
	yes := false
	command := &cobra.Command{Use: "run <machine> <project> <start|stop|restart|rebuild>", Short: "Submit one confirmed typed remote lifecycle operation", Args: cobra.ExactArgs(3), RunE: func(command *cobra.Command, args []string) error {
		if !yes {
			return usageError("REMOTE_CONFIRMATION_REQUIRED", "remote operations require --yes after reviewing the machine, project, and action")
		}
		if requestID == "" {
			var err error
			requestID, err = identifier.New("remote-cli")
			if err != nil {
				return err
			}
		}
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		machine, err := resolveMachine(command.Context(), client, args[0])
		if err != nil {
			return err
		}
		request := generated.RemoteOperationRequest{
			RequestId: requestID, ProjectId: args[1], Action: generated.RemoteOperationRequestAction(args[2]), ConfirmRisk: yes,
		}
		if environmentID != "" {
			request.EnvironmentId = &environmentID
		}
		receipt, err := client.CreateMachineOperation(command.Context(), machine.Id, request)
		if err != nil {
			return err
		}
		return writeResult(options, "machine.run", receipt, func(writer io.Writer) error {
			_, err := fmt.Fprintf(writer, "%s accepted by %s\noperation: %s\nrequest: %s\n", args[2], machine.Name, receipt.OperationId, receipt.RequestId)
			return err
		})
	}}
	command.Flags().StringVar(&environmentID, "environment", "", "optional registered remote environment ID")
	command.Flags().StringVar(&requestID, "request-id", "", "stable idempotency request ID")
	command.Flags().BoolVar(&yes, "yes", false, "confirm the reviewed remote lifecycle operation")
	return command
}

func resolveMachine(ctx context.Context, client *httpclient.Client, value string) (generated.Machine, error) {
	if machine, err := client.Machine(ctx, value); err == nil {
		return machine, nil
	}
	machines, err := client.Machines(ctx)
	if err != nil {
		return generated.Machine{}, err
	}
	var matches []generated.Machine
	for _, machine := range machines {
		if strings.EqualFold(machine.Name, value) {
			matches = append(matches, machine)
		}
	}
	if len(matches) != 1 {
		return generated.Machine{}, usageError("MACHINE_NOT_FOUND", "use an exact machine ID or unique display name")
	}
	return matches[0], nil
}

func writeMachineResult(options *rootOptions, command string, machine generated.Machine) error {
	return writeResult(options, command, machine, func(writer io.Writer) error {
		_, err := fmt.Fprintf(writer, "%s (%s) · %s · grants=%s\n", machine.Name, machine.Id, machine.State, strings.Join(stringValues(machine.GrantedCapabilities), ","))
		return err
	})
}

func generatedCapabilities(values []string) []generated.FleetCapability {
	result := make([]generated.FleetCapability, len(values))
	for index, value := range values {
		result[index] = generated.FleetCapability(value)
	}
	return result
}
