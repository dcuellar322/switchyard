package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"switchyard.dev/switchyard/internal/transport/contract/generated"
)

type desktopSnapshot struct {
	System            generated.SystemInfo       `json:"system"`
	Host              *generated.HostObservation `json:"host,omitempty"`
	Projects          []desktopProjectSnapshot   `json:"projects"`
	Workspaces        []generated.Workspace      `json:"workspaces"`
	Operations        []generated.Operation      `json:"operations"`
	PortConflictCount int                        `json:"portConflictCount"`
	Warnings          []string                   `json:"warnings"`
}

type desktopProjectSnapshot struct {
	Project  generated.Project             `json:"project"`
	Runtime  *generated.RuntimeObservation `json:"runtime,omitempty"`
	Health   *generated.ProjectHealth      `json:"health,omitempty"`
	Warnings []string                      `json:"warnings"`
}

func newDesktopCommand(options *rootOptions) *cobra.Command {
	command := &cobra.Command{Use: "desktop", Short: "Support the thin native desktop adapter"}
	command.AddCommand(newDesktopSnapshotCommand(options))
	return command
}

func newDesktopSnapshotCommand(options *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "snapshot",
		Short: "Read one bounded tray and notification snapshot",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			client, err := daemonClient(command.Context(), options)
			if err != nil {
				return err
			}
			system, err := client.System(command.Context())
			if err != nil {
				return err
			}
			projects, err := client.Projects(command.Context())
			if err != nil {
				return err
			}

			snapshot := desktopSnapshot{
				System: system, Projects: make([]desktopProjectSnapshot, 0, len(projects)),
				Workspaces: []generated.Workspace{}, Operations: []generated.Operation{}, Warnings: []string{},
			}
			if host, hostErr := client.Host(command.Context()); hostErr == nil {
				snapshot.Host = &host
			} else {
				snapshot.Warnings = append(snapshot.Warnings, "host observation unavailable")
			}
			if workspaces, workspaceErr := client.Workspaces(command.Context()); workspaceErr == nil {
				snapshot.Workspaces = workspaces
			} else {
				snapshot.Warnings = append(snapshot.Warnings, "workspace snapshot unavailable")
			}
			if operations, operationErr := client.Operations(command.Context(), "", 25); operationErr == nil {
				snapshot.Operations = operations
			} else {
				snapshot.Warnings = append(snapshot.Warnings, "operation snapshot unavailable")
			}
			if ports, portErr := client.PortRegistry(command.Context()); portErr == nil {
				snapshot.PortConflictCount = len(ports.Conflicts)
			} else {
				snapshot.Warnings = append(snapshot.Warnings, "port conflict snapshot unavailable")
			}

			for _, project := range projects {
				item := desktopProjectSnapshot{Project: project, Warnings: []string{}}
				if project.TrustState == generated.ProjectTrustStateTrusted {
					if runtime, runtimeErr := client.Runtime(command.Context(), project.Id); runtimeErr == nil {
						item.Runtime = &runtime
					} else {
						item.Warnings = append(item.Warnings, "runtime observation unavailable")
					}
					if health, healthErr := client.Health(command.Context(), project.Id); healthErr == nil {
						item.Health = &health
					} else {
						item.Warnings = append(item.Warnings, "health observation unavailable")
					}
				}
				snapshot.Projects = append(snapshot.Projects, item)
			}

			return writeResult(options, "desktop.snapshot", snapshot, func(writer io.Writer) error {
				_, err := fmt.Fprintf(
					writer,
					"daemon=%s version=%s projects=%d workspaces=%d conflicts=%d warnings=%d\n",
					snapshot.System.Status,
					snapshot.System.Version,
					len(snapshot.Projects),
					len(snapshot.Workspaces),
					snapshot.PortConflictCount,
					len(snapshot.Warnings),
				)
				return err
			})
		},
	}
}
