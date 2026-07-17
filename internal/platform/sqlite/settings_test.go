package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	settingsApplication "switchyard.dev/switchyard/internal/settings/application"
	"switchyard.dev/switchyard/internal/settings/domain"
)

func TestSettingsRepositoryPersistsRevisionAndValueFreeAudit(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database, err := Open(ctx, filepath.Join(t.TempDir(), "switchyard.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	repository := NewSettingsRepository(database)
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	defaults := persistedSettings(t.TempDir(), now)
	current, err := repository.Initialize(ctx, defaults)
	if err != nil || current.Revision != 1 {
		t.Fatalf("initialize = %#v error=%v", current, err)
	}
	current.Appearance.Density = "compact"
	current.Revision = 2
	current.UpdatedAt = now.Add(time.Minute)
	updated, err := repository.Update(ctx, 1, current, settingsApplication.Audit{
		ActorType: "browser", ActorID: "session", Sections: []string{"appearance"}, OccurredAt: current.UpdatedAt,
	})
	if err != nil || updated.Revision != 2 {
		t.Fatalf("update = %#v error=%v", updated, err)
	}
	var revision, audits int
	var sections string
	if err := database.connection.QueryRow(`SELECT revision FROM settings WHERE singleton=1`).Scan(&revision); err != nil {
		t.Fatal(err)
	}
	if err := database.connection.QueryRow(`SELECT COUNT(*), sections_json FROM settings_audit_events`).Scan(&audits, &sections); err != nil {
		t.Fatal(err)
	}
	if revision != 2 || audits != 1 || sections != `["appearance"]` {
		t.Fatalf("revision=%d audits=%d sections=%s", revision, audits, sections)
	}
}

func persistedSettings(root string, at time.Time) domain.Settings {
	return domain.Settings{
		Revision: 1, ProjectRoots: []string{root}, Ports: domain.PortPreferences{RangeStart: 15_000, RangeEnd: 19_999, Excluded: []int{}},
		Retention: domain.RetentionPreferences{LogAgeSeconds: 604_800, LogMaximumBytes: 256 << 20, MetricRawSeconds: 3600, MetricMinuteSeconds: 86_400, MetricQuarterHourSeconds: 2_592_000, MaximumMetricHistoryPoints: 1000},
		Tools:     domain.ToolPreferences{Terminal: "integrated", Editor: "vscode"},
		AI: domain.AIPreferences{DefaultProvider: domain.ProviderCodex, Providers: []domain.ProviderPreferences{
			{ID: domain.ProviderCodex, Enabled: true, Executable: "codex"}, {ID: domain.ProviderClaude, Enabled: true, Executable: "claude"}, {ID: domain.ProviderOpenAI},
		}},
		Permissions: domain.PermissionPreferences{DefaultAgentProfile: "observe"}, Appearance: domain.AppearancePreferences{Density: "comfortable", TimeDisplay: "relative", Theme: "dark"}, UpdatedAt: at,
	}
}
