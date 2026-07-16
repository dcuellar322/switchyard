// Package domain owns the canonical project manifest model and invariants.
package domain

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
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
	Process *ProcessConfig `json:"process,omitempty" yaml:"process,omitempty"`
}

// ComposeConfig identifies portable Compose inputs.
type ComposeConfig struct {
	Files       []string `json:"files" yaml:"files" jsonschema:"required,minItems=1"`
	ProjectName string   `json:"projectName,omitempty" yaml:"projectName,omitempty"`
	Context     string   `json:"context,omitempty" yaml:"context,omitempty"`
}

// ProcessConfig declares project-wide environment and native process definitions.
type ProcessConfig struct {
	Environment map[string]string    `json:"environment,omitempty" yaml:"environment,omitempty"`
	Secrets     map[string]SecretRef `json:"secrets,omitempty" yaml:"secrets,omitempty"`
	Processes   []ProcessDefinition  `json:"processes" yaml:"processes" jsonschema:"required,minItems=1"`
}

// ProcessDefinition is one shell-free native service command.
type ProcessDefinition struct {
	ID                 string               `json:"id" yaml:"id" jsonschema:"required"`
	Command            []string             `json:"command" yaml:"command" jsonschema:"required,minItems=1"`
	WorkingDirectory   string               `json:"workingDirectory,omitempty" yaml:"workingDirectory,omitempty"`
	Shell              bool                 `json:"shell,omitempty" yaml:"shell,omitempty"`
	Environment        map[string]string    `json:"environment,omitempty" yaml:"environment,omitempty"`
	Secrets            map[string]SecretRef `json:"secrets,omitempty" yaml:"secrets,omitempty"`
	Restart            RestartPolicy        `json:"restart,omitempty" yaml:"restart,omitempty"`
	StopTimeoutSeconds int                  `json:"stopTimeoutSeconds,omitempty" yaml:"stopTimeoutSeconds,omitempty" jsonschema:"minimum=0,maximum=300"`
}

// SecretRef identifies a value in an operating-system credential store without embedding it.
type SecretRef struct {
	Provider string `json:"provider" yaml:"provider" jsonschema:"required,enum=keychain"`
	Key      string `json:"key" yaml:"key" jsonschema:"required"`
	Account  string `json:"account,omitempty" yaml:"account,omitempty"`
}

