package application_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
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
	if duplicateProject.ID != project.ID || duplicateProposal.ID == proposal.ID {
		t.Fatal("pending rescan did not replace the existing proposal")
	}
	superseded, err := service.GetProposal(ctx, proposal.ID)
	if err != nil || superseded.Status != discoveryDomain.StatusSuperseded {
		t.Fatalf("superseded proposal = %#v error=%v", superseded, err)
	}
	trusted, accepted, err := service.TrustProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("Accept() error = %v", err)
	}
	if trusted.TrustState != catalogDomain.TrustTrusted || trusted.ManifestRevision != 1 || accepted.Status != discoveryDomain.StatusAccepted {
		t.Fatalf("unexpected accepted aggregate: %#v %#v", trusted, accepted)
	}
	retrusted, repeated, err := service.TrustProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("repeat TrustProject() error = %v", err)
	}
	if retrusted.ManifestRevision != 1 || repeated.ID != accepted.ID || repeated.Status != discoveryDomain.StatusAccepted {
		t.Fatalf("repeat trust changed aggregate: %#v %#v", retrusted, repeated)
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

func TestPendingScanRefreshesProposalFromPortableManifest(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "switchyard.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	service := application.NewService(sqlite.NewCatalogRepository(database), adapters.Defaults())
	root := t.TempDir()
	project, source, err := service.Scan(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(source.Unresolved) == 0 {
		t.Fatal("initial proposal unexpectedly resolved")
	}
	manifestDirectory := filepath.Join(root, ".switchyard")
	if err := os.MkdirAll(manifestDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	portable := `schemaVersion: switchyard.dev/v1
kind: Project
metadata:
  id: refreshed-project
  name: Refreshed project
  tags: [go, process]
repository:
  root: .
runtime:
  driver: process
  process:
    processes:
      - id: daemon
        command: [go, version]
        workingDirectory: .
services:
  - id: daemon
    source:
      process: daemon
`
	if err := os.WriteFile(filepath.Join(manifestDirectory, "project.yml"), []byte(portable), 0o600); err != nil {
		t.Fatal(err)
	}
	refreshedProject, refreshed, err := service.Scan(ctx, root)
	if err != nil {
		t.Fatalf("rescan error = %v", err)
	}
	if refreshedProject.ID != project.ID || refreshed.ID == source.ID {
		t.Fatalf("rescan identities project=%q proposal=%q", refreshedProject.ID, refreshed.ID)
	}
	if refreshedProject.Slug != "refreshed-project" || refreshedProject.DisplayName != "Refreshed project" {
		t.Fatalf("refreshed project metadata = %#v", refreshedProject)
	}
	if !refreshed.Validation.Valid || len(refreshed.Unresolved) != 0 {
		t.Fatalf("refreshed proposal validation=%#v unresolved=%v", refreshed.Validation, refreshed.Unresolved)
	}
	previous, err := service.GetProposal(ctx, source.ID)
	if err != nil || previous.Status != discoveryDomain.StatusSuperseded {
		t.Fatalf("previous proposal = %#v error=%v", previous, err)
	}
}

func TestTrustedProjectRescanCreatesReviewableRevision(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "switchyard.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	service := application.NewService(sqlite.NewCatalogRepository(database), adapters.Defaults())
	root := t.TempDir()
	composePath := filepath.Join(root, "compose.yaml")
	writeCompose := func(port string) {
		t.Helper()
		contents := "services:\n  frontend:\n    image: example/frontend\n    ports:\n      - \"" + port + ":5173\"\n"
		if err := os.WriteFile(composePath, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	writeCompose("8080")
	project, first, err := service.Scan(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	trusted, _, err := service.TrustProject(ctx, project.ID)
	if err != nil || trusted.ManifestRevision != 1 {
		t.Fatalf("initial trust project=%#v error=%v", trusted, err)
	}

	writeCompose("8181")
	rescannedProject, rescanned, err := service.Scan(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if rescannedProject.TrustState != catalogDomain.TrustTrusted || rescannedProject.ManifestRevision != 1 {
		t.Fatalf("rescan changed current trust = %#v", rescannedProject)
	}
	if rescanned.ID == first.ID || rescanned.Status != discoveryDomain.StatusProposed || rescanned.Candidate.Ports[0].Host != 8181 {
		t.Fatalf("rescan proposal = %#v", rescanned)
	}
	current, err := service.EffectiveManifest(ctx, project.ID, nil)
	if err != nil || current.Manifest.Ports[0].Host != 8080 {
		t.Fatalf("rescan changed accepted manifest = %#v error=%v", current.Manifest, err)
	}
	updated, _, err := service.TrustProject(ctx, project.ID)
	if err != nil || updated.ManifestRevision != 2 {
		t.Fatalf("revision trust project=%#v error=%v", updated, err)
	}
	effective, err := service.EffectiveManifest(ctx, project.ID, nil)
	if err != nil || effective.Manifest.Ports[0].Host != 8181 {
		t.Fatalf("accepted revision = %#v error=%v", effective.Manifest, err)
	}
}

func TestTrustReportsUnresolvedProposalFields(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "switchyard.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	service := application.NewService(sqlite.NewCatalogRepository(database), adapters.Defaults())
	project, _, err := service.Scan(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = service.TrustProject(ctx, project.ID)
	if !errors.Is(err, application.ErrInvalidProposal) {
		t.Fatalf("TrustProject() error = %v", err)
	}
	if !strings.Contains(err.Error(), "unresolved fields: /runtime/driver, /services") || strings.Contains(err.Error(), "[]") {
		t.Fatalf("TrustProject() error is not actionable: %v", err)
	}
}

func TestAssistedProposalRequiresHumanApproval(t *testing.T) {
	ctx := context.Background()
	database, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "switchyard.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	service := application.NewService(sqlite.NewCatalogRepository(database), adapters.Defaults())
	fixture, _ := filepath.Abs("../../../test/fixtures/mixed-project")
	_, proposal, err := service.Scan(ctx, fixture)
	if err != nil {
		t.Fatal(err)
	}
	revision, err := service.CreateRevisionAs(ctx, proposal.ID, proposal.Candidate, proposal.ConfidenceByField, proposal.Unresolved, "ai-provider", "fixture")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.AcceptAs(ctx, revision.ID, application.MutationActor{Type: "agent", ID: "fixture/agent"}); !errors.Is(err, application.ErrHumanApprovalRequired) {
		t.Fatalf("agent acceptance error = %v", err)
	}
	if _, accepted, err := service.AcceptAs(ctx, revision.ID, application.MutationActor{Type: "browser", ID: "human-session"}); err != nil || accepted.Status != discoveryDomain.StatusAccepted {
		t.Fatalf("human acceptance = %#v error=%v", accepted, err)
	}
}
