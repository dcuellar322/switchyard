package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	settingsApplication "switchyard.dev/switchyard/internal/settings/application"
	settingsDomain "switchyard.dev/switchyard/internal/settings/domain"
)

func TestSettingsHTTPReadsAndUpdatesOneRevision(t *testing.T) {
	t.Parallel()
	service := &settingsServiceStub{status: settingsApplication.Status{Settings: httpSettings(), PendingRestart: []string{}}}
	server := NewIPC(Dependencies{Settings: service, Logger: slog.Default()})

	read := httptest.NewRequest(http.MethodGet, "/api/v1/settings", nil)
	readResponse := httptest.NewRecorder()
	server.ServeHTTP(readResponse, read)
	if readResponse.Code != http.StatusOK {
		t.Fatalf("GET status=%d body=%s", readResponse.Code, readResponse.Body.String())
	}

	body, _ := json.Marshal(map[string]any{"expectedRevision": 1, "settings": httpSettings()})
	update := httptest.NewRequest(http.MethodPut, "/api/v1/settings", bytes.NewReader(body))
	update.Header.Set(idempotencyHeader, "settings-request-one")
	updateResponse := httptest.NewRecorder()
	server.ServeHTTP(updateResponse, update)
	if updateResponse.Code != http.StatusOK {
		t.Fatalf("PUT status=%d body=%s", updateResponse.Code, updateResponse.Body.String())
	}
	if service.expected != 1 || service.actor != (settingsApplication.Actor{Type: "ipc", ID: "ipc"}) {
		t.Fatalf("expected=%d actor=%#v", service.expected, service.actor)
	}
}

type settingsServiceStub struct {
	status   settingsApplication.Status
	expected int64
	actor    settingsApplication.Actor
}

func (s *settingsServiceStub) Status(context.Context) (settingsApplication.Status, error) {
	return s.status, nil
}

func (s *settingsServiceStub) Update(_ context.Context, expected int64, settings settingsDomain.Settings, actor settingsApplication.Actor) (settingsApplication.Status, error) {
	s.expected, s.actor = expected, actor
	settings.Revision++
	s.status.Settings = settings
	return s.status, nil
}

func httpSettings() settingsDomain.Settings {
	return settingsDomain.Settings{
		Revision: 1, ProjectRoots: []string{"/tmp/projects"}, Ports: settingsDomain.PortPreferences{RangeStart: 15_000, RangeEnd: 19_999, Excluded: []int{}},
		Retention: settingsDomain.RetentionPreferences{LogAgeSeconds: 604_800, LogMaximumBytes: 256 << 20, MetricRawSeconds: 3600, MetricMinuteSeconds: 86_400, MetricQuarterHourSeconds: 2_592_000, MaximumMetricHistoryPoints: 1000},
		Tools:     settingsDomain.ToolPreferences{Terminal: "integrated", Editor: "vscode"},
		AI: settingsDomain.AIPreferences{DefaultProvider: settingsDomain.ProviderCodex, Providers: []settingsDomain.ProviderPreferences{
			{ID: settingsDomain.ProviderCodex, Enabled: true, Executable: "codex"}, {ID: settingsDomain.ProviderClaude, Enabled: true, Executable: "claude"}, {ID: settingsDomain.ProviderOpenAI},
		}},
		Permissions: settingsDomain.PermissionPreferences{DefaultAgentProfile: "observe"}, Appearance: settingsDomain.AppearancePreferences{Density: "comfortable", TimeDisplay: "relative", Theme: "dark"}, UpdatedAt: time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC),
	}
}
