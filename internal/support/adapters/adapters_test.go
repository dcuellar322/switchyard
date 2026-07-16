// Package adapters tests the local support evidence boundaries.
package adapters

import (
	"archive/zip"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"switchyard.dev/switchyard/internal/support/application"
	"switchyard.dev/switchyard/internal/support/domain"
)

func TestInternalLogSourceReturnsOnlyAllowlistedRedactedFields(t *testing.T) {
	t.Parallel()

	dataDir := t.TempDir()
	path := filepath.Join(dataDir, "internal.ndjson")
	line := `{"time":"2026-07-16T12:00:00Z","level":"ERROR","msg":"token=secret-value","component":"runtime","error":"` + dataDir + `/failure","untrusted":"must-not-appear"}` + "\n"
	if err := os.WriteFile(path, []byte(line), 0o600); err != nil {
		t.Fatalf("write internal log: %v", err)
	}
	source, err := NewInternalLogSource(dataDir, func(value string) (string, bool) {
		result := strings.ReplaceAll(value, "secret-value", "[REDACTED]")
		return result, result != value
	})
	if err != nil {
		t.Fatalf("NewInternalLogSource() error = %v", err)
	}
	entries, err := source.List(context.Background(), application.LogQuery{Limit: 10, MinimumLevel: "WARN"})
	if err != nil || len(entries) != 1 {
		t.Fatalf("List() = %#v, %v", entries, err)
	}
	encoded, _ := json.Marshal(entries)
	if strings.Contains(string(encoded), "secret-value") || strings.Contains(string(encoded), dataDir) || strings.Contains(string(encoded), "must-not-appear") {
		t.Fatalf("unsafe entries = %s", encoded)
	}
}

func TestArchiveWriterCreatesPrivateExactBundleAndRefusesOverwrite(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "support.zip")
	preview := domain.Preview{SchemaVersion: domain.PreviewSchema, GeneratedAt: time.Now(), System: domain.SystemIdentity{Version: "1.0.0"}}
	receipt, err := (ArchiveWriter{}).Write(path, preview)
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if receipt.SHA256 == "" || receipt.SizeBytes == 0 {
		t.Fatalf("receipt = %#v", receipt)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat bundle: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("bundle mode = %v", info.Mode().Perm())
	}
	archive, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("open bundle: %v", err)
	}
	defer func() { _ = archive.Close() }()
	if len(archive.File) != 2 || archive.File[0].Name != "manifest.json" || archive.File[1].Name != "internal-errors.ndjson" {
		t.Fatalf("archive files = %#v", archive.File)
	}
	if _, err := (ArchiveWriter{}).Write(path, preview); err == nil {
		t.Fatal("Write() overwrote an existing bundle")
	}
}

func TestConfigurationRoundTripReplacesOnlySanitizedSnapshot(t *testing.T) {
	t.Parallel()

	dataDir := t.TempDir()
	first := domain.SanitizedConfiguration{HTTPBinding: "loopback", IPCMode: "owner-only"}
	second := domain.SanitizedConfiguration{HTTPBinding: "loopback", IPCMode: "owner-only", RoutingEnabled: true}
	if err := WriteConfiguration(dataDir, first); err != nil {
		t.Fatalf("WriteConfiguration(first) error = %v", err)
	}
	if err := WriteConfiguration(dataDir, second); err != nil {
		t.Fatalf("WriteConfiguration(second) error = %v", err)
	}
	got, err := ReadConfiguration(dataDir)
	if err != nil {
		t.Fatalf("ReadConfiguration() error = %v", err)
	}
	if !got.RoutingEnabled || got.HTTPBinding != second.HTTPBinding {
		t.Fatalf("configuration = %#v", got)
	}
	info, err := os.Stat(filepath.Join(dataDir, configurationFile))
	if err != nil {
		t.Fatalf("stat configuration: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("configuration mode = %v", info.Mode().Perm())
	}
}
