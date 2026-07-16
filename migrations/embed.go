// Package migrations owns the embedded, ordered SQLite migration files.
package migrations

import "embed"

// FS contains every database migration shipped with the binary.
//
//go:embed *.sql
var FS embed.FS
