package adapters

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	pluginsApplication "switchyard.dev/switchyard/internal/plugins/application"
	"switchyard.dev/switchyard/internal/plugins/domain"
	pluginsdk "switchyard.dev/switchyard/sdk/plugin"
)

type processFixtureHandler struct{}

func (processFixtureHandler) Health() pluginsdk.HealthResult {
	return pluginsdk.HealthResult{Status: "healthy", Message: "ready", Checked: time.Now().UTC()}
}
func (processFixtureHandler) Inspect(pluginsdk.InspectRequest) (pluginsdk.InspectResult, error) {
	return pluginsdk.InspectResult{}, errors.New("not used")
}
func (processFixtureHandler) Operate(pluginsdk.OperateRequest) (pluginsdk.OperateResult, error) {
	return pluginsdk.OperateResult{}, errors.New("not used")
}

func TestPluginProcessHelper(_ *testing.T) {
	if os.Getenv("SWITCHYARD_PLUGIN_PROTOCOL") != pluginsdk.ProtocolVersion {
		return
	}
	manifest := pluginsdk.Manifest{
		SchemaVersion: pluginsdk.ManifestVersion, ID: "process-fixture", Name: "Process fixture", Version: "1.0.0", ProtocolVersion: pluginsdk.ProtocolVersion,
		Capabilities: []pluginsdk.Capability{pluginsdk.CapabilityProjectInspect}, RequestedScopes: []pluginsdk.Scope{pluginsdk.ScopeProjectMetadataRead},
	}
	if err := pluginsdk.Serve(context.Background(), os.Stdin, os.Stdout, manifest, processFixtureHandler{}); err != nil {
		os.Exit(2)
	}
}

func TestCrashingPluginProcessHelper(_ *testing.T) {
	if os.Getenv("SWITCHYARD_PLUGIN_PROTOCOL") == pluginsdk.ProtocolVersion {
		_, _ = os.Stderr.WriteString("token=top-secret\n")
		os.Exit(17)
	}
}

type redactFixture struct{}

func (redactFixture) RedactText(value string) (string, bool) {
	if value == "" {
		return value, false
	}
	return "[redacted]", true
}

func TestProcessRunnerNegotiatesAndContainsCrash(t *testing.T) {
	base := processFixturePlugin(t)
	base.ID = "process-fixture"
	base.Name = "Process fixture"
	base.Version = "1.0.0"
	base.ProtocolVersion = pluginsdk.ProtocolVersion
	base.Capabilities = []string{string(pluginsdk.CapabilityProjectInspect)}
	base.RequestedScopes = []string{string(pluginsdk.ScopeProjectMetadataRead)}
	runner := NewProcessRunner("test-host", redactFixture{})
	base.Arguments = []string{"-test.run=^TestPluginProcessHelper$"}
	var health pluginsdk.HealthResult
	logs, err := runner.Call(t.Context(), pluginsApplication.Invocation{Plugin: base, Scopes: []pluginsdk.Scope{pluginsdk.ScopeProjectMetadataRead}, Method: "plugin.health", Params: struct{}{}, Result: &health})
	if err != nil || health.Status != "healthy" || len(logs) != 0 {
		t.Fatalf("Call() health=%#v logs=%#v err=%v", health, logs, err)
	}
	base.Arguments = []string{"-test.run=^TestCrashingPluginProcessHelper$"}
	logs, err = runner.Call(t.Context(), pluginsApplication.Invocation{Plugin: base, Scopes: []pluginsdk.Scope{pluginsdk.ScopeProjectMetadataRead}, Method: "plugin.health", Params: struct{}{}, Result: &health})
	if err == nil || !slices.ContainsFunc(logs, func(entry domain.LogEntry) bool { return entry.Message == "[redacted]" }) {
		t.Fatalf("crash logs=%#v err=%v", logs, err)
	}
}

func TestProcessRunnerRejectsPackageChangedAfterReview(t *testing.T) {
	base := processFixturePlugin(t)
	if err := os.WriteFile(base.ManifestPath, []byte(`{"changed":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := NewProcessRunner("test-host", redactFixture{}).Call(t.Context(), pluginsApplication.Invocation{Plugin: base})
	if err == nil || !strings.Contains(err.Error(), "fingerprint changed") {
		t.Fatalf("changed package error = %v", err)
	}
}

func processFixturePlugin(t *testing.T) domain.Plugin {
	t.Helper()
	directory := t.TempDir()
	executable := filepath.Join(directory, "fixture")
	source, err := os.Open(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = source.Close() }()
	target, err := os.OpenFile(executable, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o700)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(target, source); err != nil {
		_ = target.Close()
		t.Fatal(err)
	}
	if err := target.Close(); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(directory, "plugin.json")
	manifest := []byte(`{"schemaVersion":"switchyard.plugin/v1"}`)
	if err := os.WriteFile(manifestPath, manifest, 0o600); err != nil {
		t.Fatal(err)
	}
	fingerprint, err := executableFingerprint(manifest, executable)
	if err != nil {
		t.Fatal(err)
	}
	return domain.Plugin{
		ID: "process-fixture", Name: "Process fixture", Version: "1.0.0", ProtocolVersion: pluginsdk.ProtocolVersion,
		ManifestPath: manifestPath, Executable: executable, Fingerprint: fingerprint,
		Capabilities: []string{string(pluginsdk.CapabilityProjectInspect)}, RequestedScopes: []string{string(pluginsdk.ScopeProjectMetadataRead)},
	}
}
