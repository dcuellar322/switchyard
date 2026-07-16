package adapters

import (
	agents "switchyard.dev/switchyard/internal/agents/application"
	manifestApplication "switchyard.dev/switchyard/internal/manifest/application"
	manifestDomain "switchyard.dev/switchyard/internal/manifest/domain"
)

// ManifestValidator adapts the canonical validation use case to assisted onboarding.
type ManifestValidator struct{}

// Validate performs the same side-effect-free checks used before catalog acceptance.
func (ManifestValidator) Validate(root string, candidate manifestDomain.Manifest) agents.CandidateValidation {
	result := manifestApplication.Validate(root, candidate)
	return agents.CandidateValidation{Valid: result.Valid, Errors: result.Errors, Warnings: result.Warnings}
}
