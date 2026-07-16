// Package providers defines shared inputs for provider-specific agent installers.
package providers

import (
	"fmt"
	"path/filepath"
	"strings"

	agents "switchyard.dev/switchyard/internal/agents/application"
)

// InstallScope selects project-local or user-level provider configuration.
type InstallScope string

const (
	// ScopeProject writes configuration below a repository root.
	ScopeProject InstallScope = "project"
	// ScopeUser writes configuration below the current user's home directory.
	ScopeUser InstallScope = "user"
)

// InstallRequest is the fully resolved, provider-neutral installer input.
type InstallRequest struct {
	Scope      InstallScope
	Root       string
	Home       string
	Executable string
	DataDir    string
	Profile    agents.Profile
	AgentID    string
	ProjectIDs []string
}

// Validate rejects incomplete or unsupported installer inputs.
func (r InstallRequest) Validate() error {
	if r.Scope != ScopeProject && r.Scope != ScopeUser {
		return fmt.Errorf("unsupported install scope %q", r.Scope)
	}
	for name, value := range map[string]string{"root": r.Root, "home": r.Home, "executable": r.Executable, "data directory": r.DataDir} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", name)
		}
		if !filepath.IsAbs(value) {
			return fmt.Errorf("%s must be absolute", name)
		}
	}
	if _, err := agents.ParseProfile(string(r.Profile)); err != nil {
		return err
	}
	return nil
}

// MCPArgs returns stable stdio-server arguments shared by provider configs.
func (r InstallRequest) MCPArgs(provider string) []string {
	args := []string{"--data-dir", r.DataDir, "mcp", "serve", "--transport", "stdio", "--provider", provider, "--agent-id", r.AgentID, "--profile", string(r.Profile)}
	for _, projectID := range r.ProjectIDs {
		args = append(args, "--project", projectID)
	}
	return args
}
