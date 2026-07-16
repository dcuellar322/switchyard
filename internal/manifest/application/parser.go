// Package application parses, resolves, and validates project manifests.
package application

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"

	"switchyard.dev/switchyard/internal/manifest/domain"
)

// ParseYAML decodes exactly one manifest and rejects unknown fields.
func ParseYAML(contents []byte) (domain.Manifest, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(contents))
	decoder.KnownFields(true)
	var manifest domain.Manifest
	if err := decoder.Decode(&manifest); err != nil {
		return domain.Manifest{}, fmt.Errorf("decode manifest: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return domain.Manifest{}, errors.New("manifest must contain exactly one YAML document")
	}
	normalizeManifestVersion(&manifest)
	if err := manifest.Validate(); err != nil {
		return domain.Manifest{}, err
	}
	if err := validateSchema(manifest); err != nil {
		return domain.Manifest{}, err
	}
	return manifest, nil
}

func decodeMap(contents []byte) (map[string]any, error) {
	var value map[string]any
	decoder := yaml.NewDecoder(bytes.NewReader(contents))
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("decode manifest source: %w", err)
	}
	return value, nil
}

func mapToManifest(value map[string]any) (domain.Manifest, error) {
	contents, err := json.Marshal(value)
	if err != nil {
		return domain.Manifest{}, fmt.Errorf("encode effective manifest: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	var manifest domain.Manifest
	if err := decoder.Decode(&manifest); err != nil {
		return domain.Manifest{}, fmt.Errorf("decode effective manifest: %w", err)
	}
	normalizeManifestVersion(&manifest)
	if err := manifest.Validate(); err != nil {
		return domain.Manifest{}, err
	}
	if err := validateSchema(manifest); err != nil {
		return domain.Manifest{}, err
	}
	return manifest, nil
}

func normalizeManifestVersion(manifest *domain.Manifest) bool {
	if manifest.SchemaVersion != domain.LegacySchemaVersion {
		return false
	}
	manifest.SchemaVersion = domain.SchemaVersion
	return true
}

// MigrateYAML upgrades a valid alpha/beta manifest to the stable v1 schema
// identifier while preserving its YAML node order and comments.
func MigrateYAML(contents []byte) ([]byte, bool, error) {
	if _, err := ParseYAML(contents); err != nil {
		return nil, false, err
	}
	var document yaml.Node
	if err := yaml.Unmarshal(contents, &document); err != nil {
		return nil, false, fmt.Errorf("decode manifest migration document: %w", err)
	}
	changed := migrateSchemaNode(&document)
	if !changed {
		return append([]byte(nil), contents...), false, nil
	}
	var output bytes.Buffer
	encoder := yaml.NewEncoder(&output)
	encoder.SetIndent(2)
	if err := encoder.Encode(&document); err != nil {
		return nil, false, fmt.Errorf("encode migrated manifest: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return nil, false, err
	}
	return output.Bytes(), true, nil
}

func migrateSchemaNode(document *yaml.Node) bool {
	if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return false
	}
	root := document.Content[0]
	for index := 0; index+1 < len(root.Content); index += 2 {
		if root.Content[index].Value == "schemaVersion" && root.Content[index+1].Value == domain.LegacySchemaVersion {
			root.Content[index+1].Value = domain.SchemaVersion
			return true
		}
	}
	return false
}
