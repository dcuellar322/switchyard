// Command switchyard-fixture-plugin is a deliberately small external adapter
// used by the SDK documentation and host conformance suite.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"switchyard.dev/switchyard/sdk/plugin"
)

var manifest = plugin.Manifest{
	SchemaVersion: plugin.ManifestVersion, ID: "fixture-inspector", Name: "Fixture inspector", Version: "1.0.0", ProtocolVersion: plugin.ProtocolVersion,
	Capabilities:    []plugin.Capability{plugin.CapabilityProjectInspect, plugin.CapabilityProjectOperate},
	RequestedScopes: []plugin.Scope{plugin.ScopeProjectMetadataRead, plugin.ScopeProjectFilesRead, plugin.ScopeProjectOperate},
}

type handler struct{}

func (handler) Health() plugin.HealthResult {
	return plugin.HealthResult{Status: "healthy", Message: "fixture adapter ready", Checked: time.Now().UTC()}
}

func (handler) Inspect(request plugin.InspectRequest) (plugin.InspectResult, error) {
	if request.Project.Root == "" {
		return plugin.InspectResult{}, errors.New("fixture inspection requires project.files.read")
	}
	facts := []plugin.Fact{}
	for _, name := range []string{"package.json", "go.mod", ".switchyard/project.yml"} {
		_, err := os.Stat(filepath.Join(request.Project.Root, filepath.FromSlash(name)))
		if err == nil {
			facts = append(facts, plugin.Fact{ID: "file." + filepath.Base(name), Label: "Detected file", Value: name, Source: name})
		} else if !errors.Is(err, os.ErrNotExist) {
			return plugin.InspectResult{}, err
		}
	}
	return plugin.InspectResult{
		Summary: "Fixture adapter inspected declared project files", Facts: facts,
		Actions:  []plugin.Action{{ID: "fixture.echo", Name: "Echo fixture input", Description: "Returns bounded JSON input without executing a command.", Risk: "mutating"}},
		Warnings: []string{}, Observed: time.Now().UTC(),
	}, nil
}

func (handler) Operate(request plugin.OperateRequest) (plugin.OperateResult, error) {
	if request.Action != "fixture.echo" {
		return plugin.OperateResult{}, errors.New("unknown fixture action")
	}
	if len(request.Input) == 0 {
		request.Input = json.RawMessage(`{}`)
	}
	return plugin.OperateResult{Status: "succeeded", Summary: "Fixture input returned", Output: request.Input}, nil
}

func main() {
	if err := plugin.Serve(context.Background(), os.Stdin, os.Stdout, manifest, handler{}); err != nil {
		_, _ = os.Stderr.WriteString("fixture plugin stopped: " + err.Error() + "\n")
		os.Exit(1)
	}
}
