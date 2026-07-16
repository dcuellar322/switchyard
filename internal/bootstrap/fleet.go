package bootstrap

import (
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	fleetApplication "switchyard.dev/switchyard/internal/fleet/application"
	"switchyard.dev/switchyard/internal/fleet/domain"
)

func parseRemoteControllers(values []string) ([]fleetApplication.ControllerGrant, error) {
	result := make([]fleetApplication.ControllerGrant, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		fingerprint, capabilitiesText, ok := strings.Cut(value, "=")
		fingerprint = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(fingerprint), ":", ""))
		decoded, decodeErr := hex.DecodeString(fingerprint)
		if !ok || decodeErr != nil || len(decoded) != 32 || capabilitiesText == "" {
			return nil, fmt.Errorf("invalid remote controller %q; expected SHA256=capability,capability", value)
		}
		if _, exists := seen[fingerprint]; exists {
			return nil, fmt.Errorf("duplicate remote controller certificate %s", fingerprint)
		}
		seen[fingerprint] = struct{}{}
		capabilities := []domain.Capability{}
		for _, item := range strings.Split(capabilitiesText, ",") {
			capability := domain.Capability(strings.TrimSpace(item))
			if !slices.Contains(domain.KnownCapabilities, capability) || slices.Contains(capabilities, capability) {
				return nil, fmt.Errorf("invalid or duplicate remote controller capability %q", item)
			}
			capabilities = append(capabilities, capability)
		}
		if !slices.Contains(capabilities, domain.CapabilityInventoryRead) {
			return nil, errors.New("every remote controller requires inventory.read")
		}
		result = append(result, fleetApplication.ControllerGrant{Fingerprint: fingerprint, Capabilities: capabilities})
	}
	if len(result) == 0 {
		return nil, errors.New("at least one remote controller grant is required")
	}
	return result, nil
}

func validateRemoteConfig(config Config) error {
	values := []string{
		config.RemoteTLSCertificate, config.RemoteTLSKey, config.RemoteClientCA,
		config.RemoteMachineID, config.RemoteMachineName,
	}
	if config.RemoteAddr == "" {
		for _, value := range values {
			if value != "" {
				return errors.New("remote agent settings require --remote-address")
			}
		}
		if len(config.RemoteControllers) > 0 {
			return errors.New("remote controller grants require --remote-address")
		}
		return nil
	}
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return errors.New("remote agent requires server certificate, key, client CA, machine ID, and machine name")
		}
	}
	for _, path := range []string{config.RemoteTLSCertificate, config.RemoteTLSKey, config.RemoteClientCA} {
		if !filepath.IsAbs(path) {
			return fmt.Errorf("remote certificate paths must be absolute: %s", path)
		}
	}
	if len(config.RemoteMachineID) > 128 || len(config.RemoteMachineName) > 128 {
		return errors.New("remote machine identity fields must be at most 128 characters")
	}
	_, err := parseRemoteControllers(config.RemoteControllers)
	return err
}
