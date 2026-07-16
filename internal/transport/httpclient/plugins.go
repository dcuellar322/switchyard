package httpclient

import (
	"context"
	"fmt"
	"net/http"

	"switchyard.dev/switchyard/internal/transport/contract/generated"
)

// Plugins lists registrations and optionally refreshes deterministic discovery.
func (c *Client) Plugins(ctx context.Context, refresh bool) ([]generated.PluginRegistration, error) {
	if refresh {
		response, err := c.generated.RefreshPluginsWithResponse(ctx)
		if err != nil {
			return nil, fmt.Errorf("refresh plugins: %w", err)
		}
		if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
			return nil, apiError("refresh plugins", response.StatusCode(), response.ApplicationproblemJSONDefault)
		}
		return *response.JSON200, nil
	}
	response, err := c.generated.ListPluginsWithResponse(ctx)
	if err != nil {
		return nil, fmt.Errorf("list plugins: %w", err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return nil, apiError("list plugins", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON200, nil
}

// TrustPlugin records one exact reviewed package fingerprint.
func (c *Client) TrustPlugin(ctx context.Context, id, fingerprint string) (generated.PluginRegistration, error) {
	response, err := c.generated.TrustPluginWithResponse(ctx, id, generated.PluginTrustRequest{Fingerprint: fingerprint})
	if err != nil {
		return generated.PluginRegistration{}, fmt.Errorf("trust plugin: %w", err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return generated.PluginRegistration{}, apiError("trust plugin", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON200, nil
}

// EnablePlugin enables a trusted plugin with explicit reviewed scopes.
func (c *Client) EnablePlugin(ctx context.Context, id string, scopes []string) (generated.PluginRegistration, error) {
	values := make([]generated.PluginEnableRequestGrantedScopes, len(scopes))
	for index, scope := range scopes {
		values[index] = generated.PluginEnableRequestGrantedScopes(scope)
	}
	response, err := c.generated.EnablePluginWithResponse(ctx, id, generated.PluginEnableRequest{GrantedScopes: values})
	if err != nil {
		return generated.PluginRegistration{}, fmt.Errorf("enable plugin: %w", err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return generated.PluginRegistration{}, apiError("enable plugin", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON200, nil
}

// DisablePlugin disables a plugin and revokes its grants.
func (c *Client) DisablePlugin(ctx context.Context, id string) (generated.PluginRegistration, error) {
	response, err := c.generated.DisablePluginWithResponse(ctx, id)
	if err != nil {
		return generated.PluginRegistration{}, fmt.Errorf("disable plugin: %w", err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return generated.PluginRegistration{}, apiError("disable plugin", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON200, nil
}

// CheckPlugin runs one supervised health exchange.
func (c *Client) CheckPlugin(ctx context.Context, id string) (generated.PluginRegistration, error) {
	response, err := c.generated.CheckPluginHealthWithResponse(ctx, id)
	if err != nil {
		return generated.PluginRegistration{}, fmt.Errorf("check plugin: %w", err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return generated.PluginRegistration{}, apiError("check plugin", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON200, nil
}

// PluginLogs reads bounded, redacted supervision output.
func (c *Client) PluginLogs(ctx context.Context, id string, limit int) ([]generated.PluginLogEntry, error) {
	response, err := c.generated.ListPluginLogsWithResponse(ctx, id, &generated.ListPluginLogsParams{Limit: &limit})
	if err != nil {
		return nil, fmt.Errorf("list plugin logs: %w", err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return nil, apiError("list plugin logs", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON200, nil
}

// InspectPlugin reads structured facts through reviewed scopes.
func (c *Client) InspectPlugin(ctx context.Context, id, projectID string) (generated.PluginInspection, error) {
	response, err := c.generated.InspectProjectWithPluginWithResponse(ctx, id, projectID)
	if err != nil {
		return generated.PluginInspection{}, fmt.Errorf("inspect with plugin: %w", err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return generated.PluginInspection{}, apiError("inspect with plugin", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON200, nil
}

// CreatePluginOperation queues one durable typed plugin action.
func (c *Client) CreatePluginOperation(ctx context.Context, id, projectID, action string, input map[string]any, key string) (generated.Operation, error) {
	response, err := c.generated.CreatePluginOperationWithResponse(ctx, id, projectID,
		&generated.CreatePluginOperationParams{IdempotencyKey: key}, generated.PluginOperationRequest{Action: action, Input: input})
	if err != nil {
		return generated.Operation{}, fmt.Errorf("create plugin operation: %w", err)
	}
	if response.StatusCode() != http.StatusAccepted || response.JSON202 == nil {
		return generated.Operation{}, apiError("create plugin operation", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON202, nil
}
