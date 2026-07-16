package httpapi

import (
	"encoding/json"
	"net/http"

	agents "switchyard.dev/switchyard/internal/agents/application"
	operations "switchyard.dev/switchyard/internal/operations/application"
	"switchyard.dev/switchyard/internal/transport/contract/generated"
)

func (h *handler) ListAIProposalProviders(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.ai.Providers(r.Context()))
}

func (h *handler) PreviewAIManifestEvidence(w http.ResponseWriter, r *http.Request, proposalID generated.ProposalId, _ generated.PreviewAIManifestEvidenceParams) {
	var request generated.AIGenerationLimits
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "REQUEST_INVALID", "Request body invalid", "Provide supported assisted-onboarding limits.")
		return
	}
	preview, err := h.ai.Preview(r.Context(), proposalID, generationLimits(request))
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, preview)
}

func (h *handler) CreateAIManifestEnhancement(w http.ResponseWriter, r *http.Request, proposalID generated.ProposalId, params generated.CreateAIManifestEnhancementParams) {
	var request generated.CreateAIManifestEnhancementRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil || request.Provider == "" {
		writeProblem(w, r, http.StatusBadRequest, "REQUEST_INVALID", "Request body invalid", "Choose one configured provider and supported limits.")
		return
	}
	proposal, err := h.catalog.GetProposal(r.Context(), proposalID)
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	limits, err := generationLimits(request.Limits).Normalize()
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	identity := identityFrom(r.Context())
	input, _ := json.Marshal(map[string]any{"proposalId": proposalID, "provider": request.Provider, "limits": limits})
	operation, err := h.operations.Submit(r.Context(), operations.SubmitRequest{
		ProjectID: proposal.ProjectID, Kind: "manifest.enhance", IdempotencyKey: params.IdempotencyKey,
		Input: input, ActorType: string(identity.Access), ActorID: identity.ActorID,
	})
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, operationResponse(operation))
}

func (h *handler) GetAIManifestEnhancement(w http.ResponseWriter, r *http.Request, proposalID generated.ProposalId, operationID generated.OperationId) {
	run, err := h.ai.GetRun(r.Context(), operationID)
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	if run.SourceProposalID != proposalID {
		writeProblem(w, r, http.StatusNotFound, "AI_RUN_NOT_FOUND", "Assisted onboarding run not found", "No assisted run exists for this proposal and operation.")
		return
	}
	writeJSON(w, http.StatusOK, run)
}

func generationLimits(input generated.AIGenerationLimits) agents.Limits {
	limits := agents.Limits{}
	if input.EvidenceBytes != nil {
		limits.EvidenceBytes = *input.EvidenceBytes
	}
	if input.OutputBytes != nil {
		limits.OutputBytes = *input.OutputBytes
	}
	if input.TimeoutSeconds != nil {
		limits.TimeoutSeconds = *input.TimeoutSeconds
	}
	if input.MaxTurns != nil {
		limits.MaxTurns = *input.MaxTurns
	}
	if input.MaxOutputTokens != nil {
		limits.MaxOutputTokens = *input.MaxOutputTokens
	}
	if input.MaxBudgetUsd != nil {
		limits.MaxBudgetUSD = *input.MaxBudgetUsd
	}
	return limits
}
