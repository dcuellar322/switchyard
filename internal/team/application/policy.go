package application

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"switchyard.dev/switchyard/internal/team/domain"
)

// Registry returns signed plugin metadata allowed by effective policy.
func (s *Service) Registry(ctx context.Context) ([]domain.RegistryEntry, error) {
	policy, err := s.EffectivePolicy(ctx)
	if err != nil {
		return nil, err
	}
	bundles, err := s.repository.ListBundles(ctx, domain.KindPluginRegistry)
	if err != nil {
		return nil, err
	}
	byID := map[string]domain.RegistryEntry{}
	for _, bundle := range bundles {
		if len(policy.SourceBundleIDs) > 0 && !slices.Contains(policy.AllowedPluginPublishers, bundle.Metadata.PublisherID) {
			continue
		}
		var registry domain.PluginRegistry
		if err := strictJSON(bundle.Payload, &registry); err != nil {
			return nil, err
		}
		for _, entry := range registry.Entries {
			if entry.Publisher != bundle.Metadata.PublisherID {
				continue
			}
			current, exists := byID[entry.ID]
			if !exists || entry.Version > current.Version {
				byID[entry.ID] = entry
			}
		}
	}
	result := make([]domain.RegistryEntry, 0, len(byID))
	for _, entry := range byID {
		result = append(result, entry)
	}
	slices.SortFunc(result, func(left, right domain.RegistryEntry) int { return strings.Compare(left.ID, right.ID) })
	return result, nil
}

// EffectivePolicy intersects every installed policy and enterprise bundle.
func (s *Service) EffectivePolicy(ctx context.Context) (domain.EffectivePolicy, error) {
	result := domain.EffectivePolicy{
		AllowedRemoteCapabilities: []string{"inventory.read", "project.operate", "environment.manage"},
		AllowedRemoteActions:      []string{"start", "stop", "restart", "rebuild"},
		TelemetryAllowed:          true,
	}
	for _, kind := range []domain.BundleKind{domain.KindPolicyPack, domain.KindEnterpriseConfig} {
		bundles, err := s.repository.ListBundles(ctx, kind)
		if err != nil {
			return domain.EffectivePolicy{}, err
		}
		for _, bundle := range bundles {
			policy, signedConfiguration, err := s.bundlePolicy(ctx, bundle)
			if err != nil {
				return domain.EffectivePolicy{}, err
			}
			result.SourceBundleIDs = append(result.SourceBundleIDs, bundle.Metadata.ID)
			result.AllowedRemoteCapabilities = intersection(result.AllowedRemoteCapabilities, policy.AllowedRemoteCapabilities)
			result.AllowedRemoteActions = intersection(result.AllowedRemoteActions, policy.AllowedRemoteActions)
			result.AllowedPluginPublishers = mergePublisherPolicy(result.AllowedPluginPublishers, policy.AllowedPluginPublishers, len(result.SourceBundleIDs) == 1)
			result.TelemetryAllowed = result.TelemetryAllowed && policy.TelemetryAllowed
			result.RequireSignedConfiguration = result.RequireSignedConfiguration || signedConfiguration
		}
	}
	return result, nil
}

func (s *Service) bundlePolicy(ctx context.Context, bundle domain.Bundle) (domain.PolicyPack, bool, error) {
	if bundle.Kind == domain.KindPolicyPack {
		var policy domain.PolicyPack
		if err := strictJSON(bundle.Payload, &policy); err != nil {
			return domain.PolicyPack{}, false, err
		}
		return policy, false, nil
	}
	var enterprise domain.EnterpriseConfig
	if err := strictJSON(bundle.Payload, &enterprise); err != nil {
		return domain.PolicyPack{}, false, err
	}
	for _, publisherID := range enterprise.RequiredPublisherIDs {
		if _, err := s.repository.GetPublisher(ctx, publisherID); err != nil {
			return domain.PolicyPack{}, false, fmt.Errorf("%w: required publisher %s is not trusted", ErrPolicyDenied, publisherID)
		}
	}
	return enterprise.Policy, enterprise.RequireSignedConfiguration, nil
}

// AuthorizeRemote enforces signed policy as an additional fleet restriction.
func (s *Service) AuthorizeRemote(ctx context.Context, capability, action string) error {
	policy, err := s.EffectivePolicy(ctx)
	if err != nil {
		return err
	}
	if !slices.Contains(policy.AllowedRemoteCapabilities, capability) || action != "" && !slices.Contains(policy.AllowedRemoteActions, action) {
		return ErrPolicyDenied
	}
	return nil
}

// EffectiveTelemetryAllowed reports whether policy permits a user to opt in.
func (s *Service) EffectiveTelemetryAllowed(ctx context.Context) (bool, error) {
	policy, err := s.EffectivePolicy(ctx)
	return policy.TelemetryAllowed, err
}

func intersection(left, right []string) []string {
	result := []string{}
	for _, value := range left {
		if slices.Contains(right, value) {
			result = append(result, value)
		}
	}
	return result
}

func mergePublisherPolicy(current, next []string, first bool) []string {
	if first {
		return slices.Clone(next)
	}
	return intersection(current, next)
}
