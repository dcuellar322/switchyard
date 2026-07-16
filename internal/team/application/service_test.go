package application

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"slices"
	"testing"
	"time"

	"switchyard.dev/switchyard/internal/team/domain"
)

func TestSignedBundleTrustVerificationAndTamperRejection(t *testing.T) {
	t.Parallel()
	service, repository := newTeamTestService(t)
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.TrustPublisher(context.Background(), "Release team", base64.StdEncoding.EncodeToString(publicKey), false, Actor{}); !errors.Is(err, ErrConfirmation) {
		t.Fatalf("TrustPublisher() error = %v", err)
	}
	publisher, err := service.TrustPublisher(context.Background(), "Release team", base64.StdEncoding.EncodeToString(publicKey), true, Actor{Type: "user", ID: "test"})
	if err != nil {
		t.Fatal(err)
	}
	bundle := signTeamBundle(t, privateKey, domain.KindPolicyPack, "policy.base", domain.PolicyPack{
		AllowedRemoteCapabilities: []string{"inventory.read"},
		AllowedRemoteActions:      []string{"start"},
		AllowedPluginPublishers:   []string{publisher.ID},
		TelemetryAllowed:          false,
	})
	if _, err := service.Install(context.Background(), bundle, true, Actor{Type: "user", ID: "test"}); err != nil {
		t.Fatal(err)
	}

	tampered := bundle
	tampered.Metadata.ID = "policy.tampered"
	if _, err := service.Install(context.Background(), tampered, true, Actor{}); !errors.Is(err, ErrSignature) {
		t.Fatalf("Install(tampered) error = %v, want signature error", err)
	}
	if len(repository.audit) != 2 {
		t.Fatalf("audit events = %d, want trust and install", len(repository.audit))
	}
}

func TestEffectivePolicyAndCuratedRegistryAreRestrictive(t *testing.T) {
	t.Parallel()
	service, _ := newTeamTestService(t)
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	publisher, err := service.TrustPublisher(context.Background(), "Policy owner", base64.StdEncoding.EncodeToString(publicKey), true, Actor{})
	if err != nil {
		t.Fatal(err)
	}
	policy := signTeamBundle(t, privateKey, domain.KindPolicyPack, "policy.restrictive", domain.PolicyPack{
		AllowedRemoteCapabilities: []string{"inventory.read"}, AllowedRemoteActions: []string{"start"},
		AllowedPluginPublishers: []string{publisher.ID}, TelemetryAllowed: false,
	})
	installTeamBundle(t, service, policy)
	registry := signTeamBundle(t, privateKey, domain.KindPluginRegistry, "registry.official", domain.PluginRegistry{Entries: []domain.RegistryEntry{
		{ID: "verified-plugin", Name: "Verified", Version: "1.0.0", Publisher: publisher.ID, DownloadURL: "https://plugins.example.test/verified.tar.gz", SHA256: repeatHex("ab"), Platforms: []string{"linux/amd64"}},
		{ID: "wrong-publisher", Name: "Wrong", Version: "1.0.0", Publisher: "publisher-00000000000000000000000000000000", DownloadURL: "https://plugins.example.test/wrong.tar.gz", SHA256: repeatHex("cd")},
	}})
	installTeamBundle(t, service, registry)
	enterprise := signTeamBundle(t, privateKey, domain.KindEnterpriseConfig, "enterprise.required", domain.EnterpriseConfig{
		Policy:               domain.PolicyPack{AllowedRemoteCapabilities: []string{"inventory.read"}, AllowedRemoteActions: []string{"start"}, AllowedPluginPublishers: []string{publisher.ID}},
		RequiredPublisherIDs: []string{"publisher-00000000000000000000000000000000"}, RequireSignedConfiguration: true,
	})
	if _, err := service.Install(context.Background(), enterprise, true, Actor{}); !errors.Is(err, ErrPolicyDenied) {
		t.Fatalf("Install(enterprise with unknown required publisher) error = %v", err)
	}

	policyResult, err := service.EffectivePolicy(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(policyResult.AllowedRemoteCapabilities, []string{"inventory.read"}) || policyResult.TelemetryAllowed {
		t.Fatalf("effective policy = %#v", policyResult)
	}
	if err := service.AuthorizeRemote(context.Background(), "inventory.read", "start"); err != nil {
		t.Fatal(err)
	}
	if err := service.AuthorizeRemote(context.Background(), "project.operate", "start"); !errors.Is(err, ErrPolicyDenied) {
		t.Fatalf("AuthorizeRemote() error = %v", err)
	}
	entries, err := service.Registry(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].ID != "verified-plugin" {
		t.Fatalf("registry = %#v", entries)
	}
}

func TestTemplateRenderAndEncryptedSyncReview(t *testing.T) {
	t.Parallel()
	service, _ := newTeamTestService(t)
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.TrustPublisher(context.Background(), "Template owner", base64.StdEncoding.EncodeToString(publicKey), true, Actor{}); err != nil {
		t.Fatal(err)
	}
	template := signTeamBundle(t, privateKey, domain.KindProjectTemplate, "template.go", domain.ProjectTemplate{
		Manifest:  json.RawMessage(`{"schemaVersion":"switchyard.dev/v1alpha1","name":"{{name}}","root":"{{root}}"}`),
		Variables: []domain.TemplateVariable{{ID: "name", Label: "Name", Required: true}, {ID: "root", Label: "Root", Default: "."}},
	})
	installTeamBundle(t, service, template)
	rendered, err := service.RenderTemplate(context.Background(), template.Metadata.ID, map[string]string{"name": "sample"})
	if err != nil {
		t.Fatal(err)
	}
	if string(rendered) == "" || !json.Valid(rendered) {
		t.Fatalf("rendered manifest = %s", rendered)
	}

	document, err := service.ExportSync(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	importService, importRepository := newTeamTestService(t)
	preview, err := importService.ImportSync(context.Background(), document, false, Actor{})
	if !errors.Is(err, ErrConfirmation) || preview.BundleCount != 1 {
		t.Fatalf("ImportSync preview=%#v error=%v", preview, err)
	}
	if _, err := importService.ImportSync(context.Background(), document, true, Actor{Type: "user", ID: "test"}); err != nil {
		t.Fatal(err)
	}
	if len(importRepository.publishers) != 1 || len(importRepository.bundles) != 1 {
		t.Fatalf("imported publishers=%d bundles=%d", len(importRepository.publishers), len(importRepository.bundles))
	}
}

type teamTestRepository struct {
	publishers map[string]domain.Publisher
	bundles    map[string]domain.Bundle
	audit      []domain.AuditEvent
}

func newTeamTestService(t *testing.T) (*Service, *teamTestRepository) {
	t.Helper()
	repository := &teamTestRepository{publishers: map[string]domain.Publisher{}, bundles: map[string]domain.Bundle{}}
	service, err := NewService(repository, manifestValidatorStub{})
	if err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC) }
	return service, repository
}

