package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	catalog "switchyard.dev/switchyard/internal/catalog/application"
	catalogDomain "switchyard.dev/switchyard/internal/catalog/domain"
	"switchyard.dev/switchyard/internal/runtime/domain"
)

// ErrProjectUntrusted prevents repository-derived runtime execution before approval.
var ErrProjectUntrusted = errors.New("project must be trusted before runtime use")

// CatalogSource adapts catalog use cases to the narrow runtime project boundary.
type CatalogSource struct {
	catalog      *catalog.Service
	environments EnvironmentSource
}

// RuntimeEnvironment is the narrow worktree overlay consumed by runtime
// resolution. The accepted catalog manifest remains the source of behavior.
type RuntimeEnvironment struct {
	ID                 string
	ProjectID          string
	Name               string
	Root               string
	Available          bool
	ComposeProjectName string
	PortLeases         map[string]int
}

// EnvironmentSource resolves registered worktrees without exposing their storage.
type EnvironmentSource interface {
	ResolveRuntimeEnvironment(context.Context, string) (RuntimeEnvironment, error)
	ListRuntimeEnvironmentIDs(context.Context) ([]string, error)
}

// NewCatalogSource creates a trusted runtime input adapter.
func NewCatalogSource(service *catalog.Service, environments ...EnvironmentSource) *CatalogSource {
	result := &CatalogSource{catalog: service}
	if len(environments) > 0 {
		result.environments = environments[0]
	}
	return result
}

// ResolveRuntime returns only the accepted, effective runtime declaration.
func (s *CatalogSource) ResolveRuntime(ctx context.Context, projectID string) (domain.ProjectRuntime, error) {
	requestedID := projectID
	projectID, environment, err := s.resolveEnvironment(ctx, projectID)
	if err != nil {
		return domain.ProjectRuntime{}, err
	}
	project, err := s.catalog.GetProject(ctx, projectID)
	if err != nil {
		return domain.ProjectRuntime{}, err
	}
	if project.TrustState != catalogDomain.TrustTrusted {
		return domain.ProjectRuntime{}, ErrProjectUntrusted
	}
	effective, err := s.catalog.EffectiveManifest(ctx, projectID, nil)
	if err != nil {
		return domain.ProjectRuntime{}, err
	}
	document, err := json.Marshal(effective.Manifest)
	if err != nil {
		return domain.ProjectRuntime{}, fmt.Errorf("encode effective manifest identity: %w", err)
	}
	digest := sha256.Sum256(document)
	result := domain.ProjectRuntime{
		ProjectID: project.ID, ProjectSlug: project.Slug, Root: project.PrimaryLocation,
		Kind: domain.Kind(effective.Manifest.Runtime.Driver), ManifestHash: hex.EncodeToString(digest[:]), Ports: map[string]domain.PortDeclaration{},
	}
	if compose := effective.Manifest.Runtime.Compose; compose != nil {
		result.Compose = &domain.ComposeRuntime{
			Files: append([]string(nil), compose.Files...), ProjectName: compose.ProjectName, Context: compose.Context,
			Profiles: append([]string(nil), compose.Profiles...),
		}
	}
	if process := effective.Manifest.Runtime.Process; process != nil {
		result.Process = &domain.ProcessRuntime{
			Environment: cloneStringMap(process.Environment),
			Secrets:     make(map[string]domain.SecretReference, len(process.Secrets)),
		}
		for key, reference := range process.Secrets {
			result.Process.Secrets[key] = domain.SecretReference{Provider: reference.Provider, Key: reference.Key, Account: reference.Account}
		}
		for _, definition := range process.Processes {
			resolved := domain.ProcessDefinition{
				ID: definition.ID, Command: append([]string(nil), definition.Command...), WorkingDirectory: definition.WorkingDirectory,
				Shell: definition.Shell, Environment: cloneStringMap(definition.Environment),
				Secrets:            make(map[string]domain.SecretReference, len(definition.Secrets)),
				Restart:            domain.RestartPolicy{Mode: definition.Restart.Mode, MaxRetries: definition.Restart.MaxRetries, BackoffSeconds: definition.Restart.BackoffSeconds},
				StopTimeoutSeconds: definition.StopTimeoutSeconds,
			}
			for key, reference := range definition.Secrets {
				resolved.Secrets[key] = domain.SecretReference{Provider: reference.Provider, Key: reference.Key, Account: reference.Account}
			}
			result.Process.Processes = append(result.Process.Processes, resolved)
		}
	}
	for _, service := range effective.Manifest.Services {
		runtimeName := service.Source.ComposeService
		if runtimeName == "" {
			runtimeName = service.Source.Process
		}
		declaration := domain.ServiceDeclaration{
			ID: service.ID, RuntimeName: runtimeName, Dependencies: append([]string(nil), service.Dependencies...),
		}
		for index, check := range service.HealthChecks {
			id := check.ID
			if id == "" {
				id = fmt.Sprintf("%s-%d", check.Type, index+1)
			}
			declaration.HealthChecks = append(declaration.HealthChecks, domain.HealthCheckDefinition{
				ID: id, ServiceID: service.ID, Type: check.Type, URL: check.URL, Address: check.Address,
				ExpectedStatus: check.ExpectedStatus, JSONPath: check.JSONPath, ExpectedValue: check.ExpectedValue,
				Command: append([]string(nil), check.Command...), Members: append([]string(nil), check.Members...), Mode: check.Mode,
				InitialDelaySeconds: check.InitialDelaySeconds, IntervalSeconds: check.IntervalSeconds,
				TimeoutSeconds: check.TimeoutSeconds, Retries: check.Retries, Severity: check.Severity, Required: check.Required,
			})
		}
		for _, port := range effective.Manifest.Ports {
			result.Ports[port.ID] = domain.PortDeclaration{ID: port.ID, Service: port.Service, Host: port.Host, Target: port.Target, Protocol: port.Protocol}
			if port.Service == service.ID && port.Protocol == "tcp" {
				declaration.HostPorts = append(declaration.HostPorts, port.Host)
			}
		}
		result.Services = append(result.Services, declaration)
	}
	if environment.ID != "" {
		applyRuntimeEnvironment(&result, requestedID, environment)
	}
	return result, nil
}

