package httpclient

import (
	"context"
	"fmt"
	"net/http"

	"switchyard.dev/switchyard/internal/transport/contract/generated"
)

// DaemonSettings returns current revisioned local preferences.
func (c *Client) DaemonSettings(ctx context.Context) (generated.DaemonSettingsStatus, error) {
	response, err := c.generated.GetDaemonSettingsWithResponse(ctx)
	if err != nil {
		return generated.DaemonSettingsStatus{}, fmt.Errorf("read daemon settings: %w", err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return generated.DaemonSettingsStatus{}, apiError("read daemon settings", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON200, nil
}

// UpdateDaemonSettings atomically replaces preferences at their embedded revision.
func (c *Client) UpdateDaemonSettings(ctx context.Context, settings generated.DaemonSettings, idempotencyKey string) (generated.DaemonSettingsStatus, error) {
	response, err := c.generated.UpdateDaemonSettingsWithResponse(ctx,
		&generated.UpdateDaemonSettingsParams{IdempotencyKey: idempotencyKey},
		generated.UpdateDaemonSettingsRequest{ExpectedRevision: settings.Revision, Settings: settings},
	)
	if err != nil {
		return generated.DaemonSettingsStatus{}, fmt.Errorf("update daemon settings: %w", err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return generated.DaemonSettingsStatus{}, apiError("update daemon settings", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON200, nil
}
