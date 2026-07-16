// Package application owns plugin discovery reconciliation, trust, capability
// enforcement, supervision, and project-facing use cases.
package application

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"switchyard.dev/switchyard/internal/plugins/domain"
	pluginsdk "switchyard.dev/switchyard/sdk/plugin"
)

var (
	// ErrNotFound identifies an unknown registration.
	ErrNotFound = errors.New("plugin not found")
	// ErrTrustRequired identifies an executable that has not been reviewed.
	ErrTrustRequired = errors.New("plugin trust required")
	// ErrFingerprint identifies a changed or mismatched package identity.
	ErrFingerprint = errors.New("plugin fingerprint changed")
	// ErrPermissionDenied identifies an undeclared or ungranted capability.
	ErrPermissionDenied = errors.New("plugin permission denied")
	// ErrDisabled identifies a known registration that is not enabled.
	ErrDisabled = errors.New("plugin is disabled")
	// ErrInvocation contains a supervised process or protocol failure.
	ErrInvocation = errors.New("plugin invocation failed")
	// ErrDiscovery identifies a malformed installed package.
	ErrDiscovery = errors.New("plugin discovery failed")
)

// Repository persists discovery, explicit trust, grants, health, and logs.
type Repository interface {
	Reconcile(context.Context, []domain.Plugin, time.Time) error
	List(context.Context) ([]domain.Plugin, error)
	Get(context.Context, string) (domain.Plugin, error)
	Trust(context.Context, string, string, time.Time) error
	SetEnabled(context.Context, string, bool, []string, time.Time) error
	SetHealth(context.Context, string, domain.HealthState, string, string, time.Time) error
	AppendLogs(context.Context, []domain.LogEntry) error
	Logs(context.Context, string, int) ([]domain.LogEntry, error)
}

// Discovery reads package declarations without executing them.
type Discovery interface {
	Discover(context.Context) ([]domain.Plugin, error)
}

// Invocation is one supervised, host-authorized plugin process call.
type Invocation struct {
	Plugin domain.Plugin
	Scopes []pluginsdk.Scope
	Method string
	Params any
	Result any
}

// Runner owns external process lifetime and JSON-RPC exchange.
type Runner interface {
	Call(context.Context, Invocation) ([]domain.LogEntry, error)
}

// Project is the only catalog data a plugin may receive.
type Project struct {
	ID, DisplayName, Root string
	Trusted               bool
}

// ProjectSource resolves an explicit application boundary to catalog state.
type ProjectSource interface {
	Project(context.Context, string) (Project, error)
}

// Service coordinates plugin use without exposing process details to domains.
type Service struct {
	repository Repository
	discovery  Discovery
	runner     Runner
	projects   ProjectSource
	now        func() time.Time
}

// NewService constructs the plugin application service.
func NewService(repository Repository, discovery Discovery, runner Runner, projects ProjectSource) (*Service, error) {
	if repository == nil || discovery == nil || runner == nil || projects == nil {
		return nil, errors.New("plugin service dependencies are required")
	}
	return &Service{repository: repository, discovery: discovery, runner: runner, projects: projects, now: time.Now}, nil
}

// Refresh performs deterministic discovery only; it never launches a plugin.
func (s *Service) Refresh(ctx context.Context) ([]domain.Plugin, error) {
	discovered, err := s.discovery.Discover(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDiscovery, err)
	}
	if err := s.repository.Reconcile(ctx, discovered, s.now().UTC()); err != nil {
		return nil, err
	}
	return s.repository.List(ctx)
}

// List returns current durable discovery and review state without execution.
func (s *Service) List(ctx context.Context) ([]domain.Plugin, error) {
	return s.repository.List(ctx)
}

// Logs returns bounded, redacted supervision records.
func (s *Service) Logs(ctx context.Context, id string, limit int) ([]domain.LogEntry, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	return s.repository.Logs(ctx, id, limit)
}

// Trust records the exact fingerprint the user reviewed.
func (s *Service) Trust(ctx context.Context, id, fingerprint string) (domain.Plugin, error) {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return domain.Plugin{}, err
	}
	if !current.Available || fingerprint == "" || fingerprint != current.Fingerprint {
		return domain.Plugin{}, ErrFingerprint
	}
	if err := s.repository.Trust(ctx, id, fingerprint, s.now().UTC()); err != nil {
		return domain.Plugin{}, err
	}
	return s.repository.Get(ctx, id)
}