func (s *CatalogSource) resolveEnvironment(ctx context.Context, requestedID string) (string, RuntimeEnvironment, error) {
	if s.environments == nil || !strings.HasPrefix(requestedID, "env-") {
		return requestedID, RuntimeEnvironment{}, nil
	}
	environment, err := s.environments.ResolveRuntimeEnvironment(ctx, requestedID)
	if err != nil {
		return "", RuntimeEnvironment{}, err
	}
	if !environment.Available {
		return "", RuntimeEnvironment{}, fmt.Errorf("project environment %s is unavailable", requestedID)
	}
	return environment.ProjectID, environment, nil
}

func applyRuntimeEnvironment(result *domain.ProjectRuntime, requestedID string, environment RuntimeEnvironment) {
	result.ProjectID = requestedID
	result.ProjectSlug += "-" + environment.Name
	result.Root = environment.Root
	if result.Compose != nil {
		result.Compose.ProjectName = environment.ComposeProjectName
		result.Compose.PortOverrides = cloneIntMap(environment.PortLeases)
	}
	for id, host := range environment.PortLeases {
		port, exists := result.Ports[id]
		if !exists {
			continue
		}
		port.Host = host
		result.Ports[id] = port
	}
	for index := range result.Services {
		result.Services[index].HostPorts = result.Services[index].HostPorts[:0]
		for _, port := range result.Ports {
			if port.Service == result.Services[index].ID && port.Protocol == "tcp" {
				result.Services[index].HostPorts = append(result.Services[index].HostPorts, port.Host)
			}
		}
	}
}

func cloneIntMap(source map[string]int) map[string]int {
	result := make(map[string]int, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func cloneStringMap(source map[string]string) map[string]string {
	result := make(map[string]string, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

// ListRuntimeProjectIDs lists trusted projects eligible for event subscriptions.
func (s *CatalogSource) ListRuntimeProjectIDs(ctx context.Context) ([]string, error) {
	projects, err := s.catalog.ListProjects(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(projects))
	for _, project := range projects {
		if project.TrustState == catalogDomain.TrustTrusted {
			ids = append(ids, project.ID)
		}
	}
	if s.environments != nil {
		environments, environmentErr := s.environments.ListRuntimeEnvironmentIDs(ctx)
		if environmentErr != nil {
			return nil, environmentErr
		}
		ids = append(ids, environments...)
	}
	return ids, nil
}
