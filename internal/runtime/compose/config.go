package compose

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"switchyard.dev/switchyard/internal/runtime/domain"
)

type normalizedConfig struct {
	ProjectName string
	Services    []string
	Connection  dockerConnection
	BaseArgs    []string
}

type configReader struct {
	runner            commandRunner
	contexts          contextResolver
	artifactDirectory string
}

func (r configReader) Normalize(ctx context.Context, project domain.ProjectRuntime) (normalizedConfig, error) {
	if project.Compose == nil || len(project.Compose.Files) == 0 {
		return normalizedConfig{}, errors.New("compose runtime requires at least one file")
	}
	connection, err := r.contexts.Resolve(ctx, project.Compose.Context, project.Root)
	if err != nil {
		return normalizedConfig{}, err
	}
	arguments, err := composeBaseArguments(project, connection, project.Compose.ProjectName)
	if err != nil {
		return normalizedConfig{}, err
	}
	if len(project.Compose.PortOverrides) > 0 {
		overridePath, overrideErr := writePortOverride(r.artifactDirectory, project)
		if overrideErr != nil {
			return normalizedConfig{}, overrideErr
		}
		arguments = appendComposeFile(arguments, overridePath)
	}
	baseArguments := append([]string(nil), arguments...)
	arguments = append(arguments, "config", "--format", "json")
	var stdout, stderr limitedBuffer
	err = r.runner.Run(ctx, domain.Command{Executable: "docker", Arguments: arguments, WorkingDirectory: project.Root}, &stdout, &stderr)
	if err != nil {
		return normalizedConfig{}, commandError("normalize Docker Compose config", err, stderr.String())
	}
	var document struct {
		Name     string                     `json:"name"`
		Services map[string]json.RawMessage `json:"services"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &document); err != nil {
		return normalizedConfig{}, fmt.Errorf("decode normalized Docker Compose config: %w", err)
	}
	if document.Name == "" || len(document.Services) == 0 {
		return normalizedConfig{}, errors.New("normalized Docker Compose config has no project name or services")
	}
	services := make([]string, 0, len(document.Services))
	for service := range document.Services {
		services = append(services, service)
	}
	sort.Strings(services)
	return normalizedConfig{ProjectName: document.Name, Services: services, Connection: connection, BaseArgs: baseArguments}, nil
}

func appendComposeFile(arguments []string, path string) []string {
	projectNameIndex := len(arguments)
	for index, argument := range arguments {
		if argument == "--project-name" {
			projectNameIndex = index
			break
		}
	}
	result := make([]string, 0, len(arguments)+2)
	result = append(result, arguments[:projectNameIndex]...)
	result = append(result, "--file", path)
	result = append(result, arguments[projectNameIndex:]...)
	return result
}

func composeBaseArguments(project domain.ProjectRuntime, connection dockerConnection, projectName string) ([]string, error) {
	arguments := append(connection.cliPrefix(), "compose", "--project-directory", project.Root)
	for _, file := range project.Compose.Files {
		path := file
		if !filepath.IsAbs(path) {
			path = filepath.Join(project.Root, path)
		}
		clean := filepath.Clean(path)
		within, err := filepath.Rel(project.Root, clean)
		if err != nil || within == ".." || strings.HasPrefix(within, ".."+string(filepath.Separator)) || filepath.IsAbs(within) {
			return nil, fmt.Errorf("compose file leaves trusted project root: %s", file)
		}
		arguments = append(arguments, "--file", clean)
	}
	if projectName != "" {
		arguments = append(arguments, "--project-name", projectName)
	}
	return arguments, nil
}
