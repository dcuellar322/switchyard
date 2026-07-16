package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	catalog "switchyard.dev/switchyard/internal/catalog/application"
	catalogDomain "switchyard.dev/switchyard/internal/catalog/domain"
	"switchyard.dev/switchyard/internal/ports/domain"
)

// CatalogSource resolves accepted manifests into declaration facts without reading live state.
type CatalogSource struct {
	catalog *catalog.Service
	now     func() time.Time
}

// NewCatalogSource adapts accepted manifests to declaration facts.
func NewCatalogSource(service *catalog.Service) *CatalogSource {
	return &CatalogSource{catalog: service, now: time.Now}
}

// Facts returns provenance-bearing declarations from trusted projects.
func (s *CatalogSource) Facts(ctx context.Context) ([]domain.Fact, error) {
	projects, err := s.catalog.ListProjects(ctx)
	if err != nil {
		return nil, err
	}
	var facts []domain.Fact
	for _, project := range projects {
		if project.TrustState != catalogDomain.TrustTrusted {
			continue
		}
		effective, resolveErr := s.catalog.EffectiveManifest(ctx, project.ID, nil)
		if resolveErr != nil {
			return nil, fmt.Errorf("resolve ports for %s: %w", project.ID, resolveErr)
		}
		source := "manifest"
		if effective.Manifest.Runtime.Driver == "compose" {
			source = "compose"
		}
		for _, port := range effective.Manifest.Ports {
			facts = append(facts, domain.Fact{
				ID: stableFactID("declaration", project.ID, port.ID), Kind: domain.KindDeclaration,
				ProjectID: project.ID, ProjectName: project.DisplayName, ServiceID: port.Service, PortID: port.ID,
				Host: "0.0.0.0", Port: port.Host, Target: port.Target, Protocol: port.Protocol,
				Source: source, Evidence: declarationEvidence(source), ObservedAt: s.now().UTC(),
			})
		}
	}
	return facts, nil
}

func stableFactID(parts ...string) string {
	digest := sha256.Sum256([]byte(fmt.Sprint(parts)))
	return "portfact_" + hex.EncodeToString(digest[:12])
}

func declarationEvidence(source string) string {
	if source == "compose" {
		return "accepted manifest derived from Compose published ports"
	}
	return "accepted effective project manifest"
}
