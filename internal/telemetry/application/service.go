// Package application owns explicit telemetry consent, bounded anonymous
// counters, and delivery. Disabled telemetry neither records nor sends.
package application

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"runtime"
	"slices"
	"strings"
	"time"

	"switchyard.dev/switchyard/internal/foundation/identifier"
	"switchyard.dev/switchyard/internal/telemetry/domain"
)

var (
	// ErrInvalidSettings indicates that the requested endpoint is not explicit HTTPS.
	ErrInvalidSettings = errors.New("telemetry settings are invalid")
	// ErrConfirmation indicates that enabling delivery was not explicitly confirmed.
	ErrConfirmation = errors.New("enabling telemetry requires explicit confirmation")
	// ErrDisabled indicates that delivery was requested without active consent.
	ErrDisabled = errors.New("anonymous telemetry is disabled")
	// ErrPolicyDenied indicates that signed policy prohibits telemetry opt-in.
	ErrPolicyDenied = errors.New("enterprise policy disables telemetry")
)

var allowedCounters = []string{
	"daemon.started", "runtime.operation", "action.operation", "workspace.operation",
	"plugin.operation", "diagnostic.operation", "ai.operation", "remote.operation",
}

// Repository persists consent, fixed counters, delivery state, and audit.
type Repository interface {
	Status(context.Context) (domain.Status, error)
	Configure(context.Context, domain.Settings, bool, domain.AuditEvent) error
	Increment(context.Context, string, time.Time) error
	RecordDelivery(context.Context, bool, string, time.Time) error
}

// Sender delivers one bounded payload to the explicitly configured endpoint.
type Sender interface {
	Send(context.Context, string, domain.Payload) error
}

// Policy may restrict whether a local user can opt in to telemetry.
type Policy interface {
	EffectiveTelemetryAllowed(context.Context) (bool, error)
}

// Build supplies the public version included in the exact payload preview.
type Build struct{ Version string }

// Actor identifies the authenticated requester recorded in consent audit.
type Actor struct{ Type, ID string }

// Service owns telemetry consent, anonymous aggregation, preview, and delivery.
type Service struct {
	repository Repository
	sender     Sender
	policy     Policy
	build      Build
	now        func() time.Time
}

// NewService constructs an opt-in telemetry boundary with no default endpoint.
func NewService(repository Repository, sender Sender, build Build, policies ...Policy) (*Service, error) {
	if repository == nil || sender == nil || build.Version == "" {
		return nil, errors.New("telemetry dependencies and build are required")
	}
	service := &Service{repository: repository, sender: sender, build: build, now: time.Now}
	if len(policies) > 0 {
		service.policy = policies[0]
	}
	return service, nil
}

// Status returns consent and the exact payload that would be sent now.
func (s *Service) Status(ctx context.Context) (domain.Status, error) {
	status, err := s.repository.Status(ctx)
	if err != nil {
		return domain.Status{}, err
	}
	if status.Settings.Enabled {
		preview := s.payload(status)
		status.Preview = &preview
	}
	return status, nil
}

// Configure opts in after confirmation or opts out and clears all counters.
func (s *Service) Configure(ctx context.Context, enabled bool, endpoint string, confirm bool, actor Actor) (domain.Status, error) {
	endpoint = strings.TrimSpace(endpoint)
	if enabled {
		if !confirm {
			return domain.Status{}, ErrConfirmation
		}
		parsed, err := url.Parse(endpoint)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
			return domain.Status{}, ErrInvalidSettings
		}
		if s.policy != nil {
			allowed, err := s.policy.EffectiveTelemetryAllowed(ctx)
			if err != nil {
				return domain.Status{}, err
			}
			if !allowed {
				return domain.Status{}, ErrPolicyDenied
			}
		}
	}
	current, err := s.repository.Status(ctx)
	if err != nil {
		return domain.Status{}, err
	}
	installationID := current.Settings.InstallationID
	if enabled && installationID == "" {
		installationID, err = identifier.New("anonymous")
		if err != nil {
			return domain.Status{}, err
		}
	}
	if !enabled {
		endpoint, installationID = "", ""
	}
	now := s.now().UTC()
	settings := domain.Settings{Enabled: enabled, Endpoint: endpoint, InstallationID: installationID, UpdatedAt: now}
	if err := s.repository.Configure(ctx, settings, !enabled, domain.AuditEvent{
		Type: "telemetry.configured", ActorType: actorType(actor), ActorID: actorID(actor),
		Detail: fmt.Sprintf("enabled=%t counters_cleared=%t", enabled, !enabled), OccurredAt: now,
	}); err != nil {
		return domain.Status{}, err
	}
	return s.Status(ctx)
}

