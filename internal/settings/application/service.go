// Package application coordinates durable settings, optimistic updates, safe
// project-root authorization, and restart-effect reporting.
package application

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"switchyard.dev/switchyard/internal/settings/domain"
)

var (
	// ErrInvalidSettings reports validation or path canonicalization failure.
	ErrInvalidSettings = errors.New("settings are invalid")
	// ErrRevisionConflict reports a stale optimistic-concurrency revision.
	ErrRevisionConflict = errors.New("settings revision conflict")
	// ErrOutsideProjectRoots rejects implicit discovery outside approved roots.
	ErrOutsideProjectRoots = errors.New("repository is outside configured project roots")
)

// Actor is the authenticated principal recorded for every settings mutation.
type Actor struct{ Type, ID string }

// Audit records a value-free settings change summary.
type Audit struct {
	ActorType  string
	ActorID    string
	Sections   []string
	OccurredAt time.Time
}

// Repository owns the singleton document and mutation audit.
type Repository interface {
	Initialize(context.Context, domain.Settings) (domain.Settings, error)
	Get(context.Context) (domain.Settings, error)
	Update(context.Context, int64, domain.Settings, Audit) (domain.Settings, error)
}

// Status distinguishes current durable preferences from daemon-startup values.
type Status struct {
	Settings       domain.Settings `json:"settings"`
	PendingRestart []string        `json:"pendingRestart"`
}

// Service owns one process's effective snapshot plus current durable settings.
type Service struct {
	repository Repository
	now        func() time.Time
	mu         sync.RWMutex
	effective  domain.Settings
}

// NewService constructs the settings use-case boundary.
func NewService(repository Repository) (*Service, error) {
	if repository == nil {
		return nil, errors.New("settings repository is required")
	}
	return &Service{repository: repository, now: time.Now}, nil
}

// Initialize canonicalizes defaults, creates the singleton once, and captures
// the values that this daemon process actually applied.
func (s *Service) Initialize(ctx context.Context, defaults domain.Settings) (Status, error) {
	normalized, err := normalize(defaults)
	if err != nil {
		return Status{}, err
	}
	normalized.Revision = 1
	normalized.UpdatedAt = s.now().UTC()
	current, err := s.repository.Initialize(ctx, normalized)
	if err != nil {
		return Status{}, err
	}
	current, err = validatePersisted(current)
	if err != nil {
		return Status{}, fmt.Errorf("persisted settings: %w", err)
	}
	s.mu.Lock()
	s.effective = current
	s.mu.Unlock()
	return Status{Settings: current, PendingRestart: []string{}}, nil
}

// Status returns current settings and fields waiting for a daemon restart.
func (s *Service) Status(ctx context.Context) (Status, error) {
	current, err := s.repository.Get(ctx)
	if err != nil {
		return Status{}, err
	}
	s.mu.RLock()
	effective := s.effective
	s.mu.RUnlock()
	return Status{Settings: current, PendingRestart: restartChanges(effective, current)}, nil
}

// Update validates and atomically replaces the full document at one revision.
func (s *Service) Update(ctx context.Context, expectedRevision int64, requested domain.Settings, actor Actor) (Status, error) {
	current, err := s.repository.Get(ctx)
	if err != nil {
		return Status{}, err
	}
	if expectedRevision != current.Revision {
		return Status{}, ErrRevisionConflict
	}
	normalized, err := normalize(requested)
	if err != nil {
		return Status{}, err
	}
	sections := changedSections(current, normalized)
	if len(sections) == 0 {
		s.mu.RLock()
		effective := s.effective
		s.mu.RUnlock()
		return Status{Settings: current, PendingRestart: restartChanges(effective, current)}, nil
	}
	normalized.Revision = expectedRevision + 1
	normalized.UpdatedAt = s.now().UTC()
	updated, err := s.repository.Update(ctx, expectedRevision, normalized, Audit{
		ActorType: actorType(actor), ActorID: actorID(actor), Sections: sections, OccurredAt: normalized.UpdatedAt,
	})
	if err != nil {
		return Status{}, err
	}
	s.mu.RLock()
	effective := s.effective
	s.mu.RUnlock()
	return Status{Settings: updated, PendingRestart: restartChanges(effective, updated)}, nil
}

// AuthorizeProjectRoot enforces the current root allowlist. An explicit
// outside-root override is required for a one-off scan.
func (s *Service) AuthorizeProjectRoot(ctx context.Context, candidate string, allowOutside bool) error {
	if allowOutside {
		return nil
	}
	current, err := s.repository.Get(ctx)
	if err != nil {
		return err
	}
	canonical, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return fmt.Errorf("%w: resolve repository: %v", ErrInvalidSettings, err)
	}
	for _, root := range current.ProjectRoots {
		if containedBy(root, canonical) {
			return nil
		}
	}
	return fmt.Errorf("%w: %s", ErrOutsideProjectRoots, canonical)
}

// PreferredTools exposes only the two user-facing adapter choices through an
// explicit application interface consumed by the actions context.
func (s *Service) PreferredTools(ctx context.Context) (string, string, error) {
	current, err := s.repository.Get(ctx)
	if err != nil {
		return "", "", err
	}
	return current.Tools.Terminal, current.Tools.Editor, nil
}