// RestartPolicy enables bounded crash restart only when explicitly requested.
type RestartPolicy struct {
	Mode           string `json:"mode,omitempty" yaml:"mode,omitempty" jsonschema:"enum=never,enum=on-failure"`
	MaxRetries     int    `json:"maxRetries,omitempty" yaml:"maxRetries,omitempty" jsonschema:"minimum=0,maximum=20"`
	BackoffSeconds int    `json:"backoffSeconds,omitempty" yaml:"backoffSeconds,omitempty" jsonschema:"minimum=0,maximum=300"`
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
	ID                  string   `json:"id,omitempty" yaml:"id,omitempty"`
	Type                string   `json:"type" yaml:"type" jsonschema:"required,enum=http,enum=tcp,enum=process,enum=docker,enum=command,enum=composite"`
	URL                 string   `json:"url,omitempty" yaml:"url,omitempty"`
	Address             string   `json:"address,omitempty" yaml:"address,omitempty"`
	ExpectedStatus      int      `json:"expectedStatus,omitempty" yaml:"expectedStatus,omitempty" jsonschema:"minimum=0,maximum=599"`
	JSONPath            string   `json:"jsonPath,omitempty" yaml:"jsonPath,omitempty"`
	ExpectedValue       string   `json:"expectedValue,omitempty" yaml:"expectedValue,omitempty"`
	Command             []string `json:"command,omitempty" yaml:"command,omitempty"`
	Members             []string `json:"members,omitempty" yaml:"members,omitempty"`
	Mode                string   `json:"mode,omitempty" yaml:"mode,omitempty" jsonschema:"enum=all,enum=any"`
	InitialDelaySeconds int      `json:"initialDelaySeconds,omitempty" yaml:"initialDelaySeconds,omitempty" jsonschema:"minimum=0,maximum=3600"`
	IntervalSeconds     int      `json:"intervalSeconds,omitempty" yaml:"intervalSeconds,omitempty" jsonschema:"minimum=0,maximum=3600"`
	TimeoutSeconds      int      `json:"timeoutSeconds,omitempty" yaml:"timeoutSeconds,omitempty" jsonschema:"minimum=0,maximum=300"`
	Retries             int      `json:"retries,omitempty" yaml:"retries,omitempty" jsonschema:"minimum=0,maximum=20"`
	Severity            string   `json:"severity,omitempty" yaml:"severity,omitempty" jsonschema:"enum=info,enum=warning,enum=critical"`
	Required            bool     `json:"required,omitempty" yaml:"required,omitempty"`
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
	ID               string            `json:"id" yaml:"id" jsonschema:"required"`
	Name             string            `json:"name" yaml:"name" jsonschema:"required"`
	Type             string            `json:"type" yaml:"type" jsonschema:"required,enum=terminal.open,enum=editor.open,enum=browser.open,enum=command,enum=command.run,enum=agent.start,enum=git.fetch,enum=git.pull,enum=git.push,enum=tests.run,enum=migration.run"`
	Command          []string          `json:"command,omitempty" yaml:"command,omitempty"`
	WorkingDirectory string            `json:"workingDirectory,omitempty" yaml:"workingDirectory,omitempty"`
	Shell            bool              `json:"shell,omitempty" yaml:"shell,omitempty"`
	CaptureOutput    bool              `json:"captureOutput,omitempty" yaml:"captureOutput,omitempty"`
	Provider         string            `json:"provider,omitempty" yaml:"provider,omitempty"`
	Target           string            `json:"target,omitempty" yaml:"target,omitempty"`
	Risk             string            `json:"risk,omitempty" yaml:"risk,omitempty" jsonschema:"enum=read_only,enum=mutating,enum=networked,enum=destructive,enum=interactive"`
	TimeoutSeconds   int               `json:"timeoutSeconds,omitempty" yaml:"timeoutSeconds,omitempty" jsonschema:"minimum=0,maximum=86400"`
	Environment      map[string]string `json:"environment,omitempty" yaml:"environment,omitempty"`
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
	problems = append(problems, validateProcessBindings(m.Runtime, m.Services)...)
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
		problems = append(problems, validateServiceHealth(service)...)
	}
	return problems
}

func validateServiceHealth(service Service) []error {
	var problems []error
	checkIDs := make(map[string]struct{}, len(service.HealthChecks))
	for index, check := range service.HealthChecks {
		checkID := healthCheckID(check, index)
		if _, exists := checkIDs[checkID]; exists {
			problems = append(problems, fmt.Errorf("service %q has duplicate health check ID %q", service.ID, checkID))
		}
		checkIDs[checkID] = struct{}{}
		problems = append(problems, validateHealthCheck(service.ID, checkID, check)...)
	}
	for index, check := range service.HealthChecks {
		if check.Type != "composite" {
			continue
		}
		checkID := healthCheckID(check, index)
		for _, member := range check.Members {
			if member == checkID {
				problems = append(problems, fmt.Errorf("service %q composite health check %q cannot reference itself", service.ID, checkID))
			} else if _, exists := checkIDs[member]; !exists {
				problems = append(problems, fmt.Errorf("service %q composite health check %q references unknown member %q", service.ID, checkID, member))
			}
		}
	}
	return append(problems, validateCompositeCycles(service)...)
}

