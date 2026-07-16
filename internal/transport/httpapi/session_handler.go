package httpapi

import (
	"encoding/json"
	"net/http"
	"time"

	"switchyard.dev/switchyard/internal/transport/contract/generated"
)

func (h *handler) CreateBrowserBootstrapToken(w http.ResponseWriter, r *http.Request) {
	if identityFrom(r.Context()).Access != accessIPC {
		writeProblem(w, r, http.StatusNotFound, "ENDPOINT_NOT_FOUND", "Endpoint not found", "Bootstrap tokens are available only over privileged local IPC.")
		return
	}
	bootstrap, err := h.sessions.IssueBootstrap()
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusCreated, generated.BrowserBootstrap{
		Token: bootstrap.Token, ExpiresAt: bootstrap.ExpiresAt,
	})
}

func (h *handler) CreateBrowserSession(w http.ResponseWriter, r *http.Request) {
	var request generated.CreateBrowserSessionRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "REQUEST_INVALID", "Request body invalid", "Provide exactly one bootstrapToken string.")
		return
	}
	session, err := h.sessions.Exchange(request.BootstrapToken)
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName, Value: session.ID, Path: "/", HttpOnly: true,
		SameSite: http.SameSiteStrictMode, MaxAge: int(time.Until(session.ExpiresAt).Seconds()),
	})
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusCreated, generated.BrowserSession{
		CsrfToken: session.CSRFToken, ExpiresAt: session.ExpiresAt,
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
