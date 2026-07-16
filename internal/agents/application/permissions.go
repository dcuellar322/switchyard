// Package application owns provider-neutral agent authorization policy.
package application

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// Profile is a built-in least-privilege agent capability set.
type Profile string

const (
	// ProfileObserve permits only bounded reads.
	ProfileObserve Profile = "observe"
	// ProfileDevelop adds ordinary lifecycle, action, and cancellation mutations.
	ProfileDevelop Profile = "develop"
	// ProfileMaintain adds rebuild and deterministic proposal creation.
	ProfileMaintain Profile = "maintain"
	// ProfileAdmin adds trust decisions and destructive operations.
	ProfileAdmin Profile = "admin"
)

// Capability identifies one application mutation exposed through an agent adapter.
type Capability string

const (
	// CapabilityLifecycle controls ordinary runtime lifecycle changes.
	CapabilityLifecycle Capability = "lifecycle"
	// CapabilityAction controls accepted manifest actions.
	CapabilityAction Capability = "action"
	// CapabilityRebuild controls runtime rebuild operations.
	CapabilityRebuild Capability = "rebuild"
	// CapabilityProposalCreate controls deterministic repository scans.
	CapabilityProposalCreate Capability = "manifest_proposal_create"
	// CapabilityProposalAccept controls explicit manifest trust decisions.
	CapabilityProposalAccept Capability = "manifest_proposal_accept"
	// CapabilityOperationCancel controls durable operation cancellation.
	CapabilityOperationCancel Capability = "operation_cancel"
	// CapabilityDestructive controls teardown and destructive trusted actions.
	CapabilityDestructive Capability = "destructive"
)

var (
	// ErrPermissionDenied identifies an agent profile or project-scope denial.
	ErrPermissionDenied = errors.New("agent permission denied")
	identifierPattern   = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._:/-]{0,127}$`)
)

// Scope binds an authorization profile to one provider and optional projects.
type Scope struct {
	Provider   string
	AgentID    string
	Profile    Profile
	ProjectIDs []string
}

// ParseProfile validates a built-in profile name.
func ParseProfile(value string) (Profile, error) {
	profile := Profile(strings.ToLower(strings.TrimSpace(value)))
	switch profile {
	case ProfileObserve, ProfileDevelop, ProfileMaintain, ProfileAdmin:
		return profile, nil
	default:
		return "", fmt.Errorf("unknown agent profile %q", value)
	}
}

// NewScope validates a provider identity and returns an immutable scope.
func NewScope(provider, agentID string, profile Profile, projectIDs []string) (Scope, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	agentID = strings.TrimSpace(agentID)
	if !identifierPattern.MatchString(provider) {
		return Scope{}, errors.New("agent provider must be a bounded identifier")
	}
	if agentID == "" {
		agentID = provider
	}
	if !identifierPattern.MatchString(agentID) {
		return Scope{}, errors.New("agent identity must be a bounded identifier")
	}
	if _, err := ParseProfile(string(profile)); err != nil {
		return Scope{}, err
	}
	projects := slices.Clone(projectIDs)
	for index := range projects {
		projects[index] = strings.TrimSpace(projects[index])
		if !identifierPattern.MatchString(projects[index]) {
			return Scope{}, fmt.Errorf("project scope %q is not a bounded identifier", projects[index])
		}
	}
	slices.Sort(projects)
	projects = slices.Compact(projects)
	return Scope{Provider: provider, AgentID: agentID, Profile: profile, ProjectIDs: projects}, nil
}

// ActorID is the non-secret identity persisted in mutation audit records.
func (s Scope) ActorID() string { return s.Provider + "/" + s.AgentID }

// Allows reports whether the profile includes a capability.
func (s Scope) Allows(capability Capability) bool {
	required, known := map[Capability]Profile{
		CapabilityLifecycle:       ProfileDevelop,
		CapabilityAction:          ProfileDevelop,
		CapabilityOperationCancel: ProfileDevelop,
		CapabilityRebuild:         ProfileMaintain,
		CapabilityProposalCreate:  ProfileMaintain,
		CapabilityProposalAccept:  ProfileAdmin,
		CapabilityDestructive:     ProfileAdmin,
	}[capability]
	if !known {
		return false
	}
	return profileRank(s.Profile) >= profileRank(required)
}

// Authorize rejects a capability or project outside the configured scope.
func (s Scope) Authorize(capability Capability, projectID string) error {
	if !s.Allows(capability) {
		return fmt.Errorf("%w: profile %s does not allow %s", ErrPermissionDenied, s.Profile, capability)
	}
	return s.AuthorizeRead(projectID)
}

// AuthorizeRead rejects access to a project outside the configured scope.
func (s Scope) AuthorizeRead(projectID string) error {
	if projectID != "" && len(s.ProjectIDs) > 0 && !slices.Contains(s.ProjectIDs, projectID) {
		return fmt.Errorf("%w: project %s is outside this agent scope", ErrPermissionDenied, projectID)
	}
	return nil
}

func profileRank(profile Profile) int {
	switch profile {
	case ProfileObserve:
		return 0
	case ProfileDevelop:
		return 1
	case ProfileMaintain:
		return 2
	case ProfileAdmin:
		return 3
	default:
		return -1
	}
}
