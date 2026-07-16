package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	catalog "switchyard.dev/switchyard/internal/catalog/application"
	catalogDomain "switchyard.dev/switchyard/internal/catalog/domain"
	"switchyard.dev/switchyard/internal/runtime/domain"
)

// ErrProjectUntrusted prevents repository-derived runtime execution before approval.
var ErrProjectUntrusted = errors.New("project must be trusted before runtime use")

// CatalogSource adapts catalog use cases to the narrow runtime project boundary.
type CatalogSource struct {
	catalog *catalog.Service
}

// NewCatalogSource creates a trusted runtime input adapter.
func NewCatalogSource(service *catalog.Service) *CatalogSource {
	return &CatalogSource{catalog: service}
}

// ResolveRuntime returns only the accepted, effective runtime declaration.
func (s *CatalogSource) ResolveRuntime(ctx context.Context, projectID string) (domain.ProjectRuntime, error) {
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
		Kind: domain.Kind(effective.Manifest.Runtime.Driver), ManifestHash: hex.EncodeToString(digest[:]),
	}
	if compose := effective.Manifest.Runtime.Compose; compose != nil {
		result.Compose = &domain.ComposeRuntime{
			Files: append([]string(nil), compose.Files...), ProjectName: compose.ProjectName, Context: compose.Context,
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
		for _, port := range effective.Manifest.Ports {
			if port.Service == service.ID && port.Protocol == "tcp" {
				declaration.HostPorts = append(declaration.HostPorts, port.Host)
			}
		}
		result.Services = append(result.Services, declaration)
	}
	return result, nil
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
	return ids, nil
}
