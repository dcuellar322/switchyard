package httpclient

import (
	"context"
	"fmt"
	"net/http"

	"switchyard.dev/switchyard/internal/transport/contract/generated"
)

// Diagnose runs deterministic diagnosis and optionally requests one configured provider.
func (c *Client) Diagnose(ctx context.Context, projectID, provider string) (generated.Diagnosis, error) {
	request := generated.CreateDiagnosisRequest{}
	if provider != "" {
		request.Provider = &provider
	}
	response, err := c.generated.CreateProjectDiagnosisWithResponse(ctx, projectID, request)
	if err != nil {
		return generated.Diagnosis{}, fmt.Errorf("diagnose project: %w", err)
	}
	if response.StatusCode() != http.StatusCreated || response.JSON201 == nil {
		return generated.Diagnosis{}, apiError("diagnose project", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON201, nil
}

// LatestDiagnosis reads the latest durable project result.
func (c *Client) LatestDiagnosis(ctx context.Context, projectID string) (generated.Diagnosis, error) {
	response, err := c.generated.GetLatestProjectDiagnosisWithResponse(ctx, projectID)
	if err != nil {
		return generated.Diagnosis{}, fmt.Errorf("read latest diagnosis: %w", err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return generated.Diagnosis{}, apiError("read latest diagnosis", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON200, nil
}

// RecordDiagnosticFeedback stores local-only review.
func (c *Client) RecordDiagnosticFeedback(ctx context.Context, diagnosisID, hypothesisID, verdict, note string) (generated.DiagnosticFeedback, error) {
	request := generated.DiagnosticFeedbackRequest{HypothesisId: hypothesisID, Verdict: generated.DiagnosticFeedbackRequestVerdict(verdict)}
	if note != "" {
		request.Note = &note
	}
	response, err := c.generated.CreateDiagnosticFeedbackWithResponse(ctx, diagnosisID, request)
	if err != nil {
		return generated.DiagnosticFeedback{}, fmt.Errorf("record diagnostic feedback: %w", err)
	}
	if response.StatusCode() != http.StatusCreated || response.JSON201 == nil {
		return generated.DiagnosticFeedback{}, apiError("record diagnostic feedback", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON201, nil
}

// RunDiagnosticAction queues one action already validated against a diagnosis.
func (c *Client) RunDiagnosticAction(ctx context.Context, diagnosisID, actionID, key string) (generated.Operation, error) {
	response, err := c.generated.CreateDiagnosticActionOperationWithResponse(ctx, diagnosisID, actionID, &generated.CreateDiagnosticActionOperationParams{IdempotencyKey: key})
	if err != nil {
		return generated.Operation{}, fmt.Errorf("run diagnostic action: %w", err)
	}
	if response.StatusCode() != http.StatusAccepted || response.JSON202 == nil {
		return generated.Operation{}, apiError("run diagnostic action", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON202, nil
}

// AutomationRecipes lists all saved recipes.
func (c *Client) AutomationRecipes(ctx context.Context, projectID string) ([]generated.AutomationRecipe, error) {
	params := &generated.ListAutomationRecipesParams{}
	if projectID != "" {
		params.ProjectId = &projectID
	}
	response, err := c.generated.ListAutomationRecipesWithResponse(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("list automation recipes: %w", err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return nil, apiError("list automation recipes", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON200, nil
}

// CreateAutomationRecipe saves one disabled recipe.
func (c *Client) CreateAutomationRecipe(ctx context.Context, request generated.CreateAutomationRecipeRequest) (generated.AutomationRecipe, error) {
	response, err := c.generated.CreateAutomationRecipeWithResponse(ctx, request)
	if err != nil {
		return generated.AutomationRecipe{}, fmt.Errorf("create automation recipe: %w", err)
	}
	if response.StatusCode() != http.StatusCreated || response.JSON201 == nil {
		return generated.AutomationRecipe{}, apiError("create automation recipe", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON201, nil
}

// SetAutomationRecipeEnabled explicitly enables or disables a recipe.
func (c *Client) SetAutomationRecipeEnabled(ctx context.Context, id string, enabled bool) (generated.AutomationRecipe, error) {
	response, err := c.generated.UpdateAutomationRecipeWithResponse(ctx, id, generated.UpdateAutomationRecipeRequest{Enabled: enabled})
	if err != nil {
		return generated.AutomationRecipe{}, fmt.Errorf("update automation recipe: %w", err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return generated.AutomationRecipe{}, apiError("update automation recipe", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON200, nil
}

// EvaluateAutomations evaluates one project's deterministic triggers now.
func (c *Client) EvaluateAutomations(ctx context.Context, projectID string) (generated.AutomationEvaluation, error) {
	response, err := c.generated.CreateAutomationEvaluationWithResponse(ctx, projectID)
	if err != nil {
		return generated.AutomationEvaluation{}, fmt.Errorf("evaluate automation recipes: %w", err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return generated.AutomationEvaluation{}, apiError("evaluate automation recipes", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON200, nil
}

// DiagnosticNotifications reads local deduplicated warnings.
func (c *Client) DiagnosticNotifications(ctx context.Context, projectID string, includeAcknowledged bool) ([]generated.DiagnosticNotification, error) {
	limit := 100
	params := &generated.ListDiagnosticNotificationsParams{IncludeAcknowledged: &includeAcknowledged, Limit: &limit}
	if projectID != "" {
		params.ProjectId = &projectID
	}
	response, err := c.generated.ListDiagnosticNotificationsWithResponse(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("list diagnostic notifications: %w", err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return nil, apiError("list diagnostic notifications", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON200, nil
}

// AcknowledgeDiagnosticNotification marks one local warning reviewed.
func (c *Client) AcknowledgeDiagnosticNotification(ctx context.Context, id string) (generated.DiagnosticNotification, error) {
	response, err := c.generated.AcknowledgeDiagnosticNotificationWithResponse(ctx, id)
	if err != nil {
		return generated.DiagnosticNotification{}, fmt.Errorf("acknowledge diagnostic notification: %w", err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return generated.DiagnosticNotification{}, apiError("acknowledge diagnostic notification", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON200, nil
}
