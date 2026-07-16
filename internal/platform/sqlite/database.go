// Package sqlite owns the local SQLite connection and migration adapter.
package sqlite

import (
	"context"
	"database/sql"
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
	provider, err := goose.NewProvider(
		goose.DialectSQLite3,
		d.connection,
		migrations.FS,
		goose.WithDisableGlobalRegistry(true),
	)
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
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("apply sqlite migrations: %w", err)
	}
	return nil
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
