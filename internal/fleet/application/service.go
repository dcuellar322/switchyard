// Package application coordinates optional remote machine inventory and typed
// operations through explicit trust, permission, and audit boundaries.
package application

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"switchyard.dev/switchyard/internal/fleet/domain"
	"switchyard.dev/switchyard/internal/foundation/identifier"
)

var (
	ErrNotFound           = errors.New("remote machine not found")
	ErrInvalidMachine     = errors.New("remote machine configuration is invalid")
	ErrPermissionDenied   = errors.New("remote capability is not granted")
	ErrConfirmationNeeded = errors.New("remote mutation requires explicit confirmation")
	ErrPeerIdentity       = errors.New("remote peer identity changed")
)

type Repository interface {
	Create(context.Context, domain.Machine) error
	List(context.Context) ([]domain.Machine, error)
	Get(context.Context, string) (domain.Machine, error)
	UpdateAccess(context.Context, string, bool, []domain.Capability, time.Time) error
	RecordObservation(context.Context, string, domain.Snapshot, domain.MachineState, string, time.Time) error
	Delete(context.Context, string) error
	RecordAudit(context.Context, domain.AuditEvent) error
}

type PeerClient interface {
	Snapshot(context.Context, domain.Machine) (domain.Snapshot, error)
	Operate(context.Context, domain.Machine, domain.OperationRequest) (domain.OperationReceipt, error)
}

// Policy is an optional restrictive enterprise boundary. Local reviewed grants
// still apply when no policy bundles are installed.
type Policy interface {
	AuthorizeRemote(context.Context, string, string) error
}

type Actor struct{ Type, ID string }

type RegisterRequest struct {
	Name, Endpoint, CertificateFingerprint string
	Credentials                            domain.CredentialReferences
	GrantedCapabilities                    []domain.Capability
	ConfirmRisk                            bool
}

type Service struct {
	repository Repository
	peers      PeerClient
	policy     Policy
	now        func() time.Time
}

func NewService(repository Repository, peers PeerClient, policies ...Policy) (*Service, error) {
	if repository == nil || peers == nil {
		return nil, errors.New("fleet service dependencies are required")
	}
	service := &Service{repository: repository, peers: peers, now: time.Now}
	if len(policies) > 0 {
		service.policy = policies[0]
	}
	return service, nil
}