func validateHealthCheck(serviceID, checkID string, check HealthCheck) []error {
	var problems []error
	if check.Severity != "" && check.Severity != "info" && check.Severity != "warning" && check.Severity != "critical" {
		problems = append(problems, fmt.Errorf("service %q health check %q has invalid severity %q", serviceID, checkID, check.Severity))
	}
	switch check.Type {
	case "http":
		if check.URL == "" {
			problems = append(problems, fmt.Errorf("service %q HTTP health check requires a URL", serviceID))
		}
	case "tcp":
		if check.Address == "" {
			problems = append(problems, fmt.Errorf("service %q TCP health check requires an address", serviceID))
		}
	case "process", "docker":
	case "command":
		if len(check.Command) == 0 {
			problems = append(problems, fmt.Errorf("service %q command health check requires an argument array", serviceID))
		}
	case "composite":
		if len(check.Members) == 0 {
			problems = append(problems, fmt.Errorf("service %q composite health check requires members", serviceID))
		}
		if check.Mode != "" && check.Mode != "all" && check.Mode != "any" {
			problems = append(problems, fmt.Errorf("service %q composite health check mode must be all or any", serviceID))
		}
	default:
		problems = append(problems, fmt.Errorf("service %q has unknown health check type %q", serviceID, check.Type))
	}
	return problems
}

func healthCheckID(check HealthCheck, index int) string {
	if check.ID != "" {
		return check.ID
	}
	return fmt.Sprintf("%s-%d", check.Type, index+1)
}

func validateCompositeCycles(service Service) []error {
	graph := make(map[string][]string)
	for index, check := range service.HealthChecks {
		if check.Type != "composite" {
			continue
		}
		id := healthCheckID(check, index)
		graph[id] = append([]string(nil), check.Members...)
	}
	state := make(map[string]uint8, len(graph))
	var problems []error
	var visit func(string)
	visit = func(id string) {
		if state[id] == 1 {
			problems = append(problems, fmt.Errorf("service %q composite health checks contain a cycle at %q", service.ID, id))
			return
		}
		if state[id] == 2 {
			return
		}
		state[id] = 1
		for _, member := range graph[id] {
			if _, composite := graph[member]; composite {
				visit(member)
			}
		}
		state[id] = 2
	}
	for id := range graph {
		visit(id)
	}
	return problems
}

func validateRuntime(runtime Runtime) []error {
	var problems []error
	if runtime.Driver == "compose" && (runtime.Compose == nil || len(runtime.Compose.Files) == 0) {
		problems = append(problems, errors.New("compose runtime requires at least one Compose file"))
	}
	if runtime.Driver == "process" && (runtime.Process == nil || len(runtime.Process.Processes) == 0) {
		problems = append(problems, errors.New("process runtime requires at least one process definition"))
	}
	if runtime.Compose != nil && runtime.Process != nil {
		problems = append(problems, errors.New("runtime cannot declare both compose and process configuration"))
	}
	if runtime.Process != nil {
		problems = append(problems, validateProcesses(*runtime.Process)...)
	}
	return problems
}

func validateProcesses(config ProcessConfig) []error {
	var problems []error
	processIDs := make([]string, 0, len(config.Processes))
	for _, process := range config.Processes {
		processIDs = append(processIDs, process.ID)
		if len(process.Command) == 0 || strings.TrimSpace(process.Command[0]) == "" {
			problems = append(problems, fmt.Errorf("process %q requires a command argument array", process.ID))
		}
		if !process.Shell && usesShellExecutable(process.Command) {
			problems = append(problems, fmt.Errorf("process %q uses shell syntax without shell: true", process.ID))
		}
		if process.Shell && len(process.Command) != 1 {
			problems = append(problems, fmt.Errorf("shell process %q must provide exactly one command string", process.ID))
		}
		if process.Restart.Mode != "" && process.Restart.Mode != "never" && process.Restart.Mode != "on-failure" {
			problems = append(problems, fmt.Errorf("process %q has unknown restart mode %q", process.ID, process.Restart.Mode))
		}
		problems = append(problems, validateEnvironment("process "+process.ID, process.Environment, process.Secrets)...)
	}
	problems = append(problems, uniqueIDs("process", processIDs)...)
	problems = append(problems, validateEnvironment("process runtime", config.Environment, config.Secrets)...)
	return problems
}