// Enable grants a reviewed subset and requires a successful protocol health call.
func (s *Service) Enable(ctx context.Context, id string, requested []string) (domain.Plugin, error) {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return domain.Plugin{}, err
	}
	if err := executableTrusted(current); err != nil {
		return domain.Plugin{}, err
	}
	granted, err := validatedGrants(current, requested)
	if err != nil {
		return domain.Plugin{}, err
	}
	if err := s.checkHealth(ctx, current, granted); err != nil {
		return domain.Plugin{}, err
	}
	if err := s.repository.SetEnabled(ctx, id, true, requested, s.now().UTC()); err != nil {
		return domain.Plugin{}, err
	}
	return s.repository.Get(ctx, id)
}

// Disable revokes every grant without executing plugin code.
func (s *Service) Disable(ctx context.Context, id string) (domain.Plugin, error) {
	if _, err := s.repository.Get(ctx, id); err != nil {
		return domain.Plugin{}, err
	}
	if err := s.repository.SetEnabled(ctx, id, false, nil, s.now().UTC()); err != nil {
		return domain.Plugin{}, err
	}
	return s.repository.Get(ctx, id)
}

// Health launches only a trusted, enabled process with its persisted grants.
func (s *Service) Health(ctx context.Context, id string) (domain.Plugin, error) {
	current, err := s.enabled(ctx, id)
	if err != nil {
		return domain.Plugin{}, err
	}
	granted, err := validatedGrants(current, current.GrantedScopes)
	if err != nil {
		return domain.Plugin{}, err
	}
	callErr := s.checkHealth(ctx, current, granted)
	result, getErr := s.repository.Get(ctx, id)
	if callErr != nil {
		return result, callErr
	}
	return result, getErr
}

// Inspect invokes one enabled declared read capability for a trusted project.
func (s *Service) Inspect(ctx context.Context, id, projectID string) (pluginsdk.InspectResult, error) {
	current, project, granted, err := s.authorizedProject(ctx, id, projectID, pluginsdk.CapabilityProjectInspect, pluginsdk.ScopeProjectMetadataRead)
	if err != nil {
		return pluginsdk.InspectResult{}, err
	}
	request := pluginsdk.InspectRequest{Project: pluginsdk.Project{ID: project.ID, DisplayName: project.DisplayName}}
	if slices.Contains(granted, pluginsdk.ScopeProjectFilesRead) {
		request.Project.Root = project.Root
	}
	var result pluginsdk.InspectResult
	logs, err := s.runner.Call(ctx, Invocation{Plugin: current, Scopes: granted, Method: "project.inspect", Params: request, Result: &result})
	s.record(ctx, current.ID, logs, err)
	if err != nil {
		return pluginsdk.InspectResult{}, fmt.Errorf("%w: %v", ErrInvocation, err)
	}
	return result, err
}

// Operate executes one typed action after trust and scope checks.
func (s *Service) Operate(ctx context.Context, id, projectID, action string, input []byte) (pluginsdk.OperateResult, error) {
	current, project, granted, err := s.authorizedProject(ctx, id, projectID, pluginsdk.CapabilityProjectOperate, pluginsdk.ScopeProjectOperate)
	if err != nil {
		return pluginsdk.OperateResult{}, err
	}
	request := pluginsdk.OperateRequest{Project: pluginsdk.Project{ID: project.ID, DisplayName: project.DisplayName}, Action: strings.TrimSpace(action), Input: input}
	if slices.Contains(granted, pluginsdk.ScopeProjectFilesRead) {
		request.Project.Root = project.Root
	}
	if request.Action == "" {
		return pluginsdk.OperateResult{}, errors.New("plugin action is required")
	}
	var result pluginsdk.OperateResult
	logs, err := s.runner.Call(ctx, Invocation{Plugin: current, Scopes: granted, Method: "project.operate", Params: request, Result: &result})
	s.record(ctx, current.ID, logs, err)
	if err != nil {
		return pluginsdk.OperateResult{}, fmt.Errorf("%w: %v", ErrInvocation, err)
	}
	return result, err
}

// ValidateOperation verifies current trust, grants, and project trust before a
// durable operation is accepted. Execution rechecks the same state.
func (s *Service) ValidateOperation(ctx context.Context, id, projectID string) error {
	_, _, _, err := s.authorizedProject(ctx, id, projectID, pluginsdk.CapabilityProjectOperate, pluginsdk.ScopeProjectOperate)
	return err
}

