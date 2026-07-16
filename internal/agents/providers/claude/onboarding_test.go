package claude

import (
	"context"
	"strings"
	"testing"
)

func TestDecodeClaudeResultReadsStructuredOutputAndUsage(t *testing.T) {
	output, model, usage, err := decodeClaudeResult([]byte(`{"model":"claude-fixture","structured_output":{"version":"switchyard.dev/ai-proposal/v1alpha1"},"usage":{"input_tokens":10,"output_tokens":5},"total_cost_usd":0.01}`))
	if err != nil {
		t.Fatal(err)
	}
	if model != "claude-fixture" || usage.InputTokens != 10 || usage.CostUSD != .01 || !strings.Contains(string(output), "ai-proposal") {
		t.Fatalf("output=%s model=%s usage=%#v", output, model, usage)
	}
}

func TestUnavailableClaudeIsCapabilityResult(t *testing.T) {
	provider := NewProposalProvider(ProposalConfig{Executable: "/definitely/missing/claude"})
	descriptor := provider.Descriptor(context.Background())
	if descriptor.Available || descriptor.Reason == "" {
		t.Fatalf("descriptor = %#v", descriptor)
	}
}
