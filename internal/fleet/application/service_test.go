package application

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"switchyard.dev/switchyard/internal/fleet/domain"
)

type memoryRepository struct {
	machine domain.Machine
	audits  []domain.AuditEvent
}

func (r *memoryRepository) Create(_ context.Context, machine domain.Machine) error {
	r.machine = machine
	return nil
}
func (r *memoryRepository) List(context.Context) ([]domain.Machine, error) {
	return []domain.Machine{r.machine}, nil
}
func (r *memoryRepository) Get(_ context.Context, id string) (domain.Machine, error) {
	if r.machine.ID != id {
		return domain.Machine{}, ErrNotFound
	}
	return r.machine, nil
}
func (r *memoryRepository) UpdateAccess(_ context.Context, id string, enabled bool, grants []domain.Capability, now time.Time) error {
	if r.machine.ID != id {
		return ErrNotFound
	}
	r.machine.Enabled, r.machine.GrantedCapabilities, r.machine.UpdatedAt = enabled, slices.Clone(grants), now
	if !enabled {
		r.machine.State = domain.MachineDisabled
	}
	return nil
}
func (r *memoryRepository) RecordObservation(_ context.Context, id string, snapshot domain.Snapshot, state domain.MachineState, message string, now time.Time) error {
	if r.machine.ID != id {
		return ErrNotFound
	}
	r.machine.State, r.machine.LastError, r.machine.UpdatedAt = state, message, now
	if snapshot.Identity.MachineID != "" {
		r.machine.Capabilities = slices.Clone(snapshot.Identity.Capabilities)
		r.machine.PeerID, r.machine.PeerVersion = snapshot.Identity.MachineID, snapshot.Identity.Version
		r.machine.OS, r.machine.Architecture, r.machine.LastSeenAt = snapshot.Identity.OS, snapshot.Identity.Architecture, &now
	}
	return nil
}
func (r *memoryRepository) Delete(_ context.Context, id string) error {
	if r.machine.ID != id {
		return ErrNotFound
	}
	r.machine = domain.Machine{}
	return nil
}
func (r *memoryRepository) RecordAudit(_ context.Context, event domain.AuditEvent) error {
	r.audits = append(r.audits, event)
	return nil
}

type peerStub struct {
	snapshot domain.Snapshot
	receipt  domain.OperationReceipt
	err      error
}

func (p *peerStub) Snapshot(context.Context, domain.Machine) (domain.Snapshot, error) {
	return p.snapshot, p.err
}
func (p *peerStub) Operate(context.Context, domain.Machine, domain.OperationRequest) (domain.OperationReceipt, error) {
	return p.receipt, p.err
}

func remoteIdentity() domain.Identity {
	return domain.Identity{ProtocolVersion: domain.ProtocolVersion, MachineID: "machine-peer", Name: "Build box", Version: "1.0.0", OS: "linux", Architecture: "amd64", Capabilities: slices.Clone(domain.KnownCapabilities)}
}

func registerRequest() RegisterRequest {
	return RegisterRequest{
		Name: "Build box", Endpoint: "https://127.0.0.1:19618", CertificateFingerprint: strings.Repeat("a", 64),
		Credentials:         domain.CredentialReferences{CACertificate: "/certs/ca.pem", ClientCertificate: "/certs/client.pem", ClientKey: "/certs/client-key.pem"},
		GrantedCapabilities: []domain.Capability{domain.CapabilityInventoryRead, domain.CapabilityProjectOperate}, ConfirmRisk: true,
	}
}

func TestServiceRequiresConfirmationAndPinsObservedIdentity(t *testing.T) {
	t.Parallel()
	repository := &memoryRepository{}
	peer := &peerStub{snapshot: domain.Snapshot{Identity: remoteIdentity(), ObservedAt: time.Now()}}
	service, err := NewService(repository, peer)
	if err != nil {
		t.Fatal(err)
	}
	request := registerRequest()
	request.ConfirmRisk = false
	if _, err := service.Register(context.Background(), request, Actor{}); !errors.Is(err, ErrConfirmationNeeded) {
		t.Fatalf("Register() error = %v", err)
	}
	request.ConfirmRisk = true
	machine, err := service.Register(context.Background(), request, Actor{Type: "user", ID: "fixture"})
	if err != nil {
		t.Fatal(err)
	}
	if machine.PeerID != "machine-peer" || machine.State != domain.MachineOnline || !machine.CredentialConfigured {
		t.Fatalf("machine = %#v", machine)
	}
	if len(repository.audits) != 2 || repository.audits[0].ActorID != "fixture" {
		t.Fatalf("audits = %#v", repository.audits)
	}
}

