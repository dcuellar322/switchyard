// Package domain owns workspace graph and coordinated-execution invariants.
package domain

import (
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"
)

// FailurePolicy determines how a workspace start responds to a project failure.
type FailurePolicy string

const (
	// FailurePolicyRollback stops projects started by the failed execution.
	FailurePolicyRollback FailurePolicy = "rollback"
	// FailurePolicyContinue starts independent branches and blocks dependents.
	FailurePolicyContinue FailurePolicy = "continue"
)

// MemberRole describes a project's responsibility in a workspace.
type MemberRole string

const (
	// MemberRoleApplication is a user-facing application project.
	MemberRoleApplication MemberRole = "application"
	// MemberRoleDependency supports one or more application projects.
	MemberRoleDependency MemberRole = "dependency"
	// MemberRoleTooling provides development-only workspace tooling.
	MemberRoleTooling MemberRole = "tooling"
)

// RecipeKind is one bounded launch experience a workspace may offer.
type RecipeKind string

const (
	// RecipeOpenURL opens a declared local URL.
	RecipeOpenURL RecipeKind = "open_url"
	// RecipeOpenTerminal opens a terminal at a declared project.
	RecipeOpenTerminal RecipeKind = "open_terminal"
	// RecipeOpenEditor opens an editor at a declared project.
	RecipeOpenEditor RecipeKind = "open_editor"
	// RecipeStartAgent starts a configured coding-agent experience.
	RecipeStartAgent RecipeKind = "start_agent"
)

// ProjectStatus is a durable, project-visible execution status.
type ProjectStatus string

const (
	// ProjectQueued has not begun lifecycle work.
	ProjectQueued ProjectStatus = "queued"
	// ProjectBlocked cannot proceed because of a dependency outcome.
	ProjectBlocked ProjectStatus = "blocked"
	// ProjectStarting is invoking the project start use case.
	ProjectStarting ProjectStatus = "starting"
	// ProjectCheckingHealth is waiting for readiness.
	ProjectCheckingHealth ProjectStatus = "checking_health"
	// ProjectRunning completed start and readiness checks.
	ProjectRunning ProjectStatus = "running"
	// ProjectStartFailed could not reach a ready state.
	ProjectStartFailed ProjectStatus = "start_failed"
	// ProjectStopping is invoking the non-destructive stop use case.
	ProjectStopping ProjectStatus = "stopping"
	// ProjectStopped completed the requested stop.
	ProjectStopped ProjectStatus = "stopped"
	// ProjectStopFailed could not be stopped safely.
	ProjectStopFailed ProjectStatus = "stop_failed"
	// ProjectRollingBack is stopping a project started by a failed run.
	ProjectRollingBack ProjectStatus = "rolling_back"
	// ProjectRolledBack completed compensating stop with data preserved.
	ProjectRolledBack ProjectStatus = "rolled_back"
	// ProjectRollbackFailed could not complete compensating stop.
	ProjectRollbackFailed ProjectStatus = "rollback_failed"
	// ProjectCancelled stopped progressing after context cancellation.
	ProjectCancelled ProjectStatus = "cancelled"
)

// ExecutionKind is a coordinated workspace lifecycle mutation.
type ExecutionKind string

const (
	// ExecutionStart starts selected projects dependency-first.
	ExecutionStart ExecutionKind = "start"
	// ExecutionStop stops selected projects dependent-first.
	ExecutionStop ExecutionKind = "stop"
)

// ExecutionState is the aggregate outcome of a workspace operation.
type ExecutionState string

const (
	// ExecutionRunning has project work in progress.
	ExecutionRunning ExecutionState = "running"
	// ExecutionSucceeded completed every selected project.
	ExecutionSucceeded ExecutionState = "succeeded"
	// ExecutionPartial completed an independent subset.
	ExecutionPartial ExecutionState = "partially_succeeded"
	// ExecutionFailed did not leave a useful completed subset.
	ExecutionFailed ExecutionState = "failed"
	// ExecutionCancelled stopped after context cancellation.
	ExecutionCancelled ExecutionState = "cancelled"
)

var (
	// ErrInvalidWorkspace identifies a malformed workspace aggregate.
	ErrInvalidWorkspace = errors.New("invalid workspace")
	// ErrDependencyCycle identifies a graph that cannot be topologically ordered.
	ErrDependencyCycle = errors.New("workspace dependency cycle")
	// ErrUnknownProfile identifies an unavailable workspace profile.
	ErrUnknownProfile = errors.New("unknown workspace profile")
)

// Member is one project participating in a workspace.
type Member struct {
	ProjectID     string        `json:"projectId"`
	Role          MemberRole    `json:"role"`
	Order         int           `json:"order"`
	HealthGate    bool          `json:"healthGate"`
	HealthTimeout time.Duration `json:"healthTimeout"`
	Status        ProjectStatus `json:"status,omitempty"`
	Message       string        `json:"message,omitempty"`
}

// Dependency means ProjectID may start only after DependsOnProjectID is ready.
type Dependency struct {
	ProjectID          string `json:"projectId"`
	DependsOnProjectID string `json:"dependsOnProjectId"`
}

