package plugin_test

import (
	"encoding/json"
	"testing"
	"time"

	"switchyard.dev/switchyard/sdk/plugin"
	"switchyard.dev/switchyard/sdk/plugin/plugintest"
)

type fixtureHandler struct{}

func (fixtureHandler) Health() plugin.HealthResult {
	return plugin.HealthResult{Status: "healthy", Checked: time.Now().UTC()}
}
func (fixtureHandler) Inspect(request plugin.InspectRequest) (plugin.InspectResult, error) {
	return plugin.InspectResult{Summary: request.Project.DisplayName, Facts: []plugin.Fact{}, Actions: []plugin.Action{}, Warnings: []string{}}, nil
}
func (fixtureHandler) Operate(plugin.OperateRequest) (plugin.OperateResult, error) {
	return plugin.OperateResult{Status: "succeeded", Summary: "done", Output: json.RawMessage(`{}`)}, nil
}

func TestConformanceHarness(t *testing.T) {
	plugintest.RunConformance(t, plugin.Manifest{
		SchemaVersion: plugin.ManifestVersion, ID: "fixture", Name: "Fixture", Version: "1.0.0", ProtocolVersion: plugin.ProtocolVersion,
		Capabilities:    []plugin.Capability{plugin.CapabilityProjectInspect, plugin.CapabilityProjectOperate},
		RequestedScopes: []plugin.Scope{plugin.ScopeProjectMetadataRead, plugin.ScopeProjectOperate},
	}, fixtureHandler{})
}

func TestManifestRejectsCapabilityWithoutScope(t *testing.T) {
	manifest := plugin.Manifest{SchemaVersion: plugin.ManifestVersion, ID: "fixture", Name: "Fixture", Version: "1.0.0", ProtocolVersion: plugin.ProtocolVersion, Capabilities: []plugin.Capability{plugin.CapabilityProjectOperate}}
	if err := manifest.Validate(); err == nil {
		t.Fatal("expected capability/scope validation error")
	}
}
