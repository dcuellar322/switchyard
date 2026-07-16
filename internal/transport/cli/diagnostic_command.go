package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"switchyard.dev/switchyard/internal/foundation/identifier"
	"switchyard.dev/switchyard/internal/transport/contract/generated"
)

func newDiagnoseCommand(options *rootOptions) *cobra.Command {
	provider := ""
	command := &cobra.Command{
		Use: "diagnose <project>", Short: "Build an evidence-backed project diagnosis", Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			client, err := daemonClient(command.Context(), options)
			if err != nil {
				return err
			}
			project, err := resolveProject(command.Context(), client, args[0])
			if err != nil {
				return err
			}
			diagnosis, err := client.Diagnose(command.Context(), project.Id, provider)
			if err != nil {
				return err
			}
			return writeDiagnosis(options, "diagnose", diagnosis)
		},
	}
	command.Flags().StringVar(&provider, "provider", provider, "optional configured AI provider; deterministic rules always run first")
	command.AddCommand(newDiagnosisLatestCommand(options), newDiagnosisFeedbackCommand(options), newDiagnosisActionCommand(options), newDiagnosticNotificationsCommand(options))
	return command
}

func newDiagnosisLatestCommand(options *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "latest <project>", Short: "Read the latest durable diagnosis", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		project, err := resolveProject(command.Context(), client, args[0])
		if err != nil {
			return err
		}
		diagnosis, err := client.LatestDiagnosis(command.Context(), project.Id)
		if err != nil {
			return err
		}
		return writeDiagnosis(options, "diagnose.latest", diagnosis)
	}}
}

func newDiagnosisFeedbackCommand(options *rootOptions) *cobra.Command {
	verdict, note := "", ""
	command := &cobra.Command{Use: "feedback <diagnosis> <hypothesis>", Short: "Record local-only accuracy feedback", Args: cobra.ExactArgs(2), RunE: func(command *cobra.Command, args []string) error {
		if verdict != "accurate" && verdict != "false_positive" {
			return usageError("DIAGNOSTIC_FEEDBACK_INVALID", "--verdict must be accurate or false_positive")
		}
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		feedback, err := client.RecordDiagnosticFeedback(command.Context(), args[0], args[1], verdict, note)
		if err != nil {
			return err
		}
		return writeResult(options, "diagnose.feedback", feedback, func(writer io.Writer) error {
			_, err := fmt.Fprintf(writer, "%s recorded locally for %s\n", feedback.Verdict, feedback.HypothesisId)
			return err
		})
	}}
	command.Flags().StringVar(&verdict, "verdict", verdict, "accurate or false_positive")
	command.Flags().StringVar(&note, "note", note, "optional local note (never sent as telemetry)")
	_ = command.MarkFlagRequired("verdict")
	return command
}

func newDiagnosisActionCommand(options *rootOptions) *cobra.Command {
	yes := false
	command := &cobra.Command{Use: "run <diagnosis> <action>", Short: "Run an existing approved action cited by a diagnosis", Args: cobra.ExactArgs(2), RunE: func(command *cobra.Command, args []string) error {
		if !yes {
			return usageError("DIAGNOSTIC_ACTION_CONFIRMATION_REQUIRED", "diagnostic actions require --yes after reviewing the action and evidence")
		}
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		key, err := identifier.New("cli")
		if err != nil {
			return err
		}
		operation, err := client.RunDiagnosticAction(command.Context(), args[0], args[1], key)
		if err != nil {
			return err
		}
		return writeResult(options, "diagnose.run", operation, func(writer io.Writer) error {
			_, err := fmt.Fprintf(writer, "%s queued as %s\n", args[1], operation.Id)
			return err
		})
	}}
	command.Flags().BoolVar(&yes, "yes", false, "confirm the existing approved action")
	return command
}

func newDiagnosticNotificationsCommand(options *rootOptions) *cobra.Command {
	include := false
	command := &cobra.Command{Use: "notifications [project]", Short: "List local crash, port, resource, and health warnings", Args: cobra.MaximumNArgs(1), RunE: func(command *cobra.Command, args []string) error {
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		projectID := ""
		if len(args) == 1 {
			project, resolveErr := resolveProject(command.Context(), client, args[0])
			if resolveErr != nil {
				return resolveErr
			}
			projectID = project.Id
		}
		values, err := client.DiagnosticNotifications(command.Context(), projectID, include)
		if err != nil {
			return err
		}
		return writeResult(options, "diagnose.notifications", values, func(writer io.Writer) error {
			rows := make([][]string, 0, len(values))
			for _, value := range values {
				rows = append(rows, []string{value.ProjectId, value.Code, fmt.Sprint(value.Occurrences), value.Title})
			}
			return humanList(writer, []string{"PROJECT", "CODE", "COUNT", "TITLE"}, rows)
		})
	}}
	command.Flags().BoolVar(&include, "all", include, "include acknowledged notifications")
	command.AddCommand(newDiagnosticNotificationAcknowledgeCommand(options))
	return command
}

func newDiagnosticNotificationAcknowledgeCommand(options *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "acknowledge <notification>", Short: "Mark one local diagnostic warning reviewed", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		value, err := client.AcknowledgeDiagnosticNotification(command.Context(), args[0])
		if err != nil {
			return err
		}
		return writeResult(options, "diagnose.notifications.acknowledge", value, func(writer io.Writer) error {
			_, err := fmt.Fprintf(writer, "%s acknowledged\n", value.Id)
			return err
		})
	}}
}

