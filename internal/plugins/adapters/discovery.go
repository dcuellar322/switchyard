// Package adapters contains filesystem and process boundaries for plugins.
package adapters

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"switchyard.dev/switchyard/internal/plugins/domain"
	pluginsdk "switchyard.dev/switchyard/sdk/plugin"
)

const maxPluginExecutableBytes = 128 << 20

// DirectoryDiscovery reads packages installed under one user-owned directory.
type DirectoryDiscovery struct{ directory string }

// NewDirectoryDiscovery creates a deterministic package-directory scanner.
func NewDirectoryDiscovery(directory string) *DirectoryDiscovery {
	return &DirectoryDiscovery{directory: directory}
}

// Discover parses manifests and executable identities without starting code.
func (d *DirectoryDiscovery) Discover(ctx context.Context) ([]domain.Plugin, error) {
	if err := os.MkdirAll(d.directory, 0o700); err != nil {
		return nil, fmt.Errorf("create plugin directory: %w", err)
	}
	entries, err := os.ReadDir(d.directory)
	if err != nil {
		return nil, fmt.Errorf("read plugin directory: %w", err)
	}
	if len(entries) > 100 {
		return nil, errors.New("plugin directory exceeds 100 package limit")
	}
	result := make([]domain.Plugin, 0, len(entries))
	seen := map[string]string{}
	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		if !entry.IsDir() {
			continue
		}
		manifestPath := filepath.Join(d.directory, entry.Name(), "plugin.json")
		current, loadErr := loadPlugin(manifestPath)
		if errors.Is(loadErr, os.ErrNotExist) {
			continue
		}
		if loadErr != nil {
			return nil, loadErr
		}
		if previous, exists := seen[current.ID]; exists {
			return nil, fmt.Errorf("duplicate plugin id %q in %s and %s", current.ID, previous, manifestPath)
		}
		seen[current.ID] = manifestPath
		result = append(result, current)
	}
	slices.SortFunc(result, func(left, right domain.Plugin) int { return strings.Compare(left.ID, right.ID) })
	return result, nil
}

func loadPlugin(manifestPath string) (domain.Plugin, error) {
	raw, err := readBoundedFile(manifestPath, 64<<10)
	if err != nil {
		return domain.Plugin{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var manifest pluginsdk.Manifest
	if err := decoder.Decode(&manifest); err != nil {
		return domain.Plugin{}, fmt.Errorf("decode plugin manifest %s: %w", manifestPath, err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return domain.Plugin{}, fmt.Errorf("decode plugin manifest %s: multiple JSON documents are not allowed", manifestPath)
	}
	if manifest.Executable == "" {
		return domain.Plugin{}, fmt.Errorf("plugin manifest %s requires executable", manifestPath)
	}
	if err := manifest.Validate(); err != nil {
		return domain.Plugin{}, fmt.Errorf("validate plugin manifest %s: %w", manifestPath, err)
	}
	executable, err := resolveExecutable(filepath.Dir(manifestPath), manifest.Executable)
	if err != nil {
		return domain.Plugin{}, fmt.Errorf("validate plugin executable for %s: %w", manifest.ID, err)
	}
	fingerprint, err := executableFingerprint(raw, executable)
	if err != nil {
		return domain.Plugin{}, fmt.Errorf("fingerprint plugin %s: %w", manifest.ID, err)
	}
	now := time.Now().UTC()
	return domain.Plugin{
		ID: manifest.ID, Name: manifest.Name, Version: manifest.Version, ProtocolVersion: manifest.ProtocolVersion,
		ManifestPath: manifestPath, Executable: executable, Arguments: slices.Clone(manifest.Arguments), Fingerprint: fingerprint,
		Capabilities: stringCapabilities(manifest.Capabilities), RequestedScopes: stringScopes(manifest.RequestedScopes),
		Available: true, Trust: domain.TrustUntrusted, Health: domain.HealthUnknown, DiscoveredAt: now, UpdatedAt: now,
	}, nil
}

func resolveExecutable(directory, value string) (string, error) {
	if filepath.IsAbs(value) || strings.ContainsRune(value, 0) {
		return "", errors.New("executable must be relative to its plugin package")
	}
	root, err := filepath.Abs(directory)
	if err != nil {
		return "", err
	}
	candidate, err := filepath.Abs(filepath.Join(root, filepath.Clean(value)))
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(root, candidate)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", errors.New("executable escapes plugin package")
	}
	info, err := os.Lstat(candidate)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("executable must be a regular file, not a symlink")
	}
	if err := validateExecutablePermissions(info.Mode()); err != nil {
		return "", err
	}
	return candidate, nil
}

func executableFingerprint(manifest []byte, executable string) (string, error) {
	hash := sha256.New()
	_, _ = hash.Write(manifest)
	file, err := os.Open(executable)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()
	written, err := io.Copy(hash, io.LimitReader(file, maxPluginExecutableBytes+1))
	if err != nil {
		return "", err
	}
	if written > maxPluginExecutableBytes {
		return "", errors.New("plugin executable exceeds 128 MiB fingerprint limit")
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func readBoundedFile(path string, limit int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	raw, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > limit {
		return nil, errors.New("file exceeds plugin limit")
	}
	return raw, nil
}

func stringCapabilities(values []pluginsdk.Capability) []string {
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = string(value)
	}
	return result
}
func stringScopes(values []pluginsdk.Scope) []string {
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = string(value)
	}
	return result
}