func validateProcessBindings(runtime Runtime, services []Service) []error {
	var problems []error
	for _, service := range services {
		if runtime.Driver == "process" && service.Source.Process == "" {
			problems = append(problems, fmt.Errorf("process runtime service %q must reference a process definition", service.ID))
		}
		if runtime.Driver == "compose" && service.Source.ComposeService == "" {
			problems = append(problems, fmt.Errorf("compose runtime service %q must reference a Compose service", service.ID))
		}
	}
	if runtime.Process == nil {
		return problems
	}
	definitions := make(map[string]struct{}, len(runtime.Process.Processes))
	for _, process := range runtime.Process.Processes {
		definitions[process.ID] = struct{}{}
	}
	for _, service := range services {
		if service.Source.Process == "" {
			continue
		}
		if _, ok := definitions[service.Source.Process]; !ok {
			problems = append(problems, fmt.Errorf("service %q references unknown process %q", service.ID, service.Source.Process))
		}
	}
	if cycle := dependencyCycle(services); len(cycle) > 0 {
		problems = append(problems, fmt.Errorf("service dependency cycle: %s", strings.Join(cycle, " -> ")))
	}
	return problems
}

func dependencyCycle(services []Service) []string {
	dependencies := make(map[string][]string, len(services))
	for _, service := range services {
		dependencies[service.ID] = service.Dependencies
	}
	state := make(map[string]uint8, len(services))
	var stack []string
	var visit func(string) []string
	visit = func(id string) []string {
		if state[id] == 1 {
			for index, item := range stack {
				if item == id {
					return append(append([]string(nil), stack[index:]...), id)
				}
			}
		}
		if state[id] == 2 {
			return nil
		}
		state[id] = 1
		stack = append(stack, id)
		for _, dependency := range dependencies[id] {
			if cycle := visit(dependency); len(cycle) > 0 {
				return cycle
			}
		}
		stack = stack[:len(stack)-1]
		state[id] = 2
		return nil
	}
	for _, service := range services {
		if cycle := visit(service.ID); len(cycle) > 0 {
			return cycle
		}
	}
	return nil
}

func usesShellExecutable(command []string) bool {
	if len(command) == 0 || strings.ContainsAny(command[0], " \t\r\n|&;<>()$`") {
		return true
	}
	base := strings.ToLower(strings.TrimSuffix(filepath.Base(command[0]), ".exe"))
	return slices.Contains([]string{"sh", "bash", "zsh", "fish", "cmd", "powershell", "pwsh"}, base)
}

var environmentName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func validateEnvironment(owner string, values map[string]string, secrets map[string]SecretRef) []error {
	var problems []error
	for key := range values {
		if !environmentName.MatchString(key) {
			problems = append(problems, fmt.Errorf("%s has invalid environment name %q", owner, key))
		}
		if _, exists := secrets[key]; exists {
			problems = append(problems, fmt.Errorf("%s environment %q cannot be both a value and secret reference", owner, key))
		}
	}
	for key, reference := range secrets {
		if !environmentName.MatchString(key) {
			problems = append(problems, fmt.Errorf("%s has invalid secret environment name %q", owner, key))
		}
		if reference.Provider != "keychain" || strings.TrimSpace(reference.Key) == "" {
			problems = append(problems, fmt.Errorf("%s secret %q requires a keychain key", owner, key))
		}
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
		if (action.Type == "command" || action.Type == "command.run" || action.Type == "tests.run" || action.Type == "migration.run") && len(action.Command) == 0 {
			problems = append(problems, fmt.Errorf("command action %q requires an argument array", action.ID))
		}
		if action.Shell && len(action.Command) != 1 {
			problems = append(problems, fmt.Errorf("shell action %q requires exactly one command string", action.ID))
		}
		if action.Type == "agent.start" && action.Provider == "" {
			problems = append(problems, fmt.Errorf("agent action %q requires a provider", action.ID))
		}
		if action.Type == "browser.open" && action.Target == "" {
			problems = append(problems, fmt.Errorf("browser action %q requires a target", action.ID))
		}
		problems = append(problems, validateEnvironment("action "+action.ID, action.Environment, nil)...)
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
