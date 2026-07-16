// Package domain owns project environment identity and isolation invariants.
package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	// MaximumComposeProjectName stays within the portable Compose name budget.
	MaximumComposeProjectName = 63
	// MaximumPortOffset is the logical lease-offset namespace. Concrete port
	// adapters still validate that any resolved host port is in range.
	MaximumPortOffset = 65535
)

var composeNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// Availability states whether an environment can currently host a runtime.
type Availability string

const (
	// AvailabilityAvailable means the registered checkout can host a runtime.
	AvailabilityAvailable Availability = "available"
	// AvailabilityUnavailable retains an observed checkout that cannot run.
	AvailabilityUnavailable Availability = "unavailable"
)

// State is the runtime-facing lifecycle of a registered environment.
type State string

const (
	// StateRegistered has durable identity but no completed runtime allocation.
	StateRegistered State = "registered"
	// StateActive has a running runtime with a validated local route target.
	StateActive State = "active"
	// StateInactive is configured and currently stopped.
	StateInactive State = "inactive"
	// StateUnavailable cannot currently host a runtime.
	StateUnavailable State = "unavailable"
)

// PortLease is one exact environment-owned host-port assignment. The logical
// offset helps allocation, but consumers must use these validated exact leases
// rather than adding an offset to arbitrary host ports.
type PortLease struct {
	PortID     string `json:"portId"`
	Protocol   string `json:"protocol"`
	TargetPort int    `json:"targetPort"`
	HostPort   int    `json:"hostPort"`
}

// RuntimeAllocation separates Compose and port identities between checkouts.
type RuntimeAllocation struct {
	ComposeProjectName string      `json:"composeProjectName"`
	PortLeaseNamespace string      `json:"portLeaseNamespace"`
	PortOffset         int         `json:"portOffset"`
	PortLeases         []PortLease `json:"portLeases"`
}

// Environment is one Git worktree registered as an independently runnable
// location of a catalog project.
type Environment struct {
	ID                string            `json:"id"`
	ProjectID         string            `json:"projectId"`
	Name              string            `json:"name"`
	Path              string            `json:"path"`
	Head              string            `json:"head,omitempty"`
	Branch            string            `json:"branch,omitempty"`
	Detached          bool              `json:"detached"`
	Bare              bool              `json:"bare"`
	Locked            bool              `json:"locked"`
	Primary           bool              `json:"primary"`
	Availability      Availability      `json:"availability"`
	UnavailableReason string            `json:"unavailableReason,omitempty"`
	State             State             `json:"state"`
	Hostname          string            `json:"hostname"`
	Target            string            `json:"target,omitempty"`
	Allocation        RuntimeAllocation `json:"allocation"`
	RegisteredAt      time.Time         `json:"registeredAt"`
	LastObservedAt    time.Time         `json:"lastObservedAt"`
	UpdatedAt         time.Time         `json:"updatedAt"`
}

// StableID returns a path-sensitive identifier without exposing the path.
func StableID(projectID, path string) (string, error) {
	projectID = strings.TrimSpace(projectID)
	path = filepath.Clean(strings.TrimSpace(path))
	if projectID == "" || path == "" || path == "." {
		return "", errors.New("project ID and worktree path are required")
	}
	digest := sha256.Sum256([]byte(projectID + "\x00" + path))
	return "env-" + hex.EncodeToString(digest[:15]), nil
}

// ComposeProjectName returns a stable, human-readable, collision-resistant
// name. The digest is derived from the environment identity, not a display
// label, so branch renames cannot collide with another worktree.
func ComposeProjectName(projectSlug, branch, environmentID string) (string, error) {
	if environmentID == "" {
		return "", errors.New("environment ID is required")
	}
	base := sanitizeName(projectSlug + "-" + branch)
	if base == "" {
		base = "project"
	}
	digest := sha256.Sum256([]byte(environmentID))
	suffix := hex.EncodeToString(digest[:8])
	prefixBudget := MaximumComposeProjectName - len("sy--") - len(suffix)
	if len(base) > prefixBudget {
		base = strings.Trim(base[:prefixBudget], "-_")
	}
	name := "sy-" + base + "-" + suffix
	if !composeNamePattern.MatchString(name) {
		return "", fmt.Errorf("derived invalid Compose project name %q", name)
	}
	return name, nil
}

