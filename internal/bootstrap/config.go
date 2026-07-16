package bootstrap

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

const defaultHTTPAddress = "127.0.0.1:19616"

// Config controls process-level daemon composition.
type Config struct {
	DataDir  string
	HTTPAddr string
	Logger   *slog.Logger
}

// DefaultConfig uses a per-user data directory and loopback-only HTTP.
func DefaultConfig() (Config, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return Config{}, fmt.Errorf("resolve user config directory: %w", err)
	}
	return Config{
		DataDir:  filepath.Join(base, "Switchyard"),
		HTTPAddr: defaultHTTPAddress,
		Logger:   slog.Default(),
	}, nil
}
