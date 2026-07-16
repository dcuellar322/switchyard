package adapters

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	fleetApplication "switchyard.dev/switchyard/internal/fleet/application"
	"switchyard.dev/switchyard/internal/fleet/domain"
)

type agentHTTPStub struct {
	fingerprint string
	request     domain.OperationRequest
	err         error
}

func (a *agentHTTPStub) Identity(fingerprint string) (domain.Identity, error) {
	a.fingerprint = fingerprint
	return domain.Identity{ProtocolVersion: domain.ProtocolVersion, MachineID: "peer", Name: "Peer", Version: "1", OS: "linux", Architecture: "amd64"}, a.err
}

func (a *agentHTTPStub) Snapshot(_ context.Context, fingerprint string) (domain.Snapshot, error) {
	a.fingerprint = fingerprint
	return domain.Snapshot{}, a.err
}
func (a *agentHTTPStub) Operate(_ context.Context, fingerprint string, request domain.OperationRequest) (domain.OperationReceipt, error) {
	a.fingerprint, a.request = fingerprint, request
	return domain.OperationReceipt{RequestID: request.RequestID, OperationID: "op-1", State: "queued", AcceptedAt: time.Now()}, a.err
}

func authenticatedRequest(method, target, body string) (*http.Request, string) {
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.Header.Set(protocolHeader, domain.ProtocolVersion)
	raw := []byte("fixture-client-certificate")
	request.TLS = &tls.ConnectionState{PeerCertificates: []*x509.Certificate{{Raw: raw}}}
	digest := sha256.Sum256(raw)
	return request, hex.EncodeToString(digest[:])
}

func TestAgentHandlerDerivesControllerIdentityAndRejectsUnknownInput(t *testing.T) {
	t.Parallel()
	agent := &agentHTTPStub{}
	handler := NewAgentHandler(agent)

	request, expectedFingerprint := authenticatedRequest(http.MethodPost, "/remote/v1/operations", `{"requestId":"request-1","projectId":"project-1","action":"start","confirmRisk":true}`)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusAccepted || agent.fingerprint != expectedFingerprint || agent.request.Action != domain.ActionStart {
		t.Fatalf("response=%d fingerprint=%q request=%#v", response.Code, agent.fingerprint, agent.request)
	}
	if response.Header().Get(protocolHeader) != domain.ProtocolVersion || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("headers = %#v", response.Header())
	}

	request, _ = authenticatedRequest(http.MethodPost, "/remote/v1/operations", `{"requestId":"request-1","projectId":"project-1","action":"start","confirmRisk":true,"shell":"rm"}`)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("unknown field status = %d", response.Code)
	}
}

func TestAgentHandlerRequiresProtocolCertificateAndApplicationGrant(t *testing.T) {
	t.Parallel()
	agent := &agentHTTPStub{}
	handler := NewAgentHandler(agent)
	request := httptest.NewRequest(http.MethodGet, "/remote/v1/identity", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUpgradeRequired {
		t.Fatalf("missing protocol status = %d", response.Code)
	}

	request.Header.Set(protocolHeader, domain.ProtocolVersion)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("missing certificate status = %d", response.Code)
	}

	agent.err = fleetApplication.ErrPermissionDenied
	request, _ = authenticatedRequest(http.MethodGet, "/remote/v1/identity", "")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden || !strings.Contains(response.Body.String(), "CAPABILITY_DENIED") {
		t.Fatalf("denied response = %d %s", response.Code, response.Body.String())
	}

	agent.err = errors.New("secret internal detail")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if strings.Contains(response.Body.String(), "secret") {
		t.Fatalf("internal error leaked: %s", response.Body.String())
	}
}
