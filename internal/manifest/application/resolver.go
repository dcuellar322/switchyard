package application

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"switchyard.dev/switchyard/internal/manifest/domain"
)

const (
	portableManifest = ".switchyard/project.yml"
	localManifest    = ".switchyard/project.local.yml"
)

// EffectiveManifest includes every effective field's winning source.
type EffectiveManifest struct {
	Manifest   domain.Manifest   `json:"manifest"`
	Provenance map[string]string `json:"provenance"`
	Sources    []SourceDocument  `json:"sources"`
}

// SourceDocument is one existing precedence layer.
type SourceDocument struct {
	Name string `json:"name"`
	Path string `json:"path,omitempty"`
}

// Resolve merges discovery, accepted inference, portable YAML, local YAML, and runtime overrides.
func Resolve(root string, discovered, accepted domain.Manifest, runtimeOverride []byte) (EffectiveManifest, error) {
	merged := make(map[string]any)
	provenance := make(map[string]string)
	var sources []SourceDocument
	for _, source := range []struct {
		name     string
		path     string
		manifest *domain.Manifest
		contents []byte
	}{
		{name: "discovery", manifest: &discovered},
		{name: "accepted-inference", manifest: &accepted},
		{name: "portable-manifest", path: filepath.Join(root, portableManifest)},
		{name: "local-overlay", path: filepath.Join(root, localManifest)},
		{name: "runtime-override", contents: runtimeOverride},
	} {
		value, exists, err := sourceMap(source.path, source.contents, source.manifest)
		if err != nil {
			return EffectiveManifest{}, err
		}
		if !exists {
			continue
		}
		mergeMap(merged, value, "", source.name, provenance)
		sources = append(sources, SourceDocument{Name: source.name, Path: relativePath(root, source.path)})
	}
	manifest, err := mapToManifest(merged)
	if err != nil {
		return EffectiveManifest{}, err
	}
	return EffectiveManifest{Manifest: manifest, Provenance: provenance, Sources: sources}, nil
}

func sourceMap(path string, contents []byte, manifest *domain.Manifest) (map[string]any, bool, error) {
	if manifest != nil {
		if manifest.SchemaVersion == "" {
			return nil, false, nil
		}
		encoded, err := json.Marshal(manifest)
		if err != nil {
			return nil, false, err
		}
		var value map[string]any
		if err := json.Unmarshal(encoded, &value); err != nil {
			return nil, false, err
		}
		return value, true, nil
	}
	if len(contents) == 0 && path != "" {
		read, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		if err != nil {
			return nil, false, fmt.Errorf("read manifest source %s: %w", path, err)
		}
		contents = read
	}
	if len(contents) == 0 {
		return nil, false, nil
	}
	value, err := decodeMap(contents)
	return value, true, err
}

func mergeMap(target, source map[string]any, prefix, sourceName string, provenance map[string]string) {
	keys := make([]string, 0, len(source))
	for key := range source {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		path := prefix + "/" + escapePointer(key)
		sourceChild, sourceIsMap := source[key].(map[string]any)
		targetChild, targetIsMap := target[key].(map[string]any)
		if sourceIsMap {
			if !targetIsMap {
				targetChild = make(map[string]any)
				target[key] = targetChild
			}
			mergeMap(targetChild, sourceChild, path, sourceName, provenance)
			continue
		}
		target[key] = source[key]
		provenance[path] = sourceName
	}
}

func escapePointer(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, "~", "~0"), "/", "~1")
}

func relativePath(root, path string) string {
	if path == "" {
		return ""
	}
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return relative
}
