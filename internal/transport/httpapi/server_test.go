package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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
	handler := New(
		systemStub{info: application.Info{
			Status: "ready", Version: "0.1.0", Commit: "abc", APIVersion: "v1",
			DatabaseSchemaVersion: 1, StartedAt: startedAt,
		}},
		http.NotFoundHandler(),
		http.NotFoundHandler(),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
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
