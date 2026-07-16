// Command schema-gen generates the canonical project manifest JSON Schema.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/invopop/jsonschema"

	"switchyard.dev/switchyard/internal/manifest/domain"
)

func main() {
	reflector := jsonschema.Reflector{
		AllowAdditionalProperties:  false,
		RequiredFromJSONSchemaTags: true,
	}
	schema := reflector.Reflect(&domain.Manifest{})
	schema.ID = jsonschema.ID("https://switchyard.dev/schema/project.v1.json")
	contents, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		fatal("encode manifest schema", err)
	}
	contents = append(contents, '\n')
	if err := os.WriteFile("internal/manifest/schema/project.schema.json", contents, 0o644); err != nil {
		fatal("write manifest schema", err)
	}
}

func fatal(message string, err error) {
	_, _ = fmt.Fprintf(os.Stderr, "%s: %v\n", message, err)
	os.Exit(1)
}
