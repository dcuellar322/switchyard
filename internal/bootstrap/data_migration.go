package bootstrap

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"switchyard.dev/switchyard/internal/platform/sqlite"
)

// DataMigrationStatus is the preview returned by offline v1 migration tools.
type DataMigrationStatus = sqlite.MigrationStatus

// InspectDataMigration previews embedded database migrations without mutation.
func InspectDataMigration(ctx context.Context, dataDir string) (DataMigrationStatus, error) {
	return sqlite.InspectMigrations(ctx, filepath.Join(dataDir, "switchyard.db"))
}

// BackupData creates a verified, SQLite-consistent manual backup.
func BackupData(ctx context.Context, dataDir, destination string) error {
	return sqlite.Backup(ctx, filepath.Join(dataDir, "switchyard.db"), destination)
}

// MigrateData upgrades alpha/beta data while refusing to race a live daemon.
func MigrateData(ctx context.Context, dataDir string) (DataMigrationStatus, error) {
	lockPath := filepath.Join(dataDir, "daemon.lock")
	if _, err := os.Stat(lockPath); err == nil {
		return DataMigrationStatus{}, fmt.Errorf("daemon lock exists at %s; stop Switchyard before migrating data", lockPath)
	} else if !os.IsNotExist(err) {
		return DataMigrationStatus{}, err
	}
	return sqlite.Migrate(ctx, filepath.Join(dataDir, "switchyard.db"))
}
