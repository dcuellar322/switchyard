package sqlite

import (
	"context"
	"testing"
	"time"

	actionsDomain "switchyard.dev/switchyard/internal/actions/domain"
	portsDomain "switchyard.dev/switchyard/internal/ports/domain"
)

func TestPortReservationsReconcileManifestChanges(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database, err := Open(ctx, t.TempDir()+"/switchyard.db")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()
	insertDeveloperTestProject(t, database)
	repository := NewPortReservationRepository(database)
	declarations := []portsDomain.Fact{{
		ID: "decl", Kind: portsDomain.KindDeclaration, ProjectID: "project-1", ProjectName: "Project",
		ServiceID: "web", PortID: "web", Host: "127.0.0.1", Port: 18081, Target: 8080, Protocol: "tcp",
	}}
	reservations, err := repository.Reconcile(ctx, declarations, time.Now())
	if err != nil || len(reservations) != 1 || reservations[0].Port != 18081 {
		t.Fatalf("reservations=%#v err=%v", reservations, err)
	}
	reservations, err = repository.Reconcile(ctx, nil, time.Now())
	if err != nil || len(reservations) != 0 {
		t.Fatalf("stale reservations=%#v err=%v", reservations, err)
	}
}

func TestActionAuditPersistsOutcomeWithoutOutput(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database, err := Open(ctx, t.TempDir()+"/switchyard.db")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()
	insertDeveloperTestProject(t, database)
	if _, err := database.connection.ExecContext(ctx, `INSERT INTO operations
        (id, project_id, kind, state, idempotency_key, input_json, cancellation_requested, requested_at, updated_at)
        VALUES ('operation-1', 'project-1', 'action.run', 'running', 'test-key-123', '{}', 0, ?, ?)`, formatTime(time.Now()), formatTime(time.Now())); err != nil {
		t.Fatal(err)
	}
	repository := NewActionAuditRepository(database)
	audit := actionsDomain.Audit{ID: "audit-1", OperationID: "operation-1", ProjectID: "project-1", ActionID: "tests", ActionType: "tests.run",
		Risk: actionsDomain.RiskMutating, ActorType: "ipc", ActorID: "cli", State: "running", WorkingDirectory: "/trusted", StartedAt: time.Now()}
	if err := repository.Begin(ctx, audit); err != nil {
		t.Fatal(err)
	}
	if err := repository.Finish(ctx, audit.ID, "succeeded", "", time.Now()); err != nil {
		t.Fatal(err)
	}
	var state, errorCode string
	if err := database.connection.QueryRowContext(ctx, `SELECT state, COALESCE(error_code, '') FROM action_audit WHERE id = ?`, audit.ID).Scan(&state, &errorCode); err != nil {
		t.Fatal(err)
	}
	if state != "succeeded" || errorCode != "" {
		t.Fatalf("state=%q error=%q", state, errorCode)
	}
}

func insertDeveloperTestProject(t *testing.T, database *Database) {
	t.Helper()
	now := formatTime(time.Now())
	_, err := database.connection.Exec(`INSERT INTO projects
        (id, slug, display_name, description, trust_state, primary_location, manifest_revision, created_at, updated_at)
        VALUES ('project-1', 'project', 'Project', '', 'trusted', ?, 1, ?, ?)`, t.TempDir(), now, now)
	if err != nil {
		t.Fatal(err)
	}
}
