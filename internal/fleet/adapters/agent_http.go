package adapters

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	fleetApplication "switchyard.dev/switchyard/internal/fleet/application"
	"switchyard.dev/switchyard/internal/fleet/domain"
)

const (
	protocolHeader  = "X-Switchyard-Protocol"
	maximumBodySize = 64 << 10
)

type remoteAgent interface {
	Identity(string) (domain.Identity, error)
	Snapshot(context.Context, string) (domain.Snapshot, error)
	Operate(context.Context, string, domain.OperationRequest) (domain.OperationReceipt, error)
}

// NewAgentHandler creates the intentionally narrow remote-agent transport. The
// enclosing listener must require and verify client certificates; this handler
// additionally derives the certificate identity used for application grants.
func NewAgentHandler(agent remoteAgent) http.Handler {
	mux := http.NewServeMux()
	handler := &agentHandler{agent: agent}
	mux.HandleFunc("GET /remote/v1/identity", handler.identity)
	mux.HandleFunc("GET /remote/v1/snapshot", handler.snapshot)
	mux.HandleFunc("POST /remote/v1/operations", handler.operate)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(protocolHeader, domain.ProtocolVersion)
		if r.Header.Get(protocolHeader) != domain.ProtocolVersion {
			writeRemoteError(w, http.StatusUpgradeRequired, "PROTOCOL_INCOMPATIBLE", "A compatible remote protocol header is required.")
			return
		}
		mux.ServeHTTP(w, r)
	})
}

type agentHandler struct{ agent remoteAgent }

func (h *agentHandler) identity(w http.ResponseWriter, r *http.Request) {
	fingerprint, ok := controllerFingerprint(w, r)
	if !ok {
		return
	}
	identity, err := h.agent.Identity(fingerprint)
	if err != nil {
		writeRemoteApplicationError(w, err)
		return
	}
	writeRemoteJSON(w, http.StatusOK, identity)
}

func (h *agentHandler) snapshot(w http.ResponseWriter, r *http.Request) {
	fingerprint, ok := controllerFingerprint(w, r)
	if !ok {
		return
	}
	snapshot, err := h.agent.Snapshot(r.Context(), fingerprint)
	if err != nil {
		writeRemoteApplicationError(w, err)
		return
	}
	writeRemoteJSON(w, http.StatusOK, snapshot)
}

func (h *agentHandler) operate(w http.ResponseWriter, r *http.Request) {
	fingerprint, ok := controllerFingerprint(w, r)
	if !ok {
		return
	}
	var request domain.OperationRequest
	if err := decodeRemoteJSON(w, r, &request); err != nil {
		writeRemoteError(w, http.StatusBadRequest, "REQUEST_INVALID", "The remote operation request is invalid.")
		return
	}
	receipt, err := h.agent.Operate(r.Context(), fingerprint, request)
	if err != nil {
		writeRemoteApplicationError(w, err)
		return
	}
	writeRemoteJSON(w, http.StatusAccepted, receipt)
}

func controllerFingerprint(w http.ResponseWriter, r *http.Request) (string, bool) {
	if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 || len(r.TLS.PeerCertificates[0].Raw) == 0 {
		writeRemoteError(w, http.StatusUnauthorized, "CLIENT_IDENTITY_REQUIRED", "A verified client certificate is required.")
		return "", false
	}
	digest := sha256.Sum256(r.TLS.PeerCertificates[0].Raw)
	return hex.EncodeToString(digest[:]), true
}

func decodeRemoteJSON(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maximumBodySize)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request contains multiple JSON values")
	}
	return nil
}

type remoteProblem struct {
	Code   string `json:"code"`
	Detail string `json:"detail"`
}

func writeRemoteApplicationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, fleetApplication.ErrPermissionDenied):
		writeRemoteError(w, http.StatusForbidden, "CAPABILITY_DENIED", "The controller certificate is not granted this capability.")
	case errors.Is(err, fleetApplication.ErrConfirmationNeeded):
		writeRemoteError(w, http.StatusConflict, "CONFIRMATION_REQUIRED", "Explicit risk confirmation is required.")
	default:
		writeRemoteError(w, http.StatusServiceUnavailable, "REMOTE_UNAVAILABLE", "The requested local application service is unavailable.")
	}
}

func writeRemoteError(w http.ResponseWriter, status int, code, detail string) {
	writeRemoteJSON(w, status, remoteProblem{Code: code, Detail: detail})
}

func writeRemoteJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func normalizeRemoteEndpoint(value string) string {
	return strings.TrimSuffix(value, "/")
}
