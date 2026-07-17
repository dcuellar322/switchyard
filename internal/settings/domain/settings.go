// Package domain owns durable user preferences and their portable validation
// rules. Secret values are deliberately absent; providers retain references
// to external credential stores only.
package domain

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"
)

const (
	// ProviderCodex identifies the local Codex CLI adapter.
	ProviderCodex = "codex"
	// ProviderClaude identifies the local Claude Code CLI adapter.
	ProviderClaude = "claude"
	// ProviderOpenAI identifies the OpenAI-compatible HTTP adapter.
	ProviderOpenAI = "openai-compatible"
)

var environmentReference = regexp.MustCompile(`^env:[A-Za-z_][A-Za-z0-9_]{0,126}$`)

// PortPreferences define the default bounded search space shown to users.
type PortPreferences struct {
	RangeStart int   `json:"rangeStart"`
	RangeEnd   int   `json:"rangeEnd"`
	Excluded   []int `json:"excluded"`
}

// RetentionPreferences define daemon-owned log and metric bounds in seconds
// and bytes. They take effect after the daemon restarts.
type RetentionPreferences struct {
	LogAgeSeconds              int64 `json:"logAgeSeconds"`
	LogMaximumBytes            int64 `json:"logMaximumBytes"`
	MetricRawSeconds           int64 `json:"metricRawSeconds"`
	MetricMinuteSeconds        int64 `json:"metricMinuteSeconds"`
	MetricQuarterHourSeconds   int64 `json:"metricQuarterHourSeconds"`
	MaximumMetricHistoryPoints int   `json:"maximumMetricHistoryPoints"`
}

// ToolPreferences select user-facing terminal and editor integrations.
type ToolPreferences struct {
	Terminal string `json:"terminal"`
	Editor   string `json:"editor"`
}

// ProviderPreferences contain non-secret adapter configuration. Credential
// references currently use env:NAME and never contain the referenced value.
type ProviderPreferences struct {
	ID                  string `json:"id"`
	Enabled             bool   `json:"enabled"`
	Executable          string `json:"executable,omitempty"`
	Endpoint            string `json:"endpoint,omitempty"`
	Model               string `json:"model,omitempty"`
	CredentialReference string `json:"credentialReference,omitempty"`
}

// AIPreferences define the default assisted workflow and fixed provider set.
type AIPreferences struct {
	DefaultProvider string                `json:"defaultProvider"`
	Providers       []ProviderPreferences `json:"providers"`
}

// PermissionPreferences define the least-privilege default for new MCP
// sessions. Explicit command-line profiles continue to win.
type PermissionPreferences struct {
	DefaultAgentProfile string `json:"defaultAgentProfile"`
}

// AppearancePreferences are durable across browsers and desktop shells.
type AppearancePreferences struct {
	Density     string `json:"density"`
	TimeDisplay string `json:"timeDisplay"`
	Theme       string `json:"theme"`
}

// Settings is the complete durable singleton. Revision supports optimistic
// concurrency so two settings screens cannot silently overwrite each other.
type Settings struct {
	Revision     int64                 `json:"revision"`
	ProjectRoots []string              `json:"projectRoots"`
	Ports        PortPreferences       `json:"ports"`
	Retention    RetentionPreferences  `json:"retention"`
	Tools        ToolPreferences       `json:"tools"`
	AI           AIPreferences         `json:"ai"`
	Permissions  PermissionPreferences `json:"permissions"`
	Appearance   AppearancePreferences `json:"appearance"`
	UpdatedAt    time.Time             `json:"updatedAt"`
}

// Validate checks portable settings invariants after application-level path
// canonicalization has completed.
func (s Settings) Validate() error {
	problems := validateProjectAndPorts(s)
	problems = append(problems, validateRetentionAndPreferences(s)...)
	problems = append(problems, validateAI(s.AI)...)
	return errors.Join(problems...)
}

func validateProjectAndPorts(s Settings) []error {
	var problems []error
	if len(s.ProjectRoots) == 0 || len(s.ProjectRoots) > 32 {
		problems = append(problems, errors.New("one to 32 project roots are required"))
	}
	if s.Ports.RangeStart < 1024 || s.Ports.RangeEnd > 65535 || s.Ports.RangeStart > s.Ports.RangeEnd || s.Ports.RangeEnd-s.Ports.RangeStart > 10_000 {
		problems = append(problems, errors.New("port range must contain at most 10001 ports between 1024 and 65535"))
	}
	if len(s.Ports.Excluded) > 512 {
		problems = append(problems, errors.New("at most 512 excluded ports are supported"))
	}
	seenPorts := map[int]struct{}{}
	for _, port := range s.Ports.Excluded {
		if port < s.Ports.RangeStart || port > s.Ports.RangeEnd {
			problems = append(problems, fmt.Errorf("excluded port %d is outside the preferred range", port))
		}
		if _, exists := seenPorts[port]; exists {
			problems = append(problems, fmt.Errorf("excluded port %d is duplicated", port))
		}
		seenPorts[port] = struct{}{}
	}
	return problems
}

