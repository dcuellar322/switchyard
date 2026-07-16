package adapters

import (
	"context"
	"errors"
	"os"
	"slices"
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
	base := domain.Plugin{
		ID: "process-fixture", Name: "Process fixture", Version: "1.0.0", ProtocolVersion: pluginsdk.ProtocolVersion,
		ManifestPath: t.TempDir() + "/plugin.json", Executable: os.Args[0],
		Capabilities: []string{string(pluginsdk.CapabilityProjectInspect)}, RequestedScopes: []string{string(pluginsdk.ScopeProjectMetadataRead)},
	}
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
