// Package schema embeds the generated portable manifest contract.
package schema

import _ "embed"

// Project is the generated JSON Schema for a project manifest.
//
//go:embed project.schema.json
var Project []byte
