// Package adapters connects portable team configuration to concrete validation
// and encryption libraries.
package adapters

import (
	"encoding/json"

	manifestDomain "switchyard.dev/switchyard/internal/manifest/domain"
)

// ManifestValidator validates a fully rendered v1 project declaration.
type ManifestValidator struct{}

func (ManifestValidator) ValidateManifestJSON(document []byte) error {
	var manifest manifestDomain.Manifest
	if err := json.Unmarshal(document, &manifest); err != nil {
		return err
	}
	return manifest.Validate()
}
