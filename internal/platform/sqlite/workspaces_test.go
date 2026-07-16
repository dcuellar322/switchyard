package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"testing"
	"time"

	workspace "switchyard.dev/switchyard/internal/workspace/application"
	"switchyard.dev/switchyard/internal/workspace/domain"
)

func TestWorkspaceRepositoryRoundTripsGraphAndLatestExecution(t *testing.T) {
	t.Parallel()

	database := openWorkspaceTestDatabase(t)
	seedWorkspaceProjects(t, database, "database", "api", "web")
	repository := NewWorkspaceRepository(database)
	now := time.Date(2026, 7, 16, 15, 0, 0, 123, time.UTC)
	item := persistedWorkspace(now)
	if err := item.Validate(); err != nil {
		t.Fatalf("fixture Validate() error = %v", err)
	}
	if err := repository.Create(context.Background(), item); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := repository.Get(context.Background(), item.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Name != item.Name || got.DefaultProfileID != "low-memory" || len(got.Members) != 3 ||
		len(got.Dependencies) != 2 || len(got.Profiles) != 1 || len(got.Recipes) != 2 {
		t.Fatalf("Get() = %#v", got)
	}
	if got.Members[1].HealthTimeout != 45*time.Second || !got.Members[1].HealthGate {
		t.Fatalf("health-gated member = %#v", got.Members[1])
	}
	if profile := got.Profiles[0]; !profile.LowMemory || profile.MaxParallel != 1 ||
		!slices.Equal(profile.ProjectIDs, []string{"web"}) {
		t.Fatalf("profile = %#v", profile)
	}
	if !slices.Equal(got.Recipes[1].Arguments, []string{"--project", "api"}) {
		t.Fatalf("recipe arguments = %#v", got.Recipes[1].Arguments)
	}

	finished := now.Add(time.Minute)
	execution := domain.ExecutionSummary{
		ID: "workspace-run-1", WorkspaceID: item.ID, Kind: domain.ExecutionStart,
		State: domain.ExecutionPartial, Policy: domain.FailurePolicyContinue, ProfileID: "low-memory",
		ErrorMessage: "web failed", StartedAt: now, FinishedAt: &finished,
		Projects: []domain.ProjectResult{
			{ProjectID: "database", Role: domain.MemberRoleDependency, Status: domain.ProjectRunning, Order: 0},
			{ProjectID: "api", Role: domain.MemberRoleApplication, Status: domain.ProjectRunning, Order: 1},
			{ProjectID: "web", Role: domain.MemberRoleApplication, Status: domain.ProjectStartFailed, Message: "web failed", Order: 2},
		},
	}
	if err := repository.SaveExecution(context.Background(), execution); err != nil {
		t.Fatalf("SaveExecution() error = %v", err)
	}
	got, err = repository.Get(context.Background(), item.ID)
	if err != nil {
		t.Fatalf("Get(after execution) error = %v", err)
	}
	if got.LastRun == nil || got.LastRun.State != domain.ExecutionPartial || got.Members[2].Status != domain.ProjectStartFailed ||
		got.Members[2].Message != "web failed" {
		t.Fatalf("latest execution projection = %#v", got)
	}
}

func TestWorkspaceRepositoryUpdateListDeleteAndRevisionConflict(t *testing.T) {
	t.Parallel()

	database := openWorkspaceTestDatabase(t)
	seedWorkspaceProjects(t, database, "database", "api", "web")
	repository := NewWorkspaceRepository(database)
	item := persistedWorkspace(time.Now().UTC())
	if err := repository.Create(context.Background(), item); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	item.Name = "Renamed"
	item.Revision = 2
	item.UpdatedAt = item.UpdatedAt.Add(time.Minute)
	item.Recipes = item.Recipes[:1]
	if err := repository.Update(context.Background(), item, 1); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if err := repository.Update(context.Background(), item, 1); !errors.Is(err, workspace.ErrRevisionConflict) {
		t.Fatalf("stale Update() error = %v, want ErrRevisionConflict", err)
	}
	items, err := repository.List(context.Background())
	if err != nil || len(items) != 1 || items[0].Name != "Renamed" || len(items[0].Recipes) != 1 {
		t.Fatalf("List() = %#v, error %v", items, err)
	}
	if err := repository.Delete(context.Background(), item.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := repository.Get(context.Background(), item.ID); !errors.Is(err, workspace.ErrNotFound) {
		t.Fatalf("Get(deleted) error = %v, want ErrNotFound", err)
	}
	var projectCount int
	if err := database.connection.QueryRow(`SELECT COUNT(*) FROM projects`).Scan(&projectCount); err != nil {
		t.Fatalf("count projects: %v", err)
	}
	if projectCount != 3 {
		t.Fatalf("project count after workspace delete = %d, want 3", projectCount)
	}
}

func TestWorkspaceMigrationAddsOperationAndAuditWorkspaceTargets(t *testing.T) {
	t.Parallel()

	database := openWorkspaceTestDatabase(t)
	for _, table := range []string{"operations", "audit_events"} {
		rows, err := database.connection.Query(`PRAGMA table_info(` + table + `)`)
		if err != nil {
			t.Fatalf("table_info(%s): %v", table, err)
		}
		found := false
		for rows.Next() {
			var columnID, notNull, primaryKey int
			var name, kind string
			var defaultValue any
			if err := rows.Scan(&columnID, &name, &kind, &notNull, &defaultValue, &primaryKey); err != nil {
				_ = rows.Close()
				t.Fatalf("scan table_info(%s): %v", table, err)
			}
			found = found || name == "workspace_id"
		}
		_ = rows.Close()
		if !found {
			t.Fatalf("%s does not include workspace_id", table)
		}
	}
}

func TestWorkspaceRepositorySupportsEnvironmentMembersWithoutCrossDomainForeignKey(t *testing.T) {
	t.Parallel()
	database := openWorkspaceTestDatabase(t)
	repository := NewWorkspaceRepository(database)
	now := time.Now().UTC()
	item := domain.Workspace{
		ID: "workspace-environment", Name: "Feature checkout", DefaultFailurePolicy: domain.FailurePolicyRollback,
		Revision: 1, CreatedAt: now, UpdatedAt: now,
		Members: []domain.Member{{ProjectID: "env-feature", Role: domain.MemberRoleApplication}},
	}
	if err := repository.Create(context.Background(), item); err != nil {
		t.Fatalf("Create(environment member) error = %v", err)
	}
	stored, err := repository.Get(context.Background(), item.ID)
	if err != nil || len(stored.Members) != 1 || stored.Members[0].ProjectID != "env-feature" {
		t.Fatalf("Get()=%#v error=%v", stored, err)
	}
}

func TestWorkspaceRepositoryRecoversInterruptedExecution(t *testing.T) {
	t.Parallel()
	database := openWorkspaceTestDatabase(t)
	seedWorkspaceProjects(t, database, "database", "api", "web")
	repository := NewWorkspaceRepository(database)
	now := time.Now().UTC()
	item := persistedWorkspace(now)
	if err := repository.Create(context.Background(), item); err != nil {
		t.Fatal(err)
	}
	execution := domain.ExecutionSummary{
		ID: "workspace-run-interrupted", WorkspaceID: item.ID, Kind: domain.ExecutionStart,
		State: domain.ExecutionRunning, Policy: domain.FailurePolicyContinue, StartedAt: now,
		Projects: []domain.ProjectResult{
			{ProjectID: "database", Role: domain.MemberRoleDependency, Status: domain.ProjectRunning},
			{ProjectID: "api", Role: domain.MemberRoleApplication, Status: domain.ProjectCheckingHealth, Order: 1},
			{ProjectID: "web", Role: domain.MemberRoleApplication, Status: domain.ProjectQueued, Order: 2},
		},
	}
	if err := repository.SaveExecution(context.Background(), execution); err != nil {
		t.Fatal(err)
	}
	recoveredAt := now.Add(time.Minute)
	if err := repository.RecoverWorkspaceExecutions(context.Background(), recoveredAt); err != nil {
		t.Fatal(err)
	}
	stored, err := repository.Get(context.Background(), item.ID)
	if err != nil || stored.LastRun == nil {
		t.Fatalf("Get()=%#v error=%v", stored, err)
	}
	if stored.LastRun.State != domain.ExecutionPartial || stored.LastRun.FinishedAt == nil || stored.Members[1].Status != domain.ProjectCancelled {
		t.Fatalf("recovered execution = %#v", stored.LastRun)
	}
}

func openWorkspaceTestDatabase(t *testing.T) *Database {
	t.Helper()
	database, err := Open(context.Background(), filepath.Join(t.TempDir(), "switchyard.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})
	return database
}

func seedWorkspaceProjects(t *testing.T, database *Database, projectIDs ...string) {
	t.Helper()
	now := formatTime(time.Now())
	for _, projectID := range projectIDs {
		_, err := database.connection.Exec(`INSERT INTO projects
            (id, slug, display_name, trust_state, primary_location, created_at, updated_at)
            VALUES (?, ?, ?, 'trusted', ?, ?, ?)`, projectID, projectID, projectID, "/tmp/"+projectID, now, now)
		if err != nil {
			t.Fatalf("seed project %s: %v", projectID, err)
		}
	}
}

func persistedWorkspace(now time.Time) domain.Workspace {
	return domain.Workspace{
		ID: "workspace-product", Name: "Product stack", Description: "Local product dependencies",
		DefaultFailurePolicy: domain.FailurePolicyContinue, DefaultProfileID: "low-memory",
		Revision: 1, CreatedAt: now, UpdatedAt: now,
		Members: []domain.Member{
			{ProjectID: "database", Role: domain.MemberRoleDependency, Order: 0},
			{ProjectID: "api", Role: domain.MemberRoleApplication, Order: 1, HealthGate: true, HealthTimeout: 45 * time.Second},
			{ProjectID: "web", Role: domain.MemberRoleApplication, Order: 2},
		},
		Dependencies: []domain.Dependency{
			{ProjectID: "api", DependsOnProjectID: "database"},
			{ProjectID: "web", DependsOnProjectID: "api"},
		},
		Profiles: []domain.Profile{{
			ID: "low-memory", Name: "Low memory", Description: "Start the web path serially",
			ProjectIDs: []string{"web"}, MaxParallel: 1, LowMemory: true, MemoryBudgetBytes: 2 << 30,
		}},
		Recipes: []domain.Recipe{
			{ID: "open-web", Name: "Open web", Kind: domain.RecipeOpenURL, ProjectID: "web", Target: "http://product.localhost", Order: 0},
			{ID: "agent-api", Name: "Start API agent", Kind: domain.RecipeStartAgent, ProjectID: "api", Arguments: []string{"--project", "api"}, Order: 1},
		},
	}
}
