package httpclient

import (
	"context"
	"fmt"
	"net/http"

	"switchyard.dev/switchyard/internal/transport/contract/generated"
)

// TeamPublishers lists explicitly trusted signing identities.
func (c *Client) TeamPublishers(ctx context.Context) ([]generated.TeamPublisher, error) {
	response, err := c.generated.ListTeamPublishersWithResponse(ctx)
	if err != nil {
		return nil, fmt.Errorf("list team publishers: %w", err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return nil, apiError("list team publishers", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON200, nil
}

// TrustTeamPublisher stores one confirmed exact public key.
func (c *Client) TrustTeamPublisher(ctx context.Context, request generated.TeamPublisherTrustRequest) (generated.TeamPublisher, error) {
	response, err := c.generated.TrustTeamPublisherWithResponse(ctx, request)
	if err != nil {
		return generated.TeamPublisher{}, fmt.Errorf("trust team publisher: %w", err)
	}
	if response.StatusCode() != http.StatusCreated || response.JSON201 == nil {
		return generated.TeamPublisher{}, apiError("trust team publisher", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON201, nil
}

// TeamBundles lists installed verified shared configuration.
func (c *Client) TeamBundles(ctx context.Context, kind string) ([]generated.TeamBundle, error) {
	params := &generated.ListTeamBundlesParams{}
	if kind != "" {
		value := generated.TeamBundleKind(kind)
		params.Kind = &value
	}
	response, err := c.generated.ListTeamBundlesWithResponse(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("list team bundles: %w", err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return nil, apiError("list team bundles", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON200, nil
}

// InstallTeamBundle verifies and installs one confirmed signed bundle.
func (c *Client) InstallTeamBundle(ctx context.Context, request generated.TeamBundleInstallRequest) (generated.TeamBundle, error) {
	response, err := c.generated.InstallTeamBundleWithResponse(ctx, request)
	if err != nil {
		return generated.TeamBundle{}, fmt.Errorf("install team bundle: %w", err)
	}
	if response.StatusCode() != http.StatusCreated || response.JSON201 == nil {
		return generated.TeamBundle{}, apiError("install team bundle", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON201, nil
}

// RenderTeamTemplate resolves and validates one signed project template.
func (c *Client) RenderTeamTemplate(ctx context.Context, id string, values map[string]string) (map[string]any, error) {
	response, err := c.generated.RenderTeamProjectTemplateWithResponse(ctx, id, generated.TeamTemplateRenderRequest{Values: values})
	if err != nil {
		return nil, fmt.Errorf("render team template: %w", err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return nil, apiError("render team template", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON200, nil
}

// EffectiveTeamPolicy reads the restrictive signed policy intersection.
func (c *Client) EffectiveTeamPolicy(ctx context.Context) (generated.EffectiveTeamPolicy, error) {
	response, err := c.generated.GetEffectiveTeamPolicyWithResponse(ctx)
	if err != nil {
		return generated.EffectiveTeamPolicy{}, fmt.Errorf("read effective team policy: %w", err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return generated.EffectiveTeamPolicy{}, apiError("read effective team policy", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON200, nil
}

// CuratedPlugins lists signed plugin metadata allowed by policy.
func (c *Client) CuratedPlugins(ctx context.Context) ([]generated.CuratedPlugin, error) {
	response, err := c.generated.ListCuratedPluginsWithResponse(ctx)
	if err != nil {
		return nil, fmt.Errorf("list curated plugins: %w", err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return nil, apiError("list curated plugins", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON200, nil
}

// ExportTeamSync reads configuration-only data for caller-side encryption.
func (c *Client) ExportTeamSync(ctx context.Context) (generated.TeamSyncDocument, error) {
	response, err := c.generated.ExportTeamSyncWithResponse(ctx)
	if err != nil {
		return generated.TeamSyncDocument{}, fmt.Errorf("export team sync: %w", err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return generated.TeamSyncDocument{}, apiError("export team sync", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON200, nil
}

// PreviewTeamSync verifies one decrypted sync document without mutation.
func (c *Client) PreviewTeamSync(ctx context.Context, document generated.TeamSyncDocument) (generated.TeamSyncPreview, error) {
	response, err := c.generated.PreviewTeamSyncWithResponse(ctx, document)
	if err != nil {
		return generated.TeamSyncPreview{}, fmt.Errorf("preview team sync: %w", err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return generated.TeamSyncPreview{}, apiError("preview team sync", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON200, nil
}

// ImportTeamSync applies one previewed configuration document.
func (c *Client) ImportTeamSync(ctx context.Context, document generated.TeamSyncDocument, confirm bool) (generated.TeamSyncPreview, error) {
	response, err := c.generated.ImportTeamSyncWithResponse(ctx, generated.TeamSyncImportRequest{Document: document, ConfirmRisk: confirm})
	if err != nil {
		return generated.TeamSyncPreview{}, fmt.Errorf("import team sync: %w", err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return generated.TeamSyncPreview{}, apiError("import team sync", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON200, nil
}

// TelemetryStatus reads consent and the exact current payload preview.
func (c *Client) TelemetryStatus(ctx context.Context) (generated.TelemetryStatus, error) {
	response, err := c.generated.GetTelemetryStatusWithResponse(ctx)
	if err != nil {
		return generated.TelemetryStatus{}, fmt.Errorf("read telemetry status: %w", err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return generated.TelemetryStatus{}, apiError("read telemetry status", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON200, nil
}

// UpdateTelemetry explicitly opts in or opts out and clears counters.
func (c *Client) UpdateTelemetry(ctx context.Context, request generated.TelemetrySettingsRequest) (generated.TelemetryStatus, error) {
	response, err := c.generated.UpdateTelemetrySettingsWithResponse(ctx, request)
	if err != nil {
		return generated.TelemetryStatus{}, fmt.Errorf("update telemetry settings: %w", err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return generated.TelemetryStatus{}, apiError("update telemetry settings", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON200, nil
}

// SendTelemetry delivers the exact current anonymous payload now.
func (c *Client) SendTelemetry(ctx context.Context) (generated.TelemetryStatus, error) {
	response, err := c.generated.SendTelemetryNowWithResponse(ctx)
	if err != nil {
		return generated.TelemetryStatus{}, fmt.Errorf("send anonymous telemetry: %w", err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return generated.TelemetryStatus{}, apiError("send anonymous telemetry", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON200, nil
}
