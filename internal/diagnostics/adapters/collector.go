// Package adapters maps existing application read models into bounded diagnostic evidence.
package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	actionsDomain "switchyard.dev/switchyard/internal/actions/domain"
	catalogDomain "switchyard.dev/switchyard/internal/catalog/domain"
	"switchyard.dev/switchyard/internal/diagnostics/domain"
	manifestApplication "switchyard.dev/switchyard/internal/manifest/application"
	observabilityDomain "switchyard.dev/switchyard/internal/observability/domain"
	operationsDomain "switchyard.dev/switchyard/internal/operations/domain"
	portsDomain "switchyard.dev/switchyard/internal/ports/domain"
	runtimeDomain "switchyard.dev/switchyard/internal/runtime/domain"
	sourcecontrolDomain "switchyard.dev/switchyard/internal/sourcecontrol/domain"
)

type catalogSource interface {
	GetProject(context.Context, string) (catalogDomain.Project, error)
	ListProjects(context.Context) ([]catalogDomain.Project, error)
	EffectiveManifest(context.Context, string, []byte) (manifestApplication.EffectiveManifest, error)
}

type runtimeSource interface {
	Inspect(context.Context, string) (runtimeDomain.Observation, error)
}

type healthSource interface {
	Get(context.Context, string) (observabilityDomain.ProjectHealth, error)
}

type logSource interface {
	Logs(context.Context, string, string, string, string, string, int) ([]runtimeDomain.LogEntry, error)
}

type portSource interface {
	Registry(context.Context) (portsDomain.Registry, error)
}

type gitSource interface {
	Get(context.Context, string) (sourcecontrolDomain.State, error)
}

type actionSource interface {
	List(context.Context, string) (actionsDomain.ProjectActions, error)
}

type resourceSource interface {
	Overview(context.Context) (observabilityDomain.ResourceOverview, error)
	CleanupPreview(context.Context, string) (observabilityDomain.CleanupPreview, error)
}

type operationSource interface {
	List(context.Context, string, int64) ([]operationsDomain.Operation, error)
}

// Collector gathers existing read models without executing repository commands beyond Git observation.
type Collector struct {
	catalog    catalogSource
	runtime    runtimeSource
	health     healthSource
	logs       logSource
	ports      portSource
	git        gitSource
	actions    actionSource
	resources  resourceSource
	operations operationSource
	now        func() time.Time
}

// NewCollector creates the explicit cross-domain diagnostic adapter.
func NewCollector(catalog catalogSource, runtime runtimeSource, health healthSource, logs logSource, ports portSource, git gitSource, actions actionSource, resources resourceSource, operations operationSource) *Collector {
	return &Collector{catalog: catalog, runtime: runtime, health: health, logs: logs, ports: ports, git: git, actions: actions, resources: resources, operations: operations, now: time.Now}
}

// Collect creates a bounded, redacted bundle and preserves partial-source warnings.
func (c *Collector) Collect(ctx context.Context, projectID string) (domain.Bundle, error) {
	project, err := c.catalog.GetProject(ctx, projectID)
	if err != nil {
		return domain.Bundle{}, err
	}
	now := c.now().UTC()
	bundle := domain.Bundle{
		ProjectID: project.ID, ProjectName: project.DisplayName, ProjectState: "unknown", TrustState: string(project.TrustState),
		ProjectAgeDay: max(0, int(now.Sub(project.UpdatedAt).Hours()/24)), CollectedAt: now,
		Evidence: []domain.Evidence{}, Actions: []domain.Action{}, Warnings: []string{},
		Snapshot: domain.ProjectSnapshot{ConfigSources: map[string]string{}, RecentLogs: []domain.LogLine{}, PortConflicts: []domain.PortConflict{}},
	}
	c.addEvidence(&bundle, "project", "project", "Registered project identity and age", "catalog", project, false, false, false, now)
	c.collectRuntime(ctx, &bundle)
	c.collectHealth(ctx, &bundle)
	c.collectGit(ctx, &bundle)
	c.collectPorts(ctx, &bundle)
	c.collectConfig(ctx, &bundle)
	c.collectLogs(ctx, &bundle)
	c.collectResources(ctx, &bundle)
	c.collectOperations(ctx, &bundle)
	actions, actionErr := c.ApprovedActions(ctx, projectID)
	if actionErr != nil {
		bundle.Warnings = append(bundle.Warnings, "Approved project actions are unavailable.")
	} else {
		bundle.Actions = actions
		c.addEvidence(&bundle, "actions", "actions", "Existing approved action identifiers and risk", "actions", actions, false, false, false, now)
	}
	sort.Strings(bundle.Warnings)
	return bundle, nil
}

