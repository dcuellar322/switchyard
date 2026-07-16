package application

import (
	"fmt"
	"os"
	"path/filepath"
)

// MigrationResult describes a previewed or applied portable-manifest upgrade.
type MigrationResult struct {
	Path       string `json:"path"`
	From       string `json:"from"`
	To         string `json:"to"`
	Changed    bool   `json:"changed"`
	Applied    bool   `json:"applied"`
	BackupPath string `json:"backupPath,omitempty"`
	Preview    string `json:"preview,omitempty"`
}

// MigrateFile previews or safely applies the alpha/beta-to-v1 manifest
// migration. Applying always creates a non-overwriting backup first.
func MigrateFile(path string, apply bool) (MigrationResult, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return MigrationResult{}, fmt.Errorf("resolve manifest path: %w", err)
	}
	contents, err := os.ReadFile(absolute)
	if err != nil {
		return MigrationResult{}, fmt.Errorf("read manifest: %w", err)
	}
	migrated, changed, err := MigrateYAML(contents)
	if err != nil {
		return MigrationResult{}, err
	}
	result := MigrationResult{Path: absolute, From: "switchyard.dev/v1alpha1", To: "switchyard.dev/v1", Changed: changed}
	if !changed || !apply {
		if changed {
			result.Preview = string(migrated)
		}
		return result, nil
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return MigrationResult{}, err
	}
	backupPath := absolute + ".v1alpha1.bak"
	if err := writeExclusiveFile(backupPath, contents, info.Mode().Perm()); err != nil {
		return MigrationResult{}, fmt.Errorf("create migration backup: %w", err)
	}
	if err := replaceFile(absolute, migrated, info.Mode().Perm()); err != nil {
		return MigrationResult{}, fmt.Errorf("replace migrated manifest (backup retained at %s): %w", backupPath, err)
	}
	result.Applied, result.BackupPath = true, backupPath
	return result, nil
}

func writeExclusiveFile(path string, contents []byte, mode os.FileMode) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	if _, err := file.Write(contents); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func replaceFile(path string, contents []byte, mode os.FileMode) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".switchyard-manifest-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()
	if err := temporary.Chmod(mode); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(contents); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}
