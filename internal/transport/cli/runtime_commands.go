package cli

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"switchyard.dev/switchyard/internal/foundation/identifier"
	"switchyard.dev/switchyard/internal/transport/contract/generated"
)

func newStatusCommand(options *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "status <project>", Short: "Observe current project runtime state", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		project, err := resolveProject(command.Context(), client, args[0])
		if err != nil {
			return err
		}
		observation, err := client.Runtime(command.Context(), project.Id)
		if err != nil {
			return err
		}
		return writeResult(options, "runtime.status", observation, func(w io.Writer) error {
			if _, err := fmt.Fprintf(w, "%s: %s (%s)\n", project.Slug, observation.State, observation.Origin); err != nil {
				return err
			}
			if observation.Engine != nil && !observation.Engine.Connected {
				message := "Docker Engine is unavailable"
				if observation.Engine.ErrorMessage != nil {
					message = *observation.Engine.ErrorMessage
				}
				_, err := fmt.Fprintln(w, message)
				return err
			}
			rows := make([][]string, 0, len(observation.Services))
			for _, service := range observation.Services {
				rows = append(rows, []string{service.Id, service.State, service.Health, publishedPorts(service.Ports), runtimeIdentity(service)})
			}
			return humanList(w, []string{"SERVICE", "STATE", "HEALTH", "PORTS", "RUNTIME"}, rows)
		})
	}}
}

func newPlanCommand(options *rootOptions) *cobra.Command {
	removeVolumes := false
	command := &cobra.Command{Use: "plan <action> <project>", Short: "Preview a runtime lifecycle action", Args: cobra.ExactArgs(2), RunE: func(command *cobra.Command, args []string) error {
		action, err := runtimeAction(args[0])
		if err != nil {
			return err
		}
		if removeVolumes && action != generated.RuntimeAction("teardown") {
			return usageError("VOLUMES_UNSUPPORTED", "--volumes is supported only for teardown plans")
		}
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		project, err := resolveProject(command.Context(), client, args[1])
		if err != nil {
			return err
		}
		plan, err := client.PlanRuntime(command.Context(), project.Id, action, removeVolumes)
		if err != nil {
			return err
		}
		return writeResult(options, "runtime.plan", plan, func(w io.Writer) error { return writeHumanPlan(w, plan) })
	}}
	command.Flags().BoolVar(&removeVolumes, "volumes", false, "include Compose volumes in a teardown plan")
	return command
}

func newLifecycleCommand(options *rootOptions, actionName string) *cobra.Command {
	removeVolumes, yes := false, false
	command := &cobra.Command{Use: actionName + " <project>", Short: lifecycleSummary(actionName), Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		if actionName == "teardown" && !yes {
			return usageError("CONFIRMATION_REQUIRED", "teardown requires --yes after reviewing `switchyard plan teardown <project>`")
		}
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		project, err := resolveProject(command.Context(), client, args[0])
		if err != nil {
			return err
		}
		key, err := identifier.New("cli")
		if err != nil {
			return err
		}
		operation, err := client.CreateRuntimeOperation(command.Context(), project.Id, generated.RuntimeAction(actionName), removeVolumes, key)
		if err != nil {
			return err
		}
		return writeResult(options, "runtime.operation", operation, func(w io.Writer) error {
			_, err := fmt.Fprintf(w, "%s queued for %s\noperation: %s\nstate: %s\n", actionName, project.Slug, operation.Id, operation.State)
			return err
		})
	}}
	if actionName == "teardown" {
		command.Flags().BoolVar(&yes, "yes", false, "confirm the destructive teardown")
		command.Flags().BoolVar(&removeVolumes, "volumes", false, "also remove Compose volumes")
	}
	return command
}

func newLogsCommand(options *rootOptions) *cobra.Command {
	service, since, runID, operationID, export, tail := "", "", "", "", "", 200
	command := &cobra.Command{Use: "logs <project>", Short: "Read a bounded runtime log snapshot", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		project, err := resolveProject(command.Context(), client, args[0])
		if err != nil {
			return err
		}
		if export != "" {
			format := generated.ExportProjectLogsParamsFormat(export)
			if !format.Valid() {
				return usageError("LOG_EXPORT_FORMAT_INVALID", "--export must be plain or ndjson")
			}
			contents, err := client.ExportRuntimeLogs(command.Context(), project.Id, service, runID, operationID, format)
			if err != nil {
				return err
			}
			return writeResult(options, "runtime.logs.export", map[string]any{"format": format, "content": string(contents)}, func(w io.Writer) error {
				_, err := w.Write(contents)
				return err
			})
		}
		entries, err := client.RuntimeLogs(command.Context(), project.Id, service, since, runID, operationID, tail)
		if err != nil {
			return err
		}
		return writeResult(options, "runtime.logs", entries, func(w io.Writer) error {
			for _, entry := range entries {
				if _, err := fmt.Fprintf(w, "%s %-12s %-6s %s\n", entry.Timestamp.Format("15:04:05.000"), entry.ServiceId, entry.Stream, entry.Message); err != nil {
					return err
				}
			}
			return nil
		})
	}}
	command.Flags().StringVar(&service, "service", "", "limit to a runtime service")
	command.Flags().StringVar(&since, "since", "", "runtime timestamp or duration boundary")
	command.Flags().StringVar(&runID, "run", "", "limit to one runtime run")
	command.Flags().StringVar(&operationID, "operation", "", "limit to one lifecycle operation")
	command.Flags().StringVar(&export, "export", "", "export persisted redacted logs as plain or ndjson")
	command.Flags().IntVar(&tail, "tail", 200, "maximum lines across selected runtimes")
	return command
}

