package application_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"switchyard.dev/switchyard/internal/catalog/application"
	catalogDomain "switchyard.dev/switchyard/internal/catalog/domain"
	"switchyard.dev/switchyard/internal/discovery/adapters"
	discoveryDomain "switchyard.dev/switchyard/internal/discovery/domain"
	"switchyard.dev/switchyard/internal/platform/sqlite"
)

func TestScanReviewAcceptAndResolve(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "switchyard.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Error(err)
		}
	})
	service := application.NewService(sqlite.NewCatalogRepository(database), adapters.Defaults())
	fixture, err := filepath.Abs("../../../test/fixtures/mixed-project")
	if err != nil {
		t.Fatal(err)
	}

	project, proposal, err := service.Scan(ctx, fixture)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if project.TrustState != catalogDomain.TrustPending || proposal.Status != discoveryDomain.StatusProposed {
		t.Fatalf("unexpected pending aggregate: %#v %#v", project, proposal)
	}
	if !proposal.Validation.Valid {
		t.Fatalf("proposal validation = %#v", proposal.Validation)
	}
	duplicateProject, duplicateProposal, err := service.Scan(ctx, fixture)
	if err != nil {
		t.Fatalf("duplicate Scan() error = %v", err)
	}
	if duplicateProject.ID != project.ID || duplicateProposal.ID != proposal.ID {
		t.Fatal("duplicate scan created a competing proposal")
	}
	trusted, accepted, err := service.TrustProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("Accept() error = %v", err)
	}
	if trusted.TrustState != catalogDomain.TrustTrusted || trusted.ManifestRevision != 1 || accepted.Status != discoveryDomain.StatusAccepted {
		t.Fatalf("unexpected accepted aggregate: %#v %#v", trusted, accepted)
	}
	effective, err := service.EffectiveManifest(ctx, project.ID, nil)
	if err != nil {
		t.Fatalf("EffectiveManifest() error = %v", err)
	}
	if effective.Manifest.Metadata.Name != "Switchyard Mixed Fixture" {
		t.Fatalf("effective name = %q", effective.Manifest.Metadata.Name)
	}
	if err := service.RemoveProject(ctx, project.ID); err != nil {
		t.Fatalf("RemoveProject() error = %v", err)
	}
	if _, err := service.GetProject(ctx, project.ID); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("GetProject() after removal error = %v", err)
	}
}
