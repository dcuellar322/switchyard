// Package domain owns deterministic discovery evidence and reviewable proposals.
package domain

import (
	"encoding/json"
	"time"

	manifest "switchyard.dev/switchyard/internal/manifest/domain"
)

// ScannerVersion identifies the rules used to build deterministic proposals.
const ScannerVersion = "deterministic/v2"

// SourceRange points to exact, one-based source lines.
type SourceRange struct {
	StartLine int `json:"startLine"`
	EndLine   int `json:"endLine"`
}

// Evidence is a bounded scanner observation. Data must never contain secrets.
type Evidence struct {
	ID         string          `json:"id"`
	Scanner    string          `json:"scanner"`
	Kind       string          `json:"kind"`
	SourcePath string          `json:"sourcePath"`
	Location   SourceRange     `json:"location"`
	Confidence float64         `json:"confidence"`
	Data       json.RawMessage `json:"data"`
	Warnings   []string        `json:"warnings"`
}

// ProposalStatus describes the human-review lifecycle.
type ProposalStatus string

const (
	// StatusProposed awaits human review.
	StatusProposed ProposalStatus = "proposed"
	// StatusAccepted is the trusted candidate snapshot.
	StatusAccepted ProposalStatus = "accepted"
	// StatusRejected records a declined candidate.
	StatusRejected ProposalStatus = "rejected"
	// StatusSuperseded is an older accepted candidate replaced by a revision.
	StatusSuperseded ProposalStatus = "superseded"
)

// Validation is persisted with a proposal without importing an application package.
type Validation struct {
	Valid    bool     `json:"valid"`
	Errors   []string `json:"errors"`
	Warnings []string `json:"warnings"`
}

// Proposal is an untrusted candidate assembled only from deterministic evidence.
type Proposal struct {
	ID                string             `json:"id"`
	ProjectID         string             `json:"projectId"`
	ScannerVersion    string             `json:"scannerVersion"`
	SchemaVersion     string             `json:"schemaVersion"`
	Candidate         manifest.Manifest  `json:"candidate"`
	Evidence          []Evidence         `json:"evidence"`
	ConfidenceByField map[string]float64 `json:"confidenceByField"`
	Unresolved        []string           `json:"unresolved"`
	Validation        Validation         `json:"validation"`
	Status            ProposalStatus     `json:"status"`
	CreatedAt         time.Time          `json:"createdAt"`
}
