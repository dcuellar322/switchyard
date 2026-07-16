package httpclient

import (
	"context"
	"fmt"
	"net/http"

	"switchyard.dev/switchyard/internal/transport/contract/generated"
)

// Machines lists explicitly configured remote peers.
func (c *Client) Machines(ctx context.Context) ([]generated.Machine, error) {
	response, err := c.generated.ListMachinesWithResponse(ctx)
	if err != nil {
		return nil, fmt.Errorf("list remote machines: %w", err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return nil, apiError("list remote machines", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON200, nil
}

// CreateMachine registers and probes one certificate-pinned peer.
func (c *Client) CreateMachine(ctx context.Context, request generated.MachineRegistrationRequest) (generated.Machine, error) {
	response, err := c.generated.CreateMachineWithResponse(ctx, request)
	if err != nil {
		return generated.Machine{}, fmt.Errorf("register remote machine: %w", err)
	}
	if response.StatusCode() != http.StatusCreated || response.JSON201 == nil {
		return generated.Machine{}, apiError("register remote machine", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON201, nil
}

// Machine reads one peer registration.
func (c *Client) Machine(ctx context.Context, id string) (generated.Machine, error) {
	response, err := c.generated.GetMachineWithResponse(ctx, id)
	if err != nil {
		return generated.Machine{}, fmt.Errorf("get remote machine: %w", err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return generated.Machine{}, apiError("get remote machine", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON200, nil
}

// DeleteMachine removes one local registration without mutating the peer.
func (c *Client) DeleteMachine(ctx context.Context, id string, confirm bool) error {
	response, err := c.generated.DeleteMachineWithResponse(ctx, id, &generated.DeleteMachineParams{ConfirmRisk: confirm})
	if err != nil {
		return fmt.Errorf("remove remote machine: %w", err)
	}
	if response.StatusCode() != http.StatusNoContent {
		return apiError("remove remote machine", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return nil
}

// UpdateMachineAccess replaces a peer's reviewed local grants.
func (c *Client) UpdateMachineAccess(ctx context.Context, id string, request generated.MachineAccessRequest) (generated.Machine, error) {
	response, err := c.generated.UpdateMachineAccessWithResponse(ctx, id, request)
	if err != nil {
		return generated.Machine{}, fmt.Errorf("update remote machine access: %w", err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return generated.Machine{}, apiError("update remote machine access", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON200, nil
}

// ProbeMachine refreshes authenticated peer identity and state.
func (c *Client) ProbeMachine(ctx context.Context, id string) (generated.Machine, error) {
	response, err := c.generated.ProbeMachineWithResponse(ctx, id)
	if err != nil {
		return generated.Machine{}, fmt.Errorf("probe remote machine: %w", err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return generated.Machine{}, apiError("probe remote machine", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON200, nil
}

// MachineSnapshot reads the current bounded inventory from a peer.
func (c *Client) MachineSnapshot(ctx context.Context, id string) (generated.FleetSnapshot, error) {
	response, err := c.generated.GetMachineSnapshotWithResponse(ctx, id)
	if err != nil {
		return generated.FleetSnapshot{}, fmt.Errorf("read remote machine snapshot: %w", err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return generated.FleetSnapshot{}, apiError("read remote machine snapshot", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON200, nil
}

// CreateMachineOperation submits a confirmed typed lifecycle operation.
func (c *Client) CreateMachineOperation(ctx context.Context, id string, request generated.RemoteOperationRequest) (generated.RemoteOperationReceipt, error) {
	response, err := c.generated.CreateMachineOperationWithResponse(ctx, id, request)
	if err != nil {
		return generated.RemoteOperationReceipt{}, fmt.Errorf("create remote machine operation: %w", err)
	}
	if response.StatusCode() != http.StatusAccepted || response.JSON202 == nil {
		return generated.RemoteOperationReceipt{}, apiError("create remote machine operation", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON202, nil
}
