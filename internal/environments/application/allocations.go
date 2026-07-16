package application

import (
	"fmt"

	"switchyard.dev/switchyard/internal/environments/domain"
)

func checkRuntimeConflicts(candidate domain.Environment, all []domain.Environment) error {
	for _, environment := range all {
		if environment.ID == candidate.ID {
			continue
		}
		if environment.Hostname == candidate.Hostname {
			return fmt.Errorf("%w: hostname %s is already assigned to %s", ErrRuntimeConflict, candidate.Hostname, environment.ID)
		}
		for _, requested := range candidate.Allocation.PortLeases {
			for _, existing := range environment.Allocation.PortLeases {
				if requested.Protocol == existing.Protocol && requested.HostPort == existing.HostPort {
					return fmt.Errorf("%w: %s port %d is already leased to %s", ErrRuntimeConflict, requested.Protocol, requested.HostPort, environment.ID)
				}
			}
		}
	}
	return nil
}

func existingAllocations(existing []domain.Environment, replacingProjectID string) (map[string]domain.Environment, map[int]struct{}) {
	byID := make(map[string]domain.Environment)
	used := make(map[int]struct{})
	for _, environment := range existing {
		if environment.ProjectID == replacingProjectID {
			byID[environment.ID] = environment
			continue
		}
		if environment.Allocation.PortOffset > 0 {
			used[environment.Allocation.PortOffset] = struct{}{}
		}
	}
	return byID, used
}

func reserveOffset(offset int, used map[int]struct{}) bool {
	if offset < 1 || offset > domain.MaximumPortOffset {
		return false
	}
	if _, exists := used[offset]; exists {
		return false
	}
	used[offset] = struct{}{}
	return true
}

func allocateOffset(seed int, used map[int]struct{}) (int, error) {
	for attempt := 0; attempt < domain.MaximumPortOffset; attempt++ {
		candidate := (seed-1+attempt)%domain.MaximumPortOffset + 1
		if reserveOffset(candidate, used) {
			return candidate, nil
		}
	}
	return 0, ErrAllocationExhausted
}

func validateAllocations(environments []domain.Environment) error {
	composeNames := make(map[string]string, len(environments))
	namespaces := make(map[string]string, len(environments))
	for _, environment := range environments {
		if err := environment.Validate(); err != nil {
			return fmt.Errorf("validate environment %q: %w", environment.ID, err)
		}
		if owner, exists := composeNames[environment.Allocation.ComposeProjectName]; exists && owner != environment.ID {
			return fmt.Errorf("compose project name collision between %q and %q", owner, environment.ID)
		}
		if owner, exists := namespaces[environment.Allocation.PortLeaseNamespace]; exists && owner != environment.ID {
			return fmt.Errorf("port lease namespace collision between %q and %q", owner, environment.ID)
		}
		composeNames[environment.Allocation.ComposeProjectName] = environment.ID
		namespaces[environment.Allocation.PortLeaseNamespace] = environment.ID
	}
	return nil
}
