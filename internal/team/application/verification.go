package application

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"switchyard.dev/switchyard/internal/team/domain"
)

var portableID = regexp.MustCompile(`^[a-z][a-z0-9.-]{0,127}$`)
var windowsAbsolutePath = regexp.MustCompile(`^[A-Za-z]:[\\/]`)

func (s *Service) verify(bundle domain.Bundle, publisher domain.Publisher) error {
	if bundle.SchemaVersion != domain.BundleSchemaVersion || !slices.Contains(domain.KnownBundleKinds, bundle.Kind) ||
		!portableID.MatchString(bundle.Metadata.ID) || bundle.Metadata.Name == "" || bundle.Metadata.Version == "" ||
		bundle.Metadata.PublisherID != publisher.ID || bundle.Signature.KeyID != publisher.ID ||
		bundle.Signature.Algorithm != domain.SignatureAlgorithm || bundle.Metadata.CreatedAt.IsZero() {
		return ErrInvalidBundle
	}
	if bundle.Metadata.ExpiresAt != nil && !bundle.Metadata.ExpiresAt.After(s.now()) {
		return fmt.Errorf("%w: bundle expired", ErrInvalidBundle)
	}
	publicKey, err := decodePublicKey(publisher.PublicKey)
	if err != nil {
		return ErrInvalidPublisher
	}
	signature, err := base64.StdEncoding.DecodeString(bundle.Signature.Value)
	if err != nil || len(signature) != ed25519.SignatureSize {
		return ErrSignature
	}
	canonical, err := CanonicalBundle(bundle)
	if err != nil || !ed25519.Verify(publicKey, canonical, signature) {
		return ErrSignature
	}
	return validatePayload(bundle.Kind, bundle.Payload)
}

func validatePayload(kind domain.BundleKind, raw json.RawMessage) error {
	if err := validatePortablePayload(raw); err != nil {
		return err
	}
	switch kind {
	case domain.KindProjectTemplate:
		var payload domain.ProjectTemplate
		if err := strictJSON(raw, &payload); err != nil || len(payload.Manifest) == 0 || len(payload.Variables) > 64 {
			return ErrInvalidBundle
		}
		seen := map[string]struct{}{}
		for _, variable := range payload.Variables {
			if !portableID.MatchString(variable.ID) || variable.Label == "" {
				return ErrInvalidBundle
			}
			if _, ok := seen[variable.ID]; ok {
				return ErrInvalidBundle
			}
			seen[variable.ID] = struct{}{}
		}
	case domain.KindPolicyPack:
		var payload domain.PolicyPack
		if err := strictJSON(raw, &payload); err != nil || !validPolicy(payload) {
			return ErrInvalidBundle
		}
	case domain.KindEnterpriseConfig:
		var payload domain.EnterpriseConfig
		if err := strictJSON(raw, &payload); err != nil || !validPolicy(payload.Policy) || !validPublisherIDs(payload.RequiredPublisherIDs) {
			return ErrInvalidBundle
		}
	case domain.KindPluginRegistry:
		return validateRegistry(raw)
	default:
		return ErrInvalidBundle
	}
	return nil
}

func validateRegistry(raw json.RawMessage) error {
	var payload domain.PluginRegistry
	if err := strictJSON(raw, &payload); err != nil || len(payload.Entries) > 1000 {
		return ErrInvalidBundle
	}
	seen := map[string]struct{}{}
	for _, entry := range payload.Entries {
		parsed, err := url.Parse(entry.DownloadURL)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" || !portableID.MatchString(entry.ID) ||
			entry.Name == "" || entry.Version == "" || entry.Publisher == "" || len(entry.SHA256) != 64 {
			return ErrInvalidBundle
		}
		if _, err := hex.DecodeString(entry.SHA256); err != nil {
			return ErrInvalidBundle
		}
		if _, exists := seen[entry.ID]; exists {
			return ErrInvalidBundle
		}
		seen[entry.ID] = struct{}{}
	}
	return nil
}

