package httpapi

import (
	"encoding/json"
	"net/http"

	catalogApplication "switchyard.dev/switchyard/internal/catalog/application"
	"switchyard.dev/switchyard/internal/transport/contract/generated"
)

func (h *handler) CreateManifestProposal(w http.ResponseWriter, r *http.Request, _ generated.CreateManifestProposalParams) {
	var request generated.CreateManifestProposalRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil || request.Path == "" {
		writeProblem(w, r, http.StatusBadRequest, "REQUEST_INVALID", "Request body invalid", "Provide exactly one non-empty repository path.")
		return
	}
	_, proposal, err := h.catalog.ScanAs(r.Context(), request.Path, catalogMutationActor(r))
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, proposal)
}

func (h *handler) GetManifestProposal(w http.ResponseWriter, r *http.Request, proposalID generated.ProposalId) {
	proposal, err := h.catalog.GetProposal(r.Context(), proposalID)
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, proposal)
}

func (h *handler) ValidateManifestProposal(w http.ResponseWriter, r *http.Request, proposalID generated.ProposalId, _ generated.ValidateManifestProposalParams) {
	proposal, err := h.catalog.Validate(r.Context(), proposalID)
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, proposal)
}

func (h *handler) AcceptManifestProposal(w http.ResponseWriter, r *http.Request, proposalID generated.ProposalId, _ generated.AcceptManifestProposalParams) {
	project, proposal, err := h.catalog.AcceptAs(r.Context(), proposalID, catalogMutationActor(r))
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"project": project, "proposal": proposal})
}

func (h *handler) ListProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := h.catalog.ListProjects(r.Context())
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, projects)
}

func (h *handler) GetProject(w http.ResponseWriter, r *http.Request, projectID generated.ProjectId) {
	project, err := h.catalog.GetProject(r.Context(), projectID)
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, project)
}

func (h *handler) TrustProject(w http.ResponseWriter, r *http.Request, projectID generated.ProjectId, _ generated.TrustProjectParams) {
	project, proposal, err := h.catalog.TrustProjectAs(r.Context(), projectID, catalogMutationActor(r))
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"project": project, "proposal": proposal})
}

func (h *handler) RemoveProject(w http.ResponseWriter, r *http.Request, projectID generated.ProjectId, _ generated.RemoveProjectParams) {
	if err := h.catalog.RemoveProjectAs(r.Context(), projectID, catalogMutationActor(r)); err != nil {
		writeApplicationError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func catalogMutationActor(r *http.Request) catalogApplication.MutationActor {
	identity := identityFrom(r.Context())
	return catalogApplication.MutationActor{Type: string(identity.Access), ID: identity.ActorID}
}

func (h *handler) ExplainProjectManifest(w http.ResponseWriter, r *http.Request, projectID generated.ProjectId) {
	effective, err := h.catalog.EffectiveManifest(r.Context(), projectID, nil)
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, effective)
}

func (h *handler) DiffProjectManifest(w http.ResponseWriter, r *http.Request, projectID generated.ProjectId) {
	diff, err := h.catalog.Diff(r.Context(), projectID)
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, diff)
}

func (h *handler) ValidateProjectManifest(w http.ResponseWriter, r *http.Request, projectID generated.ProjectId) {
	validation, err := h.catalog.ValidateProject(r.Context(), projectID)
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, validation)
}
