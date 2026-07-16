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
	if err := manifest.Validate(); err != nil {
		return domain.Manifest{}, err
	}
	if err := validateSchema(manifest); err != nil {
		return domain.Manifest{}, err
	}
	return manifest, nil
}