// RunHealth periodically observes enabled plugins. Failures are recorded and
// never escape the goroutine into daemon process control.
func (s *Service) RunHealth(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			plugins, err := s.List(ctx)
			if err != nil {
				continue
			}
			for _, current := range plugins {
				if current.Enabled {
					_, _ = s.Health(ctx, current.ID)
				}
			}
		}
	}
}

func (s *Service) checkHealth(ctx context.Context, current domain.Plugin, granted []pluginsdk.Scope) error {
	var result pluginsdk.HealthResult
	logs, err := s.runner.Call(ctx, Invocation{Plugin: current, Scopes: granted, Method: "plugin.health", Params: struct{}{}, Result: &result})
	s.record(ctx, current.ID, logs, err)
	if err != nil {
		_ = s.repository.SetHealth(context.WithoutCancel(ctx), current.ID, domain.HealthUnhealthy, "Plugin process or protocol unavailable", boundedError(err), s.now().UTC())
		return fmt.Errorf("%w: %v", ErrInvocation, err)
	}
	state := domain.HealthState(result.Status)
	if err := s.repository.SetHealth(ctx, current.ID, state, result.Message, "", s.now().UTC()); err != nil {
		return err
	}
	return nil
}

func (s *Service) enabled(ctx context.Context, id string) (domain.Plugin, error) {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return domain.Plugin{}, err
	}
	if err := executableTrusted(current); err != nil {
		return domain.Plugin{}, err
	}
	if !current.Enabled {
		return domain.Plugin{}, ErrDisabled
	}
	return current, nil
}

func (s *Service) authorizedProject(ctx context.Context, id, projectID string, capability pluginsdk.Capability, scope pluginsdk.Scope) (domain.Plugin, Project, []pluginsdk.Scope, error) {
	current, err := s.enabled(ctx, id)
	if err != nil {
		return domain.Plugin{}, Project{}, nil, err
	}
	if !slices.Contains(current.Capabilities, string(capability)) || !slices.Contains(current.GrantedScopes, string(scope)) {
		return domain.Plugin{}, Project{}, nil, fmt.Errorf("%w: %s requires %s", ErrPermissionDenied, capability, scope)
	}
	project, err := s.projects.Project(ctx, projectID)
	if err != nil {
		return domain.Plugin{}, Project{}, nil, err
	}
	if !project.Trusted {
		return domain.Plugin{}, Project{}, nil, errors.New("plugin access requires a trusted project")
	}
	granted, err := validatedGrants(current, current.GrantedScopes)
	return current, project, granted, err
}

func executableTrusted(current domain.Plugin) error {
	if !current.Available {
		return ErrNotFound
	}
	if current.TrustedFingerprint == "" {
		return ErrTrustRequired
	}
	if current.TrustedFingerprint != current.Fingerprint || current.Trust == domain.TrustChanged {
		return ErrFingerprint
	}
	if current.ProtocolVersion != pluginsdk.ProtocolVersion {
		return fmt.Errorf("plugin protocol %s is incompatible with host %s", current.ProtocolVersion, pluginsdk.ProtocolVersion)
	}
	return nil
}

func validatedGrants(current domain.Plugin, values []string) ([]pluginsdk.Scope, error) {
	seen := map[string]struct{}{}
	result := make([]pluginsdk.Scope, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if !slices.Contains(current.RequestedScopes, value) {
			return nil, fmt.Errorf("%w: scope %s was not requested", ErrPermissionDenied, value)
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, pluginsdk.Scope(value))
	}
	slices.Sort(result)
	return result, nil
}

func (s *Service) record(ctx context.Context, pluginID string, logs []domain.LogEntry, callErr error) {
	for index := range logs {
		logs[index].PluginID = pluginID
		if logs[index].Created.IsZero() {
			logs[index].Created = s.now().UTC()
		}
	}
	if callErr != nil {
		logs = append(logs, domain.LogEntry{PluginID: pluginID, Level: "error", Message: boundedError(callErr), Created: s.now().UTC()})
	}
	if len(logs) > 0 {
		_ = s.repository.AppendLogs(context.WithoutCancel(ctx), logs)
	}
}

func boundedError(err error) string {
	value := strings.TrimSpace(err.Error())
	if len(value) > 2048 {
		value = value[:2048]
	}
	return value
}