func validateRetentionAndPreferences(s Settings) []error {
	var problems []error
	if s.Retention.LogAgeSeconds < 3600 || s.Retention.LogAgeSeconds > 365*24*3600 || s.Retention.LogMaximumBytes < 1<<20 || s.Retention.LogMaximumBytes > 1<<40 {
		problems = append(problems, errors.New("log retention must be one hour to one year and 1 MiB to 1 TiB"))
	}
	if s.Retention.MetricRawSeconds < 60 || s.Retention.MetricRawSeconds > s.Retention.MetricMinuteSeconds || s.Retention.MetricMinuteSeconds > s.Retention.MetricQuarterHourSeconds || s.Retention.MetricQuarterHourSeconds > 365*24*3600 {
		problems = append(problems, errors.New("metric retention tiers must be ordered between one minute and one year"))
	}
	if s.Retention.MaximumMetricHistoryPoints < 100 || s.Retention.MaximumMetricHistoryPoints > 10_000 {
		problems = append(problems, errors.New("maximum metric history points must be between 100 and 10000"))
	}
	if !slices.Contains([]string{"integrated", "system"}, s.Tools.Terminal) {
		problems = append(problems, errors.New("terminal preference must be integrated or system"))
	}
	if !slices.Contains([]string{"vscode", "none"}, s.Tools.Editor) {
		problems = append(problems, errors.New("editor preference must be vscode or none"))
	}
	if !slices.Contains([]string{"observe", "develop", "maintain", "admin"}, s.Permissions.DefaultAgentProfile) {
		problems = append(problems, errors.New("default agent profile is invalid"))
	}
	if !slices.Contains([]string{"comfortable", "compact"}, s.Appearance.Density) || !slices.Contains([]string{"relative", "absolute"}, s.Appearance.TimeDisplay) || !slices.Contains([]string{"dark", "high-contrast"}, s.Appearance.Theme) {
		problems = append(problems, errors.New("appearance preferences are invalid"))
	}
	return problems
}

func validateAI(configuration AIPreferences) []error {
	var problems []error
	if !slices.Contains([]string{"none", ProviderCodex, ProviderClaude, ProviderOpenAI}, configuration.DefaultProvider) {
		problems = append(problems, errors.New("default AI provider is invalid"))
	}
	if len(configuration.Providers) != 3 {
		return append(problems, errors.New("codex, claude, and openai-compatible provider settings are required"))
	}
	seen := map[string]struct{}{}
	for _, provider := range configuration.Providers {
		problems = append(problems, validateProvider(provider)...)
		if !slices.Contains([]string{ProviderCodex, ProviderClaude, ProviderOpenAI}, provider.ID) {
			problems = append(problems, fmt.Errorf("unknown AI provider %q", provider.ID))
		}
		if _, exists := seen[provider.ID]; exists {
			problems = append(problems, fmt.Errorf("AI provider %q is duplicated", provider.ID))
		}
		seen[provider.ID] = struct{}{}
	}
	if configuration.DefaultProvider == "none" {
		return problems
	}
	if _, exists := seen[configuration.DefaultProvider]; !exists {
		problems = append(problems, errors.New("default AI provider has no configuration"))
	} else {
		for _, provider := range configuration.Providers {
			if provider.ID == configuration.DefaultProvider && !provider.Enabled {
				problems = append(problems, errors.New("default AI provider must be enabled"))
			}
		}
	}
	return problems
}

func validateProvider(provider ProviderPreferences) []error {
	var problems []error
	if len(provider.Executable) > 4096 || len(provider.Model) > 256 || len(provider.Endpoint) > 2048 {
		problems = append(problems, fmt.Errorf("AI provider %q configuration is too long", provider.ID))
	}
	if strings.ContainsAny(provider.Executable, "\r\n\x00") || strings.ContainsAny(provider.Model, "\r\n\x00") {
		problems = append(problems, fmt.Errorf("AI provider %q configuration contains control characters", provider.ID))
	}
	if provider.CredentialReference != "" && !environmentReference.MatchString(provider.CredentialReference) {
		problems = append(problems, fmt.Errorf("AI provider %q credential reference must use env:NAME", provider.ID))
	}
	problems = append(problems, validateProviderKind(provider)...)
	return problems
}

func validateProviderKind(provider ProviderPreferences) []error {
	var problems []error
	localCLI := provider.ID == ProviderCodex || provider.ID == ProviderClaude
	if provider.Enabled && localCLI && provider.Executable == "" {
		problems = append(problems, fmt.Errorf("AI provider %q requires an executable when enabled", provider.ID))
	}
	if provider.Enabled && provider.ID == ProviderOpenAI && (provider.Endpoint == "" || provider.CredentialReference == "") {
		problems = append(problems, errors.New("openai-compatible requires an endpoint and credential reference when enabled"))
	}
	if provider.ID != ProviderOpenAI && (provider.Endpoint != "" || provider.CredentialReference != "") {
		problems = append(problems, fmt.Errorf("AI provider %q cannot configure an HTTP endpoint or credential reference", provider.ID))
	}
	if provider.ID == ProviderOpenAI && provider.Endpoint != "" {
		endpoint, err := url.Parse(provider.Endpoint)
		if err != nil || endpoint.Scheme != "https" || endpoint.Host == "" || endpoint.User != nil || endpoint.Fragment != "" {
			problems = append(problems, errors.New("openai-compatible endpoint must be an absolute HTTPS URL without credentials or a fragment"))
		}
	}
	return problems
}
