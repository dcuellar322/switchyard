package codex

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	agents "switchyard.dev/switchyard/internal/agents/application"
	providerProcess "switchyard.dev/switchyard/internal/agents/providers/process"
)

func TestProposalProviderUsesReadOnlyEmptySchemaConstrainedInvocation(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "codex")
	if err := os.WriteFile(executable, []byte("fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SWITCHYARD_UNRELATED_SECRET", "must-not-leak")
	runner := &capturingRunner{output: json.RawMessage(`{"version":"switchyard.dev/ai-proposal/v1alpha1"}`), t: t}
	provider := NewProposalProvider(ProposalConfig{Executable: executable, Model: "fixture-model", Runner: runner})
	result, err := provider.ProposeManifest(context.Background(), agents.ProviderRequest{
		Bundle: json.RawMessage(`{"unique":"bundle-only-canary"}`), OutputSchema: json.RawMessage(`{"type":"object"}`), Limits: agents.Limits{OutputBytes: 4096},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(runner.command.Args, "read-only") || !slices.Contains(runner.command.Args, "--ignore-user-config") || !slices.Contains(runner.command.Args, "--ignore-rules") || !slices.Contains(runner.command.Args, "--ephemeral") {
		t.Fatalf("args = %v", runner.command.Args)
	}
	if strings.Contains(strings.Join(runner.command.Args, " "), "bundle-only-canary") || !strings.Contains(string(runner.command.Stdin), "bundle-only-canary") {
		t.Fatal("bundle must travel only through stdin")
	}
	if strings.Contains(strings.Join(runner.command.Environment, "\n"), "SWITCHYARD_UNRELATED_SECRET") {
		t.Fatal("daemon environment leaked to provider")
	}
	if result.Model != "fixture-model" || !strings.Contains(string(result.Output), "ai-proposal") {
		t.Fatalf("result = %#v", result)
	}
}

type capturingRunner struct {
	command providerProcess.Command
	output  []byte
	t       *testing.T
}

func (r *capturingRunner) Run(_ context.Context, command providerProcess.Command) (providerProcess.Result, error) {
	r.command = command
	index := slices.Index(command.Args, "--output-last-message")
	if index < 0 || index+1 >= len(command.Args) {
		r.t.Fatal("missing output file argument")
	}
	if err := os.WriteFile(command.Args[index+1], r.output, 0o600); err != nil {
		r.t.Fatal(err)
	}
	schemaIndex := slices.Index(command.Args, "--output-schema")
	if schemaIndex < 0 {
		r.t.Fatal("missing output schema")
	}
	info, err := os.Stat(command.Args[schemaIndex+1])
	if err != nil || info.Mode().Perm() != 0o600 {
		r.t.Fatalf("schema mode = %v err=%v", info, err)
	}
	return providerProcess.Result{}, nil
}
