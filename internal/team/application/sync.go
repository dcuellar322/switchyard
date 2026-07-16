package application

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"switchyard.dev/switchyard/internal/team/domain"
)

func (s *Service) ExportSync(ctx context.Context) (domain.SyncDocument, error) {
	publishers, err := s.repository.ListPublishers(ctx)
	if err != nil {
		return domain.SyncDocument{}, err
	}
	bundles, err := s.repository.ListBundles(ctx, "")
	if err != nil {
		return domain.SyncDocument{}, err
	}
	for index := range bundles {
		bundles[index].InstalledAt = nil
	}
	return domain.SyncDocument{
		SchemaVersion: domain.SyncSchemaVersion, Publishers: publishers,
		Bundles: bundles, ExportedAt: s.now().UTC(),
	}, nil
}

func (s *Service) PreviewSync(ctx context.Context, document domain.SyncDocument) (domain.SyncPreview, error) {
	if document.SchemaVersion != domain.SyncSchemaVersion || len(document.Publishers) > 100 || len(document.Bundles) > 1000 {
		return domain.SyncPreview{}, errors.New("sync document is invalid or exceeds limits")
	}
	publishers := map[string]domain.Publisher{}
	for _, publisher := range document.Publishers {
		publicKey, err := decodePublicKey(publisher.PublicKey)
		if err != nil || publisher.ID != PublisherID(publicKey) {
			return domain.SyncPreview{}, ErrInvalidPublisher
		}
		publishers[publisher.ID] = publisher
	}
	warnings := []string{"Importing publisher trust requires explicit confirmation; encryption provides confidentiality, not publisher trust."}
	current, err := s.repository.ListPublishers(ctx)
	if err != nil {
		return domain.SyncPreview{}, err
	}
	for _, publisher := range current {
		incoming, replacing := publishers[publisher.ID]
		if !replacing {
			publishers[publisher.ID] = publisher
			continue
		}
		if incoming.PublicKey != publisher.PublicKey {
			return domain.SyncPreview{}, fmt.Errorf("%w: publisher identity collision for %s", ErrInvalidPublisher, publisher.ID)
		}
		if incoming.Name != publisher.Name {
			warnings = append(warnings, "Existing publisher "+publisher.ID+" display name will be replaced.")
		}
	}
	currentBundles, err := s.repository.ListBundles(ctx, "")
	if err != nil {
		return domain.SyncPreview{}, err
	}
	bundleSignatures := make(map[string]string, len(currentBundles))
	for _, bundle := range currentBundles {
		bundleSignatures[bundle.Metadata.ID] = bundle.Signature.Value
	}
	preview := domain.SyncPreview{
		PublisherCount: len(document.Publishers), BundleCount: len(document.Bundles),
		BundleIDs: []string{}, Warnings: warnings,
	}
	for _, bundle := range document.Bundles {
		publisher, ok := publishers[bundle.Metadata.PublisherID]
		if !ok {
			return domain.SyncPreview{}, ErrPublisherUntrusted
		}
		if err := s.verify(bundle, publisher); err != nil {
			return domain.SyncPreview{}, err
		}
		if signature, replacing := bundleSignatures[bundle.Metadata.ID]; replacing && signature != bundle.Signature.Value {
			preview.Warnings = append(preview.Warnings, "Existing bundle "+bundle.Metadata.ID+" will be replaced by a different signed revision.")
		}
		preview.BundleIDs = append(preview.BundleIDs, bundle.Metadata.ID)
	}
	slices.Sort(preview.BundleIDs)
	return preview, nil
}

func (s *Service) ImportSync(ctx context.Context, document domain.SyncDocument, confirm bool, actor Actor) (domain.SyncPreview, error) {
	preview, err := s.PreviewSync(ctx, document)
	if err != nil {
		return domain.SyncPreview{}, err
	}
	if !confirm {
		return preview, ErrConfirmation
	}
	now := s.now().UTC()
	for index := range document.Publishers {
		document.Publishers[index].TrustedAt = now
	}
	for index := range document.Bundles {
		document.Bundles[index].InstalledAt = &now
	}
	if err := s.repository.ApplySync(ctx, document); err != nil {
		return domain.SyncPreview{}, err
	}
	s.audit(ctx, domain.AuditEvent{
		Type: "sync.imported", ActorType: actorType(actor), ActorID: actorID(actor),
		SubjectID: domain.SyncSchemaVersion, Detail: fmt.Sprintf("publishers=%d bundles=%d", preview.PublisherCount, preview.BundleCount), OccurredAt: now,
	})
	return preview, nil
}
