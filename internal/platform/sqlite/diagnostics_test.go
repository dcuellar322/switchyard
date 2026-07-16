package sqlite

import (
	"context"
	"fmt"
	"testing"
	"time"

	"switchyard.dev/switchyard/internal/diagnostics/domain"
)

func TestDiagnosticRepositoryRoundTripAndRetention(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database, err := Open(ctx, t.TempDir()+"/switchyard.db")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()
	insertDeveloperTestProject(t, database)
	repository := NewDiagnosticRepository(database)
	base := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	for index := range 105 {
		diagnosis := domain.Diagnosis{
			ID: fmt.Sprintf("diagnosis-%03d", index), Version: domain.DiagnosisVersion, ProjectID: "project-1",
			Bundle:     domain.Bundle{Version: domain.BundleVersion, ProjectID: "project-1", SHA256: "fixture"},
			Hypotheses: []domain.Hypothesis{}, Warnings: []string{}, GeneratedAt: base.Add(time.Duration(index) * time.Minute), Deterministic: true,
		}
		if err := repository.SaveDiagnosis(ctx, diagnosis); err != nil {
			t.Fatal(err)
		}
	}
	latest, err := repository.LatestDiagnosis(ctx, "project-1")
	if err != nil || latest.ID != "diagnosis-104" {
		t.Fatalf("latest=%#v err=%v", latest, err)
	}
	var count int
	if err := database.connection.QueryRowContext(ctx, `SELECT COUNT(*) FROM diagnoses WHERE project_id = 'project-1'`).Scan(&count); err != nil || count != 100 {
		t.Fatalf("retained diagnoses=%d err=%v", count, err)
	}

	feedback := domain.Feedback{ID: "feedback-1", DiagnosisID: latest.ID, HypothesisID: "known", Verdict: "accurate", CreatedAt: base}
	if err := repository.SaveFeedback(ctx, feedback); err != nil {
		t.Fatal(err)
	}
	notification := domain.Notification{ID: "notification-1", ProjectID: "project-1", Code: "REPEATED_CRASH", Title: "Repeated crash", Detail: "API restarted", FirstSeenAt: base, LastSeenAt: base}
	if _, err := repository.UpsertNotification(ctx, notification); err != nil {
		t.Fatal(err)
	}
	notification.LastSeenAt = base.Add(time.Minute)
	stored, err := repository.UpsertNotification(ctx, notification)
	if err != nil || stored.Occurrences != 2 {
		t.Fatalf("notification=%#v err=%v", stored, err)
	}
	stored, err = repository.AcknowledgeNotification(ctx, stored.ID, base.Add(2*time.Minute))
	if err != nil || stored.AcknowledgedAt == nil {
		t.Fatalf("acknowledged notification=%#v err=%v", stored, err)
	}
	notification.LastSeenAt = base.Add(3 * time.Minute)
	stored, err = repository.UpsertNotification(ctx, notification)
	if err != nil || stored.Occurrences != 3 || stored.AcknowledgedAt != nil {
		t.Fatalf("reopened notification=%#v err=%v", stored, err)
	}

	recipe := domain.Recipe{ID: "recipe-1", ProjectID: "project-1", Name: "Inspect crash", TriggerCode: "REPEATED_CRASH", ActionID: "tests", CooldownSeconds: 3600, MaxRunsPerDay: 3, CreatedAt: base, UpdatedAt: base}
	if err := repository.SaveRecipe(ctx, recipe); err != nil {
		t.Fatal(err)
	}
	recipe, err = repository.UpdateRecipeEnabled(ctx, recipe.ID, true, base.Add(time.Minute))
	if err != nil || !recipe.Enabled {
		t.Fatalf("enabled recipe=%#v err=%v", recipe, err)
	}
	recipe, err = repository.MarkRecipeRun(ctx, recipe.ID, base.Add(2*time.Minute))
	if err != nil || recipe.RunsToday != 1 || recipe.LastRunAt == nil {
		t.Fatalf("run recipe=%#v err=%v", recipe, err)
	}
}