func validatePortablePayload(raw json.RawMessage) error {
	var value any
	if err := strictJSON(raw, &value); err != nil {
		return ErrInvalidBundle
	}
	var inspect func(any) error
	inspect = func(current any) error {
		switch item := current.(type) {
		case map[string]any:
			for key, child := range item {
				normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, "_", ""), "-", ""))
				if slices.Contains([]string{"password", "token", "apikey", "privatekey", "secretvalue", "credential"}, normalized) {
					return fmt.Errorf("%w: payload contains prohibited secret-bearing field %q", ErrInvalidBundle, key)
				}
				if err := inspect(child); err != nil {
					return err
				}
			}
		case []any:
			for _, child := range item {
				if err := inspect(child); err != nil {
					return err
				}
			}
		case string:
			trimmed := strings.TrimSpace(item)
			if filepath.IsAbs(trimmed) || windowsAbsolutePath.MatchString(trimmed) || strings.Contains(trimmed, "-----BEGIN ") {
				return fmt.Errorf("%w: payload contains a host path or private material", ErrInvalidBundle)
			}
		}
		return nil
	}
	return inspect(value)
}

// SignBundle validates and signs a portable canonical bundle envelope.
func SignBundle(bundle domain.Bundle, privateKey ed25519.PrivateKey) (domain.Bundle, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return domain.Bundle{}, errors.New("Ed25519 private key is invalid")
	}
	bundle.SchemaVersion = domain.BundleSchemaVersion
	if !slices.Contains(domain.KnownBundleKinds, bundle.Kind) || !portableID.MatchString(bundle.Metadata.ID) ||
		bundle.Metadata.Name == "" || bundle.Metadata.Version == "" || bundle.Metadata.CreatedAt.IsZero() {
		return domain.Bundle{}, ErrInvalidBundle
	}
	if err := validatePayload(bundle.Kind, bundle.Payload); err != nil {
		return domain.Bundle{}, err
	}
	bundle.Signature = domain.Signature{
		KeyID: PublisherID(privateKey.Public().(ed25519.PublicKey)), Algorithm: domain.SignatureAlgorithm,
	}
	bundle.Metadata.PublisherID = bundle.Signature.KeyID
	bundle.InstalledAt = nil
	canonical, err := CanonicalBundle(bundle)
	if err != nil {
		return domain.Bundle{}, err
	}
	bundle.Signature.Value = base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, canonical))
	return bundle, nil
}

// CanonicalBundle returns the normalized unsigned bytes covered by Ed25519.
func CanonicalBundle(bundle domain.Bundle) ([]byte, error) {
	payload, err := normalizeJSON(bundle.Payload)
	if err != nil {
		return nil, err
	}
	unsigned := struct {
		SchemaVersion string                `json:"schemaVersion"`
		Kind          domain.BundleKind     `json:"kind"`
		Metadata      domain.BundleMetadata `json:"metadata"`
		Payload       json.RawMessage       `json:"payload"`
	}{bundle.SchemaVersion, bundle.Kind, bundle.Metadata, payload}
	return json.Marshal(unsigned)
}

func normalizeJSON(raw []byte) ([]byte, error) {
	var value any
	if err := strictJSON(raw, &value); err != nil {
		return nil, err
	}
	return json.Marshal(value)
}

func strictJSON(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("JSON contains multiple values")
	}
	return nil
}

func decodePublicKey(value string) (ed25519.PublicKey, error) {
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(value))
	if err != nil || len(decoded) != ed25519.PublicKeySize {
		return nil, ErrInvalidPublisher
	}
	return ed25519.PublicKey(decoded), nil
}

func validPolicy(policy domain.PolicyPack) bool {
	return subset(policy.AllowedRemoteCapabilities, []string{"inventory.read", "project.operate", "environment.manage"}) &&
		subset(policy.AllowedRemoteActions, []string{"start", "stop", "restart", "rebuild"}) &&
		validPublisherIDs(policy.AllowedPluginPublishers)
}

func validPublisherIDs(values []string) bool {
	seen := map[string]struct{}{}
	for _, value := range values {
		if !strings.HasPrefix(value, "publisher-") || len(value) != len("publisher-")+32 {
			return false
		}
		if _, err := hex.DecodeString(strings.TrimPrefix(value, "publisher-")); err != nil {
			return false
		}
		if _, exists := seen[value]; exists {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func subset(values, allowed []string) bool {
	seen := map[string]struct{}{}
	for _, value := range values {
		if !slices.Contains(allowed, value) {
			return false
		}
		if _, exists := seen[value]; exists {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}
