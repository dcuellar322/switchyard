package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var cliCommands = []string{
	"debug.logs", "doctor", "doctor.bundle", "doctor.bundle.preview", "manifest.diff", "manifest.explain", "manifest.validate", "open", "operation.cancel",
	"operation.get", "operation.list", "project.add", "project.get", "project.list", "project.remove", "project.trust",
	"runtime.logs", "runtime.metrics", "runtime.plan", "runtime.status", "runtime.operation", "ui", "version",
}

func newSchemaCommand(options *rootOptions) *cobra.Command {
	command := &cobra.Command{Use: "schema", Short: "Print machine-readable output schemas"}
	cli := &cobra.Command{Use: "cli <command>", Short: "Print a CLI command envelope schema", Args: cobra.ExactArgs(1), ValidArgs: cliCommands, RunE: func(_ *cobra.Command, args []string) error {
		if !contains(cliCommands, args[0]) {
			return usageError("CLI_SCHEMA_UNKNOWN", fmt.Sprintf("unknown command schema %q; available: %s", args[0], strings.Join(cliCommands, ", ")))
		}
		schema := map[string]any{
			"$schema": "https://json-schema.org/draft/2020-12/schema",
			"$id":     "https://switchyard.dev/schema/cli/" + strings.ReplaceAll(args[0], ".", "-") + ".v1.json",
			"type":    "object", "additionalProperties": false,
			"required": []string{"schemaVersion", "command", "data"},
			"properties": map[string]any{
				"schemaVersion": map[string]any{"const": cliSchemaVersion},
				"command":       map[string]any{"const": args[0]},
				"data":          commandDataSchema(args[0]),
			},
		}
		return writePrettyJSON(options.stdout, schema)
	}}
	command.AddCommand(cli)
	return command
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func commandDataSchema(command string) map[string]any {
	openAPIRef := func(name string) map[string]any {
		return map[string]any{"$ref": "https://switchyard.dev/schema/openapi.v1.json#/components/schemas/" + name}
	}
	if schema, ok := supportCommandDataSchema(command); ok {
		return schema
	}
	switch command {
	case "project.get":
		return openAPIRef("Project")
	case "project.list":
		return map[string]any{"type": "array", "items": openAPIRef("Project")}
	case "project.add":
		return openAPIRef("ManifestProposal")
	case "project.trust":
		return openAPIRef("AcceptedManifestProposal")
	case "operation.get", "operation.cancel":
		return openAPIRef("Operation")
	case "runtime.operation":
		return openAPIRef("Operation")
	case "operation.list":
		return map[string]any{"type": "array", "items": openAPIRef("Operation")}
	case "runtime.status":
		return openAPIRef("RuntimeObservation")
	case "runtime.plan":
		return openAPIRef("RuntimePlan")
	case "runtime.logs":
		return map[string]any{"type": "array", "items": openAPIRef("RuntimeLogEntry")}
	case "runtime.metrics":
		return map[string]any{"type": "array", "items": openAPIRef("RuntimeMetricSample")}
	case "manifest.explain":
		return openAPIRef("EffectiveManifest")
	case "manifest.diff":
		return openAPIRef("ManifestDiff")
	case "manifest.validate":
		return openAPIRef("ManifestValidation")
	case "project.remove":
		return map[string]any{"type": "object", "required": []string{"id", "slug", "removed", "repositoryFilesChanged"}}
	case "open":
		return map[string]any{"type": "object", "required": []string{"projectId", "target", "opened"}}
	case "ui":
		return map[string]any{"type": "object", "required": []string{"url", "expiresAt"}}
	case "version":
		return map[string]any{"type": "object"}
	default:
		return map[string]any{}
	}
}

func supportCommandDataSchema(command string) (map[string]any, bool) {
	switch command {
	case "doctor":
		return map[string]any{"$ref": "https://switchyard.dev/schema/openapi.v1.json#/components/schemas/SystemInfo"}, true
	case "doctor.bundle", "doctor.bundle.preview":
		return map[string]any{"type": "object"}, true
	case "debug.logs":
		return map[string]any{"type": "array", "items": map[string]any{"type": "object"}}, true
	default:
		return nil, false
	}
}