func writeDiagnosis(options *rootOptions, kind string, diagnosis generated.Diagnosis) error {
	return writeResult(options, kind, diagnosis, func(writer io.Writer) error {
		if len(diagnosis.Hypotheses) == 0 {
			_, err := fmt.Fprintln(writer, "No known failure pattern was detected in the current bounded evidence.")
			return err
		}
		for _, item := range diagnosis.Hypotheses {
			if _, err := fmt.Fprintf(writer, "%-7s %3.0f%% %-13s %s\n  %s\n", strings.ToUpper(string(item.Severity)), item.Confidence*100, item.Source, item.Title, item.Summary); err != nil {
				return err
			}
		}
		return nil
	})
}

func newAutomationCommand(options *rootOptions) *cobra.Command {
	command := &cobra.Command{Use: "automation", Aliases: []string{"automations"}, Short: "Inspect and control bounded automation recipes"}
	command.AddCommand(newAutomationListCommand(options), newAutomationCreateCommand(options), newAutomationToggleCommand(options, true), newAutomationToggleCommand(options, false), newAutomationEvaluateCommand(options))
	return command
}

func newAutomationListCommand(options *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "list [project]", Short: "List enabled and disabled recipes", Args: cobra.MaximumNArgs(1), RunE: func(command *cobra.Command, args []string) error {
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		projectID := ""
		if len(args) == 1 {
			project, resolveErr := resolveProject(command.Context(), client, args[0])
			if resolveErr != nil {
				return resolveErr
			}
			projectID = project.Id
		}
		values, err := client.AutomationRecipes(command.Context(), projectID)
		if err != nil {
			return err
		}
		return writeResult(options, "automation.list", values, func(writer io.Writer) error {
			rows := make([][]string, 0, len(values))
			for _, value := range values {
				rows = append(rows, []string{value.Id, fmt.Sprint(value.Enabled), string(value.TriggerCode), value.ActionId, fmt.Sprintf("%d/%d", value.RunsToday, value.MaxRunsPerDay)})
			}
			return humanList(writer, []string{"RECIPE", "ENABLED", "TRIGGER", "ACTION", "RUNS"}, rows)
		})
	}}
}

func newAutomationCreateCommand(options *rootOptions) *cobra.Command {
	name, trigger := "", ""
	cooldown, maximum := 3600, 3
	command := &cobra.Command{Use: "create <project> <action>", Short: "Save a disabled recipe for later review", Args: cobra.ExactArgs(2), RunE: func(command *cobra.Command, args []string) error {
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		project, err := resolveProject(command.Context(), client, args[0])
		if err != nil {
			return err
		}
		recipe, err := client.CreateAutomationRecipe(command.Context(), generated.CreateAutomationRecipeRequest{ProjectId: project.Id, Name: name, TriggerCode: generated.AutomationTrigger(trigger), ActionId: args[1], CooldownSeconds: cooldown, MaxRunsPerDay: maximum})
		if err != nil {
			return err
		}
		return writeResult(options, "automation.create", recipe, func(writer io.Writer) error {
			_, err := fmt.Fprintf(writer, "%s saved disabled; review then enable explicitly\n", recipe.Id)
			return err
		})
	}}
	command.Flags().StringVar(&name, "name", name, "human-readable recipe name")
	command.Flags().StringVar(&trigger, "trigger", trigger, "supported deterministic finding code")
	command.Flags().IntVar(&cooldown, "cooldown", cooldown, "minimum seconds between runs")
	command.Flags().IntVar(&maximum, "max-per-day", maximum, "maximum runs per UTC day")
	_ = command.MarkFlagRequired("name")
	_ = command.MarkFlagRequired("trigger")
	return command
}

func newAutomationToggleCommand(options *rootOptions, enabled bool) *cobra.Command {
	name, short := "disable", "Disable a saved recipe immediately"
	if enabled {
		name, short = "enable", "Enable a reviewed recipe with its explicit limits"
	}
	yes := false
	command := &cobra.Command{Use: name + " <recipe>", Short: short, Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		if enabled && !yes {
			return usageError("AUTOMATION_CONFIRMATION_REQUIRED", "enabling automation requires --yes after reviewing its trigger, action, cooldown, and daily limit")
		}
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		recipe, err := client.SetAutomationRecipeEnabled(command.Context(), args[0], enabled)
		if err != nil {
			return err
		}
		return writeResult(options, "automation."+name, recipe, func(writer io.Writer) error {
			_, err := fmt.Fprintf(writer, "%s enabled=%t\n", recipe.Id, recipe.Enabled)
			return err
		})
	}}
	command.Flags().BoolVar(&yes, "yes", false, "confirm the reviewed recipe")
	return command
}

func newAutomationEvaluateCommand(options *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "evaluate <project>", Short: "Evaluate triggers and dispatch due safe recipes", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		project, err := resolveProject(command.Context(), client, args[0])
		if err != nil {
			return err
		}
		result, err := client.EvaluateAutomations(command.Context(), project.Id)
		if err != nil {
			return err
		}
		return writeResult(options, "automation.evaluate", result, func(writer io.Writer) error {
			_, err := fmt.Fprintf(writer, "%d operation(s) dispatched\n", len(result.OperationIds))
			return err
		})
	}}
}
