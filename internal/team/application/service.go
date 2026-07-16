// Package application verifies, installs, renders, and evaluates portable team
// configuration through explicit trust and confirmation boundaries.
package application

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"switchyard.dev/switchyard/internal/team/domain"
)

var (
	ErrNotFound           = errors.New("team configuration not found")
	ErrInvalidPublisher   = errors.New("publisher identity is invalid")
	ErrPublisherUntrusted = errors.New("bundle publisher is not trusted")
	ErrInvalidBundle      = errors.New("signed configuration bundle is invalid")
	ErrSignature          = errors.New("bundle signature verification failed")
	ErrConfirmation       = errors.New("team configuration change requires explicit confirmation")
	ErrPolicyDenied       = errors.New("enterprise policy denied the capability")
)

type Repository interface {
	TrustPublisher(context.Context, domain.Publisher) error
	ListPublishers(context.Context) ([]domain.Publisher, error)
	GetPublisher(context.Context, string) (domain.Publisher, error)
	InstallBundle(context.Context, domain.Bundle) error
	ListBundles(context.Context, domain.BundleKind) ([]domain.Bundle, error)
	GetBundle(context.Context, string) (domain.Bundle, error)
	ApplySync(context.Context, domain.SyncDocument) error
	RecordAudit(context.Context, domain.AuditEvent) error
}

type ManifestValidator interface {
	ValidateManifestJSON([]byte) error
}

type Service struct {
	repository Repository
	manifests  ManifestValidator
	now        func() time.Time
}

type Actor struct{ Type, ID string }

func NewService(repository Repository, manifests ManifestValidator) (*Service, error) {
	if repository == nil || manifests == nil {
		return nil, errors.New("team configuration dependencies are required")
	}
	return &Service{repository: repository, manifests: manifests, now: time.Now}, nil
}

func PublisherID(publicKey ed25519.PublicKey) string {
	digest := sha256.Sum256(publicKey)
	return "publisher-" + hex.EncodeToString(digest[:16])
}

func (s *Service) TrustPublisher(ctx context.Context, name, encodedPublicKey string, confirm bool, actor Actor) (domain.Publisher, error) {
	if !confirm {
		return domain.Publisher{}, ErrConfirmation
	}
	publicKey, err := decodePublicKey(encodedPublicKey)
	if err != nil || strings.TrimSpace(name) == "" || len(name) > 128 {
		return domain.Publisher{}, ErrInvalidPublisher
	}
	publisher := domain.Publisher{
		ID: PublisherID(publicKey), Name: strings.TrimSpace(name),
		PublicKey: base64.StdEncoding.EncodeToString(publicKey), TrustedAt: s.now().UTC(),
	}
	if err := s.repository.TrustPublisher(ctx, publisher); err != nil {
		return domain.Publisher{}, err
	}
	s.audit(ctx, domain.AuditEvent{
		Type: "publisher.trusted", ActorType: actorType(actor), ActorID: actorID(actor),
		SubjectID: publisher.ID, Detail: "explicit public signing key trust", OccurredAt: publisher.TrustedAt,
	})
	return publisher, nil
}

func (s *Service) Publishers(ctx context.Context) ([]domain.Publisher, error) {
	return s.repository.ListPublishers(ctx)
}

func (s *Service) Install(ctx context.Context, bundle domain.Bundle, confirm bool, actor Actor) (domain.Bundle, error) {
	if !confirm {
		return domain.Bundle{}, ErrConfirmation
	}
	publisher, err := s.repository.GetPublisher(ctx, bundle.Metadata.PublisherID)
	if errors.Is(err, ErrNotFound) {
		return domain.Bundle{}, ErrPublisherUntrusted
	}
	if err != nil {
		return domain.Bundle{}, err
	}
	if err := s.verify(bundle, publisher); err != nil {
		return domain.Bundle{}, err
	}
	if bundle.Kind == domain.KindEnterpriseConfig {
		var enterprise domain.EnterpriseConfig
		if err := strictJSON(bundle.Payload, &enterprise); err != nil {
			return domain.Bundle{}, ErrInvalidBundle
		}
		for _, requiredPublisherID := range enterprise.RequiredPublisherIDs {
			if _, err := s.repository.GetPublisher(ctx, requiredPublisherID); err != nil {
				return domain.Bundle{}, fmt.Errorf("%w: required publisher %s is not trusted", ErrPolicyDenied, requiredPublisherID)
			}
		}
	}
	installedAt := s.now().UTC()
	bundle.InstalledAt = &installedAt
	if err := s.repository.InstallBundle(ctx, bundle); err != nil {
		return domain.Bundle{}, err
	}
	s.audit(ctx, domain.AuditEvent{
		Type: "bundle.installed", ActorType: actorType(actor), ActorID: actorID(actor),
		SubjectID: bundle.Metadata.ID, Detail: string(bundle.Kind) + " signed by " + bundle.Metadata.PublisherID, OccurredAt: installedAt,
	})
	return bundle, nil
}

func (s *Service) Bundles(ctx context.Context, kind domain.BundleKind) ([]domain.Bundle, error) {
	if kind != "" && !slices.Contains(domain.KnownBundleKinds, kind) {
		return nil, ErrInvalidBundle
	}
	return s.repository.ListBundles(ctx, kind)
}

func (s *Service) audit(ctx context.Context, event domain.AuditEvent) {
	_ = s.repository.RecordAudit(context.WithoutCancel(ctx), event)
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