// Recipe is a declarative, bounded workspace launch action.
type Recipe struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Kind      RecipeKind `json:"kind"`
	ProjectID string     `json:"projectId,omitempty"`
	Target    string     `json:"target,omitempty"`
	Arguments []string   `json:"arguments"`
	Order     int        `json:"order"`
}

// Profile selects a dependency-closed subset and an execution concurrency cap.
type Profile struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	Description       string   `json:"description,omitempty"`
	ProjectIDs        []string `json:"projectIds"`
	MaxParallel       int      `json:"maxParallel"`
	LowMemory         bool     `json:"lowMemory"`
	MemoryBudgetBytes uint64   `json:"memoryBudgetBytes,omitempty"`
}

// ProjectResult is one member's visible status within a coordinated execution.
type ProjectResult struct {
	ProjectID  string        `json:"projectId"`
	Role       MemberRole    `json:"role"`
	Status     ProjectStatus `json:"status"`
	Message    string        `json:"message,omitempty"`
	Order      int           `json:"order"`
	StartedAt  *time.Time    `json:"startedAt,omitempty"`
	FinishedAt *time.Time    `json:"finishedAt,omitempty"`
}

// ExecutionSummary is the durable status and outcome of one workspace mutation.
type ExecutionSummary struct {
	ID           string          `json:"id"`
	WorkspaceID  string          `json:"workspaceId"`
	Kind         ExecutionKind   `json:"kind"`
	State        ExecutionState  `json:"state"`
	Policy       FailurePolicy   `json:"policy"`
	ProfileID    string          `json:"profileId,omitempty"`
	RemoveData   bool            `json:"removeData"`
	Projects     []ProjectResult `json:"projects"`
	ErrorMessage string          `json:"errorMessage,omitempty"`
	StartedAt    time.Time       `json:"startedAt"`
	FinishedAt   *time.Time      `json:"finishedAt,omitempty"`
}

// Workspace is a durable dependency graph and its latest execution summary.
type Workspace struct {
	ID                   string            `json:"id"`
	Name                 string            `json:"name"`
	Description          string            `json:"description,omitempty"`
	DefaultFailurePolicy FailurePolicy     `json:"policy"`
	DefaultProfileID     string            `json:"profile,omitempty"`
	Members              []Member          `json:"members"`
	Dependencies         []Dependency      `json:"dependencies"`
	Recipes              []Recipe          `json:"recipes"`
	Profiles             []Profile         `json:"profiles"`
	LastRun              *ExecutionSummary `json:"lastRun,omitempty"`
	Revision             int64             `json:"revision"`
	CreatedAt            time.Time         `json:"createdAt"`
	UpdatedAt            time.Time         `json:"updatedAt"`
}

// Validate enforces aggregate references, bounded values, and DAG structure.
func (w Workspace) Validate() error {
	if strings.TrimSpace(w.ID) == "" || strings.TrimSpace(w.Name) == "" {
		return fmt.Errorf("%w: id and name are required", ErrInvalidWorkspace)
	}
	if w.DefaultFailurePolicy != FailurePolicyRollback && w.DefaultFailurePolicy != FailurePolicyContinue {
		return fmt.Errorf("%w: unsupported failure policy %q", ErrInvalidWorkspace, w.DefaultFailurePolicy)
	}
	members, err := w.validateMembers()
	if err != nil {
		return err
	}
	if err := w.validateDependencies(members); err != nil {
		return err
	}
	if err := w.validateProfiles(members); err != nil {
		return err
	}
	if err := w.validateRecipes(members); err != nil {
		return err
	}
	_, err = w.TopologicalLayers(nil)
	return err
}

func (w Workspace) validateMembers() (map[string]Member, error) {
	if len(w.Members) == 0 {
		return nil, fmt.Errorf("%w: at least one member is required", ErrInvalidWorkspace)
	}
	members := make(map[string]Member, len(w.Members))
	for _, member := range w.Members {
		if member.ProjectID == "" || member.Order < 0 {
			return nil, fmt.Errorf("%w: member project and non-negative order are required", ErrInvalidWorkspace)
		}
		if member.Role != MemberRoleApplication && member.Role != MemberRoleDependency && member.Role != MemberRoleTooling {
			return nil, fmt.Errorf("%w: unsupported member role %q", ErrInvalidWorkspace, member.Role)
		}
		if member.HealthGate && member.HealthTimeout <= 0 {
			return nil, fmt.Errorf("%w: health-gated project %s requires a timeout", ErrInvalidWorkspace, member.ProjectID)
		}
		if _, exists := members[member.ProjectID]; exists {
			return nil, fmt.Errorf("%w: duplicate member %s", ErrInvalidWorkspace, member.ProjectID)
		}
		members[member.ProjectID] = member
	}
	return members, nil
}