// Record increments one fixed vocabulary counter only while consent is active.
func (s *Service) Record(ctx context.Context, counter string) {
	if !slices.Contains(allowedCounters, counter) {
		return
	}
	status, err := s.repository.Status(ctx)
	if err != nil || !status.Settings.Enabled {
		return
	}
	_ = s.repository.Increment(context.WithoutCancel(ctx), counter, s.now().UTC())
}

// ObserveOperation maps internal operation kinds into a non-identifying fixed
// category. Project, action, provider, plugin, and workspace IDs are discarded.
func (s *Service) ObserveOperation(ctx context.Context, kind string) {
	counter := ""
	switch {
	case strings.HasPrefix(kind, "runtime."):
		counter = "runtime.operation"
	case strings.HasPrefix(kind, "action."):
		counter = "action.operation"
	case strings.HasPrefix(kind, "workspace."):
		counter = "workspace.operation"
	case strings.HasPrefix(kind, "plugin."):
		counter = "plugin.operation"
	case strings.HasPrefix(kind, "diagnostic."):
		counter = "diagnostic.operation"
	case strings.HasPrefix(kind, "manifest."):
		counter = "ai.operation"
	case strings.HasPrefix(kind, "remote."):
		counter = "remote.operation"
	}
	if counter == "" {
		return
	}
	s.Record(ctx, counter)
}

// Send delivers the exact current payload only while consent is active.
func (s *Service) Send(ctx context.Context) (domain.Status, error) {
	status, err := s.repository.Status(ctx)
	if err != nil {
		return domain.Status{}, err
	}
	if !status.Settings.Enabled {
		return domain.Status{}, ErrDisabled
	}
	payload := s.payload(status)
	sendErr := s.sender.Send(ctx, status.Settings.Endpoint, payload)
	message := ""
	if sendErr != nil {
		message = boundedError(sendErr)
	}
	if err := s.repository.RecordDelivery(context.WithoutCancel(ctx), sendErr == nil, message, s.now().UTC()); err != nil {
		return domain.Status{}, errors.Join(sendErr, err)
	}
	current, statusErr := s.Status(ctx)
	return current, errors.Join(sendErr, statusErr)
}

func (s *Service) payload(status domain.Status) domain.Payload {
	return domain.Payload{
		SchemaVersion: domain.SchemaVersion, InstallationID: status.Settings.InstallationID,
		Version: s.build.Version, OS: runtime.GOOS, Architecture: runtime.GOARCH,
		Counters: append([]domain.Counter(nil), status.Counters...), GeneratedAt: s.now().UTC(),
	}
}

// Run periodically attempts delivery; disabled telemetry remains a no-op.
func (s *Service) Run(ctx context.Context, interval time.Duration) {
	if interval < time.Hour {
		interval = 24 * time.Hour
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _ = s.Send(ctx)
		}
	}
}

func boundedError(err error) string {
	value := strings.ReplaceAll(err.Error(), "\n", " ")
	if len(value) > 256 {
		value = value[:256]
	}
	return value
}

func actorType(actor Actor) string {
	if actor.Type != "" {
		return actor.Type
	}
	return "local"
}
func actorID(actor Actor) string {
	if actor.ID != "" {
		return actor.ID
	}
	return "unknown"
}