func (s *Service) Register(ctx context.Context, request RegisterRequest, actor Actor) (domain.Machine, error) {
	if !request.ConfirmRisk {
		return domain.Machine{}, ErrConfirmationNeeded
	}
	name := strings.TrimSpace(request.Name)
	endpoint, err := normalizeEndpoint(request.Endpoint)
	if err != nil || name == "" || len(name) > 128 || !request.Credentials.Complete() || !absoluteCredentials(request.Credentials) {
		return domain.Machine{}, ErrInvalidMachine
	}
	fingerprint, err := normalizeFingerprint(request.CertificateFingerprint)
	if err != nil {
		return domain.Machine{}, err
	}
	grants, err := normalizeCapabilities(request.GrantedCapabilities)
	if err != nil || !slices.Contains(grants, domain.CapabilityInventoryRead) {
		return domain.Machine{}, fmt.Errorf("%w: inventory.read is required for registration", ErrInvalidMachine)
	}
	id, err := identifier.New("machine")
	if err != nil {
		return domain.Machine{}, err
	}
	now := s.now().UTC()
	machine := domain.Machine{
		ID: id, Name: name, Endpoint: endpoint, CertificateFingerprint: fingerprint,
		Credentials: request.Credentials, CredentialConfigured: true, Enabled: true,
		GrantedCapabilities: grants, State: domain.MachinePending, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.repository.Create(ctx, machine); err != nil {
		return domain.Machine{}, err
	}
	s.recordAudit(ctx, domain.AuditEvent{MachineID: id, Type: "machine.registered", ActorType: actorType(actor), ActorID: actorID(actor), Detail: "explicit certificate pin and grants", OccurredAt: now})
	return s.Probe(ctx, id, actor)
}

func (s *Service) List(ctx context.Context) ([]domain.Machine, error) {
	items, err := s.repository.List(ctx)
	for index := range items {
		items[index].CredentialConfigured = items[index].Credentials.Complete()
	}
	return items, err
}

func (s *Service) Get(ctx context.Context, id string) (domain.Machine, error) {
	machine, err := s.repository.Get(ctx, id)
	machine.CredentialConfigured = machine.Credentials.Complete()
	return machine, err
}

func (s *Service) ConfigureAccess(ctx context.Context, id string, enabled bool, grants []domain.Capability, confirmRisk bool, actor Actor) (domain.Machine, error) {
	if !confirmRisk {
		return domain.Machine{}, ErrConfirmationNeeded
	}
	machine, err := s.repository.Get(ctx, id)
	if err != nil {
		return domain.Machine{}, err
	}
	normalized, err := normalizeCapabilities(grants)
	if err != nil {
		return domain.Machine{}, err
	}
	for _, grant := range normalized {
		if s.policy != nil {
			if err := s.policy.AuthorizeRemote(ctx, string(grant), ""); err != nil {
				return domain.Machine{}, fmt.Errorf("%w: %v", ErrPermissionDenied, err)
			}
		}
		if len(machine.Capabilities) > 0 && !slices.Contains(machine.Capabilities, grant) {
			return domain.Machine{}, fmt.Errorf("%w: peer does not declare %s", ErrPermissionDenied, grant)
		}
	}
	now := s.now().UTC()
	if err := s.repository.UpdateAccess(ctx, id, enabled, normalized, now); err != nil {
		return domain.Machine{}, err
	}
	s.recordAudit(ctx, domain.AuditEvent{MachineID: id, Type: "machine.access.updated", ActorType: actorType(actor), ActorID: actorID(actor), Detail: fmt.Sprintf("enabled=%t grants=%s", enabled, capabilityText(normalized)), OccurredAt: now})
	return s.Get(ctx, id)
}

func (s *Service) Probe(ctx context.Context, id string, actor Actor) (domain.Machine, error) {
	machine, err := s.repository.Get(ctx, id)
	if err != nil {
		return domain.Machine{}, err
	}
	if !machine.Enabled || !slices.Contains(machine.GrantedCapabilities, domain.CapabilityInventoryRead) {
		return domain.Machine{}, ErrPermissionDenied
	}
	snapshot, probeErr := s.peers.Snapshot(ctx, machine)
	now := s.now().UTC()
	if probeErr != nil {
		_ = s.repository.RecordObservation(context.WithoutCancel(ctx), id, domain.Snapshot{}, domain.MachineOffline, boundedError(probeErr), now)
		s.recordAudit(ctx, domain.AuditEvent{MachineID: id, Type: "machine.probe.failed", ActorType: actorType(actor), ActorID: actorID(actor), Detail: boundedError(probeErr), OccurredAt: now})
		return s.Get(ctx, id)
	}
	if err := snapshot.Identity.Validate(); err != nil || machine.PeerID != "" && machine.PeerID != snapshot.Identity.MachineID {
		_ = s.repository.RecordObservation(context.WithoutCancel(ctx), id, domain.Snapshot{}, domain.MachineDegraded, ErrPeerIdentity.Error(), now)
		return domain.Machine{}, ErrPeerIdentity
	}
	if err := s.repository.RecordObservation(ctx, id, snapshot, domain.MachineOnline, "", now); err != nil {
		return domain.Machine{}, err
	}
	s.recordAudit(ctx, domain.AuditEvent{MachineID: id, Type: "machine.probed", ActorType: actorType(actor), ActorID: actorID(actor), Detail: fmt.Sprintf("projects=%d environments=%d", len(snapshot.Projects), len(snapshot.Environments)), OccurredAt: now})
	return s.Get(ctx, id)
}

func (s *Service) Snapshot(ctx context.Context, id string) (domain.Snapshot, error) {
	machine, err := s.repository.Get(ctx, id)
	if err != nil {
		return domain.Snapshot{}, err
	}
	if !machine.HasGrant(domain.CapabilityInventoryRead) {
		return domain.Snapshot{}, ErrPermissionDenied
	}
	return s.peers.Snapshot(ctx, machine)
}

func (s *Service) Operate(ctx context.Context, machineID string, request domain.OperationRequest, actor Actor) (domain.OperationReceipt, error) {
	machine, err := s.repository.Get(ctx, machineID)
	if err != nil {
		return domain.OperationReceipt{}, err
	}
	capability := domain.CapabilityProjectOperate
	if request.EnvironmentID != "" {
		capability = domain.CapabilityEnvironmentManage
	}
	if !machine.HasGrant(capability) {
		s.recordAudit(ctx, domain.AuditEvent{MachineID: machineID, Type: "remote.operation.denied", ActorType: actorType(actor), ActorID: actorID(actor), RequestID: request.RequestID, Detail: string(capability), OccurredAt: s.now().UTC()})
		return domain.OperationReceipt{}, ErrPermissionDenied
	}
	if s.policy != nil {
		if err := s.policy.AuthorizeRemote(ctx, string(capability), string(request.Action)); err != nil {
			s.recordAudit(ctx, domain.AuditEvent{MachineID: machineID, Type: "remote.operation.denied", ActorType: actorType(actor), ActorID: actorID(actor), RequestID: request.RequestID, Detail: "enterprise policy", OccurredAt: s.now().UTC()})
			return domain.OperationReceipt{}, fmt.Errorf("%w: %v", ErrPermissionDenied, err)
		}
	}
	if !request.ConfirmRisk {
		return domain.OperationReceipt{}, ErrConfirmationNeeded
	}
	if request.ProjectID == "" || !request.Action.Valid() {
		return domain.OperationReceipt{}, errors.New("remote operation request is invalid")
	}
	if request.RequestID == "" {
		request.RequestID, err = identifier.New("remote")
		if err != nil {
			return domain.OperationReceipt{}, err
		}
	}
	receipt, callErr := s.peers.Operate(ctx, machine, request)
	detail := "accepted"
	eventType := "remote.operation.accepted"
	if callErr != nil {
		detail, eventType = boundedError(callErr), "remote.operation.failed"
	}
	s.recordAudit(ctx, domain.AuditEvent{MachineID: machineID, Type: eventType, ActorType: actorType(actor), ActorID: actorID(actor), RequestID: request.RequestID, Detail: detail, OccurredAt: s.now().UTC()})
	return receipt, callErr
}

func (s *Service) Remove(ctx context.Context, id string, confirmRisk bool, actor Actor) error {
	if !confirmRisk {
		return ErrConfirmationNeeded
	}
	if _, err := s.repository.Get(ctx, id); err != nil {
		return err
	}
	s.recordAudit(ctx, domain.AuditEvent{MachineID: id, Type: "machine.removed", ActorType: actorType(actor), ActorID: actorID(actor), OccurredAt: s.now().UTC()})
	return s.repository.Delete(ctx, id)
}

func normalizeEndpoint(value string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", ErrInvalidMachine
	}
	parsed.Path = strings.TrimSuffix(parsed.EscapedPath(), "/")
	if parsed.Path != "" && parsed.Path != "/" {
		return "", fmt.Errorf("%w: endpoint may not include a path", ErrInvalidMachine)
	}
	parsed.Path, parsed.RawPath = "", ""
	return parsed.String(), nil
}