// ApprovedActions exposes the current typed action vocabulary to automation validation.
func (c *Collector) ApprovedActions(ctx context.Context, projectID string) ([]domain.Action, error) {
	projectActions, err := c.actions.List(ctx, projectID)
	if err != nil {
		return nil, err
	}
	result := make([]domain.Action, 0, len(projectActions.Actions))
	for _, action := range projectActions.Actions {
		result = append(result, domain.Action{ID: action.ID, Name: action.Name, Type: action.Type, Risk: string(action.Risk)})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}

// ProjectIDs returns stable local project identifiers for scheduled evaluation.
func (c *Collector) ProjectIDs(ctx context.Context) ([]string, error) {
	projects, err := c.catalog.ListProjects(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(projects))
	for _, project := range projects {
		if project.TrustState == catalogDomain.TrustTrusted {
			result = append(result, project.ID)
		}
	}
	sort.Strings(result)
	return result, nil
}

func (c *Collector) collectRuntime(ctx context.Context, bundle *domain.Bundle) {
	observation, err := c.runtime.Inspect(ctx, bundle.ProjectID)
	if err != nil {
		bundle.Warnings = append(bundle.Warnings, "Runtime status is unavailable.")
		bundle.Snapshot.Runtime = domain.RuntimeSnapshot{State: "unknown", EngineConnected: false, Services: []domain.ServiceSnapshot{}}
		return
	}
	bundle.ProjectState = string(observation.State)
	runtime := domain.RuntimeSnapshot{State: string(observation.State), Driver: string(observation.Driver), EngineConnected: observation.Engine == nil || observation.Engine.Connected, Services: []domain.ServiceSnapshot{}}
	for _, service := range observation.Services {
		snapshot := domain.ServiceSnapshot{ID: service.ID, State: service.State, Health: service.Health}
		if service.Container != nil {
			snapshot.ExitCode, snapshot.RestartCount = service.Container.ExitCode, service.Container.RestartCount
		}
		if service.Process != nil {
			snapshot.ExitCode, snapshot.RestartCount = service.Process.ExitCode, service.Process.RestartCount
		}
		runtime.Services = append(runtime.Services, snapshot)
	}
	bundle.Snapshot.Runtime = runtime
	c.addEvidence(bundle, "runtime", "runtime", "Current runtime and service state", "runtime", runtime, false, false, false, observation.ObservedAt)
}

func (c *Collector) collectHealth(ctx context.Context, bundle *domain.Bundle) {
	health, err := c.health.Get(ctx, bundle.ProjectID)
	if err != nil {
		bundle.Warnings = append(bundle.Warnings, "Health checks are unavailable.")
		return
	}
	snapshot := domain.HealthSnapshot{Status: string(health.Status), ObserverState: string(health.ObserverState), Failures: []domain.HealthCheck{}}
	for _, result := range health.Results {
		if result.Required {
			bundle.Snapshot.RequiredChecks++
		}
		if result.Status == observabilityDomain.StatusHealthy {
			continue
		}
		failure := domain.HealthCheck{ID: result.CheckID, ServiceID: result.ServiceID, Status: string(result.Status), Severity: result.Severity, Required: result.Required, Message: bounded(result.Message, 1000)}
		snapshot.Failures = append(snapshot.Failures, failure)
		c.addEvidence(bundle, "health:"+result.CheckID, "health", "Failed or unknown health check", "health", failure, false, false, false, result.ObservedAt)
	}
	bundle.Snapshot.Health = snapshot
	c.addEvidence(bundle, "health", "health", "Aggregate project health", "health", snapshot, false, false, false, health.ObservedAt)
}

func (c *Collector) collectGit(ctx context.Context, bundle *domain.Bundle) {
	state, err := c.git.Get(ctx, bundle.ProjectID)
	if err != nil {
		bundle.Warnings = append(bundle.Warnings, "Git state is unavailable.")
		return
	}
	snapshot := domain.GitSnapshot{Repository: state.Repository, Branch: state.Branch, Modified: state.Changes.Modified, Staged: state.Changes.Staged, Untracked: state.Changes.Untracked, Conflicted: state.Changes.Conflicted, Operation: state.OperationState}
	bundle.Snapshot.Git = snapshot
	if state.LastCommit != nil {
		c.noteActivity(bundle, state.LastCommit.CommittedAt)
	}
	c.addEvidence(bundle, "git", "git", "Repository state without file content", "source-control", snapshot, true, false, false, state.ObservedAt)
}

func (c *Collector) collectPorts(ctx context.Context, bundle *domain.Bundle) {
	registry, err := c.ports.Registry(ctx)
	if err != nil {
		bundle.Warnings = append(bundle.Warnings, "Port registry is unavailable.")
		return
	}
	for _, conflict := range registry.Conflicts {
		if !conflictIncludesProject(conflict, bundle.ProjectID) {
			continue
		}
		mapped := domain.PortConflict{ID: conflict.ID, Type: string(conflict.Type), Port: conflict.Port, Summary: bounded(conflict.Summary, 1000)}
		bundle.Snapshot.PortConflicts = append(bundle.Snapshot.PortConflicts, mapped)
		c.addEvidence(bundle, "port:"+conflict.ID, "port", "Current project port conflict", "ports", mapped, false, false, false, registry.ObservedAt)
	}
}

func (c *Collector) collectConfig(ctx context.Context, bundle *domain.Bundle) {
	effective, err := c.catalog.EffectiveManifest(ctx, bundle.ProjectID, nil)
	if err != nil {
		bundle.Warnings = append(bundle.Warnings, "Configuration provenance is unavailable.")
		return
	}
	bundle.Snapshot.ConfigSources = effective.Provenance
	c.addEvidence(bundle, "config", "config", "Effective configuration field provenance", "manifest", effective.Provenance, true, false, false, bundle.CollectedAt)
}

func (c *Collector) collectLogs(ctx context.Context, bundle *domain.Bundle) {
	logs, err := c.logs.Logs(ctx, bundle.ProjectID, "", "", "", "", 100)
	if err != nil {
		bundle.Warnings = append(bundle.Warnings, "Recent logs are unavailable.")
		return
	}
	for index, entry := range logs {
		id := fmt.Sprintf("log:%d", entry.Sequence)
		if entry.Sequence == 0 {
			id = fmt.Sprintf("log:index-%d", index)
		}
		message := bounded(entry.Message, 2048)
		line := domain.LogLine{EvidenceID: id, ServiceID: entry.ServiceID, Level: entry.Level, Message: message, Timestamp: entry.Timestamp, Redacted: entry.Redacted}
		bundle.Snapshot.RecentLogs = append(bundle.Snapshot.RecentLogs, line)
		c.addEvidence(bundle, id, "log", "Recent redacted log line", "logs", line, true, entry.Redacted, len(message) < len(entry.Message), entry.Timestamp)
	}
}

func (c *Collector) collectResources(ctx context.Context, bundle *domain.Bundle) {
	overview, err := c.resources.Overview(ctx)
	if err != nil {
		bundle.Warnings = append(bundle.Warnings, "Resource observations are unavailable.")
	} else {
		for _, project := range overview.Projects {
			if project.ProjectID != bundle.ProjectID {
				continue
			}
			warnings := make([]string, 0, len(project.Warnings))
			for _, warning := range project.Warnings {
				warnings = append(warnings, warning.Message)
			}
			snapshot := domain.ResourceSnapshot{CPUPercent: project.Metric.CPUPercent, MemoryBytes: project.Metric.MemoryBytes, RestartCount: project.Metric.RestartCount, Warnings: warnings}
			bundle.Snapshot.Resources = snapshot
			c.addEvidence(bundle, "resources", "resources", "Current usage and sustained budget warnings", "resources", snapshot, false, false, false, overview.ObservedAt)
			break
		}
	}
	preview, previewErr := c.resources.CleanupPreview(ctx, bundle.ProjectID)
	if previewErr != nil {
		bundle.Warnings = append(bundle.Warnings, "Cleanup dry-run preview is unavailable.")
		return
	}
	cleanup := domain.CleanupSnapshot{EstimatedBytes: preview.EstimatedBytes, Candidates: len(preview.Resources), UnknownSizes: preview.UnknownSizes, Executable: false}
	bundle.Snapshot.Cleanup = cleanup
	c.addEvidence(bundle, "cleanup", "cleanup", "Non-executable storage cleanup preview", "resources", cleanup, false, false, false, preview.ObservedAt)
}

func (c *Collector) collectOperations(ctx context.Context, bundle *domain.Bundle) {
	operations, err := c.operations.List(ctx, bundle.ProjectID, 25)
	if err != nil {
		bundle.Warnings = append(bundle.Warnings, "Recent operation history is unavailable.")
		return
	}
	type operationSummary struct {
		Kind  string `json:"kind"`
		State string `json:"state"`
		Code  string `json:"errorCode,omitempty"`
	}
	summaries := []operationSummary{}
	for _, operation := range operations {
		c.noteActivity(bundle, operation.UpdatedAt)
		if operation.State == operationsDomain.StateFailed && strings.HasPrefix(operation.Kind, "runtime.") {
			bundle.Snapshot.FailedRuns++
		}
		summaries = append(summaries, operationSummary{Kind: operation.Kind, State: string(operation.State), Code: operation.ErrorCode})
	}
	c.addEvidence(bundle, "operations", "operations", "Recent operation outcomes without command input", "operations", summaries, false, false, false, bundle.CollectedAt)
}

func (c *Collector) noteActivity(bundle *domain.Bundle, at time.Time) {
	if at.IsZero() || at.After(bundle.CollectedAt) {
		return
	}
	age := max(0, int(bundle.CollectedAt.Sub(at).Hours()/24))
	if age < bundle.ProjectAgeDay {
		bundle.ProjectAgeDay = age
	}
}

func (c *Collector) addEvidence(bundle *domain.Bundle, id, kind, summary, source string, value any, untrusted, redacted, truncated bool, observedAt time.Time) {
	encoded, err := json.Marshal(value)
	if err != nil {
		bundle.Warnings = append(bundle.Warnings, "Diagnostic evidence could not be encoded: "+id)
		return
	}
	bundle.Evidence = append(bundle.Evidence, domain.Evidence{ID: id, Kind: kind, Summary: summary, Source: source, Data: encoded, Untrusted: untrusted, Redacted: redacted, Truncated: truncated, ObservedAt: observedAt.UTC()})
}

func conflictIncludesProject(conflict portsDomain.Conflict, projectID string) bool {
	for _, fact := range conflict.Facts {
		if fact.ProjectID == projectID {
			return true
		}
	}
	return false
}

func bounded(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "…"
}
