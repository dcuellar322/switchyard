package httpapi

import (
	"encoding/json"
	"net/http"

	diagnosticsDomain "switchyard.dev/switchyard/internal/diagnostics/domain"
	operations "switchyard.dev/switchyard/internal/operations/application"
	"switchyard.dev/switchyard/internal/transport/contract/generated"
)

func (h *handler) CreateProjectDiagnosis(w http.ResponseWriter, r *http.Request, projectID generated.ProjectId) {
	var request generated.CreateDiagnosisRequest
	if err := decodePluginBody(w, r, &request, 4<<10); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "REQUEST_INVALID", "Request body invalid", "Provide an optional configured provider identifier.")
		return
	}
	provider := ""
	if request.Provider != nil {
		provider = *request.Provider
	}
	diagnosis, err := h.diagnostics.Diagnose(r.Context(), projectID, provider)
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, diagnosisResponse(diagnosis))
}

func (h *handler) GetLatestProjectDiagnosis(w http.ResponseWriter, r *http.Request, projectID generated.ProjectId) {
	diagnosis, err := h.diagnostics.Latest(r.Context(), projectID)
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, diagnosisResponse(diagnosis))
}

func (h *handler) GetDiagnosis(w http.ResponseWriter, r *http.Request, diagnosisID generated.DiagnosisId) {
	diagnosis, err := h.diagnostics.Get(r.Context(), diagnosisID)
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, diagnosisResponse(diagnosis))
}

func (h *handler) CreateDiagnosticFeedback(w http.ResponseWriter, r *http.Request, diagnosisID generated.DiagnosisId) {
	var request generated.DiagnosticFeedbackRequest
	if err := decodePluginBody(w, r, &request, 4<<10); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "REQUEST_INVALID", "Request body invalid", "Choose accurate or false_positive and an optional short local note.")
		return
	}
	note := ""
	if request.Note != nil {
		note = *request.Note
	}
	feedback, err := h.diagnostics.RecordFeedback(r.Context(), diagnosisID, request.HypothesisId, string(request.Verdict), note)
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, feedbackResponse(feedback))
}

func (h *handler) CreateDiagnosticActionOperation(w http.ResponseWriter, r *http.Request, diagnosisID generated.DiagnosisId, actionID generated.ActionId, params generated.CreateDiagnosticActionOperationParams) {
	diagnosis, err := h.diagnostics.AuthorizeAction(r.Context(), diagnosisID, actionID)
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	identity := identityFrom(r.Context())
	input, _ := json.Marshal(map[string]any{
		"actionId": actionID, "confirmRisk": false, "allowOutsideRoot": false,
		"actorType": string(identity.Access), "actorId": identity.ActorID,
	})
	operation, err := h.operations.Submit(r.Context(), operations.SubmitRequest{
		ProjectID: diagnosis.ProjectID, Kind: "action.run", Input: input, IdempotencyKey: params.IdempotencyKey,
		ActorType: string(identity.Access), ActorID: identity.ActorID,
	})
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, operationResponse(operation))
}

func (h *handler) ListAutomationRecipes(w http.ResponseWriter, r *http.Request, params generated.ListAutomationRecipesParams) {
	projectID := ""
	if params.ProjectId != nil {
		projectID = *params.ProjectId
	}
	recipes, err := h.automations.List(r.Context(), projectID)
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, recipeResponses(recipes))
}

func (h *handler) CreateAutomationRecipe(w http.ResponseWriter, r *http.Request) {
	var request generated.CreateAutomationRecipeRequest
	if err := decodePluginBody(w, r, &request, 8<<10); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "REQUEST_INVALID", "Request body invalid", "Provide a project, supported trigger, approved action, cooldown, and daily limit.")
		return
	}
	recipe, err := h.automations.Save(r.Context(), request.ProjectId, request.Name, string(request.TriggerCode), request.ActionId, request.CooldownSeconds, request.MaxRunsPerDay)
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, recipeResponse(recipe))
}

func (h *handler) UpdateAutomationRecipe(w http.ResponseWriter, r *http.Request, recipeID generated.RecipeId) {
	var request generated.UpdateAutomationRecipeRequest
	if err := decodePluginBody(w, r, &request, 4<<10); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "REQUEST_INVALID", "Request body invalid", "Set enabled to true or false.")
		return
	}
	recipe, err := h.automations.SetEnabled(r.Context(), recipeID, request.Enabled)
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, recipeResponse(recipe))
}

func (h *handler) CreateAutomationEvaluation(w http.ResponseWriter, r *http.Request, projectID generated.ProjectId) {
	operationIDs, err := h.automations.Evaluate(r.Context(), projectID)
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, generated.AutomationEvaluation{OperationIds: operationIDs})
}

func (h *handler) ListDiagnosticNotifications(w http.ResponseWriter, r *http.Request, params generated.ListDiagnosticNotificationsParams) {
	projectID, include, limit := "", false, 100
	if params.ProjectId != nil {
		projectID = *params.ProjectId
	}
	if params.IncludeAcknowledged != nil {
		include = *params.IncludeAcknowledged
	}
	if params.Limit != nil {
		limit = *params.Limit
	}
	values, err := h.diagnostics.Notifications(r.Context(), projectID, include, limit)
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, notificationResponses(values))
}

