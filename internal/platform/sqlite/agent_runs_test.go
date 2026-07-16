package sqlite

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	agents "switchyard.dev/switchyard/internal/agents/application"
)

func TestAgentRunRepositoryPersistsExactReceiptAndTerminalReview(t *testing.T) {
	ctx := context.Background()
	database, err := Open(ctx, filepath.Join(t.TempDir(), "switchyard.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	now := time.Now().UTC().Truncate(time.Millisecond)
	_, err = database.connection.ExecContext(ctx, `INSERT INTO projects
		(id, slug, display_name, description, trust_state, primary_location, manifest_revision, created_at, updated_at)
		VALUES ('project-1','project-1','Project 1','','pending','/tmp/project-1',0,?,?)`, formatTime(now), formatTime(now))
	if err != nil {
		t.Fatal(err)
	}
	_, err = database.connection.ExecContext(ctx, `INSERT INTO manifest_proposals
		(id, project_id, scanner_version, schema_version, candidate_json, confidence_json, unresolved_json, validation_json, status, created_at)
		VALUES ('proposal-1','project-1','deterministic/v1','switchyard.dev/v1alpha1','{}','{}','[]','{}','proposed',?)`, formatTime(now))
	if err != nil {
		t.Fatal(err)
	}
	_, err = database.connection.ExecContext(ctx, `INSERT INTO operations
		(id, project_id, kind, state, idempotency_key, input_json, requested_at, updated_at)
		VALUES ('op-1','project-1','manifest.enhance','running','fixture-key','{}',?,?)`, formatTime(now), formatTime(now))
	if err != nil {
		t.Fatal(err)
	}
	repository := NewAgentRunRepository(database)
	bundle := json.RawMessage(`{"evidence":[{"id":"ev-1"}]}`)
	run := agents.Run{OperationID: "op-1", ProjectID: "project-1", SourceProposalID: "proposal-1", Provider: "fixture", State: agents.RunRunning, Bundle: bundle, BundleSHA256: "abc", Limits: agents.Limits{TimeoutSeconds: 90}, Fields: []agents.FieldReview{}, Conflicts: []agents.Conflict{}, Warnings: []string{}, DryRun: agents.DryRun{Errors: []string{}, Warnings: []string{}}, StartedAt: now}
	if err := repository.Start(ctx, run); err != nil {
		t.Fatal(err)
	}
	loaded, err := repository.Get(ctx, "op-1")
	if err != nil {
		t.Fatal(err)
	}
	if string(loaded.Bundle) != string(bundle) || loaded.State != agents.RunRunning {
		t.Fatalf("loaded = %#v", loaded)
	}
	finished := now.Add(time.Second)
	run.State, run.FinishedAt, run.Fields = agents.RunSucceeded, &finished, []agents.FieldReview{{Path: "/runtime", Source: "ai", Confidence: .79, EvidenceIDs: []string{"ev-1"}, Warnings: []string{}}}
	run.DryRun = agents.DryRun{Valid: true, SchemaValid: true, EvidenceBacked: true, RepositorySafe: true, Errors: []string{}, Warnings: []string{}}
	if err := repository.Finish(ctx, run); err != nil {
		t.Fatal(err)
	}
	loaded, err = repository.Get(ctx, "op-1")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.State != agents.RunSucceeded || !loaded.DryRun.Valid || len(loaded.Fields) != 1 || loaded.FinishedAt == nil {
		t.Fatalf("loaded = %#v", loaded)
	}
}
