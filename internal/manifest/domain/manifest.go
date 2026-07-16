// Package domain owns the canonical project manifest model and invariants.
package domain

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
)

const (
	// SchemaVersion is the first portable manifest contract.
	SchemaVersion = "switchyard.dev/v1alpha1"
	// KindProject is the only Phase 3 document kind.
	KindProject = "Project"
)

var manifestID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// Manifest is the canonical portable project declaration.
type Manifest struct {
	SchemaVersion  string         `json:"schemaVersion" yaml:"schemaVersion" jsonschema:"required,enum=switchyard.dev/v1alpha1"`
	Kind           string         `json:"kind" yaml:"kind" jsonschema:"required,enum=Project"`
	Metadata       Metadata       `json:"metadata" yaml:"metadata" jsonschema:"required"`
	Repository     Repository     `json:"repository" yaml:"repository" jsonschema:"required"`
	Runtime        Runtime        `json:"runtime,omitempty" yaml:"runtime,omitempty"`
	Lifecycle      Lifecycle      `json:"lifecycle,omitempty" yaml:"lifecycle,omitempty"`
	Services       []Service      `json:"services,omitempty" yaml:"services,omitempty"`
	Ports          []Port         `json:"ports,omitempty" yaml:"ports,omitempty"`
	Endpoints      []Endpoint     `json:"endpoints,omitempty" yaml:"endpoints,omitempty"`
	Actions        []Action       `json:"actions,omitempty" yaml:"actions,omitempty"`
	ResourcePolicy ResourcePolicy `json:"resourcePolicy,omitempty" yaml:"resourcePolicy,omitempty"`
}

// Metadata identifies a project for humans and automation.
type Metadata struct {
	ID          string   `json:"id" yaml:"id" jsonschema:"required,pattern=^[a-z0-9][a-z0-9-]*$,maxLength=63"`
	Name        string   `json:"name" yaml:"name" jsonschema:"required,minLength=1,maxLength=120"`
	Description string   `json:"description,omitempty" yaml:"description,omitempty" jsonschema:"maxLength=1000"`
	Tags        []string `json:"tags,omitempty" yaml:"tags,omitempty"`
}

// Repository declares portable repository facts.
type Repository struct {
	Root          string `json:"root" yaml:"root" jsonschema:"required"`
	DefaultBranch string `json:"defaultBranch,omitempty" yaml:"defaultBranch,omitempty"`
}

// Runtime selects the project lifecycle driver.
type Runtime struct {
	Driver  string         `json:"driver,omitempty" yaml:"driver,omitempty" jsonschema:"enum=compose,enum=process,enum=external"`
	Compose *ComposeConfig `json:"compose,omitempty" yaml:"compose,omitempty"`
}

// ComposeConfig identifies portable Compose inputs.
type ComposeConfig struct {
	Files       []string `json:"files" yaml:"files" jsonschema:"required,minItems=1"`
	ProjectName string   `json:"projectName,omitempty" yaml:"projectName,omitempty"`
	Context     string   `json:"context,omitempty" yaml:"context,omitempty"`
}

// Lifecycle declares the standard project mutations.
type Lifecycle struct {
	Start    *Invocation `json:"start,omitempty" yaml:"start,omitempty"`
	Stop     *Invocation `json:"stop,omitempty" yaml:"stop,omitempty"`
	Restart  *Invocation `json:"restart,omitempty" yaml:"restart,omitempty"`
	Rebuild  *Invocation `json:"rebuild,omitempty" yaml:"rebuild,omitempty"`
	Teardown *Invocation `json:"teardown,omitempty" yaml:"teardown,omitempty"`
}

// Invocation references an approved typed action.
type Invocation struct {
	Action  string         `json:"action" yaml:"action" jsonschema:"required"`
	Risk    string         `json:"risk,omitempty" yaml:"risk,omitempty" jsonschema:"enum=safe,enum=caution,enum=destructive"`
	Options map[string]any `json:"options,omitempty" yaml:"options,omitempty"`
}

// Service is a project-visible runtime unit.
type Service struct {
	ID           string        `json:"id" yaml:"id" jsonschema:"required"`
	DisplayName  string        `json:"displayName,omitempty" yaml:"displayName,omitempty"`
	Source       ServiceSource `json:"source" yaml:"source" jsonschema:"required"`
	Dependencies []string      `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`
	HealthChecks []HealthCheck `json:"healthChecks,omitempty" yaml:"healthChecks,omitempty"`
}

