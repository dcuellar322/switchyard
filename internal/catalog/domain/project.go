// Package domain owns project catalog entities and trust transitions.
package domain

import (
	"errors"
	"time"
)

// TrustState controls whether repository-derived commands may be used.
type TrustState string

const (
	// TrustPending requires human review before runtime use.
	TrustPending TrustState = "pending"
	// TrustTrusted permits the accepted manifest to drive later runtime plans.
	TrustTrusted TrustState = "trusted"
	// TrustRejected records an explicit refusal to trust a proposal.
	TrustRejected TrustState = "rejected"
)

// Project is a repository registered with the local catalog.
type Project struct {
	ID               string     `json:"id"`
	Slug             string     `json:"slug"`
	DisplayName      string     `json:"displayName"`
	Description      string     `json:"description,omitempty"`
	TrustState       TrustState `json:"trustState"`
	PrimaryLocation  string     `json:"primaryLocation"`
	Tags             []string   `json:"tags"`
	ManifestRevision int64      `json:"manifestRevision"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}

// Trust returns a copy approved for runtime use.
func (p Project) Trust(at time.Time) (Project, error) {
	if p.TrustState != TrustPending {
		return Project{}, errors.New("only a pending project can be trusted")
	}
	p.TrustState = TrustTrusted
	p.ManifestRevision++
	p.UpdatedAt = at
	return p, nil
}