func TestServiceEnforcesDeclaredGrantsConfirmationAndAudit(t *testing.T) {
	t.Parallel()
	repository := &memoryRepository{}
	peer := &peerStub{snapshot: domain.Snapshot{Identity: remoteIdentity(), ObservedAt: time.Now()}, receipt: domain.OperationReceipt{OperationID: "op-1", State: "queued"}}
	service, _ := NewService(repository, peer)
	machine, err := service.Register(context.Background(), registerRequest(), Actor{Type: "user", ID: "fixture"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ConfigureAccess(context.Background(), machine.ID, true, []domain.Capability{domain.CapabilityInventoryRead, domain.CapabilityEnvironmentManage}, false, Actor{}); !errors.Is(err, ErrConfirmationNeeded) {
		t.Fatalf("ConfigureAccess confirmation error = %v", err)
	}
	if _, err := service.Operate(context.Background(), machine.ID, domain.OperationRequest{RequestID: "request-1", ProjectID: "project-1", EnvironmentID: "env-1", Action: domain.ActionStart, ConfirmRisk: true}, Actor{}); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("Operate environment error = %v", err)
	}
	receipt, err := service.Operate(context.Background(), machine.ID, domain.OperationRequest{RequestID: "request-2", ProjectID: "project-1", Action: domain.ActionRestart, ConfirmRisk: true}, Actor{Type: "user", ID: "fixture"})
	if err != nil || receipt.OperationID != "op-1" {
		t.Fatalf("receipt=%#v error=%v", receipt, err)
	}
	last := repository.audits[len(repository.audits)-1]
	if last.Type != "remote.operation.accepted" || last.RequestID != "request-2" || last.ActorID != "fixture" {
		t.Fatalf("last audit = %#v", last)
	}
}

func TestServiceRecordsOfflineWithoutReturningPeerInternals(t *testing.T) {
	t.Parallel()
	repository := &memoryRepository{}
	peer := &peerStub{snapshot: domain.Snapshot{Identity: remoteIdentity(), ObservedAt: time.Now()}}
	service, _ := NewService(repository, peer)
	machine, err := service.Register(context.Background(), registerRequest(), Actor{})
	if err != nil {
		t.Fatal(err)
	}
	peer.err = errors.New("private\npeer failure")
	machine, err = service.Probe(context.Background(), machine.ID, Actor{})
	if err != nil {
		t.Fatal(err)
	}
	if machine.State != domain.MachineOffline || machine.LastError != "private peer failure" {
		t.Fatalf("machine = %#v", machine)
	}
}

type inventoryStub struct{ err error }

func (i inventoryStub) Inventory(context.Context) ([]domain.Project, []domain.Environment, error) {
	return []domain.Project{{ID: "project-1"}}, nil, i.err
}

type operatorStub struct{ controller string }

func (o *operatorStub) SubmitRemote(_ context.Context, request domain.OperationRequest, controller string) (domain.OperationReceipt, error) {
	o.controller = controller
	return domain.OperationReceipt{RequestID: request.RequestID}, nil
}

func TestAgentServiceSeparatesTLSIdentityFromCapabilityAuthorization(t *testing.T) {
	t.Parallel()
	operator := &operatorStub{}
	agent, err := NewAgentService(remoteIdentity(), inventoryStub{}, operator, []ControllerGrant{{
		Fingerprint:  "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		Capabilities: []domain.Capability{domain.CapabilityInventoryRead},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := agent.Snapshot(context.Background(), "aa"); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("Snapshot() error = %v", err)
	}
	request := domain.OperationRequest{RequestID: "request-1", ProjectID: "project-1", Action: domain.ActionStart, ConfirmRisk: true}
	if _, err := agent.Operate(context.Background(), "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", request); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("Operate() error = %v", err)
	}
}