func (w Workspace) validateDependencies(members map[string]Member) error {
	seen := make(map[string]struct{}, len(w.Dependencies))
	for _, edge := range w.Dependencies {
		if _, ok := members[edge.ProjectID]; !ok {
			return fmt.Errorf("%w: dependency source %s is not a member", ErrInvalidWorkspace, edge.ProjectID)
		}
		if _, ok := members[edge.DependsOnProjectID]; !ok {
			return fmt.Errorf("%w: dependency target %s is not a member", ErrInvalidWorkspace, edge.DependsOnProjectID)
		}
		if edge.ProjectID == edge.DependsOnProjectID {
			return fmt.Errorf("%w: project %s depends on itself", ErrDependencyCycle, edge.ProjectID)
		}
		key := edge.ProjectID + "\x00" + edge.DependsOnProjectID
		if _, ok := seen[key]; ok {
			return fmt.Errorf("%w: duplicate dependency %s -> %s", ErrInvalidWorkspace, edge.ProjectID, edge.DependsOnProjectID)
		}
		seen[key] = struct{}{}
	}
	return nil
}

func (w Workspace) validateProfiles(members map[string]Member) error {
	seen := make(map[string]struct{}, len(w.Profiles))
	for _, profile := range w.Profiles {
		if profile.ID == "" || strings.TrimSpace(profile.Name) == "" || profile.MaxParallel < 1 || profile.MaxParallel > 64 {
			return fmt.Errorf("%w: profile id, name, and maxParallel between 1 and 64 are required", ErrInvalidWorkspace)
		}
		if _, ok := seen[profile.ID]; ok {
			return fmt.Errorf("%w: duplicate profile %s", ErrInvalidWorkspace, profile.ID)
		}
		seen[profile.ID] = struct{}{}
		if len(profile.ProjectIDs) == 0 {
			return fmt.Errorf("%w: profile %s must select at least one project", ErrInvalidWorkspace, profile.ID)
		}
		projects := make(map[string]struct{}, len(profile.ProjectIDs))
		for _, projectID := range profile.ProjectIDs {
			if _, ok := members[projectID]; !ok {
				return fmt.Errorf("%w: profile %s references non-member %s", ErrInvalidWorkspace, profile.ID, projectID)
			}
			if _, ok := projects[projectID]; ok {
				return fmt.Errorf("%w: profile %s repeats project %s", ErrInvalidWorkspace, profile.ID, projectID)
			}
			projects[projectID] = struct{}{}
		}
	}
	if w.DefaultProfileID != "" {
		if _, ok := seen[w.DefaultProfileID]; !ok {
			return fmt.Errorf("%w: default profile %s does not exist", ErrInvalidWorkspace, w.DefaultProfileID)
		}
	}
	return nil
}

func (w Workspace) validateRecipes(members map[string]Member) error {
	seen := make(map[string]struct{}, len(w.Recipes))
	for _, recipe := range w.Recipes {
		if recipe.ID == "" || strings.TrimSpace(recipe.Name) == "" || recipe.Order < 0 {
			return fmt.Errorf("%w: recipe id, name, and non-negative order are required", ErrInvalidWorkspace)
		}
		if !slices.Contains([]RecipeKind{RecipeOpenURL, RecipeOpenTerminal, RecipeOpenEditor, RecipeStartAgent}, recipe.Kind) {
			return fmt.Errorf("%w: unsupported recipe kind %q", ErrInvalidWorkspace, recipe.Kind)
		}
		if recipe.ProjectID != "" {
			if _, ok := members[recipe.ProjectID]; !ok {
				return fmt.Errorf("%w: recipe %s references non-member %s", ErrInvalidWorkspace, recipe.ID, recipe.ProjectID)
			}
		}
		if len(recipe.Arguments) > 32 {
			return fmt.Errorf("%w: recipe %s has too many arguments", ErrInvalidWorkspace, recipe.ID)
		}
		for _, argument := range recipe.Arguments {
			if len(argument) > 2048 {
				return fmt.Errorf("%w: recipe %s argument exceeds 2048 bytes", ErrInvalidWorkspace, recipe.ID)
			}
		}
		switch recipe.Kind {
		case RecipeOpenURL:
			parsed, err := url.Parse(recipe.Target)
			if err != nil || parsed.Host == "" || parsed.User != nil || parsed.Scheme != "http" && parsed.Scheme != "https" {
				return fmt.Errorf("%w: recipe %s requires an absolute HTTP URL without credentials", ErrInvalidWorkspace, recipe.ID)
			}
		case RecipeOpenTerminal, RecipeOpenEditor, RecipeStartAgent:
			if recipe.ProjectID == "" {
				return fmt.Errorf("%w: recipe %s requires a project", ErrInvalidWorkspace, recipe.ID)
			}
		}
		if _, ok := seen[recipe.ID]; ok {
			return fmt.Errorf("%w: duplicate recipe %s", ErrInvalidWorkspace, recipe.ID)
		}
		seen[recipe.ID] = struct{}{}
	}
	return nil
}

// Profile returns a detached profile by ID.
func (w Workspace) Profile(id string) (Profile, bool) {
	for _, profile := range w.Profiles {
		if profile.ID == id {
			profile.ProjectIDs = slices.Clone(profile.ProjectIDs)
			return profile, true
		}
	}
	return Profile{}, false
}