// ServiceSource binds a service to one runtime declaration.
type ServiceSource struct {
	ComposeService string `json:"composeService,omitempty" yaml:"composeService,omitempty"`
	Process        string `json:"process,omitempty" yaml:"process,omitempty"`
}

// HealthCheck declares bounded readiness validation.
type HealthCheck struct {
	Type           string   `json:"type" yaml:"type" jsonschema:"required,enum=http,enum=tcp,enum=command"`
	URL            string   `json:"url,omitempty" yaml:"url,omitempty"`
	ExpectedStatus int      `json:"expectedStatus,omitempty" yaml:"expectedStatus,omitempty"`
	Command        []string `json:"command,omitempty" yaml:"command,omitempty"`
}

// Port is a portable port declaration, not an observed binding.
type Port struct {
	ID       string `json:"id" yaml:"id" jsonschema:"required"`
	Service  string `json:"service,omitempty" yaml:"service,omitempty"`
	Host     int    `json:"host" yaml:"host" jsonschema:"required,minimum=1,maximum=65535"`
	Target   int    `json:"target" yaml:"target" jsonschema:"required,minimum=1,maximum=65535"`
	Protocol string `json:"protocol" yaml:"protocol" jsonschema:"required,enum=tcp,enum=udp"`
}

// Endpoint is a user-facing local URL.
type Endpoint struct {
	ID      string `json:"id" yaml:"id" jsonschema:"required"`
	Name    string `json:"name" yaml:"name" jsonschema:"required"`
	URL     string `json:"url" yaml:"url" jsonschema:"required"`
	Primary bool   `json:"primary,omitempty" yaml:"primary,omitempty"`
}

// Action is an explicit argument-array command or typed built-in capability.
type Action struct {
	ID               string   `json:"id" yaml:"id" jsonschema:"required"`
	Name             string   `json:"name" yaml:"name" jsonschema:"required"`
	Type             string   `json:"type" yaml:"type" jsonschema:"required"`
	Command          []string `json:"command,omitempty" yaml:"command,omitempty"`
	WorkingDirectory string   `json:"workingDirectory,omitempty" yaml:"workingDirectory,omitempty"`
	Shell            bool     `json:"shell,omitempty" yaml:"shell,omitempty"`
	CaptureOutput    bool     `json:"captureOutput,omitempty" yaml:"captureOutput,omitempty"`
	Provider         string   `json:"provider,omitempty" yaml:"provider,omitempty"`
}

// ResourcePolicy contains warning thresholds only.
type ResourcePolicy struct {
	Warnings ResourceWarnings `json:"warnings,omitempty" yaml:"warnings,omitempty"`
}

// ResourceWarnings are advisory local thresholds.
type ResourceWarnings struct {
	MemoryMiB  int `json:"memoryMiB,omitempty" yaml:"memoryMiB,omitempty" jsonschema:"minimum=0"`
	CPUPercent int `json:"cpuPercent,omitempty" yaml:"cpuPercent,omitempty" jsonschema:"minimum=0,maximum=100"`
	StorageGiB int `json:"storageGiB,omitempty" yaml:"storageGiB,omitempty" jsonschema:"minimum=0"`
}

// Validate checks invariants that JSON Schema cannot express clearly.
func (m Manifest) Validate() error {
	var problems []error
	if m.SchemaVersion != SchemaVersion {
		problems = append(problems, fmt.Errorf("schemaVersion must be %q", SchemaVersion))
	}
	if m.Kind != KindProject {
		problems = append(problems, fmt.Errorf("kind must be %q", KindProject))
	}
	if m.Metadata.ID == "" || m.Metadata.Name == "" {
		problems = append(problems, errors.New("metadata.id and metadata.name are required"))
	}
	if len(m.Metadata.ID) > 63 || !manifestID.MatchString(m.Metadata.ID) {
		problems = append(problems, errors.New("metadata.id must be a lowercase slug no longer than 63 characters"))
	}
	if m.Repository.Root == "" {
		problems = append(problems, errors.New("repository.root is required"))
	}
	problems = append(problems, uniqueIDs("service", serviceIDs(m.Services))...)
	problems = append(problems, uniqueIDs("port", portIDs(m.Ports))...)
	problems = append(problems, uniqueIDs("action", actionIDs(m.Actions))...)
	problems = append(problems, uniqueIDs("endpoint", endpointIDs(m.Endpoints))...)
	problems = append(problems, validateServices(m.Services)...)
	problems = append(problems, validateRuntime(m.Runtime)...)
	problems = append(problems, validatePorts(m.Ports, m.Services)...)
	problems = append(problems, validateEndpoints(m.Endpoints)...)
	problems = append(problems, validateActions(m.Actions)...)
	return errors.Join(problems...)
}

