package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	teamApplication "switchyard.dev/switchyard/internal/team/application"
	teamDomain "switchyard.dev/switchyard/internal/team/domain"
	telemetryApplication "switchyard.dev/switchyard/internal/telemetry/application"
	"switchyard.dev/switchyard/internal/transport/contract/generated"
)

func (h *handler) ListTeamPublishers(w http.ResponseWriter, r *http.Request) {
	items, err := h.team.Publishers(r.Context())
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *handler) TrustTeamPublisher(w http.ResponseWriter, r *http.Request) {
	var request generated.TeamPublisherTrustRequest
	if err := decodeTeamBody(w, r, &request, 8<<10); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "PUBLISHER_REQUEST_INVALID", "Publisher request invalid", "Provide one reviewed base64 Ed25519 public key.")
		return
	}
	publisher, err := h.team.TrustPublisher(r.Context(), request.Name, request.PublicKey, request.ConfirmRisk, requestTeamActor(r))
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, publisher)
}

func (h *handler) ListTeamBundles(w http.ResponseWriter, r *http.Request, params generated.ListTeamBundlesParams) {
	kind := teamDomain.BundleKind("")
	if params.Kind != nil {
		kind = teamDomain.BundleKind(*params.Kind)
	}
	items, err := h.team.Bundles(r.Context(), kind)
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *handler) InstallTeamBundle(w http.ResponseWriter, r *http.Request) {
	var request generated.TeamBundleInstallRequest
	if err := decodeTeamBody(w, r, &request, 2<<20); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "BUNDLE_REQUEST_INVALID", "Bundle request invalid", "Provide one bounded signed configuration bundle.")
		return
	}
	bundle, err := generatedBundle(request.Bundle)
	if err != nil {
		writeProblem(w, r, http.StatusBadRequest, "BUNDLE_REQUEST_INVALID", "Bundle request invalid", "The bundle envelope could not be decoded.")
		return
	}
	bundle, err = h.team.Install(r.Context(), bundle, request.ConfirmRisk, requestTeamActor(r))
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, bundle)
}

func (h *handler) RenderTeamProjectTemplate(w http.ResponseWriter, r *http.Request, bundleID generated.BundleId) {
	var request generated.TeamTemplateRenderRequest
	if err := decodeTeamBody(w, r, &request, 64<<10); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "TEMPLATE_VALUES_INVALID", "Template values invalid", "Provide a bounded string value map.")
		return
	}
	manifest, err := h.team.RenderTemplate(r.Context(), bundleID, request.Values)
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(manifest)
}

func (h *handler) GetEffectiveTeamPolicy(w http.ResponseWriter, r *http.Request) {
	policy, err := h.team.EffectivePolicy(r.Context())
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, policy)
}

func (h *handler) ListCuratedPlugins(w http.ResponseWriter, r *http.Request) {
	entries, err := h.team.Registry(r.Context())
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, entries)
}

func (h *handler) ExportTeamSync(w http.ResponseWriter, r *http.Request) {
	document, err := h.team.ExportSync(r.Context())
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, document)
}

func (h *handler) PreviewTeamSync(w http.ResponseWriter, r *http.Request) {
	var request generated.TeamSyncDocument
	if err := decodeTeamBody(w, r, &request, 10<<20); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "SYNC_DOCUMENT_INVALID", "Sync document invalid", "Provide a bounded decrypted Switchyard sync document.")
		return
	}
	document, err := generatedSyncDocument(request)
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	preview, err := h.team.PreviewSync(r.Context(), document)
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, preview)
}

func (h *handler) ImportTeamSync(w http.ResponseWriter, r *http.Request) {
	var request generated.TeamSyncImportRequest
	if err := decodeTeamBody(w, r, &request, 10<<20); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "SYNC_DOCUMENT_INVALID", "Sync document invalid", "Provide the exact previewed sync document and explicit confirmation.")
		return
	}
	document, err := generatedSyncDocument(request.Document)
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	preview, err := h.team.ImportSync(r.Context(), document, request.ConfirmRisk, requestTeamActor(r))
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, preview)
}

func (h *handler) GetTelemetryStatus(w http.ResponseWriter, r *http.Request) {
	status, err := h.telemetry.Status(r.Context())
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (h *handler) UpdateTelemetrySettings(w http.ResponseWriter, r *http.Request) {
	var request generated.TelemetrySettingsRequest
	if err := decodeTeamBody(w, r, &request, 4<<10); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "TELEMETRY_SETTINGS_INVALID", "Telemetry settings invalid", "Provide an explicit HTTPS endpoint and consent.")
		return
	}
	endpoint := ""
	if request.Endpoint != nil {
		endpoint = *request.Endpoint
	}
	actor := requestTeamActor(r)
	status, err := h.telemetry.Configure(r.Context(), request.Enabled, endpoint, request.ConfirmRisk, telemetryApplication.Actor{Type: actor.Type, ID: actor.ID})
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (h *handler) SendTelemetryNow(w http.ResponseWriter, r *http.Request) {
	status, err := h.telemetry.Send(r.Context())
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func decodeTeamBody(w http.ResponseWriter, r *http.Request, target any, limit int64) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, limit))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request contains multiple JSON values")
	}
	return nil
}

func generatedBundle(value generated.TeamBundle) (teamDomain.Bundle, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return teamDomain.Bundle{}, err
	}
	var result teamDomain.Bundle
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	err = decoder.Decode(&result)
	return result, err
}

func generatedSyncDocument(value generated.TeamSyncDocument) (teamDomain.SyncDocument, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return teamDomain.SyncDocument{}, err
	}
	var result teamDomain.SyncDocument
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	err = decoder.Decode(&result)
	return result, err
}

func requestTeamActor(r *http.Request) teamApplication.Actor {
	actorType, actorID := RequestActor(r.Context())
	return teamApplication.Actor{Type: actorType, ID: actorID}
}
