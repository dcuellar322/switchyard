package bootstrap

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

const defaultHTTPAddress = "127.0.0.1:19616"

// Config controls process-level daemon composition.
type Config struct {
	DataDir                    string
	HTTPAddr                   string
	IPCAddr                    string
	RoutingAddr                string
	Logger                     *slog.Logger
	LogRingCapacity            int
	LogSegmentBytes            int64
	LogRetentionAge            time.Duration
	LogRetentionBytes          int64
	MetricSampleInterval       time.Duration
	MetricRawRetention         time.Duration
	MetricMinuteRetention      time.Duration
	MetricQuarterHourRetention time.Duration
	MetricMaximumHistoryPoints int
	RedactionPatterns          []string
	AICodexExecutable          string
	AICodexModel               string
	AIClaudeExecutable         string
	AIClaudeModel              string
	AIOpenAIEndpoint           string
	AIOpenAIModel              string
	AIOpenAIAPIKeyEnv          string
	RemoteAddr                 string
	RemoteTLSCertificate       string
	RemoteTLSKey               string
	RemoteClientCA             string
	RemoteMachineID            string
	RemoteMachineName          string
	RemoteControllers          []string
}

// DefaultConfig uses a per-user data directory and loopback-only HTTP.
func DefaultConfig() (Config, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return Config{}, fmt.Errorf("resolve user config directory: %w", err)
	}
	//nolint:gosec // G101: AIOpenAIAPIKeyEnv stores an environment-variable name, never credential material.
	return Config{
		DataDir:                    filepath.Join(base, "Switchyard"),
		HTTPAddr:                   defaultHTTPAddress,
		Logger:                     slog.Default(),
		LogRingCapacity:            2_000,
		LogSegmentBytes:            1 << 20,
		LogRetentionAge:            7 * 24 * time.Hour,
		LogRetentionBytes:          256 << 20,
		MetricSampleInterval:       10 * time.Second,
		MetricRawRetention:         time.Hour,
		MetricMinuteRetention:      24 * time.Hour,
		MetricQuarterHourRetention: 30 * 24 * time.Hour,
		MetricMaximumHistoryPoints: 1_000,
		AICodexExecutable:          "codex",
		AIClaudeExecutable:         "claude",
		AIOpenAIAPIKeyEnv:          "OPENAI_API_KEY",
	}, nil
}
