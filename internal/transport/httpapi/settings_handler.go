package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"

	settingsApplication "switchyard.dev/switchyard/internal/settings/application"
	settingsDomain "switchyard.dev/switchyard/internal/settings/domain"
	"switchyard.dev/switchyard/internal/transport/contract/generated"
)

func (h *handler) GetDaemonSettings(w http.ResponseWriter, r *http.Request) {
	status, err := h.settings.Status(r.Context())
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (h *handler) UpdateDaemonSettings(w http.ResponseWriter, r *http.Request, _ generated.UpdateDaemonSettingsParams) {
	var request generated.UpdateDaemonSettingsRequest
	if err := decodeTeamBody(w, r, &request, 64<<10); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "SETTINGS_REQUEST_INVALID", "Settings request invalid", "Provide one complete generated settings document and its current revision.")
		return
	}
	settings, err := settingsFromGenerated(request.Settings)
	if err != nil {
		writeProblem(w, r, http.StatusBadRequest, "SETTINGS_REQUEST_INVALID", "Settings request invalid", "The generated settings document could not be decoded.")
		return
	}
	identity := identityFrom(r.Context())
	status, err := h.settings.Update(r.Context(), request.ExpectedRevision, settings, settingsApplication.Actor{Type: string(identity.Access), ID: identity.ActorID})
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func settingsFromGenerated(value generated.DaemonSettings) (settingsDomain.Settings, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return settingsDomain.Settings{}, err
	}
	var settings settingsDomain.Settings
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	err = decoder.Decode(&settings)
	return settings, err
}
