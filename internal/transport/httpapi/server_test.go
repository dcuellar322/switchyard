package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	operations "switchyard.dev/switchyard/internal/operations/application"
	"switchyard.dev/switchyard/internal/operations/domain"
	session "switchyard.dev/switchyard/internal/session/application"
	"switchyard.dev/switchyard/internal/system/application"
	"switchyard.dev/switchyard/internal/transport/contract/generated"
)

type systemStub struct {
	info application.Info
}

func (s systemStub) Get(context.Context) (application.Info, error) { return s.info, nil }

func TestGetSystemReturnsGeneratedContract(t *testing.T) {
	t.Parallel()

	startedAt := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	handler := NewIPC(Dependencies{
		System: systemStub{info: application.Info{
			Status: "ready", Version: "0.1.0", Commit: "abc", APIVersion: "v1",
			DatabaseSchemaVersion: 1, StartedAt: startedAt,
		}}, Sessions: session.NewManager(), Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/system", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var info generated.SystemInfo
	if err := json.NewDecoder(response.Body).Decode(&info); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if info.Version != "0.1.0" || info.DatabaseSchemaVersion != 1 {
		t.Fatalf("response = %#v", info)
	}
	if response.Header().Get(correlationHeader) == "" {
		t.Fatal("missing correlation response header")
	}
}

type operationStub struct{}

func (operationStub) Get(context.Context, string) (domain.Operation, error) {
	return domain.Operation{}, operations.ErrNotFound
}

func (operationStub) Cancel(context.Context, string, string, string, string) (domain.Operation, error) {
	return domain.Operation{}, operations.ErrNotFound
}

func TestBrowserSessionAndCSRFSecurity(t *testing.T) {
	t.Parallel()

	sessions := session.NewManager()
	dependencies := Dependencies{
		System: systemStub{}, Operations: operationStub{}, Sessions: sessions,
		Events: http.NotFoundHandler(), Web: http.NotFoundHandler(),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	ipc := NewIPC(dependencies)
	bootstrapResponse := httptest.NewRecorder()
	ipc.ServeHTTP(bootstrapResponse, httptest.NewRequest(http.MethodPost, "/api/v1/auth/bootstrap-tokens", nil))
	if bootstrapResponse.Code != http.StatusCreated {
		t.Fatalf("bootstrap status = %d", bootstrapResponse.Code)
	}
	var bootstrap generated.BrowserBootstrap
	if err := json.NewDecoder(bootstrapResponse.Body).Decode(&bootstrap); err != nil {
		t.Fatalf("decode bootstrap: %v", err)
	}

	browser := NewBrowser(dependencies)
	unauthorized := httptest.NewRecorder()
	browser.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/api/v1/system", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d", unauthorized.Code)
	}
	body := strings.NewReader(`{"bootstrapToken":"` + bootstrap.Token + `"}`)
	exchangeRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/sessions", body)
	exchangeRequest.Header.Set("Content-Type", "application/json")
	exchangeResponse := httptest.NewRecorder()
	browser.ServeHTTP(exchangeResponse, exchangeRequest)
	if exchangeResponse.Code != http.StatusCreated {
		t.Fatalf("exchange status = %d, body = %s", exchangeResponse.Code, exchangeResponse.Body.String())
	}
	var browserSession generated.BrowserSession
	if err := json.NewDecoder(exchangeResponse.Body).Decode(&browserSession); err != nil {
		t.Fatalf("decode session: %v", err)
	}
	result := exchangeResponse.Result()
	t.Cleanup(func() { _ = result.Body.Close() })
	cookies := result.Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatalf("session cookies = %#v", cookies)
	}

	missingCSRF := httptest.NewRequest(http.MethodPost, "/api/v1/operations/missing/cancel", nil)
	missingCSRF.AddCookie(cookies[0])
	missingCSRF.Header.Set(idempotencyHeader, "cancel-key")
	missingCSRFResponse := httptest.NewRecorder()
	browser.ServeHTTP(missingCSRFResponse, missingCSRF)
	if missingCSRFResponse.Code != http.StatusForbidden {
		t.Fatalf("missing CSRF status = %d", missingCSRFResponse.Code)
	}

	wrongOrigin := httptest.NewRequest(http.MethodGet, "/ws/v1/events", nil)
	wrongOrigin.Host = "127.0.0.1:19616"
	wrongOrigin.Header.Set("Origin", "http://attacker.invalid")
	wrongOrigin.AddCookie(cookies[0])
	wrongOriginResponse := httptest.NewRecorder()
	browser.ServeHTTP(wrongOriginResponse, wrongOrigin)
	if wrongOriginResponse.Code != http.StatusForbidden {
		t.Fatalf("wrong origin status = %d", wrongOriginResponse.Code)
	}

	missingIdempotency := httptest.NewRequest(http.MethodPost, "/api/v1/operations/missing/cancel", nil)
	missingIdempotency.AddCookie(cookies[0])
	missingIdempotency.Header.Set(csrfHeader, browserSession.CsrfToken)
	missingIdempotencyResponse := httptest.NewRecorder()
	browser.ServeHTTP(missingIdempotencyResponse, missingIdempotency)
	if missingIdempotencyResponse.Code != http.StatusBadRequest {
		t.Fatalf("missing idempotency status = %d", missingIdempotencyResponse.Code)
	}

	valid := httptest.NewRequest(http.MethodPost, "/api/v1/operations/missing/cancel", nil)
	valid.AddCookie(cookies[0])
	valid.Header.Set(csrfHeader, browserSession.CsrfToken)
	valid.Header.Set(idempotencyHeader, "cancel-key")
	validResponse := httptest.NewRecorder()
	browser.ServeHTTP(validResponse, valid)
	if validResponse.Code != http.StatusNotFound {
		t.Fatalf("authorized mutation status = %d, body = %s", validResponse.Code, validResponse.Body.String())
	}
}
