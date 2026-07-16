// Package plugin defines the stable out-of-process Switchyard plugin protocol
// and a small server SDK for plugin authors.
package plugin

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
)

const (
	// ProtocolVersion is negotiated before any plugin capability may be used.
	ProtocolVersion = "switchyard.plugin/v1alpha1"
	// ManifestVersion identifies the on-disk plugin capability manifest.
	ManifestVersion = "switchyard.plugin-manifest/v1alpha1"
	// MaxMessageBytes is the maximum JSON-RPC request or response size.
	MaxMessageBytes = 1 << 20
)

// Capability identifies one method family implemented by a plugin.
type Capability string

const (
	// CapabilityProjectInspect permits structured trusted-project observation.
	CapabilityProjectInspect Capability = "project.inspect"
	// CapabilityProjectOperate permits typed host-audited project actions.
	CapabilityProjectOperate Capability = "project.operate"
)

// Scope is a host-granted data or mutation permission.
type Scope string

const (
	// ScopeProjectMetadataRead reveals bounded project identity fields.
	ScopeProjectMetadataRead Scope = "project.metadata.read"
	// ScopeProjectFilesRead reveals the trusted project root to the process.
	ScopeProjectFilesRead Scope = "project.files.read"
	// ScopeProjectOperate authorizes only the typed project.operate method.
	ScopeProjectOperate Scope = "project.operate"
)

var identifierPattern = regexp.MustCompile(`^[a-z][a-z0-9.-]{0,63}$`)

// Manifest is both the discovered package declaration and negotiated identity.
// Executable and Arguments are host-owned launch configuration and are omitted
// from an initialize response.
type Manifest struct {
	SchemaVersion   string       `json:"schemaVersion"`
	ID              string       `json:"id"`
	Name            string       `json:"name"`
	Version         string       `json:"version"`
	ProtocolVersion string       `json:"protocolVersion"`
	Executable      string       `json:"executable,omitempty"`
	Arguments       []string     `json:"arguments,omitempty"`
	Capabilities    []Capability `json:"capabilities"`
	RequestedScopes []Scope      `json:"requestedScopes"`
}

// Validate rejects ambiguous, unsupported, or over-broad declarations.
func (m Manifest) Validate() error {
	if err := m.validateIdentity(); err != nil {
		return err
	}
	if err := validateArguments(m.Arguments); err != nil {
		return err
	}
	if err := validateCapabilities(m.Capabilities, m.RequestedScopes); err != nil {
		return err
	}
	return validateScopes(m.RequestedScopes)
}

func (m Manifest) validateIdentity() error {
	if m.SchemaVersion != ManifestVersion {
		return fmt.Errorf("unsupported plugin manifest version %q", m.SchemaVersion)
	}
	if !identifierPattern.MatchString(m.ID) {
		return errors.New("plugin id must be a lowercase bounded identifier")
	}
	if strings.TrimSpace(m.Name) == "" || len(m.Name) > 120 {
		return errors.New("plugin name must contain at most 120 characters")
	}
	if strings.TrimSpace(m.Version) == "" || len(m.Version) > 64 {
		return errors.New("plugin version must contain at most 64 characters")
	}
	if m.ProtocolVersion != ProtocolVersion {
		return fmt.Errorf("unsupported plugin protocol %q; host supports %s", m.ProtocolVersion, ProtocolVersion)
	}
	if len(m.Capabilities) == 0 || len(m.Capabilities) > 16 || len(m.RequestedScopes) > 16 || len(m.Arguments) > 32 {
		return errors.New("plugin capabilities, scopes, or arguments exceed protocol limits")
	}
	return nil
}

func validateArguments(arguments []string) error {
	for _, argument := range arguments {
		if strings.ContainsRune(argument, 0) || len(argument) > 4096 {
			return errors.New("plugin argument is invalid")
		}
	}
	return nil
}

func validateCapabilities(values []Capability, scopes []Scope) error {
	capabilities := slices.Clone(values)
	slices.Sort(capabilities)
	if len(slices.Compact(capabilities)) != len(values) {
		return errors.New("plugin capabilities must be unique")
	}
	for _, capability := range capabilities {
		if capability != CapabilityProjectInspect && capability != CapabilityProjectOperate {
			return fmt.Errorf("unknown plugin capability %q", capability)
		}
	}
	if slices.Contains(values, CapabilityProjectInspect) && !slices.Contains(scopes, ScopeProjectMetadataRead) {
		return errors.New("project.inspect requires project.metadata.read")
	}
	if slices.Contains(values, CapabilityProjectOperate) && !slices.Contains(scopes, ScopeProjectOperate) {
		return errors.New("project.operate requires project.operate scope")
	}
	return nil
}

func validateScopes(values []Scope) error {
	scopes := slices.Clone(values)
	slices.Sort(scopes)
	if len(slices.Compact(scopes)) != len(values) {
		return errors.New("plugin requested scopes must be unique")
	}
	for _, scope := range scopes {
		if scope != ScopeProjectMetadataRead && scope != ScopeProjectFilesRead && scope != ScopeProjectOperate {
			return fmt.Errorf("unknown plugin scope %q", scope)
		}
	}
	return nil
}

// InitializeParams are the only permissions a host grants to one process.
type InitializeParams struct {
	ProtocolVersion string  `json:"protocolVersion"`
	HostVersion     string  `json:"hostVersion"`
	GrantedScopes   []Scope `json:"grantedScopes"`
}

// InitializeResult confirms the running executable's identity and grants.
type InitializeResult struct {
	ProtocolVersion string   `json:"protocolVersion"`
	Plugin          Manifest `json:"plugin"`
	GrantedScopes   []Scope  `json:"grantedScopes"`
}

// HealthResult is a bounded liveness observation.
type HealthResult struct {
	Status  string    `json:"status"`
	Message string    `json:"message,omitempty"`
	Checked time.Time `json:"checkedAt"`
}

// Project describes only host-approved project metadata.
type Project struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	Root        string `json:"root,omitempty"`
}

// InspectRequest asks a declared adapter to observe one trusted project.
type InspectRequest struct {
	Project Project `json:"project"`
}

// Fact is structured, display-safe adapter evidence.
type Fact struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Value   string `json:"value"`
	Source  string `json:"source"`
	Warning string `json:"warning,omitempty"`
}

// Action is one typed operation advertised by a plugin.
type Action struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Risk        string `json:"risk"`
}

// InspectResult contains bounded observations and advertised typed actions.
type InspectResult struct {
	Summary  string    `json:"summary"`
	Facts    []Fact    `json:"facts"`
	Actions  []Action  `json:"actions"`
	Warnings []string  `json:"warnings"`
	Observed time.Time `json:"observedAt"`
}

// OperateRequest invokes one plugin-declared action. Input is schema-owned by
// that action and remains bounded by MaxMessageBytes.
type OperateRequest struct {
	Project Project         `json:"project"`
	Action  string          `json:"action"`
	Input   json.RawMessage `json:"input"`
}

// OperateResult is the plugin's structured receipt, not host authorization.
type OperateResult struct {
	Status  string          `json:"status"`
	Summary string          `json:"summary"`
	Output  json.RawMessage `json:"output"`
}

// Handler implements a plugin's declared capabilities.
type Handler interface {
	Health() HealthResult
	Inspect(InspectRequest) (InspectResult, error)
	Operate(OperateRequest) (OperateResult, error)
}
