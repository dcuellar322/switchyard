package cli

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

func newOpenCommand(options *rootOptions) *cobra.Command {
	printOnly := false
	command := &cobra.Command{Use: "open <project>", Short: "Open the primary project endpoint", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		project, err := resolveProject(command.Context(), client, args[0])
		if err != nil {
			return err
		}
		effective, err := client.ExplainManifest(command.Context(), project.Id)
		if err != nil {
			return err
		}
		target, err := primaryEndpoint(effective.Manifest)
		if err != nil {
			return err
		}
		if !printOnly {
			if err := openURL(target); err != nil {
				return fmt.Errorf("open project endpoint: %w", err)
			}
		}
		result := map[string]any{"projectId": project.Id, "target": target, "opened": !printOnly}
		return writeResult(options, "open", result, func(w io.Writer) error { _, err := fmt.Fprintln(w, target); return err })
	}}
	command.Flags().BoolVar(&printOnly, "print", false, "print the endpoint without launching an application")
	return command
}

func primaryEndpoint(manifest map[string]any) (string, error) {
	ports := manifestPorts(manifest)
	values, ok := manifest["endpoints"].([]any)
	if !ok {
		return "", errors.New("project has no endpoint to open")
	}
	var fallback string
	for _, value := range values {
		endpoint, ok := value.(map[string]any)
		if !ok {
			continue
		}
		url, _ := endpoint["url"].(string)
		if url == "" {
			continue
		}
		if fallback == "" {
			fallback = url
		}
		if primary, _ := endpoint["primary"].(bool); primary {
			return expandEndpoint(url, ports), nil
		}
	}
	if fallback == "" {
		return "", errors.New("project has no endpoint to open")
	}
	return expandEndpoint(fallback, ports), nil
}

func manifestPorts(manifest map[string]any) map[string]string {
	result := map[string]string{}
	values, _ := manifest["ports"].([]any)
	for _, value := range values {
		port, ok := value.(map[string]any)
		if !ok {
			continue
		}
		id, _ := port["id"].(string)
		host, ok := port["host"].(float64)
		if id != "" && ok {
			result[id] = fmt.Sprint(int(host))
		}
	}
	return result
}

func expandEndpoint(value string, ports map[string]string) string {
	for id, port := range ports {
		value = strings.ReplaceAll(value, "${ports."+id+"}", port)
	}
	return value
}
