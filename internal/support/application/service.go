// Package application coordinates previewable, redacted support evidence.
package application

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"switchyard.dev/switchyard/internal/support/domain"
)

var (
	// ErrInvalidQuery identifies unsupported support log bounds or levels.
	ErrInvalidQuery = errors.New("invalid internal log query")
	// ErrInvalidInput identifies an incomplete support preview input.
	ErrInvalidInput = errors.New("invalid support preview input")
)

// AdapterProbe observes non-executable local capability availability.
type AdapterProbe interface {
	Probe(context.Context) ([]domain.AdapterAvailability, error)
}

// InternalLogs reads the dedicated redacted control-plane log only.
type InternalLogs interface {
	List(context.Context, LogQuery) ([]domain.InternalLogEntry, error)
}

// Clock makes preview timestamps deterministic in tests.
type Clock func() time.Time

// LogQuery bounds and filters internal daemon events.
type LogQuery struct {
	Limit        int
	MinimumLevel string
}

// PreviewInput contains only explicitly allowlisted evidence.
type PreviewInput struct {
	System             domain.SystemIdentity
	Configuration      domain.SanitizedConfiguration
	AdditionalAdapters []domain.AdapterAvailability
}

// Service owns support preview and internal log use cases.
type Service struct {
	adapters AdapterProbe
	logs     InternalLogs
	now      Clock
}

// NewService creates the bounded support service.
func NewService(adapters AdapterProbe, logs InternalLogs, now Clock) (*Service, error) {
	if adapters == nil || logs == nil {
		return nil, errors.New("support service dependencies are required")
	}
	if now == nil {
		now = time.Now
	}
	return &Service{adapters: adapters, logs: logs, now: now}, nil
}

// Preview returns the exact document that an archive writer will persist.
func (s *Service) Preview(ctx context.Context, input PreviewInput) (domain.Preview, error) {
	if strings.TrimSpace(input.System.Version) == "" || input.System.DatabaseSchema < 1 {
		return domain.Preview{}, ErrInvalidInput
	}
	adapters, err := s.adapters.Probe(ctx)
	if err != nil {
		return domain.Preview{}, fmt.Errorf("probe support adapters: %w", err)
	}
	adapters = append(adapters, input.AdditionalAdapters...)
	slices.SortFunc(adapters, func(left, right domain.AdapterAvailability) int {
		return strings.Compare(left.ID, right.ID)
	})
	errorsOnly, err := s.Logs(ctx, LogQuery{Limit: 100, MinimumLevel: "WARN"})
	if err != nil {
		return domain.Preview{}, err
	}
	if errorsOnly == nil {
		errorsOnly = make([]domain.InternalLogEntry, 0)
	}
	return domain.Preview{
		SchemaVersion:  domain.PreviewSchema,
		GeneratedAt:    s.now().UTC(),
		System:         input.System,
		Adapters:       adapters,
		Configuration:  input.Configuration,
		InternalErrors: errorsOnly,
		Included: []string{
			"build and API versions", "adapter availability", "database schema version",
			"up to 100 recent redacted internal warnings and errors", "allowlisted sanitized daemon configuration",
		},
		Excluded: []string{
			"project source and repository contents", "resolved secrets and credentials",
			"project application logs and terminal output", "raw environment variables", "database contents",
		},
		Redaction: "credential patterns, configured user patterns, local home paths, and data-directory paths are replaced before persistence",
	}, nil
}

// Logs returns a bounded list from the dedicated internal log source.
func (s *Service) Logs(ctx context.Context, query LogQuery) ([]domain.InternalLogEntry, error) {
	query.MinimumLevel = strings.ToUpper(strings.TrimSpace(query.MinimumLevel))
	if query.MinimumLevel == "" {
		query.MinimumLevel = "DEBUG"
	}
	if query.Limit < 1 || query.Limit > 2_000 || !slices.Contains([]string{"DEBUG", "INFO", "WARN", "ERROR"}, query.MinimumLevel) {
		return nil, ErrInvalidQuery
	}
	entries, err := s.logs.List(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("read internal daemon logs: %w", err)
	}
	if len(entries) > query.Limit {
		entries = entries[len(entries)-query.Limit:]
	}
	return entries, nil
}