func newMetricsCommand(options *rootOptions) *cobra.Command {
	service := ""
	command := &cobra.Command{Use: "metrics <project>", Short: "Read current runtime resource samples", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		project, err := resolveProject(command.Context(), client, args[0])
		if err != nil {
			return err
		}
		samples, err := client.RuntimeMetrics(command.Context(), project.Id, service)
		if err != nil {
			return err
		}
		return writeResult(options, "runtime.metrics", samples, func(w io.Writer) error {
			rows := make([][]string, 0, len(samples))
			for _, sample := range samples {
				rows = append(rows, []string{sample.ServiceId, fmt.Sprintf("%.2f%%", sample.CpuPercent), byteCount(sample.MemoryBytes), byteCount(sample.NetworkRxBytes), byteCount(sample.NetworkTxBytes)})
			}
			return humanList(w, []string{"SERVICE", "CPU", "MEMORY", "NET RX", "NET TX"}, rows)
		})
	}}
	command.Flags().StringVar(&service, "service", "", "limit to a runtime service")
	return command
}

func runtimeAction(value string) (generated.RuntimeAction, error) {
	for _, allowed := range []string{"start", "stop", "restart", "pause", "unpause", "rebuild", "teardown"} {
		if value == allowed {
			return generated.RuntimeAction(value), nil
		}
	}
	return "", usageError("RUNTIME_ACTION_INVALID", "supported actions: start, stop, restart, pause, unpause, rebuild, teardown")
}

func writeHumanPlan(writer io.Writer, plan generated.RuntimePlan) error {
	if _, err := fmt.Fprintf(writer, "%s\nrisk: %s\n", plan.Summary, plan.Risk); err != nil {
		return err
	}
	for _, effect := range plan.Effects {
		if _, err := fmt.Fprintf(writer, "- %s\n", effect); err != nil {
			return err
		}
	}
	for _, command := range plan.Commands {
		arguments := make([]string, 0, len(command.Arguments)+1)
		arguments = append(arguments, strconv.Quote(command.Executable))
		for _, argument := range command.Arguments {
			arguments = append(arguments, strconv.Quote(argument))
		}
		if _, err := fmt.Fprintf(writer, "command: %s\ncwd: %s\n", strings.Join(arguments, " "), command.WorkingDirectory); err != nil {
			return err
		}
	}
	return nil
}

func publishedPorts(ports []generated.PublishedPort) string {
	values := make([]string, 0, len(ports))
	for _, port := range ports {
		if port.HostPort != nil && *port.HostPort > 0 {
			values = append(values, fmt.Sprintf("%d->%d/%s", *port.HostPort, port.ContainerPort, port.Protocol))
		}
	}
	return strings.Join(values, ",")
}

func shortContainerID(value string) string {
	if len(value) > 12 {
		return value[:12]
	}
	return value
}

func runtimeIdentity(service generated.RuntimeServiceObservation) string {
	if service.Container != nil {
		return shortContainerID(service.Container.Id)
	}
	if service.Process != nil {
		identity := "pid:" + strconv.FormatInt(int64(service.Process.Pid), 10)
		if service.State != "running" && service.State != "starting" {
			identity += " (last)"
		}
		return identity
	}
	return "-"
}

func lifecycleSummary(action string) string {
	return strings.ToUpper(action[:1]) + action[1:] + " project runtime"
}

func byteCount(value int64) string {
	const unit = int64(1024)
	if value < unit {
		return fmt.Sprintf("%d B", value)
	}
	divisor, exponent := unit, 0
	for quotient := value / unit; quotient >= unit; quotient /= unit {
		divisor *= unit
		exponent++
	}
	return fmt.Sprintf("%.1f %ciB", float64(value)/float64(divisor), "KMGTPE"[exponent])
}
