// Package sqlite owns the local SQLite connection and migration adapter.
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/pressly/goose/v3"
	// Register the pure-Go SQLite driver with database/sql.
	_ "modernc.org/sqlite"

	"switchyard.dev/switchyard/internal/platform/sqlite/generated"
	"switchyard.dev/switchyard/migrations"
)

// Database is a migrated SQLite connection and its typed queries.
type Database struct {
	connection *sql.DB
	queries    *generated.Queries
	path       string
}

// MigrationStatus is the side-effect-free compatibility view used by the v1
// data migration command.
type MigrationStatus struct {
	Path               string `json:"path"`
	CurrentVersion     int64  `json:"currentVersion"`
	TargetVersion      int64  `json:"targetVersion"`
	MigrationRequired  bool   `json:"migrationRequired"`
	PreMigrationBackup string `json:"preMigrationBackup,omitempty"`
}

// Open connects to path, enables local safety pragmas, and applies migrations.
func Open(ctx context.Context, path string) (*Database, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve database path: %w", err)
	}
	file, err := os.OpenFile(absPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("create private sqlite database: %w", err)
	}
	if err := file.Close(); err != nil {
		return nil, fmt.Errorf("close new sqlite database: %w", err)
	}
	if err := os.Chmod(absPath, 0o600); err != nil {
		return nil, fmt.Errorf("restrict sqlite database permissions: %w", err)
	}
	dsn := (&url.URL{
		Scheme:   "file",
		Path:     absPath,
		RawQuery: "_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)",
	}).String()
	connection, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}
	connection.SetMaxOpenConns(1)

	database := &Database{connection: connection, queries: generated.New(connection), path: absPath}
	if err := database.initialize(ctx); err != nil {
		_ = connection.Close()
		return nil, err
	}
	return database, nil
}

func (d *Database) initialize(ctx context.Context) error {
	if err := d.connection.PingContext(ctx); err != nil {
		return fmt.Errorf("ping sqlite database: %w", err)
	}
	provider, err := migrationProvider(d.connection)
	if err != nil {
		return fmt.Errorf("create migration provider: %w", err)
	}
	currentVersion, targetVersion, err := provider.GetVersions(ctx)
	if err != nil {
		return fmt.Errorf("read sqlite migration compatibility: %w", err)
	}
	if currentVersion > targetVersion {
		return fmt.Errorf(
			"database schema version %d is newer than this binary supports (%d); install a compatible Switchyard version before opening this data",
			currentVersion,
			targetVersion,
		)
	}
	if currentVersion > 0 && currentVersion < targetVersion {
		backupPath := preMigrationBackupPath(d.path, currentVersion, targetVersion)
		if err := backupConnection(ctx, d.connection, backupPath); err != nil {
			return fmt.Errorf("create pre-migration backup: %w", err)
		}
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("apply sqlite migrations: %w", err)
	}
	return nil
}

func migrationProvider(connection *sql.DB) (*goose.Provider, error) {
	return goose.NewProvider(
		goose.DialectSQLite3,
		connection,
		migrations.FS,
		goose.WithDisableGlobalRegistry(true),
	)
}

// InspectMigrations reads schema compatibility without applying migrations.
func InspectMigrations(ctx context.Context, path string) (MigrationStatus, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return MigrationStatus{}, fmt.Errorf("resolve database path: %w", err)
	}
	if _, err := os.Stat(absolute); err != nil {
		return MigrationStatus{}, fmt.Errorf("inspect database: %w", err)
	}
	connection, err := sql.Open("sqlite", (&url.URL{Scheme: "file", Path: absolute, RawQuery: "mode=ro&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"}).String())
	if err != nil {
		return MigrationStatus{}, err
	}
	defer func() { _ = connection.Close() }()
	provider, err := migrationProvider(connection)
	if err != nil {
		return MigrationStatus{}, err
	}
	current, target, err := provider.GetVersions(ctx)
	if err != nil {
		return MigrationStatus{}, err
	}
	status := MigrationStatus{Path: absolute, CurrentVersion: current, TargetVersion: target, MigrationRequired: current < target}
	if current > 0 && current < target {
		status.PreMigrationBackup = preMigrationBackupPath(absolute, current, target)
	}
	return status, nil
}

// Migrate upgrades an existing alpha/beta database through the embedded,
// ordered migrations. Open creates and verifies the pre-migration backup.
func Migrate(ctx context.Context, path string) (MigrationStatus, error) {
	database, err := Open(ctx, path)
	if err != nil {
		return MigrationStatus{}, err
	}
	if err := database.Close(); err != nil {
		return MigrationStatus{}, err
	}
	return InspectMigrations(ctx, path)
}

// Backup creates a verified SQLite-consistent backup without changing source data.
func Backup(ctx context.Context, source, destination string) error {
	absolute, err := filepath.Abs(source)
	if err != nil {
		return err
	}
	connection, err := sql.Open("sqlite", (&url.URL{Scheme: "file", Path: absolute, RawQuery: "mode=ro&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"}).String())
	if err != nil {
		return err
	}
	defer func() { _ = connection.Close() }()
	return backupConnection(ctx, connection, destination)
}

func backupConnection(ctx context.Context, connection *sql.DB, destination string) error {
	if err := integrityCheck(ctx, connection); err != nil {
		return fmt.Errorf("source integrity check: %w", err)
	}
	absolute, err := filepath.Abs(destination)
	if err != nil {
		return err
	}
	if _, err := os.Stat(absolute); err == nil {
		return fmt.Errorf("backup destination already exists: %s", absolute)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if _, err := connection.ExecContext(ctx, "VACUUM INTO ?", absolute); err != nil {
		return err
	}
	if err := os.Chmod(absolute, 0o600); err != nil {
		return err
	}
	backup, err := sql.Open("sqlite", (&url.URL{Scheme: "file", Path: absolute, RawQuery: "mode=ro"}).String())
	if err != nil {
		return err
	}
	defer func() { _ = backup.Close() }()
	return integrityCheck(ctx, backup)
}

func integrityCheck(ctx context.Context, connection *sql.DB) error {
	var result string
	if err := connection.QueryRowContext(ctx, "PRAGMA quick_check").Scan(&result); err != nil {
		return err
	}
	if result != "ok" {
		return fmt.Errorf("SQLite quick_check returned %q", result)
	}
	return nil
}

func preMigrationBackupPath(path string, current, target int64) string {
	return fmt.Sprintf("%s.v%d.pre-v%d.bak", path, current, target)
}

// SchemaVersion returns the application schema version recorded by migration.
func (d *Database) SchemaVersion(ctx context.Context) (int64, error) {
	health, err := d.queries.GetSystemHealth(ctx)
	if err != nil {
		return 0, fmt.Errorf("query system health: %w", err)
	}
	return health.SchemaVersion, nil
}

// Close releases the database connection.
func (d *Database) Close() error {
	if err := d.connection.Close(); err != nil {
		return fmt.Errorf("close sqlite database: %w", err)
	}
	return nil
}