func normalizeFingerprint(value string) (string, error) {
	value = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), ":", ""))
	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) != 32 {
		return "", fmt.Errorf("%w: certificate fingerprint must be SHA-256", ErrInvalidMachine)
	}
	return value, nil
}

func normalizeCapabilities(values []domain.Capability) ([]domain.Capability, error) {
	result := make([]domain.Capability, 0, len(values))
	for _, capability := range values {
		if !slices.Contains(domain.KnownCapabilities, capability) {
			return nil, fmt.Errorf("%w: unknown capability %q", ErrInvalidMachine, capability)
		}
		if !slices.Contains(result, capability) {
			result = append(result, capability)
		}
	}
	slices.Sort(result)
	return result, nil
}

func absoluteCredentials(value domain.CredentialReferences) bool {
	return filepath.IsAbs(value.CACertificate) && filepath.IsAbs(value.ClientCertificate) && filepath.IsAbs(value.ClientKey)
}

func capabilityText(values []domain.Capability) string {
	items := make([]string, len(values))
	for index, value := range values {
		items[index] = string(value)
	}
	return strings.Join(items, ",")
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

func (s *Service) recordAudit(ctx context.Context, event domain.AuditEvent) {
	_ = s.repository.RecordAudit(context.WithoutCancel(ctx), event)
}
