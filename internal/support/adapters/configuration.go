package adapters

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"switchyard.dev/switchyard/internal/support/domain"
)

const configurationFile = "support-config.json"

// WriteConfiguration atomically persists only the support-safe allowlist.
func WriteConfiguration(dataDir string, configuration domain.SanitizedConfiguration) error {
	encoded, err := json.MarshalIndent(configuration, "", "  ")
	if err != nil {
		return fmt.Errorf("encode sanitized support configuration: %w", err)
	}
	encoded = append(encoded, '\n')
	temporary, err := os.CreateTemp(dataDir, ".support-config-*.tmp")
	if err != nil {
		return fmt.Errorf("create sanitized support configuration: %w", err)
	}
	path := temporary.Name()
	committed := false
	defer func() {
		_ = temporary.Close()
		if !committed {
			_ = os.Remove(path)
		}
	}()
	if err := temporary.Chmod(0o600); err != nil {
		return fmt.Errorf("restrict sanitized support configuration: %w", err)
	}
	if _, err := temporary.Write(encoded); err != nil {
		return fmt.Errorf("write sanitized support configuration: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("sync sanitized support configuration: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close sanitized support configuration: %w", err)
	}
	output := filepath.Join(dataDir, configurationFile)
	if info, err := os.Lstat(output); err == nil && !info.Mode().IsRegular() {
		return fmt.Errorf("sanitized support configuration path is not a regular file: %s", output)
	} else if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("inspect sanitized support configuration: %w", err)
	}
	if err := replaceFile(path, output); err != nil {
		return fmt.Errorf("commit sanitized support configuration: %w", err)
	}
	committed = true
	return nil
}

// ReadConfiguration reads the daemon-authored support-safe allowlist.
func ReadConfiguration(dataDir string) (domain.SanitizedConfiguration, error) {
	path := filepath.Join(dataDir, configurationFile)
	info, err := os.Lstat(path)
	if err != nil {
		return domain.SanitizedConfiguration{}, fmt.Errorf("inspect sanitized support configuration: %w", err)
	}
	if !info.Mode().IsRegular() {
		return domain.SanitizedConfiguration{}, fmt.Errorf("sanitized support configuration is not a regular file: %s", path)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return domain.SanitizedConfiguration{}, fmt.Errorf("read sanitized support configuration: %w", err)
	}
	var configuration domain.SanitizedConfiguration
	if err := json.Unmarshal(contents, &configuration); err != nil {
		return domain.SanitizedConfiguration{}, fmt.Errorf("decode sanitized support configuration: %w", err)
	}
	return configuration, nil
}
