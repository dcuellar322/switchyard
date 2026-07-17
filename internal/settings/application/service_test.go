package application

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"switchyard.dev/switchyard/internal/settings/domain"
)

func TestSettingsLifecycleRestartEffectsAndRootAuthorization(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	repository := &repositoryStub{}
	service, err := NewService(repository)
	if err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC) }
	status, err := service.Initialize(context.Background(), validSettings(root))
	if err != nil {
		t.Fatal(err)
	}
	if status.Settings.Revision != 1 || len(status.PendingRestart) != 0 {
		t.Fatalf("initial status = %#v", status)
	}
	unchanged, err := service.Update(context.Background(), 1, status.Settings, Actor{})
	if err != nil || unchanged.Settings.Revision != 1 {
		t.Fatalf("no-op update = %#v error=%v", unchanged, err)
	}

	requested := status.Settings
	requested.Appearance.Density = "compact"
	requested.Retention.LogAgeSeconds = 8 * 24 * 3600
	status, err = service.Update(context.Background(), 1, requested, Actor{Type: "browser", ID: "session-one"})
	if err != nil {
		t.Fatal(err)
	}
	if status.Settings.Revision != 2 || len(status.PendingRestart) != 1 || status.PendingRestart[0] != "retention" {
		t.Fatalf("updated status = %#v", status)
	}
	if repository.audit.ActorType != "browser" || repository.audit.ActorID != "session-one" || len(repository.audit.Sections) != 2 {
		t.Fatalf("audit = %#v", repository.audit)
	}
	if _, err := service.Update(context.Background(), 1, requested, Actor{}); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("stale update error = %v", err)
	}

	inside := filepath.Join(root, "nested")
	if err := os.Mkdir(inside, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := service.AuthorizeProjectRoot(context.Background(), inside, false); err != nil {
		t.Fatalf("inside root error = %v", err)
	}
	if err := service.AuthorizeProjectRoot(context.Background(), t.TempDir(), false); !errors.Is(err, ErrOutsideProjectRoots) {
		t.Fatalf("outside root error = %v", err)
	}
	if err := service.AuthorizeProjectRoot(context.Background(), t.TempDir(), true); err != nil {
		t.Fatalf("explicit override error = %v", err)
	}
}

func TestSettingsRejectsFilesystemRootAndCredentialValues(t *testing.T) {
	t.Parallel()
	service, _ := NewService(&repositoryStub{})
	settings := validSettings(t.TempDir())
	settings.ProjectRoots = []string{string(filepath.Separator)}
	if _, err := service.Initialize(context.Background(), settings); !errors.Is(err, ErrInvalidSettings) {
		t.Fatalf("filesystem root error = %v", err)
	}
	settings = validSettings(t.TempDir())
	settings.AI.Providers[2].CredentialReference = "secret-value"
	if _, err := service.Initialize(context.Background(), settings); !errors.Is(err, ErrInvalidSettings) {
		t.Fatalf("credential value error = %v", err)
	}
}

func TestInitializeKeepsTemporarilyUnavailablePersistedRoot(t *testing.T) {
	t.Parallel()
	persisted := validSettings(t.TempDir())
	persisted.Revision = 3
	persisted.ProjectRoots = []string{filepath.Join(t.TempDir(), "detached-volume")}
	repository := &repositoryStub{settings: persisted}
	service, _ := NewService(repository)
	status, err := service.Initialize(context.Background(), validSettings(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	if status.Settings.Revision != 3 || status.Settings.ProjectRoots[0] != persisted.ProjectRoots[0] {
		t.Fatalf("status = %#v", status)
	}
}

func validSettings(root string) domain.Settings {
	return domain.Settings{
		ProjectRoots: []string{root},
		Ports:        domain.PortPreferences{RangeStart: 15_000, RangeEnd: 19_999, Excluded: []int{15_001}},
		Retention: domain.RetentionPreferences{
			LogAgeSeconds: 7 * 24 * 3600, LogMaximumBytes: 256 << 20,
			MetricRawSeconds: 3600, MetricMinuteSeconds: 24 * 3600,
			MetricQuarterHourSeconds: 30 * 24 * 3600, MaximumMetricHistoryPoints: 1000,
		},
		Tools: domain.ToolPreferences{Terminal: "integrated", Editor: "vscode"},
		AI: domain.AIPreferences{DefaultProvider: domain.ProviderCodex, Providers: []domain.ProviderPreferences{
			{ID: domain.ProviderCodex, Enabled: true, Executable: "codex"},
			{ID: domain.ProviderClaude, Enabled: true, Executable: "claude"},
			{ID: domain.ProviderOpenAI, Enabled: false},
		}},
		Permissions: domain.PermissionPreferences{DefaultAgentProfile: "observe"},
		Appearance:  domain.AppearancePreferences{Density: "comfortable", TimeDisplay: "relative", Theme: "dark"},
	}
}

type repositoryStub struct {
	settings domain.Settings
	audit    Audit
}

func (r *repositoryStub) Initialize(_ context.Context, defaults domain.Settings) (domain.Settings, error) {
	if r.settings.Revision == 0 {
		r.settings = defaults
	}
	return r.settings, nil
}

func (r *repositoryStub) Get(context.Context) (domain.Settings, error) { return r.settings, nil }

func (r *repositoryStub) Update(_ context.Context, expected int64, settings domain.Settings, audit Audit) (domain.Settings, error) {
	if r.settings.Revision != expected {
		return domain.Settings{}, ErrRevisionConflict
	}
	r.settings, r.audit = settings, audit
	return settings, nil
}
