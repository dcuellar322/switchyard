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

	resources "switchyard.dev/switchyard/internal/observability/application"
	"switchyard.dev/switchyard/internal/observability/domain"
	"switchyard.dev/switchyard/internal/transport/contract/generated"
)

type resourceStub struct {
	overview domain.ResourceOverview
	storage  domain.StorageInventory
	preview  domain.CleanupPreview
	history  domain.MetricHistory
	err      error
	query    resourceHistoryQuery
}

type resourceHistoryQuery struct {
	projectID, service, resolution string
	from, to                       time.Time
	maxPoints                      int
}

func (s *resourceStub) Overview(context.Context) (domain.ResourceOverview, error) {
	return s.overview, s.err
}

func (s *resourceStub) Storage(context.Context) (domain.StorageInventory, error) {
	return s.storage, s.err
}

func (s *resourceStub) CleanupPreview(context.Context, string) (domain.CleanupPreview, error) {
	return s.preview, s.err
}

func (s *resourceStub) History(_ context.Context, projectID, service, resolution string, from, to time.Time, maxPoints int) (domain.MetricHistory, error) {
	s.query = resourceHistoryQuery{projectID: projectID, service: service, resolution: resolution, from: from, to: to, maxPoints: maxPoints}
	return s.history, s.err
}

func resourceTestRouter(service resourceService) http.Handler {
	return NewIPC(Dependencies{Resources: service, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
}

func TestResourceOverviewUsesBoundedGeneratedContract(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	service := &resourceStub{overview: domain.ResourceOverview{
		ObservedAt: at, Projects: []domain.ProjectSnapshot{},
		Storage:   domain.StorageSummary{Classification: domain.StorageShared},
		Footprint: domain.Footprint{Classification: "exclusive"},
		Retention: domain.RetentionPolicy{SampleIntervalSeconds: 10, RawSeconds: 3600, MinuteSeconds: 86400, QuarterHourSeconds: 2592000, MaximumHistoryPoints: 1000, LogSeconds: 604800, LogBytes: 256 << 20},
		Warnings:  []string{},
	}}
	response := httptest.NewRecorder()
	resourceTestRouter(service).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/resources", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var result generated.ResourceOverview
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if !result.ObservedAt.Equal(at) || result.Projects == nil || result.Warnings == nil || result.Retention.MaximumHistoryPoints != 1000 {
		t.Fatalf("overview = %#v", result)
	}
}

func TestMetricHistoryParsesBoundedQuery(t *testing.T) {
	t.Parallel()
	from := time.Date(2026, 7, 16, 10, 0, 0, 0, time.UTC)
	to := from.Add(time.Hour)
	service := &resourceStub{history: domain.MetricHistory{
		ProjectID: "project-1", ServiceID: "api", ResolutionSeconds: 60,
		From: from, To: to, Points: []domain.MetricPoint{},
	}}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/projects/project-1/metrics/history?service=api&resolution=1m&maxPoints=120&from="+from.Format(time.RFC3339)+"&to="+to.Format(time.RFC3339), nil)
	resourceTestRouter(service).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if service.query.projectID != "project-1" || service.query.service != "api" || service.query.resolution != "1m" || service.query.maxPoints != 120 || !service.query.from.Equal(from) || !service.query.to.Equal(to) {
		t.Fatalf("query = %#v", service.query)
	}
}

func TestInvalidResourceQueryIsProblemDetail(t *testing.T) {
	t.Parallel()
	service := &resourceStub{err: resources.ErrInvalidResourceQuery}
	response := httptest.NewRecorder()
	resourceTestRouter(service).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/projects/project-1/metrics/history?from=2026-07-16T10:00:00Z&to=2026-07-16T11:00:00Z", nil))
	if response.Code != http.StatusBadRequest || response.Header().Get("Content-Type") != "application/problem+json" {
		t.Fatalf("status = %d, content-type = %q, body = %s", response.Code, response.Header().Get("Content-Type"), response.Body.String())
	}
	var problem problemDetails
	if err := json.NewDecoder(response.Body).Decode(&problem); err != nil {
		t.Fatal(err)
	}
	if problem.Code != "RESOURCE_QUERY_INVALID" {
		t.Fatalf("problem = %#v", problem)
	}
}

func TestCleanupPreviewContractCannotExecute(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	service := &resourceStub{preview: domain.CleanupPreview{
		ProjectID: "", Risk: "destructive", Executable: false, Resources: []domain.StorageResource{}, Warnings: []string{"Preview only."}, ObservedAt: at,
	}}
	response := httptest.NewRecorder()
	resourceTestRouter(service).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/resources/cleanup-preview", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var preview generated.CleanupPreview
	if err := json.NewDecoder(response.Body).Decode(&preview); err != nil {
		t.Fatal(err)
	}
	if preview.Executable || preview.Risk != generated.CleanupPreviewRiskDestructive || preview.Resources == nil {
		t.Fatalf("preview = %#v", preview)
	}
}
