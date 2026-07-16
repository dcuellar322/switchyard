// Package application builds, validates, persists, and reviews project diagnoses.
package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"switchyard.dev/switchyard/internal/diagnostics/domain"
	"switchyard.dev/switchyard/internal/foundation/identifier"
)

const maxBundleBytes = 256 << 10

var (
	// ErrDiagnosisNotFound identifies an unknown durable diagnosis.
	ErrDiagnosisNotFound = errors.New("diagnosis not found")
	// ErrInvalidFeedback identifies a feedback record outside the local review contract.
	ErrInvalidFeedback = errors.New("invalid diagnostic feedback")
	// ErrActionNotSuggested prevents a diagnosis from becoming an arbitrary action launcher.
	ErrActionNotSuggested = errors.New("action is not an approved diagnostic suggestion")
)

// Collector produces a bounded, already-redacted cross-domain snapshot.
type Collector interface {
	Collect(context.Context, string) (domain.Bundle, error)
}

// Provider receives inert evidence and a strict response schema.
type Provider interface {
	Diagnose(context.Context, string, json.RawMessage, json.RawMessage) (json.RawMessage, string, error)
}

// Repository stores diagnoses, feedback, and local notifications.
type Repository interface {
	SaveDiagnosis(context.Context, domain.Diagnosis) error
	GetDiagnosis(context.Context, string) (domain.Diagnosis, error)
	LatestDiagnosis(context.Context, string) (domain.Diagnosis, error)
	SaveFeedback(context.Context, domain.Feedback) error
	UpsertNotification(context.Context, domain.Notification) (domain.Notification, error)
	ListNotifications(context.Context, string, bool, int) ([]domain.Notification, error)
	AcknowledgeNotification(context.Context, string, time.Time) (domain.Notification, error)
}

// Service keeps deterministic diagnosis authoritative and optional AI constrained.
type Service struct {
	collector Collector
	provider  Provider
	repo      Repository
	now       func() time.Time
}

// NewService creates the diagnostic application boundary.
func NewService(collector Collector, provider Provider, repo Repository) (*Service, error) {
	if collector == nil || repo == nil {
		return nil, errors.New("diagnostic collector and repository are required")
	}
	return &Service{collector: collector, provider: provider, repo: repo, now: time.Now}, nil
}

// Diagnose always evaluates deterministic rules before any optional provider call.
func (s *Service) Diagnose(ctx context.Context, projectID, providerID string) (domain.Diagnosis, error) {
	bundle, err := s.collector.Collect(ctx, projectID)
	if err != nil {
		return domain.Diagnosis{}, err
	}
	if err := sealBundle(&bundle); err != nil {
		return domain.Diagnosis{}, err
	}
	id, err := identifier.New("diagnosis")
	if err != nil {
		return domain.Diagnosis{}, err
	}
	diagnosis := domain.Diagnosis{
		ID: id, Version: domain.DiagnosisVersion, ProjectID: projectID, Provider: providerID,
		Bundle: bundle, Hypotheses: Evaluate(bundle), Warnings: append([]string(nil), bundle.Warnings...),
		GeneratedAt: s.now().UTC(), Deterministic: providerID == "",
	}
	if providerID != "" {
		s.enhance(ctx, &diagnosis, providerID)
	}
	sortHypotheses(diagnosis.Hypotheses)
	if err := s.repo.SaveDiagnosis(ctx, diagnosis); err != nil {
		return domain.Diagnosis{}, fmt.Errorf("save diagnosis: %w", err)
	}
	if err := s.recordNotifications(ctx, diagnosis); err != nil {
		diagnosis.Warnings = append(diagnosis.Warnings, "A local diagnostic notification could not be recorded.")
	}
	return diagnosis, nil
}

// Get returns one durable diagnosis.
func (s *Service) Get(ctx context.Context, id string) (domain.Diagnosis, error) {
	return s.repo.GetDiagnosis(ctx, id)
}

// Latest returns the newest durable diagnosis for a project.
func (s *Service) Latest(ctx context.Context, projectID string) (domain.Diagnosis, error) {
	return s.repo.LatestDiagnosis(ctx, projectID)
}

// RecordFeedback persists local-only review without invoking the provider.
func (s *Service) RecordFeedback(ctx context.Context, diagnosisID, hypothesisID, verdict, note string) (domain.Feedback, error) {
	if verdict != "accurate" && verdict != "false_positive" || len(note) > 500 {
		return domain.Feedback{}, ErrInvalidFeedback
	}
	diagnosis, err := s.repo.GetDiagnosis(ctx, diagnosisID)
	if err != nil {
		return domain.Feedback{}, err
	}
	if !containsHypothesis(diagnosis.Hypotheses, hypothesisID) {
		return domain.Feedback{}, ErrInvalidFeedback
	}
	id, err := identifier.New("feedback")
	if err != nil {
		return domain.Feedback{}, err
	}
	feedback := domain.Feedback{ID: id, DiagnosisID: diagnosisID, HypothesisID: hypothesisID, Verdict: verdict, Note: strings.TrimSpace(note), CreatedAt: s.now().UTC()}
	if err := s.repo.SaveFeedback(ctx, feedback); err != nil {
		return domain.Feedback{}, err
	}
	return feedback, nil
}

