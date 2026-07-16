// Package domain owns port facts, conflicts, reservations, and suggestions.
package domain

import "time"

// FactKind keeps configuration, protection, and live listener evidence distinct.
type FactKind string

// Supported port fact kinds.
const (
	KindDeclaration FactKind = "declaration"
	KindReservation FactKind = "reservation"
	KindBinding     FactKind = "binding"
)

// ConflictType identifies why two otherwise independent facts cannot coexist.
type ConflictType string

// Supported port conflict classifications.
const (
	ConflictDeclaredDeclared ConflictType = "DECLARED_VS_DECLARED"
	ConflictDeclaredReserved ConflictType = "DECLARED_VS_RESERVED"
	ConflictDeclaredBound    ConflictType = "DECLARED_VS_BOUND"
	ConflictReservedReserved ConflictType = "RESERVED_VS_RESERVED"
	ConflictUnknownBinding   ConflictType = "BOUND_BY_UNKNOWN_PROCESS"
	ConflictProtocolMismatch ConflictType = "PROTOCOL_MISMATCH"
	ConflictHostOverlap      ConflictType = "HOST_ADDRESS_OVERLAP"
)

// Fact is one provenance-bearing claim about a host port.
type Fact struct {
	ID          string    `json:"id"`
	Kind        FactKind  `json:"kind"`
	ProjectID   string    `json:"projectId,omitempty"`
	ProjectName string    `json:"projectName,omitempty"`
	ServiceID   string    `json:"serviceId,omitempty"`
	PortID      string    `json:"portId,omitempty"`
	Host        string    `json:"host"`
	Port        int       `json:"port"`
	Target      int       `json:"target,omitempty"`
	Protocol    string    `json:"protocol"`
	Source      string    `json:"source"`
	Evidence    string    `json:"evidence"`
	ProcessID   int       `json:"processId,omitempty"`
	ObservedAt  time.Time `json:"observedAt"`
}

// Conflict explains a pair of incompatible facts without proposing mutation.
type Conflict struct {
	ID      string       `json:"id"`
	Type    ConflictType `json:"type"`
	Port    int          `json:"port"`
	Summary string       `json:"summary"`
	Facts   []Fact       `json:"facts"`
}

// Registry is one current, explicitly partial host view.
type Registry struct {
	Facts      []Fact     `json:"facts"`
	Conflicts  []Conflict `json:"conflicts"`
	ObservedAt time.Time  `json:"observedAt"`
	Warnings   []string   `json:"warnings"`
}

// Suggestion is a free port selected from an explicit preferred range.
type Suggestion struct {
	Port       int       `json:"port"`
	RangeStart int       `json:"rangeStart"`
	RangeEnd   int       `json:"rangeEnd"`
	Protocol   string    `json:"protocol"`
	ObservedAt time.Time `json:"observedAt"`
}
