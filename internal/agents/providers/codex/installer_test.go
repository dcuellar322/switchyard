package codex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	agents "switchyard.dev/switchyard/internal/agents/application"
	"switchyard.dev/switchyard/internal/agents/providers"
)

func TestInstallProjectIsIdempotentAndPreservesConfig(t *testing.T) {
	root := t.TempDir()
	request := providers.InstallRequest{Scope: providers.ScopeProject, Root: root, Home: t.TempDir(), Executable: "/usr/local/bin/switchyard", DataDir: "/tmp/switchyard-data", Profile: agents.ProfileDevelop, AgentID: "reviewer", ProjectIDs: []string{"project-1"}}
	configPath := filepath.Join(root, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("model = \"gpt-5\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err := Install(request); err != nil {
			t.Fatalf("Install() error = %v", err)
		}
	}
	config, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(config)
	if !strings.Contains(text, "model = \"gpt-5\"") || strings.Count(text, "[mcp_servers.switchyard]") != 1 || !strings.Contains(text, `"--profile", "develop"`) {
		t.Fatalf("config = %s", text)
	}
	for _, path := range []string{filepath.Join(root, ".agents", "skills", "switchyard-operate", "SKILL.md"), filepath.Join(root, "AGENTS.md")} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s: %v", path, err)
		}
	}
}

func TestInstallRejectsUnmanagedServerCollision(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("[mcp_servers.switchyard]\ncommand = \"other\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Install(providers.InstallRequest{Scope: providers.ScopeProject, Root: root, Home: t.TempDir(), Executable: "/bin/switchyard", DataDir: "/tmp/data", Profile: agents.ProfileObserve, AgentID: "agent"})
	if err == nil {
		t.Fatal("Install() expected collision error")
	}
}
