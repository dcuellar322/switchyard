package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"switchyard.dev/switchyard/internal/transport/contract/generated"
)

func TestReadSettingsDocumentIsStrictAndBounded(t *testing.T) {
	t.Parallel()
	settings := generated.DaemonSettings{
		Revision: 1, ProjectRoots: []string{t.TempDir()}, Ports: generated.PortPreferences{RangeStart: 15_000, RangeEnd: 19_999, Excluded: []int{}},
		Retention: generated.RetentionPreferences{LogAgeSeconds: 604_800, LogMaximumBytes: 256 << 20, MetricRawSeconds: 3600, MetricMinuteSeconds: 86_400, MetricQuarterHourSeconds: 2_592_000, MaximumMetricHistoryPoints: 1000},
		Tools:     generated.ToolPreferences{Terminal: generated.Integrated, Editor: generated.Vscode},
		Ai: generated.AIPreferences{DefaultProvider: generated.AIPreferencesDefaultProviderCodex, Providers: []generated.AIProviderPreferences{
			{Id: generated.AIProviderPreferencesIdCodex, Enabled: true}, {Id: generated.AIProviderPreferencesIdClaude, Enabled: true}, {Id: generated.AIProviderPreferencesIdOpenaiCompatible},
		}},
		Permissions: generated.PermissionPreferences{DefaultAgentProfile: generated.Observe}, Appearance: generated.AppearancePreferences{Density: generated.Comfortable, TimeDisplay: generated.Relative, Theme: generated.Dark}, UpdatedAt: time.Now().UTC(),
	}
	encoded, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	decoded, err := readSettingsDocument(path)
	if err != nil || decoded.Revision != 1 {
		t.Fatalf("decoded=%#v error=%v", decoded, err)
	}
	if err := os.WriteFile(path, append(encoded, []byte(` {"extra":true}`)...), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readSettingsDocument(path); err == nil {
		t.Fatal("multiple JSON values were accepted")
	}
}