func (h *handler) AcknowledgeDiagnosticNotification(w http.ResponseWriter, r *http.Request, notificationID generated.NotificationId) {
	value, err := h.diagnostics.Acknowledge(r.Context(), notificationID)
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, notificationResponse(value))
}

func diagnosisResponse(value diagnosticsDomain.Diagnosis) generated.Diagnosis {
	evidence := make([]generated.DiagnosticEvidence, 0, len(value.Bundle.Evidence))
	for _, item := range value.Bundle.Evidence {
		var data any
		_ = json.Unmarshal(item.Data, &data)
		evidence = append(evidence, generated.DiagnosticEvidence{
			Id: item.ID, Kind: item.Kind, Summary: item.Summary, Source: item.Source, Data: data,
			Untrusted: item.Untrusted, Redacted: item.Redacted, Truncated: item.Truncated, ObservedAt: item.ObservedAt,
		})
	}
	hypotheses := make([]generated.DiagnosticHypothesis, 0, len(value.Hypotheses))
	for _, item := range value.Hypotheses {
		actions := make([]generated.DiagnosticSuggestedAction, 0, len(item.SuggestedActions))
		for _, action := range item.SuggestedActions {
			actions = append(actions, generated.DiagnosticSuggestedAction{ActionId: action.ActionID, Name: action.Name, Risk: generated.DiagnosticSuggestedActionRisk(action.Risk), Reason: action.Reason})
		}
		hypotheses = append(hypotheses, generated.DiagnosticHypothesis{
			Id: item.ID, Code: item.Code, Title: item.Title, Summary: item.Summary,
			Severity: generated.DiagnosticHypothesisSeverity(item.Severity), Confidence: item.Confidence,
			Source: generated.DiagnosticHypothesisSource(item.Source), EvidenceIds: item.EvidenceIDs, SuggestedActions: actions, Notifies: item.Notifies,
		})
	}
	result := generated.Diagnosis{
		Id: value.ID, Version: value.Version, ProjectId: value.ProjectID, BundleSha256: value.Bundle.SHA256,
		BundleBytes: value.Bundle.EncodedBytes, Evidence: evidence, Hypotheses: hypotheses, Warnings: value.Warnings,
		GeneratedAt: value.GeneratedAt, Deterministic: value.Deterministic,
		CleanupPreview: generated.DiagnosticCleanupPreview{
			EstimatedBytes: value.Bundle.Snapshot.Cleanup.EstimatedBytes, Candidates: value.Bundle.Snapshot.Cleanup.Candidates,
			UnknownSizes: value.Bundle.Snapshot.Cleanup.UnknownSizes, Executable: false,
		},
	}
	if value.Provider != "" {
		result.Provider = &value.Provider
	}
	if value.Model != "" {
		result.Model = &value.Model
	}
	return result
}

func feedbackResponse(value diagnosticsDomain.Feedback) generated.DiagnosticFeedback {
	result := generated.DiagnosticFeedback{Id: value.ID, DiagnosisId: value.DiagnosisID, HypothesisId: value.HypothesisID, Verdict: generated.DiagnosticFeedbackVerdict(value.Verdict), CreatedAt: value.CreatedAt}
	if value.Note != "" {
		result.Note = &value.Note
	}
	return result
}

func recipeResponses(values []diagnosticsDomain.Recipe) []generated.AutomationRecipe {
	result := make([]generated.AutomationRecipe, 0, len(values))
	for _, value := range values {
		result = append(result, recipeResponse(value))
	}
	return result
}

func recipeResponse(value diagnosticsDomain.Recipe) generated.AutomationRecipe {
	result := generated.AutomationRecipe{
		Id: value.ID, ProjectId: value.ProjectID, Name: value.Name, TriggerCode: generated.AutomationTrigger(value.TriggerCode), ActionId: value.ActionID,
		Enabled: value.Enabled, CooldownSeconds: value.CooldownSeconds, MaxRunsPerDay: value.MaxRunsPerDay,
		LastRunAt: value.LastRunAt, RunsToday: value.RunsToday, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
	if value.RunsDay != "" {
		result.RunsDay = &value.RunsDay
	}
	return result
}

func notificationResponses(values []diagnosticsDomain.Notification) []generated.DiagnosticNotification {
	result := make([]generated.DiagnosticNotification, 0, len(values))
	for _, value := range values {
		result = append(result, notificationResponse(value))
	}
	return result
}

func notificationResponse(value diagnosticsDomain.Notification) generated.DiagnosticNotification {
	return generated.DiagnosticNotification{
		Id: value.ID, ProjectId: value.ProjectID, Code: value.Code, Title: value.Title, Detail: value.Detail,
		Occurrences: value.Occurrences, FirstSeenAt: value.FirstSeenAt, LastSeenAt: value.LastSeenAt, AcknowledgedAt: value.AcknowledgedAt,
	}
}
