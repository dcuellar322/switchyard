package application_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	agentsAdapters "switchyard.dev/switchyard/internal/agents/adapters"
	agents "switchyard.dev/switchyard/internal/agents/application"
	catalog "switchyard.dev/switchyard/internal/catalog/application"
	discoveryAdapters "switchyard.dev/switchyard/internal/discovery/adapters"
	discoveryDomain "switchyard.dev/switchyard/internal/discovery/domain"
	manifestDomain "switchyard.dev/switchyard/internal/manifest/domain"
	observabilityAdapters "switchyard.dev/switchyard/internal/observability/adapters"
	"switchyard.dev/switchyard/internal/platform/sqlite"
)

func TestAmbiguousFixtureReceivesValidReviewableProviderProposal(t *testing.T) {
	ctx := context.Background()
	database, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "switchyard.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	catalogService := catalog.NewService(sqlite.NewCatalogRepository(database), discoveryAdapters.Defaults())
	fixture, _ := filepath.Abs("../../../test/fixtures/ai-ambiguous-project")
	_, deterministic, err := catalogService.Scan(ctx, fixture)
	if err != nil {
		t.Fatal(err)
	}
	if deterministic.Candidate.Runtime.Driver != "" || len(deterministic.Unresolved) == 0 {
		t.Fatalf("deterministic proposal should remain available and explicit about ambiguity: %#v", deterministic)
	}
	redactor, err := observabilityAdapters.NewRedactor(nil)
	if err != nil {
		t.Fatal(err)
	}
	provider := &bundleAwareProvider{}
	registry, _ := agents.NewRegistry(provider)
	runs := &memoryRuns{}
	service, err := agents.NewEnhancementService(catalogService, runs, agentsAdapters.RepositoryReader{}, redactor, agentsAdapters.ManifestValidator{}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Execute(ctx, "op-fixture", deterministic.ID, "fixture-provider", agents.Limits{}); err != nil {
		t.Fatal(err)
	}
	run, err := service.GetRun(ctx, "op-fixture")
	if err != nil {
		t.Fatal(err)
	}
	if run.State != agents.RunSucceeded || !run.DryRun.Valid || run.ResultProposalID == "" {
		t.Fatalf("run = %#v", run)
	}
	if strings.Contains(string(run.Bundle), "sk-switchyard-fixture-secret-never-send") || strings.Contains(string(run.Bundle), ".env") {
		t.Fatalf("secret material crossed provider boundary: %s", run.Bundle)
	}
	result, err := catalogService.GetProposal(ctx, run.ResultProposalID)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Validation.Valid || len(result.Unresolved) != 0 || result.Candidate.Runtime.Driver != "process" {
		t.Fatalf("result = %#v", result)
	}
	original, err := catalogService.GetProposal(ctx, deterministic.ID)
	if err != nil {
		t.Fatal(err)
	}
	if original.Status != discoveryDomain.StatusSuperseded {
		t.Fatalf("source status = %s", original.Status)
	}
	if _, err := os.Stat("/tmp/switchyard-prompt-injection"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("prompt injection sentinel exists: %v", err)
	}
}

type bundleAwareProvider struct{}

func (*bundleAwareProvider) Descriptor(context.Context) agents.ProviderDescriptor {
	return agents.ProviderDescriptor{ID: "fixture-provider", Name: "Hermetic fixture provider", Kind: "test", Available: true, SupportedBudgetKinds: []string{"evidence_bytes"}}
}
func (*bundleAwareProvider) ProposeManifest(_ context.Context, request agents.ProviderRequest) (agents.ProviderResult, error) {
	var bundle agents.EvidenceBundle
	if err := json.Unmarshal(request.Bundle, &bundle); err != nil {
		return agents.ProviderResult{}, err
	}
	var evidenceID string
	var command []string
	for _, item := range bundle.Evidence {
		if item.Kind != "node.script" {
			continue
		}
		var data struct {
			Script  string   `json:"script"`
			Command []string `json:"command"`
		}
		if json.Unmarshal(item.Data, &data) == nil && data.Script == "dev" {
			evidenceID, command = item.ID, data.Command
			break
		}
	}
	if evidenceID == "" {
		return agents.ProviderResult{}, errors.New("fixture dev evidence missing")
	}
	candidate := bundle.Candidate
	candidate.Runtime = manifestDomain.Runtime{Driver: "process", Process: &manifestDomain.ProcessConfig{Processes: []manifestDomain.ProcessDefinition{{ID: "web", Command: command, WorkingDirectory: "."}}}}
	candidate.Services = []manifestDomain.Service{{ID: "web", DisplayName: "Web", Source: manifestDomain.ServiceSource{Process: "web"}, Dependencies: []string{}, HealthChecks: []manifestDomain.HealthCheck{}}}
	output, err := json.Marshal(agents.ProposalOutput{Version: agents.OutputVersion, Candidate: candidate, Claims: []agents.FieldClaim{{Path: "/runtime", EvidenceIDs: []string{evidenceID}, Rationale: "Node dev script"}, {Path: "/services", EvidenceIDs: []string{evidenceID}, Rationale: "Node process"}}, Warnings: []string{}})
	return agents.ProviderResult{Output: output, Model: "hermetic"}, err
}
func (p *bundleAwareProvider) Diagnose(ctx context.Context, request agents.ProviderRequest) (agents.ProviderResult, error) {
	return p.ProposeManifest(ctx, request)
}

type memoryRuns struct{ value agents.Run }

func (r *memoryRuns) Start(_ context.Context, run agents.Run) error  { r.value = run; return nil }
func (r *memoryRuns) Finish(_ context.Context, run agents.Run) error { r.value = run; return nil }
func (r *memoryRuns) Get(_ context.Context, id string) (agents.Run, error) {
	if r.value.OperationID != id {
		return agents.Run{}, agents.ErrRunNotFound
	}
	return r.value, nil
}
