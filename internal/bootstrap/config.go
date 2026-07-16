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
	DataDir            string
	HTTPAddr           string
	IPCAddr            string
	Logger             *slog.Logger
	LogRingCapacity    int
	LogSegmentBytes    int64
	LogRetentionAge    time.Duration
	LogRetentionBytes  int64
	RedactionPatterns  []string
	AICodexExecutable  string
	AICodexModel       string
	AIClaudeExecutable string
	AIClaudeModel      string
	AIOpenAIEndpoint   string
	AIOpenAIModel      string
	AIOpenAIAPIKeyEnv  string
}

// DefaultConfig uses a per-user data directory and loopback-only HTTP.
func DefaultConfig() (Config, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return Config{}, fmt.Errorf("resolve user config directory: %w", err)
	}
	return Config{
		DataDir:            filepath.Join(base, "Switchyard"),
		HTTPAddr:           defaultHTTPAddress,
		Logger:             slog.Default(),
		LogRingCapacity:    2_000,
		LogSegmentBytes:    1 << 20,
		LogRetentionAge:    7 * 24 * time.Hour,
		LogRetentionBytes:  256 << 20,
		AICodexExecutable:  "codex",
		AIClaudeExecutable: "claude",
		AIOpenAIAPIKeyEnv:  "OPENAI_API_KEY",
	}, nil
}
