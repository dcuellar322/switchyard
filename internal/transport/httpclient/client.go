// Package httpclient is the typed local API adapter used by CLI clients.
package httpclient

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"switchyard.dev/switchyard/internal/transport/contract/generated"
)

// Client wraps the generated transport without leaking response mechanics.
type Client struct {
	generated *generated.ClientWithResponses
}

// New creates a typed client for a daemon address.
func New(address string) (*Client, error) {
	baseURL := address
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		baseURL = "http://" + baseURL
	}
	client, err := generated.NewClientWithResponses(strings.TrimRight(baseURL, "/") + "/api/v1")
	if err != nil {
		return nil, fmt.Errorf("create local API client: %w", err)
	}
	return &Client{generated: client}, nil
}

// System returns the generated system contract.
func (c *Client) System(ctx context.Context) (generated.SystemInfo, error) {
	response, err := c.generated.GetSystemWithResponse(ctx)
	if err != nil {
		return generated.SystemInfo{}, fmt.Errorf("request system status: %w", err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return generated.SystemInfo{}, fmt.Errorf("request system status: unexpected HTTP %d", response.StatusCode())
	}
	return *response.JSON200, nil
}
