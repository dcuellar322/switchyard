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
	for _, service := range effective.Manifest.Services {
		result.Services = append(result.Services, domain.ServiceDeclaration{ID: service.ID, RuntimeName: service.Source.ComposeService})
	}
	return result, nil
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
