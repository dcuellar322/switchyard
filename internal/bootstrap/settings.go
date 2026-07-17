package bootstrap

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"switchyard.dev/switchyard/internal/platform/sqlite"
	settingsApplication "switchyard.dev/switchyard/internal/settings/application"
	settingsDomain "switchyard.dev/switchyard/internal/settings/domain"
)

func newSettingsService(ctx context.Context, database *sqlite.Database, config Config) (*settingsApplication.Service, settingsDomain.Settings, error) {
	service, err := settingsApplication.NewService(sqlite.NewSettingsRepository(database))
	if err != nil {
		return nil, settingsDomain.Settings{}, err
	}
	defaults, err := defaultSettings(config)
	if err != nil {
		return nil, settingsDomain.Settings{}, err
	}
	status, err := service.Initialize(ctx, defaults)
	if err != nil {
		return nil, settingsDomain.Settings{}, err
	}
	return service, status.Settings, nil
}

func prepareEffectiveSettings(ctx context.Context, database *sqlite.Database, config Config) (*settingsApplication.Service, Config, error) {
	service, effective, err := newSettingsService(ctx, database, config)
	if err != nil {
		return nil, Config{}, err
	}
	config = applySettings(config, effective)
	if err := writeSupportConfiguration(config, effective); err != nil {
		return nil, Config{}, err
	}
	return service, config, nil
}

func openConfiguredDatabase(ctx context.Context, config Config) (*sqlite.Database, *settingsApplication.Service, Config, error) {
	database, err := sqlite.Open(ctx, filepath.Join(config.DataDir, "switchyard.db"))
	if err != nil {
		return nil, nil, Config{}, err
	}
	service, effectiveConfig, err := prepareEffectiveSettings(ctx, database, config)
	if err != nil {
		_ = database.Close()
		return nil, nil, Config{}, err
	}
	return database, service, effectiveConfig, nil
}

func defaultSettings(config Config) (settingsDomain.Settings, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return settingsDomain.Settings{}, err
	}
	credentialReference := ""
	if strings.TrimSpace(config.AIOpenAIAPIKeyEnv) != "" {
		credentialReference = "env:" + strings.TrimSpace(config.AIOpenAIAPIKeyEnv)
	}
	return settingsDomain.Settings{
		ProjectRoots: []string{home},
		Ports:        settingsDomain.PortPreferences{RangeStart: 15_000, RangeEnd: 19_999, Excluded: []int{}},
		Retention: settingsDomain.RetentionPreferences{
			LogAgeSeconds: config.LogRetentionAge.Milliseconds() / 1000, LogMaximumBytes: config.LogRetentionBytes,
			MetricRawSeconds:           config.MetricRawRetention.Milliseconds() / 1000,
			MetricMinuteSeconds:        config.MetricMinuteRetention.Milliseconds() / 1000,
			MetricQuarterHourSeconds:   config.MetricQuarterHourRetention.Milliseconds() / 1000,
			MaximumMetricHistoryPoints: config.MetricMaximumHistoryPoints,
		},
		Tools: settingsDomain.ToolPreferences{Terminal: "integrated", Editor: "vscode"},
		AI: settingsDomain.AIPreferences{DefaultProvider: settingsDomain.ProviderCodex, Providers: []settingsDomain.ProviderPreferences{
			{ID: settingsDomain.ProviderCodex, Enabled: config.AICodexExecutable != "", Executable: config.AICodexExecutable, Model: config.AICodexModel},
			{ID: settingsDomain.ProviderClaude, Enabled: config.AIClaudeExecutable != "", Executable: config.AIClaudeExecutable, Model: config.AIClaudeModel},
			{ID: settingsDomain.ProviderOpenAI, Enabled: config.AIOpenAIEndpoint != "", Endpoint: config.AIOpenAIEndpoint, Model: config.AIOpenAIModel, CredentialReference: credentialReference},
		}},
		Permissions: settingsDomain.PermissionPreferences{DefaultAgentProfile: "observe"},
		Appearance:  settingsDomain.AppearancePreferences{Density: "comfortable", TimeDisplay: "relative", Theme: "dark"},
	}, nil
}

func applySettings(config Config, settings settingsDomain.Settings) Config {
	config.LogRetentionAge = time.Duration(settings.Retention.LogAgeSeconds) * time.Second
	config.LogRetentionBytes = settings.Retention.LogMaximumBytes
	config.MetricRawRetention = time.Duration(settings.Retention.MetricRawSeconds) * time.Second
	config.MetricMinuteRetention = time.Duration(settings.Retention.MetricMinuteSeconds) * time.Second
	config.MetricQuarterHourRetention = time.Duration(settings.Retention.MetricQuarterHourSeconds) * time.Second
	config.MetricMaximumHistoryPoints = settings.Retention.MaximumMetricHistoryPoints
	for _, provider := range settings.AI.Providers {
		switch provider.ID {
		case settingsDomain.ProviderCodex:
			config.AICodexExecutable, config.AICodexModel = enabledExecutable(provider), provider.Model
		case settingsDomain.ProviderClaude:
			config.AIClaudeExecutable, config.AIClaudeModel = enabledExecutable(provider), provider.Model
		case settingsDomain.ProviderOpenAI:
			config.AIOpenAIEndpoint, config.AIOpenAIModel = enabledEndpoint(provider), provider.Model
			config.AIOpenAIAPIKeyEnv = strings.TrimPrefix(provider.CredentialReference, "env:")
		}
	}
	return config
}

func enabledExecutable(provider settingsDomain.ProviderPreferences) string {
	if provider.Enabled {
		return provider.Executable
	}
	return ""
}

func enabledEndpoint(provider settingsDomain.ProviderPreferences) string {
	if provider.Enabled {
		return provider.Endpoint
	}
	return ""
}