func validateServices(services []Service) []error {
	serviceSet := make(map[string]struct{}, len(services))
	for _, service := range services {
		serviceSet[service.ID] = struct{}{}
	}
	var problems []error
	for _, service := range services {
		sources := 0
		if service.Source.ComposeService != "" {
			sources++
		}
		if service.Source.Process != "" {
			sources++
		}
		if sources != 1 {
			problems = append(problems, fmt.Errorf("service %q must declare exactly one source", service.ID))
		}
		for _, dependency := range service.Dependencies {
			if _, ok := serviceSet[dependency]; !ok {
				problems = append(problems, fmt.Errorf("service %q references unknown dependency %q", service.ID, dependency))
			}
		}
		for _, check := range service.HealthChecks {
			switch check.Type {
			case "http":
				if check.URL == "" {
					problems = append(problems, fmt.Errorf("service %q HTTP health check requires a URL", service.ID))
				}
			case "tcp":
			case "command":
				if len(check.Command) == 0 {
					problems = append(problems, fmt.Errorf("service %q command health check requires an argument array", service.ID))
				}
			default:
				problems = append(problems, fmt.Errorf("service %q has unknown health check type %q", service.ID, check.Type))
			}
		}
	}
	return problems
}

func validateRuntime(runtime Runtime) []error {
	var problems []error
	if runtime.Driver == "compose" && (runtime.Compose == nil || len(runtime.Compose.Files) == 0) {
		problems = append(problems, errors.New("compose runtime requires at least one Compose file"))
	}
	return problems
}

func validatePorts(ports []Port, services []Service) []error {
	var problems []error
	for _, port := range ports {
		if port.Host < 1 || port.Host > 65535 || port.Target < 1 || port.Target > 65535 {
			problems = append(problems, fmt.Errorf("port %q must use values between 1 and 65535", port.ID))
		}
		if port.Service != "" && !slices.Contains(serviceIDs(services), port.Service) {
			problems = append(problems, fmt.Errorf("port %q references unknown service %q", port.ID, port.Service))
		}
		if port.Protocol != "tcp" && port.Protocol != "udp" {
			problems = append(problems, fmt.Errorf("port %q protocol must be tcp or udp", port.ID))
		}
	}
	return problems
}

func validateEndpoints(endpoints []Endpoint) []error {
	var problems []error
	for _, endpoint := range endpoints {
		if !strings.Contains(endpoint.URL, "${ports.") {
			if parsed, err := url.ParseRequestURI(endpoint.URL); err != nil || parsed.Scheme == "" {
				problems = append(problems, fmt.Errorf("endpoint %q has invalid URL", endpoint.ID))
			}
		}
	}
	return problems
}

func validateActions(actions []Action) []error {
	var problems []error
	for _, action := range actions {
		if action.Type == "command" && len(action.Command) == 0 {
			problems = append(problems, fmt.Errorf("command action %q requires an argument array", action.ID))
		}
	}
	return problems
}

func serviceIDs(values []Service) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, value.ID)
	}
	return result
}

func portIDs(values []Port) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, value.ID)
	}
	return result
}

func actionIDs(values []Action) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, value.ID)
	}
	return result
}

func endpointIDs(values []Endpoint) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, value.ID)
	}
	return result
}

func uniqueIDs(kind string, values []string) []error {
	seen := make(map[string]struct{}, len(values))
	var problems []error
	for _, value := range values {
		if value == "" {
			problems = append(problems, fmt.Errorf("%s id is required", kind))
			continue
		}
		if _, ok := seen[value]; ok {
			problems = append(problems, fmt.Errorf("duplicate %s id %q", kind, value))
		}
		seen[value] = struct{}{}
	}
	return problems
}
