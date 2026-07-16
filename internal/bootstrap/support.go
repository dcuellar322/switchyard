package bootstrap

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	observabilityAdapters "switchyard.dev/switchyard/internal/observability/adapters"
	"switchyard.dev/switchyard/internal/platform/daemonlog"
	supportAdapters "switchyard.dev/switchyard/internal/support/adapters"
	supportApplication "switchyard.dev/switchyard/internal/support/application"
	"switchyard.dev/switchyard/internal/support/domain"
)

func prepareDaemonSupport(config Config) (*observabilityAdapters.Redactor, *daemonlog.FileHandler, *slog.Logger, error) {
	redactor, err := observabilityAdapters.NewRedactor(config.RedactionPatterns)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("compile log redaction patterns: %w", err)
	}
	internalLog, err := daemonlog.Open(filepath.Join(config.DataDir, "internal.ndjson"), 4<<20, redactor.RedactText)
	if err != nil {
		return nil, nil, nil, err
	}
	if err := writeSupportConfiguration(config); err != nil {
		_ = internalLog.Close()
		return nil, nil, nil, err
	}
	logger := slog.New(daemonlog.Tee(config.Logger.Handler(), internalLog))
	return redactor, internalLog, logger, nil
}

// NewSupportService composes the private filesystem and environment adapters
// behind the support application boundary.
func NewSupportService(dataDir string, redactionPatterns []string) (*supportApplication.Service, error) {
	redactor, err := observabilityAdapters.NewRedactor(redactionPatterns)
	if err != nil {
		return nil, err
	}
	logs, err := supportAdapters.NewInternalLogSource(dataDir, redactor.RedactText)
	if err != nil {
		return nil, err
	}
	return supportApplication.NewService(supportAdapters.EnvironmentProbe{}, logs, nil)
}

// ReadSupportConfiguration reads the daemon-authored sanitized allowlist.
func ReadSupportConfiguration(dataDir string) (domain.SanitizedConfiguration, error) {
	return supportAdapters.ReadConfiguration(dataDir)
}

// WriteSupportBundle writes only the previously reviewed preview.
func WriteSupportBundle(path string, preview domain.Preview) (domain.BundleReceipt, error) {
	return (supportAdapters.ArchiveWriter{}).Write(path, preview)
}

func writeSupportConfiguration(config Config) error {
	ipcMode := "default owner-only local IPC"
	if strings.TrimSpace(config.IPCAddr) != "" {
		ipcMode = "custom owner-only local IPC"
	}
	configuration := domain.SanitizedConfiguration{
		HTTPBinding:        "loopback TCP",
		IPCMode:            ipcMode,
		RoutingEnabled:     strings.TrimSpace(config.RoutingAddr) != "",
		RemoteAgentEnabled: strings.TrimSpace(config.RemoteAddr) != "",
		Retention: domain.RetentionConfiguration{
			LogAge: config.LogRetentionAge.String(), LogMaximumBytes: config.LogRetentionBytes,
			MetricRaw: config.MetricRawRetention.String(), MetricMinute: config.MetricMinuteRetention.String(),
			MetricQuarterHour: config.MetricQuarterHourRetention.String(), MaximumHistoryRows: config.MetricMaximumHistoryPoints,
		},
		Providers: []domain.ProviderConfiguration{
			{ID: "codex", Model: config.AICodexModel, Configured: strings.TrimSpace(config.AICodexExecutable) != ""},
			{ID: "claude", Model: config.AIClaudeModel, Configured: strings.TrimSpace(config.AIClaudeExecutable) != ""},
			{
				ID: "openai-compatible", Model: config.AIOpenAIModel,
				Configured:                    strings.TrimSpace(config.AIOpenAIEndpoint) != "",
				CredentialReferenceConfigured: strings.TrimSpace(config.AIOpenAIAPIKeyEnv) != "",
			},
		},
	}
	return supportAdapters.WriteConfiguration(config.DataDir, configuration)
}
