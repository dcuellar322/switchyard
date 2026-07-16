// Package claude installs Switchyard MCP and skill configuration for Claude Code.
package claude

import (
	"bytes"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"

	"switchyard.dev/switchyard/internal/agents/guidance"
	"switchyard.dev/switchyard/internal/agents/providers"
)

type mcpConfig struct {
	Type    string   `json:"type"`
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

// Install writes an idempotent provider config, shared skill, and project guidance.
func Install(request providers.InstallRequest) ([]string, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}
	configPath := filepath.Join(request.Root, ".mcp.json")
	skillPath := filepath.Join(request.Root, ".claude", "skills", "switchyard-operate", "SKILL.md")
	if request.Scope == providers.ScopeUser {
		configPath = filepath.Join(request.Home, ".claude.json")
		skillPath = filepath.Join(request.Home, ".claude", "skills", "switchyard-operate", "SKILL.md")
	}
	if err := installConfig(configPath, request); err != nil {
		return nil, err
	}
	if err := guidance.WriteFileAtomic(skillPath, guidance.Skill(), 0o644); err != nil {
		return nil, err
	}
	paths := []string{configPath, skillPath}
	if request.Scope == providers.ScopeProject {
		agentsPath := filepath.Join(request.Root, "AGENTS.md")
		if err := installProjectGuidance(agentsPath); err != nil {
			return nil, err
		}
		claudePath := filepath.Join(request.Root, "CLAUDE.md")
		if err := installAgentsImport(claudePath); err != nil {
			return nil, err
		}
		paths = append(paths, agentsPath, claudePath)
	}
	return paths, nil
}

func installConfig(path string, request providers.InstallRequest) error {
	existing, err := guidance.ReadOptionalRegularFile(path)
	if err != nil {
		return err
	}
	document := map[string]any{}
	if len(bytes.TrimSpace(existing)) > 0 {
		if err := json.Unmarshal(existing, &document); err != nil {
			return errors.New("existing Claude MCP config is not valid JSON")
		}
	}
	servers, ok := document["mcpServers"].(map[string]any)
	if !ok {
		if document["mcpServers"] != nil {
			return errors.New("existing Claude mcpServers value is not an object")
		}
		servers = map[string]any{}
		document["mcpServers"] = servers
	}
	servers["switchyard"] = mcpConfig{Type: "stdio", Command: request.Executable, Args: request.MCPArgs("claude")}
	updated, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return err
	}
	updated = append(updated, '\n')
	return guidance.WriteFileAtomic(path, updated, 0o644)
}

func installProjectGuidance(path string) error {
	existing, err := guidance.ReadOptionalRegularFile(path)
	if err != nil {
		return err
	}
	updated, err := guidance.UpsertProjectBlock(string(existing))
	if err != nil {
		return err
	}
	return guidance.WriteFileAtomic(path, []byte(updated), 0o644)
}

func installAgentsImport(path string) error {
	existing, err := guidance.ReadOptionalRegularFile(path)
	if err != nil {
		return err
	}
	content := string(existing)
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == "@AGENTS.md" {
			return nil
		}
	}
	if strings.TrimSpace(content) != "" {
		content = strings.TrimRight(content, "\n") + "\n\n"
	}
	return guidance.WriteFileAtomic(path, []byte(content+"@AGENTS.md\n"), 0o644)
}
