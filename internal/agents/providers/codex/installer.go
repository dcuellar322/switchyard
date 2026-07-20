// Package codex installs Switchyard MCP and skill configuration for Codex.
package codex

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"switchyard.dev/switchyard/internal/agents/guidance"
	"switchyard.dev/switchyard/internal/agents/providers"
)

const (
	configBegin = "# switchyard:begin"
	configEnd   = "# switchyard:end"
)

// Install writes an idempotent provider config, shared skill, and project guidance.
func Install(request providers.InstallRequest) ([]string, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}
	base := request.Root
	configPath := filepath.Join(base, ".codex", "config.toml")
	skillPath := filepath.Join(base, ".agents", "skills", "switchyard-operate", "SKILL.md")
	if request.Scope == providers.ScopeUser {
		base = request.Home
		configPath = filepath.Join(base, ".codex", "config.toml")
		skillPath = filepath.Join(base, ".agents", "skills", "switchyard-operate", "SKILL.md")
	}
	guidancePath := filepath.Join(request.Root, "AGENTS.md")
	if request.Scope == providers.ScopeProject {
		if err := guidance.ValidateContainedPaths(base, configPath, skillPath, guidancePath); err != nil {
			return nil, err
		}
	}
	existing, err := guidance.ReadOptionalRegularFile(configPath)
	if err != nil {
		return nil, err
	}
	updated, err := upsertConfig(string(existing), configBlock(request))
	if err != nil {
		return nil, err
	}
	if err := guidance.WriteFileAtomic(configPath, []byte(updated), 0o644); err != nil {
		return nil, err
	}
	if err := guidance.WriteFileAtomic(skillPath, guidance.Skill(), 0o644); err != nil {
		return nil, err
	}
	paths := []string{configPath, skillPath}
	if request.Scope == providers.ScopeProject {
		if err := installProjectGuidance(guidancePath); err != nil {
			return nil, err
		}
		paths = append(paths, guidancePath)
	}
	return paths, nil
}

func configBlock(request providers.InstallRequest) string {
	args := request.MCPArgs("codex")
	quoted := make([]string, len(args))
	for index, argument := range args {
		quoted[index] = strconv.Quote(argument)
	}
	return strings.Join([]string{
		configBegin,
		"[mcp_servers.switchyard]",
		"command = " + strconv.Quote(request.Executable),
		"args = [" + strings.Join(quoted, ", ") + "]",
		"startup_timeout_sec = 10",
		"tool_timeout_sec = 35",
		"required = false",
		configEnd,
	}, "\n")
}

func upsertConfig(existing, block string) (string, error) {
	begin := strings.Index(existing, configBegin)
	end := strings.Index(existing, configEnd)
	if (begin >= 0) != (end >= 0) || begin >= 0 && end < begin {
		return "", errors.New("malformed Switchyard Codex config markers")
	}
	remainder := existing
	if begin >= 0 {
		end += len(configEnd)
		remainder = existing[:begin] + existing[end:]
	}
	if strings.Contains(remainder, "[mcp_servers.switchyard]") {
		return "", fmt.Errorf("%s already exists outside Switchyard's managed block", "[mcp_servers.switchyard]")
	}
	if begin >= 0 {
		return strings.TrimSpace(existing[:begin]+block+existing[end:]) + "\n", nil
	}
	if strings.TrimSpace(existing) == "" {
		return block + "\n", nil
	}
	return strings.TrimRight(existing, "\n") + "\n\n" + block + "\n", nil
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
