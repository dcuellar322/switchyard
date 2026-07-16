// Package domain defines optional peer federation identities and grants without
// depending on HTTP, TLS, persistence, runtimes, or tunnels.
package domain

import (
	"errors"
	"slices"
	"time"
)

// ProtocolVersion is the stable narrow peer-agent contract identifier.
const ProtocolVersion = "switchyard.remote/v1"

// Capability is one separately grantable remote-agent permission.
type Capability string

const (
	// CapabilityInventoryRead permits bounded identity and inventory reads.
	CapabilityInventoryRead Capability = "inventory.read"
	// CapabilityProjectOperate permits typed project lifecycle requests.
	CapabilityProjectOperate Capability = "project.operate"
	// CapabilityEnvironmentManage permits typed registered-environment lifecycle requests.
	CapabilityEnvironmentManage Capability = "environment.manage"
)

// KnownCapabilities is the complete v1 remote permission vocabulary.
var KnownCapabilities = []Capability{
	CapabilityInventoryRead,
	CapabilityProjectOperate,
	CapabilityEnvironmentManage,
}

// MachineState describes the controller's latest bounded peer observation.
type MachineState string

const (
	// MachinePending has not completed its first authenticated probe.
	MachinePending MachineState = "pending"
	// MachineOnline completed its latest authenticated probe.
	MachineOnline MachineState = "online"
	// MachineDegraded is authenticated but incompatible or partially available.
	MachineDegraded MachineState = "degraded"
	// MachineOffline could not complete its latest authenticated probe.
	MachineOffline MachineState = "offline"
	// MachineDisabled has local access explicitly disabled.
	MachineDisabled MachineState = "disabled"
)

// CredentialReferences point at local certificate files. Private key material
// is never serialized into API responses or persisted in a bundle.
type CredentialReferences struct {
	CACertificate     string `json:"-"`
	ClientCertificate string `json:"-"`
	ClientKey         string `json:"-"`
}

// Complete reports whether all three local mTLS credential references exist.
func (r CredentialReferences) Complete() bool {
	return r.CACertificate != "" && r.ClientCertificate != "" && r.ClientKey != ""
}

// Machine is one explicitly configured remote Switchyard agent.
type Machine struct {
	ID                     string               `json:"id"`
	Name                   string               `json:"name"`
	Endpoint               string               `json:"endpoint"`
	CertificateFingerprint string               `json:"certificateFingerprint"`
	Credentials            CredentialReferences `json:"-"`
	CredentialConfigured   bool                 `json:"credentialConfigured"`
	Enabled                bool                 `json:"enabled"`
	Capabilities           []Capability         `json:"capabilities"`
	GrantedCapabilities    []Capability         `json:"grantedCapabilities"`
	State                  MachineState         `json:"state"`
	PeerID                 string               `json:"peerId,omitempty"`
	PeerVersion            string               `json:"peerVersion,omitempty"`
	OS                     string               `json:"os,omitempty"`
	Architecture           string               `json:"architecture,omitempty"`
	LastError              string               `json:"lastError,omitempty"`
	LastSeenAt             *time.Time           `json:"lastSeenAt,omitempty"`
	CreatedAt              time.Time            `json:"createdAt"`
	UpdatedAt              time.Time            `json:"updatedAt"`
}

// HasGrant requires an enabled machine, a peer-declared capability, and a local grant.
func (m Machine) HasGrant(capability Capability) bool {
	return m.Enabled && slices.Contains(m.Capabilities, capability) && slices.Contains(m.GrantedCapabilities, capability)
}

// Identity is the bounded self-description returned by a remote agent.
type Identity struct {
	ProtocolVersion string       `json:"protocolVersion"`
	MachineID       string       `json:"machineId"`
	Name            string       `json:"name"`
	Version         string       `json:"version"`
	OS              string       `json:"os"`
	Architecture    string       `json:"architecture"`
	Capabilities    []Capability `json:"capabilities"`
}

// Validate enforces the complete compatible peer identity contract.
func (i Identity) Validate() error {
	if i.ProtocolVersion != ProtocolVersion || i.MachineID == "" || i.Name == "" || i.Version == "" || i.OS == "" || i.Architecture == "" {
		return errors.New("remote identity is incomplete or incompatible")
	}
	for _, capability := range i.Capabilities {
		if !slices.Contains(KnownCapabilities, capability) {
			return errors.New("remote identity declares an unknown capability")
		}
	}
	return nil
}

// Project is the redacted remote project summary exposed to controllers.
type Project struct {
	ID          string `json:"id"`
	Slug        string `json:"slug"`
	DisplayName string `json:"displayName"`
	Runtime     string `json:"runtime"`
	State       string `json:"state"`
	Health      string `json:"health"`
	Degraded    bool   `json:"degraded"`
}

// Environment is the redacted remote development-environment summary.
type Environment struct {
	ID           string `json:"id"`
	ProjectID    string `json:"projectId"`
	Name         string `json:"name"`
	Branch       string `json:"branch,omitempty"`
	State        string `json:"state"`
	Availability string `json:"availability"`
}

// Snapshot is intentionally read-only and excludes locations, logs, secrets,
// terminal output, Git changes, environment values, and runtime identifiers.
type Snapshot struct {
	Identity     Identity      `json:"identity"`
	Projects     []Project     `json:"projects"`
	Environments []Environment `json:"environments"`
	ObservedAt   time.Time     `json:"observedAt"`
}

// OperationAction is the complete typed remote lifecycle vocabulary.
type OperationAction string

const (
	// ActionStart starts a remote project or registered environment.
	ActionStart OperationAction = "start"
	// ActionStop stops a remote project or registered environment.
	ActionStop OperationAction = "stop"
	// ActionRestart restarts a remote project or registered environment.
	ActionRestart OperationAction = "restart"
	// ActionRebuild rebuilds a remote project or registered environment.
	ActionRebuild OperationAction = "rebuild"
)

// Valid reports whether the action belongs to the narrow v1 vocabulary.
func (a OperationAction) Valid() bool {
	return a == ActionStart || a == ActionStop || a == ActionRestart || a == ActionRebuild
}

// OperationRequest describes one confirmed idempotent remote lifecycle request.
type OperationRequest struct {
	RequestID     string          `json:"requestId"`
	ProjectID     string          `json:"projectId"`
	EnvironmentID string          `json:"environmentId,omitempty"`
	Action        OperationAction `json:"action"`
	ConfirmRisk   bool            `json:"confirmRisk"`
}

// OperationReceipt identifies the durable operation accepted by the peer.
type OperationReceipt struct {
	RequestID   string    `json:"requestId"`
	OperationID string    `json:"operationId"`
	State       string    `json:"state"`
	AcceptedAt  time.Time `json:"acceptedAt"`
}

// AuditEvent records one controller-side or agent-side federation decision.
type AuditEvent struct {
	MachineID, Type, ActorType, ActorID, RequestID, Detail string
	OccurredAt                                             time.Time
}
