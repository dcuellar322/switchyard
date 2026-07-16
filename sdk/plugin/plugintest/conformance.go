// Package plugintest provides a reusable protocol conformance harness.
package plugintest

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"switchyard.dev/switchyard/sdk/plugin"
)

// RunConformance verifies negotiation, declared reads, denied undeclared
// mutation, and malformed-protocol isolation for a plugin handler.
func RunConformance(t *testing.T, manifest plugin.Manifest, handler plugin.Handler) {
	t.Helper()
	t.Run("declared inspection", func(t *testing.T) {
		input := strings.NewReader(
			`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"switchyard.plugin/v1","hostVersion":"test","grantedScopes":["project.metadata.read"]}}` + "\n" +
				`{"jsonrpc":"2.0","id":2,"method":"project.inspect","params":{"project":{"id":"fixture","displayName":"Fixture"}}}` + "\n",
		)
		var output bytes.Buffer
		if err := plugin.Serve(context.Background(), input, &output, manifest, handler); err != nil {
			t.Fatal(err)
		}
		responses := decodeResponses(t, output.Bytes())
		if len(responses) != 2 || responses[1].Error != nil {
			t.Fatalf("responses = %#v", responses)
		}
	})
	t.Run("undeclared mutation denied", func(t *testing.T) {
		input := strings.NewReader(
			`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"switchyard.plugin/v1","hostVersion":"test","grantedScopes":["project.metadata.read"]}}` + "\n" +
				`{"jsonrpc":"2.0","id":2,"method":"project.operate","params":{"project":{"id":"fixture","displayName":"Fixture"},"action":"echo","input":{}}}` + "\n",
		)
		var output bytes.Buffer
		if err := plugin.Serve(context.Background(), input, &output, manifest, handler); err != nil {
			t.Fatal(err)
		}
		responses := decodeResponses(t, output.Bytes())
		if len(responses) != 2 || responses[1].Error == nil || responses[1].Error.Code != -32003 {
			t.Fatalf("responses = %#v", responses)
		}
	})
	t.Run("malformed request contained", func(t *testing.T) {
		var output bytes.Buffer
		if err := plugin.Serve(context.Background(), strings.NewReader("not-json\n"), &output, manifest, handler); err != nil {
			t.Fatal(err)
		}
		responses := decodeResponses(t, output.Bytes())
		if len(responses) != 1 || responses[0].Error == nil || responses[0].Error.Code != -32700 {
			t.Fatalf("responses = %#v", responses)
		}
	})
}

type response struct {
	Error *plugin.RPCError `json:"error"`
}

func decodeResponses(t *testing.T, raw []byte) []response {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(raw))
	result := []response{}
	for decoder.More() {
		var value response
		if err := decoder.Decode(&value); err != nil {
			t.Fatal(err)
		}
		result = append(result, value)
	}
	return result
}