type manifestValidatorStub struct{}

func (manifestValidatorStub) ValidateManifestJSON(value []byte) error {
	if !json.Valid(value) {
		return errors.New("invalid JSON")
	}
	return nil
}

func signTeamBundle(t *testing.T, key ed25519.PrivateKey, kind domain.BundleKind, id string, payload any) domain.Bundle {
	t.Helper()
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := SignBundle(domain.Bundle{Kind: kind, Metadata: domain.BundleMetadata{ID: id, Name: id, Version: "1.0.0", CreatedAt: time.Date(2026, 7, 16, 10, 0, 0, 0, time.UTC)}, Payload: encoded}, key)
	if err != nil {
		t.Fatal(err)
	}
	return bundle
}

func installTeamBundle(t *testing.T, service *Service, bundle domain.Bundle) {
	t.Helper()
	if _, err := service.Install(context.Background(), bundle, true, Actor{}); err != nil {
		t.Fatal(err)
	}
}

func repeatHex(pair string) string {
	result := ""
	for len(result) < 64 {
		result += pair
	}
	return result
}

func (r *teamTestRepository) TrustPublisher(_ context.Context, value domain.Publisher) error {
	r.publishers[value.ID] = value
	return nil
}
func (r *teamTestRepository) ListPublishers(context.Context) ([]domain.Publisher, error) {
	result := make([]domain.Publisher, 0, len(r.publishers))
	for _, value := range r.publishers {
		result = append(result, value)
	}
	return result, nil
}
func (r *teamTestRepository) GetPublisher(_ context.Context, id string) (domain.Publisher, error) {
	value, ok := r.publishers[id]
	if !ok {
		return domain.Publisher{}, ErrNotFound
	}
	return value, nil
}
func (r *teamTestRepository) InstallBundle(_ context.Context, value domain.Bundle) error {
	r.bundles[value.Metadata.ID] = value
	return nil
}
func (r *teamTestRepository) ListBundles(_ context.Context, kind domain.BundleKind) ([]domain.Bundle, error) {
	result := []domain.Bundle{}
	for _, value := range r.bundles {
		if kind == "" || value.Kind == kind {
			result = append(result, value)
		}
	}
	return result, nil
}
func (r *teamTestRepository) GetBundle(_ context.Context, id string) (domain.Bundle, error) {
	value, ok := r.bundles[id]
	if !ok {
		return domain.Bundle{}, ErrNotFound
	}
	return value, nil
}
func (r *teamTestRepository) ApplySync(_ context.Context, document domain.SyncDocument) error {
	for _, publisher := range document.Publishers {
		r.publishers[publisher.ID] = publisher
	}
	for _, bundle := range document.Bundles {
		r.bundles[bundle.Metadata.ID] = bundle
	}
	return nil
}
func (r *teamTestRepository) RecordAudit(_ context.Context, event domain.AuditEvent) error {
	r.audit = append(r.audit, event)
	return nil
}
