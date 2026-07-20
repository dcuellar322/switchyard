// Package adapters tests the local support evidence boundaries.
package adapters

import (
	"archive/zip"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
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
	line, err := json.Marshal(map[string]string{
		"time":      "2026-07-16T12:00:00Z",
		"level":     "ERROR",
		"msg":       "token=secret-value",
		"component": "runtime",
		"error":     filepath.Join(dataDir, "failure"),
		"untrusted": "must-not-appear",
	})
	if err != nil {
		t.Fatalf("encode internal log fixture: %v", err)
	}
	if err := os.WriteFile(path, append(line, '\n'), 0o600); err != nil {
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
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
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

func TestCommitExclusiveNeverReplacesExistingDestination(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	source := filepath.Join(directory, "source")
	destination := filepath.Join(directory, "destination")
	if err := os.WriteFile(source, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := commitExclusive(source, destination); err == nil {
		t.Fatal("exclusive commit replaced an existing destination")
	}
	contents, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "existing" {
		t.Fatalf("destination contents = %q", contents)
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
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("configuration mode = %v", info.Mode().Perm())
	}
}
