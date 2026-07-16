package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	agents "switchyard.dev/switchyard/internal/agents/application"
	"switchyard.dev/switchyard/internal/agents/providers"
)

func TestInstallProjectMergesConfigAndImportsAgents(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".mcp.json"), []byte(`{"mcpServers":{"other":{"type":"stdio","command":"other"}},"setting":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	request := providers.InstallRequest{Scope: providers.ScopeProject, Root: root, Home: t.TempDir(), Executable: "/usr/local/bin/switchyard", DataDir: "/tmp/switchyard-data", Profile: agents.ProfileObserve, AgentID: "claude"}
	for range 2 {
		if _, err := Install(request); err != nil {
			t.Fatalf("Install() error = %v", err)
		}
	}
	content, err := os.ReadFile(filepath.Join(root, ".mcp.json"))
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(content, &document); err != nil {
		t.Fatal(err)
	}
	servers := document["mcpServers"].(map[string]any)
	if len(servers) != 2 || servers["switchyard"] == nil || document["setting"] != true {
		t.Fatalf("config = %#v", document)
	}
	claudeFile, err := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(claudeFile), "@AGENTS.md") != 1 {
		t.Fatalf("CLAUDE.md = %q", claudeFile)
	}
}
