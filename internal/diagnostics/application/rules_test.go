package application

import (
	"encoding/json"
	"testing"
	"time"

	"switchyard.dev/switchyard/internal/diagnostics/domain"
)

func TestKnownFailureRulesAreDeterministicAndEvidenceBacked(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	evidenceIDs := []string{"project", "runtime", "operations", "resources", "git", "health:db", "port:collision", "log:1", "cleanup"}
	evidence := make([]domain.Evidence, 0, len(evidenceIDs))
	for _, id := range evidenceIDs {
		evidence = append(evidence, domain.Evidence{ID: id, Data: json.RawMessage(`{}`), ObservedAt: now})
	}
	bundle := domain.Bundle{
		ProjectState: "stopped", ProjectAgeDay: 45, Evidence: evidence,
		Actions: []domain.Action{
			{ID: "health-check", Name: "Health check", Type: "health.check", Risk: "read_only"},
			{ID: "destroy-config", Name: "Delete config", Type: "config.delete", Risk: "destructive"},
		},
		Snapshot: domain.ProjectSnapshot{
			Runtime:       domain.RuntimeSnapshot{Driver: "process", EngineConnected: true, Services: []domain.ServiceSnapshot{{ID: "api", RestartCount: 3}}},
			Health:        domain.HealthSnapshot{Failures: []domain.HealthCheck{{ID: "db", ServiceID: "database", Required: true, Status: "unhealthy", Severity: "error", Message: "connection failed"}}},
			PortConflicts: []domain.PortConflict{{ID: "collision", Port: 3000, Summary: "port 3000 is occupied"}},
			Resources:     domain.ResourceSnapshot{Warnings: []string{"memory pressure"}},
			Git:           domain.GitSnapshot{Repository: true, Conflicted: 2, Operation: "merge"},
			RecentLogs:    []domain.LogLine{{EvidenceID: "log:1", Message: "address already in use; connection refused; permission denied; no space left; missing required environment", Timestamp: now, Redacted: true}},
			FailedRuns:    3,
			Cleanup:       domain.CleanupSnapshot{Candidates: 2, EstimatedBytes: 4096, Executable: false},
		},
	}
	hypotheses := Evaluate(bundle)
	want := map[string]bool{
		"PORT_CONFLICT": false, "UNHEALTHY_DEPENDENCY": false, "REPEATED_CRASH": false,
		"RESOURCE_PRESSURE": false, "PORT_BIND_FAILED": false, "DEPENDENCY_UNREACHABLE": false,
		"PERMISSION_DENIED": false, "RESOURCE_EXHAUSTED": false, "CONFIGURATION_MISSING": false,
		"GIT_OPERATION_INCOMPLETE": false, "STALE_PROJECT": false, "CLEANUP_PREVIEW": false,
	}
	knownEvidence := map[string]bool{}
	for _, item := range evidence {
		knownEvidence[item.ID] = true
	}
	for _, hypothesis := range hypotheses {
		if _, ok := want[hypothesis.Code]; ok {
			want[hypothesis.Code] = true
		}
		for _, id := range hypothesis.EvidenceIDs {
			if !knownEvidence[id] {
				t.Errorf("%s cited missing evidence %q", hypothesis.Code, id)
			}
		}
		for _, action := range hypothesis.SuggestedActions {
			if action.ActionID == "destroy-config" {
				t.Errorf("%s suggested destructive action", hypothesis.Code)
			}
		}
	}
	for code, found := range want {
		if !found {
			t.Errorf("deterministic rule %s did not fire; hypotheses=%#v", code, hypotheses)
		}
	}
}

func TestRuntimeDisconnectionAndDegradationAreKnownFailures(t *testing.T) {
	t.Parallel()
	bundle := domain.Bundle{
		ProjectState: "degraded",
		Evidence:     []domain.Evidence{{ID: "runtime", Data: json.RawMessage(`{}`)}},
		Snapshot:     domain.ProjectSnapshot{Runtime: domain.RuntimeSnapshot{Driver: "compose", EngineConnected: false}},
	}
	hypotheses := Evaluate(bundle)
	if len(hypotheses) != 2 || hypotheses[0].Source != "deterministic" || hypotheses[1].Source != "deterministic" {
		t.Fatalf("hypotheses=%#v", hypotheses)
	}
}