// PortLeaseNamespace is collision-resistant and intentionally independent of
// user-facing names. Port adapters use it as the owner of environment leases.
func PortLeaseNamespace(environmentID string) (string, error) {
	if !strings.HasPrefix(environmentID, "env-") || len(environmentID) <= len("env-") {
		return "", errors.New("valid environment ID is required")
	}
	return "worktree:" + strings.TrimPrefix(environmentID, "env-"), nil
}

// PortOffsetSeed returns a stable logical offset candidate. Registration
// resolves the extremely unlikely collision against all existing environments.
func PortOffsetSeed(environmentID string) int {
	digest := sha256.Sum256([]byte("port-offset\x00" + environmentID))
	value := int(digest[0])<<8 | int(digest[1])
	return value%MaximumPortOffset + 1
}

// LocalhostName returns a stable collision-resistant friendly route. It does
// not claim the name is free; the routing registry reports any actual conflict.
func LocalhostName(projectSlug, environmentID string) (string, error) {
	if environmentID == "" {
		return "", errors.New("environment ID is required")
	}
	label := sanitizeName(projectSlug)
	if label == "" {
		label = "project"
	}
	digest := sha256.Sum256([]byte("localhost\x00" + environmentID))
	suffix := hex.EncodeToString(digest[:5])
	labelBudget := 63 - len(suffix) - 1
	if len(label) > labelBudget {
		label = strings.Trim(label[:labelBudget], "-")
	}
	return label + "-" + suffix + ".localhost", nil
}

// Validate checks the isolation and availability invariants of an environment.
func (e Environment) Validate() error {
	var problems []error
	if e.ID == "" || e.ProjectID == "" || e.Name == "" {
		problems = append(problems, errors.New("environment ID, project ID, and name are required"))
	}
	if !filepath.IsAbs(e.Path) {
		problems = append(problems, errors.New("environment path must be absolute"))
	}
	if len(e.Allocation.ComposeProjectName) > MaximumComposeProjectName || !composeNamePattern.MatchString(e.Allocation.ComposeProjectName) {
		problems = append(problems, errors.New("environment Compose project name is invalid"))
	}
	if e.Allocation.PortLeaseNamespace == "" || e.Allocation.PortOffset < 1 || e.Allocation.PortOffset > MaximumPortOffset {
		problems = append(problems, errors.New("environment port allocation is invalid"))
	}
	if !validAvailability(e.Availability) {
		problems = append(problems, errors.New("environment availability is invalid"))
	}
	if e.Availability == AvailabilityUnavailable && e.UnavailableReason == "" {
		problems = append(problems, errors.New("unavailable environment requires a reason"))
	}
	if !validState(e.State) {
		problems = append(problems, errors.New("environment runtime state is invalid"))
	}
	if e.Hostname == "" {
		problems = append(problems, errors.New("environment hostname is required"))
	}
	if e.State == StateActive && e.Target == "" {
		problems = append(problems, errors.New("active environment requires a route target"))
	}
	problems = append(problems, validateLeases(e.Allocation.PortLeases)...)
	return errors.Join(problems...)
}

func validAvailability(value Availability) bool {
	return value == AvailabilityAvailable || value == AvailabilityUnavailable
}

func validState(value State) bool {
	return value == StateRegistered || value == StateActive || value == StateInactive || value == StateUnavailable
}

func validateLeases(leases []PortLease) []error {
	problems := make([]error, 0)
	seen := make(map[string]struct{}, len(leases))
	for _, lease := range leases {
		key := fmt.Sprintf("%s/%d", lease.Protocol, lease.HostPort)
		if lease.PortID == "" || lease.TargetPort < 1 || lease.TargetPort > 65535 || lease.HostPort < 1 || lease.HostPort > 65535 || lease.Protocol != "tcp" && lease.Protocol != "udp" {
			problems = append(problems, fmt.Errorf("environment port lease %q is invalid", lease.PortID))
		} else if _, exists := seen[key]; exists {
			problems = append(problems, fmt.Errorf("environment port lease %s is duplicated", key))
		}
		seen[key] = struct{}{}
	}
	return problems
}

func sanitizeName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	previousSeparator := false
	for _, character := range value {
		valid := character >= 'a' && character <= 'z' || character >= '0' && character <= '9'
		if valid {
			builder.WriteRune(character)
			previousSeparator = false
		} else if !previousSeparator && builder.Len() > 0 {
			builder.WriteByte('-')
			previousSeparator = true
		}
	}
	return strings.Trim(builder.String(), "-")
}