// AuthorizeAction proves that a requested action came from a validated diagnosis.
func (s *Service) AuthorizeAction(ctx context.Context, diagnosisID, actionID string) (domain.Diagnosis, error) {
	diagnosis, err := s.repo.GetDiagnosis(ctx, diagnosisID)
	if err != nil {
		return domain.Diagnosis{}, err
	}
	for _, hypothesis := range diagnosis.Hypotheses {
		for _, action := range hypothesis.SuggestedActions {
			if action.ActionID == actionID {
				return diagnosis, nil
			}
		}
	}
	return domain.Diagnosis{}, ErrActionNotSuggested
}

// Notifications lists bounded local warnings; includeAcknowledged is explicit.
func (s *Service) Notifications(ctx context.Context, projectID string, includeAcknowledged bool, limit int) ([]domain.Notification, error) {
	if limit < 1 || limit > 500 {
		limit = 100
	}
	return s.repo.ListNotifications(ctx, projectID, includeAcknowledged, limit)
}

// Acknowledge marks one local warning reviewed.
func (s *Service) Acknowledge(ctx context.Context, id string) (domain.Notification, error) {
	return s.repo.AcknowledgeNotification(ctx, id, s.now().UTC())
}

func (s *Service) enhance(ctx context.Context, diagnosis *domain.Diagnosis, providerID string) {
	if s.provider == nil {
		diagnosis.Warnings = append(diagnosis.Warnings, "Optional AI diagnosis is not configured; deterministic results are complete.")
		return
	}
	bundle, err := providerBundle(diagnosis.Bundle)
	if err != nil {
		diagnosis.Warnings = append(diagnosis.Warnings, "The provider bundle could not be encoded.")
		return
	}
	schema, err := providerOutputSchema()
	if err != nil {
		diagnosis.Warnings = append(diagnosis.Warnings, "The provider response schema could not be prepared.")
		return
	}
	raw, model, err := s.provider.Diagnose(ctx, providerID, bundle, schema)
	if err != nil {
		diagnosis.Warnings = append(diagnosis.Warnings, "Optional AI diagnosis failed; deterministic results remain available: "+boundedMessage(err.Error(), 500))
		return
	}
	output, err := decodeProviderOutput(raw)
	if err != nil {
		diagnosis.Warnings = append(diagnosis.Warnings, "Optional AI output was rejected: "+boundedMessage(err.Error(), 500))
		return
	}
	hypotheses, warnings := validateProviderHypotheses(output, diagnosis.Bundle)
	diagnosis.Hypotheses = append(diagnosis.Hypotheses, hypotheses...)
	diagnosis.Warnings = append(diagnosis.Warnings, warnings...)
	diagnosis.Model = model
	diagnosis.Deterministic = len(hypotheses) == 0
}

func (s *Service) recordNotifications(ctx context.Context, diagnosis domain.Diagnosis) error {
	var result error
	for _, hypothesis := range diagnosis.Hypotheses {
		if !hypothesis.Notifies || hypothesis.Source != "deterministic" {
			continue
		}
		id := "notification_" + diagnosis.ProjectID + "_" + strings.ToLower(hypothesis.Code)
		_, err := s.repo.UpsertNotification(ctx, domain.Notification{
			ID: id, ProjectID: diagnosis.ProjectID, Code: hypothesis.Code, Title: hypothesis.Title,
			Detail: boundedMessage(hypothesis.Summary, 1000), Occurrences: 1,
			FirstSeenAt: diagnosis.GeneratedAt, LastSeenAt: diagnosis.GeneratedAt,
		})
		result = errors.Join(result, err)
	}
	return result
}

func sealBundle(bundle *domain.Bundle) error {
	bundle.Version = domain.BundleVersion
	if bundle.Evidence == nil {
		bundle.Evidence = []domain.Evidence{}
	}
	if bundle.Actions == nil {
		bundle.Actions = []domain.Action{}
	}
	if bundle.Warnings == nil {
		bundle.Warnings = []string{}
	}
	bundle.SHA256, bundle.EncodedBytes = "", 0
	encoded, err := json.Marshal(bundle)
	if err != nil {
		return fmt.Errorf("encode diagnostic bundle: %w", err)
	}
	if len(encoded) > maxBundleBytes {
		return fmt.Errorf("diagnostic bundle exceeds %d bytes", maxBundleBytes)
	}
	digest := sha256.Sum256(encoded)
	bundle.SHA256 = hex.EncodeToString(digest[:])
	bundle.EncodedBytes = len(encoded)
	return nil
}

func containsHypothesis(values []domain.Hypothesis, id string) bool {
	for _, value := range values {
		if value.ID == id {
			return true
		}
	}
	return false
}

func sortHypotheses(values []domain.Hypothesis) {
	sort.SliceStable(values, func(i, j int) bool {
		if values[i].Confidence != values[j].Confidence {
			return values[i].Confidence > values[j].Confidence
		}
		return values[i].Code < values[j].Code
	})
}

func boundedMessage(value string, limit int) string {
	value = strings.TrimSpace(value)
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "…"
}