func normalize(settings domain.Settings) (domain.Settings, error) {
	settings.ProjectRoots = slices.Clone(settings.ProjectRoots)
	settings.Ports.Excluded = slices.Clone(settings.Ports.Excluded)
	settings.AI.Providers = slices.Clone(settings.AI.Providers)
	canonical := make([]string, 0, len(settings.ProjectRoots))
	for _, configured := range settings.ProjectRoots {
		configured = strings.TrimSpace(configured)
		absolute, err := filepath.Abs(configured)
		if err != nil {
			return domain.Settings{}, fmt.Errorf("%w: resolve project root: %v", ErrInvalidSettings, err)
		}
		resolved, err := filepath.EvalSymlinks(absolute)
		if err != nil {
			return domain.Settings{}, fmt.Errorf("%w: resolve project root %q: %v", ErrInvalidSettings, configured, err)
		}
		info, err := os.Stat(resolved)
		if err != nil || !info.IsDir() {
			return domain.Settings{}, fmt.Errorf("%w: project root %q is not a directory", ErrInvalidSettings, configured)
		}
		if filepath.Dir(resolved) == resolved {
			return domain.Settings{}, fmt.Errorf("%w: filesystem roots cannot be approved project roots", ErrInvalidSettings)
		}
		canonical = append(canonical, filepath.Clean(resolved))
	}
	slices.Sort(canonical)
	settings.ProjectRoots = slices.Compact(canonical)
	slices.Sort(settings.Ports.Excluded)
	settings.AI.DefaultProvider = strings.TrimSpace(settings.AI.DefaultProvider)
	for index := range settings.AI.Providers {
		provider := &settings.AI.Providers[index]
		provider.ID = strings.TrimSpace(provider.ID)
		provider.Executable = strings.TrimSpace(provider.Executable)
		provider.Endpoint = strings.TrimSpace(provider.Endpoint)
		provider.Model = strings.TrimSpace(provider.Model)
		provider.CredentialReference = strings.TrimSpace(provider.CredentialReference)
	}
	slices.SortFunc(settings.AI.Providers, func(left, right domain.ProviderPreferences) int { return strings.Compare(left.ID, right.ID) })
	if err := settings.Validate(); err != nil {
		return domain.Settings{}, fmt.Errorf("%w: %v", ErrInvalidSettings, err)
	}
	return settings, nil
}

func validatePersisted(settings domain.Settings) (domain.Settings, error) {
	settings.ProjectRoots = slices.Clone(settings.ProjectRoots)
	settings.Ports.Excluded = slices.Clone(settings.Ports.Excluded)
	settings.AI.Providers = slices.Clone(settings.AI.Providers)
	for index, root := range settings.ProjectRoots {
		if !filepath.IsAbs(root) || filepath.Dir(filepath.Clean(root)) == filepath.Clean(root) {
			return domain.Settings{}, fmt.Errorf("%w: persisted project root is not a canonical directory path", ErrInvalidSettings)
		}
		settings.ProjectRoots[index] = filepath.Clean(root)
	}
	slices.Sort(settings.ProjectRoots)
	settings.ProjectRoots = slices.Compact(settings.ProjectRoots)
	slices.Sort(settings.Ports.Excluded)
	slices.SortFunc(settings.AI.Providers, func(left, right domain.ProviderPreferences) int { return strings.Compare(left.ID, right.ID) })
	if err := settings.Validate(); err != nil {
		return domain.Settings{}, fmt.Errorf("%w: %v", ErrInvalidSettings, err)
	}
	return settings, nil
}

func containedBy(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	return err == nil && relative != ".." && !filepath.IsAbs(relative) && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func restartChanges(effective, current domain.Settings) []string {
	changes := []string{}
	if effective.Retention != current.Retention {
		changes = append(changes, "retention")
	}
	if !slices.Equal(effective.AI.Providers, current.AI.Providers) {
		changes = append(changes, "ai.providers")
	}
	return changes
}

func changedSections(before, after domain.Settings) []string {
	sections := []string{}
	if !slices.Equal(before.ProjectRoots, after.ProjectRoots) {
		sections = append(sections, "projectRoots")
	}
	if before.Ports.RangeStart != after.Ports.RangeStart || before.Ports.RangeEnd != after.Ports.RangeEnd || !slices.Equal(before.Ports.Excluded, after.Ports.Excluded) {
		sections = append(sections, "ports")
	}
	if before.Retention != after.Retention {
		sections = append(sections, "retention")
	}
	if before.Tools != after.Tools {
		sections = append(sections, "tools")
	}
	if before.AI.DefaultProvider != after.AI.DefaultProvider || !slices.Equal(before.AI.Providers, after.AI.Providers) {
		sections = append(sections, "ai")
	}
	if before.Permissions != after.Permissions {
		sections = append(sections, "permissions")
	}
	if before.Appearance != after.Appearance {
		sections = append(sections, "appearance")
	}
	return sections
}

func actorType(actor Actor) string {
	if strings.TrimSpace(actor.Type) != "" {
		return strings.TrimSpace(actor.Type)
	}
	return "local"
}

func actorID(actor Actor) string {
	if strings.TrimSpace(actor.ID) != "" {
		return strings.TrimSpace(actor.ID)
	}
	return "unknown"
}
